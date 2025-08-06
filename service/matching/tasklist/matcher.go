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
	"fmt"
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/ctxutils"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/event"
)

// taskMatcherImpl matches a task producer with a task consumer
// Producers are usually rpc calls from history or taskReader
// that drains backlog from db. Consumers are the task list pollers
type taskMatcherImpl struct {
	log log.Logger
	// synchronous task channel to match producer/consumer for any isolation group
	// tasks having no isolation requirement are added to this channel
	// and pollers from all isolation groups read from this channel
	taskC chan *InternalTask
	// synchronos task channels to match producer/consumer for a certain isolation group
	// the key is the name of the isolation group
	isolatedTaskC map[string]chan *InternalTask
	// synchronous task channel to match query task - the reason to have
	// separate channel for this is because there are cases when consumers
	// are interested in queryTasks but not others. Example is when domain is
	// not active in a cluster
	queryTaskC chan *InternalTask
	// ratelimiter that limits the rate at which tasks can be dispatched to consumers
	limiter quotas.Limiter

	fwdr   Forwarder
	scope  metrics.Scope // domain metric scope
	config *config.TaskListConfig

	cancelCtx  context.Context // used to cancel long polling
	cancelFunc context.CancelFunc
	cancelLock sync.Mutex

	tasklist     *Identifier
	tasklistKind types.TaskListKind

	numReadPartitionsFn func(*config.TaskListConfig) int
}

// ErrTasklistThrottled implies a tasklist was throttled
var ErrTasklistThrottled = errors.New("tasklist limit exceeded")

