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
// Source: dlq_handler.go

// Package replication is a generated GoMock package.
package replication

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "github.com/uber/cadence/common/types"
)

// MockDLQHandler is a mock of DLQHandler interface.
type MockDLQHandler struct {
	ctrl     *gomock.Controller
	recorder *MockDLQHandlerMockRecorder
}

// MockDLQHandlerMockRecorder is the mock recorder for MockDLQHandler.
type MockDLQHandlerMockRecorder struct {
	mock *MockDLQHandler
}

// NewMockDLQHandler creates a new mock instance.
func NewMockDLQHandler(ctrl *gomock.Controller) *MockDLQHandler {
	mock := &MockDLQHandler{ctrl: ctrl}
	mock.recorder = &MockDLQHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDLQHandler) EXPECT() *MockDLQHandlerMockRecorder {
	return m.recorder
}

// GetMessageCount mocks base method.
func (m *MockDLQHandler) GetMessageCount(ctx context.Context, forceFetch bool) (map[string]int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMessageCount", ctx, forceFetch)
	ret0, _ := ret[0].(map[string]int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMessageCount indicates an expected call of GetMessageCount.
func (mr *MockDLQHandlerMockRecorder) GetMessageCount(ctx, forceFetch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMessageCount", reflect.TypeOf((*MockDLQHandler)(nil).GetMessageCount), ctx, forceFetch)
}

// MergeMessages mocks base method.
func (m *MockDLQHandler) MergeMessages(ctx context.Context, sourceCluster string, lastMessageID int64, pageSize int, pageToken []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MergeMessages", ctx, sourceCluster, lastMessageID, pageSize, pageToken)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MergeMessages indicates an expected call of MergeMessages.
func (mr *MockDLQHandlerMockRecorder) MergeMessages(ctx, sourceCluster, lastMessageID, pageSize, pageToken interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MergeMessages", reflect.TypeOf((*MockDLQHandler)(nil).MergeMessages), ctx, sourceCluster, lastMessageID, pageSize, pageToken)
}

// PurgeMessages mocks base method.
func (m *MockDLQHandler) PurgeMessages(ctx context.Context, sourceCluster string, lastMessageID int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PurgeMessages", ctx, sourceCluster, lastMessageID)
	ret0, _ := ret[0].(error)
	return ret0
}

// PurgeMessages indicates an expected call of PurgeMessages.
func (mr *MockDLQHandlerMockRecorder) PurgeMessages(ctx, sourceCluster, lastMessageID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PurgeMessages", reflect.TypeOf((*MockDLQHandler)(nil).PurgeMessages), ctx, sourceCluster, lastMessageID)
}

// ReadMessages mocks base method.
func (m *MockDLQHandler) ReadMessages(ctx context.Context, sourceCluster string, lastMessageID int64, pageSize int, pageToken []byte) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadMessages", ctx, sourceCluster, lastMessageID, pageSize, pageToken)
	ret0, _ := ret[0].([]*types.ReplicationTask)
	ret1, _ := ret[1].([]*types.ReplicationTaskInfo)
	ret2, _ := ret[2].([]byte)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// ReadMessages indicates an expected call of ReadMessages.
func (mr *MockDLQHandlerMockRecorder) ReadMessages(ctx, sourceCluster, lastMessageID, pageSize, pageToken interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadMessages", reflect.TypeOf((*MockDLQHandler)(nil).ReadMessages), ctx, sourceCluster, lastMessageID, pageSize, pageToken)
}

// Start mocks base method.
func (m *MockDLQHandler) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *MockDLQHandlerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockDLQHandler)(nil).Start))
}

// Stop mocks base method.
func (m *MockDLQHandler) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockDLQHandlerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockDLQHandler)(nil).Stop))
}
