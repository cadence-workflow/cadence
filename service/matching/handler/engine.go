// Copyright (c) 2017-2020 Uber Technologies Inc.

// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.

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

package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/event"
	"github.com/uber/cadence/service/matching/tasklist"
)

// If sticky poller is not seem in last 10s, we treat it as sticky worker unavailable
// This seems aggressive, but the default sticky schedule_to_start timeout is 5s, so 10s seems reasonable.
const _stickyPollerUnavailableWindow = 10 * time.Second

// Implements matching.Engine
// TODO: Switch implementation from lock/channel based to a partitioned agent
// to simplify code and reduce possibility of synchronization errors.
type (
	queryResult struct {
		workerResponse *types.MatchingRespondQueryTaskCompletedRequest
		internalError  error
	}

	// lockableQueryTaskMap maps query TaskID (which is a UUID generated in QueryWorkflow() call) to a channel
	// that QueryWorkflow() will block on. The channel is unblocked either by worker sending response through
	// RespondQueryTaskCompleted() or through an internal service error causing cadence to be unable to dispatch
	// query task to workflow worker.
	lockableQueryTaskMap struct {
		sync.RWMutex
		queryTaskMap map[string]chan *queryResult
	}

	matchingEngineImpl struct {
		shutdownCompletion   *sync.WaitGroup
		shutdown             chan struct{}
		taskManager          persistence.TaskManager
		clusterMetadata      cluster.Metadata
		historyService       history.Client
		matchingClient       matching.Client
		tokenSerializer      common.TaskTokenSerializer
		logger               log.Logger
		metricsClient        metrics.Client
		taskListsLock        sync.RWMutex                             // locks mutation of taskLists
		taskLists            map[tasklist.Identifier]tasklist.Manager // Convert to LRU cache
		config               *config.Config
		lockableQueryTaskMap lockableQueryTaskMap
		domainCache          cache.DomainCache
		versionChecker       client.VersionChecker
		membershipResolver   membership.Resolver
		isolationState       isolationgroup.State
		timeSource           clock.TimeSource

		waitForQueryResultFn func(hCtx *handlerContext, isStrongConsistencyQuery bool, queryResultCh <-chan *queryResult) (*types.QueryWorkflowResponse, error)
	}

	// HistoryInfo consists of two integer regarding the history size and history count
	// HistoryInfo struct {
	//	historySize  int64
	//	historyCount int64
	// }
)

var (
	// EmptyPollForDecisionTaskResponse is the response when there are no decision tasks to hand out
	emptyPollForDecisionTaskResponse = &types.MatchingPollForDecisionTaskResponse{}
	// EmptyPollForActivityTaskResponse is the response when there are no activity tasks to hand out
	emptyPollForActivityTaskResponse   = &types.MatchingPollForActivityTaskResponse{}
	historyServiceOperationRetryPolicy = common.CreateHistoryServiceRetryPolicy()

	errPumpClosed = errors.New("task list pump closed its channel")

	_stickyPollerUnavailableError = &types.StickyWorkerUnavailableError{Message: "sticky worker is unavailable, please use non-sticky task list."}
)

var _ Engine = (*matchingEngineImpl)(nil) // Asserts that interface is indeed implemented

// NewEngine creates an instance of matching engine
func NewEngine(
	taskManager persistence.TaskManager,
	clusterMetadata cluster.Metadata,
	historyService history.Client,
	matchingClient matching.Client,
	config *config.Config,
	logger log.Logger,
	metricsClient metrics.Client,
	domainCache cache.DomainCache,
	resolver membership.Resolver,
	isolationState isolationgroup.State,
	timeSource clock.TimeSource,
) Engine {

	e := &matchingEngineImpl{
		shutdown:             make(chan struct{}),
		shutdownCompletion:   &sync.WaitGroup{},
		taskManager:          taskManager,
		clusterMetadata:      clusterMetadata,
		historyService:       historyService,
		tokenSerializer:      common.NewJSONTaskTokenSerializer(),
		taskLists:            make(map[tasklist.Identifier]tasklist.Manager),
		logger:               logger.WithTags(tag.ComponentMatchingEngine),
		metricsClient:        metricsClient,
		matchingClient:       matchingClient,
		config:               config,
		lockableQueryTaskMap: lockableQueryTaskMap{queryTaskMap: make(map[string]chan *queryResult)},
		domainCache:          domainCache,
		versionChecker:       client.NewVersionChecker(),
		membershipResolver:   resolver,
		isolationState:       isolationState,
		timeSource:           timeSource,
	}

	e.shutdownCompletion.Add(1)
	go e.subscribeToMembershipChanges()

	e.waitForQueryResultFn = e.waitForQueryResult
	return e
}

func (e *matchingEngineImpl) Start() {
}

func (e *matchingEngineImpl) Stop() {
	close(e.shutdown)
	// Executes Stop() on each task list outside of lock
	for _, l := range e.getTaskLists(math.MaxInt32) {
		l.Stop()
	}
	e.shutdownCompletion.Wait()
}

func (e *matchingEngineImpl) getTaskLists(maxCount int) []tasklist.Manager {
	e.taskListsLock.RLock()
	defer e.taskListsLock.RUnlock()
	lists := make([]tasklist.Manager, 0, len(e.taskLists))
	count := 0
	for _, tlMgr := range e.taskLists {
		lists = append(lists, tlMgr)
		count++
		if count >= maxCount {
			break
		}
	}
	return lists
}

func (e *matchingEngineImpl) String() string {
	// Executes taskList.String() on each task list outside of lock
	buf := new(bytes.Buffer)
	for _, l := range e.getTaskLists(1000) {
		fmt.Fprintf(buf, "\n%s", l.String())
	}
	return buf.String()
}

// Returns taskListManager for a task list. If not already cached gets new range from DB and
// if successful creates one.
func (e *matchingEngineImpl) getTaskListManager(taskList *tasklist.Identifier, taskListKind *types.TaskListKind) (tasklist.Manager, error) {
	// The first check is an optimization so almost all requests will have a task list manager
	// and return avoiding the write lock
	e.taskListsLock.RLock()
	if result, ok := e.taskLists[*taskList]; ok {
		e.taskListsLock.RUnlock()
		return result, nil
	}
	e.taskListsLock.RUnlock()

	err := e.errIfShardLoss(taskList)
	if err != nil {
		return nil, err
	}

	// If it gets here, write lock and check again in case a task list is created between the two locks
	e.taskListsLock.Lock()
	if result, ok := e.taskLists[*taskList]; ok {
		e.taskListsLock.Unlock()
		return result, nil
	}

	// common tagged logger
	logger := e.logger.WithTags(
		tag.WorkflowTaskListName(taskList.GetName()),
		tag.WorkflowTaskListType(taskList.GetType()),
		tag.WorkflowDomainID(taskList.GetDomainID()),
	)

	logger.Info("Task list manager state changed", tag.LifeCycleStarting)
	mgr, err := tasklist.NewManager(
		e.domainCache,
		e.logger,
		e.metricsClient,
		e.taskManager,
		e.clusterMetadata,
		e.isolationState,
		e.matchingClient,
		e.removeTaskListManager,
		taskList,
		taskListKind,
		e.config,
		e.timeSource,
		e.timeSource.Now(),
		e.historyService,
	)
	if err != nil {
		e.taskListsLock.Unlock()
		logger.Info("Task list manager state changed", tag.LifeCycleStartFailed, tag.Error(err))
		return nil, err
	}

	e.taskLists[*taskList] = mgr
	e.metricsClient.Scope(metrics.MatchingTaskListMgrScope).UpdateGauge(
		metrics.TaskListManagersGauge,
		float64(len(e.taskLists)),
	)
	e.taskListsLock.Unlock()
	err = mgr.Start()
	if err != nil {
		logger.Info("Task list manager state changed", tag.LifeCycleStartFailed, tag.Error(err))
		return nil, err
	}
	logger.Info("Task list manager state changed", tag.LifeCycleStarted)
	event.Log(event.E{
		TaskListName: taskList.GetName(),
		TaskListKind: taskListKind,
		TaskListType: taskList.GetType(),
		TaskInfo: persistence.TaskInfo{
			DomainID: taskList.GetDomainID(),
		},
		EventName: "TaskListManager Started",
		Host:      e.config.HostName,
	})
	return mgr, nil
}

