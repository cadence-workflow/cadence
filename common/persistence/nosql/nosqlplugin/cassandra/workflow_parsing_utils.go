// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cassandra

import (
	"time"

	cql "github.com/gocql/gocql"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

var _emptyUUID = cql.UUID{}

func parseWorkflowExecutionInfo(result map[string]interface{}) *persistence.InternalWorkflowExecutionInfo {
	info := &persistence.InternalWorkflowExecutionInfo{}
	var completionEventData []byte
	var completionEventEncoding constants.EncodingType
	var autoResetPoints []byte
	var autoResetPointsEncoding constants.EncodingType
	var activeClusterSelectionPolicy []byte
	var activeClusterSelectionPolicyEncoding constants.EncodingType

	for k, v := range result {
		switch k {
		case "domain_id":
			info.DomainID = v.(gocql.UUID).String()
		case "workflow_id":
			info.WorkflowID = v.(string)
		case "run_id":
			info.RunID = v.(gocql.UUID).String()
		case "first_run_id":
			info.FirstExecutionRunID = v.(gocql.UUID).String()
			if info.FirstExecutionRunID == _emptyUUID.String() {
				// for backward compatibility, the gocql library doesn't handle the null uuid correectly https://github.com/gocql/gocql/blob/master/marshal.go#L1807
				info.FirstExecutionRunID = ""
			} else if info.FirstExecutionRunID == emptyRunID {
				info.FirstExecutionRunID = ""
			}
		case "parent_domain_id":
			info.ParentDomainID = v.(gocql.UUID).String()
			if info.ParentDomainID == emptyDomainID {
				info.ParentDomainID = ""
			}
		case "parent_workflow_id":
			info.ParentWorkflowID = v.(string)
		case "parent_run_id":
			info.ParentRunID = v.(gocql.UUID).String()
			if info.ParentRunID == emptyRunID {
				info.ParentRunID = ""
			}
		case "initiated_id":
			info.InitiatedID = v.(int64)
		case "completion_event_batch_id":
			info.CompletionEventBatchID = v.(int64)
		case "completion_event":
			completionEventData = v.([]byte)
		case "completion_event_data_encoding":
			completionEventEncoding = constants.EncodingType(v.(string))
		case "auto_reset_points":
			autoResetPoints = v.([]byte)
		case "auto_reset_points_encoding":
			autoResetPointsEncoding = constants.EncodingType(v.(string))
		case "task_list":
			info.TaskList = v.(string)
		case "task_list_kind":
			info.TaskListKind = types.TaskListKind(int32(v.(int)))
		case "workflow_type_name":
			info.WorkflowTypeName = v.(string)
		case "workflow_timeout":
			info.WorkflowTimeout = common.SecondsToDuration(int64(v.(int)))
		case "decision_task_timeout":
			info.DecisionStartToCloseTimeout = common.SecondsToDuration(int64(v.(int)))
		case "execution_context":
			info.ExecutionContext = v.([]byte)
		case "state":
			info.State = v.(int)
		case "close_status":
			info.CloseStatus = v.(int)
		case "last_first_event_id":
			info.LastFirstEventID = v.(int64)
		case "last_event_task_id":
			info.LastEventTaskID = v.(int64)
		case "next_event_id":
			info.NextEventID = v.(int64)
		case "last_processed_event":
			info.LastProcessedEvent = v.(int64)
		case "start_time":
			info.StartTimestamp = v.(time.Time)
		case "last_updated_time":
			info.LastUpdatedTimestamp = v.(time.Time)
		case "create_request_id":
			info.CreateRequestID = v.(gocql.UUID).String()
		case "signal_count":
			info.SignalCount = int32(v.(int))
		case "history_size":
			info.HistorySize = v.(int64)
		case "decision_version":
			info.DecisionVersion = v.(int64)
		case "decision_schedule_id":
			info.DecisionScheduleID = v.(int64)
		case "decision_started_id":
			info.DecisionStartedID = v.(int64)
		case "decision_request_id":
			info.DecisionRequestID = v.(string)
		case "decision_timeout":
			info.DecisionTimeout = common.SecondsToDuration(int64(v.(int)))
		case "decision_attempt":
			info.DecisionAttempt = v.(int64)
		case "decision_timestamp":
			info.DecisionStartedTimestamp = time.Unix(0, v.(int64))
		case "decision_scheduled_timestamp":
			info.DecisionScheduledTimestamp = time.Unix(0, v.(int64))
		case "decision_original_scheduled_timestamp":
			info.DecisionOriginalScheduledTimestamp = time.Unix(0, v.(int64))
		case "cancel_requested":
			info.CancelRequested = v.(bool)
		case "cancel_request_id":
			info.CancelRequestID = v.(string)
		case "sticky_task_list":
			info.StickyTaskList = v.(string)
		case "sticky_schedule_to_start_timeout":
			info.StickyScheduleToStartTimeout = common.SecondsToDuration(int64(v.(int)))
		case "client_library_version":
			info.ClientLibraryVersion = v.(string)
		case "client_feature_version":
			info.ClientFeatureVersion = v.(string)
		case "client_impl":
			info.ClientImpl = v.(string)
		case "attempt":
			info.Attempt = int32(v.(int))
		case "has_retry_policy":
			info.HasRetryPolicy = v.(bool)
		case "init_interval":
			info.InitialInterval = common.SecondsToDuration(int64(v.(int)))
		case "backoff_coefficient":
			info.BackoffCoefficient = v.(float64)
		case "max_interval":
			info.MaximumInterval = common.SecondsToDuration(int64(v.(int)))
		case "max_attempts":
			info.MaximumAttempts = int32(v.(int))
		case "expiration_time":
			info.ExpirationTime = v.(time.Time)
		case "non_retriable_errors":
			info.NonRetriableErrors = v.([]string)
		case "branch_token":
			info.BranchToken = v.([]byte)
		case "cron_schedule":
			info.CronSchedule = v.(string)
		case "expiration_seconds":
			info.ExpirationInterval = common.SecondsToDuration(int64(v.(int)))
		case "search_attributes":
			info.SearchAttributes = v.(map[string][]byte)
		case "memo":
			info.Memo = v.(map[string][]byte)
		case "partition_config":
			info.PartitionConfig = v.(map[string]string)
		case "active_cluster_selection_policy":
			activeClusterSelectionPolicy = v.([]byte)
		case "active_cluster_selection_policy_encoding":
			activeClusterSelectionPolicyEncoding = constants.EncodingType(v.(string))
		case "cron_overlap_policy":
			info.CronOverlapPolicy = types.CronOverlapPolicy(int32(v.(int)))
		}
	}
	info.CompletionEvent = persistence.NewDataBlob(completionEventData, completionEventEncoding)
	info.AutoResetPoints = persistence.NewDataBlob(autoResetPoints, autoResetPointsEncoding)
	info.ActiveClusterSelectionPolicy = persistence.NewDataBlob(activeClusterSelectionPolicy, activeClusterSelectionPolicyEncoding)
	return info
}

// TODO: remove this after all 2DC workflows complete
func parseReplicationState(
	result map[string]interface{},
) *persistence.ReplicationState {

	if len(result) == 0 {
		return nil
	}

	info := &persistence.ReplicationState{}
	for k, v := range result {
		switch k {
		case "current_version":
			info.CurrentVersion = v.(int64)
		case "start_version":
			info.StartVersion = v.(int64)
		case "last_write_version":
			info.LastWriteVersion = v.(int64)
		case "last_write_event_id":
			info.LastWriteEventID = v.(int64)
		case "last_replication_info":
			info.LastReplicationInfo = make(map[string]*persistence.ReplicationInfo)
			replicationInfoMap := v.(map[string]map[string]interface{})
			for key, value := range replicationInfoMap {
				info.LastReplicationInfo[key] = parseReplicationInfo(value)
			}
		}
	}

	return info
}

func parseReplicationInfo(
	result map[string]interface{},
) *persistence.ReplicationInfo {

	info := &persistence.ReplicationInfo{}
	for k, v := range result {
		switch k {
		case "version":
			info.Version = v.(int64)
		case "last_event_id":
			info.LastEventID = v.(int64)
		}
	}

	return info
}

func parseActivityInfo(
	domainID string,
	result map[string]interface{},
) *persistence.InternalActivityInfo {

	info := &persistence.InternalActivityInfo{}
	var sharedEncoding constants.EncodingType
	var scheduledEventData, startedEventData []byte
	for k, v := range result {
		switch k {
		case "version":
			info.Version = v.(int64)
		case "schedule_id":
			info.ScheduleID = v.(int64)
		case "scheduled_event_batch_id":
			info.ScheduledEventBatchID = v.(int64)
		case "scheduled_event":
			scheduledEventData = v.([]byte)
		case "scheduled_time":
			info.ScheduledTime = v.(time.Time)
		case "started_id":
			info.StartedID = v.(int64)
		case "started_event":
			startedEventData = v.([]byte)
		case "started_time":
			info.StartedTime = v.(time.Time)
		case "activity_id":
			info.ActivityID = v.(string)
		case "request_id":
			info.RequestID = v.(string)
		case "details":
			info.Details = v.([]byte)
		case "schedule_to_start_timeout":
			info.ScheduleToStartTimeout = common.SecondsToDuration(int64(v.(int)))
		case "schedule_to_close_timeout":
			info.ScheduleToCloseTimeout = common.SecondsToDuration(int64(v.(int)))
		case "start_to_close_timeout":
			info.StartToCloseTimeout = common.SecondsToDuration(int64(v.(int)))
		case "heart_beat_timeout":
			info.HeartbeatTimeout = common.SecondsToDuration(int64(v.(int)))
		case "cancel_requested":
			info.CancelRequested = v.(bool)
		case "cancel_request_id":
			info.CancelRequestID = v.(int64)
		case "last_hb_updated_time":
			info.LastHeartBeatUpdatedTime = v.(time.Time)
		case "timer_task_status":
			info.TimerTaskStatus = int32(v.(int))
		case "attempt":
			info.Attempt = int32(v.(int))
		case "task_list":
			info.TaskList = v.(string)
		case "task_list_kind":
			info.TaskListKind = types.TaskListKind(int32(v.(int)))
		case "started_identity":
			info.StartedIdentity = v.(string)
		case "has_retry_policy":
			info.HasRetryPolicy = v.(bool)
		case "init_interval":
			info.InitialInterval = common.SecondsToDuration(int64(v.(int)))
		case "backoff_coefficient":
			info.BackoffCoefficient = v.(float64)
		case "max_interval":
			info.MaximumInterval = common.SecondsToDuration(int64(v.(int)))
		case "max_attempts":
			info.MaximumAttempts = (int32)(v.(int))
		case "expiration_time":
			info.ExpirationTime = v.(time.Time)
		case "non_retriable_errors":
			info.NonRetriableErrors = v.([]string)
		case "last_failure_reason":
			info.LastFailureReason = v.(string)
		case "last_worker_identity":
			info.LastWorkerIdentity = v.(string)
		case "last_failure_details":
			info.LastFailureDetails = v.([]byte)
		case "event_data_encoding":
			sharedEncoding = constants.EncodingType(v.(string))
		}
	}
	info.DomainID = domainID
	info.ScheduledEvent = persistence.NewDataBlob(scheduledEventData, sharedEncoding)
	info.StartedEvent = persistence.NewDataBlob(startedEventData, sharedEncoding)

	return info
}

func parseTimerInfo(
	result map[string]interface{},
) *persistence.TimerInfo {

	info := &persistence.TimerInfo{}
	for k, v := range result {
		switch k {
		case "version":
			info.Version = v.(int64)
		case "timer_id":
			info.TimerID = v.(string)
		case "started_id":
			info.StartedID = v.(int64)
		case "expiry_time":
			info.ExpiryTime = v.(time.Time)
		case "task_id":
			// task_id is a misleading variable, it actually serves
			// the purpose of indicating whether a timer task is
			// generated for this timer info
			info.TaskStatus = v.(int64)
		}
	}
	return info
}

func parseChildExecutionInfo(
	result map[string]interface{},
) *persistence.InternalChildExecutionInfo {

	info := &persistence.InternalChildExecutionInfo{}
	var encoding constants.EncodingType
	var initiatedData []byte
	var startedData []byte
	for k, v := range result {
		switch k {
		case "version":
			info.Version = v.(int64)
		case "initiated_id":
			info.InitiatedID = v.(int64)
		case "initiated_event_batch_id":
			info.InitiatedEventBatchID = v.(int64)
		case "initiated_event":
			initiatedData = v.([]byte)
		case "started_id":
			info.StartedID = v.(int64)
		case "started_workflow_id":
			info.StartedWorkflowID = v.(string)
		case "started_run_id":
			info.StartedRunID = v.(gocql.UUID).String()
		case "started_event":
			startedData = v.([]byte)
		case "create_request_id":
			info.CreateRequestID = v.(gocql.UUID).String()
		case "event_data_encoding":
			encoding = constants.EncodingType(v.(string))
		case "domain_id":
			info.DomainID = v.(gocql.UUID).String()
			if info.DomainID == _emptyUUID.String() {
				// for backward compatibility, the gocql library doesn't handle the null uuid correectly https://github.com/gocql/gocql/blob/master/marshal.go#L1807
				info.DomainID = ""
			}
		case "domain_name":
			info.DomainNameDEPRECATED = v.(string)
		case "workflow_type_name":
			info.WorkflowTypeName = v.(string)
		case "parent_close_policy":
			info.ParentClosePolicy = types.ParentClosePolicy(v.(int))
		}
	}
	info.InitiatedEvent = persistence.NewDataBlob(initiatedData, encoding)
	info.StartedEvent = persistence.NewDataBlob(startedData, encoding)
	return info
}

func parseRequestCancelInfo(
	result map[string]interface{},
) *persistence.RequestCancelInfo {

	info := &persistence.RequestCancelInfo{}
	for k, v := range result {
		switch k {
		case "version":
			info.Version = v.(int64)
		case "initiated_id":
			info.InitiatedID = v.(int64)
		case "initiated_event_batch_id":
			info.InitiatedEventBatchID = v.(int64)
		case "cancel_request_id":
			info.CancelRequestID = v.(string)
		}
	}

	return info
}

func parseSignalInfo(
	result map[string]interface{},
) *persistence.SignalInfo {

	info := &persistence.SignalInfo{}
	for k, v := range result {
		switch k {
		case "version":
			info.Version = v.(int64)
		case "initiated_id":
			info.InitiatedID = v.(int64)
		case "initiated_event_batch_id":
			info.InitiatedEventBatchID = v.(int64)
		case "signal_request_id":
			info.SignalRequestID = v.(gocql.UUID).String()
		case "signal_name":
			info.SignalName = v.(string)
		case "input":
			info.Input = v.([]byte)
		case "control":
			info.Control = v.([]byte)
		}
	}

	return info
}

func parseHistoryEventBatchBlob(
	result map[string]interface{},
) *persistence.DataBlob {

	eventBatch := &persistence.DataBlob{Encoding: constants.EncodingTypeJSON}
	for k, v := range result {
		switch k {
		case "encoding_type":
			eventBatch.Encoding = constants.EncodingType(v.(string))
		case "data":
			eventBatch.Data = v.([]byte)
		}
	}

	return eventBatch
}

func parseTimerTaskInfo(
	result map[string]interface{},
) *persistence.TimerTaskInfo {

	info := &persistence.TimerTaskInfo{}
	for k, v := range result {
		switch k {
		case "domain_id":
			info.DomainID = v.(gocql.UUID).String()
		case "workflow_id":
			info.WorkflowID = v.(string)
		case "run_id":
			info.RunID = v.(gocql.UUID).String()
		case "visibility_ts":
			info.VisibilityTimestamp = v.(time.Time)
		case "task_id":
			info.TaskID = v.(int64)
		case "type":
			info.TaskType = v.(int)
		case "timeout_type":
			info.TimeoutType = v.(int)
		case "event_id":
			info.EventID = v.(int64)
		case "schedule_attempt":
			info.ScheduleAttempt = v.(int64)
		case "version":
			info.Version = v.(int64)
		}
	}

	return info
}

func parseTransferTaskInfo(
	result map[string]interface{},
) *persistence.TransferTaskInfo {

	info := &persistence.TransferTaskInfo{}
	for k, v := range result {
		switch k {
		case "domain_id":
			info.DomainID = v.(gocql.UUID).String()
		case "workflow_id":
			info.WorkflowID = v.(string)
		case "run_id":
			info.RunID = v.(gocql.UUID).String()
		case "visibility_ts":
			info.VisibilityTimestamp = v.(time.Time)
		case "task_id":
			info.TaskID = v.(int64)
		case "target_domain_id":
			info.TargetDomainID = v.(gocql.UUID).String()
		case "target_domain_ids":
			targetDomainIDs := make(map[string]struct{})
			dList := mustConvertToSlice(result["target_domain_ids"])
			for _, v := range dList {
				targetDomainIDs[v.(gocql.UUID).String()] = struct{}{}
			}
			info.TargetDomainIDs = targetDomainIDs
		case "target_workflow_id":
			info.TargetWorkflowID = v.(string)
		case "target_run_id":
			info.TargetRunID = v.(gocql.UUID).String()
			if info.TargetRunID == persistence.TransferTaskTransferTargetRunID {
				info.TargetRunID = ""
			}
		case "target_child_workflow_only":
			info.TargetChildWorkflowOnly = v.(bool)
		case "task_list":
			info.TaskList = v.(string)
		case "type":
			info.TaskType = v.(int)
		case "schedule_id":
			info.ScheduleID = v.(int64)
		case "record_visibility":
			info.RecordVisibility = v.(bool)
		case "version":
			info.Version = v.(int64)
		}
	}

	return info
}

func parseReplicationTaskInfo(
	result map[string]interface{},
) *nosqlplugin.ReplicationTask {

	info := &persistence.InternalReplicationTaskInfo{}
	for k, v := range result {
		switch k {
		case "domain_id":
			info.DomainID = v.(gocql.UUID).String()
		case "workflow_id":
			info.WorkflowID = v.(string)
		case "run_id":
			info.RunID = v.(gocql.UUID).String()
		case "task_id":
			info.TaskID = v.(int64)
		case "type":
			info.TaskType = v.(int)
		case "first_event_id":
			info.FirstEventID = v.(int64)
		case "next_event_id":
			info.NextEventID = v.(int64)
		case "version":
			info.Version = v.(int64)
		case "scheduled_id":
			info.ScheduledID = v.(int64)
		case "branch_token":
			info.BranchToken = v.([]byte)
		case "new_run_branch_token":
			info.NewRunBranchToken = v.([]byte)
		case "created_time":
			info.CreationTime = time.Unix(0, v.(int64))
		}
	}

	return info
}

func parseChecksum(result map[string]interface{}) checksum.Checksum {
	csum := checksum.Checksum{}
	if len(result) == 0 {
		return csum
	}
	for k, v := range result {
		switch k {
		case "flavor":
			csum.Flavor = checksum.Flavor(v.(int))
		case "version":
			csum.Version = v.(int)
		case "value":
			csum.Value = v.([]byte)
		}
	}
	return csum
}
