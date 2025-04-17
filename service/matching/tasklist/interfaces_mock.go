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
//	mockgen -package tasklist -source interfaces.go -destination interfaces_mock.go github.com/uber/cadence/service/matching/tasklist TaskCompleter
//

// Package tasklist is a generated GoMock package.
package tasklist

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "go.uber.org/mock/gomock"

	types "github.com/uber/cadence/common/types"
)

// MockManager is a mock of Manager interface.
type MockManager struct {
	ctrl     *gomock.Controller
	recorder *MockManagerMockRecorder
	isgomock struct{}
}

// MockManagerMockRecorder is the mock recorder for MockManager.
type MockManagerMockRecorder struct {
	mock *MockManager
}

// NewMockManager creates a new mock instance.
func NewMockManager(ctrl *gomock.Controller) *MockManager {
	mock := &MockManager{ctrl: ctrl}
	mock.recorder = &MockManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManager) EXPECT() *MockManagerMockRecorder {
	return m.recorder
}

// AddTask mocks base method.
func (m *MockManager) AddTask(ctx context.Context, params AddTaskParams) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddTask", ctx, params)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddTask indicates an expected call of AddTask.
func (mr *MockManagerMockRecorder) AddTask(ctx, params any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTask", reflect.TypeOf((*MockManager)(nil).AddTask), ctx, params)
}

// CancelPoller mocks base method.
func (m *MockManager) CancelPoller(pollerID string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CancelPoller", pollerID)
}

// CancelPoller indicates an expected call of CancelPoller.
func (mr *MockManagerMockRecorder) CancelPoller(pollerID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelPoller", reflect.TypeOf((*MockManager)(nil).CancelPoller), pollerID)
}

// DescribeTaskList mocks base method.
func (m *MockManager) DescribeTaskList(includeTaskListStatus bool) *types.DescribeTaskListResponse {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeTaskList", includeTaskListStatus)
	ret0, _ := ret[0].(*types.DescribeTaskListResponse)
	return ret0
}

// DescribeTaskList indicates an expected call of DescribeTaskList.
func (mr *MockManagerMockRecorder) DescribeTaskList(includeTaskListStatus any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeTaskList", reflect.TypeOf((*MockManager)(nil).DescribeTaskList), includeTaskListStatus)
}

// DispatchQueryTask mocks base method.
func (m *MockManager) DispatchQueryTask(ctx context.Context, taskID string, request *types.MatchingQueryWorkflowRequest) (*types.QueryWorkflowResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DispatchQueryTask", ctx, taskID, request)
	ret0, _ := ret[0].(*types.QueryWorkflowResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DispatchQueryTask indicates an expected call of DispatchQueryTask.
func (mr *MockManagerMockRecorder) DispatchQueryTask(ctx, taskID, request any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DispatchQueryTask", reflect.TypeOf((*MockManager)(nil).DispatchQueryTask), ctx, taskID, request)
}

// DispatchTask mocks base method.
func (m *MockManager) DispatchTask(ctx context.Context, task *InternalTask) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DispatchTask", ctx, task)
	ret0, _ := ret[0].(error)
	return ret0
}

// DispatchTask indicates an expected call of DispatchTask.
func (mr *MockManagerMockRecorder) DispatchTask(ctx, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DispatchTask", reflect.TypeOf((*MockManager)(nil).DispatchTask), ctx, task)
}

// GetAllPollerInfo mocks base method.
func (m *MockManager) GetAllPollerInfo() []*types.PollerInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllPollerInfo")
	ret0, _ := ret[0].([]*types.PollerInfo)
	return ret0
}

// GetAllPollerInfo indicates an expected call of GetAllPollerInfo.
func (mr *MockManagerMockRecorder) GetAllPollerInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllPollerInfo", reflect.TypeOf((*MockManager)(nil).GetAllPollerInfo))
}

// GetTask mocks base method.
func (m *MockManager) GetTask(ctx context.Context, maxDispatchPerSecond *float64) (*InternalTask, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTask", ctx, maxDispatchPerSecond)
	ret0, _ := ret[0].(*InternalTask)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTask indicates an expected call of GetTask.
func (mr *MockManagerMockRecorder) GetTask(ctx, maxDispatchPerSecond any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTask", reflect.TypeOf((*MockManager)(nil).GetTask), ctx, maxDispatchPerSecond)
}

// GetTaskListKind mocks base method.
func (m *MockManager) GetTaskListKind() types.TaskListKind {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTaskListKind")
	ret0, _ := ret[0].(types.TaskListKind)
	return ret0
}

// GetTaskListKind indicates an expected call of GetTaskListKind.
func (mr *MockManagerMockRecorder) GetTaskListKind() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTaskListKind", reflect.TypeOf((*MockManager)(nil).GetTaskListKind))
}

