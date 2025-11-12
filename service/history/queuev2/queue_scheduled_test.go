package queuev2

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	historyconfig "github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

func setupBasicShardMocks(mockShard *shard.MockContext, mockTimeSource clock.TimeSource, mockExecutionManager *persistence.MockExecutionManager) {
	mockShard.EXPECT().GetShardID().Return(1).AnyTimes()
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).AnyTimes()
	mockShard.EXPECT().GetTimeSource().Return(mockTimeSource).AnyTimes()
	mockShard.EXPECT().GetConfig().Return(&historyconfig.Config{
		TaskCriticalRetryCount: dynamicproperties.GetIntPropertyFn(3),
	}).AnyTimes()
	mockShard.EXPECT().GetQueueState(persistence.HistoryTaskCategoryTimer).Return(&types.QueueState{
		VirtualQueueStates: map[int64]*types.VirtualQueueState{
			0: {
				VirtualSliceStates: []*types.VirtualSliceState{
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{
								ScheduledTimeNano: mockTimeSource.Now().Add(-1 * time.Hour).UnixNano(),
							},
							ExclusiveMax: &types.TaskKey{
								ScheduledTimeNano: mockTimeSource.Now().UnixNano(),
							},
						},
					},
				},
			},
		},
		ExclusiveMaxReadLevel: &types.TaskKey{
			ScheduledTimeNano: mockTimeSource.Now().UnixNano(),
		},
	}, nil).AnyTimes()
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).AnyTimes()
	mockExecutionManager.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{}, nil).AnyTimes()
	mockExecutionManager.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(&persistence.RangeCompleteHistoryTaskResponse{}, nil).AnyTimes()
	mockShard.EXPECT().UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTimer, cluster.TestCurrentClusterName).Return(persistence.NewHistoryTaskKey(time.Now(), 10)).AnyTimes()
	mockShard.EXPECT().UpdateQueueState(persistence.HistoryTaskCategoryTimer, gomock.Any()).Return(nil).AnyTimes()
}

func TestScheduledQueue_LifeCycle(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockTaskProcessor := task.NewMockProcessor(ctrl)
	mockTaskExecutor := task.NewMockExecutor(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsClient := metrics.NoopClient
	mockMetricsScope := metrics.NoopScope
	mockTimeSource := clock.NewMockedTimeSource()
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)

	// Setup mock expectations
	mockShard.EXPECT().GetShardID().Return(1).AnyTimes()
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).AnyTimes()
	mockShard.EXPECT().GetTimeSource().Return(mockTimeSource).AnyTimes()
	mockShard.EXPECT().GetQueueState(persistence.HistoryTaskCategoryTimer).Return(&types.QueueState{
		VirtualQueueStates: map[int64]*types.VirtualQueueState{
			rootQueueID: {
				VirtualSliceStates: []*types.VirtualSliceState{
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{
								ScheduledTimeNano: mockTimeSource.Now().Add(-1 * time.Hour).UnixNano(),
							},
							ExclusiveMax: &types.TaskKey{
								ScheduledTimeNano: mockTimeSource.Now().UnixNano(),
							},
						},
					},
				},
			},
		},
		ExclusiveMaxReadLevel: &types.TaskKey{
			ScheduledTimeNano: mockTimeSource.Now().UnixNano(),
		},
	}, nil).AnyTimes()
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).AnyTimes()
	mockExecutionManager.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{}, nil).AnyTimes()
	mockExecutionManager.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(&persistence.RangeCompleteHistoryTaskResponse{}, nil).AnyTimes()
	mockShard.EXPECT().UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTimer, cluster.TestCurrentClusterName).Return(persistence.NewHistoryTaskKey(time.Now(), 10)).AnyTimes()
	mockShard.EXPECT().UpdateQueueState(persistence.HistoryTaskCategoryTimer, gomock.Any()).Return(nil).AnyTimes()

	options := &Options{
		DeleteBatchSize:                      dynamicproperties.GetIntPropertyFn(100),
		RedispatchInterval:                   dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		PageSize:                             dynamicproperties.GetIntPropertyFn(100),
		PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollInterval:                      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollIntervalJitterCoefficient:     dynamicproperties.GetFloatPropertyFn(0.1),
		UpdateAckInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		UpdateAckIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
		MaxPollRPS:                           dynamicproperties.GetIntPropertyFn(100),
		MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
		PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		VirtualSliceForceAppendInterval:      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		CriticalPendingTaskCount:             dynamicproperties.GetIntPropertyFn(90),
		EnablePendingTaskCountAlert:          func() bool { return true },
		MaxVirtualQueueCount:                 dynamicproperties.GetIntPropertyFn(2),
	}

	queue := NewScheduledQueue(
		mockShard,
		persistence.HistoryTaskCategoryTimer,
		mockTaskProcessor,
		mockTaskExecutor,
		mockLogger,
		mockMetricsClient,
		mockMetricsScope,
		options,
	).(*scheduledQueue)

	// Test Start
	queue.Start()
	assert.Equal(t, common.DaemonStatusStarted, atomic.LoadInt32(&queue.status))

	queue.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
		Tasks: []persistence.Task{
			&persistence.DecisionTimeoutTask{
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Time{},
				},
			},
		},
	})

	// Advance time to trigger poll
	mockTimeSource.Advance(options.MaxPollInterval() * 2)

	// Advance time to trigger update ack
	mockTimeSource.Advance(options.UpdateAckInterval() * 2)

	// Test Stop
	queue.Stop()
	assert.Equal(t, common.DaemonStatusStopped, atomic.LoadInt32(&queue.status))
}

