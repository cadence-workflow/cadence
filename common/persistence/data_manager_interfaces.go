// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

// Geneate rate limiter wrappers.
//go:generate mockgen -package $GOPACKAGE -destination data_manager_interfaces_mock.go -self_package github.com/uber/cadence/common/persistence github.com/uber/cadence/common/persistence Task,ShardManager,ExecutionManager,ExecutionManagerFactory,TaskManager,HistoryManager,DomainManager,QueueManager,ConfigStoreManager
//go:generate gowrap gen -g -p . -i ConfigStoreManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/configstore_generated.go
//go:generate gowrap gen -g -p . -i DomainManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/domain_generated.go
//go:generate gowrap gen -g -p . -i HistoryManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/history_generated.go
//go:generate gowrap gen -g -p . -i ExecutionManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/execution_generated.go
//go:generate gowrap gen -g -p . -i QueueManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/queue_generated.go
//go:generate gowrap gen -g -p . -i TaskManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/task_generated.go
//go:generate gowrap gen -g -p . -i ShardManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/shard_generated.go

// Geneate error injector wrappers.
//go:generate gowrap gen -g -p . -i ConfigStoreManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/configstore_generated.go
//go:generate gowrap gen -g -p . -i ShardManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/shard_generated.go
//go:generate gowrap gen -g -p . -i ExecutionManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/execution_generated.go
//go:generate gowrap gen -g -p . -i TaskManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/task_generated.go
//go:generate gowrap gen -g -p . -i HistoryManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/history_generated.go
//go:generate gowrap gen -g -p . -i DomainManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/domain_generated.go
//go:generate gowrap gen -g -p . -i QueueManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/queue_generated.go

// Generate metered wrappers.
//go:generate gowrap gen -g -p . -i ConfigStoreManager -t ./wrappers/templates/metered.tmpl -o wrappers/metered/configstore_generated.go
//go:generate gowrap gen -g -p . -i ShardManager -t ./wrappers/templates/metered.tmpl -o wrappers/metered/shard_generated.go
//go:generate gowrap gen -g -p . -i TaskManager -t ./wrappers/templates/metered.tmpl -o wrappers/metered/task_generated.go
//go:generate gowrap gen -g -p . -i HistoryManager -t ./wrappers/templates/metered.tmpl -o wrappers/metered/history_generated.go
//go:generate gowrap gen -g -p . -i DomainManager -t ./wrappers/templates/metered.tmpl -o wrappers/metered/domain_generated.go
//go:generate gowrap gen -g -p . -i QueueManager -t ./wrappers/templates/metered.tmpl -o wrappers/metered/queue_generated.go

// execution metered wrapper is special
//go:generate gowrap gen -g -p . -i ExecutionManager -t ./wrappers/templates/metered_execution.tmpl -o wrappers/metered/execution_generated.go

package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pborman/uuid"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/types"
)

// Domain status
const (
	DomainStatusRegistered = iota
	DomainStatusDeprecated
	DomainStatusDeleted
)

const (
	// EventStoreVersion is already deprecated, this is used for forward
	// compatibility (so that rollback is possible).
	// TODO we can remove it after fixing all the query templates and when
	// we decide the compatibility is no longer needed.
	EventStoreVersion = 2
)

// CreateWorkflowMode workflow creation mode
type CreateWorkflowMode int

// QueueType is an enum that represents various queue types in persistence
type QueueType int

// Queue types used in queue table
// Use positive numbers for queue type
// Negative numbers are reserved for DLQ
const (
	DomainReplicationQueueType QueueType = iota + 1
)

// Create Workflow Execution Mode
const (
	// Fail if current record exists
	// Only applicable for CreateWorkflowExecution
	CreateWorkflowModeBrandNew CreateWorkflowMode = iota
	// Update current record only if workflow is closed
	// Only applicable for CreateWorkflowExecution
	CreateWorkflowModeWorkflowIDReuse
	// Update current record only if workflow is open
	// Only applicable for UpdateWorkflowExecution
	CreateWorkflowModeContinueAsNew
	// Do not update current record since workflow to
	// applicable for CreateWorkflowExecution, UpdateWorkflowExecution
	CreateWorkflowModeZombie
)

// UpdateWorkflowMode update mode
type UpdateWorkflowMode int

// Update Workflow Execution Mode
const (
	// Update workflow, including current record
	// NOTE: update on current record is a condition update
	UpdateWorkflowModeUpdateCurrent UpdateWorkflowMode = iota
	// Update workflow, without current record
	// NOTE: current record CANNOT point to the workflow to be updated
	UpdateWorkflowModeBypassCurrent
	// Update workflow, ignoring current record
	// NOTE: current record may or may not point to the workflow
	// this mode should only be used for (re-)generating workflow tasks
	// and there's no other changes to the workflow
	UpdateWorkflowModeIgnoreCurrent
)

// ConflictResolveWorkflowMode conflict resolve mode
type ConflictResolveWorkflowMode int

// Conflict Resolve Workflow Mode
const (
	// Conflict resolve workflow, including current record
	// NOTE: update on current record is a condition update
	ConflictResolveWorkflowModeUpdateCurrent ConflictResolveWorkflowMode = iota
	// Conflict resolve workflow, without current record
	// NOTE: current record CANNOT point to the workflow to be updated
	ConflictResolveWorkflowModeBypassCurrent
)

// Workflow execution states
const (
	WorkflowStateCreated = iota
	WorkflowStateRunning
	WorkflowStateCompleted
	WorkflowStateZombie
	WorkflowStateVoid
	WorkflowStateCorrupted
)

// Workflow execution close status
const (
	WorkflowCloseStatusNone = iota
	WorkflowCloseStatusCompleted
	WorkflowCloseStatusFailed
	WorkflowCloseStatusCanceled
	WorkflowCloseStatusTerminated
	WorkflowCloseStatusContinuedAsNew
	WorkflowCloseStatusTimedOut
)

// Types of task lists
const (
	TaskListTypeDecision = iota
	TaskListTypeActivity
)

// Kinds of task lists
const (
	TaskListKindNormal = iota
	TaskListKindSticky
)

// Transfer task types
const (
	TransferTaskTypeDecisionTask = iota
	TransferTaskTypeActivityTask
	TransferTaskTypeCloseExecution
	TransferTaskTypeCancelExecution
	TransferTaskTypeStartChildExecution
	TransferTaskTypeSignalExecution
	TransferTaskTypeRecordWorkflowStarted
	TransferTaskTypeResetWorkflow
	TransferTaskTypeUpsertWorkflowSearchAttributes
	TransferTaskTypeRecordWorkflowClosed
	TransferTaskTypeRecordChildExecutionCompleted
	TransferTaskTypeApplyParentClosePolicy
)

// Deprecated: Types of cross-cluster tasks. These are deprecated as of
// May 2024
const (
	CrossClusterTaskTypeStartChildExecution = iota + 1
	CrossClusterTaskTypeCancelExecution
	CrossClusterTaskTypeSignalExecution
	CrossClusterTaskTypeRecordChildExeuctionCompleted
	CrossClusterTaskTypeApplyParentClosePolicy
)

// Types of replication tasks
const (
	ReplicationTaskTypeHistory = iota
	ReplicationTaskTypeSyncActivity
	ReplicationTaskTypeFailoverMarker
)

// Types of timers
const (
	TaskTypeDecisionTimeout = iota
	TaskTypeActivityTimeout
	TaskTypeUserTimer
	TaskTypeWorkflowTimeout
	TaskTypeDeleteHistoryEvent
	TaskTypeActivityRetryTimer
	TaskTypeWorkflowBackoffTimer
)

// WorkflowRequestType is the type of workflow request
type WorkflowRequestType int

// Types of workflow requests
const (
	WorkflowRequestTypeStart WorkflowRequestType = iota
	WorkflowRequestTypeSignal
	WorkflowRequestTypeCancel
	WorkflowRequestTypeReset
)

// CreateWorkflowRequestMode is the mode of create workflow request
type CreateWorkflowRequestMode int

// Modes of create workflow request
const (
	// Fail if data with the same domain_id, workflow_id, request_id exists
	// It is used for transactions started by external API requests
	// to allow us detecting duplicate requests
	CreateWorkflowRequestModeNew CreateWorkflowRequestMode = iota
	// Upsert the data without checking duplication
	// It is used for transactions started by replication stack to achieve
	// eventual consistency
	CreateWorkflowRequestModeReplicated
)

// UnknownNumRowsAffected is returned when the number of rows that an API affected cannot be determined
const UnknownNumRowsAffected = -1

// Types of workflow backoff timeout
const (
	WorkflowBackoffTimeoutTypeRetry = iota
	WorkflowBackoffTimeoutTypeCron
)

