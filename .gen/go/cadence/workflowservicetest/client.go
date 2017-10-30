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

package workflowservicetest

import (
	"context"

	"github.com/golang/mock/gomock"
	"github.com/uber/cadence/.gen/go/cadence/workflowserviceclient"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/yarpc"
)

// MockClient implements a gomock-compatible mock client for service
// WorkflowService.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *_MockClientRecorder
}

var _ workflowserviceclient.Interface = (*MockClient)(nil)

type _MockClientRecorder struct {
	mock *MockClient
}

// Build a new mock client for service WorkflowService.
//
// 	mockCtrl := gomock.NewController(t)
// 	client := workflowservicetest.NewMockClient(mockCtrl)
//
// Use EXPECT() to set expectations on the mock.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &_MockClientRecorder{mock}
	return mock
}

// EXPECT returns an object that allows you to define an expectation on the
// WorkflowService mock client.
func (m *MockClient) EXPECT() *_MockClientRecorder {
	return m.recorder
}

// DeprecateDomain responds to a DeprecateDomain call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().DeprecateDomain(gomock.Any(), ...).Return(...)
// 	... := client.DeprecateDomain(...)
func (m *MockClient) DeprecateDomain(
	ctx context.Context,
	_DeprecateRequest *shared.DeprecateDomainRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _DeprecateRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "DeprecateDomain", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) DeprecateDomain(
	ctx interface{},
	_DeprecateRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _DeprecateRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "DeprecateDomain", args...)
}

// DescribeDomain responds to a DescribeDomain call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().DescribeDomain(gomock.Any(), ...).Return(...)
// 	... := client.DescribeDomain(...)
func (m *MockClient) DescribeDomain(
	ctx context.Context,
	_DescribeRequest *shared.DescribeDomainRequest,
	opts ...yarpc.CallOption,
) (success *shared.DescribeDomainResponse, err error) {

	args := []interface{}{ctx, _DescribeRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "DescribeDomain", args...)
	success, _ = ret[i].(*shared.DescribeDomainResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) DescribeDomain(
	ctx interface{},
	_DescribeRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _DescribeRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "DescribeDomain", args...)
}

// GetWorkflowExecutionHistory responds to a GetWorkflowExecutionHistory call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), ...).Return(...)
// 	... := client.GetWorkflowExecutionHistory(...)
func (m *MockClient) GetWorkflowExecutionHistory(
	ctx context.Context,
	_GetRequest *shared.GetWorkflowExecutionHistoryRequest,
	opts ...yarpc.CallOption,
) (success *shared.GetWorkflowExecutionHistoryResponse, err error) {

	args := []interface{}{ctx, _GetRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "GetWorkflowExecutionHistory", args...)
	success, _ = ret[i].(*shared.GetWorkflowExecutionHistoryResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) GetWorkflowExecutionHistory(
	ctx interface{},
	_GetRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _GetRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "GetWorkflowExecutionHistory", args...)
}

// ListClosedWorkflowExecutions responds to a ListClosedWorkflowExecutions call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), ...).Return(...)
// 	... := client.ListClosedWorkflowExecutions(...)
func (m *MockClient) ListClosedWorkflowExecutions(
	ctx context.Context,
	_ListRequest *shared.ListClosedWorkflowExecutionsRequest,
	opts ...yarpc.CallOption,
) (success *shared.ListClosedWorkflowExecutionsResponse, err error) {

	args := []interface{}{ctx, _ListRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "ListClosedWorkflowExecutions", args...)
	success, _ = ret[i].(*shared.ListClosedWorkflowExecutionsResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) ListClosedWorkflowExecutions(
	ctx interface{},
	_ListRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _ListRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "ListClosedWorkflowExecutions", args...)
}

// ListOpenWorkflowExecutions responds to a ListOpenWorkflowExecutions call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), ...).Return(...)
// 	... := client.ListOpenWorkflowExecutions(...)
func (m *MockClient) ListOpenWorkflowExecutions(
	ctx context.Context,
	_ListRequest *shared.ListOpenWorkflowExecutionsRequest,
	opts ...yarpc.CallOption,
) (success *shared.ListOpenWorkflowExecutionsResponse, err error) {

	args := []interface{}{ctx, _ListRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "ListOpenWorkflowExecutions", args...)
	success, _ = ret[i].(*shared.ListOpenWorkflowExecutionsResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) ListOpenWorkflowExecutions(
	ctx interface{},
	_ListRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _ListRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "ListOpenWorkflowExecutions", args...)
}

// PollForActivityTask responds to a PollForActivityTask call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().PollForActivityTask(gomock.Any(), ...).Return(...)
// 	... := client.PollForActivityTask(...)
func (m *MockClient) PollForActivityTask(
	ctx context.Context,
	_PollRequest *shared.PollForActivityTaskRequest,
	opts ...yarpc.CallOption,
) (success *shared.PollForActivityTaskResponse, err error) {

	args := []interface{}{ctx, _PollRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "PollForActivityTask", args...)
	success, _ = ret[i].(*shared.PollForActivityTaskResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) PollForActivityTask(
	ctx interface{},
	_PollRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _PollRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "PollForActivityTask", args...)
}

