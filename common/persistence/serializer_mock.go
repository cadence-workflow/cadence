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
// Source: github.com/uber/cadence/common/persistence (interfaces: PayloadSerializer)
//
// Generated by this command:
//
//	mockgen -package persistence -destination serializer_mock.go -self_package github.com/uber/cadence/common/persistence github.com/uber/cadence/common/persistence PayloadSerializer
//

// Package persistence is a generated GoMock package.
package persistence

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"

	checksum "github.com/uber/cadence/common/checksum"
	constants "github.com/uber/cadence/common/constants"
	types "github.com/uber/cadence/common/types"
)

// MockPayloadSerializer is a mock of PayloadSerializer interface.
type MockPayloadSerializer struct {
	ctrl     *gomock.Controller
	recorder *MockPayloadSerializerMockRecorder
	isgomock struct{}
}

// MockPayloadSerializerMockRecorder is the mock recorder for MockPayloadSerializer.
type MockPayloadSerializerMockRecorder struct {
	mock *MockPayloadSerializer
}

// NewMockPayloadSerializer creates a new mock instance.
func NewMockPayloadSerializer(ctrl *gomock.Controller) *MockPayloadSerializer {
	mock := &MockPayloadSerializer{ctrl: ctrl}
	mock.recorder = &MockPayloadSerializerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPayloadSerializer) EXPECT() *MockPayloadSerializerMockRecorder {
	return m.recorder
}

