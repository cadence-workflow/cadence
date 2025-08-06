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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination state_builder_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/shard"
)

type (
	// StateBuilder is the mutable state builder
	StateBuilder interface {
		ApplyEvents(
			domainID string,
			requestID string,
			workflowExecution types.WorkflowExecution,
			history []*types.HistoryEvent,
			newRunHistory []*types.HistoryEvent,
		) (MutableState, error)

		GetMutableState() MutableState
	}

	stateBuilderImpl struct {
		shard           shard.Context
		clusterMetadata cluster.Metadata
		domainCache     cache.DomainCache
		logger          log.Logger

		mutableState MutableState
	}
)

const (
	errMessageHistorySizeZero = "encounter history size being zero"
)

var _ StateBuilder = (*stateBuilderImpl)(nil)

// NewStateBuilder creates a state builder
func NewStateBuilder(
	shard shard.Context,
	logger log.Logger,
	mutableState MutableState,
) StateBuilder {

	return &stateBuilderImpl{
		shard:           shard,
		clusterMetadata: shard.GetService().GetClusterMetadata(),
		domainCache:     shard.GetDomainCache(),
		logger:          logger,
		mutableState:    mutableState,
	}
}

func (b *stateBuilderImpl) ApplyEvents(
	domainID string,
	requestID string,
	workflowExecution types.WorkflowExecution,
	history []*types.HistoryEvent,
	newRunHistory []*types.HistoryEvent,
) (MutableState, error) {

	if len(history) == 0 {
		return nil, errors.NewInternalFailureError(errMessageHistorySizeZero)
	}
	firstEvent := history[0]
	lastEvent := history[len(history)-1]
	var newRunMutableStateBuilder MutableState

	b.logger.Debugf("stateBuilderImpl Applying events for domain %s, wfID %v, first event [id:%v, version:%v], last event [id:%v, version:%v]",
		domainID, workflowExecution.WorkflowID, firstEvent.ID, firstEvent.Version, lastEvent.ID, lastEvent.Version)

	// need to clear the stickiness since workflow turned to passive
	b.mutableState.ClearStickyness()

	historyLength := len(history)
	for i, event := range history {
		b.logger.Debugf("stateBuilderImpl Applying event %v of %v. Calling UpdateCurrentVersion for domain %s, wfID %v, event [id:%v, version:%v]",
			i+1, historyLength, domainID, workflowExecution.WorkflowID, event.ID, event.Version)

		// NOTE: stateBuilder is also being used in the active side
		if err := b.mutableState.UpdateCurrentVersion(event.Version, true); err != nil {
			return nil, err
		}
		versionHistories := b.mutableState.GetVersionHistories()
		if versionHistories == nil {
			return nil, ErrMissingVersionHistories
		}
		versionHistory, err := versionHistories.GetCurrentVersionHistory()
		if err != nil {
			return nil, err
		}
		if err := versionHistory.AddOrUpdateItem(persistence.NewVersionHistoryItem(
			event.ID,
			event.Version,
		)); err != nil {
			return nil, err
		}
		b.mutableState.GetExecutionInfo().LastEventTaskID = event.TaskID

		switch event.GetEventType() {
		case types.EventTypeWorkflowExecutionStarted:
			attributes := event.WorkflowExecutionStartedEventAttributes
			var parentDomainID *string
			// If ParentWorkflowDomainID is present use it, otherwise fallback to ParentWorkflowDomain
			// as ParentWorkflowDomainID will not be present on older histories.
			if attributes.ParentWorkflowDomainID != nil {
				parentDomainID = attributes.ParentWorkflowDomainID
			} else if attributes.GetParentWorkflowDomain() != "" {
				parentDomainEntry, err := b.domainCache.GetDomain(
					attributes.GetParentWorkflowDomain(),
				)
				if err != nil {
					return nil, err
				}
				parentDomainID = &parentDomainEntry.GetInfo().ID
			}

			if err := b.mutableState.ReplicateWorkflowExecutionStartedEvent(
				parentDomainID,
				workflowExecution,
				requestID,
				event,
				true,
			); err != nil {
				return nil, err
			}

			if err := b.mutableState.SetHistoryTree(
				workflowExecution.GetRunID(),
			); err != nil {
				return nil, err
			}

		case types.EventTypeDecisionTaskScheduled:
			attributes := event.DecisionTaskScheduledEventAttributes
			// use event.GetTimestamp() as DecisionOriginalScheduledTimestamp, because the heartbeat is not happening here.
			_, err := b.mutableState.ReplicateDecisionTaskScheduledEvent(
				event.Version,
				event.ID,
				attributes.TaskList.GetName(),
				attributes.GetStartToCloseTimeoutSeconds(),
				attributes.GetAttempt(),
				event.GetTimestamp(),
				event.GetTimestamp(),
				false,
			)
			if err != nil {
				return nil, err
			}

		case types.EventTypeDecisionTaskStarted:
			attributes := event.DecisionTaskStartedEventAttributes
			_, err := b.mutableState.ReplicateDecisionTaskStartedEvent(
				nil,
				event.Version,
				attributes.GetScheduledEventID(),
				event.ID,
				attributes.GetRequestID(),
				event.GetTimestamp(),
			)
			if err != nil {
				return nil, err
			}

		case types.EventTypeDecisionTaskCompleted:
			if err := b.mutableState.ReplicateDecisionTaskCompletedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeDecisionTaskTimedOut:
			if err := b.mutableState.ReplicateDecisionTaskTimedOutEvent(
				event,
			); err != nil {
				return nil, err
			}

			// this is for transient decision
			err := b.mutableState.ReplicateTransientDecisionTaskScheduled()
			if err != nil {
				return nil, err
			}

		case types.EventTypeDecisionTaskFailed:
			if err := b.mutableState.ReplicateDecisionTaskFailedEvent(event); err != nil {
				return nil, err
			}

			// this is for transient decision
			err := b.mutableState.ReplicateTransientDecisionTaskScheduled()
			if err != nil {
				return nil, err
			}

		case types.EventTypeActivityTaskScheduled:
			if _, err := b.mutableState.ReplicateActivityTaskScheduledEvent(
				firstEvent.ID,
				event,
				false,
			); err != nil {
				return nil, err
			}

		case types.EventTypeActivityTaskStarted:
			if err := b.mutableState.ReplicateActivityTaskStartedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeActivityTaskCompleted:
			if err := b.mutableState.ReplicateActivityTaskCompletedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeActivityTaskFailed:
			if err := b.mutableState.ReplicateActivityTaskFailedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeActivityTaskTimedOut:
			if err := b.mutableState.ReplicateActivityTaskTimedOutEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeActivityTaskCancelRequested:
			if err := b.mutableState.ReplicateActivityTaskCancelRequestedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeActivityTaskCanceled:
			if err := b.mutableState.ReplicateActivityTaskCanceledEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeRequestCancelActivityTaskFailed:
			// No mutable state action is needed

		case types.EventTypeTimerStarted:
			if _, err := b.mutableState.ReplicateTimerStartedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeTimerFired:
			if err := b.mutableState.ReplicateTimerFiredEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeTimerCanceled:
			if err := b.mutableState.ReplicateTimerCanceledEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeCancelTimerFailed:
			// no mutable state action is needed

		case types.EventTypeStartChildWorkflowExecutionInitiated:
			if _, err := b.mutableState.ReplicateStartChildWorkflowExecutionInitiatedEvent(
				firstEvent.ID,
				event,
				// create a new request ID which is used by transfer queue processor
				// if domain is failed over at this point
				uuid.New(),
			); err != nil {
				return nil, err
			}

		case types.EventTypeStartChildWorkflowExecutionFailed:
			if err := b.mutableState.ReplicateStartChildWorkflowExecutionFailedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeChildWorkflowExecutionStarted:
			if err := b.mutableState.ReplicateChildWorkflowExecutionStartedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeChildWorkflowExecutionCompleted:
			if err := b.mutableState.ReplicateChildWorkflowExecutionCompletedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeChildWorkflowExecutionFailed:
			if err := b.mutableState.ReplicateChildWorkflowExecutionFailedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeChildWorkflowExecutionCanceled:
			if err := b.mutableState.ReplicateChildWorkflowExecutionCanceledEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeChildWorkflowExecutionTimedOut:
			if err := b.mutableState.ReplicateChildWorkflowExecutionTimedOutEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeChildWorkflowExecutionTerminated:
			if err := b.mutableState.ReplicateChildWorkflowExecutionTerminatedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeRequestCancelExternalWorkflowExecutionInitiated:
			if _, err := b.mutableState.ReplicateRequestCancelExternalWorkflowExecutionInitiatedEvent(
				firstEvent.ID,
				event,
				// create a new request ID which is used by transfer queue processor
				// if domain is failed over at this point
				uuid.New(),
			); err != nil {
				return nil, err
			}

		case types.EventTypeRequestCancelExternalWorkflowExecutionFailed:
			if err := b.mutableState.ReplicateRequestCancelExternalWorkflowExecutionFailedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeExternalWorkflowExecutionCancelRequested:
			if err := b.mutableState.ReplicateExternalWorkflowExecutionCancelRequested(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeSignalExternalWorkflowExecutionInitiated:
			// Create a new request ID which is used by transfer queue processor if domain is failed over at this point
			signalRequestID := uuid.New()
			if _, err := b.mutableState.ReplicateSignalExternalWorkflowExecutionInitiatedEvent(
				firstEvent.ID,
				event,
				signalRequestID,
			); err != nil {
				return nil, err
			}

		case types.EventTypeSignalExternalWorkflowExecutionFailed:
			if err := b.mutableState.ReplicateSignalExternalWorkflowExecutionFailedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeExternalWorkflowExecutionSignaled:
			if err := b.mutableState.ReplicateExternalWorkflowExecutionSignaled(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeMarkerRecorded:
			// No mutable state action is needed

		case types.EventTypeWorkflowExecutionSignaled:
			if err := b.mutableState.ReplicateWorkflowExecutionSignaled(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeWorkflowExecutionCancelRequested:
			if err := b.mutableState.ReplicateWorkflowExecutionCancelRequestedEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeUpsertWorkflowSearchAttributes:
			if err := b.mutableState.ReplicateUpsertWorkflowSearchAttributesEvent(
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeWorkflowExecutionCompleted:
			if err := b.mutableState.ReplicateWorkflowExecutionCompletedEvent(
				firstEvent.ID,
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeWorkflowExecutionFailed:
			if err := b.mutableState.ReplicateWorkflowExecutionFailedEvent(
				firstEvent.ID,
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeWorkflowExecutionTimedOut:
			if err := b.mutableState.ReplicateWorkflowExecutionTimedoutEvent(
				firstEvent.ID,
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeWorkflowExecutionCanceled:
			if err := b.mutableState.ReplicateWorkflowExecutionCanceledEvent(
				firstEvent.ID,
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeWorkflowExecutionTerminated:
			if err := b.mutableState.ReplicateWorkflowExecutionTerminatedEvent(
				firstEvent.ID,
				event,
			); err != nil {
				return nil, err
			}

		case types.EventTypeWorkflowExecutionContinuedAsNew:

			// The length of newRunHistory can be zero in resend case
			if len(newRunHistory) != 0 {
				newRunMutableStateBuilder = NewMutableStateBuilderWithVersionHistories(
					b.shard,
					b.logger,
					b.mutableState.GetDomainEntry(),
				)
				newRunStateBuilder := NewStateBuilder(b.shard, b.logger, newRunMutableStateBuilder)
				newRunID := event.WorkflowExecutionContinuedAsNewEventAttributes.GetNewExecutionRunID()
				newExecution := types.WorkflowExecution{
					WorkflowID: workflowExecution.WorkflowID,
					RunID:      newRunID,
				}
				_, err := newRunStateBuilder.ApplyEvents(
					domainID,
					uuid.New(),
					newExecution,
					newRunHistory,
					nil,
				)
				if err != nil {
					return nil, err
				}
			}

			err := b.mutableState.ReplicateWorkflowExecutionContinuedAsNewEvent(
				firstEvent.ID,
				domainID,
				event,
			)
			if err != nil {
				return nil, err
			}

		default:
			return nil, &types.BadRequestError{Message: "Unknown event type"}
		}

		b.logger.Debugf("Applied event %v of %v for domain %s, wfID %v, event [id:%v, version:%v]",
			i+1, historyLength, domainID, workflowExecution.WorkflowID, event.ID, event.Version)
	}

	b.mutableState.GetExecutionInfo().SetLastFirstEventID(firstEvent.ID)
	b.mutableState.GetExecutionInfo().SetNextEventID(lastEvent.ID + 1)

	b.mutableState.SetHistoryBuilder(NewHistoryBuilderFromEvents(history, b.mutableState))

	return newRunMutableStateBuilder, nil
}

func (b *stateBuilderImpl) GetMutableState() MutableState {

	return b.mutableState
}

func (b *stateBuilderImpl) unixNanoToTime(
	unixNano int64,
) time.Time {

	return time.Unix(0, unixNano)
}
