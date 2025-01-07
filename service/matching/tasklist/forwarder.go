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

package tasklist

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

type (
	// forwarderImpl is the type that contains state pertaining to
	// the api call forwarder component
	forwarderImpl struct {
		scope        metrics.Scope
		cfg          *config.ForwarderConfig
		taskListID   *Identifier
		taskListKind types.TaskListKind
		client       matching.Client

		// token channels that vend tokens necessary to make
		// API calls exposed by forwarder. Tokens are used
		// to enforce maxOutstanding forwarded calls from this
		// instance. And channels are used so that the caller
		// can use them in a select{} block along with other
		// conditions
		addReqToken  atomic.Value
		pollReqToken atomic.Value

		// cached values of maxOutstanding dynamic config values.
		// these are used to detect changes
		outstandingTasksLimit int32
		outstandingPollsLimit int32

		// todo: implement a rate limiter that automatically
		// adjusts rate based on ServiceBusy errors from API calls
		limiter *quotas.DynamicRateLimiter

		isolationGroups []string
	}
	// ForwarderReqToken is the token that must be acquired before
	// making forwarder API calls. This type contains the state
	// for the token itself
	ForwarderReqToken struct {
		ch         chan *ForwarderReqToken
		isolatedCh map[string]chan *ForwarderReqToken
	}
)

var (
	ErrNoParent            = errors.New("cannot find parent task list for forwarding")
	ErrTaskListKind        = errors.New("forwarding is not supported on sticky task list")
	ErrInvalidTaskListType = errors.New("unrecognized task list type")
	ErrForwarderSlowDown   = errors.New("tasklist forwarding throttle limit exceeded")
)

// noopForwarderTokenC refers to a token channel that blocks forever
var noopForwarderTokenC <-chan *ForwarderReqToken = make(chan *ForwarderReqToken)

// newForwarder returns an instance of Forwarder object which
// can be used to forward api request calls from a task list
// child partition to a task list parent partition. The returned
// forwarder is tied to a single task list. All of the exposed
// methods can return the following errors:
// Returns following errors:
//   - errNoParent: If this task list doesn't have a parent to forward to
//   - errTaskListKind: If the task list is a sticky task list. Sticky task lists are never partitioned
//   - errForwarderSlowDown: When the rate limit is exceeded
//   - errInvalidTaskType: If the task list type is invalid
func newForwarder(
	cfg *config.ForwarderConfig,
	taskListID *Identifier,
	kind types.TaskListKind,
	client matching.Client,
	isolationGroups []string,
	scope metrics.Scope,
) Forwarder {
	rpsFunc := func() float64 { return float64(cfg.ForwarderMaxRatePerSecond()) }
	fwdr := &forwarderImpl{
		cfg:                   cfg,
		client:                client,
		taskListID:            taskListID,
		taskListKind:          kind,
		outstandingTasksLimit: int32(cfg.ForwarderMaxOutstandingTasks() * (len(isolationGroups) + 1)),
		outstandingPollsLimit: int32(cfg.ForwarderMaxOutstandingPolls()),
		limiter:               quotas.NewDynamicRateLimiter(rpsFunc),
		isolationGroups:       isolationGroups,
		scope:                 scope,
	}
	fwdr.addReqToken.Store(newForwarderReqToken(int(fwdr.outstandingTasksLimit), nil))
	fwdr.pollReqToken.Store(newForwarderReqToken(int(fwdr.outstandingPollsLimit), isolationGroups))
	return fwdr
}

