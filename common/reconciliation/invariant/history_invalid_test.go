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

	c "github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

func TestHistoryInvalid_Check(t *testing.T) {
	testCases := []struct {
		name           string
		execution      interface{}
		domainErr      error
		historyResps   []*persistence.ReadHistoryBranchResponse
		historyErrs    []error
		wfExecResp     *persistence.GetWorkflowExecutionResponse
		wfExecErr      error
		expectedResult CheckResult
	}{
		{
			name:      "not a concrete execution",
			execution: "not-an-execution",
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   HistoryInvalid,
				Info:            "failed to check: expected concrete execution",
			},
		},
		{
			name:      "domain cache error",
			execution: getOpenConcreteExecution(),
			domainErr: errors.New("domain not found"),
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   HistoryInvalid,
				Info:            "failed to check: expected DomainName",
				InfoDetails:     "domain not found",
			},
		},
		{
			name:      "corrupted history",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{nil},
			historyErrs:  []error{persistence.ErrCorruptedHistory},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   HistoryInvalid,
				Info:            "corrupt history",
				InfoDetails:     persistence.ErrCorruptedHistory.Error(),
			},
		},
		{
			name:      "read error",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{nil},
			historyErrs:  []error{errors.New("db connection failed")},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   HistoryInvalid,
				Info:            "failed to read history",
				InfoDetails:     "db connection failed",
			},
		},
		{
			name:      "empty history",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{HistoryEvents: []*types.HistoryEvent{}, NextPageToken: nil},
			},
			historyErrs: []error{nil},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   HistoryInvalid,
				Info:            "empty history",
				InfoDetails:     "no history events found for workflow",
			},
		},
		{
			name:      "get workflow execution error",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{HistoryEvents: []*types.HistoryEvent{{ID: 1}}},
			},
			historyErrs: []error{nil},
			wfExecErr:   errors.New("db error"),
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   HistoryInvalid,
				Info:            "failed to get workflow execution",
				InfoDetails:     "db error",
			},
		},
		{
			name:      "event count mismatch on closed workflow",
			execution: getClosedConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{HistoryEvents: []*types.HistoryEvent{{ID: 1}, {ID: 2}}},
			},
			historyErrs: []error{nil},
			wfExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State:       closedState,
						NextEventID: 10,
					},
				},
			},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   HistoryInvalid,
				Info:            "event count mismatch",
				InfoDetails:     "history has last event ID 2 but execution record expects 9",
			},
		},
		{
			name:      "open workflow skips event count check",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{HistoryEvents: []*types.HistoryEvent{{ID: 1}, {ID: 2}}},
			},
			historyErrs: []error{nil},
			wfExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State:       openState,
						NextEventID: 10,
					},
				},
			},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   HistoryInvalid,
			},
		},
		{
			name:      "healthy single page",
			execution: getClosedConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{HistoryEvents: []*types.HistoryEvent{{ID: 1}}},
			},
			historyErrs: []error{nil},
			wfExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State:       closedState,
						NextEventID: 2,
					},
				},
			},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   HistoryInvalid,
			},
		},
		{
			name:      "healthy multi page",
			execution: getClosedConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{
					HistoryEvents: []*types.HistoryEvent{{ID: 1}},
					NextPageToken: []byte("next"),
				},
				{
					HistoryEvents: []*types.HistoryEvent{{ID: 2}, {ID: 3}},
					NextPageToken: nil,
				},
			},
			historyErrs: []error{nil, nil},
			wfExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State:       closedState,
						NextEventID: 4,
					},
				},
			},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   HistoryInvalid,
			},
		},
		{
			name:      "corruption on second page",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{
					HistoryEvents: []*types.HistoryEvent{{ID: 1}},
					NextPageToken: []byte("next"),
				},
				nil,
			},
			historyErrs: []error{nil, persistence.ErrCorruptedHistory},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   HistoryInvalid,
				Info:            "corrupt history",
				InfoDetails:     persistence.ErrCorruptedHistory.Error(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			if _, ok := tc.execution.(*entity.ConcreteExecution); ok {
				mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, tc.domainErr).MaxTimes(1)
			}

			historyManager := &mocks.HistoryV2Manager{}
			for i := range tc.historyResps {
				historyManager.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(tc.historyResps[i], tc.historyErrs[i]).Once()
			}

			execManager := &mocks.ExecutionManager{}
			execManager.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(tc.wfExecResp, tc.wfExecErr)

			inv := NewHistoryInvalid(
				persistence.NewPersistenceRetryer(execManager, historyManager, c.CreatePersistenceRetryPolicy()),
				mockDomainCache,
			)

			result := inv.Check(context.Background(), tc.execution)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestHistoryInvalid_Fix(t *testing.T) {
	testCases := []struct {
		name             string
		execution        interface{}
		historyResps     []*persistence.ReadHistoryBranchResponse
		historyErrs      []error
		wfExecResp       *persistence.GetWorkflowExecutionResponse
		wfExecErr        error
		deleteExecErr    error
		deleteCurrentErr error
		expectedResult   FixResult
	}{
		{
			name:      "fix skipped because healthy",
			execution: getClosedConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{HistoryEvents: []*types.HistoryEvent{{ID: 1}}},
			},
			historyErrs: []error{nil},
			wfExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State:       closedState,
						NextEventID: 2,
					},
				},
			},
			expectedResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: HistoryInvalid,
				Info:          "skipped fix because execution was healthy",
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeHealthy,
					InvariantName:   HistoryInvalid,
				},
			},
		},
		{
			name:      "fix succeeds for corrupted history",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{nil},
			historyErrs:  []error{persistence.ErrCorruptedHistory},
			expectedResult: FixResult{
				FixResultType: FixResultTypeFixed,
				InvariantName: HistoryInvalid,
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   HistoryInvalid,
					Info:            "corrupt history",
					InfoDetails:     persistence.ErrCorruptedHistory.Error(),
				},
			},
		},
		{
			name:      "fix succeeds for event count mismatch",
			execution: getClosedConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{
				{HistoryEvents: []*types.HistoryEvent{{ID: 1}, {ID: 2}}},
			},
			historyErrs: []error{nil},
			wfExecResp: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						State:       closedState,
						NextEventID: 10,
					},
				},
			},
			expectedResult: FixResult{
				FixResultType: FixResultTypeFixed,
				InvariantName: HistoryInvalid,
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   HistoryInvalid,
					Info:            "event count mismatch",
					InfoDetails:     "history has last event ID 2 but execution record expects 9",
				},
			},
		},
		{
			name:      "fix fails on delete error",
			execution: getOpenConcreteExecution(),
			historyResps: []*persistence.ReadHistoryBranchResponse{nil},
			historyErrs:  []error{persistence.ErrCorruptedHistory},
			deleteExecErr: errors.New("delete failed"),
			expectedResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: HistoryInvalid,
				Info:          "failed to delete concrete workflow execution",
				InfoDetails:   "delete failed",
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   HistoryInvalid,
					Info:            "corrupt history",
					InfoDetails:     persistence.ErrCorruptedHistory.Error(),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()

			historyManager := &mocks.HistoryV2Manager{}
			for i := range tc.historyResps {
				historyManager.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(tc.historyResps[i], tc.historyErrs[i]).Once()
			}

			execManager := &mocks.ExecutionManager{}
			execManager.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(tc.wfExecResp, tc.wfExecErr)
			execManager.On("DeleteWorkflowExecution", mock.Anything, mock.Anything).Return(tc.deleteExecErr)
			execManager.On("DeleteCurrentWorkflowExecution", mock.Anything, mock.Anything).Return(tc.deleteCurrentErr)

			inv := NewHistoryInvalid(
				persistence.NewPersistenceRetryer(execManager, historyManager, c.CreatePersistenceRetryPolicy()),
				mockDomainCache,
			)

			result := inv.Fix(context.Background(), tc.execution)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}
