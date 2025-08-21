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
	"math/rand"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func TestShardInfo(t *testing.T) {
	expected := &ShardInfo{
		StolenSinceRenew:                      int32(rand.Intn(1000)),
		UpdatedAt:                             time.Now(),
		ReplicationAckLevel:                   int64(rand.Intn(1000)),
		TransferAckLevel:                      int64(rand.Intn(1000)),
		TimerAckLevel:                         time.Now(),
		DomainNotificationVersion:             int64(rand.Intn(1000)),
		ClusterTransferAckLevel:               map[string]int64{"key_1": int64(rand.Intn(1000)), "key_2": int64(rand.Intn(1000))},
		ClusterTimerAckLevel:                  map[string]time.Time{"key_1": time.Now(), "key_2": time.Now()},
		Owner:                                 "test_owner",
		ClusterReplicationLevel:               map[string]int64{"key_1": int64(rand.Intn(1000)), "key_2": int64(rand.Intn(1000))},
		PendingFailoverMarkers:                []byte("PendingFailoverMarkers"),
		PendingFailoverMarkersEncoding:        "PendingFailoverMarkersEncoding",
		ReplicationDlqAckLevel:                map[string]int64{"key_1": int64(rand.Intn(1000)), "key_2": int64(rand.Intn(1000))},
		TransferProcessingQueueStates:         []byte("TransferProcessingQueueStates"),
		TransferProcessingQueueStatesEncoding: "TransferProcessingQueueStatesEncoding",
		TimerProcessingQueueStates:            []byte("TimerProcessingQueueStates"),
		TimerProcessingQueueStatesEncoding:    "TimerProcessingQueueStatesEncoding",
		QueueStates: map[int32]*types.QueueState{
			0: &types.QueueState{
				VirtualQueueStates: map[int64]*types.VirtualQueueState{
					0: {
						VirtualSliceStates: []*types.VirtualSliceState{
							{
								TaskRange: &types.TaskRange{
									InclusiveMin: &types.TaskKey{
										TaskID: 1000,
									},
									ExclusiveMax: &types.TaskKey{
										TaskID: 2000,
									},
								},
							},
						},
					},
				},
				ExclusiveMaxReadLevel: &types.TaskKey{
					TaskID: 1000,
				},
			},
		},
	}
	actual := shardInfoFromThrift(shardInfoToThrift(expected))
	assert.Equal(t, expected.StolenSinceRenew, actual.StolenSinceRenew)
	assert.Equal(t, expected.UpdatedAt.Sub(actual.UpdatedAt), time.Duration(0))
	assert.Equal(t, expected.ReplicationAckLevel, actual.ReplicationAckLevel)
	assert.Equal(t, expected.TransferAckLevel, actual.TransferAckLevel)
	assert.Equal(t, expected.TimerAckLevel.Sub(actual.TimerAckLevel), time.Duration(0))
	assert.Equal(t, expected.DomainNotificationVersion, actual.DomainNotificationVersion)
	assert.Equal(t, expected.ClusterTransferAckLevel, actual.ClusterTransferAckLevel)
	assert.Equal(t, expected.Owner, actual.Owner)
	assert.Equal(t, expected.ClusterReplicationLevel, actual.ClusterReplicationLevel)
	assert.Equal(t, expected.PendingFailoverMarkers, actual.PendingFailoverMarkers)
	assert.Equal(t, expected.PendingFailoverMarkersEncoding, actual.PendingFailoverMarkersEncoding)
	assert.Equal(t, expected.ReplicationDlqAckLevel, actual.ReplicationDlqAckLevel)
	assert.Equal(t, expected.TransferProcessingQueueStates, actual.TransferProcessingQueueStates)
	assert.Equal(t, expected.TransferProcessingQueueStatesEncoding, actual.TransferProcessingQueueStatesEncoding)
	assert.Equal(t, expected.TimerProcessingQueueStates, actual.TimerProcessingQueueStates)
	assert.Equal(t, expected.TimerProcessingQueueStatesEncoding, actual.TimerProcessingQueueStatesEncoding)
	assert.Len(t, actual.ClusterTimerAckLevel, 2)
	assert.Contains(t, actual.ClusterTimerAckLevel, "key_1")
	assert.Contains(t, actual.ClusterTimerAckLevel, "key_2")
	assert.Equal(t, expected.ClusterTimerAckLevel["key_1"].Sub(actual.ClusterTimerAckLevel["key_1"]), time.Duration(0))
	assert.Equal(t, expected.ClusterTimerAckLevel["key_2"].Sub(actual.ClusterTimerAckLevel["key_2"]), time.Duration(0))
	assert.Nil(t, shardInfoFromThrift(nil))
	assert.Nil(t, shardInfoToThrift(nil))
}

