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

package ndc

import (
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestReplicationTaskResetEvent(t *testing.T) {
	clusterMetadata := cluster.GetTestClusterMetadata(true)
	historySerializer := persistence.NewPayloadSerializer()
	taskStartTime := time.Now()
	domainID := uuid.New()
	workflowID := uuid.New()
	runID := uuid.New()
	versionHistoryItems := []*types.VersionHistoryItem{}
	versionHistoryItems = append(versionHistoryItems, persistence.NewVersionHistoryItem(3, 0).ToInternalType())

	eventType := types.EventTypeDecisionTaskFailed
	resetCause := types.DecisionTaskFailedCauseResetWorkflow
	event := &types.HistoryEvent{
		ID:        3,
		EventType: &eventType,
		Version:   0,
		DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
			BaseRunID:        uuid.New(),
			ForkEventVersion: 0,
			NewRunID:         runID,
			Cause:            &resetCause,
		},
	}
	events := []*types.HistoryEvent{}
	events = append(events, event)
	eventsBlob, err := historySerializer.SerializeBatchEvents(events, common.EncodingTypeThriftRW)
	require.NoError(t, err)
	request := &types.ReplicateEventsV2Request{
		DomainUUID: domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		VersionHistoryItems: versionHistoryItems,
		Events:              eventsBlob.ToInternal(),
		NewRunEvents:        nil,
	}

	task, err := newReplicationTask(clusterMetadata, historySerializer, taskStartTime, testlogger.New(t), request)
	require.NoError(t, err)
	assert.True(t, task.isWorkflowReset())
}