// newTaskMatcher returns a task matcher instance. The returned instance can be
// used by task producers and consumers to find a match. Both sync matches and non-sync
// matches should use this implementation
func newTaskMatcher(
	config *config.TaskListConfig,
	fwdr Forwarder,
	scope metrics.Scope,
	isolationGroups []string,
	log log.Logger,
	tasklist *Identifier,
	tasklistKind types.TaskListKind,
	limiter quotas.Limiter,
) TaskMatcher {
	isolatedTaskC := make(map[string]chan *InternalTask)
	for _, g := range isolationGroups {
		isolatedTaskC[g] = make(chan *InternalTask)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	matcher := &taskMatcherImpl{
		log:           log,
		scope:         scope,
		fwdr:          fwdr,
		taskC:         make(chan *InternalTask),
		isolatedTaskC: isolatedTaskC,
		queryTaskC:    make(chan *InternalTask),
		config:        config,
		tasklist:      tasklist,
		tasklistKind:  tasklistKind,
		limiter:       limiter,
		cancelCtx:     cancelCtx,
		cancelFunc:    cancelFunc,
	}

	return matcher
}

// DisconnectBlockedPollers gradually disconnects pollers which are blocked on long polling
func (tm *taskMatcherImpl) DisconnectBlockedPollers() {
	tm.cancelFunc()
}

// Offer offers a task to a potential consumer (poller)
// If the task is successfully matched with a consumer, this
// method will return true and no error. If the task is matched
// but consumer returned error, then this method will return
// true and error message. This method should not be used for query
// task. This method should ONLY be used for sync match.
//
// When a local poller is not available and forwarding to a parent
// task list partition is possible, this method will attempt forwarding
// to the parent partition.
//
// Cases when this method will block:
//
// Ratelimit:
// When a ratelimit token is not available, this method might block
// waiting for a token until the provided context timeout. Rate limits are
// not enforced for forwarded tasks from child partition.
//
// Forwarded tasks that originated from db backlog:
// When this method is called with a task that is forwarded from a
// remote partition and if (1) this task list is root (2) task
// was from db backlog - this method will block until context timeout
// trying to match with a poller. The caller is expected to set the
// correct context timeout.
//
// returns error when:
//   - ratelimit is exceeded (does not apply to query task)
//   - context deadline is exceeded
//   - task is matched and consumer returns error in response channel
func (tm *taskMatcherImpl) Offer(ctx context.Context, task *InternalTask) (bool, error) {
	startT := time.Now()
	if !task.IsForwarded() {
		err := tm.ratelimit(ctx)
		if err != nil {
			tm.scope.IncCounter(metrics.SyncThrottlePerTaskListCounter)
			return false, err
		}
	}
	e := event.E{
		TaskListName: tm.tasklist.GetName(),
		TaskListType: tm.tasklist.GetType(),
		TaskListKind: tm.tasklistKind.Ptr(),
	}
	localWaitTime := tm.config.LocalTaskWaitTime()
	if localWaitTime > 0 {
		childCtx, cancel := context.WithTimeout(ctx, localWaitTime)
		select {
		case tm.getTaskC(task) <- task: // poller picked up the task
			cancel()
			if task.ResponseC != nil {
				// if there is a response channel, block until resp is received
				// and return error if the response contains error
				err := <-task.ResponseC
				tm.scope.RecordTimer(metrics.SyncMatchLocalPollLatencyPerTaskList, time.Since(startT))
				if err == nil {
					e.EventName = "Offer task due to local wait"
					e.Payload = map[string]any{
						"TaskIsForwarded": task.IsForwarded(),
					}
					event.Log(e)
				}
				return true, err
			}
			return false, nil
		case <-childCtx.Done():
			cancel()
		}
	}
	select {
	case tm.getTaskC(task) <- task: // poller picked up the task
		if task.ResponseC != nil {
			// if there is a response channel, block until resp is received
			// and return error if the response contains error
			err := <-task.ResponseC
			tm.scope.RecordTimer(metrics.SyncMatchLocalPollLatencyPerTaskList, time.Since(startT))
			return true, err
		}
		return false, nil
	default:
		// no poller waiting for tasks, try forwarding this task to the
		// root partition if possible
		select {
		case token := <-tm.fwdrAddReqTokenC():
			e.EventName = "Attempting to Forward Task"
			event.Log(e)
			err := tm.fwdr.ForwardTask(ctx, task)
			token.release()
			if err == nil {
				// task was remotely sync matched on the parent partition
				tm.scope.RecordTimer(metrics.SyncMatchForwardPollLatencyPerTaskList, time.Since(startT))
				return true, nil
			}
			if errors.Is(err, ErrForwarderSlowDown) {
				tm.scope.IncCounter(metrics.SyncMatchForwardTaskThrottleErrorPerTasklist)
			}
		default:
			if !tm.isForwardingAllowed() && // we are the root partition and forwarding is not possible
				task.source == types.TaskSourceDbBacklog && // task was from backlog (stored in db)
				task.IsForwarded() { // task came from a child partition
				// a forwarded backlog task from a child partition, block trying
				// to match with a poller until ctx timeout
				return tm.OfferOrTimeout(ctx, startT, task)
			}
		}

		return false, nil
	}
}

// OfferOrTimeout offers a task to a poller and blocks until a poller picks up the task or context timeouts
func (tm *taskMatcherImpl) OfferOrTimeout(ctx context.Context, startT time.Time, task *InternalTask) (bool, error) {
	select {
	case tm.getTaskC(task) <- task: // poller picked up the task
		if task.ResponseC != nil {
			select {
			case err := <-task.ResponseC:
				tm.scope.RecordTimer(metrics.SyncMatchLocalPollLatencyPerTaskList, time.Since(startT))
				return true, err
			case <-ctx.Done():
				return false, nil
			}
		}
		return task.ActivityTaskDispatchInfo != nil, nil
	case <-ctx.Done():
		return false, nil
	}
}

// OfferQuery will either match task to local poller or will forward query task.
// Local match is always attempted before forwarding is attempted. If local match occurs
// response and error are both nil, if forwarding occurs then response or error is returned.
func (tm *taskMatcherImpl) OfferQuery(ctx context.Context, task *InternalTask) (*types.MatchingQueryWorkflowResponse, error) {
	select {
	case tm.queryTaskC <- task:
		<-task.ResponseC
		return nil, nil
	default:
	}

	fwdrTokenC := tm.fwdrAddReqTokenC()

	for {
		select {
		case tm.queryTaskC <- task:
			<-task.ResponseC
			return nil, nil
		case token := <-fwdrTokenC:
			resp, err := tm.fwdr.ForwardQueryTask(ctx, task)
			token.release()
			if err == nil {
				return resp, nil
			}
			if err == ErrForwarderSlowDown {
				// if we are rate limited, try only local match for the
				// remainder of the context timeout left
				fwdrTokenC = noopForwarderTokenC
				continue
			}
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// MustOffer blocks until a consumer is found to handle this task
// Returns error only when context is canceled, expired or the ratelimit is set to zero (allow nothing)
func (tm *taskMatcherImpl) MustOffer(ctx context.Context, task *InternalTask) error {
	e := event.E{
		TaskListName: tm.tasklist.GetName(),
		TaskListType: tm.tasklist.GetType(),
		TaskListKind: tm.tasklistKind.Ptr(),
		TaskInfo:     task.Info(),
	}
	if err := tm.ratelimit(ctx); err != nil {
		e.EventName = "Throttled While Dispatching"
		event.Log(e)
		return fmt.Errorf("rate limit error dispatching: %w", err)
	}

	startT := time.Now()
	// attempt a match with local poller first. When that
	// doesn't succeed, try both local match and remote match
	taskC := tm.getTaskC(task)
	localWaitTime := tm.config.LocalTaskWaitTime()
	childCtx, cancel := context.WithTimeout(ctx, localWaitTime)
	select {
	case taskC <- task: // poller picked up the task
		cancel()
		tm.scope.IncCounter(metrics.AsyncMatchLocalPollCounterPerTaskList)
		tm.scope.RecordTimer(metrics.AsyncMatchLocalPollLatencyPerTaskList, time.Since(startT))
		e.EventName = "Dispatched to Local Poller"
		event.Log(e)
		return nil
	case <-ctx.Done():
		cancel()
		e.EventName = "Context Done While Dispatching to Local Poller"
		event.Log(e)
		return fmt.Errorf("context done when trying to forward local task: %w", ctx.Err())
	case <-childCtx.Done():
		cancel()
	}

	attempt := 0
forLoop:
	for {
		if err := ctx.Err(); err != nil {
			e.EventName = "Context Done While Dispatching to Local or Forwarding"
			event.Log(e)
			return fmt.Errorf("failed to offer task: %w", ctx.Err())
		}
		select {
		case taskC <- task: // poller picked up the task
			e.EventName = "Dispatched to Local Poller"
			event.Log(e)
			tm.scope.IncCounter(metrics.AsyncMatchLocalPollCounterPerTaskList)
			tm.scope.RecordTimer(metrics.AsyncMatchLocalPollAttemptPerTaskList, time.Duration(attempt))
			tm.scope.RecordTimer(metrics.AsyncMatchLocalPollLatencyPerTaskList, time.Since(startT))
			return nil
		case token := <-tm.fwdrAddReqTokenC():
			e.EventName = "Attempting to Forward Task"
			event.Log(e)
			childCtx, cancel := context.WithTimeout(ctx, time.Second*2)
			err := tm.fwdr.ForwardTask(childCtx, task)
			token.release()
			if err != nil {
				if errors.Is(err, ErrForwarderSlowDown) {
					tm.scope.IncCounter(metrics.AsyncMatchForwardTaskThrottleErrorPerTasklist)
				}
				e.EventName = "Task Forwarding Failed"
				e.Payload = map[string]any{"error": err.Error()}
				event.Log(e)
				e.Payload = nil
				tm.log.Debug("failed to forward task",
					tag.Error(err),
					tag.TaskID(task.Event.TaskID),
				)
				// forwarder returns error only when the call is rate limited. To
				// avoid a busy loop on such rate limiting events, we only attempt to make
				// the next forwarded call after this childCtx expires. Till then, we block
				// hoping for a local poller match
				select {
				case taskC <- task: // poller picked up the task
					e.EventName = "Dispatched to Local Poller (after failed forward)"
					event.Log(e)
					cancel()
					tm.scope.IncCounter(metrics.AsyncMatchLocalPollAfterForwardFailedCounterPerTaskList)
					tm.scope.RecordTimer(metrics.AsyncMatchLocalPollAfterForwardFailedAttemptPerTaskList, time.Duration(attempt))
					tm.scope.RecordTimer(metrics.AsyncMatchLocalPollAfterForwardFailedLatencyPerTaskList, time.Since(startT))
					return nil
				case <-childCtx.Done():
					attempt++
					cancel()
					continue forLoop
				}
			}
			cancel()

			e.EventName = "Task Forwarded"
			event.Log(e)
			tm.scope.IncCounter(metrics.AsyncMatchForwardPollCounterPerTaskList)
			tm.scope.RecordTimer(metrics.AsyncMatchForwardPollAttemptPerTaskList, time.Duration(attempt))
			tm.scope.RecordTimer(metrics.AsyncMatchForwardPollLatencyPerTaskList, time.Since(startT))

			// at this point, we forwarded the task to a parent partition which
			// in turn dispatched the task to a poller. Make sure we delete the
			// task from the database
			task.Finish(nil)
			return nil
		case <-ctx.Done():
			e.EventName = "Context Done While Dispatching to Local or Forwarding"
			event.Log(e)
			return fmt.Errorf("failed to offer task: %w", ctx.Err())
		}
	}
}

// Poll blocks until a task is found or context deadline is exceeded
// On success, the returned task could be a query task or a regular task
// Returns ErrNoTasks when context deadline is exceeded
// Returns ErrMatcherClosed when matching is closed
func (tm *taskMatcherImpl) Poll(ctx context.Context, isolationGroup string) (*InternalTask, error) {
	startT := time.Now()
	isolatedTaskC, ok := tm.isolatedTaskC[isolationGroup]
	if !ok && isolationGroup != "" {
		// fallback to default isolation group instead of making poller crash if the isolation group is invalid
		isolatedTaskC = tm.taskC
		tm.scope.IncCounter(metrics.PollerInvalidIsolationGroupCounter)
	}

	// we want cancellation of taskMatcher to be treated as cancellation of client context
	// original context (ctx) won't be affected
	ctxWithCancelPropagation, stopFn := ctxutils.WithPropagatedContextCancel(ctx, tm.cancelCtx)
	defer stopFn()

	var task *InternalTask
	var err error
	defer func() {
		if task != nil {
			task.AutoConfigHint = &types.AutoConfigHint{
				EnableAutoConfig:   tm.config.EnableClientAutoConfig(),
				PollerWaitTimeInMs: time.Since(startT).Milliseconds(),
			}
		}
	}()

	// try local match first without blocking until context timeout
	if task, err = tm.pollNonBlocking(ctxWithCancelPropagation, isolatedTaskC, tm.taskC, tm.queryTaskC); err == nil {
		tm.scope.RecordTimer(metrics.PollLocalMatchLatencyPerTaskList, time.Since(startT))
		return task, nil
	}
	// there is no local poller available to pickup this task. Now block waiting
	// either for a local poller or a forwarding token to be available. When a
	// forwarding token becomes available, send this poll to a parent partition
	tm.log.Debug("falling back to non-local polling",
		tag.IsolationGroup(isolationGroup),
		tag.Dynamic("isolated channel", len(isolatedTaskC)),
		tag.Dynamic("fallback channel", len(tm.taskC)),
	)
	event.Log(event.E{
		TaskListName: tm.tasklist.GetName(),
		TaskListType: tm.tasklist.GetType(),
		TaskListKind: tm.tasklistKind.Ptr(),
		EventName:    "Matcher Falling Back to Non-Local Polling",
	})
	task, err = tm.pollOrForward(ctxWithCancelPropagation, startT, isolationGroup, isolatedTaskC, tm.taskC, tm.queryTaskC)
	return task, err
}

// PollForQuery blocks until a *query* task is found or context deadline is exceeded
// Returns ErrNoTasks when context deadline is exceeded
// Returns ErrMatcherClosed when matching is closed
func (tm *taskMatcherImpl) PollForQuery(ctx context.Context) (*InternalTask, error) {
	startT := time.Now()
	// try local match first without blocking until context timeout
	if task, err := tm.pollNonBlocking(ctx, nil, nil, tm.queryTaskC); err == nil {
		tm.scope.RecordTimer(metrics.PollLocalMatchLatencyPerTaskList, time.Since(startT))
		return task, nil
	}

	ctxWithCancelPropagation, stopFn := ctxutils.WithPropagatedContextCancel(ctx, tm.cancelCtx)
	defer stopFn()

	// there is no local poller available to pickup this task. Now block waiting
	// either for a local poller or a forwarding token to be available. When a
	// forwarding token becomes available, send this poll to a parent partition
	return tm.pollOrForward(ctxWithCancelPropagation, startT, "", nil, nil, tm.queryTaskC)
}

func (tm *taskMatcherImpl) RefreshCancelContext() {
	tm.cancelLock.Lock()
	defer tm.cancelLock.Unlock()
	tm.cancelCtx, tm.cancelFunc = context.WithCancel(context.Background())
}

func (tm *taskMatcherImpl) pollOrForward(
	ctx context.Context,
	startT time.Time,
	isolationGroup string,
	isolatedTaskC <-chan *InternalTask,
	taskC <-chan *InternalTask,
	queryTaskC <-chan *InternalTask,
) (*InternalTask, error) {
	select {
	case task := <-isolatedTaskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.RecordTimer(metrics.PollLocalMatchLatencyPerTaskList, time.Since(startT))
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task (pollOrForward)",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": true,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-taskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.RecordTimer(metrics.PollLocalMatchLatencyPerTaskList, time.Since(startT))
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task (pollOrForward)",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": false,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-queryTaskC:
		tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		return task, nil
	case <-ctx.Done():
		tm.scope.IncCounter(metrics.PollTimeoutPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			EventName:    "Poll Timeout",
		})
		return nil, ErrNoTasks
	case token := <-tm.fwdrPollReqTokenC():
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			EventName:    "Attempting to Forward Poll",
			Payload: map[string]any{
				"IsolationGroup": isolationGroup,
			},
		})
		if task, err := tm.fwdr.ForwardPoll(ctx); err == nil {
			token.release()
			tm.scope.RecordTimer(metrics.PollForwardMatchLatencyPerTaskList, time.Since(startT))
			event.Log(event.E{
				TaskListName: tm.tasklist.GetName(),
				TaskListType: tm.tasklist.GetType(),
				TaskListKind: tm.tasklistKind.Ptr(),
				EventName:    "Forwarded Poll returned task",
			})
			return task, nil
		}
		token.release()
		return tm.poll(ctx, startT, isolatedTaskC, taskC, queryTaskC)
	}
}

func (tm *taskMatcherImpl) poll(
	ctx context.Context,
	startT time.Time,
	isolatedTaskC <-chan *InternalTask,
	taskC <-chan *InternalTask,
	queryTaskC <-chan *InternalTask,
) (*InternalTask, error) {
	select {
	case task := <-isolatedTaskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.RecordTimer(metrics.PollLocalMatchAfterForwardFailedLatencyPerTaskList, time.Since(startT))
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task (poll)",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": true,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-taskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.RecordTimer(metrics.PollLocalMatchAfterForwardFailedLatencyPerTaskList, time.Since(startT))
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task (poll)",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": false,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-queryTaskC:
		tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		return task, nil
	case <-ctx.Done():
		tm.scope.IncCounter(metrics.PollTimeoutPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			EventName:    "Poll Timeout",
		})
		return nil, ErrNoTasks
	}
}