func TestDomainInfo(t *testing.T) {
	expected := &DomainInfo{
		Name:                         "domain_name",
		Description:                  "description",
		Owner:                        "owner",
		Status:                       int32(rand.Intn(1000)),
		Retention:                    time.Duration(int64(rand.Intn(1000))),
		EmitMetric:                   true,
		ArchivalBucket:               "archival_bucket",
		ArchivalStatus:               int16(rand.Intn(1000)),
		ConfigVersion:                int64(rand.Intn(1000)),
		NotificationVersion:          int64(rand.Intn(1000)),
		FailoverNotificationVersion:  int64(rand.Intn(1000)),
		FailoverVersion:              int64(rand.Intn(1000)),
		ActiveClusterName:            "ActiveClusterName",
		ActiveClustersConfig:         []byte("activeClustersConfig"),
		ActiveClustersConfigEncoding: "activeClustersConfigEncoding",
		Clusters:                     []string{"cluster_a", "cluster_b"},
		Data:                         map[string]string{"key_1": "value_1", "key_2": "value_2"},
		BadBinaries:                  []byte("BadBinaries"),
		BadBinariesEncoding:          "BadBinariesEncoding",
		HistoryArchivalStatus:        int16(rand.Intn(1000)),
		HistoryArchivalURI:           "HistoryArchivalURI",
		VisibilityArchivalStatus:     int16(rand.Intn(1000)),
		VisibilityArchivalURI:        "VisibilityArchivalURI",
		FailoverEndTimestamp:         common.TimePtr(time.Now()),
		PreviousFailoverVersion:      int64(rand.Intn(1000)),
		LastUpdatedTimestamp:         time.Now(),
	}
	actual := domainInfoFromThrift(domainInfoToThrift(expected))
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Description, actual.Description)
	assert.Equal(t, expected.Owner, actual.Owner)
	assert.Equal(t, expected.Status, actual.Status)
	assert.True(t, (expected.Retention-actual.Retention) < time.Second)
	assert.Equal(t, expected.EmitMetric, actual.EmitMetric)
	assert.Equal(t, expected.ArchivalBucket, actual.ArchivalBucket)
	assert.Equal(t, expected.ArchivalStatus, actual.ArchivalStatus)
	assert.Equal(t, expected.ConfigVersion, actual.ConfigVersion)
	assert.Equal(t, expected.NotificationVersion, actual.NotificationVersion)
	assert.Equal(t, expected.FailoverNotificationVersion, actual.FailoverNotificationVersion)
	assert.Equal(t, expected.ActiveClusterName, actual.ActiveClusterName)
	assert.Equal(t, expected.Clusters, actual.Clusters)
	assert.Equal(t, expected.ActiveClustersConfig, actual.ActiveClustersConfig)
	assert.Equal(t, expected.ActiveClustersConfigEncoding, actual.ActiveClustersConfigEncoding)
	assert.Equal(t, expected.Data, actual.Data)
	assert.Equal(t, expected.BadBinaries, actual.BadBinaries)
	assert.Equal(t, expected.BadBinariesEncoding, actual.BadBinariesEncoding)
	assert.Equal(t, expected.HistoryArchivalStatus, actual.HistoryArchivalStatus)
	assert.Equal(t, expected.HistoryArchivalURI, actual.HistoryArchivalURI)
	assert.Equal(t, expected.VisibilityArchivalStatus, actual.VisibilityArchivalStatus)
	assert.Equal(t, expected.VisibilityArchivalURI, actual.VisibilityArchivalURI)
	assert.Equal(t, expected.FailoverEndTimestamp.Sub(*actual.FailoverEndTimestamp), time.Duration(0))
	assert.Equal(t, expected.PreviousFailoverVersion, actual.PreviousFailoverVersion)
	assert.Equal(t, expected.LastUpdatedTimestamp.Sub(actual.LastUpdatedTimestamp), time.Duration(0))
	assert.Nil(t, domainInfoFromThrift(nil))
	assert.Nil(t, domainInfoToThrift(nil))
}

