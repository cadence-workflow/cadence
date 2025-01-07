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
// Source: loadbalancer.go

// Package matching is a generated GoMock package.
package matching

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	types "github.com/uber/cadence/common/types"
)

// MockLoadBalancer is a mock of LoadBalancer interface.
type MockLoadBalancer struct {
	ctrl     *gomock.Controller
	recorder *MockLoadBalancerMockRecorder
}

// MockLoadBalancerMockRecorder is the mock recorder for MockLoadBalancer.
type MockLoadBalancerMockRecorder struct {
	mock *MockLoadBalancer
}

// NewMockLoadBalancer creates a new mock instance.
func NewMockLoadBalancer(ctrl *gomock.Controller) *MockLoadBalancer {
	mock := &MockLoadBalancer{ctrl: ctrl}
	mock.recorder = &MockLoadBalancerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLoadBalancer) EXPECT() *MockLoadBalancerMockRecorder {
	return m.recorder
}

// PickReadPartition mocks base method.
func (m *MockLoadBalancer) PickReadPartition(domainID string, taskList types.TaskList, taskListType int, forwardedFrom string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PickReadPartition", domainID, taskList, taskListType, forwardedFrom)
	ret0, _ := ret[0].(string)
	return ret0
}

// PickReadPartition indicates an expected call of PickReadPartition.
func (mr *MockLoadBalancerMockRecorder) PickReadPartition(domainID, taskList, taskListType, forwardedFrom interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PickReadPartition", reflect.TypeOf((*MockLoadBalancer)(nil).PickReadPartition), domainID, taskList, taskListType, forwardedFrom)
}

// PickWritePartition mocks base method.
func (m *MockLoadBalancer) PickWritePartition(domainID string, taskList types.TaskList, taskListType int, forwardedFrom string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PickWritePartition", domainID, taskList, taskListType, forwardedFrom)
	ret0, _ := ret[0].(string)
	return ret0
}

// PickWritePartition indicates an expected call of PickWritePartition.
func (mr *MockLoadBalancerMockRecorder) PickWritePartition(domainID, taskList, taskListType, forwardedFrom interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PickWritePartition", reflect.TypeOf((*MockLoadBalancer)(nil).PickWritePartition), domainID, taskList, taskListType, forwardedFrom)
}

// UpdateWeight mocks base method.
func (m *MockLoadBalancer) UpdateWeight(domainID string, taskList types.TaskList, taskListType int, forwardedFrom, partition string, info *types.LoadBalancerHints) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateWeight", domainID, taskList, taskListType, forwardedFrom, partition, info)
}

// UpdateWeight indicates an expected call of UpdateWeight.
func (mr *MockLoadBalancerMockRecorder) UpdateWeight(domainID, taskList, taskListType, forwardedFrom, partition, info interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateWeight", reflect.TypeOf((*MockLoadBalancer)(nil).UpdateWeight), domainID, taskList, taskListType, forwardedFrom, partition, info)
}
