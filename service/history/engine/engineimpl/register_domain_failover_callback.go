// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package engineimpl

import (
	"context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
)

func (e *historyEngineImpl) registerDomainFailoverCallback() {

	// NOTE: READ BEFORE MODIFICATION
	//
	// Tasks, e.g. transfer tasks and timer tasks, are created when holding the shard lock
	// meaning tasks -> release of shard lock
	//
	// Domain change notification follows the following steps, order matters
	// 1. lock all task processing.
	// 2. domain changes visible to everyone (Note: lock of task processing prevents task processing logic seeing the domain changes).
	// 3. failover min and max task levels are calculated, then update to shard.
	// 4. failover start & task processing unlock & shard domain version notification update. (order does not matter for this discussion)
	//
	// The above guarantees that task created during the failover will be processed.
	// If the task is created after domain change:
	// 		then active processor will handle it. (simple case)
	// If the task is created before domain change:
	//		task -> release of shard lock
	//		failover min / max task levels calculated & updated to shard (using shard lock) -> failover start
	// above 2 guarantees that failover start is after persistence of the task.

	// first set the failover callback
	e.shard.GetDomainCache().RegisterDomainChangeCallback(
		e.shard.GetShardID(),
		e.shard.GetDomainNotificationVersion(),
		e.lockProcessingForFailover,
		e.domainChangeCB,
	)
}

func (e *historyEngineImpl) domainChangeCB(nextDomains []*cache.DomainCacheEntry) {
	defer func() {
		e.unlockProcessingForFailover()
	}()

	if len(nextDomains) == 0 {
		return
	}

	shardNotificationVersion := e.shard.GetDomainNotificationVersion()
	failoverDomainIDs := map[string]struct{}{}

	for _, nextDomain := range nextDomains {
		e.failoverPredicate(shardNotificationVersion, nextDomain, func() {
			failoverDomainIDs[nextDomain.GetInfo().ID] = struct{}{}
		})
	}

	if len(failoverDomainIDs) > 0 {
		e.logger.Info("Domain Failover Start.", tag.WorkflowDomainIDs(failoverDomainIDs))

		e.txProcessor.FailoverDomain(failoverDomainIDs)
		e.timerProcessor.FailoverDomain(failoverDomainIDs)

		now := e.shard.GetTimeSource().Now()
		// the fake tasks will not be actually used, we just need to make sure
		// its length > 0 and has correct timestamp, to trigger a db scan
		fakeDecisionTask := []persistence.Task{&persistence.DecisionTask{}}
		fakeDecisionTimeoutTask := []persistence.Task{&persistence.DecisionTimeoutTask{TaskData: persistence.TaskData{VisibilityTimestamp: now}}}
		e.txProcessor.NotifyNewTask(e.currentClusterName, &hcommon.NotifyTaskInfo{Tasks: fakeDecisionTask})
		e.timerProcessor.NotifyNewTask(e.currentClusterName, &hcommon.NotifyTaskInfo{Tasks: fakeDecisionTimeoutTask})
	}

	failoverMarkerTasks := e.generateGracefulFailoverTasksForDomainUpdateCallback(shardNotificationVersion, nextDomains)

	// This is a debug metric
	e.metricsClient.IncCounter(metrics.FailoverMarkerScope, metrics.HistoryFailoverCallbackCount)
	if len(failoverMarkerTasks) > 0 {
		if err := e.shard.ReplicateFailoverMarkers(
			context.Background(),
			failoverMarkerTasks,
		); err != nil {
			e.logger.Error("Failed to insert failover marker to replication queue.", tag.Error(err))
			e.metricsClient.IncCounter(metrics.FailoverMarkerScope, metrics.FailoverMarkerInsertFailure)
			// fail this failover callback and it retries on next domain cache refresh
			return
		}
	}

	//nolint:errcheck
	e.shard.UpdateDomainNotificationVersion(nextDomains[len(nextDomains)-1].GetNotificationVersion() + 1)
}

func (e *historyEngineImpl) generateGracefulFailoverTasksForDomainUpdateCallback(shardNotificationVersion int64, nextDomains []*cache.DomainCacheEntry) []*persistence.FailoverMarkerTask {

	// handle graceful failover on active to passive
	// make sure task processor failover the domain before inserting the failover marker
	failoverMarkerTasks := []*persistence.FailoverMarkerTask{}
	for _, nextDomain := range nextDomains {
		domainFailoverNotificationVersion := nextDomain.GetFailoverNotificationVersion()
		domainActiveCluster := nextDomain.GetReplicationConfig().ActiveClusterName
		previousFailoverVersion := nextDomain.GetPreviousFailoverVersion()
		previousClusterName, err := e.clusterMetadata.ClusterNameForFailoverVersion(previousFailoverVersion)
		if err != nil && previousFailoverVersion != common.InitialPreviousFailoverVersion {
			e.logger.Error("Failed to handle graceful failover", tag.WorkflowDomainID(nextDomain.GetInfo().ID), tag.Error(err))
			continue
		}

		if nextDomain.IsGlobalDomain() &&
			domainFailoverNotificationVersion >= shardNotificationVersion &&
			domainActiveCluster != e.currentClusterName &&
			previousFailoverVersion != common.InitialPreviousFailoverVersion &&
			previousClusterName == e.currentClusterName {
			// the visibility timestamp will be set in shard context
			failoverMarkerTasks = append(failoverMarkerTasks, &persistence.FailoverMarkerTask{
				TaskData: persistence.TaskData{
					Version: nextDomain.GetFailoverVersion(),
				},
				DomainID: nextDomain.GetInfo().ID,
			})
			// This is a debug metric
			e.metricsClient.IncCounter(metrics.FailoverMarkerScope, metrics.FailoverMarkerCallbackCount)
		}
	}
	return failoverMarkerTasks
}

func (e *historyEngineImpl) lockProcessingForFailover() {
	e.logger.Debug("locking processing for failover")
	e.txProcessor.LockTaskProcessing()
	e.timerProcessor.LockTaskProcessing()
}

func (e *historyEngineImpl) unlockProcessingForFailover() {
	e.logger.Debug("unlocking processing for failover")
	e.txProcessor.UnlockTaskProcessing()
	e.timerProcessor.UnlockTaskProcessing()
}

func (e historyEngineImpl) failoverPredicate(shardNotificationVersion int64, nextDomain *cache.DomainCacheEntry, action func()) {
	domainFailoverNotificationVersion := nextDomain.GetFailoverNotificationVersion()
	domainActiveCluster := nextDomain.GetReplicationConfig().ActiveClusterName

	if nextDomain.IsGlobalDomain() &&
		domainFailoverNotificationVersion >= shardNotificationVersion &&
		domainActiveCluster == e.currentClusterName {
		action()
	}
}