func TestDomainInfoRoundtripPanictest(t *testing.T) {
	tests := map[string]struct {
		in *DomainInfo
	}{
		"empty roundtrip": {
			in: &DomainInfo{},
		},
		"nil roundtrip": {
			in: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.in, domainInfoFromThrift(domainInfoToThrift(td.in)))
		})
	}
}

func TestHistoryTreeInfo(t *testing.T) {
	expected := &HistoryTreeInfo{
		CreatedTimestamp: time.Now(),
		Ancestors: []*types.HistoryBranchRange{
			{
				BranchID:    "branch_id",
				BeginNodeID: int64(rand.Intn(1000)),
				EndNodeID:   int64(rand.Intn(1000)),
			},
			{
				BranchID:    "branch_id",
				BeginNodeID: int64(rand.Intn(1000)),
				EndNodeID:   int64(rand.Intn(1000)),
			},
		},
		Info: "info",
	}
	actual := historyTreeInfoFromThrift(historyTreeInfoToThrift(expected))
	assert.Equal(t, expected.CreatedTimestamp.Sub(actual.CreatedTimestamp), time.Duration(0))
	assert.Equal(t, expected.Ancestors, actual.Ancestors)
	assert.Equal(t, expected.Info, actual.Info)
	assert.Nil(t, historyTreeInfoFromThrift(nil))
	assert.Nil(t, historyTreeInfoToThrift(nil))
}

func TestWorkflowExecutionInfo(t *testing.T) {
	expected := &WorkflowExecutionInfo{
		ParentDomainID:                     UUID(uuid.New()),
		ParentWorkflowID:                   "ParentWorkflowID",
		ParentRunID:                        UUID(uuid.New()),
		InitiatedID:                        int64(rand.Intn(1000)),
		CompletionEventBatchID:             common.Int64Ptr(int64(rand.Intn(1000))),
		CompletionEvent:                    []byte("CompletionEvent"),
		CompletionEventEncoding:            "CompletionEventEncoding",
		TaskList:                           "TaskList",
		TaskListKind:                       types.TaskListKindNormal,
		WorkflowTypeName:                   "WorkflowTypeName",
		WorkflowTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionTaskTimeout:                time.Minute * time.Duration(rand.Intn(10)),
		ExecutionContext:                   []byte("ExecutionContext"),
		State:                              int32(rand.Intn(1000)),
		CloseStatus:                        int32(rand.Intn(1000)),
		StartVersion:                       int64(rand.Intn(1000)),
		LastWriteEventID:                   common.Int64Ptr(int64(rand.Intn(1000))),
		LastEventTaskID:                    int64(rand.Intn(1000)),
		LastFirstEventID:                   int64(rand.Intn(1000)),
		LastProcessedEvent:                 int64(rand.Intn(1000)),
		StartTimestamp:                     time.UnixMilli(1752018142820),
		LastUpdatedTimestamp:               time.UnixMilli(1752018142821),
		DecisionVersion:                    int64(rand.Intn(1000)),
		DecisionScheduleID:                 int64(rand.Intn(1000)),
		DecisionStartedID:                  int64(rand.Intn(1000)),
		DecisionTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionAttempt:                    int64(rand.Intn(1000)),
		DecisionStartedTimestamp:           time.UnixMilli(1752018142822),
		DecisionScheduledTimestamp:         time.UnixMilli(1752018142823),
		CancelRequested:                    true,
		DecisionOriginalScheduledTimestamp: time.UnixMilli(1752018142824),
		CreateRequestID:                    "CreateRequestID",
		DecisionRequestID:                  "DecisionRequestID",
		CancelRequestID:                    "CancelRequestID",
		StickyTaskList:                     "StickyTaskList",
		StickyScheduleToStartTimeout:       time.Minute * time.Duration(rand.Intn(10)),
		RetryAttempt:                       int64(rand.Intn(1000)),
		RetryInitialInterval:               time.Minute * time.Duration(rand.Intn(10)),
		RetryMaximumInterval:               time.Minute * time.Duration(rand.Intn(10)),
		RetryMaximumAttempts:               int32(rand.Intn(1000)),
		RetryExpiration:                    time.Minute * time.Duration(rand.Intn(10)),
		RetryBackoffCoefficient:            rand.Float64() * 1000,
		RetryExpirationTimestamp:           time.UnixMilli(1752018142825),
		RetryNonRetryableErrors:            []string{"RetryNonRetryableErrors"},
		HasRetryPolicy:                     true,
		CronSchedule:                       "CronSchedule",
		CronOverlapPolicy:                  types.CronOverlapPolicySkipped,
		EventStoreVersion:                  int32(rand.Intn(1000)),
		EventBranchToken:                   []byte("EventBranchToken"),
		SignalCount:                        int64(rand.Intn(1000)),
		HistorySize:                        int64(rand.Intn(1000)),
		ClientLibraryVersion:               "ClientLibraryVersion",
		ClientFeatureVersion:               "ClientFeatureVersion",
		ClientImpl:                         "ClientImpl",
		AutoResetPoints:                    []byte("AutoResetPoints"),
		AutoResetPointsEncoding:            "AutoResetPointsEncoding",
		SearchAttributes:                   map[string][]byte{"key_1": []byte("SearchAttributes")},
		Memo:                               map[string][]byte{"key_1": []byte("Memo")},
		VersionHistories:                   []byte("VersionHistories"),
		VersionHistoriesEncoding:           "VersionHistoriesEncoding",
		FirstExecutionRunID:                UUID(uuid.New()),
		PartitionConfig:                    map[string]string{"zone": "dca1"},
		Checksum:                           []byte("Checksum"),
		ChecksumEncoding:                   "ChecksumEncoding",
		IsCron:                             true,
	}
	actual := workflowExecutionInfoFromThrift(workflowExecutionInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, workflowExecutionInfoFromThrift(nil))
	assert.Nil(t, workflowExecutionInfoToThrift(nil))
}

