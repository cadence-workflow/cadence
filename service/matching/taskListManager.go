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
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	// Time budget for empty task to propagate through the function stack and be returned to
	// pollForActivityTask or pollForDecisionTask handler.
	returnEmptyTaskTimeBudget time.Duration = time.Second
)

var taskListActivityTypeTag = metrics.TaskListTypeTag("activity")
var taskListDecisionTypeTag = metrics.TaskListTypeTag("decision")

type (
	addTaskParams struct {
		execution     *types.WorkflowExecution
		taskInfo      *persistence.TaskInfo
		source        types.TaskSource
		forwardedFrom string
	}

	taskListManager interface {
		Start() error
		Stop()
		// AddTask adds a task to the task list. This method will first attempt a synchronous
		// match with a poller. When that fails, task will be written to database and later
		// asynchronously matched with a poller
		AddTask(ctx context.Context, params addTaskParams) (syncMatch bool, err error)
		// GetTask blocks waiting for a task Returns error when context deadline is exceeded
		// maxDispatchPerSecond is the max rate at which tasks are allowed to be dispatched
		// from this task list to pollers
		GetTask(ctx context.Context, maxDispatchPerSecond *float64) (*internalTask, error)
		// DispatchTask dispatches a task to a poller. When there are no pollers to pick
		// up the task, this method will return error. Task will not be persisted to db
		DispatchTask(ctx context.Context, task *internalTask) error
		// DispatchQueryTask will dispatch query to local or remote poller. If forwarded then result or error is returned,
		// if dispatched to local poller then nil and nil is returned.
		DispatchQueryTask(ctx context.Context, taskID string, request *types.MatchingQueryWorkflowRequest) (*types.QueryWorkflowResponse, error)
		CancelPoller(pollerID string)
		GetAllPollerInfo() []*types.PollerInfo
		// DescribeTaskList returns information about the target tasklist
		DescribeTaskList(includeTaskListStatus bool) *types.DescribeTaskListResponse
		String() string
	}

	// Single task list in memory state
	taskListManagerImpl struct {
		taskListID       *taskListID
		taskListKind     types.TaskListKind // sticky taskList has different process in persistence
		config           *taskListConfig
		db               *taskListDB
		engine           *matchingEngineImpl
		taskWriter       *taskWriter
		taskReader       *taskReader // reads tasks from db and async matches it with poller
		taskGC           *taskGC
		taskAckManager   messaging.AckManager // tracks ackLevel for delivered messages
		matcher          *TaskMatcher         // for matching a task producer with a poller
		domainCache      cache.DomainCache
		logger           log.Logger
		metricsClient    metrics.Client
		domainNameValue  atomic.Value
		metricScopeValue atomic.Value // domain/tasklist tagged metric scope
		// pollerHistory stores poller which poll from this tasklist in last few minutes
		pollerHistory *pollerHistory
		// outstandingPollsMap is needed to keep track of all outstanding pollers for a
		// particular tasklist.  PollerID generated by frontend is used as the key and
		// CancelFunc is the value.  This is used to cancel the context to unblock any
		// outstanding poller when the frontend detects client connection is closed to
		// prevent tasks being dispatched to zombie pollers.
		outstandingPollsLock sync.Mutex
		outstandingPollsMap  map[string]context.CancelFunc

		shutdownCh chan struct{}  // Delivers stop to the pump that populates taskBuffer
		startWG    sync.WaitGroup // ensures that background processes do not start until setup is ready
		stopped    int32
	}
)

const (
	// maxSyncMatchWaitTime is the max amount of time that we are willing to wait for a sync match to happen
	maxSyncMatchWaitTime = 200 * time.Millisecond
)

var _ taskListManager = (*taskListManagerImpl)(nil)

var errRemoteSyncMatchFailed = &types.RemoteSyncMatchedError{Message: "remote sync match failed"}

