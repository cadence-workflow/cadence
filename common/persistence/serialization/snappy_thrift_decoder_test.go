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
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

func TestSnappyThriftDecoderRoundTrip(t *testing.T) {
	parser, err := NewParser(constants.EncodingTypeThriftRWSnappy, constants.EncodingTypeThriftRWSnappy)
	require.NoError(t, err)

	now := time.Now().Round(time.Second)

	testCases := []struct {
		name string
		data interface{}
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
		},
		{
			name: "RequestCancelInfo",
			data: &RequestCancelInfo{
				Version:               1,
				InitiatedEventBatchID: 2,
				CancelRequestID:       "test_cancel_request_id",
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
		},
		{
			name: "TaskListInfo",
			data: &TaskListInfo{
				Kind:            1,
				AckLevel:        2,
				ExpiryTimestamp: now,
				LastUpdated:     now,
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
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode the data using the parser
			blob := encodeWithParser(t, parser, tc.data)

			// Decode the data using the parser and verify it matches
			decoded := decodeWithParser(t, parser, blob, tc.data)
			assert.Equal(t, tc.data, decoded)
		})
	}
}

func TestSnappyThriftDecoderErrorHandling(t *testing.T) {
	decoder := newSnappyThriftDecoder()

	testCases := []struct {
		name        string
		data        []byte
		decodeFunc  func([]byte) (interface{}, error)
		expectError bool
	}{
		{
			name: "Invalid snappy data for ShardInfo",
			data: []byte("invalid snappy data"),
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.shardInfoFromBlob(data)
			},
			expectError: true,
		},
		{
			name: "Empty data for DomainInfo",
			data: []byte{},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.domainInfoFromBlob(data)
			},
			expectError: true,
		},
		{
			name: "Corrupted snappy data for ActivityInfo",
			data: []byte{0xff, 0xff, 0xff, 0xff},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.activityInfoFromBlob(data)
			},
			expectError: true,
		},
		{
			name: "Valid snappy but invalid thrift for WorkflowExecutionInfo",
			data: func() []byte {
				// Create valid snappy compressed data but with invalid thrift content
				invalidThrift := []byte("not thrift data")
				compressed := snappy.Encode(nil, invalidThrift)
				return compressed
			}(),
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.workflowExecutionInfoFromBlob(data)
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.decodeFunc(tc.data)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestSnappyThriftRWDecode(t *testing.T) {
	// Test the low-level snappyThriftRWDecode function directly

	t.Run("Valid data", func(t *testing.T) {
		// Create a simple thrift struct and encode it
		encoder := newSnappyThriftEncoder()
		shardInfo := &ShardInfo{
			StolenSinceRenew:    1,
			UpdatedAt:           time.Now(),
			ReplicationAckLevel: 2,
			TransferAckLevel:    3,
		}

		encoded, err := encoder.shardInfoToBlob(shardInfo)
		require.NoError(t, err)

		// Now decode it back
		decoder := newSnappyThriftDecoder()
		decoded, err := decoder.shardInfoFromBlob(encoded)
		require.NoError(t, err)

		assert.Equal(t, shardInfo.StolenSinceRenew, decoded.StolenSinceRenew)
		assert.Equal(t, shardInfo.ReplicationAckLevel, decoded.ReplicationAckLevel)
		assert.Equal(t, shardInfo.TransferAckLevel, decoded.TransferAckLevel)
	})

	t.Run("Invalid snappy compression", func(t *testing.T) {
		decoder := newSnappyThriftDecoder()

		// Test with invalid snappy data
		invalidData := []byte("this is not snappy compressed data")
		_, err := decoder.shardInfoFromBlob(invalidData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "snappy")
	})

	t.Run("Valid snappy but invalid thrift", func(t *testing.T) {
		decoder := newSnappyThriftDecoder()

		// Create valid snappy data but with invalid thrift content
		invalidThrift := []byte("not a valid thrift message")
		compressed := snappy.Encode(nil, invalidThrift)

		_, err := decoder.shardInfoFromBlob(compressed)
		assert.Error(t, err)
	})
}

func TestSnappyThriftDecoderInterface(t *testing.T) {
	// Verify that snappyThriftDecoder implements the decoder interface
	var _ decoder = (*snappyThriftDecoder)(nil)

	// Test that newSnappyThriftDecoder returns a valid decoder
	decoder := newSnappyThriftDecoder()
	assert.NotNil(t, decoder)
	assert.IsType(t, &snappyThriftDecoder{}, decoder)
}

func TestSnappyThriftDecoderNilHandling(t *testing.T) {
	decoder := newSnappyThriftDecoder()

	// Test each method with nil data
	testCases := []struct {
		name       string
		decodeFunc func() (interface{}, error)
	}{
		{
			name: "shardInfoFromBlob with nil",
			decodeFunc: func() (interface{}, error) {
				return decoder.shardInfoFromBlob(nil)
			},
		},
		{
			name: "domainInfoFromBlob with nil",
			decodeFunc: func() (interface{}, error) {
				return decoder.domainInfoFromBlob(nil)
			},
		},
		{
			name: "activityInfoFromBlob with nil",
			decodeFunc: func() (interface{}, error) {
				return decoder.activityInfoFromBlob(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.decodeFunc()
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

// Helper functions for encoding and decoding with parser
func encodeWithParser(t *testing.T, parser Parser, data interface{}) []byte {
	var blob []byte
	var err error

	switch v := data.(type) {
	case *ShardInfo:
		db, e := parser.ShardInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *DomainInfo:
		db, e := parser.DomainInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *HistoryTreeInfo:
		db, e := parser.HistoryTreeInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *WorkflowExecutionInfo:
		db, e := parser.WorkflowExecutionInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *ActivityInfo:
		db, e := parser.ActivityInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *ChildExecutionInfo:
		db, e := parser.ChildExecutionInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *SignalInfo:
		db, e := parser.SignalInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *RequestCancelInfo:
		db, e := parser.RequestCancelInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TimerInfo:
		db, e := parser.TimerInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TaskInfo:
		db, e := parser.TaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TaskListInfo:
		db, e := parser.TaskListInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TransferTaskInfo:
		db, e := parser.TransferTaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TimerTaskInfo:
		db, e := parser.TimerTaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *ReplicationTaskInfo:
		db, e := parser.ReplicationTaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	default:
		t.Fatalf("Unknown type %T", v)
	}

	require.NoError(t, err)
	require.NotEmpty(t, blob)
	return blob
}

func decodeWithParser(t *testing.T, parser Parser, blob []byte, data interface{}) interface{} {
	var result interface{}
	var err error

	encoding := string(constants.EncodingTypeThriftRWSnappy)

	switch data.(type) {
	case *ShardInfo:
		result, err = parser.ShardInfoFromBlob(blob, encoding)
	case *DomainInfo:
		result, err = parser.DomainInfoFromBlob(blob, encoding)
	case *HistoryTreeInfo:
		result, err = parser.HistoryTreeInfoFromBlob(blob, encoding)
	case *WorkflowExecutionInfo:
		result, err = parser.WorkflowExecutionInfoFromBlob(blob, encoding)
	case *ActivityInfo:
		result, err = parser.ActivityInfoFromBlob(blob, encoding)
	case *ChildExecutionInfo:
		result, err = parser.ChildExecutionInfoFromBlob(blob, encoding)
	case *SignalInfo:
		result, err = parser.SignalInfoFromBlob(blob, encoding)
	case *RequestCancelInfo:
		result, err = parser.RequestCancelInfoFromBlob(blob, encoding)
	case *TimerInfo:
		result, err = parser.TimerInfoFromBlob(blob, encoding)
	case *TaskInfo:
		result, err = parser.TaskInfoFromBlob(blob, encoding)
	case *TaskListInfo:
		result, err = parser.TaskListInfoFromBlob(blob, encoding)
	case *TransferTaskInfo:
		result, err = parser.TransferTaskInfoFromBlob(blob, encoding)
	case *TimerTaskInfo:
		result, err = parser.TimerTaskInfoFromBlob(blob, encoding)
	case *ReplicationTaskInfo:
		result, err = parser.ReplicationTaskInfoFromBlob(blob, encoding)
	default:
		t.Fatalf("Unknown type %T", data)
	}

	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}
