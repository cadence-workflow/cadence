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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestHierarchicalWeightedRoundRobinTaskScheduler_SchedulerContract(t *testing.T) {
	controller := gomock.NewController(t)

	realProcessor := NewParallelTaskProcessor(
		testlogger.New(t),
		metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{}),
		&ParallelTaskProcessorOptions{
			QueueSize:   1,
			WorkerCount: dynamicproperties.GetIntPropertyFn(1),
			RetryPolicy: backoff.NewExponentialRetryPolicy(time.Millisecond),
		},
	)

	// Create hierarchical scheduler with string keys based on priority
	scheduler, err := NewHierarchicalWeightedRoundRobinTaskScheduler(
		testlogger.New(t),
		metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{}),
		clock.NewMockedTimeSource(),
		realProcessor,
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, PriorityTask]{
			BufferSize: 1000,
			TaskToWeightedKeysFn: func(task PriorityTask) []WeightedKey[string] {
				priority := task.Priority()
				// Create a simple hierarchy: group -> priority
				// Groups based on priority ranges with different weights
				var group string
				var groupWeight int
				if priority == 0 {
					group = "group0"
					groupWeight = 3
				} else if priority == 1 {
					group = "group1"
					groupWeight = 2
				} else {
					group = "group2"
					groupWeight = 1
				}

				// Second level: individual priority
				priorityKey := string(rune('0' + priority))
				priorityWeight := 3 - priority
				if priorityWeight < 1 {
					priorityWeight = 1
				}

				return []WeightedKey[string]{
					{Key: group, Weight: groupWeight},
					{Key: priorityKey, Weight: priorityWeight},
				}
			},
		},
	)
	require.NoError(t, err)

	// Reuse the existing testSchedulerContract function
	testSchedulerContract(require.New(t), controller, scheduler, realProcessor)
}
