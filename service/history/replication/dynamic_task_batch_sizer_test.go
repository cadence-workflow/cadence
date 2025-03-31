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

package replication

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

// TestDynamicTaskBatchSizer checks that batch size is changed dynamically based on the results of GetTasks
func TestDynamicTaskBatchSizer(t *testing.T) {
	var (
		// range [0, 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000]
		// start point: 500
		min       = 0
		max       = 1000
		stepCount = 11

		sizer = NewDynamicTaskBatchSizer(0, testlogger.New(t), &config.Config{
			ReplicatorProcessorMinTaskBatchSize:   func(_ int) int { return min },
			ReplicatorProcessorMaxTaskBatchSize:   func(_ int) int { return max },
			ReplicatorProcessorBatchSizeStepCount: func(_ int) int { return stepCount },
		}, metrics.NewNoopMetricsClient()).(*dynamicTaskBatchSizerImpl)
	)

	t.Run("initial batch size", func(t *testing.T) {
		assert.Equal(t, 500, sizer.value())
	})

	t.Run("an error occurred 3 times", func(t *testing.T) {
		err := errors.New("error")

		sizer.analyse(err, nil)
		assert.Equal(t, 400, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())

		sizer.analyse(err, nil)
		assert.Equal(t, 300, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())

		sizer.analyse(err, nil)
		assert.Equal(t, 200, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())
	})

	t.Run("read level has changed 2 times and there are tasks in db", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 0,
			lastReadTaskID:     10,
			taskInfos:          make([]persistence.Task, 10),
			msgs: &types.ReplicationMessages{
				HasMore: true,
			},
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 300, sizer.value())
		assert.True(t, sizer.isFetchedTasks.Load())

		sizer.analyse(nil, state)
		assert.Equal(t, 400, sizer.value())
		assert.True(t, sizer.isFetchedTasks.Load())
	})

	t.Run("read level has changed, there are tasks in db, but shrunk", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 0,
			lastReadTaskID:     10,
			msgs: &types.ReplicationMessages{
				HasMore: true,
			},
			taskInfos: make([]persistence.Task, 10),
			isShrunk:  true,
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 300, sizer.value())
		assert.True(t, sizer.isFetchedTasks.Load())
	})

	t.Run("read level has not changed, replication tasks are returned", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 10,
			lastReadTaskID:     10,
			taskInfos:          make([]persistence.Task, 10),
			msgs: &types.ReplicationMessages{
				HasMore: true,
			},
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 200, sizer.value())
		assert.True(t, sizer.isFetchedTasks.Load())
	})

	t.Run("read level has not changed, replication tasks are not returned", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 10,
			lastReadTaskID:     10,
			msgs: &types.ReplicationMessages{
				HasMore: false,
			},
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 200, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())
	})

	t.Run("read level has changed 2 times and there are no tasks in db", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 0,
			lastReadTaskID:     10,
			msgs: &types.ReplicationMessages{
				HasMore: false,
			},
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 200, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())

		sizer.analyse(nil, state)
		assert.Equal(t, 200, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())
	})

	t.Run("an error occurred 3 times", func(t *testing.T) {
		err := errors.New("error")

		sizer.analyse(err, nil)
		assert.Equal(t, 100, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())

		sizer.analyse(err, nil)
		assert.Equal(t, 0, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())

		sizer.analyse(err, nil)
		assert.Equal(t, 0, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())
	})

	t.Run("max has been changed to 500", func(t *testing.T) {
		// range [0, 50, 100, 150, 200, 250, 300, 350, 400, 450, 500]
		// start point: 250
		max = 500

		assert.Equal(t, 250, sizer.value())
		assert.False(t, sizer.isFetchedTasks.Load())
	})

	t.Run("shrunk 10 times", func(t *testing.T) {
		var want = []int{
			200, 150, 100, 50, 0,
			0, 0, 0, 0, 0,
		}

		for i := 0; i < len(want); i++ {
			state := &getTasksResult{
				taskInfos: make([]persistence.Task, 10),
				isShrunk:  true,
			}

			sizer.analyse(nil, state)
			assert.Equal(t, want[i], sizer.value())
			assert.True(t, sizer.isFetchedTasks.Load())
		}
	})

	t.Run("read level has changed and there are tasks in db", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 0,
			lastReadTaskID:     10,
			taskInfos:          make([]persistence.Task, 10),
			msgs: &types.ReplicationMessages{
				HasMore: true,
			},
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 50, sizer.value())
		assert.True(t, sizer.isFetchedTasks.Load())
	})

	t.Run("read level has changed and there are no tasks in db", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 0,
			lastReadTaskID:     10,
			taskInfos:          make([]persistence.Task, 5),
			msgs: &types.ReplicationMessages{
				HasMore: false,
			},
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 50, sizer.value())
		assert.True(t, sizer.isFetchedTasks.Load())
	})

	t.Run("read level has not changed, there are returned tasks, previously tasks were returned", func(t *testing.T) {
		state := &getTasksResult{
			previousReadTaskID: 10,
			lastReadTaskID:     10,
			taskInfos:          make([]persistence.Task, 5),
			msgs: &types.ReplicationMessages{
				HasMore: false,
			},
		}

		sizer.analyse(nil, state)
		assert.Equal(t, 0, sizer.value())
		assert.True(t, sizer.isFetchedTasks.Load())
	})
}
