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

package matching

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestPollerWeight(t *testing.T) {
	n := 4
	pw := newWeightSelector(n, 100)
	// uninitialized weights should return -1
	p, cumulativeWeights := pw.pick()
	assert.Equal(t, -1, p)
	assert.Equal(t, []int64{0, 0, 0, 0}, cumulativeWeights)
	// all 0 weights should return -1
	for i := 0; i < n; i++ {
		pw.update(n, i, 0)
		p, cumulativeWeights := pw.pick()
		assert.Equal(t, -1, p)
		assert.Equal(t, []int64{0, 0, 0, 0}, cumulativeWeights)
	}
	// if only one item has non-zero weight, always pick that item
	pw.update(n, 3, 400)
	for i := 0; i < 100; i++ {
		p, cumulativeWeights := pw.pick()
		assert.Equal(t, 3, p)
		assert.Equal(t, []int64{0, 0, 0, 400}, cumulativeWeights)
	}
	pw.update(n, 2, 300)
	pw.update(n, 1, 200)
	pw.update(n, 0, 100)
	// test pick probabilities
	testPickProbHelper(t, pw, time.Now().UnixNano())

	// shrink size and test pick probabilities
	pw.update(n-1, 2, 200)
	testPickProbHelper(t, pw, time.Now().UnixNano())

	// expand size and test pick probabilities
	pw.update(n, 3, 300)
	pw.update(n+1, 4, 400)
	testPickProbHelper(t, pw, time.Now().UnixNano())

	// no panics
	pw.update(n, 4, 0)
}

func testPickProbHelper(t *testing.T, pw *weightSelector, seed int64) {
	t.Helper()
	rand.Seed(seed)
	// Collect pick results
	results := make(map[int]int)
	numPicks := 1000000
	for i := 0; i < numPicks; i++ {
		index, _ := pw.pick()
		results[index]++
	}
	// Calculate expected probabilities
	totalWeight := int64(0)
	for _, w := range pw.weights {
		totalWeight += w
	}
	expectedProbs := make([]float64, len(pw.weights))
	for i, w := range pw.weights {
		expectedProbs[i] = float64(w) / float64(totalWeight)
	}
	// Check that pick results are approximately proportional to weights
	for i := 0; i < len(pw.weights); i++ {
		expectedCount := expectedProbs[i] * float64(numPicks)
		actualCount := float64(results[i])
		delta := expectedCount * 0.02 // Allow 2% error margin
		if actualCount < expectedCount-delta || actualCount > expectedCount+delta {
			t.Errorf("Index %d: expected count approximately %.0f, got %d", i, expectedCount, results[i])
		}
	}
}

func TestNewWeightedLoadBalancer(t *testing.T) {
	ctrl := gomock.NewController(t)
	roundRobinMock := NewMockLoadBalancer(ctrl)
	p := NewMockPartitionConfigProvider(ctrl)
	p.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	logger := testlogger.New(t)
	p.EXPECT().GetLogger().Return(logger).AnyTimes()
	lb := NewWeightedLoadBalancer(roundRobinMock, p, logger)
	assert.NotNil(t, lb)
	weightedLB, ok := lb.(*weightedLoadBalancer)
	assert.NotNil(t, weightedLB)
	assert.True(t, ok)
	assert.Equal(t, roundRobinMock, weightedLB.fallbackLoadBalancer)
	assert.Equal(t, p, weightedLB.provider)
	assert.NotNil(t, weightedLB.weightCache)
	assert.NotNil(t, weightedLB.logger)
}

