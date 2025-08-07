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

package tasklist

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/stats"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/event"
	"github.com/uber/cadence/service/matching/liveness"
	"github.com/uber/cadence/service/matching/poller"
)

const (
	// Time budget for empty task to propagate through the function stack and be returned to
	// pollForActivityTask or pollForDecisionTask handler.
	returnEmptyTaskTimeBudget = time.Second
	noIsolationTimeout        = time.Duration(0)
	minimumIsolationDuration  = time.Millisecond * 50
)

var (
	// ErrNoTasks is exported temporarily for integration test
	ErrNoTasks                        = errors.New("no tasks")
	persistenceOperationRetryPolicy   = common.CreatePersistenceRetryPolicy()
	taskListActivityTypeTag           = metrics.TaskListTypeTag("activity")
	taskListDecisionTypeTag           = metrics.TaskListTypeTag("decision")
	IsolationLeakCauseError           = metrics.IsolationLeakCause("error")
	IsolationLeakCauseGroupDrained    = metrics.IsolationLeakCause("group_drained")
	IsolationLeakCauseNoRecentPollers = metrics.IsolationLeakCause("no_recent_pollers")
	IsolationLeakCausePartitionChange = metrics.IsolationLeakCause("partition_change")
	IsolationLeakCauseExpired         = metrics.IsolationLeakCause("expired")

	defaultPartition = &types.TaskListPartition{}
)

type (
	pollerIDCtxKey       struct{}
	identityCtxKey       struct{}
	isolationGroupCtxKey struct{}

	AddTaskParams struct {
		TaskInfo                 *persistence.TaskInfo
		Source                   types.TaskSource
		ForwardedFrom            string
		ActivityTaskDispatchInfo *types.ActivityTaskDispatchInfo
	}

	// Single task list in memory state
	taskListManagerImpl struct {
		createTime      time.Time
		enableIsolation bool
		taskListID      *Identifier
		taskListKind    types.TaskListKind // sticky taskList has different process in persistence
		config          *config.TaskListConfig
		db              *taskListDB
		taskWriter      *taskWriter
		taskReader      *taskReader // reads tasks from db and async matches it with poller
		liveness        *liveness.Liveness
		taskGC          *taskGC
		taskAckManager  messaging.AckManager // tracks ackLevel for delivered messages
		matcher         TaskMatcher          // for matching a task producer with a poller
		limiter         *taskListLimiter
		clusterMetadata cluster.Metadata
		domainCache     cache.DomainCache
		isolationState  isolationgroup.State
		logger          log.Logger
		scope           metrics.Scope
		timeSource      clock.TimeSource
		matchingClient  matching.Client
		domainName      string
		// pollers stores poller which poll from this tasklist in last few minutes
		pollers       poller.Manager
		startWG       sync.WaitGroup // ensures that background processes do not start until setup is ready
		stopWG        sync.WaitGroup
		stopped       int32
		stoppedLock   sync.RWMutex
		closeCallback func(Manager)
		throttleRetry *backoff.ThrottleRetry

		qpsTracker     stats.QPSTrackerGroup
		adaptiveScaler AdaptiveScaler

		partitionConfigLock sync.RWMutex
		partitionConfig     *types.TaskListPartitionConfig
		historyService      history.Client
		taskCompleter       TaskCompleter
	}
)

const (
	// maxSyncMatchWaitTime is the max amount of time that we are willing to wait for a sync match to happen
	maxSyncMatchWaitTime = 200 * time.Millisecond
)

var errRemoteSyncMatchFailed = &types.RemoteSyncMatchedError{Message: "remote sync match failed"}

