// Copyright (c) 2021 Uber Technologies, Inc.
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

package replication

import (
	"context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/shard"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination task_executor_mock.go -self_package github.com/uber/cadence/service/history/replication

type (
	// TaskExecutor is the executor for replication task
	TaskExecutor interface {
		execute(replicationTask *types.ReplicationTask, forceApply bool) (metrics.ScopeIdx, error)
	}

	taskExecutorImpl struct {
		currentCluster  string
		sourceCluster   string
		shard           shard.Context
		domainCache     cache.DomainCache
		historyResender ndc.HistoryResender
		historyEngine   engine.Engine

		metricsClient metrics.Client
		logger        log.Logger
	}
)

var _ TaskExecutor = (*taskExecutorImpl)(nil)

// NewTaskExecutor creates an replication task executor
// The executor uses by 1) DLQ replication task handler 2) history replication task processor
func NewTaskExecutor(
	sourceCluster string,
	shard shard.Context,
	domainCache cache.DomainCache,
	historyResender ndc.HistoryResender,
	historyEngine engine.Engine,
	metricsClient metrics.Client,
	logger log.Logger,
) TaskExecutor {
	return &taskExecutorImpl{
		currentCluster:  shard.GetClusterMetadata().GetCurrentClusterName(),
		sourceCluster:   sourceCluster,
		shard:           shard,
		domainCache:     domainCache,
		historyResender: historyResender,
		historyEngine:   historyEngine,
		metricsClient:   metricsClient,
		logger:          logger,
	}
}

func (e *taskExecutorImpl) execute(
	replicationTask *types.ReplicationTask,
	forceApply bool,
) (metrics.ScopeIdx, error) {

	var err error
	var scope metrics.ScopeIdx
	switch replicationTask.GetTaskType() {
	case types.ReplicationTaskTypeSyncActivity:
		scope = metrics.SyncActivityTaskScope
		err = e.handleActivityTask(replicationTask, forceApply)
	case types.ReplicationTaskTypeHistoryV2:
		scope = metrics.HistoryReplicationV2TaskScope
		err = e.handleHistoryReplicationTaskV2(replicationTask, forceApply)
	case types.ReplicationTaskTypeFailoverMarker:
		scope = metrics.FailoverMarkerScope
		err = e.handleFailoverReplicationTask(replicationTask)
	default:
		e.logger.Error("Unknown task type.")
		scope = metrics.ReplicatorScope
		err = ErrUnknownReplicationTask
	}

	return scope, err
}

func (e *taskExecutorImpl) handleActivityTask(
	task *types.ReplicationTask,
	forceApply bool,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			attr := task.SyncActivityTaskAttributes
			e.logger.Error(
				"handleActivityTask encountered panic.",
				tag.WorkflowDomainID(attr.GetDomainID()),
				tag.WorkflowID(attr.GetWorkflowID()),
				tag.WorkflowRunID(attr.GetRunID()),
				tag.Value(r),
			)
			panic(r)
		}
	}()

	attr := task.SyncActivityTaskAttributes
	doContinue, err := e.filterTask(attr.GetDomainID(), forceApply)
	if err != nil || !doContinue {
		return err
	}

	replicationStopWatch := e.metricsClient.StartTimer(metrics.SyncActivityTaskScope, metrics.CadenceLatency)
	defer replicationStopWatch.Stop()
	request := &types.SyncActivityRequest{
		DomainID:           attr.DomainID,
		WorkflowID:         attr.WorkflowID,
		RunID:              attr.RunID,
		Version:            attr.Version,
		ScheduledID:        attr.ScheduledID,
		ScheduledTime:      attr.ScheduledTime,
		StartedID:          attr.StartedID,
		StartedTime:        attr.StartedTime,
		LastHeartbeatTime:  attr.LastHeartbeatTime,
		Details:            attr.Details,
		Attempt:            attr.Attempt,
		LastFailureReason:  attr.LastFailureReason,
		LastFailureDetails: attr.LastFailureDetails,
		LastWorkerIdentity: attr.LastWorkerIdentity,
		VersionHistory:     attr.GetVersionHistory(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), replicationTimeout)
	defer cancel()

	var syncActivityAction func() error
	// Check if the number of shards between clusters are equal. If not, redirect the request.
	if e.shard.GetShardID() != common.WorkflowIDToHistoryShard(attr.WorkflowID, e.shard.GetConfig().NumberOfShards) {
		syncActivityAction = func() error {
			return e.shard.GetService().GetClientBean().GetHistoryClient().SyncActivity(ctx, request)
		}
	} else {
		syncActivityAction = func() error {
			return e.historyEngine.SyncActivity(ctx, request)
		}
	}

	err = syncActivityAction()
	retryErr, ok := toRetryTaskV2Error(err)
	if !ok {
		return err
	}
	// Handle resend error
	e.metricsClient.IncCounter(metrics.HistoryRereplicationByActivityReplicationScope, metrics.CadenceClientRequests)
	stopwatch := e.metricsClient.StartTimer(metrics.HistoryRereplicationByActivityReplicationScope, metrics.CadenceClientLatency)
	defer stopwatch.Stop()

	resendErr := e.historyResender.SendSingleWorkflowHistory(
		e.sourceCluster,
		retryErr.GetDomainID(),
		retryErr.GetWorkflowID(),
		retryErr.GetRunID(),
		retryErr.StartEventID,
		retryErr.StartEventVersion,
		retryErr.EndEventID,
		retryErr.EndEventVersion,
	)
	switch {
	case resendErr == nil:
		break
	case resendErr == ndc.ErrSkipTask:
		e.logger.Error(
			"skip replication sync activity task",
			tag.WorkflowDomainID(retryErr.GetDomainID()),
			tag.WorkflowID(retryErr.GetWorkflowID()),
			tag.WorkflowRunID(retryErr.GetRunID()),
		)
		return nil
	default:
		e.logger.Error(
			"error resend history for sync activity",
			tag.WorkflowDomainID(retryErr.GetDomainID()),
			tag.WorkflowID(retryErr.GetWorkflowID()),
			tag.WorkflowRunID(retryErr.GetRunID()),
			tag.Error(resendErr),
		)
		// should return the replication error, not the resending error
		return err
	}
	// should try again after back fill the history
	return syncActivityAction()
}

