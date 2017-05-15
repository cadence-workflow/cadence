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

import "github.com/uber/cadence/.gen/go/matching"
import "github.com/uber/cadence/.gen/go/shared"
import "github.com/stretchr/testify/mock"

import "github.com/uber/tchannel-go/thrift"

// MatchingClient is an autogenerated mock type for the Client type
type MatchingClient struct {
	mock.Mock
}

// AddActivityTask provides a mock function with given fields: ctx, addRequest
func (_m *MatchingClient) AddActivityTask(ctx thrift.Context, addRequest *matching.AddActivityTaskRequest) error {
	ret := _m.Called(ctx, addRequest)

	var r0 error
	if rf, ok := ret.Get(0).(func(thrift.Context, *matching.AddActivityTaskRequest) error); ok {
		r0 = rf(ctx, addRequest)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddDecisionTask provides a mock function with given fields: ctx, addRequest
func (_m *MatchingClient) AddDecisionTask(ctx thrift.Context, addRequest *matching.AddDecisionTaskRequest) error {
	ret := _m.Called(ctx, addRequest)

	var r0 error
	if rf, ok := ret.Get(0).(func(thrift.Context, *matching.AddDecisionTaskRequest) error); ok {
		r0 = rf(ctx, addRequest)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PollForActivityTask provides a mock function with given fields: ctx, pollRequest
func (_m *MatchingClient) PollForActivityTask(ctx thrift.Context,
	pollRequest *matching.PollForActivityTaskRequest) (*shared.PollForActivityTaskResponse, error) {
	ret := _m.Called(ctx, pollRequest)

	var r0 *shared.PollForActivityTaskResponse
	if rf, ok := ret.Get(0).(func(thrift.Context,
		*matching.PollForActivityTaskRequest) *shared.PollForActivityTaskResponse); ok {
		r0 = rf(ctx, pollRequest)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*shared.PollForActivityTaskResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(thrift.Context, *matching.PollForActivityTaskRequest) error); ok {
		r1 = rf(ctx, pollRequest)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PollForDecisionTask provides a mock function with given fields: ctx, pollRequest
func (_m *MatchingClient) PollForDecisionTask(ctx thrift.Context,
	pollRequest *matching.PollForDecisionTaskRequest) (*matching.PollForDecisionTaskResponse, error) {
	ret := _m.Called(ctx, pollRequest)

	var r0 *matching.PollForDecisionTaskResponse
	if rf, ok := ret.Get(0).(func(thrift.Context, *matching.PollForDecisionTaskRequest) *matching.PollForDecisionTaskResponse); ok {
		r0 = rf(ctx, pollRequest)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*matching.PollForDecisionTaskResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(thrift.Context, *matching.PollForDecisionTaskRequest) error); ok {
		r1 = rf(ctx, pollRequest)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