func TestWeightedLoadBalancer_PickWritePartition(t *testing.T) {
	testCases := []struct {
		name           string
		domainID       string
		taskList       types.TaskList
		taskListType   int
		forwardedFrom  string
		expectedResult string
		setupMock      func(m *MockLoadBalancer)
	}{
		{
			name:     "Basic case",
			domainID: "domainA",
			taskList: types.TaskList{Name: "taskListA"},
			setupMock: func(m *MockLoadBalancer) {
				req := &types.AddDecisionTaskRequest{
					DomainUUID:    "domainA",
					TaskList:      &types.TaskList{Name: "taskListA"},
					ForwardedFrom: "",
				}
				m.EXPECT().
					PickWritePartition(0, req).
					Return("partitionA")
			},
			expectedResult: "partitionA",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockFallbackLB := NewMockLoadBalancer(ctrl)
			if tc.setupMock != nil {
				tc.setupMock(mockFallbackLB)
			}

			lb := &weightedLoadBalancer{
				fallbackLoadBalancer: mockFallbackLB,
			}

			req := &types.AddDecisionTaskRequest{
				DomainUUID:    tc.domainID,
				TaskList:      &tc.taskList,
				ForwardedFrom: tc.forwardedFrom,
			}

			result := lb.PickWritePartition(tc.taskListType, req)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestWeightedLoadBalancer_PickReadPartition(t *testing.T) {
	testCases := []struct {
		name               string
		domainID           string
		taskList           types.TaskList
		taskListType       int
		forwardedFrom      string
		weightCacheReturn  interface{}
		fallbackReturn     string
		expectedResult     string
		expectFallbackCall bool
	}{
		{
			name:               "WeightCache returns nil",
			domainID:           "domainA",
			taskList:           types.TaskList{Name: "taskListA"},
			weightCacheReturn:  nil,
			fallbackReturn:     "fallbackPartition",
			expectedResult:     "fallbackPartition",
			expectFallbackCall: true,
		},
		{
			name:               "WeightCache returns invalid type",
			domainID:           "domainB",
			taskList:           types.TaskList{Name: "taskListB"},
			weightCacheReturn:  "invalidType",
			fallbackReturn:     "fallbackPartition",
			expectedResult:     "fallbackPartition",
			expectFallbackCall: true,
		},
		{
			name:               "WeightSelector pick returns negative",
			domainID:           "domainC",
			taskList:           types.TaskList{Name: "taskListC"},
			weightCacheReturn:  newWeightSelector(2, 100),
			fallbackReturn:     "fallbackPartition",
			expectedResult:     "fallbackPartition",
			expectFallbackCall: true,
		},
		{
			name:     "WeightSelector pick returns non-negative",
			domainID: "domainD",
			taskList: types.TaskList{Name: "taskListD"},
			weightCacheReturn: func() *weightSelector {
				pw := newWeightSelector(2, 10)
				pw.update(2, 0, 0)
				pw.update(2, 1, 11)
				return pw
			}(),
			expectedResult:     getPartitionTaskListName("taskListD", 1),
			expectFallbackCall: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &types.AddDecisionTaskRequest{
				DomainUUID:    tc.domainID,
				TaskList:      &tc.taskList,
				ForwardedFrom: tc.forwardedFrom,
			}
			ctrl := gomock.NewController(t)
			// Create mocks.
			mockWeightCache := cache.NewMockCache(ctrl)
			mockFallbackLoadBalancer := NewMockLoadBalancer(ctrl)

			// Set up the mocks.
			taskListKey := key{
				domainID:     tc.domainID,
				taskListName: tc.taskList.GetName(),
				taskListType: tc.taskListType,
			}
			mockWeightCache.EXPECT().
				Get(taskListKey).
				Return(tc.weightCacheReturn)

			if tc.expectFallbackCall {
				mockFallbackLoadBalancer.EXPECT().
					PickReadPartition(tc.taskListType, req, "").
					Return(tc.fallbackReturn)
			}

			logger := testlogger.New(t)

			// Create the weightedLoadBalancer instance.
			lb := &weightedLoadBalancer{
				weightCache:          mockWeightCache,
				fallbackLoadBalancer: mockFallbackLoadBalancer,
				logger:               logger,
			}

			// Call the method under test.
			result := lb.PickReadPartition(tc.taskListType, req, "")

			// Assert the result.
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestWeightedLoadBalancer_UpdateWeight(t *testing.T) {
	testCases := []struct {
		name              string
		domainID          string
		taskList          types.TaskList
		taskListType      int
		forwardedFrom     string
		partition         string
		loadBalancerHints *types.LoadBalancerHints
		setupMock         func(*cache.MockCache, *MockPartitionConfigProvider)
	}{
		{
			name:     "Sticky task list",
			domainID: "domainA",
			taskList: types.TaskList{Name: "a", Kind: types.TaskListKindSticky.Ptr()},
		},
		{
			name:          "forwarded request",
			domainID:      "domainA",
			taskList:      types.TaskList{Name: "a"},
			forwardedFrom: "tasklist",
		},
		{
			name:     "partitioned task list",
			domainID: "domainA",
			taskList: types.TaskList{Name: "/__cadence_sys/aaa/1"},
		},
		{
			name:     "nil loadBalancerHints",
			domainID: "domainA",
			taskList: types.TaskList{Name: "a"},
		},
		{
			name:      "1 partition",
			domainID:  "domainA",
			taskList:  types.TaskList{Name: "a"},
			partition: "a",
			loadBalancerHints: &types.LoadBalancerHints{
				BacklogCount: 1,
			},
			setupMock: func(mockCache *cache.MockCache, mockPartitionConfigProvider *MockPartitionConfigProvider) {
				mockPartitionConfigProvider.EXPECT().GetNumberOfReadPartitions("domainA", types.TaskList{Name: "a"}, 0).Return(1)
				mockCache.EXPECT().Delete(key{
					domainID:     "domainA",
					taskListName: "a",
					taskListType: 0,
				})
			},
		},
		{
			name:      "partition 0",
			domainID:  "domainA",
			taskList:  types.TaskList{Name: "a"},
			partition: "a",
			loadBalancerHints: &types.LoadBalancerHints{
				BacklogCount: 1,
			},
			setupMock: func(mockCache *cache.MockCache, mockPartitionConfigProvider *MockPartitionConfigProvider) {
				mockPartitionConfigProvider.EXPECT().GetNumberOfReadPartitions("domainA", types.TaskList{Name: "a"}, 0).Return(2)
				mockCache.EXPECT().Get(key{
					domainID:     "domainA",
					taskListName: "a",
					taskListType: 0,
				}).Return(nil)
				mockCache.EXPECT().PutIfNotExist(key{
					domainID:     "domainA",
					taskListName: "a",
					taskListType: 0,
				}, newWeightSelector(2, 100)).Return(newWeightSelector(2, 100), nil)
			},
		},
		{
			name:      "partition 1",
			domainID:  "domainA",
			taskList:  types.TaskList{Name: "a"},
			partition: "/__cadence_sys/a/1",
			loadBalancerHints: &types.LoadBalancerHints{
				BacklogCount: 1,
			},
			setupMock: func(mockCache *cache.MockCache, mockPartitionConfigProvider *MockPartitionConfigProvider) {
				mockPartitionConfigProvider.EXPECT().GetNumberOfReadPartitions("domainA", types.TaskList{Name: "a"}, 0).Return(2)
				mockCache.EXPECT().Get(key{
					domainID:     "domainA",
					taskListName: "a",
					taskListType: 0,
				}).Return(newWeightSelector(2, 100))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &types.AddDecisionTaskRequest{
				DomainUUID:    tc.domainID,
				TaskList:      &tc.taskList,
				ForwardedFrom: tc.forwardedFrom,
			}
			ctrl := gomock.NewController(t)
			mockWeightCache := cache.NewMockCache(ctrl)
			mockPartitionConfigProvider := NewMockPartitionConfigProvider(ctrl)
			lb := &weightedLoadBalancer{
				weightCache: mockWeightCache,
				provider:    mockPartitionConfigProvider,
				logger:      testlogger.New(t),
			}
			if tc.setupMock != nil {
				tc.setupMock(mockWeightCache, mockPartitionConfigProvider)
			}

			lb.UpdateWeight(tc.taskListType, req, tc.partition, tc.loadBalancerHints)
		})
	}
}

func TestCalcWeightFromLoadBalancerHints(t *testing.T) {
	tests := []struct {
		name     string
		info     types.LoadBalancerHints
		expected int64
	}{
		{
			name:     "Zero QPS and backlog count",
			info:     types.LoadBalancerHints{BacklogCount: 0, RatePerSecond: 0},
			expected: 0,
		},
		{
			name:     "Small QPS below threshold",
			info:     types.LoadBalancerHints{BacklogCount: 10, RatePerSecond: 0.005},
			expected: 10,
		},
		{
			name:     "QPS above threshold with no backlog",
			info:     types.LoadBalancerHints{BacklogCount: 0, RatePerSecond: 2},
			expected: int64(math.Ceil(2 * 0.01)), // smoothingNumber calculation
		},
		{
			name:     "QPS above threshold with backlog",
			info:     types.LoadBalancerHints{BacklogCount: 100, RatePerSecond: 5},
			expected: 100 + int64(math.Ceil(5*0.01)), // backlog + smoothingNumber
		},
		{
			name:     "Large QPS",
			info:     types.LoadBalancerHints{BacklogCount: 50, RatePerSecond: 100},
			expected: 50 + int64(math.Ceil(100*0.01)), // backlog + smoothingNumber
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcWeightFromLoadBalancerHints(&tt.info)
			assert.Equal(t, tt.expected, result, "unexpected result for %s", tt.name)
		})
	}
}

func TestPickBetween(t *testing.T) {
	tl := types.TaskList{Name: "a"}
	cases := []struct {
		name    string
		hints   map[int]*types.LoadBalancerHints
		config  *types.TaskListPartitionConfig
		from    []int
		allowed []int
	}{
		{
			name:    "unknown TL",
			config:  createDefaultConfig(1, 1),
			from:    []int{0, 1, 2, 3},
			allowed: []int{-1},
		},
		{
			name:    "no partitions specified",
			config:  createDefaultConfig(1, 1),
			from:    []int{},
			allowed: []int{-1},
		},
		{
			name:    "single partition specified",
			config:  createDefaultConfig(1, 1),
			from:    []int{0},
			allowed: []int{0},
		},
		{
			name:    "single partition",
			config:  createDefaultConfig(1, 1),
			from:    []int{0, 1, 2, 3},
			allowed: []int{-1},
		},
		{
			name:    "not initialized",
			config:  createDefaultConfig(4, 4),
			from:    []int{0, 1, 2, 3},
			allowed: []int{-1},
		},
		{
			name:   "partially initialized",
			config: createDefaultConfig(4, 4),
			hints: map[int]*types.LoadBalancerHints{
				0: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				1: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				2: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
			},
			from:    []int{0, 1, 2, 3},
			allowed: []int{-1},
		},
		{
			name:   "fully initialized",
			config: createDefaultConfig(4, 4),
			hints: map[int]*types.LoadBalancerHints{
				0: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				1: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				2: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				3: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
			},
			from:    []int{0, 1, 2, 3},
			allowed: []int{0, 1, 2, 3},
		},
		{
			name:   "invalid partition",
			config: createDefaultConfig(4, 4),
			hints: map[int]*types.LoadBalancerHints{
				0: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				1: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				2: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				3: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
			},
			from:    []int{0, 1, 2, 4},
			allowed: []int{-1},
		},
		{
			name:   "only consider specified - below threshold",
			config: createDefaultConfig(4, 4),
			hints: map[int]*types.LoadBalancerHints{
				0: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				1: {
					BacklogCount:  1,
					RatePerSecond: 1,
				},
				2: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				3: {
					BacklogCount:  1,
					RatePerSecond: 1,
				},
			},
			from:    []int{1, 3},
			allowed: []int{-1},
		},
		{
			name:   "only consider specified - above threshold",
			config: createDefaultConfig(4, 4),
			hints: map[int]*types.LoadBalancerHints{
				0: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				1: {
					BacklogCount:  _backlogThreshold,
					RatePerSecond: 1,
				},
				2: {
					BacklogCount:  1000000,
					RatePerSecond: 100,
				},
				3: {
					BacklogCount:  1,
					RatePerSecond: 1,
				},
			},
			from:    []int{1, 3},
			allowed: []int{1, 3},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &types.AddDecisionTaskRequest{
				DomainUUID:    testDomain,
				TaskList:      &tl,
				ForwardedFrom: "",
			}
			ctrl := gomock.NewController(t)
			weightCache := cache.New(&cache.Options{
				TTL:             0,
				InitialCapacity: 100,
				Pin:             false,
				MaxCount:        3000,
				ActivelyEvict:   false,
				MetricsScope:    metrics.NoopScope,
				Logger:          testlogger.New(t),
			})
			mockPartitionConfigProvider := NewMockPartitionConfigProvider(ctrl)
			mockPartitionConfigProvider.EXPECT().GetNumberOfReadPartitions(testDomain, tl, 0).Return(len(tc.config.ReadPartitions)).Times(len(tc.hints))
			lb := &weightedLoadBalancer{
				weightCache: weightCache,
				provider:    mockPartitionConfigProvider,
				logger:      testlogger.New(t),
			}
			for partitionID, hint := range tc.hints {
				lb.UpdateWeight(0, req, getPartitionTaskListName(tl.Name, partitionID), hint)
			}
			actual := lb.PickBetween(testDomain, tl.Name, 0, tc.from)
			assert.Contains(t, tc.allowed, actual)
		})
	}
}
