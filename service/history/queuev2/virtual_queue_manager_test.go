package queuev2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/task"
)

func TestVirtualQueueManager_VirtualQueues(t *testing.T) {
	tests := []struct {
		name            string
		initialStates   map[int64][]VirtualSliceState
		expectedStates  map[int64][]VirtualSliceState
		setupMockQueues func(map[int64]*MockVirtualQueue)
	}{
		{
			name:           "empty virtual queues",
			initialStates:  map[int64][]VirtualSliceState{},
			expectedStates: map[int64][]VirtualSliceState{},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				// No mocks to set up for empty case
			},
		},
		{
			name: "single queue with single slice",
			initialStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
			},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[1].EXPECT().GetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				})
			},
		},
		{
			name: "multiple queues with multiple slices",
			initialStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				},
			},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[1].EXPECT().GetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				})
				mocks[2].EXPECT().GetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				})
			},
		},
		{
			name: "queue with empty state",
			initialStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {},
			},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[1].EXPECT().GetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				})
				mocks[2].EXPECT().GetState().Return([]VirtualSliceState{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Create mock dependencies
			mockProcessor := task.NewMockProcessor(ctrl)
			mockRedispatcher := task.NewMockRedispatcher(ctrl)
			mockTaskInitializer := func(t persistence.Task) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskID().Return(t.GetTaskID())
				return mockTask
			}
			mockQueueReader := NewMockQueueReader(ctrl)
			mockLogger := log.NewNoop()
			mockMetricsScope := metrics.NoopScope

			// Create virtual queues map with mocks
			virtualQueues := make(map[int64]VirtualQueue)
			mockQueues := make(map[int64]*MockVirtualQueue)
			for queueID := range tt.initialStates {
				mockQueue := NewMockVirtualQueue(ctrl)
				mockQueues[queueID] = mockQueue
				virtualQueues[queueID] = mockQueue
			}

			// Set up mock expectations
			tt.setupMockQueues(mockQueues)

			// Create manager instance
			manager := &virtualQueueManagerImpl{
				processor:       mockProcessor,
				taskInitializer: mockTaskInitializer,
				redispatcher:    mockRedispatcher,
				queueReader:     mockQueueReader,
				logger:          mockLogger,
				metricsScope:    mockMetricsScope,
				options: &VirtualQueueManagerOptions{
					RootQueueOptions: &VirtualQueueOptions{},
					NonRootQueueOptions: &VirtualQueueOptions{
						PageSize: dynamicproperties.GetIntPropertyFn(100),
					},
				},
				status:        common.DaemonStatusInitialized,
				virtualQueues: virtualQueues,
			}

			// Execute test
			vqs := manager.VirtualQueues()
			states := make(map[int64][]VirtualSliceState)
			for queueID, virtualQueue := range vqs {
				states[queueID] = virtualQueue.GetState()
			}

			// Verify results
			assert.Equal(t, tt.expectedStates, states)
		})
	}
}

func TestVirtualQueueManager_UpdateAndGetState(t *testing.T) {
	tests := []struct {
		name            string
		initialStates   map[int64][]VirtualSliceState
		expectedStates  map[int64][]VirtualSliceState
		setupMockQueues func(map[int64]*MockVirtualQueue)
	}{
		{
			name:           "empty virtual queues",
			initialStates:  map[int64][]VirtualSliceState{},
			expectedStates: map[int64][]VirtualSliceState{},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				// No mocks to set up for empty case
			},
		},
		{
			name: "single queue with single slice",
			initialStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
			},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[1].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				})
			},
		},
		{
			name: "multiple queues with multiple slices",
			initialStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				},
			},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[1].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				})
				mocks[2].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				})
			},
		},
		{
			name: "queue with empty state gets removed",
			initialStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
			},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[1].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				})
				mocks[2].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{})
				mocks[2].EXPECT().Stop()
			},
		},
		{
			name: "all queues empty get removed",
			initialStates: map[int64][]VirtualSliceState{
				1: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
				2: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[1].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{})
				mocks[1].EXPECT().Stop()
				mocks[2].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{})
				mocks[2].EXPECT().Stop()
			},
		},
		{
			name: "empty root queue is not removed",
			initialStates: map[int64][]VirtualSliceState{
				0: {
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
			},
			expectedStates: map[int64][]VirtualSliceState{},
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue) {
				mocks[0].EXPECT().UpdateAndGetState().Return([]VirtualSliceState{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Create mock dependencies
			mockProcessor := task.NewMockProcessor(ctrl)
			mockRedispatcher := task.NewMockRedispatcher(ctrl)
			mockTaskInitializer := func(t persistence.Task) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskID().Return(t.GetTaskID())
				return mockTask
			}
			mockQueueReader := NewMockQueueReader(ctrl)
			mockLogger := log.NewNoop()
			mockMetricsScope := metrics.NoopScope

			// Create virtual queues map with mocks
			virtualQueues := make(map[int64]VirtualQueue)
			mockQueues := make(map[int64]*MockVirtualQueue)
			for queueID := range tt.initialStates {
				mockQueue := NewMockVirtualQueue(ctrl)
				mockQueues[queueID] = mockQueue
				virtualQueues[queueID] = mockQueue
			}

			// Set up mock expectations
			tt.setupMockQueues(mockQueues)

			// Create manager instance
			manager := &virtualQueueManagerImpl{
				processor:       mockProcessor,
				taskInitializer: mockTaskInitializer,
				redispatcher:    mockRedispatcher,
				queueReader:     mockQueueReader,
				logger:          mockLogger,
				metricsScope:    mockMetricsScope,
				options: &VirtualQueueManagerOptions{
					RootQueueOptions: &VirtualQueueOptions{},
					NonRootQueueOptions: &VirtualQueueOptions{
						MaxPendingTasksCount: dynamicproperties.GetIntPropertyFn(100),
					},
				},
				status:        common.DaemonStatusInitialized,
				virtualQueues: virtualQueues,
			}

			// Execute test
			states := manager.UpdateAndGetState()

			// Verify results
			assert.Equal(t, tt.expectedStates, states)
		})
	}
}

