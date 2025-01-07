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

package execution

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/shard"
)

var (
	currentBranchToken = []byte("branchToken")
	testShardID        = 42
)

func TestGetChildExecutionInfo(t *testing.T) {
	childInfo := &persistence.ChildExecutionInfo{
		DomainID: constants.TestDomainID,
	}
	ctx := createShardCtx(t)
	m := loadMutableState(t, ctx, &persistence.WorkflowMutableState{
		ExecutionInfo: standardExecutionInfo(),
		ChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{
			1: childInfo,
		}})

	result, ok := m.GetChildExecutionInfo(1)

	assert.True(t, ok)
	assert.Equal(t, result, childInfo)
}

func TestGetChildExecutionInitiatedEvent(t *testing.T) {
	initiatedEventID := int64(1)
	sampleEvent := &types.HistoryEvent{
		ID: 2,
	}
	cases := []struct {
		name                 string
		eventCacheAffordance func(e *events.MockCache)
		childInfo            *persistence.ChildExecutionInfo
		expected             *types.HistoryEvent
		expectedErr          error
	}{
		{
			name:        "Missing Child Workflow Info",
			expectedErr: ErrMissingChildWorkflowInfo,
		},
		{
			name: "Event stored on info",
			childInfo: &persistence.ChildExecutionInfo{
				InitiatedEvent: sampleEvent,
			},
			expected: sampleEvent,
		},
		{
			name: "Get from EventCache",
			childInfo: &persistence.ChildExecutionInfo{
				InitiatedEventBatchID: 10,
				InitiatedID:           11,
			},
			eventCacheAffordance: func(e *events.MockCache) {
				e.EXPECT().GetEvent(
					gomock.Any(), // context
					testShardID,
					constants.TestDomainID,
					constants.TestWorkflowID,
					constants.TestRunID,
					int64(10), // InitiatedEventBatchID
					int64(11), // InitiatedID
					currentBranchToken,
				).Return(sampleEvent, nil)
			},
			expected: sampleEvent,
		},
		{
			name: "Error from EventCache",
			childInfo: &persistence.ChildExecutionInfo{
				InitiatedEventBatchID: 10,
				InitiatedID:           11,
			},
			eventCacheAffordance: func(e *events.MockCache) {
				e.EXPECT().GetEvent(
					gomock.Any(), // context
					testShardID,
					constants.TestDomainID,
					constants.TestWorkflowID,
					constants.TestRunID,
					int64(10), // InitiatedEventBatchID
					int64(11), // InitiatedID
					currentBranchToken,
				).Return(nil, &types.AccessDeniedError{Message: "Oh no!"})
			},
			expectedErr: ErrMissingChildWorkflowInitiatedEvent,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := createShardCtx(t)
			if testCase.eventCacheAffordance != nil {
				testCase.eventCacheAffordance(ctx.MockEventsCache)
			}
			childExecutionInfo := map[int64]*persistence.ChildExecutionInfo{}
			if testCase.childInfo != nil {
				childExecutionInfo[initiatedEventID] = testCase.childInfo
			}
			m := loadMutableState(t, ctx, &persistence.WorkflowMutableState{
				ExecutionInfo:       standardExecutionInfo(),
				ChildExecutionInfos: childExecutionInfo,
			})

			result, err := m.GetChildExecutionInitiatedEvent(context.Background(), initiatedEventID)
			assert.Equal(t, testCase.expected, result)
			assert.Equal(t, testCase.expectedErr, err)
		})
	}
}

func TestGetPendingChildExecutionInfos(t *testing.T) {
	childExecutionInfos := map[int64]*persistence.ChildExecutionInfo{
		1: {
			DomainID: constants.TestDomainID,
		},
	}
	ctx := createShardCtx(t)

	m := loadMutableState(t, ctx, &persistence.WorkflowMutableState{
		ExecutionInfo:       standardExecutionInfo(),
		ChildExecutionInfos: childExecutionInfos,
	})

	assert.Equal(t, childExecutionInfos, m.GetPendingChildExecutionInfos())
}

