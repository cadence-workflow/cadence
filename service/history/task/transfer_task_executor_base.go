// Copyright (c) 2020 Uber Technologies, Inc.
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

package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/multierr"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/visibility"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/worker/archiver"
)

const (
	taskDefaultTimeout             = 3 * time.Second
	taskGetExecutionContextTimeout = 500 * time.Millisecond
	taskRPCCallTimeout             = 2 * time.Second
	secondsInDay                   = int32(24 * time.Hour / time.Second)
	defaultDomainName              = "defaultDomainName"
)

type (
	transferTaskExecutorBase struct {
		shard          shard.Context
		dlqWriter      TaskDLQWriter
		archiverClient archiver.Client
		executionCache execution.Cache
		logger         log.Logger
		metricsClient  metrics.Client
		matchingClient matching.Client
		visibilityMgr  persistence.VisibilityManager
		config         *config.Config
		throttleRetry  *backoff.ThrottleRetry
	}
)

func newTransferTaskExecutorBase(
	shard shard.Context,
	archiverClient archiver.Client,
	executionCache execution.Cache,
	logger log.Logger,
	config *config.Config,
	dlqWriter TaskDLQWriter,
) *transferTaskExecutorBase {
	return &transferTaskExecutorBase{
		shard:          shard,
		dlqWriter:      dlqWriter,
		archiverClient: archiverClient,
		executionCache: executionCache,
		logger:         logger,
		metricsClient:  shard.GetMetricsClient(),
		matchingClient: shard.GetService().GetMatchingClient(),
		visibilityMgr:  shard.GetService().GetVisibilityManager(),
		config:         config,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(taskRetryPolicy),
			backoff.WithRetryableError(common.IsServiceTransientError),
		),
	}
}

func (t *transferTaskExecutorBase) pushActivity(
	ctx context.Context,
	task *persistence.ActivityTask,
	pushActivityInfo *pushActivityToMatchingInfo,
) error {

	ctx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
	defer cancel()

	if task.GetTaskType() != persistence.TransferTaskTypeActivityTask {
		t.logger.Fatal("Cannot process non activity task", tag.TaskType(task.GetTaskType()))
	}

	shouldPush, err := shouldPushToMatching(ctx, t.shard, task)
	if err != nil {
		return err
	}

	if !shouldPush {
		// The domain is an active-active domain and the task is pending
		// For the first few minutes, we will retry the task and then discard it if it is still pending.
		if t.shard.GetTimeSource().Now().Before(task.GetVisibilityTimestamp().Add(t.config.StandbyTaskMissingEventsDiscardDelay())) {
			err = standbyTaskPostActionNoOp(ctx, task, pushActivityInfo, t.logger)
			return err
		}

		return standbyTaskPostActionWriteToDLQ(
			t.dlqWriter,
			t.shard,
			t.config.HistoryTaskDLQMode,
		)(ctx, task, pushActivityInfo, t.logger)
	}

	activityScheduleToStartTimeout := min(pushActivityInfo.activityScheduleToStartTimeout, constants.MaxTaskTimeout)
	taskList := &pushActivityInfo.tasklist
	partitionConfig := pushActivityInfo.partitionConfig
	_, err = t.matchingClient.AddActivityTask(ctx, &types.AddActivityTaskRequest{
		DomainUUID:       task.TargetDomainID,
		SourceDomainUUID: task.DomainID,
		Execution: &types.WorkflowExecution{
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
		},
		TaskList:                      taskList,
		ScheduleID:                    task.ScheduleID,
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(activityScheduleToStartTimeout),
		PartitionConfig:               partitionConfig,
	})
	return err
}