func TestScheduledQueue_NotifyNewTask_EmptyTasks(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockTaskProcessor := task.NewMockProcessor(ctrl)
	mockTaskExecutor := task.NewMockExecutor(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsClient := metrics.NoopClient
	mockMetricsScope := &mocks.Scope{}
	mockTimeSource := clock.NewMockedTimeSource()
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)

	setupBasicShardMocks(mockShard, mockTimeSource, mockExecutionManager)

	options := &Options{
		DeleteBatchSize:                      dynamicproperties.GetIntPropertyFn(100),
		RedispatchInterval:                   dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		PageSize:                             dynamicproperties.GetIntPropertyFn(100),
		PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollInterval:                      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollIntervalJitterCoefficient:     dynamicproperties.GetFloatPropertyFn(0.1),
		UpdateAckInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		UpdateAckIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
		MaxPollRPS:                           dynamicproperties.GetIntPropertyFn(100),
		MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
		PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		VirtualSliceForceAppendInterval:      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		CriticalPendingTaskCount:             dynamicproperties.GetIntPropertyFn(90),
		EnablePendingTaskCountAlert:          func() bool { return true },
		MaxVirtualQueueCount:                 dynamicproperties.GetIntPropertyFn(2),
	}

	queue := NewScheduledQueue(
		mockShard,
		persistence.HistoryTaskCategoryTimer,
		mockTaskProcessor,
		mockTaskExecutor,
		mockLogger,
		mockMetricsClient,
		mockMetricsScope,
		options,
	).(*scheduledQueue)

	assert.NotPanics(t, func() {
		queue.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
			Tasks: []persistence.Task{},
		})
	})
}

