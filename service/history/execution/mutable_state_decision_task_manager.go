// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination mutable_state_decision_task_manager_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	mutableStateDecisionTaskManager interface {
		ReplicateDecisionTaskScheduledEvent(
			version int64,
			scheduleID int64,
			taskList string,
			startToCloseTimeoutSeconds int32,
			attempt int64,
			scheduleTimestamp int64,
			originalScheduledTimestamp int64,
			bypassTaskGeneration bool,
		) (*DecisionInfo, error)
		ReplicateTransientDecisionTaskScheduled() error
		ReplicateDecisionTaskStartedEvent(
			decision *DecisionInfo,
			version int64,
			scheduleID int64,
			startedID int64,
			requestID string,
			timestamp int64,
		) (*DecisionInfo, error)
		ReplicateDecisionTaskCompletedEvent(event *types.HistoryEvent) error
		ReplicateDecisionTaskFailedEvent(*types.HistoryEvent) error
		ReplicateDecisionTaskTimedOutEvent(*types.HistoryEvent) error

		AddDecisionTaskScheduleToStartTimeoutEvent(scheduleEventID int64) (*types.HistoryEvent, error)
		AddDecisionTaskScheduledEventAsHeartbeat(
			bypassTaskGeneration bool,
			originalScheduledTimestamp int64,
		) (*DecisionInfo, error)
		AddDecisionTaskScheduledEvent(bypassTaskGeneration bool) (*DecisionInfo, error)
		AddFirstDecisionTaskScheduled(startEvent *types.HistoryEvent) error
		AddDecisionTaskStartedEvent(
			scheduleEventID int64,
			requestID string,
			request *types.PollForDecisionTaskRequest,
		) (*types.HistoryEvent, *DecisionInfo, error)
		AddDecisionTaskCompletedEvent(
			scheduleEventID int64,
			startedEventID int64,
			request *types.RespondDecisionTaskCompletedRequest,
			maxResetPoints int,
		) (*types.HistoryEvent, error)
		AddDecisionTaskFailedEvent(
			scheduleEventID int64,
			startedEventID int64,
			cause types.DecisionTaskFailedCause,
			details []byte,
			identity string,
			reason string,
			binChecksum string,
			baseRunID string,
			newRunID string,
			forkEventVersion int64,
			resetRequestID string,
		) (*types.HistoryEvent, error)
		AddDecisionTaskTimedOutEvent(scheduleEventID int64, startedEventID int64) (*types.HistoryEvent, error)
		AddDecisionTaskResetTimeoutEvent(
			scheduleEventID int64,
			baseRunID string,
			newRunID string,
			forkEventVersion int64,
			reason string,
			resetRequestID string,
		) (*types.HistoryEvent, error)

		FailDecision(incrementAttempt bool)
		DeleteDecision()
		UpdateDecision(decision *DecisionInfo)

		HasPendingDecision() bool
		GetPendingDecision() (*DecisionInfo, bool)
		HasInFlightDecision() bool
		GetInFlightDecision() (*DecisionInfo, bool)
		HasProcessedOrPendingDecision() bool
		GetDecisionInfo(scheduleEventID int64) (*DecisionInfo, bool)
		GetDecisionScheduleToStartTimeout() time.Duration

		CreateTransientDecisionEvents(decision *DecisionInfo, identity string) (*types.HistoryEvent, *types.HistoryEvent)
	}

	mutableStateDecisionTaskManagerImpl struct {
		msb *mutableStateBuilder
	}
)

func newMutableStateDecisionTaskManager(msb *mutableStateBuilder) mutableStateDecisionTaskManager {
	return &mutableStateDecisionTaskManagerImpl{
		msb: msb,
	}
}

