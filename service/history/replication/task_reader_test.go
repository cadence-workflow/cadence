// The MIT License (MIT)
//
// Copyright (c) 2017-2022 Uber Technologies Inc.
//
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

package replication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
)

const (
	testBatchSize    = 50
	testUpperLatency = 5 * time.Second
)

var (
	testTime = time.Now()

	testReplicationTasks = []*persistence.ReplicationTaskInfo{
		{TaskID: 50, CreationTime: testTime.Add(-1 * time.Second).UnixNano()},
		{TaskID: 51, CreationTime: testTime.Add(-2 * time.Second).UnixNano()},
	}
)

func TestDynamicTaskReader(t *testing.T) {
	tests := []struct {
		name              string
		prepareExecutions func(m *persistence.MockExecutionManager)
		readLevel         int64
		maxReadLevel      int64
		lastCreateTime    time.Time
		expectCreateTime  time.Time
		expectResponse    []*persistence.ReplicationTaskInfo
		expectErr         string
	}{
		{
			name:         "read replication tasks - first read will use default batch size",
			readLevel:    50,
			maxReadLevel: 100,
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetReplicationTasks(gomock.Any(), &persistence.GetReplicationTasksRequest{
					ReadLevel:    51,
					MaxReadLevel: 101,
					BatchSize:    testBatchSize,
				}).Return(&persistence.GetReplicationTasksResponse{Tasks: testReplicationTasks}, nil)
			},
			expectResponse: testReplicationTasks,
		},
		{
			name:           "read replication tasks - lagging less than upper latency - calculates custom batch size",
			readLevel:      50,
			maxReadLevel:   100,
			lastCreateTime: testTime.Add(-time.Second),
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetReplicationTasks(gomock.Any(), &persistence.GetReplicationTasksRequest{
					ReadLevel:    51,
					MaxReadLevel: 101,
					BatchSize:    30, // minReadTaskSize (20) + (latency (1s) / maxLatency (5s) * defaultBatchSize (50))
				}).Return(&persistence.GetReplicationTasksResponse{Tasks: testReplicationTasks}, nil)
			},
			expectResponse: testReplicationTasks,
		},
		{
			name:           "read replication tasks - lagging more than upper latency - uses default batch size",
			readLevel:      50,
			maxReadLevel:   100,
			lastCreateTime: testTime.Add(-(testUpperLatency + time.Second)),
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetReplicationTasks(gomock.Any(), &persistence.GetReplicationTasksRequest{
					ReadLevel:    51,
					MaxReadLevel: 101,
					BatchSize:    50,
				}).Return(&persistence.GetReplicationTasksResponse{Tasks: testReplicationTasks}, nil)
			},
			expectResponse: testReplicationTasks,
		},
		{
			name:           "read replication tasks - negative task latency - uses min batch size",
			readLevel:      50,
			maxReadLevel:   100,
			lastCreateTime: testTime.Add(time.Second),
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetReplicationTasks(gomock.Any(), &persistence.GetReplicationTasksRequest{
					ReadLevel:    51,
					MaxReadLevel: 101,
					BatchSize:    20, // minReadTaskSize (20)
				}).Return(&persistence.GetReplicationTasksResponse{Tasks: testReplicationTasks}, nil)
			},
			expectResponse: testReplicationTasks,
		},
		{
			name:         "ensure last creation time is updated",
			readLevel:    50,
			maxReadLevel: 100,
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetReplicationTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetReplicationTasksResponse{Tasks: testReplicationTasks}, nil)
			},
			expectResponse:   testReplicationTasks,
			expectCreateTime: testTime.Add(-1 * time.Second),
		},
		{
			name:         "failed to read replication tasks - return error",
			readLevel:    50,
			maxReadLevel: 100,
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetReplicationTasks(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to read replication tasks"))
			},
			expectErr: "failed to read replication tasks",
		},
		{
			name:         "do not hit persistence when no task will be returned",
			readLevel:    50,
			maxReadLevel: 50,
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetReplicationTasks(gomock.Any(), gomock.Any()).Times(0)
			},
			expectResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			em := persistence.NewMockExecutionManager(ctrl)
			tt.prepareExecutions(em)

			config := config.Config{
				ReplicatorProcessorFetchTasksBatchSize: dynamicconfig.GetIntPropertyFilteredByShardID(testBatchSize),
				ReplicatorUpperLatency:                 dynamicconfig.GetDurationPropertyFn(testUpperLatency),
			}

			timeSource := clock.NewMockedTimeSourceAt(testTime)

			reader := NewDynamicTaskReader(testShardID, em, timeSource, &config)
			if tt.lastCreateTime != (time.Time{}) {
				reader.lastTaskCreationTime.Store(tt.lastCreateTime)
			}

			response, _, err := reader.Read(context.Background(), tt.readLevel, tt.maxReadLevel)

			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectResponse, response)
			}

			if tt.expectCreateTime != (time.Time{}) {
				createTime := reader.lastTaskCreationTime.Load().(time.Time)
				assert.Zero(t, tt.expectCreateTime.Sub(createTime))
			}
		})
	}
}
