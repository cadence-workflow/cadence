package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
)

func TestNewMitigator(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	logger := testlogger.New(t)
	metricsScope := metrics.NoopScope
	options := &MitigatorOptions{
		MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
	}

	mitigator := NewMitigator(
		mockVirtualQueueManager,
		mockMonitor,
		logger,
		metricsScope,
		options,
	)

	require.NotNil(t, mitigator)

	// Verify internal structure
	impl, ok := mitigator.(*mitigatorImpl)
	require.True(t, ok)
	assert.Equal(t, mockMonitor, impl.monitor)
	assert.Equal(t, logger, impl.logger)
	assert.Equal(t, metricsScope, impl.metricsScope)
	assert.Equal(t, options, impl.options)

	// Verify handlers are properly initialized
	assert.NotNil(t, impl.handlers)
	assert.Len(t, impl.handlers, 1)
	_, exists := impl.handlers[AlertTypeQueuePendingTaskCount]
	assert.True(t, exists)
}

func TestMitigator_Mitigate_KnownAlertType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	logger := testlogger.New(t)
	metricsScope := metrics.NoopScope
	options := &MitigatorOptions{
		MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
	}

	mitigator := NewMitigator(
		mockVirtualQueueManager,
		mockMonitor,
		logger,
		metricsScope,
		options,
	)
	impl, ok := mitigator.(*mitigatorImpl)
	require.True(t, ok)
	handlerCalled := false
	impl.handlers[AlertTypeQueuePendingTaskCount] = func(alert Alert) {
		handlerCalled = true
	}

	alert := Alert{
		AlertType: AlertTypeQueuePendingTaskCount,
		AlertAttributesQueuePendingTaskCount: &AlertAttributesQueuePendingTaskCount{
			CurrentPendingTaskCount:  150,
			CriticalPendingTaskCount: 100,
		},
	}

	// Expect ResolveAlert to be called on the monitor
	mockMonitor.EXPECT().ResolveAlert(AlertTypeQueuePendingTaskCount).Times(1)

	mitigator.Mitigate(alert)
	assert.True(t, handlerCalled)
}

func TestMitigator_Mitigate_UnknownAlertType(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	logger := testlogger.New(t)
	metricsScope := metrics.NoopScope
	options := &MitigatorOptions{
		MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
	}

	mitigator := NewMitigator(
		mockVirtualQueueManager,
		mockMonitor,
		logger,
		metricsScope,
		options,
	)

	// Create an alert with an unknown/unhandled alert type
	unknownAlertType := AlertType(999)
	alert := Alert{
		AlertType: unknownAlertType,
	}

	mitigator.Mitigate(alert)
}

