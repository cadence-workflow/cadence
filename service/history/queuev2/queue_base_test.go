package queuev2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

func TestQueueBase_ProcessNewTasks(t *testing.T) {
	tests := []struct {
		name                         string
		category                     persistence.HistoryTaskCategory
		initialVirtualSlice          VirtualSliceState
		expectedVirtualSlices        []VirtualSlice
		expectedNewVirtualSliceState VirtualSliceState
		expectError                  bool
		setupMocks                   func(*gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager)
	}{
		{
			name:     "Successfully process new tasks for immediate category",
			category: persistence.HistoryTaskCategoryTransfer,
			initialVirtualSlice: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedNewVirtualSliceState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(201),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectError: false,
			setupMocks: func(ctrl *gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager) {
				mockShard := shard.NewMockContext(ctrl)
				mockTaskProcessor := task.NewMockProcessor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).AnyTimes()
				mockShard.EXPECT().GetTimeSource().Return(mockTimeSource).AnyTimes()
				mockShard.EXPECT().UpdateIfNeededAndGetQueueMaxReadLevel(
					persistence.HistoryTaskCategoryTransfer,
					cluster.TestCurrentClusterName,
				).Return(persistence.NewImmediateTaskKey(200))
				mockVirtualQueueManager.EXPECT().AddNewVirtualSliceToRootQueue(gomock.Any()).DoAndReturn(func(s VirtualSlice) {
					assert.Equal(t, s.GetState().Range.InclusiveMinTaskKey, persistence.NewImmediateTaskKey(100))
					assert.Equal(t, s.GetState().Range.ExclusiveMaxTaskKey, persistence.NewImmediateTaskKey(201))
				})
				return mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager
			},
		},
		{
			name:     "Successfully process new tasks for scheduled category",
			category: persistence.HistoryTaskCategoryTimer,
			initialVirtualSlice: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(100, 0), 100),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedNewVirtualSliceState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(201, 0), 201),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectError: false,
			setupMocks: func(ctrl *gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager) {
				mockShard := shard.NewMockContext(ctrl)
				mockTaskProcessor := task.NewMockProcessor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).AnyTimes()
				mockShard.EXPECT().GetTimeSource().Return(mockTimeSource).AnyTimes()
				mockShard.EXPECT().UpdateIfNeededAndGetQueueMaxReadLevel(
					persistence.HistoryTaskCategoryTimer,
					cluster.TestCurrentClusterName,
				).Return(persistence.NewHistoryTaskKey(time.Unix(201, 0), 201))
				mockVirtualQueueManager.EXPECT().AddNewVirtualSliceToRootQueue(gomock.Any()).DoAndReturn(func(s VirtualSlice) {
					assert.Equal(t, s.GetState().Range.InclusiveMinTaskKey, persistence.NewHistoryTaskKey(time.Unix(100, 0), 100))
					assert.Equal(t, s.GetState().Range.ExclusiveMaxTaskKey, persistence.NewHistoryTaskKey(time.Unix(201, 0), 201))
				})

				return mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager
			},
		},
		{
			name:     "No new tasks to process",
			category: persistence.HistoryTaskCategoryTransfer,
			initialVirtualSlice: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedNewVirtualSliceState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectError: false,
			setupMocks: func(ctrl *gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager) {
				mockShard := shard.NewMockContext(ctrl)
				mockTaskProcessor := task.NewMockProcessor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).AnyTimes()
				mockShard.EXPECT().GetTimeSource().Return(mockTimeSource).AnyTimes()
				mockShard.EXPECT().UpdateIfNeededAndGetQueueMaxReadLevel(
					persistence.HistoryTaskCategoryTransfer,
					cluster.TestCurrentClusterName,
				).Return(persistence.NewImmediateTaskKey(99))

				return mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctrl := gomock.NewController(t)

			mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager := tt.setupMocks(ctrl)

			queueBase := &queueBase{
				shard:                mockShard,
				taskProcessor:        mockTaskProcessor,
				metricsClient:        metrics.NoopClient,
				metricsScope:         metrics.NoopScope,
				logger:               testlogger.New(t),
				category:             tt.category,
				timeSource:           mockTimeSource,
				virtualQueueManager:  mockVirtualQueueManager,
				newVirtualSliceState: tt.initialVirtualSlice,
			}

			// Execute
			queueBase.processNewTasks()

			// Verify
			if !tt.expectError {
				assert.Equal(t, queueBase.newVirtualSliceState, tt.expectedNewVirtualSliceState)
			}
		})
	}
}

