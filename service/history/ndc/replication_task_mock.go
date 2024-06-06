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
// Source: replication_task.go

// Package ndc is a generated GoMock package.
package ndc

import (
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"

	log "github.com/uber/cadence/common/log"
	persistence "github.com/uber/cadence/common/persistence"
	types "github.com/uber/cadence/common/types"
)

// MockreplicationTask is a mock of replicationTask interface.
type MockreplicationTask struct {
	ctrl     *gomock.Controller
	recorder *MockreplicationTaskMockRecorder
}

// MockreplicationTaskMockRecorder is the mock recorder for MockreplicationTask.
type MockreplicationTaskMockRecorder struct {
	mock *MockreplicationTask
}

// NewMockreplicationTask creates a new mock instance.
func NewMockreplicationTask(ctrl *gomock.Controller) *MockreplicationTask {
	mock := &MockreplicationTask{ctrl: ctrl}
	mock.recorder = &MockreplicationTaskMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockreplicationTask) EXPECT() *MockreplicationTaskMockRecorder {
	return m.recorder
}

// getDomainID mocks base method.
func (m *MockreplicationTask) getDomainID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getDomainID")
	ret0, _ := ret[0].(string)
	return ret0
}

// getDomainID indicates an expected call of getDomainID.
func (mr *MockreplicationTaskMockRecorder) getDomainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getDomainID", reflect.TypeOf((*MockreplicationTask)(nil).getDomainID))
}

// getEventTime mocks base method.
func (m *MockreplicationTask) getEventTime() time.Time {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getEventTime")
	ret0, _ := ret[0].(time.Time)
	return ret0
}

// getEventTime indicates an expected call of getEventTime.
func (mr *MockreplicationTaskMockRecorder) getEventTime() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getEventTime", reflect.TypeOf((*MockreplicationTask)(nil).getEventTime))
}

// getEvents mocks base method.
func (m *MockreplicationTask) getEvents() []*types.HistoryEvent {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getEvents")
	ret0, _ := ret[0].([]*types.HistoryEvent)
	return ret0
}

// getEvents indicates an expected call of getEvents.
func (mr *MockreplicationTaskMockRecorder) getEvents() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getEvents", reflect.TypeOf((*MockreplicationTask)(nil).getEvents))
}

// getExecution mocks base method.
func (m *MockreplicationTask) getExecution() *types.WorkflowExecution {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getExecution")
	ret0, _ := ret[0].(*types.WorkflowExecution)
	return ret0
}

// getExecution indicates an expected call of getExecution.
func (mr *MockreplicationTaskMockRecorder) getExecution() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getExecution", reflect.TypeOf((*MockreplicationTask)(nil).getExecution))
}

// getFirstEvent mocks base method.
func (m *MockreplicationTask) getFirstEvent() *types.HistoryEvent {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getFirstEvent")
	ret0, _ := ret[0].(*types.HistoryEvent)
	return ret0
}

// getFirstEvent indicates an expected call of getFirstEvent.
func (mr *MockreplicationTaskMockRecorder) getFirstEvent() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getFirstEvent", reflect.TypeOf((*MockreplicationTask)(nil).getFirstEvent))
}

// getLastEvent mocks base method.
func (m *MockreplicationTask) getLastEvent() *types.HistoryEvent {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getLastEvent")
	ret0, _ := ret[0].(*types.HistoryEvent)
	return ret0
}

// getLastEvent indicates an expected call of getLastEvent.
func (mr *MockreplicationTaskMockRecorder) getLastEvent() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getLastEvent", reflect.TypeOf((*MockreplicationTask)(nil).getLastEvent))
}

// getLogger mocks base method.
func (m *MockreplicationTask) getLogger() log.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getLogger")
	ret0, _ := ret[0].(log.Logger)
	return ret0
}

// getLogger indicates an expected call of getLogger.
func (mr *MockreplicationTaskMockRecorder) getLogger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getLogger", reflect.TypeOf((*MockreplicationTask)(nil).getLogger))
}

