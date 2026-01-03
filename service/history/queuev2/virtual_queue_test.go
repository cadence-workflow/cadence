package queuev2

import (
	"container/list"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
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
	mockRescheduler := task.NewMockRescheduler(ctrl)
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
		mockRescheduler,
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
	mockRescheduler := task.NewMockRescheduler(ctrl)
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
	mockVirtualSlice1.EXPECT().IsEmpty().Return(false)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice1, 1)

	mockVirtualSlice2.EXPECT().UpdateAndGetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(11),
		},
		Predicate: NewUniversalPredicate(),
	})
	mockVirtualSlice2.EXPECT().IsEmpty().Return(true)
	mockMonitor.EXPECT().RemoveSlice(mockVirtualSlice2)

	mockTimeSource := clock.NewMockedTimeSource()
	mockRateLimiter := quotas.NewMockLimiter(ctrl)

	queue := NewVirtualQueue(
		mockProcessor,
		mockRescheduler,
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
			mockRescheduler := task.NewMockRescheduler(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsScope := metrics.NoopScope
			mockTimeSource := clock.NewMockedTimeSource()
			mockRateLimiter := quotas.NewMockLimiter(ctrl)

			existingSlices, incomingSlices, monitor := tt.setupMocks(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRescheduler,
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
	mockRescheduler := task.NewMockRescheduler(ctrl)
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
	mockTask1.EXPECT().GetVisibilityTimestamp().Return(mockTimeSource.Now().Add(time.Second * -1))
	mockTask1.EXPECT().SetInitialSubmitTime(gomock.Any()).Times(1)
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
	mockTask3.EXPECT().GetVisibilityTimestamp().Return(mockTimeSource.Now().Add(time.Second * -1))
	mockTask3.EXPECT().SetInitialSubmitTime(gomock.Any()).Times(1)

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
	mockRescheduler.EXPECT().RescheduleTask(mockTask2, mockTimeSource.Now().Add(time.Second*1))
	mockRescheduler.EXPECT().RescheduleTask(mockTask3, mockTimeSource.Now().Add(taskSchedulerThrottleBackoffInterval))

	queue := NewVirtualQueue(
		mockProcessor,
		mockRescheduler,
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
	mockRescheduler := task.NewMockRescheduler(ctrl)
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
		mockRescheduler,
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
	mockRescheduler := task.NewMockRescheduler(ctrl)
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
		mockRescheduler,
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

	gomock.InOrder(
		// first time we call loadAndSubmitTasks, we should pause, so set the total pending task count to be larger than MaxPendingTasksCount
		mockMonitor.EXPECT().GetTotalPendingTaskCount().Return(101).Times(1),
		// then we should resume from pause and load the tasks
		// to simplify the test, we just assume that there is no more tasks to load
		mockMonitor.EXPECT().GetTotalPendingTaskCount().Return(0).MaxTimes(1),
		mockVirtualSlice1.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{}, nil).MaxTimes(1),
		mockVirtualSlice1.EXPECT().GetPendingTaskCount().Return(0).MaxTimes(1),
		mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice1, 0).MaxTimes(1),
		mockVirtualSlice1.EXPECT().HasMoreTasks().Return(false).MaxTimes(1),
		mockVirtualSlice1.EXPECT().Clear().Times(1),
	)

	queue.Start()

	// wait for the pause controller to resume
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(time.Second * 10)

	queue.Stop()
}

func TestVirtualQueue_IterateSlices(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRescheduler := task.NewMockRescheduler(ctrl)
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
		mockRescheduler,
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

	states := []VirtualSliceState{}
	queue.IterateSlices(func(slice VirtualSlice) {
		states = append(states, slice.GetState())
	})

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

func TestVirtualQueue_ClearSlices(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRescheduler := task.NewMockRescheduler(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsScope := metrics.NoopScope
	mockPageSize := dynamicproperties.GetIntPropertyFn(10)
	mockMonitor := NewMockMonitor(ctrl)

	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)
	mockVirtualSlice2 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
		mockVirtualSlice2,
	}

	mockVirtualSlice1.EXPECT().Clear().Times(1)
	mockVirtualSlice1.EXPECT().GetPendingTaskCount().Return(0).Times(1)
	mockVirtualSlice1.EXPECT().HasMoreTasks().Return(false).Times(1)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice1, 0).Times(1)
	mockVirtualSlice2.EXPECT().Clear().Times(1)
	mockVirtualSlice2.EXPECT().GetPendingTaskCount().Return(0).Times(1)
	mockVirtualSlice2.EXPECT().HasMoreTasks().Return(true).Times(1)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice2, 0).Times(1)
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

	queue := NewVirtualQueue(
		mockProcessor,
		mockRescheduler,
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

	queue.ClearSlices(func(slice VirtualSlice) bool {
		return true
	})

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
	assert.Equal(t, mockVirtualSlice2, queue.(*virtualQueueImpl).sliceToRead.Value.(VirtualSlice))
}