// ForwardTask forwards an activity or decision task to the parent task list partition if it exist
func (fwdr *forwarderImpl) ForwardTask(ctx context.Context, task *InternalTask) error {
	if fwdr.taskListKind == types.TaskListKindSticky {
		return ErrTaskListKind
	}

	name := fwdr.taskListID.Parent(fwdr.cfg.ForwarderMaxChildrenPerNode())
	if name == "" {
		return ErrNoParent
	}

	if !fwdr.limiter.Allow() {
		return ErrForwarderSlowDown
	}

	var err error

	sw := fwdr.scope.StartTimer(metrics.ForwardTaskLatencyPerTaskList)
	defer sw.Stop()
	switch fwdr.taskListID.GetType() {
	case persistence.TaskListTypeDecision:
		_, err = fwdr.client.AddDecisionTask(ctx, &types.AddDecisionTaskRequest{
			DomainUUID: task.Event.DomainID,
			Execution:  task.WorkflowExecution(),
			TaskList: &types.TaskList{
				Name: name,
				Kind: &fwdr.taskListKind,
			},
			ScheduleID:                    task.Event.ScheduleID,
			ScheduleToStartTimeoutSeconds: &task.Event.ScheduleToStartTimeoutSeconds,
			Source:                        &task.source,
			ForwardedFrom:                 fwdr.taskListID.GetName(),
			PartitionConfig:               task.Event.PartitionConfig,
		})
	case persistence.TaskListTypeActivity:
		_, err = fwdr.client.AddActivityTask(ctx, &types.AddActivityTaskRequest{
			DomainUUID:       fwdr.taskListID.GetDomainID(),
			SourceDomainUUID: task.Event.DomainID,
			Execution:        task.WorkflowExecution(),
			TaskList: &types.TaskList{
				Name: name,
				Kind: &fwdr.taskListKind,
			},
			ScheduleID:                    task.Event.ScheduleID,
			ScheduleToStartTimeoutSeconds: &task.Event.ScheduleToStartTimeoutSeconds,
			Source:                        &task.source,
			ForwardedFrom:                 fwdr.taskListID.GetName(),
			PartitionConfig:               task.Event.PartitionConfig,
		})
	default:
		return ErrInvalidTaskListType
	}

	return fwdr.handleErr(err)
}

// ForwardQueryTask forwards a query task to parent task list partition, if it exist
func (fwdr *forwarderImpl) ForwardQueryTask(
	ctx context.Context,
	task *InternalTask,
) (*types.QueryWorkflowResponse, error) {

	if fwdr.taskListKind == types.TaskListKindSticky {
		return nil, ErrTaskListKind
	}

	name := fwdr.taskListID.Parent(fwdr.cfg.ForwarderMaxChildrenPerNode())
	if name == "" {
		return nil, ErrNoParent
	}

	sw := fwdr.scope.StartTimer(metrics.ForwardQueryLatencyPerTaskList)
	defer sw.Stop()
	resp, err := fwdr.client.QueryWorkflow(ctx, &types.MatchingQueryWorkflowRequest{
		DomainUUID: task.Query.Request.DomainUUID,
		TaskList: &types.TaskList{
			Name: name,
			Kind: &fwdr.taskListKind,
		},
		QueryRequest:  task.Query.Request.QueryRequest,
		ForwardedFrom: fwdr.taskListID.GetName(),
	})

	return resp, fwdr.handleErr(err)
}

// ForwardPoll forwards a poll request to parent task list partition if it exist
func (fwdr *forwarderImpl) ForwardPoll(ctx context.Context) (*InternalTask, error) {
	if fwdr.taskListKind == types.TaskListKindSticky {
		return nil, ErrTaskListKind
	}

	name := fwdr.taskListID.Parent(fwdr.cfg.ForwarderMaxChildrenPerNode())
	if name == "" {
		return nil, ErrNoParent
	}

	sw := fwdr.scope.StartTimer(metrics.ForwardPollLatencyPerTaskList)
	defer sw.Stop()
	pollerID := PollerIDFromContext(ctx)
	identity := IdentityFromContext(ctx)
	isolationGroup := IsolationGroupFromContext(ctx)

	switch fwdr.taskListID.GetType() {
	case persistence.TaskListTypeDecision:
		resp, err := fwdr.client.PollForDecisionTask(ctx, &types.MatchingPollForDecisionTaskRequest{
			DomainUUID: fwdr.taskListID.GetDomainID(),
			PollerID:   pollerID,
			PollRequest: &types.PollForDecisionTaskRequest{
				TaskList: &types.TaskList{
					Name: name,
					Kind: &fwdr.taskListKind,
				},
				Identity: identity,
			},
			ForwardedFrom:  fwdr.taskListID.GetName(),
			IsolationGroup: isolationGroup,
		})
		if err != nil {
			return nil, fwdr.handleErr(err)
		}
		return newInternalStartedTask(&startedTaskInfo{decisionTaskInfo: resp}), nil
	case persistence.TaskListTypeActivity:
		resp, err := fwdr.client.PollForActivityTask(ctx, &types.MatchingPollForActivityTaskRequest{
			DomainUUID: fwdr.taskListID.GetDomainID(),
			PollerID:   pollerID,
			PollRequest: &types.PollForActivityTaskRequest{
				TaskList: &types.TaskList{
					Name: name,
					Kind: &fwdr.taskListKind,
				},
				Identity: identity,
			},
			ForwardedFrom:  fwdr.taskListID.GetName(),
			IsolationGroup: isolationGroup,
		})
		if err != nil {
			return nil, fwdr.handleErr(err)
		}
		return newInternalStartedTask(&startedTaskInfo{activityTaskInfo: resp}), nil
	}

	return nil, ErrInvalidTaskListType
}