func (m *mutableStateDecisionTaskManagerImpl) ReplicateDecisionTaskScheduledEvent(
	version int64,
	scheduleID int64,
	taskList string,
	startToCloseTimeoutSeconds int32,
	attempt int64,
	scheduleTimestamp int64,
	originalScheduledTimestamp int64,
	bypassTaskGeneration bool,
) (*DecisionInfo, error) {

	// set workflow state to running, since decision is scheduled
	// NOTE: for zombie workflow, should not change the state
	state, _ := m.msb.GetWorkflowStateCloseStatus()
	if state != persistence.WorkflowStateZombie {
		if err := m.msb.UpdateWorkflowStateCloseStatus(
			persistence.WorkflowStateRunning,
			persistence.WorkflowCloseStatusNone,
		); err != nil {
			return nil, err
		}
	}

	decision := &DecisionInfo{
		Version:                    version,
		ScheduleID:                 scheduleID,
		StartedID:                  constants.EmptyEventID,
		RequestID:                  constants.EmptyUUID,
		DecisionTimeout:            startToCloseTimeoutSeconds,
		TaskList:                   taskList,
		Attempt:                    attempt,
		ScheduledTimestamp:         scheduleTimestamp,
		StartedTimestamp:           0,
		OriginalScheduledTimestamp: originalScheduledTimestamp,
	}

	m.UpdateDecision(decision)

	if !bypassTaskGeneration {
		if err := m.msb.taskGenerator.GenerateDecisionScheduleTasks(decision.ScheduleID); err != nil {
			return nil, err
		}
	}
	return decision, nil
}

func (m *mutableStateDecisionTaskManagerImpl) ReplicateTransientDecisionTaskScheduled() error {
	if m.HasPendingDecision() || m.msb.GetExecutionInfo().DecisionAttempt == 0 {
		return nil
	}

	// the schedule ID for this decision is guaranteed to be wrong
	// since the next event ID is assigned at the very end of when
	// all events are applied for replication.
	// this is OK
	// 1. if a failover happen just after this transient decision,
	// AddDecisionTaskStartedEvent will handle the correction of schedule ID
	// and set the attempt to 0
	// 2. if no failover happen during the life time of this transient decision
	// then ReplicateDecisionTaskScheduledEvent will overwrite everything
	// including the decision schedule ID
	decision := &DecisionInfo{
		Version:            m.msb.GetCurrentVersion(),
		ScheduleID:         m.msb.GetNextEventID(),
		StartedID:          constants.EmptyEventID,
		RequestID:          constants.EmptyUUID,
		DecisionTimeout:    m.msb.GetExecutionInfo().DecisionStartToCloseTimeout,
		TaskList:           m.msb.GetExecutionInfo().TaskList,
		Attempt:            m.msb.GetExecutionInfo().DecisionAttempt,
		ScheduledTimestamp: m.msb.timeSource.Now().UnixNano(),
		StartedTimestamp:   0,
	}

	m.UpdateDecision(decision)

	return m.msb.taskGenerator.GenerateDecisionScheduleTasks(decision.ScheduleID)
}

func (m *mutableStateDecisionTaskManagerImpl) ReplicateDecisionTaskStartedEvent(
	decision *DecisionInfo,
	version int64,
	scheduleID int64,
	startedID int64,
	requestID string,
	timestamp int64,
) (*DecisionInfo, error) {
	// Replicator calls it with a nil decision info, and it is safe to always lookup the decision in this case as it
	// does not have to deal with transient decision case.
	var ok bool
	if decision == nil {
		decision, ok = m.GetDecisionInfo(scheduleID)
		if !ok {
			return nil, errors.NewInternalFailureError(fmt.Sprintf("unable to find decision: %v", scheduleID))
		}
		// setting decision attempt to 0 for decision task replication
		// this mainly handles transient decision completion
		// for transient decision, active side will write 2 batch in a "transaction"
		// 1. decision task scheduled & decision task started
		// 2. decision task completed & other events
		// since we need to treat each individual event batch as one transaction
		// certain "magic" needs to be done, i.e. setting attempt to 0 so
		// if first batch is replicated, but not the second one, decision can be correctly timed out
		decision.Attempt = 0
	}

	// Update mutable decision state
	decision = &DecisionInfo{
		Version:                    version,
		ScheduleID:                 scheduleID,
		StartedID:                  startedID,
		RequestID:                  requestID,
		DecisionTimeout:            decision.DecisionTimeout,
		Attempt:                    decision.Attempt,
		StartedTimestamp:           timestamp,
		ScheduledTimestamp:         decision.ScheduledTimestamp,
		TaskList:                   decision.TaskList,
		OriginalScheduledTimestamp: decision.OriginalScheduledTimestamp,
	}

	m.UpdateDecision(decision)
	return decision, m.msb.taskGenerator.GenerateDecisionStartTasks(scheduleID)
}

