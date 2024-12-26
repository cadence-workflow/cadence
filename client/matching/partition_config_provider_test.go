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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestSyncedTaskListPartitionConfig(t *testing.T) {
	c := &syncedTaskListPartitionConfig{TaskListPartitionConfig: types.TaskListPartitionConfig{Version: 50}}

	var g errgroup.Group
	for i := int64(0); i < 100; i++ {
		v := i
		g.Go(func() error {
			c.updateConfig(types.TaskListPartitionConfig{Version: v})
			return nil
		})
	}
	require.NoError(t, g.Wait())

	assert.Equal(t, int64(99), c.Version)
}

func setUpMocksForPartitionConfigProvider(t *testing.T, enableReadFromCache bool) (*partitionConfigProviderImpl, *cache.MockCache) {
	ctrl := gomock.NewController(t)
	mockCache := cache.NewMockCache(ctrl)
	logger := log.NewNoop()

	domainIDToName := func(domainID string) (string, error) {
		return "test-domain", nil
	}

	return &partitionConfigProviderImpl{
		configCache:         mockCache,
		logger:              logger,
		metricsClient:       metrics.NewNoopMetricsClient(),
		domainIDToName:      domainIDToName,
		enableReadFromCache: dynamicconfig.GetBoolPropertyFilteredByTaskListInfo(enableReadFromCache),
		nReadPartitions:     dynamicconfig.GetIntPropertyFilteredByTaskListInfo(3),
		nWritePartitions:    dynamicconfig.GetIntPropertyFilteredByTaskListInfo(5),
	}, mockCache
}

func TestNewPartitionConfigProvider(t *testing.T) {
	dc := dynamicconfig.NewCollection(dynamicconfig.NewNopClient(), testlogger.New(t))
	logger := testlogger.New(t)
	domainIDToName := func(domainID string) (string, error) {
		return "test-domain", nil
	}
	p := NewPartitionConfigProvider(logger, metrics.NewNoopMetricsClient(), domainIDToName, dc)
	assert.NotNil(t, p)
	pImpl, ok := p.(*partitionConfigProviderImpl)
	assert.True(t, ok)
	assert.NotNil(t, pImpl)
	assert.Equal(t, logger, pImpl.logger)
	assert.NotNil(t, pImpl.configCache)
	assert.NotNil(t, pImpl.domainIDToName)
	assert.NotNil(t, pImpl.enableReadFromCache)
	assert.NotNil(t, pImpl.nReadPartitions)
	assert.NotNil(t, pImpl.nWritePartitions)
}

func TestGetNumberOfReadPartitions(t *testing.T) {
	testCases := []struct {
		name                string
		taskListKind        types.TaskListKind
		enableReadFromCache bool
		cachedConfigExists  bool
		expectedPartitions  int
	}{
		{
			name:                "get read partitions from dynamic config",
			taskListKind:        types.TaskListKindNormal,
			enableReadFromCache: false,
			expectedPartitions:  3,
		},
		{
			name:                "get read partitions from cache",
			taskListKind:        types.TaskListKindNormal,
			enableReadFromCache: true,
			cachedConfigExists:  true,
			expectedPartitions:  4,
		},
		{
			name:                "get read partitions for sticky tasklist",
			taskListKind:        types.TaskListKindSticky,
			enableReadFromCache: true,
			expectedPartitions:  1,
		},
		{
			name:                "cache config missing, fallback to default",
			taskListKind:        types.TaskListKindNormal,
			enableReadFromCache: true,
			cachedConfigExists:  false,
			expectedPartitions:  1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			partitionProvider, mockCache := setUpMocksForPartitionConfigProvider(t, tc.enableReadFromCache)

			if tc.enableReadFromCache && tc.taskListKind == types.TaskListKindNormal {
				if tc.cachedConfigExists {
					mockCache.EXPECT().Get(gomock.Any()).Return(&syncedTaskListPartitionConfig{
						TaskListPartitionConfig: types.TaskListPartitionConfig{ReadPartitions: partitions(4)},
					}).Times(1)
				} else {
					mockCache.EXPECT().Get(gomock.Any()).Return(nil).Times(1)
				}
			}

			kind := tc.taskListKind
			taskList := types.TaskList{Name: "test-task-list", Kind: &kind}
			p := partitionProvider.GetNumberOfReadPartitions("test-domain-id", taskList, 0)

			// Validate result
			assert.Equal(t, tc.expectedPartitions, p)
		})
	}
}