// AddReqTokenC returns a channel that can be used to wait for a token
// that's necessary before making a ForwardTask or ForwardQueryTask API call.
// After the API call is invoked, token.release() must be invoked
// TODO: consider having separate token pools for different isolation groups
func (fwdr *forwarderImpl) AddReqTokenC() <-chan *ForwarderReqToken {
	fwdr.refreshTokenC(&fwdr.addReqToken, &fwdr.outstandingTasksLimit, int32(fwdr.cfg.ForwarderMaxOutstandingTasks()*(len(fwdr.isolationGroups)+1)), nil)
	return fwdr.addReqToken.Load().(*ForwarderReqToken).ch
}

// PollReqTokenC returns a channel that can be used to wait for a token
// that's necessary before making a ForwardPoll API call. After the API
// call is invoked, token.release() must be invoked
// For tasklists with isolation enabled, we have separate token pools for different isolation groups
func (fwdr *forwarderImpl) PollReqTokenC(isolationGroup string) <-chan *ForwarderReqToken {
	fwdr.refreshTokenC(&fwdr.pollReqToken, &fwdr.outstandingPollsLimit, int32(fwdr.cfg.ForwarderMaxOutstandingPolls()), fwdr.isolationGroups)
	if isolationGroup == "" {
		return fwdr.pollReqToken.Load().(*ForwarderReqToken).ch
	}
	return fwdr.pollReqToken.Load().(*ForwarderReqToken).isolatedCh[isolationGroup]
}

func (fwdr *forwarderImpl) refreshTokenC(value *atomic.Value, curr *int32, maxLimit int32, isolationGroups []string) {
	currLimit := atomic.LoadInt32(curr)
	if currLimit != maxLimit {
		if atomic.CompareAndSwapInt32(curr, currLimit, maxLimit) {
			value.Store(newForwarderReqToken(int(maxLimit), isolationGroups))
		}
	}
}

func (fwdr *forwarderImpl) handleErr(err error) error {
	if _, ok := err.(*types.ServiceBusyError); ok {
		return ErrForwarderSlowDown
	}
	return err
}

func newForwarderReqToken(maxOutstanding int, isolationGroups []string) *ForwarderReqToken {
	isolatedCh := make(map[string]chan *ForwarderReqToken, len(isolationGroups))
	for _, ig := range isolationGroups {
		isolatedCh[ig] = make(chan *ForwarderReqToken, maxOutstanding)
	}
	reqToken := &ForwarderReqToken{ch: make(chan *ForwarderReqToken, maxOutstanding), isolatedCh: isolatedCh}
	for i := 0; i < maxOutstanding; i++ {
		reqToken.ch <- reqToken
		for _, ch := range reqToken.isolatedCh {
			ch <- reqToken
		}
	}
	return reqToken
}

func (token *ForwarderReqToken) release(isolationGroup string) {
	if isolationGroup == "" {
		token.ch <- token
	} else {
		token.isolatedCh[isolationGroup] <- token
	}
}