func NewManager(
	domainCache cache.DomainCache,
	logger log.Logger,
	metricsClient metrics.Client,
	taskManager persistence.TaskManager,
	clusterMetadata cluster.Metadata,
	isolationState isolationgroup.State,
	matchingClient matching.Client,
	closeCallback func(Manager),
	taskList *Identifier,
	taskListKind types.TaskListKind,
	cfg *config.Config,
	timeSource clock.TimeSource,
	createTime time.Time,
	historyService history.Client,
) (Manager, error) {
	domainName, err := domainCache.GetDomainName(taskList.GetDomainID())
	if err != nil {
		return nil, err
	}

	taskListConfig := newTaskListConfig(taskList, cfg, domainName)

	scope := common.NewPerTaskListScope(domainName, taskList.GetName(), taskListKind, metricsClient, metrics.MatchingTaskListMgrScope).
		Tagged(getTaskListTypeTag(taskList.GetType()))
	db := newTaskListDB(taskManager, taskList.GetDomainID(), domainName, taskList.GetName(), taskList.GetType(), int(taskListKind), logger)

	tlMgr := &taskListManagerImpl{
		createTime:      createTime,
		enableIsolation: taskListConfig.EnableTasklistIsolation(),
		domainCache:     domainCache,
		clusterMetadata: clusterMetadata,
		isolationState:  isolationState,
		taskListID:      taskList,
		taskListKind:    taskListKind,
		logger:          logger.WithTags(tag.WorkflowDomainName(domainName), tag.WorkflowTaskListName(taskList.GetName()), tag.WorkflowTaskListType(taskList.GetType())),
		db:              db,
		taskAckManager:  messaging.NewAckManager(logger),
		taskGC:          newTaskGC(db, taskListConfig),
		config:          taskListConfig,
		matchingClient:  matchingClient,
		domainName:      domainName,
		scope:           scope,
		timeSource:      timeSource,
		closeCallback:   closeCallback,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(persistenceOperationRetryPolicy),
			backoff.WithRetryableError(persistence.IsTransientError),
		),
		historyService: historyService,
	}

	tlMgr.pollers = poller.NewPollerManager(func() {
		scope.UpdateGauge(metrics.PollerPerTaskListCounter,
			float64(tlMgr.pollers.GetCount()))
	}, timeSource)

	livenessInterval := taskListConfig.IdleTasklistCheckInterval()
	tlMgr.liveness = liveness.NewLiveness(timeSource, livenessInterval, func() {
		tlMgr.logger.Info("Task list manager stopping because no recent events", tag.Dynamic("interval", livenessInterval))
		tlMgr.Stop()
	})

	baseEvent := event.E{
		TaskListName: taskList.GetName(),
		TaskListKind: &taskListKind,
		TaskListType: taskList.GetType(),
	}

	tlMgr.qpsTracker = stats.NewEmaFixedWindowQPSTracker(timeSource, 0.5, taskListConfig.QPSTrackerInterval(), baseEvent)
	if taskList.IsRoot() && taskListKind == types.TaskListKindNormal {
		adaptiveScalerScope := common.NewPerTaskListScope(domainName, taskList.GetName(), taskListKind, metricsClient, metrics.MatchingAdaptiveScalerScope).
			Tagged(getTaskListTypeTag(taskList.GetType()))
		tlMgr.adaptiveScaler = NewAdaptiveScaler(taskList, tlMgr, taskListConfig, timeSource, tlMgr.logger, adaptiveScalerScope, matchingClient, baseEvent)
	}
	var isolationGroups []string
	if tlMgr.isIsolationMatcherEnabled() {
		isolationGroups = cfg.AllIsolationGroups()
	}
	var fwdr Forwarder
	if tlMgr.isFowardingAllowed(taskList, taskListKind) {
		fwdr = newForwarder(&taskListConfig.ForwarderConfig, taskList, taskListKind, matchingClient, scope)
	}
	numReadPartitionsFn := func() int {
		if taskListConfig.EnableGetNumberOfPartitionsFromCache() {
			partitionConfig := tlMgr.TaskListPartitionConfig()
			r := 1
			if partitionConfig != nil {
				r = len(partitionConfig.ReadPartitions)
			}
			return r
		}
		return taskListConfig.NumReadPartitions()
	}
	tlMgr.limiter = newTaskListLimiter(timeSource, tlMgr.scope, taskListConfig, numReadPartitionsFn)
	tlMgr.matcher = newTaskMatcher(taskListConfig, fwdr, tlMgr.scope, isolationGroups, tlMgr.logger, taskList, taskListKind, tlMgr.limiter).(*taskMatcherImpl)
	tlMgr.taskWriter = newTaskWriter(tlMgr)
	tlMgr.taskReader = newTaskReader(tlMgr, isolationGroups)
	tlMgr.taskCompleter = newTaskCompleter(tlMgr, historyServiceOperationRetryPolicy)
	tlMgr.startWG.Add(1)
	return tlMgr, nil
}

// Starts reading pump for the given task list.
// The pump fills up taskBuffer from persistence.
func (c *taskListManagerImpl) Start() error {
	defer c.startWG.Done()

	if !c.taskListID.IsRoot() && c.taskListKind == types.TaskListKindNormal {
		var info *persistence.TaskListInfo
		err := c.throttleRetry.Do(context.Background(), func(ctx context.Context) error {
			var err error
			info, err = c.db.GetTaskListInfo(c.taskListID.GetRoot())
			return err
		})
		if err != nil {
			// This is an edge case, and only occur before we fully migrate partition config to database for a tasklist.
			// Currently, if a task list is configured with multiple partitions in dynamicconfig before the creation of the task list,
			// a non-root partition can receive a request before the root partition and when it task list manager tries to read partition config from the root partition it will get this error.
			// For example, if in the dynamicconfig we set all task list to have 2 partitions by default, all the non-root partitions of newly created task lists will get this error.
			// This will not happen once we fully migrate partition config to database. Because in that case, root partition will always be created before non-root partitions.
			var e *types.EntityNotExistsError
			if !errors.As(err, &e) {
				c.Stop()
				return err
			}
		} else {
			c.partitionConfig = info.AdaptivePartitionConfig.ToInternalType()
		}
	}
	if err := c.taskWriter.Start(); err != nil {
		c.Stop()
		return err
	}
	if c.taskListID.IsRoot() && c.taskListKind == types.TaskListKindNormal {
		c.partitionConfig = c.db.PartitionConfig().ToInternalType()
		c.logger.Info("get task list partition config from db", tag.Dynamic("root-partition", c.taskListID.GetRoot()), tag.Dynamic("task-list-partition-config", c.partitionConfig))
		if c.partitionConfig != nil {
			startConfig := c.partitionConfig
			// push update notification to all non-root partitions on start
			c.stopWG.Add(1)
			go func() {
				defer c.stopWG.Done()
				c.notifyPartitionConfig(context.Background(), nil, startConfig)
			}()
		}
	}
	c.liveness.Start()
	c.taskReader.Start()
	c.qpsTracker.Start()
	if c.adaptiveScaler != nil {
		c.adaptiveScaler.Start()
	}

	return nil
}

// Stop stops task list manager and calls Stop on all background child objects
func (c *taskListManagerImpl) Stop() {
	c.stoppedLock.Lock()
	defer c.stoppedLock.Unlock()
	if !atomic.CompareAndSwapInt32(&c.stopped, 0, 1) {
		return
	}
	c.closeCallback(c)
	if c.adaptiveScaler != nil {
		c.adaptiveScaler.Stop()
	}
	c.qpsTracker.Stop()
	c.liveness.Stop()
	c.taskWriter.Stop()
	c.taskReader.Stop()
	c.matcher.DisconnectBlockedPollers()
	c.stopWG.Wait()
	c.logger.Info("Task list manager state changed", tag.LifeCycleStopped)
}

func (c *taskListManagerImpl) handleErr(err error) error {
	var e *persistence.ConditionFailedError
	if errors.As(err, &e) {
		// This indicates the task list may have moved to another host.
		c.scope.IncCounter(metrics.ConditionFailedErrorPerTaskListCounter)
		c.logger.Info("Stopping task list due to persistence condition failure.", tag.Error(err))
		c.Stop()
		if c.taskListKind == types.TaskListKindSticky {
			// TODO: we don't see this error in our logs, we might be able to remove this error
			err = &types.InternalServiceError{Message: constants.StickyTaskConditionFailedErrorMsg}
		}
	}
	return err
}