func TestNewReplicationTask(t *testing.T) {
	encodingType1 := types.EncodingTypeJSON
	encodingType2 := types.EncodingTypeThriftRW

	historyEvent1 := &types.HistoryEvent{
		Version: 0,
	}
	historyEvent2 := &types.HistoryEvent{
		Version: 1,
	}

	historyEvents := []*types.HistoryEvent{historyEvent1, historyEvent2}
	serializer := persistence.NewPayloadSerializer()
	serializedEvents, err := serializer.SerializeBatchEvents(historyEvents, common.EncodingTypeThriftRW)
	assert.NoError(t, err)

	historyEvent3 := &types.HistoryEvent{
		ID:      0,
		Version: 2,
	}
	historyEvent4 := &types.HistoryEvent{
		ID:      1,
		Version: 3,
	}

	historyEvents2 := []*types.HistoryEvent{historyEvent3}
	serializedEvents2, err := serializer.SerializeBatchEvents(historyEvents2, common.EncodingTypeThriftRW)
	assert.NoError(t, err)

	historyEvents3 := []*types.HistoryEvent{historyEvent3, historyEvent4}
	serializedEvents3, err := serializer.SerializeBatchEvents(historyEvents3, common.EncodingTypeThriftRW)
	assert.NoError(t, err)

	historyEvents4 := []*types.HistoryEvent{historyEvent4}
	serializedEvents4, err := serializer.SerializeBatchEvents(historyEvents4, common.EncodingTypeThriftRW)
	assert.NoError(t, err)

	tests := map[string]struct {
		request          *types.ReplicateEventsV2Request
		eventsAffordance func() []*types.HistoryEvent
		expectedErrorMsg string
	}{
		"Case1: empty case": {
			request:          &types.ReplicateEventsV2Request{},
			expectedErrorMsg: "invalid domain ID",
		},
		"Case2: nil case": {
			request:          nil,
			expectedErrorMsg: "invalid domain ID",
		},
		"Case3-1: fail case with invalid domain id": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID: "12345678-1234-5678-9012-",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
				VersionHistoryItems: nil,
				Events:              nil,
				NewRunEvents:        nil,
			},
			expectedErrorMsg: "invalid domain ID",
		},
		"Case3-2: fail case in validateReplicateEventsRequest with invalid run id": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID: "12345678-1234-5678-9012-123456789011",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
				VersionHistoryItems: nil,
				Events:              nil,
				NewRunEvents:        nil,
			},
			expectedErrorMsg: "invalid run ID",
		},
		"Case3-3: fail case in validateReplicateEventsRequest with invalid workflow execution": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID:          "12345678-1234-5678-9012-123456789011",
				WorkflowExecution:   nil,
				VersionHistoryItems: nil,
				Events:              nil,
				NewRunEvents:        nil,
			},
			expectedErrorMsg: "invalid execution",
		},
		"Case3-4: fail case in validateReplicateEventsRequest with event is empty": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID: "12345678-1234-5678-9012-123456789011",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "12345678-1234-5678-9012-123456789012",
				},
				VersionHistoryItems: nil,
				Events:              nil,
				NewRunEvents:        nil,
			},
			expectedErrorMsg: "encounter empty history batch",
		},
		"Case3-5: fail case in validateReplicateEventsRequest with DeserializeBatchEvents error": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID: "12345678-1234-5678-9012-123456789011",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "12345678-1234-5678-9012-123456789012",
				},
				VersionHistoryItems: nil,
				Events: &types.DataBlob{
					EncodingType: &encodingType1,
					Data:         []byte("test-data"),
				},
				NewRunEvents: nil,
			},
			expectedErrorMsg: "cadence deserialization error: DeserializeBatchEvents encoding: \"thriftrw\", error: Invalid binary encoding version.",
		},
		"Case3-6: fail case in validateReplicateEventsRequest with event ID mismatch": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID: "12345678-1234-5678-9012-123456789011",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "12345678-1234-5678-9012-123456789012",
				},
				VersionHistoryItems: nil,
				Events: &types.DataBlob{
					EncodingType: &encodingType2,
					Data:         serializedEvents.Data,
				},
				NewRunEvents: nil,
			},
			expectedErrorMsg: "event ID mismatch",
		},
		"Case3-7: fail case in validateReplicateEventsRequest with event version mismatch": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID: "12345678-1234-5678-9012-123456789011",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "12345678-1234-5678-9012-123456789012",
				},
				VersionHistoryItems: nil,
				Events: &types.DataBlob{
					EncodingType: &encodingType2,
					Data:         serializedEvents2.Data,
				},
				NewRunEvents: &types.DataBlob{
					EncodingType: &encodingType2,
					Data:         serializedEvents3.Data,
				},
			},
			expectedErrorMsg: "event version mismatch",
		},
		"Case3-8: fail case in validateReplicateEventsRequest with ErrEventVersionMismatch": {
			request: &types.ReplicateEventsV2Request{
				DomainUUID: "12345678-1234-5678-9012-123456789011",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "12345678-1234-5678-9012-123456789012",
				},
				VersionHistoryItems: nil,
				Events: &types.DataBlob{
					EncodingType: &encodingType2,
					Data:         serializedEvents2.Data,
				},
				NewRunEvents: &types.DataBlob{
					EncodingType: &encodingType2,
					Data:         serializedEvents4.Data,
				},
			},
			expectedErrorMsg: "event version mismatch",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			replicator := createTestHistoryReplicator(t)
			_, err := newReplicationTask(replicator.clusterMetadata,
				replicator.historySerializer,
				time.Now(),
				replicator.logger,
				test.request,
			)
			assert.Equal(t, test.expectedErrorMsg, err.Error())
		})
	}
}

