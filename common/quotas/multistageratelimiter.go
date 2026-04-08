// Copyright (c) 2019 Uber Technologies, Inc.
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

package quotas

import (
	"context"
	"strings"
)

const (
	// since a domain name doesn't contain a slash, it is safe to use it as a separator for task list keys
	taskListKeySeparator = "/"
)

// MultiStageRateLimiter indicates a domain specific rate limit policy
type MultiStageRateLimiter struct {
	domainLimiters   ICollection[string]
	taskListLimiters ICollection[string] // optional
	globalLimiter    Limiter
}

type TaskListKey struct {
	Domain   string
	TaskList string
}

func (k TaskListKey) String() string {
	return k.Domain + taskListKeySeparator + k.TaskList
}

func ParseTaskListKey(key string) TaskListKey {
	parts := strings.SplitN(key, taskListKeySeparator, 2)
	if len(parts) != 2 {
		return TaskListKey{}
	}
	return TaskListKey{
		Domain:   parts[0],
		TaskList: parts[1],
	}
}

// NewMultiStageRateLimiter returns a new quota rate limiter. This is about
// an order of magnitude slower than
func NewMultiStageRateLimiter(global Limiter, domainLimiters ICollection[string], taskListLimiters ICollection[string]) *MultiStageRateLimiter {
	return &MultiStageRateLimiter{
		domainLimiters:   domainLimiters,
		taskListLimiters: taskListLimiters,
		globalLimiter:    global,
	}
}

// Allow attempts to allow a request to go through. The method returns
// immediately with a true or false indicating if the request can make
// progress
func (d *MultiStageRateLimiter) Allow(info Info) (allowed bool) {
	domain := info.Domain
	taskList := info.TaskList

	if taskList != "" && d.taskListLimiters != nil {
		taskListKey := TaskListKey{
			Domain:   domain,
			TaskList: taskList,
		}
		rsv := d.taskListLimiters.For(taskListKey.String()).Reserve()
		defer func() {
			rsv.Used(allowed) // returns the token if allowed but not used
		}()

		if !rsv.Allow() {
			return false
		}
	}

	if domain != "" && d.domainLimiters != nil {
		// take a reservation with the domain limiter first
		rsv := d.domainLimiters.For(domain).Reserve()
		defer func() {
			rsv.Used(allowed) // returns the token if allowed but not used
		}()

		if !rsv.Allow() {
			return false
		}
	}

	// ensure that the reservation does not break the global rate limit, if it
	// does, cancel the reservation and do not allow to proceed.
	if !d.globalLimiter.Allow() {
		return false
	}
	return true
}

// Wait waits up till the context deadline for a rate limit token to allow the request
// to go through. This waits on the per-domain limiter, then waits on the global limiter.
func (d *MultiStageRateLimiter) Wait(ctx context.Context, info Info) error {
	domain := info.Domain
	taskList := info.TaskList

	// A limitation: if the task-list limiter allows but the domain
	// limiter denies, the task-list token is consumed.
	if taskList != "" && d.taskListLimiters != nil {
		taskListKey := TaskListKey{
			Domain:   domain,
			TaskList: taskList,
		}
		if err := d.taskListLimiters.For(taskListKey.String()).Wait(ctx); err != nil {
			return err
		}
	}

	if domain != "" && d.domainLimiters != nil {
		if err := d.domainLimiters.For(domain).Wait(ctx); err != nil {
			return err
		}
	}

	// A limitation in this implementation is that when domain limiter
	// allows but global limiter does not, we have already consumed a token
	// from the domain limiter and cannot return it, because Wait() doesn't
	// return a reservation. This should not affect throughput since
	// global limiter is the bottleneck in this situation.
	return d.globalLimiter.Wait(ctx)
}
