// Copyright (c) 2021 Uber Technologies, Inc.
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

package nosqlplugin

import (
	"time"

	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// WorkflowExecution stores workflow execution metadata
	WorkflowExecution = persistence.InternalWorkflowMutableState

	// WorkflowExecutionRequest is for creating/updating a workflow execution
	WorkflowExecutionRequest struct {
		// basic information/data
		persistence.InternalWorkflowExecutionInfo
		VersionHistories *persistence.DataBlob
		Checksums        *checksum.Checksum
		LastWriteVersion int64
		CurrentTimeStamp time.Time
		// condition checking for updating execution info
		PreviousNextEventIDCondition *int64

		// MapsWriteMode controls how to write into the six maps(activityInfoMap, timerInfoMap, childWorkflowInfoMap, signalInfoMap and signalRequestedIDs)
		MapsWriteMode WorkflowExecutionMapsWriteMode

		// For WorkflowExecutionMapsWriteMode of create, update and reset
		ActivityInfos      map[int64]*persistence.InternalActivityInfo
		TimerInfos         map[string]*persistence.TimerInfo
		WorkflowTimerTasks *persistence.DataBlob
		ChildWorkflowInfos map[int64]*persistence.InternalChildExecutionInfo
		RequestCancelInfos map[int64]*persistence.RequestCancelInfo
		SignalInfos        map[int64]*persistence.SignalInfo
		SignalRequestedIDs []string // This map has no value, hence use array to store keys

		// For WorkflowExecutionMapsWriteMode of update only
		ActivityInfoKeysToDelete       []int64
		TimerInfoKeysToDelete          []string
		ChildWorkflowInfoKeysToDelete  []int64
		RequestCancelInfoKeysToDelete  []int64
		SignalInfoKeysToDelete         []int64
		SignalRequestedIDsKeysToDelete []string

		// EventBufferWriteMode controls how to write into the buffered event list
		// only needed for UpdateWorkflowExecutionWithTasks API
		EventBufferWriteMode EventBufferWriteMode
		// the batch of event to be appended, only for EventBufferWriteModeAppend
		NewBufferedEventBatch *persistence.DataBlob
	}

	// WorkflowExecutionMapsWriteMode controls how to write WorkflowExecutionMaps
	WorkflowExecutionMapsWriteMode int
	// EventBufferWriteMode controls how to write EventBuffer
	EventBufferWriteMode int

	// TimerTask is background timer task
	TimerTask = persistence.TimerTaskInfo

	// ReplicationTask is for replication
	ReplicationTask = persistence.InternalReplicationTaskInfo

	// CrossClusterTask is for cross cluster transfer task
	CrossClusterTask struct {
		TransferTask
		TargetCluster string
	}

	// TransferTask is for regular transfer task
	TransferTask = persistence.TransferTaskInfo

	HistoryMigrationTask struct {
		Transfer      *TransferTask
		Timer         *TimerTask
		Replication   *ReplicationTask
		Task          *persistence.DataBlob
		TaskID        int64
		ScheduledTime time.Time
	}

	// ShardCondition is the condition for making changes within a shard
	ShardCondition struct {
		ShardID int
		RangeID int64
	}

	// CurrentWorkflowWriteRequest is for insert/update current_workflow record
	CurrentWorkflowWriteRequest struct {
		WriteMode CurrentWorkflowWriteMode
		Row       CurrentWorkflowRow
		Condition *CurrentWorkflowWriteCondition
	}

	// CurrentWorkflowWriteCondition is the condition for updating current_workflow record
	CurrentWorkflowWriteCondition struct {
		CurrentRunID     *string
		LastWriteVersion *int64
		State            *int
	}

	// CurrentWorkflowWriteMode controls how to write current_workflow
	CurrentWorkflowWriteMode int

	// CurrentWorkflowRow is the current_workflow row
	CurrentWorkflowRow struct {
		ShardID          int
		DomainID         string
		WorkflowID       string
		RunID            string
		State            int
		CloseStatus      int
		CreateRequestID  string
		LastWriteVersion int64
	}

	// WorkflowRequestRow is the request which has been applied to a workflow
	WorkflowRequestRow struct {
		ShardID     int
		DomainID    string
		WorkflowID  string
		RequestType persistence.WorkflowRequestType
		RequestID   string
		Version     int64
		RunID       string
	}

	WorkflowRequestWriteMode int

	WorkflowRequestsWriteRequest struct {
		Rows      []*WorkflowRequestRow
		WriteMode WorkflowRequestWriteMode
	}

	ActiveClusterSelectionPolicyRow struct {
		ShardID    int
		DomainID   string
		WorkflowID string
		RunID      string
		Policy     *persistence.DataBlob
	}

	// TasksFilter is for filtering tasks
	TasksFilter struct {
		TaskListFilter
		// Exclusive
		MinTaskID int64
		// Inclusive
		MaxTaskID int64
		BatchSize int
	}

	// TaskRowForInsert is the struct to inserting task
	TaskRowForInsert struct {
		TaskRow
		// <= 0 means no TTL
		TTLSeconds int
	}

	// TaskRow represent a task row
	TaskRow struct {
		DomainID     string
		TaskListName string
		TaskListType int
		TaskID       int64

		WorkflowID      string
		RunID           string
		ScheduledID     int64
		Expiry          time.Time
		CreatedTime     time.Time
		PartitionConfig map[string]string
	}

	// TaskListFilter is for filtering tasklist
	TaskListFilter struct {
		DomainID     string
		TaskListName string
		TaskListType int
	}

	// TaskListRow is a tasklist row
	TaskListRow struct {
		DomainID     string
		TaskListName string
		TaskListType int

		RangeID                 int64
		TaskListKind            int
		AckLevel                int64
		CurrentTimeStamp        time.Time
		LastUpdatedTime         time.Time
		AdaptivePartitionConfig *persistence.TaskListPartitionConfig
	}

	// ListTaskListResult is the result of list tasklists
	ListTaskListResult struct {
		TaskLists     []*TaskListRow
		NextPageToken []byte
	}

	// ShardRow is the same as persistence.InternalShardInfo
	// Separate them later when there is a need.
	ShardRow struct {
		*persistence.InternalShardInfo
		Data         []byte
		DataEncoding string
	}

	// ConflictedShardRow contains the partial information about a shard returned when a conditional write fails
	ConflictedShardRow struct {
		ShardID int
		// PreviousRangeID is the condition of previous change that used for conditional update
		PreviousRangeID int64
		// optional detailed information for logging purpose
		Details string
	}

	// DomainRow defines the row struct for queue message
	DomainRow struct {
		Info                        *persistence.DomainInfo
		Config                      *persistence.InternalDomainConfig
		ReplicationConfig           *persistence.InternalDomainReplicationConfig
		ConfigVersion               int64
		FailoverVersion             int64
		FailoverNotificationVersion int64
		PreviousFailoverVersion     int64
		FailoverEndTime             *time.Time
		NotificationVersion         int64
		LastUpdatedTime             time.Time
		IsGlobalDomain              bool
		CurrentTimeStamp            time.Time
	}

	// DomainAuditLogRow defines the row struct for domain audit log
	DomainAuditLogRow struct {
		DomainID            string
		EventID             string
		StateBefore         []byte
		StateBeforeEncoding string
		StateAfter          []byte
		StateAfterEncoding  string
		OperationType       persistence.DomainAuditOperationType
		CreatedTime         time.Time
		LastUpdatedTime     time.Time
		Identity            string
		IdentityType        string
		Comment             string
		TTLSeconds          int64 // TTL for the audit log entry in seconds
	}

	// DomainAuditLogFilter contains the filter criteria for querying domain audit logs
	DomainAuditLogFilter struct {
		DomainID      string
		OperationType persistence.DomainAuditOperationType
		// MinCreatedTime is inclusive
		MinCreatedTime *time.Time
		// MaxCreatedTime is exclusive
		MaxCreatedTime *time.Time
		PageSize       int
		NextPageToken  []byte
	}

	// SelectMessagesBetweenRequest is a request struct for SelectMessagesBetween
	SelectMessagesBetweenRequest struct {
		QueueType               persistence.QueueType
		ExclusiveBeginMessageID int64
		InclusiveEndMessageID   int64
		PageSize                int
		NextPageToken           []byte
	}

	// SelectMessagesBetweenResponse is a response struct for SelectMessagesBetween
	SelectMessagesBetweenResponse struct {
		Rows          []QueueMessageRow
		NextPageToken []byte
	}

	// QueueMessageRow defines the row struct for queue message
	QueueMessageRow struct {
		QueueType        persistence.QueueType
		ID               int64
		Payload          []byte
		CurrentTimeStamp time.Time
	}

	// QueueMetadataRow defines the row struct for metadata
	QueueMetadataRow struct {
		QueueType        persistence.QueueType
		ClusterAckLevels map[string]int64
		Version          int64
		CurrentTimeStamp time.Time
	}

	// HistoryNodeRow represents a row in history_node table
	HistoryNodeRow struct {
		ShardID  int
		TreeID   string
		BranchID string
		NodeID   int64
		// Note: use pointer so that it's easier to multiple by -1 if needed
		TxnID           *int64
		Data            []byte
		DataEncoding    string
		CreateTimestamp time.Time
	}

	// HistoryNodeFilter contains the column names within history_node table that
	// can be used to filter results through a WHERE clause
	HistoryNodeFilter struct {
		ShardID  int
		TreeID   string
		BranchID string
		// Inclusive
		MinNodeID int64
		// Exclusive
		MaxNodeID     int64
		NextPageToken []byte
		PageSize      int
	}

	// HistoryTreeRow represents a row in history_tree table
	HistoryTreeRow struct {
		ShardID         int
		TreeID          string
		BranchID        string
		Ancestors       []*types.HistoryBranchRange
		CreateTimestamp time.Time
		Info            string
	}

	// HistoryTreeFilter contains the column names within history_tree table that
	// can be used to filter results through a WHERE clause
	HistoryTreeFilter struct {
		ShardID  int
		TreeID   string
		BranchID *string
	}
)