func (e *matchingEngineImpl) getTaskListByDomainLocked(domainID string) *types.GetTaskListsByDomainResponse {
	decisionTaskListMap := make(map[string]*types.DescribeTaskListResponse)
	activityTaskListMap := make(map[string]*types.DescribeTaskListResponse)
	for tl, tlm := range e.taskLists {
		if tl.GetDomainID() == domainID && tlm.GetTaskListKind() == types.TaskListKindNormal {
			if types.TaskListType(tl.GetType()) == types.TaskListTypeDecision {
				decisionTaskListMap[tl.GetRoot()] = tlm.DescribeTaskList(false)
			} else {
				activityTaskListMap[tl.GetRoot()] = tlm.DescribeTaskList(false)
			}
		}
	}
	return &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: decisionTaskListMap,
		ActivityTaskListMap: activityTaskListMap,
	}
}

// For use in tests
func (e *matchingEngineImpl) updateTaskList(taskList *tasklist.Identifier, mgr tasklist.Manager) {
	e.taskListsLock.Lock()
	defer e.taskListsLock.Unlock()
	e.taskLists[*taskList] = mgr
}

func (e *matchingEngineImpl) removeTaskListManager(tlMgr tasklist.Manager) {
	id := tlMgr.TaskListID()
	e.taskListsLock.Lock()
	defer e.taskListsLock.Unlock()
	currentTlMgr, ok := e.taskLists[*id]
	if ok && tlMgr == currentTlMgr {
		delete(e.taskLists, *id)
	}

	e.metricsClient.Scope(metrics.MatchingTaskListMgrScope).UpdateGauge(
		metrics.TaskListManagersGauge,
		float64(len(e.taskLists)),
	)
}

// AddDecisionTask either delivers task directly to waiting poller or save it into task list persistence.
func (e *matchingEngineImpl) AddDecisionTask(
	hCtx *handlerContext,
	request *types.AddDecisionTaskRequest,
) (*types.AddDecisionTaskResponse, error) {
	startT := time.Now()
	domainID := request.GetDomainUUID()
	taskListName := request.GetTaskList().GetName()
	taskListKind := request.GetTaskList().Kind
	taskListType := persistence.TaskListTypeDecision

	event.Log(event.E{
		TaskListName: taskListName,
		TaskListKind: taskListKind,
		TaskListType: taskListType,
		TaskInfo: persistence.TaskInfo{
			DomainID:        domainID,
			ScheduleID:      request.GetScheduleID(),
			PartitionConfig: request.GetPartitionConfig(),
		},
		EventName: "Received AddDecisionTask",
		Host:      e.config.HostName,
		Payload: map[string]any{
			"RequestForwardedFrom": request.GetForwardedFrom(),
		},
	})
	e.emitInfoOrDebugLog(
		domainID,
		"Received AddDecisionTask",
		tag.WorkflowTaskListName(taskListName),
		tag.WorkflowID(request.Execution.GetWorkflowID()),
		tag.WorkflowRunID(request.Execution.GetRunID()),
		tag.WorkflowDomainID(domainID),
		tag.WorkflowTaskListType(taskListType),
		tag.WorkflowScheduleID(request.GetScheduleID()),
		tag.WorkflowTaskListKind(int32(request.GetTaskList().GetKind())),
	)

	taskListID, err := tasklist.NewIdentifier(domainID, taskListName, taskListType)
	if err != nil {
		return nil, err
	}

	// get the domainName
	domainName, err := e.domainCache.GetDomainName(domainID)
	if err != nil {
		return nil, err
	}

	// Only emit traffic metrics if the tasklist is not sticky and is not forwarded
	if int32(request.GetTaskList().GetKind()) == 0 && request.ForwardedFrom == "" {
		e.metricsClient.Scope(metrics.MatchingAddTaskScope).Tagged(metrics.DomainTag(domainName),
			metrics.TaskListTag(taskListName), metrics.TaskListTypeTag("decision_task"),
			metrics.MatchingHostTag(e.config.HostName)).IncCounter(metrics.CadenceTasklistRequests)
		e.emitInfoOrDebugLog(domainID, "Emitting tasklist counter on decision task",
			tag.WorkflowTaskListName(taskListName),
			tag.Dynamic("taskListBaseName", taskListID.GetRoot()))
	}

	tlMgr, err := e.getTaskListManager(taskListID, taskListKind)
	if err != nil {
		return nil, err
	}

	if taskListKind != nil && *taskListKind == types.TaskListKindSticky {
		// check if the sticky worker is still available, if not, fail this request early
		if !tlMgr.HasPollerAfter(e.timeSource.Now().Add(-_stickyPollerUnavailableWindow)) {
			return nil, _stickyPollerUnavailableError
		}
	}

	taskInfo := &persistence.TaskInfo{
		DomainID:                      domainID,
		RunID:                         request.Execution.GetRunID(),
		WorkflowID:                    request.Execution.GetWorkflowID(),
		ScheduleID:                    request.GetScheduleID(),
		ScheduleToStartTimeoutSeconds: request.GetScheduleToStartTimeoutSeconds(),
		CreatedTime:                   e.timeSource.Now(),
		PartitionConfig:               request.GetPartitionConfig(),
	}

	syncMatched, err := tlMgr.AddTask(hCtx.Context, tasklist.AddTaskParams{
		TaskInfo:      taskInfo,
		Source:        request.GetSource(),
		ForwardedFrom: request.GetForwardedFrom(),
	})
	if err != nil {
		return nil, err
	}
	if syncMatched {
		hCtx.scope.RecordTimer(metrics.SyncMatchLatencyPerTaskList, time.Since(startT))
	}
	return &types.AddDecisionTaskResponse{
		PartitionConfig: tlMgr.TaskListPartitionConfig(),
	}, nil
}

