// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
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
//

// Code generated by MockGen. DO NOT EDIT.
// Source: Context.go

// Package history is a generated GoMock package.
package execution

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"

	shared "github.com/uber/cadence/.gen/go/shared"
	persistence "github.com/uber/cadence/common/persistence"
)

// MockContext is a mock of Context interface
type MockContext struct {
	ctrl     *gomock.Controller
	recorder *MockContextMockRecorder
}

// MockContextMockRecorder is the mock recorder for MockContext
type MockContextMockRecorder struct {
	mock *MockContext
}

// NewMockContext creates a new mock instance
func NewMockContext(ctrl *gomock.Controller) *MockContext {
	mock := &MockContext{ctrl: ctrl}
	mock.recorder = &MockContextMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockContext) EXPECT() *MockContextMockRecorder {
	return m.recorder
}

// GetDomainName mocks base method
func (m *MockContext) GetDomainName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDomainName")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetDomainName indicates an expected call of GetDomainName
func (mr *MockContextMockRecorder) GetDomainName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDomainName", reflect.TypeOf((*MockContext)(nil).GetDomainName))
}

// GetDomainID mocks base method
func (m *MockContext) GetDomainID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDomainID")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetDomainID indicates an expected call of GetDomainID
func (mr *MockContextMockRecorder) GetDomainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDomainID", reflect.TypeOf((*MockContext)(nil).GetDomainID))
}

// GetExecution mocks base method
func (m *MockContext) GetExecution() *shared.WorkflowExecution {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetExecution")
	ret0, _ := ret[0].(*shared.WorkflowExecution)
	return ret0
}

// GetExecution indicates an expected call of GetExecution
func (mr *MockContextMockRecorder) GetExecution() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecution", reflect.TypeOf((*MockContext)(nil).GetExecution))
}

// GetWorkflowExecution mocks base method
func (m *MockContext) GetWorkflowExecution() MutableState {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetWorkflowExecution")
	ret0, _ := ret[0].(MutableState)
	return ret0
}

// GetWorkflowExecution indicates an expected call of GetWorkflowExecution
func (mr *MockContextMockRecorder) GetWorkflowExecution() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetWorkflowExecution", reflect.TypeOf((*MockContext)(nil).GetWorkflowExecution))
}

// SetWorkflowExecution mocks base method
func (m *MockContext) SetWorkflowExecution(mutableState MutableState) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetWorkflowExecution", mutableState)
}

// SetWorkflowExecution indicates an expected call of SetWorkflowExecution
func (mr *MockContextMockRecorder) SetWorkflowExecution(mutableState interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetWorkflowExecution", reflect.TypeOf((*MockContext)(nil).SetWorkflowExecution), mutableState)
}

// LoadWorkflowExecution mocks base method
func (m *MockContext) LoadWorkflowExecution() (MutableState, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadWorkflowExecution")
	ret0, _ := ret[0].(MutableState)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadWorkflowExecution indicates an expected call of LoadWorkflowExecution
func (mr *MockContextMockRecorder) LoadWorkflowExecution() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadWorkflowExecution", reflect.TypeOf((*MockContext)(nil).LoadWorkflowExecution))
}

// LoadWorkflowExecutionForReplication mocks base method
func (m *MockContext) LoadWorkflowExecutionForReplication(incomingVersion int64) (MutableState, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadWorkflowExecutionForReplication", incomingVersion)
	ret0, _ := ret[0].(MutableState)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadWorkflowExecutionForReplication indicates an expected call of LoadWorkflowExecutionForReplication
func (mr *MockContextMockRecorder) LoadWorkflowExecutionForReplication(incomingVersion interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadWorkflowExecutionForReplication", reflect.TypeOf((*MockContext)(nil).LoadWorkflowExecutionForReplication), incomingVersion)
}

// LoadExecutionStats mocks base method
func (m *MockContext) LoadExecutionStats() (*persistence.ExecutionStats, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadExecutionStats")
	ret0, _ := ret[0].(*persistence.ExecutionStats)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadExecutionStats indicates an expected call of LoadExecutionStats
func (mr *MockContextMockRecorder) LoadExecutionStats() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadExecutionStats", reflect.TypeOf((*MockContext)(nil).LoadExecutionStats))
}

// Clear mocks base method
func (m *MockContext) Clear() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Clear")
}

// Clear indicates an expected call of Clear
func (mr *MockContextMockRecorder) Clear() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clear", reflect.TypeOf((*MockContext)(nil).Clear))
}

// Lock mocks base method
func (m *MockContext) Lock(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Lock", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Lock indicates an expected call of Lock
func (mr *MockContextMockRecorder) Lock(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lock", reflect.TypeOf((*MockContext)(nil).Lock), ctx)
}

// Unlock mocks base method
func (m *MockContext) Unlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Unlock")
}

