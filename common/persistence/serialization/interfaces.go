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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go -self_package github.com/uber/cadence/common/persistence/serialization

package serialization

import (
	"time"

	"go.uber.org/thriftrw/protocol/stream"
	"go.uber.org/thriftrw/wire"

	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// ShardInfo blob in a serialization agnostic format
	ShardInfo struct {
		StolenSinceRenew                          int32
		UpdatedAt                                 time.Time
		ReplicationAckLevel                       int64
		TransferAckLevel                          int64
		TimerAckLevel                             time.Time
		DomainNotificationVersion                 int64
		ClusterTransferAckLevel                   map[string]int64
		ClusterTimerAckLevel                      map[string]time.Time
		Owner                                     string
		ClusterReplicationLevel                   map[string]int64
		PendingFailoverMarkers                    []byte
		PendingFailoverMarkersEncoding            string
		ReplicationDlqAckLevel                    map[string]int64
		TransferProcessingQueueStates             []byte
		TransferProcessingQueueStatesEncoding     string
		CrossClusterProcessingQueueStates         []byte
		CrossClusterProcessingQueueStatesEncoding string
		TimerProcessingQueueStates                []byte
		TimerProcessingQueueStatesEncoding        string
		QueueStates                               map[int32]*types.QueueState
	}

	// DomainInfo blob in a serialization agnostic format
	DomainInfo struct {
		Name                         string // TODO: This field seems not to be required. We already store domain name in another column.
		Description                  string
		Owner                        string
		Status                       int32
		Retention                    time.Duration
		EmitMetric                   bool
		ArchivalBucket               string
		ArchivalStatus               int16
		ConfigVersion                int64
		NotificationVersion          int64
		FailoverNotificationVersion  int64
		FailoverVersion              int64
		ActiveClusterName            string
		ActiveClustersConfig         []byte
		ActiveClustersConfigEncoding string
		Clusters                     []string
		Data                         map[string]string
		BadBinaries                  []byte
		BadBinariesEncoding          string
		HistoryArchivalStatus        int16
		HistoryArchivalURI           string
		VisibilityArchivalStatus     int16
		VisibilityArchivalURI        string
		FailoverEndTimestamp         *time.Time // TODO: There is logic checking if it's nil, should revisit this
		PreviousFailoverVersion      int64
		LastUpdatedTimestamp         time.Time
		IsolationGroups              []byte
		IsolationGroupsEncoding      string
		AsyncWorkflowConfig          []byte
		AsyncWorkflowConfigEncoding  string
	}

	// HistoryBranchRange blob in a serialization agnostic format
	HistoryBranchRange struct {
		BranchID    string
		BeginNodeID int64
		EndNodeID   int64
	}

	// HistoryTreeInfo blob in a serialization agnostic format
	HistoryTreeInfo struct {
		CreatedTimestamp time.Time
		Ancestors        []*types.HistoryBranchRange
		Info             string
	}

	// WorkflowExecutionInfo blob in a serialization agnostic format
	WorkflowExecutionInfo struct {
		ParentDomainID                       UUID
		ParentWorkflowID                     string
		ParentRunID                          UUID
		InitiatedID                          int64
		CompletionEventBatchID               *int64 // TODO: This is not updated because of backward compatibility issue. Should revisit it later.
		CompletionEvent                      []byte
		CompletionEventEncoding              string
		TaskList                             string
		TaskListKind                         types.TaskListKind
		IsCron                               bool
		WorkflowTypeName                     string
		WorkflowTimeout                      time.Duration
		DecisionTaskTimeout                  time.Duration
		ExecutionContext                     []byte
		State                                int32
		CloseStatus                          int32
		StartVersion                         int64
		LastWriteEventID                     *int64 // TODO: We have logic checking if LastWriteEventID != nil. The field seems to be deprecated. Should revisit it later.
		LastEventTaskID                      int64
		LastFirstEventID                     int64
		LastProcessedEvent                   int64
		StartTimestamp                       time.Time
		LastUpdatedTimestamp                 time.Time
		DecisionVersion                      int64
		DecisionScheduleID                   int64
		DecisionStartedID                    int64
		DecisionTimeout                      time.Duration
		DecisionAttempt                      int64
		DecisionStartedTimestamp             time.Time
		DecisionScheduledTimestamp           time.Time
		CancelRequested                      bool
		DecisionOriginalScheduledTimestamp   time.Time
		CreateRequestID                      string
		DecisionRequestID                    string
		CancelRequestID                      string
		StickyTaskList                       string
		StickyScheduleToStartTimeout         time.Duration
		RetryAttempt                         int64
		RetryInitialInterval                 time.Duration
		RetryMaximumInterval                 time.Duration
		RetryMaximumAttempts                 int32
		RetryExpiration                      time.Duration
		RetryBackoffCoefficient              float64
		RetryExpirationTimestamp             time.Time
		RetryNonRetryableErrors              []string
		HasRetryPolicy                       bool
		CronSchedule                         string
		CronOverlapPolicy                    types.CronOverlapPolicy
		EventStoreVersion                    int32
		EventBranchToken                     []byte
		SignalCount                          int64
		HistorySize                          int64
		ClientLibraryVersion                 string
		ClientFeatureVersion                 string
		ClientImpl                           string
		AutoResetPoints                      []byte
		AutoResetPointsEncoding              string
		SearchAttributes                     map[string][]byte
		Memo                                 map[string][]byte
		VersionHistories                     []byte
		VersionHistoriesEncoding             string
		FirstExecutionRunID                  UUID
		PartitionConfig                      map[string]string
		Checksum                             []byte
		ChecksumEncoding                     string
		ActiveClusterSelectionPolicy         []byte
		ActiveClusterSelectionPolicyEncoding string
	}

	// ActivityInfo blob in a serialization agnostic format
	ActivityInfo struct {
		Version                  int64
		ScheduledEventBatchID    int64
		ScheduledEvent           []byte
		ScheduledEventEncoding   string
		ScheduledTimestamp       time.Time
		StartedID                int64
		StartedEvent             []byte
		StartedEventEncoding     string
		StartedTimestamp         time.Time
		ActivityID               string
		RequestID                string
		ScheduleToStartTimeout   time.Duration
		ScheduleToCloseTimeout   time.Duration
		StartToCloseTimeout      time.Duration
		HeartbeatTimeout         time.Duration
		CancelRequested          bool
		CancelRequestID          int64
		TimerTaskStatus          int32
		Attempt                  int32
		TaskList                 string
		TaskListKind             types.TaskListKind
		StartedIdentity          string
		HasRetryPolicy           bool
		RetryInitialInterval     time.Duration
		RetryMaximumInterval     time.Duration
		RetryMaximumAttempts     int32
		RetryExpirationTimestamp time.Time
		RetryBackoffCoefficient  float64
		RetryNonRetryableErrors  []string
		RetryLastFailureReason   string
		RetryLastWorkerIdentity  string
		RetryLastFailureDetails  []byte
	}

	// ChildExecutionInfo blob in a serialization agnostic format
	ChildExecutionInfo struct {
		Version                int64
		InitiatedEventBatchID  int64
		StartedID              int64
		InitiatedEvent         []byte
		InitiatedEventEncoding string
		StartedWorkflowID      string
		StartedRunID           UUID
		StartedEvent           []byte
		StartedEventEncoding   string
		CreateRequestID        string
		DomainID               string
		DomainNameDEPRECATED   string
		WorkflowTypeName       string
		ParentClosePolicy      int32
	}

	// SignalInfo blob in a serialization agnostic format
	SignalInfo struct {
		Version               int64
		InitiatedEventBatchID int64
		RequestID             string
		Name                  string
		Input                 []byte
		Control               []byte
	}

	// RequestCancelInfo blob in a serialization agnostic format
	RequestCancelInfo struct {
		Version               int64
		InitiatedEventBatchID int64
		CancelRequestID       string
	}

	// TimerInfo blob in a serialization agnostic format
	TimerInfo struct {
		Version         int64
		StartedID       int64
		ExpiryTimestamp time.Time
		TaskID          int64
	}

	// TaskInfo blob in a serialization agnostic format
	TaskInfo struct {
		WorkflowID       string
		RunID            UUID
		ScheduleID       int64
		ExpiryTimestamp  time.Time
		CreatedTimestamp time.Time
		PartitionConfig  map[string]string
	}

	TaskListPartition struct {
		IsolationGroups []string
	}

	TaskListPartitionConfig struct {
		Version            int64
		NumReadPartitions  int32
		NumWritePartitions int32
		ReadPartitions     map[int32]*TaskListPartition
		WritePartitions    map[int32]*TaskListPartition
	}
	// TaskListInfo blob in a serialization agnostic format
	TaskListInfo struct {
		Kind                    int16
		AckLevel                int64
		ExpiryTimestamp         time.Time
		LastUpdated             time.Time
		AdaptivePartitionConfig *TaskListPartitionConfig
	}

	// TransferTaskInfo blob in a serialization agnostic format
	TransferTaskInfo struct {
		DomainID                UUID
		WorkflowID              string
		RunID                   UUID
		TaskType                int16
		TargetDomainID          UUID
		TargetDomainIDs         []UUID
		TargetWorkflowID        string
		TargetRunID             UUID
		TaskList                string
		TargetChildWorkflowOnly bool
		ScheduleID              int64
		Version                 int64
		VisibilityTimestamp     time.Time
	}

	// CrossClusterTaskInfo blob in a serialization agnostic format
	// Cross cluster tasks are exactly like transfer tasks so
	// instead of creating another struct and duplicating the same
	// logic everywhere. We reuse TransferTaskInfo
	CrossClusterTaskInfo         = TransferTaskInfo
	sqlblobsCrossClusterTaskInfo = sqlblobs.TransferTaskInfo

	// TimerTaskInfo blob in a serialization agnostic format
	TimerTaskInfo struct {
		DomainID        UUID
		WorkflowID      string
		RunID           UUID
		TaskType        int16
		TimeoutType     *int16 // TODO: The default value for TimeoutType doesn't make sense. No equivalent value for nil.
		Version         int64
		ScheduleAttempt int64
		EventID         int64
	}

	// ReplicationTaskInfo blob in a serialization agnostic format
	ReplicationTaskInfo struct {
		DomainID                UUID
		WorkflowID              string
		RunID                   UUID
		TaskType                int16
		Version                 int64
		FirstEventID            int64
		NextEventID             int64
		ScheduledID             int64
		EventStoreVersion       int32
		NewRunEventStoreVersion int32
		BranchToken             []byte
		NewRunBranchToken       []byte
		CreationTimestamp       time.Time
	}
)

