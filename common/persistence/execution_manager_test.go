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

package persistence

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

var (
	testIndex             = "test-index"
	testDomain            = "test-domain"
	testDomainID          = "bfd5c907-f899-4baf-a7b2-2ab85e623ebd"
	testPageSize          = 10
	testEarliestTime      = int64(1547596872371000000)
	testLatestTime        = int64(2547596872371000000)
	testWorkflowType      = "test-wf-type"
	testWorkflowID        = "test-wid"
	testCloseStatus       = int32(1)
	testTableName         = "test-table-name"
	testRunID             = "test-run-id"
	testSearchAttributes1 = map[string]interface{}{"TestAttr1": "val1", "TestAttr2": 2, "TestAttr3": false}
	testSearchAttributes2 = map[string]interface{}{"TestAttr1": "val2", "TestAttr2": 2, "TestAttr3": false}
	testSearchAttributes3 = map[string]interface{}{"TestAttr2": 2, "TestAttr3": false}
)

func TestExecutionManager_ProxyStoreMethods(t *testing.T) {
	for _, tc := range []struct {
		method       string
		prepareMocks func(*MockExecutionStore)
	}{
		{
			method: "GetShardID",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().GetShardID().Return(1).Times(1)
			},
		},
		{
			method: "GetName",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().GetName().Return("test").Times(1)
			},
		},
		{
			method: "Close",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().Close().Return().Times(1)
			},
		},
		{
			method: "GetHistoryTasks",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			method: "RangeCompleteHistoryTask",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			method: "DeleteReplicationTaskFromDLQ",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().DeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			method: "RangeDeleteReplicationTaskFromDLQ",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().RangeDeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			method: "DeleteWorkflowExecution",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			method: "DeleteCurrentWorkflowExecution",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			method: "GetCurrentExecution",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			method: "ListCurrentExecutions",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			method: "IsWorkflowExecutionExists",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			method: "GetReplicationDLQSize",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().GetReplicationDLQSize(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
	} {
		t.Run(tc.method, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedStore := NewMockExecutionStore(ctrl)
			tc.prepareMocks(mockedStore)
			manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), nil, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			v := reflect.ValueOf(manager)
			method := v.MethodByName(tc.method)
			methodType := method.Type()
			args := methodType.NumIn()
			var vals []reflect.Value
			// If a method requires arguments, we expect the first argument to be a context
			// and the rest to be zero values of the correct type.
			// For methods like Close and GetShardID, we don't expect any arguments.
			if args > 0 {
				vals = append(vals, reflect.ValueOf(context.Background()))
				for i := 1; i < args; i++ {
					vals = append(vals, reflect.Zero(methodType.In(i)))
				}
			}

			callRes := method.Call(vals)
			if callRes == nil {
				return
			}
			resultErr := callRes[len(callRes)-1].Interface()
			err, ok := resultErr.(error)
			if ok {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecutionManager_GetWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	request := &GetWorkflowExecutionRequest{
		DomainID: testDomainID,
		Execution: types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		RangeID: 1,
	}

	activityOne := sampleInternalActivityInfo("activity1")
	activityTwo := sampleInternalActivityInfo("activity2")

	wfCompletionEvent := NewDataBlob([]byte("wf-event"), constants.EncodingTypeThriftRW)
	wfCompletionEventData := generateTestHistoryEvent(99)

	wfInfo := sampleInternalWorkflowExecutionInfo()
	wfInfo.CompletionEvent = wfCompletionEvent
	wfInfo.AutoResetPoints = NewDataBlob([]byte("test-reset-points"), constants.EncodingTypeThriftRW)

	mockedStore.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(&InternalGetWorkflowExecutionResponse{
		State: &InternalWorkflowMutableState{
			ExecutionInfo: wfInfo,
			ActivityInfos: map[int64]*InternalActivityInfo{
				1: activityOne,
				2: activityTwo,
			},
			TimerInfos: map[string]*TimerInfo{
				"test-timer": {
					Version: 1,
				},
			},
		},
	}, nil)

	mockedSerializer.EXPECT().DeserializeEvent(activityOne.ScheduledEvent).Return(&types.HistoryEvent{
		ID: 1,
	}, nil).Times(1)
	mockedSerializer.EXPECT().DeserializeEvent(activityOne.StartedEvent).Return(&types.HistoryEvent{
		ID: 1,
	}, nil).Times(1)

	mockedSerializer.EXPECT().DeserializeEvent(activityTwo.ScheduledEvent).Return(&types.HistoryEvent{
		ID: 2,
	}, nil).Times(1)
	mockedSerializer.EXPECT().DeserializeEvent(activityTwo.StartedEvent).Return(&types.HistoryEvent{
		ID: 2,
	}, nil).Times(1)

	mockedSerializer.EXPECT().DeserializeEvent(wfCompletionEvent).Return(wfCompletionEventData, nil).Times(1)
	mockedSerializer.EXPECT().DeserializeResetPoints(gomock.Any()).Return(&types.ResetPoints{}, nil).Times(1)
	mockedSerializer.EXPECT().DeserializeChecksum(gomock.Any()).Return(checksum.Checksum{}, nil).Times(1)

	activeClusterSelPlcyData := sampleActiveClusterSelectionPolicyData()
	mockedSerializer.EXPECT().DeserializeActiveClusterSelectionPolicy(activeClusterSelPlcyData).Return(generateActiveClusterSelectionPolicy(), nil).Times(1)

	res, err := manager.GetWorkflowExecution(context.Background(), request)
	assert.NoError(t, err)

	expectedExecutionInfo := sampleWorkflowExecutionInfo()
	expectedExecutionInfo.CompletionEvent = wfCompletionEventData
	expectedExecutionInfo.AutoResetPoints = &types.ResetPoints{}

	assert.Equal(t, &WorkflowMutableState{
		ExecutionInfo:       expectedExecutionInfo,
		ChildExecutionInfos: make(map[int64]*ChildExecutionInfo),
		ActivityInfos: map[int64]*ActivityInfo{
			1: sampleActivityInfo("activity1", 1),
			2: sampleActivityInfo("activity2", 2),
		},
		TimerInfos: map[string]*TimerInfo{
			"test-timer": {
				Version: 1,
			},
		},
		ExecutionStats: &ExecutionStats{
			HistorySize: 1024,
		},
		BufferedEvents: make([]*types.HistoryEvent, 0),
	}, res.State)
	// Expectations for the deserialization of activity events.
	assert.Equal(t, &MutableStateStats{MutableStateSize: 170, ExecutionInfoSize: 20, ActivityInfoSize: 150, TimerInfoSize: 0, ChildInfoSize: 0, SignalInfoSize: 0, BufferedEventsSize: 0, ActivityInfoCount: 2, TimerInfoCount: 1, ChildInfoCount: 0, SignalInfoCount: 0, RequestCancelInfoCount: 0, BufferedEventsCount: 0}, res.MutableStateStats)
}

func TestExecutionManager_GetWorkflowExecution_NoWorkflow(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	request := &GetWorkflowExecutionRequest{
		DomainID: "testDomain",
		Execution: types.WorkflowExecution{
			WorkflowID: "nonexistentWorkflow",
			RunID:      "nonexistentRunID",
		},
		RangeID: 1,
	}

	mockedStore.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{})

	_, err := manager.GetWorkflowExecution(context.Background(), request)
	assert.Error(t, err)
	assert.IsType(t, &types.EntityNotExistsError{}, err)
}

func TestExecutionManager_UpdateWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	expectedInfo := sampleInternalWorkflowMutation()

	mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(expectedInfo.ExecutionInfo.CompletionEvent, nil).Times(2)
	mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
	mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
	mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
	mockedSerializer.EXPECT().SerializeEvent(childWorkflowStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
	mockedSerializer.EXPECT().SerializeResetPoints(generateResetPoints(), constants.EncodingTypeThriftRW).Return(expectedInfo.ExecutionInfo.AutoResetPoints, nil).Times(2)
	mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(2)

	request := &UpdateWorkflowExecutionRequest{
		RangeID:                1,
		Mode:                   UpdateWorkflowModeBypassCurrent,
		UpdateWorkflowMutation: *sampleWorkflowMutation(),
		Encoding:               constants.EncodingTypeThriftRW,
		NewWorkflowSnapshot: &WorkflowSnapshot{
			ExecutionInfo: sampleWorkflowExecutionInfo(),
			ExecutionStats: &ExecutionStats{
				HistorySize: 1024,
			},
			Checksum: generateChecksum(),
		},
	}

	mockedSerializer.EXPECT().SerializeChecksum(request.UpdateWorkflowMutation.Checksum, constants.EncodingTypeJSON).Return(expectedInfo.ChecksumData, nil).Times(2)

	expectedRequest := &InternalUpdateWorkflowExecutionRequest{
		RangeID:                1,
		Mode:                   UpdateWorkflowModeBypassCurrent,
		UpdateWorkflowMutation: *expectedInfo,
		NewWorkflowSnapshot: &InternalWorkflowSnapshot{
			ExecutionInfo: expectedInfo.ExecutionInfo,
		},
	}
	mockedStore.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req *InternalUpdateWorkflowExecutionRequest) error {
		assert.Equal(t, expectedRequest.UpdateWorkflowMutation, req.UpdateWorkflowMutation)
		return nil
	})

	res, err := manager.UpdateWorkflowExecution(context.Background(), request)
	assert.NoError(t, err)
	stats := &MutableStateUpdateSessionStats{
		MutableStateSize:    90,
		ExecutionInfoSize:   40,
		ActivityInfoSize:    20,
		TimerInfoSize:       10,
		ChildInfoSize:       20,
		ActivityInfoCount:   1,
		TimerInfoCount:      1,
		ChildInfoCount:      1,
		TaskCountByCategory: map[HistoryTaskCategory]int{},
	}
	assert.Equal(t, stats, res.MutableStateUpdateSessionStats)
}

func TestSerializeWorkflowSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name         string
		prepareMocks func(*MockPayloadSerializer)
		input        *WorkflowSnapshot
		checkRes     func(*testing.T, *InternalWorkflowSnapshot, error)
	}{
		{
			name: "success",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.NoError(t, err)
				assert.Equal(t, sampleInternalWorkflowSnapshot(), res)
			},
		},
		{
			name: "nil info",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).Times(1)
			},
			input: &WorkflowSnapshot{},
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.NoError(t, err)
				assert.Equal(t, &InternalWorkflowSnapshot{
					ExecutionInfo:       &InternalWorkflowExecutionInfo{},
					WorkflowTimerTasks:  nil,
					ChecksumData:        sampleTestCheckSumData(),
					StartVersion:        constants.EmptyVersion,
					LastWriteVersion:    constants.EmptyVersion,
					ActivityInfos:       make([]*InternalActivityInfo, 0),
					ChildExecutionInfos: make([]*InternalChildExecutionInfo, 0),
				}, res)
			},
		},
		{
			name: "serialize event error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(nil, assert.AnError).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "serialize points error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(nil, assert.AnError).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "serialize version histories error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(NewDataBlob([]byte("test-reset-points"), constants.EncodingTypeThriftRW), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(nil, assert.AnError).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "serialize activity scheduled event error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(nil, assert.AnError).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "serialize activity started event error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(nil, assert.AnError).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "serialize child workflow scheduled event error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(nil, assert.AnError).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "serialize child workflow started event error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowStartedEvent(), constants.EncodingTypeThriftRW).Return(nil, assert.AnError).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			input: sampleWorkflowSnapshot(),
			checkRes: func(t *testing.T, res *InternalWorkflowSnapshot, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockedSerializer := NewMockPayloadSerializer(ctrl)
			tc.prepareMocks(mockedSerializer)
			manager := NewExecutionManagerImpl(nil, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			}).(*executionManagerImpl)
			res, err := manager.SerializeWorkflowSnapshot(tc.input, constants.EncodingTypeThriftRW)
			tc.checkRes(t, res, err)
		})
	}
}

func TestDeserializeBufferedEvents(t *testing.T) {
	for _, tc := range []struct {
		name         string
		prepareMocks func(*MockPayloadSerializer)
		checkRes     func(*testing.T, []*types.HistoryEvent, error)
	}{
		{
			name: "success",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				eventCounter := 0
				mockedSerializer.EXPECT().DeserializeBatchEvents(gomock.Any()).DoAndReturn(func(data *DataBlob) ([]*types.HistoryEvent, error) {
					res := []*types.HistoryEvent{{ID: int64(eventCounter)}, {ID: int64(eventCounter + 1)}}
					eventCounter += 2
					return res, nil
				}).Times(2)
			},
			checkRes: func(t *testing.T, events []*types.HistoryEvent, err error) {
				assert.NoError(t, err)
				assert.Equal(t, []*types.HistoryEvent{{ID: 0}, {ID: 1}, {ID: 2}, {ID: 3}}, events)
			},
		},
		{
			name: "error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().DeserializeBatchEvents(gomock.Any()).Return(nil, assert.AnError).Times(1)
			},
			checkRes: func(t *testing.T, res []*types.HistoryEvent, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedSerializer := NewMockPayloadSerializer(ctrl)

			tc.prepareMocks(mockedSerializer)

			manager := NewExecutionManagerImpl(nil, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			}).(*executionManagerImpl)

			events := []*DataBlob{
				sampleEventData(),
				sampleEventData(),
			}

			res, err := manager.DeserializeBufferedEvents(events)
			tc.checkRes(t, res, err)
		})
	}
}

func TestPutReplicationTaskToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), nil, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	now := time.Now().UTC()

	task := &PutReplicationTaskToDLQRequest{
		SourceClusterName: "test-cluster",
		TaskInfo: &ReplicationTaskInfo{
			DomainID:     testDomainID,
			WorkflowID:   testWorkflowID,
			CreationTime: now.UnixNano(),
		},
		DomainName: testDomain,
	}

	mockedStore.EXPECT().PutReplicationTaskToDLQ(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req *InternalPutReplicationTaskToDLQRequest) error {
		assert.Equal(t, "test-cluster", req.SourceClusterName)
		assert.Equal(t, testDomainID, req.TaskInfo.DomainID)
		assert.Equal(t, testWorkflowID, req.TaskInfo.WorkflowID)
		assert.Equal(t, now, req.TaskInfo.CreationTime)

		assert.WithinDuration(t, now, req.TaskInfo.CurrentTimeStamp, time.Second)
		return nil
	})

	err := manager.PutReplicationTaskToDLQ(context.Background(), task)
	assert.NoError(t, err)
}

func TestGetReplicationTasksFromDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), nil, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	request := &GetReplicationTasksFromDLQRequest{
		SourceClusterName: "test-cluster",
		ReadLevel:         1,
		MaxReadLevel:      2,
		BatchSize:         10,
		NextPageToken:     nil,
	}

	now := time.Now().UTC().Round(time.Second)

	mockedStore.EXPECT().GetReplicationTasksFromDLQ(gomock.Any(), request).Return(
		&GetHistoryTasksResponse{
			Tasks: []Task{
				&HistoryReplicationTask{
					WorkflowIdentifier: WorkflowIdentifier{
						DomainID:   testDomainID,
						WorkflowID: testWorkflowID,
					},
					TaskData: TaskData{
						TaskID:              1,
						VisibilityTimestamp: now,
					},
				},
			},
			NextPageToken: []byte("test-token"),
		}, nil)

	res, err := manager.GetReplicationTasksFromDLQ(context.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, &GetHistoryTasksResponse{
		Tasks: []Task{
			&HistoryReplicationTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
				},
				TaskData: TaskData{
					TaskID:              1,
					VisibilityTimestamp: now,
				},
			},
		},
		NextPageToken: []byte("test-token"),
	}, res)
}