func TestVirtualQueue_SplitSlices(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockProcessor := task.NewMockProcessor(ctrl)
	mockRescheduler := task.NewMockRescheduler(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsScope := metrics.NoopScope
	mockPageSize := dynamicproperties.GetIntPropertyFn(10)

	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)
	mockVirtualSlice2 := NewMockVirtualSlice(ctrl)
	mockVirtualSlice3 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
		mockVirtualSlice2,
	}
	mockMonitor := NewMockMonitor(ctrl)

	mockVirtualSlice1.EXPECT().GetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
		},
		Predicate: NewUniversalPredicate(),
	})
	mockVirtualSlice1.EXPECT().HasMoreTasks().Return(true).Times(1)
	mockMonitor.EXPECT().RemoveSlice(mockVirtualSlice2).Times(1)
	mockVirtualSlice3.EXPECT().GetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(15),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
		},
		Predicate: NewUniversalPredicate(),
	})
	mockVirtualSlice3.EXPECT().GetPendingTaskCount().Return(0).Times(1)
	mockMonitor.EXPECT().SetSlicePendingTaskCount(mockVirtualSlice3, 0).Times(1)

	mockTimeSource := clock.NewMockedTimeSource()
	mockRateLimiter := quotas.NewMockLimiter(ctrl)

	queue := NewVirtualQueue(
		mockProcessor,
		mockRescheduler,
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

	queue.SplitSlices(func(slice VirtualSlice) (remaining []VirtualSlice, split bool) {
		if slice == mockVirtualSlice1 {
			return nil, false
		} else if slice == mockVirtualSlice2 {
			return []VirtualSlice{mockVirtualSlice3}, true
		}
		return nil, false
	})

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
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(15),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
			},
			Predicate: NewUniversalPredicate(),
		},
	}
	assert.Equal(t, expectedStates, states)
	assert.Equal(t, mockVirtualSlice1, queue.(*virtualQueueImpl).sliceToRead.Value.(VirtualSlice))
}

