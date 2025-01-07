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

package tasklist

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/worker/scanner/executor"
)

const (
	executorMaxDeferredTasks = 10000
	taskListBatchSize        = 32 // maximum number of task list we process concurrently
	taskBatchSize            = 16
	taskListGracePeriod      = 48 * time.Hour // amount of time a executorTask list has to be idle before it becomes a candidate for deletion
)

type (
	// Scavenger is the type that holds the state for task list scavenger daemon
	Scavenger struct {
		ctx                      context.Context
		db                       p.TaskManager
		cache                    cache.DomainCache
		executor                 executor.Executor
		scope                    metrics.Scope
		logger                   log.Logger
		stats                    stats
		status                   int32
		getOrphanTasksPageSizeFn dynamicconfig.IntPropertyFn
		taskBatchSizeFn          dynamicconfig.IntPropertyFn
		maxTasksPerJobFn         dynamicconfig.IntPropertyFn
		cleanOrphans             dynamicconfig.BoolPropertyFn
		pollInterval             time.Duration
		timeSource               clock.TimeSource

		// stopC is used to signal the scavenger to stop
		stopC chan struct{}
		// stopWG is used to wait for the scavenger to stop
		stopWG sync.WaitGroup
		// stoppedC is closed once the scavenger is stopped
		stopped chan struct{}
	}

	stats struct {
		tasklist struct {
			nProcessed int64
			nDeleted   int64
		}
		task struct {
			nProcessed int64
			nDeleted   int64
		}
	}
	// Options is used to customize scavenger operations
	Options struct {
		GetOrphanTasksPageSizeFn dynamicconfig.IntPropertyFn
		TaskBatchSizeFn          dynamicconfig.IntPropertyFn
		EnableCleaning           dynamicconfig.BoolPropertyFn
		MaxTasksPerJobFn         dynamicconfig.IntPropertyFn
		ExecutorPollInterval     time.Duration
	}

	// executorTask is a runnable task that adheres to the executor.Task interface
	// for the scavenger, each of this task processes a single task list
	executorTask struct {
		taskListInfo p.TaskListInfo
		scvg         *Scavenger
	}

	// orphanExecutorTask is a runnable task that processes a limited block of
	// orphans
	orphanExecutorTask struct {
		scvg *Scavenger
	}
)

// NewScavenger returns an instance of executorTask list scavenger daemon
// The Scavenger can be started by calling the Start() method on the
// returned object. Calling the Start() method will result in one
// complete iteration over all of the task lists in the system. For
// each task list, the scavenger will attempt
//   - deletion of expired tasks in the task lists
//   - deletion of task list itself, if there are no tasks and the task list hasn't been updated for a grace period
//
// The scavenger will retry on all persistence errors infinitely and will only stop under
// two conditions
//   - either all task lists are processed successfully (or)
//   - Stop() method is called to stop the scavenger
func NewScavenger(
	ctx context.Context,
	db p.TaskManager,
	metricsClient metrics.Client,
	logger log.Logger,
	opts *Options,
	cache cache.DomainCache,
) *Scavenger {
	taskExecutor := executor.NewFixedSizePoolExecutor(
		taskListBatchSize,
		executorMaxDeferredTasks,
		metricsClient,
		metrics.TaskListScavengerScope,
	)

	if opts == nil {
		opts = &Options{}
	}

	cleanOrphans := opts.EnableCleaning
	if cleanOrphans == nil {
		cleanOrphans = func(opts ...dynamicconfig.FilterOption) bool {
			return false
		}
	}

	getOrphanTasksPageSize := opts.GetOrphanTasksPageSizeFn
	if getOrphanTasksPageSize == nil {
		getOrphanTasksPageSize = func(opts ...dynamicconfig.FilterOption) int {
			return dynamicconfig.ScannerGetOrphanTasksPageSize.DefaultInt()
		}
	}

	taskBatchSizeFn := opts.TaskBatchSizeFn
	if taskBatchSizeFn == nil {
		taskBatchSizeFn = func(opts ...dynamicconfig.FilterOption) int {
			return taskBatchSize
		}
	}

	maxTasksPerJobFn := opts.MaxTasksPerJobFn
	if maxTasksPerJobFn == nil {
		maxTasksPerJobFn = func(opts ...dynamicconfig.FilterOption) int {
			return dynamicconfig.ScannerMaxTasksProcessedPerTasklistJob.DefaultInt()
		}
	}

	pollInterval := opts.ExecutorPollInterval
	if pollInterval == 0 {
		pollInterval = time.Minute
	}
	return &Scavenger{
		ctx:                      ctx,
		db:                       db,
		cache:                    cache,
		scope:                    metricsClient.Scope(metrics.TaskListScavengerScope),
		logger:                   logger,
		stopC:                    make(chan struct{}),
		stopped:                  make(chan struct{}),
		executor:                 taskExecutor,
		cleanOrphans:             cleanOrphans,
		taskBatchSizeFn:          taskBatchSizeFn,
		pollInterval:             pollInterval,
		maxTasksPerJobFn:         maxTasksPerJobFn,
		getOrphanTasksPageSizeFn: getOrphanTasksPageSize,
		timeSource:               clock.NewRealTimeSource(),
	}
}

