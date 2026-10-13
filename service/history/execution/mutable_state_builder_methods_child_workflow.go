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
	"context"
	"fmt"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// childWorkflowCreateRequestIDNamespace is the namespace of the request IDs that start child workflows.
// It must never change, because every cluster must derive the same request ID for the same child.
var childWorkflowCreateRequestIDNamespace = uuid.Parse("c856ffa8-57a2-4f71-b82f-4a768903ea6f")

// GetChildExecutionInfo gives details about a child execution that is currently in progress.
func (e *mutableStateBuilder) GetChildExecutionInfo(
	initiatedEventID int64,
) (*persistence.ChildExecutionInfo, bool) {

	ci, ok := e.pendingChildExecutionInfoIDs[initiatedEventID]
	return ci, ok
}

// GetChildExecutionInitiatedEvent reads out the ChildExecutionInitiatedEvent from mutable state for in-progress child
// executions
func (e *mutableStateBuilder) GetChildExecutionInitiatedEvent(
	ctx context.Context,
	initiatedEventID int64,
) (*types.HistoryEvent, error) {

	ci, ok := e.pendingChildExecutionInfoIDs[initiatedEventID]
	if !ok {
		return nil, ErrMissingChildWorkflowInfo
	}

	// Needed for backward compatibility reason
	if ci.InitiatedEvent != nil {
		return ci.InitiatedEvent, nil
	}

	currentBranchToken, err := e.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}
	initiatedEvent, err := e.eventsCache.GetEvent(
		ctx,
		e.shard.GetShardID(),
		e.executionInfo.DomainID,
		e.executionInfo.WorkflowID,
		e.executionInfo.RunID,
		ci.InitiatedEventBatchID,
		ci.InitiatedID,
		currentBranchToken,
	)
	if err != nil {
		// do not return the original error
		// since original error can be of type entity not exists
		// which can cause task processing side to fail silently
		// However, if the error is a persistence transient error,
		// we return the original error, because we fail to get
		// the event because of failure from database
		if persistence.IsTransientError(err) {
			return nil, err
		}
		return nil, ErrMissingChildWorkflowInitiatedEvent
	}
	return initiatedEvent, nil
}

func (e *mutableStateBuilder) GetPendingChildExecutionInfos() map[int64]*persistence.ChildExecutionInfo {
	return e.pendingChildExecutionInfoIDs
}

// DeletePendingChildExecution deletes details about a ChildExecutionInfo.
func (e *mutableStateBuilder) DeletePendingChildExecution(
	initiatedEventID int64,
) error {

	if _, ok := e.pendingChildExecutionInfoIDs[initiatedEventID]; ok {
		delete(e.pendingChildExecutionInfoIDs, initiatedEventID)
	} else {
		e.logError(
			fmt.Sprintf("unable to find child workflow event ID: %v in mutable state", initiatedEventID),
			tag.ErrorTypeInvalidMutableStateAction,
		)
		// log data inconsistency instead of returning an error
		e.logDataInconsistency()
	}

	delete(e.updateChildExecutionInfos, initiatedEventID)
	e.deleteChildExecutionInfos[initiatedEventID] = struct{}{}
	return nil
}

func (e *mutableStateBuilder) AddStartChildWorkflowExecutionInitiatedEvent(
	decisionCompletedEventID int64,
	attributes *types.StartChildWorkflowExecutionDecisionAttributes,
) (*types.HistoryEvent, *persistence.ChildExecutionInfo, error) {

	opTag := tag.WorkflowActionChildWorkflowInitiated
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	event := e.hBuilder.AddStartChildWorkflowExecutionInitiatedEvent(
		decisionCompletedEventID,
		attributes,
		e.GetDomainEntry().GetInfo().Name,
	)
	// Write the event to cache only on active cluster
	e.eventsCache.PutEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID,
		event.ID, event)

	ci, err := e.ReplicateStartChildWorkflowExecutionInitiatedEvent(decisionCompletedEventID, event)
	if err != nil {
		return nil, nil, err
	}
	return event, ci, nil
}

