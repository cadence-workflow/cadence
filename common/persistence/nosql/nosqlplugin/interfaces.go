// Copyright (c) 2020 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go -self_package github.com/uber/cadence/common/persistence/nosql/nosqlplugin

package nosqlplugin

import (
	"context"
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
)

type (
	// Plugin defines the interface for any NoSQL database that needs to implement
	Plugin interface {
		CreateDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (DB, error)
		CreateAdminDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (AdminDB, error)
	}

	// AdminDB is for tooling and testing
	AdminDB interface {
		SetupTestDatabase(schemaBaseDir string, replicas int) error
		TeardownTestDatabase() error
	}

	// DB defines the API for regular NoSQL operations of a Cadence server
	DB interface {
		PluginName() string
		Close()

		ClientErrorChecker
		tableCRUD
	}
	// tableCRUD defines the API for interacting with the database tables
	// NOTE 1: All SELECT interfaces require strong consistency (eventual consistency will not work) unless specify in the method.
	//
	// NOTE 2: About schema: only the columns that need to be used directly in queries are considered 'significant',
	// including partition key, range key or index key and conditional columns, are required to be in the schema.
	// All other non 'significant' columns are opaque for implementation.
	// Therefore, it's recommended to use a data blob to store all the other columns, so that adding new
	// column will not require schema changes. This approach has been proved very successful in MySQL/Postgres implementation of SQL interfaces.
	// Cassandra implementation cannot do it due to backward-compatibility. Any other NoSQL implementation should use datablob for non-significant columns.
	// Follow the comment for each tableCRUD for what are 'significant' columns.
	tableCRUD interface {
		HistoryEventsCRUD
		MessageQueueCRUD
		DomainCRUD
		ShardCRUD
		VisibilityCRUD
		TaskCRUD
		WorkflowCRUD
		ConfigStoreCRUD
		DomainAuditLogCRUD
	}

	// ClientErrorChecker checks for common nosql errors on client
	ClientErrorChecker interface {
		IsTimeoutError(error) bool
		IsNotFoundError(error) bool
		IsThrottlingError(error) bool
		IsDBUnavailableError(error) bool
	}

	/**
	 * HistoryEventsCRUD is for History events storage system
	 * Recommendation: use two tables: history_tree for branch records and history_node for node records
	 * if a single update query can operate on two tables.
	 *
	 * Significant columns:
	 * history_tree partition key: (shardID, treeID), range key: (branchID)
	 * history_node partition key: (shardID, treeID), range key: (branchID, nodeID ASC, txnID DESC)
	 */
	HistoryEventsCRUD interface {
		// InsertIntoHistoryTreeAndNode inserts one or two rows: tree row and node row(at least one of them)
		InsertIntoHistoryTreeAndNode(ctx context.Context, treeRow *HistoryTreeRow, nodeRow *HistoryNodeRow) error

		// SelectFromHistoryNode read nodes based on a filter
		SelectFromHistoryNode(ctx context.Context, filter *HistoryNodeFilter) ([]*HistoryNodeRow, []byte, error)

		// DeleteFromHistoryTreeAndNode delete a branch record, and a list of ranges of nodes.
		// for each range, it will delete all nodes starting from MinNodeID(inclusive)
		DeleteFromHistoryTreeAndNode(ctx context.Context, treeFilter *HistoryTreeFilter, nodeFilters []*HistoryNodeFilter) error

		// SelectAllHistoryTrees will return all tree branches with pagination
		SelectAllHistoryTrees(ctx context.Context, nextPageToken []byte, pageSize int) ([]*HistoryTreeRow, []byte, error)

		// SelectFromHistoryTree read branch records for a tree.
		// It returns without pagination, because we assume one tree won't have too many branches.
		SelectFromHistoryTree(ctx context.Context, filter *HistoryTreeFilter) ([]*HistoryTreeRow, error)
	}

	/***
	 * MessageQueueCRUD is for the message queue storage system
	 *
	 * Recommendation: use two tables(queue_message,and queue_metadata) to implement this interface
	 *
	 * Significant columns:
	 * queue_message partition key: (queueType), range key: (messageID)
	 * queue_metadata partition key: (queueType), range key: N/A, query condition column(version)
	 */
	MessageQueueCRUD interface {
		// Insert message into queue, return error if failed or already exists
		// Must return conditionFailed error if row already exists
		InsertIntoQueue(ctx context.Context, row *QueueMessageRow) error
		// Get the ID of last message inserted into the queue
		SelectLastEnqueuedMessageID(ctx context.Context, queueType persistence.QueueType) (int64, error)
		// Read queue messages starting from the exclusiveBeginMessageID
		SelectMessagesFrom(ctx context.Context, queueType persistence.QueueType, exclusiveBeginMessageID int64, maxRows int) ([]*QueueMessageRow, error)
		// Read queue message starting from exclusiveBeginMessageID int64, inclusiveEndMessageID int64
		SelectMessagesBetween(ctx context.Context, request SelectMessagesBetweenRequest) (*SelectMessagesBetweenResponse, error)
		// Delete all messages before exclusiveBeginMessageID
		DeleteMessagesBefore(ctx context.Context, queueType persistence.QueueType, exclusiveBeginMessageID int64) error
		// Delete all messages in a range between exclusiveBeginMessageID and inclusiveEndMessageID
		DeleteMessagesInRange(ctx context.Context, queueType persistence.QueueType, exclusiveBeginMessageID int64, inclusiveEndMessageID int64) error
		// Delete one message
		DeleteMessage(ctx context.Context, queueType persistence.QueueType, messageID int64) error

		// Insert an empty metadata row, starting from a version
		InsertQueueMetadata(ctx context.Context, row QueueMetadataRow) error
		// **Conditionally** update a queue metadata row, if current version is matched(meaning current == row.Version - 1),
		// then the current version will increase by one when updating the metadata row
		// Must return conditionFailed error if the condition is not met
		UpdateQueueMetadataCas(ctx context.Context, row QueueMetadataRow) error
		// Read a QueueMetadata
		SelectQueueMetadata(ctx context.Context, queueType persistence.QueueType) (*QueueMetadataRow, error)
		// GetQueueSize return the queue size
		GetQueueSize(ctx context.Context, queueType persistence.QueueType) (int64, error)
	}

	/***
	* DomainCRUD is for domain + domain metadata storage system
	*
	* Recommendation: two tables(domain, domain_metadata) to implement if conditional updates on two tables is supported
	*
	* Significant columns:
	* domain: partition key( a constant value), range key(domainName), local secondary index(domainID)
	* domain_metadata: partition key( a constant value), range key(N/A), query condition column(notificationVersion)
	*
	* Note 1: About Cassandra's implementation: Because of historical reasons, Cassandra uses two table,
	* domains and domains_by_name_v2. Therefore, Cassandra implementation lost the atomicity causing some edge cases,
	* and the implementation is more complicated than it should be.
	*
	* Note 2: Cassandra doesn't support conditional updates on multiple tables. Hence the domain_metadata table is implemented
	* as a special record as "domain metadata". It is an integer number as notification version.
	* The main purpose of it is to notify clusters that there is some changes in domains, so domain cache needs to refresh.
	* It always increase by one, whenever a domain is updated or inserted.
	* Updating this failover metadata with domain insert/update needs to be atomic.
	* Because Batch LWTs is only allowed within one table and same partition.
	* The Cassandra implementation stores it in the same table as domain in domains_by_name_v2.
	*
	* Note 3: It's okay to use a constant value for partition key because domain table is serving very small volume of traffic.
	 */
	DomainCRUD interface {
		// Insert a new record to domain
		// return types.DomainAlreadyExistsError error if failed or already exists
		// Must return ConditionFailure error if other condition doesn't match
		InsertDomain(ctx context.Context, row *DomainRow) error
		// Update domain data
		// Must return ConditionFailure error if update condition doesn't match
		UpdateDomain(ctx context.Context, row *DomainRow) error
		// Get one domain data, either by domainID or domainName
		SelectDomain(ctx context.Context, domainID *string, domainName *string) (*DomainRow, error)
		// Get all domain data
		SelectAllDomains(ctx context.Context, pageSize int, pageToken []byte) ([]*DomainRow, []byte, error)
		//  Delete a domain, either by domainID or domainName
		DeleteDomain(ctx context.Context, domainID *string, domainName *string) error
		// right now domain metadata is just an integer as notification version
		SelectDomainMetadata(ctx context.Context) (int64, error)
	}

	/**
	* ShardCRUD is for shard storage of workflow execution.
	* Recommendation: use one table if database support batch conditional update on multiple tables, otherwise combine with WorkflowCRUD (likeCassandra)
	*
	* Significant columns:
	* domain: partition key(shardID), range key(N/A), local secondary index(domainID), query condition column(rangeID)
	*
	* Note 1: shard will be required to run conditional update with WorkflowCRUD. So in some nosql database like Cassandra,
	* ShardCRUD and WorkflowCRUD must be implemented within the same table. Because Cassandra only allows LightWeight transaction
	* executed within a single table.
	* Note 2: unlike Cassandra, most NoSQL databases don't return the previous rows when conditional write fails. In this case,
	* an extra read query is needed to get the previous row.
	 */
	ShardCRUD interface {
		// InsertShard creates a new shard.
		// Return error is there is any thing wrong
		// Return the ShardOperationConditionFailure when doesn't meet the condition
		InsertShard(ctx context.Context, row *ShardRow) error
		// SelectShard gets a shard, rangeID is the current rangeID in shard row
		SelectShard(ctx context.Context, shardID int, currentClusterName string) (rangeID int64, shard *ShardRow, err error)
		// UpdateRangeID updates the rangeID
		// Return error is there is any thing wrong
		// Return the ShardOperationConditionFailure when doesn't meet the condition
		UpdateRangeID(ctx context.Context, shardID int, rangeID int64, previousRangeID int64) error
		// UpdateShard updates a shard
		// Return error is there is any thing wrong
		// Return the ShardOperationConditionFailure when doesn't meet the condition
		UpdateShard(ctx context.Context, row *ShardRow, previousRangeID int64) error
	}

	/**
	* VisibilityCRUD is for visibility using database.
	* Database visibility usually is no longer recommended. AdvancedVisibility(with Kafka+ElasticSearch) is more powerful and scalable.
	* Feel free to skip this interface for any NoSQL plugin(use TODO() in the implementation)
	*
	* Recommendation: use one table with multiple indexes
	*
	* Significant columns:
	* domain: partition key(domainID), range key(workflowID, runID),
	*         local secondary index #1(startTime),
	*         local secondary index #2(closedTime),
	*         local secondary index #3(workflowType, startTime),
	*         local secondary index #4(workflowType, closedTime),
	*         local secondary index #5(workflowID, startTime),
	*         local secondary index #6(workflowID, closedTime),
	*         local secondary index #7(closeStatus, closedTime),
	*
	* NOTE 1: Cassandra implementation of visibility uses three tables: open_executions, closed_executions and closed_executions_v2,
	* because Cassandra doesn't support cross-partition indexing.
	* Records in open_executions and closed_executions are clustered by start_time. Records in  closed_executions_v2 are by close_time.
	* This optimizes the performance, but introduce a lot of complexity.
	* In some other databases, this may be be necessary. Please refer to MySQL/Postgres implementation which uses only
	* one table with multiple indexes.
	*
	* NOTE 2: TTL(time to live records) is for auto-deleting expired records in visibility. For databases that don't support TTL,
	* please implement DeleteVisibility method. If TTL is supported, then DeleteVisibility can be a noop.
	 */
	VisibilityCRUD interface {
		InsertVisibility(ctx context.Context, ttlSeconds int64, row *VisibilityRowForInsert) error
		UpdateVisibility(ctx context.Context, ttlSeconds int64, row *VisibilityRowForUpdate) error
		SelectVisibility(ctx context.Context, filter *VisibilityFilter) (*SelectVisibilityResponse, error)
		DeleteVisibility(ctx context.Context, domainID, workflowID, runID string) error
		// TODO deprecated this in the future in favor of SelectVisibility
		// Special case: return nil,nil if not found(since we will deprecate it, it's not worth refactor to be consistent)
		SelectOneClosedWorkflow(ctx context.Context, domainID, workflowID, runID string) (*VisibilityRow, error)
	}

	VisibilityRowForInsert struct {
		VisibilityRow
		DomainID string
	}

	VisibilityRowForUpdate struct {
		VisibilityRow
		DomainID string
		// NOTE: this is only for some implementation (e.g. Cassandra) that uses multiple tables,
		// they needs to delete record from the open execution table. Ignore this field if not need it
		UpdateOpenToClose bool
		//  Similar as UpdateOpenToClose
		UpdateCloseToOpen bool
	}

	// TODO separate in the future when need it
	VisibilityRow = persistence.InternalVisibilityWorkflowExecutionInfo

	SelectVisibilityResponse struct {
		Executions    []*VisibilityRow
		NextPageToken []byte
	}

	// VisibilityFilter contains the column names within executions_visibility table that
	// can be used to filter results through a WHERE clause
	VisibilityFilter struct {
		ListRequest  persistence.InternalListWorkflowExecutionsRequest
		FilterType   VisibilityFilterType
		SortType     VisibilitySortType
		WorkflowType string
		WorkflowID   string
		CloseStatus  int32
	}

	VisibilityFilterType int
	VisibilitySortType   int

	/**
	* TaskCRUD is for tasklist and worker tasks storage
	* The task here is only referred to workflow/activity worker tasks. `Task` is a overloaded term in Cadence.
	* There is another 'task' storage which is for internal purpose only in WorkflowCRUD.
	*
	* Recommendation: use two tables(tasklist + task) to implement
	* tasklist table stores the metadata mainly for
	*   * rangeID: ownership management, and taskID management  everytime a matching host claim ownership of a tasklist,
	*              it must increase the value succesfully . also used as base section of the taskID. E.g, if rangeID is 1,
	*              then allowed taskID ranged will be [100K, 2*100K-1].
	*   * ackLevel: max taskID that can be safely deleted.
	* Any task record is associated with a tasklist. Any updates on a task should use rangeID of the associated tasklist as condition.
	*
	* Significant columns:
	* tasklist: partition key(domainID, taskListName, taskListType), range key(N/A), query condition column(rangeID)
	* task:partition key(domainID, taskListName, taskListType), range key(taskID), query condition column(rangeID)
	*
	* NOTE 1: Cassandra implementation uses the same table for tasklist and task, because Cassandra only allows
	*        batch conditional updates(LightWeight transaction) executed within a single table.
	* NOTE 2: TTL(time to live records) is for auto-deleting task and some tasklists records. For databases that don't
	*        support TTL, please implement ListTaskList method, and allows TaskListScavenger like MySQL/Postgres.
	*        If TTL is supported, then ListTaskList can be a noop.
	 */
	TaskCRUD interface {
		// SelectTaskList returns a single tasklist row.
		// Return IsNotFoundError if the row doesn't exist
		SelectTaskList(ctx context.Context, filter *TaskListFilter) (*TaskListRow, error)
		// InsertTaskList insert a single tasklist row
		// Return TaskOperationConditionFailure if the row already exists
		InsertTaskList(ctx context.Context, row *TaskListRow) error
		// UpdateTaskList updates a single tasklist row
		// Return TaskOperationConditionFailure if the condition doesn't meet
		UpdateTaskList(ctx context.Context, row *TaskListRow, previousRangeID int64) error
		// UpdateTaskList updates a single tasklist row, and set an TTL on the record
		// Return TaskOperationConditionFailure if the condition doesn't meet
		// Ignore TTL if it's not supported, which becomes exactly the same as UpdateTaskList, but ListTaskList must be
		// implemented for TaskListScavenger
		UpdateTaskListWithTTL(ctx context.Context, ttlSeconds int64, row *TaskListRow, previousRangeID int64) error
		// ListTaskList returns all tasklists.
		// Noop if TTL is already implemented in other methods
		ListTaskList(ctx context.Context, pageSize int, nextPageToken []byte) (*ListTaskListResult, error)
		// DeleteTaskList deletes a single tasklist row
		// Return TaskOperationConditionFailure if the condition doesn't meet
		DeleteTaskList(ctx context.Context, filter *TaskListFilter, previousRangeID int64) error
		// InsertTasks inserts a batch of tasks
		// Return TaskOperationConditionFailure if the condition doesn't meet
		InsertTasks(ctx context.Context, tasksToInsert []*TaskRowForInsert, tasklistCondition *TaskListRow) error
		// SelectTasks return tasks that associated to a tasklist
		SelectTasks(ctx context.Context, filter *TasksFilter) ([]*TaskRow, error)
		// DeleteTask delete a batch of tasks
		// Also return the number of rows deleted -- if it's not supported then ignore the batchSize, and return persistence.UnknownNumRowsAffected
		RangeDeleteTasks(ctx context.Context, filter *TasksFilter) (rowsDeleted int, err error)
		// GetTasksCount return the number of tasks
		GetTasksCount(ctx context.Context, filter *TasksFilter) (int64, error)
	}

	/**
	* WorkflowCRUD is for core data models of workflow execution.
	*
	* Recommendation: If possible, use 8 tables(current_workflow, workflow_execution, transfer_task, replication_task, cross_cluster_task, timer_task, buffered_event_list, replication_dlq_task) to implement
	* current_workflow is to track the currentRunID of a workflowID for ensuring the ID-Uniqueness of Cadence workflows.
	* 		Each record is for one workflowID
	* workflow_execution is to store the core data of workflow execution.
	*		Each record is for one runID(workflow execution run).
	* Different from TaskCRUD, transfer_task, replication_task, cross_cluster_task, timer_task are all internal background tasks within Cadence server.
	* transfer_task is to store the background tasks that need to be processed by historyEngine, right after the transaction.
	*		There are lots of usage in historyEngine, like creating activity/childWF/etc task, and updating search attributes, etc.
	* replication_task is to store also background tasks that need to be processed right after the transaction,
	*		but only for CrossDC(XDC) replication feature. Each record is a replication task generated from a source cluster.
	*		Replication task stores a reference to a batch of history events(see historyCRUD).
	* timer_task is to store the durable timers that will fire in the future. Therefore this table should be indexed by the firingTime.
	*		The durable timers are not only for workflow timers, but also for all kinds of timeouts, and workflow deletion, etc.
	* cross_cluster_task is to store also background tasks that need to be processed right after the transaction, and only for
	*		but only for cross cluster feature. Each record is a cross cluster task generated for a target cluster.
	*		CrossCluster task stores information similar to TransferTask.
	* buffered_event_list is to store the buffered event of a workflow execution
	* The above 7 tables will be required to execute transaction write with the condition of shard record from ShardCRUD.
	* replication_dlq_task is DeadLetterQueue when target cluster pulling and applying replication task. Each record represents
	*		a task for a target cluster.
	*
	* Significant columns:
	* current_workflow: partition key(shardID), range key(domainID, workflowID), query condition column(currentRunID, lastWriteVersion, state)
	* workflow_execution: partition key(shardID), range key(domainID, workflowID, runID), query condition column(nextEventID)
	* transfer_task: partition key(shardID), range key(taskID)
	* replication_task: partition key(shardID), range key(taskID)
	* cross_cluster_task: partition key(shardID), range key(clusterName, taskID)
	* timer_task: partition key(shardID), range key(visibilityTimestamp)
	* buffered_event_list: partition key(shardID), range key(domainID, workflowID, runID)
	* replication_dlq_task: partition key(shardID), range key(clusterName, taskID)
	*
	* NOTE: Cassandra limits lightweight transaction to execute within one table. So the 6 tables + shard table are implemented
	*   	via a single table `execution` in Cassandra, using `rowType` to differentiate the 7 tables, and using `permanentRunID`
	*		to differentiate current_workflow and workflow_execution
	* NOTE: Cassandra implementation uses 6 maps to store activityInfo, timerInfo, childWorkflowInfo, requestCancels,
	*		signalInfo and signalRequestedInfo.Those should be fine to be stored in the same record as its workflow_execution.
	*		However, signalInfo stores the in progress signal data. It may be too big for a single record. For example, DynamoDB
	*		requires 400KB of a record. In that case, it may be better to have a separate table for signalInfo.
	* NOTE: Cassandra implementation of workflow_execution uses maps without "frozen". This has the advantage of deleting activity/timer/childWF/etc
	*		by keys. The equivalent of this may require a read before overwriting the existing. Eg. [ "act1": <some data>, "act2": <some data>]
	*		When deleting "act1", Cassandra implementation can delete without read. If storing in the same record of workflwo_execution,
	*		it will require to read the whole activityInfo map for deleting.
	* NOTE: Optional optimization: taskID that are writing into internal tasks(transfer/replication/crossCluster) are immutable and always increasing.
	*		So it is possible to write the tasks in a single record, indexing by the lowest or highest taskID.
	*		This approach can't be used by timerTasks as timers are ordered by visibilityTimestamp.
	*		This is useful for DynamoDB because a transaction cannot contain more than 25 unique items.
	*
	 */
	WorkflowCRUD interface {
		// InsertWorkflowExecutionWithTasks is for creating a new workflow execution record. Within a transaction, it also:
		// 1. Create or update the record of current_workflow with the same workflowID, based on CurrentWorkflowExecutionWriteMode,
		//		and also check if the condition is met.
		// 2. Create the workflow_execution record, including basic info and 6 maps(activityInfoMap, timerInfoMap,
		//		childWorkflowInfoMap, signalInfoMap and signalRequestedIDs)
		// 3. Create history tasks
		// 4. Create workflow requests for requests deduplication
		// 5. Check if the condition of shard rangeID is met
		// The API returns error if there is any. If any of the condition is not met, returns WorkflowOperationConditionFailure
		InsertWorkflowExecutionWithTasks(
			ctx context.Context,
			requests *WorkflowRequestsWriteRequest,
			currentWorkflowRequest *CurrentWorkflowWriteRequest,
			execution *WorkflowExecutionRequest,
			tasksByCategory map[persistence.HistoryTaskCategory][]*HistoryMigrationTask,
			activeClusterSelectionPolicyRow *ActiveClusterSelectionPolicyRow,
			shardCondition *ShardCondition,
		) error

		// UpdateWorkflowExecutionWithTasks is for updating a new workflow execution record.
		// Within a transaction, it also:
		// 1. If currentWorkflowRequest is not nil, Update the record of current_workflow with the same workflowID, based on CurrentWorkflowExecutionWriteMode,
		//		and also check if the condition is met.
		// 2. Update mutatedExecution as workflow_execution record, including basic info and 6 maps(activityInfoMap, timerInfoMap,
		//		childWorkflowInfoMap, signalInfoMap and signalRequestedIDs)
		// 3. if insertedExecution is not nil, then also insert a new workflow_execution record including basic info and add to 6 maps:
		// 		(activityInfoMap, timerInfoMap, childWorkflowInfoMap, signalInfoMap and signalRequestedIDs)
		//      Also insert activeClusterSelectionPolicyRow if it is not nil corresponding to the new execution
		// 4. if resetExecution is not nil, then also update the workflow_execution record including basic info and reset/override 6 maps(activityInfoMap, timerInfoMap,
		//		childWorkflowInfoMap, signalInfoMap and signalRequestedIDs
		// 5. Create history tasks
		// 6. Create workflow requests for requests deduplication
		// 7. Check if the condition of shard rangeID is met
		// The API returns error if there is any. If any of the condition is not met, returns WorkflowOperationConditionFailure
		UpdateWorkflowExecutionWithTasks(
			ctx context.Context,
			requests *WorkflowRequestsWriteRequest,
			currentWorkflowRequest *CurrentWorkflowWriteRequest,
			mutatedExecution *WorkflowExecutionRequest,
			insertedExecution *WorkflowExecutionRequest,
			activeClusterSelectionPolicyRow *ActiveClusterSelectionPolicyRow,
			resetExecution *WorkflowExecutionRequest,
			tasksByCategory map[persistence.HistoryTaskCategory][]*HistoryMigrationTask,
			shardCondition *ShardCondition,
		) error

		// current_workflow table
		// Return the current_workflow row
		SelectCurrentWorkflow(ctx context.Context, shardID int, domainID, workflowID string) (*CurrentWorkflowRow, error)
		// Paging through all current_workflow rows in a shard
		SelectAllCurrentWorkflows(ctx context.Context, shardID int, pageToken []byte, pageSize int) ([]*persistence.CurrentWorkflowExecution, []byte, error)
		// Delete the current_workflow row, if currentRunIDCondition is met
		DeleteCurrentWorkflow(ctx context.Context, shardID int, domainID, workflowID, currentRunIDCondition string) error

		// workflow_execution table
		// Return the workflow execution row
		SelectWorkflowExecution(ctx context.Context, shardID int, domainID, workflowID, runID string) (*WorkflowExecution, error)
		// Paging through all  workflow execution rows in a shard
		SelectAllWorkflowExecutions(ctx context.Context, shardID int, pageToken []byte, pageSize int) ([]*persistence.InternalListConcreteExecutionsEntity, []byte, error)
		// Return whether or not an execution is existing.
		IsWorkflowExecutionExists(ctx context.Context, shardID int, domainID, workflowID, runID string) (bool, error)
		// Delete the workflow execution row
		DeleteWorkflowExecution(ctx context.Context, shardID int, domainID, workflowID, runID string) error

		// transfer_task table
		// within a shard, paging through transfer tasks order by taskID(ASC), filtered by minTaskID(inclusive) and maxTaskID(exclusive)
		SelectTransferTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*HistoryMigrationTask, []byte, error)
		// delete a single transfer task
		DeleteTransferTask(ctx context.Context, shardID int, taskID int64) error
		// delete a range of transfer tasks
		RangeDeleteTransferTasks(ctx context.Context, shardID int, inclusiveBeginTaskID, exclusiveEndTaskID int64) error

		// timer_task table
		// within a shard, paging through timer tasks order by taskID(ASC), filtered by visibilityTimestamp
		SelectTimerTasksOrderByVisibilityTime(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTime, exclusiveMaxTime time.Time) ([]*HistoryMigrationTask, []byte, error)
		// delete a single timer task
		DeleteTimerTask(ctx context.Context, shardID int, taskID int64, visibilityTimestamp time.Time) error
		// delete a single workflow timer task with specific workflow identifiers
		DeleteWorkflowTimerTask(ctx context.Context, shardID int, visibilityTimestamp time.Time, taskID int64) error
		// delete a range of timer tasks
		RangeDeleteTimerTasks(ctx context.Context, shardID int, inclusiveMinTime, exclusiveMaxTime time.Time) error

		// replication_task table
		// within a shard, paging through replication tasks order by taskID(ASC), filtered by minTaskID(inclusive) and maxTaskID(exclusive)
		SelectReplicationTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*HistoryMigrationTask, []byte, error)
		// delete a single replication task
		DeleteReplicationTask(ctx context.Context, shardID int, taskID int64) error
		// delete a range of replication tasks
		RangeDeleteReplicationTasks(ctx context.Context, shardID int, exclusiveEndTaskID int64) error
		// insert replication task with shard condition check
		InsertReplicationTask(ctx context.Context, tasks []*HistoryMigrationTask, condition ShardCondition) error

		// cross_cluster_task table
		// delete a single transfer task
		DeleteCrossClusterTask(ctx context.Context, shardID int, targetCluster string, taskID int64) error

		// replication_dlq_task
		// insert a new replication task to DLQ
		InsertReplicationDLQTask(ctx context.Context, shardID int, sourceCluster string, task *HistoryMigrationTask) error
		// within a shard, for a sourceCluster, paging through replication tasks order by taskID(ASC), filtered by minTaskID(inclusive) and maxTaskID(exclusive)
		SelectReplicationDLQTasksOrderByTaskID(ctx context.Context, shardID int, sourceCluster string, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*HistoryMigrationTask, []byte, error)
		// return the DLQ size
		SelectReplicationDLQTasksCount(ctx context.Context, shardID int, sourceCluster string) (int64, error)
		// delete a single replication DLQ task
		DeleteReplicationDLQTask(ctx context.Context, shardID int, sourceCluster string, taskID int64) error
		// delete a range of replication DLQ tasks
		RangeDeleteReplicationDLQTasks(ctx context.Context, shardID int, sourceCluster string, inclusiveBeginTaskID, exclusiveEndTaskID int64) error

		// select the active cluster selection policy
		SelectActiveClusterSelectionPolicy(ctx context.Context, shardID int, domainID, wfID, rID string) (*ActiveClusterSelectionPolicyRow, error)
		// delete the active cluster selection policy row
		DeleteActiveClusterSelectionPolicy(ctx context.Context, shardID int, domainID, workflowID, runID string) error
	}

	/***
	* ConfigStoreCRUD is for storing dynamic configuration parameters
	*
	* Recommendation: one table
	*
	* Significant columns:
	* domain: partition key(row_type), range key(version)
	 */
	ConfigStoreCRUD interface {
		// InsertConfig insert a config entry with version. Return nosqlplugin.NewConditionFailure if the same version of the row_type is existing
		InsertConfig(ctx context.Context, row *persistence.InternalConfigStoreEntry) error
		// SelectLatestConfig returns the config entry of the row_type with the largest(latest) version value
		SelectLatestConfig(ctx context.Context, rowType int) (*persistence.InternalConfigStoreEntry, error)
	}

	/***
	* DomainAuditLogCRUD is for domain audit log storage system
	*
	* Recommendation: use one table
	*
	* Significant columns:
	* domain_audit_log: partition key(domainID, operationType), range key(createdTime DESC, eventID ASC)
	*
	* Note: This table is used for audit trail of domain changes, storing the before/after state
	* of domains along with metadata about who made the change and when.
	 */
	DomainAuditLogCRUD interface {
		// InsertDomainAuditLog inserts a new audit log entry for a domain operation
		// Return error if there is any failure
		InsertDomainAuditLog(ctx context.Context, row *DomainAuditLogRow) error

		// SelectDomainAuditLogs returns audit log entries for a domain and operation type
		// Returns paginated results ordered by created_time DESC, event_id ASC
		SelectDomainAuditLogs(ctx context.Context, filter *DomainAuditLogFilter) ([]*DomainAuditLogRow, []byte, error)
	}
)