func TestVirtualQueueManager_AddNewVirtualSlice(t *testing.T) {
	tests := []struct {
		name            string
		initialQueues   map[int64]VirtualQueue
		newSlice        VirtualSlice
		setupMockQueues func(map[int64]*MockVirtualQueue, *MockVirtualSlice)
		verifyQueues    func(*testing.T, map[int64]VirtualQueue)
		appendSlice     bool
	}{
		{
			name: "merge slice to existing root queue",
			initialQueues: map[int64]VirtualQueue{
				rootQueueID: nil, // Will be replaced with mock
			},
			newSlice: nil, // Will be replaced with mock
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue, slice *MockVirtualSlice) {
				mocks[rootQueueID].EXPECT().MergeSlices(slice)
			},
			verifyQueues: func(t *testing.T, queues map[int64]VirtualQueue) {
				assert.Contains(t, queues, int64(rootQueueID))
				assert.Len(t, queues, 1)
			},
		},
		{
			name: "append slice to existing root queue",
			initialQueues: map[int64]VirtualQueue{
				rootQueueID: nil, // Will be replaced with mock
			},
			newSlice: nil, // Will be replaced with mock
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue, slice *MockVirtualSlice) {
				mocks[rootQueueID].EXPECT().AppendSlices(slice)
			},
			verifyQueues: func(t *testing.T, queues map[int64]VirtualQueue) {
				assert.Contains(t, queues, int64(rootQueueID))
				assert.Len(t, queues, 1)
			},
			appendSlice: true,
		},
		{
			name:          "create new root queue when none exists",
			initialQueues: map[int64]VirtualQueue{},
			newSlice:      nil, // Will be replaced with mock
			setupMockQueues: func(mocks map[int64]*MockVirtualQueue, slice *MockVirtualSlice) {
				// No expectations needed as we're creating a new queue
			},
			verifyQueues: func(t *testing.T, queues map[int64]VirtualQueue) {
				assert.Contains(t, queues, int64(rootQueueID))
				assert.Len(t, queues, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Create mock dependencies
			mockProcessor := task.NewMockProcessor(ctrl)
			mockRedispatcher := task.NewMockRedispatcher(ctrl)
			mockTaskInitializer := func(t persistence.Task) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskID().Return(t.GetTaskID())
				return mockTask
			}
			mockQueueReader := NewMockQueueReader(ctrl)
			mockLogger := log.NewNoop()
			mockMetricsScope := metrics.NoopScope
			mockTimeSource := clock.NewMockedTimeSource()

			// Create mock slice
			mockSlice := NewMockVirtualSlice(ctrl)

			// Create virtual queues map with mocks
			virtualQueues := make(map[int64]VirtualQueue)
			mockQueues := make(map[int64]*MockVirtualQueue)
			for queueID := range tt.initialQueues {
				mockQueue := NewMockVirtualQueue(ctrl)
				mockQueues[queueID] = mockQueue
				virtualQueues[queueID] = mockQueue
			}

			// Set up mock expectations
			tt.setupMockQueues(mockQueues, mockSlice)

			forceNewSliceDuration := time.Minute
			// Create manager instance
			manager := &virtualQueueManagerImpl{
				processor:       mockProcessor,
				taskInitializer: mockTaskInitializer,
				redispatcher:    mockRedispatcher,
				queueReader:     mockQueueReader,
				logger:          mockLogger,
				metricsScope:    mockMetricsScope,
				timeSource:      mockTimeSource,
				options: &VirtualQueueManagerOptions{
					RootQueueOptions: &VirtualQueueOptions{
						PageSize: dynamicproperties.GetIntPropertyFn(100),
					},
					NonRootQueueOptions: &VirtualQueueOptions{
						PageSize: dynamicproperties.GetIntPropertyFn(100),
					},
					VirtualSliceForceAppendInterval: dynamicproperties.GetDurationPropertyFn(forceNewSliceDuration),
				},
				status:        common.DaemonStatusInitialized,
				virtualQueues: virtualQueues,
				createVirtualQueueFn: func(queueID int64, s ...VirtualSlice) VirtualQueue {
					vq := NewMockVirtualQueue(ctrl)
					vq.EXPECT().Start()
					return vq
				},
				nextForceNewSliceTime: mockTimeSource.Now(),
			}

			// Execute test
			if tt.appendSlice {
				mockTimeSource.Advance(forceNewSliceDuration)
			}
			manager.AddNewVirtualSliceToRootQueue(mockSlice)

			// Verify results
			tt.verifyQueues(t, manager.virtualQueues)
		})
	}
}
