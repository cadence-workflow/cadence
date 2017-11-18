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

// Code generated by thriftrw-plugin-yarpc
// @generated

package historyservicetest

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/history/historyserviceclient"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/yarpc"
)

// MockClient implements a gomock-compatible mock client for service
// HistoryService.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *_MockClientRecorder
}

var _ historyserviceclient.Interface = (*MockClient)(nil)

type _MockClientRecorder struct {
	mock *MockClient
}

// Build a new mock client for service HistoryService.
//
// 	mockCtrl := gomock.NewController(t)
// 	client := historyservicetest.NewMockClient(mockCtrl)
//
// Use EXPECT() to set expectations on the mock.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &_MockClientRecorder{mock}
	return mock
}

// EXPECT returns an object that allows you to define an expectation on the
// HistoryService mock client.
func (m *MockClient) EXPECT() *_MockClientRecorder {
	return m.recorder
}

// DescribeWorkflowExecution responds to a DescribeWorkflowExecution call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().DescribeWorkflowExecution(gomock.Any(), ...).Return(...)
// 	... := client.DescribeWorkflowExecution(...)
func (m *MockClient) DescribeWorkflowExecution(
	ctx context.Context,
	_DescribeRequest *history.DescribeWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (success *history.DescribeWorkflowExecutionResponse, err error) {

	args := []interface{}{ctx, _DescribeRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "DescribeWorkflowExecution", args...)
	success, _ = ret[i].(*history.DescribeWorkflowExecutionResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) DescribeWorkflowExecution(
	ctx interface{},
	_DescribeRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _DescribeRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "DescribeWorkflowExecution", args...)
}

// RecordActivityTaskHeartbeat responds to a RecordActivityTaskHeartbeat call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), ...).Return(...)
// 	... := client.RecordActivityTaskHeartbeat(...)
func (m *MockClient) RecordActivityTaskHeartbeat(
	ctx context.Context,
	_HeartbeatRequest *history.RecordActivityTaskHeartbeatRequest,
	opts ...yarpc.CallOption,
) (success *shared.RecordActivityTaskHeartbeatResponse, err error) {

	args := []interface{}{ctx, _HeartbeatRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RecordActivityTaskHeartbeat", args...)
	success, _ = ret[i].(*shared.RecordActivityTaskHeartbeatResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RecordActivityTaskHeartbeat(
	ctx interface{},
	_HeartbeatRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _HeartbeatRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RecordActivityTaskHeartbeat", args...)
}

// RecordActivityTaskStarted responds to a RecordActivityTaskStarted call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RecordActivityTaskStarted(gomock.Any(), ...).Return(...)
// 	... := client.RecordActivityTaskStarted(...)
func (m *MockClient) RecordActivityTaskStarted(
	ctx context.Context,
	_AddRequest *history.RecordActivityTaskStartedRequest,
	opts ...yarpc.CallOption,
) (success *history.RecordActivityTaskStartedResponse, err error) {

	args := []interface{}{ctx, _AddRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RecordActivityTaskStarted", args...)
	success, _ = ret[i].(*history.RecordActivityTaskStartedResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RecordActivityTaskStarted(
	ctx interface{},
	_AddRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _AddRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RecordActivityTaskStarted", args...)
}

// RecordChildExecutionCompleted responds to a RecordChildExecutionCompleted call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RecordChildExecutionCompleted(gomock.Any(), ...).Return(...)
// 	... := client.RecordChildExecutionCompleted(...)
func (m *MockClient) RecordChildExecutionCompleted(
	ctx context.Context,
	_CompletionRequest *history.RecordChildExecutionCompletedRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _CompletionRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RecordChildExecutionCompleted", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RecordChildExecutionCompleted(
	ctx interface{},
	_CompletionRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _CompletionRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RecordChildExecutionCompleted", args...)
}

// RecordDecisionTaskStarted responds to a RecordDecisionTaskStarted call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RecordDecisionTaskStarted(gomock.Any(), ...).Return(...)
// 	... := client.RecordDecisionTaskStarted(...)
func (m *MockClient) RecordDecisionTaskStarted(
	ctx context.Context,
	_AddRequest *history.RecordDecisionTaskStartedRequest,
	opts ...yarpc.CallOption,
) (success *history.RecordDecisionTaskStartedResponse, err error) {

	args := []interface{}{ctx, _AddRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RecordDecisionTaskStarted", args...)
	success, _ = ret[i].(*history.RecordDecisionTaskStartedResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RecordDecisionTaskStarted(
	ctx interface{},
	_AddRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _AddRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RecordDecisionTaskStarted", args...)
}

// RequestCancelWorkflowExecution responds to a RequestCancelWorkflowExecution call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), ...).Return(...)
// 	... := client.RequestCancelWorkflowExecution(...)
func (m *MockClient) RequestCancelWorkflowExecution(
	ctx context.Context,
	_CancelRequest *history.RequestCancelWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _CancelRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RequestCancelWorkflowExecution", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RequestCancelWorkflowExecution(
	ctx interface{},
	_CancelRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _CancelRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RequestCancelWorkflowExecution", args...)
}

// RespondActivityTaskCanceled responds to a RespondActivityTaskCanceled call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RespondActivityTaskCanceled(gomock.Any(), ...).Return(...)
// 	... := client.RespondActivityTaskCanceled(...)
func (m *MockClient) RespondActivityTaskCanceled(
	ctx context.Context,
	_CanceledRequest *history.RespondActivityTaskCanceledRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _CanceledRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RespondActivityTaskCanceled", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RespondActivityTaskCanceled(
	ctx interface{},
	_CanceledRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _CanceledRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RespondActivityTaskCanceled", args...)
}

// RespondActivityTaskCompleted responds to a RespondActivityTaskCompleted call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RespondActivityTaskCompleted(gomock.Any(), ...).Return(...)
// 	... := client.RespondActivityTaskCompleted(...)
func (m *MockClient) RespondActivityTaskCompleted(
	ctx context.Context,
	_CompleteRequest *history.RespondActivityTaskCompletedRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _CompleteRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RespondActivityTaskCompleted", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RespondActivityTaskCompleted(
	ctx interface{},
	_CompleteRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _CompleteRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RespondActivityTaskCompleted", args...)
}

// RespondActivityTaskFailed responds to a RespondActivityTaskFailed call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RespondActivityTaskFailed(gomock.Any(), ...).Return(...)
// 	... := client.RespondActivityTaskFailed(...)
func (m *MockClient) RespondActivityTaskFailed(
	ctx context.Context,
	_FailRequest *history.RespondActivityTaskFailedRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _FailRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RespondActivityTaskFailed", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RespondActivityTaskFailed(
	ctx interface{},
	_FailRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _FailRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RespondActivityTaskFailed", args...)
}

// RespondDecisionTaskCompleted responds to a RespondDecisionTaskCompleted call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), ...).Return(...)
// 	... := client.RespondDecisionTaskCompleted(...)
func (m *MockClient) RespondDecisionTaskCompleted(
	ctx context.Context,
	_CompleteRequest *history.RespondDecisionTaskCompletedRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _CompleteRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RespondDecisionTaskCompleted", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RespondDecisionTaskCompleted(
	ctx interface{},
	_CompleteRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _CompleteRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RespondDecisionTaskCompleted", args...)
}

// ScheduleDecisionTask responds to a ScheduleDecisionTask call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().ScheduleDecisionTask(gomock.Any(), ...).Return(...)
// 	... := client.ScheduleDecisionTask(...)
func (m *MockClient) ScheduleDecisionTask(
	ctx context.Context,
	_ScheduleRequest *history.ScheduleDecisionTaskRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _ScheduleRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "ScheduleDecisionTask", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) ScheduleDecisionTask(
	ctx interface{},
	_ScheduleRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _ScheduleRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "ScheduleDecisionTask", args...)
}

// SignalWorkflowExecution responds to a SignalWorkflowExecution call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().SignalWorkflowExecution(gomock.Any(), ...).Return(...)
// 	... := client.SignalWorkflowExecution(...)
func (m *MockClient) SignalWorkflowExecution(
	ctx context.Context,
	_SignalRequest *history.SignalWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _SignalRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "SignalWorkflowExecution", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) SignalWorkflowExecution(
	ctx interface{},
	_SignalRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _SignalRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "SignalWorkflowExecution", args...)
}

// StartWorkflowExecution responds to a StartWorkflowExecution call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().StartWorkflowExecution(gomock.Any(), ...).Return(...)
// 	... := client.StartWorkflowExecution(...)
func (m *MockClient) StartWorkflowExecution(
	ctx context.Context,
	_StartRequest *history.StartWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (success *shared.StartWorkflowExecutionResponse, err error) {

	args := []interface{}{ctx, _StartRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "StartWorkflowExecution", args...)
	success, _ = ret[i].(*shared.StartWorkflowExecutionResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) StartWorkflowExecution(
	ctx interface{},
	_StartRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _StartRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "StartWorkflowExecution", args...)
}

// TerminateWorkflowExecution responds to a TerminateWorkflowExecution call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().TerminateWorkflowExecution(gomock.Any(), ...).Return(...)
// 	... := client.TerminateWorkflowExecution(...)
func (m *MockClient) TerminateWorkflowExecution(
	ctx context.Context,
	_TerminateRequest *history.TerminateWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _TerminateRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "TerminateWorkflowExecution", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) TerminateWorkflowExecution(
	ctx interface{},
	_TerminateRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _TerminateRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "TerminateWorkflowExecution", args...)
}