func (t *transferTaskExecutorBase) pushDecision(
	ctx context.Context,
	task *persistence.DecisionTask,
	pushDecisionInfo *pushDecisionToMatchingInfo,
) error {

	ctx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
	defer cancel()

	if task.GetTaskType() != persistence.TransferTaskTypeDecisionTask {
		t.logger.Fatal("Cannot process non decision task", tag.TaskType(task.GetTaskType()))
	}

	shouldPush, err := shouldPushToMatching(ctx, t.shard, task)
	if err != nil {
		return err
	}
	if !shouldPush {
		// The domain is an active-active domain and the task is pending
		// For the first few minutes, we will retry the task and then discard it if it is still pending.
		if t.shard.GetTimeSource().Now().Before(task.GetVisibilityTimestamp().Add(t.config.StandbyTaskMissingEventsDiscardDelay())) {
			return standbyTaskPostActionNoOp(ctx, task, pushDecisionInfo, t.logger)
		}
		return standbyTaskPostActionWriteToDLQ(
			t.dlqWriter,
			t.shard,
			t.config.HistoryTaskDLQMode,
		)(ctx, task, pushDecisionInfo, t.logger)
	}

	decisionScheduleToStartTimeout := min(pushDecisionInfo.decisionScheduleToStartTimeout, constants.MaxTaskTimeout)
	tasklist := &pushDecisionInfo.tasklist
	partitionConfig := pushDecisionInfo.partitionConfig
	_, err = t.matchingClient.AddDecisionTask(ctx, &types.AddDecisionTaskRequest{
		DomainUUID: task.DomainID,
		Execution: &types.WorkflowExecution{
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
		},
		TaskList:                      tasklist,
		ScheduleID:                    task.ScheduleID,
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(decisionScheduleToStartTimeout),
		PartitionConfig:               partitionConfig,
	})
	return err
}

// visibilityRecord holds the fields of a visibility record that are read from mutable state.
// It is built while the workflow lock is held, so the visibility store can be written after the lock is released.
type visibilityRecord struct {
	workflowTypeName               string
	startTimeUnixNano              int64
	executionTimeUnixNano          int64
	workflowTimeout                int32
	taskList                       string
	visibilityMemo                 *types.Memo
	isCron                         bool
	cronSchedule                   string
	numClusters                    int16
	updateTimeUnixNano             int64
	searchAttributes               map[string][]byte
	headers                        map[string][]byte
	clusterAttribute               *types.ClusterAttribute
	executionStatus                types.WorkflowExecutionStatus
	scheduledExecutionTimeUnixNano int64

	// only set for closed records
	endTimeUnixNano int64
	closeStatus     types.WorkflowExecutionCloseStatus
	historyLength   int64
}

// newStartedVisibilityRecord builds the record for RecordWorkflowStarted and UpsertWorkflowSearchAttributes tasks
func (t *transferTaskExecutorBase) newStartedVisibilityRecord(
	ctx context.Context,
	mutableState execution.MutableState,
	domainEntry *cache.DomainCacheEntry,
) (*visibilityRecord, error) {
	executionInfo := mutableState.GetExecutionInfo()
	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return nil, err
	}
	isCron := len(executionInfo.CronSchedule) > 0
	cronSchedule := ""
	if isCron {
		cronSchedule = executionInfo.CronSchedule
	}
	isAdvancedVisibilityEnabled := common.IsAdvancedVisibilityWritingEnabled(
		t.config.WriteVisibilityStoreName(),
		t.config.IsAdvancedVisConfigExist,
	)
	executionStatus, scheduledExecutionTimestamp := determineExecutionStatusForVisibility(startEvent, mutableState, isAdvancedVisibilityEnabled)

	return &visibilityRecord{
		workflowTypeName:               executionInfo.WorkflowTypeName,
		startTimeUnixNano:              startEvent.GetTimestamp(),
		executionTimeUnixNano:          getWorkflowExecutionTimestamp(mutableState, startEvent).UnixNano(),
		workflowTimeout:                executionInfo.WorkflowTimeout,
		taskList:                       executionInfo.TaskList,
		visibilityMemo:                 getWorkflowMemo(executionInfo.Memo),
		isCron:                         isCron,
		cronSchedule:                   cronSchedule,
		numClusters:                    int16(len(domainEntry.GetReplicationConfig().Clusters)),
		updateTimeUnixNano:             t.shard.GetTimeSource().Now().UnixNano(),
		searchAttributes:               copySearchAttributes(executionInfo.SearchAttributes),
		headers:                        getWorkflowHeaders(startEvent),
		clusterAttribute:               executionInfo.ActiveClusterSelectionPolicy.GetClusterAttribute(),
		executionStatus:                executionStatus,
		scheduledExecutionTimeUnixNano: scheduledExecutionTimestamp,
	}, nil
}

