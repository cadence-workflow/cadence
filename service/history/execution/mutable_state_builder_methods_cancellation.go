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
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func (e *mutableStateBuilder) IsCancelRequested() (bool, string) {
	if e.executionInfo.CancelRequested {
		return e.executionInfo.CancelRequested, e.executionInfo.CancelRequestID
	}

	return false, ""
}

// GetRequestCancelInfo gives details about a request cancellation that is currently in progress.
func (e *mutableStateBuilder) GetRequestCancelInfo(
	initiatedEventID int64,
) (*persistence.RequestCancelInfo, bool) {

	ri, ok := e.pendingRequestCancelInfoIDs[initiatedEventID]
	return ri, ok
}

func (e *mutableStateBuilder) AddWorkflowExecutionCancelRequestedEvent(
	cause string,
	request *types.HistoryRequestCancelWorkflowExecutionRequest,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowCancelRequested
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if e.executionInfo.CancelRequested {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowState(e.executionInfo.State),
			tag.Bool(e.executionInfo.CancelRequested),
			tag.Key(e.executionInfo.CancelRequestID),
		)
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddWorkflowExecutionCancelRequestedEvent(cause, request)
	if err := e.ReplicateWorkflowExecutionCancelRequestedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionCancelRequestedEvent(
	event *types.HistoryEvent,
) error {

	e.executionInfo.CancelRequested = true
	requestID := event.WorkflowExecutionCancelRequestedEventAttributes.RequestID
	e.executionInfo.CancelRequestID = requestID
	e.insertWorkflowRequest(persistence.WorkflowRequest{
		RequestID:   requestID,
		Version:     event.Version,
		RequestType: persistence.WorkflowRequestTypeCancel,
	})
	return nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionCanceledEvent(
	decisionTaskCompletedEventID int64,
	attributes *types.CancelWorkflowExecutionDecisionAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowCanceled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddWorkflowExecutionCanceledEvent(decisionTaskCompletedEventID, attributes)
	if err := e.ReplicateWorkflowExecutionCanceledEvent(decisionTaskCompletedEventID, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionCanceledEvent(
	firstEventID int64,
	event *types.HistoryEvent,
) error {
	return e.closeWorkflow(firstEventID, persistence.WorkflowCloseStatusCanceled, event)
}

func (e *mutableStateBuilder) AddRequestCancelExternalWorkflowExecutionInitiatedEvent(
	decisionCompletedEventID int64,
	cancelRequestID string,
	request *types.RequestCancelExternalWorkflowExecutionDecisionAttributes,
) (*types.HistoryEvent, *persistence.RequestCancelInfo, error) {

	opTag := tag.WorkflowActionExternalWorkflowCancelInitiated
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	event := e.hBuilder.AddRequestCancelExternalWorkflowExecutionInitiatedEvent(decisionCompletedEventID, request)
	rci, err := e.ReplicateRequestCancelExternalWorkflowExecutionInitiatedEvent(decisionCompletedEventID, event, cancelRequestID)
	if err != nil {
		return nil, nil, err
	}
	return event, rci, nil
}

func (e *mutableStateBuilder) ReplicateRequestCancelExternalWorkflowExecutionInitiatedEvent(
	firstEventID int64,
	event *types.HistoryEvent,
	cancelRequestID string,
) (*persistence.RequestCancelInfo, error) {

	// TODO: Evaluate if we need cancelRequestID also part of history event
	initiatedEventID := event.ID
	rci := &persistence.RequestCancelInfo{
		Version:               event.Version,
		InitiatedEventBatchID: firstEventID,
		InitiatedID:           initiatedEventID,
		CancelRequestID:       cancelRequestID,
	}

	e.pendingRequestCancelInfoIDs[rci.InitiatedID] = rci
	e.updateRequestCancelInfos[rci.InitiatedID] = rci

	return rci, e.taskGenerator.GenerateRequestCancelExternalTasks(event)
}

func (e *mutableStateBuilder) AddExternalWorkflowExecutionCancelRequested(
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionExternalWorkflowCancelRequested
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	_, ok := e.GetRequestCancelInfo(initiatedID)
	if !ok {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddExternalWorkflowExecutionCancelRequested(initiatedID, domain, workflowID, runID)
	if err := e.ReplicateExternalWorkflowExecutionCancelRequested(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateExternalWorkflowExecutionCancelRequested(
	event *types.HistoryEvent,
) error {

	initiatedID := event.ExternalWorkflowExecutionCancelRequestedEventAttributes.GetInitiatedEventID()

	return e.DeletePendingRequestCancel(initiatedID)
}

func (e *mutableStateBuilder) AddRequestCancelExternalWorkflowExecutionFailedEvent(
	decisionTaskCompletedEventID int64,
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
	cause types.CancelExternalWorkflowExecutionFailedCause,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionExternalWorkflowCancelFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	_, ok := e.GetRequestCancelInfo(initiatedID)
	if !ok {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddRequestCancelExternalWorkflowExecutionFailedEvent(decisionTaskCompletedEventID, initiatedID,
		domain, workflowID, runID, cause)
	if err := e.ReplicateRequestCancelExternalWorkflowExecutionFailedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateRequestCancelExternalWorkflowExecutionFailedEvent(
	event *types.HistoryEvent,
) error {

	initiatedID := event.RequestCancelExternalWorkflowExecutionFailedEventAttributes.GetInitiatedEventID()

	return e.DeletePendingRequestCancel(initiatedID)
}
