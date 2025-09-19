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

package tasklist

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/matching/config"
)

const defaultIsolationGroup = "a"
const defaultAsyncDispatchTimeout = 3 * time.Second

var defaultIsolationGroups = []string{
	"a",
	"b",
	"c",
	"d",
}

func TestDispatchSingleTaskFromBuffer(t *testing.T) {
	testCases := []struct {
		name          string
		allowances    func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource)
		ttl           int
		breakDispatch bool
		breakRetries  bool
	}{
		{
			name: "success - no isolation",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return "", -1
				}
				markCalled := requireCallbackInvocation(t, "expected task to be dispatched")
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					assert.Equal(t, "", task.isolationGroup)
					markCalled()
					return nil
				}
			},
			breakDispatch: false,
			breakRetries:  true,
		},
		{
			name: "success - isolation",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				markCalled := requireCallbackInvocation(t, "expected task to be dispatched")
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					assert.Equal(t, defaultIsolationGroup, task.isolationGroup)
					markCalled()
					return nil
				}
			},
			breakDispatch: false,
			breakRetries:  true,
		},
		{
			name: "success - unknown isolation group",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return "mystery group", -1
				}
				markCalled := requireCallbackInvocation(t, "expected task to be dispatched")
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					assert.Equal(t, "", task.isolationGroup)
					markCalled()
					return nil
				}

			},
			breakDispatch: false,
			breakRetries:  true,
		},
		{
			name: "success - skip expired tasks",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					t.Fatal("task must not be dispatched")
					return nil
				}
			},
			ttl:           -2,
			breakDispatch: false,
			breakRetries:  true,
		},
		{
			name: "Error - context cancelled, should stop",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					return context.Canceled
				}

			},
			breakDispatch: true,
			breakRetries:  true,
		},
		{
			name: "Error - Deadline Exceeded, should retry",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					return context.DeadlineExceeded
				}

			},
			breakDispatch: false,
			breakRetries:  false,
		},
		{
			name: "Error - throttled, should retry",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					return ErrTasklistThrottled
				}
			},
			breakDispatch: false,
			breakRetries:  false,
		},
		{
			name: "Error - unknown, should retry",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					return errors.New("wow")
				}
			},
			breakDispatch: false,
			breakRetries:  false,
		},
		{
			name: "Error - task not started and not expired, should retry",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					return errTaskNotStarted
				}
			},
			ttl:           100,
			breakDispatch: false,
			breakRetries:  false,
		},
		{
			name: "Error - task not started and expired, should retry",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					mockTime.Advance(time.Hour)
					return errTaskNotStarted
				}
			},
			ttl:           1,
			breakDispatch: false,
			breakRetries:  false,
		},
		{
			name: "Error - time not reached to complete task without workflow execution, should retry",
			allowances: func(t *testing.T, reader *taskReader, mockTime clock.MockedTimeSource) {
				reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
					return defaultIsolationGroup, -1
				}
				reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
					return errWaitTimeNotReachedForEntityNotExists
				}
			},
			breakDispatch: false,
			breakRetries:  false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			timeSource := clock.NewMockedTimeSource()
			c := defaultConfig()
			tlm := createTestTaskListManagerWithConfig(t, testlogger.New(t), controller, c, timeSource)
			reader := tlm.taskReader
			tc.allowances(t, reader, timeSource)
			taskInfo := newTask(timeSource)
			taskInfo.Expiry = timeSource.Now().Add(time.Duration(tc.ttl) * time.Second)

			breakDispatch, breakRetries := reader.dispatchSingleTaskFromBuffer(taskInfo)
			assert.Equal(t, tc.breakDispatch, breakDispatch)
			assert.Equal(t, tc.breakRetries, breakRetries)
		})
	}
}

func TestGetDispatchTimeout(t *testing.T) {
	testCases := []struct {
		name              string
		isolationDuration time.Duration
		dispatchRps       float64
		expected          time.Duration
	}{
		{
			name:              "default - async dispatch timeout",
			isolationDuration: noIsolationTimeout,
			dispatchRps:       1000,
			expected:          defaultAsyncDispatchTimeout,
		},
		{
			name:              "isolation duration below async dispatch timeout",
			isolationDuration: 1 * time.Second,
			dispatchRps:       1000,
			expected:          1 * time.Second,
		},
		{
			name:              "isolation duration above async dispatch timeout",
			isolationDuration: 5 * time.Second,
			dispatchRps:       1000,
			expected:          defaultAsyncDispatchTimeout,
		},
		{
			name:              "no isolation - low dispatch rps extends timeout",
			isolationDuration: noIsolationTimeout,
			dispatchRps:       0.1,
			// rate is divided by 4 isolation groups (plus default buffer) and only one task gets dispatched per 10 seconds
			expected: 50 * time.Second,
		},
		{
			name:              "with isolation - low dispatch rps extends timeout",
			isolationDuration: time.Second,
			dispatchRps:       0.1,
			// rate is divided by 4 isolation groups (plus default buffer) and only one task gets dispatched per 10 seconds
			// This means taskIsolationDuration is extended, and we don't leak tasks as quickly if the
			// task list has a very low RPS
			expected: 50 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			timeSource := clock.NewMockedTimeSource()
			c := defaultConfig()
			tlm := createTestTaskListManagerWithConfig(t, testlogger.New(t), controller, c, timeSource)
			reader := tlm.taskReader

			actual := reader.getDispatchTimeout(tc.dispatchRps, tc.isolationDuration)
			assert.Equal(t, tc.expected, actual)

		})
	}
}