// newClosedVisibilityRecord builds the record for the closed visibility record of a workflow.
// completionEvent is passed in since callers also use it for other side effects of the close.
func (t *transferTaskExecutorBase) newClosedVisibilityRecord(
	ctx context.Context,
	mutableState execution.MutableState,
	domainEntry *cache.DomainCacheEntry,
	completionEvent *types.HistoryEvent,
) (*visibilityRecord, error) {
	executionInfo := mutableState.GetExecutionInfo()
	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return nil, err
	}
	isCron := len(executionInfo.CronSchedule) > 0
	cronSchedule := ""
	if isCron {
		cronSchedule = executionInfo.CronSchedule
	}
	scheduledExecutionTimestamp := startEvent.GetTimestamp()
	if startEvent.WorkflowExecutionStartedEventAttributes != nil &&
		startEvent.WorkflowExecutionStartedEventAttributes.GetFirstDecisionTaskBackoffSeconds() > 0 {
		scheduledExecutionTimestamp = startEvent.GetTimestamp() +
			int64(startEvent.WorkflowExecutionStartedEventAttributes.GetFirstDecisionTaskBackoffSeconds())*int64(time.Second)
	}

	return &visibilityRecord{
		workflowTypeName:               executionInfo.WorkflowTypeName,
		startTimeUnixNano:              startEvent.GetTimestamp(),
		executionTimeUnixNano:          getWorkflowExecutionTimestamp(mutableState, startEvent).UnixNano(),
		taskList:                       executionInfo.TaskList,
		visibilityMemo:                 getWorkflowMemo(executionInfo.Memo),
		isCron:                         isCron,
		cronSchedule:                   cronSchedule,
		numClusters:                    int16(len(domainEntry.GetReplicationConfig().Clusters)),
		updateTimeUnixNano:             t.shard.GetTimeSource().Now().UnixNano(),
		searchAttributes:               copySearchAttributes(executionInfo.SearchAttributes),
		headers:                        getWorkflowHeaders(startEvent),
		clusterAttribute:               executionInfo.ActiveClusterSelectionPolicy.GetClusterAttribute(),
		executionStatus:                getWorkflowExecutionStatus(mutableState, completionEvent),
		scheduledExecutionTimeUnixNano: scheduledExecutionTimestamp,
		endTimeUnixNano:                completionEvent.GetTimestamp(),
		closeStatus:                    *persistence.ToInternalWorkflowExecutionCloseStatus(executionInfo.CloseStatus),
		historyLength:                  mutableState.GetNextEventID() - 1,
	}, nil
}

