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

package replication

import (
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/rangeiter"
	"github.com/uber/cadence/service/history/config"
)

// DynamicTaskBatchSizer is responsible for the batch size used to retrieve ReplicationTasks by TaskAckManager
// It adjusts the task batch size based on the error and the getTasksResult,
// and use the following rules:
//
//  1. If there is an error, decrease the task batch size.
//     In case of an increased load to the database, we should reduce the load to the database
//
//  2. If the task batch size is shrunk, decrease the task batch size.
//     The payload size of messages is too big, so we should decrease
//     the number of messages to be sure that future messages will not be shrunk
//
//  3. If the read level of a passive cluster has not been changed and there are no fetched tasks,
//     not changed the task batch size. There is no need to change because the replication is not stuck,
//     there just are no new tasks
//
//  4. If the read level of a passive cluster has not been changed and if there are fetched tasks,
//     decrease the task batch size. There were no new replication tasks in the previous call
//     or the replication was stuck on the passive side
//
//  5. If the read level of a passive cluster has been changed and if there are more tasks in db,
//     increase the task batch size. We should retrieve the maximum possible value at the next time,
//     as there are more tasks to be replicated
//
//  6. If the read level of a passive cluster has been changed and if there are no more tasks in db,
//     not change the size. The existing size is already enough, and there are no more tasks to be replicated
type DynamicTaskBatchSizer interface {
	analyse(err error, state *getTasksResult)
	value() int
}

// dynamicTaskBatchSizerImpl is the implementation of DynamicTaskBatchSizer
type dynamicTaskBatchSizerImpl struct {
	iter   rangeiter.Iterator[int]
	logger log.Logger
}

// NewDynamicTaskBatchSizer creates a new dynamicTaskBatchSizerImpl
func NewDynamicTaskBatchSizer(shardID int, logger log.Logger, config *config.Config) DynamicTaskBatchSizer {
	logger = logger.WithTags(tag.ComponentReplicationDynamicTaskBatchSizer)
	return &dynamicTaskBatchSizerImpl{
		logger: logger,
		iter: rangeiter.NewDynamicConfigLinearIterator(
			func() int { return config.ReplicatorProcessorMinTaskBatchSize(shardID) },
			func() int { return config.ReplicatorProcessorMaxTaskBatchSize(shardID) },
			func() int { return config.ReplicatorProcessorBatchSizeStepCount(shardID) },
			logger,
		),
	}
}

func (d *dynamicTaskBatchSizerImpl) analyse(err error, state *getTasksResult) {
	switch {
	case err != nil:
		d.logger.Debug(
			"Decrease task batch size due to error",
			tag.Error(err), tag.ReplicationTaskBatchSize(d.iter.Previous()))
	case state.isShrunk:
		d.logger.Debug(
			"Decrease task batch size due to message shrinking",
			tag.ReplicationTaskBatchSize(d.iter.Previous()))
	case state.previousReadTaskID == state.lastReadTaskID && len(state.taskInfos) > 0:
		d.logger.Debug(
			"Decrease task batch size due to possible replication stuck",
			tag.ReplicationTaskBatchSize(d.iter.Previous()))
	case state.msgs.HasMore:
		d.logger.Debug(
			"Increase task batch size due to more replication tasks in database",
			tag.ReplicationTaskBatchSize(d.iter.Next()))
	}
}

func (d *dynamicTaskBatchSizerImpl) value() int {
	return d.iter.Value()
}