// AddActivityTask either delivers task directly to waiting poller or save it into task list persistence.
func (e *matchingEngineImpl) AddActivityTask(
	hCtx *handlerContext,
	request *types.AddActivityTaskRequest,
) (*types.AddActivityTaskResponse, error) {
	startT := time.Now()
	domainID := request.GetDomainUUID()
	taskListName := request.GetTaskList().GetName()
	taskListKind := request.GetTaskList().Kind
	taskListType := persistence.TaskListTypeActivity

	e.emitInfoOrDebugLog(
		domainID,
		"Received AddActivityTask",
		tag.WorkflowTaskListName(taskListName),
		tag.WorkflowID(request.Execution.GetWorkflowID()),
		tag.WorkflowRunID(request.Execution.GetRunID()),
		tag.WorkflowDomainID(domainID),
		tag.WorkflowTaskListType(taskListType),
		tag.WorkflowScheduleID(request.GetScheduleID()),
		tag.WorkflowTaskListKind(int32(request.GetTaskList().GetKind())),
	)

	taskListID, err := tasklist.NewIdentifier(domainID, taskListName, taskListType)
	if err != nil {
		return nil, err
	}

	// get the domainName
	domainName, err := e.domainCache.GetDomainName(domainID)
	if err != nil {
		return nil, err
	}

	// Only emit traffic metrics if the tasklist is not sticky and is not forwarded
	if int32(request.GetTaskList().GetKind()) == 0 && request.ForwardedFrom == "" {
		e.metricsClient.Scope(metrics.MatchingAddTaskScope).Tagged(metrics.DomainTag(domainName),
			metrics.TaskListTag(taskListName), metrics.TaskListTypeTag("activity_task"),
			metrics.MatchingHostTag(e.config.HostName)).IncCounter(metrics.CadenceTasklistRequests)
		e.emitInfoOrDebugLog(domainID, "Emitting tasklist counter on activity task",
			tag.WorkflowTaskListName(taskListName),
			tag.Dynamic("taskListBaseName", taskListID.GetRoot()))
	}

	tlMgr, err := e.getTaskListManager(taskListID, taskListKind)
	if err != nil {
		return nil, err
	}

	taskInfo := &persistence.TaskInfo{
		DomainID:                      request.GetSourceDomainUUID(),
		RunID:                         request.Execution.GetRunID(),
		WorkflowID:                    request.Execution.GetWorkflowID(),
		ScheduleID:                    request.GetScheduleID(),
		ScheduleToStartTimeoutSeconds: request.GetScheduleToStartTimeoutSeconds(),
		CreatedTime:                   e.timeSource.Now(),
		PartitionConfig:               request.GetPartitionConfig(),
	}

	syncMatched, err := tlMgr.AddTask(hCtx.Context, tasklist.AddTaskParams{
		TaskInfo:                 taskInfo,
		Source:                   request.GetSource(),
		ForwardedFrom:            request.GetForwardedFrom(),
		ActivityTaskDispatchInfo: request.ActivityTaskDispatchInfo,
	})
	if err != nil {
		return nil, err
	}
	if syncMatched {
		hCtx.scope.RecordTimer(metrics.SyncMatchLatencyPerTaskList, time.Since(startT))
	}
	return &types.AddActivityTaskResponse{
		PartitionConfig: tlMgr.TaskListPartitionConfig(),
	}, nil
}

