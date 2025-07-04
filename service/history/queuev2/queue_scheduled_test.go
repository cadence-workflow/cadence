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
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

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