func (e *taskExecutorImpl) handleHistoryReplicationTaskV2(
	task *types.ReplicationTask,
	forceApply bool,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			attr := task.HistoryTaskV2Attributes
			e.logger.Error(
				"handleHistoryReplicationTaskV2 encountered panic.",
				tag.WorkflowDomainID(attr.GetDomainID()),
				tag.WorkflowID(attr.GetWorkflowID()),
				tag.WorkflowRunID(attr.GetRunID()),
				tag.Value(r),
			)
			panic(r)
		}
	}()

	attr := task.HistoryTaskV2Attributes
	doContinue, err := e.filterTask(attr.GetDomainID(), forceApply)
	if err != nil || !doContinue {
		return err
	}

	replicationStopWatch := e.metricsClient.StartTimer(metrics.HistoryReplicationV2TaskScope, metrics.CadenceLatency)
	defer replicationStopWatch.Stop()
	request := &types.ReplicateEventsV2Request{
		DomainUUID: attr.DomainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: attr.WorkflowID,
			RunID:      attr.RunID,
		},
		VersionHistoryItems: attr.VersionHistoryItems,
		Events:              attr.Events,
		// new run events does not need version history since there is no prior events
		NewRunEvents: attr.NewRunEvents,
	}
	ctx, cancel := context.WithTimeout(context.Background(), replicationTimeout)
	defer cancel()

	var historyReplicationAction func() error
	// Check if the number of shards between clusters are equal. If not, redirect the request.
	if e.shard.GetShardID() != common.WorkflowIDToHistoryShard(attr.WorkflowID, e.shard.GetConfig().NumberOfShards) {
		historyReplicationAction = func() error {
			return e.shard.GetService().GetClientBean().GetHistoryClient().ReplicateEventsV2(ctx, request)
		}
	} else {
		historyReplicationAction = func() error {
			return e.historyEngine.ReplicateEventsV2(ctx, request)
		}
	}

	err = historyReplicationAction()
	retryErr, ok := toRetryTaskV2Error(err)
	if !ok {
		return err
	}
	e.metricsClient.IncCounter(metrics.HistoryRereplicationByHistoryReplicationScope, metrics.CadenceClientRequests)
	resendStopWatch := e.metricsClient.StartTimer(metrics.HistoryRereplicationByHistoryReplicationScope, metrics.CadenceClientLatency)
	defer resendStopWatch.Stop()

	resendErr := e.historyResender.SendSingleWorkflowHistory(
		e.sourceCluster,
		retryErr.GetDomainID(),
		retryErr.GetWorkflowID(),
		retryErr.GetRunID(),
		retryErr.StartEventID,
		retryErr.StartEventVersion,
		retryErr.EndEventID,
		retryErr.EndEventVersion,
	)
	switch {
	case resendErr == nil:
		break
	case resendErr == ndc.ErrSkipTask:
		e.logger.Error(
			"skip replication history task",
			tag.WorkflowDomainID(retryErr.GetDomainID()),
			tag.WorkflowID(retryErr.GetWorkflowID()),
			tag.WorkflowRunID(retryErr.GetRunID()),
		)
		return nil
	default:
		e.logger.Error(
			"error resend history for history event v2",
			tag.WorkflowDomainID(retryErr.GetDomainID()),
			tag.WorkflowID(retryErr.GetWorkflowID()),
			tag.WorkflowRunID(retryErr.GetRunID()),
			tag.Error(resendErr),
		)
		// should return the replication error, not the resending error
		return err
	}

	return historyReplicationAction()
}

func (e *taskExecutorImpl) handleFailoverReplicationTask(
	task *types.ReplicationTask,
) error {
	failoverAttributes := task.GetFailoverMarkerAttributes()
	failoverAttributes.CreationTime = task.CreationTime
	return e.shard.AddingPendingFailoverMarker(failoverAttributes)
}

func (e *taskExecutorImpl) filterTask(
	domainID string,
	forceApply bool,
) (bool, error) {
	if forceApply {
		return true, nil
	}
	domainEntry, err := e.domainCache.GetDomainByID(domainID)
	if err != nil {
		return false, err
	}
	shouldProcessTask := false
	for _, targetCluster := range domainEntry.GetReplicationConfig().Clusters {
		if e.currentCluster == targetCluster.ClusterName {
			shouldProcessTask = true
			break
		}
	}
	return shouldProcessTask, nil
}

func toRetryTaskV2Error(err error) (*types.RetryTaskV2Error, bool) {
	retError, ok := err.(*types.RetryTaskV2Error)
	return retError, ok
}