// DeserializeAsyncWorkflowsConfig mocks base method.
func (m *MockPayloadSerializer) DeserializeAsyncWorkflowsConfig(data *DataBlob) (*types.AsyncWorkflowConfiguration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeAsyncWorkflowsConfig", data)
	ret0, _ := ret[0].(*types.AsyncWorkflowConfiguration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeAsyncWorkflowsConfig indicates an expected call of DeserializeAsyncWorkflowsConfig.
func (mr *MockPayloadSerializerMockRecorder) DeserializeAsyncWorkflowsConfig(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeAsyncWorkflowsConfig", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeAsyncWorkflowsConfig), data)
}

// DeserializeBadBinaries mocks base method.
func (m *MockPayloadSerializer) DeserializeBadBinaries(data *DataBlob) (*types.BadBinaries, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeBadBinaries", data)
	ret0, _ := ret[0].(*types.BadBinaries)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeBadBinaries indicates an expected call of DeserializeBadBinaries.
func (mr *MockPayloadSerializerMockRecorder) DeserializeBadBinaries(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeBadBinaries", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeBadBinaries), data)
}

// DeserializeBatchEvents mocks base method.
func (m *MockPayloadSerializer) DeserializeBatchEvents(data *DataBlob) ([]*types.HistoryEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeBatchEvents", data)
	ret0, _ := ret[0].([]*types.HistoryEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeBatchEvents indicates an expected call of DeserializeBatchEvents.
func (mr *MockPayloadSerializerMockRecorder) DeserializeBatchEvents(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeBatchEvents", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeBatchEvents), data)
}

// DeserializeChecksum mocks base method.
func (m *MockPayloadSerializer) DeserializeChecksum(data *DataBlob) (checksum.Checksum, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeChecksum", data)
	ret0, _ := ret[0].(checksum.Checksum)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeChecksum indicates an expected call of DeserializeChecksum.
func (mr *MockPayloadSerializerMockRecorder) DeserializeChecksum(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeChecksum", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeChecksum), data)
}

// DeserializeDynamicConfigBlob mocks base method.
func (m *MockPayloadSerializer) DeserializeDynamicConfigBlob(data *DataBlob) (*types.DynamicConfigBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeDynamicConfigBlob", data)
	ret0, _ := ret[0].(*types.DynamicConfigBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeDynamicConfigBlob indicates an expected call of DeserializeDynamicConfigBlob.
func (mr *MockPayloadSerializerMockRecorder) DeserializeDynamicConfigBlob(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeDynamicConfigBlob", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeDynamicConfigBlob), data)
}

// DeserializeEvent mocks base method.
func (m *MockPayloadSerializer) DeserializeEvent(data *DataBlob) (*types.HistoryEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeEvent", data)
	ret0, _ := ret[0].(*types.HistoryEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeEvent indicates an expected call of DeserializeEvent.
func (mr *MockPayloadSerializerMockRecorder) DeserializeEvent(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeEvent", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeEvent), data)
}

// DeserializeIsolationGroups mocks base method.
func (m *MockPayloadSerializer) DeserializeIsolationGroups(data *DataBlob) (*types.IsolationGroupConfiguration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeIsolationGroups", data)
	ret0, _ := ret[0].(*types.IsolationGroupConfiguration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeIsolationGroups indicates an expected call of DeserializeIsolationGroups.
func (mr *MockPayloadSerializerMockRecorder) DeserializeIsolationGroups(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeIsolationGroups", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeIsolationGroups), data)
}

// DeserializePendingFailoverMarkers mocks base method.
func (m *MockPayloadSerializer) DeserializePendingFailoverMarkers(data *DataBlob) ([]*types.FailoverMarkerAttributes, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializePendingFailoverMarkers", data)
	ret0, _ := ret[0].([]*types.FailoverMarkerAttributes)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializePendingFailoverMarkers indicates an expected call of DeserializePendingFailoverMarkers.
func (mr *MockPayloadSerializerMockRecorder) DeserializePendingFailoverMarkers(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializePendingFailoverMarkers", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializePendingFailoverMarkers), data)
}

// DeserializeProcessingQueueStates mocks base method.
func (m *MockPayloadSerializer) DeserializeProcessingQueueStates(data *DataBlob) (*types.ProcessingQueueStates, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeProcessingQueueStates", data)
	ret0, _ := ret[0].(*types.ProcessingQueueStates)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeProcessingQueueStates indicates an expected call of DeserializeProcessingQueueStates.
func (mr *MockPayloadSerializerMockRecorder) DeserializeProcessingQueueStates(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeProcessingQueueStates", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeProcessingQueueStates), data)
}

// DeserializeResetPoints mocks base method.
func (m *MockPayloadSerializer) DeserializeResetPoints(data *DataBlob) (*types.ResetPoints, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeResetPoints", data)
	ret0, _ := ret[0].(*types.ResetPoints)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeResetPoints indicates an expected call of DeserializeResetPoints.
func (mr *MockPayloadSerializerMockRecorder) DeserializeResetPoints(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeResetPoints", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeResetPoints), data)
}

// DeserializeVersionHistories mocks base method.
func (m *MockPayloadSerializer) DeserializeVersionHistories(data *DataBlob) (*types.VersionHistories, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeVersionHistories", data)
	ret0, _ := ret[0].(*types.VersionHistories)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeVersionHistories indicates an expected call of DeserializeVersionHistories.
func (mr *MockPayloadSerializerMockRecorder) DeserializeVersionHistories(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeVersionHistories", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeVersionHistories), data)
}

// DeserializeVisibilityMemo mocks base method.
func (m *MockPayloadSerializer) DeserializeVisibilityMemo(data *DataBlob) (*types.Memo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeserializeVisibilityMemo", data)
	ret0, _ := ret[0].(*types.Memo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeserializeVisibilityMemo indicates an expected call of DeserializeVisibilityMemo.
func (mr *MockPayloadSerializerMockRecorder) DeserializeVisibilityMemo(data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeserializeVisibilityMemo", reflect.TypeOf((*MockPayloadSerializer)(nil).DeserializeVisibilityMemo), data)
}

// SerializeAsyncWorkflowsConfig mocks base method.
func (m *MockPayloadSerializer) SerializeAsyncWorkflowsConfig(config *types.AsyncWorkflowConfiguration, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeAsyncWorkflowsConfig", config, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeAsyncWorkflowsConfig indicates an expected call of SerializeAsyncWorkflowsConfig.
func (mr *MockPayloadSerializerMockRecorder) SerializeAsyncWorkflowsConfig(config, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeAsyncWorkflowsConfig", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeAsyncWorkflowsConfig), config, encodingType)
}

// SerializeBadBinaries mocks base method.
func (m *MockPayloadSerializer) SerializeBadBinaries(event *types.BadBinaries, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeBadBinaries", event, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeBadBinaries indicates an expected call of SerializeBadBinaries.
func (mr *MockPayloadSerializerMockRecorder) SerializeBadBinaries(event, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeBadBinaries", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeBadBinaries), event, encodingType)
}

// SerializeBatchEvents mocks base method.
func (m *MockPayloadSerializer) SerializeBatchEvents(batch []*types.HistoryEvent, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeBatchEvents", batch, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeBatchEvents indicates an expected call of SerializeBatchEvents.
func (mr *MockPayloadSerializerMockRecorder) SerializeBatchEvents(batch, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeBatchEvents", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeBatchEvents), batch, encodingType)
}

// SerializeChecksum mocks base method.
func (m *MockPayloadSerializer) SerializeChecksum(sum checksum.Checksum, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeChecksum", sum, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeChecksum indicates an expected call of SerializeChecksum.
func (mr *MockPayloadSerializerMockRecorder) SerializeChecksum(sum, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeChecksum", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeChecksum), sum, encodingType)
}

// SerializeDynamicConfigBlob mocks base method.
func (m *MockPayloadSerializer) SerializeDynamicConfigBlob(blob *types.DynamicConfigBlob, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeDynamicConfigBlob", blob, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeDynamicConfigBlob indicates an expected call of SerializeDynamicConfigBlob.
func (mr *MockPayloadSerializerMockRecorder) SerializeDynamicConfigBlob(blob, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeDynamicConfigBlob", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeDynamicConfigBlob), blob, encodingType)
}

// SerializeEvent mocks base method.
func (m *MockPayloadSerializer) SerializeEvent(event *types.HistoryEvent, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeEvent", event, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeEvent indicates an expected call of SerializeEvent.
func (mr *MockPayloadSerializerMockRecorder) SerializeEvent(event, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeEvent", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeEvent), event, encodingType)
}

// SerializeIsolationGroups mocks base method.
func (m *MockPayloadSerializer) SerializeIsolationGroups(event *types.IsolationGroupConfiguration, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeIsolationGroups", event, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeIsolationGroups indicates an expected call of SerializeIsolationGroups.
func (mr *MockPayloadSerializerMockRecorder) SerializeIsolationGroups(event, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeIsolationGroups", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeIsolationGroups), event, encodingType)
}

// SerializePendingFailoverMarkers mocks base method.
func (m *MockPayloadSerializer) SerializePendingFailoverMarkers(markers []*types.FailoverMarkerAttributes, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializePendingFailoverMarkers", markers, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializePendingFailoverMarkers indicates an expected call of SerializePendingFailoverMarkers.
func (mr *MockPayloadSerializerMockRecorder) SerializePendingFailoverMarkers(markers, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializePendingFailoverMarkers", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializePendingFailoverMarkers), markers, encodingType)
}

// SerializeProcessingQueueStates mocks base method.
func (m *MockPayloadSerializer) SerializeProcessingQueueStates(states *types.ProcessingQueueStates, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeProcessingQueueStates", states, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeProcessingQueueStates indicates an expected call of SerializeProcessingQueueStates.
func (mr *MockPayloadSerializerMockRecorder) SerializeProcessingQueueStates(states, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeProcessingQueueStates", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeProcessingQueueStates), states, encodingType)
}

// SerializeResetPoints mocks base method.
func (m *MockPayloadSerializer) SerializeResetPoints(event *types.ResetPoints, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeResetPoints", event, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeResetPoints indicates an expected call of SerializeResetPoints.
func (mr *MockPayloadSerializerMockRecorder) SerializeResetPoints(event, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeResetPoints", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeResetPoints), event, encodingType)
}

// SerializeVersionHistories mocks base method.
func (m *MockPayloadSerializer) SerializeVersionHistories(histories *types.VersionHistories, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeVersionHistories", histories, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeVersionHistories indicates an expected call of SerializeVersionHistories.
func (mr *MockPayloadSerializerMockRecorder) SerializeVersionHistories(histories, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeVersionHistories", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeVersionHistories), histories, encodingType)
}

// SerializeVisibilityMemo mocks base method.
func (m *MockPayloadSerializer) SerializeVisibilityMemo(memo *types.Memo, encodingType constants.EncodingType) (*DataBlob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeVisibilityMemo", memo, encodingType)
	ret0, _ := ret[0].(*DataBlob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SerializeVisibilityMemo indicates an expected call of SerializeVisibilityMemo.
func (mr *MockPayloadSerializerMockRecorder) SerializeVisibilityMemo(memo, encodingType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeVisibilityMemo", reflect.TypeOf((*MockPayloadSerializer)(nil).SerializeVisibilityMemo), memo, encodingType)
}
