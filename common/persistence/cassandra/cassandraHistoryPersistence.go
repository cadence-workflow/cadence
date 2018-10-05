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

package cassandra

import (
	"fmt"

	"github.com/gocql/gocql"
	"github.com/uber-common/bark"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service/config"
)

const (
	templateAppendHistoryEvents = `INSERT INTO events (` +
		`domain_id, workflow_id, run_id, first_event_id, event_batch_version, range_id, tx_id, data, data_encoding) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`

	templateOverwriteHistoryEvents = `UPDATE events ` +
		`SET event_batch_version = ?, range_id = ?, tx_id = ?, data = ?, data_encoding = ? ` +
		`WHERE domain_id = ? AND workflow_id = ? AND run_id = ? AND first_event_id = ? ` +
		`IF range_id <= ? AND tx_id < ?`

	templateGetWorkflowExecutionHistory = `SELECT first_event_id, event_batch_version, data, data_encoding FROM events ` +
		`WHERE domain_id = ? ` +
		`AND workflow_id = ? ` +
		`AND run_id = ? ` +
		`AND first_event_id >= ? ` +
		`AND first_event_id < ?`

	templateDeleteWorkflowExecutionHistory = `DELETE FROM events ` +
		`WHERE domain_id = ? ` +
		`AND workflow_id = ? ` +
		`AND run_id = ? `

	//Below are V2 templates
	v2templateInsertNode = `INSERT INTO events_v2 (` +
		`tree_id, branch_id, row_type, ancestors, deleted, node_id, txn_id, data, data_encoding) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS `

	v2templateOverrideNode = `UPDATE events_v2 ` +
		`SET txn_id = ?, data = ?, data_encoding = ? ` +
		`WHERE tree_id = ? AND branch_id = ? AND row_type = ? AND node_id = ? ` +
		`IF txn_id < ? `

	v2templateReadNode = `SELECT tree_id, branch_id, row_type, ancestors, deleted, node_id, txn_id, data, data_encoding FROM events_v2 ` +
		`WHERE tree_id = ? AND branch_id = ? AND row_type = ? AND node_id >= ? AND node_id < ? `

	v2templateUpdateTreeRoot = `UPDATE events_v2 ` +
		`SET txn_id = ? ` +
		`WHERE tree_id = ? AND branch_id = ? AND row_type = ? AND node_id = ? ` +
		`IF txn_id = ? `

	v2templateUpdateBranch = `UPDATE events_v2 ` +
		`SET deleted = true ` +
		`WHERE tree_id = ? AND branch_id = ? AND row_type = ? AND node_id = ? ` +
		`IF deleted = false `

	v2templateDeleteNodes = `DELETE FROM events_v2 ` +
		`WHERE tree_id = ? AND branch_id = ? AND row_type = ? AND node_id >= ? AND node_id < ?`

	v2templateDeleteRoot = `DELETE FROM events_v2 ` +
		`WHERE tree_id = ? AND branch_id = ? AND row_type = ? AND node_id = ? ` +
		`IF txn_id =? `
)

const (
	// nodeID of the tree root
	rootNodeID = 0
	// constant branchID for the tree root
	rootNodeBranchId = "10000000-0000-f000-f000-000000000000"
	// the initial txn_id of each node(including root node)
	initialTransactionID = 0

	// Row types for table events_v2
	rowTypeHistoryBranch = iota
	rowTypeHistoryNode
)

type (
	cassandraHistoryPersistence struct {
		cassandraStore
	}
)

// newHistoryPersistence is used to create an instance of HistoryManager implementation
func newHistoryPersistence(cfg config.Cassandra, logger bark.Logger) (p.HistoryStore,
	error) {
	cluster := NewCassandraCluster(cfg.Hosts, cfg.Port, cfg.User, cfg.Password, cfg.Datacenter)
	cluster.Keyspace = cfg.Keyspace
	cluster.ProtoVersion = cassandraProtoVersion
	cluster.Consistency = gocql.LocalQuorum
	cluster.SerialConsistency = gocql.LocalSerial
	cluster.Timeout = defaultSessionTimeout
	if cfg.MaxConns > 0 {
		cluster.NumConns = cfg.MaxConns
	}
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}

	return &cassandraHistoryPersistence{cassandraStore: cassandraStore{session: session, logger: logger}}, nil
}