// getNewEvents mocks base method.
func (m *MockreplicationTask) getNewEvents() []*types.HistoryEvent {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getNewEvents")
	ret0, _ := ret[0].([]*types.HistoryEvent)
	return ret0
}

// getNewEvents indicates an expected call of getNewEvents.
func (mr *MockreplicationTaskMockRecorder) getNewEvents() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getNewEvents", reflect.TypeOf((*MockreplicationTask)(nil).getNewEvents))
}

// getRunID mocks base method.
func (m *MockreplicationTask) getRunID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getRunID")
	ret0, _ := ret[0].(string)
	return ret0
}

// getRunID indicates an expected call of getRunID.
func (mr *MockreplicationTaskMockRecorder) getRunID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getRunID", reflect.TypeOf((*MockreplicationTask)(nil).getRunID))
}

// getSourceCluster mocks base method.
func (m *MockreplicationTask) getSourceCluster() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getSourceCluster")
	ret0, _ := ret[0].(string)
	return ret0
}

// getSourceCluster indicates an expected call of getSourceCluster.
func (mr *MockreplicationTaskMockRecorder) getSourceCluster() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getSourceCluster", reflect.TypeOf((*MockreplicationTask)(nil).getSourceCluster))
}

// getVersion mocks base method.
func (m *MockreplicationTask) getVersion() int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getVersion")
	ret0, _ := ret[0].(int64)
	return ret0
}

// getVersion indicates an expected call of getVersion.
func (mr *MockreplicationTaskMockRecorder) getVersion() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getVersion", reflect.TypeOf((*MockreplicationTask)(nil).getVersion))
}

// getVersionHistory mocks base method.
func (m *MockreplicationTask) getVersionHistory() *persistence.VersionHistory {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getVersionHistory")
	ret0, _ := ret[0].(*persistence.VersionHistory)
	return ret0
}

// getVersionHistory indicates an expected call of getVersionHistory.
func (mr *MockreplicationTaskMockRecorder) getVersionHistory() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getVersionHistory", reflect.TypeOf((*MockreplicationTask)(nil).getVersionHistory))
}

// getWorkflowID mocks base method.
func (m *MockreplicationTask) getWorkflowID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getWorkflowID")
	ret0, _ := ret[0].(string)
	return ret0
}

// getWorkflowID indicates an expected call of getWorkflowID.
func (mr *MockreplicationTaskMockRecorder) getWorkflowID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getWorkflowID", reflect.TypeOf((*MockreplicationTask)(nil).getWorkflowID))
}

// getWorkflowResetMetadata mocks base method.
func (m *MockreplicationTask) getWorkflowResetMetadata() (string, string, int64, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getWorkflowResetMetadata")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(int64)
	ret3, _ := ret[3].(bool)
	return ret0, ret1, ret2, ret3
}

// getWorkflowResetMetadata indicates an expected call of getWorkflowResetMetadata.
func (mr *MockreplicationTaskMockRecorder) getWorkflowResetMetadata() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getWorkflowResetMetadata", reflect.TypeOf((*MockreplicationTask)(nil).getWorkflowResetMetadata))
}

// isWorkflowReset mocks base method.
func (m *MockreplicationTask) isWorkflowReset() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "isWorkflowReset")
	ret0, _ := ret[0].(bool)
	return ret0
}

// isWorkflowReset indicates an expected call of isWorkflowReset.
func (mr *MockreplicationTaskMockRecorder) isWorkflowReset() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "isWorkflowReset", reflect.TypeOf((*MockreplicationTask)(nil).isWorkflowReset))
}

// splitTask mocks base method.
func (m *MockreplicationTask) splitTask(taskStartTime time.Time) (replicationTask, replicationTask, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "splitTask", taskStartTime)
	ret0, _ := ret[0].(replicationTask)
	ret1, _ := ret[1].(replicationTask)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// splitTask indicates an expected call of splitTask.
func (mr *MockreplicationTaskMockRecorder) splitTask(taskStartTime interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "splitTask", reflect.TypeOf((*MockreplicationTask)(nil).splitTask), taskStartTime)
}