func TestVirtualQueue_AppendSlices(t *testing.T) {
	tests := []struct {
		name                   string
		setupMocks             func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor)
		expectedStates         []VirtualSliceState
		expectedSliceToReadIdx *int // nil if sliceToRead should be nil, otherwise index of expected slice
	}{
		{
			name: "Append empty slice list",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)

				return []VirtualSlice{existingSlice}, []VirtualSlice{}, monitor
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
			expectedSliceToReadIdx: common.Ptr(0),
		},
		{
			name: "Append single slice to empty queue",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				incomingSlice.EXPECT().HasMoreTasks().Return(true).Times(1)

				return []VirtualSlice{}, []VirtualSlice{incomingSlice}, monitor
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
			expectedSliceToReadIdx: common.Ptr(0),
		},
		{
			name: "Append multiple slices to empty queue",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				incomingSlice1 := NewMockVirtualSlice(ctrl)
				incomingSlice2 := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				incomingSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				incomingSlice1.EXPECT().HasMoreTasks().Return(true).Times(1)

				incomingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)

				return []VirtualSlice{}, []VirtualSlice{incomingSlice1, incomingSlice2}, monitor
			},
			expectedStates: []VirtualSliceState{
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
			},
			expectedSliceToReadIdx: common.Ptr(0),
		},
		{
			name: "Append slices to queue with existing slices",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				existingSlice1 := NewMockVirtualSlice(ctrl)
				existingSlice2 := NewMockVirtualSlice(ctrl)
				incomingSlice1 := NewMockVirtualSlice(ctrl)
				incomingSlice2 := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				existingSlice1.EXPECT().HasMoreTasks().Return(false).Times(1)

				existingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				existingSlice2.EXPECT().HasMoreTasks().Return(true).Times(1)

				incomingSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(21),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)

				incomingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(31),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(40),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)

				return []VirtualSlice{existingSlice1, existingSlice2}, []VirtualSlice{incomingSlice1, incomingSlice2}, monitor
			},
			expectedStates: []VirtualSliceState{
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
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(21),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
					},
					Predicate: NewUniversalPredicate(),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(31),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(40),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedSliceToReadIdx: common.Ptr(1),
		},
		{
			name: "Append slices when no existing slice has more tasks",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				existingSlice.EXPECT().HasMoreTasks().Return(false).Times(1)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				incomingSlice.EXPECT().HasMoreTasks().Return(true).Times(1)

				return []VirtualSlice{existingSlice}, []VirtualSlice{incomingSlice}, monitor
			},
			expectedStates: []VirtualSliceState{
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
			},
			expectedSliceToReadIdx: common.Ptr(1),
		},
		{
			name: "Append slices when all slices have no more tasks",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				existingSlice.EXPECT().HasMoreTasks().Return(false).Times(1)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				}).Times(1)
				incomingSlice.EXPECT().HasMoreTasks().Return(false).Times(1)

				return []VirtualSlice{existingSlice}, []VirtualSlice{incomingSlice}, monitor
			},
			expectedStates: []VirtualSliceState{
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
			},
			expectedSliceToReadIdx: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockProcessor := task.NewMockProcessor(ctrl)
			mockRescheduler := task.NewMockRescheduler(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsScope := metrics.NoopScope
			mockTimeSource := clock.NewMockedTimeSource()
			mockRateLimiter := quotas.NewMockLimiter(ctrl)

			existingSlices, incomingSlices, monitor := tt.setupMocks(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRescheduler,
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

			queue.AppendSlices(incomingSlices...)
			states := queue.GetState()

			assert.Equal(t, tt.expectedStates, states)

			// Check sliceToRead
			queueImpl := queue.(*virtualQueueImpl)
			if tt.expectedSliceToReadIdx == nil {
				assert.Nil(t, queueImpl.sliceToRead, "sliceToRead should be nil")
			} else {
				assert.NotNil(t, queueImpl.sliceToRead, "sliceToRead should not be nil")
				if queueImpl.sliceToRead != nil {
					expectedSlice := append(existingSlices, incomingSlices...)[*tt.expectedSliceToReadIdx]
					assert.Equal(t, expectedSlice, queueImpl.sliceToRead.Value.(VirtualSlice))
				}
			}
		})
	}
}

