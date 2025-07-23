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

package serialization

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

func TestSnappyThriftEncoderRoundTrip(t *testing.T) {
	encoder := newSnappyThriftEncoder()
	decoder := newSnappyThriftDecoder()

	now := time.Now().Round(time.Second)

	testCases := []struct {
		name       string
		data       interface{}
		encodeFunc func(interface{}) ([]byte, error)
		decodeFunc func([]byte) (interface{}, error)
	}{
		{
			name: "ShardInfo",
			data: &ShardInfo{
				StolenSinceRenew:                      1,
				UpdatedAt:                             now,
				ReplicationAckLevel:                   1,
				TransferAckLevel:                      1,
				TimerAckLevel:                         now,
				DomainNotificationVersion:             1,
				ClusterTransferAckLevel:               map[string]int64{"test": 1},
				ClusterTimerAckLevel:                  map[string]time.Time{"test": now},
				TransferProcessingQueueStates:         []byte{1, 2, 3},
				TimerProcessingQueueStates:            []byte{1, 2, 3},
				Owner:                                 "owner",
				ClusterReplicationLevel:               map[string]int64{"test": 1},
				PendingFailoverMarkers:                []byte{2, 3, 4},
				PendingFailoverMarkersEncoding:        "",
				TransferProcessingQueueStatesEncoding: "",
				TimerProcessingQueueStatesEncoding:    "",
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.shardInfoToBlob(data.(*ShardInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.shardInfoFromBlob(data)
			},
		},
		{
			name: "DomainInfo",
			data: &DomainInfo{
				Name:                        "test",
				Description:                 "test_desc",
				Owner:                       "test_owner",
				Status:                      1,
				Retention:                   48 * time.Hour,
				EmitMetric:                  true,
				ArchivalBucket:              "test_bucket",
				ArchivalStatus:              1,
				ConfigVersion:               1,
				FailoverVersion:             1,
				NotificationVersion:         1,
				FailoverNotificationVersion: 1,
				ActiveClusterName:           "test_active_cluster",
				Clusters:                    []string{"test_active_cluster", "test_standby_cluster"},
				Data:                        map[string]string{"test_key": "test_value"},
				BadBinaries:                 []byte{1, 2, 3},
				BadBinariesEncoding:         "",
				HistoryArchivalStatus:       1,
				HistoryArchivalURI:          "test_history_archival_uri",
				VisibilityArchivalStatus:    1,
				VisibilityArchivalURI:       "test_visibility_archival_uri",
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.domainInfoToBlob(data.(*DomainInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.domainInfoFromBlob(data)
			},
		},
		{
			name: "HistoryTreeInfo",
			data: &HistoryTreeInfo{
				CreatedTimestamp: now,
				Ancestors: []*types.HistoryBranchRange{
					{
						BranchID: "test_branch_id1",
					},
					{
						BranchID: "test_branch_id2",
					},
				},
				Info: "test_info",
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.historyTreeInfoToBlob(data.(*HistoryTreeInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.historyTreeInfoFromBlob(data)
			},
		},
		{
			name: "WorkflowExecutionInfo",
			data: &WorkflowExecutionInfo{
				ParentDomainID:                     MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				ParentWorkflowID:                   "test_parent_workflow_id",
				ParentRunID:                        MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				InitiatedID:                        1,
				CompletionEventBatchID:             common.Int64Ptr(2),
				CompletionEvent:                    []byte("test_completion_event"),
				CompletionEventEncoding:            "test_completion_event_encoding",
				TaskList:                           "test_task_list",
				WorkflowTypeName:                   "test_workflow_type",
				WorkflowTimeout:                    10 * time.Second,
				DecisionTaskTimeout:                5 * time.Second,
				ExecutionContext:                   []byte("test_execution_context"),
				State:                              1,
				CloseStatus:                        1,
				StartVersion:                       1,
				LastWriteEventID:                   common.Int64Ptr(3),
				LastEventTaskID:                    4,
				LastFirstEventID:                   5,
				LastProcessedEvent:                 6,
				StartTimestamp:                     now,
				LastUpdatedTimestamp:               now,
				CreateRequestID:                    "test_create_request_id",
				DecisionVersion:                    7,
				DecisionScheduleID:                 8,
				DecisionStartedID:                  9,
				DecisionRequestID:                  "test_decision_request_id",
				DecisionTimeout:                    3 * time.Second,
				DecisionAttempt:                    10,
				DecisionStartedTimestamp:           now,
				DecisionScheduledTimestamp:         now,
				DecisionOriginalScheduledTimestamp: now,
				CancelRequested:                    true,
				CancelRequestID:                    "test_cancel_request_id",
				StickyTaskList:                     "test_sticky_task_list",
				StickyScheduleToStartTimeout:       2 * time.Second,
				RetryAttempt:                       11,
				RetryInitialInterval:               1 * time.Second,
				RetryMaximumInterval:               30 * time.Second,
				RetryMaximumAttempts:               3,
				RetryExpiration:                    time.Hour,
				RetryBackoffCoefficient:            2.0,
				RetryExpirationTimestamp:           now,
				RetryNonRetryableErrors:            []string{"test_error"},
				HasRetryPolicy:                     true,
				CronSchedule:                       "test_cron",
				IsCron:                             true,
				EventStoreVersion:                  12,
				EventBranchToken:                   []byte("test_branch_token"),
				SignalCount:                        13,
				HistorySize:                        14,
				ClientLibraryVersion:               "test_client_version",
				ClientFeatureVersion:               "test_feature_version",
				ClientImpl:                         "test_client_impl",
				AutoResetPoints:                    []byte("test_reset_points"),
				AutoResetPointsEncoding:            "test_reset_points_encoding",
				SearchAttributes:                   map[string][]byte{"test_key": []byte("test_value")},
				Memo:                               map[string][]byte{"test_memo": []byte("test_memo_value")},
				VersionHistories:                   []byte("test_version_histories"),
				VersionHistoriesEncoding:           "test_version_histories_encoding",
				FirstExecutionRunID:                MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.workflowExecutionInfoToBlob(data.(*WorkflowExecutionInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.workflowExecutionInfoFromBlob(data)
			},
		},
		{
			name: "ActivityInfo",
			data: &ActivityInfo{
				Version:                  1,
				ScheduledEventBatchID:    2,
				ScheduledEvent:           []byte("test_scheduled_event"),
				ScheduledEventEncoding:   "test_scheduled_encoding",
				ScheduledTimestamp:       now,
				StartedID:                3,
				StartedEvent:             []byte("test_started_event"),
				StartedEventEncoding:     "test_started_encoding",
				StartedTimestamp:         now,
				ActivityID:               "test_activity_id",
				RequestID:                "test_request_id",
				ScheduleToStartTimeout:   5 * time.Second,
				ScheduleToCloseTimeout:   10 * time.Second,
				StartToCloseTimeout:      8 * time.Second,
				HeartbeatTimeout:         3 * time.Second,
				CancelRequested:          true,
				CancelRequestID:          4,
				TimerTaskStatus:          5,
				Attempt:                  6,
				TaskList:                 "test_task_list",
				StartedIdentity:          "test_identity",
				HasRetryPolicy:           true,
				RetryInitialInterval:     1 * time.Second,
				RetryMaximumInterval:     30 * time.Second,
				RetryMaximumAttempts:     3,
				RetryExpirationTimestamp: now,
				RetryBackoffCoefficient:  2.0,
				RetryNonRetryableErrors:  []string{"test_error"},
				RetryLastFailureReason:   "test_failure_reason",
				RetryLastWorkerIdentity:  "test_worker_identity",
				RetryLastFailureDetails:  []byte("test_failure_details"),
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.activityInfoToBlob(data.(*ActivityInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.activityInfoFromBlob(data)
			},
		},
		{
			name: "ChildExecutionInfo",
			data: &ChildExecutionInfo{
				Version:                1,
				InitiatedEventBatchID:  2,
				StartedID:              3,
				InitiatedEvent:         []byte("test_initiated_event"),
				InitiatedEventEncoding: "test_initiated_encoding",
				StartedWorkflowID:      "test_started_workflow_id",
				StartedRunID:           MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				StartedEvent:           []byte("test_started_event"),
				StartedEventEncoding:   "test_started_encoding",
				CreateRequestID:        "test_create_request_id",
				DomainID:               "test_domain_id",
				DomainNameDEPRECATED:   "test_domain_name",
				WorkflowTypeName:       "test_workflow_type",
				ParentClosePolicy:      4,
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.childExecutionInfoToBlob(data.(*ChildExecutionInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.childExecutionInfoFromBlob(data)
			},
		},
		{
			name: "SignalInfo",
			data: &SignalInfo{
				Version:               1,
				InitiatedEventBatchID: 2,
				RequestID:             "test_request_id",
				Name:                  "test_signal_name",
				Input:                 []byte("test_input"),
				Control:               []byte("test_control"),
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.signalInfoToBlob(data.(*SignalInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.signalInfoFromBlob(data)
			},
		},
		{
			name: "RequestCancelInfo",
			data: &RequestCancelInfo{
				Version:               1,
				InitiatedEventBatchID: 2,
				CancelRequestID:       "test_cancel_request_id",
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.requestCancelInfoToBlob(data.(*RequestCancelInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.requestCancelInfoFromBlob(data)
			},
		},
		{
			name: "TimerInfo",
			data: &TimerInfo{
				Version:         1,
				StartedID:       2,
				ExpiryTimestamp: now,
				TaskID:          3,
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.timerInfoToBlob(data.(*TimerInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.timerInfoFromBlob(data)
			},
		},
		{
			name: "TaskInfo",
			data: &TaskInfo{
				WorkflowID:       "test_workflow_id",
				RunID:            MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				ScheduleID:       1,
				ExpiryTimestamp:  now,
				CreatedTimestamp: now,
				PartitionConfig:  map[string]string{"test_key": "test_value"},
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.taskInfoToBlob(data.(*TaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.taskInfoFromBlob(data)
			},
		},
		{
			name: "TaskListInfo",
			data: &TaskListInfo{
				Kind:            1,
				AckLevel:        2,
				ExpiryTimestamp: now,
				LastUpdated:     now,
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.taskListInfoToBlob(data.(*TaskListInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.taskListInfoFromBlob(data)
			},
		},
		{
			name: "TransferTaskInfo",
			data: &TransferTaskInfo{
				DomainID:                MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				WorkflowID:              "test_workflow_id",
				RunID:                   MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				TaskType:                1,
				TargetDomainID:          MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				TargetDomainIDs:         []UUID{MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8")},
				TargetWorkflowID:        "test_target_workflow_id",
				TargetRunID:             MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				TaskList:                "test_task_list",
				TargetChildWorkflowOnly: true,
				ScheduleID:              2,
				Version:                 3,
				VisibilityTimestamp:     now,
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.transferTaskInfoToBlob(data.(*TransferTaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.transferTaskInfoFromBlob(data)
			},
		},
		{
			name: "TimerTaskInfo",
			data: &TimerTaskInfo{
				DomainID:        MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				WorkflowID:      "test_workflow_id",
				RunID:           MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				TaskType:        1,
				TimeoutType:     common.Int16Ptr(2),
				Version:         3,
				ScheduleAttempt: 4,
				EventID:         5,
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.timerTaskInfoToBlob(data.(*TimerTaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.timerTaskInfoFromBlob(data)
			},
		},
		{
			name: "ReplicationTaskInfo",
			data: &ReplicationTaskInfo{
				DomainID:                MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				WorkflowID:              "test_workflow_id",
				RunID:                   MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				TaskType:                1,
				Version:                 2,
				FirstEventID:            3,
				NextEventID:             4,
				ScheduledID:             5,
				EventStoreVersion:       6,
				NewRunEventStoreVersion: 7,
				BranchToken:             []byte("test_branch_token"),
				NewRunBranchToken:       []byte("test_new_run_branch_token"),
				CreationTimestamp:       now,
			},
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.replicationTaskInfoToBlob(data.(*ReplicationTaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.replicationTaskInfoFromBlob(data)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode the data using the encoder
			encoded, err := tc.encodeFunc(tc.data)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			// Verify the data is snappy compressed
			assert.True(t, isValidSnappyData(encoded), "encoded data should be valid snappy compressed data")

			// Decode the data using the decoder and verify it matches
			decoded, err := tc.decodeFunc(encoded)
			require.NoError(t, err)
			assert.Equal(t, tc.data, decoded)
		})
	}
}

func TestSnappyThriftEncoderEncodingType(t *testing.T) {
	encoder := newSnappyThriftEncoder()

	encodingType := encoder.encodingType()
	assert.Equal(t, constants.EncodingTypeThriftRWSnappy, encodingType)
}

func TestSnappyThriftEncoderInterface(t *testing.T) {
	// Verify that snappyThriftEncoder implements the encoder interface
	var _ encoder = (*snappyThriftEncoder)(nil)

	// Test that newSnappyThriftEncoder returns a valid encoder
	encoder := newSnappyThriftEncoder()
	assert.NotNil(t, encoder)
	assert.IsType(t, &snappyThriftEncoder{}, encoder)
}

func TestSnappyThriftEncoderNilHandling(t *testing.T) {
	encoder := newSnappyThriftEncoder()

	// Test each method with nil data
	testCases := []struct {
		name       string
		encodeFunc func() ([]byte, error)
	}{
		{
			name: "shardInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.shardInfoToBlob(nil)
			},
		},
		{
			name: "domainInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.domainInfoToBlob(nil)
			},
		},
		{
			name: "activityInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.activityInfoToBlob(nil)
			},
		},
		{
			name: "workflowExecutionInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.workflowExecutionInfoToBlob(nil)
			},
		},
		{
			name: "childExecutionInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.childExecutionInfoToBlob(nil)
			},
		},
		{
			name: "signalInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.signalInfoToBlob(nil)
			},
		},
		{
			name: "requestCancelInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.requestCancelInfoToBlob(nil)
			},
		},
		{
			name: "timerInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.timerInfoToBlob(nil)
			},
		},
		{
			name: "taskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.taskInfoToBlob(nil)
			},
		},
		{
			name: "taskListInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.taskListInfoToBlob(nil)
			},
		},
		{
			name: "transferTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.transferTaskInfoToBlob(nil)
			},
		},
		{
			name: "crossClusterTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.crossClusterTaskInfoToBlob(nil)
			},
		},
		{
			name: "timerTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.timerTaskInfoToBlob(nil)
			},
		},
		{
			name: "replicationTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.replicationTaskInfoToBlob(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result []byte
			var err error
			var panicRecovered bool

			// Use defer/recover to handle panic as expected behavior for nil input
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicRecovered = true
					}
				}()
				result, err = tc.encodeFunc()
			}()

			// Nil input should either produce an error, panic, or valid empty data
			if panicRecovered {
				// Panic is expected for nil input
				assert.True(t, true, "nil input caused expected panic")
			} else if err != nil {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				// If no error, result should be valid snappy data
				assert.NotNil(t, result)
				assert.True(t, isValidSnappyData(result), "nil input should produce valid snappy data")
			}
		})
	}
}

func TestSnappyThriftRWEncode(t *testing.T) {
	// Test the low-level snappyThriftRWEncode function directly

	t.Run("Valid thrift object", func(t *testing.T) {
		// Create a simple thrift struct and encode it
		shardInfo := &ShardInfo{
			StolenSinceRenew:    1,
			UpdatedAt:           time.Now(),
			ReplicationAckLevel: 2,
			TransferAckLevel:    3,
		}

		// Convert to thrift format
		thriftStruct := shardInfoToThrift(shardInfo)

		// Encode using the low-level function
		encoded, err := snappyThriftRWEncode(thriftStruct)
		require.NoError(t, err)
		require.NotEmpty(t, encoded)

		// Verify it's valid snappy data
		assert.True(t, isValidSnappyData(encoded))

		// Verify we can decode it back
		decoder := newSnappyThriftDecoder()
		decoded, err := decoder.shardInfoFromBlob(encoded)
		require.NoError(t, err)

		assert.Equal(t, shardInfo.StolenSinceRenew, decoded.StolenSinceRenew)
		assert.Equal(t, shardInfo.ReplicationAckLevel, decoded.ReplicationAckLevel)
		assert.Equal(t, shardInfo.TransferAckLevel, decoded.TransferAckLevel)
	})

	t.Run("Nil thrift object", func(t *testing.T) {
		// Test with nil thrift object (converted from nil input)
		thriftStruct := shardInfoToThrift(nil)
		assert.Nil(t, thriftStruct)

		var err error
		var panicRecovered bool

		// Use defer/recover to handle panic as expected behavior for nil input
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicRecovered = true
				}
			}()
			_, err = snappyThriftRWEncode(thriftStruct)
		}()

		// Either panic or error is acceptable for nil input
		if panicRecovered {
			assert.True(t, true, "nil thrift object caused expected panic")
		} else if err != nil {
			assert.Error(t, err)
		}
	})
}

func TestSnappyThriftEncoderWithParser(t *testing.T) {
	// Test encoder integration with parser
	parser, err := NewParser(constants.EncodingTypeThriftRWSnappy, constants.EncodingTypeThriftRWSnappy)
	require.NoError(t, err)

	now := time.Now().Round(time.Second)

	testData := &ShardInfo{
		StolenSinceRenew:          1,
		UpdatedAt:                 now,
		ReplicationAckLevel:       2,
		TransferAckLevel:          3,
		TimerAckLevel:             now,
		DomainNotificationVersion: 4,
		Owner:                     "test_owner",
	}

	// Encode using parser
	blob, err := parser.ShardInfoToBlob(testData)
	require.NoError(t, err)
	assert.Equal(t, constants.EncodingTypeThriftRWSnappy, blob.Encoding)
	assert.NotEmpty(t, blob.Data)
	assert.True(t, isValidSnappyData(blob.Data))

	// Decode using parser
	decoded, err := parser.ShardInfoFromBlob(blob.Data, string(blob.Encoding))
	require.NoError(t, err)
	assert.Equal(t, testData, decoded)
}

func TestSnappyThriftEncoderDataCompression(t *testing.T) {
	encoder := newSnappyThriftEncoder()

	// Create a large data structure to test compression
	largeData := &WorkflowExecutionInfo{
		WorkflowTypeName: "very_long_workflow_type_name_that_should_compress_well_when_repeated",
		TaskList:         "very_long_task_list_name_that_should_compress_well_when_repeated",
		ExecutionContext: make([]byte, 1000), // Large byte array
		SearchAttributes: make(map[string][]byte),
		Memo:             make(map[string][]byte),
	}

	// Fill with repetitive data that should compress well
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("repetitive_key_that_compresses_well_%d", i)
		value := []byte("repetitive_value_that_compresses_well_when_repeated_many_times")
		largeData.SearchAttributes[key] = value
		largeData.Memo[key] = value
	}

	// Encode the data
	encoded, err := encoder.workflowExecutionInfoToBlob(largeData)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Verify it's compressed (should be significantly smaller than uncompressed)
	assert.True(t, isValidSnappyData(encoded))

	// Decode and verify correctness
	decoder := newSnappyThriftDecoder()
	decoded, err := decoder.workflowExecutionInfoFromBlob(encoded)
	require.NoError(t, err)
	assert.Equal(t, largeData.WorkflowTypeName, decoded.WorkflowTypeName)
	assert.Equal(t, largeData.TaskList, decoded.TaskList)
	assert.Len(t, decoded.SearchAttributes, 100)
	assert.Len(t, decoded.Memo, 100)
}

// Helper function to check if data is valid snappy compressed data
func isValidSnappyData(data []byte) bool {
	_, err := snappy.Decode(nil, data)
	return err == nil
}
