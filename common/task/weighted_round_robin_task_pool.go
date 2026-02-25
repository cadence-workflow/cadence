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

package task

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
)

type weightedRoundRobinTaskPool[K comparable] struct {
	sync.Mutex
	status        int32
	pool          *WeightedRoundRobinChannelPool[K, PriorityTask]
	ctx           context.Context
	cancel        context.CancelFunc
	options       *WeightedRoundRobinTaskPoolOptions[K]
	logger        log.Logger
	taskCount     atomic.Int64        // O(1) task count tracking
	schedule      []chan PriorityTask // Current schedule
	scheduleIndex int                 // Current position in schedule
}

// NewWeightedRoundRobinTaskPool creates a new WRR task pool
func NewWeightedRoundRobinTaskPool[K comparable](
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
	options *WeightedRoundRobinTaskPoolOptions[K],
) TaskPool {
	metricsScope := metricsClient.Scope(metrics.TaskSchedulerScope)
	ctx, cancel := context.WithCancel(context.Background())

	pool := &weightedRoundRobinTaskPool[K]{
		status: common.DaemonStatusInitialized,
		pool: NewWeightedRoundRobinChannelPool[K, PriorityTask](
			logger,
			metricsScope,
			timeSource,
			WeightedRoundRobinChannelPoolOptions{
				BufferSize:              options.QueueSize,
				IdleChannelTTLInSeconds: defaultIdleChannelTTLInSeconds,
			}),
		ctx:     ctx,
		cancel:  cancel,
		options: options,
		logger:  logger,
	}

	return pool
}

func (p *weightedRoundRobinTaskPool[K]) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	p.pool.Start()

	p.logger.Info("Weighted round robin task pool started.")
}

func (p *weightedRoundRobinTaskPool[K]) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	p.cancel()
	p.pool.Stop()

	// Drain all tasks and nack them, updating the counter
	taskChs := p.pool.GetAllChannels()
	for _, taskCh := range taskChs {
		p.drainAndNackTasks(taskCh)
	}

	p.logger.Info("Weighted round robin task pool stopped.")
}

func (p *weightedRoundRobinTaskPool[K]) Submit(task PriorityTask) error {
	if p.isStopped() {
		return ErrTaskSchedulerClosed
	}

	key := p.options.TaskToChannelKeyFn(task)
	weight := p.options.ChannelKeyToWeightFn(key)
	taskCh, releaseFn := p.pool.GetOrCreateChannel(key, weight)
	defer releaseFn()

	select {
	case taskCh <- task:
		p.taskCount.Add(1)
		if p.isStopped() {
			p.drainAndNackTasks(taskCh)
		}
		return nil
	case <-p.ctx.Done():
		return ErrTaskSchedulerClosed
	}
}

func (p *weightedRoundRobinTaskPool[K]) TrySubmit(task PriorityTask) (bool, error) {
	if p.isStopped() {
		return false, ErrTaskSchedulerClosed
	}

	key := p.options.TaskToChannelKeyFn(task)
	weight := p.options.ChannelKeyToWeightFn(key)
	taskCh, releaseFn := p.pool.GetOrCreateChannel(key, weight)
	defer releaseFn()

	select {
	case taskCh <- task:
		p.taskCount.Add(1)
		if p.isStopped() {
			p.drainAndNackTasks(taskCh)
		}
		return true, nil
	case <-p.ctx.Done():
		return false, ErrTaskSchedulerClosed
	default:
		return false, nil
	}
}

func (p *weightedRoundRobinTaskPool[K]) GetNextTask() (PriorityTask, bool) {
	if p.isStopped() {
		return nil, false
	}

	p.Lock()
	defer p.Unlock()

	// Get a fresh schedule if we don't have one or if we've reached the end
	if p.schedule == nil || p.scheduleIndex >= len(p.schedule) {
		p.schedule = p.pool.GetSchedule()
		p.scheduleIndex = 0

		if len(p.schedule) == 0 {
			return nil, false
		}
	}

	// Try to get a task starting from the current index
	startIndex := p.scheduleIndex
	for {
		select {
		case task := <-p.schedule[p.scheduleIndex]:
			// Found a task, advance index and return
			p.scheduleIndex++
			p.taskCount.Add(-1)
			return task, true
		case <-p.ctx.Done():
			return nil, false
		default:
			// No task in this channel, try next
			p.scheduleIndex++

			// If we've reached the end, get a fresh schedule and continue
			if p.scheduleIndex >= len(p.schedule) {
				p.schedule = p.pool.GetSchedule()
				p.scheduleIndex = 0

				if len(p.schedule) == 0 {
					return nil, false
				}
			}

			// If we've made a full loop without finding a task, return false
			if p.scheduleIndex == startIndex {
				return nil, false
			}
		}
	}
}

func (p *weightedRoundRobinTaskPool[K]) Len() int {
	return int(p.taskCount.Load())
}

func (p *weightedRoundRobinTaskPool[K]) drainAndNackTasks(taskCh <-chan PriorityTask) {
	for {
		select {
		case task := <-taskCh:
			p.taskCount.Add(-1)
			task.Nack(nil)
		default:
			return
		}
	}
}

func (p *weightedRoundRobinTaskPool[K]) isStopped() bool {
	return atomic.LoadInt32(&p.status) == common.DaemonStatusStopped
}
