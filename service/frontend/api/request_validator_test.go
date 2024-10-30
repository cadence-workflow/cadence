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

package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
)

func setupMocksForRequestValidator(t *testing.T) (*requestValidatorImpl, *mockDeps) {
	logger := testlogger.New(t)
	metricsClient := metrics.NewNoopMetricsClient()
	dynamicClient := dynamicconfig.NewInMemoryClient()
	config := frontendcfg.NewConfig(
		dynamicconfig.NewCollection(
			dynamicClient,
			logger,
		),
		numHistoryShards,
		false,
		"hostname",
	)
	deps := &mockDeps{
		dynamicClient: dynamicClient,
	}
	v := NewRequestValidator(logger, metricsClient, config)
	return v.(*requestValidatorImpl), deps
}

func TestValidateRefreshWorkflowTasksRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.RefreshWorkflowTasksRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.RefreshWorkflowTasksRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			expectError: false,
		},
		{
			name:          "not set",
			req:           nil,
			expectError:   true,
			expectedError: "Request is nil.",
		},
		{
			name: "execution not set",
			req: &types.RefreshWorkflowTasksRequest{
				Domain:    "domain",
				Execution: nil,
			},
			expectError:   true,
			expectedError: "Execution is not set on request.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, _ := setupMocksForRequestValidator(t)

			err := v.ValidateRefreshWorkflowTasksRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateValidateDescribeTaskListRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.DescribeTaskListRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.DescribeTaskListRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			expectError: false,
		},
		{
			name:          "not set",
			req:           nil,
			expectError:   true,
			expectedError: "Request is nil.",
		},
		{
			name: "domain not set",
			req: &types.DescribeTaskListRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "task list type not set",
			req: &types.DescribeTaskListRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
			},
			expectError:   true,
			expectedError: "TaskListType is not set on request.",
		},
		{
			name: "task list not set",
			req: &types.DescribeTaskListRequest{
				Domain:       "domain",
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			expectError:   true,
			expectedError: "TaskList is not set on request.",
		},
		{
			name: "task list name not set",
			req: &types.DescribeTaskListRequest{
				Domain:       "domain",
				TaskListType: types.TaskListTypeActivity.Ptr(),
				TaskList:     &types.TaskList{},
			},
			expectError:   true,
			expectedError: "TaskList is not set on request.",
		},
		{
			name: "task list name too long",
			req: &types.DescribeTaskListRequest{
				Domain:       "domain",
				TaskListType: types.TaskListTypeActivity.Ptr(),
				TaskList: &types.TaskList{
					Name: "taskListName",
				},
			},
			expectError:   true,
			expectedError: "TaskList length exceeds limit.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicconfig.TaskListNameMaxLength, 5))

			err := v.ValidateDescribeTaskListRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateValidateListTaskListPartitionsRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ListTaskListPartitionsRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.ListTaskListPartitionsRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
			},
			expectError: false,
		},
		{
			name:          "not set",
			req:           nil,
			expectError:   true,
			expectedError: "Request is nil.",
		},
		{
			name: "domain not set",
			req: &types.ListTaskListPartitionsRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "task list not set",
			req: &types.ListTaskListPartitionsRequest{
				Domain: "domain",
			},
			expectError:   true,
			expectedError: "TaskList is not set on request.",
		},
		{
			name: "task list name not set",
			req: &types.ListTaskListPartitionsRequest{
				Domain:   "domain",
				TaskList: &types.TaskList{},
			},
			expectError:   true,
			expectedError: "TaskList is not set on request.",
		},
		{
			name: "task list name too long",
			req: &types.ListTaskListPartitionsRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "taskListName",
				},
			},
			expectError:   true,
			expectedError: "TaskList length exceeds limit.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicconfig.TaskListNameMaxLength, 5))

			err := v.ValidateListTaskListPartitionsRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateValidateGetTaskListsByDomainRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.GetTaskListsByDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.GetTaskListsByDomainRequest{
				Domain: "domain",
			},
			expectError: false,
		},
		{
			name:          "not set",
			req:           nil,
			expectError:   true,
			expectedError: "Request is nil.",
		},
		{
			name: "domain not set",
			req: &types.GetTaskListsByDomainRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicconfig.TaskListNameMaxLength, 5))

			err := v.ValidateGetTaskListsByDomainRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateValidateResetStickyTaskListRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ResetStickyTaskListRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.ResetStickyTaskListRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wid",
				},
			},
			expectError: false,
		},
		{
			name:          "not set",
			req:           nil,
			expectError:   true,
			expectedError: "Request is nil.",
		},
		{
			name: "domain not set",
			req: &types.ResetStickyTaskListRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "execution not set",
			req: &types.ResetStickyTaskListRequest{
				Domain:    "domain",
				Execution: nil,
			},
			expectError:   true,
			expectedError: "Execution is not set on request.",
		},
		{
			name: "workflowID not set",
			req: &types.ResetStickyTaskListRequest{
				Domain:    "domain",
				Execution: &types.WorkflowExecution{},
			},
			expectError:   true,
			expectedError: "WorkflowId is not set on request.",
		},
		{
			name: "Invalid RunID",
			req: &types.ResetStickyTaskListRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "a",
				},
			},
			expectError:   true,
			expectedError: "Invalid RunId.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicconfig.TaskListNameMaxLength, 5))

			err := v.ValidateResetStickyTaskListRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