// HasPollerAfter mocks base method.
func (m *MockManager) HasPollerAfter(accessTime time.Time) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasPollerAfter", accessTime)
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasPollerAfter indicates an expected call of HasPollerAfter.
func (mr *MockManagerMockRecorder) HasPollerAfter(accessTime any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasPollerAfter", reflect.TypeOf((*MockManager)(nil).HasPollerAfter), accessTime)
}

// LoadBalancerHints mocks base method.
func (m *MockManager) LoadBalancerHints() *types.LoadBalancerHints {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadBalancerHints")
	ret0, _ := ret[0].(*types.LoadBalancerHints)
	return ret0
}

// LoadBalancerHints indicates an expected call of LoadBalancerHints.
func (mr *MockManagerMockRecorder) LoadBalancerHints() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadBalancerHints", reflect.TypeOf((*MockManager)(nil).LoadBalancerHints))
}

// RefreshTaskListPartitionConfig mocks base method.
func (m *MockManager) RefreshTaskListPartitionConfig(arg0 context.Context, arg1 *types.TaskListPartitionConfig) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshTaskListPartitionConfig", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RefreshTaskListPartitionConfig indicates an expected call of RefreshTaskListPartitionConfig.
func (mr *MockManagerMockRecorder) RefreshTaskListPartitionConfig(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshTaskListPartitionConfig", reflect.TypeOf((*MockManager)(nil).RefreshTaskListPartitionConfig), arg0, arg1)
}

// Start mocks base method.
func (m *MockManager) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockManagerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockManager)(nil).Start))
}

// Stop mocks base method.
func (m *MockManager) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockManagerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockManager)(nil).Stop))
}

// String mocks base method.
func (m *MockManager) String() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "String")
	ret0, _ := ret[0].(string)
	return ret0
}

// String indicates an expected call of String.
func (mr *MockManagerMockRecorder) String() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "String", reflect.TypeOf((*MockManager)(nil).String))
}

// TaskListID mocks base method.
func (m *MockManager) TaskListID() *Identifier {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TaskListID")
	ret0, _ := ret[0].(*Identifier)
	return ret0
}

// TaskListID indicates an expected call of TaskListID.
func (mr *MockManagerMockRecorder) TaskListID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TaskListID", reflect.TypeOf((*MockManager)(nil).TaskListID))
}

// TaskListPartitionConfig mocks base method.
func (m *MockManager) TaskListPartitionConfig() *types.TaskListPartitionConfig {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TaskListPartitionConfig")
	ret0, _ := ret[0].(*types.TaskListPartitionConfig)
	return ret0
}

// TaskListPartitionConfig indicates an expected call of TaskListPartitionConfig.
func (mr *MockManagerMockRecorder) TaskListPartitionConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TaskListPartitionConfig", reflect.TypeOf((*MockManager)(nil).TaskListPartitionConfig))
}

// UpdateTaskListPartitionConfig mocks base method.
func (m *MockManager) UpdateTaskListPartitionConfig(arg0 context.Context, arg1 *types.TaskListPartitionConfig) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateTaskListPartitionConfig", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateTaskListPartitionConfig indicates an expected call of UpdateTaskListPartitionConfig.
func (mr *MockManagerMockRecorder) UpdateTaskListPartitionConfig(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTaskListPartitionConfig", reflect.TypeOf((*MockManager)(nil).UpdateTaskListPartitionConfig), arg0, arg1)
}

// MockTaskMatcher is a mock of TaskMatcher interface.
type MockTaskMatcher struct {
	ctrl     *gomock.Controller
	recorder *MockTaskMatcherMockRecorder
	isgomock struct{}
}

// MockTaskMatcherMockRecorder is the mock recorder for MockTaskMatcher.
type MockTaskMatcherMockRecorder struct {
	mock *MockTaskMatcher
}

// NewMockTaskMatcher creates a new mock instance.
func NewMockTaskMatcher(ctrl *gomock.Controller) *MockTaskMatcher {
	mock := &MockTaskMatcher{ctrl: ctrl}
	mock.recorder = &MockTaskMatcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTaskMatcher) EXPECT() *MockTaskMatcherMockRecorder {
	return m.recorder
}