func TestWorkflowExecutionInfo_NoTaskListKind(t *testing.T) {
	expected := &WorkflowExecutionInfo{
		ParentDomainID:                     UUID(uuid.New()),
		ParentWorkflowID:                   "ParentWorkflowID",
		ParentRunID:                        UUID(uuid.New()),
		InitiatedID:                        int64(rand.Intn(1000)),
		CompletionEventBatchID:             common.Int64Ptr(int64(rand.Intn(1000))),
		CompletionEvent:                    []byte("CompletionEvent"),
		CompletionEventEncoding:            "CompletionEventEncoding",
		TaskList:                           "TaskList",
		TaskListKind:                       types.TaskListKindNormal,
		WorkflowTypeName:                   "WorkflowTypeName",
		WorkflowTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionTaskTimeout:                time.Minute * time.Duration(rand.Intn(10)),
		ExecutionContext:                   []byte("ExecutionContext"),
		State:                              int32(rand.Intn(1000)),
		CloseStatus:                        int32(rand.Intn(1000)),
		StartVersion:                       int64(rand.Intn(1000)),
		LastWriteEventID:                   common.Int64Ptr(int64(rand.Intn(1000))),
		LastEventTaskID:                    int64(rand.Intn(1000)),
		LastFirstEventID:                   int64(rand.Intn(1000)),
		LastProcessedEvent:                 int64(rand.Intn(1000)),
		StartTimestamp:                     time.UnixMilli(1752018142820),
		LastUpdatedTimestamp:               time.UnixMilli(1752018142821),
		DecisionVersion:                    int64(rand.Intn(1000)),
		DecisionScheduleID:                 int64(rand.Intn(1000)),
		DecisionStartedID:                  int64(rand.Intn(1000)),
		DecisionTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionAttempt:                    int64(rand.Intn(1000)),
		DecisionStartedTimestamp:           time.UnixMilli(1752018142822),
		DecisionScheduledTimestamp:         time.UnixMilli(1752018142823),
		CancelRequested:                    true,
		DecisionOriginalScheduledTimestamp: time.UnixMilli(1752018142824),
		CreateRequestID:                    "CreateRequestID",
		DecisionRequestID:                  "DecisionRequestID",
		CancelRequestID:                    "CancelRequestID",
		StickyTaskList:                     "StickyTaskList",
		StickyScheduleToStartTimeout:       time.Minute * time.Duration(rand.Intn(10)),
		RetryAttempt:                       int64(rand.Intn(1000)),
		RetryInitialInterval:               time.Minute * time.Duration(rand.Intn(10)),
		RetryMaximumInterval:               time.Minute * time.Duration(rand.Intn(10)),
		RetryMaximumAttempts:               int32(rand.Intn(1000)),
		RetryExpiration:                    time.Minute * time.Duration(rand.Intn(10)),
		RetryBackoffCoefficient:            rand.Float64() * 1000,
		RetryExpirationTimestamp:           time.UnixMilli(1752018142825),
		RetryNonRetryableErrors:            []string{"RetryNonRetryableErrors"},
		HasRetryPolicy:                     true,
		CronSchedule:                       "CronSchedule",
		CronOverlapPolicy:                  types.CronOverlapPolicySkipped,
		EventStoreVersion:                  int32(rand.Intn(1000)),
		EventBranchToken:                   []byte("EventBranchToken"),
		SignalCount:                        int64(rand.Intn(1000)),
		HistorySize:                        int64(rand.Intn(1000)),
		ClientLibraryVersion:               "ClientLibraryVersion",
		ClientFeatureVersion:               "ClientFeatureVersion",
		ClientImpl:                         "ClientImpl",
		AutoResetPoints:                    []byte("AutoResetPoints"),
		AutoResetPointsEncoding:            "AutoResetPointsEncoding",
		SearchAttributes:                   map[string][]byte{"key_1": []byte("SearchAttributes")},
		Memo:                               map[string][]byte{"key_1": []byte("Memo")},
		VersionHistories:                   []byte("VersionHistories"),
		VersionHistoriesEncoding:           "VersionHistoriesEncoding",
		FirstExecutionRunID:                UUID(uuid.New()),
		PartitionConfig:                    map[string]string{"zone": "dca1"},
		Checksum:                           []byte("Checksum"),
		ChecksumEncoding:                   "ChecksumEncoding",
		IsCron:                             true,
	}
	thriftVersion := workflowExecutionInfoToThrift(expected)
	// Nil should come back as NORMAL, the default value
	thriftVersion.TaskListKind = nil
	actual := workflowExecutionInfoFromThrift(thriftVersion)
	assert.Equal(t, expected, actual)
}

