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

package task

import (
	"sync"
	"time"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	ctask "github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/shard"
)

type shadowTask struct {
	sync.Mutex
	Info

	shard      shard.Context
	state      ctask.State
	priority   int
	attempt    int
	submitTime time.Time
	logger     log.Logger
	scopeIdx   int
	scope      metrics.Scope // initialized when processing task to make the initialization parallel

	// TODO: following three fields should be removed after new task lifecycle is implemented
	taskFilter        Filter
	queueType         QueueType
	shouldProcessTask bool
}

func (t *shadowTask) Execute() error {
	if t.scope == nil {
		t.scope = getOrCreateDomainTaggedScope(t.shard, t.scopeIdx, t.GetDomainID(), t.logger)
	}
	var err error
	t.shouldProcessTask, err = t.taskFilter(t.Info)
	if err != nil {
		return nil
	}
	if t.shouldProcessTask {
		t.scope.IncCounter(metrics.TaskShadowRequestsPerDomain)
	}
	time.Sleep(backoff.JitDuration(
		time.Millisecond*200,
		0.95,
	))
	return nil
}

func (t *shadowTask) HandleErr(err error) error {
	return nil
}

func (t *shadowTask) RetryErr(err error) bool {
	return false
}

func (t *shadowTask) Ack() {
	t.Lock()
	defer t.Unlock()
	t.state = ctask.TaskStateAcked
	if t.shouldProcessTask {
		t.scope.RecordTimer(metrics.TaskShadowLatencyPerDomain, time.Since(t.submitTime))
	}
}

func (t *shadowTask) Nack() {
	t.Lock()
	t.state = ctask.TaskStateNacked
	t.Unlock()
}

func (t *shadowTask) State() ctask.State {
	t.Lock()
	defer t.Unlock()
	return t.state
}

func (t *shadowTask) Priority() int {
	t.Lock()
	defer t.Unlock()
	return t.priority
}

func (t *shadowTask) SetPriority(priority int) {
	t.Lock()
	defer t.Unlock()
	t.priority = priority
}

func (t *shadowTask) GetAttempt() int {
	return 0
}

func (t *shadowTask) GetShard() shard.Context {
	return t.shard
}

func (t *shadowTask) GetInfo() Info {
	return t.Info
}

func (t *shadowTask) GetQueueType() QueueType {
	return t.queueType
}

func (t *shadowTask) ShadowCopy() Task {
	return &shadowTask{
		Info:       t.Info,
		shard:      t.shard,
		state:      ctask.TaskStatePending,
		priority:   t.Priority(),
		queueType:  t.queueType,
		scopeIdx:   t.scopeIdx,
		scope:      nil,
		logger:     t.logger,
		submitTime: t.submitTime,
		taskFilter: t.taskFilter,
	}
}