func (t *transferTaskExecutorBase) recordWorkflowStarted(
	ctx context.Context,
	task persistence.Task,
	record *visibilityRecord,
) error {

	domain := defaultDomainName

	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.GetDomainID())
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			return err
		}
	} else {
		domain = domainEntry.GetInfo().Name
		// if sampled for longer retention is enabled, only record those sampled events
		if domainEntry.IsSampledForLongerRetentionEnabled(task.GetWorkflowID()) &&
			!domainEntry.IsSampledForLongerRetention(task.GetWorkflowID()) {
			return nil
		}
	}

	// headers are appended into search attributes if enabled
	searchAttributes := copySearchAttributes(record.searchAttributes)
	if t.config.EnableContextHeaderInVisibility(domainEntry.GetInfo().Name) {
		// fail open, if error occurs, just log it; successfully appended headers will be stored
		if searchAttributes, err = appendContextHeaderToSearchAttributes(searchAttributes, record.headers, t.config.ValidSearchAttributes(), t.config.SearchAttributesHiddenValueKeys()); err != nil {
			t.logger.Error("failed to add headers to search attributes", tag.Error(err))
		}
	}

	request := &persistence.RecordWorkflowExecutionStartedRequest{
		DomainUUID: task.GetDomainID(),
		Domain:     domain,
		Execution: types.WorkflowExecution{
			WorkflowID: task.GetWorkflowID(),
			RunID:      task.GetRunID(),
		},
		WorkflowTypeName:            record.workflowTypeName,
		StartTimestamp:              record.startTimeUnixNano,
		ExecutionTimestamp:          record.executionTimeUnixNano,
		WorkflowTimeout:             int64(record.workflowTimeout),
		TaskID:                      task.GetTaskID(),
		Memo:                        record.visibilityMemo,
		TaskList:                    record.taskList,
		IsCron:                      record.isCron,
		CronSchedule:                record.cronSchedule,
		NumClusters:                 record.numClusters,
		ClusterAttributeScope:       record.clusterAttribute.GetScope(),
		ClusterAttributeName:        record.clusterAttribute.GetName(),
		UpdateTimestamp:             record.updateTimeUnixNano,
		SearchAttributes:            searchAttributes,
		ShardID:                     int16(t.shard.GetShardID()),
		ExecutionStatus:             record.executionStatus,
		ScheduledExecutionTimestamp: record.scheduledExecutionTimeUnixNano,
	}

	if t.config.EnableRecordWorkflowExecutionUninitialized(domain) {
		uninitializedRequest := &persistence.RecordWorkflowExecutionUninitializedRequest{
			DomainUUID: task.GetDomainID(),
			Domain:     domain,
			Execution: types.WorkflowExecution{
				WorkflowID: task.GetWorkflowID(),
				RunID:      task.GetRunID(),
			},
			WorkflowTypeName: record.workflowTypeName,
			UpdateTimestamp:  record.updateTimeUnixNano,
			ShardID:          int64(t.shard.GetShardID()),
		}
		if err := t.visibilityMgr.RecordWorkflowExecutionUninitialized(ctx, uninitializedRequest); err != nil {
			t.logger.Error("Failed to record uninitialized workflow execution", tag.Error(err))
		}
	}

	return t.visibilityMgr.RecordWorkflowExecutionStarted(ctx, request)
}

func (t *transferTaskExecutorBase) upsertWorkflowExecution(
	ctx context.Context,
	task persistence.Task,
	record *visibilityRecord,
) error {

	domain, err := t.shard.GetDomainCache().GetDomainName(task.GetDomainID())
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			return err
		}
		domain = defaultDomainName
	}

	// headers are appended into search attributes if enabled
	searchAttributes := copySearchAttributes(record.searchAttributes)
	if t.config.EnableContextHeaderInVisibility(domain) {
		// fail open, if error occurs, just log it; successfully appended headers will be stored
		if searchAttributes, err = appendContextHeaderToSearchAttributes(searchAttributes, record.headers, t.config.ValidSearchAttributes(), t.config.SearchAttributesHiddenValueKeys()); err != nil {
			t.logger.Error("failed to add headers to search attributes", tag.Error(err))
		}
	}

	request := &persistence.UpsertWorkflowExecutionRequest{
		DomainUUID: task.GetDomainID(),
		Domain:     domain,
		Execution: types.WorkflowExecution{
			WorkflowID: task.GetWorkflowID(),
			RunID:      task.GetRunID(),
		},
		WorkflowTypeName:            record.workflowTypeName,
		StartTimestamp:              record.startTimeUnixNano,
		ExecutionTimestamp:          record.executionTimeUnixNano,
		WorkflowTimeout:             int64(record.workflowTimeout),
		TaskID:                      task.GetTaskID(),
		Memo:                        record.visibilityMemo,
		TaskList:                    record.taskList,
		IsCron:                      record.isCron,
		NumClusters:                 record.numClusters,
		ClusterAttributeScope:       record.clusterAttribute.GetScope(),
		ClusterAttributeName:        record.clusterAttribute.GetName(),
		SearchAttributes:            searchAttributes,
		UpdateTimestamp:             record.updateTimeUnixNano,
		ShardID:                     int64(t.shard.GetShardID()),
		ExecutionStatus:             record.executionStatus,
		CronSchedule:                record.cronSchedule,
		ScheduledExecutionTimestamp: record.scheduledExecutionTimeUnixNano,
	}

	return t.visibilityMgr.UpsertWorkflowExecution(ctx, request)
}