// Close gracefully releases the resources held by this object
func (h *cassandraHistoryPersistence) Close() {
	if h.session != nil {
		h.session.Close()
	}
}

// NewHistoryBranch creates a new branch from tree root. If tree doesn't exist, then create one. Return error if the branch already exists.
func (h *cassandraHistoryPersistence) NewHistoryBranch(request *p.NewHistoryBranchRequest) error {
	//TODO
	return nil
}

func (h *cassandraHistoryPersistence) createRoot(treeID string) (bool, error) {
	var query *gocql.Query

	query = h.session.Query(v2templateInsertNode,
		treeID,
		rootNodeBranchId,
		rowTypeHistoryNode,
		nil,
		false,
		rootNodeID,
		initialTransactionID,
		nil, //data
		nil) //data_encoding

	previous := make(map[string]interface{})
	applied, err := query.MapScanCAS(previous)
	if err != nil {
		if isThrottlingError(err) {
			return false, &workflow.ServiceBusyError{
				Message: fmt.Sprintf("createRoot operation failed. Error: %v", err),
			}
		} else if isTimeoutError(err) {
			// Write may have succeeded, but we don't know
			// return this info to the caller so they have the option of trying to find out by executing a read
			return false, &p.TimeoutError{Msg: fmt.Sprintf("createRoot timed out. Error: %v", err)}
		}
		return false, &workflow.InternalServiceError{
			Message: fmt.Sprintf("createRoot operation failed. Error: %v", err),
		}
	}

	return applied, nil
}

// AppendHistoryNodes add(or override) a node to a history branch
func (h *cassandraHistoryPersistence) AppendHistoryNode(request *p.InternalAppendHistoryNodeRequest) error {
	//TODO
	return nil
}

// ReadHistoryBranch returns history node data for a branch
func (h *cassandraHistoryPersistence) ReadHistoryBranch(request *p.InternalReadHistoryBranchRequest) (*p.InternalReadHistoryBranchResponse, error) {
	//TODO
	return nil, nil
}

// ForkHistoryBranch forks a new branch from a old branch
func (h *cassandraHistoryPersistence) ForkHistoryBranch(request *p.ForkHistoryBranchRequest) (*p.ForkHistoryBranchResponse, error) {
	//TODO
	return nil, nil
}

// DeleteHistoryBranch removes a branch
func (h *cassandraHistoryPersistence) DeleteHistoryBranch(request *p.DeleteHistoryBranchRequest) error {
	//TODO
	return nil
}

// GetHistoryTree returns all branch information of a tree
func (h *cassandraHistoryPersistence) GetHistoryTree(request *p.GetHistoryTreeRequest) (*p.GetHistoryTreeResponse, error) {
	//TODO
	return nil, nil
}

func (h *cassandraHistoryPersistence) AppendHistoryEvents(request *p.InternalAppendHistoryEventsRequest) error {
	var query *gocql.Query

	if request.Overwrite {
		query = h.session.Query(templateOverwriteHistoryEvents,
			request.EventBatchVersion,
			request.RangeID,
			request.TransactionID,
			request.Events.Data,
			request.Events.Encoding,
			request.DomainID,
			*request.Execution.WorkflowId,
			*request.Execution.RunId,
			request.FirstEventID,
			request.RangeID,
			request.TransactionID)
	} else {
		query = h.session.Query(templateAppendHistoryEvents,
			request.DomainID,
			*request.Execution.WorkflowId,
			*request.Execution.RunId,
			request.FirstEventID,
			request.EventBatchVersion,
			request.RangeID,
			request.TransactionID,
			request.Events.Data,
			request.Events.Encoding)
	}

	previous := make(map[string]interface{})
	applied, err := query.MapScanCAS(previous)
	if err != nil {
		if isThrottlingError(err) {
			return &workflow.ServiceBusyError{
				Message: fmt.Sprintf("AppendHistoryEvents operation failed. Error: %v", err),
			}
		} else if isTimeoutError(err) {
			// Write may have succeeded, but we don't know
			// return this info to the caller so they have the option of trying to find out by executing a read
			return &p.TimeoutError{Msg: fmt.Sprintf("AppendHistoryEvents timed out. Error: %v", err)}
		}
		return &workflow.InternalServiceError{
			Message: fmt.Sprintf("AppendHistoryEvents operation failed. Error: %v", err),
		}
	}

	if !applied {
		return &p.ConditionFailedError{
			Msg: "Failed to append history events.",
		}
	}

	return nil
}

