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

package timeout

import (
	"time"

	"github.com/uber/cadence/common/types"
)

type TimeoutType string

const (
	TimeoutTypeExecution     TimeoutType = "The Workflow Execution has timed out"
	TimeoutTypeActivity      TimeoutType = "Activity task has timed out"
	TimeoutTypeDecision      TimeoutType = "Decision task has timed out"
	TimeoutTypeChildWorkflow TimeoutType = "Child Workflow Execution has timed out"
)

func (tt TimeoutType) String() string {
	return string(tt)
}

type ExecutionTimeoutMetadata struct {
	ExecutionTime     time.Duration
	ConfiguredTimeout time.Duration
	Tasklist          *types.TaskList
	LastOngoingEvent  *types.HistoryEvent
}

type ChildWfTimeoutMetadata struct {
	ExecutionTime     time.Duration
	ConfiguredTimeout time.Duration
	Execution         *types.WorkflowExecution
}

type ActivityTimeoutMetadata struct {
	TimeoutType       *types.TimeoutType
	ConfiguredTimeout time.Duration
	TimeElapsed       time.Duration
	RetryPolicy       *types.RetryPolicy
	HeartBeatTimeout  time.Duration
	Tasklist          *types.TaskList
}

type DecisionTimeoutMetadata struct {
	ConfiguredTimeout time.Duration
}

type PollersMetadata struct {
	TaskListName    string
	TaskListBacklog int64
}

type HeartbeatingMetadata struct {
	TimeElapsed time.Duration
	RetryPolicy *types.RetryPolicy
}

type TimeoutIssuesMetadata struct {
	ExecutionTimeout *ExecutionTimeoutMetadata
	ActivityTimeout  *ActivityTimeoutMetadata
	ChildWfTimeout   *ChildWfTimeoutMetadata
	DecisionTimeout  *DecisionTimeoutMetadata
}

type TimeoutRootcauseMetadata struct {
	PollersMetadata      *PollersMetadata
	HeartBeatingMetadata *HeartbeatingMetadata
}