func (t *transferTaskExecutorBase) recordWorkflowClosed(
	ctx context.Context,
	task persistence.Task,
	record *visibilityRecord,
) error {

	// Record closing in visibility store
	retentionSeconds := int64(0)
	domain := defaultDomainName
	recordWorkflowClose := true
	archiveVisibility := false

	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.GetDomainID())
	if err != nil && !isWorkflowNotExistError(err) {
		return err
	}

	if err == nil {
		// retention in domain config is in days, convert to seconds
		retentionSeconds = int64(domainEntry.GetRetentionDays(task.GetWorkflowID())) * int64(secondsInDay)
		domain = domainEntry.GetInfo().Name
		// if sampled for longer retention is enabled, only record those sampled events
		if domainEntry.IsSampledForLongerRetentionEnabled(task.GetWorkflowID()) &&
			!domainEntry.IsSampledForLongerRetention(task.GetWorkflowID()) {
			recordWorkflowClose = false
		}

		clusterConfiguredForVisibilityArchival := t.shard.GetService().GetArchivalMetadata().GetVisibilityConfig().ClusterConfiguredForArchival()
		domainConfiguredForVisibilityArchival := domainEntry.GetConfig().VisibilityArchivalStatus == types.ArchivalStatusEnabled
		archiveVisibility = clusterConfiguredForVisibilityArchival && domainConfiguredForVisibilityArchival
	}

	// headers are appended into search attributes if enabled
	searchAttributes := copySearchAttributes(record.searchAttributes)
	if t.config.EnableContextHeaderInVisibility(domainEntry.GetInfo().Name) {
		// fail open, if error occurs, just log it; successfully appended headers will be stored
		if searchAttributes, err = appendContextHeaderToSearchAttributes(searchAttributes, record.headers, t.config.ValidSearchAttributes(), t.config.SearchAttributesHiddenValueKeys()); err != nil {
			t.logger.Error("failed to add headers to search attributes", tag.Error(err))
		}
	}

	if recordWorkflowClose {
		if err := t.visibilityMgr.RecordWorkflowExecutionClosed(ctx, &persistence.RecordWorkflowExecutionClosedRequest{
			DomainUUID: task.GetDomainID(),
			Domain:     domain,
			Execution: types.WorkflowExecution{
				WorkflowID: task.GetWorkflowID(),
				RunID:      task.GetRunID(),
			},
			WorkflowTypeName:            record.workflowTypeName,
			StartTimestamp:              record.startTimeUnixNano,
			ExecutionTimestamp:          record.executionTimeUnixNano,
			CloseTimestamp:              record.endTimeUnixNano,
			Status:                      record.closeStatus,
			HistoryLength:               record.historyLength,
			RetentionSeconds:            retentionSeconds,
			TaskID:                      task.GetTaskID(),
			Memo:                        record.visibilityMemo,
			TaskList:                    record.taskList,
			SearchAttributes:            searchAttributes,
			IsCron:                      record.isCron,
			CronSchedule:                record.cronSchedule,
			ClusterAttributeScope:       record.clusterAttribute.GetScope(),
			ClusterAttributeName:        record.clusterAttribute.GetName(),
			UpdateTimestamp:             record.updateTimeUnixNano,
			NumClusters:                 record.numClusters,
			ShardID:                     int16(t.shard.GetShardID()),
			ExecutionStatus:             record.executionStatus,
			ScheduledExecutionTimestamp: record.scheduledExecutionTimeUnixNano,
		}); err != nil {
			return err
		}
	}

	if archiveVisibility {
		archiveCtx, cancel := context.WithTimeout(ctx, t.config.TransferProcessorVisibilityArchivalTimeLimit())
		defer cancel()
		_, err := t.archiverClient.Archive(archiveCtx, &archiver.ClientRequest{
			ArchiveRequest: &archiver.ArchiveRequest{
				DomainID:           task.GetDomainID(),
				DomainName:         domain,
				WorkflowID:         task.GetWorkflowID(),
				RunID:              task.GetRunID(),
				WorkflowTypeName:   record.workflowTypeName,
				StartTimestamp:     record.startTimeUnixNano,
				ExecutionTimestamp: record.executionTimeUnixNano,
				CloseTimestamp:     record.endTimeUnixNano,
				CloseStatus:        record.closeStatus,
				HistoryLength:      record.historyLength,
				Memo:               record.visibilityMemo,
				SearchAttributes:   searchAttributes,
				VisibilityURI:      domainEntry.GetConfig().VisibilityArchivalURI,
				URI:                domainEntry.GetConfig().HistoryArchivalURI,
				Targets:            []archiver.ArchivalTarget{archiver.ArchiveTargetVisibility},
			},
			CallerService:        service.History,
			AttemptArchiveInline: true, // archive visibility inline by default
		})
		return err
	}
	return nil
}