const (
	// InitialFailoverNotificationVersion is the initial failover version for a domain
	InitialFailoverNotificationVersion int64 = 0

	// TransferTaskTransferTargetWorkflowID is the the dummy workflow ID for transfer tasks of types
	// that do not have a target workflow
	TransferTaskTransferTargetWorkflowID = "20000000-0000-f000-f000-000000000001"
	// TransferTaskTransferTargetRunID is the the dummy run ID for transfer tasks of types
	// that do not have a target workflow
	TransferTaskTransferTargetRunID = "30000000-0000-f000-f000-000000000002"
	// Deprecated: This is deprecated as of May 24
	// CrossClusterTaskDefaultTargetRunID is the the dummy run ID for cross-cluster tasks of types
	// that do not have a target workflow
	CrossClusterTaskDefaultTargetRunID = TransferTaskTransferTargetRunID

	// indicate invalid workflow state transition
	invalidStateTransitionMsg = "unable to change workflow state from %v to %v, close status %v"
)

const numItemsInGarbageInfo = 3

type ConfigType int

const (
	DynamicConfig ConfigType = iota
	GlobalIsolationGroupConfig
)

type (
	// ShardInfo describes a shard
	ShardInfo struct {
		ShardID                       int                               `json:"shard_id"`
		Owner                         string                            `json:"owner"`
		RangeID                       int64                             `json:"range_id"`
		StolenSinceRenew              int                               `json:"stolen_since_renew"`
		UpdatedAt                     time.Time                         `json:"updated_at"`
		ReplicationAckLevel           int64                             `json:"replication_ack_level"`
		ReplicationDLQAckLevel        map[string]int64                  `json:"replication_dlq_ack_level"`
		TransferAckLevel              int64                             `json:"transfer_ack_level"`
		TimerAckLevel                 time.Time                         `json:"timer_ack_level"`
		ClusterTransferAckLevel       map[string]int64                  `json:"cluster_transfer_ack_level"`
		ClusterTimerAckLevel          map[string]time.Time              `json:"cluster_timer_ack_level"`
		TransferProcessingQueueStates *types.ProcessingQueueStates      `json:"transfer_processing_queue_states"`
		TimerProcessingQueueStates    *types.ProcessingQueueStates      `json:"timer_processing_queue_states"`
		ClusterReplicationLevel       map[string]int64                  `json:"cluster_replication_level"`
		DomainNotificationVersion     int64                             `json:"domain_notification_version"`
		PendingFailoverMarkers        []*types.FailoverMarkerAttributes `json:"pending_failover_markers"`
	}

	// WorkflowExecutionInfo describes a workflow execution
	WorkflowExecutionInfo struct {
		DomainID                           string
		WorkflowID                         string
		RunID                              string
		FirstExecutionRunID                string
		ParentDomainID                     string
		ParentWorkflowID                   string
		ParentRunID                        string
		InitiatedID                        int64
		CompletionEventBatchID             int64
		CompletionEvent                    *types.HistoryEvent
		TaskList                           string
		WorkflowTypeName                   string
		WorkflowTimeout                    int32
		DecisionStartToCloseTimeout        int32
		ExecutionContext                   []byte
		State                              int
		CloseStatus                        int
		LastFirstEventID                   int64
		LastEventTaskID                    int64
		NextEventID                        int64
		LastProcessedEvent                 int64
		StartTimestamp                     time.Time
		LastUpdatedTimestamp               time.Time
		CreateRequestID                    string
		SignalCount                        int32
		DecisionVersion                    int64
		DecisionScheduleID                 int64
		DecisionStartedID                  int64
		DecisionRequestID                  string
		DecisionTimeout                    int32
		DecisionAttempt                    int64
		DecisionStartedTimestamp           int64
		DecisionScheduledTimestamp         int64
		DecisionOriginalScheduledTimestamp int64
		CancelRequested                    bool
		CancelRequestID                    string
		StickyTaskList                     string
		StickyScheduleToStartTimeout       int32
		ClientLibraryVersion               string
		ClientFeatureVersion               string
		ClientImpl                         string
		AutoResetPoints                    *types.ResetPoints
		Memo                               map[string][]byte
		SearchAttributes                   map[string][]byte
		PartitionConfig                    map[string]string
		// for retry
		Attempt            int32
		HasRetryPolicy     bool
		InitialInterval    int32
		BackoffCoefficient float64
		MaximumInterval    int32
		ExpirationTime     time.Time
		MaximumAttempts    int32
		NonRetriableErrors []string
		BranchToken        []byte
		// Cron
		CronSchedule      string
		IsCron            bool
		ExpirationSeconds int32 // TODO: is this field useful?
	}

	// ExecutionStats is the statistics about workflow execution
	ExecutionStats struct {
		HistorySize int64
	}

	// ReplicationState represents mutable state information for global domains.
	// This information is used by replication protocol when applying events from remote clusters
	// TODO: remove this struct after all 2DC workflows complete
	ReplicationState struct {
		CurrentVersion      int64
		StartVersion        int64
		LastWriteVersion    int64
		LastWriteEventID    int64
		LastReplicationInfo map[string]*ReplicationInfo
	}

	// CurrentWorkflowExecution describes a current execution record
	CurrentWorkflowExecution struct {
		DomainID     string
		WorkflowID   string
		RunID        string
		State        int
		CurrentRunID string
	}

	// TransferTaskInfo describes a transfer task
	TransferTaskInfo struct {
		DomainID                string
		WorkflowID              string
		RunID                   string
		VisibilityTimestamp     time.Time
		TaskID                  int64
		TargetDomainID          string
		TargetDomainIDs         map[string]struct{} // used for ApplyParentPolicy request
		TargetWorkflowID        string
		TargetRunID             string
		TargetChildWorkflowOnly bool
		TaskList                string
		TaskType                int
		ScheduleID              int64
		Version                 int64
		RecordVisibility        bool
	}

	// CrossClusterTaskInfo describes a cross-cluster task
	// Cross cluster tasks are exactly like transfer tasks so
	// instead of creating another struct and duplicating the same
	// logic everywhere. We reuse TransferTaskInfo
	// This is a deprecated feature as of May 24
	CrossClusterTaskInfo = TransferTaskInfo

	// ReplicationTaskInfo describes the replication task created for replication of history events
	ReplicationTaskInfo struct {
		DomainID          string
		WorkflowID        string
		RunID             string
		TaskID            int64
		TaskType          int
		FirstEventID      int64
		NextEventID       int64
		Version           int64
		ScheduledID       int64
		BranchToken       []byte
		NewRunBranchToken []byte
		CreationTime      int64
	}

	// TimerTaskInfo describes a timer task.
	TimerTaskInfo struct {
		DomainID            string
		WorkflowID          string
		RunID               string
		VisibilityTimestamp time.Time
		TaskID              int64
		TaskType            int
		TimeoutType         int
		EventID             int64
		ScheduleAttempt     int64
		Version             int64
	}

	// TaskListInfo describes a state of a task list implementation.
	TaskListInfo struct {
		DomainID                string
		Name                    string
		TaskType                int
		RangeID                 int64
		AckLevel                int64
		Kind                    int
		Expiry                  time.Time
		LastUpdated             time.Time
		AdaptivePartitionConfig *TaskListPartitionConfig
	}

	TaskListPartition struct {
		IsolationGroups []string
	}

	// TaskListPartitionConfig represents the configuration for task list partitions.
	TaskListPartitionConfig struct {
		Version         int64
		ReadPartitions  map[int]*TaskListPartition
		WritePartitions map[int]*TaskListPartition
	}

	// TaskInfo describes either activity or decision task
	TaskInfo struct {
		DomainID                      string
		WorkflowID                    string
		RunID                         string
		TaskID                        int64
		ScheduleID                    int64
		ScheduleToStartTimeoutSeconds int32
		Expiry                        time.Time
		CreatedTime                   time.Time
		PartitionConfig               map[string]string
	}

	// TaskKey gives primary key info for a specific task
	TaskKey struct {
		DomainID     string
		TaskListName string
		TaskType     int
		TaskID       int64
	}

	// ReplicationInfo represents the information stored for last replication event details per cluster
	ReplicationInfo struct {
		Version     int64
		LastEventID int64
	}

	// VersionHistoryItem contains the event id and the associated version
	VersionHistoryItem struct {
		EventID int64
		Version int64
	}

	// VersionHistory provides operations on version history
	VersionHistory struct {
		BranchToken []byte
		Items       []*VersionHistoryItem
	}

	// VersionHistories contains a set of VersionHistory
	VersionHistories struct {
		CurrentVersionHistoryIndex int
		Histories                  []*VersionHistory
	}

	// WorkflowMutableState indicates workflow related state
	WorkflowMutableState struct {
		ActivityInfos       map[int64]*ActivityInfo
		TimerInfos          map[string]*TimerInfo
		ChildExecutionInfos map[int64]*ChildExecutionInfo
		RequestCancelInfos  map[int64]*RequestCancelInfo
		SignalInfos         map[int64]*SignalInfo
		SignalRequestedIDs  map[string]struct{}
		ExecutionInfo       *WorkflowExecutionInfo
		ExecutionStats      *ExecutionStats
		BufferedEvents      []*types.HistoryEvent
		VersionHistories    *VersionHistories
		ReplicationState    *ReplicationState // TODO: remove this after all 2DC workflows complete
		Checksum            checksum.Checksum
	}

	// ActivityInfo details.
	ActivityInfo struct {
		Version                  int64
		ScheduleID               int64
		ScheduledEventBatchID    int64
		ScheduledEvent           *types.HistoryEvent
		ScheduledTime            time.Time
		StartedID                int64
		StartedEvent             *types.HistoryEvent
		StartedTime              time.Time
		DomainID                 string
		ActivityID               string
		RequestID                string
		Details                  []byte
		ScheduleToStartTimeout   int32
		ScheduleToCloseTimeout   int32
		StartToCloseTimeout      int32
		HeartbeatTimeout         int32
		CancelRequested          bool
		CancelRequestID          int64
		LastHeartBeatUpdatedTime time.Time
		TimerTaskStatus          int32
		// For retry
		Attempt            int32
		StartedIdentity    string
		TaskList           string
		HasRetryPolicy     bool
		InitialInterval    int32
		BackoffCoefficient float64
		MaximumInterval    int32
		ExpirationTime     time.Time
		MaximumAttempts    int32
		NonRetriableErrors []string
		LastFailureReason  string
		LastWorkerIdentity string
		LastFailureDetails []byte
		// Not written to database - This is used only for deduping heartbeat timer creation
		LastHeartbeatTimeoutVisibilityInSeconds int64
	}

	// TimerInfo details - metadata about user timer info.
	TimerInfo struct {
		Version    int64
		TimerID    string
		StartedID  int64
		ExpiryTime time.Time
		TaskStatus int64
	}

	// ChildExecutionInfo has details for pending child executions.
	ChildExecutionInfo struct {
		Version               int64
		InitiatedID           int64
		InitiatedEventBatchID int64
		InitiatedEvent        *types.HistoryEvent
		StartedID             int64
		StartedWorkflowID     string
		StartedRunID          string
		StartedEvent          *types.HistoryEvent
		CreateRequestID       string
		DomainID              string
		DomainNameDEPRECATED  string // deprecated: please use DomainID field instead
		WorkflowTypeName      string
		ParentClosePolicy     types.ParentClosePolicy
	}

	// RequestCancelInfo has details for pending external workflow cancellations
	RequestCancelInfo struct {
		Version               int64
		InitiatedEventBatchID int64
		InitiatedID           int64
		CancelRequestID       string
	}

	// SignalInfo has details for pending external workflow signal
	SignalInfo struct {
		Version               int64
		InitiatedEventBatchID int64
		InitiatedID           int64
		SignalRequestID       string
		SignalName            string
		Input                 []byte
		Control               []byte
	}

	// CreateShardRequest is used to create a shard in executions table
	CreateShardRequest struct {
		ShardInfo *ShardInfo
	}

	// GetShardRequest is used to get shard information
	GetShardRequest struct {
		ShardID int
	}

	// GetShardResponse is the response to GetShard
	GetShardResponse struct {
		ShardInfo *ShardInfo
	}

	// UpdateShardRequest is used to update shard information
	UpdateShardRequest struct {
		ShardInfo       *ShardInfo
		PreviousRangeID int64
	}

	// CreateWorkflowExecutionRequest is used to write a new workflow execution
	CreateWorkflowExecutionRequest struct {
		RangeID int64

		Mode CreateWorkflowMode

		PreviousRunID            string
		PreviousLastWriteVersion int64

		NewWorkflowSnapshot WorkflowSnapshot

		WorkflowRequestMode CreateWorkflowRequestMode
		DomainName          string
	}

	// CreateWorkflowExecutionResponse is the response to CreateWorkflowExecutionRequest
	CreateWorkflowExecutionResponse struct {
		MutableStateUpdateSessionStats *MutableStateUpdateSessionStats
	}

	// GetWorkflowExecutionRequest is used to retrieve the info of a workflow execution
	GetWorkflowExecutionRequest struct {
		DomainID   string
		Execution  types.WorkflowExecution
		DomainName string
		RangeID    int64
	}

	// GetWorkflowExecutionResponse is the response to GetworkflowExecutionRequest
	GetWorkflowExecutionResponse struct {
		State             *WorkflowMutableState
		MutableStateStats *MutableStateStats
	}

	// GetCurrentExecutionRequest is used to retrieve the current RunId for an execution
	GetCurrentExecutionRequest struct {
		DomainID   string
		WorkflowID string
		DomainName string
	}

	// ListCurrentExecutionsRequest is request to ListCurrentExecutions
	ListCurrentExecutionsRequest struct {
		PageSize  int
		PageToken []byte
	}

	// ListCurrentExecutionsResponse is the response to ListCurrentExecutionsRequest
	ListCurrentExecutionsResponse struct {
		Executions []*CurrentWorkflowExecution
		PageToken  []byte
	}

	// IsWorkflowExecutionExistsRequest is used to check if the concrete execution exists
	IsWorkflowExecutionExistsRequest struct {
		DomainID   string
		DomainName string
		WorkflowID string
		RunID      string
	}

	// ListConcreteExecutionsRequest is request to ListConcreteExecutions
	ListConcreteExecutionsRequest struct {
		PageSize  int
		PageToken []byte
	}

	// ListConcreteExecutionsResponse is response to ListConcreteExecutions
	ListConcreteExecutionsResponse struct {
		Executions []*ListConcreteExecutionsEntity
		PageToken  []byte
	}

	// ListConcreteExecutionsEntity is a single entity in ListConcreteExecutionsResponse
	ListConcreteExecutionsEntity struct {
		ExecutionInfo    *WorkflowExecutionInfo
		VersionHistories *VersionHistories
	}

	// GetCurrentExecutionResponse is the response to GetCurrentExecution
	GetCurrentExecutionResponse struct {
		StartRequestID   string
		RunID            string
		State            int
		CloseStatus      int
		LastWriteVersion int64
	}

	// IsWorkflowExecutionExistsResponse is the response to IsWorkflowExecutionExists
	IsWorkflowExecutionExistsResponse struct {
		Exists bool
	}

	// UpdateWorkflowExecutionRequest is used to update a workflow execution
	UpdateWorkflowExecutionRequest struct {
		RangeID int64

		Mode UpdateWorkflowMode

		UpdateWorkflowMutation WorkflowMutation

		NewWorkflowSnapshot *WorkflowSnapshot

		WorkflowRequestMode CreateWorkflowRequestMode

		Encoding common.EncodingType // optional binary encoding type

		DomainName string
	}

	// ConflictResolveWorkflowExecutionRequest is used to reset workflow execution state for a single run
	ConflictResolveWorkflowExecutionRequest struct {
		RangeID int64

		Mode ConflictResolveWorkflowMode

		// workflow to be resetted
		ResetWorkflowSnapshot WorkflowSnapshot

		// maybe new workflow
		NewWorkflowSnapshot *WorkflowSnapshot

		// current workflow
		CurrentWorkflowMutation *WorkflowMutation

		WorkflowRequestMode CreateWorkflowRequestMode

		Encoding common.EncodingType // optional binary encoding type

		DomainName string
	}

	// WorkflowEvents is used as generic workflow history events transaction container
	WorkflowEvents struct {
		DomainID    string
		WorkflowID  string
		RunID       string
		BranchToken []byte
		Events      []*types.HistoryEvent
	}

	// WorkflowRequest is used as requestID and it's corresponding failover version container
	WorkflowRequest struct {
		RequestID   string
		Version     int64
		RequestType WorkflowRequestType
	}

	// WorkflowMutation is used as generic workflow execution state mutation
	WorkflowMutation struct {
		ExecutionInfo    *WorkflowExecutionInfo
		ExecutionStats   *ExecutionStats
		VersionHistories *VersionHistories

		UpsertActivityInfos       []*ActivityInfo
		DeleteActivityInfos       []int64
		UpsertTimerInfos          []*TimerInfo
		DeleteTimerInfos          []string
		UpsertChildExecutionInfos []*ChildExecutionInfo
		DeleteChildExecutionInfos []int64
		UpsertRequestCancelInfos  []*RequestCancelInfo
		DeleteRequestCancelInfos  []int64
		UpsertSignalInfos         []*SignalInfo
		DeleteSignalInfos         []int64
		UpsertSignalRequestedIDs  []string
		DeleteSignalRequestedIDs  []string
		NewBufferedEvents         []*types.HistoryEvent
		ClearBufferedEvents       bool

		TransferTasks     []Task
		CrossClusterTasks []Task
		ReplicationTasks  []Task
		TimerTasks        []Task

		WorkflowRequests []*WorkflowRequest

		Condition int64
		Checksum  checksum.Checksum
	}

	// WorkflowSnapshot is used as generic workflow execution state snapshot
	WorkflowSnapshot struct {
		ExecutionInfo    *WorkflowExecutionInfo
		ExecutionStats   *ExecutionStats
		VersionHistories *VersionHistories

		ActivityInfos       []*ActivityInfo
		TimerInfos          []*TimerInfo
		ChildExecutionInfos []*ChildExecutionInfo
		RequestCancelInfos  []*RequestCancelInfo
		SignalInfos         []*SignalInfo
		SignalRequestedIDs  []string

		TransferTasks     []Task
		CrossClusterTasks []Task
		ReplicationTasks  []Task
		TimerTasks        []Task

		WorkflowRequests []*WorkflowRequest

		Condition int64
		Checksum  checksum.Checksum
	}

	// DeleteWorkflowExecutionRequest is used to delete a workflow execution
	DeleteWorkflowExecutionRequest struct {
		DomainID   string
		WorkflowID string
		RunID      string
		DomainName string
	}

	// DeleteCurrentWorkflowExecutionRequest is used to delete the current workflow execution
	DeleteCurrentWorkflowExecutionRequest struct {
		DomainID   string
		WorkflowID string
		RunID      string
		DomainName string
	}

	// GetTransferTasksRequest is used to read tasks from the transfer task queue
	GetTransferTasksRequest struct {
		ReadLevel     int64
		MaxReadLevel  int64
		BatchSize     int
		NextPageToken []byte
	}

	// GetTransferTasksResponse is the response to GetTransferTasksRequest
	GetTransferTasksResponse struct {
		Tasks         []*TransferTaskInfo
		NextPageToken []byte
	}

	// GetCrossClusterTasksRequest is used to read tasks from the cross-cluster task queue
	GetCrossClusterTasksRequest struct {
		TargetCluster string
		ReadLevel     int64
		MaxReadLevel  int64
		BatchSize     int
		NextPageToken []byte
	}

	// GetCrossClusterTasksResponse is the response to GetCrossClusterTasksRequest
	GetCrossClusterTasksResponse struct {
		Tasks         []*CrossClusterTaskInfo
		NextPageToken []byte
	}

	// GetReplicationTasksRequest is used to read tasks from the replication task queue
	GetReplicationTasksRequest struct {
		ReadLevel     int64
		MaxReadLevel  int64
		BatchSize     int
		NextPageToken []byte
	}

	// GetReplicationTasksResponse is the response to GetReplicationTask
	GetReplicationTasksResponse struct {
		Tasks         []*ReplicationTaskInfo
		NextPageToken []byte
	}

	// CompleteTransferTaskRequest is used to complete a task in the transfer task queue
	CompleteTransferTaskRequest struct {
		TaskID int64
	}

	// RangeCompleteTransferTaskRequest is used to complete a range of tasks in the transfer task queue
	RangeCompleteTransferTaskRequest struct {
		ExclusiveBeginTaskID int64
		InclusiveEndTaskID   int64
		PageSize             int
	}

	// RangeCompleteTransferTaskResponse is the response of RangeCompleteTransferTask
	RangeCompleteTransferTaskResponse struct {
		TasksCompleted int
	}

	// CompleteCrossClusterTaskRequest is used to complete a task in the cross-cluster task queue
	CompleteCrossClusterTaskRequest struct {
		TargetCluster string
		TaskID        int64
	}

	// RangeCompleteCrossClusterTaskRequest is used to complete a range of tasks in the cross-cluster task queue
	RangeCompleteCrossClusterTaskRequest struct {
		TargetCluster        string
		ExclusiveBeginTaskID int64
		InclusiveEndTaskID   int64
		PageSize             int
	}

	// RangeCompleteCrossClusterTaskResponse is the response of RangeCompleteCrossClusterTask
	RangeCompleteCrossClusterTaskResponse struct {
		TasksCompleted int
	}

	// CompleteReplicationTaskRequest is used to complete a task in the replication task queue
	CompleteReplicationTaskRequest struct {
		TaskID int64
	}

	// RangeCompleteReplicationTaskRequest is used to complete a range of task in the replication task queue
	RangeCompleteReplicationTaskRequest struct {
		InclusiveEndTaskID int64
		PageSize           int
	}

	// RangeCompleteReplicationTaskResponse is the response of RangeCompleteReplicationTask
	RangeCompleteReplicationTaskResponse struct {
		TasksCompleted int
	}

	// PutReplicationTaskToDLQRequest is used to put a replication task to dlq
	PutReplicationTaskToDLQRequest struct {
		SourceClusterName string
		TaskInfo          *ReplicationTaskInfo
		DomainName        string
	}

	// GetReplicationTasksFromDLQRequest is used to get replication tasks from dlq
	GetReplicationTasksFromDLQRequest struct {
		SourceClusterName string
		GetReplicationTasksRequest
	}

	// GetReplicationDLQSizeRequest is used to get one replication task from dlq
	GetReplicationDLQSizeRequest struct {
		SourceClusterName string
	}

	// DeleteReplicationTaskFromDLQRequest is used to delete replication task from DLQ
	DeleteReplicationTaskFromDLQRequest struct {
		SourceClusterName string
		TaskID            int64
	}

	// RangeDeleteReplicationTaskFromDLQRequest is used to delete replication tasks from DLQ
	RangeDeleteReplicationTaskFromDLQRequest struct {
		SourceClusterName    string
		ExclusiveBeginTaskID int64
		InclusiveEndTaskID   int64
		PageSize             int
	}

	// RangeDeleteReplicationTaskFromDLQResponse is the response of RangeDeleteReplicationTaskFromDLQ
	RangeDeleteReplicationTaskFromDLQResponse struct {
		TasksCompleted int
	}

	// GetReplicationTasksFromDLQResponse is the response for GetReplicationTasksFromDLQ
	GetReplicationTasksFromDLQResponse = GetReplicationTasksResponse

	// GetReplicationDLQSizeResponse is the response for GetReplicationDLQSize
	GetReplicationDLQSizeResponse struct {
		Size int64
	}

	// RangeCompleteTimerTaskRequest is used to complete a range of tasks in the timer task queue
	RangeCompleteTimerTaskRequest struct {
		InclusiveBeginTimestamp time.Time
		ExclusiveEndTimestamp   time.Time
		PageSize                int
	}

	// RangeCompleteTimerTaskResponse is the response of RangeCompleteTimerTask
	RangeCompleteTimerTaskResponse struct {
		TasksCompleted int
	}

	// CompleteTimerTaskRequest is used to complete a task in the timer task queue
	CompleteTimerTaskRequest struct {
		VisibilityTimestamp time.Time
		TaskID              int64
	}

	// LeaseTaskListRequest is used to request lease of a task list
	LeaseTaskListRequest struct {
		DomainID     string
		DomainName   string
		TaskList     string
		TaskType     int
		TaskListKind int
		RangeID      int64
	}

	// LeaseTaskListResponse is response to LeaseTaskListRequest
	LeaseTaskListResponse struct {
		TaskListInfo *TaskListInfo
	}

	GetTaskListRequest struct {
		DomainID   string
		DomainName string
		TaskList   string
		TaskType   int
	}

	GetTaskListResponse struct {
		TaskListInfo *TaskListInfo
	}

	// UpdateTaskListRequest is used to update task list implementation information
	UpdateTaskListRequest struct {
		TaskListInfo *TaskListInfo
		DomainName   string
	}

	// UpdateTaskListResponse is the response to UpdateTaskList
	UpdateTaskListResponse struct {
	}

	// ListTaskListRequest contains the request params needed to invoke ListTaskList API
	ListTaskListRequest struct {
		PageSize  int
		PageToken []byte
	}

	// ListTaskListResponse is the response from ListTaskList API
	ListTaskListResponse struct {
		Items         []TaskListInfo
		NextPageToken []byte
	}

	// DeleteTaskListRequest contains the request params needed to invoke DeleteTaskList API
	DeleteTaskListRequest struct {
		DomainID     string
		DomainName   string
		TaskListName string
		TaskListType int
		RangeID      int64
	}

	GetTaskListSizeRequest struct {
		DomainID     string
		DomainName   string
		TaskListName string
		TaskListType int
		AckLevel     int64
	}

	GetTaskListSizeResponse struct {
		Size int64
	}

	// CreateTasksRequest is used to create a new task for a workflow exectution
	CreateTasksRequest struct {
		TaskListInfo *TaskListInfo
		Tasks        []*CreateTaskInfo
		DomainName   string
	}

	// CreateTaskInfo describes a task to be created in CreateTasksRequest
	CreateTaskInfo struct {
		Data   *TaskInfo
		TaskID int64
	}

	// CreateTasksResponse is the response to CreateTasksRequest
	CreateTasksResponse struct {
	}

	// GetTasksRequest is used to retrieve tasks of a task list
	GetTasksRequest struct {
		DomainID     string
		TaskList     string
		TaskType     int
		ReadLevel    int64  // range exclusive
		MaxReadLevel *int64 // optional: range inclusive when specified
		BatchSize    int
		DomainName   string
	}

	// GetTasksResponse is the response to GetTasksRequests
	GetTasksResponse struct {
		Tasks []*TaskInfo
	}

	// CompleteTaskRequest is used to complete a task
	CompleteTaskRequest struct {
		TaskList   *TaskListInfo
		TaskID     int64
		DomainName string
	}

	// CompleteTasksLessThanRequest contains the request params needed to invoke CompleteTasksLessThan API
	CompleteTasksLessThanRequest struct {
		DomainID     string
		TaskListName string
		TaskType     int
		TaskID       int64 // Tasks less than or equal to this ID will be completed
		Limit        int   // Limit on the max number of tasks that can be completed. Required param
		DomainName   string
	}

	// CompleteTasksLessThanResponse is the response of CompleteTasksLessThan
	CompleteTasksLessThanResponse struct {
		TasksCompleted int
	}

	// GetOrphanTasksRequest contains the request params need to invoke the GetOrphanTasks API
	GetOrphanTasksRequest struct {
		Limit int
	}

	// GetOrphanTasksResponse is the response to GetOrphanTasksRequests
	GetOrphanTasksResponse struct {
		Tasks []*TaskKey
	}

	// GetTimerIndexTasksRequest is the request for GetTimerIndexTasks
	// TODO: replace this with an iterator that can configure min and max index.
	GetTimerIndexTasksRequest struct {
		MinTimestamp  time.Time
		MaxTimestamp  time.Time
		BatchSize     int
		NextPageToken []byte
	}

	// GetTimerIndexTasksResponse is the response for GetTimerIndexTasks
	GetTimerIndexTasksResponse struct {
		Timers        []*TimerTaskInfo
		NextPageToken []byte
	}

	// DomainInfo describes the domain entity
	DomainInfo struct {
		ID          string
		Name        string
		Status      int
		Description string
		OwnerEmail  string
		Data        map[string]string
	}

	// DomainConfig describes the domain configuration
	DomainConfig struct {
		// NOTE: this retention is in days, not in seconds
		Retention                int32
		EmitMetric               bool
		HistoryArchivalStatus    types.ArchivalStatus
		HistoryArchivalURI       string
		VisibilityArchivalStatus types.ArchivalStatus
		VisibilityArchivalURI    string
		BadBinaries              types.BadBinaries
		IsolationGroups          types.IsolationGroupConfiguration
		AsyncWorkflowConfig      types.AsyncWorkflowConfiguration
	}

	// DomainReplicationConfig describes the cross DC domain replication configuration
	DomainReplicationConfig struct {
		ActiveClusterName string
		Clusters          []*ClusterReplicationConfig
	}

	// ClusterReplicationConfig describes the cross DC cluster replication configuration
	ClusterReplicationConfig struct {
		ClusterName string
		// Note: if adding new properties of non-primitive types, remember to update GetCopy()
	}

	// CreateDomainRequest is used to create the domain
	CreateDomainRequest struct {
		Info              *DomainInfo
		Config            *DomainConfig
		ReplicationConfig *DomainReplicationConfig
		IsGlobalDomain    bool
		ConfigVersion     int64
		FailoverVersion   int64
		LastUpdatedTime   int64
	}

	// CreateDomainResponse is the response for CreateDomain
	CreateDomainResponse struct {
		ID string
	}

	// GetDomainRequest is used to read domain
	GetDomainRequest struct {
		ID   string
		Name string
	}

	// GetDomainResponse is the response for GetDomain
	GetDomainResponse struct {
		Info                        *DomainInfo
		Config                      *DomainConfig
		ReplicationConfig           *DomainReplicationConfig
		IsGlobalDomain              bool
		ConfigVersion               int64
		FailoverVersion             int64
		FailoverNotificationVersion int64
		PreviousFailoverVersion     int64
		FailoverEndTime             *int64
		LastUpdatedTime             int64
		NotificationVersion         int64
	}

	// UpdateDomainRequest is used to update domain
	UpdateDomainRequest struct {
		Info                        *DomainInfo
		Config                      *DomainConfig
		ReplicationConfig           *DomainReplicationConfig
		ConfigVersion               int64
		FailoverVersion             int64
		FailoverNotificationVersion int64
		PreviousFailoverVersion     int64
		FailoverEndTime             *int64
		LastUpdatedTime             int64
		NotificationVersion         int64
	}

	// DeleteDomainRequest is used to delete domain entry from domains table
	DeleteDomainRequest struct {
		ID string
	}

	// DeleteDomainByNameRequest is used to delete domain entry from domains_by_name table
	DeleteDomainByNameRequest struct {
		Name string
	}

	// ListDomainsRequest is used to list domains
	ListDomainsRequest struct {
		PageSize      int
		NextPageToken []byte
	}

	// ListDomainsResponse is the response for GetDomain
	ListDomainsResponse struct {
		Domains       []*GetDomainResponse
		NextPageToken []byte
	}

	// GetMetadataResponse is the response for GetMetadata
	GetMetadataResponse struct {
		NotificationVersion int64
	}

	// MutableStateStats is the size stats for MutableState
	MutableStateStats struct {
		// Total size of mutable state
		MutableStateSize int

		// Breakdown of size into more granular stats
		ExecutionInfoSize  int
		ActivityInfoSize   int
		TimerInfoSize      int
		ChildInfoSize      int
		SignalInfoSize     int
		BufferedEventsSize int

		// Item count for various information captured within mutable state
		ActivityInfoCount      int
		TimerInfoCount         int
		ChildInfoCount         int
		SignalInfoCount        int
		RequestCancelInfoCount int
		BufferedEventsCount    int
	}

	// MutableStateUpdateSessionStats is size stats for mutableState updating session
	MutableStateUpdateSessionStats struct {
		MutableStateSize int // Total size of mutable state update

		// Breakdown of mutable state size update for more granular stats
		ExecutionInfoSize  int
		ActivityInfoSize   int
		TimerInfoSize      int
		ChildInfoSize      int
		SignalInfoSize     int
		BufferedEventsSize int

		// Item counts in this session update
		ActivityInfoCount      int
		TimerInfoCount         int
		ChildInfoCount         int
		SignalInfoCount        int
		RequestCancelInfoCount int

		// Deleted item counts in this session update
		DeleteActivityInfoCount      int
		DeleteTimerInfoCount         int
		DeleteChildInfoCount         int
		DeleteSignalInfoCount        int
		DeleteRequestCancelInfoCount int

		TransferTasksCount    int
		CrossClusterTaskCount int
		TimerTasksCount       int
		ReplicationTasksCount int
	}

	// UpdateWorkflowExecutionResponse is response for UpdateWorkflowExecutionRequest
	UpdateWorkflowExecutionResponse struct {
		MutableStateUpdateSessionStats *MutableStateUpdateSessionStats
	}

	// ConflictResolveWorkflowExecutionResponse is response for ConflictResolveWorkflowExecutionRequest
	ConflictResolveWorkflowExecutionResponse struct {
		MutableStateUpdateSessionStats *MutableStateUpdateSessionStats
	}

	// AppendHistoryNodesRequest is used to append a batch of history nodes
	AppendHistoryNodesRequest struct {
		// true if this is the first append request to the branch
		IsNewBranch bool
		// the info for clean up data in background
		Info string
		// The branch to be appended
		BranchToken []byte
		// The batch of events to be appended. The first eventID will become the nodeID of this batch
		Events []*types.HistoryEvent
		// requested TransactionID for this write operation. For the same eventID, the node with larger TransactionID always wins
		TransactionID int64
		// optional binary encoding type
		Encoding common.EncodingType
		// The shard to get history node data
		ShardID *int

		// DomainName to get metrics created with the domain
		DomainName string
	}

	// AppendHistoryNodesResponse is a response to AppendHistoryNodesRequest
	AppendHistoryNodesResponse struct {
		// The data blob that was persisted to database
		DataBlob DataBlob
	}

	// ReadHistoryBranchRequest is used to read a history branch
	ReadHistoryBranchRequest struct {
		// The branch to be read
		BranchToken []byte
		// Get the history nodes from MinEventID. Inclusive.
		MinEventID int64
		// Get the history nodes upto MaxEventID.  Exclusive.
		MaxEventID int64
		// Maximum number of batches of events per page. Not that number of events in a batch >=1, it is not number of events per page.
		// However for a single page, it is also possible that the returned events is less than PageSize (event zero events) due to stale events.
		PageSize int
		// Token to continue reading next page of history append transactions.  Pass in empty slice for first page
		NextPageToken []byte
		// The shard to get history branch data
		ShardID *int

		DomainName string
	}

	// ReadHistoryBranchResponse is the response to ReadHistoryBranchRequest
	ReadHistoryBranchResponse struct {
		// History events
		HistoryEvents []*types.HistoryEvent
		// Token to read next page if there are more events beyond page size.
		// Use this to set NextPageToken on ReadHistoryBranchRequest to read the next page.
		// Empty means we have reached the last page, not need to continue
		NextPageToken []byte
		// Size of history read from store
		Size int
		// the first_event_id of last loaded batch
		LastFirstEventID int64
	}

	// ReadHistoryBranchByBatchResponse is the response to ReadHistoryBranchRequest
	ReadHistoryBranchByBatchResponse struct {
		// History events by batch
		History []*types.History
		// Token to read next page if there are more events beyond page size.
		// Use this to set NextPageToken on ReadHistoryBranchRequest to read the next page.
		// Empty means we have reached the last page, not need to continue
		NextPageToken []byte
		// Size of history read from store
		Size int
		// the first_event_id of last loaded batch
		LastFirstEventID int64
	}

	// ReadRawHistoryBranchResponse is the response to ReadHistoryBranchRequest
	ReadRawHistoryBranchResponse struct {
		// HistoryEventBlobs history event blobs
		HistoryEventBlobs []*DataBlob
		// Token to read next page if there are more events beyond page size.
		// Use this to set NextPageToken on ReadHistoryBranchRequest to read the next page.
		// Empty means we have reached the last page, not need to continue
		NextPageToken []byte
		// Size of history read from store
		Size int
	}

	// ForkHistoryBranchRequest is used to fork a history branch
	ForkHistoryBranchRequest struct {
		// The base branch to fork from
		ForkBranchToken []byte
		// The nodeID to fork from, the new branch will start from ( inclusive ), the base branch will stop at(exclusive)
		// Application must provide a void forking nodeID, it must be a valid nodeID in that branch. A valid nodeID is the firstEventID of a valid batch of events.
		// And ForkNodeID > 1 because forking from 1 doesn't make any sense.
		ForkNodeID int64
		// the info for clean up data in background
		Info string
		// The shard to get history branch data
		ShardID *int
		// DomainName to create metrics for Domain Cost Attribution
		DomainName string
	}

	// ForkHistoryBranchResponse is the response to ForkHistoryBranchRequest
	ForkHistoryBranchResponse struct {
		// branchToken to represent the new branch
		NewBranchToken []byte
	}

	// CompleteForkBranchRequest is used to complete forking
	CompleteForkBranchRequest struct {
		// the new branch returned from ForkHistoryBranchRequest
		BranchToken []byte
		// true means the fork is success, will update the flag, otherwise will delete the new branch
		Success bool
		// The shard to update history branch data
		ShardID *int
	}

	// DeleteHistoryBranchRequest is used to remove a history branch
	DeleteHistoryBranchRequest struct {
		// branch to be deleted
		BranchToken []byte
		// The shard to delete history branch data
		ShardID *int
		// DomainName to generate metrics for Domain Cost Attribution
		DomainName string
	}

	// GetHistoryTreeRequest is used to retrieve branch info of a history tree
	GetHistoryTreeRequest struct {
		// A UUID of a tree
		TreeID string
		// Get data from this shard
		ShardID *int
		// optional: can provide treeID via branchToken if treeID is empty
		BranchToken []byte
		// DomainName to create metrics
		DomainName string
	}

	// HistoryBranchDetail contains detailed information of a branch
	HistoryBranchDetail struct {
		TreeID   string
		BranchID string
		ForkTime time.Time
		Info     string
	}

	// GetHistoryTreeResponse is a response to GetHistoryTreeRequest
	GetHistoryTreeResponse struct {
		// all branches of a tree
		Branches []*workflow.HistoryBranch
	}

	// GetAllHistoryTreeBranchesRequest is a request of GetAllHistoryTreeBranches
	GetAllHistoryTreeBranchesRequest struct {
		// pagination token
		NextPageToken []byte
		// maximum number of branches returned per page
		PageSize int
	}

	// GetAllHistoryTreeBranchesResponse is a response to GetAllHistoryTreeBranches
	GetAllHistoryTreeBranchesResponse struct {
		// pagination token
		NextPageToken []byte
		// all branches of all trees
		Branches []HistoryBranchDetail
	}

	// CreateFailoverMarkersRequest is request to create failover markers
	CreateFailoverMarkersRequest struct {
		RangeID int64
		Markers []*FailoverMarkerTask
	}

	// FetchDynamicConfigResponse is a response to FetchDynamicConfigResponse
	FetchDynamicConfigResponse struct {
		Snapshot *DynamicConfigSnapshot
	}

	// UpdateDynamicConfigRequest is a request to update dynamic config with snapshot
	UpdateDynamicConfigRequest struct {
		Snapshot *DynamicConfigSnapshot
	}

	DynamicConfigSnapshot struct {
		Version int64
		Values  *types.DynamicConfigBlob
	}

	// Closeable is an interface for any entity that supports a close operation to release resources
	Closeable interface {
		Close()
	}

	// ShardManager is used to manage all shards
	ShardManager interface {
		Closeable
		GetName() string
		CreateShard(ctx context.Context, request *CreateShardRequest) error
		GetShard(ctx context.Context, request *GetShardRequest) (*GetShardResponse, error)
		UpdateShard(ctx context.Context, request *UpdateShardRequest) error
	}

	// ExecutionManager is used to manage workflow executions
	ExecutionManager interface {
		Closeable
		GetName() string
		GetShardID() int

		CreateWorkflowExecution(ctx context.Context, request *CreateWorkflowExecutionRequest) (*CreateWorkflowExecutionResponse, error)
		GetWorkflowExecution(ctx context.Context, request *GetWorkflowExecutionRequest) (*GetWorkflowExecutionResponse, error)
		UpdateWorkflowExecution(ctx context.Context, request *UpdateWorkflowExecutionRequest) (*UpdateWorkflowExecutionResponse, error)
		ConflictResolveWorkflowExecution(ctx context.Context, request *ConflictResolveWorkflowExecutionRequest) (*ConflictResolveWorkflowExecutionResponse, error)
		DeleteWorkflowExecution(ctx context.Context, request *DeleteWorkflowExecutionRequest) error
		DeleteCurrentWorkflowExecution(ctx context.Context, request *DeleteCurrentWorkflowExecutionRequest) error
		GetCurrentExecution(ctx context.Context, request *GetCurrentExecutionRequest) (*GetCurrentExecutionResponse, error)
		IsWorkflowExecutionExists(ctx context.Context, request *IsWorkflowExecutionExistsRequest) (*IsWorkflowExecutionExistsResponse, error)

		// Transfer task related methods
		GetTransferTasks(ctx context.Context, request *GetTransferTasksRequest) (*GetTransferTasksResponse, error)
		CompleteTransferTask(ctx context.Context, request *CompleteTransferTaskRequest) error
		RangeCompleteTransferTask(ctx context.Context, request *RangeCompleteTransferTaskRequest) (*RangeCompleteTransferTaskResponse, error)

		// Replication task related methods
		GetReplicationTasks(ctx context.Context, request *GetReplicationTasksRequest) (*GetReplicationTasksResponse, error)
		CompleteReplicationTask(ctx context.Context, request *CompleteReplicationTaskRequest) error
		RangeCompleteReplicationTask(ctx context.Context, request *RangeCompleteReplicationTaskRequest) (*RangeCompleteReplicationTaskResponse, error)
		PutReplicationTaskToDLQ(ctx context.Context, request *PutReplicationTaskToDLQRequest) error
		GetReplicationTasksFromDLQ(ctx context.Context, request *GetReplicationTasksFromDLQRequest) (*GetReplicationTasksFromDLQResponse, error)
		GetReplicationDLQSize(ctx context.Context, request *GetReplicationDLQSizeRequest) (*GetReplicationDLQSizeResponse, error)
		DeleteReplicationTaskFromDLQ(ctx context.Context, request *DeleteReplicationTaskFromDLQRequest) error
		RangeDeleteReplicationTaskFromDLQ(ctx context.Context, request *RangeDeleteReplicationTaskFromDLQRequest) (*RangeDeleteReplicationTaskFromDLQResponse, error)
		CreateFailoverMarkerTasks(ctx context.Context, request *CreateFailoverMarkersRequest) error

		// Timer related methods.
		GetTimerIndexTasks(ctx context.Context, request *GetTimerIndexTasksRequest) (*GetTimerIndexTasksResponse, error)
		CompleteTimerTask(ctx context.Context, request *CompleteTimerTaskRequest) error
		RangeCompleteTimerTask(ctx context.Context, request *RangeCompleteTimerTaskRequest) (*RangeCompleteTimerTaskResponse, error)

		// Scan operations
		ListConcreteExecutions(ctx context.Context, request *ListConcreteExecutionsRequest) (*ListConcreteExecutionsResponse, error)
		ListCurrentExecutions(ctx context.Context, request *ListCurrentExecutionsRequest) (*ListCurrentExecutionsResponse, error)
	}

	// ExecutionManagerFactory creates an instance of ExecutionManager for a given shard
	ExecutionManagerFactory interface {
		Closeable
		NewExecutionManager(shardID int) (ExecutionManager, error)
	}

	// TaskManager is used to manage tasks
	TaskManager interface {
		Closeable
		GetName() string
		LeaseTaskList(ctx context.Context, request *LeaseTaskListRequest) (*LeaseTaskListResponse, error)
		UpdateTaskList(ctx context.Context, request *UpdateTaskListRequest) (*UpdateTaskListResponse, error)
		GetTaskList(ctx context.Context, request *GetTaskListRequest) (*GetTaskListResponse, error)
		ListTaskList(ctx context.Context, request *ListTaskListRequest) (*ListTaskListResponse, error)
		DeleteTaskList(ctx context.Context, request *DeleteTaskListRequest) error
		GetTaskListSize(ctx context.Context, request *GetTaskListSizeRequest) (*GetTaskListSizeResponse, error)
		CreateTasks(ctx context.Context, request *CreateTasksRequest) (*CreateTasksResponse, error)
		GetTasks(ctx context.Context, request *GetTasksRequest) (*GetTasksResponse, error)
		CompleteTask(ctx context.Context, request *CompleteTaskRequest) error
		CompleteTasksLessThan(ctx context.Context, request *CompleteTasksLessThanRequest) (*CompleteTasksLessThanResponse, error)
		GetOrphanTasks(ctx context.Context, request *GetOrphanTasksRequest) (*GetOrphanTasksResponse, error)
	}

	// HistoryManager is used to manager workflow history events
	HistoryManager interface {
		Closeable
		GetName() string

		// The below are history V2 APIs
		// V2 regards history events growing as a tree, decoupled from workflow concepts
		// For Cadence, treeID is new runID, except for fork(reset), treeID will be the runID that it forks from.

		// AppendHistoryNodes add(or override) a batch of nodes to a history branch
		AppendHistoryNodes(ctx context.Context, request *AppendHistoryNodesRequest) (*AppendHistoryNodesResponse, error)
		// ReadHistoryBranch returns history node data for a branch
		ReadHistoryBranch(ctx context.Context, request *ReadHistoryBranchRequest) (*ReadHistoryBranchResponse, error)
		// ReadHistoryBranchByBatch returns history node data for a branch ByBatch
		ReadHistoryBranchByBatch(ctx context.Context, request *ReadHistoryBranchRequest) (*ReadHistoryBranchByBatchResponse, error)
		// ReadRawHistoryBranch returns history node raw data for a branch ByBatch
		// NOTE: this API should only be used by 3+DC
		ReadRawHistoryBranch(ctx context.Context, request *ReadHistoryBranchRequest) (*ReadRawHistoryBranchResponse, error)
		// ForkHistoryBranch forks a new branch from a old branch
		ForkHistoryBranch(ctx context.Context, request *ForkHistoryBranchRequest) (*ForkHistoryBranchResponse, error)
		// DeleteHistoryBranch removes a branch
		// If this is the last branch to delete, it will also remove the root node
		DeleteHistoryBranch(ctx context.Context, request *DeleteHistoryBranchRequest) error
		// GetHistoryTree returns all branch information of a tree
		GetHistoryTree(ctx context.Context, request *GetHistoryTreeRequest) (*GetHistoryTreeResponse, error)
		// GetAllHistoryTreeBranches returns all branches of all trees
		GetAllHistoryTreeBranches(ctx context.Context, request *GetAllHistoryTreeBranchesRequest) (*GetAllHistoryTreeBranchesResponse, error)
	}

	// DomainManager is used to manage metadata CRUD for domain entities
	DomainManager interface {
		Closeable
		GetName() string
		CreateDomain(ctx context.Context, request *CreateDomainRequest) (*CreateDomainResponse, error)
		GetDomain(ctx context.Context, request *GetDomainRequest) (*GetDomainResponse, error)
		UpdateDomain(ctx context.Context, request *UpdateDomainRequest) error
		DeleteDomain(ctx context.Context, request *DeleteDomainRequest) error
		DeleteDomainByName(ctx context.Context, request *DeleteDomainByNameRequest) error
		ListDomains(ctx context.Context, request *ListDomainsRequest) (*ListDomainsResponse, error)
		GetMetadata(ctx context.Context) (*GetMetadataResponse, error)
	}

	// QueueManager is used to manage queue store
	QueueManager interface {
		Closeable
		EnqueueMessage(ctx context.Context, messagePayload []byte) error
		ReadMessages(ctx context.Context, lastMessageID int64, maxCount int) (QueueMessageList, error)
		DeleteMessagesBefore(ctx context.Context, messageID int64) error
		UpdateAckLevel(ctx context.Context, messageID int64, clusterName string) error
		GetAckLevels(ctx context.Context) (map[string]int64, error)
		EnqueueMessageToDLQ(ctx context.Context, messagePayload []byte) error
		ReadMessagesFromDLQ(ctx context.Context, firstMessageID int64, lastMessageID int64, pageSize int, pageToken []byte) ([]*QueueMessage, []byte, error)
		DeleteMessageFromDLQ(ctx context.Context, messageID int64) error
		RangeDeleteMessagesFromDLQ(ctx context.Context, firstMessageID int64, lastMessageID int64) error
		UpdateDLQAckLevel(ctx context.Context, messageID int64, clusterName string) error
		GetDLQAckLevels(ctx context.Context) (map[string]int64, error)
		GetDLQSize(ctx context.Context) (int64, error)
	}

	// QueueMessage is the message that stores in the queue
	QueueMessage struct {
		ID        int64     `json:"message_id"`
		QueueType QueueType `json:"queue_type"`
		Payload   []byte    `json:"message_payload"`
	}

	QueueMessageList []*QueueMessage

	ConfigStoreManager interface {
		Closeable
		FetchDynamicConfig(ctx context.Context, cfgType ConfigType) (*FetchDynamicConfigResponse, error)
		UpdateDynamicConfig(ctx context.Context, request *UpdateDynamicConfigRequest, cfgType ConfigType) error
		// can add functions for config types other than dynamic config
	}
)