func TestDeserializeChildExecutionInfos(t *testing.T) {
	tests := []struct {
		name         string
		prepareMocks func(*MockPayloadSerializer)
		checkRes     func(*testing.T, map[int64]*ChildExecutionInfo, error)
	}{
		{
			name: "success",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				// Child 1 init event
				mockedSerializer.EXPECT().DeserializeEvent(sampleEventDataWithVersion(1)).Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}, nil)
				// Child 1 start event
				mockedSerializer.EXPECT().DeserializeEvent(sampleEventDataWithVersion(2)).Return(&types.HistoryEvent{
					ID:      2,
					Version: 2,
				}, nil)
				// Child 2 init event
				mockedSerializer.EXPECT().DeserializeEvent(sampleEventDataWithVersion(3)).Return(&types.HistoryEvent{
					ID:      3,
					Version: 3,
				}, nil)

				// Child 2 start event is mimicking legacy behavior where runID and workflowID were not stored inside the info
				// but was extracted from the startEvent
				mockedSerializer.EXPECT().DeserializeEvent(sampleEventDataWithVersion(4)).Return(&types.HistoryEvent{
					ID:      4,
					Version: 4,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						WorkflowExecution: &types.WorkflowExecution{
							WorkflowID: "legacy-workflow-id",
							RunID:      "legacy-run-id",
						},
					},
				}, nil)
			},
			checkRes: func(t *testing.T, events map[int64]*ChildExecutionInfo, err error) {
				assert.NoError(t, err)
				assert.Equal(t, map[int64]*ChildExecutionInfo{
					1: {
						Version:           1,
						DomainID:          testDomainID,
						WorkflowTypeName:  testWorkflowType,
						InitiatedID:       1,
						InitiatedEvent:    &types.HistoryEvent{ID: 1, Version: 1},
						StartedID:         2,
						StartedEvent:      &types.HistoryEvent{ID: 2, Version: 2},
						CreateRequestID:   "create-request-id",
						StartedWorkflowID: "workflow-id",
						StartedRunID:      "run-id",
					},
					2: {
						Version:          3,
						DomainID:         testDomainID,
						WorkflowTypeName: testWorkflowType,
						InitiatedID:      3,
						InitiatedEvent:   &types.HistoryEvent{ID: 3, Version: 3},
						StartedID:        4,
						StartedEvent: &types.HistoryEvent{ID: 4, Version: 4,
							ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
								WorkflowExecution: &types.WorkflowExecution{
									WorkflowID: "legacy-workflow-id",
									RunID:      "legacy-run-id",
								},
							}},
						CreateRequestID:   "create-request-id",
						StartedWorkflowID: "legacy-workflow-id",
						StartedRunID:      "legacy-run-id",
					},
				}, events)
			},
		},
		{
			name: "deserialize event error",
			prepareMocks: func(mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().DeserializeEvent(gomock.Any()).Return(nil, assert.AnError).Times(1)
			},
			checkRes: func(t *testing.T, events map[int64]*ChildExecutionInfo, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockedSerializer := NewMockPayloadSerializer(ctrl)

			test.prepareMocks(mockedSerializer)

			manager := &executionManagerImpl{
				serializer: mockedSerializer,
			}

			result, err := manager.DeserializeChildExecutionInfos(map[int64]*InternalChildExecutionInfo{
				1: sampleInternalChildExecutionInfo(1, 2),
				2: sampleInternalChildExecutionInfo(3, 4),
			})
			test.checkRes(t, result, err)
		})
	}
}

func TestListConcreteExecutions(t *testing.T) {
	request := &ListConcreteExecutionsRequest{
		PageSize:  10,
		PageToken: []byte("next"),
	}

	internalResponse := &InternalListConcreteExecutionsResponse{
		Executions: []*InternalListConcreteExecutionsEntity{
			{
				ExecutionInfo:    sampleInternalWorkflowExecutionInfo(),
				VersionHistories: sampleEventData(),
			},
		},
		NextPageToken: []byte("next"),
	}

	testCases := []struct {
		name         string
		prepareMocks func(*MockExecutionStore, *MockPayloadSerializer)
		checkRes     func(*testing.T, *ListConcreteExecutionsResponse, error)
	}{
		{
			name: "success",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedStore.EXPECT().ListConcreteExecutions(gomock.Any(), request).Return(internalResponse, nil)

				mockedSerializer.EXPECT().DeserializeEvent(internalResponse.Executions[0].ExecutionInfo.CompletionEvent).Return(completionEvent(), nil)
				mockedSerializer.EXPECT().DeserializeResetPoints(internalResponse.Executions[0].ExecutionInfo.AutoResetPoints).Return(&types.ResetPoints{
					Points: []*types.ResetPointInfo{
						{
							RunID: testRunID,
						},
					},
				}, nil)
				mockedSerializer.EXPECT().DeserializeActiveClusterSelectionPolicy(internalResponse.Executions[0].ExecutionInfo.ActiveClusterSelectionPolicy).Return(generateActiveClusterSelectionPolicy(), nil)
				mockedSerializer.EXPECT().DeserializeVersionHistories(internalResponse.Executions[0].VersionHistories).Return(&types.VersionHistories{
					CurrentVersionHistoryIndex: 1,
					Histories: []*types.VersionHistory{
						{
							BranchToken: []byte("branch-token-1"),
							Items: []*types.VersionHistoryItem{
								{
									EventID: 1,
									Version: 1,
								},
							},
						},
						{
							BranchToken: []byte("branch-token-2"),
							Items: []*types.VersionHistoryItem{
								{
									EventID: 2,
									Version: 1,
								},
								{
									EventID: 3,
									Version: 2,
								},
							},
						},
					},
				}, nil)
			},
			checkRes: func(t *testing.T, response *ListConcreteExecutionsResponse, err error) {
				executionInfo := sampleWorkflowExecutionInfo()
				executionInfo.CompletionEvent = completionEvent()
				executionInfo.AutoResetPoints = &types.ResetPoints{
					Points: []*types.ResetPointInfo{
						{
							RunID: testRunID,
						},
					},
				}
				assert.Equal(t, &ListConcreteExecutionsResponse{
					Executions: []*ListConcreteExecutionsEntity{
						{
							ExecutionInfo: executionInfo,
							VersionHistories: &VersionHistories{
								CurrentVersionHistoryIndex: 1,
								Histories: []*VersionHistory{
									{
										BranchToken: []byte("branch-token-1"),
										Items: []*VersionHistoryItem{
											{
												EventID: 1,
												Version: 1,
											},
										},
									},
									{
										BranchToken: []byte("branch-token-2"),
										Items: []*VersionHistoryItem{
											{
												EventID: 2,
												Version: 1,
											},
											{
												EventID: 3,
												Version: 2,
											},
										},
									},
								},
							},
						},
					},
					PageToken: []byte("next"),
				}, response)
			},
		},
		{
			name: "persistence error",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedStore.EXPECT().ListConcreteExecutions(gomock.Any(), request).Return(nil, assert.AnError)
			},
			checkRes: func(t *testing.T, response *ListConcreteExecutionsResponse, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "deserialize execution info error",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedStore.EXPECT().ListConcreteExecutions(gomock.Any(), request).Return(internalResponse, nil)
				mockedSerializer.EXPECT().DeserializeEvent(internalResponse.Executions[0].ExecutionInfo.CompletionEvent).Return(nil, assert.AnError)
			},
			checkRes: func(t *testing.T, response *ListConcreteExecutionsResponse, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedStore := NewMockExecutionStore(ctrl)
			mockedSerializer := NewMockPayloadSerializer(ctrl)

			tc.prepareMocks(mockedStore, mockedSerializer)

			manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			res, err := manager.ListConcreteExecutions(context.Background(), request)

			tc.checkRes(t, res, err)
		})
	}
}