// PollForDecisionTask tries to get the decision task using exponential backoff.
func (e *matchingEngineImpl) PollForDecisionTask(
	hCtx *handlerContext,
	req *types.MatchingPollForDecisionTaskRequest,
) (*types.MatchingPollForDecisionTaskResponse, error) {
	domainID := req.GetDomainUUID()
	pollerID := req.GetPollerID()
	request := req.PollRequest
	taskListName := request.GetTaskList().GetName()
	taskListKind := request.GetTaskList().Kind
	e.logger.Debug("Received PollForDecisionTask for taskList",
		tag.WorkflowTaskListName(taskListName),
		tag.WorkflowDomainID(domainID),
	)
	event.Log(event.E{
		TaskListName: taskListName,
		TaskListKind: taskListKind,
		TaskListType: persistence.TaskListTypeDecision,
		TaskInfo: persistence.TaskInfo{
			DomainID: domainID,
		},
		EventName: "Received PollForDecisionTask",
		Host:      e.config.HostName,
		Payload: map[string]any{
			"RequestForwardedFrom": req.GetForwardedFrom(),
			"IsolationGroup":       req.GetIsolationGroup(),
		},
	})
pollLoop:
	for {
		if err := common.IsValidContext(hCtx.Context); err != nil {
			return nil, err
		}

		taskListID, err := tasklist.NewIdentifier(domainID, taskListName, persistence.TaskListTypeDecision)
		if err != nil {
			return nil, fmt.Errorf("couldn't create new decision tasklist %w", err)
		}

		// Add frontend generated pollerID to context so tasklistMgr can support cancellation of
		// long-poll when frontend calls CancelOutstandingPoll API
		pollerCtx := tasklist.ContextWithPollerID(hCtx.Context, pollerID)
		pollerCtx = tasklist.ContextWithIdentity(pollerCtx, request.GetIdentity())
		pollerCtx = tasklist.ContextWithIsolationGroup(pollerCtx, req.GetIsolationGroup())
		tlMgr, err := e.getTaskListManager(taskListID, taskListKind)
		if err != nil {
			return nil, fmt.Errorf("couldn't load tasklist namanger: %w", err)
		}
		startT := time.Now() // Record the start time
		task, err := tlMgr.GetTask(pollerCtx, nil)
		if err != nil {
			// TODO: Is empty poll the best reply for errPumpClosed?
			if errors.Is(err, tasklist.ErrNoTasks) || errors.Is(err, errPumpClosed) {
				e.logger.Debug("no decision tasks",
					tag.WorkflowTaskListName(taskListName),
					tag.WorkflowDomainID(domainID),
					tag.Error(err),
				)
				event.Log(event.E{
					TaskListName: taskListName,
					TaskListKind: taskListKind,
					TaskListType: persistence.TaskListTypeDecision,
					TaskInfo: persistence.TaskInfo{
						DomainID: domainID,
					},
					EventName: "PollForDecisionTask returned no tasks",
					Host:      e.config.HostName,
					Payload: map[string]any{
						"RequestForwardedFrom": req.GetForwardedFrom(),
					},
				})
				domainName, _ := e.domainCache.GetDomainName(domainID)
				return &types.MatchingPollForDecisionTaskResponse{
					PartitionConfig:   tlMgr.TaskListPartitionConfig(),
					LoadBalancerHints: tlMgr.LoadBalancerHints(),
					AutoConfigHint: &types.AutoConfigHint{
						EnableAutoConfig:   e.config.EnableClientAutoConfig(domainName, taskListName, persistence.TaskListTypeDecision),
						PollerWaitTimeInMs: time.Since(startT).Milliseconds(),
					},
				}, nil
			}
			return nil, fmt.Errorf("couldn't get task: %w", err)
		}

		if task.IsStarted() {
			event.Log(event.E{
				TaskListName: taskListName,
				TaskListKind: taskListKind,
				TaskListType: persistence.TaskListTypeDecision,
				TaskInfo:     task.Info(),
				EventName:    "PollForDecisionTask returning already started task",
				Host:         e.config.HostName,
			})
			resp := task.PollForDecisionResponse()
			resp.PartitionConfig = tlMgr.TaskListPartitionConfig()
			resp.LoadBalancerHints = tlMgr.LoadBalancerHints()
			resp.AutoConfigHint = task.AutoConfigHint
			return resp, nil
			// TODO: Maybe add history expose here?
		}

		e.emitForwardedFromStats(hCtx.scope, task.IsForwarded(), req.GetForwardedFrom())
		if task.IsQuery() {
			task.Finish(nil) // this only means query task sync match succeed.

			// for query task, we don't need to update history to record decision task started. but we need to know
			// the NextEventID so front end knows what are the history events to load for this decision task.
			mutableStateResp, err := e.historyService.GetMutableState(hCtx.Context, &types.GetMutableStateRequest{
				DomainUUID: req.DomainUUID,
				Execution:  task.WorkflowExecution(),
			})
			if err != nil {
				// will notify query client that the query task failed
				e.deliverQueryResult(task.Query.TaskID, &queryResult{internalError: err}) //nolint:errcheck
				return &types.MatchingPollForDecisionTaskResponse{
					PartitionConfig:   tlMgr.TaskListPartitionConfig(),
					LoadBalancerHints: tlMgr.LoadBalancerHints(),
				}, nil
			}

			isStickyEnabled := false
			supportsSticky := e.versionChecker.SupportsStickyQuery(mutableStateResp.GetClientImpl(), mutableStateResp.GetClientFeatureVersion()) == nil
			if len(mutableStateResp.StickyTaskList.GetName()) != 0 && supportsSticky {
				isStickyEnabled = true
			}
			resp := &types.RecordDecisionTaskStartedResponse{
				PreviousStartedEventID:    mutableStateResp.PreviousStartedEventID,
				NextEventID:               mutableStateResp.NextEventID,
				WorkflowType:              mutableStateResp.WorkflowType,
				StickyExecutionEnabled:    isStickyEnabled,
				WorkflowExecutionTaskList: mutableStateResp.TaskList,
				BranchToken:               mutableStateResp.CurrentBranchToken,
				HistorySize:               mutableStateResp.HistorySize,
			}
			return e.createPollForDecisionTaskResponse(task, resp, hCtx.scope, tlMgr.TaskListPartitionConfig(), tlMgr.LoadBalancerHints()), nil
		}

		e.emitTaskIsolationMetrics(hCtx.scope, task.Event.PartitionConfig, req.GetIsolationGroup())
		resp, err := e.recordDecisionTaskStarted(hCtx.Context, request, task)

		if err != nil {
			switch err.(type) {
			case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError, *types.EventAlreadyStartedError:
				domainName, _ := e.domainCache.GetDomainName(domainID)
				hCtx.scope.
					Tagged(metrics.DomainTag(domainName)).
					Tagged(metrics.TaskListTag(taskListName)).
					IncCounter(metrics.PollDecisionTaskAlreadyStartedCounterPerTaskList)

				e.emitInfoOrDebugLog(
					task.Event.DomainID,
					"Duplicated decision task",
					tag.WorkflowDomainID(domainID),
					tag.WorkflowID(task.Event.WorkflowID),
					tag.WorkflowRunID(task.Event.RunID),
					tag.WorkflowTaskListName(taskListName),
					tag.WorkflowScheduleID(task.Event.ScheduleID),
					tag.TaskID(task.Event.TaskID),
					tag.Error(err),
				)
				task.Finish(nil)
			default:
				e.emitInfoOrDebugLog(
					task.Event.DomainID,
					"unknown error recording task started",
					tag.WorkflowDomainID(domainID),
					tag.Error(err),
					tag.WorkflowTaskListName(taskListName),
				)
				task.Finish(err)
			}

			continue pollLoop
		}

		task.Finish(nil)
		event.Log(event.E{
			TaskListName: taskListName,
			TaskListKind: taskListKind,
			TaskListType: persistence.TaskListTypeDecision,
			TaskInfo:     task.Info(),
			EventName:    "PollForDecisionTask returning task",
			Host:         e.config.HostName,
			Payload: map[string]any{
				"TaskIsForwarded":      task.IsForwarded(),
				"RequestForwardedFrom": req.GetForwardedFrom(),
				"Latency":              time.Since(task.Info().CreatedTime).Milliseconds(),
				"IsolationGroup":       req.GetIsolationGroup(),
			},
		})

		return e.createPollForDecisionTaskResponse(task, resp, hCtx.scope, tlMgr.TaskListPartitionConfig(), tlMgr.LoadBalancerHints()), nil
	}
}