func TestVirtualQueue_MergeWithLastSlice(t *testing.T) {
	tests := []struct {
		name                   string
		setupMocks             func(ctrl *gomock.Controller) ([]VirtualSlice, VirtualSlice, Monitor)
		expectedStates         []VirtualSliceState
		expectedSliceToReadIdx *int // nil if sliceToRead should be nil, otherwise index of expected slice
	}{
		{
			name: "Merge with empty queue",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, VirtualSlice, Monitor) {
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().GetPendingTaskCount().Return(5)
				incomingSlice.EXPECT().HasMoreTasks().Return(true)
				monitor.EXPECT().SetSlicePendingTaskCount(incomingSlice, 5)

				return []VirtualSlice{}, incomingSlice, monitor
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
			expectedSliceToReadIdx: common.Ptr(0),
		},
		{
			name: "Merge with last slice - no merge possible",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, VirtualSlice, Monitor) {
				existingSlice1 := NewMockVirtualSlice(ctrl)
				existingSlice2 := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice1.EXPECT().HasMoreTasks().Return(false)

				existingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice2.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{}, false)
				existingSlice2.EXPECT().HasMoreTasks().Return(true)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(15),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().GetPendingTaskCount().Return(3)
				monitor.EXPECT().SetSlicePendingTaskCount(incomingSlice, 3)

				return []VirtualSlice{existingSlice1, existingSlice2}, incomingSlice, monitor
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
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(15),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedSliceToReadIdx: common.Ptr(1), // existingSlice1 has more tasks
		},
		{
			name: "Merge with last slice - successful merge",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, VirtualSlice, Monitor) {
				existingSlice1 := NewMockVirtualSlice(ctrl)
				existingSlice2 := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice1.EXPECT().HasMoreTasks().Return(true)

				existingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice2.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{mergedSlice}, true)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(8),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mergedSlice.EXPECT().GetPendingTaskCount().Return(7)

				monitor.EXPECT().RemoveSlice(existingSlice2)
				monitor.EXPECT().RemoveSlice(incomingSlice)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice, 7)

				return []VirtualSlice{existingSlice1, existingSlice2}, incomingSlice, monitor
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
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedSliceToReadIdx: common.Ptr(0), // existingSlice1 has more tasks
		},
		{
			name: "Merge with last slice - multiple merged slices result",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, VirtualSlice, Monitor) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice1 := NewMockVirtualSlice(ctrl)
				mergedSlice2 := NewMockVirtualSlice(ctrl)
				monitor := NewMockMonitor(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{mergedSlice1, mergedSlice2}, true)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mergedSlice1.EXPECT().GetPendingTaskCount().Return(4)
				mergedSlice1.EXPECT().HasMoreTasks().Return(true)

				mergedSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(7),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mergedSlice2.EXPECT().GetPendingTaskCount().Return(6)

				monitor.EXPECT().RemoveSlice(existingSlice)
				monitor.EXPECT().RemoveSlice(incomingSlice)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice1, 4)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice2, 6)

				return []VirtualSlice{existingSlice}, incomingSlice, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					Predicate: NewUniversalPredicate(),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(7),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedSliceToReadIdx: common.Ptr(0), // mergedSlice1 has more tasks
		},
		{
			name: "Merge with single existing slice - no merge possible",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, VirtualSlice, Monitor) {
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
				existingSlice.EXPECT().HasMoreTasks().Return(true)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(10),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().GetPendingTaskCount().Return(2)
				monitor.EXPECT().SetSlicePendingTaskCount(incomingSlice, 2)

				return []VirtualSlice{existingSlice}, incomingSlice, monitor
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
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(10),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedSliceToReadIdx: common.Ptr(0), // existingSlice has more tasks
		},
		{
			name: "Merge with single existing slice - successful merge",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, VirtualSlice, Monitor) {
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
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				mergedSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mergedSlice.EXPECT().GetPendingTaskCount().Return(5)
				mergedSlice.EXPECT().HasMoreTasks().Return(true)

				monitor.EXPECT().RemoveSlice(existingSlice)
				monitor.EXPECT().RemoveSlice(incomingSlice)
				monitor.EXPECT().SetSlicePendingTaskCount(mergedSlice, 5)

				return []VirtualSlice{existingSlice}, incomingSlice, monitor
			},
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedSliceToReadIdx: common.Ptr(0), // mergedSlice has more tasks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProcessor := task.NewMockProcessor(ctrl)
			mockRescheduler := task.NewMockRescheduler(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsScope := metrics.NoopScope
			mockTimeSource := clock.NewMockedTimeSource()
			mockRateLimiter := quotas.NewMockLimiter(ctrl)

			existingSlices, incomingSlice, monitor := tt.setupMocks(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRescheduler,
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

			queue.MergeWithLastSlice(incomingSlice)
			states := queue.GetState()

			assert.Equal(t, tt.expectedStates, states)

			// Check sliceToRead
			queueImpl := queue.(*virtualQueueImpl)
			if tt.expectedSliceToReadIdx == nil {
				assert.Nil(t, queueImpl.sliceToRead, "sliceToRead should be nil")
			} else {
				assert.NotNil(t, queueImpl.sliceToRead, "sliceToRead should not be nil")
				if queueImpl.sliceToRead != nil {
					// For MergeWithLastSlice, we need to build the final slice list differently
					// since the incoming slice is merged/appended to the existing slices
					var finalSlices []VirtualSlice
					for _, state := range states {
						// Find the slice that matches this state
						for e := queueImpl.virtualSlices.Front(); e != nil; e = e.Next() {
							slice := e.Value.(VirtualSlice)
							if slice.GetState() == state {
								finalSlices = append(finalSlices, slice)
								break
							}
						}
					}
					expectedSlice := finalSlices[*tt.expectedSliceToReadIdx]
					assert.Equal(t, expectedSlice, queueImpl.sliceToRead.Value.(VirtualSlice))
				}
			}
		})
	}
}

