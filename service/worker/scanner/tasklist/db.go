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

package tasklist

import (
	"context"
	"time"

	"github.com/uber/cadence/common/backoff"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var retryForeverPolicy = newRetryForeverPolicy()

func (s *Scavenger) completeTasks(info *p.TaskListInfo, taskID int64, limit int) (int, error) {
	var resp *p.CompleteTasksLessThanResponse
	var err error
	domainName, errorDomain := s.cache.GetDomainName(info.DomainID)
	if errorDomain != nil {
		return 0, errorDomain
	}
	err = s.retryForever(func(ctx context.Context) error {
		resp, err = s.db.CompleteTasksLessThan(ctx, &p.CompleteTasksLessThanRequest{
			DomainID:     info.DomainID,
			TaskListName: info.Name,
			TaskType:     info.TaskType,
			TaskID:       taskID,
			Limit:        limit,
			DomainName:   domainName,
		})
		return err
	})
	if resp != nil {
		return resp.TasksCompleted, err
	}
	return 0, err
}

func (s *Scavenger) getOrphanTasks(limit int) (*p.GetOrphanTasksResponse, error) {
	var tasks *p.GetOrphanTasksResponse
	var err error
	err = s.retryForever(func(ctx context.Context) error {
		tasks, err = s.db.GetOrphanTasks(ctx, &p.GetOrphanTasksRequest{
			Limit: limit,
		})
		return err
	})
	return tasks, err
}

func (s *Scavenger) completeTask(info *p.TaskListInfo, taskid int64) error {
	var err error
	domainName, errorDomain := s.cache.GetDomainName(info.DomainID)
	if errorDomain != nil {
		return errorDomain
	}
	err = s.retryForever(func(ctx context.Context) error {
		err = s.db.CompleteTask(ctx, &p.CompleteTaskRequest{
			TaskList:   info,
			TaskID:     taskid,
			DomainName: domainName,
		})
		return err
	})
	return err
}

func (s *Scavenger) getTasks(info *p.TaskListInfo, batchSize int) (*p.GetTasksResponse, error) {
	var err error
	var resp *p.GetTasksResponse
	domainName, errorDomain := s.cache.GetDomainName(info.DomainID)
	if errorDomain != nil {
		return nil, errorDomain
	}
	err = s.retryForever(func(ctx context.Context) error {
		resp, err = s.db.GetTasks(ctx, &p.GetTasksRequest{
			DomainID:   info.DomainID,
			TaskList:   info.Name,
			TaskType:   info.TaskType,
			ReadLevel:  -1, // get the first N tasks sorted by taskID
			BatchSize:  batchSize,
			DomainName: domainName,
		})
		return err
	})
	return resp, err
}

func (s *Scavenger) listTaskList(pageSize int, pageToken []byte) (*p.ListTaskListResponse, error) {
	var err error
	var resp *p.ListTaskListResponse
	err = s.retryForever(func(ctx context.Context) error {
		resp, err = s.db.ListTaskList(ctx, &p.ListTaskListRequest{
			PageSize:  pageSize,
			PageToken: pageToken,
		})
		return err
	})
	return resp, err
}

func (s *Scavenger) deleteTaskList(info *p.TaskListInfo) error {
	domainName, errorDomain := s.cache.GetDomainName(info.DomainID)
	if errorDomain != nil {
		return errorDomain
	}
	op := func(ctx context.Context) error {
		return s.db.DeleteTaskList(ctx, &p.DeleteTaskListRequest{
			DomainID:     info.DomainID,
			TaskListName: info.Name,
			TaskListType: info.TaskType,
			RangeID:      info.RangeID,
			DomainName:   domainName,
		})
	}
	// retry only on service busy errors
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryForeverPolicy),
		backoff.WithRetryableError(func(err error) bool {
			_, ok := err.(*types.ServiceBusyError)
			return ok
		}),
	)
	return throttleRetry.Do(s.ctx, op)
}

func (s *Scavenger) retryForever(op func(ctx context.Context) error) error {
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryForeverPolicy),
		backoff.WithRetryableError(s.isRetryable),
	)
	return throttleRetry.Do(s.ctx, op)
}

func newRetryForeverPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(250 * time.Millisecond)
	policy.SetExpirationInterval(backoff.NoInterval)
	policy.SetMaximumInterval(30 * time.Second)
	return policy
}

func (s *Scavenger) isRetryable(err error) bool {
	return s.Alive()
}
