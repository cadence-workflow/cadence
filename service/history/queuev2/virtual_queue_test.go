package queuev2

import (
	"container/list"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/service/history/task"
)

func TestVirtualQueueImpl_GetState(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRedispatcher := task.NewMockRedispatcher(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsScope := metrics.NoopScope
	mockPageSize := dynamicproperties.GetIntPropertyFn(10)

	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)
	mockVirtualSlice2 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
		mockVirtualSlice2,
	}

	mockVirtualSlice1.EXPECT().GetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
		},
		Predicate: NewUniversalPredicate(),
	})
	mockVirtualSlice2.EXPECT().GetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
		},
		Predicate: NewUniversalPredicate(),
	})

	mockTimeSource := clock.NewMockedTimeSource()
	mockRateLimiter := quotas.NewMockLimiter(ctrl)
	mockMonitor := NewMockMonitor(ctrl)

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockRateLimiter,
		mockMonitor,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize:                             mockPageSize,
			MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
			PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
			PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		},
	)

	states := queue.GetState()
	expectedStates := []VirtualSliceState{
		{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
			},
			Predicate: NewUniversalPredicate(),
		},
		{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
			},
			Predicate: NewUniversalPredicate(),
		},
	}
	assert.Equal(t, expectedStates, states)
}

func TestVirtualQueueImpl_UpdateAndGetState(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRedispatcher := task.NewMockRedispatcher(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsScope := metrics.NoopScope
	mockPageSize := dynamicproperties.GetIntPropertyFn(10)

	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)
	mockVirtualSlice2 := NewMockVirtualSlice(ctrl)
	mockMonitor := NewMockMonitor(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
		mockVirtualSlice2,
	}

	mockVirtualSlice1.EXPECT().UpdateAndGetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
		},
		Predicate: NewUniversalPredicate(),
	})
	mockVirtualSlice1.EXPECT().GetPendingTaskCount().Return(1)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice1, 1)

	mockVirtualSlice2.EXPECT().UpdateAndGetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(11),
		},
		Predicate: NewUniversalPredicate(),
	})
	mockMonitor.EXPECT().RemoveSlice(mockVirtualSlice2)

	mockTimeSource := clock.NewMockedTimeSource()
	mockRateLimiter := quotas.NewMockLimiter(ctrl)

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockRateLimiter,
		mockMonitor,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize:                             mockPageSize,
			MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
			PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
			PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		},
	)

	states := queue.UpdateAndGetState()
	expectedStates := []VirtualSliceState{
		{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
			},
			Predicate: NewUniversalPredicate(),
		},
	}
	assert.Equal(t, expectedStates, states)
}

func TestVirtualQueue_MergeSlices(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor)
		expectedStates []VirtualSliceState
	}{
		{
			name: "Merge non-overlapping slices",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				existingSlice2 := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().HasMoreTasks().Return(true)
				existingSlice.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{}, false)
				existingSlice.EXPECT().GetPendingTaskCount().Return(1)
				monitor.EXPECT().SetSlicePendingTaskCount(existingSlice, 1)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().TryMergeWithVirtualSlice(existingSlice2).Return([]VirtualSlice{}, false)
				incomingSlice.EXPECT().GetPendingTaskCount().Return(2)
				monitor.EXPECT().SetSlicePendingTaskCount(incomingSlice, 2)

				existingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice2.EXPECT().GetPendingTaskCount().Return(3)
				monitor.EXPECT().SetSlicePendingTaskCount(existingSlice2, 3)

				return []VirtualSlice{existingSlice, existingSlice2}, []VirtualSlice{incomingSlice}, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
		},
		{
			name: "Merge overlapping slices",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				existingSlice2 := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice := NewMockVirtualSlice(ctrl)
				mergedSlice2 := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().TryMergeWithVirtualSlice(gomock.Any()).Return([]VirtualSlice{mergedSlice}, true)
				existingSlice.EXPECT().GetPendingTaskCount().Return(1)
				monitor.EXPECT().SetSlicePendingTaskCount(existingSlice, 1)
				mergedSlice.EXPECT().GetPendingTaskCount().Return(1)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice, 1)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mergedSlice.EXPECT().TryMergeWithVirtualSlice(existingSlice2).Return([]VirtualSlice{mergedSlice2}, true)
				mergedSlice2.EXPECT().GetPendingTaskCount().Return(2)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice2, 2)

				existingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mergedSlice2.EXPECT().HasMoreTasks().Return(true).AnyTimes()

				monitor.EXPECT().RemoveSlice(existingSlice)
				monitor.EXPECT().RemoveSlice(incomingSlice)
				monitor.EXPECT().RemoveSlice(existingSlice2)
				monitor.EXPECT().RemoveSlice(mergedSlice)

				return []VirtualSlice{existingSlice, existingSlice2}, []VirtualSlice{incomingSlice}, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
		},
		{
			name: "Merge empty queue with new slices",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().GetPendingTaskCount().Return(1)
				monitor.EXPECT().SetSlicePendingTaskCount(incomingSlice, 1)
				incomingSlice.EXPECT().HasMoreTasks().Return(true).AnyTimes()

				return []VirtualSlice{}, []VirtualSlice{incomingSlice}, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProcessor := task.NewMockProcessor(ctrl)
			mockRedispatcher := task.NewMockRedispatcher(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsScope := metrics.NoopScope
			mockTimeSource := clock.NewMockedTimeSource()
			mockRateLimiter := quotas.NewMockLimiter(ctrl)

			existingSlices, incomingSlices, monitor := tt.setupMocks(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRedispatcher,
				mockLogger,
				mockMetricsScope,
				mockTimeSource,
				mockRateLimiter,
				monitor,
				existingSlices,
				&VirtualQueueOptions{
					PageSize:                             dynamicproperties.GetIntPropertyFn(10),
					MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
					PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
					PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
				},
			)

			queue.MergeSlices(incomingSlices...)
			states := queue.GetState()

			assert.Equal(t, tt.expectedStates, states)
		})
	}
}