const (
	AllOpen VisibilityFilterType = iota
	AllClosed
	OpenByWorkflowType
	ClosedByWorkflowType
	OpenByWorkflowID
	ClosedByWorkflowID
	ClosedByClosedStatus
)

// enums of VisibilitySortType
const (
	SortByStartTime VisibilitySortType = iota
	SortByClosedTime
)

// enums of CurrentWorkflowWriteMode
const (
	CurrentWorkflowWriteModeNoop CurrentWorkflowWriteMode = iota
	CurrentWorkflowWriteModeUpdate
	CurrentWorkflowWriteModeInsert
)

// enums of WorkflowExecutionMapsWriteMode
const (
	// WorkflowExecutionMapsWriteModeCreate will upsert new entry to maps
	WorkflowExecutionMapsWriteModeCreate WorkflowExecutionMapsWriteMode = iota
	// WorkflowExecutionMapsWriteModeUpdate will upsert new entry to maps and also delete entries from maps
	WorkflowExecutionMapsWriteModeUpdate
	// WorkflowExecutionMapsWriteModeReset will reset(override) the whole maps
	WorkflowExecutionMapsWriteModeReset
)

const (
	// EventBufferWriteModeNone is for not doing anything to the event buffer
	EventBufferWriteModeNone EventBufferWriteMode = iota
	// EventBufferWriteModeAppend will append a new event to the event buffer
	EventBufferWriteModeAppend
	// EventBufferWriteModeClear will clear(delete all event from) the event buffer
	EventBufferWriteModeClear
)

const (
	WorkflowRequestWriteModeInsert WorkflowRequestWriteMode = iota
	WorkflowRequestWriteModeUpsert
)

// GetCurrentRunID returns the current runID
func (w *CurrentWorkflowWriteCondition) GetCurrentRunID() string {
	if w == nil || w.CurrentRunID == nil {
		return ""
	}
	return *w.CurrentRunID
}
