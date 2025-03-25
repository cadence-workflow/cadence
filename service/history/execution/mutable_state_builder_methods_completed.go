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

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func (e *mutableStateBuilder) IsWorkflowCompleted() bool {
	return e.executionInfo.State == persistence.WorkflowStateCompleted
}

func (e *mutableStateBuilder) AddCompletedWorkflowEvent(
	decisionCompletedEventID int64,
	attributes *types.CompleteWorkflowExecutionDecisionAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddCompletedWorkflowEvent(decisionCompletedEventID, attributes)
	if err := e.ReplicateWorkflowExecutionCompletedEvent(decisionCompletedEventID, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionCompletedEvent(
	firstEventID int64,
	event *types.HistoryEvent,
) error {

	if err := e.UpdateWorkflowStateCloseStatus(
		persistence.WorkflowStateCompleted,
		persistence.WorkflowCloseStatusCompleted,
	); err != nil {
		return err
	}
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)

	return e.taskGenerator.GenerateWorkflowCloseTasks(event, e.config.WorkflowDeletionJitterRange(e.domainEntry.GetInfo().Name))
}

func (e *mutableStateBuilder) AddFailWorkflowEvent(
	decisionCompletedEventID int64,
	attributes *types.FailWorkflowExecutionDecisionAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddFailWorkflowEvent(decisionCompletedEventID, attributes)
	if err := e.ReplicateWorkflowExecutionFailedEvent(decisionCompletedEventID, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionFailedEvent(
	firstEventID int64,
	event *types.HistoryEvent,
) error {

	if err := e.UpdateWorkflowStateCloseStatus(
		persistence.WorkflowStateCompleted,
		persistence.WorkflowCloseStatusFailed,
	); err != nil {
		return err
	}
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)

	return e.taskGenerator.GenerateWorkflowCloseTasks(event, e.config.WorkflowDeletionJitterRange(e.domainEntry.GetInfo().Name))
}

// GetCompletionEvent retrieves the workflow completion event from mutable state
func (e *mutableStateBuilder) GetCompletionEvent(
	ctx context.Context,
) (*types.HistoryEvent, error) {
	if e.executionInfo.State != persistence.WorkflowStateCompleted {
		return nil, ErrMissingWorkflowCompletionEvent
	}

	// Needed for backward compatibility reason
	if e.executionInfo.CompletionEvent != nil {
		return e.executionInfo.CompletionEvent, nil
	}

	// Needed for backward compatibility reason
	if e.executionInfo.CompletionEventBatchID == constants.EmptyEventID {
		return nil, ErrMissingWorkflowCompletionEvent
	}

	currentBranchToken, err := e.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}

	// Completion EventID is always one less than NextEventID after workflow is completed
	completionEventID := e.executionInfo.NextEventID - 1
	firstEventID := e.executionInfo.CompletionEventBatchID
	completionEvent, err := e.eventsCache.GetEvent(
		ctx,
		e.shard.GetShardID(),
		e.executionInfo.DomainID,
		e.executionInfo.WorkflowID,
		e.executionInfo.RunID,
		firstEventID,
		completionEventID,
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
		return nil, ErrMissingWorkflowCompletionEvent
	}

	return completionEvent, nil
}