func TestAppendOrMergeSlice(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice, Monitor)
		expectedStates []VirtualSliceState
	}{
		{
			name: "Append when no merge possible",
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{}, false)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				incomingSlice.EXPECT().GetPendingTaskCount().Return(2)
				monitor.EXPECT().SetSlicePendingTaskCount(incomingSlice, 2)

				return existingSlice, incomingSlice, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
		},
		{
			name: "Merge when slices overlap",
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{mergedSlice}, true)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				monitor.EXPECT().RemoveSlice(existingSlice)
				monitor.EXPECT().RemoveSlice(incomingSlice)
				mergedSlice.EXPECT().GetPendingTaskCount().Return(1)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice, 1)

				return existingSlice, incomingSlice, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
		},
		{
			name: "Append to empty list",
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice, Monitor) {
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().GetPendingTaskCount().Return(10)
				monitor.EXPECT().SetSlicePendingTaskCount(incomingSlice, 10)

				return nil, incomingSlice, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
		},
		{
			name: "Merge with multiple resulting slices",
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice1 := NewMockVirtualSlice(ctrl)
				mergedSlice2 := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{mergedSlice1, mergedSlice2}, true)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				monitor.EXPECT().RemoveSlice(existingSlice)
				monitor.EXPECT().RemoveSlice(incomingSlice)
				mergedSlice1.EXPECT().GetPendingTaskCount().Return(1)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice1, 1)
				mergedSlice2.EXPECT().GetPendingTaskCount().Return(2)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice2, 2)

				return existingSlice, incomingSlice, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					Predicate: NewUniversalPredicate(),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			existingSlice, incomingSlice, monitor := tt.setupMocks(ctrl)
			slices := list.New()

			if existingSlice != nil {
				slices.PushBack(existingSlice)
			}

			virtualQueue := &virtualQueueImpl{
				virtualSlices: slices,
				monitor:       monitor,
			}

			virtualQueue.appendOrMergeSlice(slices, incomingSlice)

			// Convert list to slice of states for comparison
			var states []VirtualSliceState
			for e := slices.Front(); e != nil; e = e.Next() {
				states = append(states, e.Value.(VirtualSlice).GetState())
			}

			assert.Equal(t, tt.expectedStates, states)
		})
	}
}

