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

package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/types"
)

func TestPersistenceRetryerListConcreteExecutions(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *ListConcreteExecutionsRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedResponse               *ListConcreteExecutionsResponse
		expectedError                  error
	}{
		"Success": {
			request:              &ListConcreteExecutionsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Eq(&ListConcreteExecutionsRequest{})).Return(&ListConcreteExecutionsResponse{}, nil)
			},
			expectedResponse: &ListConcreteExecutionsResponse{},
			expectedError:    nil,
		},
		"Transient Error": {
			request:              &ListConcreteExecutionsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Eq(&ListConcreteExecutionsRequest{})).Return(nil, &types.InternalServiceError{}),
					mockExecutionManager.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Eq(&ListConcreteExecutionsRequest{})).Return(&ListConcreteExecutionsResponse{}, nil),
				)
			},
			expectedResponse: &ListConcreteExecutionsResponse{},
			expectedError:    nil,
		},
		"Fatal Error": {
			request:              &ListConcreteExecutionsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Eq(&ListConcreteExecutionsRequest{})).Return(nil, &types.AccessDeniedError{}).Times(1)
			},
			expectedResponse: nil,
			expectedError:    &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			resp, err := retryer.ListConcreteExecutions(context.Background(), test.request)
			assert.Equal(t, test.expectedResponse, resp)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestPersistenceRetryerGetWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *GetWorkflowExecutionRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedResponse               *GetWorkflowExecutionResponse
		expectedError                  error
	}{
		"Success": {
			request:              &GetWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Eq(&GetWorkflowExecutionRequest{})).Return(&GetWorkflowExecutionResponse{}, nil)
			},
			expectedResponse: &GetWorkflowExecutionResponse{},
			expectedError:    nil,
		},
		"Transient Error": {
			request:              &GetWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Eq(&GetWorkflowExecutionRequest{})).Return(nil, &types.InternalServiceError{}),
					mockExecutionManager.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Eq(&GetWorkflowExecutionRequest{})).Return(&GetWorkflowExecutionResponse{}, nil),
				)
			},
			expectedResponse: &GetWorkflowExecutionResponse{},
			expectedError:    nil,
		},
		"Fatal Error": {
			request:              &GetWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Eq(&GetWorkflowExecutionRequest{})).Return(nil, &types.AccessDeniedError{}).Times(1)
			},
			expectedResponse: nil,
			expectedError:    &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			resp, err := retryer.GetWorkflowExecution(context.Background(), test.request)
			assert.Equal(t, test.expectedResponse, resp)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestPersistenceRetryerGetCurrentExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *GetCurrentExecutionRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedResponse               *GetCurrentExecutionResponse
		expectedError                  error
	}{
		"Success": {
			request:              &GetCurrentExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Eq(&GetCurrentExecutionRequest{})).Return(&GetCurrentExecutionResponse{}, nil)
			},
			expectedResponse: &GetCurrentExecutionResponse{},
			expectedError:    nil,
		},
		"Transient Error": {
			request:              &GetCurrentExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Eq(&GetCurrentExecutionRequest{})).Return(nil, &types.InternalServiceError{}),
					mockExecutionManager.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Eq(&GetCurrentExecutionRequest{})).Return(&GetCurrentExecutionResponse{}, nil),
				)
			},
			expectedResponse: &GetCurrentExecutionResponse{},
			expectedError:    nil,
		},
		"Fatal Error": {
			request:              &GetCurrentExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Eq(&GetCurrentExecutionRequest{})).Return(nil, &types.AccessDeniedError{}).Times(1)
			},
			expectedResponse: nil,
			expectedError:    &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			resp, err := retryer.GetCurrentExecution(context.Background(), test.request)
			assert.Equal(t, test.expectedResponse, resp)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestPersistenceRetryerListCurrentExecutions(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *ListCurrentExecutionsRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedResponse               *ListCurrentExecutionsResponse
		expectedError                  error
	}{
		"Success": {
			request:              &ListCurrentExecutionsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Eq(&ListCurrentExecutionsRequest{})).Return(&ListCurrentExecutionsResponse{}, nil)
			},
			expectedResponse: &ListCurrentExecutionsResponse{},
			expectedError:    nil,
		},
		"Transient Error": {
			request:              &ListCurrentExecutionsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Eq(&ListCurrentExecutionsRequest{})).Return(nil, &types.InternalServiceError{}),
					mockExecutionManager.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Eq(&ListCurrentExecutionsRequest{})).Return(&ListCurrentExecutionsResponse{}, nil),
				)
			},
			expectedResponse: &ListCurrentExecutionsResponse{},
			expectedError:    nil,
		},
		"Fatal Error": {
			request:              &ListCurrentExecutionsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Eq(&ListCurrentExecutionsRequest{})).Return(nil, &types.AccessDeniedError{}).Times(1)
			},
			expectedResponse: nil,
			expectedError:    &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			resp, err := retryer.ListCurrentExecutions(context.Background(), test.request)
			assert.Equal(t, test.expectedResponse, resp)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestPersistenceRetryerIsWorkflowExecutionExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *IsWorkflowExecutionExistsRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedResponse               *IsWorkflowExecutionExistsResponse
		expectedError                  error
	}{
		"Success": {
			request:              &IsWorkflowExecutionExistsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Eq(&IsWorkflowExecutionExistsRequest{})).Return(&IsWorkflowExecutionExistsResponse{}, nil)
			},
			expectedResponse: &IsWorkflowExecutionExistsResponse{},
			expectedError:    nil,
		},
		"Transient Error": {
			request:              &IsWorkflowExecutionExistsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Eq(&IsWorkflowExecutionExistsRequest{})).Return(nil, &types.InternalServiceError{}),
					mockExecutionManager.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Eq(&IsWorkflowExecutionExistsRequest{})).Return(&IsWorkflowExecutionExistsResponse{}, nil),
				)
			},
			expectedResponse: &IsWorkflowExecutionExistsResponse{},
			expectedError:    nil,
		},
		"Fatal Error": {
			request:              &IsWorkflowExecutionExistsRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Eq(&IsWorkflowExecutionExistsRequest{})).Return(nil, &types.AccessDeniedError{}).Times(1)
			},
			expectedResponse: nil,
			expectedError:    &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			resp, err := retryer.IsWorkflowExecutionExists(context.Background(), test.request)
			assert.Equal(t, test.expectedResponse, resp)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestPersistenceRetryerReadHistoryBranch(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                      *ReadHistoryBranchRequest
		mockHistoryManager           *MockHistoryManager
		mockHistoryManagerAccordance func(mockHistoryManager *MockHistoryManager)
		expectedResponse             *ReadHistoryBranchResponse
		expectedError                error
	}{
		"Success": {
			request:            &ReadHistoryBranchRequest{},
			mockHistoryManager: NewMockHistoryManager(ctrl),
			mockHistoryManagerAccordance: func(mockHistoryManager *MockHistoryManager) {
				mockHistoryManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Eq(&ReadHistoryBranchRequest{})).Return(&ReadHistoryBranchResponse{}, nil)
			},
			expectedResponse: &ReadHistoryBranchResponse{},
			expectedError:    nil,
		},
		"Transient Error": {
			request:            &ReadHistoryBranchRequest{},
			mockHistoryManager: NewMockHistoryManager(ctrl),
			mockHistoryManagerAccordance: func(mockHistoryManager *MockHistoryManager) {
				gomock.InOrder(
					mockHistoryManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Eq(&ReadHistoryBranchRequest{})).Return(nil, &types.InternalServiceError{}),
					mockHistoryManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Eq(&ReadHistoryBranchRequest{})).Return(&ReadHistoryBranchResponse{}, nil),
				)
			},
			expectedResponse: &ReadHistoryBranchResponse{},
			expectedError:    nil,
		},
		"Fatal Error": {
			request:            &ReadHistoryBranchRequest{},
			mockHistoryManager: NewMockHistoryManager(ctrl),
			mockHistoryManagerAccordance: func(mockHistoryManager *MockHistoryManager) {
				mockHistoryManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Eq(&ReadHistoryBranchRequest{})).Return(nil, &types.AccessDeniedError{}).Times(1)
			},
			expectedResponse: nil,
			expectedError:    &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockHistoryManager != nil {
				test.mockHistoryManagerAccordance(test.mockHistoryManager)
			}
			retryer := NewPersistenceRetryer(NewMockExecutionManager(ctrl), test.mockHistoryManager, backoff.NewExponentialRetryPolicy(time.Nanosecond))

			resp, err := retryer.ReadHistoryBranch(context.Background(), test.request)
			assert.Equal(t, test.expectedResponse, resp)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestPersistenceRetryerDeleteWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *DeleteWorkflowExecutionRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedError                  error
	}{
		"Success": {
			request:              &DeleteWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteWorkflowExecutionRequest{})).Return(nil)
			},
			expectedError: nil,
		},
		"Transient Error": {
			request:              &DeleteWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteWorkflowExecutionRequest{})).Return(&types.InternalServiceError{}),
					mockExecutionManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteWorkflowExecutionRequest{})).Return(nil),
				)
			},
			expectedError: nil,
		},
		"Fatal Error": {
			request:              &DeleteWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteWorkflowExecutionRequest{})).Return(&types.AccessDeniedError{}).Times(1)
			},
			expectedError: &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			err := retryer.DeleteWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Equal(t, test.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPersistenceRetryerDeleteCurrentWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *DeleteCurrentWorkflowExecutionRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedError                  error
	}{
		"Success": {
			request:              &DeleteCurrentWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteCurrentWorkflowExecutionRequest{})).Return(nil)
			},
			expectedError: nil,
		},
		"Transient Error": {
			request:              &DeleteCurrentWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteCurrentWorkflowExecutionRequest{})).Return(&types.InternalServiceError{}),
					mockExecutionManager.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteCurrentWorkflowExecutionRequest{})).Return(nil),
				)
			},
			expectedError: nil,
		},
		"Fatal Error": {
			request:              &DeleteCurrentWorkflowExecutionRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Eq(&DeleteCurrentWorkflowExecutionRequest{})).Return(&types.AccessDeniedError{}).Times(1)
			},
			expectedError: &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			err := retryer.DeleteCurrentWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Equal(t, test.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPersistenceRetryerGetShardID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockExecutionManager := NewMockExecutionManager(ctrl)
	mockExecutionManager.EXPECT().GetShardID().Return(42)
	retryer := NewPersistenceRetryer(mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

	assert.Equal(t, 42, retryer.GetShardID())
}

func TestPersistenceRetryerGetHistoryTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *GetHistoryTasksRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedResponse               *GetHistoryTasksResponse
		expectedError                  error
	}{
		"Success": {
			request:              &GetHistoryTasksRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Eq(&GetHistoryTasksRequest{})).Return(&GetHistoryTasksResponse{}, nil)
			},
			expectedResponse: &GetHistoryTasksResponse{},
			expectedError:    nil,
		},
		"Transient Error": {
			request:              &GetHistoryTasksRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Eq(&GetHistoryTasksRequest{})).Return(nil, &types.InternalServiceError{}),
					mockExecutionManager.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Eq(&GetHistoryTasksRequest{})).Return(&GetHistoryTasksResponse{}, nil),
				)
			},
			expectedResponse: &GetHistoryTasksResponse{},
			expectedError:    nil,
		},
		"Fatal Error": {
			request:              &GetHistoryTasksRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Eq(&GetHistoryTasksRequest{})).Return(nil, &types.AccessDeniedError{}).Times(1)
			},
			expectedResponse: nil,
			expectedError:    &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			resp, err := retryer.GetHistoryTasks(context.Background(), test.request)
			assert.Equal(t, test.expectedResponse, resp)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestPersistenceRetryerCompleteHistoryTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request                        *CompleteHistoryTaskRequest
		mockExecutionManager           *MockExecutionManager
		mockExecutionManagerAccordance func(mockExecutionManager *MockExecutionManager)
		expectedError                  error
	}{
		"Success": {
			request:              &CompleteHistoryTaskRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().CompleteHistoryTask(gomock.Any(), gomock.Eq(&CompleteHistoryTaskRequest{})).Return(nil)
			},
			expectedError: nil,
		},
		"Transient Error": {
			request:              &CompleteHistoryTaskRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				gomock.InOrder(
					mockExecutionManager.EXPECT().CompleteHistoryTask(gomock.Any(), gomock.Eq(&CompleteHistoryTaskRequest{})).Return(&types.InternalServiceError{}),
					mockExecutionManager.EXPECT().CompleteHistoryTask(gomock.Any(), gomock.Eq(&CompleteHistoryTaskRequest{})).Return(nil),
				)
			},
			expectedError: nil,
		},
		"Fatal Error": {
			request:              &CompleteHistoryTaskRequest{},
			mockExecutionManager: NewMockExecutionManager(ctrl),
			mockExecutionManagerAccordance: func(mockExecutionManager *MockExecutionManager) {
				mockExecutionManager.EXPECT().CompleteHistoryTask(gomock.Any(), gomock.Eq(&CompleteHistoryTaskRequest{})).Return(&types.AccessDeniedError{}).Times(1)
			},
			expectedError: &types.AccessDeniedError{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockExecutionManager != nil {
				test.mockExecutionManagerAccordance(test.mockExecutionManager)
			}
			retryer := NewPersistenceRetryer(test.mockExecutionManager, NewMockHistoryManager(ctrl), backoff.NewExponentialRetryPolicy(time.Nanosecond))

			err := retryer.CompleteHistoryTask(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Equal(t, test.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
