// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/uber/cadence/service/frontend/wrappers/clusterredirection (interfaces: ClusterRedirectionPolicy)
//
// Generated by this command:
//
//	mockgen -package clusterredirection -destination policy_mock.go -self_package github.com/uber/cadence/service/frontend/wrappers/clusterredirection github.com/uber/cadence/service/frontend/wrappers/clusterredirection ClusterRedirectionPolicy
//

// Package clusterredirection is a generated GoMock package.
package clusterredirection

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"

	cache "github.com/uber/cadence/common/cache"
	types "github.com/uber/cadence/common/types"
)

// MockClusterRedirectionPolicy is a mock of ClusterRedirectionPolicy interface.
type MockClusterRedirectionPolicy struct {
	ctrl     *gomock.Controller
	recorder *MockClusterRedirectionPolicyMockRecorder
	isgomock struct{}
}

// MockClusterRedirectionPolicyMockRecorder is the mock recorder for MockClusterRedirectionPolicy.
type MockClusterRedirectionPolicyMockRecorder struct {
	mock *MockClusterRedirectionPolicy
}

// NewMockClusterRedirectionPolicy creates a new mock instance.
func NewMockClusterRedirectionPolicy(ctrl *gomock.Controller) *MockClusterRedirectionPolicy {
	mock := &MockClusterRedirectionPolicy{ctrl: ctrl}
	mock.recorder = &MockClusterRedirectionPolicyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClusterRedirectionPolicy) EXPECT() *MockClusterRedirectionPolicyMockRecorder {
	return m.recorder
}

// Redirect mocks base method.
func (m *MockClusterRedirectionPolicy) Redirect(ctx context.Context, domainEntry *cache.DomainCacheEntry, workflowExecution *types.WorkflowExecution, actClSelPolicyForNewWF *types.ActiveClusterSelectionPolicy, apiName string, requestedConsistencyLevel types.QueryConsistencyLevel, call func(string) error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Redirect", ctx, domainEntry, workflowExecution, actClSelPolicyForNewWF, apiName, requestedConsistencyLevel, call)
	ret0, _ := ret[0].(error)
	return ret0
}

// Redirect indicates an expected call of Redirect.
func (mr *MockClusterRedirectionPolicyMockRecorder) Redirect(ctx, domainEntry, workflowExecution, actClSelPolicyForNewWF, apiName, requestedConsistencyLevel, call any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Redirect", reflect.TypeOf((*MockClusterRedirectionPolicy)(nil).Redirect), ctx, domainEntry, workflowExecution, actClSelPolicyForNewWF, apiName, requestedConsistencyLevel, call)
}