// pollForActivityTaskOperation takes one task from the task manager, update workflow execution history, mark task as
// completed and return it to user. If a task from task manager is already started, return an empty response, without
// error. Timeouts handled by the timer queue.
func (e *matchingEngineImpl) PollForActivityTask(
	hCtx *handlerContext,
	req *types.MatchingPollForActivityTaskRequest,
) (*types.MatchingPollForActivityTaskResponse, error) {
	domainID := req.GetDomainUUID()
	pollerID := req.GetPollerID()
	request := req.PollRequest
	taskListName := request.GetTaskList().GetName()
	e.logger.Debug("Received PollForActivityTask",
		tag.WorkflowTaskListName(taskListName),
		tag.WorkflowDomainID(domainID),
	)

pollLoop:
	for {
		err := common.IsValidContext(hCtx.Context)
		if err != nil {
			return nil, err
		}

		taskListID, err := tasklist.NewIdentifier(domainID, taskListName, persistence.TaskListTypeActivity)
		if err != nil {
			return nil, err
		}

		var maxDispatch *float64
		if request.TaskListMetadata != nil {
			maxDispatch = request.TaskListMetadata.MaxTasksPerSecond
		}
		// Add frontend generated pollerID to context so tasklistMgr can support cancellation of
		// long-poll when frontend calls CancelOutstandingPoll API
		pollerCtx := tasklist.ContextWithPollerID(hCtx.Context, pollerID)
		pollerCtx = tasklist.ContextWithIdentity(pollerCtx, request.GetIdentity())
		pollerCtx = tasklist.ContextWithIsolationGroup(pollerCtx, req.GetIsolationGroup())
		taskListKind := request.TaskList.Kind
		tlMgr, err := e.getTaskListManager(taskListID, taskListKind)
		if err != nil {
			return nil, fmt.Errorf("couldn't load tasklist namanger: %w", err)
		}
		startT := time.Now() // Record the start time
		task, err := tlMgr.GetTask(pollerCtx, maxDispatch)
		if err != nil {
			// TODO: Is empty poll the best reply for errPumpClosed?
			if errors.Is(err, tasklist.ErrNoTasks) || errors.Is(err, errPumpClosed) {
				domainName, _ := e.domainCache.GetDomainName(domainID)
				return &types.MatchingPollForActivityTaskResponse{
					PartitionConfig:   tlMgr.TaskListPartitionConfig(),
					LoadBalancerHints: tlMgr.LoadBalancerHints(),
					AutoConfigHint: &types.AutoConfigHint{
						EnableAutoConfig:   e.config.EnableClientAutoConfig(domainName, taskListName, persistence.TaskListTypeDecision),
						PollerWaitTimeInMs: time.Since(startT).Milliseconds(),
					},
				}, nil
			}
			e.logger.Error("Received unexpected err while getting task",
				tag.WorkflowTaskListName(taskListName),
				tag.WorkflowDomainID(domainID),
				tag.Error(err),
			)
			return nil, err
		}

		if task.IsStarted() {
			// tasks received from remote are already started. So, simply forward the response
			resp := task.PollForActivityResponse()
			resp.PartitionConfig = tlMgr.TaskListPartitionConfig()
			resp.LoadBalancerHints = tlMgr.LoadBalancerHints()
			resp.AutoConfigHint = task.AutoConfigHint
			return resp, nil
		}
		e.emitForwardedFromStats(hCtx.scope, task.IsForwarded(), req.GetForwardedFrom())
		e.emitTaskIsolationMetrics(hCtx.scope, task.Event.PartitionConfig, req.GetIsolationGroup())
		if task.ActivityTaskDispatchInfo != nil {
			task.Finish(nil)
			return e.createSyncMatchPollForActivityTaskResponse(task, task.ActivityTaskDispatchInfo, tlMgr.TaskListPartitionConfig(), tlMgr.LoadBalancerHints()), nil
		}

		resp, err := e.recordActivityTaskStarted(hCtx.Context, request, task)
		if err != nil {
			switch err.(type) {
			case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError, *types.EventAlreadyStartedError:
				domainName, _ := e.domainCache.GetDomainName(domainID)

				hCtx.scope.
					Tagged(metrics.DomainTag(domainName)).
					Tagged(metrics.TaskListTag(taskListName)).
					IncCounter(metrics.PollActivityTaskAlreadyStartedCounterPerTaskList)

				e.emitInfoOrDebugLog(
					task.Event.DomainID,
					"Duplicated activity task",
					tag.WorkflowDomainID(domainID),
					tag.WorkflowID(task.Event.WorkflowID),
					tag.WorkflowRunID(task.Event.RunID),
					tag.WorkflowTaskListName(taskListName),
					tag.WorkflowScheduleID(task.Event.ScheduleID),
					tag.TaskID(task.Event.TaskID),
				)
				task.Finish(nil)
			default:
				task.Finish(err)
			}

			continue pollLoop
		}
		task.Finish(nil)
		return e.createPollForActivityTaskResponse(task, resp, hCtx.scope, tlMgr.TaskListPartitionConfig(), tlMgr.LoadBalancerHints()), nil
	}
}

func (e *matchingEngineImpl) createSyncMatchPollForActivityTaskResponse(
	task *tasklist.InternalTask,
	activityTaskDispatchInfo *types.ActivityTaskDispatchInfo,
	partitionConfig *types.TaskListPartitionConfig,
	loadBalancerHints *types.LoadBalancerHints,
) *types.MatchingPollForActivityTaskResponse {

	scheduledEvent := activityTaskDispatchInfo.ScheduledEvent
	attributes := scheduledEvent.ActivityTaskScheduledEventAttributes
	response := &types.MatchingPollForActivityTaskResponse{}
	response.ActivityID = attributes.ActivityID
	response.ActivityType = attributes.ActivityType
	response.Header = attributes.Header
	response.Input = attributes.Input
	response.WorkflowExecution = task.WorkflowExecution()
	response.ScheduledTimestampOfThisAttempt = activityTaskDispatchInfo.ScheduledTimestampOfThisAttempt
	response.ScheduledTimestamp = scheduledEvent.Timestamp
	response.ScheduleToCloseTimeoutSeconds = attributes.ScheduleToCloseTimeoutSeconds
	response.StartedTimestamp = activityTaskDispatchInfo.StartedTimestamp
	response.StartToCloseTimeoutSeconds = attributes.StartToCloseTimeoutSeconds
	response.HeartbeatTimeoutSeconds = attributes.HeartbeatTimeoutSeconds

	token := &common.TaskToken{
		DomainID:        task.Event.DomainID,
		WorkflowID:      task.Event.WorkflowID,
		WorkflowType:    activityTaskDispatchInfo.WorkflowType.GetName(),
		RunID:           task.Event.RunID,
		ScheduleID:      task.Event.ScheduleID,
		ScheduleAttempt: common.Int64Default(activityTaskDispatchInfo.Attempt),
		ActivityID:      attributes.GetActivityID(),
		ActivityType:    attributes.GetActivityType().GetName(),
	}

	response.TaskToken, _ = e.tokenSerializer.Serialize(token)
	response.Attempt = int32(token.ScheduleAttempt)
	response.HeartbeatDetails = activityTaskDispatchInfo.HeartbeatDetails
	response.WorkflowType = activityTaskDispatchInfo.WorkflowType
	response.WorkflowDomain = activityTaskDispatchInfo.WorkflowDomain
	response.PartitionConfig = partitionConfig
	response.LoadBalancerHints = loadBalancerHints
	response.AutoConfigHint = task.AutoConfigHint
	return response
}

// QueryWorkflow creates a DecisionTask with query data, send it through sync match channel, wait for that DecisionTask
// to be processed by worker, and then return the query result.
func (e *matchingEngineImpl) QueryWorkflow(
	hCtx *handlerContext,
	queryRequest *types.MatchingQueryWorkflowRequest,
) (*types.QueryWorkflowResponse, error) {
	domainID := queryRequest.GetDomainUUID()
	taskListName := queryRequest.GetTaskList().GetName()
	taskListKind := queryRequest.GetTaskList().Kind
	taskListID, err := tasklist.NewIdentifier(domainID, taskListName, persistence.TaskListTypeDecision)
	if err != nil {
		return nil, err
	}

	tlMgr, err := e.getTaskListManager(taskListID, taskListKind)
	if err != nil {
		return nil, err
	}

	if taskListKind != nil && *taskListKind == types.TaskListKindSticky {
		// check if the sticky worker is still available, if not, fail this request early
		if !tlMgr.HasPollerAfter(e.timeSource.Now().Add(-_stickyPollerUnavailableWindow)) {
			return nil, _stickyPollerUnavailableError
		}
	}

	taskID := uuid.New()
	resp, err := tlMgr.DispatchQueryTask(hCtx.Context, taskID, queryRequest)

	// if get response or error it means that query task was handled by forwarding to another matching host
	// this remote host's result can be returned directly
	if resp != nil || err != nil {
		return resp, err
	}

	// if get here it means that dispatch of query task has occurred locally
	// must wait on result channel to get query result
	queryResultCh := make(chan *queryResult, 1)
	e.lockableQueryTaskMap.put(taskID, queryResultCh)
	defer e.lockableQueryTaskMap.delete(taskID)
	return e.waitForQueryResultFn(hCtx, queryRequest.GetQueryRequest().GetQueryConsistencyLevel() == types.QueryConsistencyLevelStrong, queryResultCh)
}