func (m *mutableStateDecisionTaskManagerImpl) ReplicateDecisionTaskCompletedEvent(
	event *types.HistoryEvent,
) error {
	m.beforeAddDecisionTaskCompletedEvent()
	maxResetPoints := constants.DefaultHistoryMaxAutoResetPoints // use default when it is not set in the config
	if m.msb.GetDomainEntry() != nil && m.msb.GetDomainEntry().GetInfo() != nil && m.msb.config != nil {
		domainName := m.msb.GetDomainEntry().GetInfo().Name
		maxResetPoints = m.msb.config.MaxAutoResetPoints(domainName)
	}
	return m.afterAddDecisionTaskCompletedEvent(event, maxResetPoints)
}

func (m *mutableStateDecisionTaskManagerImpl) ReplicateDecisionTaskFailedEvent(event *types.HistoryEvent) error {
	if event != nil && event.DecisionTaskFailedEventAttributes.GetCause() == types.DecisionTaskFailedCauseResetWorkflow {
		m.msb.insertWorkflowRequest(persistence.WorkflowRequest{
			RequestID:   event.DecisionTaskFailedEventAttributes.RequestID,
			Version:     event.Version,
			RequestType: persistence.WorkflowRequestTypeReset,
		})
	}
	m.FailDecision(true)
	return nil
}

func (m *mutableStateDecisionTaskManagerImpl) ReplicateDecisionTaskTimedOutEvent(
	event *types.HistoryEvent,
) error {
	timeoutType := event.DecisionTaskTimedOutEventAttributes.GetTimeoutType()
	incrementAttempt := true
	// Do not increment decision attempt in the case of sticky scheduleToStart timeout to
	// prevent creating next decision as transient
	// Note: this is just best effort, stickiness can be cleared before the timer fires,
	// and we can't tell is the decision that is having scheduleToStart timeout is sticky
	// or not.
	if timeoutType == types.TimeoutTypeScheduleToStart &&
		m.msb.executionInfo.StickyTaskList != "" {
		incrementAttempt = false
	}
	if event.DecisionTaskTimedOutEventAttributes.GetCause() == types.DecisionTaskTimedOutCauseReset {
		m.msb.insertWorkflowRequest(persistence.WorkflowRequest{
			RequestID:   event.DecisionTaskTimedOutEventAttributes.GetRequestID(),
			Version:     event.Version,
			RequestType: persistence.WorkflowRequestTypeReset,
		})
	}
	m.FailDecision(incrementAttempt)
	return nil
}

func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskScheduleToStartTimeoutEvent(
	scheduleEventID int64,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskTimedOut
	if m.msb.executionInfo.DecisionScheduleID != scheduleEventID || m.msb.executionInfo.DecisionStartedID > 0 {
		m.msb.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(m.msb.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
		)
		return nil, m.msb.createInternalServerError(opTag)
	}

	var event *types.HistoryEvent
	// stickyness will be cleared in ReplicateDecisionTaskTimedOutEvent
	// Avoid creating new history events when decisions are continuously timing out
	if m.msb.executionInfo.DecisionAttempt == 0 {
		event = m.msb.hBuilder.AddDecisionTaskTimedOutEvent(
			scheduleEventID,
			0,
			types.TimeoutTypeScheduleToStart,
			"",
			"",
			constants.EmptyVersion,
			"",
			types.DecisionTaskTimedOutCauseTimeout,
			"",
		)
		if err := m.ReplicateDecisionTaskTimedOutEvent(event); err != nil {
			return nil, err
		}
	} else {
		if err := m.ReplicateDecisionTaskTimedOutEvent(&types.HistoryEvent{
			DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
				TimeoutType: types.TimeoutTypeScheduleToStart.Ptr(),
			},
		}); err != nil {
			return nil, err
		}
	}

	return event, nil
}