func (tm *taskMatcherImpl) pollLocalWait(
	ctx context.Context,
	isolatedTaskC <-chan *InternalTask,
	taskC <-chan *InternalTask,
	queryTaskC <-chan *InternalTask,
) (*InternalTask, error) {
	select {
	case task := <-isolatedTaskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task Nonblocking",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": true,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-taskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task Nonblocking",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": false,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-queryTaskC:
		tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		return task, nil
	case <-ctx.Done():
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			EventName:    "Matcher Found No Tasks Nonblocking",
		})
		return nil, ErrNoTasks
	}
}

func (tm *taskMatcherImpl) pollNonBlocking(
	ctx context.Context,
	isolatedTaskC <-chan *InternalTask,
	taskC <-chan *InternalTask,
	queryTaskC <-chan *InternalTask,
) (*InternalTask, error) {
	waitTime := tm.config.LocalPollWaitTime()
	if waitTime > 0 {
		childCtx, cancel := context.WithTimeout(ctx, waitTime)
		defer cancel()
		return tm.pollLocalWait(childCtx, isolatedTaskC, taskC, queryTaskC)
	}
	select {
	case task := <-isolatedTaskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task Nonblocking",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": true,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-taskC:
		if task.ResponseC != nil {
			tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		}
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			TaskInfo:     task.Info(),
			EventName:    "Matched Task Nonblocking",
			Payload: map[string]any{
				"TaskIsForwarded":   task.IsForwarded(),
				"SyncMatched":       task.ResponseC != nil,
				"FromIsolatedTaskC": false,
				"IsolationGroup":    task.isolationGroup,
			},
		})
		return task, nil
	case task := <-queryTaskC:
		tm.scope.IncCounter(metrics.PollSuccessWithSyncPerTaskListCounter)
		tm.scope.IncCounter(metrics.PollSuccessPerTaskListCounter)
		return task, nil
	default:
		event.Log(event.E{
			TaskListName: tm.tasklist.GetName(),
			TaskListType: tm.tasklist.GetType(),
			TaskListKind: tm.tasklistKind.Ptr(),
			EventName:    "Matcher Found No Tasks Nonblocking",
		})
		return nil, ErrNoTasks
	}
}

