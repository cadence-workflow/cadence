// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/uber/cadence/common/quotas (interfaces: Limiter,LimiterFactory)

// Package quotas is a generated GoMock package.
package quotas

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	rate "golang.org/x/time/rate"
)

// MockLimiter is a mock of Limiter interface.
type MockLimiter struct {
	ctrl     *gomock.Controller
	recorder *MockLimiterMockRecorder
}

// MockLimiterMockRecorder is the mock recorder for MockLimiter.
type MockLimiterMockRecorder struct {
	mock *MockLimiter
}

// NewMockLimiter creates a new mock instance.
func NewMockLimiter(ctrl *gomock.Controller) *MockLimiter {
	mock := &MockLimiter{ctrl: ctrl}
	mock.recorder = &MockLimiterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLimiter) EXPECT() *MockLimiterMockRecorder {
	return m.recorder
}

// Allow mocks base method.
func (m *MockLimiter) Allow() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Allow")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Allow indicates an expected call of Allow.
func (mr *MockLimiterMockRecorder) Allow() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Allow", reflect.TypeOf((*MockLimiter)(nil).Allow))
}

// Reserve mocks base method.
func (m *MockLimiter) Reserve() *rate.Reservation {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Reserve")
	ret0, _ := ret[0].(*rate.Reservation)
	return ret0
}

// Reserve indicates an expected call of Reserve.
func (mr *MockLimiterMockRecorder) Reserve() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reserve", reflect.TypeOf((*MockLimiter)(nil).Reserve))
}

// Wait mocks base method.
func (m *MockLimiter) Wait(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockLimiterMockRecorder) Wait(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockLimiter)(nil).Wait), arg0)
}

// MockLimiterFactory is a mock of LimiterFactory interface.
type MockLimiterFactory struct {
	ctrl     *gomock.Controller
	recorder *MockLimiterFactoryMockRecorder
}

// MockLimiterFactoryMockRecorder is the mock recorder for MockLimiterFactory.
type MockLimiterFactoryMockRecorder struct {
	mock *MockLimiterFactory
}

// NewMockLimiterFactory creates a new mock instance.
func NewMockLimiterFactory(ctrl *gomock.Controller) *MockLimiterFactory {
	mock := &MockLimiterFactory{ctrl: ctrl}
	mock.recorder = &MockLimiterFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLimiterFactory) EXPECT() *MockLimiterFactoryMockRecorder {
	return m.recorder
}

// GetLimiter mocks base method.
func (m *MockLimiterFactory) GetLimiter(arg0 string) Limiter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLimiter", arg0)
	ret0, _ := ret[0].(Limiter)
	return ret0
}

// GetLimiter indicates an expected call of GetLimiter.
func (mr *MockLimiterFactoryMockRecorder) GetLimiter(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLimiter", reflect.TypeOf((*MockLimiterFactory)(nil).GetLimiter), arg0)
}