func TestMitigator_collectPendingTaskStats(t *testing.T) {
	tests := []struct {
		name                              string
		setupMocks                        func(*gomock.Controller) (*MockVirtualQueueManager, map[string][]VirtualSlice, map[VirtualSlice]map[string]int)
		expectedTotalPendingTaskCount     int
		expectedPendingTaskCountPerDomain map[string]int
		expectedSlicesPerDomainLength     map[string]int
		validateResults                   func(*testing.T, pendingTaskStats, map[string][]VirtualSlice, map[VirtualSlice]map[string]int)
	}{
		{
			name: "empty virtual queues",
			setupMocks: func(ctrl *gomock.Controller) (*MockVirtualQueueManager, map[string][]VirtualSlice, map[VirtualSlice]map[string]int) {
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
				mockVirtualQueueManager.EXPECT().VirtualQueues().Return(map[int64]VirtualQueue{}).Times(1)
				return mockVirtualQueueManager, map[string][]VirtualSlice{}, map[VirtualSlice]map[string]int{}
			},
			expectedTotalPendingTaskCount:     0,
			expectedPendingTaskCountPerDomain: map[string]int{},
			expectedSlicesPerDomainLength:     map[string]int{},
			validateResults: func(t *testing.T, stats pendingTaskStats, expectedSlicesPerDomain map[string][]VirtualSlice, expectedPendingTaskCountPerDomainPerSlice map[VirtualSlice]map[string]int) {
				assert.Empty(t, stats.pendingTaskCountPerDomain)
				assert.Empty(t, stats.pendingTaskCountPerDomainPerSlice)
				assert.Empty(t, stats.slicesPerDomain)
				assert.Empty(t, stats.pendingTaskCountPerDomainPerSlice)
			},
		},
		{
			name: "single queue single slice",
			setupMocks: func(ctrl *gomock.Controller) (*MockVirtualQueueManager, map[string][]VirtualSlice, map[VirtualSlice]map[string]int) {
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
				mockVirtualQueue := NewMockVirtualQueue(ctrl)
				mockVirtualSlice := NewMockVirtualSlice(ctrl)

				virtualQueues := map[int64]VirtualQueue{0: mockVirtualQueue}
				pendingTaskStats := PendingTaskStats{
					PendingTaskCountPerDomain: map[string]int{
						"domain1": 10,
						"domain2": 5,
					},
				}

				mockVirtualQueueManager.EXPECT().VirtualQueues().Return(virtualQueues).Times(1)
				mockVirtualQueue.EXPECT().IterateSlices(gomock.Any()).DoAndReturn(func(f func(VirtualSlice)) {
					f(mockVirtualSlice)
				}).Times(1)
				mockVirtualSlice.EXPECT().PendingTaskStats().Return(pendingTaskStats).Times(1)
				mockVirtualSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				expectedSlicesPerDomain := map[string][]VirtualSlice{
					"domain1": {mockVirtualSlice},
					"domain2": {mockVirtualSlice},
				}

				expectedPendingTaskCountPerDomainPerSlice := map[VirtualSlice]map[string]int{
					mockVirtualSlice: {
						"domain1": 10,
						"domain2": 5,
					},
				}

				return mockVirtualQueueManager, expectedSlicesPerDomain, expectedPendingTaskCountPerDomainPerSlice
			},
			expectedTotalPendingTaskCount: 15,
			expectedPendingTaskCountPerDomain: map[string]int{
				"domain1": 10,
				"domain2": 5,
			},
			expectedSlicesPerDomainLength: map[string]int{
				"domain1": 1,
				"domain2": 1,
			},
			validateResults: func(t *testing.T, stats pendingTaskStats, expectedSlicesPerDomain map[string][]VirtualSlice, expectedPendingTaskCountPerDomainPerSlice map[VirtualSlice]map[string]int) {
				assert.Equal(t, expectedSlicesPerDomain, stats.slicesPerDomain)
				assert.Equal(t, expectedPendingTaskCountPerDomainPerSlice, stats.pendingTaskCountPerDomainPerSlice)
			},
		},
		{
			name: "multiple queues multiple slices",
			setupMocks: func(ctrl *gomock.Controller) (*MockVirtualQueueManager, map[string][]VirtualSlice, map[VirtualSlice]map[string]int) {
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
				mockVirtualQueue1 := NewMockVirtualQueue(ctrl)
				mockVirtualQueue2 := NewMockVirtualQueue(ctrl)
				mockVirtualSlice1 := NewMockVirtualSlice(ctrl)
				mockVirtualSlice2 := NewMockVirtualSlice(ctrl)
				mockVirtualSlice3 := NewMockVirtualSlice(ctrl)

				virtualQueues := map[int64]VirtualQueue{
					0: mockVirtualQueue1,
					1: mockVirtualQueue2,
				}

				pendingTaskStats1 := PendingTaskStats{
					PendingTaskCountPerDomain: map[string]int{
						"domain1": 10,
						"domain2": 5,
					},
				}
				pendingTaskStats2 := PendingTaskStats{
					PendingTaskCountPerDomain: map[string]int{
						"domain1": 8,
						"domain3": 3,
					},
				}
				pendingTaskStats3 := PendingTaskStats{
					PendingTaskCountPerDomain: map[string]int{
						"domain2": 7,
						"domain3": 4,
					},
				}

				mockVirtualQueueManager.EXPECT().VirtualQueues().Return(virtualQueues).Times(1)
				mockVirtualQueue1.EXPECT().IterateSlices(gomock.Any()).DoAndReturn(func(f func(VirtualSlice)) {
					f(mockVirtualSlice1)
					f(mockVirtualSlice2)
				}).Times(1)
				mockVirtualQueue2.EXPECT().IterateSlices(gomock.Any()).DoAndReturn(func(f func(VirtualSlice)) {
					f(mockVirtualSlice3)
				}).Times(1)

				mockVirtualSlice1.EXPECT().PendingTaskStats().Return(pendingTaskStats1).Times(1)
				mockVirtualSlice2.EXPECT().PendingTaskStats().Return(pendingTaskStats2).Times(1)
				mockVirtualSlice3.EXPECT().PendingTaskStats().Return(pendingTaskStats3).Times(1)

				mockVirtualSlice1.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(0),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mockVirtualSlice2.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(200),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(300),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()
				mockVirtualSlice3.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(400),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				// Expected slices sorted by InclusiveMinTaskKey in descending order
				// slice1=0, slice2=200, slice3=100 -> sorted: slice2(200), slice3(100), slice1(0)
				expectedSlicesPerDomain := map[string][]VirtualSlice{
					"domain1": {mockVirtualSlice2, mockVirtualSlice1}, // slice2[200] > slice1[0]
					"domain2": {mockVirtualSlice3, mockVirtualSlice1}, // slice3[100] > slice1[0]
					"domain3": {mockVirtualSlice2, mockVirtualSlice3}, // slice2[200] > slice3[100]
				}

				expectedPendingTaskCountPerDomainPerSlice := map[VirtualSlice]map[string]int{
					mockVirtualSlice1: {
						"domain1": 10,
						"domain2": 5,
					},
					mockVirtualSlice2: {
						"domain1": 8,
						"domain3": 3,
					},
					mockVirtualSlice3: {
						"domain2": 7,
						"domain3": 4,
					},
				}

				return mockVirtualQueueManager, expectedSlicesPerDomain, expectedPendingTaskCountPerDomainPerSlice
			},
			expectedTotalPendingTaskCount: 37, // 10+5 + 8+3 + 7+4 = 37
			expectedPendingTaskCountPerDomain: map[string]int{
				"domain1": 18, // 10+8
				"domain2": 12, // 5+7
				"domain3": 7,  // 3+4
			},
			expectedSlicesPerDomainLength: map[string]int{
				"domain1": 2, // slice1, slice2
				"domain2": 2, // slice1, slice3
				"domain3": 2, // slice2, slice3
			},
			validateResults: func(t *testing.T, stats pendingTaskStats, expectedSlicesPerDomain map[string][]VirtualSlice, expectedPendingTaskCountPerDomainPerSlice map[VirtualSlice]map[string]int) {
				// Verify the exact slice order
				assert.Equal(t, expectedSlicesPerDomain, stats.slicesPerDomain)
				// Verify the pendingTaskCountPerDomainPerSlice
				assert.Equal(t, expectedPendingTaskCountPerDomainPerSlice, stats.pendingTaskCountPerDomainPerSlice)
			},
		},
		{
			name: "empty slices",
			setupMocks: func(ctrl *gomock.Controller) (*MockVirtualQueueManager, map[string][]VirtualSlice, map[VirtualSlice]map[string]int) {
				mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
				mockVirtualQueue := NewMockVirtualQueue(ctrl)
				mockVirtualSlice := NewMockVirtualSlice(ctrl)

				virtualQueues := map[int64]VirtualQueue{0: mockVirtualQueue}
				emptyPendingTaskStats := PendingTaskStats{
					PendingTaskCountPerDomain: map[string]int{},
				}

				mockVirtualQueueManager.EXPECT().VirtualQueues().Return(virtualQueues).Times(1)
				mockVirtualQueue.EXPECT().IterateSlices(gomock.Any()).DoAndReturn(func(f func(VirtualSlice)) {
					f(mockVirtualSlice)
				}).Times(1)
				mockVirtualSlice.EXPECT().PendingTaskStats().Return(emptyPendingTaskStats).Times(1)
				mockVirtualSlice.EXPECT().GetState().Return(VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(0),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
					},
					Predicate: NewUniversalPredicate(),
				}).AnyTimes()

				expectedSlicesPerDomain := map[string][]VirtualSlice{}
				expectedPendingTaskCountPerDomainPerSlice := map[VirtualSlice]map[string]int{
					mockVirtualSlice: {},
				}

				return mockVirtualQueueManager, expectedSlicesPerDomain, expectedPendingTaskCountPerDomainPerSlice
			},
			expectedTotalPendingTaskCount:     0,
			expectedPendingTaskCountPerDomain: map[string]int{},
			expectedSlicesPerDomainLength:     map[string]int{},
			validateResults: func(t *testing.T, stats pendingTaskStats, expectedSlicesPerDomain map[string][]VirtualSlice, expectedPendingTaskCountPerDomainPerSlice map[VirtualSlice]map[string]int) {
				assert.Empty(t, stats.slicesPerDomain)
				assert.Equal(t, expectedPendingTaskCountPerDomainPerSlice, stats.pendingTaskCountPerDomainPerSlice)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockMonitor := NewMockMonitor(ctrl)
			logger := testlogger.New(t)
			metricsScope := metrics.NoopScope
			options := &MitigatorOptions{
				MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
			}

			mockVirtualQueueManager, expectedSlicesPerDomain, expectedPendingTaskCountPerDomainPerSlice := tt.setupMocks(ctrl)

			mitigator := NewMitigator(
				mockVirtualQueueManager,
				mockMonitor,
				logger,
				metricsScope,
				options,
			)

			impl, ok := mitigator.(*mitigatorImpl)
			require.True(t, ok)
			impl.virtualQueueManager = mockVirtualQueueManager

			stats := impl.collectPendingTaskStats()

			// Verify basic aggregated data
			assert.Equal(t, tt.expectedTotalPendingTaskCount, stats.totalPendingTaskCount)
			assert.Equal(t, tt.expectedPendingTaskCountPerDomain, stats.pendingTaskCountPerDomain)

			// Verify slices per domain lengths
			for domain, expectedLength := range tt.expectedSlicesPerDomainLength {
				assert.Len(t, stats.slicesPerDomain[domain], expectedLength, "domain: %s", domain)
			}

			// Run custom validation if provided
			if tt.validateResults != nil {
				tt.validateResults(t, stats, expectedSlicesPerDomain, expectedPendingTaskCountPerDomainPerSlice)
			}
		})
	}
}