func TestVirtualQueue_InsertSingleTask(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(ctrl *gomock.Controller) ([]VirtualSlice, task.Task, task.Processor, task.Rescheduler, Monitor, clock.TimeSource)
		expectedResult bool
	}{
		{
			name: "Successfully insert task into first slice",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, task.Task, task.Processor, task.Rescheduler, Monitor, clock.TimeSource) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()

				mockSlice1.EXPECT().InsertTask(mockTask).Return(true)
				mockSlice1.EXPECT().GetPendingTaskCount().Return(5)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice1, 5)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(-time.Second), 1))
				mockTask.EXPECT().GetVisibilityTimestamp().Return(mockTimeSource.Now().Add(-time.Second))
				mockTask.EXPECT().SetInitialSubmitTime(gomock.Any())
				mockProcessor.EXPECT().TrySubmit(mockTask).Return(true, nil)

				return []VirtualSlice{mockSlice1, mockSlice2}, mockTask, mockProcessor, mockRescheduler, mockMonitor, mockTimeSource
			},
			expectedResult: true,
		},
		{
			name: "Successfully insert task into second slice",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, task.Task, task.Processor, task.Rescheduler, Monitor, clock.TimeSource) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()

				mockSlice1.EXPECT().InsertTask(mockTask).Return(false)
				mockSlice2.EXPECT().InsertTask(mockTask).Return(true)
				mockSlice2.EXPECT().GetPendingTaskCount().Return(3)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice2, 3)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(-time.Second), 1))
				mockTask.EXPECT().GetVisibilityTimestamp().Return(mockTimeSource.Now().Add(-time.Second))
				mockTask.EXPECT().SetInitialSubmitTime(gomock.Any())
				mockProcessor.EXPECT().TrySubmit(mockTask).Return(true, nil)

				return []VirtualSlice{mockSlice1, mockSlice2}, mockTask, mockProcessor, mockRescheduler, mockMonitor, mockTimeSource
			},
			expectedResult: true,
		},
		{
			name: "No slice accepts the task",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, task.Task, task.Processor, task.Rescheduler, Monitor, clock.TimeSource) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()

				mockSlice1.EXPECT().InsertTask(mockTask).Return(false)
				mockSlice2.EXPECT().InsertTask(mockTask).Return(false)

				return []VirtualSlice{mockSlice1, mockSlice2}, mockTask, mockProcessor, mockRescheduler, mockMonitor, mockTimeSource
			},
			expectedResult: false,
		},
		{
			name: "Insert task with submission error",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, task.Task, task.Processor, task.Rescheduler, Monitor, clock.TimeSource) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()

				mockSlice1.EXPECT().InsertTask(mockTask).Return(true)
				mockSlice1.EXPECT().GetPendingTaskCount().Return(1)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice1, 1)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(-time.Second), 1))
				mockTask.EXPECT().GetVisibilityTimestamp().Return(mockTimeSource.Now().Add(-time.Second))
				mockTask.EXPECT().SetInitialSubmitTime(gomock.Any())
				mockProcessor.EXPECT().TrySubmit(mockTask).Return(true, assert.AnError)

				return []VirtualSlice{mockSlice1}, mockTask, mockProcessor, mockRescheduler, mockMonitor, mockTimeSource
			},
			expectedResult: true,
		},
		{
			name: "Insert task with empty slice list",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, task.Task, task.Processor, task.Rescheduler, Monitor, clock.TimeSource) {
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				mockTimeSource := clock.NewMockedTimeSource()

				return []VirtualSlice{}, mockTask, mockProcessor, mockRescheduler, mockMonitor, mockTimeSource
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			slices, mockTask, mockProcessor, mockRescheduler, mockMonitor, mockTimeSource := tt.setupMocks(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsScope := metrics.NoopScope
			mockRateLimiter := quotas.NewMockLimiter(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRescheduler,
				mockLogger,
				mockMetricsScope,
				mockTimeSource,
				mockRateLimiter,
				mockMonitor,
				slices,
				&VirtualQueueOptions{
					PageSize:                             dynamicproperties.GetIntPropertyFn(10),
					MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
					PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
					PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
				},
			)

			result := queue.InsertSingleTask(mockTask)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestVirtualQueue_ResetProgress(t *testing.T) {
	tests := []struct {
		name                   string
		setupMocks             func(ctrl *gomock.Controller) ([]VirtualSlice, Monitor, persistence.HistoryTaskKey)
		expectedSliceToReadIdx *int
	}{
		{
			name: "Reset progress on all slices before sliceToRead",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, Monitor, persistence.HistoryTaskKey) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockSlice3 := NewMockVirtualSlice(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				resetKey := persistence.NewHistoryTaskKey(time.Now(), 5)

				mockSlice1.EXPECT().ResetProgress(resetKey)
				mockSlice1.EXPECT().GetPendingTaskCount().Return(2)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice1, 2)
				mockSlice1.EXPECT().HasMoreTasks().Return(false)

				mockSlice2.EXPECT().ResetProgress(resetKey)
				mockSlice2.EXPECT().GetPendingTaskCount().Return(3)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice2, 3)
				mockSlice2.EXPECT().HasMoreTasks().Return(true)

				return []VirtualSlice{mockSlice1, mockSlice2, mockSlice3}, mockMonitor, resetKey
			},
			expectedSliceToReadIdx: common.Ptr(1),
		},
		{
			name: "Reset progress with no sliceToRead set",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, Monitor, persistence.HistoryTaskKey) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				resetKey := persistence.NewHistoryTaskKey(time.Now(), 10)

				mockSlice1.EXPECT().ResetProgress(resetKey)
				mockSlice1.EXPECT().GetPendingTaskCount().Return(0)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice1, 0)
				mockSlice1.EXPECT().HasMoreTasks().Return(false)

				mockSlice2.EXPECT().ResetProgress(resetKey)
				mockSlice2.EXPECT().GetPendingTaskCount().Return(1)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice2, 1)
				mockSlice2.EXPECT().HasMoreTasks().Return(true)

				return []VirtualSlice{mockSlice1, mockSlice2}, mockMonitor, resetKey
			},
			expectedSliceToReadIdx: common.Ptr(1),
		},
		{
			name: "Reset progress with no slices having more tasks",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, Monitor, persistence.HistoryTaskKey) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				resetKey := persistence.NewHistoryTaskKey(time.Now(), 15)

				mockSlice1.EXPECT().ResetProgress(resetKey)
				mockSlice1.EXPECT().GetPendingTaskCount().Return(0)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice1, 0)
				mockSlice1.EXPECT().HasMoreTasks().Return(false)

				mockSlice2.EXPECT().ResetProgress(resetKey)
				mockSlice2.EXPECT().GetPendingTaskCount().Return(0)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice2, 0)
				mockSlice2.EXPECT().HasMoreTasks().Return(false)

				return []VirtualSlice{mockSlice1, mockSlice2}, mockMonitor, resetKey
			},
			expectedSliceToReadIdx: nil,
		},
		{
			name: "Reset progress with empty slice list",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, Monitor, persistence.HistoryTaskKey) {
				mockMonitor := NewMockMonitor(ctrl)
				resetKey := persistence.NewHistoryTaskKey(time.Now(), 20)

				return []VirtualSlice{}, mockMonitor, resetKey
			},
			expectedSliceToReadIdx: nil,
		},
		{
			name: "Reset progress stops at first slice when it's sliceToRead",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, Monitor, persistence.HistoryTaskKey) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockMonitor := NewMockMonitor(ctrl)
				resetKey := persistence.NewHistoryTaskKey(time.Now(), 25)

				mockSlice1.EXPECT().ResetProgress(resetKey)
				mockSlice1.EXPECT().GetPendingTaskCount().Return(4)
				mockMonitor.EXPECT().SetSlicePendingTaskCount(mockSlice1, 4)
				mockSlice1.EXPECT().HasMoreTasks().Return(true)

				return []VirtualSlice{mockSlice1, mockSlice2}, mockMonitor, resetKey
			},
			expectedSliceToReadIdx: common.Ptr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			slices, mockMonitor, resetKey := tt.setupMocks(ctrl)
			mockProcessor := task.NewMockProcessor(ctrl)
			mockRescheduler := task.NewMockRescheduler(ctrl)
			mockLogger := testlogger.New(t)
			mockMetricsScope := metrics.NoopScope
			mockTimeSource := clock.NewMockedTimeSource()
			mockRateLimiter := quotas.NewMockLimiter(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRescheduler,
				mockLogger,
				mockMetricsScope,
				mockTimeSource,
				mockRateLimiter,
				mockMonitor,
				slices,
				&VirtualQueueOptions{
					PageSize:                             dynamicproperties.GetIntPropertyFn(10),
					MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
					PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
					PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
				},
			)

			queueImpl := queue.(*virtualQueueImpl)

			if len(slices) > 0 && tt.name != "Reset progress with no sliceToRead set" && tt.name != "Reset progress with empty slice list" {
				if tt.name == "Reset progress stops at first slice when it's sliceToRead" {
					queueImpl.sliceToRead = queueImpl.virtualSlices.Front()
				} else {
					queueImpl.sliceToRead = queueImpl.virtualSlices.Front().Next()
				}
			} else if tt.name == "Reset progress with no sliceToRead set" {
				queueImpl.sliceToRead = nil
			}

			queue.ResetProgress(resetKey)

			if tt.expectedSliceToReadIdx == nil {
				assert.Nil(t, queueImpl.sliceToRead, "sliceToRead should be nil")
			} else {
				assert.NotNil(t, queueImpl.sliceToRead, "sliceToRead should not be nil")
				if queueImpl.sliceToRead != nil {
					expectedSlice := slices[*tt.expectedSliceToReadIdx]
					assert.Equal(t, expectedSlice, queueImpl.sliceToRead.Value.(VirtualSlice))
				}
			}
		})
	}
}

