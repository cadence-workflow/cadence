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

package matching

import (
	"errors"
	"sync"

	"github.com/pborman/uuid"
	"github.com/uber-common/bark"

	"math"

	h "github.com/uber/cadence/.gen/go/history"
	m "github.com/uber/cadence/.gen/go/matching"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/logging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/tchannel-go/thrift"
)

// Implements matching.Engine
// TODO: Switch implementation from lock/channel based to a partitioned agent
// to simplify code and reduce possiblity of synchronization errors.
type matchingEngineImpl struct {
	taskManager     persistence.TaskManager
	historyService  history.Client
	tokenSerializer common.TaskTokenSerializer
	logger          bark.Logger
	metricsClient   metrics.Client
	taskListsLock   sync.RWMutex                   // locks mutation of taskLists
	taskLists       map[taskListID]taskListManager // Convert to LRU cache
	config          *Config
}

type taskListID struct {
	domainID     string
	taskListName string
	taskType     int
}

var (
	// EmptyPollForDecisionTaskResponse is the response when there are no decision tasks to hand out
	emptyPollForDecisionTaskResponse = m.NewPollForDecisionTaskResponse()
	// EmptyPollForActivityTaskResponse is the response when there are no activity tasks to hand out
	emptyPollForActivityTaskResponse   = workflow.NewPollForActivityTaskResponse()
	persistenceOperationRetryPolicy    = common.CreatePersistanceRetryPolicy()
	historyServiceOperationRetryPolicy = common.CreateHistoryServiceRetryPolicy()

	// ErrNoTasks is exported temporarily for integration test
	ErrNoTasks    = errors.New("No tasks")
	errPumpClosed = errors.New("Task list pump closed its channel")
)

func (t *taskListID) String() string {
	var r string
	if t.taskType == persistence.TaskListTypeActivity {
		r += "activity"
	} else {
		r += "decision"
	}
	r += " task list \""
	r += t.taskListName
	r += "\""
	return r
}

var _ Engine = (*matchingEngineImpl)(nil) // Asserts that interface is indeed implemented

// NewEngine creates an instance of matching engine
func NewEngine(taskManager persistence.TaskManager,
	historyService history.Client,
	config *Config,
	logger bark.Logger,
	metricsClient metrics.Client) Engine {

	return &matchingEngineImpl{
		taskManager:     taskManager,
		historyService:  historyService,
		tokenSerializer: common.NewJSONTaskTokenSerializer(),
		taskLists:       make(map[taskListID]taskListManager),
		logger: logger.WithFields(bark.Fields{
			logging.TagWorkflowComponent: logging.TagValueMatchingEngineComponent,
		}),
		metricsClient: metricsClient,
		config:        config,
	}
}

func (e *matchingEngineImpl) Start() {
	// As task lists are initialized lazily nothing is done on startup at this point.
}

func (e *matchingEngineImpl) Stop() {
	// Executes Stop() on each task list outside of lock
	for _, l := range e.getTaskLists(math.MaxInt32) {
		l.Stop()
	}
}

func (e *matchingEngineImpl) getTaskLists(maxCount int) (lists []taskListManager) {
	e.taskListsLock.Lock()
	lists = make([]taskListManager, 0, len(e.taskLists))
	count := 0
	for _, tlMgr := range e.taskLists {
		lists = append(lists, tlMgr)
		count++
		if count >= maxCount {
			break
		}
	}
	e.taskListsLock.Unlock()
	return
}

func (e *matchingEngineImpl) String() string {
	// Executes taskList.String() on each task list outside of lock
	var r string
	for _, l := range e.getTaskLists(1000) {
		r += "\n"
		r += l.String()
	}
	return r
}