func TestQueueBase_UpdateQueueState(t *testing.T) {
	tests := []struct {
		name                      string
		category                  persistence.HistoryTaskCategory
		initialExclusiveAckLevel  persistence.HistoryTaskKey
		initialVirtualSliceState  VirtualSliceState
		expectedExclusiveAckLevel persistence.HistoryTaskKey
		expectError               bool
		setupMocks                func(*gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager)
	}{
		{
			name:                     "Successfully update queue state with new ack level",
			category:                 persistence.HistoryTaskCategoryTransfer,
			initialExclusiveAckLevel: persistence.NewImmediateTaskKey(100),
			initialVirtualSliceState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1000),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedExclusiveAckLevel: persistence.NewImmediateTaskKey(200),
			expectError:               false,
			setupMocks: func(ctrl *gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager) {
				mockShard := shard.NewMockContext(ctrl)
				mockTaskProcessor := task.NewMockProcessor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
				mockExecutionManager := persistence.NewMockExecutionManager(ctrl)

				mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).AnyTimes()
				mockExecutionManager.EXPECT().RangeCompleteHistoryTask(gomock.Any(), &persistence.RangeCompleteHistoryTaskRequest{
					TaskCategory:        persistence.HistoryTaskCategoryTransfer,
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
					PageSize:            100,
				}).Return(&persistence.RangeCompleteHistoryTaskResponse{
					TasksCompleted: 10,
				}, nil)
				mockShard.EXPECT().UpdateQueueState(
					persistence.HistoryTaskCategoryTransfer,
					gomock.Any(),
				).Return(nil)

				mockVirtualQueueManager.EXPECT().UpdateAndGetState().Return(map[int64][]VirtualSliceState{
					1: {
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(200),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(300),
							},
							Predicate: NewUniversalPredicate(),
						},
					},
				})

				return mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager
			},
		},
		{
			name:                     "Failed to range complete history tasks",
			category:                 persistence.HistoryTaskCategoryTransfer,
			initialExclusiveAckLevel: persistence.NewImmediateTaskKey(100),
			initialVirtualSliceState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1000),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedExclusiveAckLevel: persistence.NewImmediateTaskKey(100),
			expectError:               true,
			setupMocks: func(ctrl *gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager) {
				mockShard := shard.NewMockContext(ctrl)
				mockTaskProcessor := task.NewMockProcessor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
				mockExecutionManager := persistence.NewMockExecutionManager(ctrl)

				mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).AnyTimes()
				mockExecutionManager.EXPECT().RangeCompleteHistoryTask(gomock.Any(), &persistence.RangeCompleteHistoryTaskRequest{
					TaskCategory:        persistence.HistoryTaskCategoryTransfer,
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
					PageSize:            100,
				}).Return(nil, assert.AnError)

				mockVirtualQueueManager.EXPECT().UpdateAndGetState().Return(map[int64][]VirtualSliceState{
					1: {
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(200),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(300),
							},
							Predicate: NewUniversalPredicate(),
						},
					},
				})

				return mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager
			},
		},
		{
			name:                     "Successfully update queue state with no new ack level",
			category:                 persistence.HistoryTaskCategoryTransfer,
			initialExclusiveAckLevel: persistence.NewImmediateTaskKey(100),
			initialVirtualSliceState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1000),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedExclusiveAckLevel: persistence.NewImmediateTaskKey(100),
			expectError:               false,
			setupMocks: func(ctrl *gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager) {
				mockShard := shard.NewMockContext(ctrl)
				mockTaskProcessor := task.NewMockProcessor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

				mockShard.EXPECT().UpdateQueueState(
					persistence.HistoryTaskCategoryTransfer,
					gomock.Any(),
				).Return(nil)

				mockVirtualQueueManager.EXPECT().UpdateAndGetState().Return(map[int64][]VirtualSliceState{
					1: {
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(300),
							},
							Predicate: NewUniversalPredicate(),
						},
					},
				})

				return mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager
			},
		},
		{
			name:                     "Failed to update queue state",
			category:                 persistence.HistoryTaskCategoryTransfer,
			initialExclusiveAckLevel: persistence.NewImmediateTaskKey(300),
			initialVirtualSliceState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1000),
					ExclusiveMaxTaskKey: persistence.MaximumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedExclusiveAckLevel: persistence.NewImmediateTaskKey(200),
			expectError:               true,
			setupMocks: func(ctrl *gomock.Controller) (*shard.MockContext, *task.MockProcessor, clock.TimeSource, *MockVirtualQueueManager) {
				mockShard := shard.NewMockContext(ctrl)
				mockTaskProcessor := task.NewMockProcessor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)

				mockShard.EXPECT().UpdateQueueState(
					persistence.HistoryTaskCategoryTransfer,
					gomock.Any(),
				).Return(assert.AnError)

				mockVirtualQueueManager.EXPECT().UpdateAndGetState().Return(map[int64][]VirtualSliceState{
					1: {
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(200),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(300),
							},
							Predicate: NewUniversalPredicate(),
						},
					},
				})

				return mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctrl := gomock.NewController(t)

			mockShard, mockTaskProcessor, mockTimeSource, mockVirtualQueueManager := tt.setupMocks(ctrl)

			queueBase := &queueBase{
				shard:                 mockShard,
				taskProcessor:         mockTaskProcessor,
				metricsClient:         metrics.NoopClient,
				metricsScope:          metrics.NoopScope,
				logger:                testlogger.New(t),
				category:              tt.category,
				timeSource:            mockTimeSource,
				monitor:               NewMonitor(tt.category),
				virtualQueueManager:   mockVirtualQueueManager,
				exclusiveAckLevel:     tt.initialExclusiveAckLevel,
				newVirtualSliceState:  tt.initialVirtualSliceState,
				updateQueueStateTimer: mockTimeSource.NewTimer(time.Second * 10),
				options: &Options{
					DeleteBatchSize:                    dynamicproperties.GetIntPropertyFn(100),
					UpdateAckInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
					UpdateAckIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.1),
				},
			}

			// Execute
			queueBase.updateQueueState(context.Background())

			// Verify
			if !tt.expectError {
				assert.Equal(t, tt.expectedExclusiveAckLevel, queueBase.exclusiveAckLevel)
			}
		})
	}
}

