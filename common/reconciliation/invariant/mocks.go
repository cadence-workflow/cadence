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
// Source: types.go

// Package invariant is a generated GoMock package.
package invariant

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockInvariant is a mock of Invariant interface.
type MockInvariant struct {
	ctrl     *gomock.Controller
	recorder *MockInvariantMockRecorder
}

// MockInvariantMockRecorder is the mock recorder for MockInvariant.
type MockInvariantMockRecorder struct {
	mock *MockInvariant
}

// NewMockInvariant creates a new mock instance.
func NewMockInvariant(ctrl *gomock.Controller) *MockInvariant {
	mock := &MockInvariant{ctrl: ctrl}
	mock.recorder = &MockInvariantMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInvariant) EXPECT() *MockInvariantMockRecorder {
	return m.recorder
}

// Check mocks base method.
func (m *MockInvariant) Check(arg0 context.Context, arg1 interface{}) CheckResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Check", arg0, arg1)
	ret0, _ := ret[0].(CheckResult)
	return ret0
}

// Check indicates an expected call of Check.
func (mr *MockInvariantMockRecorder) Check(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Check", reflect.TypeOf((*MockInvariant)(nil).Check), arg0, arg1)
}

// Fix mocks base method.
func (m *MockInvariant) Fix(arg0 context.Context, arg1 interface{}) FixResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Fix", arg0, arg1)
	ret0, _ := ret[0].(FixResult)
	return ret0
}

// Fix indicates an expected call of Fix.
func (mr *MockInvariantMockRecorder) Fix(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Fix", reflect.TypeOf((*MockInvariant)(nil).Fix), arg0, arg1)
}

// Name mocks base method.
func (m *MockInvariant) Name() Name {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(Name)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockInvariantMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockInvariant)(nil).Name))
}

// MockManager is a mock of Manager interface.
type MockManager struct {
	ctrl     *gomock.Controller
	recorder *MockManagerMockRecorder
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

// RunChecks mocks base method.
func (m *MockManager) RunChecks(arg0 context.Context, arg1 interface{}) ManagerCheckResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunChecks", arg0, arg1)
	ret0, _ := ret[0].(ManagerCheckResult)
	return ret0
}

// RunChecks indicates an expected call of RunChecks.
func (mr *MockManagerMockRecorder) RunChecks(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunChecks", reflect.TypeOf((*MockManager)(nil).RunChecks), arg0, arg1)
}

// RunFixes mocks base method.
func (m *MockManager) RunFixes(arg0 context.Context, arg1 interface{}) ManagerFixResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunFixes", arg0, arg1)
	ret0, _ := ret[0].(ManagerFixResult)
	return ret0
}

// RunFixes indicates an expected call of RunFixes.
func (mr *MockManagerMockRecorder) RunFixes(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunFixes", reflect.TypeOf((*MockManager)(nil).RunFixes), arg0, arg1)
}