func (h *cassandraHistoryPersistence) GetWorkflowExecutionHistory(request *p.InternalGetWorkflowExecutionHistoryRequest) (
	*p.InternalGetWorkflowExecutionHistoryResponse, error) {
	execution := request.Execution
	query := h.session.Query(templateGetWorkflowExecutionHistory,
		request.DomainID,
		*execution.WorkflowId,
		*execution.RunId,
		request.FirstEventID,
		request.NextEventID)

	iter := query.PageSize(request.PageSize).PageState(request.NextPageToken).Iter()
	if iter == nil {
		return nil, &workflow.InternalServiceError{
			Message: "GetWorkflowExecutionHistory operation failed.  Not able to create query iterator.",
		}
	}

	found := false
	nextPageToken := iter.PageState()

	//NOTE: in this method, we need to make sure eventBatchVersion is NOT decreasing(otherwise we skip the events)
	lastEventBatchVersion := request.LastEventBatchVersion

	eventBatchVersionPointer := new(int64)
	eventBatchVersion := common.EmptyVersion

	eventBatch := &p.DataBlob{}
	history := make([]*p.DataBlob, 0)

	for iter.Scan(nil, &eventBatchVersionPointer, &eventBatch.Data, &eventBatch.Encoding) {
		found = true

		if eventBatchVersionPointer != nil {
			eventBatchVersion = *eventBatchVersionPointer
		}
		if eventBatchVersion >= lastEventBatchVersion {
			history = append(history, eventBatch)
			lastEventBatchVersion = eventBatchVersion
		}

		eventBatchVersionPointer = new(int64)
		eventBatchVersion = common.EmptyVersion
		eventBatch = &p.DataBlob{}
	}

	if err := iter.Close(); err != nil {
		return nil, &workflow.InternalServiceError{
			Message: fmt.Sprintf("GetWorkflowExecutionHistory operation failed. Error: %v", err),
		}
	}

	if !found && len(request.NextPageToken) == 0 {
		// adding the check of request next token being not nil, since
		// there can be case when found == false at the very end of pagination.
		return nil, &workflow.EntityNotExistsError{
			Message: fmt.Sprintf("Workflow execution history not found.  WorkflowId: %v, RunId: %v",
				*execution.WorkflowId, *execution.RunId),
		}
	}

	response := &p.InternalGetWorkflowExecutionHistoryResponse{
		NextPageToken:         nextPageToken,
		History:               history,
		LastEventBatchVersion: lastEventBatchVersion,
	}

	return response, nil
}

func (h *cassandraHistoryPersistence) DeleteWorkflowExecutionHistory(
	request *p.DeleteWorkflowExecutionHistoryRequest) error {
	execution := request.Execution
	query := h.session.Query(templateDeleteWorkflowExecutionHistory,
		request.DomainID,
		*execution.WorkflowId,
		*execution.RunId)

	err := query.Exec()
	if err != nil {
		if isThrottlingError(err) {
			return &workflow.ServiceBusyError{
				Message: fmt.Sprintf("DeleteWorkflowExecutionHistory operation failed. Error: %v", err),
			}
		}
		return &workflow.InternalServiceError{
			Message: fmt.Sprintf("DeleteWorkflowExecutionHistory operation failed. Error: %v", err),
		}
	}

	return nil
}
