// Code generated by MockGen. DO NOT EDIT.
// Source: queryParser.go

// Package s3store is a generated GoMock package.
package s3store

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockQueryParser is a mock of QueryParser interface.
type MockQueryParser struct {
	ctrl     *gomock.Controller
	recorder *MockQueryParserMockRecorder
}

// MockQueryParserMockRecorder is the mock recorder for MockQueryParser.
type MockQueryParserMockRecorder struct {
	mock *MockQueryParser
}

// NewMockQueryParser creates a new mock instance.
func NewMockQueryParser(ctrl *gomock.Controller) *MockQueryParser {
	mock := &MockQueryParser{ctrl: ctrl}
	mock.recorder = &MockQueryParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockQueryParser) EXPECT() *MockQueryParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockQueryParser) Parse(query string) (*parsedQuery, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Parse", query)
	ret0, _ := ret[0].(*parsedQuery)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Parse indicates an expected call of Parse.
func (mr *MockQueryParserMockRecorder) Parse(query interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockQueryParser)(nil).Parse), query)
}
