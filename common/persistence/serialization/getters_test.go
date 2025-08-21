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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func TestGettersForAllNilInfos(t *testing.T) {
	for _, info := range []any{
		&WorkflowExecutionInfo{},
		&TransferTaskInfo{},
		&TimerTaskInfo{},
		&ReplicationTaskInfo{},
		&TaskListInfo{},
		&TaskInfo{},
		&TimerInfo{},
		&RequestCancelInfo{},
		&SignalInfo{},
		&ChildExecutionInfo{},
		&ActivityInfo{},
		&HistoryTreeInfo{},
		&DomainInfo{},
		&ShardInfo{},
	} {
		name := reflect.TypeOf(info).String()
		t.Run(name, func(t *testing.T) {
			res := nilView(info)
			if diff := cmp.Diff(expectedNil[name], res); diff != "" {
				t.Errorf("nilValue mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGettersForEmptyInfos(t *testing.T) {
	for _, info := range []any{
		&WorkflowExecutionInfo{},
		&TransferTaskInfo{},
		&TimerTaskInfo{},
		&ReplicationTaskInfo{},
		&TaskListInfo{},
		&TaskInfo{},
		&TimerInfo{},
		&RequestCancelInfo{},
		&SignalInfo{},
		&ChildExecutionInfo{},
		&ActivityInfo{},
		&HistoryTreeInfo{},
		&DomainInfo{},
		&ShardInfo{},
	} {
		name := reflect.TypeOf(info).String()
		t.Run(name, func(t *testing.T) {
			res := emptyView(info)
			if diff := cmp.Diff(expectedEmpty[name], res); diff != "" {
				t.Errorf("emptyValue mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

var (
	parentDomainID = MustParseUUID("00000000-0000-0000-0000-000000000001")
	parentRunID    = MustParseUUID("00000000-0000-0000-0000-000000000002")

	taskDomainID = MustParseUUID("00000000-0000-0000-0000-000000000003")
	taskRunID    = MustParseUUID("00000000-0000-0000-0000-000000000004")

	replicationTaskDomainID      = MustParseUUID("00000000-0000-0000-0000-000000000005")
	replicationTaskRunID         = MustParseUUID("00000000-0000-0000-0000-000000000006")
	replicationCreationTimestamp = time.Unix(10, 0)

	taskListInfoExpireTime     = time.Unix(20, 0)
	taskListInfoLastUpdateTime = time.Unix(30, 0)

	taskInfoRunID      = MustParseUUID("00000000-0000-0000-0000-000000000007")
	taskInfoExpiryTime = time.Unix(40, 0)
	taskInfoCreateTime = time.Unix(50, 0)

	timerInfoExpireTime = time.Unix(60, 0)

	signalInfoInput   = []byte("signalInput")
	signalInfoControl = []byte("signalControl")

	childExecutionInfoInitiatedEvent = []byte("initiatedEvent")
	childExecutionInfoStartedRunID   = MustParseUUID("00000000-0000-0000-0000-000000000008")
	childExecutionInfoStartedEvent   = []byte("startedEvent")

	activityInfoScheduledTime     = time.Unix(70, 0)
	activeInfoStartedTime         = time.Unix(80, 0)
	activeInfoRetryExpirationTime = time.Unix(90, 0)

	historyTreeEventCreatedTime = time.Unix(100, 0)

	domainInfoFailoverEndTimestamp = time.Unix(110, 0)
	domainInfoLastUpdatedTimestamp = time.Unix(120, 0)

	shardInfoUpdatedTime   = time.Unix(130, 0)
	shardInfoTimerAckLevel = time.Unix(140, 0)
)

func TestGettersForInfos(t *testing.T) {
	for _, info := range []any{
		&WorkflowExecutionInfo{
			ParentDomainID:          parentDomainID,
			ParentWorkflowID:        "parentWorkflowID",
			ParentRunID:             parentRunID,
			InitiatedID:             1,
			CompletionEventBatchID:  common.Int64Ptr(2),
			CompletionEvent:         []byte("completionEvent"),
			CompletionEventEncoding: "completionEventEncoding",
			TaskList:                "taskList",
			TaskListKind:            types.TaskListKindEphemeral,
			WorkflowTypeName:        "workflowTypeName",
			WorkflowTimeout:         3,
			DecisionTimeout:         4,
			ExecutionContext:        []byte("executionContext"),
			State:                   5,
			CloseStatus:             6,
			LastFirstEventID:        7,
			AutoResetPoints:         []byte("resetpoints"),
			SearchAttributes:        map[string][]byte{"key": []byte("value")},
		},
		&TransferTaskInfo{
			DomainID:                taskDomainID,
			WorkflowID:              "workflowID",
			RunID:                   taskRunID,
			TaskType:                1,
			TargetDomainID:          parentDomainID,
			TargetDomainIDs:         []UUID{parentDomainID, taskDomainID},
			TargetWorkflowID:        "targetID",
			TargetRunID:             parentRunID,
			TaskList:                "tasklist",
			TargetChildWorkflowOnly: true,
			ScheduleID:              2,
			Version:                 3,
			VisibilityTimestamp:     taskInfoCreateTime,
		},
		&TimerTaskInfo{
			DomainID:        taskDomainID,
			WorkflowID:      "workflowID",
			RunID:           taskRunID,
			TaskType:        1,
			TimeoutType:     common.Int16Ptr(2),
			Version:         3,
			ScheduleAttempt: 4,
			EventID:         5,
		},
		&ReplicationTaskInfo{
			DomainID:                replicationTaskDomainID,
			WorkflowID:              "workflowID",
			RunID:                   replicationTaskRunID,
			TaskType:                1,
			Version:                 2,
			FirstEventID:            3,
			NextEventID:             4,
			ScheduledID:             5,
			EventStoreVersion:       6,
			NewRunEventStoreVersion: 7,
			BranchToken:             []byte("branchToken"),
			NewRunBranchToken:       []byte("newRunBranchToken"),
			CreationTimestamp:       replicationCreationTimestamp,
		},
		&TaskListInfo{
			Kind:            1,
			AckLevel:        2,
			ExpiryTimestamp: taskListInfoExpireTime,
			LastUpdated:     taskListInfoLastUpdateTime,
		},
		&TaskInfo{
			WorkflowID:       "workflowID",
			RunID:            taskInfoRunID,
			ScheduleID:       1,
			ExpiryTimestamp:  taskInfoExpiryTime,
			CreatedTimestamp: taskInfoCreateTime,
			PartitionConfig: map[string]string{
				"key": "value",
			},
		},
		&TimerInfo{
			Version:         1,
			StartedID:       2,
			ExpiryTimestamp: timerInfoExpireTime,
			TaskID:          3,
		},
		&RequestCancelInfo{
			Version:               1,
			InitiatedEventBatchID: 2,
			CancelRequestID:       "cancelRequestID",
		},
		&SignalInfo{
			Version:               1,
			InitiatedEventBatchID: 2,
			RequestID:             "signalRequestID",
			Name:                  "signalName",
			Input:                 signalInfoInput,
			Control:               signalInfoControl,
		},
		&ChildExecutionInfo{
			Version:                1,
			InitiatedEventBatchID:  2,
			StartedID:              3,
			InitiatedEvent:         childExecutionInfoInitiatedEvent,
			InitiatedEventEncoding: "initiatedEventEncoding",
			StartedWorkflowID:      "startedWorkflowID",
			StartedRunID:           childExecutionInfoStartedRunID,
			StartedEvent:           childExecutionInfoStartedEvent,
			StartedEventEncoding:   "startedEventEncoding",
			CreateRequestID:        "createRequestID",
			DomainID:               "domainID",
			WorkflowTypeName:       "workflowTypeName",
			ParentClosePolicy:      1,
		},
		&ActivityInfo{
			Version:                  1,
			ScheduledEventBatchID:    2,
			ScheduledEvent:           []byte("scheduledEvent"),
			ScheduledEventEncoding:   "scheduledEventEncoding",
			ScheduledTimestamp:       activityInfoScheduledTime,
			StartedID:                3,
			StartedEvent:             []byte("startedEvent"),
			StartedEventEncoding:     "startedEventEncoding",
			StartedTimestamp:         activeInfoStartedTime,
			ActivityID:               "activityID",
			RequestID:                "requestID",
			ScheduleToCloseTimeout:   time.Duration(1),
			ScheduleToStartTimeout:   time.Duration(2),
			StartToCloseTimeout:      time.Duration(3),
			HeartbeatTimeout:         time.Duration(4),
			CancelRequested:          true,
			CancelRequestID:          4,
			TimerTaskStatus:          5,
			Attempt:                  6,
			TaskList:                 "taskList",
			TaskListKind:             types.TaskListKindSticky,
			StartedIdentity:          "startedIdentity",
			HasRetryPolicy:           true,
			RetryInitialInterval:     time.Duration(5),
			RetryMaximumInterval:     time.Duration(6),
			RetryMaximumAttempts:     7,
			RetryExpirationTimestamp: activeInfoRetryExpirationTime,
			RetryBackoffCoefficient:  8,
			RetryNonRetryableErrors:  []string{"error1", "error2"},
			RetryLastWorkerIdentity:  "retryLastWorkerIdentity",
			RetryLastFailureReason:   "retryLastFailureReason",
			RetryLastFailureDetails:  []byte("retryLastFailureDetails"),
		},
		&HistoryTreeInfo{
			CreatedTimestamp: historyTreeEventCreatedTime,
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID: "branchID1",
				},
				{
					BranchID: "branchID2",
				},
			},
			Info: "historyTreeInfo",
		},
		&DomainInfo{
			Name:                         "name",
			Description:                  "description",
			Owner:                        "owner",
			Status:                       1,
			Retention:                    time.Duration(1),
			EmitMetric:                   true,
			ArchivalBucket:               "archivalBucket",
			ArchivalStatus:               2,
			ConfigVersion:                3,
			NotificationVersion:          4,
			FailoverNotificationVersion:  5,
			FailoverVersion:              6,
			ActiveClusterName:            "cluster1",
			Clusters:                     []string{"cluster1", "cluster2"},
			ActiveClustersConfig:         []byte("activeClustersConfig"),
			ActiveClustersConfigEncoding: "activeClustersConfigEncoding",
			Data: map[string]string{
				"datakey": "datavalue",
			},
			BadBinaries:              []byte("badBinaries"),
			BadBinariesEncoding:      "badBinariesEncoding",
			HistoryArchivalStatus:    7,
			HistoryArchivalURI:       "historyArchivalURI",
			VisibilityArchivalStatus: 8,
			VisibilityArchivalURI:    "visibilityArchivalURI",
			FailoverEndTimestamp:     &domainInfoFailoverEndTimestamp,
			PreviousFailoverVersion:  9,
			LastUpdatedTimestamp:     domainInfoLastUpdatedTimestamp,
			IsolationGroups:          []byte("isolationGroups"),
			IsolationGroupsEncoding:  "isolationGroupsEncoding",
		},
		&ShardInfo{
			StolenSinceRenew:          1,
			UpdatedAt:                 shardInfoUpdatedTime,
			ReplicationAckLevel:       2,
			TransferAckLevel:          3,
			TimerAckLevel:             shardInfoTimerAckLevel,
			DomainNotificationVersion: 4,
			ClusterTransferAckLevel: map[string]int64{
				"cluster1": 5,
			},
			ClusterTimerAckLevel: map[string]time.Time{
				"cluster1": shardInfoTimerAckLevel,
			},
			Owner: "owner",
			ClusterReplicationLevel: map[string]int64{
				"cluster1": 6,
			},
			PendingFailoverMarkers:         []byte("pendingFailoverMarkers"),
			PendingFailoverMarkersEncoding: "pendingFailoverMarkersEncoding",
			ReplicationDlqAckLevel: map[string]int64{
				"cluster1": 7,
			},
			TransferProcessingQueueStates:             []byte("transferProcessingQueueStates"),
			TransferProcessingQueueStatesEncoding:     "transferProcessingQueueStatesEncoding",
			TimerProcessingQueueStates:                []byte("timerProcessingQueueStates"),
			TimerProcessingQueueStatesEncoding:        "timerProcessingQueueStatesEncoding",
			CrossClusterProcessingQueueStates:         []byte("crossClusterProcessingQueueStates"),
			CrossClusterProcessingQueueStatesEncoding: "crossClusterProcessingQueueStatesEncoding",
			QueueStates: map[int32]*types.QueueState{
				0: &types.QueueState{
					VirtualQueueStates: map[int64]*types.VirtualQueueState{
						0: {
							VirtualSliceStates: []*types.VirtualSliceState{
								{
									TaskRange: &types.TaskRange{
										InclusiveMin: &types.TaskKey{
											TaskID: 1,
										},
										ExclusiveMax: &types.TaskKey{
											TaskID: 2,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		name := reflect.TypeOf(info).String()
		t.Run(name, func(t *testing.T) {
			val := infoView(info)
			require.Equal(t, expectedNonEmpty[name], val)
		})
	}
}

func nilView(info any) map[string]any {
	infoVal := reflect.ValueOf(info)
	infoT := reflect.TypeOf(infoVal.Interface())
	v := reflect.Zero(infoT)
	res := make(map[string]any)
	for i := 0; i < infoT.NumMethod(); i++ {
		method := infoT.Method(i)
		if strings.HasPrefix(method.Name, "Get") {
			res[method.Name] = v.MethodByName(method.Name).Call(nil)[0].Interface()
		}
	}

	return res
}

func emptyView(info any) map[string]any {
	infoVal := reflect.ValueOf(info)
	infoT := reflect.TypeOf(infoVal.Interface())
	res := make(map[string]any)
	for i := 0; i < infoT.NumMethod(); i++ {
		method := infoT.Method(i)
		if strings.HasPrefix(method.Name, "Get") {
			res[method.Name] = infoVal.MethodByName(method.Name).Call(nil)[0].Interface()
		}
	}

	return res
}

func infoView(info any) map[string]any {
	v := reflect.ValueOf(info)
	infoT := reflect.TypeOf(v.Interface())
	res := make(map[string]any)
	for i := 0; i < infoT.NumMethod(); i++ {
		method := infoT.Method(i)
		if strings.HasPrefix(method.Name, "Get") {
			resVal := v.MethodByName(method.Name).Call(nil)[0]
			res[method.Name] = resVal.Interface()
		}
	}

	return res
}
