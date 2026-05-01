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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
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
		true,
		"hostname",
		logger,
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
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskListNameMaxLength, 5))

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
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskListNameMaxLength, 5))

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
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskListNameMaxLength, 5))

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
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskListNameMaxLength, 5))

			err := v.ValidateResetStickyTaskListRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCountWorkflowExecutionsRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.CountWorkflowExecutionsRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.CountWorkflowExecutionsRequest{
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
			req: &types.CountWorkflowExecutionsRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, _ := setupMocksForRequestValidator(t)

			err := v.ValidateCountWorkflowExecutionsRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateListWorkflowExecutionsRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ListWorkflowExecutionsRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.ListWorkflowExecutionsRequest{
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
			req: &types.ListWorkflowExecutionsRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "page size too large",
			req: &types.ListWorkflowExecutionsRequest{
				Domain:   "domain",
				PageSize: 101,
			},
			expectError:   true,
			expectedError: "Pagesize is larger than allow 100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendESIndexMaxResultWindow, 100))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendVisibilityMaxPageSize, 10))

			err := v.ValidateListWorkflowExecutionsRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int32(10), tc.req.GetPageSize())
			}
		})
	}
}

func TestValidateListOpenWorkflowExecutionsRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ListOpenWorkflowExecutionsRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				MaximumPageSize: 0,
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
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "startTimeFilter not set",
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain: "domain",
			},
			expectError:   true,
			expectedError: "StartTimeFilter is required",
		},
		{
			name: "Earliest time not set",
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain:          "domain",
				StartTimeFilter: &types.StartTimeFilter{},
			},
			expectError:   true,
			expectedError: "EarliestTime in StartTimeFilter is required",
		},
		{
			name: "Latest time not set",
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
				},
			},
			expectError:   true,
			expectedError: "LatestTime in StartTimeFilter is required",
		},
		{
			name: "earliest time later than latest time",
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(3)),
					LatestTime:   common.Ptr(int64(2)),
				},
			},
			expectError:   true,
			expectedError: "EarliestTime in StartTimeFilter should not be larger than LatestTime",
		},
		{
			name: "both execution and type filter are specified",
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				ExecutionFilter: &types.WorkflowExecutionFilter{},
				TypeFilter:      &types.WorkflowTypeFilter{},
			},
			expectError:   true,
			expectedError: "Only one of ExecutionFilter or TypeFilter is allowed",
		},
		{
			name: "page size too large",
			req: &types.ListOpenWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				MaximumPageSize: 101,
			},
			expectError:   true,
			expectedError: "Pagesize is larger than allow 100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendESIndexMaxResultWindow, 100))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendVisibilityMaxPageSize, 10))

			err := v.ValidateListOpenWorkflowExecutionsRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int32(10), tc.req.GetMaximumPageSize())
			}
		})
	}
}

func TestValidateListClosedWorkflowExecutionsRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ListClosedWorkflowExecutionsRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				MaximumPageSize: 0,
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
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "startTimeFilter not set",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
			},
			expectError:   true,
			expectedError: "StartTimeFilter is required",
		},
		{
			name: "Earliest time not set",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain:          "domain",
				StartTimeFilter: &types.StartTimeFilter{},
			},
			expectError:   true,
			expectedError: "EarliestTime in StartTimeFilter is required",
		},
		{
			name: "Latest time not set",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
				},
			},
			expectError:   true,
			expectedError: "LatestTime in StartTimeFilter is required",
		},
		{
			name: "earliest time later than latest time",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(3)),
					LatestTime:   common.Ptr(int64(2)),
				},
			},
			expectError:   true,
			expectedError: "EarliestTime in StartTimeFilter should not be larger than LatestTime",
		},
		{
			name: "both execution and type filter are specified",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				ExecutionFilter: &types.WorkflowExecutionFilter{},
				TypeFilter:      &types.WorkflowTypeFilter{},
			},
			expectError:   true,
			expectedError: "Only one of ExecutionFilter, TypeFilter or StatusFilter is allowed",
		},
		{
			name: "both execution and status filter are specified",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				ExecutionFilter: &types.WorkflowExecutionFilter{},
				StatusFilter:    types.WorkflowExecutionCloseStatusFailed.Ptr(),
			},
			expectError:   true,
			expectedError: "Only one of ExecutionFilter, TypeFilter or StatusFilter is allowed",
		},
		{
			name: "both type and status filter are specified",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				TypeFilter:   &types.WorkflowTypeFilter{},
				StatusFilter: types.WorkflowExecutionCloseStatusFailed.Ptr(),
			},
			expectError:   true,
			expectedError: "Only one of ExecutionFilter, TypeFilter or StatusFilter is allowed",
		},
		{
			name: "page size too large",
			req: &types.ListClosedWorkflowExecutionsRequest{
				Domain: "domain",
				StartTimeFilter: &types.StartTimeFilter{
					EarliestTime: common.Ptr(int64(1)),
					LatestTime:   common.Ptr(int64(2)),
				},
				MaximumPageSize: 101,
			},
			expectError:   true,
			expectedError: "Pagesize is larger than allow 100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendESIndexMaxResultWindow, 100))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendVisibilityMaxPageSize, 10))

			err := v.ValidateListClosedWorkflowExecutionsRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int32(10), tc.req.GetMaximumPageSize())
			}
		})
	}
}