func TestCreateWorkflowExecution(t *testing.T) {
	for _, tc := range []struct {
		name         string
		prepareMocks func(*MockExecutionStore, *MockPayloadSerializer)
		checkRes     func(*testing.T, *CreateWorkflowExecutionResponse, error)
	}{
		{
			name: "success",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				// Prepare CreateWorkflow call
				expectedRequest := &InternalCreateWorkflowExecutionRequest{
					RangeID:                  1,
					Mode:                     CreateWorkflowModeWorkflowIDReuse,
					PreviousRunID:            testRunID,
					PreviousLastWriteVersion: 1,
					NewWorkflowSnapshot:      *sampleInternalWorkflowSnapshot(),
					WorkflowRequestMode:      CreateWorkflowRequestModeReplicated,
					CurrentTimeStamp:         time.Now(),
				}
				mockedStore.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, actualRequest *InternalCreateWorkflowExecutionRequest) (*CreateWorkflowExecutionResponse, error) {
						assert.Equal(t, expectedRequest.RangeID, actualRequest.RangeID)
						assert.Equal(t, expectedRequest.Mode, actualRequest.Mode)
						assert.Equal(t, expectedRequest.PreviousRunID, actualRequest.PreviousRunID)
						assert.Equal(t, expectedRequest.PreviousLastWriteVersion, actualRequest.PreviousLastWriteVersion)
						assert.Equal(t, expectedRequest.WorkflowRequestMode, actualRequest.WorkflowRequestMode)
						assert.Equal(t, expectedRequest.NewWorkflowSnapshot, actualRequest.NewWorkflowSnapshot)

						assert.WithinDuration(t, expectedRequest.CurrentTimeStamp, actualRequest.CurrentTimeStamp, time.Second)
						return nil, nil
					})

				// Prepare DeserializeWorkflow call
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			checkRes: func(t *testing.T, response *CreateWorkflowExecutionResponse, err error) {
				assert.Equal(t, &CreateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &MutableStateUpdateSessionStats{
						MutableStateSize:    91,
						ExecutionInfoSize:   20,
						ActivityInfoSize:    29,
						TimerInfoSize:       22,
						ChildInfoSize:       20,
						ActivityInfoCount:   1,
						TimerInfoCount:      2,
						ChildInfoCount:      1,
						TaskCountByCategory: map[HistoryTaskCategory]int{},
					},
				}, response)
			},
		},
		{
			name: "persistence error",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				// Prepare DeserializeWorkflow call
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)

				// Persistence call will fail
				mockedStore.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			checkRes: func(t *testing.T, response *CreateWorkflowExecutionResponse, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "serialize workflow snapshot error",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(nil, assert.AnError).Times(1)
			},
			checkRes: func(t *testing.T, response *CreateWorkflowExecutionResponse, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedStore := NewMockExecutionStore(ctrl)
			mockedSerializer := NewMockPayloadSerializer(ctrl)

			tc.prepareMocks(mockedStore, mockedSerializer)

			request := &CreateWorkflowExecutionRequest{
				RangeID:                  1,
				Mode:                     CreateWorkflowModeWorkflowIDReuse,
				PreviousRunID:            testRunID,
				PreviousLastWriteVersion: 1,
				NewWorkflowSnapshot:      *sampleWorkflowSnapshot(),
				WorkflowRequestMode:      CreateWorkflowRequestModeReplicated,
			}

			manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			res, err := manager.CreateWorkflowExecution(context.Background(), request)

			tc.checkRes(t, res, err)
		})
	}
}

func TestConflictResolveWorkflowExecution(t *testing.T) {
	for _, tc := range []struct {
		name         string
		request      *ConflictResolveWorkflowExecutionRequest
		prepareMocks func(*MockExecutionStore, *MockPayloadSerializer)
		checkRes     func(*testing.T, *ConflictResolveWorkflowExecutionResponse, error)
	}{
		{
			name: "only snapshot",
			request: &ConflictResolveWorkflowExecutionRequest{
				RangeID:               1,
				Mode:                  ConflictResolveWorkflowModeBypassCurrent,
				ResetWorkflowSnapshot: *sampleWorkflowSnapshot(),
				Encoding:              constants.EncodingTypeThriftRW,
			},
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				expectedRequest := &InternalConflictResolveWorkflowExecutionRequest{
					RangeID:               1,
					Mode:                  ConflictResolveWorkflowModeBypassCurrent,
					ResetWorkflowSnapshot: *sampleInternalWorkflowSnapshot(),
					CurrentTimeStamp:      time.Now(),
				}
				mockedStore.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, actualRequest *InternalConflictResolveWorkflowExecutionRequest) error {
						assert.Equal(t, expectedRequest.RangeID, actualRequest.RangeID)
						assert.Equal(t, expectedRequest.Mode, actualRequest.Mode)
						assert.Equal(t, expectedRequest.ResetWorkflowSnapshot, actualRequest.ResetWorkflowSnapshot)

						assert.WithinDuration(t, expectedRequest.CurrentTimeStamp, actualRequest.CurrentTimeStamp, time.Second)
						return nil
					})

				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleCheckSumData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(1)
			},
			checkRes: func(t *testing.T, response *ConflictResolveWorkflowExecutionResponse, err error) {
				assert.NoError(t, err)
				assert.Equal(t, &ConflictResolveWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &MutableStateUpdateSessionStats{
						MutableStateSize:    91,
						ExecutionInfoSize:   20,
						ActivityInfoSize:    29,
						TimerInfoSize:       22,
						ChildInfoSize:       20,
						ActivityInfoCount:   1,
						TimerInfoCount:      2,
						ChildInfoCount:      1,
						TaskCountByCategory: map[HistoryTaskCategory]int{},
					},
				}, response)
			},
		},
		{
			name: "only snapshot fail",
			request: &ConflictResolveWorkflowExecutionRequest{
				RangeID:               1,
				Mode:                  ConflictResolveWorkflowModeBypassCurrent,
				ResetWorkflowSnapshot: *sampleWorkflowSnapshot(),
				Encoding:              constants.EncodingTypeThriftRW,
			},
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(nil, assert.AnError).Times(1)
			},
			checkRes: func(t *testing.T, response *ConflictResolveWorkflowExecutionResponse, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "current workflow mutation",
			request: &ConflictResolveWorkflowExecutionRequest{
				RangeID:                 1,
				Mode:                    ConflictResolveWorkflowModeBypassCurrent,
				ResetWorkflowSnapshot:   *sampleWorkflowSnapshot(),
				Encoding:                constants.EncodingTypeThriftRW,
				CurrentWorkflowMutation: sampleWorkflowMutation(),
			},
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				expectedRequest := &InternalConflictResolveWorkflowExecutionRequest{
					RangeID:                 1,
					Mode:                    ConflictResolveWorkflowModeBypassCurrent,
					ResetWorkflowSnapshot:   *sampleInternalWorkflowSnapshot(),
					CurrentWorkflowMutation: sampleInternalWorkflowMutation(),
					CurrentTimeStamp:        time.Now(),
				}

				mockedStore.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, actualRequest *InternalConflictResolveWorkflowExecutionRequest) error {
					assert.Equal(t, expectedRequest.RangeID, actualRequest.RangeID)
					assert.Equal(t, expectedRequest.Mode, actualRequest.Mode)
					assert.Equal(t, expectedRequest.ResetWorkflowSnapshot, actualRequest.ResetWorkflowSnapshot)

					assert.WithinDuration(t, expectedRequest.CurrentTimeStamp, actualRequest.CurrentTimeStamp, time.Second)
					return nil
				})

				// Mutation call
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)

				// Expect mutation doubles the calls for workflow execution serialization
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(2)
				mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleResetPointsData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(activityScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(2)
				mockedSerializer.EXPECT().SerializeEvent(activityStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(2)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowScheduledEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeEvent(childWorkflowStartedEvent(), constants.EncodingTypeThriftRW).Return(sampleEventData(), nil).Times(1)
				mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleCheckSumData(), nil).Times(2)
				mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(generateActiveClusterSelectionPolicy(), constants.EncodingTypeThriftRW).Return(sampleActiveClusterSelectionPolicyData(), nil).Times(2)
			},
			checkRes: func(t *testing.T, response *ConflictResolveWorkflowExecutionResponse, err error) {
				assert.NoError(t, err)
				assert.Equal(t, &ConflictResolveWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &MutableStateUpdateSessionStats{
						MutableStateSize:    161,
						ExecutionInfoSize:   40,
						ActivityInfoSize:    49,
						TimerInfoSize:       32,
						ChildInfoSize:       40,
						ActivityInfoCount:   2,
						TimerInfoCount:      3,
						ChildInfoCount:      2,
						TaskCountByCategory: map[HistoryTaskCategory]int{},
					},
				}, response)
			},
		},
		{
			name: "new workflow snapshot",
			request: &ConflictResolveWorkflowExecutionRequest{
				RangeID:               1,
				Mode:                  ConflictResolveWorkflowModeBypassCurrent,
				ResetWorkflowSnapshot: *sampleWorkflowSnapshot(),
				Encoding:              constants.EncodingTypeThriftRW,
			},
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedSerializer.EXPECT().SerializeEvent(completionEvent(), constants.EncodingTypeThriftRW).Return(nil, assert.AnError).Times(1)
			},
			checkRes: func(t *testing.T, response *ConflictResolveWorkflowExecutionResponse, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedStore := NewMockExecutionStore(ctrl)
			mockedSerializer := NewMockPayloadSerializer(ctrl)

			tc.prepareMocks(mockedStore, mockedSerializer)

			manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			res, err := manager.ConflictResolveWorkflowExecution(context.Background(), tc.request)

			tc.checkRes(t, res, err)
		})
	}
}

func TestCreateFailoverMarkerTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), nil, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	req := &CreateFailoverMarkersRequest{
		Markers: []*FailoverMarkerTask{{
			TaskData: TaskData{
				Version:             0,
				TaskID:              0,
				VisibilityTimestamp: time.Time{},
			},
			DomainID: "1",
		}},
		RangeID:          1,
		CurrentTimeStamp: time.Now(),
	}

	mockedStore.EXPECT().CreateFailoverMarkerTasks(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *CreateFailoverMarkersRequest) error {
			assert.Equal(t, req.RangeID, req.RangeID)
			assert.Equal(t, req.Markers, req.Markers)

			assert.WithinDuration(t, req.CurrentTimeStamp, req.CurrentTimeStamp, time.Second)
			return nil
		})

	err := manager.CreateFailoverMarkerTasks(context.Background(), req)
	assert.NoError(t, err)
}

func TestGetActiveClusterSelectionPolicy(t *testing.T) {
	ctx := context.Background()
	domainID := "domainID"
	workflowID := "workflowID"
	runID := "runID"

	request := &GetActiveClusterSelectionPolicyRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	tests := []struct {
		name         string
		prepareMocks func(*MockExecutionStore, *MockPayloadSerializer)
		want         *types.ActiveClusterSelectionPolicy
		wantErr      error
	}{
		{
			name: "success",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				data := sampleActiveClusterSelectionPolicyData()
				mockedStore.EXPECT().GetActiveClusterSelectionPolicy(ctx, request).
					Return(data, nil)
				mockedSerializer.EXPECT().DeserializeActiveClusterSelectionPolicy(data).Return(sampleActiveClusterSelectionPolicy(), nil)
			},
			want: sampleActiveClusterSelectionPolicy(),
		},
		{
			name: "store error",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedStore.EXPECT().GetActiveClusterSelectionPolicy(ctx, request).
					Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
		{
			name: "store returned nil",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				mockedStore.EXPECT().GetActiveClusterSelectionPolicy(ctx, request).
					Return(nil, nil)
			},
			wantErr: &types.EntityNotExistsError{
				Message: "active cluster selection policy not found",
			},
		},
		{
			name: "deserialize error",
			prepareMocks: func(mockedStore *MockExecutionStore, mockedSerializer *MockPayloadSerializer) {
				data := sampleActiveClusterSelectionPolicyData()
				mockedStore.EXPECT().GetActiveClusterSelectionPolicy(ctx, request).
					Return(data, nil)
				mockedSerializer.EXPECT().DeserializeActiveClusterSelectionPolicy(data).Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedStore := NewMockExecutionStore(ctrl)
			mockedSerializer := NewMockPayloadSerializer(ctrl)
			manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.prepareMocks(mockedStore, mockedSerializer)

			policy, err := manager.GetActiveClusterSelectionPolicy(ctx, request)
			if test.wantErr != nil {
				assert.EqualError(t, err, test.wantErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.want, policy)
			}
		})
	}
}