// DisconnectBlockedPollers mocks base method.
func (m *MockTaskMatcher) DisconnectBlockedPollers() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DisconnectBlockedPollers")
}

// DisconnectBlockedPollers indicates an expected call of DisconnectBlockedPollers.
func (mr *MockTaskMatcherMockRecorder) DisconnectBlockedPollers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DisconnectBlockedPollers", reflect.TypeOf((*MockTaskMatcher)(nil).DisconnectBlockedPollers))
}

// MustOffer mocks base method.
func (m *MockTaskMatcher) MustOffer(ctx context.Context, task *InternalTask) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MustOffer", ctx, task)
	ret0, _ := ret[0].(error)
	return ret0
}

// MustOffer indicates an expected call of MustOffer.
func (mr *MockTaskMatcherMockRecorder) MustOffer(ctx, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MustOffer", reflect.TypeOf((*MockTaskMatcher)(nil).MustOffer), ctx, task)
}

// Offer mocks base method.
func (m *MockTaskMatcher) Offer(ctx context.Context, task *InternalTask) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Offer", ctx, task)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Offer indicates an expected call of Offer.
func (mr *MockTaskMatcherMockRecorder) Offer(ctx, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Offer", reflect.TypeOf((*MockTaskMatcher)(nil).Offer), ctx, task)
}

// OfferOrTimeout mocks base method.
func (m *MockTaskMatcher) OfferOrTimeout(ctx context.Context, startT time.Time, task *InternalTask) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OfferOrTimeout", ctx, startT, task)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OfferOrTimeout indicates an expected call of OfferOrTimeout.
func (mr *MockTaskMatcherMockRecorder) OfferOrTimeout(ctx, startT, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OfferOrTimeout", reflect.TypeOf((*MockTaskMatcher)(nil).OfferOrTimeout), ctx, startT, task)
}