// Returns taskListManager for a task list. If not already cached gets new range from DB and if successful creates one.
func (e *matchingEngineImpl) getTaskListManager(taskList *taskListID) (taskListManager, error) {
	e.taskListsLock.RLock()
	if result, ok := e.taskLists[*taskList]; ok {
		e.taskListsLock.RUnlock()
		return result, nil
	}
	e.taskListsLock.RUnlock()
	mgr := newTaskListManager(e, taskList, e.config)
	e.taskListsLock.Lock()
	if result, ok := e.taskLists[*taskList]; ok {
		e.taskListsLock.Unlock()
		return result, nil
	}
	e.taskLists[*taskList] = mgr
	e.taskListsLock.Unlock()
	logging.LogTaskListLoadingEvent(e.logger, taskList.taskListName, taskList.taskType)
	err := mgr.Start()
	if err != nil {
		logging.LogTaskListLoadingFailedEvent(e.logger, taskList.taskListName, taskList.taskType, err)
		return nil, err
	}
	logging.LogTaskListLoadedEvent(e.logger, taskList.taskListName, taskList.taskType)
	return mgr, nil
}

func (e *matchingEngineImpl) removeTaskListManager(id *taskListID) {
	e.taskListsLock.Lock()
	defer e.taskListsLock.Unlock()
	delete(e.taskLists, *id)
}

// AddDecisionTask either delivers task directly to waiting poller or save it into task list persistence.
func (e *matchingEngineImpl) AddDecisionTask(addRequest *m.AddDecisionTaskRequest) error {
	domainID := addRequest.GetDomainUUID()
	taskListName := addRequest.GetTaskList().GetName()
	e.logger.Debugf("Received AddDecisionTask for taskList=%v, WorkflowID=%v, RunID=%v",
		addRequest.TaskList.Name, addRequest.Execution.WorkflowId, addRequest.Execution.RunId)
	taskList := newTaskListID(domainID, taskListName, persistence.TaskListTypeDecision)
	tlMgr, err := e.getTaskListManager(taskList)
	if err != nil {
		return err
	}
	taskInfo := &persistence.TaskInfo{
		DomainID:   domainID,
		RunID:      addRequest.GetExecution().GetRunId(),
		WorkflowID: addRequest.GetExecution().GetWorkflowId(),
		ScheduleID: addRequest.GetScheduleId(),
	}
	return tlMgr.AddTask(addRequest.GetExecution(), taskInfo)
}

// AddActivityTask either delivers task directly to waiting poller or save it into task list persistence.
func (e *matchingEngineImpl) AddActivityTask(addRequest *m.AddActivityTaskRequest) error {
	domainID := addRequest.GetDomainUUID()
	sourceDomainID := addRequest.GetSourceDomainUUID()
	taskListName := addRequest.GetTaskList().GetName()
	e.logger.Debugf("Received AddActivityTask for taskList=%v WorkflowID=%v, RunID=%v",
		taskListName, addRequest.Execution.WorkflowId, addRequest.Execution.RunId)
	taskList := newTaskListID(domainID, taskListName, persistence.TaskListTypeActivity)
	tlMgr, err := e.getTaskListManager(taskList)
	if err != nil {
		return err
	}
	taskInfo := &persistence.TaskInfo{
		DomainID:               sourceDomainID,
		RunID:                  addRequest.GetExecution().GetRunId(),
		WorkflowID:             addRequest.GetExecution().GetWorkflowId(),
		ScheduleID:             addRequest.GetScheduleId(),
		ScheduleToStartTimeout: addRequest.GetScheduleToStartTimeoutSeconds(),
	}
	return tlMgr.AddTask(addRequest.GetExecution(), taskInfo)
}

