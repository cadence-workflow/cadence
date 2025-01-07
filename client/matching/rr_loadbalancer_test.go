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
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/types"
)

func TestNewRoundRobinLoadBalancer(t *testing.T) {
	ctrl := gomock.NewController(t)
	p := NewMockPartitionConfigProvider(ctrl)
	lb := NewRoundRobinLoadBalancer(p)
	assert.NotNil(t, lb)
	rb, ok := lb.(*roundRobinLoadBalancer)
	assert.NotNil(t, rb)
	assert.True(t, ok)
	assert.Equal(t, p, rb.provider)
}

func TestPickPartition(t *testing.T) {
	tests := []struct {
		name           string
		domainID       string
		taskList       types.TaskList
		taskListType   int
		forwardedFrom  string
		nPartitions    int
		setupCache     func(mockCache *cache.MockCache)
		expectedResult string
	}{
		{
			name:           "ForwardedFrom is not empty",
			domainID:       "testDomain",
			taskList:       types.TaskList{Name: "testTaskList", Kind: types.TaskListKindNormal.Ptr()},
			taskListType:   1,
			forwardedFrom:  "otherDomain",
			nPartitions:    3,
			setupCache:     nil,
			expectedResult: "testTaskList",
		},
		{
			name:           "Sticky task list",
			domainID:       "testDomain",
			taskList:       types.TaskList{Name: "testTaskList", Kind: types.TaskListKindSticky.Ptr()},
			taskListType:   1,
			forwardedFrom:  "",
			nPartitions:    3,
			setupCache:     nil,
			expectedResult: "testTaskList",
		},
		{
			name:           "Reserved task list prefix",
			domainID:       "testDomain",
			taskList:       types.TaskList{Name: fmt.Sprintf("%vTest", common.ReservedTaskListPrefix), Kind: types.TaskListKindNormal.Ptr()},
			taskListType:   1,
			forwardedFrom:  "",
			nPartitions:    3,
			setupCache:     nil,
			expectedResult: fmt.Sprintf("%vTest", common.ReservedTaskListPrefix),
		},
		{
			name:           "nPartitions <= 1",
			domainID:       "testDomain",
			taskList:       types.TaskList{Name: "testTaskList", Kind: types.TaskListKindNormal.Ptr()},
			taskListType:   1,
			forwardedFrom:  "",
			nPartitions:    1,
			setupCache:     nil,
			expectedResult: "testTaskList",
		},
		{
			name:          "Cache miss and partitioned task list",
			domainID:      "testDomain",
			taskList:      types.TaskList{Name: "testTaskList", Kind: types.TaskListKindNormal.Ptr()},
			taskListType:  1,
			forwardedFrom: "",
			nPartitions:   3,
			setupCache: func(mockCache *cache.MockCache) {
				mockCache.EXPECT().Get(key{
					domainID:     "testDomain",
					taskListName: "testTaskList",
					taskListType: 1,
				}).Return(nil)
				mockCache.EXPECT().PutIfNotExist(key{
					domainID:     "testDomain",
					taskListName: "testTaskList",
					taskListType: 1,
				}, gomock.Any()).DoAndReturn(func(key key, val interface{}) (interface{}, error) {
					if *val.(*int64) != -1 {
						panic("Expected value to be -1")
					}
					return val, nil
				})
			},
			expectedResult: "testTaskList",
		},
		{
			name:          "Cache error and partitioned task list",
			domainID:      "testDomain",
			taskList:      types.TaskList{Name: "testTaskList", Kind: types.TaskListKindNormal.Ptr()},
			taskListType:  1,
			forwardedFrom: "",
			nPartitions:   3,
			setupCache: func(mockCache *cache.MockCache) {
				mockCache.EXPECT().Get(key{
					domainID:     "testDomain",
					taskListName: "testTaskList",
					taskListType: 1,
				}).Return(nil)
				mockCache.EXPECT().PutIfNotExist(key{
					domainID:     "testDomain",
					taskListName: "testTaskList",
					taskListType: 1,
				}, gomock.Any()).Return(nil, fmt.Errorf("cache error"))
			},
			expectedResult: "testTaskList",
		},
		{
			name:          "Cache hit and partitioned task list",
			domainID:      "testDomain",
			taskList:      types.TaskList{Name: "testTaskList", Kind: types.TaskListKindNormal.Ptr()},
			taskListType:  1,
			forwardedFrom: "",
			nPartitions:   3,
			setupCache: func(mockCache *cache.MockCache) {
				mockCache.EXPECT().Get(key{
					domainID:     "testDomain",
					taskListName: "testTaskList",
					taskListType: 1,
				}).Return(new(int64))
			},
			expectedResult: "/__cadence_sys/testTaskList/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCache := cache.NewMockCache(ctrl)

			// If the test requires setting up cache behavior, call setupCache
			if tt.setupCache != nil {
				tt.setupCache(mockCache)
			}

			// Call the pickPartition function
			result := pickPartition(
				tt.domainID,
				tt.taskList,
				tt.taskListType,
				tt.forwardedFrom,
				tt.nPartitions,
				mockCache,
			)

			// Assert that the result matches the expected result
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func setUpMocksForRoundRobinLoadBalancer(t *testing.T, pickPartitionFn func(domainID string, taskList types.TaskList, taskListType int, forwardedFrom string, nPartitions int, partitionCache cache.Cache) string) (*roundRobinLoadBalancer, *MockPartitionConfigProvider, *cache.MockCache) {
	ctrl := gomock.NewController(t)
	mockProvider := NewMockPartitionConfigProvider(ctrl)
	mockCache := cache.NewMockCache(ctrl)

	return &roundRobinLoadBalancer{
		provider:        mockProvider,
		readCache:       mockCache,
		writeCache:      mockCache,
		pickPartitionFn: pickPartitionFn,
	}, mockProvider, mockCache
}

func TestRoundRobinPickWritePartition(t *testing.T) {
	testCases := []struct {
		name              string
		forwardedFrom     string
		taskListType      int
		nPartitions       int
		taskListKind      types.TaskListKind
		expectedPartition string
	}{
		{
			name:              "single write partition, forwarded",
			forwardedFrom:     "parent-task-list",
			taskListType:      0,
			nPartitions:       1,
			taskListKind:      types.TaskListKindNormal,
			expectedPartition: "test-task-list",
		},
		{
			name:              "multiple write partitions, no forward",
			forwardedFrom:     "",
			taskListType:      0,
			nPartitions:       3,
			taskListKind:      types.TaskListKindNormal,
			expectedPartition: "custom-partition", // Simulated by fake pickPartitionFn
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fake pickPartitionFn behavior
			fakePickPartitionFn := func(domainID string, taskList types.TaskList, taskListType int, forwardedFrom string, nPartitions int, partitionCache cache.Cache) string {
				assert.Equal(t, "test-domain-id", domainID)
				assert.Equal(t, "test-task-list", taskList.Name)
				assert.Equal(t, tc.taskListKind, taskList.GetKind())
				assert.Equal(t, tc.taskListType, taskListType)
				assert.Equal(t, tc.forwardedFrom, forwardedFrom)
				assert.Equal(t, tc.nPartitions, nPartitions)
				if forwardedFrom != "" || taskList.GetKind() == types.TaskListKindSticky {
					return taskList.GetName()
				}
				return "custom-partition"
			}

			// Set up mocks with the fake pickPartitionFn
			loadBalancer, mockProvider, _ := setUpMocksForRoundRobinLoadBalancer(t, fakePickPartitionFn)

			mockProvider.EXPECT().
				GetNumberOfWritePartitions("test-domain-id", types.TaskList{Name: "test-task-list", Kind: &tc.taskListKind}, tc.taskListType).
				Return(tc.nPartitions).
				Times(1)

			kind := tc.taskListKind
			taskList := types.TaskList{Name: "test-task-list", Kind: &kind}
			partition := loadBalancer.PickWritePartition("test-domain-id", taskList, tc.taskListType, tc.forwardedFrom)

			// Validate result
			assert.Equal(t, tc.expectedPartition, partition)
		})
	}
}

func TestRoundRobinPickReadPartition(t *testing.T) {
	testCases := []struct {
		name              string
		forwardedFrom     string
		taskListType      int
		nPartitions       int
		taskListKind      types.TaskListKind
		expectedPartition string
	}{
		{
			name:              "single read partition, forwarded",
			forwardedFrom:     "parent-task-list",
			taskListType:      0,
			nPartitions:       1,
			taskListKind:      types.TaskListKindNormal,
			expectedPartition: "test-task-list",
		},
		{
			name:              "multiple read partitions, no forward",
			forwardedFrom:     "",
			taskListType:      0,
			nPartitions:       3,
			taskListKind:      types.TaskListKindNormal,
			expectedPartition: "custom-partition", // Simulated by fake pickPartitionFn
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fake pickPartitionFn behavior
			fakePickPartitionFn := func(domainID string, taskList types.TaskList, taskListType int, forwardedFrom string, nPartitions int, partitionCache cache.Cache) string {
				assert.Equal(t, "test-domain-id", domainID)
				assert.Equal(t, "test-task-list", taskList.Name)
				assert.Equal(t, tc.taskListKind, taskList.GetKind())
				assert.Equal(t, tc.taskListType, taskListType)
				assert.Equal(t, tc.forwardedFrom, forwardedFrom)
				assert.Equal(t, tc.nPartitions, nPartitions)
				if forwardedFrom != "" || taskList.GetKind() == types.TaskListKindSticky {
					return taskList.GetName()
				}
				return "custom-partition"
			}

			// Set up mocks with the fake pickPartitionFn
			loadBalancer, mockProvider, _ := setUpMocksForRoundRobinLoadBalancer(t, fakePickPartitionFn)

			mockProvider.EXPECT().
				GetNumberOfReadPartitions("test-domain-id", types.TaskList{Name: "test-task-list", Kind: &tc.taskListKind}, tc.taskListType).
				Return(tc.nPartitions).
				Times(1)

			kind := tc.taskListKind
			taskList := types.TaskList{Name: "test-task-list", Kind: &kind}
			partition := loadBalancer.PickReadPartition("test-domain-id", taskList, tc.taskListType, tc.forwardedFrom)

			// Validate result
			assert.Equal(t, tc.expectedPartition, partition)
		})
	}
}
