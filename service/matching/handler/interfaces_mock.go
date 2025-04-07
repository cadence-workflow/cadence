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
// Source: interfaces.go
//
// Generated by this command:
//
//	mockgen -package handler -source interfaces.go -destination interfaces_mock.go -package handler github.com/uber/cadence/service/matching/handler Handler
//

// Package handler is a generated GoMock package.
package handler

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"

	types "github.com/uber/cadence/common/types"
)

// MockEngine is a mock of Engine interface.
type MockEngine struct {
	ctrl     *gomock.Controller
	recorder *MockEngineMockRecorder
	isgomock struct{}
}

// MockEngineMockRecorder is the mock recorder for MockEngine.
type MockEngineMockRecorder struct {
	mock *MockEngine
}

// NewMockEngine creates a new mock instance.
func NewMockEngine(ctrl *gomock.Controller) *MockEngine {
	mock := &MockEngine{ctrl: ctrl}
	mock.recorder = &MockEngineMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEngine) EXPECT() *MockEngineMockRecorder {
	return m.recorder
}

// AddActivityTask mocks base method.
func (m *MockEngine) AddActivityTask(hCtx *handlerContext, request *types.AddActivityTaskRequest) (*types.AddActivityTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddActivityTask", hCtx, request)
	ret0, _ := ret[0].(*types.AddActivityTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddActivityTask indicates an expected call of AddActivityTask.
func (mr *MockEngineMockRecorder) AddActivityTask(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddActivityTask", reflect.TypeOf((*MockEngine)(nil).AddActivityTask), hCtx, request)
}

// AddDecisionTask mocks base method.
func (m *MockEngine) AddDecisionTask(hCtx *handlerContext, request *types.AddDecisionTaskRequest) (*types.AddDecisionTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddDecisionTask", hCtx, request)
	ret0, _ := ret[0].(*types.AddDecisionTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddDecisionTask indicates an expected call of AddDecisionTask.
func (mr *MockEngineMockRecorder) AddDecisionTask(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddDecisionTask", reflect.TypeOf((*MockEngine)(nil).AddDecisionTask), hCtx, request)
}

// CancelOutstandingPoll mocks base method.
func (m *MockEngine) CancelOutstandingPoll(hCtx *handlerContext, request *types.CancelOutstandingPollRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelOutstandingPoll", hCtx, request)
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelOutstandingPoll indicates an expected call of CancelOutstandingPoll.
func (mr *MockEngineMockRecorder) CancelOutstandingPoll(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelOutstandingPoll", reflect.TypeOf((*MockEngine)(nil).CancelOutstandingPoll), hCtx, request)
}

// DescribeTaskList mocks base method.
func (m *MockEngine) DescribeTaskList(hCtx *handlerContext, request *types.MatchingDescribeTaskListRequest) (*types.DescribeTaskListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeTaskList", hCtx, request)
	ret0, _ := ret[0].(*types.DescribeTaskListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeTaskList indicates an expected call of DescribeTaskList.
func (mr *MockEngineMockRecorder) DescribeTaskList(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeTaskList", reflect.TypeOf((*MockEngine)(nil).DescribeTaskList), hCtx, request)
}

// GetTaskListsByDomain mocks base method.
func (m *MockEngine) GetTaskListsByDomain(hCtx *handlerContext, request *types.GetTaskListsByDomainRequest) (*types.GetTaskListsByDomainResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTaskListsByDomain", hCtx, request)
	ret0, _ := ret[0].(*types.GetTaskListsByDomainResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTaskListsByDomain indicates an expected call of GetTaskListsByDomain.
func (mr *MockEngineMockRecorder) GetTaskListsByDomain(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTaskListsByDomain", reflect.TypeOf((*MockEngine)(nil).GetTaskListsByDomain), hCtx, request)
}

// ListTaskListPartitions mocks base method.
func (m *MockEngine) ListTaskListPartitions(hCtx *handlerContext, request *types.MatchingListTaskListPartitionsRequest) (*types.ListTaskListPartitionsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTaskListPartitions", hCtx, request)
	ret0, _ := ret[0].(*types.ListTaskListPartitionsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTaskListPartitions indicates an expected call of ListTaskListPartitions.
func (mr *MockEngineMockRecorder) ListTaskListPartitions(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTaskListPartitions", reflect.TypeOf((*MockEngine)(nil).ListTaskListPartitions), hCtx, request)
}

// PollForActivityTask mocks base method.
func (m *MockEngine) PollForActivityTask(hCtx *handlerContext, request *types.MatchingPollForActivityTaskRequest) (*types.MatchingPollForActivityTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollForActivityTask", hCtx, request)
	ret0, _ := ret[0].(*types.MatchingPollForActivityTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollForActivityTask indicates an expected call of PollForActivityTask.
func (mr *MockEngineMockRecorder) PollForActivityTask(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollForActivityTask", reflect.TypeOf((*MockEngine)(nil).PollForActivityTask), hCtx, request)
}

// PollForDecisionTask mocks base method.
func (m *MockEngine) PollForDecisionTask(hCtx *handlerContext, request *types.MatchingPollForDecisionTaskRequest) (*types.MatchingPollForDecisionTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollForDecisionTask", hCtx, request)
	ret0, _ := ret[0].(*types.MatchingPollForDecisionTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollForDecisionTask indicates an expected call of PollForDecisionTask.
func (mr *MockEngineMockRecorder) PollForDecisionTask(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollForDecisionTask", reflect.TypeOf((*MockEngine)(nil).PollForDecisionTask), hCtx, request)
}

// QueryWorkflow mocks base method.
func (m *MockEngine) QueryWorkflow(hCtx *handlerContext, request *types.MatchingQueryWorkflowRequest) (*types.MatchingQueryWorkflowResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryWorkflow", hCtx, request)
	ret0, _ := ret[0].(*types.MatchingQueryWorkflowResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryWorkflow indicates an expected call of QueryWorkflow.
func (mr *MockEngineMockRecorder) QueryWorkflow(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryWorkflow", reflect.TypeOf((*MockEngine)(nil).QueryWorkflow), hCtx, request)
}

// RefreshTaskListPartitionConfig mocks base method.
func (m *MockEngine) RefreshTaskListPartitionConfig(hCtx *handlerContext, request *types.MatchingRefreshTaskListPartitionConfigRequest) (*types.MatchingRefreshTaskListPartitionConfigResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshTaskListPartitionConfig", hCtx, request)
	ret0, _ := ret[0].(*types.MatchingRefreshTaskListPartitionConfigResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RefreshTaskListPartitionConfig indicates an expected call of RefreshTaskListPartitionConfig.
func (mr *MockEngineMockRecorder) RefreshTaskListPartitionConfig(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshTaskListPartitionConfig", reflect.TypeOf((*MockEngine)(nil).RefreshTaskListPartitionConfig), hCtx, request)
}

// RespondQueryTaskCompleted mocks base method.
func (m *MockEngine) RespondQueryTaskCompleted(hCtx *handlerContext, request *types.MatchingRespondQueryTaskCompletedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondQueryTaskCompleted", hCtx, request)
	ret0, _ := ret[0].(error)
	return ret0
}

// RespondQueryTaskCompleted indicates an expected call of RespondQueryTaskCompleted.
func (mr *MockEngineMockRecorder) RespondQueryTaskCompleted(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondQueryTaskCompleted", reflect.TypeOf((*MockEngine)(nil).RespondQueryTaskCompleted), hCtx, request)
}

// Start mocks base method.
func (m *MockEngine) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *MockEngineMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockEngine)(nil).Start))
}

// Stop mocks base method.
func (m *MockEngine) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockEngineMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockEngine)(nil).Stop))
}

// UpdateTaskListPartitionConfig mocks base method.
func (m *MockEngine) UpdateTaskListPartitionConfig(hCtx *handlerContext, request *types.MatchingUpdateTaskListPartitionConfigRequest) (*types.MatchingUpdateTaskListPartitionConfigResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateTaskListPartitionConfig", hCtx, request)
	ret0, _ := ret[0].(*types.MatchingUpdateTaskListPartitionConfigResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateTaskListPartitionConfig indicates an expected call of UpdateTaskListPartitionConfig.
func (mr *MockEngineMockRecorder) UpdateTaskListPartitionConfig(hCtx, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTaskListPartitionConfig", reflect.TypeOf((*MockEngine)(nil).UpdateTaskListPartitionConfig), hCtx, request)
}

// MockHandler is a mock of Handler interface.
type MockHandler struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerMockRecorder
	isgomock struct{}
}

// MockHandlerMockRecorder is the mock recorder for MockHandler.
type MockHandlerMockRecorder struct {
	mock *MockHandler
}

// NewMockHandler creates a new mock instance.
func NewMockHandler(ctrl *gomock.Controller) *MockHandler {
	mock := &MockHandler{ctrl: ctrl}
	mock.recorder = &MockHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHandler) EXPECT() *MockHandlerMockRecorder {
	return m.recorder
}

// AddActivityTask mocks base method.
func (m *MockHandler) AddActivityTask(arg0 context.Context, arg1 *types.AddActivityTaskRequest) (*types.AddActivityTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddActivityTask", arg0, arg1)
	ret0, _ := ret[0].(*types.AddActivityTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddActivityTask indicates an expected call of AddActivityTask.
func (mr *MockHandlerMockRecorder) AddActivityTask(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddActivityTask", reflect.TypeOf((*MockHandler)(nil).AddActivityTask), arg0, arg1)
}

// AddDecisionTask mocks base method.
func (m *MockHandler) AddDecisionTask(arg0 context.Context, arg1 *types.AddDecisionTaskRequest) (*types.AddDecisionTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddDecisionTask", arg0, arg1)
	ret0, _ := ret[0].(*types.AddDecisionTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddDecisionTask indicates an expected call of AddDecisionTask.
func (mr *MockHandlerMockRecorder) AddDecisionTask(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddDecisionTask", reflect.TypeOf((*MockHandler)(nil).AddDecisionTask), arg0, arg1)
}

// CancelOutstandingPoll mocks base method.
func (m *MockHandler) CancelOutstandingPoll(arg0 context.Context, arg1 *types.CancelOutstandingPollRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelOutstandingPoll", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelOutstandingPoll indicates an expected call of CancelOutstandingPoll.
func (mr *MockHandlerMockRecorder) CancelOutstandingPoll(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelOutstandingPoll", reflect.TypeOf((*MockHandler)(nil).CancelOutstandingPoll), arg0, arg1)
}

// DescribeTaskList mocks base method.
func (m *MockHandler) DescribeTaskList(arg0 context.Context, arg1 *types.MatchingDescribeTaskListRequest) (*types.DescribeTaskListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeTaskList", arg0, arg1)
	ret0, _ := ret[0].(*types.DescribeTaskListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeTaskList indicates an expected call of DescribeTaskList.
func (mr *MockHandlerMockRecorder) DescribeTaskList(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeTaskList", reflect.TypeOf((*MockHandler)(nil).DescribeTaskList), arg0, arg1)
}

// GetTaskListsByDomain mocks base method.
func (m *MockHandler) GetTaskListsByDomain(arg0 context.Context, arg1 *types.GetTaskListsByDomainRequest) (*types.GetTaskListsByDomainResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTaskListsByDomain", arg0, arg1)
	ret0, _ := ret[0].(*types.GetTaskListsByDomainResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTaskListsByDomain indicates an expected call of GetTaskListsByDomain.
func (mr *MockHandlerMockRecorder) GetTaskListsByDomain(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTaskListsByDomain", reflect.TypeOf((*MockHandler)(nil).GetTaskListsByDomain), arg0, arg1)
}

// Health mocks base method.
func (m *MockHandler) Health(arg0 context.Context) (*types.HealthStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Health", arg0)
	ret0, _ := ret[0].(*types.HealthStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Health indicates an expected call of Health.
func (mr *MockHandlerMockRecorder) Health(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Health", reflect.TypeOf((*MockHandler)(nil).Health), arg0)
}

// ListTaskListPartitions mocks base method.
func (m *MockHandler) ListTaskListPartitions(arg0 context.Context, arg1 *types.MatchingListTaskListPartitionsRequest) (*types.ListTaskListPartitionsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTaskListPartitions", arg0, arg1)
	ret0, _ := ret[0].(*types.ListTaskListPartitionsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTaskListPartitions indicates an expected call of ListTaskListPartitions.
func (mr *MockHandlerMockRecorder) ListTaskListPartitions(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTaskListPartitions", reflect.TypeOf((*MockHandler)(nil).ListTaskListPartitions), arg0, arg1)
}

// PollForActivityTask mocks base method.
func (m *MockHandler) PollForActivityTask(arg0 context.Context, arg1 *types.MatchingPollForActivityTaskRequest) (*types.MatchingPollForActivityTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollForActivityTask", arg0, arg1)
	ret0, _ := ret[0].(*types.MatchingPollForActivityTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollForActivityTask indicates an expected call of PollForActivityTask.
func (mr *MockHandlerMockRecorder) PollForActivityTask(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollForActivityTask", reflect.TypeOf((*MockHandler)(nil).PollForActivityTask), arg0, arg1)
}

// PollForDecisionTask mocks base method.
func (m *MockHandler) PollForDecisionTask(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest) (*types.MatchingPollForDecisionTaskResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollForDecisionTask", arg0, arg1)
	ret0, _ := ret[0].(*types.MatchingPollForDecisionTaskResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollForDecisionTask indicates an expected call of PollForDecisionTask.
func (mr *MockHandlerMockRecorder) PollForDecisionTask(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollForDecisionTask", reflect.TypeOf((*MockHandler)(nil).PollForDecisionTask), arg0, arg1)
}

// QueryWorkflow mocks base method.
func (m *MockHandler) QueryWorkflow(arg0 context.Context, arg1 *types.MatchingQueryWorkflowRequest) (*types.MatchingQueryWorkflowResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryWorkflow", arg0, arg1)
	ret0, _ := ret[0].(*types.MatchingQueryWorkflowResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryWorkflow indicates an expected call of QueryWorkflow.
func (mr *MockHandlerMockRecorder) QueryWorkflow(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryWorkflow", reflect.TypeOf((*MockHandler)(nil).QueryWorkflow), arg0, arg1)
}

// RefreshTaskListPartitionConfig mocks base method.
func (m *MockHandler) RefreshTaskListPartitionConfig(arg0 context.Context, arg1 *types.MatchingRefreshTaskListPartitionConfigRequest) (*types.MatchingRefreshTaskListPartitionConfigResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshTaskListPartitionConfig", arg0, arg1)
	ret0, _ := ret[0].(*types.MatchingRefreshTaskListPartitionConfigResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RefreshTaskListPartitionConfig indicates an expected call of RefreshTaskListPartitionConfig.
func (mr *MockHandlerMockRecorder) RefreshTaskListPartitionConfig(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshTaskListPartitionConfig", reflect.TypeOf((*MockHandler)(nil).RefreshTaskListPartitionConfig), arg0, arg1)
}

// RespondQueryTaskCompleted mocks base method.
func (m *MockHandler) RespondQueryTaskCompleted(arg0 context.Context, arg1 *types.MatchingRespondQueryTaskCompletedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondQueryTaskCompleted", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RespondQueryTaskCompleted indicates an expected call of RespondQueryTaskCompleted.
func (mr *MockHandlerMockRecorder) RespondQueryTaskCompleted(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondQueryTaskCompleted", reflect.TypeOf((*MockHandler)(nil).RespondQueryTaskCompleted), arg0, arg1)
}

// Start mocks base method.
func (m *MockHandler) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *MockHandlerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockHandler)(nil).Start))
}

// Stop mocks base method.
func (m *MockHandler) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockHandlerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockHandler)(nil).Stop))
}

// UpdateTaskListPartitionConfig mocks base method.
func (m *MockHandler) UpdateTaskListPartitionConfig(arg0 context.Context, arg1 *types.MatchingUpdateTaskListPartitionConfigRequest) (*types.MatchingUpdateTaskListPartitionConfigResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateTaskListPartitionConfig", arg0, arg1)
	ret0, _ := ret[0].(*types.MatchingUpdateTaskListPartitionConfigResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateTaskListPartitionConfig indicates an expected call of UpdateTaskListPartitionConfig.
func (mr *MockHandlerMockRecorder) UpdateTaskListPartitionConfig(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTaskListPartitionConfig", reflect.TypeOf((*MockHandler)(nil).UpdateTaskListPartitionConfig), arg0, arg1)
}