type (
	// Parser is used to do serialization and deserialization. A parser is backed by a
	// a single encoder which encodes into one format and a collection of decoders.
	// Parser selects the appropriate decoder for the provided blob.
	Parser interface {
		ShardInfoToBlob(*ShardInfo) (persistence.DataBlob, error)
		DomainInfoToBlob(*DomainInfo) (persistence.DataBlob, error)
		HistoryTreeInfoToBlob(*HistoryTreeInfo) (persistence.DataBlob, error)
		WorkflowExecutionInfoToBlob(*WorkflowExecutionInfo) (persistence.DataBlob, error)
		ActivityInfoToBlob(*ActivityInfo) (persistence.DataBlob, error)
		ChildExecutionInfoToBlob(*ChildExecutionInfo) (persistence.DataBlob, error)
		SignalInfoToBlob(*SignalInfo) (persistence.DataBlob, error)
		RequestCancelInfoToBlob(*RequestCancelInfo) (persistence.DataBlob, error)
		TimerInfoToBlob(*TimerInfo) (persistence.DataBlob, error)
		TaskInfoToBlob(*TaskInfo) (persistence.DataBlob, error)
		TaskListInfoToBlob(*TaskListInfo) (persistence.DataBlob, error)
		TransferTaskInfoToBlob(*TransferTaskInfo) (persistence.DataBlob, error)
		CrossClusterTaskInfoToBlob(*CrossClusterTaskInfo) (persistence.DataBlob, error)
		TimerTaskInfoToBlob(*TimerTaskInfo) (persistence.DataBlob, error)
		ReplicationTaskInfoToBlob(*ReplicationTaskInfo) (persistence.DataBlob, error)

		ShardInfoFromBlob([]byte, string) (*ShardInfo, error)
		DomainInfoFromBlob([]byte, string) (*DomainInfo, error)
		HistoryTreeInfoFromBlob([]byte, string) (*HistoryTreeInfo, error)
		WorkflowExecutionInfoFromBlob([]byte, string) (*WorkflowExecutionInfo, error)
		ActivityInfoFromBlob([]byte, string) (*ActivityInfo, error)
		ChildExecutionInfoFromBlob([]byte, string) (*ChildExecutionInfo, error)
		SignalInfoFromBlob([]byte, string) (*SignalInfo, error)
		RequestCancelInfoFromBlob([]byte, string) (*RequestCancelInfo, error)
		TimerInfoFromBlob([]byte, string) (*TimerInfo, error)
		TaskInfoFromBlob([]byte, string) (*TaskInfo, error)
		TaskListInfoFromBlob([]byte, string) (*TaskListInfo, error)
		TransferTaskInfoFromBlob([]byte, string) (*TransferTaskInfo, error)
		CrossClusterTaskInfoFromBlob([]byte, string) (*CrossClusterTaskInfo, error)
		TimerTaskInfoFromBlob([]byte, string) (*TimerTaskInfo, error)
		ReplicationTaskInfoFromBlob([]byte, string) (*ReplicationTaskInfo, error)
	}

	// encoder is used to serialize structs. Each encoder implementation uses one serialization format.
	encoder interface {
		shardInfoToBlob(*ShardInfo) ([]byte, error)
		domainInfoToBlob(*DomainInfo) ([]byte, error)
		historyTreeInfoToBlob(*HistoryTreeInfo) ([]byte, error)
		workflowExecutionInfoToBlob(*WorkflowExecutionInfo) ([]byte, error)
		activityInfoToBlob(*ActivityInfo) ([]byte, error)
		childExecutionInfoToBlob(*ChildExecutionInfo) ([]byte, error)
		signalInfoToBlob(*SignalInfo) ([]byte, error)
		requestCancelInfoToBlob(*RequestCancelInfo) ([]byte, error)
		timerInfoToBlob(*TimerInfo) ([]byte, error)
		taskInfoToBlob(*TaskInfo) ([]byte, error)
		taskListInfoToBlob(*TaskListInfo) ([]byte, error)
		transferTaskInfoToBlob(*TransferTaskInfo) ([]byte, error)
		crossClusterTaskInfoToBlob(*CrossClusterTaskInfo) ([]byte, error)
		timerTaskInfoToBlob(*TimerTaskInfo) ([]byte, error)
		replicationTaskInfoToBlob(*ReplicationTaskInfo) ([]byte, error)
		encodingType() constants.EncodingType
	}

	// decoder is used to deserialize structs. Each decoder implementation uses one serialization format.
	decoder interface {
		shardInfoFromBlob([]byte) (*ShardInfo, error)
		domainInfoFromBlob([]byte) (*DomainInfo, error)
		historyTreeInfoFromBlob([]byte) (*HistoryTreeInfo, error)
		workflowExecutionInfoFromBlob([]byte) (*WorkflowExecutionInfo, error)
		activityInfoFromBlob([]byte) (*ActivityInfo, error)
		childExecutionInfoFromBlob([]byte) (*ChildExecutionInfo, error)
		signalInfoFromBlob([]byte) (*SignalInfo, error)
		requestCancelInfoFromBlob([]byte) (*RequestCancelInfo, error)
		timerInfoFromBlob([]byte) (*TimerInfo, error)
		taskInfoFromBlob([]byte) (*TaskInfo, error)
		taskListInfoFromBlob([]byte) (*TaskListInfo, error)
		transferTaskInfoFromBlob([]byte) (*TransferTaskInfo, error)
		crossClusterTaskInfoFromBlob([]byte) (*CrossClusterTaskInfo, error)
		timerTaskInfoFromBlob([]byte) (*TimerTaskInfo, error)
		replicationTaskInfoFromBlob([]byte) (*ReplicationTaskInfo, error)
	}

	thriftRWType interface {
		ToWire() (wire.Value, error)
		FromWire(w wire.Value) error
		Encode(stream.Writer) error
		Decode(stream.Reader) error
	}
)
