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

package history

import (
	"fmt"
	"sync"
	"time"

	"github.com/pborman/uuid"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/logging"
	"github.com/uber/cadence/common/persistence"

	"github.com/uber-common/bark"
)

type (
	workflowExecutionContext struct {
		domainID          string
		workflowExecution workflow.WorkflowExecution
		shard             ShardContext
		executionManager  persistence.ExecutionManager
		logger            bark.Logger

		sync.Mutex
		msBuilder       *mutableStateBuilder
		updateCondition int64
		deleteTimerTask persistence.Task

		// map of subscriber ID to channel of signal
		subscriberNotificationChan map[string]chan bool
	}
)

var (
	persistenceOperationRetryPolicy = common.CreatePersistanceRetryPolicy()
)

func newWorkflowExecutionContext(domainID string, execution workflow.WorkflowExecution, shard ShardContext,
	executionManager persistence.ExecutionManager, logger bark.Logger) *workflowExecutionContext {
	lg := logger.WithFields(bark.Fields{
		logging.TagWorkflowExecutionID: *execution.WorkflowId,
		logging.TagWorkflowRunID:       *execution.RunId,
	})

	return &workflowExecutionContext{
		domainID:          domainID,
		workflowExecution: execution,
		shard:             shard,
		executionManager:  executionManager,
		logger:            lg,

		subscriberNotificationChan: make(map[string]chan bool),
	}
}

func (c *workflowExecutionContext) loadWorkflowExecution() (*mutableStateBuilder, error) {
	if c.msBuilder != nil {
		return c.msBuilder, nil
	}

	response, err := c.getWorkflowExecutionWithRetry(&persistence.GetWorkflowExecutionRequest{
		DomainID:  c.domainID,
		Execution: c.workflowExecution,
	})
	if err != nil {
		if common.IsPersistenceTransientError(err) {
			logging.LogPersistantStoreErrorEvent(c.logger, logging.TagValueStoreOperationGetWorkflowExecution, err, "")
		}
		return nil, err
	}

	msBuilder := newMutableStateBuilder(c.shard.GetConfig(), c.logger)
	if response != nil && response.State != nil {
		state := response.State
		msBuilder.Load(state)
		info := state.ExecutionInfo
		c.updateCondition = info.NextEventID
	}

	c.msBuilder = msBuilder
	return msBuilder, nil
}

func (c *workflowExecutionContext) updateWorkflowExecutionWithContext(context []byte, transferTasks []persistence.Task,
	timerTasks []persistence.Task, transactionID int64) error {
	c.msBuilder.executionInfo.ExecutionContext = context

	return c.updateWorkflowExecution(transferTasks, timerTasks, transactionID)
}

func (c *workflowExecutionContext) updateWorkflowExecutionWithDeleteTask(transferTasks []persistence.Task,
	timerTasks []persistence.Task, deleteTimerTask persistence.Task, transactionID int64) error {
	c.deleteTimerTask = deleteTimerTask

	return c.updateWorkflowExecution(transferTasks, timerTasks, transactionID)
}

func (c *workflowExecutionContext) updateWorkflowExecution(transferTasks []persistence.Task,
	timerTasks []persistence.Task, transactionID int64) (errRet error) {

	defer func() {
		if errRet != nil {
			// Clear all cached state in case of error
			c.clear()
		}
	}()

	// Take a snapshot of all updates we have accumulated for this execution
	updates, err := c.msBuilder.CloseUpdateSession()
	if err != nil {
		return err
	}

	builder := updates.newEventsBuilder
	if builder.history != nil && len(builder.history) > 0 {
		// Some operations only update the mutable state. For example RecordActivityTaskHeartbeat.
		firstEvent := builder.history[0]
		serializedHistory, err := builder.Serialize()
		if err != nil {
			logging.LogHistorySerializationErrorEvent(c.logger, err, "Unable to serialize execution history for update.")
			return err
		}

		if err0 := c.shard.AppendHistoryEvents(&persistence.AppendHistoryEventsRequest{
			DomainID:      c.domainID,
			Execution:     c.workflowExecution,
			TransactionID: transactionID,
			FirstEventID:  *firstEvent.EventId,
			Events:        serializedHistory,
		}); err0 != nil {
			switch err0.(type) {
			case *persistence.ConditionFailedError:
				return ErrConflict
			}

			logging.LogPersistantStoreErrorEvent(c.logger, logging.TagValueStoreOperationUpdateWorkflowExecution, err0,
				fmt.Sprintf("{updateCondition: %v}", c.updateCondition))
			return err0
		}
	}

	continueAsNew := updates.continueAsNew
	deleteExecution := false
	if c.msBuilder.executionInfo.State == persistence.WorkflowStateCompleted {
		// Workflow execution completed as part of this transaction.
		// Also transactionally delete workflow execution representing current run for the execution
		deleteExecution = true
	}
	if err1 := c.updateWorkflowExecutionWithRetry(&persistence.UpdateWorkflowExecutionRequest{
		ExecutionInfo:             c.msBuilder.executionInfo,
		TransferTasks:             transferTasks,
		TimerTasks:                timerTasks,
		Condition:                 c.updateCondition,
		DeleteTimerTask:           c.deleteTimerTask,
		UpsertActivityInfos:       updates.updateActivityInfos,
		DeleteActivityInfo:        updates.deleteActivityInfo,
		UpserTimerInfos:           updates.updateTimerInfos,
		DeleteTimerInfos:          updates.deleteTimerInfos,
		UpsertChildExecutionInfos: updates.updateChildExecutionInfos,
		DeleteChildExecutionInfo:  updates.deleteChildExecutionInfo,
		NewBufferedEvents:         updates.newBufferedEvents,
		ClearBufferedEvents:       updates.clearBufferedEvents,
		ContinueAsNew:             continueAsNew,
		CloseExecution:            deleteExecution,
	}); err1 != nil {
		switch err1.(type) {
		case *persistence.ConditionFailedError:
			return ErrConflict
		}

		logging.LogPersistantStoreErrorEvent(c.logger, logging.TagValueStoreOperationUpdateWorkflowExecution, err1,
			fmt.Sprintf("{updateCondition: %v}", c.updateCondition))
		return err1
	}

	// Update went through so update the condition for new updates
	c.updateCondition = c.msBuilder.GetNextEventID()
	c.msBuilder.executionInfo.LastUpdatedTimestamp = time.Now()

	// for all subscriber which watch changes of this workflow execution context
	for _, channel := range c.subscriberNotificationChan {
		select {
		case channel <- true:
			// this only notify watcher that there is a change
		default:
			// in case the channel is already filled with message
			// this should NOT happen, unless there is a bug
		}
	}
	return nil
}

