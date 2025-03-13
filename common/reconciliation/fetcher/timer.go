// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package fetcher

import (
	"context"
	"time"

	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
)

// TimerIterator is used to retrieve Concrete executions.
func TimerIterator(
	ctx context.Context,
	retryer persistence.Retryer,
	minTimestamp time.Time,
	maxTimestamp time.Time,
	pageSize int,
) pagination.Iterator {
	return pagination.NewIterator(ctx, nil, getUserTimers(retryer, minTimestamp, maxTimestamp, pageSize))
}

func getUserTimers(
	pr persistence.Retryer,
	minTimestamp time.Time,
	maxTimestamp time.Time,
	pageSize int,
) pagination.FetchFn {
	return func(ctx context.Context, token pagination.PageToken) (pagination.Page, error) {
		req := &persistence.GetHistoryTasksRequest{
			TaskCategory: persistence.HistoryTaskCategoryTimer,
			InclusiveMinTaskKey: persistence.HistoryTaskKey{
				ScheduledTime: minTimestamp,
			},
			ExclusiveMaxTaskKey: persistence.HistoryTaskKey{
				ScheduledTime: maxTimestamp,
			},
			PageSize: pageSize,
		}
		if token != nil {
			req.NextPageToken = token.([]byte)
		}
		resp, err := pr.GetHistoryTasks(ctx, req)

		if err != nil {
			return pagination.Page{}, err
		}

		var timers []pagination.Entity

		for _, t := range resp.Tasks {
			if t.GetTaskType() != persistence.TaskTypeUserTimer {
				continue
			}

			timer := &entity.Timer{
				ShardID:             pr.GetShardID(),
				DomainID:            t.GetDomainID(),
				WorkflowID:          t.GetWorkflowID(),
				RunID:               t.GetRunID(),
				TaskType:            t.GetTaskType(),
				VisibilityTimestamp: t.GetVisibilityTimestamp(),
			}

			if err := timer.Validate(); err != nil {
				return pagination.Page{}, err
			}
			timers = append(timers, timer)
		}
		var nextToken interface{} = resp.NextPageToken
		if len(resp.NextPageToken) == 0 {
			nextToken = nil
		}

		page := pagination.Page{
			CurrentToken: token,
			NextToken:    nextToken,
			Entities:     timers,
		}
		return page, nil
	}
}
