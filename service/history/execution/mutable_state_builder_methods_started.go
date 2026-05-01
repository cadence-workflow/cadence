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
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func (e *mutableStateBuilder) addWorkflowExecutionStartedEventForContinueAsNew(
	parentExecutionInfo *types.ParentExecutionInfo,
	execution types.WorkflowExecution,
	previousExecutionState MutableState,
	attributes *types.ContinueAsNewWorkflowExecutionDecisionAttributes,
	firstRunID string,
	firstScheduledTime time.Time,
) (*types.HistoryEvent, error) {

	previousExecutionInfo := previousExecutionState.GetExecutionInfo()
	tl := &types.TaskList{
		Name: previousExecutionInfo.TaskList,
		Kind: previousExecutionInfo.TaskListKind.Ptr(),
	}
	// ContinueAsNew can change the name, not the kind
	if attributes.TaskList != nil {
		tl.Name = attributes.TaskList.Name
	}

	workflowType := previousExecutionInfo.WorkflowTypeName
	if attributes.WorkflowType != nil {
		workflowType = attributes.WorkflowType.GetName()
	}
	wType := &types.WorkflowType{}
	wType.Name = workflowType

	decisionTimeout := previousExecutionInfo.DecisionStartToCloseTimeout
	if attributes.TaskStartToCloseTimeoutSeconds != nil {
		decisionTimeout = attributes.GetTaskStartToCloseTimeoutSeconds()
	}

	createRequest := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              e.domainEntry.GetInfo().Name,
		WorkflowID:                          execution.WorkflowID,
		TaskList:                            tl,
		WorkflowType:                        wType,
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(decisionTimeout),
		ExecutionStartToCloseTimeoutSeconds: attributes.ExecutionStartToCloseTimeoutSeconds,
		Input:                               attributes.Input,
		Header:                              attributes.Header,
		RetryPolicy:                         attributes.RetryPolicy,
		CronSchedule:                        attributes.CronSchedule,
		Memo:                                attributes.Memo,
		SearchAttributes:                    attributes.SearchAttributes,
		JitterStartSeconds:                  attributes.JitterStartSeconds,
		CronOverlapPolicy:                   attributes.CronOverlapPolicy,
		ActiveClusterSelectionPolicy:        attributes.ActiveClusterSelectionPolicy,
	}

	req := &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID:                      e.domainEntry.GetInfo().ID,
		StartRequest:                    createRequest,
		ParentExecutionInfo:             parentExecutionInfo,
		LastCompletionResult:            attributes.LastCompletionResult,
		ContinuedFailureReason:          attributes.FailureReason,
		ContinuedFailureDetails:         attributes.FailureDetails,
		ContinueAsNewInitiator:          attributes.Initiator,
		FirstDecisionTaskBackoffSeconds: attributes.BackoffStartIntervalInSeconds,
		PartitionConfig:                 previousExecutionInfo.PartitionConfig,
	}

	// if ContinueAsNew as Cron or decider, recalculate the expiration timestamp and set attempts to 0
	req.Attempt = 0
	if attributes.RetryPolicy != nil && attributes.RetryPolicy.GetExpirationIntervalInSeconds() > 0 {
		// expirationTime calculates from first decision task schedule to the end of the workflow
		expirationInSeconds := attributes.RetryPolicy.GetExpirationIntervalInSeconds() + req.GetFirstDecisionTaskBackoffSeconds()
		expirationTime := e.timeSource.Now().Add(time.Second * time.Duration(expirationInSeconds))
		req.ExpirationTimestamp = common.Int64Ptr(expirationTime.UnixNano())
	}
	// if ContinueAsNew as retry use the same expiration timestamp and increment attempts from previous execution state
	if attributes.GetInitiator() == types.ContinueAsNewInitiatorRetryPolicy {
		req.Attempt = previousExecutionState.GetExecutionInfo().Attempt + 1
		expirationTime := previousExecutionState.GetExecutionInfo().ExpirationTime
		if !expirationTime.IsZero() {
			req.ExpirationTimestamp = common.Int64Ptr(expirationTime.UnixNano())
		}
	}

	// History event only has domainName so domainID has to be passed in explicitly to update the mutable state
	var parentDomainID *string
	if parentExecutionInfo != nil {
		parentDomainID = &parentExecutionInfo.DomainUUID
	}

	event := e.hBuilder.AddWorkflowExecutionStartedEvent(req, previousExecutionInfo, firstRunID, execution.GetRunID(), firstScheduledTime)
	if err := e.ReplicateWorkflowExecutionStartedEvent(
		parentDomainID,
		execution,
		createRequest.GetRequestID(),
		event,
		false,
	); err != nil {
		return nil, err
	}

	if err := e.SetHistoryTree(e.GetExecutionInfo().RunID); err != nil {
		return nil, err
	}

	if err := e.AddFirstDecisionTaskScheduled(
		event,
	); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionStartedEvent(
	execution types.WorkflowExecution,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	request := startRequest.StartRequest
	eventID := e.GetNextEventID()
	if eventID != constants.FirstEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(eventID),
			tag.ErrorTypeInvalidHistoryAction)
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddWorkflowExecutionStartedEvent(startRequest, nil, execution.GetRunID(), execution.GetRunID(),
		time.Now())

	var parentDomainID *string
	if startRequest.ParentExecutionInfo != nil {
		parentDomainID = &startRequest.ParentExecutionInfo.DomainUUID
	}
	if err := e.ReplicateWorkflowExecutionStartedEvent(
		parentDomainID,
		execution,
		request.GetRequestID(),
		event,
		false); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionStartedEvent(
	parentDomainID *string,
	execution types.WorkflowExecution,
	requestID string,
	startEvent *types.HistoryEvent,
	generateDelayedDecisionTasks bool,
) error {

	event := startEvent.WorkflowExecutionStartedEventAttributes
	if event.GetRequestID() != "" {
		// prefer requestID from history event, ideally we should remove the requestID parameter
		// removing it may or may not be backward compatible, so keep it now
		requestID = event.GetRequestID()
	}
	e.executionInfo.CreateRequestID = requestID

	e.insertWorkflowRequest(persistence.WorkflowRequest{
		RequestID:   requestID,
		Version:     startEvent.Version,
		RequestType: persistence.WorkflowRequestTypeStart,
	})
	e.executionInfo.DomainID = e.domainEntry.GetInfo().ID
	e.executionInfo.WorkflowID = execution.GetWorkflowID()
	e.executionInfo.RunID = execution.GetRunID()
	e.executionInfo.FirstExecutionRunID = event.GetFirstExecutionRunID()
	if e.executionInfo.FirstExecutionRunID == "" {
		e.executionInfo.FirstExecutionRunID = execution.GetRunID()
	}
	e.executionInfo.TaskList = event.TaskList.GetName()
	e.executionInfo.TaskListKind = event.TaskList.GetKind()
	e.executionInfo.WorkflowTypeName = event.WorkflowType.GetName()
	e.executionInfo.WorkflowTimeout = event.GetExecutionStartToCloseTimeoutSeconds()
	e.executionInfo.DecisionStartToCloseTimeout = event.GetTaskStartToCloseTimeoutSeconds()
	e.executionInfo.StartTimestamp = e.unixNanoToTime(startEvent.GetTimestamp())

	if err := e.UpdateWorkflowStateCloseStatus(
		persistence.WorkflowStateCreated,
		persistence.WorkflowCloseStatusNone,
	); err != nil {
		return err
	}
	e.executionInfo.LastProcessedEvent = constants.EmptyEventID
	e.executionInfo.LastFirstEventID = startEvent.ID

	e.executionInfo.DecisionVersion = constants.EmptyVersion
	e.executionInfo.DecisionScheduleID = constants.EmptyEventID
	e.executionInfo.DecisionStartedID = constants.EmptyEventID
	e.executionInfo.DecisionRequestID = constants.EmptyUUID
	e.executionInfo.DecisionTimeout = 0

	e.executionInfo.CronSchedule = event.GetCronSchedule()
	if event.CronOverlapPolicy != nil {
		e.executionInfo.CronOverlapPolicy = *event.CronOverlapPolicy
	}

	if parentDomainID != nil {
		e.executionInfo.ParentDomainID = *parentDomainID
	}
	if event.ParentWorkflowExecution != nil {
		e.executionInfo.ParentWorkflowID = event.ParentWorkflowExecution.GetWorkflowID()
		e.executionInfo.ParentRunID = event.ParentWorkflowExecution.GetRunID()
	}
	if event.ParentInitiatedEventID != nil {
		e.executionInfo.InitiatedID = event.GetParentInitiatedEventID()
	} else {
		e.executionInfo.InitiatedID = constants.EmptyEventID
	}

	if event.CronOverlapPolicy != nil {
		e.executionInfo.CronOverlapPolicy = *event.CronOverlapPolicy
	}

	e.executionInfo.Attempt = event.GetAttempt()
	if event.GetExpirationTimestamp() != 0 {
		e.executionInfo.ExpirationTime = time.Unix(0, event.GetExpirationTimestamp())
	}
	if event.RetryPolicy != nil {
		e.executionInfo.HasRetryPolicy = true
		e.executionInfo.BackoffCoefficient = event.RetryPolicy.GetBackoffCoefficient()
		e.executionInfo.ExpirationSeconds = event.RetryPolicy.GetExpirationIntervalInSeconds()
		e.executionInfo.InitialInterval = event.RetryPolicy.GetInitialIntervalInSeconds()
		e.executionInfo.MaximumAttempts = event.RetryPolicy.GetMaximumAttempts()
		e.executionInfo.MaximumInterval = event.RetryPolicy.GetMaximumIntervalInSeconds()
		e.executionInfo.NonRetriableErrors = event.RetryPolicy.NonRetriableErrorReasons
	}

	e.executionInfo.AutoResetPoints = rolloverAutoResetPointsWithExpiringTime(
		event.GetPrevAutoResetPoints(),
		event.GetContinuedExecutionRunID(),
		startEvent.GetTimestamp(),
		e.domainEntry.GetRetentionDays(e.executionInfo.WorkflowID),
	)

	if event.Memo != nil {
		e.executionInfo.Memo = event.Memo.GetFields()
	}
	if event.SearchAttributes != nil {
		e.executionInfo.SearchAttributes = event.SearchAttributes.GetIndexedFields()
	}
	e.executionInfo.PartitionConfig = event.PartitionConfig
	e.executionInfo.ActiveClusterSelectionPolicy = event.ActiveClusterSelectionPolicy

	e.writeEventToCache(startEvent)

	if err := e.taskGenerator.GenerateWorkflowStartTasks(e.unixNanoToTime(startEvent.GetTimestamp()), startEvent); err != nil {
		return err
	}
	if err := e.taskGenerator.GenerateRecordWorkflowStartedTasks(startEvent); err != nil {
		return err
	}
	if generateDelayedDecisionTasks && event.GetFirstDecisionTaskBackoffSeconds() > 0 {
		if err := e.taskGenerator.GenerateDelayedDecisionTasks(startEvent); err != nil {
			return err
		}
	}
	return nil
}