func TestDeletePendingChildExecution(t *testing.T) {
	cases := []struct {
		name      string
		childInfo map[int64]*persistence.ChildExecutionInfo
		toDelete  int64
	}{
		{
			name: "DeletePresent",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				1: {
					DomainID: constants.TestDomainID,
				},
			},
			toDelete: 1,
		},
		{
			name: "DeleteNotFound",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				1: {
					DomainID: constants.TestDomainID,
				},
			},
			toDelete: 2,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := createShardCtx(t)
			m := loadMutableState(t, ctx, &persistence.WorkflowMutableState{
				ExecutionInfo:       standardExecutionInfo(),
				ChildExecutionInfos: testCase.childInfo,
			})

			err := m.DeletePendingChildExecution(testCase.toDelete)

			// Even the failure case doesn't return an error
			assert.NoError(t, err)
			_, hasChild := m.GetPendingChildExecutionInfos()[testCase.toDelete]
			_, deletedChild := m.deleteChildExecutionInfos[testCase.toDelete]

			assert.Equal(t, false, hasChild)
			assert.Equal(t, true, deletedChild)
		})
	}
}

func TestAddChildEvents(t *testing.T) {
	parentWfID := "parent"
	parentRunID := "1d00698f-08e1-4d36-a3e2-3bf109f5d2d6"
	decisionID := int64(0)
	requestID := "request"
	decisionAttributes := &types.StartChildWorkflowExecutionDecisionAttributes{
		Domain:                              constants.TestDomainName,
		WorkflowID:                          constants.TestWorkflowID,
		WorkflowType:                        &types.WorkflowType{Name: "type"},
		TaskList:                            &types.TaskList{},
		Input:                               []byte("input"),
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
		ParentClosePolicy:                   types.ParentClosePolicyAbandon.Ptr(),
		Control:                             []byte("control"),
		WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
		RetryPolicy:                         nil,
		CronSchedule:                        "",
		Header:                              &types.Header{Fields: map[string][]byte{"headerKey": []byte("headerValue")}},
		Memo:                                &types.Memo{Fields: map[string][]byte{"key": []byte("value")}},
		SearchAttributes:                    &types.SearchAttributes{IndexedFields: map[string][]byte{"indexed": []byte("field")}},
	}
	initiatedAttributes := &types.StartChildWorkflowExecutionInitiatedEventAttributes{
		Domain:                              decisionAttributes.Domain,
		WorkflowID:                          decisionAttributes.WorkflowID,
		WorkflowType:                        decisionAttributes.WorkflowType,
		TaskList:                            decisionAttributes.TaskList,
		Input:                               decisionAttributes.Input,
		ExecutionStartToCloseTimeoutSeconds: decisionAttributes.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      decisionAttributes.TaskStartToCloseTimeoutSeconds,
		ParentClosePolicy:                   decisionAttributes.ParentClosePolicy,
		Control:                             decisionAttributes.Control,
		DecisionTaskCompletedEventID:        decisionID,
		WorkflowIDReusePolicy:               decisionAttributes.WorkflowIDReusePolicy,
		RetryPolicy:                         decisionAttributes.RetryPolicy,
		CronSchedule:                        decisionAttributes.CronSchedule,
		Header:                              decisionAttributes.Header,
		Memo:                                decisionAttributes.Memo,
		SearchAttributes:                    decisionAttributes.SearchAttributes,
	}
	childInitiatedID := int64(1)
	childStartedID := int64(3)
	nextEventID := int64(2)
	currentTimestamp := int64(1_000_001)
	childExecution := &types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	childType := &types.WorkflowType{Name: "type"}

	cases := []struct {
		name           string
		childInfo      map[int64]*persistence.ChildExecutionInfo
		wfState        int
		mockAllowances func(ctx *shard.TestContext)
		eventFn        func(builder *mutableStateBuilder) (*types.HistoryEvent, error)
		assertFn       func(t *testing.T, builder *mutableStateBuilder)
		expectedEvent  *types.HistoryEvent
		expectedErr    error
	}{
		{
			name: "StartChildWorkflowExecutionInitiated",
			mockAllowances: func(ctx *shard.TestContext) {
				ctx.MockEventsCache.EXPECT().PutEvent(constants.TestDomainID, parentWfID, parentRunID, nextEventID, gomock.Any())
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				event, _, e := builder.AddStartChildWorkflowExecutionInitiatedEvent(decisionID, requestID, decisionAttributes)
				return event, e
			},
			expectedEvent: &types.HistoryEvent{
				ID:        nextEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeStartChildWorkflowExecutionInitiated.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				StartChildWorkflowExecutionInitiatedEventAttributes: initiatedAttributes,
			},
		},
		{
			name:    "StartChildWorkflowExecutionInitiated: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				event, _, e := builder.AddStartChildWorkflowExecutionInitiatedEvent(decisionID, requestID, decisionAttributes)
				return event, e
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name: "ChildWorkflowExecutionStarted",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				childInitiatedID: {
					StartedID: common.EmptyEventID,
				},
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionStartedEvent(constants.TestDomainID, &types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}, &types.WorkflowType{Name: "type"}, childInitiatedID, nil)
			},
			expectedEvent: &types.HistoryEvent{
				ID:        common.BufferedEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
					Domain:            constants.TestDomainID,
					InitiatedEventID:  childInitiatedID,
					WorkflowExecution: childExecution,
					WorkflowType:      childType,
					Header:            nil,
				},
			},
		},
		{
			name:    "ChildWorkflowExecutionStarted: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionStartedEvent(constants.TestDomainID, &types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}, &types.WorkflowType{Name: "type"}, childInitiatedID, nil)
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name: "ChildWorkflowExecutionStarted: missing child info",
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionStartedEvent(constants.TestDomainID, childExecution, childType, childInitiatedID, nil)
			},
			expectedErr: &types.InternalServiceError{Message: tag.WorkflowActionChildWorkflowStarted.Field().String + " operation failed"},
		},
		{
			name: "StartChildWorkflowExecutionFailed",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				childInitiatedID: {
					StartedID: common.EmptyEventID,
				},
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddStartChildWorkflowExecutionFailedEvent(childInitiatedID, types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning, initiatedAttributes)
			},
			expectedEvent: &types.HistoryEvent{
				ID:        common.BufferedEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeStartChildWorkflowExecutionFailed.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				StartChildWorkflowExecutionFailedEventAttributes: &types.StartChildWorkflowExecutionFailedEventAttributes{
					Domain:                       constants.TestDomainName,
					WorkflowID:                   constants.TestWorkflowID,
					WorkflowType:                 childType,
					Cause:                        types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning.Ptr(),
					Control:                      []byte("control"),
					InitiatedEventID:             childInitiatedID,
					DecisionTaskCompletedEventID: decisionID,
				},
			},
		},
		{
			name:    "StartChildWorkflowExecutionFailed: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddStartChildWorkflowExecutionFailedEvent(childInitiatedID, types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning, initiatedAttributes)
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name: "StartChildWorkflowExecutionFailed: missing child info",
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddStartChildWorkflowExecutionFailedEvent(childInitiatedID, types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning, initiatedAttributes)
			},
			expectedErr: &types.InternalServiceError{Message: tag.WorkflowActionChildWorkflowInitiationFailed.Field().String + " operation failed"},
		},
		{
			name: "ChildWorkflowExecutionCompleted",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				childInitiatedID: {
					InitiatedID:      childInitiatedID,
					StartedID:        childStartedID,
					DomainID:         constants.TestDomainID,
					WorkflowTypeName: childType.Name,
				},
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionCompletedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionCompletedEventAttributes{
					Result:                       []byte("result"),
					DecisionTaskCompletedEventID: 10, // Unused
				})
			},
			expectedEvent: &types.HistoryEvent{
				ID:        common.BufferedEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeChildWorkflowExecutionCompleted.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				ChildWorkflowExecutionCompletedEventAttributes: &types.ChildWorkflowExecutionCompletedEventAttributes{
					Result:            []byte("result"),
					Domain:            constants.TestDomainName,
					WorkflowExecution: childExecution,
					WorkflowType:      childType,
					InitiatedEventID:  childInitiatedID,
					StartedEventID:    3,
				},
			},
		},
		{
			name:    "ChildWorkflowExecutionCompleted: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionCompletedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionCompletedEventAttributes{
					Result:                       []byte("result"),
					DecisionTaskCompletedEventID: 10, // Unused
				})
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name:      "ChildWorkflowExecutionCompleted: missing child info",
			childInfo: map[int64]*persistence.ChildExecutionInfo{},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionCompletedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionCompletedEventAttributes{
					Result:                       []byte("result"),
					DecisionTaskCompletedEventID: 10, // Unused
				})
			},
			expectedErr: &types.InternalServiceError{Message: tag.WorkflowActionChildWorkflowCompleted.Field().String + " operation failed"},
		},
		{
			name: "ChildWorkflowExecutionFailed",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				childInitiatedID: {
					InitiatedID:      childInitiatedID,
					StartedID:        childStartedID,
					DomainID:         constants.TestDomainID,
					WorkflowTypeName: childType.Name,
				},
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionFailedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionFailedEventAttributes{
					Reason:                       common.StringPtr("failed"),
					DecisionTaskCompletedEventID: 10, // Unused
					Details:                      []byte("details"),
				})
			},
			expectedEvent: &types.HistoryEvent{
				ID:        common.BufferedEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeChildWorkflowExecutionFailed.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				ChildWorkflowExecutionFailedEventAttributes: &types.ChildWorkflowExecutionFailedEventAttributes{
					Reason:            common.StringPtr("failed"),
					Details:           []byte("details"),
					Domain:            constants.TestDomainName,
					WorkflowExecution: childExecution,
					WorkflowType:      childType,
					InitiatedEventID:  childInitiatedID,
					StartedEventID:    childStartedID,
				},
			},
		},
		{
			name:    "ChildWorkflowExecutionFailed: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionFailedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionFailedEventAttributes{
					Reason:                       common.StringPtr("failed"),
					DecisionTaskCompletedEventID: 10, // Unused
					Details:                      []byte("details"),
				})
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name: "ChildWorkflowExecutionFailed: missing child info",
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionFailedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionFailedEventAttributes{
					Reason:                       common.StringPtr("failed"),
					DecisionTaskCompletedEventID: 10, // Unused
					Details:                      []byte("details"),
				})
			},
			expectedErr: &types.InternalServiceError{Message: tag.WorkflowActionChildWorkflowFailed.Field().String + " operation failed"},
		},
		{
			name: "ChildWorkflowExecutionCanceled",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				childInitiatedID: {
					InitiatedID:      childInitiatedID,
					StartedID:        childStartedID,
					DomainID:         constants.TestDomainID,
					WorkflowTypeName: childType.Name,
				},
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionCanceledEvent(childInitiatedID, childExecution, &types.WorkflowExecutionCanceledEventAttributes{
					Details:                      []byte("details"),
					DecisionTaskCompletedEventID: 10, // Unused
				})
			},
			expectedEvent: &types.HistoryEvent{
				ID:        common.BufferedEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeChildWorkflowExecutionCanceled.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				ChildWorkflowExecutionCanceledEventAttributes: &types.ChildWorkflowExecutionCanceledEventAttributes{
					Details:           []byte("details"),
					Domain:            constants.TestDomainName,
					WorkflowExecution: childExecution,
					WorkflowType:      childType,
					InitiatedEventID:  childInitiatedID,
					StartedEventID:    childStartedID,
				},
			},
		},
		{
			name:    "ChildWorkflowExecutionCanceled: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionCanceledEvent(childInitiatedID, childExecution, &types.WorkflowExecutionCanceledEventAttributes{
					Details:                      []byte("details"),
					DecisionTaskCompletedEventID: 10, // Unused
				})
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name: "ChildWorkflowExecutionCanceled: missing child info",
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionCanceledEvent(childInitiatedID, childExecution, &types.WorkflowExecutionCanceledEventAttributes{
					Details:                      []byte("details"),
					DecisionTaskCompletedEventID: 10, // Unused
				})
			},
			expectedErr: &types.InternalServiceError{Message: tag.WorkflowActionChildWorkflowCanceled.Field().String + " operation failed"},
		},
		{
			name: "ChildWorkflowExecutionTerminated",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				childInitiatedID: {
					InitiatedID:      childInitiatedID,
					StartedID:        childStartedID,
					DomainID:         constants.TestDomainID,
					WorkflowTypeName: childType.Name,
				},
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				// Attributes are entirely unused
				return builder.AddChildWorkflowExecutionTerminatedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionTerminatedEventAttributes{
					Reason:   "reason",
					Details:  []byte("details"),
					Identity: "identity",
				})
			},
			expectedEvent: &types.HistoryEvent{
				ID:        common.BufferedEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeChildWorkflowExecutionTerminated.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				ChildWorkflowExecutionTerminatedEventAttributes: &types.ChildWorkflowExecutionTerminatedEventAttributes{
					Domain:            constants.TestDomainName,
					WorkflowExecution: childExecution,
					WorkflowType:      childType,
					InitiatedEventID:  childInitiatedID,
					StartedEventID:    childStartedID,
				},
			},
		},
		{
			name:    "ChildWorkflowExecutionTerminated: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				// Attributes are entirely unused
				return builder.AddChildWorkflowExecutionTerminatedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionTerminatedEventAttributes{
					Reason:   "reason",
					Details:  []byte("details"),
					Identity: "identity",
				})
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name: "ChildWorkflowExecutionTerminated: missing child info",
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				// Attributes are entirely unused
				return builder.AddChildWorkflowExecutionTerminatedEvent(childInitiatedID, childExecution, &types.WorkflowExecutionTerminatedEventAttributes{
					Reason:   "reason",
					Details:  []byte("details"),
					Identity: "identity",
				})
			},
			expectedErr: &types.InternalServiceError{Message: tag.WorkflowActionChildWorkflowTerminated.Field().String + " operation failed"},
		},
		{
			name: "ChildWorkflowExecutionTimedOut",
			childInfo: map[int64]*persistence.ChildExecutionInfo{
				childInitiatedID: {
					InitiatedID:      childInitiatedID,
					StartedID:        childStartedID,
					DomainID:         constants.TestDomainID,
					WorkflowTypeName: childType.Name,
				},
			},
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionTimedOutEvent(childInitiatedID, childExecution, &types.WorkflowExecutionTimedOutEventAttributes{
					TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
				})
			},
			expectedEvent: &types.HistoryEvent{
				ID:        common.BufferedEventID,
				Timestamp: &currentTimestamp,
				EventType: types.EventTypeChildWorkflowExecutionTimedOut.Ptr(),
				Version:   common.EmptyVersion,
				TaskID:    common.EmptyEventTaskID,
				ChildWorkflowExecutionTimedOutEventAttributes: &types.ChildWorkflowExecutionTimedOutEventAttributes{
					Domain:            constants.TestDomainName,
					WorkflowExecution: childExecution,
					WorkflowType:      childType,
					InitiatedEventID:  childInitiatedID,
					StartedEventID:    childStartedID,
					TimeoutType:       types.TimeoutTypeStartToClose.Ptr(),
				},
			},
		},
		{
			name:    "ChildWorkflowExecutionTimedOut: not mutable",
			wfState: persistence.WorkflowStateCompleted,
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionTimedOutEvent(childInitiatedID, childExecution, &types.WorkflowExecutionTimedOutEventAttributes{
					TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
				})
			},
			expectedErr: ErrWorkflowFinished,
		},
		{
			name: "ChildWorkflowExecutionTimedOut: missing child info",
			eventFn: func(builder *mutableStateBuilder) (*types.HistoryEvent, error) {
				return builder.AddChildWorkflowExecutionTimedOutEvent(childInitiatedID, childExecution, &types.WorkflowExecutionTimedOutEventAttributes{
					TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
				})
			},
			expectedErr: &types.InternalServiceError{Message: tag.WorkflowActionChildWorkflowTimedOut.Field().String + " operation failed"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := createShardCtx(t)
			ctx.Resource.TimeSource = clock.NewMockedTimeSourceAt(time.Unix(0, currentTimestamp))
			if testCase.mockAllowances != nil {
				testCase.mockAllowances(ctx)
			}

			childExecutionInfo := testCase.childInfo
			if childExecutionInfo == nil {
				childExecutionInfo = map[int64]*persistence.ChildExecutionInfo{}
			}

			m := loadMutableState(t, ctx, &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:         constants.TestDomainID,
					WorkflowID:       parentWfID,
					RunID:            parentRunID,
					WorkflowTypeName: "type",
					BranchToken:      currentBranchToken,
					NextEventID:      nextEventID,
					State:            testCase.wfState,
				},
				ChildExecutionInfos: childExecutionInfo,
			})

			event, err := testCase.eventFn(m)

			assert.Equal(t, testCase.expectedEvent, event)
			assert.Equal(t, testCase.expectedErr, err)
		})
	}
}