func TestNewQueueBase(t *testing.T) {
	queueState := &types.QueueState{
		ExclusiveMaxReadLevel: &types.TaskKey{
			TaskID: 400,
		},
		VirtualQueueStates: map[int64]*types.VirtualQueueState{
			rootQueueID: {
				VirtualSliceStates: []*types.VirtualSliceState{
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{
								TaskID: 100,
							},
							ExclusiveMax: &types.TaskKey{
								TaskID: 200,
							},
						},
					},
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{
								TaskID: 200,
							},
							ExclusiveMax: &types.TaskKey{
								TaskID: 300,
							},
						},
					},
				},
			},
			1: {
				VirtualSliceStates: []*types.VirtualSliceState{
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{
								TaskID: 300,
							},
							ExclusiveMax: &types.TaskKey{
								TaskID: 400,
							},
						},
					},
				},
			},
		},
	}
	ctrl := gomock.NewController(t)
	mockShard := shard.NewMockContext(ctrl)
	mockTaskProcessor := task.NewMockProcessor(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()

	mockShard.EXPECT().GetQueueState(persistence.HistoryTaskCategoryTransfer).Return(queueState, nil)

	mockShard.EXPECT().GetTimeSource().Return(mockTimeSource)

	queueBase := newQueueBase(
		mockShard,
		mockTaskProcessor,
		testlogger.New(t),
		metrics.NoopClient,
		metrics.NoopScope,
		persistence.HistoryTaskCategoryTransfer,
		nil,
		&Options{
			DeleteBatchSize:    dynamicproperties.GetIntPropertyFn(100),
			RedispatchInterval: dynamicproperties.GetDurationPropertyFn(time.Second * 10),
			MaxPollRPS:         dynamicproperties.GetIntPropertyFn(100),
		},
	)

	assert.Equal(t, persistence.NewImmediateTaskKey(400), queueBase.newVirtualSliceState.Range.InclusiveMinTaskKey)
	assert.Equal(t, map[int64][]VirtualSliceState{
		rootQueueID: {
			{
				Range:     Range{InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100), ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200)},
				Predicate: NewUniversalPredicate(),
			},
			{
				Range:     Range{InclusiveMinTaskKey: persistence.NewImmediateTaskKey(200), ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(300)},
				Predicate: NewUniversalPredicate(),
			},
		},
		1: {
			{
				Range:     Range{InclusiveMinTaskKey: persistence.NewImmediateTaskKey(300), ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(400)},
				Predicate: NewUniversalPredicate(),
			},
		},
	}, queueBase.virtualQueueManager.GetState())
}