func (c *taskListManagerImpl) TaskListPartitionConfig() *types.TaskListPartitionConfig {
	c.partitionConfigLock.RLock()
	defer c.partitionConfigLock.RUnlock()

	scope := c.scope.Tagged(metrics.TaskListRootPartitionTag(c.taskListID.GetRoot()))
	if c.partitionConfig == nil {
		// if partition config is nil, read/write partition count is considered 1. Emit those metrics for continuity
		scope.UpdateGauge(metrics.TaskListPartitionConfigNumReadGauge, 1)
		scope.UpdateGauge(metrics.TaskListPartitionConfigNumWriteGauge, 1)
		return nil
	}

	config := *c.partitionConfig
	c.logger.Debug("current partition config", tag.Dynamic("root-partition", c.taskListID.GetRoot()), tag.Dynamic("config", config))
	scope.UpdateGauge(metrics.TaskListPartitionConfigNumReadGauge, float64(len(config.ReadPartitions)))
	scope.UpdateGauge(metrics.TaskListPartitionConfigNumWriteGauge, float64(len(config.WritePartitions)))
	scope.UpdateGauge(metrics.TaskListPartitionConfigVersionGauge, float64(config.Version))
	return &config
}

func (c *taskListManagerImpl) PartitionReadConfig() (*types.TaskListPartition, bool) {
	tlConfig := c.TaskListPartitionConfig()
	// no config indicates 1 partition
	if tlConfig == nil {
		return defaultPartition, true
	}
	partition, ok := tlConfig.ReadPartitions[c.taskListID.Partition()]
	return partition, ok
}

func (c *taskListManagerImpl) PartitionWriteConfig() (*types.TaskListPartition, bool) {
	tlConfig := c.TaskListPartitionConfig()
	// no config indicates 1 partition
	if tlConfig == nil {
		return defaultPartition, true
	}
	partition, ok := tlConfig.WritePartitions[c.taskListID.Partition()]
	return partition, ok
}

func (c *taskListManagerImpl) LoadBalancerHints() *types.LoadBalancerHints {
	c.startWG.Wait()
	if c.timeSource.Now().Sub(c.createTime) < time.Second*10 {
		return nil
	}
	return &types.LoadBalancerHints{
		BacklogCount:  c.taskAckManager.GetBacklogCount(),
		RatePerSecond: c.qpsTracker.QPS(),
	}
}

func isTaskListPartitionConfigEqual(a types.TaskListPartitionConfig, b types.TaskListPartitionConfig) bool {
	a.Version = b.Version
	return reflect.DeepEqual(a, b)
}

func (c *taskListManagerImpl) RefreshTaskListPartitionConfig(ctx context.Context, config *types.TaskListPartitionConfig) error {
	c.startWG.Wait()
	if config == nil {
		// if config is nil, we'll reload it from database
		var info *persistence.TaskListInfo
		err := c.throttleRetry.Do(ctx, func(ctx context.Context) error {
			var err error
			info, err = c.db.GetTaskListInfo(c.taskListID.GetRoot())
			return err
		})
		if err != nil {
			return err
		}
		config = info.AdaptivePartitionConfig.ToInternalType()
		c.partitionConfigLock.Lock()
		c.partitionConfig = config
		c.partitionConfigLock.Unlock()
		return nil
	}
	c.partitionConfigLock.Lock()
	defer c.partitionConfigLock.Unlock()
	if c.partitionConfig == nil || c.partitionConfig.Version < config.Version {
		c.partitionConfig = config
	}
	return nil
}

// UpdateTaskListPartitionConfig updates the task list partition config. It is called by adaptive scaler component on the root partition.
// Root tasklist manager will update the partition config in the database and notify all non-root partitions.
func (c *taskListManagerImpl) UpdateTaskListPartitionConfig(ctx context.Context, config *types.TaskListPartitionConfig) error {
	c.startWG.Wait()
	oldConfig, newConfig, err := c.updatePartitionConfig(ctx, config)
	if err != nil {
		return err
	}
	if newConfig != nil {
		// push update notification to all non-root partitions
		c.notifyPartitionConfig(ctx, oldConfig, newConfig)
	}
	return nil
}

func (c *taskListManagerImpl) updatePartitionConfig(ctx context.Context, newConfig *types.TaskListPartitionConfig) (*types.TaskListPartitionConfig, *types.TaskListPartitionConfig, error) {
	err := validatePartitionConfig(newConfig)
	if err != nil {
		return nil, nil, err
	}
	var version int64
	c.partitionConfigLock.Lock()
	defer c.partitionConfigLock.Unlock()
	oldConfig := c.partitionConfig
	if oldConfig != nil {
		if isTaskListPartitionConfigEqual(*oldConfig, *newConfig) {
			return nil, nil, nil
		}
		version = oldConfig.Version
	} else {
		if len(newConfig.ReadPartitions) == 1 && len(newConfig.WritePartitions) == 1 {
			return nil, nil, nil
		}
	}
	err = c.throttleRetry.Do(ctx, func(ctx context.Context) error {
		return c.db.UpdateTaskListPartitionConfig(toPersistenceConfig(version+1, newConfig))
	})
	if err != nil {
		// We're not sure whether the update was persisted or not,
		// Stop the tasklist manager and let it be reloaded
		c.scope.IncCounter(metrics.TaskListPartitionUpdateFailedCounter)
		c.Stop()
		return nil, nil, err
	}
	c.partitionConfig = c.db.PartitionConfig().ToInternalType()
	return oldConfig, c.partitionConfig, nil
}

