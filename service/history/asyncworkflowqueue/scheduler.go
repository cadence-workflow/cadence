// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package asyncworkflowqueue

import (
	"context"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	dynamicquotas "github.com/uber/cadence/common/dynamicconfig/quotas"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/config"
)

type (
	// TaskScheduler is the host-level scheduler executing async workflow queue
	// tasks from all shards on a history host. Per-shard consumers submit tasks
	// into it; domains are isolated virtually via weighted round-robin
	// scheduling (AsyncWorkflowConsumerDomainWeight) and per-domain rate limits
	// (AsyncWorkflowConsumerDomainRPS).
	TaskScheduler interface {
		common.Daemon
		// Submit blocks until the task is accepted (per-domain buffer space is
		// available) or the scheduler is stopped.
		Submit(t *consumerTask) error
	}

	taskSchedulerImpl struct {
		processor task.Processor
		scheduler task.Scheduler[*consumerTask]
	}

	// domainRateLimitedProcessor throttles task execution per domain before
	// handing tasks to the underlying parallel processor. It implements the
	// generic common/task.Processor consumed by the hierarchical scheduler's
	// dispatcher.
	domainRateLimitedProcessor struct {
		baseProcessor task.Processor
		limiters      quotas.ICollection[string]
		cancelCtx     context.Context
		cancelFn      context.CancelFunc
		status        int32
	}
)

// NewTaskScheduler creates the host-level async workflow task scheduler:
// a hierarchical weighted-round-robin scheduler keyed by domain name, feeding a
// per-domain rate-limited parallel task processor.
func NewTaskScheduler(
	cfg *config.Config,
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
) (TaskScheduler, error) {
	baseProcessor := task.NewParallelTaskProcessor(
		logger,
		metricsClient,
		&task.ParallelTaskProcessorOptions{
			QueueSize:   cfg.AsyncWorkflowTaskWorkerCount(),
			WorkerCount: cfg.AsyncWorkflowTaskWorkerCount,
			RetryPolicy: common.CreateFrontendServiceRetryPolicy(),
		},
	)

	cancelCtx, cancelFn := context.WithCancel(context.Background())
	rateLimitedProcessor := &domainRateLimitedProcessor{
		baseProcessor: baseProcessor,
		limiters:      quotas.NewCollection(dynamicquotas.NewSimpleDynamicRateLimiterFactory(cfg.AsyncWorkflowConsumerDomainRPS)),
		cancelCtx:     cancelCtx,
		cancelFn:      cancelFn,
		status:        common.DaemonStatusInitialized,
	}

	scheduler, err := task.NewHierarchicalWeightedRoundRobinTaskScheduler[string, *consumerTask](
		logger,
		metricsClient,
		timeSource,
		rateLimitedProcessor,
		&task.HierarchicalWeightedRoundRobinTaskPoolOptions[string, *consumerTask]{
			BufferSize: cfg.AsyncWorkflowTaskSchedulerBufferSize(),
			TaskToWeightedKeysFn: func(t *consumerTask) []task.WeightedKey[string] {
				domain := t.Domain()
				weight := cfg.AsyncWorkflowConsumerDomainWeight(domain)
				if weight <= 0 {
					weight = 1
				}
				return []task.WeightedKey[string]{{Key: domain, Weight: weight}}
			},
		},
	)
	if err != nil {
		cancelFn()
		return nil, err
	}

	return &taskSchedulerImpl{
		processor: rateLimitedProcessor,
		scheduler: scheduler,
	}, nil
}

func (s *taskSchedulerImpl) Start() {
	s.processor.Start()
	s.scheduler.Start()
}

func (s *taskSchedulerImpl) Stop() {
	// The scheduler drains queued tasks with Nack(nil) so they are redelivered;
	// the processor then finishes in-flight tasks.
	s.scheduler.Stop()
	s.processor.Stop()
}

func (s *taskSchedulerImpl) Submit(t *consumerTask) error {
	return s.scheduler.Submit(t)
}

func (p *domainRateLimitedProcessor) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	p.baseProcessor.Start()
}

func (p *domainRateLimitedProcessor) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	p.cancelFn()
	p.baseProcessor.Stop()
}

func (p *domainRateLimitedProcessor) Submit(t task.Task) error {
	if ct, ok := t.(*consumerTask); ok {
		if err := p.limiters.For(ct.Domain()).Wait(p.cancelCtx); err != nil {
			return err
		}
	}
	return p.baseProcessor.Submit(t)
}