func TestScheduledQueue_NotifyNewTask_FieldCombinations(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		name             string
		scheduleInMemory bool
		persistenceError bool
		expectInMemory   bool
		expectDBRead     bool
	}{
		{
			name:             "ScheduleInMemory=true, PersistenceError=false - should try in-memory",
			scheduleInMemory: true,
			persistenceError: false,
			expectInMemory:   true,
			expectDBRead:     false,
		},
		{
			name:             "ScheduleInMemory=true, PersistenceError=true - should skip in-memory",
			scheduleInMemory: true,
			persistenceError: true,
			expectInMemory:   false,
			expectDBRead:     true,
		},
		{
			name:             "ScheduleInMemory=false, PersistenceError=false - should skip in-memory",
			scheduleInMemory: false,
			persistenceError: false,
			expectInMemory:   false,
			expectDBRead:     true,
		},
		{
			name:             "ScheduleInMemory=false, PersistenceError=true - should skip in-memory",
			scheduleInMemory: false,
			persistenceError: true,
			expectInMemory:   false,
			expectDBRead:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockShard := shard.NewMockContext(ctrl)
			mockTaskProcessor := task.NewMockProcessor(ctrl)
			mockTaskExecutor := task.NewMockExecutor(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsClient := metrics.NoopClient
			mockMetricsScope := &mocks.Scope{}
			mockTimeSource := clock.NewMockedTimeSource()
			mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
			mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

			setupBasicShardMocks(mockShard, mockTimeSource, mockExecutionManager)

			mockMetricsScope.On("AddCounter", metrics.NewHistoryTaskCounter, int64(1))

			options := &Options{
				DeleteBatchSize:                      dynamicproperties.GetIntPropertyFn(100),
				RedispatchInterval:                   dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				PageSize:                             dynamicproperties.GetIntPropertyFn(100),
				PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollInterval:                      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollIntervalJitterCoefficient:     dynamicproperties.GetFloatPropertyFn(0.1),
				UpdateAckInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				UpdateAckIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
				MaxPollRPS:                           dynamicproperties.GetIntPropertyFn(100),
				MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
				PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
				VirtualSliceForceAppendInterval:      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				CriticalPendingTaskCount:             dynamicproperties.GetIntPropertyFn(90),
				EnablePendingTaskCountAlert:          func() bool { return true },
				MaxVirtualQueueCount:                 dynamicproperties.GetIntPropertyFn(2),
			}

			queue := NewScheduledQueue(
				mockShard,
				persistence.HistoryTaskCategoryTimer,
				mockTaskProcessor,
				mockTaskExecutor,
				mockLogger,
				mockMetricsClient,
				mockMetricsScope,
				options,
			).(*scheduledQueue)

			queue.base.virtualQueueManager = mockVirtualQueueManager

			testTask := &persistence.DecisionTimeoutTask{
				TaskData: persistence.TaskData{
					VisibilityTimestamp: mockTimeSource.Now().Add(time.Hour),
				},
			}

			if tt.expectInMemory {
				mockVirtualQueueManager.EXPECT().InsertSingleTask(gomock.Any()).Return(true)
			}
			if tt.expectDBRead {
				mockVirtualQueueManager.EXPECT().ResetProgress(gomock.Any())
			}

			queue.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
				Tasks:            []persistence.Task{testTask},
				ScheduleInMemory: tt.scheduleInMemory,
				PersistenceError: tt.persistenceError,
			})
		})
	}
}

