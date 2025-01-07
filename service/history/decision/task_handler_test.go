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

package decision

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/testdata"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

const (
	testTaskCompletedID = int64(123)
)

func TestHandleDecisionRequestCancelExternalWorkflow(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl)
		attributes      *types.RequestCancelExternalWorkflowExecutionDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, err error)
	}{
		{
			name: "success",
			attributes: &types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
				Domain:            constants.TestDomainName,
				WorkflowID:        constants.TestWorkflowID,
				RunID:             constants.TestRunID,
				Control:           nil,
				ChildWorkflowOnly: false,
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl) {
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)

			},
		},
		{
			name: "internal service error",
			attributes: &types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
				Domain:            constants.TestDomainName,
				WorkflowID:        constants.TestWorkflowID,
				RunID:             constants.TestRunID,
				Control:           nil,
				ChildWorkflowOnly: false,
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl) {
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(nil, errors.New("some error getting domain cache"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "Unable to cancel workflow across domain: some random domain name."}, err)
			},
		},
		{
			name: "attributes validation failure",
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.True(t, taskHandler.failDecision)
				assert.NotNil(t, taskHandler.failMessage)
				assert.Equal(t, func(i int32) *types.DecisionTaskFailedCause {
					cause := new(types.DecisionTaskFailedCause)
					*cause = types.DecisionTaskFailedCause(i)
					return cause
					// 9 is types.DecisionTaskFailedCause "BAD_REQUEST_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_ATTRIBUTES"
				}(9), taskHandler.failDecisionCause)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(1).Return(&persistence.WorkflowExecutionInfo{
				DomainID:   constants.TestDomainID,
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			})
			taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddRequestCancelExternalWorkflowExecutionInitiatedEvent(testTaskCompletedID, gomock.Any(), test.attributes).AnyTimes()
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeRequestCancelExternalWorkflowExecution),
				RequestCancelExternalWorkflowExecutionDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, err)
		})
	}
}

func TestHandleDecisionRequestCancelActivity(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl)
		attributes      *types.RequestCancelActivityTaskDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, err error)
	}{
		{
			name:       "success",
			attributes: &types.RequestCancelActivityTaskDecisionAttributes{ActivityID: testdata.ActivityID},
			expectMockCalls: func(taskHandler *taskHandlerImpl) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskCancelRequestedEvent(
					testTaskCompletedID,
					testdata.ActivityID,
					testdata.Identity,
				).Times(1).Return(&types.HistoryEvent{}, &persistence.ActivityInfo{StartedID: common.EmptyEventID}, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskCanceledEvent(int64(0), int64(-23), int64(0), []byte(activityCancellationMsgActivityNotStarted), testdata.Identity).Return(nil, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)

			},
		},
		{
			name:       "AddActivityTaskCanceledEvent failure",
			attributes: &types.RequestCancelActivityTaskDecisionAttributes{ActivityID: testdata.ActivityID},
			expectMockCalls: func(taskHandler *taskHandlerImpl) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskCancelRequestedEvent(
					testTaskCompletedID,
					testdata.ActivityID,
					testdata.Identity,
				).Times(1).Return(&types.HistoryEvent{}, &persistence.ActivityInfo{StartedID: common.EmptyEventID}, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskCanceledEvent(int64(0), int64(-23), int64(0), []byte(activityCancellationMsgActivityNotStarted), testdata.Identity).Return(nil, errors.New("some random error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.Equal(t, errors.New("some random error"), err)
			},
		},
		{
			name:       "AddRequestCancelActivityTaskFailedEvent failure",
			attributes: &types.RequestCancelActivityTaskDecisionAttributes{ActivityID: testdata.ActivityID},
			expectMockCalls: func(taskHandler *taskHandlerImpl) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskCancelRequestedEvent(
					testTaskCompletedID,
					testdata.ActivityID,
					testdata.Identity,
				).Times(1).Return(&types.HistoryEvent{}, &persistence.ActivityInfo{StartedID: common.EmptyEventID}, &types.BadRequestError{Message: "some types.BadRequestError error"})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddRequestCancelActivityTaskFailedEvent(testTaskCompletedID, testdata.ActivityID, activityCancellationMsgActivityIDUnknown).Return(nil, errors.New("some random error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.Equal(t, errors.New("some random error"), err)
			},
		},
		{
			name:       "default switch case error",
			attributes: &types.RequestCancelActivityTaskDecisionAttributes{ActivityID: testdata.ActivityID},
			expectMockCalls: func(taskHandler *taskHandlerImpl) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskCancelRequestedEvent(
					testTaskCompletedID,
					testdata.ActivityID,
					testdata.Identity,
				).Times(1).Return(&types.HistoryEvent{}, &persistence.ActivityInfo{StartedID: common.EmptyEventID}, errors.New("some default error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.Equal(t, errors.New("some default error"), err)
			},
		},
		{
			name: "attributes validation failure",
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, err error) {
				assert.True(t, taskHandler.failDecision)
				assert.NotNil(t, taskHandler.failMessage)
				assert.Equal(t, func(i int32) *types.DecisionTaskFailedCause {
					cause := new(types.DecisionTaskFailedCause)
					*cause = types.DecisionTaskFailedCause(i)
					return cause
					// 2 is types.DecisionTaskFailedCause "BAD_REQUEST_CANCEL_ACTIVITY_ATTRIBUTES"
				}(2), taskHandler.failDecisionCause)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddRequestCancelExternalWorkflowExecutionInitiatedEvent(testTaskCompletedID, gomock.Any(), test.attributes).AnyTimes()
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeRequestCancelActivityTask),
				RequestCancelActivityTaskDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, err)
		})
	}
}