// PollForDecisionTask tries to get the decision task using exponential backoff.
func (e *matchingEngineImpl) PollForDecisionTask(ctx thrift.Context, req *m.PollForDecisionTaskRequest) (
	*m.PollForDecisionTaskResponse, error) {
	domainID := req.GetDomainUUID()
	request := req.GetPollRequest()
	taskListName := request.GetTaskList().GetName()
	e.logger.Debugf("Received PollForDecisionTask for taskList=%v", taskListName)
pollLoop:
	for {
		err := common.IsValidContext(ctx)
		if err != nil {
			return nil, err
		}

		taskList := newTaskListID(domainID, taskListName, persistence.TaskListTypeDecision)
		tCtx, err := e.getTask(ctx, taskList)
		if err != nil {
			// TODO: Is empty poll the best reply for errPumpClosed?
			if err == ErrNoTasks || err == errPumpClosed {
				return emptyPollForDecisionTaskResponse, nil
			}
			return nil, err
		}

		// Generate a unique requestId for this task which will be used for all retries
		requestID := uuid.New()
		resp, err := tCtx.RecordDecisionTaskStartedWithRetry(&h.RecordDecisionTaskStartedRequest{
			DomainUUID:        common.StringPtr(domainID),
			WorkflowExecution: &tCtx.workflowExecution,
			ScheduleId:        &tCtx.info.ScheduleID,
			TaskId:            &tCtx.info.TaskID,
			RequestId:         common.StringPtr(requestID),
			PollRequest:       request,
		})
		if err != nil {
			switch err.(type) {
			case *workflow.EntityNotExistsError, *h.EventAlreadyStartedError:
				e.logger.Debugf("Duplicated decision task taskList=%v, taskID=%v",
					taskListName, tCtx.info.TaskID)
				tCtx.completeTask(nil)
			default:
				tCtx.completeTask(err)
			}

			continue pollLoop
		}
		tCtx.completeTask(nil)
		return e.createPollForDecisionTaskResponse(tCtx, resp), nil
	}
}

// pollForActivityTaskOperation takes one task from the task manager, update workflow execution history, mark task as
// completed and return it to user. If a task from task manager is already started, return an empty response, without
// error. Timeouts handled by the timer queue.
func (e *matchingEngineImpl) PollForActivityTask(ctx thrift.Context, req *m.PollForActivityTaskRequest) (
	*workflow.PollForActivityTaskResponse, error) {
	domainID := req.GetDomainUUID()
	request := req.GetPollRequest()
	taskListName := request.GetTaskList().GetName()
	e.logger.Debugf("Received PollForActivityTask for taskList=%v", taskListName)
pollLoop:
	for {
		err := common.IsValidContext(ctx)
		if err != nil {
			return nil, err
		}

		taskList := newTaskListID(domainID, taskListName, persistence.TaskListTypeActivity)
		tCtx, err := e.getTask(ctx, taskList)
		if err != nil {
			// TODO: Is empty poll the best reply for errPumpClosed?
			if err == ErrNoTasks || err == errPumpClosed {
				return emptyPollForActivityTaskResponse, nil
			}
			return nil, err
		}
		// Generate a unique requestId for this task which will be used for all retries
		requestID := uuid.New()
		resp, err := tCtx.RecordActivityTaskStartedWithRetry(&h.RecordActivityTaskStartedRequest{
			DomainUUID:        common.StringPtr(domainID),
			WorkflowExecution: &tCtx.workflowExecution,
			ScheduleId:        &tCtx.info.ScheduleID,
			TaskId:            &tCtx.info.TaskID,
			RequestId:         common.StringPtr(requestID),
			PollRequest:       request,
		})
		if err != nil {
			switch err.(type) {
			case *workflow.EntityNotExistsError, *h.EventAlreadyStartedError:
				e.logger.Debugf("Duplicated activity task taskList=%v, taskID=%v",
					taskListName, tCtx.info.TaskID)
				tCtx.completeTask(nil)
			default:
				tCtx.completeTask(err)
			}

			continue pollLoop
		}
		tCtx.completeTask(nil)
		return e.createPollForActivityTaskResponse(tCtx, resp), nil
	}
}