func TestGetNumberOfWritePartitions(t *testing.T) {
	testCases := []struct {
		name                string
		taskListKind        types.TaskListKind
		enableReadFromCache bool
		cachedConfigExists  bool
		expectedPartitions  int
	}{
		{
			name:                "get write partitions from dynamic config",
			taskListKind:        types.TaskListKindNormal,
			enableReadFromCache: false,
			expectedPartitions:  3, // nWritePartitions is 5 but capped by nReadPartitions which is 3
		},
		{
			name:                "get write partitions from cache",
			taskListKind:        types.TaskListKindNormal,
			enableReadFromCache: true,
			cachedConfigExists:  true,
			expectedPartitions:  2,
		},
		{
			name:                "get write partitions from cache for sticky tasklist",
			taskListKind:        types.TaskListKindSticky,
			enableReadFromCache: true,
			expectedPartitions:  1,
		},
		{
			name:                "cache config missing, fallback to default",
			taskListKind:        types.TaskListKindNormal,
			enableReadFromCache: true,
			cachedConfigExists:  false,
			expectedPartitions:  1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			partitionProvider, mockCache := setUpMocksForPartitionConfigProvider(t, tc.enableReadFromCache)

			if tc.enableReadFromCache && tc.taskListKind == types.TaskListKindNormal {
				if tc.cachedConfigExists {
					mockCache.EXPECT().Get(gomock.Any()).Return(&syncedTaskListPartitionConfig{
						TaskListPartitionConfig: types.TaskListPartitionConfig{ReadPartitions: partitions(2), WritePartitions: partitions(5)},
					}).Times(1)
				} else {
					mockCache.EXPECT().Get(gomock.Any()).Return(nil).Times(1)
				}
			}
			kind := tc.taskListKind
			taskList := types.TaskList{Name: "test-task-list", Kind: &kind}
			p := partitionProvider.GetNumberOfWritePartitions("test-domain-id", taskList, 0)

			// Validate result
			assert.Equal(t, tc.expectedPartitions, p)
		})
	}
}

func TestUpdatePartitionConfig(t *testing.T) {
	testCases := []struct {
		name              string
		input             *types.TaskListPartitionConfig
		configExists      bool
		expectedNewConfig bool
	}{
		{
			name:              "config exists, update config",
			input:             &types.TaskListPartitionConfig{Version: 2},
			configExists:      true,
			expectedNewConfig: false,
		},
		{
			name:              "config does not exist, create new config",
			input:             &types.TaskListPartitionConfig{Version: 2},
			configExists:      false,
			expectedNewConfig: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			partitionProvider, mockCache := setUpMocksForPartitionConfigProvider(t, true)

			taskList := types.TaskList{Name: "test-task-list"}

			if tc.configExists {
				existingConfig := &syncedTaskListPartitionConfig{
					TaskListPartitionConfig: types.TaskListPartitionConfig{Version: 1},
				}
				mockCache.EXPECT().Get(gomock.Any()).Return(existingConfig).Times(1)
			} else {
				mockCache.EXPECT().Get(gomock.Any()).Return(nil).Times(1)
				mockCache.EXPECT().PutIfNotExist(gomock.Any(), gomock.Any()).Return(&syncedTaskListPartitionConfig{}, nil).Times(1)
			}

			partitionProvider.UpdatePartitionConfig("test-domain-id", taskList, 0, tc.input)
		})
	}
}

func partitions(num int) map[int]*types.TaskListPartition {
	result := make(map[int]*types.TaskListPartition, num)
	for i := 0; i < num; i++ {
		result[i] = &types.TaskListPartition{}
	}
	return result
}