// Unlock indicates an expected call of Unlock
func (mr *MockContextMockRecorder) Unlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockContext)(nil).Unlock))
}

// GetHistorySize mocks base method
func (m *MockContext) GetHistorySize() int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHistorySize")
	ret0, _ := ret[0].(int64)
	return ret0
}

// GetHistorySize indicates an expected call of GetHistorySize
func (mr *MockContextMockRecorder) GetHistorySize() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHistorySize", reflect.TypeOf((*MockContext)(nil).GetHistorySize))
}

// SetHistorySize mocks base method
func (m *MockContext) SetHistorySize(size int64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetHistorySize", size)
}

// SetHistorySize indicates an expected call of SetHistorySize
func (mr *MockContextMockRecorder) SetHistorySize(size interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHistorySize", reflect.TypeOf((*MockContext)(nil).SetHistorySize), size)
}

// ReapplyEvents mocks base method
func (m *MockContext) ReapplyEvents(eventBatches []*persistence.WorkflowEvents) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReapplyEvents", eventBatches)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReapplyEvents indicates an expected call of ReapplyEvents
func (mr *MockContextMockRecorder) ReapplyEvents(eventBatches interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReapplyEvents", reflect.TypeOf((*MockContext)(nil).ReapplyEvents), eventBatches)
}

// PersistFirstWorkflowEvents mocks base method
func (m *MockContext) PersistFirstWorkflowEvents(workflowEvents *persistence.WorkflowEvents) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PersistFirstWorkflowEvents", workflowEvents)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PersistFirstWorkflowEvents indicates an expected call of PersistFirstWorkflowEvents
func (mr *MockContextMockRecorder) PersistFirstWorkflowEvents(workflowEvents interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PersistFirstWorkflowEvents", reflect.TypeOf((*MockContext)(nil).PersistFirstWorkflowEvents), workflowEvents)
}

// PersistNonFirstWorkflowEvents mocks base method
func (m *MockContext) PersistNonFirstWorkflowEvents(workflowEvents *persistence.WorkflowEvents) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PersistNonFirstWorkflowEvents", workflowEvents)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PersistNonFirstWorkflowEvents indicates an expected call of PersistNonFirstWorkflowEvents
func (mr *MockContextMockRecorder) PersistNonFirstWorkflowEvents(workflowEvents interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PersistNonFirstWorkflowEvents", reflect.TypeOf((*MockContext)(nil).PersistNonFirstWorkflowEvents), workflowEvents)
}

// CreateWorkflowExecution mocks base method
func (m *MockContext) CreateWorkflowExecution(newWorkflow *persistence.WorkflowSnapshot, historySize int64, now time.Time, createMode persistence.CreateWorkflowMode, prevRunID string, prevLastWriteVersion int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateWorkflowExecution", newWorkflow, historySize, now, createMode, prevRunID, prevLastWriteVersion)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateWorkflowExecution indicates an expected call of CreateWorkflowExecution
func (mr *MockContextMockRecorder) CreateWorkflowExecution(newWorkflow, historySize, now, createMode, prevRunID, prevLastWriteVersion interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateWorkflowExecution", reflect.TypeOf((*MockContext)(nil).CreateWorkflowExecution), newWorkflow, historySize, now, createMode, prevRunID, prevLastWriteVersion)
}

// ConflictResolveWorkflowExecution mocks base method
func (m *MockContext) ConflictResolveWorkflowExecution(now time.Time, conflictResolveMode persistence.ConflictResolveWorkflowMode, resetMutableState MutableState, newContext Context, newMutableState MutableState, currentContext Context, currentMutableState MutableState, currentTransactionPolicy *TransactionPolicy, workflowCAS *persistence.CurrentWorkflowCAS) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConflictResolveWorkflowExecution", now, conflictResolveMode, resetMutableState, newContext, newMutableState, currentContext, currentMutableState, currentTransactionPolicy, workflowCAS)
	ret0, _ := ret[0].(error)
	return ret0
}

// ConflictResolveWorkflowExecution indicates an expected call of ConflictResolveWorkflowExecution
func (mr *MockContextMockRecorder) ConflictResolveWorkflowExecution(now, conflictResolveMode, resetMutableState, newContext, newMutableState, currentContext, currentMutableState, currentTransactionPolicy, workflowCAS interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConflictResolveWorkflowExecution", reflect.TypeOf((*MockContext)(nil).ConflictResolveWorkflowExecution), now, conflictResolveMode, resetMutableState, newContext, newMutableState, currentContext, currentMutableState, currentTransactionPolicy, workflowCAS)
}