func (c *taskListManagerImpl) notifyPartitionConfig(ctx context.Context, oldConfig, newConfig *types.TaskListPartitionConfig) {
	taskListType := types.TaskListTypeDecision.Ptr()
	if c.taskListID.GetType() == persistence.TaskListTypeActivity {
		taskListType = types.TaskListTypeActivity.Ptr()
	}
	toNotify := make(map[int]any)
	if oldConfig != nil {
		for id := range oldConfig.ReadPartitions {
			if id != 0 {
				toNotify[id] = true
			}
		}
	}
	for id := range newConfig.ReadPartitions {
		if id != 0 {
			toNotify[id] = true
		}
	}
	g := &errgroup.Group{}
	for p := range toNotify {
		taskListName := c.taskListID.GetPartition(p)
		g.Go(func() (e error) {
			defer func() { log.CapturePanic(recover(), c.logger, &e) }()

			_, e = c.matchingClient.RefreshTaskListPartitionConfig(ctx, &types.MatchingRefreshTaskListPartitionConfigRequest{
				DomainUUID:      c.taskListID.GetDomainID(),
				TaskList:        &types.TaskList{Name: taskListName, Kind: &c.taskListKind},
				TaskListType:    taskListType,
				PartitionConfig: newConfig,
			})
			if e != nil {
				c.logger.Error("failed to notify partition", tag.Error(e), tag.Dynamic("task-list-partition-name", taskListName))
			}
			return e
		})
	}
	err := g.Wait()
	if err != nil {
		c.logger.Error("failed to notify all partitions", tag.Error(err))
	}
}

// AddTask adds a task to the task list. This method will first attempt a synchronous
// match with a poller. When there are no pollers or if rate limit is exceeded, task will
// be written to database and later asynchronously matched with a poller
func (c *taskListManagerImpl) AddTask(ctx context.Context, params AddTaskParams) (bool, error) {
	c.startWG.Wait()

	if c.shouldReload() {
		c.Stop()
		return false, errShutdown
	}
	if c.config.EnableGetNumberOfPartitionsFromCache() {
		_, ok := c.PartitionWriteConfig()
		if !ok {
			return false, &types.InternalServiceError{Message: "Current partition is drained."}
		}
	}
	if params.ForwardedFrom == "" {
		// request sent by history service
		c.liveness.MarkAlive()
		if isolationGroup, ok := params.TaskInfo.PartitionConfig[isolationgroup.GroupKey]; ok {
			c.qpsTracker.ReportGroup(isolationGroup, 1)
		} else {
			c.qpsTracker.ReportCounter(1)
		}
		c.scope.UpdateGauge(metrics.EstimatedAddTaskQPSGauge, c.qpsTracker.QPS())
	}
	var syncMatch bool
	e := event.E{
		TaskListName: c.taskListID.GetName(),
		TaskListKind: &c.taskListKind,
		TaskListType: c.taskListID.GetType(),
		TaskInfo:     *params.TaskInfo,
	}
	_, err := c.executeWithRetry(func() (interface{}, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		domainEntry, err := c.domainCache.GetDomainByID(params.TaskInfo.DomainID)
		if err != nil {
			return nil, err
		}

		isForwarded := params.ForwardedFrom != ""

		if !domainEntry.IsActiveIn(c.clusterMetadata.GetCurrentClusterName()) {
			// standby task, only persist when task is not forwarded from child partition
			syncMatch = false
			if isForwarded {
				return &persistence.CreateTasksResponse{}, errRemoteSyncMatchFailed
			}

			r, err := c.taskWriter.appendTask(params.TaskInfo)
			return r, err
		}

		isolationGroup, _ := c.getIsolationGroupForTask(ctx, params.TaskInfo)
		// active task, try sync match first
		syncMatch, err = c.trySyncMatch(ctx, params, isolationGroup)
		if syncMatch {
			e.EventName = "SyncMatched so not persisted"
			event.Log(e)
			return &persistence.CreateTasksResponse{}, err
		}
		if params.ActivityTaskDispatchInfo != nil {
			return false, errRemoteSyncMatchFailed
		}

		if isForwarded {
			// forwarded from child partition - only do sync match
			// child partition will persist the task when sync match fails
			e.EventName = "Could not SyncMatched Forwarded Task so not persisted"
			event.Log(e)
			return &persistence.CreateTasksResponse{}, errRemoteSyncMatchFailed
		}

		e.EventName = "Task Sent to Writer"
		event.Log(e)
		return c.taskWriter.appendTask(params.TaskInfo)
	})

	if err == nil && !syncMatch {
		c.taskReader.Signal()
	}

	return syncMatch, err
}

// DispatchTask dispatches a task to a poller on the active side. When there are no pollers to pick
// up the task or if the rate limit is exceeded, this method will return error. Task
// *will not* be persisted to db. On the passive side, dispatches the task to the taskCompleter; it will attempt
// to complete the task if it has already been started.
func (c *taskListManagerImpl) DispatchTask(ctx context.Context, task *InternalTask) error {
	// check if this is the active cluster for the domain
	domainEntry, err := c.domainCache.GetDomainByID(task.Event.TaskInfo.DomainID)
	if err != nil {
		return fmt.Errorf("unable to fetch domain from cache: %w", err)
	}

	if domainEntry.IsActiveIn(c.clusterMetadata.GetCurrentClusterName()) {
		return c.matcher.MustOffer(ctx, task)
	}

	// optional configuration to enable cleanup of tasks, in the standby cluster, that have already been started
	if c.config.EnableStandbyTaskCompletion() && !domainEntry.GetReplicationConfig().IsActiveActive() {
		if err := c.taskCompleter.CompleteTaskIfStarted(ctx, task); err != nil {
			if errors.Is(err, errDomainIsActive) {
				return c.matcher.MustOffer(ctx, task)
			}
			return err
		}

		return nil
	}

	return c.matcher.MustOffer(ctx, task)
}

