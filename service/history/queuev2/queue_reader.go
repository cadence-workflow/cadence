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

//go:generate mockgen -package $GOPACKAGE -destination queue_reader_mock.go github.com/uber/cadence/service/history/queuev2 QueueReader
package queuev2

import (
	"context"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

type (
	QueueReader interface {
		GetTask(context.Context, *GetTaskRequest) (*GetTaskResponse, error)
	}

	GetTaskRequest struct {
		Progress  *GetTaskProgress
		Predicate Predicate
		PageSize  int
	}

	// GetTaskProgress contains the range of the slice to read, the next page token, and the next task key
	GetTaskProgress struct {
		Range
		NextPageToken []byte
		NextTaskKey   persistence.HistoryTaskKey
	}

	GetTaskResponse struct {
		Tasks    []persistence.Task
		Progress *GetTaskProgress
	}

	simpleQueueReader struct {
		shard    shard.Context
		category persistence.HistoryTaskCategory
	}
)

func NewQueueReader(
	shard shard.Context,
	category persistence.HistoryTaskCategory,
) QueueReader {
	return &simpleQueueReader{
		shard:    shard,
		category: category,
	}
}

func (r *simpleQueueReader) GetTask(ctx context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
	resp, err := r.shard.GetExecutionManager().GetHistoryTasks(ctx, &persistence.GetHistoryTasksRequest{
		TaskCategory:        r.category,
		InclusiveMinTaskKey: req.Progress.InclusiveMinTaskKey,
		ExclusiveMaxTaskKey: req.Progress.ExclusiveMaxTaskKey,
		PageSize:            req.PageSize,
		NextPageToken:       req.Progress.NextPageToken,
	})
	if err != nil {
		return nil, err
	}

	nextTaskKey := req.Progress.ExclusiveMaxTaskKey
	tasks := make([]persistence.Task, 0, len(resp.Tasks))
	for _, task := range resp.Tasks {
		// filter out tasks that don't match the predicate
		if req.Predicate.Check(task) {
			tasks = append(tasks, task)
		}
	}
	// If there are more tasks to read, set the next task key to the next task key of the last task
	if len(resp.NextPageToken) != 0 && len(resp.Tasks) > 0 {
		nextTaskKey = resp.Tasks[len(resp.Tasks)-1].GetTaskKey().Next()
	}

	return &GetTaskResponse{
		Tasks: tasks,
		Progress: &GetTaskProgress{
			Range:         req.Progress.Range,
			NextPageToken: resp.NextPageToken,
			NextTaskKey:   nextTaskKey,
		},
	}, nil
}