// MutableState is all about mutating, avoid reusing objects
func standardExecutionInfo() *persistence.WorkflowExecutionInfo {
	return &persistence.WorkflowExecutionInfo{
		DomainID:         constants.TestDomainID,
		WorkflowID:       constants.TestWorkflowID,
		RunID:            constants.TestRunID,
		WorkflowTypeName: "type",
		BranchToken:      currentBranchToken,
	}
}

func createShardCtx(t *testing.T) *shard.TestContext {
	return shard.NewTestContext(
		t,
		gomock.NewController(t),
		&persistence.ShardInfo{
			ShardID:          testShardID,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest(),
	)
}

func loadMutableState(t *testing.T, ctx *shard.TestContext, state *persistence.WorkflowMutableState) *mutableStateBuilder {
	domain := cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: constants.TestDomainID, Name: constants.TestDomainName},
		&persistence.DomainConfig{},
		true,
		&persistence.DomainReplicationConfig{},
		1,
		nil,
		0,
		0,
		0,
	)
	ctx.Resource.DomainCache.EXPECT().GetDomainID(constants.TestDomainName).AnyTimes()
	ctx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domain, nil).AnyTimes()
	ctx.Resource.DomainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()
	m := newMutableStateBuilder(ctx,
		log.NewNoop(),
		domain)
	err := m.Load(state)
	assert.NoError(t, err)
	return m
}