func TestActivityInfo(t *testing.T) {
	expected := &ActivityInfo{
		Version:                  int64(rand.Intn(1000)),
		ScheduledEventBatchID:    int64(rand.Intn(1000)),
		ScheduledEvent:           []byte("ScheduledEvent"),
		ScheduledEventEncoding:   "ScheduledEventEncoding",
		ScheduledTimestamp:       time.UnixMilli(1752018142820),
		StartedID:                int64(rand.Intn(1000)),
		StartedEvent:             []byte("StartedEvent"),
		StartedEventEncoding:     "StartedEventEncoding",
		StartedTimestamp:         time.UnixMilli(1752018142821),
		ActivityID:               "ActivityID",
		RequestID:                "RequestID",
		ScheduleToStartTimeout:   time.Minute * time.Duration(rand.Intn(10)),
		ScheduleToCloseTimeout:   time.Minute * time.Duration(rand.Intn(10)),
		StartToCloseTimeout:      time.Minute * time.Duration(rand.Intn(10)),
		HeartbeatTimeout:         time.Minute * time.Duration(rand.Intn(10)),
		CancelRequested:          true,
		CancelRequestID:          int64(rand.Intn(1000)),
		TimerTaskStatus:          int32(rand.Intn(1000)),
		Attempt:                  int32(rand.Intn(1000)),
		TaskList:                 "TaskList",
		TaskListKind:             types.TaskListKindEphemeral,
		StartedIdentity:          "StartedIdentity",
		HasRetryPolicy:           true,
		RetryInitialInterval:     time.Minute * time.Duration(rand.Intn(10)),
		RetryMaximumInterval:     time.Minute * time.Duration(rand.Intn(10)),
		RetryMaximumAttempts:     int32(rand.Intn(1000)),
		RetryExpirationTimestamp: time.Time{},
		RetryBackoffCoefficient:  rand.Float64() * 1000,
		RetryNonRetryableErrors:  []string{"RetryNonRetryableErrors"},
		RetryLastFailureReason:   "RetryLastFailureReason",
		RetryLastWorkerIdentity:  "RetryLastWorkerIdentity",
		RetryLastFailureDetails:  []byte("RetryLastFailureDetails"),
	}
	actual := activityInfoFromThrift(activityInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, activityInfoFromThrift(nil))
	assert.Nil(t, activityInfoToThrift(nil))
}

