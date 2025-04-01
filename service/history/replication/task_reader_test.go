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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
)

const (
	testBatchSize = 50
)

var (
	testTime = time.Now()

	testReplicationTasks = []persistence.Task{
		&persistence.HistoryReplicationTask{TaskData: persistence.TaskData{TaskID: 50, VisibilityTimestamp: testTime.Add(-1 * time.Second)}},
		&persistence.HistoryReplicationTask{TaskData: persistence.TaskData{TaskID: 51, VisibilityTimestamp: testTime.Add(-2 * time.Second)}},
	}
)

func TestTaskReader(t *testing.T) {
	tests := []struct {
		name              string
		prepareExecutions func(m *persistence.MockExecutionManager)
		readLevel         int64
		maxReadLevel      int64
		expectResponse    []persistence.Task
		expectErr         string
	}{
		{
			name:         "read replication tasks - first read will use default batch size",
			readLevel:    50,
			maxReadLevel: 100,
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetHistoryTasks(gomock.Any(), &persistence.GetHistoryTasksRequest{
					TaskCategory: persistence.HistoryTaskCategoryReplication,
					InclusiveMinTaskKey: persistence.HistoryTaskKey{
						TaskID: 51,
					},
					ExclusiveMaxTaskKey: persistence.HistoryTaskKey{
						TaskID: 101,
					},
					PageSize: testBatchSize,
				}).Return(&persistence.GetHistoryTasksResponse{Tasks: testReplicationTasks}, nil)
			},
			expectResponse: testReplicationTasks,
		},
		{
			name:         "do not hit persistence when no task will be returned",
			readLevel:    50,
			maxReadLevel: 50,
			prepareExecutions: func(m *persistence.MockExecutionManager) {
				m.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Times(0)
			},
			expectResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			em := persistence.NewMockExecutionManager(ctrl)
			tt.prepareExecutions(em)

			reader := NewTaskReader(testShardID, em)
			response, _, err := reader.Read(context.Background(), tt.readLevel, tt.maxReadLevel, testBatchSize)

			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectResponse, response)
			}
		})
	}
}