// IsTimeoutError check whether error is TimeoutError
func IsTimeoutError(err error) bool {
	_, ok := err.(*TimeoutError)
	return ok
}

// GetTaskID returns the task ID for transfer task
func (t *TransferTaskInfo) GetTaskID() int64 {
	return t.TaskID
}

// GetVersion returns the task version for transfer task
func (t *TransferTaskInfo) GetVersion() int64 {
	return t.Version
}

// GetTaskType returns the task type for transfer task
func (t *TransferTaskInfo) GetTaskType() int {
	return t.TaskType
}

// GetVisibilityTimestamp returns the task type for transfer task
func (t *TransferTaskInfo) GetVisibilityTimestamp() time.Time {
	return t.VisibilityTimestamp
}

// GetWorkflowID returns the workflow ID for transfer task
func (t *TransferTaskInfo) GetWorkflowID() string {
	return t.WorkflowID
}

// GetRunID returns the run ID for transfer task
func (t *TransferTaskInfo) GetRunID() string {
	return t.RunID
}

// GetTargetDomainIDs returns the targetDomainIDs for applyParentPolicy
func (t *TransferTaskInfo) GetTargetDomainIDs() map[string]struct{} {
	return t.TargetDomainIDs
}

// GetDomainID returns the domain ID for transfer task
func (t *TransferTaskInfo) GetDomainID() string {
	return t.DomainID
}