func TestChildExecutionInfo(t *testing.T) {
	expected := &ChildExecutionInfo{
		Version:                int64(rand.Intn(1000)),
		InitiatedEventBatchID:  int64(rand.Intn(1000)),
		StartedID:              int64(rand.Intn(1000)),
		InitiatedEvent:         []byte("InitiatedEvent"),
		InitiatedEventEncoding: "InitiatedEventEncoding",
		StartedWorkflowID:      "InitiatedEventEncoding",
		StartedRunID:           UUID(uuid.New()),
		StartedEvent:           []byte("StartedEvent"),
		StartedEventEncoding:   "StartedEventEncoding",
		CreateRequestID:        "CreateRequestID",
		DomainID:               "DomainID",
		DomainNameDEPRECATED:   "DomainName",
		WorkflowTypeName:       "WorkflowTypeName",
		ParentClosePolicy:      int32(rand.Intn(1000)),
	}
	actual := childExecutionInfoFromThrift(childExecutionInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, childExecutionInfoFromThrift(nil))
	assert.Nil(t, childExecutionInfoToThrift(nil))
}

func TestSignalInfo(t *testing.T) {
	expected := &SignalInfo{
		Version:               int64(rand.Intn(1000)),
		InitiatedEventBatchID: int64(rand.Intn(1000)),
		RequestID:             "RequestID",
		Name:                  "Name",
		Input:                 []byte("Input"),
		Control:               []byte("Control"),
	}
	actual := signalInfoFromThrift(signalInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, signalInfoFromThrift(nil))
	assert.Nil(t, signalInfoToThrift(nil))
}

func TestRequestCancelInfo(t *testing.T) {
	expected := &RequestCancelInfo{
		Version:               int64(rand.Intn(1000)),
		InitiatedEventBatchID: int64(rand.Intn(1000)),
		CancelRequestID:       "CancelRequestID",
	}
	actual := requestCancelInfoFromThrift(requestCancelInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, requestCancelInfoFromThrift(nil))
	assert.Nil(t, requestCancelInfoToThrift(nil))
}

func TestTimerInfo(t *testing.T) {
	expected := &TimerInfo{
		Version:         int64(rand.Intn(1000)),
		StartedID:       int64(rand.Intn(1000)),
		ExpiryTimestamp: time.Now(),
		TaskID:          int64(rand.Intn(1000)),
	}
	actual := timerInfoFromThrift(timerInfoToThrift(expected))
	assert.Equal(t, expected.Version, actual.Version)
	assert.Equal(t, expected.StartedID, actual.StartedID)
	assert.Equal(t, expected.TaskID, actual.TaskID)
	assert.Equal(t, expected.ExpiryTimestamp.Sub(actual.ExpiryTimestamp), time.Duration(0))
	assert.Nil(t, timerInfoFromThrift(nil))
	assert.Nil(t, timerInfoToThrift(nil))
}

func TestTaskInfo(t *testing.T) {
	expected := &TaskInfo{
		WorkflowID:       "WorkflowID",
		RunID:            UUID(uuid.New()),
		ScheduleID:       int64(rand.Intn(1000)),
		ExpiryTimestamp:  time.Now(),
		CreatedTimestamp: time.Now(),
		PartitionConfig:  map[string]string{"zone": "dca1"},
	}
	actual := taskInfoFromThrift(taskInfoToThrift(expected))
	assert.Equal(t, expected.WorkflowID, actual.WorkflowID)
	assert.Equal(t, expected.RunID, actual.RunID)
	assert.Equal(t, expected.ScheduleID, actual.ScheduleID)
	assert.Equal(t, expected.ExpiryTimestamp.Sub(actual.ExpiryTimestamp), time.Duration(0))
	assert.Equal(t, expected.CreatedTimestamp.Sub(actual.CreatedTimestamp), time.Duration(0))
	assert.Equal(t, expected.PartitionConfig, actual.PartitionConfig)
	assert.Nil(t, taskInfoFromThrift(nil))
	assert.Nil(t, taskInfoToThrift(nil))
}

