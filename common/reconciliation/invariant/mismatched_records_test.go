// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package invariant

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
)

func TestMismatchedRecords_Check(t *testing.T) {
	testCases := []struct {
		name             string
		execution        interface{}
		domainErr        error
		currentExecResp  *persistence.GetCurrentExecutionResponse
		currentExecErr   error
		workflowExecResp *persistence.GetWorkflowExecutionResponse
		workflowExecErr  error
		expectedResult   CheckResult
	}{
		{
			name:      "not a concrete execution",
			execution: "not-an-execution",
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   MismatchedRecords,
				Info:            "failed to check: expected concrete execution",
			},
		},
		{
			name:      "domain cache error",
			execution: getClosedConcreteExecution(),
			domainErr: errors.New("domain not found"),
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   MismatchedRecords,
				Info:            "failed to check: expected DomainName",
				InfoDetails:     "domain not found",
			},
		},
		{
			name:           "get current execution error",
			execution:      getClosedConcreteExecution(),
			currentExecErr: errors.New("db error"),
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   MismatchedRecords,
				Info:            "failed to get current execution record",
				InfoDetails:     "db error",
			},
		},
		{
			name:      "get workflow execution error",
			execution: getClosedConcreteExecution(),
			currentExecResp: &persistence.GetCurrentExecutionResponse{
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			},
			workflowExecErr: errors.New("db error"),
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   MismatchedRecords,
				Info:            "failed to get workflow execution",
				InfoDetails:     "db error",
			},
		},
		{
			name:      "mismatched close status",
			execution: getClosedConcreteExecution(),
			currentExecResp: &persistence.GetCurrentExecutionResponse{
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			},
			workflowExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						CloseStatus: persistence.WorkflowCloseStatusTerminated,
					},
				},
			},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   MismatchedRecords,
				Info:            "current record does not agree with concrete record about workflow being closed",
				InfoDetails:     "current: 1 concrete: 4",
			},
		},
		{
			name:      "matching close status",
			execution: getClosedConcreteExecution(),
			currentExecResp: &persistence.GetCurrentExecutionResponse{
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			},
			workflowExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						CloseStatus: persistence.WorkflowCloseStatusCompleted,
					},
				},
			},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   MismatchedRecords,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, tc.domainErr).MaxTimes(1)

			execManager := &mocks.ExecutionManager{}
			execManager.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(tc.currentExecResp, tc.currentExecErr)
			execManager.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(tc.workflowExecResp, tc.workflowExecErr)

			inv := NewMismatchedRecords(
				persistence.NewPersistenceRetryer(execManager, nil, common.CreatePersistenceRetryPolicy()),
				mockDomainCache,
			)

			result := inv.Check(context.Background(), tc.execution)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestMismatchedRecords_Fix(t *testing.T) {
	testCases := []struct {
		name             string
		execution        interface{}
		currentExecResp  *persistence.GetCurrentExecutionResponse
		currentExecErr   error
		workflowExecResp *persistence.GetWorkflowExecutionResponse
		workflowExecErr  error
		deleteExecErr    error
		deleteCurrentErr error
		expectedResult   FixResult
	}{
		{
			name:      "fix skipped because healthy",
			execution: getClosedConcreteExecution(),
			currentExecResp: &persistence.GetCurrentExecutionResponse{
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			},
			workflowExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						CloseStatus: persistence.WorkflowCloseStatusCompleted,
					},
				},
			},
			expectedResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: MismatchedRecords,
				Info:          "skipped fix because execution was healthy",
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeHealthy,
					InvariantName:   MismatchedRecords,
				},
			},
		},
		{
			name:      "fix succeeds for mismatched records",
			execution: getClosedConcreteExecution(),
			currentExecResp: &persistence.GetCurrentExecutionResponse{
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			},
			workflowExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						CloseStatus: persistence.WorkflowCloseStatusTerminated,
					},
				},
			},
			expectedResult: FixResult{
				FixResultType: FixResultTypeFixed,
				InvariantName: MismatchedRecords,
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   MismatchedRecords,
					Info:            "current record does not agree with concrete record about workflow being closed",
					InfoDetails:     "current: 1 concrete: 4",
				},
			},
		},
		{
			name:      "fix fails on delete error",
			execution: getClosedConcreteExecution(),
			currentExecResp: &persistence.GetCurrentExecutionResponse{
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			},
			workflowExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						CloseStatus: persistence.WorkflowCloseStatusTerminated,
					},
				},
			},
			deleteExecErr: errors.New("delete failed"),
			expectedResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: MismatchedRecords,
				Info:          "failed to delete concrete workflow execution",
				InfoDetails:   "delete failed",
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   MismatchedRecords,
					Info:            "current record does not agree with concrete record about workflow being closed",
					InfoDetails:     "current: 1 concrete: 4",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()

			execManager := &mocks.ExecutionManager{}
			execManager.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(tc.currentExecResp, tc.currentExecErr)
			execManager.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(tc.workflowExecResp, tc.workflowExecErr)
			execManager.On("DeleteWorkflowExecution", mock.Anything, mock.Anything).Return(tc.deleteExecErr)
			execManager.On("DeleteCurrentWorkflowExecution", mock.Anything, mock.Anything).Return(tc.deleteCurrentErr)

			inv := NewMismatchedRecords(
				persistence.NewPersistenceRetryer(execManager, nil, common.CreatePersistenceRetryPolicy()),
				mockDomainCache,
			)

			result := inv.Fix(context.Background(), tc.execution)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}