// String returns a string representation for transfer task
func (t *TransferTaskInfo) String() string {
	return fmt.Sprintf("%#v", t)
}

// GetTaskID returns the task ID for replication task
func (t *ReplicationTaskInfo) GetTaskID() int64 {
	return t.TaskID
}

// GetVersion returns the task version for replication task
func (t *ReplicationTaskInfo) GetVersion() int64 {
	return t.Version
}

// GetTaskType returns the task type for replication task
func (t *ReplicationTaskInfo) GetTaskType() int {
	return t.TaskType
}

// GetVisibilityTimestamp returns the task type for replication task
func (t *ReplicationTaskInfo) GetVisibilityTimestamp() time.Time {
	return time.Time{}
}

// GetWorkflowID returns the workflow ID for replication task
func (t *ReplicationTaskInfo) GetWorkflowID() string {
	return t.WorkflowID
}

// GetRunID returns the run ID for replication task
func (t *ReplicationTaskInfo) GetRunID() string {
	return t.RunID
}

// GetDomainID returns the domain ID for replication task
func (t *ReplicationTaskInfo) GetDomainID() string {
	return t.DomainID
}

// GetTaskID returns the task ID for timer task
func (t *TimerTaskInfo) GetTaskID() int64 {
	return t.TaskID
}