func TestTaskListInfo(t *testing.T) {
	cases := []*TaskListInfo{
		nil,
		{
			Kind:            0,
			AckLevel:        1,
			ExpiryTimestamp: time.UnixMicro(2),
			LastUpdated:     time.UnixMicro(3),
			AdaptivePartitionConfig: &TaskListPartitionConfig{
				Version:           4,
				NumReadPartitions: 1,
				ReadPartitions: map[int32]*TaskListPartition{
					0: {
						IsolationGroups: []string{"foo"},
					},
				},
				NumWritePartitions: 2,
				WritePartitions: map[int32]*TaskListPartition{
					0: {
						IsolationGroups: []string{"foo"},
					},
					1: {
						IsolationGroups: []string{"bar"},
					},
				},
			},
		},
		{
			Kind:            0,
			AckLevel:        1,
			ExpiryTimestamp: time.UnixMicro(2),
			LastUpdated:     time.UnixMicro(3),
			AdaptivePartitionConfig: &TaskListPartitionConfig{
				Version:            4,
				NumReadPartitions:  10,
				NumWritePartitions: 2,
			},
		},
	}
	for i, info := range cases {
		assert.Equal(t, info, taskListInfoFromThrift(taskListInfoToThrift(info)), "case %d", i)
	}
}

func TestTransferTaskInfo(t *testing.T) {
	expected := &TransferTaskInfo{
		DomainID:                UUID(uuid.New()),
		WorkflowID:              "WorkflowID",
		RunID:                   UUID(uuid.New()),
		TaskType:                int16(rand.Intn(1000)),
		TargetDomainID:          UUID(uuid.New()),
		TargetDomainIDs:         []UUID{UUID(uuid.New()), UUID(uuid.New())},
		TargetWorkflowID:        "TargetWorkflowID",
		TargetRunID:             UUID(uuid.New()),
		TaskList:                "TaskList",
		TargetChildWorkflowOnly: true,
		ScheduleID:              int64(rand.Intn(1000)),
		Version:                 int64(rand.Intn(1000)),
	}
	actual := transferTaskInfoFromThrift(transferTaskInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, transferTaskInfoFromThrift(nil))
	assert.Nil(t, transferTaskInfoToThrift(nil))
}

func TestTimerTaskInfo(t *testing.T) {
	expected := &TimerTaskInfo{
		DomainID:        UUID(uuid.New()),
		WorkflowID:      "WorkflowID",
		RunID:           UUID(uuid.New()),
		TaskType:        int16(rand.Intn(1000)),
		TimeoutType:     common.Int16Ptr(int16(rand.Intn(1000))),
		Version:         int64(rand.Intn(1000)),
		ScheduleAttempt: int64(rand.Intn(1000)),
		EventID:         int64(rand.Intn(1000)),
	}
	actual := timerTaskInfoFromThrift(timerTaskInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, timerTaskInfoFromThrift(nil))
	assert.Nil(t, timerTaskInfoToThrift(nil))
}

func TestReplicationTaskInfo(t *testing.T) {
	expected := &ReplicationTaskInfo{
		DomainID:                UUID(uuid.New()),
		WorkflowID:              "WorkflowID",
		RunID:                   UUID(uuid.New()),
		TaskType:                int16(rand.Intn(1000)),
		Version:                 int64(rand.Intn(1000)),
		FirstEventID:            int64(rand.Intn(1000)),
		NextEventID:             int64(rand.Intn(1000)),
		ScheduledID:             int64(rand.Intn(1000)),
		EventStoreVersion:       int32(rand.Intn(1000)),
		NewRunEventStoreVersion: int32(rand.Intn(1000)),
		BranchToken:             []byte("BranchToken"),
		NewRunBranchToken:       []byte("NewRunBranchToken"),
	}
	actual := replicationTaskInfoFromThrift(replicationTaskInfoToThrift(expected))
	assert.Equal(t, expected, actual)
	assert.Nil(t, replicationTaskInfoFromThrift(nil))
	assert.Nil(t, replicationTaskInfoToThrift(nil))
}

func TestCronOverlapPolicyFromThrift(t *testing.T) {
	cases := []struct {
		thrift *shared.CronOverlapPolicy
		actual types.CronOverlapPolicy
	}{
		{nil, types.CronOverlapPolicySkipped},
		{shared.CronOverlapPolicyBufferone.Ptr(), types.CronOverlapPolicyBufferOne},
		{shared.CronOverlapPolicySkipped.Ptr(), types.CronOverlapPolicySkipped},
	}
	for _, c := range cases {
		assert.Equal(t, c.actual, cronOverlapPolicyFromThrift(c.thrift))
	}
}