// DispatchQueryTask will dispatch query to local or remote poller. If forwarded then result or error is returned,
// if dispatched to local poller then nil and nil is returned.
func (c *taskListManagerImpl) DispatchQueryTask(
	ctx context.Context,
	taskID string,
	request *types.MatchingQueryWorkflowRequest,
) (*types.MatchingQueryWorkflowResponse, error) {
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
) (*InternalTask, error) {
	if c.shouldReload() {
		c.Stop()
		return nil, ErrNoTasks
	}
	c.liveness.MarkAlive()
	// TODO: consider return early if QPS and backlog count are both 0,
	// since there is no task to be returned
	task, err := c.getTask(ctx, maxDispatchPerSecond)
	if err != nil {
		return nil, fmt.Errorf("couldn't get task: %w", err)
	}
	task.domainName = c.domainName
	task.BacklogCountHint = c.taskAckManager.GetBacklogCount()
	return task, nil
}

func (c *taskListManagerImpl) getTask(ctx context.Context, maxDispatchPerSecond *float64) (*InternalTask, error) {
	c.emitMisconfiguredPartitionMetrics()
	// We need to set a shorter timeout than the original ctx; otherwise, by the time ctx deadline is
	// reached, instead of emptyTask, context timeout error is returned to the frontend by the rpc stack,
	// which counts against our SLO. By shortening the timeout by a very small amount, the emptyTask can be
	// returned to the handler before a context timeout error is generated.
	childCtx, cancel := c.newChildContext(ctx, c.config.LongPollExpirationInterval(), returnEmptyTaskTimeBudget)
	defer cancel()

	isolationGroup := IsolationGroupFromContext(ctx)
	pollerID := PollerIDFromContext(ctx)
	identity := IdentityFromContext(ctx)
	rps := c.config.TaskDispatchRPS
	if maxDispatchPerSecond != nil {
		rps = *maxDispatchPerSecond
		c.limiter.ReportLimit(rps)
	}
	c.pollers.StartPoll(pollerID, cancel, &poller.Info{
		Identity:       identity,
		IsolationGroup: isolationGroup,
		RatePerSecond:  rps,
	})
	defer c.pollers.EndPoll(pollerID)

	domainEntry, err := c.domainCache.GetDomainByID(c.taskListID.GetDomainID())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch domain from cache: %w", err)
	}

	if !domainEntry.IsActiveIn(c.clusterMetadata.GetCurrentClusterName()) {
		return c.matcher.PollForQuery(childCtx)
	}

	if c.isIsolationMatcherEnabled() {
		return c.matcher.Poll(childCtx, isolationGroup)
	}
	return c.matcher.Poll(childCtx, "")
}

// GetAllPollerInfo returns all pollers that polled from this tasklist in last few minutes
func (c *taskListManagerImpl) GetAllPollerInfo() []*types.PollerInfo {
	return c.pollers.ListInfo()
}

// HasPollerAfter checks if there is any poller after a timestamp
func (c *taskListManagerImpl) HasPollerAfter(accessTime time.Time) bool {
	return c.pollers.HasPollerAfter(accessTime)
}

func (c *taskListManagerImpl) CancelPoller(pollerID string) {
	if c.pollers.CancelPoll(pollerID) {
		c.logger.Info("canceled outstanding poller", tag.WorkflowDomainName(c.domainName))
	}
}

// DescribeTaskList returns information about the target tasklist, right now this API returns the
// pollers which polled this tasklist in last few minutes and status of tasklist's ackManager
// (readLevel, ackLevel, backlogCountHint and taskIDBlock).
func (c *taskListManagerImpl) DescribeTaskList(includeTaskListStatus bool) *types.DescribeTaskListResponse {
	response := &types.DescribeTaskListResponse{
		Pollers: c.GetAllPollerInfo(),
		TaskList: &types.TaskList{
			Name: c.taskListID.GetName(),
			Kind: &c.taskListKind,
		},
	}
	response.PartitionConfig = c.TaskListPartitionConfig()
	if !includeTaskListStatus {
		return response
	}

	idBlock := rangeIDToTaskIDBlock(c.db.RangeID(), c.config.RangeSize)
	isolationGroups := c.config.AllIsolationGroups()
	pollerCounts := c.pollers.GetCountByIsolationGroup(c.timeSource.Now().Add(-1 * c.config.TaskIsolationPollerWindow()))
	isolationGroupMetrics := make(map[string]*types.IsolationGroupMetrics, len(isolationGroups))
	for _, group := range isolationGroups {
		isolationGroupMetrics[group] = &types.IsolationGroupMetrics{
			NewTasksPerSecond: c.qpsTracker.GroupQPS(group),
			PollerCount:       int64(pollerCounts[group]),
		}
	}
	response.TaskListStatus = &types.TaskListStatus{
		ReadLevel:        c.taskAckManager.GetReadLevel(),
		AckLevel:         c.taskAckManager.GetAckLevel(),
		BacklogCountHint: c.taskAckManager.GetBacklogCount(),
		RatePerSecond:    float64(c.limiter.Limit()),
		TaskIDBlock: &types.TaskIDBlock{
			StartID: idBlock.start,
			EndID:   idBlock.end,
		},
		IsolationGroupMetrics: isolationGroupMetrics,
		NewTasksPerSecond:     c.qpsTracker.QPS(),
		Empty:                 c.taskAckManager.GetAckLevel() == c.taskWriter.GetMaxReadLevel(),
	}

	return response
}

func (c *taskListManagerImpl) ReleaseBlockedPollers() error {
	c.stoppedLock.RLock()
	defer c.stoppedLock.RUnlock()

	if atomic.LoadInt32(&c.stopped) == 1 {
		c.logger.Info("Task list manager is already stopped")
		return errShutdown
	}

	c.matcher.DisconnectBlockedPollers()
	c.matcher.RefreshCancelContext()

	return nil
}