func (e *mutableStateBuilder) ReplicateStartChildWorkflowExecutionInitiatedEvent(
	firstEventID int64,
	event *types.HistoryEvent,
) (*persistence.ChildExecutionInfo, error) {

	initiatedEventID := event.ID
	attributes := event.StartChildWorkflowExecutionInitiatedEventAttributes

	domainID := e.GetExecutionInfo().DomainID
	if domainName := attributes.GetDomain(); domainName != "" {
		// domainName may still be empty if two cadence clusters are running different versions
		var err error
		domainID, err = e.shard.GetDomainCache().GetDomainID(domainName)
		if err != nil {
			return nil, err
		}
	}
	ci := &persistence.ChildExecutionInfo{
		Version:               event.Version,
		InitiatedID:           initiatedEventID,
		InitiatedEventBatchID: firstEventID,
		StartedID:             constants.EmptyEventID,
		StartedWorkflowID:     attributes.GetWorkflowID(),
		CreateRequestID:       e.childWorkflowCreateRequestID(initiatedEventID, event.Version),
		DomainID:              domainID,
		// DomainName field is being deprecated
		// DomainName:            attributes.GetDomain(),
		WorkflowTypeName:  attributes.GetWorkflowType().GetName(),
		ParentClosePolicy: attributes.GetParentClosePolicy(),
	}

	e.pendingChildExecutionInfoIDs[ci.InitiatedID] = ci
	e.updateChildExecutionInfos[ci.InitiatedID] = ci

	return ci, e.taskGenerator.GenerateChildWorkflowTasks(event)
}

// childWorkflowCreateRequestID returns a derived request ID only when EnableDeterministicChildWorkflowRequestID is enabled.
func (e *mutableStateBuilder) childWorkflowCreateRequestID(initiatedEventID, version int64) string {
	if !e.config.EnableDeterministicChildWorkflowRequestID(e.GetDomainEntry().GetInfo().Name) {
		return uuid.New()
	}
	return newChildWorkflowCreateRequestID(e.executionInfo.RunID, initiatedEventID, version)
}

