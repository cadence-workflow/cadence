// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestParserRoundTrip(t *testing.T) {
	dc := &persistence.DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	}
	thriftParser, err := NewParser(dc)
	assert.NoError(t, err)
	now := time.Now().Round(time.Second)

	for _, testCase := range []any{
		&ShardInfo{
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
		&DomainInfo{
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
		&HistoryTreeInfo{
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
		&WorkflowExecutionInfo{
			ParentDomainID:                     MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			ParentWorkflowID:                   "test_parent_workflow_id",
			ParentRunID:                        MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			InitiatedID:                        1,
			CompletionEventBatchID:             common.Int64Ptr(1),
			CompletionEvent:                    []byte{1, 2, 3},
			CompletionEventEncoding:            "",
			TaskList:                           "test_task_list",
			WorkflowTypeName:                   "test_workflow_type_name",
			WorkflowTimeout:                    48 * time.Hour,
			ExecutionContext:                   []byte{1, 2, 3},
			State:                              2,
			CloseStatus:                        1,
			StartVersion:                       1,
			LastWriteEventID:                   common.Int64Ptr(2),
			LastEventTaskID:                    3,
			LastFirstEventID:                   5,
			LastProcessedEvent:                 4,
			StartTimestamp:                     now,
			LastUpdatedTimestamp:               now,
			DecisionVersion:                    1,
			DecisionScheduleID:                 1,
			DecisionStartedID:                  1,
			DecisionRequestID:                  "test_decision_request_id",
			DecisionTimeout:                    48 * time.Hour,
			DecisionAttempt:                    1,
			DecisionStartedTimestamp:           now,
			DecisionScheduledTimestamp:         now,
			DecisionOriginalScheduledTimestamp: now,
			CancelRequested:                    true,
			CancelRequestID:                    "test_cancel_request_id",
			StickyTaskList:                     "test_sticky_task_list",
			StickyScheduleToStartTimeout:       48 * time.Hour,
			ClientLibraryVersion:               "test_client_library_version",
			ClientFeatureVersion:               "test_client_feature_version",
			ClientImpl:                         "test_client_impl",
			SignalCount:                        1,
			HistorySize:                        100,
			AutoResetPoints:                    []byte{1, 2, 3},
			AutoResetPointsEncoding:            "",
			Memo:                               map[string][]byte{"test_memo_key": {1, 2, 3}},
			SearchAttributes:                   map[string][]byte{"test_search_attr_key": {1, 2, 3}},
		},
		&ActivityInfo{
			Version:                  1,
			ScheduledEventBatchID:    2,
			ScheduledEvent:           []byte{1, 2, 3},
			ScheduledEventEncoding:   "scheduled_event_encoding",
			ScheduledTimestamp:       now,
			StartedID:                1,
			StartedEvent:             []byte{1, 2, 3},
			StartedEventEncoding:     "started_event_encoding",
			StartedTimestamp:         now,
			ActivityID:               "test_activity_id",
			RequestID:                "test_request_id",
			ScheduleToStartTimeout:   48 * time.Hour,
			ScheduleToCloseTimeout:   48 * time.Hour,
			StartToCloseTimeout:      48 * time.Hour,
			HeartbeatTimeout:         48 * time.Hour,
			CancelRequested:          true,
			CancelRequestID:          3,
			TimerTaskStatus:          1,
			Attempt:                  1,
			TaskList:                 "test_task_list",
			StartedIdentity:          "test_started_identity",
			HasRetryPolicy:           true,
			RetryInitialInterval:     time.Hour,
			RetryBackoffCoefficient:  1.1,
			RetryMaximumInterval:     time.Hour,
			RetryMaximumAttempts:     1,
			RetryExpirationTimestamp: now.Add(time.Hour),
			RetryNonRetryableErrors:  []string{"test_retry_non_retryable_error"},
			RetryLastWorkerIdentity:  "test_retry_last_worker_identity",
		},
		&ChildExecutionInfo{
			Version:                1,
			InitiatedEventBatchID:  2,
			StartedID:              3,
			InitiatedEvent:         []byte{1, 2, 3},
			InitiatedEventEncoding: "initiated_event_encoding",
			StartedWorkflowID:      "test_started_workflow_id",
			StartedRunID:           MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			StartedEvent:           []byte{1, 2, 3},
			StartedEventEncoding:   "started_event_encoding",
			CreateRequestID:        "test_create_request_id",
			DomainID:               "test_domain_id",
			WorkflowTypeName:       "test_workflow_type_name",
			ParentClosePolicy:      1,
		},
		&SignalInfo{
			Version:               1,
			InitiatedEventBatchID: 2,
			RequestID:             "test_request_id",
			Name:                  "test_name",
			Input:                 []byte{1, 2, 3},
			Control:               []byte{1, 2, 3},
		},
		&RequestCancelInfo{
			Version:               1,
			InitiatedEventBatchID: 2,
			CancelRequestID:       "test_cancel_request_id",
		},
		&TimerInfo{
			Version:         1,
			StartedID:       2,
			ExpiryTimestamp: zeroUnix,
			TaskID:          3,
		},
		&TaskInfo{
			WorkflowID:       "test_workflow_id",
			RunID:            MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			ScheduleID:       1,
			ExpiryTimestamp:  now,
			CreatedTimestamp: now,
			PartitionConfig:  map[string]string{"test_partition_key": "test_partition_value"},
		},
		&TaskListInfo{
			Kind:            1,
			AckLevel:        2,
			ExpiryTimestamp: now,
			LastUpdated:     now,
		},
		&TransferTaskInfo{
			DomainID:                MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			WorkflowID:              "test_workflow_id",
			RunID:                   MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			TaskType:                1,
			TargetDomainID:          MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			TargetDomainIDs:         []UUID{MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"), MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430a8")},
			TargetWorkflowID:        "test_target_workflow_id",
			TargetRunID:             MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			TaskList:                "test_task_list",
			TargetChildWorkflowOnly: true,
			ScheduleID:              1,
			Version:                 2,
			VisibilityTimestamp:     now,
		},
		&TimerTaskInfo{
			DomainID:        MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			WorkflowID:      "test_workflow_id",
			RunID:           MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			TaskType:        1,
			TimeoutType:     common.Int16Ptr(1),
			Version:         2,
			ScheduleAttempt: 3,
			EventID:         4,
		},
		&ReplicationTaskInfo{
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
			BranchToken:             []byte{1, 2, 3},
			NewRunBranchToken:       []byte{1, 2, 3},
			CreationTimestamp:       now,
		},
	} {
		t.Run(reflect.TypeOf(testCase).String(), func(t *testing.T) {
			blob := parse(t, thriftParser, testCase)
			result := unparse(t, thriftParser, blob, testCase)
			assert.Equal(t, testCase, result)
		})
	}
}

func parse(t *testing.T, parser Parser, data any) persistence.DataBlob {
	var (
		blob persistence.DataBlob
		err  error
	)
	switch v := data.(type) {
	case *ShardInfo:
		blob, err = parser.ShardInfoToBlob(v)
	case *DomainInfo:
		blob, err = parser.DomainInfoToBlob(v)
	case *HistoryTreeInfo:
		blob, err = parser.HistoryTreeInfoToBlob(v)
	case *WorkflowExecutionInfo:
		blob, err = parser.WorkflowExecutionInfoToBlob(v)
	case *ActivityInfo:
		blob, err = parser.ActivityInfoToBlob(v)
	case *ChildExecutionInfo:
		blob, err = parser.ChildExecutionInfoToBlob(v)
	case *SignalInfo:
		blob, err = parser.SignalInfoToBlob(v)
	case *RequestCancelInfo:
		blob, err = parser.RequestCancelInfoToBlob(v)
	case *TimerInfo:
		blob, err = parser.TimerInfoToBlob(v)
	case *TaskInfo:
		blob, err = parser.TaskInfoToBlob(v)
	case *TaskListInfo:
		blob, err = parser.TaskListInfoToBlob(v)
	case *TransferTaskInfo:
		blob, err = parser.TransferTaskInfoToBlob(v)
	case *TimerTaskInfo:
		blob, err = parser.TimerTaskInfoToBlob(v)
	case *ReplicationTaskInfo:
		blob, err = parser.ReplicationTaskInfoToBlob(v)
	default:
		err = fmt.Errorf("unknown type %T", v)
	}
	assert.NoError(t, err)
	return blob
}

func unparse(t *testing.T, parser Parser, blob persistence.DataBlob, result any) any {
	var (
		data any
		err  error
	)
	switch v := result.(type) {
	case *ShardInfo:
		data, err = parser.ShardInfoFromBlob(blob.Data, string(blob.Encoding))
	case *DomainInfo:
		data, err = parser.DomainInfoFromBlob(blob.Data, string(blob.Encoding))
	case *HistoryTreeInfo:
		data, err = parser.HistoryTreeInfoFromBlob(blob.Data, string(blob.Encoding))
	case *WorkflowExecutionInfo:
		data, err = parser.WorkflowExecutionInfoFromBlob(blob.Data, string(blob.Encoding))
	case *ActivityInfo:
		data, err = parser.ActivityInfoFromBlob(blob.Data, string(blob.Encoding))
	case *ChildExecutionInfo:
		data, err = parser.ChildExecutionInfoFromBlob(blob.Data, string(blob.Encoding))
	case *SignalInfo:
		data, err = parser.SignalInfoFromBlob(blob.Data, string(blob.Encoding))
	case *RequestCancelInfo:
		data, err = parser.RequestCancelInfoFromBlob(blob.Data, string(blob.Encoding))
	case *TimerInfo:
		data, err = parser.TimerInfoFromBlob(blob.Data, string(blob.Encoding))
	case *TaskInfo:
		data, err = parser.TaskInfoFromBlob(blob.Data, string(blob.Encoding))
	case *TaskListInfo:
		data, err = parser.TaskListInfoFromBlob(blob.Data, string(blob.Encoding))
	case *TransferTaskInfo:
		data, err = parser.TransferTaskInfoFromBlob(blob.Data, string(blob.Encoding))
	case *TimerTaskInfo:
		data, err = parser.TimerTaskInfoFromBlob(blob.Data, string(blob.Encoding))
	case *ReplicationTaskInfo:
		data, err = parser.ReplicationTaskInfoFromBlob(blob.Data, string(blob.Encoding))
	default:
		err = fmt.Errorf("unknown type %T", v)
	}
	assert.NoError(t, err)
	return data
}

func TestParser_WorkflowExecution_with_cron(t *testing.T) {
	info := &WorkflowExecutionInfo{
		CronSchedule: "@every 1m",
		IsCron:       true,
	}
	dc := &persistence.DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	}
	parser, err := NewParser(dc)
	require.NoError(t, err)
	blob, err := parser.WorkflowExecutionInfoToBlob(info)
	require.NoError(t, err)
	result, err := parser.WorkflowExecutionInfoFromBlob(blob.Data, string(blob.Encoding))
	require.NoError(t, err)
	assert.Equal(t, info, result)
}