func (c *taskListManagerImpl) String() string {
	buf := new(bytes.Buffer)
	if c.taskListID.GetType() == persistence.TaskListTypeActivity {
		buf.WriteString("Activity")
	} else {
		buf.WriteString("Decision")
	}
	rangeID := c.db.RangeID()
	fmt.Fprintf(buf, " task list %v\n", c.taskListID.GetName())
	fmt.Fprintf(buf, "RangeID=%v\n", rangeID)
	fmt.Fprintf(buf, "TaskIDBlock=%+v\n", rangeIDToTaskIDBlock(rangeID, c.config.RangeSize))
	fmt.Fprintf(buf, "AckLevel=%v\n", c.taskAckManager.GetAckLevel())
	fmt.Fprintf(buf, "MaxReadLevel=%v\n", c.taskAckManager.GetReadLevel())

	return buf.String()
}

func (c *taskListManagerImpl) GetTaskListKind() types.TaskListKind {
	return c.taskListKind
}

func (c *taskListManagerImpl) TaskListID() *Identifier {
	return c.taskListID
}

// Retry operation on transient error. On rangeID update by another process calls c.Stop().
func (c *taskListManagerImpl) executeWithRetry(
	operation func() (interface{}, error),
) (result interface{}, err error) {

	op := func(ctx context.Context) error {
		result, err = operation()
		return err
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(persistenceOperationRetryPolicy),
		backoff.WithRetryableError(persistence.IsTransientError),
	)
	err = c.handleErr(throttleRetry.Do(context.Background(), op))
	return
}

func (c *taskListManagerImpl) trySyncMatch(ctx context.Context, params AddTaskParams, isolationGroup string) (bool, error) {
	task := newInternalTask(params.TaskInfo, nil, params.Source, params.ForwardedFrom, true, params.ActivityTaskDispatchInfo, isolationGroup)
	childCtx := ctx
	cancel := func() {}
	waitTime := maxSyncMatchWaitTime
	if params.ActivityTaskDispatchInfo != nil {
		waitTime = c.config.ActivityTaskSyncMatchWaitTime(params.ActivityTaskDispatchInfo.WorkflowDomain)
	}
	if !task.IsForwarded() {
		// when task is forwarded from another matching host, we trust the context as is
		// otherwise, we override to limit the amount of time we can block on sync match
		childCtx, cancel = c.newChildContext(ctx, waitTime, time.Second)
	}
	var matched bool
	var err error
	if params.ActivityTaskDispatchInfo != nil {
		matched, err = c.matcher.OfferOrTimeout(childCtx, c.timeSource.Now(), task)
	} else {
		matched, err = c.matcher.Offer(childCtx, task)
	}
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
	remaining := time.Until(deadline) - tailroom
	if remaining < timeout {
		timeout = time.Duration(max(0, int64(remaining)))
	}
	return context.WithTimeout(parent, timeout)
}

func (c *taskListManagerImpl) isFowardingAllowed(taskList *Identifier, kind types.TaskListKind) bool {
	return !taskList.IsRoot() && kind != types.TaskListKindSticky
}

func (c *taskListManagerImpl) isIsolationMatcherEnabled() bool {
	return c.taskListKind != types.TaskListKindSticky && c.enableIsolation
}

func (c *taskListManagerImpl) shouldReload() bool {
	return c.config.EnableTasklistIsolation() != c.enableIsolation
}

func (c *taskListManagerImpl) getIsolationGroupForTask(ctx context.Context, taskInfo *persistence.TaskInfo) (string, time.Duration) {
	if !c.enableIsolation || len(taskInfo.PartitionConfig) == 0 || c.taskListKind == types.TaskListKindSticky {
		return defaultTaskBufferIsolationGroup, noIsolationTimeout
	}
	group := taskInfo.PartitionConfig[isolationgroup.GroupKey]

	if group == defaultTaskBufferIsolationGroup {
		return defaultTaskBufferIsolationGroup, noIsolationTimeout
	}

	isDrained, err := c.isolationState.IsDrained(ctx, c.domainName, group)
	if err != nil {
		// if we're unable to get the isolation group, log the error and fallback to no isolation
		c.logger.Error("Failed to determine whether isolation group is drained", tag.IsolationGroup(group), tag.WorkflowID(taskInfo.WorkflowID), tag.WorkflowRunID(taskInfo.RunID), tag.TaskID(taskInfo.TaskID), tag.Error(err))
		c.scope.Tagged(metrics.IsolationGroupTag(group), IsolationLeakCauseError).IncCounter(metrics.TaskIsolationLeakPerTaskList)
		return defaultTaskBufferIsolationGroup, noIsolationTimeout
	}
	if isDrained {
		c.scope.Tagged(metrics.IsolationGroupTag(group), IsolationLeakCauseGroupDrained).IncCounter(metrics.TaskIsolationLeakPerTaskList)
		return defaultTaskBufferIsolationGroup, noIsolationTimeout
	}
	if !c.pollers.HasPollerFromIsolationGroupAfter(group, c.timeSource.Now().Add(-1*c.config.TaskIsolationPollerWindow())) {
		c.scope.Tagged(metrics.IsolationGroupTag(group), IsolationLeakCauseNoRecentPollers).IncCounter(metrics.TaskIsolationLeakPerTaskList)
		return defaultTaskBufferIsolationGroup, noIsolationTimeout
	}
	partition, ok := c.PartitionReadConfig()
	if !ok || (len(partition.IsolationGroups) != 0 && !slices.Contains(partition.IsolationGroups, group)) {
		c.scope.Tagged(metrics.IsolationGroupTag(group), IsolationLeakCausePartitionChange).IncCounter(metrics.TaskIsolationLeakPerTaskList)
		return defaultTaskBufferIsolationGroup, noIsolationTimeout
	}

	totalTaskIsolationDuration := c.config.TaskIsolationDuration()
	taskIsolationDuration := noIsolationTimeout
	if totalTaskIsolationDuration != noIsolationTimeout {
		taskLatency := c.timeSource.Now().Sub(taskInfo.CreatedTime)
		if taskLatency < (totalTaskIsolationDuration - minimumIsolationDuration) {
			taskIsolationDuration = totalTaskIsolationDuration - taskLatency
		} else {
			c.logger.Info("Leaking task due to taskIsolationDuration expired", tag.IsolationGroup(group), tag.IsolationDuration(taskIsolationDuration), tag.TaskLatency(taskLatency))
			c.scope.Tagged(metrics.IsolationGroupTag(group), IsolationLeakCauseExpired).IncCounter(metrics.TaskIsolationLeakPerTaskList)
			return defaultTaskBufferIsolationGroup, noIsolationTimeout
		}
	}
	return group, taskIsolationDuration

}