func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskResetTimeoutEvent(
	scheduleEventID int64,
	baseRunID string,
	newRunID string,
	forkEventVersion int64,
	reason string,
	resetRequestID string,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskTimedOut
	if m.msb.executionInfo.DecisionScheduleID != scheduleEventID {
		m.msb.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(m.msb.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
		)
		return nil, m.msb.createInternalServerError(opTag)
	}

	event := m.msb.hBuilder.AddDecisionTaskTimedOutEvent(
		scheduleEventID,
		0,
		types.TimeoutTypeScheduleToStart,
		baseRunID,
		newRunID,
		forkEventVersion,
		reason,
		types.DecisionTaskTimedOutCauseReset,
		resetRequestID,
	)

	if err := m.ReplicateDecisionTaskTimedOutEvent(event); err != nil {
		return nil, err
	}

	// always clear decision attempt for reset
	m.msb.executionInfo.DecisionAttempt = 0
	return event, nil
}

// originalScheduledTimestamp is to record the first scheduled decision during decision heartbeat.
func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskScheduledEventAsHeartbeat(
	bypassTaskGeneration bool,
	originalScheduledTimestamp int64,
) (*DecisionInfo, error) {
	opTag := tag.WorkflowActionDecisionTaskScheduled
	if m.HasPendingDecision() {
		m.msb.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(m.msb.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(m.msb.executionInfo.DecisionScheduleID))
		return nil, m.msb.createInternalServerError(opTag)
	}

	// Tasklist and decision timeout should already be set from workflow execution started event
	taskList := m.msb.executionInfo.TaskList
	if m.msb.IsStickyTaskListEnabled() {
		taskList = m.msb.executionInfo.StickyTaskList
	} else {
		// It can be because stickyness has expired due to StickyTTL config
		// In that case we need to clear stickyness so that the LastUpdateTimestamp is not corrupted.
		// In other cases, clearing stickyness shouldn't hurt anything.
		// TODO: https://github.com/uber/cadence/issues/2357:
		//  if we can use a new field(LastDecisionUpdateTimestamp), then we could get rid of it.
		m.msb.ClearStickyness()
	}
	startToCloseTimeoutSeconds := m.msb.executionInfo.DecisionStartToCloseTimeout

	// Flush any buffered events before creating the decision, otherwise it will result in invalid IDs for transient
	// decision and will cause in timeout processing to not work for transient decisions
	if m.msb.HasBufferedEvents() {
		// if creating a decision and in the mean time events are flushed from buffered events
		// than this decision cannot be a transient decision
		m.msb.executionInfo.DecisionAttempt = 0
		if err := m.msb.FlushBufferedEvents(); err != nil {
			return nil, err
		}
	}

	var newDecisionEvent *types.HistoryEvent
	scheduleID := m.msb.GetNextEventID() // we will generate the schedule event later for repeatedly failing decisions
	// Avoid creating new history events when decisions are continuously failing
	scheduleTime := m.msb.timeSource.Now().UnixNano()
	useNonTransientDecision := m.shouldUpdateLastWriteVersion()

	if m.msb.executionInfo.DecisionAttempt == 0 || useNonTransientDecision {
		newDecisionEvent = m.msb.hBuilder.AddDecisionTaskScheduledEvent(
			taskList,
			startToCloseTimeoutSeconds,
			m.msb.executionInfo.DecisionAttempt)
		scheduleID = newDecisionEvent.ID
		scheduleTime = newDecisionEvent.GetTimestamp()
		m.msb.executionInfo.DecisionAttempt = 0
	}

	return m.ReplicateDecisionTaskScheduledEvent(
		m.msb.GetCurrentVersion(),
		scheduleID,
		taskList,
		startToCloseTimeoutSeconds,
		m.msb.executionInfo.DecisionAttempt,
		scheduleTime,
		originalScheduledTimestamp,
		bypassTaskGeneration,
	)
}

