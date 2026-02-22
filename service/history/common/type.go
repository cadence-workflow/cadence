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

package common

import (
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/events"
)

type (
	// NotifyTaskInfo defines the info of task notification
	NotifyTaskInfo struct {
		ExecutionInfo    *persistence.WorkflowExecutionInfo
		Tasks            []persistence.Task
		VersionHistories *persistence.VersionHistories
		Activities       map[int64]*persistence.ActivityInfo
		History          events.PersistedBlobs
		PersistenceError bool

		// if true, the task will be scheduled in memory for the current execution, otherwise
		// it will only be scheduled after the next DB scan. This notification is sometimes passed with a fake
		// timer with the sole purpose of resetting the next scheduled DB read, that's why sometimes we want to
		// avoid scheduling the task in memory.
		ScheduleInMemory bool
	}
)
