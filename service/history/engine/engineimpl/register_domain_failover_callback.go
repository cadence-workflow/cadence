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
	"sort"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/taskdlq"
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

	initialNotificationVersion := e.shard.GetDomainNotificationVersion()

	catchUpFn := func(domainCache cache.DomainCache, prepareCallback cache.PrepareCallbackFn, callback cache.CallbackFn) {
		// this section is trying to make the shard catch up with domain changes
		domains := cache.DomainCacheEntries{}
		for _, domain := range domainCache.GetAllDomain() {
			domains = append(domains, domain)
		}
		// we must notify the change in an ordered fashion
		// since history shard has to update the shard info
		// with the domain change version.
		sort.Sort(domains)

		var updatedEntries []*cache.DomainCacheEntry
		var domainNames []string
		for _, domain := range domains {
			if domain.GetNotificationVersion() >= initialNotificationVersion {
				updatedEntries = append(updatedEntries, domain)
				domainNames = append(domainNames, domain.GetInfo().Name)
			}
		}
		if len(updatedEntries) > 0 {
			e.logger.Info("catchUpFn calling prepareCallback and callback", tag.Dynamic("domain-names", domainNames))
			prepareCallback()
			callback(updatedEntries)
		}
	}

	// first set the failover callback
	e.shard.GetDomainCache().RegisterDomainChangeCallback(
		createShardNameFromShardID(e.shard.GetShardID()),
		catchUpFn,
		e.lockTaskProcessingForDomainUpdate,
		e.domainChangeCB,
	)
}