func newTaskListManager(
	e *matchingEngineImpl,
	taskList *taskListID,
	taskListKind *types.TaskListKind,
	config *Config,
) (taskListManager, error) {

	taskListConfig, err := newTaskListConfig(taskList, config, e.domainCache)
	if err != nil {
		return nil, err
	}

	if taskListKind == nil {
		normalTaskListKind := types.TaskListKindNormal
		taskListKind = &normalTaskListKind
	}

	db := newTaskListDB(e.taskManager, taskList.domainID, taskList.name, taskList.taskType, int(*taskListKind), e.logger)

	tlMgr := &taskListManagerImpl{
		domainCache:   e.domainCache,
		metricsClient: e.metricsClient,
		engine:        e,
		shutdownCh:    make(chan struct{}),
		taskListID:    taskList,
		taskListKind:  *taskListKind,
		logger: e.logger.WithTags(tag.WorkflowTaskListName(taskList.name),
			tag.WorkflowTaskListType(taskList.taskType)),
		db:                  db,
		taskAckManager:      messaging.NewAckManager(e.logger),
		taskGC:              newTaskGC(db, taskListConfig),
		config:              taskListConfig,
		outstandingPollsMap: make(map[string]context.CancelFunc),
	}

	tlMgr.domainNameValue.Store("")
	if tlMgr.metricScope() == nil { // domain name lookup failed
		// metric scope to use when domainName lookup fails
		tlMgr.metricScopeValue.Store(newPerTaskListScope(
			"",
			tlMgr.taskListID.name,
			tlMgr.taskListKind,
			e.metricsClient,
			metrics.MatchingTaskListMgrScope,
		))
	}
	var taskListTypeMetricScope metrics.Scope
	if taskList.taskType == persistence.TaskListTypeActivity {
		taskListTypeMetricScope = tlMgr.metricScope().Tagged(taskListActivityTypeTag)
	} else {
		taskListTypeMetricScope = tlMgr.metricScope().Tagged(taskListDecisionTypeTag)
	}
	tlMgr.pollerHistory = newPollerHistory(func() {
		taskListTypeMetricScope.UpdateGauge(metrics.PollerPerTaskListCounter,
			float64(len(tlMgr.pollerHistory.getAllPollerInfo())))
	})
	tlMgr.taskWriter = newTaskWriter(tlMgr)
	tlMgr.taskReader = newTaskReader(tlMgr)
	var fwdr *Forwarder
	if tlMgr.isFowardingAllowed(taskList, *taskListKind) {
		fwdr = newForwarder(&taskListConfig.forwarderConfig, taskList, *taskListKind, e.matchingClient)
	}
	tlMgr.matcher = newTaskMatcher(taskListConfig, fwdr, tlMgr.metricScope)
	tlMgr.startWG.Add(1)
	return tlMgr, nil
}

// Starts reading pump for the given task list.
// The pump fills up taskBuffer from persistence.
func (c *taskListManagerImpl) Start() error {
	defer c.startWG.Done()

	// Make sure to grab the range first before starting task writer, as it needs the range to initialize maxReadLevel
	state, err := c.renewLeaseWithRetry()
	if err != nil {
		c.Stop()
		return err
	}

	c.taskAckManager.SetAckLevel(state.ackLevel)
	c.taskWriter.Start(c.rangeIDToTaskIDBlock(state.rangeID))
	c.taskReader.Start()

	return nil
}

// Stops pump that fills up taskBuffer from persistence.
func (c *taskListManagerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&c.stopped, 0, 1) {
		return
	}
	close(c.shutdownCh)
	c.taskWriter.Stop()
	c.taskReader.Stop()
	c.engine.removeTaskListManager(c.taskListID)
	c.logger.Info("Task list manager state changed", tag.LifeCycleStopped)
}

// AddTask adds a task to the task list. This method will first attempt a synchronous
// match with a poller. When there are no pollers or if rate limit is exceeded, task will
// be written to database and later asynchronously matched with a poller
func (c *taskListManagerImpl) AddTask(ctx context.Context, params addTaskParams) (bool, error) {
	c.startWG.Wait()
	var syncMatch bool
	_, err := c.executeWithRetry(func() (interface{}, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		domainEntry, err := c.domainCache.GetDomainByID(params.taskInfo.DomainID)
		if err != nil {
			return nil, err
		}

		if domainEntry.GetDomainNotActiveErr() != nil {
			r, err := c.taskWriter.appendTask(params.execution, params.taskInfo)
			syncMatch = false
			return r, err
		}

		syncMatch, err = c.trySyncMatch(ctx, params)
		if syncMatch {
			return &persistence.CreateTasksResponse{}, err
		}

		if params.forwardedFrom != "" {
			// forwarded from child partition - only do sync match
			// child partition will persist the task when sync match fails
			return &persistence.CreateTasksResponse{}, errRemoteSyncMatchFailed
		}

		return c.taskWriter.appendTask(params.execution, params.taskInfo)
	})

	if err != nil {
		c.logger.Error("Failed to add task",
			tag.Error(err),
			tag.WorkflowTaskListName(c.taskListID.name),
			tag.WorkflowTaskListType(c.taskListID.taskType),
		)
	} else {
		c.taskReader.Signal()
	}

	return syncMatch, err
}

