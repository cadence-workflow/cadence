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

package invariant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

type TimerInvalidTest struct {
	suite.Suite
}

func TestTimerInvalidSuite(t *testing.T) {
	suite.Run(t, new(TimerInvalidTest))
}

func (ts *TimerInvalidTest) TestCheck() {
	testCases := []struct {
		name           string
		ctxExpired     bool
		getExecResp    *persistence.GetWorkflowExecutionResponse
		getExecErr     error
		expectedResult CheckResult
		entity         interface{}
	}{
		{
			name:       "Context expired",
			ctxExpired: true,
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   "TimerInvalid",
				Info:            "failed to check: context expired or cancelled",
				InfoDetails:     "context deadline exceeded",
			},
			getExecResp: &persistence.GetWorkflowExecutionResponse{},
			getExecErr:  errors.New("random error"),
			entity:      &entity.Timer{},
		},
		{
			name: "Check if entity is a timer",
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   "TimerInvalid",
				Info:            "failed to check: expected timer entity",
				InfoDetails:     "",
			},
		},
		{
			name: "Check for persistence error",
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   "TimerInvalid",
				Info:            "failed to get workflow for timer",
				InfoDetails:     "",
			},
			getExecResp: nil,
			getExecErr:  errors.New("random error"),
			entity:      &entity.Timer{},
		},
		{
			name: "Workflow not found in persistence",
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   "TimerInvalid",
				Info:            "timer scheduled for non existing workflow",
				InfoDetails:     "",
			},
			getExecResp: nil,
			getExecErr:  &types.EntityNotExistsError{},
			entity:      &entity.Timer{},
		},
		{
			name: "Workflow is closed",
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   "TimerInvalid",
				Info:            "timer scheduled for closed workflow",
				InfoDetails:     "",
			},
			getExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State: closedState,
					},
				},
			},
			getExecErr: nil,
			entity:     &entity.Timer{},
		},
		{
			name: "Check passed",
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   "TimerInvalid",
			},
			getExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State: openState,
					},
				},
			},
			getExecErr: nil,
			entity:     &entity.Timer{},
		},
	}
	ctrl := gomock.NewController(ts.T())
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	for _, tc := range testCases {
		mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain-name", nil).AnyTimes()
		ts.Run(tc.name, func() {
			execManager := &mocks.ExecutionManager{}
			execManager.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(tc.getExecResp, tc.getExecErr)
			i := NewTimerInvalid(
				persistence.NewPersistenceRetryer(
					execManager,
					nil,
					common.CreatePersistenceRetryPolicy(),
				),
				mockDomainCache,
			)
			ctx := context.Background()
			if tc.ctxExpired {
				ctx, _ = context.WithDeadline(ctx, time.Now())
			}
			result := i.Check(ctx, tc.entity)
			ts.Equal(tc.expectedResult, result)
		})
	}
}

func (ts *TimerInvalidTest) TestFix() {
	testCases := []struct {
		name           string
		ctxExpired     bool
		getExecResp    *persistence.GetWorkflowExecutionResponse
		getExecErr     error
		expectedResult FixResult
		entity         interface{}
		ttComplete     error
	}{
		{
			name:       "context expired",
			ctxExpired: true,
			expectedResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: "TimerInvalid",
				Info:          "failed to check: context expired or cancelled",
				InfoDetails:   "context deadline exceeded",
			},
			getExecResp: &persistence.GetWorkflowExecutionResponse{},
			getExecErr:  errors.New("random error"),
			entity:      &entity.Timer{},
		},
		{
			name: "check before fix fails bc it's not a timer",
			expectedResult: FixResult{
				FixResultType: FixResultTypeFailed,
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeFailed,
					InvariantName:   "TimerInvalid",
					Info:            "failed to check: expected timer entity",
				},
				InvariantName: "TimerInvalid",
				Info:          "failed fix because check failed",
				InfoDetails:   "",
			},
		},
		{
			name: "timer type is not a user timer",
			getExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State: closedState,
					},
				},
			},
			expectedResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: "TimerInvalid",
				Info:          "timer is not a TaskTypeUserTimer",
				InfoDetails:   "",
			},
			entity: &entity.Timer{
				TaskType: persistence.TaskTypeActivityRetryTimer,
			},
		},
		{
			name: "timer deletion fails on persistence",
			getExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State: closedState,
					},
				},
			},
			expectedResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: "TimerInvalid",
				Info:          "error from persistence",
				InfoDetails:   "",
			},
			entity: &entity.Timer{
				TaskType: persistence.TaskTypeUserTimer,
			},
			ttComplete: errors.New("error from persistence"),
		},
		{
			name: "timer deleted",
			getExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State: closedState,
					},
				},
			},
			expectedResult: FixResult{
				FixResultType: FixResultTypeFixed,
				InvariantName: "TimerInvalid",
				Info:          "",
				InfoDetails:   "",
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   "TimerInvalid",
					Info:            "timer scheduled for closed workflow",
				},
			},
			entity: &entity.Timer{
				TaskType: persistence.TaskTypeUserTimer,
			},
			ttComplete: nil,
		},
	}
	ctrl := gomock.NewController(ts.T())
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	for _, tc := range testCases {
		mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain-name", nil).AnyTimes()
		ts.Run(tc.name, func() {
			execManager := &mocks.ExecutionManager{}
			execManager.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(tc.getExecResp, tc.getExecErr)
			execManager.On("CompleteTimerTask", mock.Anything, mock.Anything).Return(tc.ttComplete)
			i := NewTimerInvalid(
				persistence.NewPersistenceRetryer(
					execManager,
					nil,
					common.CreatePersistenceRetryPolicy(),
				),
				mockDomainCache,
			)
			ctx := context.Background()
			if tc.ctxExpired {
				ctx, _ = context.WithDeadline(ctx, time.Now())
			}
			result := i.Fix(ctx, tc.entity)
			ts.Equal(tc.expectedResult, result)
		})
	}

}
