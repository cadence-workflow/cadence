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
// Source: github.com/uber/cadence/tools/cli (interfaces: ManagerFactory)

// Package cli is a generated GoMock package.
package cli

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	cli "github.com/urfave/cli/v2"

	persistence "github.com/uber/cadence/common/persistence"
	client "github.com/uber/cadence/common/persistence/client"
	invariant "github.com/uber/cadence/common/reconciliation/invariant"
)

// MockManagerFactory is a mock of ManagerFactory interface.
type MockManagerFactory struct {
	ctrl     *gomock.Controller
	recorder *MockManagerFactoryMockRecorder
}

// MockManagerFactoryMockRecorder is the mock recorder for MockManagerFactory.
type MockManagerFactoryMockRecorder struct {
	mock *MockManagerFactory
}

// NewMockManagerFactory creates a new mock instance.
func NewMockManagerFactory(ctrl *gomock.Controller) *MockManagerFactory {
	mock := &MockManagerFactory{ctrl: ctrl}
	mock.recorder = &MockManagerFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManagerFactory) EXPECT() *MockManagerFactoryMockRecorder {
	return m.recorder
}

// initPersistenceFactory mocks base method.
func (m *MockManagerFactory) initPersistenceFactory(arg0 *cli.Context) (client.Factory, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "initPersistenceFactory", arg0)
	ret0, _ := ret[0].(client.Factory)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// initPersistenceFactory indicates an expected call of initPersistenceFactory.
func (mr *MockManagerFactoryMockRecorder) initPersistenceFactory(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "initPersistenceFactory", reflect.TypeOf((*MockManagerFactory)(nil).initPersistenceFactory), arg0)
}

// initializeDomainManager mocks base method.
func (m *MockManagerFactory) initializeDomainManager(arg0 *cli.Context) (persistence.DomainManager, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "initializeDomainManager", arg0)
	ret0, _ := ret[0].(persistence.DomainManager)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// initializeDomainManager indicates an expected call of initializeDomainManager.
func (mr *MockManagerFactoryMockRecorder) initializeDomainManager(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "initializeDomainManager", reflect.TypeOf((*MockManagerFactory)(nil).initializeDomainManager), arg0)
}

// initializeExecutionManager mocks base method.
func (m *MockManagerFactory) initializeExecutionManager(arg0 *cli.Context, arg1 int) (persistence.ExecutionManager, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "initializeExecutionManager", arg0, arg1)
	ret0, _ := ret[0].(persistence.ExecutionManager)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// initializeExecutionManager indicates an expected call of initializeExecutionManager.
func (mr *MockManagerFactoryMockRecorder) initializeExecutionManager(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "initializeExecutionManager", reflect.TypeOf((*MockManagerFactory)(nil).initializeExecutionManager), arg0, arg1)
}

// initializeHistoryManager mocks base method.
func (m *MockManagerFactory) initializeHistoryManager(arg0 *cli.Context) (persistence.HistoryManager, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "initializeHistoryManager", arg0)
	ret0, _ := ret[0].(persistence.HistoryManager)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// initializeHistoryManager indicates an expected call of initializeHistoryManager.
func (mr *MockManagerFactoryMockRecorder) initializeHistoryManager(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "initializeHistoryManager", reflect.TypeOf((*MockManagerFactory)(nil).initializeHistoryManager), arg0)
}

// initializeInvariantManager mocks base method.
func (m *MockManagerFactory) initializeInvariantManager(arg0 []invariant.Invariant) (invariant.Manager, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "initializeInvariantManager", arg0)
	ret0, _ := ret[0].(invariant.Manager)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// initializeInvariantManager indicates an expected call of initializeInvariantManager.
func (mr *MockManagerFactoryMockRecorder) initializeInvariantManager(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "initializeInvariantManager", reflect.TypeOf((*MockManagerFactory)(nil).initializeInvariantManager), arg0)
}

// initializeShardManager mocks base method.
func (m *MockManagerFactory) initializeShardManager(arg0 *cli.Context) (persistence.ShardManager, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "initializeShardManager", arg0)
	ret0, _ := ret[0].(persistence.ShardManager)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// initializeShardManager indicates an expected call of initializeShardManager.
func (mr *MockManagerFactoryMockRecorder) initializeShardManager(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "initializeShardManager", reflect.TypeOf((*MockManagerFactory)(nil).initializeShardManager), arg0)
}
