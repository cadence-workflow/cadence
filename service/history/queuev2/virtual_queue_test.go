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

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize: mockPageSize,
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
	mockVirtualSlice2.EXPECT().UpdateAndGetState().Return(VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(11),
		},
		Predicate: NewUniversalPredicate(),
	})

	mockTimeSource := clock.NewMockedTimeSource()

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize: mockPageSize,
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
		setupMocks     func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice)
		expectedStates []VirtualSliceState
	}{
		{
			name: "Merge non-overlapping slices",
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice) {
				existingSlice := NewMockVirtualSlice(ctrl)
				existingSlice2 := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().HasMoreTasks().Return(true)
				existingSlice.EXPECT().TryMergeWithVirtualSlice(incomingSlice).Return([]VirtualSlice{}, false)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().TryMergeWithVirtualSlice(existingSlice2).Return([]VirtualSlice{}, false)

				existingSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				return []VirtualSlice{existingSlice, existingSlice2}, []VirtualSlice{incomingSlice}
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
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice) {
				existingSlice := NewMockVirtualSlice(ctrl)
				existingSlice2 := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice := NewMockVirtualSlice(ctrl)
				mergedSlice2 := NewMockVirtualSlice(ctrl)

				existingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				existingSlice.EXPECT().TryMergeWithVirtualSlice(gomock.Any()).Return([]VirtualSlice{mergedSlice}, true)

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

				return []VirtualSlice{existingSlice, existingSlice2}, []VirtualSlice{incomingSlice}
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
			setupMocks: func(ctrl *gomock.Controller) ([]VirtualSlice, []VirtualSlice) {
				incomingSlice := NewMockVirtualSlice(ctrl)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				incomingSlice.EXPECT().HasMoreTasks().Return(true).AnyTimes()

				return []VirtualSlice{}, []VirtualSlice{incomingSlice}
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

			existingSlices, incomingSlices := tt.setupMocks(ctrl)

			queue := NewVirtualQueue(
				mockProcessor,
				mockRedispatcher,
				mockLogger,
				mockMetricsScope,
				mockTimeSource,
				existingSlices,
				&VirtualQueueOptions{
					PageSize: dynamicproperties.GetIntPropertyFn(10),
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
		setupMocks     func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice)
		expectedStates []VirtualSliceState
	}{
		{
			name: "Append when no merge possible",
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)

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

				return existingSlice, incomingSlice
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
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice := NewMockVirtualSlice(ctrl)

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

				return existingSlice, incomingSlice
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
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice) {
				incomingSlice := NewMockVirtualSlice(ctrl)

				incomingSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				return nil, incomingSlice
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
			setupMocks: func(ctrl *gomock.Controller) (VirtualSlice, VirtualSlice) {
				existingSlice := NewMockVirtualSlice(ctrl)
				incomingSlice := NewMockVirtualSlice(ctrl)
				mergedSlice1 := NewMockVirtualSlice(ctrl)
				mergedSlice2 := NewMockVirtualSlice(ctrl)

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

				return existingSlice, incomingSlice
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

			existingSlice, incomingSlice := tt.setupMocks(ctrl)
			slices := list.New()

			if existingSlice != nil {
				slices.PushBack(existingSlice)
			}

			appendOrMergeSlice(slices, incomingSlice)

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

	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)
	mockVirtualSlice2 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
		mockVirtualSlice2,
	}

	mockTask1 := task.NewMockTask(ctrl)
	mockTask1.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(time.Second*-1), 1))
	mockTask2 := task.NewMockTask(ctrl)
	mockTask2.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(time.Second*1), 2))
	mockTask3 := task.NewMockTask(ctrl)
	mockTask3.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(mockTimeSource.Now().Add(time.Second*-1), 1))

	mockVirtualSlice1.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{mockTask1, mockTask2}, nil)
	mockVirtualSlice1.EXPECT().HasMoreTasks().Return(false)
	mockVirtualSlice2.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{mockTask3}, nil)
	mockVirtualSlice2.EXPECT().HasMoreTasks().Return(false)
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
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize: mockPageSize,
		},
	).(*virtualQueueImpl)

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

	mockVirtualSlice1 := NewMockVirtualSlice(ctrl)

	mockVirtualSlices := []VirtualSlice{
		mockVirtualSlice1,
	}

	mockVirtualSlice1.EXPECT().GetTasks(gomock.Any(), 10).Return([]task.Task{}, nil).AnyTimes()
	mockVirtualSlice1.EXPECT().HasMoreTasks().Return(false).AnyTimes()

	queue := NewVirtualQueue(
		mockProcessor,
		mockRedispatcher,
		mockLogger,
		mockMetricsScope,
		mockTimeSource,
		mockVirtualSlices,
		&VirtualQueueOptions{
			PageSize: mockPageSize,
		},
	).(*virtualQueueImpl)

	queue.Start()
	queue.Stop()
}