func (c *taskListManagerImpl) emitMisconfiguredPartitionMetrics() {
	if !c.taskListID.IsRoot() || c.taskListKind == types.TaskListKindSticky {
		// only emit the metric in root partition of non-sticky tasklist
		return
	}
	if c.config.NumReadPartitions() != c.config.NumWritePartitions() {
		c.scope.UpdateGauge(metrics.TaskListReadWritePartitionMismatchGauge, 1)
	}
	pollerCount := c.pollers.GetCount()
	if pollerCount < c.config.NumReadPartitions() || pollerCount < c.config.NumWritePartitions() {
		c.scope.Tagged(metrics.IsolationEnabledTag(c.enableIsolation)).UpdateGauge(metrics.TaskListPollerPartitionMismatchGauge, 1)
	}
}

func getTaskListTypeTag(taskListType int) metrics.Tag {
	switch taskListType {
	case persistence.TaskListTypeActivity:
		return taskListActivityTypeTag
	case persistence.TaskListTypeDecision:
		return taskListDecisionTypeTag
	default:
		return metrics.TaskListTypeTag("")
	}
}

func createServiceBusyError(msg string) *types.ServiceBusyError {
	return &types.ServiceBusyError{Message: msg}
}

func rangeIDToTaskIDBlock(rangeID, rangeSize int64) taskIDBlock {
	return taskIDBlock{
		start: (rangeID-1)*rangeSize + 1,
		end:   rangeID * rangeSize,
	}
}

func toPersistenceConfig(version int64, config *types.TaskListPartitionConfig) *persistence.TaskListPartitionConfig {
	read := make(map[int]*persistence.TaskListPartition, len(config.ReadPartitions))
	write := make(map[int]*persistence.TaskListPartition, len(config.WritePartitions))
	for id, p := range config.ReadPartitions {
		read[id] = &persistence.TaskListPartition{IsolationGroups: p.IsolationGroups}
	}
	for id, p := range config.WritePartitions {
		write[id] = &persistence.TaskListPartition{IsolationGroups: p.IsolationGroups}
	}
	return &persistence.TaskListPartitionConfig{
		Version:         version,
		ReadPartitions:  read,
		WritePartitions: write,
	}
}

func validatePartitionConfig(config *types.TaskListPartitionConfig) error {
	if len(config.ReadPartitions) < 1 {
		return &types.BadRequestError{Message: "read partitions < 1"}
	}
	if len(config.WritePartitions) < 1 {
		return &types.BadRequestError{Message: "write partitions < 1"}
	}
	if len(config.ReadPartitions) < len(config.WritePartitions) {
		return &types.BadRequestError{Message: fmt.Sprintf("read partitions (%d) < write partitions (%d)", len(config.ReadPartitions), len(config.WritePartitions))}
	}
	if _, ok := config.ReadPartitions[0]; !ok {
		return &types.BadRequestError{Message: "the root partition must always be in read partitions"}
	}
	if _, ok := config.WritePartitions[0]; !ok {
		return &types.BadRequestError{Message: "the root partition must always be in write partitions"}
	}
	for id := range config.WritePartitions {
		if _, ok := config.ReadPartitions[id]; !ok {
			return &types.BadRequestError{Message: fmt.Sprintf("partition %d included in write but not read", id)}
		}
	}
	return nil
}