// GetVersion returns the task version for timer task
func (t *TimerTaskInfo) GetVersion() int64 {
	return t.Version
}

// GetTaskType returns the task type for timer task
func (t *TimerTaskInfo) GetTaskType() int {
	return t.TaskType
}

// GetVisibilityTimestamp returns the task type for timer task
func (t *TimerTaskInfo) GetVisibilityTimestamp() time.Time {
	return t.VisibilityTimestamp
}

// GetWorkflowID returns the workflow ID for timer task
func (t *TimerTaskInfo) GetWorkflowID() string {
	return t.WorkflowID
}

// GetRunID returns the run ID for timer task
func (t *TimerTaskInfo) GetRunID() string {
	return t.RunID
}

// GetDomainID returns the domain ID for timer task
func (t *TimerTaskInfo) GetDomainID() string {
	return t.DomainID
}

// String returns a string representation for timer task
func (t *TimerTaskInfo) String() string {
	return fmt.Sprintf(
		"{DomainID: %v, WorkflowID: %v, RunID: %v, VisibilityTimestamp: %v, TaskID: %v, TaskType: %v, TimeoutType: %v, EventID: %v, ScheduleAttempt: %v, Version: %v.}",
		t.DomainID, t.WorkflowID, t.RunID, t.VisibilityTimestamp, t.TaskID, t.TaskType, t.TimeoutType, t.EventID, t.ScheduleAttempt, t.Version,
	)
}