// DispatchTask dispatches a task to a poller. When there are no pollers to pick
// up the task or if rate limit is exceeded, this method will return error. Task
// *will not* be persisted to db
func (c *taskListManagerImpl) DispatchTask(ctx context.Context, task *internalTask) error {
	return c.matcher.MustOffer(ctx, task)
}

// DispatchQueryTask will dispatch query to local or remote poller. If forwarded then result or error is returned,
// if dispatched to local poller then nil and nil is returned.
func (c *taskListManagerImpl) DispatchQueryTask(
	ctx context.Context,
	taskID string,
	request *types.MatchingQueryWorkflowRequest,
) (*types.QueryWorkflowResponse, error) {
	c.startWG.Wait()
	task := newInternalQueryTask(taskID, request)
	return c.matcher.OfferQuery(ctx, task)
}

// GetTask blocks waiting for a task.
// Returns error when context deadline is exceeded
// maxDispatchPerSecond is the max rate at which tasks are allowed
// to be dispatched from this task list to pollers
func (c *taskListManagerImpl) GetTask(
	ctx context.Context,
	maxDispatchPerSecond *float64,
) (*internalTask, error) {
	task, err := c.getTask(ctx, maxDispatchPerSecond)
	if err != nil {
		return nil, err
	}
	task.domainName = c.domainName()
	task.backlogCountHint = c.taskAckManager.GetBacklogCount()
	return task, nil
}

func (c *taskListManagerImpl) getTask(ctx context.Context, maxDispatchPerSecond *float64) (*internalTask, error) {
	// We need to set a shorter timeout than the original ctx; otherwise, by the time ctx deadline is
	// reached, instead of emptyTask, context timeout error is returned to the frontend by the rpc stack,
	// which counts against our SLO. By shortening the timeout by a very small amount, the emptyTask can be
	// returned to the handler before a context timeout error is generated.
	childCtx, cancel := c.newChildContext(ctx, c.config.LongPollExpirationInterval(), returnEmptyTaskTimeBudget)
	defer cancel()

	pollerID, ok := ctx.Value(pollerIDKey).(string)
	if ok && pollerID != "" {
		// Found pollerID on context, add it to the map to allow it to be canceled in
		// response to CancelPoller call
		c.outstandingPollsLock.Lock()
		c.outstandingPollsMap[pollerID] = cancel
		c.outstandingPollsLock.Unlock()
		defer func() {
			c.outstandingPollsLock.Lock()
			delete(c.outstandingPollsMap, pollerID)
			c.outstandingPollsLock.Unlock()
		}()
	}

	identity, ok := ctx.Value(identityKey).(string)
	if ok && identity != "" {
		c.pollerHistory.updatePollerInfo(pollerIdentity(identity), maxDispatchPerSecond)
	}

	domainEntry, err := c.domainCache.GetDomainByID(c.taskListID.domainID)
	if err != nil {
		return nil, err
	}

	// the desired global rate limit for the task list comes from the
	// poller, which lives inside the client side worker. There is
	// one rateLimiter for this entire task list and as we get polls,
	// we update the ratelimiter rps if it has changed from the last
	// value. Last poller wins if different pollers provide different values
	c.matcher.UpdateRatelimit(maxDispatchPerSecond)

	if domainEntry.GetDomainNotActiveErr() != nil {
		return c.matcher.PollForQuery(childCtx)
	}

	return c.matcher.Poll(childCtx)
}

// GetAllPollerInfo returns all pollers that polled from this tasklist in last few minutes
func (c *taskListManagerImpl) GetAllPollerInfo() []*types.PollerInfo {
	return c.pollerHistory.getAllPollerInfo()
}

func (c *taskListManagerImpl) CancelPoller(pollerID string) {
	c.outstandingPollsLock.Lock()
	cancel, ok := c.outstandingPollsMap[pollerID]
	c.outstandingPollsLock.Unlock()

	if ok && cancel != nil {
		cancel()
		c.logger.Info("canceled outstanding poller", tag.WorkflowDomainName(c.domainName()))
	}
}