func (c *workflowExecutionContext) watchWorkflowExecution() (string, chan bool, error) {
	subscriberID := uuid.NewUUID().String()
	if _, ok := c.subscriberNotificationChan[subscriberID]; ok {
		// UUID collision
		return "", nil, &workflow.InternalServiceError{
			Message: "Unable to watch on workflow execution.",
		}
	}

	channel := make(chan bool, 1)
	c.subscriberNotificationChan[subscriberID] = channel
	return subscriberID, channel, nil
}

func (c *workflowExecutionContext) unwatchWorkflowExecution(subscriberID string) error {
	if _, ok := c.subscriberNotificationChan[subscriberID]; !ok {
		// cannot find the subscribe ID, which means there is a bug
		return &workflow.EntityNotExistsError{
			Message: "Unable to unwatch on workflow execution.",
		}
	}

	delete(c.subscriberNotificationChan, subscriberID)
	return nil
}

func (c *workflowExecutionContext) continueAsNewWorkflowExecution(context []byte, newStateBuilder *mutableStateBuilder,
	transferTasks []persistence.Task, timerTasks []persistence.Task, transactionID int64) error {

	domainID := newStateBuilder.executionInfo.DomainID
	newExecution := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr(newStateBuilder.executionInfo.WorkflowID),
		RunId:      common.StringPtr(newStateBuilder.executionInfo.RunID),
	}
	firstEvent := newStateBuilder.hBuilder.history[0]

	// Serialize the history
	serializedHistory, serializedError := newStateBuilder.hBuilder.Serialize()
	if serializedError != nil {
		logging.LogHistorySerializationErrorEvent(c.logger, serializedError, fmt.Sprintf(
			"HistoryEventBatch serialization error on start workflow.  WorkflowID: %v, RunID: %v", *newExecution.WorkflowId,
			*newExecution.RunId))
		return serializedError
	}

	err1 := c.shard.AppendHistoryEvents(&persistence.AppendHistoryEventsRequest{
		DomainID:  domainID,
		Execution: newExecution,
		// It is ok to use 0 for TransactionID because RunID is unique so there are
		// no potential duplicates to override.
		TransactionID: 0,
		FirstEventID:  *firstEvent.EventId,
		Events:        serializedHistory,
	})
	if err1 != nil {
		return err1
	}

	err2 := c.updateWorkflowExecutionWithContext(context, transferTasks, timerTasks, transactionID)

	if err2 != nil {
		// TODO: Delete new execution if update fails due to conflict or shard being lost
	}

	return err2
}

func (c *workflowExecutionContext) deleteWorkflowExecution() error {
	err := c.deleteWorkflowExecutionWithRetry(&persistence.DeleteWorkflowExecutionRequest{
		ExecutionInfo: c.msBuilder.executionInfo,
	})
	if err != nil && common.IsPersistenceTransientError(err) {
		// Log the error for debugging.  Transfer queue will keep on retrying on the error unless it can delete execution
		logging.LogPersistantStoreErrorEvent(c.logger, logging.TagValueStoreOperationDeleteWorkflowExecution, err,
			fmt.Sprintf("{updateCondition: %v}", c.updateCondition))
	}

	return err
}

func (c *workflowExecutionContext) getWorkflowExecutionWithRetry(
	request *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
	var response *persistence.GetWorkflowExecutionResponse
	op := func() error {
		var err error
		response, err = c.executionManager.GetWorkflowExecution(request)

		return err
	}

	err := backoff.Retry(op, persistenceOperationRetryPolicy, common.IsPersistenceTransientError)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (c *workflowExecutionContext) updateWorkflowExecutionWithRetry(
	request *persistence.UpdateWorkflowExecutionRequest) error {
	op := func() error {
		return c.shard.UpdateWorkflowExecution(request)
	}

	return backoff.Retry(op, persistenceOperationRetryPolicy, common.IsPersistenceTransientError)
}

func (c *workflowExecutionContext) deleteWorkflowExecutionWithRetry(
	request *persistence.DeleteWorkflowExecutionRequest) error {
	op := func() error {
		return c.executionManager.DeleteWorkflowExecution(request)
	}

	return backoff.Retry(op, persistenceOperationRetryPolicy, common.IsPersistenceTransientError)
}

func (c *workflowExecutionContext) clear() {
	c.msBuilder = nil
}