// Copy returns a shallow copy of shardInfo
func (s *ShardInfo) Copy() *ShardInfo {
	// TODO: do we really need to deep copy those fields?
	clusterTransferAckLevel := make(map[string]int64)
	for k, v := range s.ClusterTransferAckLevel {
		clusterTransferAckLevel[k] = v
	}
	clusterTimerAckLevel := make(map[string]time.Time)
	for k, v := range s.ClusterTimerAckLevel {
		clusterTimerAckLevel[k] = v
	}
	clusterReplicationLevel := make(map[string]int64)
	for k, v := range s.ClusterReplicationLevel {
		clusterReplicationLevel[k] = v
	}
	replicationDLQAckLevel := make(map[string]int64)
	for k, v := range s.ReplicationDLQAckLevel {
		replicationDLQAckLevel[k] = v
	}
	return &ShardInfo{
		ShardID:                       s.ShardID,
		Owner:                         s.Owner,
		RangeID:                       s.RangeID,
		StolenSinceRenew:              s.StolenSinceRenew,
		ReplicationAckLevel:           s.ReplicationAckLevel,
		TransferAckLevel:              s.TransferAckLevel,
		TimerAckLevel:                 s.TimerAckLevel,
		ClusterTransferAckLevel:       clusterTransferAckLevel,
		ClusterTimerAckLevel:          clusterTimerAckLevel,
		TransferProcessingQueueStates: s.TransferProcessingQueueStates,
		TimerProcessingQueueStates:    s.TimerProcessingQueueStates,
		DomainNotificationVersion:     s.DomainNotificationVersion,
		ClusterReplicationLevel:       clusterReplicationLevel,
		ReplicationDLQAckLevel:        replicationDLQAckLevel,
		PendingFailoverMarkers:        s.PendingFailoverMarkers,
		UpdatedAt:                     s.UpdatedAt,
	}
}

