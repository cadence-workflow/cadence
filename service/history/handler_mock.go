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

// Package history is a generated GoMock package.
package history

import (
	context "context"
	reflect "reflect"
	time "time"

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

// PrepareToStop mocks base method
func (m *MockHandler) PrepareToStop(arg0 time.Duration) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrepareToStop", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// PrepareToStop indicates an expected call of PrepareToStop
func (mr *MockHandlerMockRecorder) PrepareToStop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrepareToStop", reflect.TypeOf((*MockHandler)(nil).PrepareToStop), arg0)
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

// CloseShard mocks base method
func (m *MockHandler) CloseShard(arg0 context.Context, arg1 *types.CloseShardRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseShard", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseShard indicates an expected call of CloseShard
func (mr *MockHandlerMockRecorder) CloseShard(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseShard", reflect.TypeOf((*MockHandler)(nil).CloseShard), arg0, arg1)
}

// DescribeHistoryHost mocks base method
func (m *MockHandler) DescribeHistoryHost(arg0 context.Context, arg1 *types.DescribeHistoryHostRequest) (*types.DescribeHistoryHostResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeHistoryHost", arg0, arg1)
	ret0, _ := ret[0].(*types.DescribeHistoryHostResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeHistoryHost indicates an expected call of DescribeHistoryHost
func (mr *MockHandlerMockRecorder) DescribeHistoryHost(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeHistoryHost", reflect.TypeOf((*MockHandler)(nil).DescribeHistoryHost), arg0, arg1)
}

// DescribeMutableState mocks base method
func (m *MockHandler) DescribeMutableState(arg0 context.Context, arg1 *types.DescribeMutableStateRequest) (*types.DescribeMutableStateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeMutableState", arg0, arg1)
	ret0, _ := ret[0].(*types.DescribeMutableStateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeMutableState indicates an expected call of DescribeMutableState
func (mr *MockHandlerMockRecorder) DescribeMutableState(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeMutableState", reflect.TypeOf((*MockHandler)(nil).DescribeMutableState), arg0, arg1)
}

// DescribeQueue mocks base method
func (m *MockHandler) DescribeQueue(arg0 context.Context, arg1 *types.DescribeQueueRequest) (*types.DescribeQueueResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeQueue", arg0, arg1)
	ret0, _ := ret[0].(*types.DescribeQueueResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeQueue indicates an expected call of DescribeQueue
func (mr *MockHandlerMockRecorder) DescribeQueue(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeQueue", reflect.TypeOf((*MockHandler)(nil).DescribeQueue), arg0, arg1)
}

// DescribeWorkflowExecution mocks base method
func (m *MockHandler) DescribeWorkflowExecution(arg0 context.Context, arg1 *types.HistoryDescribeWorkflowExecutionRequest) (*types.DescribeWorkflowExecutionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeWorkflowExecution", arg0, arg1)
	ret0, _ := ret[0].(*types.DescribeWorkflowExecutionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeWorkflowExecution indicates an expected call of DescribeWorkflowExecution
func (mr *MockHandlerMockRecorder) DescribeWorkflowExecution(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeWorkflowExecution", reflect.TypeOf((*MockHandler)(nil).DescribeWorkflowExecution), arg0, arg1)
}

// GetCrossClusterTasks mocks base method
func (m *MockHandler) GetCrossClusterTasks(arg0 context.Context, arg1 *types.GetCrossClusterTasksRequest) (*types.GetCrossClusterTasksResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCrossClusterTasks", arg0, arg1)
	ret0, _ := ret[0].(*types.GetCrossClusterTasksResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCrossClusterTasks indicates an expected call of GetCrossClusterTasks
func (mr *MockHandlerMockRecorder) GetCrossClusterTasks(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCrossClusterTasks", reflect.TypeOf((*MockHandler)(nil).GetCrossClusterTasks), arg0, arg1)
}

// GetDLQReplicationMessages mocks base method
func (m *MockHandler) GetDLQReplicationMessages(arg0 context.Context, arg1 *types.GetDLQReplicationMessagesRequest) (*types.GetDLQReplicationMessagesResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDLQReplicationMessages", arg0, arg1)
	ret0, _ := ret[0].(*types.GetDLQReplicationMessagesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDLQReplicationMessages indicates an expected call of GetDLQReplicationMessages
func (mr *MockHandlerMockRecorder) GetDLQReplicationMessages(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDLQReplicationMessages", reflect.TypeOf((*MockHandler)(nil).GetDLQReplicationMessages), arg0, arg1)
}

// GetMutableState mocks base method
func (m *MockHandler) GetMutableState(arg0 context.Context, arg1 *types.GetMutableStateRequest) (*types.GetMutableStateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMutableState", arg0, arg1)
	ret0, _ := ret[0].(*types.GetMutableStateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMutableState indicates an expected call of GetMutableState
func (mr *MockHandlerMockRecorder) GetMutableState(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMutableState", reflect.TypeOf((*MockHandler)(nil).GetMutableState), arg0, arg1)
}

// GetReplicationMessages mocks base method
func (m *MockHandler) GetReplicationMessages(arg0 context.Context, arg1 *types.GetReplicationMessagesRequest) (*types.GetReplicationMessagesResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetReplicationMessages", arg0, arg1)
	ret0, _ := ret[0].(*types.GetReplicationMessagesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetReplicationMessages indicates an expected call of GetReplicationMessages
func (mr *MockHandlerMockRecorder) GetReplicationMessages(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReplicationMessages", reflect.TypeOf((*MockHandler)(nil).GetReplicationMessages), arg0, arg1)
}

// MergeDLQMessages mocks base method
func (m *MockHandler) MergeDLQMessages(arg0 context.Context, arg1 *types.MergeDLQMessagesRequest) (*types.MergeDLQMessagesResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MergeDLQMessages", arg0, arg1)
	ret0, _ := ret[0].(*types.MergeDLQMessagesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MergeDLQMessages indicates an expected call of MergeDLQMessages
func (mr *MockHandlerMockRecorder) MergeDLQMessages(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MergeDLQMessages", reflect.TypeOf((*MockHandler)(nil).MergeDLQMessages), arg0, arg1)
}

// NotifyFailoverMarkers mocks base method
func (m *MockHandler) NotifyFailoverMarkers(arg0 context.Context, arg1 *types.NotifyFailoverMarkersRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NotifyFailoverMarkers", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// NotifyFailoverMarkers indicates an expected call of NotifyFailoverMarkers
func (mr *MockHandlerMockRecorder) NotifyFailoverMarkers(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotifyFailoverMarkers", reflect.TypeOf((*MockHandler)(nil).NotifyFailoverMarkers), arg0, arg1)
}

// PollMutableState mocks base method
func (m *MockHandler) PollMutableState(arg0 context.Context, arg1 *types.PollMutableStateRequest) (*types.PollMutableStateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PollMutableState", arg0, arg1)
	ret0, _ := ret[0].(*types.PollMutableStateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PollMutableState indicates an expected call of PollMutableState
func (mr *MockHandlerMockRecorder) PollMutableState(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PollMutableState", reflect.TypeOf((*MockHandler)(nil).PollMutableState), arg0, arg1)
}

// PurgeDLQMessages mocks base method
func (m *MockHandler) PurgeDLQMessages(arg0 context.Context, arg1 *types.PurgeDLQMessagesRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PurgeDLQMessages", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// PurgeDLQMessages indicates an expected call of PurgeDLQMessages
func (mr *MockHandlerMockRecorder) PurgeDLQMessages(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PurgeDLQMessages", reflect.TypeOf((*MockHandler)(nil).PurgeDLQMessages), arg0, arg1)
}

// QueryWorkflow mocks base method
func (m *MockHandler) QueryWorkflow(arg0 context.Context, arg1 *types.HistoryQueryWorkflowRequest) (*types.HistoryQueryWorkflowResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryWorkflow", arg0, arg1)
	ret0, _ := ret[0].(*types.HistoryQueryWorkflowResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryWorkflow indicates an expected call of QueryWorkflow
func (mr *MockHandlerMockRecorder) QueryWorkflow(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryWorkflow", reflect.TypeOf((*MockHandler)(nil).QueryWorkflow), arg0, arg1)
}

// ReadDLQMessages mocks base method
func (m *MockHandler) ReadDLQMessages(arg0 context.Context, arg1 *types.ReadDLQMessagesRequest) (*types.ReadDLQMessagesResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDLQMessages", arg0, arg1)
	ret0, _ := ret[0].(*types.ReadDLQMessagesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDLQMessages indicates an expected call of ReadDLQMessages
func (mr *MockHandlerMockRecorder) ReadDLQMessages(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDLQMessages", reflect.TypeOf((*MockHandler)(nil).ReadDLQMessages), arg0, arg1)
}

// ReapplyEvents mocks base method
func (m *MockHandler) ReapplyEvents(arg0 context.Context, arg1 *types.HistoryReapplyEventsRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReapplyEvents", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReapplyEvents indicates an expected call of ReapplyEvents
func (mr *MockHandlerMockRecorder) ReapplyEvents(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReapplyEvents", reflect.TypeOf((*MockHandler)(nil).ReapplyEvents), arg0, arg1)
}

// RecordActivityTaskHeartbeat mocks base method
func (m *MockHandler) RecordActivityTaskHeartbeat(arg0 context.Context, arg1 *types.HistoryRecordActivityTaskHeartbeatRequest) (*types.RecordActivityTaskHeartbeatResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecordActivityTaskHeartbeat", arg0, arg1)
	ret0, _ := ret[0].(*types.RecordActivityTaskHeartbeatResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RecordActivityTaskHeartbeat indicates an expected call of RecordActivityTaskHeartbeat
func (mr *MockHandlerMockRecorder) RecordActivityTaskHeartbeat(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordActivityTaskHeartbeat", reflect.TypeOf((*MockHandler)(nil).RecordActivityTaskHeartbeat), arg0, arg1)
}

// RecordActivityTaskStarted mocks base method
func (m *MockHandler) RecordActivityTaskStarted(arg0 context.Context, arg1 *types.RecordActivityTaskStartedRequest) (*types.RecordActivityTaskStartedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecordActivityTaskStarted", arg0, arg1)
	ret0, _ := ret[0].(*types.RecordActivityTaskStartedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RecordActivityTaskStarted indicates an expected call of RecordActivityTaskStarted
func (mr *MockHandlerMockRecorder) RecordActivityTaskStarted(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordActivityTaskStarted", reflect.TypeOf((*MockHandler)(nil).RecordActivityTaskStarted), arg0, arg1)
}

// RecordChildExecutionCompleted mocks base method
func (m *MockHandler) RecordChildExecutionCompleted(arg0 context.Context, arg1 *types.RecordChildExecutionCompletedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecordChildExecutionCompleted", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecordChildExecutionCompleted indicates an expected call of RecordChildExecutionCompleted
func (mr *MockHandlerMockRecorder) RecordChildExecutionCompleted(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordChildExecutionCompleted", reflect.TypeOf((*MockHandler)(nil).RecordChildExecutionCompleted), arg0, arg1)
}

// RecordDecisionTaskStarted mocks base method
func (m *MockHandler) RecordDecisionTaskStarted(arg0 context.Context, arg1 *types.RecordDecisionTaskStartedRequest) (*types.RecordDecisionTaskStartedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecordDecisionTaskStarted", arg0, arg1)
	ret0, _ := ret[0].(*types.RecordDecisionTaskStartedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RecordDecisionTaskStarted indicates an expected call of RecordDecisionTaskStarted
func (mr *MockHandlerMockRecorder) RecordDecisionTaskStarted(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordDecisionTaskStarted", reflect.TypeOf((*MockHandler)(nil).RecordDecisionTaskStarted), arg0, arg1)
}

// RefreshWorkflowTasks mocks base method
func (m *MockHandler) RefreshWorkflowTasks(arg0 context.Context, arg1 *types.HistoryRefreshWorkflowTasksRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshWorkflowTasks", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RefreshWorkflowTasks indicates an expected call of RefreshWorkflowTasks
func (mr *MockHandlerMockRecorder) RefreshWorkflowTasks(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshWorkflowTasks", reflect.TypeOf((*MockHandler)(nil).RefreshWorkflowTasks), arg0, arg1)
}

// RemoveSignalMutableState mocks base method
func (m *MockHandler) RemoveSignalMutableState(arg0 context.Context, arg1 *types.RemoveSignalMutableStateRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveSignalMutableState", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveSignalMutableState indicates an expected call of RemoveSignalMutableState
func (mr *MockHandlerMockRecorder) RemoveSignalMutableState(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveSignalMutableState", reflect.TypeOf((*MockHandler)(nil).RemoveSignalMutableState), arg0, arg1)
}

// RemoveTask mocks base method
func (m *MockHandler) RemoveTask(arg0 context.Context, arg1 *types.RemoveTaskRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveTask", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveTask indicates an expected call of RemoveTask
func (mr *MockHandlerMockRecorder) RemoveTask(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveTask", reflect.TypeOf((*MockHandler)(nil).RemoveTask), arg0, arg1)
}

// ReplicateEventsV2 mocks base method
func (m *MockHandler) ReplicateEventsV2(arg0 context.Context, arg1 *types.ReplicateEventsV2Request) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplicateEventsV2", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReplicateEventsV2 indicates an expected call of ReplicateEventsV2
func (mr *MockHandlerMockRecorder) ReplicateEventsV2(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplicateEventsV2", reflect.TypeOf((*MockHandler)(nil).ReplicateEventsV2), arg0, arg1)
}

// RequestCancelWorkflowExecution mocks base method
func (m *MockHandler) RequestCancelWorkflowExecution(arg0 context.Context, arg1 *types.HistoryRequestCancelWorkflowExecutionRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RequestCancelWorkflowExecution", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RequestCancelWorkflowExecution indicates an expected call of RequestCancelWorkflowExecution
func (mr *MockHandlerMockRecorder) RequestCancelWorkflowExecution(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RequestCancelWorkflowExecution", reflect.TypeOf((*MockHandler)(nil).RequestCancelWorkflowExecution), arg0, arg1)
}

// ResetQueue mocks base method
func (m *MockHandler) ResetQueue(arg0 context.Context, arg1 *types.ResetQueueRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResetQueue", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ResetQueue indicates an expected call of ResetQueue
func (mr *MockHandlerMockRecorder) ResetQueue(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetQueue", reflect.TypeOf((*MockHandler)(nil).ResetQueue), arg0, arg1)
}

// ResetStickyTaskList mocks base method
func (m *MockHandler) ResetStickyTaskList(arg0 context.Context, arg1 *types.HistoryResetStickyTaskListRequest) (*types.HistoryResetStickyTaskListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResetStickyTaskList", arg0, arg1)
	ret0, _ := ret[0].(*types.HistoryResetStickyTaskListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ResetStickyTaskList indicates an expected call of ResetStickyTaskList
func (mr *MockHandlerMockRecorder) ResetStickyTaskList(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetStickyTaskList", reflect.TypeOf((*MockHandler)(nil).ResetStickyTaskList), arg0, arg1)
}

// ResetWorkflowExecution mocks base method
func (m *MockHandler) ResetWorkflowExecution(arg0 context.Context, arg1 *types.HistoryResetWorkflowExecutionRequest) (*types.ResetWorkflowExecutionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResetWorkflowExecution", arg0, arg1)
	ret0, _ := ret[0].(*types.ResetWorkflowExecutionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ResetWorkflowExecution indicates an expected call of ResetWorkflowExecution
func (mr *MockHandlerMockRecorder) ResetWorkflowExecution(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetWorkflowExecution", reflect.TypeOf((*MockHandler)(nil).ResetWorkflowExecution), arg0, arg1)
}

// RespondActivityTaskCanceled mocks base method
func (m *MockHandler) RespondActivityTaskCanceled(arg0 context.Context, arg1 *types.HistoryRespondActivityTaskCanceledRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondActivityTaskCanceled", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RespondActivityTaskCanceled indicates an expected call of RespondActivityTaskCanceled
func (mr *MockHandlerMockRecorder) RespondActivityTaskCanceled(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondActivityTaskCanceled", reflect.TypeOf((*MockHandler)(nil).RespondActivityTaskCanceled), arg0, arg1)
}

// RespondActivityTaskCompleted mocks base method
func (m *MockHandler) RespondActivityTaskCompleted(arg0 context.Context, arg1 *types.HistoryRespondActivityTaskCompletedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondActivityTaskCompleted", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RespondActivityTaskCompleted indicates an expected call of RespondActivityTaskCompleted
func (mr *MockHandlerMockRecorder) RespondActivityTaskCompleted(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondActivityTaskCompleted", reflect.TypeOf((*MockHandler)(nil).RespondActivityTaskCompleted), arg0, arg1)
}

// RespondActivityTaskFailed mocks base method
func (m *MockHandler) RespondActivityTaskFailed(arg0 context.Context, arg1 *types.HistoryRespondActivityTaskFailedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondActivityTaskFailed", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RespondActivityTaskFailed indicates an expected call of RespondActivityTaskFailed
func (mr *MockHandlerMockRecorder) RespondActivityTaskFailed(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondActivityTaskFailed", reflect.TypeOf((*MockHandler)(nil).RespondActivityTaskFailed), arg0, arg1)
}

// RespondCrossClusterTasksCompleted mocks base method
func (m *MockHandler) RespondCrossClusterTasksCompleted(arg0 context.Context, arg1 *types.RespondCrossClusterTasksCompletedRequest) (*types.RespondCrossClusterTasksCompletedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondCrossClusterTasksCompleted", arg0, arg1)
	ret0, _ := ret[0].(*types.RespondCrossClusterTasksCompletedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RespondCrossClusterTasksCompleted indicates an expected call of RespondCrossClusterTasksCompleted
func (mr *MockHandlerMockRecorder) RespondCrossClusterTasksCompleted(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondCrossClusterTasksCompleted", reflect.TypeOf((*MockHandler)(nil).RespondCrossClusterTasksCompleted), arg0, arg1)
}

// RespondDecisionTaskCompleted mocks base method
func (m *MockHandler) RespondDecisionTaskCompleted(arg0 context.Context, arg1 *types.HistoryRespondDecisionTaskCompletedRequest) (*types.HistoryRespondDecisionTaskCompletedResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondDecisionTaskCompleted", arg0, arg1)
	ret0, _ := ret[0].(*types.HistoryRespondDecisionTaskCompletedResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RespondDecisionTaskCompleted indicates an expected call of RespondDecisionTaskCompleted
func (mr *MockHandlerMockRecorder) RespondDecisionTaskCompleted(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondDecisionTaskCompleted", reflect.TypeOf((*MockHandler)(nil).RespondDecisionTaskCompleted), arg0, arg1)
}

// RespondDecisionTaskFailed mocks base method
func (m *MockHandler) RespondDecisionTaskFailed(arg0 context.Context, arg1 *types.HistoryRespondDecisionTaskFailedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RespondDecisionTaskFailed", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RespondDecisionTaskFailed indicates an expected call of RespondDecisionTaskFailed
func (mr *MockHandlerMockRecorder) RespondDecisionTaskFailed(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RespondDecisionTaskFailed", reflect.TypeOf((*MockHandler)(nil).RespondDecisionTaskFailed), arg0, arg1)
}

// ScheduleDecisionTask mocks base method
func (m *MockHandler) ScheduleDecisionTask(arg0 context.Context, arg1 *types.ScheduleDecisionTaskRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ScheduleDecisionTask", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ScheduleDecisionTask indicates an expected call of ScheduleDecisionTask
func (mr *MockHandlerMockRecorder) ScheduleDecisionTask(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScheduleDecisionTask", reflect.TypeOf((*MockHandler)(nil).ScheduleDecisionTask), arg0, arg1)
}

// SignalWithStartWorkflowExecution mocks base method
func (m *MockHandler) SignalWithStartWorkflowExecution(arg0 context.Context, arg1 *types.HistorySignalWithStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SignalWithStartWorkflowExecution", arg0, arg1)
	ret0, _ := ret[0].(*types.StartWorkflowExecutionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SignalWithStartWorkflowExecution indicates an expected call of SignalWithStartWorkflowExecution
func (mr *MockHandlerMockRecorder) SignalWithStartWorkflowExecution(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SignalWithStartWorkflowExecution", reflect.TypeOf((*MockHandler)(nil).SignalWithStartWorkflowExecution), arg0, arg1)
}

// SignalWorkflowExecution mocks base method
func (m *MockHandler) SignalWorkflowExecution(arg0 context.Context, arg1 *types.HistorySignalWorkflowExecutionRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SignalWorkflowExecution", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SignalWorkflowExecution indicates an expected call of SignalWorkflowExecution
func (mr *MockHandlerMockRecorder) SignalWorkflowExecution(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SignalWorkflowExecution", reflect.TypeOf((*MockHandler)(nil).SignalWorkflowExecution), arg0, arg1)
}

// StartWorkflowExecution mocks base method
func (m *MockHandler) StartWorkflowExecution(arg0 context.Context, arg1 *types.HistoryStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartWorkflowExecution", arg0, arg1)
	ret0, _ := ret[0].(*types.StartWorkflowExecutionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartWorkflowExecution indicates an expected call of StartWorkflowExecution
func (mr *MockHandlerMockRecorder) StartWorkflowExecution(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartWorkflowExecution", reflect.TypeOf((*MockHandler)(nil).StartWorkflowExecution), arg0, arg1)
}

// SyncActivity mocks base method
func (m *MockHandler) SyncActivity(arg0 context.Context, arg1 *types.SyncActivityRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncActivity", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SyncActivity indicates an expected call of SyncActivity
func (mr *MockHandlerMockRecorder) SyncActivity(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncActivity", reflect.TypeOf((*MockHandler)(nil).SyncActivity), arg0, arg1)
}

// SyncShardStatus mocks base method
func (m *MockHandler) SyncShardStatus(arg0 context.Context, arg1 *types.SyncShardStatusRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncShardStatus", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SyncShardStatus indicates an expected call of SyncShardStatus
func (mr *MockHandlerMockRecorder) SyncShardStatus(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncShardStatus", reflect.TypeOf((*MockHandler)(nil).SyncShardStatus), arg0, arg1)
}

// TerminateWorkflowExecution mocks base method
func (m *MockHandler) TerminateWorkflowExecution(arg0 context.Context, arg1 *types.HistoryTerminateWorkflowExecutionRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TerminateWorkflowExecution", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// TerminateWorkflowExecution indicates an expected call of TerminateWorkflowExecution
func (mr *MockHandlerMockRecorder) TerminateWorkflowExecution(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TerminateWorkflowExecution", reflect.TypeOf((*MockHandler)(nil).TerminateWorkflowExecution), arg0, arg1)
}
