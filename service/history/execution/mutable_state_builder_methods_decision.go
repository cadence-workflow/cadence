// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package execution

import (
	"encoding/json"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

// GetDecisionInfo returns details about the in-progress decision task
func (e *mutableStateBuilder) GetDecisionInfo(
	scheduleEventID int64,
) (*DecisionInfo, bool) {
	return e.decisionTaskManager.GetDecisionInfo(scheduleEventID)
}

// UpdateDecision updates a decision task.
func (e *mutableStateBuilder) UpdateDecision(
	decision *DecisionInfo,
) {
	e.decisionTaskManager.UpdateDecision(decision)
}

// DeleteDecision deletes a decision task.
func (e *mutableStateBuilder) DeleteDecision() {
	e.decisionTaskManager.DeleteDecision()
}

func (e *mutableStateBuilder) FailDecision(
	incrementAttempt bool,
) {
	e.decisionTaskManager.FailDecision(incrementAttempt)
}

func (e *mutableStateBuilder) HasProcessedOrPendingDecision() bool {
	return e.decisionTaskManager.HasProcessedOrPendingDecision()
}

func (e *mutableStateBuilder) HasPendingDecision() bool {
	return e.decisionTaskManager.HasPendingDecision()
}

func (e *mutableStateBuilder) GetPendingDecision() (*DecisionInfo, bool) {
	return e.decisionTaskManager.GetPendingDecision()
}

func (e *mutableStateBuilder) HasInFlightDecision() bool {
	return e.decisionTaskManager.HasInFlightDecision()
}

func (e *mutableStateBuilder) GetInFlightDecision() (*DecisionInfo, bool) {
	return e.decisionTaskManager.GetInFlightDecision()
}

func (e *mutableStateBuilder) GetDecisionScheduleToStartTimeout() time.Duration {
	return e.decisionTaskManager.GetDecisionScheduleToStartTimeout()
}

func (e *mutableStateBuilder) AddFirstDecisionTaskScheduled(
	startEvent *types.HistoryEvent,
) error {
	opTag := tag.WorkflowActionDecisionTaskScheduled
	if err := e.checkMutability(opTag); err != nil {
		return err
	}
	return e.decisionTaskManager.AddFirstDecisionTaskScheduled(startEvent)
}

func (e *mutableStateBuilder) AddDecisionTaskScheduledEvent(
	bypassTaskGeneration bool,
) (*DecisionInfo, error) {
	opTag := tag.WorkflowActionDecisionTaskScheduled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskScheduledEvent(bypassTaskGeneration)
}

// originalScheduledTimestamp is to record the first scheduled decision during decision heartbeat.
func (e *mutableStateBuilder) AddDecisionTaskScheduledEventAsHeartbeat(
	bypassTaskGeneration bool,
	originalScheduledTimestamp int64,
) (*DecisionInfo, error) {
	opTag := tag.WorkflowActionDecisionTaskScheduled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskScheduledEventAsHeartbeat(bypassTaskGeneration, originalScheduledTimestamp)
}

func (e *mutableStateBuilder) ReplicateTransientDecisionTaskScheduled() error {
	return e.decisionTaskManager.ReplicateTransientDecisionTaskScheduled()
}

func (e *mutableStateBuilder) ReplicateDecisionTaskScheduledEvent(
	version int64,
	scheduleID int64,
	taskList string,
	startToCloseTimeoutSeconds int32,
	attempt int64,
	scheduleTimestamp int64,
	originalScheduledTimestamp int64,
	bypassTaskGeneration bool,
) (*DecisionInfo, error) {
	return e.decisionTaskManager.ReplicateDecisionTaskScheduledEvent(version, scheduleID, taskList, startToCloseTimeoutSeconds, attempt, scheduleTimestamp, originalScheduledTimestamp, bypassTaskGeneration)
}

func (e *mutableStateBuilder) AddDecisionTaskStartedEvent(
	scheduleEventID int64,
	requestID string,
	request *types.PollForDecisionTaskRequest,
) (*types.HistoryEvent, *DecisionInfo, error) {
	opTag := tag.WorkflowActionDecisionTaskStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskStartedEvent(scheduleEventID, requestID, request)
}

func (e *mutableStateBuilder) ReplicateDecisionTaskStartedEvent(
	decision *DecisionInfo,
	version int64,
	scheduleID int64,
	startedID int64,
	requestID string,
	timestamp int64,
) (*DecisionInfo, error) {

	return e.decisionTaskManager.ReplicateDecisionTaskStartedEvent(decision, version, scheduleID, startedID, requestID, timestamp)
}

func (e *mutableStateBuilder) CreateTransientDecisionEvents(
	decision *DecisionInfo,
	identity string,
) (*types.HistoryEvent, *types.HistoryEvent) {
	return e.decisionTaskManager.CreateTransientDecisionEvents(decision, identity)
}

// TODO: we will release the restriction when reset API allow those pending
func (e *mutableStateBuilder) CheckResettable() error {
	if len(e.GetPendingChildExecutionInfos()) > 0 {
		return &types.BadRequestError{
			Message: "it is not allowed resetting to a point that workflow has pending child types.",
		}
	}
	if len(e.GetPendingRequestCancelExternalInfos()) > 0 {
		return &types.BadRequestError{
			Message: "it is not allowed resetting to a point that workflow has pending request cancel.",
		}
	}
	if len(e.GetPendingSignalExternalInfos()) > 0 {
		return &types.BadRequestError{
			Message: "it is not allowed resetting to a point that workflow has pending signals to send.",
		}
	}
	return nil
}

func (e *mutableStateBuilder) AddDecisionTaskCompletedEvent(
	scheduleEventID int64,
	startedEventID int64,
	request *types.RespondDecisionTaskCompletedRequest,
	maxResetPoints int,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskCompletedEvent(scheduleEventID, startedEventID, request, maxResetPoints)
}

func (e *mutableStateBuilder) ReplicateDecisionTaskCompletedEvent(
	event *types.HistoryEvent,
) error {
	return e.decisionTaskManager.ReplicateDecisionTaskCompletedEvent(event)
}

func (e *mutableStateBuilder) AddDecisionTaskTimedOutEvent(
	scheduleEventID int64,
	startedEventID int64,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskTimedOutEvent(scheduleEventID, startedEventID)
}

func (e *mutableStateBuilder) ReplicateDecisionTaskTimedOutEvent(
	event *types.HistoryEvent,
) error {
	return e.decisionTaskManager.ReplicateDecisionTaskTimedOutEvent(event)
}

func (e *mutableStateBuilder) AddDecisionTaskScheduleToStartTimeoutEvent(
	scheduleEventID int64,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskScheduleToStartTimeoutEvent(scheduleEventID)
}

func (e *mutableStateBuilder) AddDecisionTaskResetTimeoutEvent(
	scheduleEventID int64,
	baseRunID string,
	newRunID string,
	forkEventVersion int64,
	reason string,
	resetRequestID string,
) (*types.HistoryEvent, error) {
	opTag := tag.WorkflowActionDecisionTaskTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskResetTimeoutEvent(
		scheduleEventID,
		baseRunID,
		newRunID,
		forkEventVersion,
		reason,
		resetRequestID,
	)
}

func (e *mutableStateBuilder) AddDecisionTaskFailedEvent(
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
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}
	return e.decisionTaskManager.AddDecisionTaskFailedEvent(
		scheduleEventID,
		startedEventID,
		cause,
		details,
		identity,
		reason,
		binChecksum,
		baseRunID,
		newRunID,
		forkEventVersion,
		resetRequestID,
	)
}