func TestVirtualQueue_LoadAndSubmitTasks(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRedispatcher := task.NewMockRedispatcher(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsScope := metrics.NoopScope
	mockPageSize := dynamicproperties.GetIntPropertyFn(10)
	mockTimeSource := clock.NewMockedTimeSource()
	mockRateLimiter := quotas.NewMockLimiter(ctrl)
	mockRateLimiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	mockMonitor := NewMockMonitor(ctrl)
	mockPauseController := NewMockPauseController(ctrl)

	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)
	mockVirtualSlice2 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
		mockVirtualSlice2,
	}

	mockTask1 := task.NewMockTask(ctrl)
	mockTask1.EXPECT().GetDomainID().Return("some random domainID")
	mockTask1.EXPECT().GetWorkflowID().Return("some random workflowID")
	mockTask1.EXPECT().GetRunID().Return("some random runID")
	mockTask1.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(time.Second*-1), 1))
	mockTask2 := task.NewMockTask(ctrl)
	mockTask2.EXPECT().GetDomainID().Return("some random domainID")
	mockTask2.EXPECT().GetWorkflowID().Return("some random workflowID")
	mockTask2.EXPECT().GetRunID().Return("some random runID")
	mockTask2.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(time.Second*1), 2))
	mockTask3 := task.NewMockTask(ctrl)
	mockTask3.EXPECT().GetDomainID().Return("some random domainID")
	mockTask3.EXPECT().GetWorkflowID().Return("some random workflowID")
	mockTask3.EXPECT().GetRunID().Return("some random runID")
	mockTask3.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(time.Second*-1), 1))

	mockMonitor.EXPECT().GetTotalPendingTaskCount().Return(0)
	mockPauseController.EXPECT().IsPaused().Return(false)
	mockVirtualSlice1.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{mockTask1, mockTask2}, nil)
	mockVirtualSlice1.EXPECT().GetPendingTaskCount().Return(2)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice1, 2)
	mockVirtualSlice1.EXPECT().HasMoreTasks().Return(false)

	mockMonitor.EXPECT().GetTotalPendingTaskCount().Return(0)
	mockPauseController.EXPECT().IsPaused().Return(false)
	mockVirtualSlice2.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{mockTask3}, nil)
	mockVirtualSlice2.EXPECT().HasMoreTasks().Return(false)
	mockVirtualSlice2.EXPECT().GetPendingTaskCount().Return(1)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice2, 1)
	mockProcessor.EXPECT().TrySubmit(mockTask3).Return(false, nil)

	mockProcessor.EXPECT().TrySubmit(mockTask1).Return(true, nil)
	mockRedispatcher.EXPECT().RedispatchTask(mockTask2, mockTimeSource.Now().Add(time.Second*1))
	mockRedispatcher.EXPECT().AddTask(mockTask3)

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockRateLimiter,
		mockMonitor,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize:                             mockPageSize,
			MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
			PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
			PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		},
	).(*virtualQueueImpl)

	queue.pauseController = mockPauseController

	queue.loadAndSubmitTasks()

	queue.loadAndSubmitTasks()

	assert.Nil(t, queue.sliceToRead)
}

func TestVirtualQueue_LifeCycle(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRedispatcher := task.NewMockRedispatcher(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsScope := metrics.NoopScope
	mockPageSize := dynamicproperties.GetIntPropertyFn(10)
	mockTimeSource := clock.NewMockedTimeSource()
	mockRateLimiter := quotas.NewMockLimiter(ctrl)
	mockRateLimiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	mockMonitor := NewMockMonitor(ctrl)
	mockPauseController := NewMockPauseController(ctrl)
	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
	}

	mockVirtualSlice1.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{}, nil).MaxTimes(1)
	mockVirtualSlice1.EXPECT().HasMoreTasks().Return(false).MaxTimes(1)
	mockVirtualSlice1.EXPECT().GetPendingTaskCount().Return(0).MaxTimes(1)
	mockVirtualSlice1.EXPECT().Clear().Times(1)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice1, 0).MaxTimes(1)
	mockMonitor.EXPECT().GetTotalPendingTaskCount().Return(0).MaxTimes(1)

	mockPauseController.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	mockPauseController.EXPECT().IsPaused().Return(false).AnyTimes()
	mockPauseController.EXPECT().Unsubscribe(gomock.Any()).Times(1)
	mockPauseController.EXPECT().Stop().Times(1)

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockRateLimiter,
		mockMonitor,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize:                             mockPageSize,
			MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
			PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
			PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		},
	).(*virtualQueueImpl)

	queue.pauseController = mockPauseController

	queue.Start()
	queue.Stop()
}

func TestVirtualQueue_LifeCycle_Pause(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRedispatcher := task.NewMockRedispatcher(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsScope := metrics.NoopScope
	mockPageSize := dynamicproperties.GetIntPropertyFn(10)
	mockTimeSource := clock.NewMockedTimeSource()
	mockRateLimiter := quotas.NewMockLimiter(ctrl)
	mockRateLimiter.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	mockMonitor := NewMockMonitor(ctrl)
	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
	}

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockRateLimiter,
		mockMonitor,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize:                             mockPageSize,
			MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
			PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
			PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		},
	).(*virtualQueueImpl)

	mockVirtualSlice1.EXPECT().Clear().Times(1)

	// first time we call loadAndSubmitTasks, we should pause, so set the total pending task count to be larger than MaxPendingTasksCount
	mockMonitor.EXPECT().GetTotalPendingTaskCount().Return(101).Times(1)

	// then we should resume from pause and load the tasks
	// to simplify the test, we just assume that there is no more tasks to load
	mockMonitor.EXPECT().GetTotalPendingTaskCount().Return(0).Times(1)
	mockVirtualSlice1.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{}, nil).Times(1)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice1, 0).Times(1)
	mockVirtualSlice1.EXPECT().HasMoreTasks().Return(false).Times(1)
	mockVirtualSlice1.EXPECT().GetPendingTaskCount().Return(0).Times(1)

	queue.Start()

	// wait for the pause controller to resume
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(time.Second * 10)

	// sleep for a while and yield the control to the background goroutine loading tasks from virtual slices
	time.Sleep(time.Millisecond * 100)

	queue.Stop()
}