func (e *matchingEngineImpl) waitForQueryResult(hCtx *handlerContext, isStrongConsistencyQuery bool, queryResultCh <-chan *queryResult) (*types.QueryWorkflowResponse, error) {
	select {
	case result := <-queryResultCh:
		if result.internalError != nil {
			return nil, result.internalError
		}
		workerResponse := result.workerResponse
		// if query was intended as consistent query check to see if worker supports consistent query
		if isStrongConsistencyQuery {
			if err := e.versionChecker.SupportsConsistentQuery(
				workerResponse.GetCompletedRequest().GetWorkerVersionInfo().GetImpl(),
				workerResponse.GetCompletedRequest().GetWorkerVersionInfo().GetFeatureVersion()); err != nil {
				return nil, err
			}
		}
		switch workerResponse.GetCompletedRequest().GetCompletedType() {
		case types.QueryTaskCompletedTypeCompleted:
			return &types.QueryWorkflowResponse{QueryResult: workerResponse.GetCompletedRequest().GetQueryResult()}, nil
		case types.QueryTaskCompletedTypeFailed:
			return nil, &types.QueryFailedError{Message: workerResponse.GetCompletedRequest().GetErrorMessage()}
		default:
			return nil, &types.InternalServiceError{Message: "unknown query completed type"}
		}
	case <-hCtx.Done():
		return nil, hCtx.Err()
	}
}

func (e *matchingEngineImpl) RespondQueryTaskCompleted(hCtx *handlerContext, request *types.MatchingRespondQueryTaskCompletedRequest) error {
	if err := e.deliverQueryResult(request.GetTaskID(), &queryResult{workerResponse: request}); err != nil {
		hCtx.scope.IncCounter(metrics.RespondQueryTaskFailedPerTaskListCounter)
		return err
	}
	return nil
}

func (e *matchingEngineImpl) deliverQueryResult(taskID string, queryResult *queryResult) error {
	queryResultCh, ok := e.lockableQueryTaskMap.get(taskID)
	if !ok {
		return &types.InternalServiceError{Message: "query task not found, or already expired"}
	}
	queryResultCh <- queryResult
	return nil
}

func (e *matchingEngineImpl) CancelOutstandingPoll(
	hCtx *handlerContext,
	request *types.CancelOutstandingPollRequest,
) error {
	domainID := request.GetDomainUUID()
	taskListType := int(request.GetTaskListType())
	taskListName := request.GetTaskList().GetName()
	taskListKind := request.GetTaskList().Kind
	pollerID := request.GetPollerID()

	taskListID, err := tasklist.NewIdentifier(domainID, taskListName, taskListType)
	if err != nil {
		return err
	}

	tlMgr, err := e.getTaskListManager(taskListID, taskListKind)
	if err != nil {
		return err
	}

	tlMgr.CancelPoller(pollerID)
	return nil
}

func (e *matchingEngineImpl) DescribeTaskList(
	hCtx *handlerContext,
	request *types.MatchingDescribeTaskListRequest,
) (*types.DescribeTaskListResponse, error) {
	domainID := request.GetDomainUUID()
	taskListType := persistence.TaskListTypeDecision
	if request.DescRequest.GetTaskListType() == types.TaskListTypeActivity {
		taskListType = persistence.TaskListTypeActivity
	}
	taskListName := request.GetDescRequest().GetTaskList().GetName()
	taskListKind := request.GetDescRequest().GetTaskList().Kind

	taskListID, err := tasklist.NewIdentifier(domainID, taskListName, taskListType)
	if err != nil {
		return nil, err
	}

	tlMgr, err := e.getTaskListManager(taskListID, taskListKind)
	if err != nil {
		return nil, err
	}

	return tlMgr.DescribeTaskList(request.DescRequest.GetIncludeTaskListStatus()), nil
}

func (e *matchingEngineImpl) ListTaskListPartitions(
	hCtx *handlerContext,
	request *types.MatchingListTaskListPartitionsRequest,
) (*types.ListTaskListPartitionsResponse, error) {
	activityTaskListInfo, err := e.listTaskListPartitions(request, persistence.TaskListTypeActivity)
	if err != nil {
		return nil, err
	}
	decisionTaskListInfo, err := e.listTaskListPartitions(request, persistence.TaskListTypeDecision)
	if err != nil {
		return nil, err
	}
	resp := &types.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: activityTaskListInfo,
		DecisionTaskListPartitions: decisionTaskListInfo,
	}

	return resp, nil
}

func (e *matchingEngineImpl) listTaskListPartitions(
	request *types.MatchingListTaskListPartitionsRequest,
	taskListType int,
) ([]*types.TaskListPartitionMetadata, error) {
	partitions, err := e.getAllPartitions(
		request,
		taskListType,
	)
	if err != nil {
		return nil, err
	}

	var partitionHostInfo []*types.TaskListPartitionMetadata
	for _, partition := range partitions {
		host, _ := e.getHostInfo(partition)
		partitionHostInfo = append(partitionHostInfo,
			&types.TaskListPartitionMetadata{
				Key:           partition,
				OwnerHostName: host,
			})
	}
	return partitionHostInfo, nil
}

func (e *matchingEngineImpl) GetTaskListsByDomain(
	hCtx *handlerContext,
	request *types.GetTaskListsByDomainRequest,
) (*types.GetTaskListsByDomainResponse, error) {
	domainID, err := e.domainCache.GetDomainID(request.GetDomain())
	if err != nil {
		return nil, err
	}

	e.taskListsLock.RLock()
	defer e.taskListsLock.RUnlock()
	return e.getTaskListByDomainLocked(domainID), nil
}

func (e *matchingEngineImpl) UpdateTaskListPartitionConfig(
	hCtx *handlerContext,
	request *types.MatchingUpdateTaskListPartitionConfigRequest,
) (*types.MatchingUpdateTaskListPartitionConfigResponse, error) {
	domainID := request.DomainUUID
	taskListName := request.TaskList.GetName()
	taskListKind := request.TaskList.GetKind()
	taskListType := persistence.TaskListTypeDecision
	if request.GetTaskListType() == types.TaskListTypeActivity {
		taskListType = persistence.TaskListTypeActivity
	}
	domainName, err := e.domainCache.GetDomainName(domainID)
	if err != nil {
		return nil, err
	}
	if e.config.EnableAdaptiveScaler(domainName, taskListName, taskListType) {
		return nil, &types.BadRequestError{Message: "Manual update is not allowed because adaptive scaler is enabled."}
	}
	if taskListKind != types.TaskListKindNormal {
		return nil, &types.BadRequestError{Message: "Only normal tasklist's partition config can be updated."}
	}
	if request.PartitionConfig == nil {
		return nil, &types.BadRequestError{Message: "Task list partition config is not set in the request."}
	}
	taskListID, err := tasklist.NewIdentifier(domainID, taskListName, taskListType)
	if err != nil {
		return nil, err
	}
	if !taskListID.IsRoot() {
		return nil, &types.BadRequestError{Message: "Only root partition's partition config can be updated."}
	}
	tlMgr, err := e.getTaskListManager(taskListID, &taskListKind)
	if err != nil {
		return nil, err
	}
	err = tlMgr.UpdateTaskListPartitionConfig(hCtx.Context, request.PartitionConfig)
	if err != nil {
		return nil, err
	}
	return &types.MatchingUpdateTaskListPartitionConfigResponse{}, nil
}

