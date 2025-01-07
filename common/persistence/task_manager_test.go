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

package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func setUpMocksForTaskManager(t *testing.T) (*taskManager, *MockTaskStore) {
	ctrl := gomock.NewController(t)
	mockTaskStore := NewMockTaskStore(ctrl)

	taskManager := NewTaskManager(mockTaskStore).(*taskManager)

	return taskManager, mockTaskStore
}

func TestTaskManager_GetName(t *testing.T) {
	t.Run("get name", func(t *testing.T) {
		taskManager, mockTaskStore := setUpMocksForTaskManager(t)

		// Set up expectation
		mockTaskStore.EXPECT().GetName().Return("mock-task-store").Times(1)

		// Call the method
		name := taskManager.GetName()

		// Validate the result
		assert.Equal(t, "mock-task-store", name)
	})
}

func TestTaskManager_Close(t *testing.T) {
	t.Run("close task store", func(t *testing.T) {
		taskManager, mockTaskStore := setUpMocksForTaskManager(t)

		// Set up expectation
		mockTaskStore.EXPECT().Close().Times(1)

		// Call the method
		taskManager.Close()

		// No need to assert, just ensuring method is called
	})
}

func TestTaskManager_LeaseTaskList(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *LeaseTaskListRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					LeaseTaskList(gomock.Any(), gomock.Any()).
					Return(&LeaseTaskListResponse{}, nil).Times(1)
			},
			request:     &LeaseTaskListRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					LeaseTaskList(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &LeaseTaskListRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.LeaseTaskList(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_GetTaskList(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *GetTaskListRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetTaskList(gomock.Any(), gomock.Any()).
					Return(&GetTaskListResponse{}, nil).Times(1)
			},
			request:     &GetTaskListRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetTaskList(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &GetTaskListRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.GetTaskList(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_UpdateTaskList(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *UpdateTaskListRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					UpdateTaskList(gomock.Any(), gomock.Any()).
					Return(&UpdateTaskListResponse{}, nil).Times(1)
			},
			request:     &UpdateTaskListRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					UpdateTaskList(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &UpdateTaskListRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.UpdateTaskList(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_ListTaskList(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *ListTaskListRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					ListTaskList(gomock.Any(), gomock.Any()).
					Return(&ListTaskListResponse{}, nil).Times(1)
			},
			request:     &ListTaskListRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					ListTaskList(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &ListTaskListRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.ListTaskList(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_DeleteTaskList(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *DeleteTaskListRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					DeleteTaskList(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
			request:     &DeleteTaskListRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					DeleteTaskList(gomock.Any(), gomock.Any()).
					Return(errors.New("persistence error")).Times(1)
			},
			request:       &DeleteTaskListRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			err := taskManager.DeleteTaskList(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_GetTaskListSize(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *GetTaskListSizeRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetTaskListSize(gomock.Any(), gomock.Any()).
					Return(&GetTaskListSizeResponse{}, nil).Times(1)
			},
			request:     &GetTaskListSizeRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetTaskListSize(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &GetTaskListSizeRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.GetTaskListSize(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_CreateTasks(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *CreateTasksRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					CreateTasks(gomock.Any(), gomock.Any()).
					Return(&CreateTasksResponse{}, nil).Times(1)
			},
			request:     &CreateTasksRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					CreateTasks(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &CreateTasksRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.CreateTasks(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_GetTasks(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *GetTasksRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetTasks(gomock.Any(), gomock.Any()).
					Return(&GetTasksResponse{}, nil).Times(1)
			},
			request:     &GetTasksRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetTasks(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &GetTasksRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.GetTasks(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_CompleteTask(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *CompleteTaskRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					CompleteTask(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
			request:     &CompleteTaskRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					CompleteTask(gomock.Any(), gomock.Any()).
					Return(errors.New("persistence error")).Times(1)
			},
			request:       &CompleteTaskRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			err := taskManager.CompleteTask(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_CompleteTasksLessThan(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *CompleteTasksLessThanRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					CompleteTasksLessThan(gomock.Any(), gomock.Any()).
					Return(&CompleteTasksLessThanResponse{}, nil).Times(1)
			},
			request:     &CompleteTasksLessThanRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					CompleteTasksLessThan(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &CompleteTasksLessThanRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.CompleteTasksLessThan(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskManager_GetOrphanTasks(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockTaskStore)
		request       *GetOrphanTasksRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetOrphanTasks(gomock.Any(), gomock.Any()).
					Return(&GetOrphanTasksResponse{}, nil).Times(1)
			},
			request:     &GetOrphanTasksRequest{},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockTaskStore *MockTaskStore) {
				mockTaskStore.EXPECT().
					GetOrphanTasks(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &GetOrphanTasksRequest{},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskManager, mockTaskStore := setUpMocksForTaskManager(t)

			tc.setupMock(mockTaskStore)

			// Call the method
			_, err := taskManager.GetOrphanTasks(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