func newTaskListConfig(id *Identifier, cfg *config.Config, domainName string) *config.TaskListConfig {
	taskListName := id.GetName()
	taskType := id.GetType()
	return &config.TaskListConfig{
		RangeSize:          cfg.RangeSize,
		ReadRangeSize:      cfg.ReadRangeSize,
		AllIsolationGroups: cfg.AllIsolationGroups,
		EnableTasklistIsolation: func() bool {
			return cfg.EnableTasklistIsolation(domainName)
		},
		ActivityTaskSyncMatchWaitTime: cfg.ActivityTaskSyncMatchWaitTime,
		GetTasksBatchSize: func() int {
			return cfg.GetTasksBatchSize(domainName, taskListName, taskType)
		},
		UpdateAckInterval: func() time.Duration {
			return cfg.UpdateAckInterval(domainName, taskListName, taskType)
		},
		IdleTasklistCheckInterval: func() time.Duration {
			return cfg.IdleTasklistCheckInterval(domainName, taskListName, taskType)
		},
		MaxTasklistIdleTime: func() time.Duration {
			return cfg.MaxTasklistIdleTime(domainName, taskListName, taskType)
		},
		MinTaskThrottlingBurstSize: func() int {
			return cfg.MinTaskThrottlingBurstSize(domainName, taskListName, taskType)
		},
		EnableSyncMatch: func() bool {
			return cfg.EnableSyncMatch(domainName, taskListName, taskType)
		},
		LongPollExpirationInterval: func() time.Duration {
			return cfg.LongPollExpirationInterval(domainName, taskListName, taskType)
		},
		MaxTaskDeleteBatchSize: func() int {
			return cfg.MaxTaskDeleteBatchSize(domainName, taskListName, taskType)
		},
		OutstandingTaskAppendsThreshold: func() int {
			return cfg.OutstandingTaskAppendsThreshold(domainName, taskListName, taskType)
		},
		MaxTaskBatchSize: func() int {
			return cfg.MaxTaskBatchSize(domainName, taskListName, taskType)
		},
		NumWritePartitions: func() int {
			return max(1, cfg.NumTasklistWritePartitions(domainName, taskListName, taskType))
		},
		NumReadPartitions: func() int {
			return max(1, cfg.NumTasklistReadPartitions(domainName, taskListName, taskType))
		},
		EnableGetNumberOfPartitionsFromCache: func() bool {
			return cfg.EnableGetNumberOfPartitionsFromCache(domainName, id.GetRoot(), taskType)
		},
		AsyncTaskDispatchTimeout: func() time.Duration {
			return cfg.AsyncTaskDispatchTimeout(domainName, taskListName, taskType)
		},
		LocalPollWaitTime: func() time.Duration {
			return cfg.LocalPollWaitTime(domainName, taskListName, taskType)
		},
		LocalTaskWaitTime: func() time.Duration {
			return cfg.LocalTaskWaitTime(domainName, taskListName, taskType)
		},
		PartitionUpscaleRPS: func() int {
			return cfg.PartitionUpscaleRPS(domainName, taskListName, taskType)
		},
		PartitionDownscaleFactor: func() float64 {
			return cfg.PartitionDownscaleFactor(domainName, taskListName, taskType)
		},
		PartitionUpscaleSustainedDuration: func() time.Duration {
			return cfg.PartitionUpscaleSustainedDuration(domainName, taskListName, taskType)
		},
		PartitionDownscaleSustainedDuration: func() time.Duration {
			return cfg.PartitionDownscaleSustainedDuration(domainName, taskListName, taskType)
		},
		AdaptiveScalerUpdateInterval: func() time.Duration {
			return cfg.AdaptiveScalerUpdateInterval(domainName, taskListName, taskType)
		},
		EnablePartitionIsolationGroupAssignment: func() bool {
			return cfg.EnablePartitionIsolationGroupAssignment(domainName, taskListName, taskType)
		},
		IsolationGroupUpscaleSustainedDuration: func() time.Duration {
			return cfg.IsolationGroupUpscaleSustainedDuration(domainName, taskListName, taskType)
		},
		IsolationGroupDownscaleSustainedDuration: func() time.Duration {
			return cfg.IsolationGroupDownscaleSustainedDuration(domainName, taskListName, taskType)
		},
		IsolationGroupHasPollersSustainedDuration: func() time.Duration {
			return cfg.IsolationGroupHasPollersSustainedDuration(domainName, taskListName, taskType)
		},
		IsolationGroupNoPollersSustainedDuration: func() time.Duration {
			return cfg.IsolationGroupNoPollersSustainedDuration(domainName, taskListName, taskType)
		},
		IsolationGroupsPerPartition: func() int {
			return cfg.IsolationGroupsPerPartition(domainName, taskListName, taskType)
		},
		QPSTrackerInterval: func() time.Duration {
			return cfg.QPSTrackerInterval(domainName, taskListName, taskType)
		},
		EnableAdaptiveScaler: func() bool {
			return cfg.EnableAdaptiveScaler(domainName, taskListName, taskType)
		},
		EnablePartitionEmptyCheck: func() bool {
			return cfg.EnablePartitionEmptyCheck(domainName, taskListName, taskType)
		},
		TaskIsolationDuration: func() time.Duration {
			return cfg.TaskIsolationDuration(domainName, taskListName, taskType)
		},
		TaskIsolationPollerWindow: func() time.Duration {
			return cfg.TaskIsolationPollerWindow(domainName, taskListName, taskType)
		},
		ForwarderConfig: config.ForwarderConfig{
			ForwarderMaxOutstandingPolls: func() int {
				return cfg.ForwarderMaxOutstandingPolls(domainName, taskListName, taskType)
			},
			ForwarderMaxOutstandingTasks: func() int {
				return cfg.ForwarderMaxOutstandingTasks(domainName, taskListName, taskType)
			},
			ForwarderMaxRatePerSecond: func() int {
				return cfg.ForwarderMaxRatePerSecond(domainName, taskListName, taskType)
			},
			ForwarderMaxChildrenPerNode: func() int {
				return max(1, cfg.ForwarderMaxChildrenPerNode(domainName, taskListName, taskType))
			},
		},
		HostName:                  cfg.HostName,
		TaskDispatchRPS:           cfg.TaskDispatchRPS,
		TaskDispatchRPSTTL:        cfg.TaskDispatchRPSTTL,
		MaxTimeBetweenTaskDeletes: cfg.MaxTimeBetweenTaskDeletes,
		EnableStandbyTaskCompletion: func() bool {
			return cfg.EnableStandbyTaskCompletion(domainName, taskListName, taskType)
		},
		EnableClientAutoConfig: func() bool {
			return cfg.EnableClientAutoConfig(domainName, taskListName, taskType)
		},
	}
}

func IdentityFromContext(ctx context.Context) string {
	val, ok := ctx.Value(identityCtxKey{}).(string)
	if !ok {
		return ""
	}
	return val
}

func ContextWithIdentity(ctx context.Context, identity string) context.Context {
	return context.WithValue(ctx, identityCtxKey{}, identity)
}

func PollerIDFromContext(ctx context.Context) string {
	val, ok := ctx.Value(pollerIDCtxKey{}).(string)
	if !ok {
		return ""
	}
	return val
}

func ContextWithPollerID(ctx context.Context, pollerID string) context.Context {
	return context.WithValue(ctx, pollerIDCtxKey{}, pollerID)
}

func IsolationGroupFromContext(ctx context.Context) string {
	val, ok := ctx.Value(isolationGroupCtxKey{}).(string)
	if !ok {
		return ""
	}
	return val
}

func ContextWithIsolationGroup(ctx context.Context, isolationGroup string) context.Context {
	return context.WithValue(ctx, isolationGroupCtxKey{}, isolationGroup)
}