// Argument startEvent is to save additional call of msBuilder.GetStartEvent
func getWorkflowExecutionTimestamp(
	msBuilder execution.MutableState,
	startEvent *types.HistoryEvent,
) time.Time {
	// Use value 0 to represent workflows that don't need backoff. Since ES doesn't support
	// comparison between two field, we need a value to differentiate them from cron workflows
	// or later runs of a workflow that needs retry.
	executionTimestamp := time.Unix(0, 0)
	if startEvent == nil {
		return executionTimestamp
	}

	if backoffSeconds := startEvent.WorkflowExecutionStartedEventAttributes.GetFirstDecisionTaskBackoffSeconds(); backoffSeconds != 0 {
		startTimestamp := time.Unix(0, startEvent.GetTimestamp())
		executionTimestamp = startTimestamp.Add(time.Duration(backoffSeconds) * time.Second)
	}
	return executionTimestamp
}

func getWorkflowMemo(
	memo map[string][]byte,
) *types.Memo {

	if memo == nil {
		return nil
	}
	return &types.Memo{Fields: memo}
}

// context headers are appended to search attributes if in allow list; return errors when all context key is processed
func appendContextHeaderToSearchAttributes(attr, context map[string][]byte, allowedKeys map[string]interface{}, hiddenValueKeys map[string]interface{}) (map[string][]byte, error) {
	// sanity check
	if attr == nil {
		attr = make(map[string][]byte)
	}
	var errGroup error
	for k, v := range context {
		sanitizedKey, err := visibility.SanitizeSearchAttributeKey(k)
		if err != nil { // This could never happen
			multierr.Append(errGroup, fmt.Errorf("fail to sanitize context key %s: %w", k, err))
			continue
		}
		key := fmt.Sprintf(definition.HeaderFormat, sanitizedKey)
		if _, ok := attr[key]; ok { // skip if key already exists
			continue
		}
		if _, allowed := allowedKeys[key]; !allowed { // skip if not allowed
			continue
		}
		// context header are raw string bytes, need to be json encoded to be stored in search attributes
		// ignore error as it can't happen to err on json encoding string
		// context header can contain sensitive information, so we need to hide the value if it is in the hiddenValueKeys
		var val string
		if shouldRedactContextHeader(key, hiddenValueKeys) {
			val = "***redacted***" // Hide the actualvalue
		} else {
			val = string(v) // Convert []byte to string safely
		}
		data, _ := json.Marshal(val)
		attr[key] = data
	}
	return attr, errGroup
}