func TestVirtualQueue_SubmitOrRescheduleTask(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(ctrl *gomock.Controller) (task.Task, task.Processor, task.Rescheduler, metrics.Scope, clock.TimeSource, time.Time)
		expectErr  bool
	}{
		{
			name: "Submit corrupted task - should ack and return nil",
			setupMocks: func(ctrl *gomock.Controller) (task.Task, task.Processor, task.Rescheduler, metrics.Scope, clock.TimeSource, time.Time) {
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMetricsScope := metrics.NoopScope
				mockTimeSource := clock.NewMockedTimeSource()
				now := mockTimeSource.Now()

				mockTask.EXPECT().GetDomainID().Return("").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("").AnyTimes()
				mockTask.EXPECT().Ack()

				return mockTask, mockProcessor, mockRescheduler, mockMetricsScope, mockTimeSource, now
			},
			expectErr: false,
		},
		{
			name: "Reschedule future task",
			setupMocks: func(ctrl *gomock.Controller) (task.Task, task.Processor, task.Rescheduler, metrics.Scope, clock.TimeSource, time.Time) {
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMetricsScope := metrics.NoopScope
				mockTimeSource := clock.NewMockedTimeSource()
				now := mockTimeSource.Now()
				futureTime := now.Add(time.Hour)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(futureTime, 1))
				mockRescheduler.EXPECT().RescheduleTask(mockTask, futureTime)

				return mockTask, mockProcessor, mockRescheduler, mockMetricsScope, mockTimeSource, now
			},
			expectErr: false,
		},
		{
			name: "Successfully submit task",
			setupMocks: func(ctrl *gomock.Controller) (task.Task, task.Processor, task.Rescheduler, metrics.Scope, clock.TimeSource, time.Time) {
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMetricsScope := metrics.NoopScope
				mockTimeSource := clock.NewMockedTimeSource()
				now := mockTimeSource.Now()
				pastTime := now.Add(-time.Hour)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(pastTime, 1))
				mockTask.EXPECT().GetVisibilityTimestamp().Return(pastTime)
				mockTask.EXPECT().SetInitialSubmitTime(now)
				mockProcessor.EXPECT().TrySubmit(mockTask).Return(true, nil)

				return mockTask, mockProcessor, mockRescheduler, mockMetricsScope, mockTimeSource, now
			},
			expectErr: false,
		},
		{
			name: "Submit task with processor error",
			setupMocks: func(ctrl *gomock.Controller) (task.Task, task.Processor, task.Rescheduler, metrics.Scope, clock.TimeSource, time.Time) {
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMetricsScope := metrics.NoopScope
				mockTimeSource := clock.NewMockedTimeSource()
				now := mockTimeSource.Now()
				pastTime := now.Add(-time.Hour)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(pastTime, 1))
				mockTask.EXPECT().GetVisibilityTimestamp().Return(pastTime)
				mockTask.EXPECT().SetInitialSubmitTime(now)
				mockProcessor.EXPECT().TrySubmit(mockTask).Return(true, assert.AnError)

				return mockTask, mockProcessor, mockRescheduler, mockMetricsScope, mockTimeSource, now
			},
			expectErr: true,
		},
		{
			name: "Task submission throttled - should reschedule",
			setupMocks: func(ctrl *gomock.Controller) (task.Task, task.Processor, task.Rescheduler, metrics.Scope, clock.TimeSource, time.Time) {
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMetricsScope := metrics.NoopScope
				mockTimeSource := clock.NewMockedTimeSource()
				now := mockTimeSource.Now()
				pastTime := now.Add(-time.Hour)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(pastTime, 1))
				mockTask.EXPECT().GetVisibilityTimestamp().Return(pastTime)
				mockTask.EXPECT().SetInitialSubmitTime(now)
				mockProcessor.EXPECT().TrySubmit(mockTask).Return(false, nil)
				mockRescheduler.EXPECT().RescheduleTask(mockTask, now.Add(taskSchedulerThrottleBackoffInterval))

				return mockTask, mockProcessor, mockRescheduler, mockMetricsScope, mockTimeSource, now
			},
			expectErr: false,
		},
		{
			name: "Task submission throttled with error - should reschedule and return error",
			setupMocks: func(ctrl *gomock.Controller) (task.Task, task.Processor, task.Rescheduler, metrics.Scope, clock.TimeSource, time.Time) {
				mockTask := task.NewMockTask(ctrl)
				mockProcessor := task.NewMockProcessor(ctrl)
				mockRescheduler := task.NewMockRescheduler(ctrl)
				mockMetricsScope := metrics.NoopScope
				mockTimeSource := clock.NewMockedTimeSource()
				now := mockTimeSource.Now()
				pastTime := now.Add(-time.Hour)

				mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
				mockTask.EXPECT().GetWorkflowID().Return("test-workflow").AnyTimes()
				mockTask.EXPECT().GetRunID().Return("test-run").AnyTimes()
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(pastTime, 1))
				mockTask.EXPECT().GetVisibilityTimestamp().Return(pastTime)
				mockTask.EXPECT().SetInitialSubmitTime(now)
				mockProcessor.EXPECT().TrySubmit(mockTask).Return(false, assert.AnError)
				mockRescheduler.EXPECT().RescheduleTask(mockTask, now.Add(taskSchedulerThrottleBackoffInterval))

				return mockTask, mockProcessor, mockRescheduler, mockMetricsScope, mockTimeSource, now
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTask, mockProcessor, mockRescheduler, mockMetricsScope, mockTimeSource, now := tt.setupMocks(ctrl)
			mockLogger := testlogger.New(t)
			mockRateLimiter := quotas.NewMockLimiter(ctrl)
			mockMonitor := NewMockMonitor(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRescheduler,
				mockLogger,
				mockMetricsScope,
				mockTimeSource,
				mockRateLimiter,
				mockMonitor,
				[]VirtualSlice{},
				&VirtualQueueOptions{
					PageSize:                             dynamicproperties.GetIntPropertyFn(10),
					MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
					PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
					PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
				},
			)

			queueImpl := queue.(*virtualQueueImpl)
			err := queueImpl.submitOrRescheduleTask(now, mockTask)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
