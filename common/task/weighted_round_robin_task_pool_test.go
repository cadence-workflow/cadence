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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestWeightedRoundRobinTaskPool_Submit(t *testing.T) {
	tests := []struct {
		name        string
		queueSize   int
		numTasks    int
		wantErr     bool
		errContains string
	}{
		{
			name:      "submit single task",
			queueSize: 10,
			numTasks:  1,
			wantErr:   false,
		},
		{
			name:      "submit multiple tasks",
			queueSize: 10,
			numTasks:  5,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := testlogger.New(t)
			metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
			timeSource := clock.NewRealTimeSource()

			pool := NewWeightedRoundRobinTaskPool[int](
				logger,
				metricsClient,
				timeSource,
				&WeightedRoundRobinTaskPoolOptions[int]{
					QueueSize: tt.queueSize,
					TaskToChannelKeyFn: func(task PriorityTask) int {
						return task.Priority()
					},
					ChannelKeyToWeightFn: func(key int) int {
						return key
					},
				},
			)
			pool.Start()
			defer pool.Stop()

			for i := 0; i < tt.numTasks; i++ {
				task := NewMockPriorityTask(ctrl)
				task.EXPECT().Priority().Return(1).AnyTimes()
				task.EXPECT().SetPriority(1).AnyTimes()
				task.EXPECT().Nack(gomock.Any()).AnyTimes() // Called during shutdown

				err := pool.Submit(task)
				if tt.wantErr {
					require.Error(t, err)
					if tt.errContains != "" {
						assert.Contains(t, err.Error(), tt.errContains)
					}
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestWeightedRoundRobinTaskPool_TrySubmit(t *testing.T) {
	tests := []struct {
		name        string
		queueSize   int
		numTasks    int
		wantSuccess []bool
	}{
		{
			name:        "try submit with available space",
			queueSize:   10,
			numTasks:    3,
			wantSuccess: []bool{true, true, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := testlogger.New(t)
			metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
			timeSource := clock.NewRealTimeSource()

			pool := NewWeightedRoundRobinTaskPool[int](
				logger,
				metricsClient,
				timeSource,
				&WeightedRoundRobinTaskPoolOptions[int]{
					QueueSize: tt.queueSize,
					TaskToChannelKeyFn: func(task PriorityTask) int {
						return task.Priority()
					},
					ChannelKeyToWeightFn: func(key int) int {
						return key
					},
				},
			)
			pool.Start()
			defer pool.Stop()

			for i := 0; i < tt.numTasks; i++ {
				task := NewMockPriorityTask(ctrl)
				task.EXPECT().Priority().Return(1).AnyTimes()
				task.EXPECT().SetPriority(1).AnyTimes()
				task.EXPECT().Nack(gomock.Any()).AnyTimes() // Called during shutdown

				ok, err := pool.TrySubmit(task)
				require.NoError(t, err)
				assert.Equal(t, tt.wantSuccess[i], ok)
			}
		})
	}
}

func TestWeightedRoundRobinTaskPool_GetNextTask(t *testing.T) {
	tests := []struct {
		name          string
		submitPrios   []int
		expectTasks   bool
		expectedCount int
	}{
		{
			name:          "get tasks from non-empty pool",
			submitPrios:   []int{1, 2, 3},
			expectTasks:   true,
			expectedCount: 3,
		},
		{
			name:          "get task from empty pool",
			submitPrios:   []int{},
			expectTasks:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := testlogger.New(t)
			metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
			timeSource := clock.NewRealTimeSource()

			pool := NewWeightedRoundRobinTaskPool[int](
				logger,
				metricsClient,
				timeSource,
				&WeightedRoundRobinTaskPoolOptions[int]{
					QueueSize: 10,
					TaskToChannelKeyFn: func(task PriorityTask) int {
						return task.Priority()
					},
					ChannelKeyToWeightFn: func(key int) int {
						return key
					},
				},
			)
			pool.Start()
			defer pool.Stop()

			// Submit tasks
			for _, prio := range tt.submitPrios {
				task := NewMockPriorityTask(ctrl)
				task.EXPECT().Priority().Return(prio).AnyTimes()
				task.EXPECT().SetPriority(prio).AnyTimes()
				task.EXPECT().Nack(gomock.Any()).AnyTimes() // Called during shutdown
				err := pool.Submit(task)
				require.NoError(t, err)
			}

			// Get tasks
			count := 0
			for i := 0; i < len(tt.submitPrios)+1; i++ {
				task, ok := pool.GetNextTask()
				if ok {
					assert.NotNil(t, task)
					count++
				} else {
					break
				}
			}

			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestWeightedRoundRobinTaskPool_WeightedScheduling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := testlogger.New(t)
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
	timeSource := clock.NewRealTimeSource()

	pool := NewWeightedRoundRobinTaskPool[int](
		logger,
		metricsClient,
		timeSource,
		&WeightedRoundRobinTaskPoolOptions[int]{
			QueueSize: 100,
			TaskToChannelKeyFn: func(task PriorityTask) int {
				return task.Priority()
			},
			ChannelKeyToWeightFn: func(key int) int {
				return key
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Submit tasks with different priorities
	// Priority 3 should appear 3 times more than priority 1
	taskCounts := map[int]int{
		1: 10,
		2: 10,
		3: 10,
	}

	for prio, count := range taskCounts {
		for i := 0; i < count; i++ {
			task := NewMockPriorityTask(ctrl)
			task.EXPECT().Priority().Return(prio).AnyTimes()
			task.EXPECT().SetPriority(prio).AnyTimes()
			task.EXPECT().Nack(gomock.Any()).AnyTimes() // Called during shutdown or get
			err := pool.Submit(task)
			require.NoError(t, err)
		}
	}

	// Retrieve all tasks and count by priority
	retrievedCounts := map[int]int{}
	for {
		task, ok := pool.GetNextTask()
		if !ok {
			break
		}
		prio := task.Priority()
		retrievedCounts[prio]++
	}

	// Verify all tasks were retrieved
	assert.Equal(t, taskCounts[1], retrievedCounts[1])
	assert.Equal(t, taskCounts[2], retrievedCounts[2])
	assert.Equal(t, taskCounts[3], retrievedCounts[3])
}

func TestWeightedRoundRobinTaskPool_Lifecycle(t *testing.T) {
	tests := []struct {
		name            string
		startPool       bool
		stopPool        bool
		submitAfterStop bool
		expectError     bool
	}{
		{
			name:            "normal lifecycle",
			startPool:       true,
			stopPool:        true,
			submitAfterStop: false,
			expectError:     false,
		},
		{
			name:            "submit after stop",
			startPool:       true,
			stopPool:        true,
			submitAfterStop: true,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := testlogger.New(t)
			metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
			timeSource := clock.NewRealTimeSource()

			pool := NewWeightedRoundRobinTaskPool[int](
				logger,
				metricsClient,
				timeSource,
				&WeightedRoundRobinTaskPoolOptions[int]{
					QueueSize: 10,
					TaskToChannelKeyFn: func(task PriorityTask) int {
						return task.Priority()
					},
					ChannelKeyToWeightFn: func(key int) int {
						return key
					},
				},
			)

			if tt.startPool {
				pool.Start()
			}

			if tt.stopPool {
				pool.Stop()
			}

			if tt.submitAfterStop {
				task := NewMockPriorityTask(ctrl)
				task.EXPECT().Priority().Return(1).AnyTimes()

				err := pool.Submit(task)
				if tt.expectError {
					assert.Error(t, err)
					assert.Equal(t, ErrTaskSchedulerClosed, err)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestWeightedRoundRobinTaskPool_Len(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := testlogger.New(t)
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
	timeSource := clock.NewRealTimeSource()

	pool := NewWeightedRoundRobinTaskPool[int](
		logger,
		metricsClient,
		timeSource,
		&WeightedRoundRobinTaskPoolOptions[int]{
			QueueSize: 100,
			TaskToChannelKeyFn: func(task PriorityTask) int {
				return task.Priority()
			},
			ChannelKeyToWeightFn: func(key int) int {
				return key
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Initially empty
	assert.Equal(t, 0, pool.Len())

	// Submit some tasks
	for i := 0; i < 10; i++ {
		task := NewMockPriorityTask(ctrl)
		task.EXPECT().Priority().Return(1).AnyTimes()
		task.EXPECT().Nack(gomock.Any()).AnyTimes()
		err := pool.Submit(task)
		require.NoError(t, err)
	}

	// Should have 10 tasks
	assert.Equal(t, 10, pool.Len())

	// Get some tasks
	for i := 0; i < 5; i++ {
		task, ok := pool.GetNextTask()
		assert.True(t, ok)
		assert.NotNil(t, task)
	}

	// Should have 5 tasks remaining
	assert.Equal(t, 5, pool.Len())

	// Get remaining tasks
	for i := 0; i < 5; i++ {
		task, ok := pool.GetNextTask()
		assert.True(t, ok)
		assert.NotNil(t, task)
	}

	// Should be empty
	assert.Equal(t, 0, pool.Len())
}

func TestWeightedRoundRobinTaskPool_WeightedOrderingWithMultipleKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := testlogger.New(t)
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
	timeSource := clock.NewRealTimeSource()

	// Setup: 3 priorities with weights [3, 2, 1]
	// Priority 0 -> weight 3
	// Priority 1 -> weight 2
	// Priority 2 -> weight 1
	// Expected IWRR schedule: [0, 0, 1, 0, 1, 2] (interleaved weighted round-robin)
	pool := NewWeightedRoundRobinTaskPool[int](
		logger,
		metricsClient,
		timeSource,
		&WeightedRoundRobinTaskPoolOptions[int]{
			QueueSize: 100,
			TaskToChannelKeyFn: func(task PriorityTask) int {
				return task.Priority()
			},
			ChannelKeyToWeightFn: func(key int) int {
				weights := map[int]int{
					0: 3,
					1: 2,
					2: 1,
				}
				return weights[key]
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Submit 2 complete cycles worth of tasks (12 tasks total)
	// 6 tasks for priority 0, 4 tasks for priority 1, 2 tasks for priority 2
	tasksPerPriority := map[int]int{
		0: 6, // weight 3, appears 3 times per cycle, 2 cycles = 6 tasks
		1: 4, // weight 2, appears 2 times per cycle, 2 cycles = 4 tasks
		2: 2, // weight 1, appears 1 time per cycle, 2 cycles = 2 tasks
	}

	// Create and submit tasks
	for priority, count := range tasksPerPriority {
		for i := 0; i < count; i++ {
			task := NewMockPriorityTask(ctrl)
			task.EXPECT().Priority().Return(priority).AnyTimes()
			task.EXPECT().Nack(gomock.Any()).AnyTimes()
			err := pool.Submit(task)
			require.NoError(t, err)
		}
	}

	// Verify total count
	totalTasks := tasksPerPriority[0] + tasksPerPriority[1] + tasksPerPriority[2]
	assert.Equal(t, totalTasks, pool.Len())

	// Retrieve all tasks and track their priorities
	retrievedPriorities := []int{}
	for i := 0; i < totalTasks; i++ {
		task, ok := pool.GetNextTask()
		require.True(t, ok, "should get task %d of %d", i+1, totalTasks)
		require.NotNil(t, task)
		retrievedPriorities = append(retrievedPriorities, task.Priority())
	}

	// Verify the weighted round-robin pattern
	// Expected pattern repeats: [0, 0, 1, 0, 1, 2]
	// Cycle 1: [0, 0, 1, 0, 1, 2] - consumes 3 from p0, 2 from p1, 1 from p2
	// Cycle 2: [0, 0, 1, 0, 1, 2] - consumes 3 from p0, 2 from p1, 1 from p2
	expectedPattern := []int{
		0, 0, 1, 0, 1, 2, // First complete cycle
		0, 0, 1, 0, 1, 2, // Second complete cycle
	}

	assert.Equal(t, expectedPattern, retrievedPriorities,
		"retrieved tasks should follow weighted round-robin pattern [0,0,1,0,1,2] repeated twice")

	// Verify pool is now empty
	task, ok := pool.GetNextTask()
	assert.False(t, ok, "pool should be empty after retrieving all tasks")
	assert.Nil(t, task)
	assert.Equal(t, 0, pool.Len())
}

func TestWeightedRoundRobinTaskPool_WeightedOrderingAcrossMultipleCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := testlogger.New(t)
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
	timeSource := clock.NewRealTimeSource()

	// Setup with different weights: [5, 3, 1]
	pool := NewWeightedRoundRobinTaskPool[int](
		logger,
		metricsClient,
		timeSource,
		&WeightedRoundRobinTaskPoolOptions[int]{
			QueueSize: 100,
			TaskToChannelKeyFn: func(task PriorityTask) int {
				return task.Priority()
			},
			ChannelKeyToWeightFn: func(key int) int {
				weights := map[int]int{
					0: 5,
					1: 3,
					2: 1,
				}
				return weights[key]
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Submit enough tasks for one complete schedule cycle
	// Schedule length = 5 + 3 + 1 = 9
	// IWRR schedule: [0, 0, 1, 0, 1, 0, 1, 0, 2]
	tasksPerPriority := map[int]int{
		0: 5,
		1: 3,
		2: 1,
	}

	for priority, count := range tasksPerPriority {
		for i := 0; i < count; i++ {
			task := NewMockPriorityTask(ctrl)
			task.EXPECT().Priority().Return(priority).AnyTimes()
			task.EXPECT().Nack(gomock.Any()).AnyTimes()
			err := pool.Submit(task)
			require.NoError(t, err)
		}
	}

	// Retrieve tasks in small batches to verify state is maintained across calls
	retrievedPriorities := []int{}

	// Batch 1: Get 3 tasks
	for i := 0; i < 3; i++ {
		task, ok := pool.GetNextTask()
		require.True(t, ok)
		retrievedPriorities = append(retrievedPriorities, task.Priority())
	}

	// Batch 2: Get 4 tasks
	for i := 0; i < 4; i++ {
		task, ok := pool.GetNextTask()
		require.True(t, ok)
		retrievedPriorities = append(retrievedPriorities, task.Priority())
	}

	// Batch 3: Get remaining 2 tasks
	for i := 0; i < 2; i++ {
		task, ok := pool.GetNextTask()
		require.True(t, ok)
		retrievedPriorities = append(retrievedPriorities, task.Priority())
	}

	// Expected IWRR pattern for weights [5, 3, 1]
	// The interleaved weighted round-robin algorithm produces:
	// Round 4: [0] (only weight 5 > 4)
	// Round 3: [0] (only weight 5 > 3)
	// Round 2: [0, 1] (weights 5,3 > 2)
	// Round 1: [0, 1] (weights 5,3 > 1)
	// Round 0: [0, 1, 2] (weights 5,3,1 > 0)
	// Result: [0, 0, 0, 1, 0, 1, 0, 1, 2]
	expectedPattern := []int{0, 0, 0, 1, 0, 1, 0, 1, 2}

	assert.Equal(t, expectedPattern, retrievedPriorities,
		"should maintain WRR order across multiple GetNextTask calls")

	// Verify empty
	assert.Equal(t, 0, pool.Len())
}
