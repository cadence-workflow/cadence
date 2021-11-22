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
// Source: hashring.go

// Package membership is a generated GoMock package.
package membership

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockPeerProvider is a mock of PeerProvider interface.
type MockPeerProvider struct {
	ctrl     *gomock.Controller
	recorder *MockPeerProviderMockRecorder
}

// MockPeerProviderMockRecorder is the mock recorder for MockPeerProvider.
type MockPeerProviderMockRecorder struct {
	mock *MockPeerProvider
}

// NewMockPeerProvider creates a new mock instance.
func NewMockPeerProvider(ctrl *gomock.Controller) *MockPeerProvider {
	mock := &MockPeerProvider{ctrl: ctrl}
	mock.recorder = &MockPeerProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPeerProvider) EXPECT() *MockPeerProviderMockRecorder {
	return m.recorder
}

// GetMembers mocks base method.
func (m *MockPeerProvider) GetMembers(service string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMembers", service)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMembers indicates an expected call of GetMembers.
func (mr *MockPeerProviderMockRecorder) GetMembers(service interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMembers", reflect.TypeOf((*MockPeerProvider)(nil).GetMembers), service)
}

// SelfEvict mocks base method.
func (m *MockPeerProvider) SelfEvict() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SelfEvict")
	ret0, _ := ret[0].(error)
	return ret0
}

// SelfEvict indicates an expected call of SelfEvict.
func (mr *MockPeerProviderMockRecorder) SelfEvict() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SelfEvict", reflect.TypeOf((*MockPeerProvider)(nil).SelfEvict))
}

// Start mocks base method.
func (m *MockPeerProvider) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *MockPeerProviderMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockPeerProvider)(nil).Start))
}

// Stop mocks base method.
func (m *MockPeerProvider) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockPeerProviderMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockPeerProvider)(nil).Stop))
}

// Subscribe mocks base method.
func (m *MockPeerProvider) Subscribe(name string, notifyChannel chan<- *ChangedEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe", name, notifyChannel)
	ret0, _ := ret[0].(error)
	return ret0
}

// Subscribe indicates an expected call of Subscribe.
func (mr *MockPeerProviderMockRecorder) Subscribe(name, notifyChannel interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockPeerProvider)(nil).Subscribe), name, notifyChannel)
}

// WhoAmI mocks base method.
func (m *MockPeerProvider) WhoAmI() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WhoAmI")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WhoAmI indicates an expected call of WhoAmI.
func (mr *MockPeerProviderMockRecorder) WhoAmI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WhoAmI", reflect.TypeOf((*MockPeerProvider)(nil).WhoAmI))
}