// Loads a task from persistence and wraps it in a task context
func (e *matchingEngineImpl) getTask(ctx thrift.Context, taskList *taskListID) (*taskContext, error) {
	tlMgr, err := e.getTaskListManager(taskList)
	if err != nil {
		return nil, err
	}
	return tlMgr.GetTaskContext(ctx)
}

func (e *matchingEngineImpl) unloadTaskList(id *taskListID) {
	e.taskListsLock.Lock()
	tlMgr, ok := e.taskLists[*id]
	if ok {
		delete(e.taskLists, *id)
	}
	e.taskListsLock.Unlock()
	if ok {
		tlMgr.Stop()
	}
}

// Populate the decision task response based on context and scheduled/started events.
func (e *matchingEngineImpl) createPollForDecisionTaskResponse(context *taskContext,
	historyResponse *h.RecordDecisionTaskStartedResponse) *m.PollForDecisionTaskResponse {
	task := context.info

	response := m.NewPollForDecisionTaskResponse()
	response.WorkflowExecution = workflowExecutionPtr(context.workflowExecution)
	token := &common.TaskToken{
		DomainID:   task.DomainID,
		WorkflowID: task.WorkflowID,
		RunID:      task.RunID,
		ScheduleID: task.ScheduleID,
	}
	response.TaskToken, _ = e.tokenSerializer.Serialize(token)
	response.WorkflowType = historyResponse.GetWorkflowType()
	if historyResponse.GetPreviousStartedEventId() != common.EmptyEventID {
		response.PreviousStartedEventId = historyResponse.PreviousStartedEventId
	}
	response.StartedEventId = historyResponse.StartedEventId

	return response
}

// Populate the activity task response based on context and scheduled/started events.
func (e *matchingEngineImpl) createPollForActivityTaskResponse(context *taskContext,
	historyResponse *h.RecordActivityTaskStartedResponse) *workflow.PollForActivityTaskResponse {
	task := context.info

	startedEvent := historyResponse.StartedEvent
	scheduledEvent := historyResponse.ScheduledEvent
	if !scheduledEvent.IsSetActivityTaskScheduledEventAttributes() {
		panic("GetActivityTaskScheduledEventAttributes is not set")
	}
	attributes := scheduledEvent.GetActivityTaskScheduledEventAttributes()
	if !attributes.IsSetActivityId() {
		panic("ActivityTaskScheduledEventAttributes.ActivityID is not set")
	}

	response := workflow.NewPollForActivityTaskResponse()
	response.ActivityId = attributes.ActivityId
	response.ActivityType = attributes.GetActivityType()
	response.Input = attributes.GetInput()
	response.StartedEventId = common.Int64Ptr(startedEvent.GetEventId())
	response.WorkflowExecution = workflowExecutionPtr(context.workflowExecution)
	response.ScheduledTimestamp = common.Int64Ptr(scheduledEvent.GetTimestamp())
	response.ScheduleToCloseTimeoutSeconds = common.Int32Ptr(attributes.GetScheduleToCloseTimeoutSeconds())
	response.StartedTimestamp = common.Int64Ptr(startedEvent.GetTimestamp())
	response.StartToCloseTimeoutSeconds = common.Int32Ptr(attributes.GetStartToCloseTimeoutSeconds())
	response.HeartbeatTimeoutSeconds = common.Int32Ptr(attributes.GetHeartbeatTimeoutSeconds())

	token := &common.TaskToken{
		DomainID:   task.DomainID,
		WorkflowID: task.WorkflowID,
		RunID:      task.RunID,
		ScheduleID: task.ScheduleID,
	}
	response.TaskToken, _ = e.tokenSerializer.Serialize(token)
	return response
}

func newTaskListID(domainID, taskListName string, taskType int) *taskListID {
	return &taskListID{domainID: domainID, taskListName: taskListName, taskType: taskType}
}

func workflowExecutionPtr(execution workflow.WorkflowExecution) *workflow.WorkflowExecution {
	return &execution
}