func (e *historyEngineImpl) domainChangeCB(nextDomains []*cache.DomainCacheEntry) {
	defer func() {
		e.unlockProcessingForDomainUpdate()
	}()

	if len(nextDomains) == 0 {
		return
	}

	shardNotificationVersion := e.shard.GetDomainNotificationVersion()
	failoverActivePassiveDomainIDs := map[string]struct{}{}
	failoverActiveActiveDomainIDs := map[string]struct{}{}
	// dlqPartitions collects the DLQ partitions to reprocess promptly on failover, keyed
	// precisely: active-passive domains use their single default partition; active-active
	// domains use one partition per cluster attribute now active in this cluster (never
	// still-passive attributes, which would just be re-DLQ'd).
	var dlqPartitions []taskdlq.Partition

	for _, nextDomain := range nextDomains {
		domainFailoverNotificationVersion := nextDomain.GetFailoverNotificationVersion()
		domainActiveCluster := nextDomain.GetReplicationConfig().ActiveClusterName

		if !nextDomain.IsGlobalDomain() {
			continue
		}

		if !nextDomain.GetReplicationConfig().IsActiveActive() &&
			domainFailoverNotificationVersion >= shardNotificationVersion &&
			domainActiveCluster == e.currentClusterName {
			failoverActivePassiveDomainIDs[nextDomain.GetInfo().ID] = struct{}{}
			dlqPartitions = append(dlqPartitions, taskdlq.Partition{DomainID: nextDomain.GetInfo().ID})
		}
		if nextDomain.GetReplicationConfig().IsActiveActive() &&
			domainFailoverNotificationVersion >= shardNotificationVersion &&
			nextDomain.IsActiveIn(e.currentClusterName) {
			failoverActiveActiveDomainIDs[nextDomain.GetInfo().ID] = struct{}{}
			for _, ca := range activeClusterAttributesIn(nextDomain, e.currentClusterName) {
				dlqPartitions = append(dlqPartitions, taskdlq.Partition{
					DomainID:              nextDomain.GetInfo().ID,
					ClusterAttributeScope: ca.Scope,
					ClusterAttributeName:  ca.Name,
				})
			}
		}
	}

	if len(failoverActivePassiveDomainIDs) > 0 {
		e.logger.Debug("Domain Failover Start.", tag.WorkflowDomainIDs(failoverActivePassiveDomainIDs))
		// Failover queues are not created for active-active domains. Will revisit after new queue framework implementation.
		for _, processor := range e.queueProcessors {
			processor.FailoverDomain(failoverActivePassiveDomainIDs)
		}
	}

	if len(failoverActiveActiveDomainIDs) > 0 {
		e.logger.Debug("Active-Active Domain updated", tag.WorkflowDomainIDs(failoverActiveActiveDomainIDs))
	}

	// Promptly reprocess DLQ'd tasks for the failed-over partitions instead of waiting for the
	// next periodic sweep. This is non-blocking: the partitions are queued and a background loop
	// preempts any in-progress sweep to reprocess them first, re-injecting the tasks into the
	// executions table and notifying the queues so they are picked up immediately.
	if e.dlqProcessor != nil && len(dlqPartitions) > 0 {
		e.dlqProcessor.FailoverPartitions(dlqPartitions)
	}

	// Notify queues for any domain update. (active-passive and active-active)
	if len(failoverActivePassiveDomainIDs) > 0 || len(failoverActiveActiveDomainIDs) > 0 {
		e.notifyQueues()
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

// activeClusterAttributesIn returns the cluster attributes of an active-active domain whose
// active cluster is the given cluster. This is precise by construction (no previous entry
// needed): only attributes currently active in the cluster are returned, so failover-triggered
// DLQ reprocessing never touches attributes that are still passive here.
func activeClusterAttributesIn(entry *cache.DomainCacheEntry, cluster string) []types.ClusterAttribute {
	rc := entry.GetReplicationConfig()
	if rc == nil || rc.ActiveClusters == nil {
		return nil
	}
	var out []types.ClusterAttribute
	for scope, s := range rc.ActiveClusters.AttributeScopes {
		for name, info := range s.ClusterAttributes {
			if info.ActiveClusterName == cluster {
				out = append(out, types.ClusterAttribute{Scope: scope, Name: name})
			}
		}
	}
	return out
}

func (e *historyEngineImpl) notifyQueues() {
	now := e.shard.GetTimeSource().Now()
	// the fake tasks will not be actually used, we just need to make sure
	// its length > 0 and has correct timestamp, to trigger a db scan
	fakeDecisionTask := []persistence.Task{&persistence.DecisionTask{}}
	fakeDecisionTimeoutTask := []persistence.Task{&persistence.DecisionTimeoutTask{TaskData: persistence.TaskData{VisibilityTimestamp: now}}}
	transferProcessor, ok := e.queueProcessors[persistence.HistoryTaskCategoryTransfer]
	if !ok {
		e.logger.Error("transfer processor not found")
		return
	}
	transferProcessor.NotifyNewTask(e.currentClusterName, &hcommon.NotifyTaskInfo{Tasks: fakeDecisionTask})
	timerProcessor, ok := e.queueProcessors[persistence.HistoryTaskCategoryTimer]
	if !ok {
		e.logger.Error("timer processor not found")
		return
	}
	timerProcessor.NotifyNewTask(e.currentClusterName, &hcommon.NotifyTaskInfo{Tasks: fakeDecisionTimeoutTask})
}

func (e *historyEngineImpl) generateGracefulFailoverTasksForDomainUpdateCallback(shardNotificationVersion int64, nextDomains []*cache.DomainCacheEntry) []*persistence.FailoverMarkerTask {
	// handle graceful failover on active to passive
	// make sure task processor failover the domain before inserting the failover marker
	failoverMarkerTasks := []*persistence.FailoverMarkerTask{}
	for _, nextDomain := range nextDomains {
		if nextDomain.GetReplicationConfig().IsActiveActive() {
			// Currently it's unclear whether graceful failover is working for active-passive domains. We don't use it in practice.
			// Don't try to make it work for active-active domains until we determine we need it.
			// We may potentially retire existing graceful failover implementation and provide "sync replication" instead.
			continue
		}
		domainFailoverNotificationVersion := nextDomain.GetFailoverNotificationVersion()
		domainActiveCluster := nextDomain.GetReplicationConfig().ActiveClusterName
		previousFailoverVersion := nextDomain.GetPreviousFailoverVersion()
		previousClusterName, err := e.clusterMetadata.ClusterNameForFailoverVersion(previousFailoverVersion)
		if err != nil && previousFailoverVersion != constants.InitialPreviousFailoverVersion {
			e.logger.Error("Failed to handle graceful failover", tag.WorkflowDomainID(nextDomain.GetInfo().ID), tag.Error(err))
			continue
		}

		if nextDomain.IsGlobalDomain() &&
			domainFailoverNotificationVersion >= shardNotificationVersion &&
			domainActiveCluster != e.currentClusterName &&
			previousFailoverVersion != constants.InitialPreviousFailoverVersion &&
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

func (e *historyEngineImpl) lockTaskProcessingForDomainUpdate() {
	e.logger.Debug("Locking processing for domain update")
	for _, processor := range e.queueProcessors {
		processor.LockTaskProcessing()
	}
}

func (e *historyEngineImpl) unlockProcessingForDomainUpdate() {
	e.logger.Debug("Unlocking processing for failover")
	for _, processor := range e.queueProcessors {
		processor.UnlockTaskProcessing()
	}
}

func (e *historyEngineImpl) failoverPredicate(shardNotificationVersion int64, nextDomain *cache.DomainCacheEntry, action func()) {
	domainFailoverNotificationVersion := nextDomain.GetFailoverNotificationVersion()
	domainActiveCluster := nextDomain.GetReplicationConfig().ActiveClusterName

	if nextDomain.IsGlobalDomain() &&
		!nextDomain.GetReplicationConfig().IsActiveActive() &&
		domainFailoverNotificationVersion >= shardNotificationVersion &&
		domainActiveCluster == e.currentClusterName {
		action()
	}
}