// DescribeTaskList returns information about the target tasklist, right now this API returns the
// pollers which polled this tasklist in last few minutes and status of tasklist's ackManager
// (readLevel, ackLevel, backlogCountHint and taskIDBlock).
func (c *taskListManagerImpl) DescribeTaskList(includeTaskListStatus bool) *types.DescribeTaskListResponse {
	response := &types.DescribeTaskListResponse{Pollers: c.GetAllPollerInfo()}
	if !includeTaskListStatus {
		return response
	}

	taskIDBlock := c.rangeIDToTaskIDBlock(c.db.RangeID())
	response.TaskListStatus = &types.TaskListStatus{
		ReadLevel:        c.taskAckManager.GetReadLevel(),
		AckLevel:         c.taskAckManager.GetAckLevel(),
		BacklogCountHint: c.taskAckManager.GetBacklogCount(),
		RatePerSecond:    c.matcher.Rate(),
		TaskIDBlock: &types.TaskIDBlock{
			StartID: taskIDBlock.start,
			EndID:   taskIDBlock.end,
		},
	}

	return response
}

func (c *taskListManagerImpl) String() string {
	buf := new(bytes.Buffer)
	if c.taskListID.taskType == persistence.TaskListTypeActivity {
		buf.WriteString("Activity")
	} else {
		buf.WriteString("Decision")
	}
	rangeID := c.db.RangeID()
	fmt.Fprintf(buf, " task list %v\n", c.taskListID.name)
	fmt.Fprintf(buf, "RangeID=%v\n", rangeID)
	fmt.Fprintf(buf, "TaskIDBlock=%+v\n", c.rangeIDToTaskIDBlock(rangeID))
	fmt.Fprintf(buf, "AckLevel=%v\n", c.taskAckManager.GetAckLevel())
	fmt.Fprintf(buf, "MaxReadLevel=%v\n", c.taskAckManager.GetReadLevel())

	return buf.String()
}

// completeTask marks a task as processed. Only tasks created by taskReader (i.e. backlog from db) reach
// here. As part of completion:
//   - task is deleted from the database when err is nil
//   - new task is created and current task is deleted when err is not nil
func (c *taskListManagerImpl) completeTask(task *persistence.TaskInfo, err error) {
	if err != nil {
		// failed to start the task.
		// We cannot just remove it from persistence because then it will be lost.
		// We handle this by writing the task back to persistence with a higher taskID.
		// This will allow subsequent tasks to make progress, and hopefully by the time this task is picked-up
		// again the underlying reason for failing to start will be resolved.
		// Note that RecordTaskStarted only fails after retrying for a long time, so a single task will not be
		// re-written to persistence frequently.
		_, err = c.executeWithRetry(func() (interface{}, error) {
			wf := &types.WorkflowExecution{WorkflowID: task.WorkflowID, RunID: task.RunID}
			return c.taskWriter.appendTask(wf, task)
		})

		if err != nil {
			// OK, we also failed to write to persistence.
			// This should only happen in very extreme cases where persistence is completely down.
			// We still can't lose the old task so we just unload the entire task list
			c.logger.Error("Failed to complete task",
				tag.Error(err),
				tag.WorkflowTaskListName(c.taskListID.name),
				tag.WorkflowTaskListType(c.taskListID.taskType))
			c.Stop()
			return
		}
		c.taskReader.Signal()
	}
	ackLevel := c.taskAckManager.AckItem(task.TaskID)
	c.taskGC.Run(ackLevel)
}

func (c *taskListManagerImpl) renewLeaseWithRetry() (taskListState, error) {
	var newState taskListState
	op := func() (err error) {
		newState, err = c.db.RenewLease()
		return
	}
	c.metricScope().IncCounter(metrics.LeaseRequestPerTaskListCounter)
	err := backoff.Retry(op, persistenceOperationRetryPolicy, common.IsPersistenceTransientError)
	if err != nil {
		c.metricScope().IncCounter(metrics.LeaseFailurePerTaskListCounter)
		c.engine.unloadTaskList(c.taskListID)
		return newState, err
	}
	return newState, nil
}

func (c *taskListManagerImpl) rangeIDToTaskIDBlock(rangeID int64) taskIDBlock {
	return taskIDBlock{
		start: (rangeID-1)*c.config.RangeSize + 1,
		end:   rangeID * c.config.RangeSize,
	}
}

