// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package history

import "github.com/uber/cadence/common/persistence"

type (
	maxReadAckLevel func() int64

	updateClusterAckLevel func(ackLevel int64) error

	transferQueueProcessorBase struct {
		shard                 ShardContext
		options               *QueueProcessorOptions
		executionManager      persistence.ExecutionManager
		maxReadAckLevel       maxReadAckLevel
		updateClusterAckLevel updateClusterAckLevel
	}
)

func newTransferQueueProcessorBase(shard ShardContext, options *QueueProcessorOptions,
	maxReadAckLevel maxReadAckLevel, updateClusterAckLevel updateClusterAckLevel) *transferQueueProcessorBase {
	return &transferQueueProcessorBase{
		shard:                 shard,
		options:               options,
		executionManager:      shard.GetExecutionManager(),
		maxReadAckLevel:       maxReadAckLevel,
		updateClusterAckLevel: updateClusterAckLevel,
	}
}

func (t *transferQueueProcessorBase) readTasks(readLevel int64) ([]queueTaskInfo, bool, error) {
	batchSize := t.options.BatchSize
	response, err := t.executionManager.GetTransferTasks(&persistence.GetTransferTasksRequest{
		ReadLevel:    readLevel,
		MaxReadLevel: t.maxReadAckLevel(),
		BatchSize:    batchSize,
	})

	if err != nil {
		return nil, false, err
	}

	tasks := make([]queueTaskInfo, len(response.Tasks))
	for i := range response.Tasks {
		tasks[i] = response.Tasks[i]
	}

	return tasks, len(response.NextPageToken) != 0, nil
}

func (t *transferQueueProcessorBase) completeTask(taskID int64) error {
	// this is a no op on the for transfer queue active / standby processor
	return nil
}

func (t *transferQueueProcessorBase) updateAckLevel(ackLevel int64) error {
	return t.updateClusterAckLevel(ackLevel)
}