func (e *matchingEngineImpl) RefreshTaskListPartitionConfig(
	hCtx *handlerContext,
	request *types.MatchingRefreshTaskListPartitionConfigRequest,
) (*types.MatchingRefreshTaskListPartitionConfigResponse, error) {
	domainID := request.DomainUUID
	taskListName := request.TaskList.GetName()
	taskListKind := request.TaskList.GetKind()
	taskListType := persistence.TaskListTypeDecision
	if request.GetTaskListType() == types.TaskListTypeActivity {
		taskListType = persistence.TaskListTypeActivity
	}
	if taskListKind != types.TaskListKindNormal {
		return nil, &types.BadRequestError{Message: "Only normal tasklist's partition config can be updated."}
	}
	taskListID, err := tasklist.NewIdentifier(domainID, taskListName, taskListType)
	if err != nil {
		return nil, err
	}
	if taskListID.IsRoot() && request.PartitionConfig != nil {
		return nil, &types.BadRequestError{Message: "PartitionConfig must be nil for root partition."}
	}
	tlMgr, err := e.getTaskListManager(taskListID, &taskListKind)
	if err != nil {
		return nil, err
	}
	err = tlMgr.RefreshTaskListPartitionConfig(hCtx.Context, request.PartitionConfig)
	if err != nil {
		return nil, err
	}
	return &types.MatchingRefreshTaskListPartitionConfigResponse{}, nil
}

func (e *matchingEngineImpl) getHostInfo(partitionKey string) (string, error) {
	host, err := e.membershipResolver.Lookup(service.Matching, partitionKey)
	if err != nil {
		return "", err
	}
	return host.GetAddress(), nil
}

func (e *matchingEngineImpl) getAllPartitions(
	request *types.MatchingListTaskListPartitionsRequest,
	taskListType int,
) ([]string, error) {
	domainID, err := e.domainCache.GetDomainID(request.GetDomain())
	if err != nil {
		return nil, err
	}
	taskList := request.GetTaskList()
	taskListID, err := tasklist.NewIdentifier(domainID, taskList.GetName(), taskListType)
	if err != nil {
		return nil, err
	}

	rootPartition := taskListID.GetRoot()
	partitionKeys := []string{rootPartition}
	n := e.config.NumTasklistWritePartitions(request.GetDomain(), rootPartition, taskListType)
	for i := 1; i < n; i++ {
		partitionKeys = append(partitionKeys, fmt.Sprintf("%v%v/%v", constants.ReservedTaskListPrefix, rootPartition, i))
	}
	return partitionKeys, nil
}

func (e *matchingEngineImpl) unloadTaskList(tlMgr tasklist.Manager) {
	id := tlMgr.TaskListID()
	e.taskListsLock.Lock()
	currentTlMgr, ok := e.taskLists[*id]
	if !ok || tlMgr != currentTlMgr {
		e.taskListsLock.Unlock()
		return
	}
	delete(e.taskLists, *id)
	e.taskListsLock.Unlock()
	tlMgr.Stop()
}

// Populate the decision task response based on context and scheduled/started events.
func (e *matchingEngineImpl) createPollForDecisionTaskResponse(
	task *tasklist.InternalTask,
	historyResponse *types.RecordDecisionTaskStartedResponse,
	scope metrics.Scope,
	partitionConfig *types.TaskListPartitionConfig,
	loadBalancerHints *types.LoadBalancerHints,
) *types.MatchingPollForDecisionTaskResponse {

	var token []byte
	if task.IsQuery() {
		// for a query task
		queryRequest := task.Query.Request
		execution := task.WorkflowExecution()
		taskToken := &common.QueryTaskToken{
			DomainID:   queryRequest.DomainUUID,
			WorkflowID: execution.WorkflowID,
			RunID:      execution.RunID,
			TaskList:   queryRequest.TaskList.Name,
			TaskID:     task.Query.TaskID,
		}
		token, _ = e.tokenSerializer.SerializeQueryTaskToken(taskToken)
	} else {
		taskToken := &common.TaskToken{
			DomainID:        task.Event.DomainID,
			WorkflowID:      task.Event.WorkflowID,
			RunID:           task.Event.RunID,
			ScheduleID:      historyResponse.GetScheduledEventID(),
			ScheduleAttempt: historyResponse.GetAttempt(),
		}
		token, _ = e.tokenSerializer.Serialize(taskToken)
		if task.ResponseC == nil {
			scope.RecordTimer(metrics.AsyncMatchLatencyPerTaskList, time.Since(task.Event.CreatedTime))
		}
	}

	response := common.CreateMatchingPollForDecisionTaskResponse(historyResponse, task.WorkflowExecution(), token)
	if task.Query != nil {
		response.Query = task.Query.Request.QueryRequest.Query
	}
	response.BacklogCountHint = task.BacklogCountHint
	response.PartitionConfig = partitionConfig
	response.LoadBalancerHints = loadBalancerHints
	response.AutoConfigHint = task.AutoConfigHint
	return response
}

// Populate the activity task response based on context and scheduled/started events.
func (e *matchingEngineImpl) createPollForActivityTaskResponse(
	task *tasklist.InternalTask,
	historyResponse *types.RecordActivityTaskStartedResponse,
	scope metrics.Scope,
	partitionConfig *types.TaskListPartitionConfig,
	loadBalancerHints *types.LoadBalancerHints,
) *types.MatchingPollForActivityTaskResponse {

	scheduledEvent := historyResponse.ScheduledEvent
	if scheduledEvent.ActivityTaskScheduledEventAttributes == nil {
		panic("GetActivityTaskScheduledEventAttributes is not set")
	}
	attributes := scheduledEvent.ActivityTaskScheduledEventAttributes
	if attributes.ActivityID == "" {
		panic("ActivityTaskScheduledEventAttributes.ActivityID is not set")
	}
	if task.ResponseC == nil {
		scope.RecordTimer(metrics.AsyncMatchLatencyPerTaskList, time.Since(task.Event.CreatedTime))
	}

	response := &types.MatchingPollForActivityTaskResponse{}
	response.ActivityID = attributes.ActivityID
	response.ActivityType = attributes.ActivityType
	response.Header = attributes.Header
	response.Input = attributes.Input
	response.WorkflowExecution = task.WorkflowExecution()
	response.ScheduledTimestampOfThisAttempt = historyResponse.ScheduledTimestampOfThisAttempt
	response.ScheduledTimestamp = scheduledEvent.Timestamp
	response.ScheduleToCloseTimeoutSeconds = attributes.ScheduleToCloseTimeoutSeconds
	response.StartedTimestamp = historyResponse.StartedTimestamp
	response.StartToCloseTimeoutSeconds = attributes.StartToCloseTimeoutSeconds
	response.HeartbeatTimeoutSeconds = attributes.HeartbeatTimeoutSeconds

	token := &common.TaskToken{
		DomainID:        task.Event.DomainID,
		WorkflowID:      task.Event.WorkflowID,
		WorkflowType:    historyResponse.WorkflowType.GetName(),
		RunID:           task.Event.RunID,
		ScheduleID:      task.Event.ScheduleID,
		ScheduleAttempt: historyResponse.GetAttempt(),
		ActivityID:      attributes.GetActivityID(),
		ActivityType:    attributes.GetActivityType().GetName(),
	}

	response.TaskToken, _ = e.tokenSerializer.Serialize(token)
	response.Attempt = int32(token.ScheduleAttempt)
	response.HeartbeatDetails = historyResponse.HeartbeatDetails
	response.WorkflowType = historyResponse.WorkflowType
	response.WorkflowDomain = historyResponse.WorkflowDomain
	response.PartitionConfig = partitionConfig
	response.LoadBalancerHints = loadBalancerHints
	response.AutoConfigHint = task.AutoConfigHint
	return response
}