func (c *taskListManagerImpl) allocTaskIDBlock(prevBlockEnd int64) (taskIDBlock, error) {
	currBlock := c.rangeIDToTaskIDBlock(c.db.RangeID())
	if currBlock.end != prevBlockEnd {
		return taskIDBlock{},
			fmt.Errorf("allocTaskIDBlock: invalid state: prevBlockEnd:%v != currTaskIDBlock:%+v", prevBlockEnd, currBlock)
	}
	state, err := c.renewLeaseWithRetry()
	if err != nil {
		return taskIDBlock{}, err
	}
	return c.rangeIDToTaskIDBlock(state.rangeID), nil
}

// Retry operation on transient error. On rangeID update by another process calls c.Stop().
func (c *taskListManagerImpl) executeWithRetry(
	operation func() (interface{}, error),
) (result interface{}, err error) {

	op := func() error {
		result, err = operation()
		return err
	}

	err = backoff.Retry(op, persistenceOperationRetryPolicy, func(err error) bool {
		if _, ok := err.(*persistence.ConditionFailedError); ok {
			return false
		}
		return common.IsPersistenceTransientError(err)
	})

	if _, ok := err.(*persistence.ConditionFailedError); ok {
		c.metricScope().IncCounter(metrics.ConditionFailedErrorPerTaskListCounter)
		c.logger.Debug("Stopping task list due to persistence condition failure.", tag.Error(err))
		c.Stop()
		if c.taskListKind == types.TaskListKindSticky {
			err = &types.InternalServiceError{Message: common.StickyTaskConditionFailedErrorMsg}
		}
	}
	return
}

func (c *taskListManagerImpl) trySyncMatch(ctx context.Context, params addTaskParams) (bool, error) {
	task := newInternalTask(params.taskInfo, c.completeTask, params.source, params.forwardedFrom, true)
	childCtx := ctx
	cancel := func() {}
	if !task.isForwarded() {
		// when task is forwarded from another matching host, we trust the context as is
		// otherwise, we override to limit the amount of time we can block on sync match
		childCtx, cancel = c.newChildContext(ctx, maxSyncMatchWaitTime, time.Second)
	}
	matched, err := c.matcher.Offer(childCtx, task)
	cancel()
	return matched, err
}

// newChildContext creates a child context with desired timeout.
// if tailroom is non-zero, then child context timeout will be
// the minOf(parentCtx.Deadline()-tailroom, timeout). Use this
// method to create child context when childContext cannot use
// all of parent's deadline but instead there is a need to leave
// some time for parent to do some post-work
func (c *taskListManagerImpl) newChildContext(
	parent context.Context,
	timeout time.Duration,
	tailroom time.Duration,
) (context.Context, context.CancelFunc) {
	select {
	case <-parent.Done():
		return parent, func() {}
	default:
	}
	deadline, ok := parent.Deadline()
	if !ok {
		return context.WithTimeout(parent, timeout)
	}
	remaining := deadline.Sub(time.Now()) - tailroom
	if remaining < timeout {
		timeout = time.Duration(common.MaxInt64(0, int64(remaining)))
	}
	return context.WithTimeout(parent, timeout)
}

func (c *taskListManagerImpl) isFowardingAllowed(taskList *taskListID, kind types.TaskListKind) bool {
	return !taskList.IsRoot() && kind != types.TaskListKindSticky
}

func (c *taskListManagerImpl) metricScope() metrics.Scope {
	c.tryInitDomainNameAndScope()
	return c.metricScopeValue.Load().(metrics.Scope)
}

func (c *taskListManagerImpl) domainName() string {
	name := c.domainNameValue.Load().(string)
	if len(name) > 0 {
		return name
	}
	c.tryInitDomainNameAndScope()
	return c.domainNameValue.Load().(string)
}

// reload from domainCache in case it got empty result during construction
func (c *taskListManagerImpl) tryInitDomainNameAndScope() {
	domainName := c.domainNameValue.Load().(string)
	if domainName != "" {
		return
	}

	domainName, err := c.domainCache.GetDomainName(c.taskListID.domainID)
	if err != nil {
		return
	}

	scope := newPerTaskListScope(
		domainName,
		c.taskListID.name,
		c.taskListKind,
		c.metricsClient,
		metrics.MatchingTaskListMgrScope,
	)

	c.metricScopeValue.Store(scope)
	c.domainNameValue.Store(domainName)
}

func createServiceBusyError(msg string) *types.ServiceBusyError {
	return &types.ServiceBusyError{Message: msg}
}