// UpdateWorkflowExecutionAsActive mocks base method
func (m *MockContext) UpdateWorkflowExecutionAsActive(now time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateWorkflowExecutionAsActive", now)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateWorkflowExecutionAsActive indicates an expected call of UpdateWorkflowExecutionAsActive
func (mr *MockContextMockRecorder) UpdateWorkflowExecutionAsActive(now interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateWorkflowExecutionAsActive", reflect.TypeOf((*MockContext)(nil).UpdateWorkflowExecutionAsActive), now)
}

// UpdateWorkflowExecutionWithNewAsActive mocks base method
func (m *MockContext) UpdateWorkflowExecutionWithNewAsActive(now time.Time, newContext Context, newMutableState MutableState) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateWorkflowExecutionWithNewAsActive", now, newContext, newMutableState)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateWorkflowExecutionWithNewAsActive indicates an expected call of UpdateWorkflowExecutionWithNewAsActive
func (mr *MockContextMockRecorder) UpdateWorkflowExecutionWithNewAsActive(now, newContext, newMutableState interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateWorkflowExecutionWithNewAsActive", reflect.TypeOf((*MockContext)(nil).UpdateWorkflowExecutionWithNewAsActive), now, newContext, newMutableState)
}

// UpdateWorkflowExecutionAsPassive mocks base method
func (m *MockContext) UpdateWorkflowExecutionAsPassive(now time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateWorkflowExecutionAsPassive", now)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateWorkflowExecutionAsPassive indicates an expected call of UpdateWorkflowExecutionAsPassive
func (mr *MockContextMockRecorder) UpdateWorkflowExecutionAsPassive(now interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateWorkflowExecutionAsPassive", reflect.TypeOf((*MockContext)(nil).UpdateWorkflowExecutionAsPassive), now)
}

// UpdateWorkflowExecutionWithNewAsPassive mocks base method
func (m *MockContext) UpdateWorkflowExecutionWithNewAsPassive(now time.Time, newContext Context, newMutableState MutableState) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateWorkflowExecutionWithNewAsPassive", now, newContext, newMutableState)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateWorkflowExecutionWithNewAsPassive indicates an expected call of UpdateWorkflowExecutionWithNewAsPassive
func (mr *MockContextMockRecorder) UpdateWorkflowExecutionWithNewAsPassive(now, newContext, newMutableState interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateWorkflowExecutionWithNewAsPassive", reflect.TypeOf((*MockContext)(nil).UpdateWorkflowExecutionWithNewAsPassive), now, newContext, newMutableState)
}

// UpdateWorkflowExecutionWithNew mocks base method
func (m *MockContext) UpdateWorkflowExecutionWithNew(now time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentWorkflowTransactionPolicy TransactionPolicy, newWorkflowTransactionPolicy *TransactionPolicy) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateWorkflowExecutionWithNew", now, updateMode, newContext, newMutableState, currentWorkflowTransactionPolicy, newWorkflowTransactionPolicy)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateWorkflowExecutionWithNew indicates an expected call of UpdateWorkflowExecutionWithNew
func (mr *MockContextMockRecorder) UpdateWorkflowExecutionWithNew(now, updateMode, newContext, newMutableState, currentWorkflowTransactionPolicy, newWorkflowTransactionPolicy interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateWorkflowExecutionWithNew", reflect.TypeOf((*MockContext)(nil).UpdateWorkflowExecutionWithNew), now, updateMode, newContext, newMutableState, currentWorkflowTransactionPolicy, newWorkflowTransactionPolicy)
}

// ResetWorkflowExecution mocks base method
func (m *MockContext) ResetWorkflowExecution(currMutableState MutableState, updateCurr bool, closeTask, cleanupTask persistence.Task, newMutableState MutableState, newHistorySize int64, newTransferTasks, newTimerTasks, currentReplicationTasks, newReplicationTasks []persistence.Task, baseRunID string, baseRunNextEventID int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResetWorkflowExecution", currMutableState, updateCurr, closeTask, cleanupTask, newMutableState, newHistorySize, newTransferTasks, newTimerTasks, currentReplicationTasks, newReplicationTasks, baseRunID, baseRunNextEventID)
	ret0, _ := ret[0].(error)
	return ret0
}

// ResetWorkflowExecution indicates an expected call of ResetWorkflowExecution
func (mr *MockContextMockRecorder) ResetWorkflowExecution(currMutableState, updateCurr, closeTask, cleanupTask, newMutableState, newHistorySize, newTransferTasks, newTimerTasks, currentReplicationTasks, newReplicationTasks, baseRunID, baseRunNextEventID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetWorkflowExecution", reflect.TypeOf((*MockContext)(nil).ResetWorkflowExecution), currMutableState, updateCurr, closeTask, cleanupTask, newMutableState, newHistorySize, newTransferTasks, newTimerTasks, currentReplicationTasks, newReplicationTasks, baseRunID, baseRunNextEventID)
}