func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskScheduledEvent(
	bypassTaskGeneration bool,
) (*DecisionInfo, error) {
	return m.AddDecisionTaskScheduledEventAsHeartbeat(bypassTaskGeneration, m.msb.timeSource.Now().UnixNano())
}

func (m *mutableStateDecisionTaskManagerImpl) AddFirstDecisionTaskScheduled(
	startEvent *types.HistoryEvent,
) error {
	// handle first decision case, i.e. possible delayed decision
	//
	// below handles the following cases:
	// 1. if not continue as new & if workflow has no parent
	//   -> schedule decision & schedule delayed decision
	// 2. if not continue as new & if workflow has parent
	//   -> this function should not be called during workflow start, but should be called as
	//      part of schedule decision in 2 phase commit
	//
	// if continue as new
	//  1. whether has parent workflow or not
	//   -> schedule decision & schedule delayed decision
	//
	startAttr := startEvent.WorkflowExecutionStartedEventAttributes
	decisionBackoffDuration := time.Duration(startAttr.GetFirstDecisionTaskBackoffSeconds()) * time.Second

	var err error
	if decisionBackoffDuration != 0 {
		if err = m.msb.taskGenerator.GenerateDelayedDecisionTasks(
			startEvent,
		); err != nil {
			return err
		}
	} else {
		if _, err = m.AddDecisionTaskScheduledEvent(
			false,
		); err != nil {
			return err
		}
	}

	return nil
}

func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskStartedEvent(
	scheduleEventID int64,
	requestID string,
	request *types.PollForDecisionTaskRequest,
) (*types.HistoryEvent, *DecisionInfo, error) {
	opTag := tag.WorkflowActionDecisionTaskStarted
	decision, ok := m.GetDecisionInfo(scheduleEventID)
	if !ok || decision.StartedID != constants.EmptyEventID {
		m.msb.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(m.msb.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID))
		return nil, nil, m.msb.createInternalServerError(opTag)
	}

	var event *types.HistoryEvent
	scheduleID := decision.ScheduleID
	startedID := scheduleID + 1
	tasklist := request.TaskList.GetName()
	startTime := m.msb.timeSource.Now().UnixNano()
	useNonTransientDecision := m.shouldUpdateLastWriteVersion()

	// First check to see if new events came since transient decision was scheduled
	if decision.Attempt > 0 && (decision.ScheduleID != m.msb.GetNextEventID() || useNonTransientDecision) {
		// Also create a new DecisionTaskScheduledEvent since new events came in when it was scheduled
		scheduleEvent := m.msb.hBuilder.AddDecisionTaskScheduledEvent(tasklist, decision.DecisionTimeout, 0)
		scheduleID = scheduleEvent.ID
		decision.Attempt = 0
	}

	// Avoid creating new history events when decisions are continuously failing
	if decision.Attempt == 0 {
		// Now create DecisionTaskStartedEvent
		event = m.msb.hBuilder.AddDecisionTaskStartedEvent(scheduleID, requestID, request.GetIdentity())
		startedID = event.ID
		startTime = event.GetTimestamp()
	}

	decision, err := m.ReplicateDecisionTaskStartedEvent(decision, m.msb.GetCurrentVersion(), scheduleID, startedID, requestID, startTime)
	return event, decision, err
}