func TestHandleDecisionStartChildWorkflow(t *testing.T) {

	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes)
		attributes      *types.StartChildWorkflowExecutionDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes, err error)
	}{
		{
			name: "success - ParentClosePolicy enabled",
			attributes: &types.StartChildWorkflowExecutionDecisionAttributes{
				Domain:       constants.TestDomainName,
				WorkflowID:   constants.TestWorkflowID,
				WorkflowType: &types.WorkflowType{Name: testdata.WorkflowTypeName},
				TaskList:     &types.TaskList{Name: testdata.TaskListName},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddStartChildWorkflowExecutionInitiatedEvent(testTaskCompletedID, gomock.Any(), attr).Times(1)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.ParentClosePolicyTerminate.Ptr(), attr.ParentClosePolicy)
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)

			},
		},
		{
			name: "success - ParentClosePolicy disabled",
			attributes: &types.StartChildWorkflowExecutionDecisionAttributes{
				Domain:       constants.TestDomainName,
				WorkflowID:   constants.TestWorkflowID,
				WorkflowType: &types.WorkflowType{Name: testdata.WorkflowTypeName},
				TaskList:     &types.TaskList{Name: testdata.TaskListName},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes) {
				taskHandler.config.EnableParentClosePolicy = func(domain string) bool { return false }
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddStartChildWorkflowExecutionInitiatedEvent(testTaskCompletedID, gomock.Any(), attr).Times(1)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.ParentClosePolicyAbandon.Ptr(), attr.ParentClosePolicy)
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)

			},
		},
		{
			name: "success - ParentClosePolicy non nil",
			attributes: &types.StartChildWorkflowExecutionDecisionAttributes{
				Domain:            constants.TestDomainName,
				WorkflowID:        constants.TestWorkflowID,
				WorkflowType:      &types.WorkflowType{Name: testdata.WorkflowTypeName},
				TaskList:          &types.TaskList{Name: testdata.TaskListName},
				ParentClosePolicy: new(types.ParentClosePolicy),
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes) {
				taskHandler.config.EnableParentClosePolicy = func(domain string) bool { return false }
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddStartChildWorkflowExecutionInitiatedEvent(testTaskCompletedID, gomock.Any(), attr).Times(1)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)

			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.ParentClosePolicyAbandon.Ptr(), attr.ParentClosePolicy)
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)

			},
		},
		{
			name: "internal service error",
			attributes: &types.StartChildWorkflowExecutionDecisionAttributes{
				Domain:       constants.TestDomainName,
				WorkflowID:   constants.TestWorkflowID,
				WorkflowType: &types.WorkflowType{Name: testdata.WorkflowTypeName},
				TaskList:     &types.TaskList{Name: testdata.TaskListName},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes) {
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(nil, errors.New("some error getting domain cache"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "Unable to schedule child execution across domain some random domain name."}, err)
			},
		},
		{
			name: "attributes validation failure",
			attributes: &types.StartChildWorkflowExecutionDecisionAttributes{
				Domain:       constants.TestDomainName,
				WorkflowID:   constants.TestWorkflowID,
				WorkflowType: &types.WorkflowType{Name: testdata.WorkflowTypeName},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes) {
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes, err error) {
				assert.True(t, taskHandler.failDecision)
				assert.Equal(t, "missing task list name", *taskHandler.failMessage)
				assert.Equal(t, func(i int32) *types.DecisionTaskFailedCause {
					cause := new(types.DecisionTaskFailedCause)
					*cause = types.DecisionTaskFailedCause(i)
					return cause
					// 15 is types.DecisionTaskFailedCause "BAD_START_CHILD_EXECUTION_ATTRIBUTES"
				}(15), taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)
			},
		},
		{
			name: "size limit checker failure",
			attributes: &types.StartChildWorkflowExecutionDecisionAttributes{
				Domain:       constants.TestDomainName,
				WorkflowID:   constants.TestWorkflowID,
				WorkflowType: &types.WorkflowType{Name: testdata.WorkflowTypeName},
				TaskList:     &types.TaskList{Name: testdata.TaskListName},
				Input:        []byte("input"),
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes) {
				taskHandler.sizeLimitChecker.blobSizeLimitError = 1
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 2
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(testTaskCompletedID, &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
					Details: []byte("StartChildWorkflowExecutionDecisionAttributes.Input exceeds size limit."),
				}).Return(nil, errors.New("some error adding fail workflow event"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartChildWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, errors.New("some error adding fail workflow event"), err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().AnyTimes().Return(&persistence.WorkflowExecutionInfo{
				DomainID:   constants.TestDomainID,
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			})
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeStartChildWorkflowExecution),
				StartChildWorkflowExecutionDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionCancelTimer(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes)
		attributes      *types.CancelTimerDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes, err error)
	}{
		{
			name:       "success",
			attributes: &types.CancelTimerDecisionAttributes{TimerID: "test-timer-id"},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddTimerCanceledEvent(testTaskCompletedID, attr, taskHandler.identity)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().HasBufferedEvents()
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)
			},
		},
		{
			name:       "bad request error",
			attributes: &types.CancelTimerDecisionAttributes{TimerID: "test-timer-id"},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddTimerCanceledEvent(testTaskCompletedID, attr, taskHandler.identity).Return(nil, &types.BadRequestError{"some bad request error"})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddCancelTimerFailedEvent(testTaskCompletedID, attr, taskHandler.identity)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)
			},
		},
		{
			name:       "default error",
			attributes: &types.CancelTimerDecisionAttributes{TimerID: "test-timer-id"},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddTimerCanceledEvent(testTaskCompletedID, attr, taskHandler.identity).Return(nil, errors.New("some random error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, errors.New("some random error"), err)
			},
		},
		{
			name:       "attributes validation error",
			attributes: &types.CancelTimerDecisionAttributes{},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelTimerDecisionAttributes, err error) {
				assert.True(t, taskHandler.failDecision)
				assert.Equal(t, "TimerId is not set on decision.", *taskHandler.failMessage)
				assert.Equal(t, func(i int32) *types.DecisionTaskFailedCause {
					cause := new(types.DecisionTaskFailedCause)
					*cause = types.DecisionTaskFailedCause(i)
					return cause
				}(4), taskHandler.failDecisionCause)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType:                  common.Ptr(types.DecisionTypeCancelTimer),
				CancelTimerDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionSignalExternalWorkflow(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes)
		attributes      *types.SignalExternalWorkflowExecutionDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes, err error)
	}{
		{
			name: "success",
			attributes: &types.SignalExternalWorkflowExecutionDecisionAttributes{
				Domain:     constants.TestDomainName,
				Execution:  &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
				SignalName: testdata.SignalName,
				Input:      []byte("some input data"),
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(2).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddSignalExternalWorkflowExecutionInitiatedEvent(testTaskCompletedID, gomock.Any(), attr)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)
			},
		},
		{
			name: "internal service error",
			attributes: &types.SignalExternalWorkflowExecutionDecisionAttributes{
				Domain:     constants.TestDomainName,
				Execution:  &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
				SignalName: testdata.SignalName,
				Input:      []byte("some input data"),
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(1).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(nil, errors.New("some error getting domain cache"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "Unable to signal workflow across domain: some random domain name."}, err)
			},
		},
		{
			name: "attributes validation failure",
			attributes: &types.SignalExternalWorkflowExecutionDecisionAttributes{
				Domain:    constants.TestDomainName,
				Execution: &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
				Input:     []byte("some input data"),
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(1).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes, err error) {
				assert.True(t, taskHandler.failDecision)
				assert.Equal(t, "SignalName is not set on decision.", *taskHandler.failMessage)
				assert.Equal(t, func(i int32) *types.DecisionTaskFailedCause {
					cause := new(types.DecisionTaskFailedCause)
					*cause = types.DecisionTaskFailedCause(i)
					return cause
					// 14 is types.DecisionTaskFailedCause "BAD_SIGNAL_WORKFLOW_EXECUTION_ATTRIBUTES"
				}(14), taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)
			},
		},
		{
			name: "size limit checker failure",
			attributes: &types.SignalExternalWorkflowExecutionDecisionAttributes{
				Domain:     constants.TestDomainName,
				Execution:  &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
				SignalName: testdata.SignalName,
				Input:      []byte("some input data"),
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes) {
				taskHandler.sizeLimitChecker.blobSizeLimitError = 1
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 2
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(2).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(testTaskCompletedID, &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
					Details: []byte("SignalExternalWorkflowExecutionDecisionAttributes.Input exceeds size limit."),
				}).Return(nil, errors.New("some error adding fail workflow event"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.SignalExternalWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, errors.New("some error adding fail workflow event"), err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeSignalExternalWorkflowExecution),
				SignalExternalWorkflowExecutionDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionUpsertWorkflowSearchAttributes(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes)
		attributes      *types.UpsertWorkflowSearchAttributesDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes, err error)
	}{
		{
			name: "success",
			attributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{"some-key": []byte(`"some-value"`)}},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(2).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddUpsertWorkflowSearchAttributesEvent(testTaskCompletedID, attr)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, nil, err)
			},
		},
		{
			name: "attributes validation failure",
			attributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{"some-key": []byte("some-value")}},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(1).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes, err error) {
				assert.True(t, taskHandler.failDecision)
				assert.Equal(t, "some-value is not a valid search attribute value for key some-key", *taskHandler.failMessage)
				assert.Equal(t, func(i int32) *types.DecisionTaskFailedCause {
					cause := new(types.DecisionTaskFailedCause)
					*cause = types.DecisionTaskFailedCause(i)
					return cause
					// 22 is types.DecisionTaskFailedCause "BAD_SEARCH_ATTRIBUTES"
				}(22), taskHandler.failDecisionCause)
			},
		},
		{
			name: "size limit checker failure",
			attributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{"some-key": []byte(`"some-value"`)}},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes) {
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 2
				taskHandler.sizeLimitChecker.blobSizeLimitError = 1
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(2).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(testTaskCompletedID, &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
					Details: []byte("UpsertWorkflowSearchAttributesDecisionAttributes exceeds size limit."),
				}).Return(nil, errors.New("some random error adding workflow event"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes, err error) {
				assert.False(t, taskHandler.failDecision)
				assert.Empty(t, taskHandler.failMessage)
				assert.Nil(t, taskHandler.failDecisionCause)
				assert.Equal(t, errors.New("some random error adding workflow event"), err)
			},
		},
		{
			name: "internal service error",
			attributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{"some-key": []byte(`"some-value"`)}},
			},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Times(1).Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return("", errors.New("some random error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.UpsertWorkflowSearchAttributesDecisionAttributes, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "Unable to get domain for domainID: deadbeef-0123-4567-890a-bcdef0123456."}, err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeUpsertWorkflowSearchAttributes),
				UpsertWorkflowSearchAttributesDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionStartTimer(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes)
		attributes      *types.StartTimerDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes, err error)
	}{
		{
			name:       "success",
			attributes: &types.StartTimerDecisionAttributes{TimerID: "test-timer-id"},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes) {
				attr.StartToFireTimeoutSeconds = new(int64)
				*attr.StartToFireTimeoutSeconds = 60
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddTimerStartedEvent(testTaskCompletedID, attr)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "attributes validation failure",
			attributes: &types.StartTimerDecisionAttributes{TimerID: "test-timer-id"},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes, err error) {
				assert.True(t, taskHandler.stopProcessing)
			},
		},
		{
			name:       "bad request error",
			attributes: &types.StartTimerDecisionAttributes{TimerID: "test-timer-id"},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes) {
				attr.StartToFireTimeoutSeconds = new(int64)
				*attr.StartToFireTimeoutSeconds = 60
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddTimerStartedEvent(testTaskCompletedID, attr).Return(nil, nil, &types.BadRequestError{Message: "some types.BadRequestError error"})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes, err error) {
				assert.Nil(t, err)
				assert.True(t, taskHandler.failDecision)
				assert.Equal(t, types.DecisionTaskFailedCauseStartTimerDuplicateID, *taskHandler.failDecisionCause)
			},
		},
		{
			name:       "default case error",
			attributes: &types.StartTimerDecisionAttributes{TimerID: "test-timer-id"},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes) {
				attr.StartToFireTimeoutSeconds = new(int64)
				*attr.StartToFireTimeoutSeconds = 60
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddTimerStartedEvent(testTaskCompletedID, attr).Return(nil, nil, errors.New("some random error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.StartTimerDecisionAttributes, err error) {
				assert.Equal(t, "some random error", err.Error())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType:                 common.Ptr(types.DecisionTypeStartTimer),
				StartTimerDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionCompleteWorkflow(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes)
		attributes      *types.CompleteWorkflowExecutionDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error)
	}{
		{
			name:       "handler has unhandled events",
			attributes: &types.CompleteWorkflowExecutionDecisionAttributes{},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes) {
				taskHandler.hasUnhandledEventsBeforeDecisions = true
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseUnhandledDecision, *taskHandler.failDecisionCause)
				assert.Equal(t, "cannot complete workflow, new pending decisions were scheduled while this decision was processing", *taskHandler.failMessage)
			},
		},
		{
			name: "attributes validation failure",
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes, *taskHandler.failDecisionCause)
			},
		},
		{
			name:       "blob size limit check failure",
			attributes: &types.CompleteWorkflowExecutionDecisionAttributes{Result: []byte("some-result")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes) {
				taskHandler.sizeLimitChecker.blobSizeLimitError = 5
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 3
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(taskHandler.sizeLimitChecker.completedID,
					&types.FailWorkflowExecutionDecisionAttributes{
						Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
						Details: []byte("CompleteWorkflowExecutionDecisionAttributes.Result exceeds size limit."),
					})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.True(t, taskHandler.stopProcessing)
				assert.Nil(t, err)
			},
		},
		{
			name:       "workflow not running",
			attributes: &types.CompleteWorkflowExecutionDecisionAttributes{Result: []byte("some-result")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(false)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "failure to get cron duration",
			attributes: &types.CompleteWorkflowExecutionDecisionAttributes{Result: []byte("some-result")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested()
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetCronBackoffDuration(context.Background()).Return(time.Second, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, "some error", err.Error())
			},
		},
		{
			name:       "internal service error cancel requested",
			attributes: &types.CompleteWorkflowExecutionDecisionAttributes{Result: []byte("some-result")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(true, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetCronBackoffDuration(context.Background()).Return(backoff.NoBackoff, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddCompletedWorkflowEvent(taskHandler.decisionTaskCompletedID, attr).Return(nil, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "Unable to add complete workflow event."}, err)
			},
		},
		{
			name:       "cancel requested - sucess",
			attributes: &types.CompleteWorkflowExecutionDecisionAttributes{Result: []byte("some-result")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(true, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetCronBackoffDuration(context.Background()).Return(backoff.NoBackoff, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddCompletedWorkflowEvent(taskHandler.decisionTaskCompletedID, attr)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "GetStartedEvent failure on cron workflow",
			attributes: &types.CompleteWorkflowExecutionDecisionAttributes{Result: []byte("some-result")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested()
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetCronBackoffDuration(context.Background())
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetStartEvent(context.Background()).Return(nil, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CompleteWorkflowExecutionDecisionAttributes, err error) {
				assert.NotNil(t, err)
				assert.Equal(t, "some error", err.Error())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeCompleteWorkflowExecution),
				CompleteWorkflowExecutionDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionFailWorkflow(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes)
		attributes      *types.FailWorkflowExecutionDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error)
	}{
		{
			name:       "handler has unhandled events",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				taskHandler.hasUnhandledEventsBeforeDecisions = true
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseUnhandledDecision, *taskHandler.failDecisionCause)
				assert.Equal(t, "cannot complete workflow, new pending decisions were scheduled while this decision was processing", *taskHandler.failMessage)
			},
		},
		{
			name: "attributes validation failure",
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes, *taskHandler.failDecisionCause)
				assert.Equal(t, "FailWorkflowExecutionDecisionAttributes is not set on decision.", *taskHandler.failMessage)
			},
		},
		{
			name:       "blob size limit check failure",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 3
				taskHandler.sizeLimitChecker.blobSizeLimitError = 5
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(taskHandler.sizeLimitChecker.completedID,
					&types.FailWorkflowExecutionDecisionAttributes{
						Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
						Details: []byte("FailWorkflowExecutionDecisionAttributes.Details exceeds size limit."),
					})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.True(t, taskHandler.stopProcessing)
				assert.Nil(t, err)
			},
		},
		{
			name:       "workflow not running",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(false)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "cancel requested - failure to add cancel event",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(true, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddWorkflowExecutionCanceledEvent(taskHandler.decisionTaskCompletedID,
					&types.CancelWorkflowExecutionDecisionAttributes{
						Details: attr.Details,
					}).Return(nil, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.NotNil(t, err)
				assert.Equal(t, "some error", err.Error())
			},
		},
		{
			name:       "cancel requested - success",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(true, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddWorkflowExecutionCanceledEvent(taskHandler.decisionTaskCompletedID,
					&types.CancelWorkflowExecutionDecisionAttributes{
						Details: attr.Details,
					})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "failure to get backoff duration on cron",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(false, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetRetryBackoffDuration(attr.GetReason()).Return(backoff.NoBackoff)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetCronBackoffDuration(context.Background()).Return(time.Second, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.True(t, taskHandler.stopProcessing)
				assert.NotNil(t, err)
				assert.Equal(t, "some error", err.Error())
			},
		},
		{
			name:       "AddFailWorkflowEvent failure",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(false, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetRetryBackoffDuration(attr.GetReason()).Return(backoff.NoBackoff)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetCronBackoffDuration(context.Background()).Return(backoff.NoBackoff, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(taskHandler.decisionTaskCompletedID, attr).Return(nil, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.NotNil(t, err)
				assert.Equal(t, "some error", err.Error())
			},
		},
		{
			name:       "cron workflow - success",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(false, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetRetryBackoffDuration(attr.GetReason()).Return(backoff.NoBackoff)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetCronBackoffDuration(context.Background()).Return(backoff.NoBackoff, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(taskHandler.decisionTaskCompletedID, attr)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "GetStartEvent failure",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(false, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetRetryBackoffDuration(attr.GetReason())
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetStartEvent(context.Background()).Return(nil, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.NotNil(t, err)
				assert.Equal(t, "some error", err.Error())
			},
		},
		{
			name:       "success",
			attributes: &types.FailWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes) {
				attr.Reason = new(string)
				*attr.Reason = "some reason"
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(false, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetRetryBackoffDuration(attr.GetReason())
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetStartEvent(context.Background()).Return(&types.HistoryEvent{WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{}}, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddContinueAsNewEvent(context.Background(),
					taskHandler.decisionTaskCompletedID,
					taskHandler.decisionTaskCompletedID,
					gomock.Any(),
					gomock.Any())
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.FailWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType:                            common.Ptr(types.DecisionTypeFailWorkflowExecution),
				FailWorkflowExecutionDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionCancelWorkflow(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes)
		attributes      *types.CancelWorkflowExecutionDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes, err error)
	}{
		{
			name:       "handler has unhandled events",
			attributes: &types.CancelWorkflowExecutionDecisionAttributes{},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes) {
				taskHandler.hasUnhandledEventsBeforeDecisions = true
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseUnhandledDecision, *taskHandler.failDecisionCause)
				assert.Equal(t, "cannot process cancellation, new pending decisions were scheduled while this decision was processing", *taskHandler.failMessage)
			},
		},
		{
			name: "attributes validation failure",
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes, *taskHandler.failDecisionCause)
				assert.Nil(t, err)
				assert.True(t, taskHandler.stopProcessing)
			},
		},
		{
			name:       "workflow not running",
			attributes: &types.CancelWorkflowExecutionDecisionAttributes{Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes) {
				taskHandler.metricsClient = new(mocks.Client)
				taskHandler.logger = new(log.MockLogger)
				taskHandler.metricsClient.(*mocks.Client).On("IncCounter", metrics.HistoryRespondDecisionTaskCompletedScope, metrics.DecisionTypeCancelWorkflowCounter)
				taskHandler.metricsClient.(*mocks.Client).On("IncCounter", metrics.HistoryRespondDecisionTaskCompletedScope, metrics.MultipleCompletionDecisionsCounter)
				taskHandler.logger.(*log.MockLogger).On("Warn", "Multiple completion decisions", []tag.Tag{tag.WorkflowDecisionType(int64(types.DecisionTypeCancelWorkflowExecution)), tag.ErrorTypeMultipleCompletionDecisions})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(false)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "success",
			attributes: &types.CancelWorkflowExecutionDecisionAttributes{},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddWorkflowExecutionCanceledEvent(taskHandler.decisionTaskCompletedID, attr)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.CancelWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeCancelWorkflowExecution),
				CancelWorkflowExecutionDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionRecordMarker(t *testing.T) {
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.RecordMarkerDecisionAttributes)
		attributes      *types.RecordMarkerDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.RecordMarkerDecisionAttributes, err error)
	}{
		{
			name:       "blob size limit check failure",
			attributes: &types.RecordMarkerDecisionAttributes{MarkerName: "some-marker", Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.RecordMarkerDecisionAttributes) {
				taskHandler.sizeLimitChecker.blobSizeLimitError = 5
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 3
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(testTaskCompletedID, &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
					Details: []byte("RecordMarkerDecisionAttributes.Details exceeds size limit."),
				})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.RecordMarkerDecisionAttributes, err error) {
				assert.True(t, taskHandler.stopProcessing)
				assert.Nil(t, err)
			},
		},
		{
			name: "attributes validation failure",
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.RecordMarkerDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseBadRecordMarkerAttributes, *taskHandler.failDecisionCause)
				assert.Nil(t, err)
				assert.True(t, taskHandler.stopProcessing)
			},
		},
		{
			name:       "success",
			attributes: &types.RecordMarkerDecisionAttributes{MarkerName: "some-marker", Details: []byte("some-details")},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.RecordMarkerDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddRecordMarkerEvent(taskHandler.decisionTaskCompletedID, attr)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.RecordMarkerDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType:                   common.Ptr(types.DecisionTypeRecordMarker),
				RecordMarkerDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestHandleDecisionScheduleActivity(t *testing.T) {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testdata.DomainID, Name: testdata.DomainName},
		&persistence.DomainConfig{
			Retention:   1,
			BadBinaries: types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"test-binary-checksum": {Reason: "some reason"}}},
		},
		cluster.TestCurrentClusterName)
	executionInfo := &persistence.WorkflowExecutionInfo{
		DomainID:        testdata.DomainID,
		WorkflowID:      testdata.WorkflowID,
		WorkflowTimeout: 100,
	}
	validAttr := &types.ScheduleActivityTaskDecisionAttributes{
		Domain:                        testdata.DomainName,
		TaskList:                      &types.TaskList{Name: testdata.TaskListName},
		ActivityID:                    "some-activity-id",
		ActivityType:                  &types.ActivityType{Name: testdata.ActivityTypeName},
		ScheduleToCloseTimeoutSeconds: func(i int32) *int32 { return &i }(100),
		ScheduleToStartTimeoutSeconds: func(i int32) *int32 { return &i }(20),
		StartToCloseTimeoutSeconds:    func(i int32) *int32 { return &i }(80),
		Input:                         []byte("some-input"),
	}

	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes)
		attributes      *types.ScheduleActivityTaskDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error)
	}{
		{
			name:       "internal service error",
			attributes: &types.ScheduleActivityTaskDecisionAttributes{Domain: testdata.DomainName},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(nil, errors.New("some radom error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.NotNil(t, err)
				assert.Nil(t, res)
				assert.Equal(t, &types.InternalServiceError{Message: fmt.Sprintf("Unable to schedule activity across domain %v.", attr.GetDomain())}, err)
			},
		},
		{
			name:       "attributes validation failure",
			attributes: &types.ScheduleActivityTaskDecisionAttributes{Domain: testdata.DomainName},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.Nil(t, err)
				assert.Nil(t, res)
				assert.True(t, taskHandler.stopProcessing)
			},
		},
		{
			name:       "blob size limit check failure",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 3
				taskHandler.sizeLimitChecker.blobSizeLimitError = 5
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(testTaskCompletedID,
					&types.FailWorkflowExecutionDecisionAttributes{
						Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
						Details: []byte("ScheduleActivityTaskDecisionAttributes.Input exceeds size limit."),
					})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.Nil(t, err)
				assert.Nil(t, res)
				assert.True(t, taskHandler.stopProcessing)
			},
		},
		{
			name:       "success - activity started",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskScheduledEvent(context.Background(), taskHandler.decisionTaskCompletedID, attr, taskHandler.activityCountToDispatch > 0).
					Return(&types.HistoryEvent{}, &persistence.ActivityInfo{}, &types.ActivityLocalDispatchInfo{}, true, true, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskStartedEvent(&persistence.ActivityInfo{}, int64(0), gomock.Any(), taskHandler.identity)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.Nil(t, err)
				assert.Nil(t, res)
			},
		},
		{
			name:       "token serialization failure",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskScheduledEvent(context.Background(), taskHandler.decisionTaskCompletedID, attr, taskHandler.activityCountToDispatch > 0).
					Return(&types.HistoryEvent{}, &persistence.ActivityInfo{}, &types.ActivityLocalDispatchInfo{}, true, false, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskStartedEvent(&persistence.ActivityInfo{}, int64(0), gomock.Any(), taskHandler.identity)
				taskHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Serialize(&common.TaskToken{
					DomainID:     testdata.DomainID,
					WorkflowID:   testdata.WorkflowID,
					ActivityType: testdata.ActivityTypeName,
				}).Return(nil, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.Equal(t, workflow.ErrSerializingToken, err)
				assert.Nil(t, res)
			},
		},
		{
			name:       "success - decisionResult non nil",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskScheduledEvent(context.Background(), taskHandler.decisionTaskCompletedID, attr, taskHandler.activityCountToDispatch > 0).
					Return(&types.HistoryEvent{}, &persistence.ActivityInfo{}, &types.ActivityLocalDispatchInfo{}, true, false, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskStartedEvent(&persistence.ActivityInfo{}, int64(0), gomock.Any(), taskHandler.identity)
				taskHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Serialize(&common.TaskToken{
					DomainID:     testdata.DomainID,
					WorkflowID:   testdata.WorkflowID,
					ActivityType: testdata.ActivityTypeName,
				}).Return([]byte("some-serialized-data"), nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.Nil(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, []byte("some-serialized-data"), res.activityDispatchInfo.TaskToken)
			},
		},
		{
			name:       "failure to add activity task started",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskScheduledEvent(context.Background(), taskHandler.decisionTaskCompletedID, attr, taskHandler.activityCountToDispatch > 0).
					Return(&types.HistoryEvent{}, &persistence.ActivityInfo{}, &types.ActivityLocalDispatchInfo{}, true, true, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskStartedEvent(&persistence.ActivityInfo{}, int64(0), gomock.Any(), taskHandler.identity).Return(nil, errors.New("some error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.NotNil(t, err)
				assert.Equal(t, "some error", err.Error())
				assert.Nil(t, res)
			},
		},
		{
			name:       "activity scheduled - nil activityDispatchInfo",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskScheduledEvent(context.Background(), taskHandler.decisionTaskCompletedID, attr, taskHandler.activityCountToDispatch > 0).
					Return(&types.HistoryEvent{}, &persistence.ActivityInfo{}, nil, true, false, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.Nil(t, err)
				assert.Nil(t, res)
			},
		},
		{
			name:       "bad request error",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskScheduledEvent(context.Background(), taskHandler.decisionTaskCompletedID, attr, taskHandler.activityCountToDispatch > 0).
					Return(nil, nil, nil, false, false, &types.BadRequestError{
						Message: "some bad request error",
					})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.Nil(t, err)
				assert.Nil(t, res)
			},
		},
		{
			name:       "default error case",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(attr.GetDomain()).Return(domainEntry, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddActivityTaskScheduledEvent(context.Background(), taskHandler.decisionTaskCompletedID, attr, taskHandler.activityCountToDispatch > 0).
					Return(nil, nil, nil, false, false, errors.New("some default error"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ScheduleActivityTaskDecisionAttributes, res *decisionResult, err error) {
				assert.NotNil(t, err)
				assert.Equal(t, "some default error", err.Error())
				assert.Nil(t, res)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType:                           common.Ptr(types.DecisionTypeScheduleActivityTask),
				ScheduleActivityTaskDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			assert.NotNil(t, err)
			assert.Equal(t, &types.BadRequestError{Message: fmt.Sprintf("Unknown decision type: %v", decision.GetDecisionType())}, err)

			decisionRes, err := taskHandler.handleDecisionWithResult(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, decisionRes, err)
		})
	}
}

func TestHandleDecisionContinueAsNewWorkflow(t *testing.T) {
	executionInfo := &persistence.WorkflowExecutionInfo{
		DomainID:        testdata.DomainID,
		WorkflowID:      testdata.WorkflowID,
		WorkflowTimeout: 100,
		ParentDomainID:  "parent-domain-id",
	}
	validAttr := &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
		WorkflowType:                        &types.WorkflowType{Name: testdata.WorkflowTypeName},
		TaskList:                            &types.TaskList{Name: testdata.TaskListName},
		Input:                               []byte("some-input"),
		ExecutionStartToCloseTimeoutSeconds: func(i int32) *int32 { return &i }(100),
		TaskStartToCloseTimeoutSeconds:      func(i int32) *int32 { return &i }(80),
		BackoffStartIntervalInSeconds:       new(int32),
		FailureReason:                       func(i string) *string { return &i }("some reason"),
		SearchAttributes:                    &types.SearchAttributes{IndexedFields: map[string][]byte{"some-key": []byte(`"some-value"`)}},
	}
	tests := []struct {
		name            string
		expectMockCalls func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes)
		attributes      *types.ContinueAsNewWorkflowExecutionDecisionAttributes
		asserts         func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error)
	}{
		{
			name:       "handler has unhandled events",
			attributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{},
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.hasUnhandledEventsBeforeDecisions = true
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseUnhandledDecision, *taskHandler.failDecisionCause)
				assert.Equal(t, "cannot complete workflow, new pending decisions were scheduled while this decision was processing", *taskHandler.failMessage)
			},
		},
		{
			name: "attributes validation failure",
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.Equal(t, types.DecisionTaskFailedCauseBadContinueAsNewAttributes, *taskHandler.failDecisionCause)
				assert.Equal(t, "ContinueAsNewWorkflowExecutionDecisionAttributes is not set on decision.", *taskHandler.failMessage)
			},
		},
		{
			name:       "blob size limit check failure",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.sizeLimitChecker.blobSizeLimitWarn = 3
				taskHandler.sizeLimitChecker.blobSizeLimitError = 5
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.attrValidator.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(testdata.DomainID).Return(testdata.DomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(testTaskCompletedID, &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
					Details: []byte("ContinueAsNewWorkflowExecutionDecisionAttributes. Input exceeds size limit."),
				})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.True(t, taskHandler.stopProcessing)
				assert.NoError(t, err)
			},
		},
		{
			name:       "workflow not running",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.attrValidator.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(testdata.DomainID).Return(testdata.DomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(false)
				taskHandler.metricsClient = new(mocks.Client)
				taskHandler.logger = new(log.MockLogger)
				taskHandler.metricsClient.(*mocks.Client).On("IncCounter", metrics.HistoryRespondDecisionTaskCompletedScope, metrics.DecisionTypeContinueAsNewCounter)
				taskHandler.metricsClient.(*mocks.Client).On("IncCounter", metrics.HistoryRespondDecisionTaskCompletedScope, metrics.MultipleCompletionDecisionsCounter)
				taskHandler.logger.(*log.MockLogger).On("Warn", "Multiple completion decisions", []tag.Tag{tag.WorkflowDecisionType(int64(types.DecisionTypeContinueAsNewWorkflowExecution)), tag.ErrorTypeMultipleCompletionDecisions})
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "cancel requested - failure to add cancel event",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.attrValidator.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(testdata.DomainID).Return(testdata.DomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(true, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddWorkflowExecutionCanceledEvent(taskHandler.decisionTaskCompletedID,
					&types.CancelWorkflowExecutionDecisionAttributes{}).Return(nil, errors.New("some error adding execution canceled event"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.ErrorContains(t, err, "some error adding execution canceled event")
			},
		},
		{
			name:       "cancel requested - success",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.attrValidator.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(testdata.DomainID).Return(testdata.DomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(true, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddWorkflowExecutionCanceledEvent(taskHandler.decisionTaskCompletedID,
					&types.CancelWorkflowExecutionDecisionAttributes{}).Return(&types.HistoryEvent{}, nil)
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:       "failure to extract parent domain name",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.attrValidator.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(testdata.DomainID).Return(testdata.DomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(false, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().HasParentExecution().Return(true)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(executionInfo.ParentDomainID).Return("", errors.New("some error getting parent domain name"))
			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.ErrorContains(t, err, "some error getting parent domain name")
			},
		},
		{
			name:       "failure to add continueAsNew event",
			attributes: validAttr,
			expectMockCalls: func(taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes) {
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(executionInfo).Times(2)
				taskHandler.attrValidator.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(testdata.DomainID).Return(testdata.DomainName, nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsWorkflowExecutionRunning().Return(true)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().IsCancelRequested().Return(false, "")
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().HasParentExecution().Return(true)
				taskHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(executionInfo.ParentDomainID).Return("parent-domain-name", nil)
				taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddContinueAsNewEvent(context.Background(), taskHandler.decisionTaskCompletedID, taskHandler.decisionTaskCompletedID, "parent-domain-name", attr).
					Return(nil, nil, errors.New("some error adding continueAsNew event"))

			},
			asserts: func(t *testing.T, taskHandler *taskHandlerImpl, attr *types.ContinueAsNewWorkflowExecutionDecisionAttributes, err error) {
				assert.ErrorContains(t, err, "some error adding continueAsNew event")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskHandler := newTaskHandlerForTest(t)
			if test.expectMockCalls != nil {
				test.expectMockCalls(taskHandler, test.attributes)
			}
			decision := &types.Decision{
				DecisionType: common.Ptr(types.DecisionTypeContinueAsNewWorkflowExecution),
				ContinueAsNewWorkflowExecutionDecisionAttributes: test.attributes,
			}
			err := taskHandler.handleDecision(context.Background(), decision)
			test.asserts(t, taskHandler, test.attributes, err)
		})
	}
}

func TestValidateAttributes(t *testing.T) {
	t.Run("failure", func(t *testing.T) {
		taskHandler := newTaskHandlerForTest(t)
		validationFn := func() error { return errors.New("some validation error") }
		failedCause := new(types.DecisionTaskFailedCause)
		err := taskHandler.validateDecisionAttr(validationFn, *failedCause)
		assert.ErrorContains(t, err, "some validation error")
	})
}

func TestHandleDecisions(t *testing.T) {
	t.Run("workflow size exceeds limit", func(t *testing.T) {
		taskHandler := newTaskHandlerForTest(t)
		taskHandler.sizeLimitChecker.historyCountLimitError = 10
		taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetNextEventID().Return(int64(12)) //nextEventID - 1 > historyCountLimit of 10
		taskHandler.mutableState.(*execution.MockMutableState).EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
		taskHandler.mutableState.(*execution.MockMutableState).EXPECT().AddFailWorkflowEvent(taskHandler.sizeLimitChecker.completedID, &types.FailWorkflowExecutionDecisionAttributes{
			Reason:  common.StringPtr(common.FailureReasonSizeExceedsLimit),
			Details: []byte("Workflow history size / count exceeds limit."),
		}).Return(nil, errors.New("some error adding fail workflow event"))
		res, err := taskHandler.handleDecisions(context.Background(), []byte{}, []*types.Decision{})
		assert.Nil(t, res)
		assert.ErrorContains(t, err, "some error adding fail workflow event")
	})
}

func newTaskHandlerForTest(t *testing.T) *taskHandlerImpl {
	ctrl := gomock.NewController(t)
	testLogger := testlogger.New(t)
	testConfig := config.NewForTest()
	testConfig.ValidSearchAttributes = func(opts ...dynamicconfig.FilterOption) map[string]interface{} {
		validSearchAttr := make(map[string]interface{})
		validSearchAttr["some-key"] = types.IndexedValueTypeString
		return validSearchAttr
	}
	mockMutableState := execution.NewMockMutableState(ctrl)
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	workflowSizeChecker := newWorkflowSizeChecker(
		"testDomain",
		testConfig.BlobSizeLimitWarn(constants.TestDomainName),
		testConfig.BlobSizeLimitError(constants.TestDomainName),
		testConfig.HistorySizeLimitWarn(constants.TestDomainName),
		testConfig.HistorySizeLimitError(constants.TestDomainName),
		testConfig.HistoryCountLimitWarn(constants.TestDomainName),
		testConfig.HistoryCountLimitError(constants.TestDomainName),
		testTaskCompletedID,
		mockMutableState,
		&persistence.ExecutionStats{},
		metrics.NewClient(tally.NoopScope, metrics.History).Scope(metrics.HistoryRespondDecisionTaskCompletedScope, metrics.DomainTag(constants.TestDomainName)),
		testLogger,
	)
	mockMutableState.EXPECT().HasBufferedEvents().Return(false)
	taskHandler := newDecisionTaskHandler(
		testdata.Identity,
		testTaskCompletedID,
		constants.TestLocalDomainEntry,
		mockMutableState,
		newAttrValidator(mockDomainCache, metrics.NewClient(tally.NoopScope, metrics.History), testConfig, testlogger.New(t)),
		workflowSizeChecker,
		common.NewMockTaskTokenSerializer(ctrl),
		testLogger,
		mockDomainCache,
		metrics.NewClient(tally.NoopScope, metrics.History),
		testConfig,
	)
	return taskHandler
}