func (tm *taskMatcherImpl) fwdrPollReqTokenC() <-chan *ForwarderReqToken {
	if tm.fwdr == nil {
		return noopForwarderTokenC
	}
	return tm.fwdr.PollReqTokenC()
}

func (tm *taskMatcherImpl) fwdrAddReqTokenC() <-chan *ForwarderReqToken {
	if tm.fwdr == nil {
		return noopForwarderTokenC
	}
	return tm.fwdr.AddReqTokenC()
}

func (tm *taskMatcherImpl) ratelimit(ctx context.Context) error {
	err := tm.limiter.Wait(ctx)
	if errors.Is(err, clock.ErrCannotWait) {
		// "err != ctx.Err()" may also be correct, as that would mean "gave up due to context".
		//
		// in this branch: either the request would wait longer than ctx's timeout,
		// or the limiter's config does not allow any operations at all (burst 0).
		// in either case, this is returned immediately.
		return ErrTasklistThrottled
	}
	return err // nil if success, non-nil if canceled
}

func (tm *taskMatcherImpl) isForwardingAllowed() bool {
	return tm.fwdr != nil
}

func (tm *taskMatcherImpl) getTaskC(task *InternalTask) chan<- *InternalTask {
	taskC := tm.taskC
	if isolatedTaskC, ok := tm.isolatedTaskC[task.isolationGroup]; ok && task.isolationGroup != "" {
		taskC = isolatedTaskC
	}
	return taskC
}