func TestMitigator_findDomainsToClear(t *testing.T) {
	tests := []struct {
		name        string
		setupStats  func(*gomock.Controller) (pendingTaskStats, map[VirtualSlice][]string)
		targetCount int
	}{
		{
			name: "target count zero - clear everything",
			setupStats: func(ctrl *gomock.Controller) (pendingTaskStats, map[VirtualSlice][]string) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockSlice3 := NewMockVirtualSlice(ctrl)
				stats := pendingTaskStats{
					totalPendingTaskCount: 142,
					pendingTaskCountPerDomain: map[string]int{
						"domain1": 35, // higher priority
						"domain2": 45,
						"domain3": 62,
					},
					pendingTaskCountPerDomainPerSlice: map[VirtualSlice]map[string]int{
						mockSlice1: {"domain1": 21, "domain2": 34, "domain3": 55},
						mockSlice2: {"domain1": 13, "domain2": 8, "domain3": 5},
						mockSlice3: {"domain1": 1, "domain2": 3, "domain3": 2},
					},
					slicesPerDomain: map[string][]VirtualSlice{
						"domain1": {mockSlice1, mockSlice2, mockSlice3},
						"domain2": {mockSlice2, mockSlice3, mockSlice1},
						"domain3": {mockSlice3, mockSlice1, mockSlice2},
					},
				}
				expectedResult := map[VirtualSlice][]string{
					mockSlice1: {"domain3", "domain1", "domain2"},
					mockSlice2: {"domain2", "domain1", "domain3"},
					mockSlice3: {"domain3", "domain2", "domain1"},
				}
				return stats, expectedResult
			},
			targetCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockMonitor := NewMockMonitor(ctrl)
			logger := testlogger.New(t)
			metricsScope := metrics.NoopScope
			options := &MitigatorOptions{
				MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
			}

			mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
			mitigator := NewMitigator(
				mockVirtualQueueManager,
				mockMonitor,
				logger,
				metricsScope,
				options,
			)

			impl, ok := mitigator.(*mitigatorImpl)
			require.True(t, ok)

			stats, expectedResult := tt.setupStats(ctrl)
			result := impl.findDomainsToClear(stats, tt.targetCount)

			// Verify the actual result content
			assert.Equal(t, expectedResult, result, "Result should contain the expected slice-to-domains mapping")
		})
	}
}