func TestScheduledQueue_NotifyNewTask_InsertionScenarios(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		name                      string
		numTasks                  int
		insertionResults          []bool
		expectedTasksToReadFromDB int
	}{
		{
			name:                      "All tasks inserted successfully",
			numTasks:                  3,
			insertionResults:          []bool{true, true, true},
			expectedTasksToReadFromDB: 0,
		},
		{
			name:                      "Some tasks fail insertion",
			numTasks:                  3,
			insertionResults:          []bool{true, false, true},
			expectedTasksToReadFromDB: 1,
		},
		{
			name:                      "All tasks fail insertion",
			numTasks:                  3,
			insertionResults:          []bool{false, false, false},
			expectedTasksToReadFromDB: 3,
		},
		{
			name:                      "Single task success",
			numTasks:                  1,
			insertionResults:          []bool{true},
			expectedTasksToReadFromDB: 0,
		},
		{
			name:                      "Single task failure",
			numTasks:                  1,
			insertionResults:          []bool{false},
			expectedTasksToReadFromDB: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockShard := shard.NewMockContext(ctrl)
			mockTaskProcessor := task.NewMockProcessor(ctrl)
			mockTaskExecutor := task.NewMockExecutor(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsClient := metrics.NoopClient
			mockMetricsScope := &mocks.Scope{}
			mockTimeSource := clock.NewMockedTimeSource()
			mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
			mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

			setupBasicShardMocks(mockShard, mockTimeSource, mockExecutionManager)

			mockMetricsScope.On("AddCounter", metrics.NewHistoryTaskCounter, int64(tt.numTasks))

			options := &Options{
				DeleteBatchSize:                      dynamicproperties.GetIntPropertyFn(100),
				RedispatchInterval:                   dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				PageSize:                             dynamicproperties.GetIntPropertyFn(100),
				PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollInterval:                      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollIntervalJitterCoefficient:     dynamicproperties.GetFloatPropertyFn(0.1),
				UpdateAckInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				UpdateAckIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
				MaxPollRPS:                           dynamicproperties.GetIntPropertyFn(100),
				MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
				PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
				VirtualSliceForceAppendInterval:      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				CriticalPendingTaskCount:             dynamicproperties.GetIntPropertyFn(90),
				EnablePendingTaskCountAlert:          func() bool { return true },
				MaxVirtualQueueCount:                 dynamicproperties.GetIntPropertyFn(2),
			}

			queue := NewScheduledQueue(
				mockShard,
				persistence.HistoryTaskCategoryTimer,
				mockTaskProcessor,
				mockTaskExecutor,
				mockLogger,
				mockMetricsClient,
				mockMetricsScope,
				options,
			).(*scheduledQueue)

			queue.base.virtualQueueManager = mockVirtualQueueManager

			tasks := make([]persistence.Task, tt.numTasks)
			for i := 0; i < tt.numTasks; i++ {
				tasks[i] = &persistence.DecisionTimeoutTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: mockTimeSource.Now().Add(time.Duration(i+1) * time.Hour),
					},
				}
			}

			for _, result := range tt.insertionResults {
				mockVirtualQueueManager.EXPECT().InsertSingleTask(gomock.Any()).Return(result).Times(1)
			}

			if tt.expectedTasksToReadFromDB > 0 {
				mockVirtualQueueManager.EXPECT().ResetProgress(gomock.Any()).Times(1)
			}

			queue.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
				Tasks:            tasks,
				ScheduleInMemory: true,
				PersistenceError: false,
			})
		})
	}
}