func (e *matchingEngineImpl) recordDecisionTaskStarted(
	ctx context.Context,
	pollReq *types.PollForDecisionTaskRequest,
	task *tasklist.InternalTask,
) (*types.RecordDecisionTaskStartedResponse, error) {
	request := &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        task.Event.DomainID,
		WorkflowExecution: task.WorkflowExecution(),
		ScheduleID:        task.Event.ScheduleID,
		TaskID:            task.Event.TaskID,
		RequestID:         uuid.New(),
		PollRequest:       pollReq,
	}
	var resp *types.RecordDecisionTaskStartedResponse
	op := func() error {
		var err error
		resp, err = e.historyService.RecordDecisionTaskStarted(ctx, request)
		return err
	}
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(historyServiceOperationRetryPolicy),
		backoff.WithRetryableError(isMatchingRetryableError),
	)
	err := throttleRetry.Do(ctx, op)
	return resp, err
}

func (e *matchingEngineImpl) recordActivityTaskStarted(
	ctx context.Context,
	pollReq *types.PollForActivityTaskRequest,
	task *tasklist.InternalTask,
) (*types.RecordActivityTaskStartedResponse, error) {
	request := &types.RecordActivityTaskStartedRequest{
		DomainUUID:        task.Event.DomainID,
		WorkflowExecution: task.WorkflowExecution(),
		ScheduleID:        task.Event.ScheduleID,
		TaskID:            task.Event.TaskID,
		RequestID:         uuid.New(),
		PollRequest:       pollReq,
	}
	var resp *types.RecordActivityTaskStartedResponse
	op := func() error {
		var err error
		resp, err = e.historyService.RecordActivityTaskStarted(ctx, request)
		return err
	}
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(historyServiceOperationRetryPolicy),
		backoff.WithRetryableError(isMatchingRetryableError),
	)
	err := throttleRetry.Do(ctx, op)
	return resp, err
}

func (e *matchingEngineImpl) emitForwardedFromStats(
	scope metrics.Scope,
	isTaskForwarded bool,
	pollForwardedFrom string,
) {
	isPollForwarded := len(pollForwardedFrom) > 0
	switch {
	case isTaskForwarded && isPollForwarded:
		scope.IncCounter(metrics.RemoteToRemoteMatchPerTaskListCounter)
	case isTaskForwarded:
		scope.IncCounter(metrics.RemoteToLocalMatchPerTaskListCounter)
	case isPollForwarded:
		scope.IncCounter(metrics.LocalToRemoteMatchPerTaskListCounter)
	default:
		scope.IncCounter(metrics.LocalToLocalMatchPerTaskListCounter)
	}
}

func (e *matchingEngineImpl) emitTaskIsolationMetrics(
	scope metrics.Scope,
	partitionConfig map[string]string,
	pollerIsolationGroup string,
) {
	if currentGroup, ok := partitionConfig[isolationgroup.GroupKey]; ok {
		originalGroup, ok := partitionConfig[isolationgroup.OriginalGroupKey]
		if !ok {
			originalGroup = currentGroup
		}
		scope.Tagged(metrics.OriginalIsolationGroupTag(originalGroup), metrics.IsolationGroupTag(currentGroup), metrics.PollerIsolationGroupTag(pollerIsolationGroup)).IncCounter(metrics.IsolationTaskMatchPerTaskListCounter)
	}
}

func (e *matchingEngineImpl) emitInfoOrDebugLog(
	domainID string,
	msg string,
	tags ...tag.Tag,
) {
	if e.config.EnableDebugMode && e.config.EnableTaskInfoLogByDomainID(domainID) {
		e.logger.Info(msg, tags...)
	} else {
		e.logger.Debug(msg, tags...)
	}
}

func (e *matchingEngineImpl) errIfShardLoss(taskList *tasklist.Identifier) error {
	if !e.config.EnableTasklistOwnershipGuard() {
		return nil
	}

	self, err := e.membershipResolver.WhoAmI()
	if err != nil {
		return fmt.Errorf("failed to lookup self im membership: %w", err)
	}

	if e.isShuttingDown() {
		e.logger.Warn("request to get tasklist is being rejected because engine is shutting down",
			tag.WorkflowDomainID(taskList.GetDomainID()),
			tag.WorkflowTaskListType(taskList.GetType()),
			tag.WorkflowTaskListName(taskList.GetName()),
		)

		return cadence_errors.NewTaskListNotOwnedByHostError(
			"not known",
			self.Identity(),
			taskList.GetName(),
		)
	}

	// Defensive check to make sure we actually own the task list
	//   If we try to create a task list manager for a task list that is not owned by us, return an error
	//   The new task list manager will steal the task list from the current owner, which should only happen if
	//   the task list is owned by the current host.
	taskListOwner, err := e.membershipResolver.Lookup(service.Matching, taskList.GetName())
	if err != nil {
		return fmt.Errorf("failed to lookup task list owner: %w", err)
	}

	if taskListOwner.Identity() != self.Identity() {
		e.logger.Warn("Request to get tasklist is being rejected because engine does not own this shard",
			tag.WorkflowDomainID(taskList.GetDomainID()),
			tag.WorkflowTaskListType(taskList.GetType()),
			tag.WorkflowTaskListName(taskList.GetName()),
		)
		return cadence_errors.NewTaskListNotOwnedByHostError(
			taskListOwner.Identity(),
			self.Identity(),
			taskList.GetName(),
		)
	}

	return nil
}

func (e *matchingEngineImpl) isShuttingDown() bool {
	select {
	case <-e.shutdown:
		return true
	default:
		return false
	}
}

func (m *lockableQueryTaskMap) put(key string, value chan *queryResult) {
	m.Lock()
	defer m.Unlock()
	m.queryTaskMap[key] = value
}

func (m *lockableQueryTaskMap) get(key string) (chan *queryResult, bool) {
	m.RLock()
	defer m.RUnlock()
	result, ok := m.queryTaskMap[key]
	return result, ok
}

func (m *lockableQueryTaskMap) delete(key string) {
	m.Lock()
	defer m.Unlock()
	delete(m.queryTaskMap, key)
}

func isMatchingRetryableError(err error) bool {
	switch err.(type) {
	case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError, *types.EventAlreadyStartedError:
		return false
	}
	return true
}