func getWorkflowHeaders(startEvent *types.HistoryEvent) map[string][]byte {
	attr := startEvent.GetWorkflowExecutionStartedEventAttributes()
	if attr == nil || attr.Header == nil {
		return nil
	}
	headers := make(map[string][]byte, len(attr.Header.Fields))
	for k, v := range attr.Header.Fields {
		val := make([]byte, len(v))
		copy(val, v)
		headers[k] = val
	}
	return headers
}

func copySearchAttributes(
	input map[string][]byte,
) map[string][]byte {

	if input == nil {
		return nil
	}

	result := make(map[string][]byte)
	for k, v := range input {
		val := make([]byte, len(v))
		copy(val, v)
		result[k] = val
	}
	return result
}

func isWorkflowNotExistError(err error) bool {
	_, ok := err.(*types.EntityNotExistsError)
	return ok
}

func shouldRedactContextHeader(key string, hiddenValueKeys map[string]interface{}) bool {
	if hiddenValueKeys == nil {
		return false
	}
	if val, exists := hiddenValueKeys[key]; exists {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return strings.ToLower(v) == "true"
		case int:
			return v > 0 // 1 means true, 0 means false
		}
	}
	return false
}

// determineExecutionStatus determines whether a workflow execution is PENDING or STARTED
// based on whether it has a first decision task backoff and if the decision task has been scheduled.
//
// A workflow is PENDING if:
// - It has a firstDecisionTaskBackoffSeconds > 0, AND
// - No decision task has been scheduled/processed yet
//
// Otherwise, it's STARTED.
//
// Returns:
// - executionStatus: either WorkflowExecutionStatusPending or WorkflowExecutionStatusStarted
// - scheduledExecutionTimestamp: startTime + firstDecisionTaskBackoffSeconds (in nanoseconds)
func determineExecutionStatus(
	startEvent *types.HistoryEvent,
	mutableState execution.MutableState,
) (executionStatus types.WorkflowExecutionStatus, scheduledExecutionTimestamp int64) {
	executionStatus = types.WorkflowExecutionStatusStarted
	backoffSeconds := int32(0)

	if startEvent.WorkflowExecutionStartedEventAttributes != nil {
		backoffSeconds = startEvent.WorkflowExecutionStartedEventAttributes.GetFirstDecisionTaskBackoffSeconds()
	}

	if backoffSeconds > 0 {
		hasPending := mutableState.HasPendingDecision()
		hasInFlight := mutableState.HasInFlightDecision()
		hasProcessed := mutableState.HasProcessedOrPendingDecision()

		// Check if the first decision task has been scheduled yet
		// If there's no decision info, the workflow is still pending
		if !hasPending && !hasInFlight && !hasProcessed {
			executionStatus = types.WorkflowExecutionStatusPending
		}
	}

	// startTime + firstDecisionTaskBackoffSeconds
	scheduledExecutionTimestamp = startEvent.GetTimestamp() +
		int64(backoffSeconds)*int64(time.Second)

	return executionStatus, scheduledExecutionTimestamp
}

func determineExecutionStatusForVisibility(
	startEvent *types.HistoryEvent,
	mutableState execution.MutableState,
	isAdvancedVisibilityEnabled bool,
) (executionStatus types.WorkflowExecutionStatus, scheduledExecutionTimestamp int64) {
	executionStatus, scheduledExecutionTimestamp = determineExecutionStatus(startEvent, mutableState)

	// For basic DB visibility, always use STARTED status
	// Basic visibility cannot be updated mid-execution, so showing PENDING would be
	// misleading as the workflow would appear to be waiting even when it's running.
	// With advanced visibility, we can update PENDING -> STARTED via upsert.
	if !isAdvancedVisibilityEnabled && executionStatus == types.WorkflowExecutionStatusPending {
		executionStatus = types.WorkflowExecutionStatusStarted
	}

	return executionStatus, scheduledExecutionTimestamp
}