// newChildWorkflowCreateRequestID returns the same request ID on all clusters, so after a failover the child
// start finds the run started by another cluster. It is a UUID because Cassandra stores it in a UUID column.
func newChildWorkflowCreateRequestID(runID string, initiatedEventID, version int64) string {
	name := fmt.Sprintf("%s:%d:%d", runID, initiatedEventID, version)
	return uuid.NewSHA1(childWorkflowCreateRequestIDNamespace, []byte(name)).String()
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionStartedEvent(
	domain string,
	execution *types.WorkflowExecution,
	workflowType *types.WorkflowType,
	initiatedID int64,
	header *types.Header,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID != constants.EmptyEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddChildWorkflowExecutionStartedEvent(domain, execution, workflowType, initiatedID, header)
	if err := e.ReplicateChildWorkflowExecutionStartedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionStartedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ChildWorkflowExecutionStartedEventAttributes
	initiatedID := attributes.GetInitiatedEventID()

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok {
		e.logError(
			"Unable to find child workflow",
			tag.ErrorTypeInvalidMutableStateAction,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.WorkflowInitiatedID(initiatedID),
		)
		return ErrMissingChildWorkflowInfo
	}
	ci.StartedID = event.ID
	ci.StartedRunID = attributes.GetWorkflowExecution().GetRunID()
	e.updateChildExecutionInfos[ci.InitiatedID] = ci

	return nil
}

func (e *mutableStateBuilder) AddStartChildWorkflowExecutionFailedEvent(
	initiatedID int64,
	cause types.ChildWorkflowExecutionFailedCause,
	initiatedEventAttributes *types.StartChildWorkflowExecutionInitiatedEventAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowInitiationFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID != constants.EmptyEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddStartChildWorkflowExecutionFailedEvent(initiatedID, cause, initiatedEventAttributes)
	if err := e.ReplicateStartChildWorkflowExecutionFailedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateStartChildWorkflowExecutionFailedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.StartChildWorkflowExecutionFailedEventAttributes
	initiatedID := attributes.GetInitiatedEventID()

	return e.DeletePendingChildExecution(initiatedID)
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionCompletedEvent(
	initiatedID int64,
	childExecution *types.WorkflowExecution,
	attributes *types.WorkflowExecutionCompletedEventAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == constants.EmptyEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	childDomainName, err := GetChildExecutionDomainName(ci, e.shard.GetDomainCache(), e.GetDomainEntry())
	if err != nil {
		return nil, err
	}
	workflowType := &types.WorkflowType{
		Name: ci.WorkflowTypeName,
	}

	event := e.hBuilder.AddChildWorkflowExecutionCompletedEvent(
		childDomainName,
		childExecution,
		workflowType,
		ci.InitiatedID,
		ci.StartedID,
		attributes,
	)
	if err := e.ReplicateChildWorkflowExecutionCompletedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionCompletedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ChildWorkflowExecutionCompletedEventAttributes
	initiatedID := attributes.GetInitiatedEventID()

	return e.DeletePendingChildExecution(initiatedID)
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionFailedEvent(
	initiatedID int64,
	childExecution *types.WorkflowExecution,
	attributes *types.WorkflowExecutionFailedEventAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == constants.EmptyEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(!ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	domainName, err := GetChildExecutionDomainName(ci, e.shard.GetDomainCache(), e.GetDomainEntry())
	if err != nil {
		return nil, err
	}
	workflowType := &types.WorkflowType{
		Name: ci.WorkflowTypeName,
	}

	event := e.hBuilder.AddChildWorkflowExecutionFailedEvent(
		domainName,
		childExecution,
		workflowType,
		ci.InitiatedID,
		ci.StartedID,
		attributes,
	)
	if err := e.ReplicateChildWorkflowExecutionFailedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionFailedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ChildWorkflowExecutionFailedEventAttributes
	initiatedID := attributes.GetInitiatedEventID()

	return e.DeletePendingChildExecution(initiatedID)
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionCanceledEvent(
	initiatedID int64,
	childExecution *types.WorkflowExecution,
	attributes *types.WorkflowExecutionCanceledEventAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowCanceled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == constants.EmptyEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	domainName, err := GetChildExecutionDomainName(ci, e.shard.GetDomainCache(), e.GetDomainEntry())
	if err != nil {
		return nil, err
	}
	workflowType := &types.WorkflowType{
		Name: ci.WorkflowTypeName,
	}

	event := e.hBuilder.AddChildWorkflowExecutionCanceledEvent(
		domainName,
		childExecution,
		workflowType,
		ci.InitiatedID,
		ci.StartedID,
		attributes,
	)
	if err := e.ReplicateChildWorkflowExecutionCanceledEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionCanceledEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ChildWorkflowExecutionCanceledEventAttributes
	initiatedID := attributes.GetInitiatedEventID()

	return e.DeletePendingChildExecution(initiatedID)
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionTerminatedEvent(
	initiatedID int64,
	childExecution *types.WorkflowExecution,
	attributes *types.WorkflowExecutionTerminatedEventAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowTerminated
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == constants.EmptyEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	domainName, err := GetChildExecutionDomainName(ci, e.shard.GetDomainCache(), e.GetDomainEntry())
	if err != nil {
		return nil, err
	}
	workflowType := &types.WorkflowType{
		Name: ci.WorkflowTypeName,
	}

	event := e.hBuilder.AddChildWorkflowExecutionTerminatedEvent(
		domainName,
		childExecution,
		workflowType,
		ci.InitiatedID,
		ci.StartedID,
		attributes,
	)
	if err := e.ReplicateChildWorkflowExecutionTerminatedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionTerminatedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ChildWorkflowExecutionTerminatedEventAttributes
	initiatedID := attributes.GetInitiatedEventID()

	return e.DeletePendingChildExecution(initiatedID)
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionTimedOutEvent(
	initiatedID int64,
	childExecution *types.WorkflowExecution,
	attributes *types.WorkflowExecutionTimedOutEventAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == constants.EmptyEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	domainName, err := GetChildExecutionDomainName(ci, e.shard.GetDomainCache(), e.GetDomainEntry())
	if err != nil {
		return nil, err
	}
	workflowType := &types.WorkflowType{
		Name: ci.WorkflowTypeName,
	}

	event := e.hBuilder.AddChildWorkflowExecutionTimedOutEvent(
		domainName,
		childExecution,
		workflowType,
		ci.InitiatedID,
		ci.StartedID,
		attributes,
	)
	if err := e.ReplicateChildWorkflowExecutionTimedOutEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionTimedOutEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ChildWorkflowExecutionTimedOutEventAttributes
	initiatedID := attributes.GetInitiatedEventID()

	return e.DeletePendingChildExecution(initiatedID)
}
