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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination mutable_state_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"context"
	"time"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/query"
)

type (
	// DecisionInfo should be part of persistence layer
	DecisionInfo struct {
		Version         int64
		ScheduleID      int64
		StartedID       int64
		RequestID       string
		DecisionTimeout int32
		TaskList        string // This is only needed to communicate tasklist used after AddDecisionTaskScheduledEvent
		Attempt         int64
		// Scheduled and Started timestamps are useful for transient decision: when transient decision finally completes,
		// use these timestamp to create scheduled/started events.
		// Also used for recording latency metrics
		ScheduledTimestamp int64
		StartedTimestamp   int64
		// OriginalScheduledTimestamp is to record the first scheduled decision during decision heartbeat.
		// Client may heartbeat decision by RespondDecisionTaskComplete with ForceCreateNewDecisionTask == true
		// In this case, OriginalScheduledTimestamp won't change. Then when current time - OriginalScheduledTimestamp exceeds
		// some threshold, server can interrupt the heartbeat by enforcing to timeout the decision.
		OriginalScheduledTimestamp int64
	}

	// MutableState contains the current workflow execution state
	MutableState interface {
		AddActivityTaskCancelRequestedEvent(int64, string, string) (*types.HistoryEvent, *persistence.ActivityInfo, error)
		AddActivityTaskCanceledEvent(int64, int64, int64, []uint8, string) (*types.HistoryEvent, error)
		AddActivityTaskCompletedEvent(int64, int64, *types.RespondActivityTaskCompletedRequest) (*types.HistoryEvent, error)
		AddActivityTaskFailedEvent(int64, int64, *types.RespondActivityTaskFailedRequest) (*types.HistoryEvent, error)
		AddActivityTaskScheduledEvent(context.Context, int64, *types.ScheduleActivityTaskDecisionAttributes, bool) (*types.HistoryEvent, *persistence.ActivityInfo, *types.ActivityLocalDispatchInfo, bool, bool, error)
		AddActivityTaskStartedEvent(*persistence.ActivityInfo, int64, string, string) (*types.HistoryEvent, error)
		AddActivityTaskTimedOutEvent(int64, int64, types.TimeoutType, []uint8) (*types.HistoryEvent, error)
		AddCancelTimerFailedEvent(int64, *types.CancelTimerDecisionAttributes, string) (*types.HistoryEvent, error)
		AddChildWorkflowExecutionCanceledEvent(int64, *types.WorkflowExecution, *types.WorkflowExecutionCanceledEventAttributes) (*types.HistoryEvent, error)
		AddChildWorkflowExecutionCompletedEvent(int64, *types.WorkflowExecution, *types.WorkflowExecutionCompletedEventAttributes) (*types.HistoryEvent, error)
		AddChildWorkflowExecutionFailedEvent(int64, *types.WorkflowExecution, *types.WorkflowExecutionFailedEventAttributes) (*types.HistoryEvent, error)
		AddChildWorkflowExecutionStartedEvent(string, *types.WorkflowExecution, *types.WorkflowType, int64, *types.Header) (*types.HistoryEvent, error)
		AddChildWorkflowExecutionTerminatedEvent(int64, *types.WorkflowExecution, *types.WorkflowExecutionTerminatedEventAttributes) (*types.HistoryEvent, error)
		AddChildWorkflowExecutionTimedOutEvent(int64, *types.WorkflowExecution, *types.WorkflowExecutionTimedOutEventAttributes) (*types.HistoryEvent, error)
		AddCompletedWorkflowEvent(int64, *types.CompleteWorkflowExecutionDecisionAttributes) (*types.HistoryEvent, error)
		AddContinueAsNewEvent(context.Context, int64, int64, string, *types.ContinueAsNewWorkflowExecutionDecisionAttributes) (*types.HistoryEvent, MutableState, error)
		AddDecisionTaskCompletedEvent(int64, int64, *types.RespondDecisionTaskCompletedRequest, int) (*types.HistoryEvent, error)
		AddDecisionTaskFailedEvent(scheduleEventID int64, startedEventID int64, cause types.DecisionTaskFailedCause, details []byte, identity, reason, binChecksum, baseRunID, newRunID string, forkEventVersion int64, resetRequestID string) (*types.HistoryEvent, error)
		AddDecisionTaskScheduleToStartTimeoutEvent(int64) (*types.HistoryEvent, error)
		AddDecisionTaskResetTimeoutEvent(int64, string, string, int64, string, string) (*types.HistoryEvent, error)
		AddFirstDecisionTaskScheduled(*types.HistoryEvent) error
		AddDecisionTaskScheduledEvent(bypassTaskGeneration bool) (*DecisionInfo, error)
		AddDecisionTaskScheduledEventAsHeartbeat(bypassTaskGeneration bool, originalScheduledTimestamp int64) (*DecisionInfo, error)
		AddDecisionTaskStartedEvent(int64, string, *types.PollForDecisionTaskRequest) (*types.HistoryEvent, *DecisionInfo, error)
		AddDecisionTaskTimedOutEvent(int64, int64) (*types.HistoryEvent, error)
		AddExternalWorkflowExecutionCancelRequested(int64, string, string, string) (*types.HistoryEvent, error)
		AddExternalWorkflowExecutionSignaled(int64, string, string, string, []uint8) (*types.HistoryEvent, error)
		AddFailWorkflowEvent(int64, *types.FailWorkflowExecutionDecisionAttributes) (*types.HistoryEvent, error)
		AddRecordMarkerEvent(int64, *types.RecordMarkerDecisionAttributes) (*types.HistoryEvent, error)
		AddRequestCancelActivityTaskFailedEvent(int64, string, string) (*types.HistoryEvent, error)
		AddRequestCancelExternalWorkflowExecutionFailedEvent(int64, int64, string, string, string, types.CancelExternalWorkflowExecutionFailedCause) (*types.HistoryEvent, error)
		AddRequestCancelExternalWorkflowExecutionInitiatedEvent(int64, string, *types.RequestCancelExternalWorkflowExecutionDecisionAttributes) (*types.HistoryEvent, *persistence.RequestCancelInfo, error)
		AddSignalExternalWorkflowExecutionFailedEvent(int64, int64, string, string, string, []uint8, types.SignalExternalWorkflowExecutionFailedCause) (*types.HistoryEvent, error)
		AddSignalExternalWorkflowExecutionInitiatedEvent(int64, string, *types.SignalExternalWorkflowExecutionDecisionAttributes) (*types.HistoryEvent, *persistence.SignalInfo, error)
		AddSignalRequested(requestID string)
		AddStartChildWorkflowExecutionFailedEvent(int64, types.ChildWorkflowExecutionFailedCause, *types.StartChildWorkflowExecutionInitiatedEventAttributes) (*types.HistoryEvent, error)
		AddStartChildWorkflowExecutionInitiatedEvent(int64, string, *types.StartChildWorkflowExecutionDecisionAttributes) (*types.HistoryEvent, *persistence.ChildExecutionInfo, error)
		AddTimeoutWorkflowEvent(int64) (*types.HistoryEvent, error)
		AddTimerCanceledEvent(int64, *types.CancelTimerDecisionAttributes, string) (*types.HistoryEvent, error)
		AddTimerFiredEvent(string) (*types.HistoryEvent, error)
		AddTimerStartedEvent(int64, *types.StartTimerDecisionAttributes) (*types.HistoryEvent, *persistence.TimerInfo, error)
		AddUpsertWorkflowSearchAttributesEvent(int64, *types.UpsertWorkflowSearchAttributesDecisionAttributes) (*types.HistoryEvent, error)
		AddWorkflowExecutionCancelRequestedEvent(string, *types.HistoryRequestCancelWorkflowExecutionRequest) (*types.HistoryEvent, error)
		AddWorkflowExecutionCanceledEvent(int64, *types.CancelWorkflowExecutionDecisionAttributes) (*types.HistoryEvent, error)
		AddWorkflowExecutionSignaled(signalName string, input []byte, identity string, reqeustID string) (*types.HistoryEvent, error)
		AddWorkflowExecutionStartedEvent(types.WorkflowExecution, *types.HistoryStartWorkflowExecutionRequest) (*types.HistoryEvent, error)
		AddWorkflowExecutionTerminatedEvent(firstEventID int64, reason string, details []byte, identity string) (*types.HistoryEvent, error)
		ClearStickyness()
		CheckResettable() error
		CopyToPersistence() *persistence.WorkflowMutableState
		RetryActivity(ai *persistence.ActivityInfo, failureReason string, failureDetails []byte) (bool, error)
		CreateNewHistoryEvent(eventType types.EventType) *types.HistoryEvent
		CreateNewHistoryEventWithTimestamp(eventType types.EventType, timestamp int64) *types.HistoryEvent
		CreateTransientDecisionEvents(di *DecisionInfo, identity string) (*types.HistoryEvent, *types.HistoryEvent)
		DeleteDecision()
		DeleteUserTimer(timerID string) error
		DeleteActivity(scheduleEventID int64) error
		DeleteSignalRequested(requestID string)
		FailDecision(bool)
		FlushBufferedEvents() error
		GetActivityByActivityID(string) (*persistence.ActivityInfo, bool)
		GetActivityInfo(int64) (*persistence.ActivityInfo, bool)
		GetActivityScheduledEvent(context.Context, int64) (*types.HistoryEvent, error)
		GetChildExecutionInfo(int64) (*persistence.ChildExecutionInfo, bool)
		GetChildExecutionInitiatedEvent(context.Context, int64) (*types.HistoryEvent, error)
		GetCompletionEvent(context.Context) (*types.HistoryEvent, error)
		GetDecisionInfo(int64) (*DecisionInfo, bool)
		GetDecisionScheduleToStartTimeout() time.Duration
		GetDomainEntry() *cache.DomainCacheEntry
		GetStartEvent(context.Context) (*types.HistoryEvent, error)
		GetCurrentBranchToken() ([]byte, error)
		GetVersionHistories() *persistence.VersionHistories
		GetCurrentVersion() int64
		GetExecutionInfo() *persistence.WorkflowExecutionInfo
		GetHistoryBuilder() *HistoryBuilder
		GetInFlightDecision() (*DecisionInfo, bool)
		GetPendingDecision() (*DecisionInfo, bool)
		GetLastFirstEventID() int64
		GetLastWriteVersion() (int64, error)
		GetNextEventID() int64
		GetPreviousStartedEventID() int64
		GetPendingActivityInfos() map[int64]*persistence.ActivityInfo
		GetPendingTimerInfos() map[string]*persistence.TimerInfo
		GetPendingChildExecutionInfos() map[int64]*persistence.ChildExecutionInfo
		GetPendingRequestCancelExternalInfos() map[int64]*persistence.RequestCancelInfo
		GetPendingSignalExternalInfos() map[int64]*persistence.SignalInfo
		GetPendingWorkflowTimerTaskInfos() map[int]*persistence.WorkflowTimerTaskInfo
		GetRequestCancelInfo(int64) (*persistence.RequestCancelInfo, bool)
		GetRetryBackoffDuration(errReason string) time.Duration
		GetCronBackoffDuration(context.Context) (time.Duration, error)
		GetSignalInfo(int64) (*persistence.SignalInfo, bool)
		GetStartVersion() (int64, error)
		GetUserTimerInfoByEventID(int64) (*persistence.TimerInfo, bool)
		GetUserTimerInfo(string) (*persistence.TimerInfo, bool)
		GetWorkflowType() *types.WorkflowType
		GetWorkflowStateCloseStatus() (int, int)
		GetQueryRegistry() query.Registry
		SetQueryRegistry(query.Registry)
		HasBufferedEvents() bool
		HasInFlightDecision() bool
		HasParentExecution() bool
		HasPendingDecision() bool
		HasProcessedOrPendingDecision() bool
		IsCancelRequested() (bool, string)
		IsCurrentWorkflowGuaranteed() bool
		IsSignalRequested(requestID string) bool
		IsStickyTaskListEnabled() bool
		IsWorkflowExecutionRunning() bool
		IsWorkflowCompleted() bool
		IsResourceDuplicated(resourceDedupKey definition.DeduplicationID) bool
		UpdateDuplicatedResource(resourceDedupKey definition.DeduplicationID)
		Load(context.Context, *persistence.WorkflowMutableState) error
		ReplicateActivityInfo(*types.SyncActivityRequest, bool) error
		ReplicateActivityTaskCancelRequestedEvent(*types.HistoryEvent) error
		ReplicateActivityTaskCanceledEvent(*types.HistoryEvent) error
		ReplicateActivityTaskCompletedEvent(*types.HistoryEvent) error
		ReplicateActivityTaskFailedEvent(*types.HistoryEvent) error
		ReplicateActivityTaskScheduledEvent(int64, *types.HistoryEvent, bool) (*persistence.ActivityInfo, error)
		ReplicateActivityTaskStartedEvent(*types.HistoryEvent) error
		ReplicateActivityTaskTimedOutEvent(*types.HistoryEvent) error
		ReplicateChildWorkflowExecutionCanceledEvent(*types.HistoryEvent) error
		ReplicateChildWorkflowExecutionCompletedEvent(*types.HistoryEvent) error
		ReplicateChildWorkflowExecutionFailedEvent(*types.HistoryEvent) error
		ReplicateChildWorkflowExecutionStartedEvent(*types.HistoryEvent) error
		ReplicateChildWorkflowExecutionTerminatedEvent(*types.HistoryEvent) error
		ReplicateChildWorkflowExecutionTimedOutEvent(*types.HistoryEvent) error
		ReplicateDecisionTaskCompletedEvent(*types.HistoryEvent) error
		ReplicateDecisionTaskFailedEvent(*types.HistoryEvent) error
		ReplicateDecisionTaskScheduledEvent(int64, int64, string, int32, int64, int64, int64, bool) (*DecisionInfo, error)
		ReplicateDecisionTaskStartedEvent(*DecisionInfo, int64, int64, int64, string, int64) (*DecisionInfo, error)
		ReplicateDecisionTaskTimedOutEvent(*types.HistoryEvent) error
		ReplicateExternalWorkflowExecutionCancelRequested(*types.HistoryEvent) error
		ReplicateExternalWorkflowExecutionSignaled(*types.HistoryEvent) error
		ReplicateRequestCancelExternalWorkflowExecutionFailedEvent(*types.HistoryEvent) error
		ReplicateRequestCancelExternalWorkflowExecutionInitiatedEvent(int64, *types.HistoryEvent, string) (*persistence.RequestCancelInfo, error)
		ReplicateSignalExternalWorkflowExecutionFailedEvent(*types.HistoryEvent) error
		ReplicateSignalExternalWorkflowExecutionInitiatedEvent(int64, *types.HistoryEvent, string) (*persistence.SignalInfo, error)
		ReplicateStartChildWorkflowExecutionFailedEvent(*types.HistoryEvent) error
		ReplicateStartChildWorkflowExecutionInitiatedEvent(int64, *types.HistoryEvent, string) (*persistence.ChildExecutionInfo, error)
		ReplicateTimerCanceledEvent(*types.HistoryEvent) error
		ReplicateTimerFiredEvent(*types.HistoryEvent) error
		ReplicateTimerStartedEvent(*types.HistoryEvent) (*persistence.TimerInfo, error)
		ReplicateTransientDecisionTaskScheduled() error
		ReplicateUpsertWorkflowSearchAttributesEvent(*types.HistoryEvent) error
		ReplicateWorkflowExecutionCancelRequestedEvent(*types.HistoryEvent) error
		ReplicateWorkflowExecutionCanceledEvent(int64, *types.HistoryEvent) error
		ReplicateWorkflowExecutionCompletedEvent(int64, *types.HistoryEvent) error
		ReplicateWorkflowExecutionContinuedAsNewEvent(int64, string, *types.HistoryEvent) error
		ReplicateWorkflowExecutionFailedEvent(int64, *types.HistoryEvent) error
		ReplicateWorkflowExecutionSignaled(*types.HistoryEvent) error
		ReplicateWorkflowExecutionStartedEvent(*string, types.WorkflowExecution, string, *types.HistoryEvent, bool) error
		ReplicateWorkflowExecutionTerminatedEvent(int64, *types.HistoryEvent) error
		ReplicateWorkflowExecutionTimedoutEvent(int64, *types.HistoryEvent) error
		SetCurrentBranchToken(branchToken []byte) error
		SetHistoryBuilder(hBuilder *HistoryBuilder)
		SetHistoryTree(treeID string) error
		SetVersionHistories(*persistence.VersionHistories) error
		UpdateActivity(*persistence.ActivityInfo) error
		UpdateActivityProgress(ai *persistence.ActivityInfo, request *types.RecordActivityTaskHeartbeatRequest)
		UpdateDecision(*DecisionInfo)
		UpdateUserTimer(*persistence.TimerInfo) error
		UpdateCurrentVersion(version int64, forceUpdate bool) error
		UpdateWorkflowStateCloseStatus(state int, closeStatus int) error

		AddTransferTasks(transferTasks ...persistence.Task)
		AddTimerTasks(timerTasks ...persistence.Task)
		GetTransferTasks() []persistence.Task
		GetTimerTasks() []persistence.Task
		DeleteTransferTasks()
		DeleteTimerTasks()

		SetUpdateCondition(int64)
		GetUpdateCondition() int64

		StartTransaction(ctx context.Context, entry *cache.DomainCacheEntry, incomingTaskVersion int64) (bool, error)
		CloseTransactionAsMutation(now time.Time, transactionPolicy TransactionPolicy) (*persistence.WorkflowMutation, []*persistence.WorkflowEvents, error)
		CloseTransactionAsSnapshot(now time.Time, transactionPolicy TransactionPolicy) (*persistence.WorkflowSnapshot, []*persistence.WorkflowEvents, error)

		GetHistorySize() int64
		SetHistorySize(size int64)

		cache.Sizeable
	}
)