func (e *mutableStateBuilder) ReplicateDecisionTaskFailedEvent(event *types.HistoryEvent) error {
	return e.decisionTaskManager.ReplicateDecisionTaskFailedEvent(event)
}

// add BinaryCheckSum for the first decisionTaskCompletedID for auto-reset
func (e *mutableStateBuilder) addBinaryCheckSumIfNotExists(
	event *types.HistoryEvent,
	maxResetPoints int,
) error {
	binChecksum := event.GetDecisionTaskCompletedEventAttributes().GetBinaryChecksum()
	if len(binChecksum) == 0 {
		return nil
	}
	exeInfo := e.executionInfo
	var currResetPoints []*types.ResetPointInfo
	if exeInfo.AutoResetPoints != nil && exeInfo.AutoResetPoints.Points != nil {
		currResetPoints = e.executionInfo.AutoResetPoints.Points
	} else {
		currResetPoints = make([]*types.ResetPointInfo, 0, 1)
	}

	// List of all recent binary checksums associated with the types.
	var recentBinaryChecksums []string

	for _, rp := range currResetPoints {
		recentBinaryChecksums = append(recentBinaryChecksums, rp.GetBinaryChecksum())
		if rp.GetBinaryChecksum() == binChecksum {
			// this checksum already exists
			return nil
		}
	}

	recentBinaryChecksums, currResetPoints = trimBinaryChecksums(recentBinaryChecksums, currResetPoints, maxResetPoints)

	// Adding current version of the binary checksum.
	recentBinaryChecksums = append(recentBinaryChecksums, binChecksum)

	resettable := true
	err := e.CheckResettable()
	if err != nil {
		resettable = false
	}
	info := &types.ResetPointInfo{
		BinaryChecksum:           binChecksum,
		RunID:                    exeInfo.RunID,
		FirstDecisionCompletedID: event.ID,
		CreatedTimeNano:          common.Int64Ptr(e.timeSource.Now().UnixNano()),
		Resettable:               resettable,
	}
	currResetPoints = append(currResetPoints, info)
	exeInfo.AutoResetPoints = &types.ResetPoints{
		Points: currResetPoints,
	}
	bytes, err := json.Marshal(recentBinaryChecksums)
	if err != nil {
		return err
	}
	if exeInfo.SearchAttributes == nil {
		exeInfo.SearchAttributes = make(map[string][]byte)
	}
	exeInfo.SearchAttributes[definition.BinaryChecksums] = bytes
	if common.IsAdvancedVisibilityWritingEnabled(e.shard.GetConfig().WriteVisibilityStoreName(), e.shard.GetConfig().IsAdvancedVisConfigExist) {
		return e.taskGenerator.GenerateWorkflowSearchAttrTasks()
	}
	return nil
}
