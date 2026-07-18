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

package taskvalidator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/constants"
)

type mockStaleChecker struct {
	CheckAgeFunc func(response *persistence.GetWorkflowExecutionResponse) (bool, error)
}

func (m *mockStaleChecker) CheckAge(response *persistence.GetWorkflowExecutionResponse) (bool, error) {
	return m.CheckAgeFunc(response)
}

func TestWorkflowCheckforValidation(t *testing.T) {
	testCases := []struct {
		name          string
		workflowID    string
		domainID      string
		domainName    string
		runID         string
		isStale       bool
		simulateError bool
	}{
		{"NonStaleWorkflow", "workflow-1", "domain-1", "domain-name-1", "run-1", false, false},
		{"StaleWorkflow", "workflow-2", "domain-2", "domain-name-2", "run-2", true, false},
		{"ErrorInGetWorkflowExecution", "workflow-3", "domain-3", "domain-name-3", "run-3", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockLogger := zap.NewNop()
			mockMetricsClient := metrics.NewNoopMetricsClient()
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockExecutionManager := persistence.NewMockExecutionManager(mockCtrl)
			mockHistoryManager := persistence.NewMockHistoryManager(mockCtrl)

			checker, err := NewWfChecker(mockLogger, mockMetricsClient, mockDomainCache, mockExecutionManager, mockHistoryManager)
			assert.NoError(t, err, "Failed to create checker")

			mockDomainCache.EXPECT().GetDomainByID(tc.domainID).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomainName(tc.domainID).Return(tc.domainName, nil).AnyTimes()

			if tc.isStale {
				mockExecutionManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockExecutionManager.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			}

			mockExecutionManager.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, request *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
				if tc.simulateError {
					return nil, errors.New("database error")
				}
				return &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   constants.TestDomainID,
							WorkflowID: constants.TestWorkflowID,
						},
					},
				}, nil
			}).AnyTimes()

			ctx := context.Background()
			err = checker.WorkflowCheckforValidation(ctx, tc.workflowID, tc.domainID, tc.domainName, tc.runID)

			if tc.simulateError {
				assert.Error(t, err, "Expected error when GetWorkflowExecution fails")
			} else {
				assert.NoError(t, err, "Expected no error for valid workflow execution")
			}
		})
	}
}
