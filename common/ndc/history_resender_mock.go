// Code generated by MockGen. DO NOT EDIT.
// Source: history_resender.go

// Package ndc is a generated GoMock package.
package ndc

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockHistoryResender is a mock of HistoryResender interface.
type MockHistoryResender struct {
	ctrl     *gomock.Controller
	recorder *MockHistoryResenderMockRecorder
}

// MockHistoryResenderMockRecorder is the mock recorder for MockHistoryResender.
type MockHistoryResenderMockRecorder struct {
	mock *MockHistoryResender
}

// NewMockHistoryResender creates a new mock instance.
func NewMockHistoryResender(ctrl *gomock.Controller) *MockHistoryResender {
	mock := &MockHistoryResender{ctrl: ctrl}
	mock.recorder = &MockHistoryResenderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHistoryResender) EXPECT() *MockHistoryResenderMockRecorder {
	return m.recorder
}

// SendSingleWorkflowHistory mocks base method.
func (m *MockHistoryResender) SendSingleWorkflowHistory(domainID, workflowID, runID string, startEventID, startEventVersion, endEventID, endEventVersion *int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendSingleWorkflowHistory", domainID, workflowID, runID, startEventID, startEventVersion, endEventID, endEventVersion)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendSingleWorkflowHistory indicates an expected call of SendSingleWorkflowHistory.
func (mr *MockHistoryResenderMockRecorder) SendSingleWorkflowHistory(domainID, workflowID, runID, startEventID, startEventVersion, endEventID, endEventVersion interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendSingleWorkflowHistory", reflect.TypeOf((*MockHistoryResender)(nil).SendSingleWorkflowHistory), domainID, workflowID, runID, startEventID, startEventVersion, endEventID, endEventVersion)
}