func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskCompletedEvent(
	scheduleEventID int64,
	startedEventID int64,
	request *types.RespondDecisionTaskCompletedRequest,
	maxResetPoints int,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskCompleted
	decision, ok := m.GetDecisionInfo(scheduleEventID)
	if !ok || decision.StartedID != startedEventID {
		m.msb.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(m.msb.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))

		return nil, m.msb.createInternalServerError(opTag)
	}

	m.beforeAddDecisionTaskCompletedEvent()
	if decision.Attempt > 0 {
		// Create corresponding DecisionTaskSchedule and DecisionTaskStarted events for decisions we have been retrying
		scheduledEvent := m.msb.hBuilder.AddTransientDecisionTaskScheduledEvent(m.msb.executionInfo.TaskList, decision.DecisionTimeout,
			decision.Attempt, decision.ScheduledTimestamp)
		startedEvent := m.msb.hBuilder.AddTransientDecisionTaskStartedEvent(scheduledEvent.ID, decision.RequestID,
			request.GetIdentity(), decision.StartedTimestamp)
		startedEventID = startedEvent.ID
	}
	// Now write the completed event
	event := m.msb.hBuilder.AddDecisionTaskCompletedEvent(scheduleEventID, startedEventID, request)

	err := m.afterAddDecisionTaskCompletedEvent(event, maxResetPoints)
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskFailedEvent(
	scheduleEventID int64,
	startedEventID int64,
	cause types.DecisionTaskFailedCause,
	details []byte,
	identity string,
	reason string,
	binChecksum string,
	baseRunID string,
	newRunID string,
	forkEventVersion int64,
	resetRequestID string,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskFailed
	attr := types.DecisionTaskFailedEventAttributes{
		ScheduledEventID: scheduleEventID,
		StartedEventID:   startedEventID,
		Cause:            cause.Ptr(),
		Details:          details,
		Identity:         identity,
		Reason:           common.StringPtr(reason),
		BinaryChecksum:   binChecksum,
		BaseRunID:        baseRunID,
		NewRunID:         newRunID,
		ForkEventVersion: forkEventVersion,
		RequestID:        resetRequestID,
	}

	dt, ok := m.GetDecisionInfo(scheduleEventID)
	if !ok || dt.StartedID != startedEventID {
		m.msb.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(m.msb.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, m.msb.createInternalServerError(opTag)
	}

	var event *types.HistoryEvent
	// Only emit DecisionTaskFailedEvent for the very first time
	if dt.Attempt == 0 {
		event = m.msb.hBuilder.AddDecisionTaskFailedEvent(attr)
	}

	if err := m.ReplicateDecisionTaskFailedEvent(event); err != nil {
		return nil, err
	}

	// always clear decision attempt for reset
	if cause == types.DecisionTaskFailedCauseResetWorkflow ||
		cause == types.DecisionTaskFailedCauseFailoverCloseDecision {
		m.msb.executionInfo.DecisionAttempt = 0
	}
	return event, nil
}

func (m *mutableStateDecisionTaskManagerImpl) AddDecisionTaskTimedOutEvent(
	scheduleEventID int64,
	startedEventID int64,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskTimedOut
	dt, ok := m.GetDecisionInfo(scheduleEventID)
	if !ok || dt.StartedID != startedEventID {
		m.msb.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(m.msb.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, m.msb.createInternalServerError(opTag)
	}

	event := &types.HistoryEvent{
		DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
			TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
		},
	}
	// Avoid creating new history events when decisions are continuously timing out
	if dt.Attempt == 0 {
		event = m.msb.hBuilder.AddDecisionTaskTimedOutEvent(
			scheduleEventID,
			startedEventID,
			types.TimeoutTypeStartToClose,
			"",
			"",
			constants.EmptyVersion,
			"",
			types.DecisionTaskTimedOutCauseTimeout,
			"",
		)
	}

	if err := m.ReplicateDecisionTaskTimedOutEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (m *mutableStateDecisionTaskManagerImpl) FailDecision(
	incrementAttempt bool,
) {
	// Clear stickiness whenever decision fails
	m.msb.ClearStickyness()

	failDecisionInfo := &DecisionInfo{
		Version:                    constants.EmptyVersion,
		ScheduleID:                 constants.EmptyEventID,
		StartedID:                  constants.EmptyEventID,
		RequestID:                  constants.EmptyUUID,
		DecisionTimeout:            0,
		StartedTimestamp:           0,
		TaskList:                   "",
		OriginalScheduledTimestamp: 0,
	}

	if incrementAttempt {
		failDecisionInfo.Attempt = m.msb.executionInfo.DecisionAttempt + 1
		failDecisionInfo.ScheduledTimestamp = m.msb.timeSource.Now().UnixNano()

		if failDecisionInfo.Attempt >= int64(m.msb.shard.GetConfig().DecisionRetryCriticalAttempts()) {
			domainName := m.msb.GetDomainEntry().GetInfo().Name
			domainTag := metrics.DomainTag(domainName)
			m.msb.metricsClient.Scope(metrics.WorkflowContextScope, domainTag).RecordTimer(metrics.DecisionAttemptTimer, time.Duration(failDecisionInfo.Attempt))
			m.msb.logger.Warn("Critical error processing decision task, retrying.",
				tag.WorkflowDomainName(m.msb.GetDomainEntry().GetInfo().Name),
				tag.WorkflowID(m.msb.GetExecutionInfo().WorkflowID),
				tag.WorkflowRunID(m.msb.GetExecutionInfo().RunID),
			)
		}
	}
	m.UpdateDecision(failDecisionInfo)
}

// DeleteDecision deletes a decision task.
func (m *mutableStateDecisionTaskManagerImpl) DeleteDecision() {
	resetDecisionInfo := &DecisionInfo{
		Version:            constants.EmptyVersion,
		ScheduleID:         constants.EmptyEventID,
		StartedID:          constants.EmptyEventID,
		RequestID:          constants.EmptyUUID,
		DecisionTimeout:    0,
		Attempt:            0,
		StartedTimestamp:   0,
		ScheduledTimestamp: 0,
		TaskList:           "",
		// Keep the last original scheduled timestamp, so that AddDecisionAsHeartbeat can continue with it.
		OriginalScheduledTimestamp: m.getDecisionInfo().OriginalScheduledTimestamp,
	}
	m.UpdateDecision(resetDecisionInfo)
}

// UpdateDecision updates a decision task.
func (m *mutableStateDecisionTaskManagerImpl) UpdateDecision(
	decision *DecisionInfo,
) {
	m.msb.executionInfo.DecisionVersion = decision.Version
	m.msb.executionInfo.DecisionScheduleID = decision.ScheduleID
	m.msb.executionInfo.DecisionStartedID = decision.StartedID
	m.msb.executionInfo.DecisionRequestID = decision.RequestID
	m.msb.executionInfo.DecisionTimeout = decision.DecisionTimeout
	m.msb.executionInfo.DecisionAttempt = decision.Attempt
	m.msb.executionInfo.DecisionStartedTimestamp = decision.StartedTimestamp
	m.msb.executionInfo.DecisionScheduledTimestamp = decision.ScheduledTimestamp
	m.msb.executionInfo.DecisionOriginalScheduledTimestamp = decision.OriginalScheduledTimestamp

	// NOTE: do not update tasklist in execution info

	if m.msb.logger.DebugOn() {
		m.msb.logger.Debugf(
			"Decision Updated: {Schedule: %v, Started: %v, ID: %v, Timeout: %v, Attempt: %v, Timestamp: %v}, Stacktrace: %v",
			decision.ScheduleID,
			decision.StartedID,
			decision.RequestID,
			decision.DecisionTimeout,
			decision.Attempt,
			decision.StartedTimestamp,
			debug.Stack(),
		)
	}
}

func (m *mutableStateDecisionTaskManagerImpl) HasPendingDecision() bool {
	return m.msb.executionInfo.DecisionScheduleID != constants.EmptyEventID
}

func (m *mutableStateDecisionTaskManagerImpl) GetPendingDecision() (*DecisionInfo, bool) {
	if m.msb.executionInfo.DecisionScheduleID == constants.EmptyEventID {
		return nil, false
	}

	decision := m.getDecisionInfo()
	return decision, true
}

func (m *mutableStateDecisionTaskManagerImpl) HasInFlightDecision() bool {
	return m.msb.executionInfo.DecisionStartedID > 0
}

func (m *mutableStateDecisionTaskManagerImpl) GetInFlightDecision() (*DecisionInfo, bool) {
	if m.msb.executionInfo.DecisionScheduleID == constants.EmptyEventID ||
		m.msb.executionInfo.DecisionStartedID == constants.EmptyEventID {
		return nil, false
	}

	decision := m.getDecisionInfo()
	return decision, true
}

func (m *mutableStateDecisionTaskManagerImpl) HasProcessedOrPendingDecision() bool {
	return m.HasPendingDecision() || m.msb.GetPreviousStartedEventID() != constants.EmptyEventID
}

// GetDecisionInfo returns details about the in-progress decision task
func (m *mutableStateDecisionTaskManagerImpl) GetDecisionInfo(
	scheduleEventID int64,
) (*DecisionInfo, bool) {
	decision := m.getDecisionInfo()
	if scheduleEventID == decision.ScheduleID {
		return decision, true
	}
	return nil, false
}

func (m *mutableStateDecisionTaskManagerImpl) GetDecisionScheduleToStartTimeout() time.Duration {
	// we should not call IsStickyTaskListEnabled which may be false
	// if sticky TTL has expired
	// NOTE: this function is called in the same mutable state transaction as the one generating the decision task
	// we stickiness won't be cleared between creating the decision and getting the timeout
	if m.msb.executionInfo.StickyTaskList != "" {
		return time.Duration(
			m.msb.executionInfo.StickyScheduleToStartTimeout,
		) * time.Second
	}

	domainName := m.msb.GetDomainEntry().GetInfo().Name
	if m.msb.executionInfo.DecisionAttempt <
		int64(m.msb.config.NormalDecisionScheduleToStartMaxAttempts(domainName)) {
		return m.msb.config.NormalDecisionScheduleToStartTimeout(domainName)
	}
	return 0
}

func (m *mutableStateDecisionTaskManagerImpl) CreateTransientDecisionEvents(
	decision *DecisionInfo,
	identity string,
) (*types.HistoryEvent, *types.HistoryEvent) {
	tasklist := m.msb.executionInfo.TaskList
	scheduledEvent := newDecisionTaskScheduledEventWithInfo(
		decision.ScheduleID,
		decision.ScheduledTimestamp,
		tasklist,
		decision.DecisionTimeout,
		decision.Attempt,
	)

	startedEvent := newDecisionTaskStartedEventWithInfo(
		decision.StartedID,
		decision.StartedTimestamp,
		decision.ScheduleID,
		decision.RequestID,
		identity,
	)

	return scheduledEvent, startedEvent
}

func (m *mutableStateDecisionTaskManagerImpl) getDecisionInfo() *DecisionInfo {
	taskList := m.msb.executionInfo.TaskList
	if m.msb.executionInfo.StickyTaskList != "" {
		taskList = m.msb.executionInfo.StickyTaskList
	}
	return &DecisionInfo{
		Version:                    m.msb.executionInfo.DecisionVersion,
		ScheduleID:                 m.msb.executionInfo.DecisionScheduleID,
		StartedID:                  m.msb.executionInfo.DecisionStartedID,
		RequestID:                  m.msb.executionInfo.DecisionRequestID,
		DecisionTimeout:            m.msb.executionInfo.DecisionTimeout,
		Attempt:                    m.msb.executionInfo.DecisionAttempt,
		StartedTimestamp:           m.msb.executionInfo.DecisionStartedTimestamp,
		ScheduledTimestamp:         m.msb.executionInfo.DecisionScheduledTimestamp,
		TaskList:                   taskList,
		OriginalScheduledTimestamp: m.msb.executionInfo.DecisionOriginalScheduledTimestamp,
	}
}

func (m *mutableStateDecisionTaskManagerImpl) beforeAddDecisionTaskCompletedEvent() {
	// Make sure to delete decision before adding events.  Otherwise they are buffered rather than getting appended
	m.DeleteDecision()
}

func (m *mutableStateDecisionTaskManagerImpl) afterAddDecisionTaskCompletedEvent(
	event *types.HistoryEvent,
	maxResetPoints int,
) error {
	m.msb.executionInfo.LastProcessedEvent = event.GetDecisionTaskCompletedEventAttributes().GetStartedEventID()
	return m.msb.addBinaryCheckSumIfNotExists(event, maxResetPoints)
}

func (m *mutableStateDecisionTaskManagerImpl) shouldUpdateLastWriteVersion() bool {

	currentVersion := m.msb.GetCurrentVersion()
	lastWriteVersion, err := m.msb.GetLastWriteVersion()
	if err != nil {
		// The error is version history has no item. This is expected for the first batch of a workflow.
		return false
	}
	return currentVersion != lastWriteVersion
}
