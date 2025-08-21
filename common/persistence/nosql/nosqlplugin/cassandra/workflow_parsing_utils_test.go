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

package cassandra

import (
	"testing"
	"time"

	cql "github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

type mockUUID struct {
	uuid string
}

func (m mockUUID) String() string {
	return m.uuid
}

func newMockUUID(s string) mockUUID {
	return mockUUID{s}
}

func Test_parseWorkflowExecutionInfo(t *testing.T) {

	completionEventData := []byte("completion event data")
	autoResetPointsData := []byte("auto reset points data")
	activeClusterSelectionPolicyData := []byte("active cluster selection policy data")
	searchAttributes := map[string][]byte{"AttributeKey": []byte("AttributeValue")}
	memo := map[string][]byte{"MemoKey": []byte("MemoValue")}
	partitionConfig := map[string]string{"PartitionKey": "PartitionValue"}
	timeNow := time.Now()

	tests := []struct {
		args map[string]interface{}
		want *persistence.InternalWorkflowExecutionInfo
	}{
		{
			args: map[string]interface{}{
				"domain_id":                                newMockUUID("domain_id"),
				"workflow_id":                              "workflow_id",
				"run_id":                                   newMockUUID("run_id"),
				"parent_workflow_id":                       "parent_workflow_id",
				"initiated_id":                             int64(1),
				"completion_event_batch_id":                int64(2),
				"task_list":                                "task_list",
				"task_list_kind":                           2,
				"workflow_type_name":                       "workflow_type_name",
				"workflow_timeout":                         10,
				"decision_task_timeout":                    5,
				"execution_context":                        []byte("execution context"),
				"state":                                    1,
				"close_status":                             2,
				"last_first_event_id":                      int64(3),
				"last_event_task_id":                       int64(4),
				"next_event_id":                            int64(5),
				"last_processed_event":                     int64(6),
				"start_time":                               timeNow,
				"last_updated_time":                        timeNow,
				"create_request_id":                        newMockUUID("create_request_id"),
				"signal_count":                             7,
				"history_size":                             int64(8),
				"decision_version":                         int64(9),
				"decision_schedule_id":                     int64(10),
				"decision_started_id":                      int64(11),
				"decision_request_id":                      "decision_request_id",
				"decision_timeout":                         8,
				"decision_timestamp":                       int64(200),
				"decision_scheduled_timestamp":             int64(201),
				"decision_original_scheduled_timestamp":    int64(202),
				"decision_attempt":                         int64(203),
				"cancel_requested":                         true,
				"cancel_request_id":                        "cancel_request_id",
				"sticky_task_list":                         "sticky_task_list",
				"sticky_schedule_to_start_timeout":         9,
				"client_library_version":                   "client_lib_version",
				"client_feature_version":                   "client_feature_version",
				"client_impl":                              "client_impl",
				"attempt":                                  12,
				"has_retry_policy":                         true,
				"init_interval":                            10,
				"backoff_coefficient":                      1.5,
				"max_interval":                             20,
				"max_attempts":                             13,
				"expiration_time":                          timeNow,
				"non_retriable_errors":                     []string{"error1", "error2"},
				"branch_token":                             []byte("branch token"),
				"cron_schedule":                            "cron_schedule",
				"expiration_seconds":                       14,
				"search_attributes":                        searchAttributes,
				"memo":                                     memo,
				"partition_config":                         partitionConfig,
				"completion_event":                         completionEventData,
				"completion_event_data_encoding":           "Proto3",
				"auto_reset_points":                        autoResetPointsData,
				"auto_reset_points_encoding":               "Proto3",
				"active_cluster_selection_policy":          activeClusterSelectionPolicyData,
				"active_cluster_selection_policy_encoding": "Proto3",
			},
			want: &persistence.InternalWorkflowExecutionInfo{
				DomainID:                           "domain_id",
				WorkflowID:                         "workflow_id",
				RunID:                              "run_id",
				ParentWorkflowID:                   "parent_workflow_id",
				InitiatedID:                        int64(1),
				CompletionEventBatchID:             int64(2),
				TaskList:                           "task_list",
				TaskListKind:                       types.TaskListKindEphemeral,
				WorkflowTypeName:                   "workflow_type_name",
				WorkflowTimeout:                    common.SecondsToDuration(int64(10)),
				DecisionStartToCloseTimeout:        common.SecondsToDuration(int64(5)),
				ExecutionContext:                   []byte("execution context"),
				State:                              1,
				CloseStatus:                        2,
				LastFirstEventID:                   int64(3),
				LastEventTaskID:                    int64(4),
				NextEventID:                        int64(5),
				LastProcessedEvent:                 int64(6),
				StartTimestamp:                     timeNow,
				LastUpdatedTimestamp:               timeNow,
				CreateRequestID:                    "create_request_id",
				SignalCount:                        int32(7),
				HistorySize:                        int64(8),
				DecisionVersion:                    int64(9),
				DecisionScheduleID:                 int64(10),
				DecisionStartedID:                  int64(11),
				DecisionRequestID:                  "decision_request_id",
				DecisionTimeout:                    common.SecondsToDuration(int64(8)),
				DecisionStartedTimestamp:           time.Unix(0, int64(200)),
				DecisionScheduledTimestamp:         time.Unix(0, int64(201)),
				DecisionOriginalScheduledTimestamp: time.Unix(0, int64(202)),
				DecisionAttempt:                    int64(203),
				CancelRequested:                    true,
				CancelRequestID:                    "cancel_request_id",
				StickyTaskList:                     "sticky_task_list",
				StickyScheduleToStartTimeout:       common.SecondsToDuration(int64(9)),
				ClientLibraryVersion:               "client_lib_version",
				ClientFeatureVersion:               "client_feature_version",
				ClientImpl:                         "client_impl",
				Attempt:                            int32(12),
				HasRetryPolicy:                     true,
				InitialInterval:                    common.SecondsToDuration(int64(10)),
				BackoffCoefficient:                 1.5,
				MaximumInterval:                    common.SecondsToDuration(int64(20)),
				MaximumAttempts:                    int32(13),
				ExpirationTime:                     timeNow,
				NonRetriableErrors:                 []string{"error1", "error2"},
				Memo:                               memo,
				PartitionConfig:                    partitionConfig,
				ActiveClusterSelectionPolicy:       persistence.NewDataBlob(activeClusterSelectionPolicyData, "Proto3"),
			},
		},
		{
			args: map[string]interface{}{
				"first_run_id":     newMockUUID("first_run_id"),
				"parent_domain_id": newMockUUID("parent_domain_id"),
				"parent_run_id":    newMockUUID("parent_run_id"),
			},
			want: &persistence.InternalWorkflowExecutionInfo{
				FirstExecutionRunID: "first_run_id",
				ParentDomainID:      "parent_domain_id",
				ParentRunID:         "parent_run_id",
			},
		},
		{
			args: map[string]interface{}{
				"first_run_id":     newMockUUID(emptyRunID),
				"parent_domain_id": newMockUUID(emptyDomainID),
				"parent_run_id":    newMockUUID(emptyRunID),
			},
			want: &persistence.InternalWorkflowExecutionInfo{},
		},
		{
			args: map[string]interface{}{
				"first_run_id": newMockUUID(cql.UUID{}.String()),
			},
			want: &persistence.InternalWorkflowExecutionInfo{
				FirstExecutionRunID: "",
			},
		},
	}
	for _, tt := range tests {
		result := parseWorkflowExecutionInfo(tt.args)
		assert.Equal(t, result.FirstExecutionRunID, tt.want.FirstExecutionRunID)
		assert.Equal(t, result.DomainID, tt.want.DomainID)
		assert.Equal(t, result.WorkflowID, tt.want.WorkflowID)
		assert.Equal(t, result.RunID, tt.want.RunID)
		assert.Equal(t, result.ParentWorkflowID, tt.want.ParentWorkflowID)
		assert.Equal(t, result.InitiatedID, tt.want.InitiatedID)
		assert.Equal(t, result.CompletionEventBatchID, tt.want.CompletionEventBatchID)
		assert.Equal(t, result.TaskList, tt.want.TaskList)
		assert.Equal(t, result.WorkflowTypeName, tt.want.WorkflowTypeName)
		assert.Equal(t, result.WorkflowTimeout, tt.want.WorkflowTimeout)
		assert.Equal(t, result.DecisionStartToCloseTimeout, tt.want.DecisionStartToCloseTimeout)
		assert.Equal(t, result.ExecutionContext, tt.want.ExecutionContext)
		assert.Equal(t, result.State, tt.want.State)
		assert.Equal(t, result.CloseStatus, tt.want.CloseStatus)
		assert.Equal(t, result.LastFirstEventID, tt.want.LastFirstEventID)
		assert.Equal(t, result.LastEventTaskID, tt.want.LastEventTaskID)
		assert.Equal(t, result.NextEventID, tt.want.NextEventID)
		assert.Equal(t, result.LastProcessedEvent, tt.want.LastProcessedEvent)
		assert.Equal(t, result.StartTimestamp, tt.want.StartTimestamp)
		assert.Equal(t, result.LastUpdatedTimestamp, tt.want.LastUpdatedTimestamp)
		assert.Equal(t, result.CreateRequestID, tt.want.CreateRequestID)
		assert.Equal(t, result.SignalCount, tt.want.SignalCount)
		assert.Equal(t, result.HistorySize, tt.want.HistorySize)
		assert.Equal(t, result.DecisionVersion, tt.want.DecisionVersion)
		assert.Equal(t, result.DecisionScheduleID, tt.want.DecisionScheduleID)
		assert.Equal(t, result.DecisionStartedID, tt.want.DecisionStartedID)
		assert.Equal(t, result.DecisionRequestID, tt.want.DecisionRequestID)
		assert.Equal(t, result.DecisionTimeout, tt.want.DecisionTimeout)
		assert.Equal(t, result.CancelRequested, tt.want.CancelRequested)
		assert.Equal(t, result.DecisionStartedTimestamp, tt.want.DecisionStartedTimestamp)
		assert.Equal(t, result.DecisionScheduledTimestamp, tt.want.DecisionScheduledTimestamp)
		assert.Equal(t, result.DecisionOriginalScheduledTimestamp, tt.want.DecisionOriginalScheduledTimestamp)
		assert.Equal(t, result.DecisionAttempt, tt.want.DecisionAttempt)
		assert.Equal(t, result.ParentDomainID, tt.want.ParentDomainID)
		assert.Equal(t, result.ActiveClusterSelectionPolicy, tt.want.ActiveClusterSelectionPolicy)
	}
}

func Test_parseReplicationState(t *testing.T) {
	tests := []struct {
		args map[string]interface{}
		want *persistence.ReplicationState
	}{
		{
			args: map[string]interface{}{
				"current_version":     int64(1),
				"start_version":       int64(2),
				"last_write_version":  int64(3),
				"last_write_event_id": int64(4),
				"last_replication_info": map[string]map[string]interface{}{
					"map1": {
						"version":       int64(5),
						"last_event_id": int64(6),
					},
					"map2": {
						"version":       int64(7),
						"last_event_id": int64(8),
					},
				},
			},
			want: &persistence.ReplicationState{
				CurrentVersion:   int64(1),
				StartVersion:     int64(2),
				LastWriteVersion: int64(3),
				LastWriteEventID: int64(4),
				LastReplicationInfo: map[string]*persistence.ReplicationInfo{
					"map1": {
						Version:     int64(5),
						LastEventID: int64(6),
					},
					"map2": {
						Version:     int64(7),
						LastEventID: int64(8),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		result := parseReplicationState(tt.args)
		assert.Equal(t, result.CurrentVersion, tt.want.CurrentVersion)
		assert.Equal(t, result.StartVersion, tt.want.StartVersion)
		assert.Equal(t, result.LastWriteVersion, tt.want.LastWriteVersion)
		assert.Equal(t, result.LastWriteEventID, tt.want.LastWriteEventID)
		assert.Equal(t, result.LastReplicationInfo, tt.want.LastReplicationInfo)
	}
}

func Test_parseActivityInfo(t *testing.T) {
	timeNow := time.Now()
	testInput := map[string]interface{}{
		"version":                   int64(1),
		"schedule_id":               int64(2),
		"scheduled_event_batch_id":  int64(3),
		"scheduled_event":           []byte("scheduled_event"),
		"scheduled_time":            timeNow,
		"started_id":                int64(4),
		"started_event":             []byte("started_event"),
		"started_time":              timeNow,
		"activity_id":               "activity_id",
		"request_id":                "request_id",
		"details":                   []byte("details"),
		"schedule_to_start_timeout": 5,
		"schedule_to_close_timeout": 6,
		"start_to_close_timeout":    7,
		"heart_beat_timeout":        8,
		"cancel_requested":          true,
		"cancel_request_id":         int64(9),
		"last_hb_updated_time":      timeNow,
		"timer_task_status":         9,
		"attempt":                   10,
		"task_list":                 "task_list",
		"task_list_kind":            1,
		"started_identity":          "started_identity",
		"has_retry_policy":          true,
		"init_interval":             11,
		"backoff_coefficient":       1.5,
		"max_interval":              12,
		"max_attempts":              13,
		"expiration_time":           timeNow,
		"non_retriable_errors":      []string{"error1", "error2"},
		"last_failure_reason":       "last_failure_reason",
		"last_worker_identity":      "last_worker_identity",
		"last_failure_details":      []byte("last_failure_details"),
		"event_data_encoding":       "Proto3",
	}

	expected := &persistence.InternalActivityInfo{
		Version:                  int64(1),
		ScheduleID:               int64(2),
		ScheduledEventBatchID:    int64(3),
		ScheduledEvent:           persistence.NewDataBlob([]byte("scheduled_event"), "Proto3"),
		ScheduledTime:            timeNow,
		StartedID:                int64(4),
		StartedEvent:             persistence.NewDataBlob([]byte("started_event"), "Proto3"),
		StartedTime:              timeNow,
		ActivityID:               "activity_id",
		RequestID:                "request_id",
		Details:                  []byte("details"),
		ScheduleToStartTimeout:   common.SecondsToDuration(int64(5)),
		ScheduleToCloseTimeout:   common.SecondsToDuration(int64(6)),
		StartToCloseTimeout:      common.SecondsToDuration(int64(7)),
		HeartbeatTimeout:         common.SecondsToDuration(int64(8)),
		CancelRequested:          true,
		CancelRequestID:          int64(9),
		LastHeartBeatUpdatedTime: timeNow,
		TimerTaskStatus:          int32(9),
		Attempt:                  int32(10),
		TaskList:                 "task_list",
		TaskListKind:             types.TaskListKindSticky,
		StartedIdentity:          "started_identity",
		HasRetryPolicy:           true,
		InitialInterval:          common.SecondsToDuration(int64(11)),
		BackoffCoefficient:       1.5,
		MaximumInterval:          common.SecondsToDuration(int64(12)),
		MaximumAttempts:          int32(13),
		ExpirationTime:           timeNow,
		NonRetriableErrors:       []string{"error1", "error2"},
		LastFailureReason:        "last_failure_reason",
		LastWorkerIdentity:       "last_worker_identity",
		LastFailureDetails:       []byte("last_failure_details"),
		DomainID:                 "domain_id",
	}

	assert.Equal(t, expected, parseActivityInfo("domain_id", testInput))
}

func Test_parseTimerInfo(t *testing.T) {
	timeNow := time.Now()
	testInput := map[string]interface{}{
		"version":     int64(1),
		"timer_id":    "timer_id",
		"started_id":  int64(2),
		"expiry_time": timeNow,
		"task_id":     int64(3),
	}
	expected := &persistence.TimerInfo{
		Version:    int64(1),
		TimerID:    "timer_id",
		StartedID:  int64(2),
		ExpiryTime: timeNow,
		TaskStatus: int64(3),
	}
	assert.Equal(t, expected, parseTimerInfo(testInput))
}

func Test_parseChildExecutionInfo(t *testing.T) {
	startedRunID := newMockUUID("started_run_id")
	createRequestID := newMockUUID("create_request_id")
	domainID := newMockUUID("domain_id")
	testInput := map[string]interface{}{
		"version":                  int64(1),
		"initiated_id":             int64(2),
		"initiated_event_batch_id": int64(3),
		"initiated_event":          []byte("initiated_event"),
		"started_id":               int64(4),
		"started_workflow_id":      "started_workflow_id",
		"started_run_id":           startedRunID,
		"started_event":            []byte("started_event"),
		"create_request_id":        createRequestID,
		"event_data_encoding":      "Proto3",
		"domain_id":                domainID,
		"workflow_type_name":       "workflow_type_name",
		"parent_close_policy":      1,
	}
	expected := &persistence.InternalChildExecutionInfo{
		Version:               int64(1),
		InitiatedID:           int64(2),
		InitiatedEventBatchID: int64(3),
		InitiatedEvent:        persistence.NewDataBlob([]byte("initiated_event"), "Proto3"),
		StartedID:             int64(4),
		StartedWorkflowID:     "started_workflow_id",
		StartedRunID:          startedRunID.String(),
		StartedEvent:          persistence.NewDataBlob([]byte("started_event"), "Proto3"),
		CreateRequestID:       createRequestID.String(),
		DomainID:              domainID.String(),
		WorkflowTypeName:      "workflow_type_name",
		ParentClosePolicy:     1,
	}
	assert.Equal(t, expected, parseChildExecutionInfo(testInput))

	// edge case
	testInput = map[string]interface{}{
		"domain_id":   newMockUUID(_emptyUUID.String()),
		"domain_name": "domain_name",
	}
	assert.Equal(t, "domain_name", parseChildExecutionInfo(testInput).DomainNameDEPRECATED)
	assert.Equal(t, "", parseChildExecutionInfo(testInput).DomainID)
}

func Test_parseRequestCancelInfo(t *testing.T) {
	testInput := map[string]interface{}{
		"version":                  int64(1),
		"initiated_id":             int64(2),
		"initiated_event_batch_id": int64(3),
		"cancel_request_id":        "cancel_request_id",
	}
	expected := &persistence.RequestCancelInfo{
		Version:               int64(1),
		InitiatedID:           int64(2),
		InitiatedEventBatchID: int64(3),
		CancelRequestID:       "cancel_request_id",
	}
	assert.Equal(t, expected, parseRequestCancelInfo(testInput))
}

func Test_parseSignalInfo(t *testing.T) {
	testInput := map[string]interface{}{
		"version":                  int64(1),
		"initiated_id":             int64(2),
		"initiated_event_batch_id": int64(3),
		"signal_request_id":        newMockUUID("signal_request_id"),
		"signal_name":              "signal_name",
		"input":                    []byte("input"),
		"control":                  []byte("control"),
	}
	expected := &persistence.SignalInfo{
		Version:               int64(1),
		InitiatedID:           int64(2),
		InitiatedEventBatchID: int64(3),
		SignalName:            "signal_name",
		SignalRequestID:       "signal_request_id",
		Input:                 []byte("input"),
		Control:               []byte("control"),
	}
	assert.Equal(t, expected, parseSignalInfo(testInput))
}

func Test_parseTimerTaskInfo(t *testing.T) {
	timeNow := time.Now()
	testInput := map[string]interface{}{
		"version":          int64(1),
		"visibility_ts":    timeNow,
		"task_id":          int64(2),
		"run_id":           newMockUUID("run_id"),
		"type":             3,
		"timeout_type":     3,
		"event_id":         int64(4),
		"schedule_attempt": int64(5),
	}
	expected := &persistence.TimerTaskInfo{
		Version:             int64(1),
		VisibilityTimestamp: timeNow,
		TaskID:              int64(2),
		RunID:               "run_id",
		TaskType:            3,
		TimeoutType:         3,
		EventID:             int64(4),
		ScheduleAttempt:     int64(5),
	}
	assert.Equal(t, expected, parseTimerTaskInfo(testInput))
}

func Test_parseReplicationTaskInfo(t *testing.T) {
	testInput := map[string]interface{}{
		"domain_id":            newMockUUID("domain_id"),
		"workflow_id":          "workflow_id",
		"run_id":               newMockUUID("run_id"),
		"task_id":              int64(1),
		"type":                 2,
		"first_event_id":       int64(3),
		"next_event_id":        int64(4),
		"version":              int64(5),
		"scheduled_id":         int64(6),
		"branch_token":         []byte("branch_token"),
		"new_run_branch_token": []byte("new_run_branch_token"),
		"created_time":         int64(7),
	}
	expected := &nosqlplugin.ReplicationTask{
		DomainID:          "domain_id",
		WorkflowID:        "workflow_id",
		RunID:             "run_id",
		TaskID:            int64(1),
		TaskType:          2,
		FirstEventID:      int64(3),
		NextEventID:       int64(4),
		Version:           int64(5),
		ScheduledID:       int64(6),
		BranchToken:       []byte("branch_token"),
		NewRunBranchToken: []byte("new_run_branch_token"),
		CreationTime:      time.Unix(0, 7),
	}
	assert.Equal(t, expected, parseReplicationTaskInfo(testInput))
}

func Test_parseTransferTaskInfo(t *testing.T) {
	timeNow := time.Now()
	testInput := map[string]interface{}{
		"domain_id":                  newMockUUID("domain_id"),
		"workflow_id":                "workflow_id",
		"run_id":                     newMockUUID("run_id"),
		"visibility_ts":              timeNow,
		"task_id":                    int64(1),
		"target_domain_id":           newMockUUID("target_domain_id"),
		"target_domain_ids":          []interface{}{newMockUUID("target_domain_id")},
		"target_workflow_id":         "target_workflow_id",
		"target_run_id":              newMockUUID("target_run_id"),
		"target_child_workflow_only": true,
		"task_list":                  "task_list",
		"type":                       2,
		"schedule_id":                int64(3),
		"record_visibility":          true,
		"version":                    int64(4),
	}
	expected := &persistence.TransferTaskInfo{
		DomainID:                "domain_id",
		WorkflowID:              "workflow_id",
		RunID:                   "run_id",
		VisibilityTimestamp:     timeNow,
		TaskID:                  int64(1),
		TargetDomainID:          "target_domain_id",
		TargetDomainIDs:         map[string]struct{}{"target_domain_id": {}},
		TargetWorkflowID:        "target_workflow_id",
		TargetRunID:             "target_run_id",
		TargetChildWorkflowOnly: true,
		TaskList:                "task_list",
		TaskType:                2,
		ScheduleID:              int64(3),
		RecordVisibility:        true,
		Version:                 int64(4),
	}
	assert.Equal(t, expected, parseTransferTaskInfo(testInput))

	// edge case
	testInput = map[string]interface{}{
		"target_run_id": newMockUUID(persistence.TransferTaskTransferTargetRunID),
	}
	expected = &persistence.TransferTaskInfo{
		TargetRunID: "",
	}
	assert.Equal(t, expected, parseTransferTaskInfo(testInput))
}

func Test_parseChecksum(t *testing.T) {
	testInput := map[string]interface{}{
		"version": 1,
		"flavor":  2,
		"value":   []byte("value"),
	}
	expected := checksum.Checksum{
		Version: 1,
		Flavor:  2,
		Value:   []byte("value"),
	}
	assert.Equal(t, expected, parseChecksum(testInput))
}