// PollForDecisionTask responds to a PollForDecisionTask call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().PollForDecisionTask(gomock.Any(), ...).Return(...)
// 	... := client.PollForDecisionTask(...)
func (m *MockClient) PollForDecisionTask(
	ctx context.Context,
	_PollRequest *shared.PollForDecisionTaskRequest,
	opts ...yarpc.CallOption,
) (success *shared.PollForDecisionTaskResponse, err error) {

	args := []interface{}{ctx, _PollRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "PollForDecisionTask", args...)
	success, _ = ret[i].(*shared.PollForDecisionTaskResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) PollForDecisionTask(
	ctx interface{},
	_PollRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _PollRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "PollForDecisionTask", args...)
}

// QueryWorkflow responds to a QueryWorkflow call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().QueryWorkflow(gomock.Any(), ...).Return(...)
// 	... := client.QueryWorkflow(...)
func (m *MockClient) QueryWorkflow(
	ctx context.Context,
	_QueryRequest *shared.QueryWorkflowRequest,
	opts ...yarpc.CallOption,
) (success *shared.QueryWorkflowResponse, err error) {

	args := []interface{}{ctx, _QueryRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "QueryWorkflow", args...)
	success, _ = ret[i].(*shared.QueryWorkflowResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) QueryWorkflow(
	ctx interface{},
	_QueryRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _QueryRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "QueryWorkflow", args...)
}

// RecordActivityTaskHeartbeat responds to a RecordActivityTaskHeartbeat call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), ...).Return(...)
// 	... := client.RecordActivityTaskHeartbeat(...)
func (m *MockClient) RecordActivityTaskHeartbeat(
	ctx context.Context,
	_HeartbeatRequest *shared.RecordActivityTaskHeartbeatRequest,
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

// RegisterDomain responds to a RegisterDomain call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RegisterDomain(gomock.Any(), ...).Return(...)
// 	... := client.RegisterDomain(...)
func (m *MockClient) RegisterDomain(
	ctx context.Context,
	_RegisterRequest *shared.RegisterDomainRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _RegisterRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RegisterDomain", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RegisterDomain(
	ctx interface{},
	_RegisterRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _RegisterRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RegisterDomain", args...)
}

// RequestCancelWorkflowExecution responds to a RequestCancelWorkflowExecution call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), ...).Return(...)
// 	... := client.RequestCancelWorkflowExecution(...)
func (m *MockClient) RequestCancelWorkflowExecution(
	ctx context.Context,
	_CancelRequest *shared.RequestCancelWorkflowExecutionRequest,
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
	_CanceledRequest *shared.RespondActivityTaskCanceledRequest,
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
	_CompleteRequest *shared.RespondActivityTaskCompletedRequest,
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
	_FailRequest *shared.RespondActivityTaskFailedRequest,
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
	_CompleteRequest *shared.RespondDecisionTaskCompletedRequest,
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

// RespondQueryTaskCompleted responds to a RespondQueryTaskCompleted call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().RespondQueryTaskCompleted(gomock.Any(), ...).Return(...)
// 	... := client.RespondQueryTaskCompleted(...)
func (m *MockClient) RespondQueryTaskCompleted(
	ctx context.Context,
	_CompleteRequest *shared.RespondQueryTaskCompletedRequest,
	opts ...yarpc.CallOption,
) (err error) {

	args := []interface{}{ctx, _CompleteRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "RespondQueryTaskCompleted", args...)
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) RespondQueryTaskCompleted(
	ctx interface{},
	_CompleteRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _CompleteRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "RespondQueryTaskCompleted", args...)
}

// SignalWorkflowExecution responds to a SignalWorkflowExecution call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().SignalWorkflowExecution(gomock.Any(), ...).Return(...)
// 	... := client.SignalWorkflowExecution(...)
func (m *MockClient) SignalWorkflowExecution(
	ctx context.Context,
	_SignalRequest *shared.SignalWorkflowExecutionRequest,
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
	_StartRequest *shared.StartWorkflowExecutionRequest,
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
	_TerminateRequest *shared.TerminateWorkflowExecutionRequest,
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

// UpdateDomain responds to a UpdateDomain call based on the mock expectations. This
// call will fail if the mock does not expect this call. Use EXPECT to expect
// a call to this function.
//
// 	client.EXPECT().UpdateDomain(gomock.Any(), ...).Return(...)
// 	... := client.UpdateDomain(...)
func (m *MockClient) UpdateDomain(
	ctx context.Context,
	_UpdateRequest *shared.UpdateDomainRequest,
	opts ...yarpc.CallOption,
) (success *shared.UpdateDomainResponse, err error) {

	args := []interface{}{ctx, _UpdateRequest}
	for _, o := range opts {
		args = append(args, o)
	}
	i := 0
	ret := m.ctrl.Call(m, "UpdateDomain", args...)
	success, _ = ret[i].(*shared.UpdateDomainResponse)
	i++
	err, _ = ret[i].(error)
	return
}

func (mr *_MockClientRecorder) UpdateDomain(
	ctx interface{},
	_UpdateRequest interface{},
	opts ...interface{},
) *gomock.Call {
	args := append([]interface{}{ctx, _UpdateRequest}, opts...)
	return mr.mock.ctrl.RecordCall(mr.mock, "UpdateDomain", args...)
}
