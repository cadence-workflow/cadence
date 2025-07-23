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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go github.com/uber/cadence/service/matching/tasklist Manager
//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go github.com/uber/cadence/service/matching/tasklist TaskMatcher
//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go github.com/uber/cadence/service/matching/tasklist Forwarder
//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go github.com/uber/cadence/service/matching/tasklist TaskCompleter

package tasklist

import (
	"context"
	"time"

	"github.com/uber/cadence/common/types"
)

type (
	Manager interface {
		Start() error
		Stop()
		// AddTask adds a task to the task list. This method will first attempt a synchronous
		// match with a poller. When that fails, task will be written to database and later
		// asynchronously matched with a poller
		AddTask(ctx context.Context, params AddTaskParams) (syncMatch bool, err error)
		// GetTask blocks waiting for a task Returns error when context deadline is exceeded
		// maxDispatchPerSecond is the max rate at which tasks are allowed to be dispatched
		// from this task list to pollers
		GetTask(ctx context.Context, maxDispatchPerSecond *float64) (*InternalTask, error)
		// DispatchTask dispatches a task to a poller. When there are no pollers to pick
		// up the task, this method will return error. Task will not be persisted to db
		DispatchTask(ctx context.Context, task *InternalTask) error
		// DispatchQueryTask will dispatch query to local or remote poller. If forwarded then result or error is returned,
		// if dispatched to local poller then nil and nil is returned.
		DispatchQueryTask(ctx context.Context, taskID string, request *types.MatchingQueryWorkflowRequest) (*types.MatchingQueryWorkflowResponse, error)
		CancelPoller(pollerID string)
		GetAllPollerInfo() []*types.PollerInfo
		HasPollerAfter(accessTime time.Time) bool
		// DescribeTaskList returns information about the target tasklist
		DescribeTaskList(includeTaskListStatus bool) *types.DescribeTaskListResponse
		String() string
		GetTaskListKind() types.TaskListKind
		TaskListID() *Identifier
		TaskListPartitionConfig() *types.TaskListPartitionConfig
		UpdateTaskListPartitionConfig(context.Context, *types.TaskListPartitionConfig) error
		RefreshTaskListPartitionConfig(context.Context, *types.TaskListPartitionConfig) error
		LoadBalancerHints() *types.LoadBalancerHints
		ReleaseBlockedPollers() error
	}

	TaskMatcher interface {
		DisconnectBlockedPollers()
		Offer(ctx context.Context, task *InternalTask) (bool, error)
		OfferOrTimeout(ctx context.Context, startT time.Time, task *InternalTask) (bool, error)
		OfferQuery(ctx context.Context, task *InternalTask) (*types.MatchingQueryWorkflowResponse, error)
		MustOffer(ctx context.Context, task *InternalTask) error
		Poll(ctx context.Context, isolationGroup string) (*InternalTask, error)
		PollForQuery(ctx context.Context) (*InternalTask, error)
		RefreshCancelContext()
	}

	Forwarder interface {
		ForwardTask(ctx context.Context, task *InternalTask) error
		ForwardQueryTask(ctx context.Context, task *InternalTask) (*types.MatchingQueryWorkflowResponse, error)
		ForwardPoll(ctx context.Context) (*InternalTask, error)
		AddReqTokenC() <-chan *ForwarderReqToken
		PollReqTokenC() <-chan *ForwarderReqToken
	}

	TaskCompleter interface {
		CompleteTaskIfStarted(ctx context.Context, task *InternalTask) error
	}
)
