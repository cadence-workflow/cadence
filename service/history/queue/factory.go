// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package queue

import (
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/reset"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/history/workflowcache"
	"github.com/uber/cadence/service/worker/archiver"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination factory_mock.go -self_package github.com/uber/cadence/service/history/queue

type (
	Factory interface {
		Category() persistence.HistoryTaskCategory
		CreateQueue(shard.Context, execution.Cache, invariant.Invariant) Processor
	}

	transferQueueFactory struct {
		taskProcessor  task.Processor
		archivalClient archiver.Client
		wfIDCache      workflowcache.WFCache
	}

	timerQueueFactory struct {
		taskProcessor  task.Processor
		archivalClient archiver.Client
	}
)

func NewTransferQueueFactory(
	taskProcessor task.Processor,
	archivalClient archiver.Client,
	wfIDCache workflowcache.WFCache,
) Factory {
	return &transferQueueFactory{
		taskProcessor:  taskProcessor,
		archivalClient: archivalClient,
		wfIDCache:      wfIDCache,
	}
}

func (f *transferQueueFactory) Category() persistence.HistoryTaskCategory {
	return persistence.HistoryTaskCategoryTransfer
}

func (f *transferQueueFactory) CreateQueue(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) Processor {
	workflowResetter := reset.NewWorkflowResetter(shard, executionCache, shard.GetLogger())
	return NewTransferQueueProcessor(
		shard,
		f.taskProcessor,
		executionCache,
		workflowResetter,
		f.archivalClient,
		openExecutionCheck,
		f.wfIDCache,
	)
}

func (f *timerQueueFactory) Category() persistence.HistoryTaskCategory {
	return persistence.HistoryTaskCategoryTimer
}

func NewTimerQueueFactory(
	taskProcessor task.Processor,
	archivalClient archiver.Client,
) Factory {
	return &timerQueueFactory{
		taskProcessor:  taskProcessor,
		archivalClient: archivalClient,
	}
}

func (f *timerQueueFactory) CreateQueue(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) Processor {
	return NewTimerQueueProcessor(
		shard,
		f.taskProcessor,
		executionCache,
		f.archivalClient,
		openExecutionCheck,
	)
}
