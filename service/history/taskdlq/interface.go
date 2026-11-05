// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -destination interface_mock.go github.com/uber/cadence/service/history/taskdlq TaskReinjector,Processor

package taskdlq

import (
	"context"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

// TaskReinjector writes DLQ tasks back into the executions table with fresh task IDs.
// Implemented by shard.Context.ReinjectHistoryTasks.
type TaskReinjector interface {
	ReinjectHistoryTasks(ctx context.Context, tasks []persistence.Task) error
}

// AckLevelIndex is the shard-owned in-memory view of the DLQ ack-level rows the processor reads and
// advances. Implemented by *shard.DLQAckLevelIndex.
type AckLevelIndex interface {
	Seeded() bool
	Merge(levels []persistence.HistoryDLQAckLevel)
	Lookup(domainID, clusterAttributeScope, clusterAttributeName string, dirtyOnly bool) []shard.IndexedDLQAckLevel
	All() []shard.IndexedDLQAckLevel
	Advance(level persistence.HistoryDLQAckLevel)
	MarkClean(key shard.DLQAckLevelKey, generation uint64)
}

// HostBudget paces every persistence call the DLQ processor makes; one instance is shared by all
// shards on a history host. Satisfied by quotas.Limiter.
type HostBudget interface {
	Wait(ctx context.Context) error
}