// OfferQuery mocks base method.
func (m *MockTaskMatcher) OfferQuery(ctx context.Context, task *InternalTask) (*types.QueryWorkflowResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OfferQuery", ctx, task)
	ret0, _ := ret[0].(*types.QueryWorkflowResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OfferQuery indicates an expected call of OfferQuery.
func (mr *MockTaskMatcherMockRecorder) OfferQuery(ctx, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OfferQuery", reflect.TypeOf((*MockTaskMatcher)(nil).OfferQuery), ctx, task)
}

// Poll mocks base method.
func (m *MockTaskMatcher) Poll(ctx context.Context, isolationGroup string) (*InternalTask, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Poll", ctx, isolationGroup)
	ret0, _ := ret[0].(*InternalTask)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Poll indicates an expected call of Poll.
func (mr *MockTaskMatcherMockRecorder) Poll(ctx, isolationGroup any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Poll", reflect.TypeOf((*MockTaskMatcher)(nil).Poll), ctx, isolationGroup)
}

// PollForQuery mocks base method.
func (m *MockTaskMatcher) PollForQuery(ctx context.Context) (*InternalTask, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollForQuery", ctx)
	ret0, _ := ret[0].(*InternalTask)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollForQuery indicates an expected call of PollForQuery.
func (mr *MockTaskMatcherMockRecorder) PollForQuery(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollForQuery", reflect.TypeOf((*MockTaskMatcher)(nil).PollForQuery), ctx)
}

// Rate mocks base method.
func (m *MockTaskMatcher) Rate() float64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Rate")
	ret0, _ := ret[0].(float64)
	return ret0
}

// Rate indicates an expected call of Rate.
func (mr *MockTaskMatcherMockRecorder) Rate() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rate", reflect.TypeOf((*MockTaskMatcher)(nil).Rate))
}

// UpdateRatelimit mocks base method.
func (m *MockTaskMatcher) UpdateRatelimit(rps *float64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateRatelimit", rps)
}

// UpdateRatelimit indicates an expected call of UpdateRatelimit.
func (mr *MockTaskMatcherMockRecorder) UpdateRatelimit(rps any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateRatelimit", reflect.TypeOf((*MockTaskMatcher)(nil).UpdateRatelimit), rps)
}

// MockForwarder is a mock of Forwarder interface.
type MockForwarder struct {
	ctrl     *gomock.Controller
	recorder *MockForwarderMockRecorder
	isgomock struct{}
}

// MockForwarderMockRecorder is the mock recorder for MockForwarder.
type MockForwarderMockRecorder struct {
	mock *MockForwarder
}

// NewMockForwarder creates a new mock instance.
func NewMockForwarder(ctrl *gomock.Controller) *MockForwarder {
	mock := &MockForwarder{ctrl: ctrl}
	mock.recorder = &MockForwarderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockForwarder) EXPECT() *MockForwarderMockRecorder {
	return m.recorder
}

// AddReqTokenC mocks base method.
func (m *MockForwarder) AddReqTokenC() <-chan *ForwarderReqToken {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddReqTokenC")
	ret0, _ := ret[0].(<-chan *ForwarderReqToken)
	return ret0
}

// AddReqTokenC indicates an expected call of AddReqTokenC.
func (mr *MockForwarderMockRecorder) AddReqTokenC() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddReqTokenC", reflect.TypeOf((*MockForwarder)(nil).AddReqTokenC))
}

// ForwardPoll mocks base method.
func (m *MockForwarder) ForwardPoll(ctx context.Context) (*InternalTask, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ForwardPoll", ctx)
	ret0, _ := ret[0].(*InternalTask)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ForwardPoll indicates an expected call of ForwardPoll.
func (mr *MockForwarderMockRecorder) ForwardPoll(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ForwardPoll", reflect.TypeOf((*MockForwarder)(nil).ForwardPoll), ctx)
}

// ForwardQueryTask mocks base method.
func (m *MockForwarder) ForwardQueryTask(ctx context.Context, task *InternalTask) (*types.QueryWorkflowResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ForwardQueryTask", ctx, task)
	ret0, _ := ret[0].(*types.QueryWorkflowResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ForwardQueryTask indicates an expected call of ForwardQueryTask.
func (mr *MockForwarderMockRecorder) ForwardQueryTask(ctx, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ForwardQueryTask", reflect.TypeOf((*MockForwarder)(nil).ForwardQueryTask), ctx, task)
}

// ForwardTask mocks base method.
func (m *MockForwarder) ForwardTask(ctx context.Context, task *InternalTask) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ForwardTask", ctx, task)
	ret0, _ := ret[0].(error)
	return ret0
}

// ForwardTask indicates an expected call of ForwardTask.
func (mr *MockForwarderMockRecorder) ForwardTask(ctx, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ForwardTask", reflect.TypeOf((*MockForwarder)(nil).ForwardTask), ctx, task)
}

// PollReqTokenC mocks base method.
func (m *MockForwarder) PollReqTokenC() <-chan *ForwarderReqToken {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollReqTokenC")
	ret0, _ := ret[0].(<-chan *ForwarderReqToken)
	return ret0
}

// PollReqTokenC indicates an expected call of PollReqTokenC.
func (mr *MockForwarderMockRecorder) PollReqTokenC() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollReqTokenC", reflect.TypeOf((*MockForwarder)(nil).PollReqTokenC))
}

// MockTaskCompleter is a mock of TaskCompleter interface.
type MockTaskCompleter struct {
	ctrl     *gomock.Controller
	recorder *MockTaskCompleterMockRecorder
	isgomock struct{}
}

// MockTaskCompleterMockRecorder is the mock recorder for MockTaskCompleter.
type MockTaskCompleterMockRecorder struct {
	mock *MockTaskCompleter
}

// NewMockTaskCompleter creates a new mock instance.
func NewMockTaskCompleter(ctrl *gomock.Controller) *MockTaskCompleter {
	mock := &MockTaskCompleter{ctrl: ctrl}
	mock.recorder = &MockTaskCompleterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTaskCompleter) EXPECT() *MockTaskCompleterMockRecorder {
	return m.recorder
}

// CompleteTaskIfStarted mocks base method.
func (m *MockTaskCompleter) CompleteTaskIfStarted(ctx context.Context, task *InternalTask) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CompleteTaskIfStarted", ctx, task)
	ret0, _ := ret[0].(error)
	return ret0
}

// CompleteTaskIfStarted indicates an expected call of CompleteTaskIfStarted.
func (mr *MockTaskCompleterMockRecorder) CompleteTaskIfStarted(ctx, task any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CompleteTaskIfStarted", reflect.TypeOf((*MockTaskCompleter)(nil).CompleteTaskIfStarted), ctx, task)
}