func TestMitigator_processQueueSplitsAndClear(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*gomock.Controller) (map[int64]VirtualQueue, map[VirtualSlice][]string, int)
	}{
		{
			name: "no domains to clear - no operations",
			setupMocks: func(ctrl *gomock.Controller) (map[int64]VirtualQueue, map[VirtualSlice][]string, int) {
				mockVQ := NewMockVirtualQueue(ctrl)

				// SplitSlices is still called, but the split function should return false for all slices
				mockVQ.EXPECT().SplitSlices(gomock.Any()).Do(func(splitFunc func(VirtualSlice) ([]VirtualSlice, bool)) {
					// Since domainsToClear is empty, any slice should return split=false
					mockSlice := NewMockVirtualSlice(nil)
					remaining, split := splitFunc(mockSlice)
					require.False(t, split)
					require.Nil(t, remaining)
				}).Times(1)

				virtualQueues := map[int64]VirtualQueue{
					0: mockVQ,
				}
				domainsToClear := map[VirtualSlice][]string{}
				maxQueueCount := 3
				return virtualQueues, domainsToClear, maxQueueCount
			},
		},
		{
			name: "clear slices in last queue - no splitting",
			setupMocks: func(ctrl *gomock.Controller) (map[int64]VirtualQueue, map[VirtualSlice][]string, int) {
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockVQ := NewMockVirtualQueue(ctrl)

				// Should call ClearSlices with a predicate function
				mockVQ.EXPECT().ClearSlices(gomock.Any()).Do(func(predicate func(VirtualSlice) bool) {
					// Verify the predicate returns true for slices in domainsToClear
					clearedCount := 0
					for _, slice := range []VirtualSlice{mockSlice1, mockSlice2} {
						if predicate(slice) {
							clearedCount++
						}
					}
					// Since both slices are in domainsToClear, both should be cleared
					require.Equal(t, 1, clearedCount)
				}).Times(1)

				// Should pause the queue after clearing
				mockVQ.EXPECT().Pause(clearSliceThrottleDuration).Times(1)

				virtualQueues := map[int64]VirtualQueue{
					2: mockVQ, // queueID >= maxQueueCount-1 (2)
				}
				domainsToClear := map[VirtualSlice][]string{
					mockSlice1: {"domain1"},
				}
				maxQueueCount := 3
				return virtualQueues, domainsToClear, maxQueueCount
			},
		},
		{
			name: "split and move slices - successful split",
			setupMocks: func(ctrl *gomock.Controller) (map[int64]VirtualQueue, map[VirtualSlice][]string, int) {
				mockSlice := NewMockVirtualSlice(ctrl)
				mockSplitSlice := NewMockVirtualSlice(ctrl)
				mockRemainingSlice := NewMockVirtualSlice(ctrl)
				mockVQ := NewMockVirtualQueue(ctrl)

				// Set up split behavior
				mockSlice.EXPECT().TrySplitByPredicate(gomock.Any()).Return(mockSplitSlice, mockRemainingSlice, true).AnyTimes()
				mockSplitSlice.EXPECT().Clear().AnyTimes()

				// Should call SplitSlices with a function
				mockVQ.EXPECT().SplitSlices(gomock.Any()).Do(func(splitFunc func(VirtualSlice) ([]VirtualSlice, bool)) {
					// Call the split function with our mock slice
					remaining, split := splitFunc(mockSlice)
					// Should return split=true and remaining slice
					require.True(t, split)
					require.Len(t, remaining, 1)
				}).Times(1)

				virtualQueues := map[int64]VirtualQueue{
					0: mockVQ, // queueID < maxQueueCount-1
				}
				domainsToClear := map[VirtualSlice][]string{
					mockSlice: {"domain1", "domain2"},
				}
				maxQueueCount := 3

				return virtualQueues, domainsToClear, maxQueueCount
			},
		},
		{
			name: "clear slice when split fails",
			setupMocks: func(ctrl *gomock.Controller) (map[int64]VirtualQueue, map[VirtualSlice][]string, int) {
				mockSlice := NewMockVirtualSlice(ctrl)
				mockVQ := NewMockVirtualQueue(ctrl)

				// Set up split behavior to fail
				mockSlice.EXPECT().TrySplitByPredicate(gomock.Any()).Return(nil, nil, false).AnyTimes()
				mockSlice.EXPECT().Clear().AnyTimes()

				mockVQ.EXPECT().SplitSlices(gomock.Any()).Do(func(splitFunc func(VirtualSlice) ([]VirtualSlice, bool)) {
					remaining, split := splitFunc(mockSlice)
					require.True(t, split)
					require.Nil(t, remaining)
				}).Times(1)

				virtualQueues := map[int64]VirtualQueue{
					1: mockVQ,
				}
				domainsToClear := map[VirtualSlice][]string{
					mockSlice: {"domain1"},
				}
				maxQueueCount := 4

				return virtualQueues, domainsToClear, maxQueueCount
			},
		},
		{
			name: "multiple queues with mixed operations",
			setupMocks: func(ctrl *gomock.Controller) (map[int64]VirtualQueue, map[VirtualSlice][]string, int) {
				// Queue 0: split and move
				mockSlice1 := NewMockVirtualSlice(ctrl)
				mockSplitSlice1 := NewMockVirtualSlice(ctrl)
				mockRemainingSlice1 := NewMockVirtualSlice(ctrl)
				mockVQ0 := NewMockVirtualQueue(ctrl)

				// Queue 1: split and move
				mockSlice2 := NewMockVirtualSlice(ctrl)
				mockSplitSlice2 := NewMockVirtualSlice(ctrl)
				mockRemainingSlice2 := NewMockVirtualSlice(ctrl)
				mockVQ1 := NewMockVirtualQueue(ctrl)

				// Queue 2: clear (last queue)
				mockSlice3 := NewMockVirtualSlice(ctrl)
				mockVQ2 := NewMockVirtualQueue(ctrl)

				// Set up split behaviors
				mockSlice1.EXPECT().TrySplitByPredicate(gomock.Any()).Return(mockSplitSlice1, mockRemainingSlice1, true).AnyTimes()
				mockSplitSlice1.EXPECT().Clear().AnyTimes()

				mockSlice2.EXPECT().TrySplitByPredicate(gomock.Any()).Return(mockSplitSlice2, mockRemainingSlice2, true).AnyTimes()
				mockSplitSlice2.EXPECT().Clear().AnyTimes()

				// Queue 0: should split
				mockVQ0.EXPECT().SplitSlices(gomock.Any()).Times(1)

				// Queue 1: should split
				mockVQ1.EXPECT().SplitSlices(gomock.Any()).Times(1)

				// Queue 2: should clear (last queue)
				var cleared bool
				mockVQ2.EXPECT().ClearSlices(gomock.Any()).Do(func(predicate func(VirtualSlice) bool) {
					// Simulate that mockSlice3 exists in the queue and is checked
					if predicate(mockSlice3) {
						cleared = true
					}
				}).Times(1)

				// Pause should only be called if cleared is true
				mockVQ2.EXPECT().Pause(clearSliceThrottleDuration).Do(func(duration interface{}) {
					require.True(t, cleared, "Pause should only be called if slices were cleared")
				}).Times(1)

				virtualQueues := map[int64]VirtualQueue{
					0: mockVQ0,
					1: mockVQ1,
					2: mockVQ2, // Last queue (>= maxQueueCount-1)
				}
				domainsToClear := map[VirtualSlice][]string{
					mockSlice1: {"domain1"},
					mockSlice2: {"domain2"},
					mockSlice3: {"domain3"},
				}
				maxQueueCount := 3

				return virtualQueues, domainsToClear, maxQueueCount
			},
		},
		{
			name: "empty domains to clear for some slices",
			setupMocks: func(ctrl *gomock.Controller) (map[int64]VirtualQueue, map[VirtualSlice][]string, int) {
				mockSlice1 := NewMockVirtualSlice(ctrl) // Has domains to clear
				mockSplitSlice1 := NewMockVirtualSlice(ctrl)
				mockRemainingSlice1 := NewMockVirtualSlice(ctrl)
				mockVQ := NewMockVirtualQueue(ctrl)

				// Set up split behavior for slice that has domains to clear
				mockSlice1.EXPECT().TrySplitByPredicate(gomock.Any()).Return(mockSplitSlice1, mockRemainingSlice1, true).AnyTimes()
				mockSplitSlice1.EXPECT().Clear().AnyTimes()

				mockVQ.EXPECT().SplitSlices(gomock.Any()).Do(func(splitFunc func(VirtualSlice) ([]VirtualSlice, bool)) {
					// Test with slice that has domains to clear
					remaining, split := splitFunc(mockSlice1)
					require.True(t, split)
					require.Len(t, remaining, 1)

					// Test with slice that has no domains to clear
					mockSlice2 := NewMockVirtualSlice(nil)
					remaining, split = splitFunc(mockSlice2)
					require.False(t, split)
					require.Nil(t, remaining)
				}).Times(1)

				virtualQueues := map[int64]VirtualQueue{
					0: mockVQ,
				}
				domainsToClear := map[VirtualSlice][]string{
					mockSlice1: {"domain1"},
					// mockSlice2 intentionally not in domainsToClear
				}
				maxQueueCount := 3

				return virtualQueues, domainsToClear, maxQueueCount
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockMonitor := NewMockMonitor(ctrl)
			logger := testlogger.New(t)
			metricsScope := metrics.NoopScope
			options := &MitigatorOptions{
				MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(3),
			}

			virtualQueues, domainsToClear, maxQueueCount := tt.setupMocks(ctrl)
			options.MaxVirtualQueueCount = dynamicproperties.GetIntPropertyFn(maxQueueCount)

			// Create a mock VirtualQueueManager that returns our test queues
			mockVQManager := NewMockVirtualQueueManager(ctrl)

			// Set up expectations for GetOrCreateVirtualQueue calls if needed
			for queueID := range virtualQueues {
				if queueID < int64(maxQueueCount-1) {
					// Expect calls to create next queues for moving slices
					nextQueueID := queueID + 1
					if _, exists := virtualQueues[nextQueueID]; !exists {
						mockNextVQ := NewMockVirtualQueue(ctrl)
						mockNextVQ.EXPECT().Pause(clearSliceThrottleDuration).AnyTimes()
						mockNextVQ.EXPECT().MergeSlices(gomock.Any()).AnyTimes()
						mockVQManager.EXPECT().GetOrCreateVirtualQueue(nextQueueID).Return(mockNextVQ).AnyTimes()
					}
				}
			}

			mitigator := &mitigatorImpl{
				virtualQueueManager: mockVQManager,
				monitor:             mockMonitor,
				logger:              logger,
				metricsScope:        metricsScope,
				options:             options,
			}

			// Execute the method
			mitigator.processQueueSplitsAndClear(virtualQueues, domainsToClear)
		})
	}
}