// Start starts the scavenger
func (s *Scavenger) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	s.logger.Info("Tasklist scavenger starting")
	s.stopWG.Add(1)
	s.executor.Start()
	go s.run()
	s.scope.IncCounter(metrics.StartedCount)
	s.logger.Info("Tasklist scavenger started")
}

// Stop stops the scavenger
func (s *Scavenger) Stop() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	s.scope.IncCounter(metrics.StoppedCount)
	s.logger.Info("Tasklist scavenger stopping")
	close(s.stopC)
	s.executor.Stop()
	s.stopWG.Wait()
	s.logger.Info("Tasklist scavenger stopped")
	close(s.stopped)
}

// Alive returns true if the scavenger is still running
func (s *Scavenger) Alive() bool {
	return atomic.LoadInt32(&s.status) == common.DaemonStatusStarted
}

// run does a single run over all executorTask lists
func (s *Scavenger) run() {
	defer func() {
		s.emitStats()
		go s.Stop()
		s.stopWG.Done()
	}()

	// Start a task to delete orphaned tasks from the tasks table, if enabled
	if s.cleanOrphans() {
		s.executor.Submit(&orphanExecutorTask{scvg: s})
	}

	var pageToken []byte
	for {
		resp, err := s.listTaskList(taskListBatchSize, pageToken)
		if err != nil {
			s.logger.Error("listTaskList error", tag.Error(err))
			return
		}

		for _, item := range resp.Items {
			atomic.AddInt64(&s.stats.tasklist.nProcessed, 1)
			if !s.executor.Submit(s.newTask(&item)) {
				return
			}
		}

		pageToken = resp.NextPageToken
		if pageToken == nil {
			break
		}
	}

	s.awaitExecutor()
}

// process is a callback function that gets invoked from within the executor.Run() method
func (s *Scavenger) process(taskListInfo *p.TaskListInfo) executor.TaskStatus {
	return s.deleteHandler(taskListInfo)
}

func (s *Scavenger) awaitExecutor() {
	outstanding := s.executor.TaskCount()
	ticker := s.timeSource.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for outstanding > 0 {
		select {
		case <-ticker.Chan():
			outstanding = s.executor.TaskCount()
			s.scope.UpdateGauge(metrics.TaskListOutstandingCount, float64(outstanding))
		case <-s.stopC:
			return
		}
	}
}

func (s *Scavenger) emitStats() {
	s.scope.UpdateGauge(metrics.TaskProcessedCount, float64(s.stats.task.nProcessed))
	s.scope.UpdateGauge(metrics.TaskDeletedCount, float64(s.stats.task.nDeleted))
	s.scope.UpdateGauge(metrics.TaskListProcessedCount, float64(s.stats.tasklist.nProcessed))
	s.scope.UpdateGauge(metrics.TaskListDeletedCount, float64(s.stats.tasklist.nDeleted))
}

// newTask returns a new instance of an executable task which will process a single task list
func (s *Scavenger) newTask(info *p.TaskListInfo) executor.Task {
	return &executorTask{
		taskListInfo: *info,
		scvg:         s,
	}
}

// Run runs the task
func (t *executorTask) Run() executor.TaskStatus {
	return t.scvg.process(&t.taskListInfo)
}

func (t *orphanExecutorTask) Run() executor.TaskStatus {
	return t.scvg.completeOrphanTasksHandler()
}
