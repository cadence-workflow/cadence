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

	"github.com/uber/cadence/common/persistence"
)

// TaskReader will read replication tasks from database
type TaskReader struct {
	shardID          int
	executionManager persistence.ExecutionManager
}

// NewTaskReader creates new TaskReader
func NewTaskReader(shardID int, executionManager persistence.ExecutionManager) *TaskReader {
	return &TaskReader{
		shardID:          shardID,
		executionManager: executionManager,
	}
}

// Read reads and returns replications tasks from readLevel to maxReadLevel
func (r *TaskReader) Read(ctx context.Context, readLevel int64, maxReadLevel int64, batchSize int) ([]persistence.Task, bool, error) {
	// Check if it is even possible to return any results.
	// If not return early with empty response. Do not hit persistence.
	if readLevel >= maxReadLevel {
		return nil, false, nil
	}

	response, err := r.executionManager.GetHistoryTasks(ctx, &persistence.GetHistoryTasksRequest{
		TaskCategory: persistence.HistoryTaskCategoryReplication,
		InclusiveMinTaskKey: persistence.HistoryTaskKey{
			TaskID: readLevel + 1,
		},
		ExclusiveMaxTaskKey: persistence.HistoryTaskKey{
			TaskID: maxReadLevel + 1,
		},
		PageSize: batchSize,
	})
	if err != nil {
		return nil, false, err
	}

	hasMore := response.NextPageToken != nil
	return response.Tasks, hasMore, nil
}
