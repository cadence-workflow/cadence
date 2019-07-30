// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/uber/cadence/.gen/go/matching"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/yarpc"
)

// MatchingClient is an autogenerated mock type for the Client type
type MatchingClient struct {
	mock.Mock
}

// AddActivityTask provides a mock function with given fields: ctx, addRequest
func (_m *MatchingClient) AddActivityTask(ctx context.Context, addRequest *matching.AddActivityTaskRequest, opts ...yarpc.CallOption) error {
	ret := _m.Called(ctx, addRequest)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *matching.AddActivityTaskRequest) error); ok {
		r0 = rf(ctx, addRequest)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddDecisionTask provides a mock function with given fields: ctx, addRequest
func (_m *MatchingClient) AddDecisionTask(ctx context.Context, addRequest *matching.AddDecisionTaskRequest, opts ...yarpc.CallOption) error {
	ret := _m.Called(ctx, addRequest)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *matching.AddDecisionTaskRequest) error); ok {
		r0 = rf(ctx, addRequest)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PollForActivityTask provides a mock function with given fields: ctx, pollRequest
func (_m *MatchingClient) PollForActivityTask(ctx context.Context,
	pollRequest *matching.PollForActivityTaskRequest, opts ...yarpc.CallOption) (*shared.PollForActivityTaskResponse, error) {
	ret := _m.Called(ctx, pollRequest)

	var r0 *shared.PollForActivityTaskResponse
	if rf, ok := ret.Get(0).(func(context.Context,
		*matching.PollForActivityTaskRequest) *shared.PollForActivityTaskResponse); ok {
		r0 = rf(ctx, pollRequest)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*shared.PollForActivityTaskResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *matching.PollForActivityTaskRequest) error); ok {
		r1 = rf(ctx, pollRequest)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PollForDecisionTask provides a mock function with given fields: ctx, pollRequest
func (_m *MatchingClient) PollForDecisionTask(ctx context.Context,
	pollRequest *matching.PollForDecisionTaskRequest, opts ...yarpc.CallOption) (*matching.PollForDecisionTaskResponse, error) {
	ret := _m.Called(ctx, pollRequest)

	var r0 *matching.PollForDecisionTaskResponse
	if rf, ok := ret.Get(0).(func(context.Context, *matching.PollForDecisionTaskRequest) *matching.PollForDecisionTaskResponse); ok {
		r0 = rf(ctx, pollRequest)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*matching.PollForDecisionTaskResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *matching.PollForDecisionTaskRequest) error); ok {
		r1 = rf(ctx, pollRequest)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// QueryWorkflow provides a mock function with given fields: ctx, queryRequest
func (_m *MatchingClient) QueryWorkflow(ctx context.Context,
	queryRequest *matching.QueryWorkflowRequest, opts ...yarpc.CallOption) (*matching.QueryWorkflowResponse, error) {
	ret := _m.Called(ctx, queryRequest)

	var r0 *matching.QueryWorkflowResponse
	if rf, ok := ret.Get(0).(func(context.Context, *matching.QueryWorkflowRequest) *matching.QueryWorkflowResponse); ok {
		r0 = rf(ctx, queryRequest)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*matching.QueryWorkflowResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *matching.QueryWorkflowRequest) error); ok {
		r1 = rf(ctx, queryRequest)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RespondQueryTaskCompleted provides a mock function with given fields: ctx, request
func (_m *MatchingClient) RespondQueryTaskCompleted(ctx context.Context,
	request *matching.RespondQueryTaskCompletedRequest, opts ...yarpc.CallOption) error {
	ret := _m.Called(ctx, request)

	var r0 error
	if rf, ok := ret.Get(1).(func(context.Context, *matching.RespondQueryTaskCompletedRequest) error); ok {
		r0 = rf(ctx, request)
	} else {
		r0 = ret.Error(1)
	}

	return r0
}

// CancelOutstandingPoll provides a mock function with given fields: ctx, request
func (_m *MatchingClient) CancelOutstandingPoll(ctx context.Context,
	request *matching.CancelOutstandingPollRequest, opts ...yarpc.CallOption) error {
	ret := _m.Called(ctx, request)

	var r0 error
	if rf, ok := ret.Get(1).(func(context.Context, *matching.CancelOutstandingPollRequest) error); ok {
		r0 = rf(ctx, request)
	} else {
		r0 = ret.Error(1)
	}

	return r0
}

// DescribeTaskList provides a mock function with given fields: ctx, request
func (_m *MatchingClient) DescribeTaskList(ctx context.Context,
	request *matching.DescribeTaskListRequest, opts ...yarpc.CallOption) (*shared.DescribeTaskListResponse, error) {
	ret := _m.Called(ctx, request)

	var r0 *shared.DescribeTaskListResponse
	if rf, ok := ret.Get(0).(func(context.Context, *matching.DescribeTaskListRequest) *shared.DescribeTaskListResponse); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*shared.DescribeTaskListResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *matching.DescribeTaskListRequest) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
