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

// Code generated by MockGen. DO NOT EDIT.
// Source: handler.go

// Package matching is a generated GoMock package.
package matching

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	types "github.com/uber/cadence/common/types"
)

// MockHandler is a mock of Handler interface
type MockHandler struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerMockRecorder
}

// MockHandlerMockRecorder is the mock recorder for MockHandler
type MockHandlerMockRecorder struct {
	mock *MockHandler
}

// NewMockHandler creates a new mock instance
func NewMockHandler(ctrl *gomock.Controller) *MockHandler {
	mock := &MockHandler{ctrl: ctrl}
	mock.recorder = &MockHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockHandler) EXPECT() *MockHandlerMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockHandler) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start
func (mr *MockHandlerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockHandler)(nil).Start))
}

// Stop mocks base method
func (m *MockHandler) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop
func (mr *MockHandlerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockHandler)(nil).Stop))
}

// Health mocks base method
func (m *MockHandler) Health(arg0 context.Context) (*types.HealthStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Health", arg0)
	ret0, _ := ret[0].(*types.HealthStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Health indicates an expected call of Health
func (mr *MockHandlerMockRecorder) Health(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Health", reflect.TypeOf((*MockHandler)(nil).Health), arg0)
}

// AddActivityTask mocks base method
func (m *MockHandler) AddActivityTask(arg0 context.Context, arg1 *types.AddActivityTaskRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddActivityTask", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddActivityTask indicates an expected call of AddActivityTask
func (mr *MockHandlerMockRecorder) AddActivityTask(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddActivityTask", reflect.TypeOf((*MockHandler)(nil).AddActivityTask), arg0, arg1)
}

// AddDecisionTask mocks base method
func (m *MockHandler) AddDecisionTask(arg0 context.Context, arg1 *types.AddDecisionTaskRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddDecisionTask", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddDecisionTask indicates an expected call of AddDecisionTask
func (mr *MockHandlerMockRecorder) AddDecisionTask(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddDecisionTask", reflect.TypeOf((*MockHandler)(nil).AddDecisionTask), arg0, arg1)
}

// CancelOutstandingPoll mocks base method
func (m *MockHandler) CancelOutstandingPoll(arg0 context.Context, arg1 *types.CancelOutstandingPollRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelOutstandingPoll", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelOutstandingPoll indicates an expected call of CancelOutstandingPoll
func (mr *MockHandlerMockRecorder) CancelOutstandingPoll(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelOutstandingPoll", reflect.TypeOf((*MockHandler)(nil).CancelOutstandingPoll), arg0, arg1)
}

// DescribeTaskList mocks base method
func (m *MockHandler) DescribeTaskList(arg0 context.Context, arg1 *types.MatchingDescribeTaskListRequest) (*types.DescribeTaskListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeTaskList", arg0, arg1)
	ret0, _ := ret[0].(*types.DescribeTaskListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeTaskList indicates an expected call of DescribeTaskList
func (mr *MockHandlerMockRecorder) DescribeTaskList(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeTaskList", reflect.TypeOf((*MockHandler)(nil).DescribeTaskList), arg0, arg1)
}

// ListTaskListPartitions mocks base method
func (m *MockHandler) ListTaskListPartitions(arg0 context.Context, arg1 *types.MatchingListTaskListPartitionsRequest) (*types.ListTaskListPartitionsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTaskListPartitions", arg0, arg1)
	ret0, _ := ret[0].(*types.ListTaskListPartitionsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTaskListPartitions indicates an expected call of ListTaskListPartitions
func (mr *MockHandlerMockRecorder) ListTaskListPartitions(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTaskListPartitions", reflect.TypeOf((*MockHandler)(nil).ListTaskListPartitions), arg0, arg1)
}

// GetTaskListsByDomain mocks base method
func (m *MockHandler) GetTaskListsByDomain(arg0 context.Context, arg1 *types.GetTaskListsByDomainRequest) (*types.GetTaskListsByDomainResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTaskListsByDomain", arg0, arg1)
	ret0, _ := ret[0].(*types.GetTaskListsByDomainResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTaskListsByDomain indicates an expected call of GetTaskListsByDomain
func (mr *MockHandlerMockRecorder) GetTaskListsByDomain(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTaskListsByDomain", reflect.TypeOf((*MockHandler)(nil).GetTaskListsByDomain), arg0, arg1)
}

// PollForActivityTask mocks base method
func (m *MockHandler) PollForActivityTask(arg0 context.Context, arg1 *types.MatchingPollForActivityTaskRequest) (*types.PollForActivityTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollForActivityTask", arg0, arg1)
	ret0, _ := ret[0].(*types.PollForActivityTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollForActivityTask indicates an expected call of PollForActivityTask
func (mr *MockHandlerMockRecorder) PollForActivityTask(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollForActivityTask", reflect.TypeOf((*MockHandler)(nil).PollForActivityTask), arg0, arg1)
}

// PollForDecisionTask mocks base method
func (m *MockHandler) PollForDecisionTask(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest) (*types.MatchingPollForDecisionTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollForDecisionTask", arg0, arg1)
	ret0, _ := ret[0].(*types.MatchingPollForDecisionTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollForDecisionTask indicates an expected call of PollForDecisionTask
func (mr *MockHandlerMockRecorder) PollForDecisionTask(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollForDecisionTask", reflect.TypeOf((*MockHandler)(nil).PollForDecisionTask), arg0, arg1)
}

// QueryWorkflow mocks base method
func (m *MockHandler) QueryWorkflow(arg0 context.Context, arg1 *types.MatchingQueryWorkflowRequest) (*types.QueryWorkflowResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryWorkflow", arg0, arg1)
	ret0, _ := ret[0].(*types.QueryWorkflowResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryWorkflow indicates an expected call of QueryWorkflow
func (mr *MockHandlerMockRecorder) QueryWorkflow(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryWorkflow", reflect.TypeOf((*MockHandler)(nil).QueryWorkflow), arg0, arg1)
}

// RespondQueryTaskCompleted mocks base method
func (m *MockHandler) RespondQueryTaskCompleted(arg0 context.Context, arg1 *types.MatchingRespondQueryTaskCompletedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondQueryTaskCompleted", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RespondQueryTaskCompleted indicates an expected call of RespondQueryTaskCompleted
func (mr *MockHandlerMockRecorder) RespondQueryTaskCompleted(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondQueryTaskCompleted", reflect.TypeOf((*MockHandler)(nil).RespondQueryTaskCompleted), arg0, arg1)
}
