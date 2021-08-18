// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Code generated by mockery v2.6.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	persistence "github.com/uber/cadence/common/persistence"
)

// TaskManager is an autogenerated mock type for the TaskManager type
type TaskManager struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *TaskManager) Close() {
	_m.Called()
}

// CompleteTask provides a mock function with given fields: ctx, request
func (_m *TaskManager) CompleteTask(ctx context.Context, request *persistence.CompleteTaskRequest) error {
	ret := _m.Called(ctx, request)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.CompleteTaskRequest) error); ok {
		r0 = rf(ctx, request)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CompleteTasksLessThan provides a mock function with given fields: ctx, request
func (_m *TaskManager) CompleteTasksLessThan(ctx context.Context, request *persistence.CompleteTasksLessThanRequest) (int, error) {
	ret := _m.Called(ctx, request)

	var r0 int
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.CompleteTasksLessThanRequest) int); ok {
		r0 = rf(ctx, request)
	} else {
		r0 = ret.Get(0).(int)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *persistence.CompleteTasksLessThanRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateTasks provides a mock function with given fields: ctx, request
func (_m *TaskManager) CreateTasks(ctx context.Context, request *persistence.CreateTasksRequest) (*persistence.CreateTasksResponse, error) {
	ret := _m.Called(ctx, request)

	var r0 *persistence.CreateTasksResponse
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.CreateTasksRequest) *persistence.CreateTasksResponse); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*persistence.CreateTasksResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *persistence.CreateTasksRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteTaskList provides a mock function with given fields: ctx, request
func (_m *TaskManager) DeleteTaskList(ctx context.Context, request *persistence.DeleteTaskListRequest) error {
	ret := _m.Called(ctx, request)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.DeleteTaskListRequest) error); ok {
		r0 = rf(ctx, request)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetName provides a mock function with given fields:
func (_m *TaskManager) GetName() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetOrphanTasks provides a mock function with given fields: ctx, request
func (_m *TaskManager) GetOrphanTasks(ctx context.Context, request *persistence.GetOrphanTasksRequest) (*persistence.GetOrphanTasksResponse, error) {
	ret := _m.Called(ctx, request)

	var r0 *persistence.GetOrphanTasksResponse
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.GetOrphanTasksRequest) *persistence.GetOrphanTasksResponse); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*persistence.GetOrphanTasksResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *persistence.GetOrphanTasksRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTasks provides a mock function with given fields: ctx, request
func (_m *TaskManager) GetTasks(ctx context.Context, request *persistence.GetTasksRequest) (*persistence.GetTasksResponse, error) {
	ret := _m.Called(ctx, request)

	var r0 *persistence.GetTasksResponse
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.GetTasksRequest) *persistence.GetTasksResponse); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*persistence.GetTasksResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *persistence.GetTasksRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LeaseTaskList provides a mock function with given fields: ctx, request
func (_m *TaskManager) LeaseTaskList(ctx context.Context, request *persistence.LeaseTaskListRequest) (*persistence.LeaseTaskListResponse, error) {
	ret := _m.Called(ctx, request)

	var r0 *persistence.LeaseTaskListResponse
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.LeaseTaskListRequest) *persistence.LeaseTaskListResponse); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*persistence.LeaseTaskListResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *persistence.LeaseTaskListRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTaskList provides a mock function with given fields: ctx, request
func (_m *TaskManager) ListTaskList(ctx context.Context, request *persistence.ListTaskListRequest) (*persistence.ListTaskListResponse, error) {
	ret := _m.Called(ctx, request)

	var r0 *persistence.ListTaskListResponse
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.ListTaskListRequest) *persistence.ListTaskListResponse); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*persistence.ListTaskListResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *persistence.ListTaskListRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateTaskList provides a mock function with given fields: ctx, request
func (_m *TaskManager) UpdateTaskList(ctx context.Context, request *persistence.UpdateTaskListRequest) (*persistence.UpdateTaskListResponse, error) {
	ret := _m.Called(ctx, request)

	var r0 *persistence.UpdateTaskListResponse
	if rf, ok := ret.Get(0).(func(context.Context, *persistence.UpdateTaskListRequest) *persistence.UpdateTaskListResponse); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*persistence.UpdateTaskListResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *persistence.UpdateTaskListRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
