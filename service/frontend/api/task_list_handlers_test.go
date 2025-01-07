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
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

func TestDescribeTaskList(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.DescribeTaskListRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
		expected      *types.DescribeTaskListResponse
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
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeTaskListRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("domain-id", nil)
				deps.mockMatchingClient.EXPECT().DescribeTaskList(gomock.Any(), &types.MatchingDescribeTaskListRequest{
					DomainUUID: "domain-id",
					DescRequest: &types.DescribeTaskListRequest{
						Domain: "domain",
						TaskList: &types.TaskList{
							Name: "tl",
						},
						TaskListType: types.TaskListTypeActivity.Ptr(),
					},
				}).Return(&types.DescribeTaskListResponse{}, nil)
			},
			expectError: false,
			expected:    &types.DescribeTaskListResponse{},
		},
		{
			name: "matching client error",
			req: &types.DescribeTaskListRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeTaskListRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("domain-id", nil)
				deps.mockMatchingClient.EXPECT().DescribeTaskList(gomock.Any(), &types.MatchingDescribeTaskListRequest{
					DomainUUID: "domain-id",
					DescRequest: &types.DescribeTaskListRequest{
						Domain: "domain",
						TaskList: &types.TaskList{
							Name: "tl",
						},
						TaskListType: types.TaskListTypeActivity.Ptr(),
					},
				}).Return(nil, errors.New("matching client error"))
			},
			expectError:   true,
			expectedError: "matching client error",
		},
		{
			name: "domain cache error",
			req: &types.DescribeTaskListRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeTaskListRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("", errors.New("domain cache error"))
			},
			expectError:   true,
			expectedError: "domain cache error",
		},
		{
			name: "validator error",
			req: &types.DescribeTaskListRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeTaskListRequest(gomock.Any(), gomock.Any()).Return(errors.New("validator error"))
			},
			expectError:   true,
			expectedError: "validator error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)

			resp, err := wh.DescribeTaskList(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestListTaskListPartitions(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ListTaskListPartitionsRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
		expected      *types.ListTaskListPartitionsResponse
	}{
		{
			name: "success",
			req: &types.ListTaskListPartitionsRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateListTaskListPartitionsRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockMatchingClient.EXPECT().ListTaskListPartitions(gomock.Any(), &types.MatchingListTaskListPartitionsRequest{
					Domain: "domain",
					TaskList: &types.TaskList{
						Name: "tl",
					},
				}).Return(&types.ListTaskListPartitionsResponse{}, nil)
			},
			expectError: false,
			expected:    &types.ListTaskListPartitionsResponse{},
		},
		{
			name: "matching client error",
			req: &types.ListTaskListPartitionsRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateListTaskListPartitionsRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockMatchingClient.EXPECT().ListTaskListPartitions(gomock.Any(), &types.MatchingListTaskListPartitionsRequest{
					Domain: "domain",
					TaskList: &types.TaskList{
						Name: "tl",
					},
				}).Return(nil, errors.New("matching client error"))
			},
			expectError:   true,
			expectedError: "matching client error",
		},
		{
			name: "validator error",
			req: &types.ListTaskListPartitionsRequest{
				Domain: "domain",
				TaskList: &types.TaskList{
					Name: "tl",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateListTaskListPartitionsRequest(gomock.Any(), gomock.Any()).Return(errors.New("validator error"))
			},
			expectError:   true,
			expectedError: "validator error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)

			resp, err := wh.ListTaskListPartitions(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestGetTaskListsByDomain(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.GetTaskListsByDomainRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
		expected      *types.GetTaskListsByDomainResponse
	}{
		{
			name: "success",
			req: &types.GetTaskListsByDomainRequest{
				Domain: "domain",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateGetTaskListsByDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockMatchingClient.EXPECT().GetTaskListsByDomain(gomock.Any(), &types.GetTaskListsByDomainRequest{
					Domain: "domain",
				}).Return(&types.GetTaskListsByDomainResponse{}, nil)
			},
			expectError: false,
			expected:    &types.GetTaskListsByDomainResponse{},
		},
		{
			name: "matching client error",
			req: &types.GetTaskListsByDomainRequest{
				Domain: "domain",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateGetTaskListsByDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockMatchingClient.EXPECT().GetTaskListsByDomain(gomock.Any(), &types.GetTaskListsByDomainRequest{
					Domain: "domain",
				}).Return(nil, errors.New("matching client error"))
			},
			expectError:   true,
			expectedError: "matching client error",
		},
		{
			name: "validator error",
			req: &types.GetTaskListsByDomainRequest{
				Domain: "domain",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateGetTaskListsByDomainRequest(gomock.Any(), gomock.Any()).Return(errors.New("validator error"))
			},
			expectError:   true,
			expectedError: "validator error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)

			resp, err := wh.GetTaskListsByDomain(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestResetStickyTaskList(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.ResetStickyTaskListRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
		expected      *types.ResetStickyTaskListResponse
	}{
		{
			name: "success",
			req: &types.ResetStickyTaskListRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateResetStickyTaskListRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("domain-id", nil)
				deps.mockHistoryClient.EXPECT().ResetStickyTaskList(gomock.Any(), &types.HistoryResetStickyTaskListRequest{
					DomainUUID: "domain-id",
					Execution: &types.WorkflowExecution{
						WorkflowID: "wf",
					},
				}).Return(&types.HistoryResetStickyTaskListResponse{}, nil)
			},
			expectError: false,
			expected:    &types.ResetStickyTaskListResponse{},
		},
		{
			name: "history client error",
			req: &types.ResetStickyTaskListRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateResetStickyTaskListRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("domain-id", nil)
				deps.mockHistoryClient.EXPECT().ResetStickyTaskList(gomock.Any(), &types.HistoryResetStickyTaskListRequest{
					DomainUUID: "domain-id",
					Execution: &types.WorkflowExecution{
						WorkflowID: "wf",
					},
				}).Return(nil, errors.New("history client error"))
			},
			expectError:   true,
			expectedError: "history client error",
		},
		{
			name: "domain cache error",
			req: &types.ResetStickyTaskListRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateResetStickyTaskListRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("", errors.New("domain cache error"))
			},
			expectError:   true,
			expectedError: "domain cache error",
		},
		{
			name: "validator error",
			req: &types.ResetStickyTaskListRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateResetStickyTaskListRequest(gomock.Any(), gomock.Any()).Return(errors.New("validator error"))
			},
			expectError:   true,
			expectedError: "validator error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)

			resp, err := wh.ResetStickyTaskList(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}