func TestDeleteActiveClusterSelectionPolicy(t *testing.T) {
	ctx := context.Background()
	domainID := "domainID"
	workflowID := "workflowID"
	runID := "runID"

	request := &DeleteActiveClusterSelectionPolicyRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	tests := []struct {
		name         string
		prepareMocks func(*MockExecutionStore)
		wantErr      error
	}{
		{
			name: "success",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().DeleteActiveClusterSelectionPolicy(ctx, request).Return(nil)
			},
		},
		{
			name: "store error",
			prepareMocks: func(mockedStore *MockExecutionStore) {
				mockedStore.EXPECT().DeleteActiveClusterSelectionPolicy(ctx, request).Return(assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedStore := NewMockExecutionStore(ctrl)
			manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), nil, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.prepareMocks(mockedStore)

			err := manager.DeleteActiveClusterSelectionPolicy(ctx, request)
			if test.wantErr != nil {
				assert.EqualError(t, err, test.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func sampleInternalActivityInfo(name string) *InternalActivityInfo {
	return &InternalActivityInfo{
		Version:        1,
		ScheduleID:     1,
		ActivityID:     name,
		TaskList:       "TaskList",
		TaskListKind:   TaskListKindSticky,
		ScheduledEvent: NewDataBlob([]byte(fmt.Sprintf("%s-activity-scheduled-event", name)), constants.EncodingTypeThriftRW),
		StartedEvent:   NewDataBlob([]byte(fmt.Sprintf("%s-activity-started-event", name)), constants.EncodingTypeThriftRW),
	}
}

func sampleActivityInfo(name string, id int64) *ActivityInfo {
	return &ActivityInfo{
		Version:      1,
		ScheduleID:   1,
		ActivityID:   name,
		TaskList:     "TaskList",
		TaskListKind: TaskListKindSticky,
		ScheduledEvent: &types.HistoryEvent{
			ID: id,
		},
		StartedEvent: &types.HistoryEvent{
			ID: id,
		},
	}
}

var (
	startedTimestamp           = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	scheduledTimestamp         = time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC)
	originalScheduledTimestamp = time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)

	wfTimeout       = 10 * time.Second
	decisionTimeout = 5 * time.Second
)

func sampleInternalWorkflowExecutionInfo() *InternalWorkflowExecutionInfo {
	return &InternalWorkflowExecutionInfo{
		DomainID:                           testDomain,
		WorkflowTimeout:                    wfTimeout,
		DecisionStartToCloseTimeout:        decisionTimeout,
		DecisionStartedTimestamp:           startedTimestamp,
		DecisionScheduledTimestamp:         scheduledTimestamp,
		DecisionOriginalScheduledTimestamp: originalScheduledTimestamp,
		WorkflowID:                         testWorkflowID,
		RunID:                              testRunID,
		WorkflowTypeName:                   testWorkflowType,
		NextEventID:                        10,
		CompletionEvent:                    sampleEventData(),
		AutoResetPoints:                    sampleResetPointsData(),
		HistorySize:                        1024,
		ActiveClusterSelectionPolicy:       sampleActiveClusterSelectionPolicyData(),
	}
}

func sampleWorkflowExecutionInfo() *WorkflowExecutionInfo {
	return &WorkflowExecutionInfo{
		DomainID:                           testDomain,
		WorkflowTimeout:                    int32(wfTimeout.Seconds()),
		DecisionStartToCloseTimeout:        int32(decisionTimeout.Seconds()),
		DecisionScheduledTimestamp:         scheduledTimestamp.UnixNano(),
		DecisionStartedTimestamp:           startedTimestamp.UnixNano(),
		DecisionOriginalScheduledTimestamp: originalScheduledTimestamp.UnixNano(),
		WorkflowID:                         testWorkflowID,
		RunID:                              testRunID,
		WorkflowTypeName:                   testWorkflowType,
		NextEventID:                        10,
		CompletionEvent:                    completionEvent(),
		AutoResetPoints:                    generateResetPoints(),
		ActiveClusterSelectionPolicy:       generateActiveClusterSelectionPolicy(),
	}
}

func sampleInternalWorkflowMutation() *InternalWorkflowMutation {
	return &InternalWorkflowMutation{
		ExecutionInfo: sampleInternalWorkflowExecutionInfo(),
		UpsertActivityInfos: []*InternalActivityInfo{
			{
				Version:        1,
				ScheduledEvent: sampleEventData(),
				StartedEvent:   sampleEventData(),
			},
		},
		UpsertTimerInfos: []*TimerInfo{
			{
				TimerID: "test-timer",
			},
		},
		WorkflowTimerTasks: nil,
		UpsertChildExecutionInfos: []*InternalChildExecutionInfo{
			{
				DomainID:         testDomainID,
				WorkflowTypeName: testWorkflowType,
				Version:          1,
				InitiatedEvent:   sampleEventData(),
				StartedEvent:     sampleEventData(),
			},
		},
		Checksum:         generateChecksum(),
		ChecksumData:     sampleTestCheckSumData(),
		StartVersion:     constants.EmptyVersion,
		LastWriteVersion: constants.EmptyVersion,
	}
}

func sampleWorkflowMutation() *WorkflowMutation {
	return &WorkflowMutation{
		ExecutionInfo:  sampleWorkflowExecutionInfo(),
		ExecutionStats: sampleWorkflowExecutionStats(),
		UpsertActivityInfos: []*ActivityInfo{
			{
				Version:        1,
				ScheduledEvent: activityScheduledEvent(),
				StartedEvent:   activityStartedEvent(),
			},
		},
		UpsertTimerInfos: []*TimerInfo{{TimerID: "test-timer"}},
		Checksum:         generateChecksum(),
		UpsertChildExecutionInfos: []*ChildExecutionInfo{
			{
				DomainID:         testDomainID,
				WorkflowTypeName: testWorkflowType,
				Version:          1,
				InitiatedEvent:   childWorkflowScheduledEvent(),
				StartedEvent:     childWorkflowStartedEvent(),
			},
		},
	}
}

func sampleWorkflowSnapshot() *WorkflowSnapshot {
	return &WorkflowSnapshot{
		ExecutionInfo:  sampleWorkflowExecutionInfo(),
		ExecutionStats: sampleWorkflowExecutionStats(),
		VersionHistories: &VersionHistories{
			CurrentVersionHistoryIndex: 0,
			Histories: []*VersionHistory{
				{
					BranchToken: []byte("test-branch-token"),
					Items: []*VersionHistoryItem{
						{
							EventID: 1,
							Version: 1,
						},
					},
				},
			},
		},
		Checksum: generateChecksum(),
		ActivityInfos: []*ActivityInfo{
			{
				Version:        1,
				ScheduleID:     1,
				ActivityID:     "activity1",
				ScheduledEvent: activityScheduledEvent(),
				StartedID:      2,
				StartedEvent:   activityStartedEvent(),
				StartedTime:    startedTimestamp,
			},
		},
		ChildExecutionInfos: []*ChildExecutionInfo{
			{
				Version:          1,
				InitiatedID:      1,
				InitiatedEvent:   childWorkflowScheduledEvent(),
				StartedID:        2,
				StartedEvent:     childWorkflowStartedEvent(),
				CreateRequestID:  "create-request-id",
				DomainID:         testDomainID,
				WorkflowTypeName: testWorkflowType,
			},
		},
		TimerInfos: []*TimerInfo{
			{
				TimerID:    "test-timer",
				StartedID:  1,
				ExpiryTime: originalScheduledTimestamp,
				TaskStatus: 1,
			},
			{
				TimerID:    "test-timer-2",
				StartedID:  2,
				ExpiryTime: originalScheduledTimestamp,
				TaskStatus: 2,
			},
		},
	}
}

func sampleInternalWorkflowSnapshot() *InternalWorkflowSnapshot {
	return &InternalWorkflowSnapshot{
		ExecutionInfo:    sampleInternalWorkflowExecutionInfo(),
		VersionHistories: sampleEventData(),
		StartVersion:     1,
		LastWriteVersion: 1,
		ActivityInfos: []*InternalActivityInfo{
			{
				Version:        1,
				ScheduleID:     1,
				ActivityID:     "activity1",
				ScheduledEvent: sampleEventData(),
				StartedEvent:   sampleEventData(),
				StartedID:      2,
				StartedTime:    startedTimestamp,
			},
		},
		TimerInfos: []*TimerInfo{
			{
				TimerID:    "test-timer",
				StartedID:  1,
				ExpiryTime: originalScheduledTimestamp,
				TaskStatus: 1,
			},
			{
				TimerID:    "test-timer-2",
				StartedID:  2,
				ExpiryTime: originalScheduledTimestamp,
				TaskStatus: 2,
			},
		},
		ChildExecutionInfos: []*InternalChildExecutionInfo{
			{
				Version:          1,
				InitiatedID:      1,
				InitiatedEvent:   sampleEventData(),
				StartedID:        2,
				StartedEvent:     sampleEventData(),
				CreateRequestID:  "create-request-id",
				DomainID:         testDomainID,
				WorkflowTypeName: testWorkflowType,
			},
		},
		WorkflowTimerTasks: nil,
		Checksum:           generateChecksum(),
		ChecksumData:       sampleCheckSumData(),
	}
}

func activityScheduledEvent() *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        1,
		Timestamp: common.Ptr(scheduledTimestamp.UnixNano()),
		TaskID:    1,
	}
}

func activityStartedEvent() *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        2,
		Timestamp: common.Ptr(startedTimestamp.UnixNano()),
		TaskID:    1,
	}
}

func childWorkflowScheduledEvent() *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        1,
		Timestamp: common.Ptr(scheduledTimestamp.UnixNano()),
		TaskID:    1,
	}
}

func completionEvent() *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        99,
		Timestamp: common.Ptr(startedTimestamp.UnixNano()),
		TaskID:    1,
	}
}

func childWorkflowStartedEvent() *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        2,
		Timestamp: common.Ptr(startedTimestamp.UnixNano()),
		TaskID:    1,
	}
}

func sampleWorkflowExecutionStats() *ExecutionStats {
	return &ExecutionStats{
		HistorySize: 1024,
	}
}