func Test_getWorkflowResetMetadata(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance     func(mockTask *replicationTaskImpl)
		expectBaseRunID        string
		expectNewRunID         string
		expectBaseEventVersion int64
		expectIsReset          bool
	}{
		"Case1: DecisionTaskFailed with reset workflow": {
			mockTaskAffordance: func(mockTask *replicationTaskImpl) {
				decisionTaskFailedEvent := &types.HistoryEvent{
					EventType: types.EventTypeDecisionTaskFailed.Ptr(),
					DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
						BaseRunID:        "base-run-id",
						ForkEventVersion: 1,
						NewRunID:         "new-run-id",
						Cause:            types.DecisionTaskFailedCauseResetWorkflow.Ptr(),
					},
				}
				mockTask.firstEvent = decisionTaskFailedEvent
			},
			expectBaseRunID:        "base-run-id",
			expectNewRunID:         "new-run-id",
			expectBaseEventVersion: 1,
			expectIsReset:          true,
		},
		"Case2: DecisionTaskTimedOut with reset workflow": {
			mockTaskAffordance: func(mockTask *replicationTaskImpl) {
				decisionTaskTimedOutEvent := &types.HistoryEvent{
					EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
					DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
						BaseRunID:        "base-run-id-timedout",
						ForkEventVersion: 2,
						NewRunID:         "new-run-id-timedout",
						Cause:            types.DecisionTaskTimedOutCauseReset.Ptr(),
					},
				}
				mockTask.firstEvent = decisionTaskTimedOutEvent
			},
			expectBaseRunID:        "base-run-id-timedout",
			expectNewRunID:         "new-run-id-timedout",
			expectBaseEventVersion: 2,
			expectIsReset:          true,
		},
		"Case3: DecisionTaskFailed without reset workflow": {
			mockTaskAffordance: func(mockTask *replicationTaskImpl) {
				decisionTaskFailedEvent := &types.HistoryEvent{
					EventType: types.EventTypeDecisionTaskFailed.Ptr(),
					DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
						BaseRunID:        "base-run-id",
						ForkEventVersion: 1,
						NewRunID:         "new-run-id",
						Cause:            types.DecisionTaskFailedCause.Ptr(0),
					},
				}
				mockTask.firstEvent = decisionTaskFailedEvent
			},
			expectBaseRunID:        "base-run-id",
			expectNewRunID:         "new-run-id",
			expectBaseEventVersion: 1,
			expectIsReset:          false,
		},
		"Case4: DecisionTaskTimedOut without reset workflow": {
			mockTaskAffordance: func(mockTask *replicationTaskImpl) {
				decisionTaskTimedOutEvent := &types.HistoryEvent{
					EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
					DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
						BaseRunID:        "base-run-id-timedout",
						ForkEventVersion: 2,
						NewRunID:         "new-run-id-timedout",
						Cause:            types.DecisionTaskTimedOutCause.Ptr(0),
					},
				}
				mockTask.firstEvent = decisionTaskTimedOutEvent
			},
			expectBaseRunID:        "base-run-id-timedout",
			expectNewRunID:         "new-run-id-timedout",
			expectBaseEventVersion: 2,
			expectIsReset:          false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a mock task
			mockTask := &replicationTaskImpl{}

			// Apply test case mock behavior
			test.mockTaskAffordance(mockTask)

			// Call the method under test
			baseRunID, newRunID, baseEventVersion, isReset := mockTask.getWorkflowResetMetadata()

			// Assertions
			assert.Equal(t, test.expectBaseRunID, baseRunID)
			assert.Equal(t, test.expectNewRunID, newRunID)
			assert.Equal(t, test.expectBaseEventVersion, baseEventVersion)
			assert.Equal(t, test.expectIsReset, isReset)
		})
	}
}