// SerializeClusterConfigs makes an array of *ClusterReplicationConfig serializable
// by flattening them into map[string]interface{}
func SerializeClusterConfigs(replicationConfigs []*ClusterReplicationConfig) []map[string]interface{} {
	seriaizedReplicationConfigs := []map[string]interface{}{}
	for index := range replicationConfigs {
		seriaizedReplicationConfigs = append(seriaizedReplicationConfigs, replicationConfigs[index].serialize())
	}
	return seriaizedReplicationConfigs
}

// DeserializeClusterConfigs creates an array of ClusterReplicationConfigs from an array of map representations
func DeserializeClusterConfigs(replicationConfigs []map[string]interface{}) []*ClusterReplicationConfig {
	deseriaizedReplicationConfigs := []*ClusterReplicationConfig{}
	for index := range replicationConfigs {
		deseriaizedReplicationConfig := &ClusterReplicationConfig{}
		deseriaizedReplicationConfig.deserialize(replicationConfigs[index])
		deseriaizedReplicationConfigs = append(deseriaizedReplicationConfigs, deseriaizedReplicationConfig)
	}

	return deseriaizedReplicationConfigs
}

func (config *ClusterReplicationConfig) serialize() map[string]interface{} {
	output := make(map[string]interface{})
	output["cluster_name"] = config.ClusterName
	return output
}

