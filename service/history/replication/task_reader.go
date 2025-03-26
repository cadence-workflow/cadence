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
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
)

const minReadTaskSize = 20

// DynamicTaskReader will read replication tasks from database using dynamic batch sizing depending on replication lag.
type DynamicTaskReader struct {
	shardID          int
	executionManager persistence.ExecutionManager
	timeSource       clock.TimeSource
	config           *config.Config

	lastTaskCreationTime atomic.Value
}

// NewDynamicTaskReader creates new DynamicTaskReader
func NewDynamicTaskReader(
	shardID int,
	executionManager persistence.ExecutionManager,
	timeSource clock.TimeSource,
	config *config.Config,
) *DynamicTaskReader {
	return &DynamicTaskReader{
		shardID:              shardID,
		executionManager:     executionManager,
		timeSource:           timeSource,
		config:               config,
		lastTaskCreationTime: atomic.Value{},
	}
}

// Read reads and returns replications tasks from readLevel to maxReadLevel. Batch size is determined dynamically.
// If replication lag is less than config.ReplicatorUpperLatency it will be proportionally smaller.
// Otherwise default batch size of config.ReplicatorProcessorFetchTasksBatchSize will be used.
func (r *DynamicTaskReader) Read(ctx context.Context, readLevel int64, maxReadLevel int64) ([]persistence.Task, bool, error) {
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
		PageSize: r.getBatchSize(),
	})
	if err != nil {
		return nil, false, err
	}

	var lastTaskCreationTime time.Time
	for _, task := range response.Tasks {
		if task.GetVisibilityTimestamp().After(lastTaskCreationTime) {
			lastTaskCreationTime = task.GetVisibilityTimestamp()
		}
	}
	r.lastTaskCreationTime.Store(lastTaskCreationTime)

	hasMore := response.NextPageToken != nil
	return response.Tasks, hasMore, nil
}

func (r *DynamicTaskReader) getBatchSize() int {
	defaultBatchSize := r.config.ReplicatorProcessorFetchTasksBatchSize(r.shardID)
	maxReplicationLatency := r.config.ReplicatorUpperLatency()
	now := r.timeSource.Now()

	if r.lastTaskCreationTime.Load() == nil {
		return defaultBatchSize
	}
	taskLatency := now.Sub(r.lastTaskCreationTime.Load().(time.Time))
	if taskLatency < 0 {
		taskLatency = 0
	}
	if taskLatency >= maxReplicationLatency {
		return defaultBatchSize
	}
	return minReadTaskSize + int(float64(taskLatency)/float64(maxReplicationLatency)*float64(defaultBatchSize))
}
