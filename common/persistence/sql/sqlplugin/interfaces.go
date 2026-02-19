// Copyright (c) 2017 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/common/persistence/sql/sqlplugin

package sqlplugin

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
)

var (
	// ErrTTLNotSupported indicates the sql plugin does not support ttl
	ErrTTLNotSupported = errors.New("plugin implementation does not support ttl")
)

type (
	// Plugin defines the interface for any SQL database that needs to implement
	Plugin interface {
		CreateDB(cfg *config.SQL) (DB, error)
		CreateAdminDB(cfg *config.SQL) (AdminDB, error)
	}

	// DomainRow represents a row in domain table
	DomainRow struct {
		ID           serialization.UUID
		Name         string
		Data         []byte
		DataEncoding string
		IsGlobal     bool
	}

	// DomainFilter contains the column names within domain table that
	// can be used to filter results through a WHERE clause. When ID is not
	// nil, it will be used for WHERE condition. If ID is nil and Name is non-nil,
	// Name will be used for WHERE condition. When both ID and Name are nil,
	// no WHERE clause will be used
	DomainFilter struct {
		ID            *serialization.UUID
		Name          *string
		GreaterThanID *serialization.UUID
		PageSize      *int
	}

	// DomainMetadataRow represents a row in domain_metadata table
	DomainMetadataRow struct {
		NotificationVersion int64
	}

	// ShardsRow represents a row in shards table
	ShardsRow struct {
		ShardID      int64
		RangeID      int64
		Data         []byte
		DataEncoding string
	}

	// ShardsFilter contains the column names within shards table that
	// can be used to filter results through a WHERE clause
	ShardsFilter struct {
		ShardID int64
	}

	// TransferTasksRow represents a row in transfer_tasks table
	TransferTasksRow struct {
		ShardID      int
		TaskID       int64
		Data         []byte
		DataEncoding string
	}

	// CrossClusterTasksRow represents a row in cross_cluster_tasks table
	CrossClusterTasksRow struct {
		TargetCluster string
		ShardID       int
		TaskID        int64
		Data          []byte
		DataEncoding  string
	}

	// TransferTasksFilter contains the column names within transfer_tasks table that
	// can be used to filter results through a WHERE clause
	TransferTasksFilter struct {
		ShardID            int
		TaskID             int64
		InclusiveMinTaskID int64
		ExclusiveMaxTaskID int64
		PageSize           int
	}

	// CrossClusterTasksFilter contains the column names within cross_cluster_tasks table that
	// can be used to filter results through a WHERE clause
	CrossClusterTasksFilter struct {
		TargetCluster string
		ShardID       int
		TaskID        int64
		MinTaskID     int64
		MaxTaskID     int64
		PageSize      int
	}

	// ExecutionsRow represents a row in executions table
	ExecutionsRow struct {
		ShardID                  int
		DomainID                 serialization.UUID
		WorkflowID               string
		RunID                    serialization.UUID
		NextEventID              int64
		LastWriteVersion         int64
		Data                     []byte
		DataEncoding             string
		VersionHistories         []byte
		VersionHistoriesEncoding string
	}

	// ExecutionsFilter contains the column names within executions table that
	// can be used to filter results through a WHERE clause
	// To get single row, it requires ShardID, DomainID, WorkflowID, RunID
	// To get a list of rows, it requires ShardID, Size.
	// The WorkflowID and RunID are optional for listing rows. They work as the start boundary for pagination.
	ExecutionsFilter struct {
		ShardID    int
		DomainID   serialization.UUID
		WorkflowID string
		RunID      serialization.UUID
		Size       int
	}

	// CurrentExecutionsRow represents a row in current_executions table
	CurrentExecutionsRow struct {
		ShardID          int64
		DomainID         serialization.UUID
		WorkflowID       string
		RunID            serialization.UUID
		CreateRequestID  string
		State            int
		CloseStatus      int
		LastWriteVersion int64
		StartVersion     int64
	}

	// CurrentExecutionsFilter contains the column names within current_executions table that
	// can be used to filter results through a WHERE clause
	CurrentExecutionsFilter struct {
		ShardID    int64
		DomainID   serialization.UUID
		WorkflowID string
		RunID      serialization.UUID
	}

	// BufferedEventsRow represents a row in buffered_events table
	BufferedEventsRow struct {
		ShardID      int
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		Data         []byte
		DataEncoding string
	}

	// BufferedEventsFilter contains the column names within buffered_events table that
	// can be used to filter results through a WHERE clause
	BufferedEventsFilter struct {
		ShardID    int
		DomainID   serialization.UUID
		WorkflowID string
		RunID      serialization.UUID
	}

	// TasksRow represents a row in tasks table
	TasksRow struct {
		ShardID      int // this is DBShardID, not historyShardID (TODO: maybe rename it for clarification)
		DomainID     serialization.UUID
		TaskType     int64
		TaskID       int64
		TaskListName string
		Data         []byte
		DataEncoding string
	}

	// TaskKeyRow represents a result row giving task keys
	TaskKeyRow struct {
		DomainID     serialization.UUID
		TaskListName string
		TaskType     int64
		TaskID       int64
	}

	// TasksRowWithTTL represents a row in tasks table with a ttl
	TasksRowWithTTL struct {
		TasksRow TasksRow
		// TTL is optional because InsertIntoTasksWithTTL operates over a slice of TasksRowWithTTL.
		// Some items in the slice may have a TTL while others do not. It is the responsibility
		// of the plugin implementation to handle items with TTL set and items with TTL not set.
		TTL *time.Duration
	}

	// TasksFilter contains the column names within tasks table that
	// can be used to filter results through a WHERE clause
	TasksFilter struct {
		ShardID              int // this is DBShardID, not historyShardID (TODO: maybe rename it for clarification)
		DomainID             serialization.UUID
		TaskListName         string
		TaskType             int64
		TaskID               *int64
		MinTaskID            *int64
		MaxTaskID            *int64
		TaskIDLessThanEquals *int64
		Limit                *int
		PageSize             *int
	}

	// OrphanTasksFilter contains the parameters controlling orphan deletion
	OrphanTasksFilter struct {
		Limit *int
	}

	// TaskListsRow represents a row in task_lists table
	TaskListsRow struct {
		ShardID      int // this is DBShardID, not historyShardID (TODO: maybe rename it for clarification)
		DomainID     serialization.UUID
		Name         string
		TaskType     int64
		RangeID      int64
		Data         []byte
		DataEncoding string
	}

	// TaskListsRowWithTTL represents a row in task_lists table with a ttl
	TaskListsRowWithTTL struct {
		TaskListsRow TaskListsRow
		TTL          time.Duration
	}

	// TaskListsFilter contains the column names within task_lists table that
	// can be used to filter results through a WHERE clause
	TaskListsFilter struct {
		ShardID             int // this is DBShardID, not historyShardID (TODO: maybe rename it for clarification)
		DomainID            *serialization.UUID
		Name                *string
		TaskType            *int64
		DomainIDGreaterThan *serialization.UUID
		NameGreaterThan     *string
		TaskTypeGreaterThan *int64
		RangeID             *int64
		PageSize            *int
	}

	// ReplicationTasksRow represents a row in replication_tasks table
	ReplicationTasksRow struct {
		ShardID      int
		TaskID       int64
		Data         []byte
		DataEncoding string
	}

	// ReplicationTaskDLQRow represents a row in replication_tasks_dlq table
	ReplicationTaskDLQRow struct {
		SourceClusterName string
		ShardID           int
		TaskID            int64
		Data              []byte
		DataEncoding      string
	}

	// ReplicationTasksFilter contains the column names within replication_tasks table that
	// can be used to filter results through a WHERE clause
	ReplicationTasksFilter struct {
		ShardID            int
		TaskID             int64
		InclusiveMinTaskID int64
		ExclusiveMaxTaskID int64
		PageSize           int
	}

	// ReplicationTasksDLQFilter contains the column names within replication_tasks_dlq table that
	// can be used to filter results through a WHERE clause
	ReplicationTasksDLQFilter struct {
		ReplicationTasksFilter
		SourceClusterName string
	}

	// ReplicationTaskDLQFilter contains the column names within replication_tasks_dlq table that
	// can be used to filter results through a WHERE clause
	ReplicationTaskDLQFilter struct {
		SourceClusterName string
		ShardID           int
	}

	// TimerTasksRow represents a row in timer_tasks table
	TimerTasksRow struct {
		ShardID             int
		VisibilityTimestamp time.Time
		TaskID              int64
		Data                []byte
		DataEncoding        string
	}

	// TimerTasksFilter contains the column names within timer_tasks table that
	// can be used to filter results through a WHERE clause
	TimerTasksFilter struct {
		ShardID                int
		TaskID                 int64
		VisibilityTimestamp    time.Time
		MinVisibilityTimestamp time.Time
		MaxVisibilityTimestamp time.Time
		PageSize               int
	}

	// EventsRow represents a row in events table
	EventsRow struct {
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		FirstEventID int64
		BatchVersion int64
		RangeID      int64
		TxID         int64
		Data         []byte
		DataEncoding string
	}

	// EventsFilter contains the column names within events table that
	// can be used to filter results through a WHERE clause
	EventsFilter struct {
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		FirstEventID *int64
		NextEventID  *int64
		PageSize     *int
	}

	// HistoryNodeRow represents a row in history_node table
	HistoryNodeRow struct {
		ShardID  int
		TreeID   serialization.UUID
		BranchID serialization.UUID
		NodeID   int64
		// use pointer so that it's easier to multiple by -1
		TxnID        *int64
		Data         []byte
		DataEncoding string
	}

	// HistoryNodeFilter contains the column names within history_node table that
	// can be used to filter results through a WHERE clause
	HistoryNodeFilter struct {
		ShardID  int
		TreeID   serialization.UUID
		BranchID serialization.UUID
		// Inclusive
		MinNodeID *int64
		// Exclusive
		MaxNodeID *int64
		PageSize  int
	}

	// HistoryTreeRow represents a row in history_tree table
	HistoryTreeRow struct {
		ShardID      int
		TreeID       serialization.UUID
		BranchID     serialization.UUID
		Data         []byte
		DataEncoding string
	}

	// HistoryTreeFilter contains the column names within history_tree table that
	// can be used to filter results through a WHERE clause
	HistoryTreeFilter struct {
		ShardID  int
		TreeID   serialization.UUID
		BranchID *serialization.UUID
		PageSize *int
	}

	// ActivityInfoMapsRow represents a row in activity_info_maps table
	ActivityInfoMapsRow struct {
		ShardID                  int64
		DomainID                 serialization.UUID
		WorkflowID               string
		RunID                    serialization.UUID
		ScheduleID               int64
		Data                     []byte
		DataEncoding             string
		LastHeartbeatDetails     []byte
		LastHeartbeatUpdatedTime time.Time
	}

	// ActivityInfoMapsFilter contains the column names within activity_info_maps table that
	// can be used to filter results through a WHERE clause
	ActivityInfoMapsFilter struct {
		ShardID     int64
		DomainID    serialization.UUID
		WorkflowID  string
		RunID       serialization.UUID
		ScheduleIDs []int64
	}

	// TimerInfoMapsRow represents a row in timer_info_maps table
	TimerInfoMapsRow struct {
		ShardID      int64
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		TimerID      string
		Data         []byte
		DataEncoding string
	}

	// TimerInfoMapsFilter contains the column names within timer_info_maps table that
	// can be used to filter results through a WHERE clause
	TimerInfoMapsFilter struct {
		ShardID    int64
		DomainID   serialization.UUID
		WorkflowID string
		RunID      serialization.UUID
		TimerIDs   []string
	}

	// ChildExecutionInfoMapsRow represents a row in child_execution_info_maps table
	ChildExecutionInfoMapsRow struct {
		ShardID      int64
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		InitiatedID  int64
		Data         []byte
		DataEncoding string
	}

	// ChildExecutionInfoMapsFilter contains the column names within child_execution_info_maps table that
	// can be used to filter results through a WHERE clause
	ChildExecutionInfoMapsFilter struct {
		ShardID      int64
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		InitiatedIDs []int64
	}

	// RequestCancelInfoMapsRow represents a row in request_cancel_info_maps table
	RequestCancelInfoMapsRow struct {
		ShardID      int64
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		InitiatedID  int64
		Data         []byte
		DataEncoding string
	}

	// RequestCancelInfoMapsFilter contains the column names within request_cancel_info_maps table that
	// can be used to filter results through a WHERE clause
	RequestCancelInfoMapsFilter struct {
		ShardID      int64
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		InitiatedIDs []int64
	}

	// SignalInfoMapsRow represents a row in signal_info_maps table
	SignalInfoMapsRow struct {
		ShardID      int64
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		InitiatedID  int64
		Data         []byte
		DataEncoding string
	}

	// SignalInfoMapsFilter contains the column names within signal_info_maps table that
	// can be used to filter results through a WHERE clause
	SignalInfoMapsFilter struct {
		ShardID      int64
		DomainID     serialization.UUID
		WorkflowID   string
		RunID        serialization.UUID
		InitiatedIDs []int64
	}

	// SignalsRequestedSetsRow represents a row in signals_requested_sets table
	SignalsRequestedSetsRow struct {
		ShardID    int64
		DomainID   serialization.UUID
		WorkflowID string
		RunID      serialization.UUID
		SignalID   string
	}

	// SignalsRequestedSetsFilter contains the column names within signals_requested_sets table that
	// can be used to filter results through a WHERE clause
	SignalsRequestedSetsFilter struct {
		ShardID    int64
		DomainID   serialization.UUID
		WorkflowID string
		RunID      serialization.UUID
		SignalIDs  []string
	}

	// VisibilityRow represents a row in executions_visibility table
	VisibilityRow struct {
		DomainID               string
		RunID                  string
		WorkflowTypeName       string
		WorkflowID             string
		StartTime              time.Time
		ExecutionTime          time.Time
		CloseStatus            *int32
		CloseTime              *time.Time
		HistoryLength          *int64
		Memo                   []byte
		Encoding               string
		IsCron                 bool
		CronSchedule           string
		NumClusters            int16
		UpdateTime             time.Time
		ShardID                int16
		ExecutionStatus        int32
		ScheduledExecutionTime time.Time
	}

	// VisibilityFilter contains the column names within executions_visibility table that
	// can be used to filter results through a WHERE clause
	VisibilityFilter struct {
		DomainID         string
		Closed           bool
		RunID            *string
		WorkflowID       *string
		WorkflowTypeName *string
		CloseStatus      *int32
		MinStartTime     *time.Time
		MaxStartTime     *time.Time
		PageSize         *int
	}

	// QueueRow represents a row in queue table
	QueueRow struct {
		QueueType      persistence.QueueType
		MessageID      int64
		MessagePayload []byte
	}

	// QueueMetadataRow represents a row in queue_metadata table
	QueueMetadataRow struct {
		QueueType persistence.QueueType
		Data      []byte
	}

	// ClusterConfigRow represents a row in cluster_config table
	ClusterConfigRow struct {
		RowType      int
		Version      int64
		Timestamp    time.Time
		Data         []byte
		DataEncoding string
	}

	// DomainAuditLogRow represents a row in domain_audit_log table
	DomainAuditLogRow struct {
		DomainID            string
		EventID             string
		StateBefore         []byte
		StateBeforeEncoding constants.EncodingType
		StateAfter          []byte
		StateAfterEncoding  constants.EncodingType
		OperationType       persistence.DomainAuditOperationType
		CreatedTime         time.Time
		LastUpdatedTime     time.Time
		Identity            string
		IdentityType        string
		Comment             string
	}

	// DomainAuditLogFilter contains the filter criteria for querying domain audit logs
	DomainAuditLogFilter struct {
		DomainID       string
		OperationType  persistence.DomainAuditOperationType
		MinCreatedTime *time.Time
		PageSize       int
		// PageMaxCreatedTime and PageMinEventID are used to paginate Select queries
		PageMaxCreatedTime *time.Time
		PageMinEventID     *string
	}

	// tableCRUD defines the API for interacting with the database tables
	tableCRUD interface {
		InsertIntoDomain(ctx context.Context, rows *DomainRow) (sql.Result, error)
		UpdateDomain(ctx context.Context, row *DomainRow) (sql.Result, error)
		// SelectFromDomain returns domains that match filter criteria. Either ID or
		// Name can be specified to filter results. If both are not specified, all rows
		// will be returned
		SelectFromDomain(ctx context.Context, filter *DomainFilter) ([]DomainRow, error)
		// DeleteDomain deletes a single row. One of ID or Name MUST be specified
		DeleteFromDomain(ctx context.Context, filter *DomainFilter) (sql.Result, error)

		LockDomainMetadata(ctx context.Context) error
		UpdateDomainMetadata(ctx context.Context, row *DomainMetadataRow) (sql.Result, error)
		SelectFromDomainMetadata(ctx context.Context) (*DomainMetadataRow, error)

		InsertIntoShards(ctx context.Context, rows *ShardsRow) (sql.Result, error)
		UpdateShards(ctx context.Context, row *ShardsRow) (sql.Result, error)
		SelectFromShards(ctx context.Context, filter *ShardsFilter) (*ShardsRow, error)
		ReadLockShards(ctx context.Context, filter *ShardsFilter) (int, error)
		WriteLockShards(ctx context.Context, filter *ShardsFilter) (int, error)

		InsertIntoTasks(ctx context.Context, rows []TasksRow) (sql.Result, error)
		InsertIntoTasksWithTTL(ctx context.Context, rows []TasksRowWithTTL) (sql.Result, error)
		// SelectFromTasks retrieves one or more rows from the tasks table
		// Required filter params - {domainID, tasklistName, taskType, minTaskID, maxTaskID, pageSize}
		SelectFromTasks(ctx context.Context, filter *TasksFilter) ([]TasksRow, error)
		// DeleteFromTasks deletes a row from tasks table
		// Required filter params:
		//  to delete single row
		//     - {domainID, tasklistName, taskType, taskID}
		//  to delete multiple rows
		//    - {domainID, tasklistName, taskType, taskIDLessThanEquals, limit }
		//    - this will delete up to limit number of tasks less than or equal to the given task id
		DeleteFromTasks(ctx context.Context, filter *TasksFilter) (sql.Result, error)
		GetTasksCount(ctx context.Context, filter *TasksFilter) (int64, error)
		GetOrphanTasks(ctx context.Context, filter *OrphanTasksFilter) ([]TaskKeyRow, error)

		InsertIntoTaskLists(ctx context.Context, row *TaskListsRow) (sql.Result, error)
		InsertIntoTaskListsWithTTL(ctx context.Context, row *TaskListsRowWithTTL) (sql.Result, error)
		UpdateTaskLists(ctx context.Context, row *TaskListsRow) (sql.Result, error)
		UpdateTaskListsWithTTL(ctx context.Context, row *TaskListsRowWithTTL) (sql.Result, error)
		// SelectFromTaskLists returns one or more rows from task_lists table
		// Required Filter params:
		//  to read a single row: {shardID, domainID, name, taskType}
		//  to range read multiple rows: {shardID, domainIDGreaterThan, nameGreaterThan, taskTypeGreaterThan, pageSize}
		SelectFromTaskLists(ctx context.Context, filter *TaskListsFilter) ([]TaskListsRow, error)
		DeleteFromTaskLists(ctx context.Context, filter *TaskListsFilter) (sql.Result, error)
		LockTaskLists(ctx context.Context, filter *TaskListsFilter) (int64, error)

		// eventsV2
		InsertIntoHistoryNode(ctx context.Context, row *HistoryNodeRow) (sql.Result, error)
		SelectFromHistoryNode(ctx context.Context, filter *HistoryNodeFilter) ([]HistoryNodeRow, error)
		DeleteFromHistoryNode(ctx context.Context, filter *HistoryNodeFilter) (sql.Result, error)
		InsertIntoHistoryTree(ctx context.Context, row *HistoryTreeRow) (sql.Result, error)
		SelectFromHistoryTree(ctx context.Context, filter *HistoryTreeFilter) ([]HistoryTreeRow, error)
		DeleteFromHistoryTree(ctx context.Context, filter *HistoryTreeFilter) (sql.Result, error)
		GetAllHistoryTreeBranches(ctx context.Context, filter *HistoryTreeFilter) ([]HistoryTreeRow, error)

		InsertIntoExecutions(ctx context.Context, row *ExecutionsRow) (sql.Result, error)
		UpdateExecutions(ctx context.Context, row *ExecutionsRow) (sql.Result, error)
		SelectFromExecutions(ctx context.Context, filter *ExecutionsFilter) ([]ExecutionsRow, error)
		DeleteFromExecutions(ctx context.Context, filter *ExecutionsFilter) (sql.Result, error)
		ReadLockExecutions(ctx context.Context, filter *ExecutionsFilter) (int, error)
		WriteLockExecutions(ctx context.Context, filter *ExecutionsFilter) (int, error)

		LockCurrentExecutionsJoinExecutions(ctx context.Context, filter *CurrentExecutionsFilter) ([]CurrentExecutionsRow, error)

		InsertIntoCurrentExecutions(ctx context.Context, row *CurrentExecutionsRow) (sql.Result, error)
		UpdateCurrentExecutions(ctx context.Context, row *CurrentExecutionsRow) (sql.Result, error)
		// SelectFromCurrentExecutions returns one or more rows from current_executions table
		// Required params - {shardID, domainID, workflowID}
		SelectFromCurrentExecutions(ctx context.Context, filter *CurrentExecutionsFilter) (*CurrentExecutionsRow, error)
		// DeleteFromCurrentExecutions deletes a single row that matches the filter criteria
		// If a row exist, that row will be deleted and this method will return success
		// If there is no row matching the filter criteria, this method will still return success
		// Callers can check the output of Result.RowsAffected() to see if a row was deleted or not
		// Required params - {shardID, domainID, workflowID, runID}
		DeleteFromCurrentExecutions(ctx context.Context, filter *CurrentExecutionsFilter) (sql.Result, error)
		LockCurrentExecutions(ctx context.Context, filter *CurrentExecutionsFilter) (*CurrentExecutionsRow, error)

		InsertIntoTransferTasks(ctx context.Context, rows []TransferTasksRow) (sql.Result, error)
		// SelectFromTransferTasks returns rows that match filter criteria from transfer_tasks table.
		// Required filter params - {shardID, minTaskID, maxTaskID}
		SelectFromTransferTasks(ctx context.Context, filter *TransferTasksFilter) ([]TransferTasksRow, error)
		// DeleteFromTransferTasks deletes one or more rows from transfer_tasks table.
		// Required filter params - {shardID, taskID}
		DeleteFromTransferTasks(ctx context.Context, filter *TransferTasksFilter) (sql.Result, error)
		// RangeDeleteFromTransferTasks deletes one or more rows from transfer_tasks table.
		// Required filter params - {shardID, minTaskID, maxTaskID}
		RangeDeleteFromTransferTasks(ctx context.Context, filter *TransferTasksFilter) (sql.Result, error)

		// TODO: add cross-cluster tasks methods
		// InsertIntoCrossClusterTasks adds a new row to the cross_cluster_tasks table
		InsertIntoCrossClusterTasks(ctx context.Context, rows []CrossClusterTasksRow) (sql.Result, error)
		// SelectFromCrossClusterTasks returns rows that match filter criteria from cross_cluster_tasks table.
		// Required filter params - {shardID, minTaskID, maxTaskID}
		SelectFromCrossClusterTasks(ctx context.Context, filter *CrossClusterTasksFilter) ([]CrossClusterTasksRow, error)
		// DeleteFromCrossClusterTasks deletes one or more rows from cross_cluster_tasks table.
		// Required filter params - {shardID, taskID}
		DeleteFromCrossClusterTasks(ctx context.Context, filter *CrossClusterTasksFilter) (sql.Result, error)
		// RangeDeleteFromCrossClusterTasks deletes one or more rows from cross_cluster_tasks table.
		// Required filter params - {shardID, minTaskID, maxTaskID}
		RangeDeleteFromCrossClusterTasks(ctx context.Context, filter *CrossClusterTasksFilter) (sql.Result, error)

		InsertIntoTimerTasks(ctx context.Context, rows []TimerTasksRow) (sql.Result, error)
		// SelectFromTimerTasks returns one or more rows from timer_tasks table
		// Required filter Params - {shardID, taskID, minVisibilityTimestamp, maxVisibilityTimestamp, pageSize}
		SelectFromTimerTasks(ctx context.Context, filter *TimerTasksFilter) ([]TimerTasksRow, error)
		// DeleteFromTimerTasks deletes one or more rows from timer_tasks table
		// Required filter Params: {shardID, visibilityTimestamp, taskID}
		DeleteFromTimerTasks(ctx context.Context, filter *TimerTasksFilter) (sql.Result, error)
		// RangeDeleteFromTimerTasks deletes one or more rows from timer_tasks table
		// Required filter Params: {shardID, minVisibilityTimestamp, maxVisibilityTimestamp}
		RangeDeleteFromTimerTasks(ctx context.Context, filter *TimerTasksFilter) (sql.Result, error)

		InsertIntoBufferedEvents(ctx context.Context, rows []BufferedEventsRow) (sql.Result, error)
		SelectFromBufferedEvents(ctx context.Context, filter *BufferedEventsFilter) ([]BufferedEventsRow, error)
		DeleteFromBufferedEvents(ctx context.Context, filter *BufferedEventsFilter) (sql.Result, error)

		InsertIntoReplicationTasks(ctx context.Context, rows []ReplicationTasksRow) (sql.Result, error)
		// SelectFromReplicationTasks returns one or more rows from replication_tasks table
		// Required filter params - {shardID, minTaskID, maxTaskID, pageSize}
		SelectFromReplicationTasks(ctx context.Context, filter *ReplicationTasksFilter) ([]ReplicationTasksRow, error)
		// DeleteFromReplicationTasks deletes a row from replication_tasks table
		// Required filter params - {shardID, taskID}
		DeleteFromReplicationTasks(ctx context.Context, filter *ReplicationTasksFilter) (sql.Result, error)
		// DeleteFromReplicationTasks deletes multi rows from replication_tasks table
		// Required filter params - {shardID, inclusiveEndTaskID}
		RangeDeleteFromReplicationTasks(ctx context.Context, filter *ReplicationTasksFilter) (sql.Result, error)
		// InsertIntoReplicationTasksDLQ puts the replication task into DLQ
		InsertIntoReplicationTasksDLQ(ctx context.Context, row *ReplicationTaskDLQRow) (sql.Result, error)
		// SelectFromReplicationTasksDLQ returns one or more rows from replication_tasks_dlq table
		// Required filter params - {sourceClusterName, shardID, minTaskID, pageSize}
		SelectFromReplicationTasksDLQ(ctx context.Context, filter *ReplicationTasksDLQFilter) ([]ReplicationTasksRow, error)
		// SelectFromReplicationDLQ returns one row from replication_tasks_dlq table
		// Required filter params - {sourceClusterName}
		SelectFromReplicationDLQ(ctx context.Context, filter *ReplicationTaskDLQFilter) (int64, error)
		// DeleteMessageFromReplicationTasksDLQ deletes one row from replication_tasks_dlq table
		// Required filter params - {sourceClusterName, shardID, taskID}
		DeleteMessageFromReplicationTasksDLQ(ctx context.Context, filter *ReplicationTasksDLQFilter) (sql.Result, error)
		// RangeDeleteMessageFromReplicationTasksDLQ deletes one or more rows from replication_tasks_dlq table
		// Required filter params - {sourceClusterName, shardID, taskID, inclusiveTaskID}
		RangeDeleteMessageFromReplicationTasksDLQ(ctx context.Context, filter *ReplicationTasksDLQFilter) (sql.Result, error)

		ReplaceIntoActivityInfoMaps(ctx context.Context, rows []ActivityInfoMapsRow) (sql.Result, error)
		// SelectFromActivityInfoMaps returns one or more rows from activity_info_maps
		// Required filter params - {shardID, domainID, workflowID, runID}
		SelectFromActivityInfoMaps(ctx context.Context, filter *ActivityInfoMapsFilter) ([]ActivityInfoMapsRow, error)
		// DeleteFromActivityInfoMaps deletes a row from activity_info_maps table
		// Required filter params
		// - one or multiple rows delete - {shardID, domainID, workflowID, runID, scheduleIDs}
		// - range delete - {shardID, domainID, workflowID, runID}
		DeleteFromActivityInfoMaps(ctx context.Context, filter *ActivityInfoMapsFilter) (sql.Result, error)

		ReplaceIntoTimerInfoMaps(ctx context.Context, rows []TimerInfoMapsRow) (sql.Result, error)
		// SelectFromTimerInfoMaps returns one or more rows form timer_info_maps table
		// Required filter params - {shardID, domainID, workflowID, runID}
		SelectFromTimerInfoMaps(ctx context.Context, filter *TimerInfoMapsFilter) ([]TimerInfoMapsRow, error)
		// DeleteFromTimerInfoMaps deletes one or more rows from timer_info_maps
		// Required filter params
		// - one or multiple rows delete- {shardID, domainID, workflowID, runID, timerIDs}
		// - range delete - {shardID, domainID, workflowID, runID}
		DeleteFromTimerInfoMaps(ctx context.Context, filter *TimerInfoMapsFilter) (sql.Result, error)

		ReplaceIntoChildExecutionInfoMaps(ctx context.Context, rows []ChildExecutionInfoMapsRow) (sql.Result, error)
		// SelectFromChildExecutionInfoMaps returns one or more rows form child_execution_info_maps table
		// Required filter params - {shardID, domainID, workflowID, runID}
		SelectFromChildExecutionInfoMaps(ctx context.Context, filter *ChildExecutionInfoMapsFilter) ([]ChildExecutionInfoMapsRow, error)
		// DeleteFromChildExecutionInfoMaps deletes one or more rows from child_execution_info_maps
		// Required filter params
		// - onne or multiple rows delete - {shardID, domainID, workflowID, runID, initiatedIDs}
		// - range delete - {shardID, domainID, workflowID, runID}
		DeleteFromChildExecutionInfoMaps(ctx context.Context, filter *ChildExecutionInfoMapsFilter) (sql.Result, error)

		ReplaceIntoRequestCancelInfoMaps(ctx context.Context, rows []RequestCancelInfoMapsRow) (sql.Result, error)
		// SelectFromRequestCancelInfoMaps returns one or more rows form request_cancel_info_maps table
		// Required filter params - {shardID, domainID, workflowID, runID}
		SelectFromRequestCancelInfoMaps(ctx context.Context, filter *RequestCancelInfoMapsFilter) ([]RequestCancelInfoMapsRow, error)
		// DeleteFromRequestCancelInfoMaps deletes one or more rows from request_cancel_info_maps
		// Required filter params
		// - one or multiple rows delete - {shardID, domainID, workflowID, runID, initiatedIDs}
		// - range delete - {shardID, domainID, workflowID, runID}
		DeleteFromRequestCancelInfoMaps(ctx context.Context, filter *RequestCancelInfoMapsFilter) (sql.Result, error)

		ReplaceIntoSignalInfoMaps(ctx context.Context, rows []SignalInfoMapsRow) (sql.Result, error)
		// SelectFromSignalInfoMaps returns one or more rows form signal_info_maps table
		// Required filter params - {shardID, domainID, workflowID, runID}
		SelectFromSignalInfoMaps(ctx context.Context, filter *SignalInfoMapsFilter) ([]SignalInfoMapsRow, error)
		// DeleteFromSignalInfoMaps deletes one or more rows from signal_info_maps table
		// Required filter params
		// - one or multiple rows delete - {shardID, domainID, workflowID, runID, initiatedIDs}
		// - range delete - {shardID, domainID, workflowID, runID}
		DeleteFromSignalInfoMaps(ctx context.Context, filter *SignalInfoMapsFilter) (sql.Result, error)

		InsertIntoSignalsRequestedSets(ctx context.Context, rows []SignalsRequestedSetsRow) (sql.Result, error)
		// SelectFromSignalInfoMaps returns one or more rows form singals_requested_sets table
		// Required filter params - {shardID, domainID, workflowID, runID}
		SelectFromSignalsRequestedSets(ctx context.Context, filter *SignalsRequestedSetsFilter) ([]SignalsRequestedSetsRow, error)
		// DeleteFromSignalsRequestedSets deletes one or more rows from signals_requested_sets
		// Required filter params
		// - one or multiple rows delete - {shardID, domainID, workflowID, runID, signalIDs}
		// - range delete - {shardID, domainID, workflowID, runID}
		DeleteFromSignalsRequestedSets(ctx context.Context, filter *SignalsRequestedSetsFilter) (sql.Result, error)

		// InsertIntoVisibility inserts a row into visibility table. If a row already exist,
		// no changes will be made by this API
		InsertIntoVisibility(ctx context.Context, row *VisibilityRow) (sql.Result, error)
		// ReplaceIntoVisibility deletes old row (if it exist) and inserts new row into visibility table
		ReplaceIntoVisibility(ctx context.Context, row *VisibilityRow) (sql.Result, error)
		// SelectFromVisibility returns one or more rows from visibility table
		// Required filter params:
		// - getClosedWorkflowExecution - retrieves single row - {domainID, runID, closed=true}
		// - All other queries retrieve multiple rows (range):
		//   - MUST specify following required params:
		//     - domainID, minStartTime, maxStartTime, runID and pageSize where some or all of these may come from previous page token
		//   - OPTIONALLY specify one of following params
		//     - workflowID, workflowTypeName, closeStatus (along with closed=true)
		SelectFromVisibility(ctx context.Context, filter *VisibilityFilter) ([]VisibilityRow, error)
		DeleteFromVisibility(ctx context.Context, filter *VisibilityFilter) (sql.Result, error)

		InsertIntoQueue(ctx context.Context, row *QueueRow) (sql.Result, error)
		GetLastEnqueuedMessageIDForUpdate(ctx context.Context, queueType persistence.QueueType) (int64, error)
		GetMessagesFromQueue(ctx context.Context, queueType persistence.QueueType, lastMessageID int64, maxRows int) ([]QueueRow, error)
		GetMessagesBetween(ctx context.Context, queueType persistence.QueueType, firstMessageID int64, lastMessageID int64, maxRows int) ([]QueueRow, error)
		DeleteMessagesBefore(ctx context.Context, queueType persistence.QueueType, messageID int64) (sql.Result, error)
		RangeDeleteMessages(ctx context.Context, queueType persistence.QueueType, exclusiveBeginMessageID int64, inclusiveEndMessageID int64) (sql.Result, error)
		DeleteMessage(ctx context.Context, queueType persistence.QueueType, messageID int64) (sql.Result, error)
		InsertAckLevel(ctx context.Context, queueType persistence.QueueType, messageID int64, clusterName string) error
		UpdateAckLevels(ctx context.Context, queueType persistence.QueueType, clusterAckLevels map[string]int64) error
		GetAckLevels(ctx context.Context, queueType persistence.QueueType, forUpdate bool) (map[string]int64, error)
		GetQueueSize(ctx context.Context, queueType persistence.QueueType) (int64, error)

		// InsertConfig insert a config entry with version. Return nosqlplugin.NewConditionFailure if the same version of the row_type is existing
		InsertConfig(ctx context.Context, row *persistence.InternalConfigStoreEntry) error
		// SelectLatestConfig returns the config entry of the row_type with the largest(latest) version value
		SelectLatestConfig(ctx context.Context, rowType int) (*persistence.InternalConfigStoreEntry, error)

		// InsertDomainAuditLog inserts a new audit log entry for a domain operation. Returns error if there is any failure
		InsertIntoDomainAuditLog(ctx context.Context, row *DomainAuditLogRow) (sql.Result, error)
		// SelectFromDomainAuditLogs returns audit log entries for a domain. Returns paginated results ordered by created_time DESC, event_id ASC
		SelectFromDomainAuditLogs(ctx context.Context, filter *DomainAuditLogFilter) ([]*DomainAuditLogRow, error)

		// The follow provide information about the underlying sql crud implementation
		SupportsTTL() bool
		MaxAllowedTTL() (*time.Duration, error)
		SupportsAsyncTransaction() bool
	}

	// adminCRUD defines admin operations for CLI and test suites
	adminCRUD interface {
		CreateSchemaVersionTables() error
		ReadSchemaVersion(database string) (string, error)
		UpdateSchemaVersion(database string, newVersion string, minCompatibleVersion string) error
		WriteSchemaUpdateLog(oldVersion string, newVersion string, manifestMD5 string, desc string) error
		ListTables(database string) ([]string, error)
		DropTable(table string) error
		DropAllTables(database string) error
		CreateDatabase(database string) error
		DropDatabase(database string) error
		// ExecSchemaOperationQuery allows passing in any query, but it must be schema operation (DDL)
		ExecSchemaOperationQuery(ctx context.Context, stmt string, args ...interface{}) error
	}

	// Tx defines the API for a SQL transaction
	Tx interface {
		tableCRUD
		ErrorChecker

		Commit() error
		Rollback() error
	}

	// DB defines the API for regular SQL operations of a Cadence server
	DB interface {
		tableCRUD
		ErrorChecker

		GetTotalNumDBShards() int
		BeginTx(ctx context.Context, dbShardID int) (Tx, error)
		PluginName() string
		Close() error
	}

	// AdminDB defines the API for admin SQL operations for CLI and testing suites
	AdminDB interface {
		adminCRUD
		PluginName() string
		Close() error
	}

	ErrorChecker interface {
		IsDupEntryError(err error) bool
		IsNotFoundError(err error) bool
		IsTimeoutError(err error) bool
		IsThrottlingError(err error) bool
	}
)