func sampleTestCheckSumData() *DataBlob {
	return &DataBlob{
		Encoding: constants.EncodingTypeThriftRW,
		Data:     []byte("test-checksum"),
	}
}

func sampleEventData() *DataBlob {
	return NewDataBlob([]byte("test-event"), constants.EncodingTypeThriftRW)
}

func sampleResetPointsData() *DataBlob {
	return NewDataBlob([]byte("test-reset-points"), constants.EncodingTypeThriftRW)
}

func sampleCheckSumData() *DataBlob {
	return NewDataBlob([]byte("test-checksum"), constants.EncodingTypeThriftRW)
}

func sampleActiveClusterSelectionPolicyData() *DataBlob {
	return NewDataBlob([]byte("test-active-cluster-selection-policy"), constants.EncodingTypeThriftRW)
}

func sampleActiveClusterSelectionPolicy() *types.ActiveClusterSelectionPolicy {
	return &types.ActiveClusterSelectionPolicy{
		ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
		StickyRegion:                   "region-1",
	}
}

func sampleInternalChildExecutionInfo(initEventID, startedEventID int64) *InternalChildExecutionInfo {
	return &InternalChildExecutionInfo{
		Version:           initEventID,
		InitiatedID:       initEventID,
		InitiatedEvent:    sampleEventDataWithVersion(initEventID),
		StartedID:         startedEventID,
		StartedEvent:      sampleEventDataWithVersion(startedEventID),
		CreateRequestID:   "create-request-id",
		DomainID:          testDomainID,
		WorkflowTypeName:  testWorkflowType,
		StartedWorkflowID: "workflow-id",
		StartedRunID:      "run-id",
	}
}

func sampleEventDataWithVersion(i int64) *DataBlob {
	return NewDataBlob([]byte(fmt.Sprintf("test-event-%d", i)), constants.EncodingTypeThriftRW)
}

// TestUpdateWorkflowExecution_TimerTaskTracking verifies that UpdateWorkflowExecution
// populates WorkflowTimerTasks in the internal mutation when EnableWorkflowTimerTaskCleanup is enabled.
func TestUpdateWorkflowExecution_TimerTaskTracking(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	timerTaskID := int64(999)
	visTS := time.Now().Add(48 * time.Hour)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		EnableWorkflowTimerTaskCleanup: dynamicproperties.GetBoolPropertyFn(true),
	})

	mutation := sampleWorkflowMutation()
	mutation.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {
			&WorkflowTimeoutTask{
				WorkflowIdentifier: WorkflowIdentifier{DomainID: testDomainID, WorkflowID: "w1", RunID: "r1"},
				TaskData:           TaskData{TaskID: timerTaskID, VisibilityTimestamp: visTS},
			},
		},
	}

	mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(sampleActiveClusterSelectionPolicyData(), nil).AnyTimes()
	mockedStore.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *InternalUpdateWorkflowExecutionRequest) error {
			assert.Contains(t, req.UpdateWorkflowMutation.WorkflowTimerTasks, timerTaskID,
				"expected timer task to be tracked in WorkflowTimerTasks map")
			assert.WithinDuration(t, visTS, req.UpdateWorkflowMutation.WorkflowTimerTasks[timerTaskID], time.Millisecond)
			return nil
		}).Times(1)

	_, err := manager.UpdateWorkflowExecution(context.Background(), &UpdateWorkflowExecutionRequest{
		RangeID:                1,
		Mode:                   UpdateWorkflowModeBypassCurrent,
		UpdateWorkflowMutation: *mutation,
		Encoding:               constants.EncodingTypeThriftRW,
	})
	assert.NoError(t, err)
}

// TestCreateWorkflowExecution_TimerTaskTracking verifies that CreateWorkflowExecution
// populates WorkflowTimerTasks in the internal snapshot when EnableWorkflowTimerTaskCleanup is enabled.
func TestCreateWorkflowExecution_TimerTaskTracking(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	timerTaskID := int64(888)
	visTS := time.Now().Add(24 * time.Hour)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		EnableWorkflowTimerTaskCleanup: dynamicproperties.GetBoolPropertyFn(true),
	})

	snapshot := sampleWorkflowSnapshot()
	snapshot.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {
			&WorkflowTimeoutTask{
				WorkflowIdentifier: WorkflowIdentifier{DomainID: testDomainID, WorkflowID: "w1", RunID: "r1"},
				TaskData:           TaskData{TaskID: timerTaskID, VisibilityTimestamp: visTS},
			},
		},
	}

	mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(sampleActiveClusterSelectionPolicyData(), nil).AnyTimes()
	mockedStore.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *InternalCreateWorkflowExecutionRequest) (*CreateWorkflowExecutionResponse, error) {
			assert.Contains(t, req.NewWorkflowSnapshot.WorkflowTimerTasks, timerTaskID,
				"expected timer task to be tracked in WorkflowTimerTasks map")
			assert.WithinDuration(t, visTS, req.NewWorkflowSnapshot.WorkflowTimerTasks[timerTaskID], time.Millisecond)
			return &CreateWorkflowExecutionResponse{}, nil
		}).Times(1)

	_, err := manager.CreateWorkflowExecution(context.Background(), &CreateWorkflowExecutionRequest{
		RangeID:             1,
		NewWorkflowSnapshot: *snapshot,
	})
	assert.NoError(t, err)
}

// TestUpdateWorkflowExecution_TimerTaskTrackingFlagOff verifies that timer tasks are not
// tracked when EnableWorkflowTimerTaskCleanup is disabled.
func TestUpdateWorkflowExecution_TimerTaskTrackingFlagOff(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		EnableWorkflowTimerTaskCleanup: dynamicproperties.GetBoolPropertyFn(false),
	})

	mutation := sampleWorkflowMutation()
	mutation.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {
			&WorkflowTimeoutTask{
				WorkflowIdentifier: WorkflowIdentifier{DomainID: testDomainID, WorkflowID: "w1", RunID: "r1"},
				TaskData:           TaskData{TaskID: 999, VisibilityTimestamp: time.Now().Add(48 * time.Hour)},
			},
		},
	}

	mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(sampleActiveClusterSelectionPolicyData(), nil).AnyTimes()
	mockedStore.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *InternalUpdateWorkflowExecutionRequest) error {
			assert.Empty(t, req.UpdateWorkflowMutation.WorkflowTimerTasks,
				"should not track timer tasks when flag is off")
			return nil
		}).Times(1)

	_, err := manager.UpdateWorkflowExecution(context.Background(), &UpdateWorkflowExecutionRequest{
		RangeID:                1,
		Mode:                   UpdateWorkflowModeBypassCurrent,
		UpdateWorkflowMutation: *mutation,
		Encoding:               constants.EncodingTypeThriftRW,
	})
	assert.NoError(t, err)
}

// TestCreateWorkflowExecution_TimerTaskTrackingFlagOff verifies that timer tasks are not
// tracked when EnableWorkflowTimerTaskCleanup is disabled.
func TestCreateWorkflowExecution_TimerTaskTrackingFlagOff(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		EnableWorkflowTimerTaskCleanup: dynamicproperties.GetBoolPropertyFn(false),
	})

	snapshot := sampleWorkflowSnapshot()
	snapshot.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {
			&WorkflowTimeoutTask{
				WorkflowIdentifier: WorkflowIdentifier{DomainID: testDomainID, WorkflowID: "w1", RunID: "r1"},
				TaskData:           TaskData{TaskID: 888, VisibilityTimestamp: time.Now().Add(24 * time.Hour)},
			},
		},
	}

	mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(sampleActiveClusterSelectionPolicyData(), nil).AnyTimes()
	mockedStore.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *InternalCreateWorkflowExecutionRequest) (*CreateWorkflowExecutionResponse, error) {
			assert.Empty(t, req.NewWorkflowSnapshot.WorkflowTimerTasks,
				"should not track timer tasks when flag is off")
			return &CreateWorkflowExecutionResponse{}, nil
		}).Times(1)

	_, err := manager.CreateWorkflowExecution(context.Background(), &CreateWorkflowExecutionRequest{
		RangeID:             1,
		NewWorkflowSnapshot: *snapshot,
	})
	assert.NoError(t, err)
}