func TestValidateListArchivedWorkflowExecutionsRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ListArchivedWorkflowExecutionsRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.ListArchivedWorkflowExecutionsRequest{
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
			req: &types.ListArchivedWorkflowExecutionsRequest{
				Domain: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "page size too large",
			req: &types.ListArchivedWorkflowExecutionsRequest{
				Domain:   "domain",
				PageSize: 101,
			},
			expectError:   true,
			expectedError: "Pagesize is larger than allowed 100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.VisibilityArchivalQueryMaxPageSize, 100))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendVisibilityMaxPageSize, 10))

			err := v.ValidateListArchivedWorkflowExecutionsRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int32(10), tc.req.GetPageSize())
			}
		})
	}
}

func TestValidateDeprecateDomainRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.DeprecateDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.DeprecateDomainRequest{
				Name:          "domain",
				SecurityToken: "token",
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
			req: &types.DeprecateDomainRequest{
				Name: "",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "wrong token",
			req: &types.DeprecateDomainRequest{
				Name:          "domain",
				SecurityToken: "to",
			},
			expectError:   true,
			expectedError: "No permission to do this operation.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.EnableAdminProtection, true))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.AdminOperationToken, "token"))

			err := v.ValidateDeprecateDomainRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDescribeDomainRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.DescribeDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success - with domain name",
			req: &types.DescribeDomainRequest{
				Name: common.Ptr("domain"),
			},
			expectError: false,
		},
		{
			name: "success - with domain id",
			req: &types.DescribeDomainRequest{
				UUID: common.Ptr("id"),
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
			name:          "domain not set",
			req:           &types.DescribeDomainRequest{},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, _ := setupMocksForRequestValidator(t)

			err := v.ValidateDescribeDomainRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUpdateDomainRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.UpdateDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success - non failover",
			req: &types.UpdateDomainRequest{
				Name:          "domain",
				SecurityToken: "token",
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
			name:          "domain not set",
			req:           &types.UpdateDomainRequest{},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "invalid retention",
			req: &types.UpdateDomainRequest{
				Name:                                   "domain",
				WorkflowExecutionRetentionPeriodInDays: common.Ptr(int32(100)),
			},
			expectError:   true,
			expectedError: "RetentionDays is invalid.",
		},
		{
			name: "wrong token",
			req: &types.UpdateDomainRequest{
				Name:          "domain",
				SecurityToken: "to",
			},
			expectError:   true,
			expectedError: "No permission to do this operation.",
		},
		{
			name: "lockdown rejects active-passive failover",
			req: &types.UpdateDomainRequest{
				Name:              "domain",
				ActiveClusterName: common.Ptr("a"),
			},
			expectError:   true,
			expectedError: "Domain is not accepting fail overs at this time due to lockdown.",
		},
		{
			name: "lockdown rejects active-active failover",
			req: &types.UpdateDomainRequest{
				Name: "domain",
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"region0": {
									ActiveClusterName: "cluster0",
									FailoverVersion:   1,
								},
							},
						},
					},
				},
			},
			expectError:   true,
			expectedError: "Domain is not accepting fail overs at this time due to lockdown.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.EnableAdminProtection, true))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.AdminOperationToken, "token"))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.MaxRetentionDays, 3))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.Lockdown, true))

			err := v.ValidateUpdateDomainRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRegisterDomainRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.RegisterDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.RegisterDomainRequest{
				Name:          "domain",
				SecurityToken: "token",
				Data:          map[string]string{"tier": "3"},
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
			name:          "domain not set",
			req:           &types.RegisterDomainRequest{},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "name too long",
			req: &types.RegisterDomainRequest{
				Name: "domain-name-toooooooooooooo-long",
			},
			expectError:   true,
			expectedError: "Domain length exceeds limit.",
		},
		{
			name: "invalid retention",
			req: &types.RegisterDomainRequest{
				Name:                                   "domain",
				WorkflowExecutionRetentionPeriodInDays: 100,
			},
			expectError:   true,
			expectedError: "RetentionDays is invalid.",
		},
		{
			name: "missing data key",
			req: &types.RegisterDomainRequest{
				Name:          "domain",
				SecurityToken: "to",
			},
			expectError:   true,
			expectedError: "domain data error, missing required key tier . All required keys: map[tier:true]",
		},
		{
			name: "wrong token",
			req: &types.RegisterDomainRequest{
				Name:          "domain",
				SecurityToken: "to",
				Data:          map[string]string{"tier": "3"},
			},
			expectError:   true,
			expectedError: "No permission to do this operation.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, deps := setupMocksForRequestValidator(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.EnableAdminProtection, true))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.AdminOperationToken, "token"))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.MaxRetentionDays, 3))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.DomainNameMaxLength, 10))
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.RequiredDomainDataKeys, map[string]interface{}{"tier": true}))

			err := v.ValidateRegisterDomainRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