func TestScheduledQueue_NotifyNewTask_TimestampCalculation(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		name              string
		taskTimestamps    []time.Time
		expectedEarliest  time.Time
		allInsertionsFail bool
	}{
		{
			name: "Multiple tasks with different timestamps",
			taskTimestamps: []time.Time{
				time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
				time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC),
			},
			expectedEarliest:  time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
			allInsertionsFail: true,
		},
		{
			name: "Tasks with same timestamps",
			taskTimestamps: []time.Time{
				time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			expectedEarliest:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			allInsertionsFail: true,
		},
		{
			name: "Single task",
			taskTimestamps: []time.Time{
				time.Date(2023, 1, 1, 15, 30, 0, 0, time.UTC),
			},
			expectedEarliest:  time.Date(2023, 1, 1, 15, 30, 0, 0, time.UTC),
			allInsertionsFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockShard := shard.NewMockContext(ctrl)
			mockTaskProcessor := task.NewMockProcessor(ctrl)
			mockTaskExecutor := task.NewMockExecutor(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsClient := metrics.NoopClient
			mockMetricsScope := &mocks.Scope{}
			mockTimeSource := clock.NewMockedTimeSource()
			mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
			mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

			setupBasicShardMocks(mockShard, mockTimeSource, mockExecutionManager)

			mockMetricsScope.On("AddCounter", metrics.NewHistoryTaskCounter, int64(len(tt.taskTimestamps)))

			options := &Options{
				DeleteBatchSize:                      dynamicproperties.GetIntPropertyFn(100),
				RedispatchInterval:                   dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				PageSize:                             dynamicproperties.GetIntPropertyFn(100),
				PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollInterval:                      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollIntervalJitterCoefficient:     dynamicproperties.GetFloatPropertyFn(0.1),
				UpdateAckInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				UpdateAckIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
				MaxPollRPS:                           dynamicproperties.GetIntPropertyFn(100),
				MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
				PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
				VirtualSliceForceAppendInterval:      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				CriticalPendingTaskCount:             dynamicproperties.GetIntPropertyFn(90),
				EnablePendingTaskCountAlert:          func() bool { return true },
				MaxVirtualQueueCount:                 dynamicproperties.GetIntPropertyFn(2),
			}

			queue := NewScheduledQueue(
				mockShard,
				persistence.HistoryTaskCategoryTimer,
				mockTaskProcessor,
				mockTaskExecutor,
				mockLogger,
				mockMetricsClient,
				mockMetricsScope,
				options,
			).(*scheduledQueue)

			queue.base.virtualQueueManager = mockVirtualQueueManager

			tasks := make([]persistence.Task, len(tt.taskTimestamps))
			for i, ts := range tt.taskTimestamps {
				tasks[i] = &persistence.DecisionTimeoutTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: ts,
					},
				}
			}

			if tt.allInsertionsFail {
				mockVirtualQueueManager.EXPECT().InsertSingleTask(gomock.Any()).Return(false).Times(len(tasks))
				expectedKey := persistence.NewHistoryTaskKey(tt.expectedEarliest, 0)
				mockVirtualQueueManager.EXPECT().ResetProgress(expectedKey).Times(1)
			}

			queue.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
				Tasks:            tasks,
				ScheduleInMemory: true,
				PersistenceError: false,
			})
		})
	}
}

func TestScheduledQueue_NotifyNewTask_MultipleTaskTypes(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockTaskProcessor := task.NewMockProcessor(ctrl)
	mockTaskExecutor := task.NewMockExecutor(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsClient := metrics.NoopClient
	mockMetricsScope := &mocks.Scope{}
	mockTimeSource := clock.NewMockedTimeSource()
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
	mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

	setupBasicShardMocks(mockShard, mockTimeSource, mockExecutionManager)

	mockMetricsScope.On("AddCounter", metrics.NewHistoryTaskCounter, int64(4))

	options := &Options{
		DeleteBatchSize:                      dynamicproperties.GetIntPropertyFn(100),
		RedispatchInterval:                   dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		PageSize:                             dynamicproperties.GetIntPropertyFn(100),
		PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollInterval:                      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollIntervalJitterCoefficient:     dynamicproperties.GetFloatPropertyFn(0.1),
		UpdateAckInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		UpdateAckIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
		MaxPollRPS:                           dynamicproperties.GetIntPropertyFn(100),
		MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
		PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		VirtualSliceForceAppendInterval:      dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		CriticalPendingTaskCount:             dynamicproperties.GetIntPropertyFn(90),
		EnablePendingTaskCountAlert:          func() bool { return true },
		MaxVirtualQueueCount:                 dynamicproperties.GetIntPropertyFn(2),
	}

	queue := NewScheduledQueue(
		mockShard,
		persistence.HistoryTaskCategoryTimer,
		mockTaskProcessor,
		mockTaskExecutor,
		mockLogger,
		mockMetricsClient,
		mockMetricsScope,
		options,
	).(*scheduledQueue)

	queue.base.virtualQueueManager = mockVirtualQueueManager

	baseTime := mockTimeSource.Now()

	tasks := []persistence.Task{
		&persistence.DecisionTimeoutTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: baseTime.Add(1 * time.Hour),
			},
		},
		&persistence.ActivityTimeoutTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: baseTime.Add(30 * time.Minute), // Earliest
			},
		},
		&persistence.WorkflowTimeoutTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: baseTime.Add(2 * time.Hour),
			},
		},
		&persistence.UserTimerTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: baseTime.Add(45 * time.Minute),
			},
		},
	}

	mockVirtualQueueManager.EXPECT().InsertSingleTask(gomock.Any()).Return(false).Times(4)

	expectedKey := persistence.NewHistoryTaskKey(baseTime.Add(30*time.Minute), 0)
	mockVirtualQueueManager.EXPECT().ResetProgress(expectedKey).Times(1)

	queue.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
		Tasks:            tasks,
		ScheduleInMemory: true,
		PersistenceError: false,
	})
}