// TestConflictResolveWorkflowExecution_TimerTaskTracking verifies that ConflictResolveWorkflowExecution
// tracks timer tasks in all three snapshot/mutation components when the feature flag is enabled.
func TestConflictResolveWorkflowExecution_TimerTaskTracking(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	resetTaskID := int64(111)
	currentTaskID := int64(222)
	newTaskID := int64(333)
	visTS := time.Now().Add(48 * time.Hour)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		EnableWorkflowTimerTaskCleanup: dynamicproperties.GetBoolPropertyFn(true),
	})

	makeTimerTask := func(taskID int64) Task {
		return &WorkflowTimeoutTask{
			WorkflowIdentifier: WorkflowIdentifier{DomainID: testDomainID, WorkflowID: "w1", RunID: "r1"},
			TaskData:           TaskData{TaskID: taskID, VisibilityTimestamp: visTS},
		}
	}

	resetSnapshot := sampleWorkflowSnapshot()
	resetSnapshot.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {makeTimerTask(resetTaskID)},
	}

	currentMutation := sampleWorkflowMutation()
	currentMutation.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {makeTimerTask(currentTaskID)},
	}

	newSnapshot := sampleWorkflowSnapshot()
	newSnapshot.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {makeTimerTask(newTaskID)},
	}

	mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(sampleActiveClusterSelectionPolicyData(), nil).AnyTimes()
	mockedStore.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *InternalConflictResolveWorkflowExecutionRequest) error {
			assert.Contains(t, req.ResetWorkflowSnapshot.WorkflowTimerTasks, resetTaskID,
				"reset snapshot timer task should be tracked")
			assert.Contains(t, req.CurrentWorkflowMutation.WorkflowTimerTasks, currentTaskID,
				"current mutation timer task should be tracked")
			assert.Contains(t, req.NewWorkflowSnapshot.WorkflowTimerTasks, newTaskID,
				"new snapshot timer task should be tracked")
			return nil
		}).Times(1)

	_, err := manager.ConflictResolveWorkflowExecution(context.Background(), &ConflictResolveWorkflowExecutionRequest{
		RangeID:                 1,
		Mode:                    ConflictResolveWorkflowModeBypassCurrent,
		ResetWorkflowSnapshot:   *resetSnapshot,
		CurrentWorkflowMutation: currentMutation,
		NewWorkflowSnapshot:     newSnapshot,
		Encoding:                constants.EncodingTypeThriftRW,
	})
	assert.NoError(t, err)
}

// TestConflictResolveWorkflowExecution_TimerTaskTrackingFlagOff verifies that timer tasks are
// not tracked when EnableWorkflowTimerTaskCleanup is disabled.
func TestConflictResolveWorkflowExecution_TimerTaskTrackingFlagOff(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		EnableWorkflowTimerTaskCleanup: dynamicproperties.GetBoolPropertyFn(false),
	})

	makeTimerTask := func(taskID int64) Task {
		return &WorkflowTimeoutTask{
			WorkflowIdentifier: WorkflowIdentifier{DomainID: testDomainID, WorkflowID: "w1", RunID: "r1"},
			TaskData:           TaskData{TaskID: taskID, VisibilityTimestamp: time.Now().Add(48 * time.Hour)},
		}
	}

	resetSnapshot := sampleWorkflowSnapshot()
	resetSnapshot.TasksByCategory = map[HistoryTaskCategory][]Task{
		HistoryTaskCategoryTimer: {makeTimerTask(111)},
	}

	mockedSerializer.EXPECT().SerializeEvent(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeVersionHistories(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeResetPoints(gomock.Any(), gomock.Any()).Return(sampleEventData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeChecksum(gomock.Any(), gomock.Any()).Return(sampleTestCheckSumData(), nil).AnyTimes()
	mockedSerializer.EXPECT().SerializeActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(sampleActiveClusterSelectionPolicyData(), nil).AnyTimes()
	mockedStore.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *InternalConflictResolveWorkflowExecutionRequest) error {
			assert.Empty(t, req.ResetWorkflowSnapshot.WorkflowTimerTasks,
				"should not track timer tasks when flag is off")
			return nil
		}).Times(1)

	_, err := manager.ConflictResolveWorkflowExecution(context.Background(), &ConflictResolveWorkflowExecutionRequest{
		RangeID:               1,
		Mode:                  ConflictResolveWorkflowModeBypassCurrent,
		ResetWorkflowSnapshot: *resetSnapshot,
		Encoding:              constants.EncodingTypeThriftRW,
	})
	assert.NoError(t, err)
}

func TestCleanupWorkflowTimerTasks_DeletesLongLivedTimers(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	minTTL := time.Hour
	longLivedID := int64(1)
	longLivedTS := time.Now().Add(48 * time.Hour)
	shortLivedID := int64(2)
	shortLivedTS := time.Now().Add(30 * time.Minute) // < MinTTL, should be skipped
	pastID := int64(3)
	pastTS := time.Now().Add(-5 * time.Minute) // already fired, should be skipped

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		WorkflowTimerTaskCleanupMinTTL: dynamicproperties.GetDurationPropertyFn(minTTL),
	})

	mockedStore.EXPECT().GetShardID().Return(0).AnyTimes()
	mockedStore.EXPECT().SelectWorkflowTimerTasks(gomock.Any(), 0, testDomainID, testWorkflowID, testRunID).
		Return(map[int64]time.Time{
			longLivedID:  longLivedTS,
			shortLivedID: shortLivedTS,
			pastID:       pastTS,
		}, nil)

	// Only the long-lived timer should be deleted.
	mockedStore.EXPECT().DeleteTimerTask(gomock.Any(), gomock.Cond(func(req *DeleteTimerTaskRequest) bool {
		return req.TaskID == longLivedID && req.DomainID == testDomainID
	})).Return(nil).Times(1)

	err := manager.CleanupWorkflowTimerTasks(context.Background(), &CleanupWorkflowTimerTasksRequest{
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
	})
	assert.NoError(t, err)
}

func TestCleanupWorkflowTimerTasks_EmptyMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding:          dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		WorkflowTimerTaskCleanupMinTTL: dynamicproperties.GetDurationPropertyFn(time.Hour),
	})

	mockedStore.EXPECT().GetShardID().Return(0).AnyTimes()
	mockedStore.EXPECT().SelectWorkflowTimerTasks(gomock.Any(), 0, testDomainID, testWorkflowID, testRunID).
		Return(nil, nil)

	err := manager.CleanupWorkflowTimerTasks(context.Background(), &CleanupWorkflowTimerTasksRequest{
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
	})
	assert.NoError(t, err)
}

func TestRemoveWorkflowTimerTaskTracking(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockedStore := NewMockExecutionStore(ctrl)
	mockedSerializer := NewMockPayloadSerializer(ctrl)

	taskID := int64(42)
	manager := NewExecutionManagerImpl(mockedStore, testlogger.New(t), mockedSerializer, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	mockedStore.EXPECT().GetShardID().Return(0).AnyTimes()
	mockedStore.EXPECT().DeleteWorkflowTimerTaskEntry(gomock.Any(), 0, testDomainID, testWorkflowID, testRunID, taskID).
		Return(nil).Times(1)

	err := manager.RemoveWorkflowTimerTaskTracking(context.Background(), &RemoveWorkflowTimerTaskTrackingRequest{
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		TaskID:     taskID,
	})
	assert.NoError(t, err)
}
