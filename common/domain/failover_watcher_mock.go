// Code generated by MockGen. DO NOT EDIT.
// Source: failover_watcher.go

// Package domain is a generated GoMock package.
package domain

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockFailoverWatcher is a mock of FailoverWatcher interface.
type MockFailoverWatcher struct {
	ctrl     *gomock.Controller
	recorder *MockFailoverWatcherMockRecorder
}

// MockFailoverWatcherMockRecorder is the mock recorder for MockFailoverWatcher.
type MockFailoverWatcherMockRecorder struct {
	mock *MockFailoverWatcher
}

// NewMockFailoverWatcher creates a new mock instance.
func NewMockFailoverWatcher(ctrl *gomock.Controller) *MockFailoverWatcher {
	mock := &MockFailoverWatcher{ctrl: ctrl}
	mock.recorder = &MockFailoverWatcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFailoverWatcher) EXPECT() *MockFailoverWatcherMockRecorder {
	return m.recorder
}

// Start mocks base method.
func (m *MockFailoverWatcher) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *MockFailoverWatcherMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockFailoverWatcher)(nil).Start))
}

// Stop mocks base method.
func (m *MockFailoverWatcher) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockFailoverWatcherMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockFailoverWatcher)(nil).Stop))
}