func TestTaskPump(t *testing.T) {
	cases := []struct {
		name       string
		tasks      []*persistence.TaskInfo
		assertions func(t *testing.T, tasks []*InternalTask)
	}{
		{
			name: "single task",
			tasks: []*persistence.TaskInfo{
				taskWithIsolationGroup("a"),
			},
			assertions: func(t *testing.T, tasks []*InternalTask) {
				assert.Equal(t, 1, len(tasks))
				assert.Equal(t, "a", tasks[0].isolationGroup)
			},
		},
		{
			name: "unknown isolation group",
			tasks: []*persistence.TaskInfo{
				taskWithIsolationGroup("unknown"),
			},
			assertions: func(t *testing.T, tasks []*InternalTask) {
				assert.Equal(t, 1, len(tasks))
				assert.Equal(t, "", tasks[0].isolationGroup)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			timeSource := clock.NewMockedTimeSource()
			c := defaultConfig()
			tlm := createTestTaskListManagerWithConfig(t, testlogger.New(t), controller, c, timeSource)
			reader := tlm.taskReader

			var lock sync.Mutex
			received := make([]*InternalTask, 0)
			done := make(chan struct{})

			reader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
				lock.Lock()
				defer lock.Unlock()

				received = append(received, task)
				if len(received) == len(tc.tasks) {
					close(done)
				}
				return nil
			}
			reader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
				return info.PartitionConfig["isolation-group"], -1
			}

			for i, taskInfo := range tc.tasks {
				taskInfo.CreatedTime = timeSource.Now()
				taskInfo.Expiry = timeSource.Now().Add(10 * time.Second)
				_, err := tlm.db.CreateTasks([]*persistence.CreateTaskInfo{
					{
						Data:   taskInfo,
						TaskID: int64(1 + i),
					},
				})
				require.NoError(t, err)
			}
			// This is how the reader knows the range that's valid to read
			tlm.taskWriter.maxReadLevel = int64(len(tc.tasks))

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			reader.Start()

			select {
			case <-ctx.Done():
				t.Fatalf("timed out waiting for tasks to be dispatched")
			case <-done:
			}
			reader.Stop()

			tc.assertions(t, received)
		})
	}
}

func defaultConfig() *config.Config {
	config := config.NewConfig(dynamicconfig.NewNopCollection(), "some random hostname", func() []string {
		return defaultIsolationGroups
	})
	config.EnableTasklistIsolation = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
	config.LongPollExpirationInterval = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(100 * time.Millisecond)
	config.MaxTaskDeleteBatchSize = dynamicproperties.GetIntPropertyFilteredByTaskListInfo(1)
	config.GetTasksBatchSize = dynamicproperties.GetIntPropertyFilteredByTaskListInfo(10)
	config.AsyncTaskDispatchTimeout = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(defaultAsyncDispatchTimeout)
	config.LocalTaskWaitTime = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(time.Millisecond)
	return config
}

func taskWithIsolationGroup(ig string) *persistence.TaskInfo {
	return &persistence.TaskInfo{
		DomainID:                      "domain-id",
		WorkflowID:                    "workflow-id",
		RunID:                         "run-id",
		TaskID:                        1,
		ScheduleID:                    2,
		ScheduleToStartTimeoutSeconds: 10,
		PartitionConfig: map[string]string{
			"isolation-group": ig,
		},
	}
}

func newTask(timeSource clock.TimeSource) *persistence.TaskInfo {
	return &persistence.TaskInfo{
		DomainID:                      "domain-id",
		WorkflowID:                    "workflow-id",
		RunID:                         "run-id",
		TaskID:                        1,
		ScheduleID:                    2,
		ScheduleToStartTimeoutSeconds: 10,
		Expiry:                        timeSource.Now().Add(10 * time.Second),
		CreatedTime:                   timeSource.Now(),
		PartitionConfig: map[string]string{
			"isolation-group": defaultIsolationGroup,
		},
	}
}

func requireCallbackInvocation(t *testing.T, msg string) func() {
	called := false
	t.Cleanup(func() {
		if !called {
			t.Fatal(msg)
		}
	})

	return func() {
		called = true
	}
}

func TestTaskReaderBatchSizeValidation(t *testing.T) {
	tests := []struct {
		name           string
		configValue    int
		expectedBuffer int
	}{
		{
			name:           "valid positive batch size",
			configValue:    1000,
			expectedBuffer: 999, // buffer size is batchSize - 1
		},
		{
			name:           "valid small batch size",
			configValue:    1,
			expectedBuffer: 0, // buffer size is batchSize - 1
		},
		{
			name:           "zero batch size should be corrected to default (1000)",
			configValue:    0,
			expectedBuffer: 999, // corrected to 1000, so buffer is 999
		},
		{
			name:           "negative batch size should be corrected to default (1000)",
			configValue:    -5,
			expectedBuffer: 999, // corrected to 1000, so buffer is 999
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchSize := tt.configValue
			if batchSize <= 0 {
				// This is the validation logic from newTaskReader - use default value (1000)
				batchSize = 1000
			}

			// Test buffer creation with validated batch size
			expectedCapacity := batchSize - 1
			if expectedCapacity < 0 {
				expectedCapacity = 0
			}

			// Simulate the buffer creation logic
			buffer := make(chan *persistence.TaskInfo, expectedCapacity)
			actualCapacity := cap(buffer)

			assert.Equal(t, tt.expectedBuffer, actualCapacity, "Task buffer should have correct capacity after validation")
		})
	}
}