func TestScheduledQueue_LookAheadTask(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller, *MockQueueReader, clock.TimeSource, *clock.MockTimerGate)
		expectedUpdate time.Time
	}{
		{
			name: "successful look ahead with future task",
			setupMocks: func(ctrl *gomock.Controller, mockReader *MockQueueReader, mockTimeSource clock.TimeSource, mockTimerGate *clock.MockTimerGate) {
				now := mockTimeSource.Now()
				futureTime := now.Add(time.Hour)
				mockReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
							ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Second*10), 0),
						},
						NextTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Second*10), 0),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  1,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{
						&persistence.DecisionTimeoutTask{
							TaskData: persistence.TaskData{
								VisibilityTimestamp: futureTime,
							},
						},
					},
				}, nil)
				mockTimerGate.EXPECT().Update(futureTime)
			},
		},
		{
			name: "no tasks found",
			setupMocks: func(ctrl *gomock.Controller, mockReader *MockQueueReader, mockTimeSource clock.TimeSource, mockTimerGate *clock.MockTimerGate) {
				now := mockTimeSource.Now()
				mockReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
							ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Second*10), 0),
						},
						NextTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Second*10), 0),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  1,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{},
				}, nil)
				mockTimerGate.EXPECT().Update(now.Add(time.Second * 10))
			},
		},
		{
			name: "error during look ahead",
			setupMocks: func(ctrl *gomock.Controller, mockReader *MockQueueReader, mockTimeSource clock.TimeSource, mockTimerGate *clock.MockTimerGate) {
				now := mockTimeSource.Now()
				mockReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
							ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Second*10), 0),
						},
						NextTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Second*10), 0),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  1,
				}).Return(nil, errors.New("test error"))
				mockTimerGate.EXPECT().Update(now)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockShard := shard.NewMockContext(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsScope := metrics.NoopScope
			mockTimeSource := clock.NewMockedTimeSource()
			mockQueueReader := NewMockQueueReader(ctrl)
			mockTimerGate := clock.NewMockTimerGate(ctrl)

			// Setup mock expectations
			mockShard.EXPECT().GetTimeSource().Return(mockTimeSource).AnyTimes()
			mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).AnyTimes()

			options := &Options{
				DeleteBatchSize:                    dynamicproperties.GetIntPropertyFn(100),
				RedispatchInterval:                 dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				PageSize:                           dynamicproperties.GetIntPropertyFn(100),
				PollBackoffInterval:                dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				MaxPollIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.0),
				UpdateAckInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
				UpdateAckIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.1),
			}

			now := mockTimeSource.Now()
			// Create scheduled queue directly
			queue := &scheduledQueue{
				base: &queueBase{
					logger:       mockLogger,
					metricsScope: mockMetricsScope,
					category:     persistence.HistoryTaskCategoryTimer,
					options:      options,
					queueReader:  mockQueueReader,
					newVirtualSliceState: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
							ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Hour), 0),
						},
						Predicate: NewUniversalPredicate(),
					},
				},
				timerGate:  mockTimerGate,
				newTimerCh: make(chan struct{}, 1),
			}

			// Setup test-specific mocks and get expected update time
			tt.setupMocks(ctrl, mockQueueReader, mockTimeSource, mockTimerGate)

			// Execute lookAheadTask
			queue.lookAheadTask()
		})
	}
}