func (config *ClusterReplicationConfig) deserialize(input map[string]interface{}) {
	config.ClusterName = input["cluster_name"].(string)
}

// GetCopy return a copy of ClusterReplicationConfig
func (config *ClusterReplicationConfig) GetCopy() *ClusterReplicationConfig {
	res := *config
	return &res
}

// DBTimestampToUnixNano converts Milliseconds timestamp to UnixNano
func DBTimestampToUnixNano(milliseconds int64) int64 {
	return milliseconds * 1000 * 1000 // Milliseconds are 10⁻³, nanoseconds are 10⁻⁹, (-3) - (-9) = 6, so multiply by 10⁶
}

// UnixNanoToDBTimestamp converts UnixNano to Milliseconds timestamp
func UnixNanoToDBTimestamp(timestamp int64) int64 {
	return timestamp / (1000 * 1000) // Milliseconds are 10⁻³, nanoseconds are 10⁻⁹, (-9) - (-3) = -6, so divide by 10⁶
}

var internalThriftEncoder = codec.NewThriftRWEncoder()

// NewHistoryBranchToken return a new branch token
func NewHistoryBranchToken(treeID string) ([]byte, error) {
	branchID := uuid.New()
	bi := &workflow.HistoryBranch{
		TreeID:    &treeID,
		BranchID:  &branchID,
		Ancestors: []*workflow.HistoryBranchRange{},
	}
	token, err := internalThriftEncoder.Encode(bi)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// NewHistoryBranchTokenByBranchID return a new branch token with treeID/branchID
func NewHistoryBranchTokenByBranchID(treeID, branchID string) ([]byte, error) {
	bi := &workflow.HistoryBranch{
		TreeID:    &treeID,
		BranchID:  &branchID,
		Ancestors: []*workflow.HistoryBranchRange{},
	}
	token, err := internalThriftEncoder.Encode(bi)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// NewHistoryBranchTokenFromAnother make up a branchToken
func NewHistoryBranchTokenFromAnother(branchID string, anotherToken []byte) ([]byte, error) {
	var branch workflow.HistoryBranch
	err := internalThriftEncoder.Decode(anotherToken, &branch)
	if err != nil {
		return nil, err
	}

	bi := &workflow.HistoryBranch{
		TreeID:    branch.TreeID,
		BranchID:  &branchID,
		Ancestors: []*workflow.HistoryBranchRange{},
	}
	token, err := internalThriftEncoder.Encode(bi)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// BuildHistoryGarbageCleanupInfo combine the workflow identity information into a string
func BuildHistoryGarbageCleanupInfo(domainID, workflowID, runID string) string {
	return fmt.Sprintf("%v:%v:%v", domainID, workflowID, runID)
}

// SplitHistoryGarbageCleanupInfo returns workflow identity information
func SplitHistoryGarbageCleanupInfo(info string) (domainID, workflowID, runID string, err error) {
	ss := strings.Split(info, ":")
	// workflowID can contain ":" so len(ss) can be greater than 3
	if len(ss) < numItemsInGarbageInfo {
		return "", "", "", fmt.Errorf("not able to split info for  %s", info)
	}
	domainID = ss[0]
	runID = ss[len(ss)-1]
	workflowEnd := len(info) - len(runID) - 1
	workflowID = info[len(domainID)+1 : workflowEnd]
	return
}

// NewGetReplicationTasksFromDLQRequest creates a new GetReplicationTasksFromDLQRequest
func NewGetReplicationTasksFromDLQRequest(
	sourceClusterName string,
	readLevel int64,
	maxReadLevel int64,
	batchSize int,
	nextPageToken []byte,
) *GetReplicationTasksFromDLQRequest {
	return &GetReplicationTasksFromDLQRequest{
		SourceClusterName: sourceClusterName,
		GetReplicationTasksRequest: GetReplicationTasksRequest{
			ReadLevel:     readLevel,
			MaxReadLevel:  maxReadLevel,
			BatchSize:     batchSize,
			NextPageToken: nextPageToken,
		},
	}
}

// IsTransientError checks if the error is a transient persistence error
func IsTransientError(err error) bool {
	switch err.(type) {
	case *types.InternalServiceError, *types.ServiceBusyError, *TimeoutError:
		return true
	}

	return false
}

// IsBackgroundTransientError checks if the error is a transient error on background jobs
func IsBackgroundTransientError(err error) bool {
	switch err.(type) {
	case *types.InternalServiceError, *TimeoutError:
		return true
	}

	return false
}

// HasMoreRowsToDelete checks if there is more data need to be deleted
func HasMoreRowsToDelete(rowsDeleted, batchSize int) bool {
	if rowsDeleted < batchSize || // all target tasks are deleted
		rowsDeleted == UnknownNumRowsAffected || // underlying database does not support rows affected, so pageSize is not honored and all target tasks are deleted
		rowsDeleted > batchSize { // pageSize is not honored and all tasks are deleted
		return false
	}
	return true
}

func (e *WorkflowExecutionInfo) CopyMemo() map[string][]byte {
	if e.Memo == nil {
		return nil
	}
	memo := make(map[string][]byte)
	for k, v := range e.Memo {
		val := make([]byte, len(v))
		copy(val, v)
		memo[k] = val
	}
	return memo
}

func (e *WorkflowExecutionInfo) CopySearchAttributes() map[string][]byte {
	if e.SearchAttributes == nil {
		return nil
	}
	searchAttr := make(map[string][]byte)
	for k, v := range e.SearchAttributes {
		val := make([]byte, len(v))
		copy(val, v)
		searchAttr[k] = val
	}
	return searchAttr
}

func (e *WorkflowExecutionInfo) CopyPartitionConfig() map[string]string {
	if e.PartitionConfig == nil {
		return nil
	}
	partitionConfig := make(map[string]string)
	for k, v := range e.PartitionConfig {
		partitionConfig[k] = v
	}
	return partitionConfig
}

func (p *TaskListPartitionConfig) ToInternalType() *types.TaskListPartitionConfig {
	if p == nil {
		return nil
	}
	var readPartitions map[int]*types.TaskListPartition
	if p.ReadPartitions != nil {
		readPartitions = make(map[int]*types.TaskListPartition, len(p.ReadPartitions))
		for id, par := range p.ReadPartitions {
			readPartitions[id] = par.ToInternalType()
		}
	}
	var writePartitions map[int]*types.TaskListPartition
	if p.WritePartitions != nil {
		writePartitions = make(map[int]*types.TaskListPartition, len(p.WritePartitions))
		for id, par := range p.WritePartitions {
			writePartitions[id] = par.ToInternalType()
		}
	}

	return &types.TaskListPartitionConfig{
		Version:         p.Version,
		ReadPartitions:  readPartitions,
		WritePartitions: writePartitions,
	}
}

func (p *TaskListPartition) ToInternalType() *types.TaskListPartition {
	if p == nil {
		return nil
	}
	return &types.TaskListPartition{IsolationGroups: p.IsolationGroups}
}