func Test_splitTask(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance func(mockTask *replicationTaskImpl)
		taskStartTime      time.Time
		expectedNewRunTask bool
		expectedError      error
	}{
		"Case1: success case with valid newEvents and continuedAsNew event": {
			mockTaskAffordance: func(mockTask *replicationTaskImpl) {
				timestamp1 := time.Now().UnixNano()
				timestamp2 := time.Now().Add(1 * time.Second).UnixNano()

				mockTask.newEvents = []*types.HistoryEvent{
					{ID: 1, Version: 1, Timestamp: &timestamp1},
					{ID: 2, Version: 1, Timestamp: &timestamp2},
				}
				mockTask.firstEvent = &types.HistoryEvent{
					EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
					WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
						NewExecutionRunID: "new-run-id",
					},
				}
				mockTask.lastEvent = &types.HistoryEvent{
					EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
					ID:        2,
					Version:   1,
					WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
						NewExecutionRunID: "new-run-id",
					},
				}
				// Initialize execution to avoid nil pointer error
				mockTask.execution = &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}
				// Assign a logger
				mockTask.logger = log.NewNoop()
			},
			taskStartTime:      time.Now(),
			expectedNewRunTask: true,
			expectedError:      nil,
		},
		"Case2: error when no newEvents": {
			mockTaskAffordance: func(mockTask *replicationTaskImpl) {
				mockTask.newEvents = nil
				// Initialize execution to avoid nil pointer error
				mockTask.execution = &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}
				mockTask.logger = log.NewNoop() // Assign a logger
			},
			taskStartTime:      time.Now(),
			expectedNewRunTask: false,
			expectedError:      ErrNoNewRunHistory,
		},
		"Case3: error when lastEvent is not continuedAsNew": {
			mockTaskAffordance: func(mockTask *replicationTaskImpl) {
				timestamp1 := time.Now().UnixNano()

				mockTask.newEvents = []*types.HistoryEvent{
					{ID: 1, Version: 1, Timestamp: &timestamp1},
				}
				mockTask.lastEvent = &types.HistoryEvent{
					EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(), // Not continuedAsNew
					ID:        1,
					Version:   1,
				}
				// Initialize execution to avoid nil pointer error
				mockTask.execution = &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}
				mockTask.logger = log.NewNoop() // Assign a logger
			},
			taskStartTime:      time.Now(),
			expectedNewRunTask: false,
			expectedError:      ErrLastEventIsNotContinueAsNew,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a mock task
			mockTask := &replicationTaskImpl{}

			// Apply the mock behavior based on the test case
			test.mockTaskAffordance(mockTask)

			// Call the method under test
			_, newRunTask, err := mockTask.splitTask(test.taskStartTime)

			// Assertions
			if test.expectedError != nil {
				assert.Equal(t, test.expectedError, err)
				assert.Nil(t, newRunTask)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, newRunTask)
				assert.Equal(t, "new-run-id", newRunTask.getRunID())
			}
		})
	}
}

func Test_replicationTaskImpl_Getters(t *testing.T) {
	// Setup some common values for testing
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: "test-workflow-id",
		RunID:      "test-run-id",
	}
	historyEvents := []*types.HistoryEvent{
		{ID: 1, Version: 1},
		{ID: 2, Version: 1},
	}
	newHistoryEvents := []*types.HistoryEvent{
		{ID: 3, Version: 1},
		{ID: 4, Version: 1},
	}
	logger := log.NewNoop()
	versionHistory := persistence.NewVersionHistory(nil, []*persistence.VersionHistoryItem{
		{EventID: 1, Version: 1},
	})

	task := &replicationTaskImpl{
		domainID:       "test-domain-id",
		execution:      workflowExecution,
		version:        123,
		sourceCluster:  "test-cluster",
		eventTime:      time.Now(),
		events:         historyEvents,
		newEvents:      newHistoryEvents,
		logger:         logger,
		versionHistory: versionHistory,
	}

	tests := map[string]struct {
		testFunc       func() interface{}
		expectedResult interface{}
	}{
		"getDomainID": {
			testFunc:       func() interface{} { return task.getDomainID() },
			expectedResult: "test-domain-id",
		},
		"getWorkflowID": {
			testFunc:       func() interface{} { return task.getWorkflowID() },
			expectedResult: "test-workflow-id",
		},
		"getRunID": {
			testFunc:       func() interface{} { return task.getRunID() },
			expectedResult: "test-run-id",
		},
		"getEventTime": {
			testFunc:       func() interface{} { return task.getEventTime() },
			expectedResult: task.eventTime,
		},
		"getVersion": {
			testFunc:       func() interface{} { return task.getVersion() },
			expectedResult: int64(123),
		},
		"getSourceCluster": {
			testFunc:       func() interface{} { return task.getSourceCluster() },
			expectedResult: "test-cluster",
		},
		"getEvents": {
			testFunc:       func() interface{} { return task.getEvents() },
			expectedResult: historyEvents,
		},
		"getNewEvents": {
			testFunc:       func() interface{} { return task.getNewEvents() },
			expectedResult: newHistoryEvents,
		},
		"getLogger": {
			testFunc:       func() interface{} { return task.getLogger() },
			expectedResult: logger,
		},
		"getVersionHistory": {
			testFunc:       func() interface{} { return task.getVersionHistory() },
			expectedResult: versionHistory,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expectedResult, test.testFunc())
		})
	}
}
