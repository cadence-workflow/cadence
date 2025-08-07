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
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

const testDomainName = "test-domain"

type MatcherTestSuite struct {
	suite.Suite
	controller      *gomock.Controller
	client          *matching.MockClient
	fwdr            Forwarder
	cfg             *config.TaskListConfig
	taskList        *Identifier
	matcher         *taskMatcherImpl // matcher for child partition
	rootMatcher     TaskMatcher      // matcher for parent partition
	isolationGroups []string
}

func TestMatcherSuite(t *testing.T) {
	suite.Run(t, new(MatcherTestSuite))
}

func (t *MatcherTestSuite) SetupTest() {
	t.controller = gomock.NewController(t.T())
	t.client = matching.NewMockClient(t.controller)
	cfg := config.NewConfig(dynamicconfig.NewNopCollection(), "some random hostname", func() []string { return nil })
	cfg.TaskDispatchRPSTTL = 0
	t.taskList = NewTestTaskListID(t.T(), uuid.New(), constants.ReservedTaskListPrefix+"tl0/1", persistence.TaskListTypeDecision)
	tlCfg := newTaskListConfig(t.taskList, cfg, testDomainName)

	tlCfg.ForwarderConfig = config.ForwarderConfig{
		ForwarderMaxOutstandingPolls: func() int { return 1 },
		ForwarderMaxOutstandingTasks: func() int { return 1 },
		ForwarderMaxRatePerSecond:    func() int { return 2 },
		ForwarderMaxChildrenPerNode:  func() int { return 20 },
	}
	t.cfg = tlCfg
	t.isolationGroups = []string{"dca1", "dca2"}
	t.fwdr = newForwarder(&t.cfg.ForwarderConfig, t.taskList, types.TaskListKindNormal, t.client, metrics.NoopScope)
	t.matcher = newTaskMatcher(tlCfg, t.fwdr, metrics.NoopScope, []string{"dca1", "dca2"}, log.NewNoop(), t.taskList, types.TaskListKindNormal, clock.NewRatelimiter(rate.Limit(100), 100)).(*taskMatcherImpl)

	rootTaskList := NewTestTaskListID(t.T(), t.taskList.GetDomainID(), t.taskList.Parent(20), persistence.TaskListTypeDecision)
	rootTasklistCfg := newTaskListConfig(rootTaskList, cfg, testDomainName)
	t.rootMatcher = newTaskMatcher(rootTasklistCfg, nil, metrics.NoopScope, []string{"dca1", "dca2"}, log.NewNoop(), t.taskList, types.TaskListKindNormal, clock.NewRatelimiter(rate.Limit(100), 100)).(*taskMatcherImpl)
}

func (t *MatcherTestSuite) TearDownTest() {
	t.controller.Finish()
}

func (t *MatcherTestSuite) TestLocalSyncMatch() {
	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	syncMatch, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()
	t.NoError(err)
	t.True(syncMatch)
}

func (t *MatcherTestSuite) TestIsolationLocalSyncMatch() {
	const isolationGroup = "dca1"

	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, isolationGroup)
		if err == nil {
			task.Finish(nil)
		}
	})

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "dca1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	syncMatch, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()
	t.NoError(err)
	t.True(syncMatch)
}

func (t *MatcherTestSuite) TestRemoteSyncMatch() {
	t.testRemoteSyncMatch(types.TaskSourceHistory, "")
}

func (t *MatcherTestSuite) TestIsolationRemoteSyncMatch() {
	t.testRemoteSyncMatch(types.TaskSourceHistory, "dca1")
}

func (t *MatcherTestSuite) TestRemoteSyncMatchBlocking() {
	t.testRemoteSyncMatch(types.TaskSourceDbBacklog, "")
}

func (t *MatcherTestSuite) TestIsolationRemoteSyncMatchBlocking() {
	t.testRemoteSyncMatch(types.TaskSourceDbBacklog, "dca1")
}

func (t *MatcherTestSuite) testRemoteSyncMatch(taskSource types.TaskSource, isolationGroup string) {
	pollSigC := make(chan struct{})

	bgctx, bgcancel := context.WithTimeout(context.Background(), time.Second)
	go func() {
		<-pollSigC
		if taskSource == types.TaskSourceDbBacklog {
			// when task is from dbBacklog, sync match SHOULD block
			// so lets delay polling by a bit to verify that
			time.Sleep(time.Millisecond * 10)
		}
		task, err := t.matcher.Poll(bgctx, isolationGroup)
		bgcancel()
		if err == nil && !task.IsStarted() {
			task.Finish(nil)
		}
	}()

	t.client.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest, option ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
			task, err := t.rootMatcher.Poll(arg0, isolationGroup)
			if err != nil {
				return nil, err
			}

			task.Finish(nil)
			return &types.MatchingPollForDecisionTaskResponse{
				WorkflowExecution: task.WorkflowExecution(),
			}, nil
		},
	).AnyTimes()

	task := newInternalTask(t.newTaskInfo(), nil, taskSource, "", true, nil, isolationGroup)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	var err error
	var remoteSyncMatch bool
	var req *types.AddDecisionTaskRequest
	t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.AddDecisionTaskRequest, option ...yarpc.CallOption) {
			req = arg1
			task.forwardedFrom = req.GetForwardedFrom()
			close(pollSigC)
			if taskSource != types.TaskSourceDbBacklog {
				// when task is not from backlog, wait a bit for poller to arrive first
				// when task is from backlog, offer blocks, so we don't need to do this
				time.Sleep(time.Millisecond)
				// Sleep for local poll wait time so that poller will be forwarded to root partition
				time.Sleep(t.matcher.config.LocalPollWaitTime())
			}
			remoteSyncMatch, err = t.rootMatcher.Offer(ctx, task)
		},
	).Return(&types.AddDecisionTaskResponse{}, nil)

	_, err0 := t.matcher.Offer(ctx, task)
	t.NoError(err0)
	cancel()
	<-bgctx.Done() // wait for async work to finish
	t.NotNil(req)
	t.NoError(err)
	t.True(remoteSyncMatch)
	t.Equal(t.taskList.name, req.GetForwardedFrom())
	t.Equal(t.taskList.Parent(20), req.GetTaskList().GetName())
}

func (t *MatcherTestSuite) TestSyncMatchFailure() {
	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	var req *types.AddDecisionTaskRequest
	t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.AddDecisionTaskRequest, option ...yarpc.CallOption) {
			req = arg1
		},
	).Return(nil, &types.ServiceBusyError{})

	syncMatch, err := t.matcher.Offer(ctx, task)
	cancel()
	t.NotNil(req)
	t.NoError(err)
	t.False(syncMatch)
}

func (t *MatcherTestSuite) TestRateLimitHandling() {
	scope := mocks.Scope{}
	scope.On("IncCounter", metrics.SyncMatchForwardTaskThrottleErrorPerTasklist)
	scope.On("RecordTimer", mock.Anything, mock.Anything)
	t.matcher.scope = &scope
	for i := 0; i < 5; i++ {
		t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Return(&types.AddDecisionTaskResponse{}, nil).AnyTimes()
		task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := t.matcher.Offer(ctx, task)
		cancel()
		assert.NoError(t.T(), err)
	}
}

func (t *MatcherTestSuite) TestIsolationSyncMatchFailure() {
	const isolationGroup = "dca2"

	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, isolationGroup)
		if err == nil {
			task.Finish(nil)
		}
	})
	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "dca1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	syncMatch, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()
	t.NoError(err)
	t.False(syncMatch)
}

func (t *MatcherTestSuite) TestQueryLocalSyncMatch() {
	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.PollForQuery(ctx)
		if err == nil && task.IsQuery() {
			task.Finish(nil)
		}
	})

	task := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	resp, err := t.matcher.OfferQuery(ctx, task)
	cancel()
	wait()
	t.NoError(err)
	t.Nil(resp)
}

func (t *MatcherTestSuite) TestQueryRemoteSyncMatch() {
	ready, wait := ensureAsyncAfterReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.PollForQuery(ctx)
		if err == nil && task.IsQuery() {
			task.Finish(nil)
		}
	})

	var remotePollResp *types.MatchingPollForDecisionTaskResponse
	t.client.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest, option ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
			task, err := t.rootMatcher.PollForQuery(arg0)
			if err != nil {
				return nil, err
			} else if task.IsQuery() {
				task.Finish(nil)
				res := &types.MatchingPollForDecisionTaskResponse{
					Query: &types.WorkflowQuery{},
				}
				remotePollResp = res
				return res, nil
			}
			return nil, nil
		},
	).AnyTimes()

	task := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	var req *types.MatchingQueryWorkflowRequest
	t.client.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.MatchingQueryWorkflowRequest, option ...yarpc.CallOption) {
			req = arg1
			task.forwardedFrom = req.GetForwardedFrom()
			ready()
			t.rootMatcher.OfferQuery(ctx, task)
		},
	).Return(&types.MatchingQueryWorkflowResponse{QueryResult: []byte("answer")}, nil)

	result, err := t.matcher.OfferQuery(ctx, task)
	cancel()
	wait()
	t.NotNil(req)
	t.NoError(err)
	t.NotNil(result)
	t.NotNil(remotePollResp.Query)
	t.Equal("answer", string(result.QueryResult))
	t.Equal(t.taskList.name, req.GetForwardedFrom())
	t.Equal(t.taskList.Parent(20), req.GetTaskList().GetName())
}

func (t *MatcherTestSuite) TestQueryRemoteSyncMatchError() {
	<-t.fwdr.PollReqTokenC()

	matched := false
	ready, wait := ensureAsyncAfterReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.PollForQuery(ctx)
		if err == nil && task.IsQuery() {
			matched = true
			task.Finish(nil)
		}
	})

	task := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	var req *types.MatchingQueryWorkflowRequest
	t.client.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.MatchingQueryWorkflowRequest, option ...yarpc.CallOption) {
			req = arg1
			ready()
		},
	).Return(nil, &types.ServiceBusyError{})

	result, err := t.matcher.OfferQuery(ctx, task)
	cancel()
	wait()
	t.NotNil(req)
	t.NoError(err)
	t.Nil(result)
	t.True(matched)
}

func (t *MatcherTestSuite) TestMustOfferLocalMatch() {
	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := t.matcher.MustOffer(ctx, task)
	cancel()
	wait()
	t.NoError(err)
}

func (t *MatcherTestSuite) TestIsolationMustOfferLocalMatch() {
	// force disable remote forwarding
	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "dca1")
		if err == nil {
			task.Finish(nil)
		}
	})

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "dca1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := t.matcher.MustOffer(ctx, task)
	cancel()
	wait()
	t.NoError(err)
}

func (t *MatcherTestSuite) TestMustOfferRemoteMatch() {
	pollSigC := make(chan struct{})
	forwardPollSigC := make(chan struct{})

	t.client.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest, option ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
			close(forwardPollSigC)
			<-pollSigC
			time.Sleep(time.Millisecond * 500) // delay poll to verify that offer blocks on parent
			task, err := t.rootMatcher.Poll(arg0, "")
			if err != nil {
				return nil, err
			}

			task.Finish(nil)
			return &types.MatchingPollForDecisionTaskResponse{
				WorkflowExecution: task.WorkflowExecution(),
			}, nil
		},
	).AnyTimes()

	taskCompleted := false
	completionFunc := func(*persistence.TaskInfo, error) {
		taskCompleted = true
	}

	task := newInternalTask(t.newTaskInfo(), completionFunc, types.TaskSourceDbBacklog, "", false, nil, "")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)

	var err error
	var remoteSyncMatch bool
	var req *types.AddDecisionTaskRequest
	t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Return(nil, &types.ServiceBusyError{}).Times(1)
	t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.AddDecisionTaskRequest, option ...yarpc.CallOption) {
			req = arg1
			task := newInternalTask(task.Event.TaskInfo, nil, types.TaskSourceDbBacklog, req.GetForwardedFrom(), true, nil, "")
			close(pollSigC)
			remoteSyncMatch, err = t.rootMatcher.Offer(ctx, task)
		},
	).Return(&types.AddDecisionTaskResponse{}, nil)

	// Poll needs to happen before MustOffer, or else it goes into the non-blocking path.
	var pollResultMu sync.Mutex
	var polledTask *InternalTask
	var pollErr error
	wait := ensureAsyncReady(time.Second*3, func(ctx context.Context) {
		pollResultMu.Lock()
		defer pollResultMu.Unlock()
		polledTask, pollErr = t.matcher.Poll(ctx, "")
	})
	<-forwardPollSigC

	t.NoError(t.matcher.MustOffer(ctx, task))
	cancel()
	wait()
	pollResultMu.Lock()
	defer pollResultMu.Unlock()
	t.NoError(pollErr)
	t.NotNil(polledTask)
	t.NotNil(req)
	t.NoError(err)
	t.True(remoteSyncMatch)
	t.True(taskCompleted)
	t.Equal(t.taskList.name, req.GetForwardedFrom())
	t.Equal(t.taskList.Parent(20), req.GetTaskList().GetName())
}

func (t *MatcherTestSuite) TestMustOfferRemoteRateLimit() {
	scope := mocks.Scope{}
	scope.On("IncCounter", metrics.AsyncMatchForwardTaskThrottleErrorPerTasklist)
	scope.On("RecordTimer", mock.Anything, mock.Anything)
	t.matcher.scope = &scope
	completionFunc := func(*persistence.TaskInfo, error) {}
	for i := 0; i < 5; i++ {
		scope.On("IncCounter", metrics.AsyncMatchForwardPollCounterPerTaskList)
		t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Return(&types.AddDecisionTaskResponse{}, nil)
		task := newInternalTask(t.newTaskInfo(), completionFunc, types.TaskSourceDbBacklog, "", false, nil, "")
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		t.NoError(t.matcher.MustOffer(ctx, task))
		cancel()
	}
}

func (t *MatcherTestSuite) TestIsolationMustOfferRemoteMatch() {
	pollSigC := make(chan struct{})
	forwardPollSigC := make(chan struct{})

	t.client.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest, option ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
			close(forwardPollSigC)
			<-pollSigC
			time.Sleep(time.Millisecond * 500) // delay poll to verify that offer blocks on parent
			task, err := t.rootMatcher.Poll(arg0, "dca1")
			if err != nil {
				return nil, err
			}

			task.Finish(nil)
			return &types.MatchingPollForDecisionTaskResponse{
				WorkflowExecution: task.WorkflowExecution(),
			}, nil
		},
	).AnyTimes()

	taskCompleted := false
	completionFunc := func(*persistence.TaskInfo, error) {
		taskCompleted = true
	}

	task := newInternalTask(t.newTaskInfo(), completionFunc, types.TaskSourceDbBacklog, "", false, nil, "dca1")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)

	var err error
	var remoteSyncMatch bool
	var req *types.AddDecisionTaskRequest
	t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Return(nil, &types.ServiceBusyError{}).Times(1)
	t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.AddDecisionTaskRequest, option ...yarpc.CallOption) {
			req = arg1
			task := newInternalTask(task.Event.TaskInfo, nil, types.TaskSourceDbBacklog, req.GetForwardedFrom(), true, nil, "dca1")
			close(pollSigC)
			remoteSyncMatch, err = t.rootMatcher.Offer(arg0, task)
		},
	).Return(&types.AddDecisionTaskResponse{}, nil)

	// Poll needs to happen before MustOffer, or else it goes into the non-blocking path.
	var pollResultMu sync.Mutex
	var polledTask *InternalTask
	var pollErr error
	wait := ensureAsyncReady(time.Second*3, func(ctx context.Context) {
		pollResultMu.Lock()
		defer pollResultMu.Unlock()
		polledTask, pollErr = t.matcher.Poll(ctx, "dca1")
	})
	<-forwardPollSigC

	t.NoError(t.matcher.MustOffer(ctx, task))
	cancel()
	wait()
	pollResultMu.Lock()
	defer pollResultMu.Unlock()
	t.NoError(pollErr)
	t.NotNil(polledTask)
	t.NotNil(req)
	t.NoError(err)
	t.True(remoteSyncMatch)
	t.True(taskCompleted)
	t.Equal(t.taskList.name, req.GetForwardedFrom())
	t.Equal(t.taskList.Parent(20), req.GetTaskList().GetName())
}

func (t *MatcherTestSuite) TestPollersDisconnectedAfterDisconnectBlockedPollers() {
	defer goleak.VerifyNone(t.T())
	t.disableRemoteForwarding()
	t.matcher.DisconnectBlockedPollers()

	longPollingCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	task, err := t.matcher.Poll(longPollingCtx, "")
	t.ErrorIs(err, ErrNoTasks, "closed matcher should result in no tasks")
	t.Nil(task)
	t.NoError(longPollingCtx.Err(), "the parent context was not cancelled, the child context was cancelled")
}

func (t *MatcherTestSuite) TestPollersConnectedAfterDisconnectBlockedPollersSetCancelContext() {
	defer goleak.VerifyNone(t.T())
	t.disableRemoteForwarding()
	t.matcher.DisconnectBlockedPollers()
	t.matcher.RefreshCancelContext()

	longPollingCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	task, err := t.matcher.Poll(longPollingCtx, "")
	t.ErrorIs(err, ErrNoTasks, "no tasks to be matched to poller")
	t.Nil(task)
	t.ErrorIs(longPollingCtx.Err(), context.DeadlineExceeded, "the child context wasn't cancelled, the parent context timed out")
}

func (t *MatcherTestSuite) TestRemotePoll() {
	pollToken := <-t.fwdr.PollReqTokenC()

	var req *types.MatchingPollForDecisionTaskRequest
	t.client.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest, option ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
			req = arg1
			return nil, nil
		},
	)

	ready, wait := ensureAsyncAfterReady(0, func(_ context.Context) {
		pollToken.release()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	ready()
	task, err := t.matcher.Poll(ctx, "")
	cancel()
	wait()
	t.NoError(err)
	t.NotNil(req)
	t.NotNil(task)
	t.True(task.IsStarted())
}

func (t *MatcherTestSuite) TestRemotePollForQuery() {
	pollToken := <-t.fwdr.PollReqTokenC()

	var req *types.MatchingPollForDecisionTaskRequest
	t.client.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest, option ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
			req = arg1
			return nil, nil
		},
	)

	ready, wait := ensureAsyncAfterReady(0, func(_ context.Context) {
		pollToken.release()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ready()
	task, err := t.matcher.PollForQuery(ctx)
	wait()
	t.NoError(err)
	t.NotNil(req)
	t.NotNil(task)
	t.True(task.IsStarted())
}

func (t *MatcherTestSuite) TestIsolationPollFailure() {
	t.disableRemoteForwarding()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	task, err := t.matcher.Poll(ctx, "invalid-group")
	cancel()
	t.Error(err)
	t.Nil(task)
}

func (t *MatcherTestSuite) TestOffer_RateLimited() {
	t.matcher.limiter = clock.NewRatelimiter(0, 0)
	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	ctx := context.Background()

	matched, err := t.matcher.Offer(ctx, task)

	t.ErrorIs(err, ErrTasklistThrottled)
	t.False(matched)
}

func (t *MatcherTestSuite) TestOffer_NoTimeoutSyncMatchedNoError() {
	defer goleak.VerifyNone(t.T())

	t.matcher.config.LocalTaskWaitTime = func() time.Duration { return 0 }

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	t.disableRemoteForwarding()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	syncMatched, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()

	t.NoError(err)
	t.True(syncMatched)
}

func (t *MatcherTestSuite) TestOffer_NoTimeoutSyncMatchedError() {
	defer goleak.VerifyNone(t.T())

	t.matcher.config.LocalTaskWaitTime = func() time.Duration { return 0 }

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	t.disableRemoteForwarding()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	task.ResponseC <- errShutdown

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	syncMatched, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()

	t.Error(err)
	t.True(syncMatched)
}

func (t *MatcherTestSuite) TestOffer_NoTimeoutAsyncMatchedNoError() {
	defer goleak.VerifyNone(t.T())

	t.matcher.config.LocalTaskWaitTime = func() time.Duration { return 0 }

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	t.disableRemoteForwarding()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	syncMatched, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()

	t.NoError(err)
	t.False(syncMatched)
}

func (t *MatcherTestSuite) TestOffer_AsyncMatchedNoError() {
	defer goleak.VerifyNone(t.T())

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	t.disableRemoteForwarding()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	syncMatch, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()

	t.NoError(err)
	t.False(syncMatch)
}

func (t *MatcherTestSuite) TestOfferOrTimeout_SyncMatchTimedOut() {
	defer goleak.VerifyNone(t.T())

	t.disableRemoteForwarding()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		time.Sleep(time.Millisecond * 100)
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	matched, err := t.matcher.OfferOrTimeout(ctx, time.Now(), task)
	wait()

	t.NoError(err)
	t.False(matched)
}

func (t *MatcherTestSuite) TestOfferOrTimeout_AsyncMatchNotMatched() {
	defer goleak.VerifyNone(t.T())

	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	matched, err := t.matcher.OfferOrTimeout(ctx, time.Now(), task)
	cancel()
	wait()

	t.NoError(err)
	t.False(matched)
}

func (t *MatcherTestSuite) TestOfferOrTimeout_AsyncMatchMatched() {
	defer goleak.VerifyNone(t.T())

	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		if err == nil {
			task.Finish(nil)
		}
	})

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, &types.ActivityTaskDispatchInfo{}, "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	matched, err := t.matcher.OfferOrTimeout(ctx, time.Now(), task)
	cancel()
	wait()

	t.NoError(err)
	t.True(matched)
}

func (t *MatcherTestSuite) TestOfferOrTimeout_TimedOut() {
	t.disableRemoteForwarding()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, &types.ActivityTaskDispatchInfo{}, "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	cancel()

	matched, err := t.matcher.OfferOrTimeout(ctx, time.Now(), task)

	t.NoError(err)
	t.False(matched)
}

func (t *MatcherTestSuite) TestOfferQuery_ForwardError() {
	ctx := context.Background()
	task := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	fn := func() <-chan *ForwarderReqToken {
		c := make(chan *ForwarderReqToken, 1)
		c <- &ForwarderReqToken{
			ch: make(chan *ForwarderReqToken, 1),
		}
		return c
	}

	mockForwarder.EXPECT().AddReqTokenC().Return(fn()).Times(1)
	mockForwarder.EXPECT().ForwardQueryTask(ctx, task).Return(nil, ErrNoParent).Times(1)

	retTask, err := t.matcher.OfferQuery(ctx, task)

	t.ErrorIs(err, ErrNoParent)
	t.Nil(retTask)
}

func (t *MatcherTestSuite) TestOfferQuery_ContextExpired() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	cancel()

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	task := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})

	mockForwarder.EXPECT().AddReqTokenC().Times(1)

	retTask, err := t.matcher.OfferQuery(ctx, task)

	t.ErrorIs(err, context.Canceled)
	t.Nil(retTask)
}

func (t *MatcherTestSuite) TestMustOffer_ContextExpiredFirstAttempt() {
	ctx, cancel := context.WithTimeout(context.Background(), t.matcher.config.LocalTaskWaitTime())
	defer cancel()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	err := t.matcher.MustOffer(ctx, task)

	t.ErrorIs(err, context.DeadlineExceeded)
}

func (t *MatcherTestSuite) TestMustOffer_ContextExpiredAfterFirstAttempt() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*t.matcher.config.LocalTaskWaitTime())
	defer cancel()

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	mockForwarder.EXPECT().AddReqTokenC().Times(1)

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	err := t.matcher.MustOffer(ctx, task)

	t.ErrorIs(err, context.DeadlineExceeded)
}

func (t *MatcherTestSuite) TestMustOffer_LocalMatchAfterChildCtxExpired() {
	ctx := context.Background()

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	t.disableRemoteForwarding()

	mockForwarder.EXPECT().AddReqTokenC().AnyTimes()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	go func() {
		// Waits for the child context to expire; by default, it is 10 ms.
		// Forces no forwarding and the context background will never expire, therefore guaranteeing that the poller
		// will pick up the task after the first attempt.
		time.Sleep(200 * time.Millisecond)
		retTask, err := t.matcher.Poll(ctx, "")
		if err == nil {
			retTask.Finish(nil)
		}
	}()

	err := t.matcher.MustOffer(ctx, task)

	t.NoError(err)
}

func (t *MatcherTestSuite) TestMustOffer_LocalMatchAfterForwardError() {
	ctx := context.Background()

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	forwardToken := &ForwarderReqToken{
		ch: make(chan *ForwarderReqToken, 1),
	}

	fn := func() <-chan *ForwarderReqToken {
		c := make(chan *ForwarderReqToken, 1)
		c <- forwardToken
		return c
	}

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	mockForwarder.EXPECT().AddReqTokenC().Return(fn()).Times(1)
	mockForwarder.EXPECT().ForwardTask(gomock.Any(), task).Return(ErrNoParent).Times(1)

	go func() {
		<-forwardToken.ch
		retTask, err := t.matcher.Poll(ctx, "")
		if err == nil {
			retTask.Finish(nil)
		}
	}()

	err := t.matcher.MustOffer(ctx, task)

	t.NoError(err)
}

func (t *MatcherTestSuite) TestMustOffer_ContextExpiredAfterForwardError() {
	ctx, cancel := context.WithCancel(context.Background())

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	forwardToken := &ForwarderReqToken{
		ch: make(chan *ForwarderReqToken, 1),
	}

	// Have 1 token ready and waiting
	c := make(chan *ForwarderReqToken, 1)
	c <- forwardToken

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", false, nil, "")

	mockForwarder.EXPECT().AddReqTokenC().Return(c).Times(1)
	mockForwarder.EXPECT().ForwardTask(gomock.Any(), task).Return(ErrNoParent).Times(1)

	go func() {
		// Wait for the token to be returned
		<-forwardToken.ch
		// using cancel here to simulate the context being expired
		cancel()
	}()

	err := t.matcher.MustOffer(ctx, task)
	t.ErrorIs(err, context.Canceled)
	t.ErrorContains(err, "failed to offer task:")
}

func (t *MatcherTestSuite) Test_pollOrForward_PollIsolatedTask() {
	ctx := context.Background()
	startT := time.Now()
	isolationGroup := "dca1"
	isolatedTaskC := make(chan *InternalTask, 1)

	t.disableRemoteForwarding()

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	// Mock the fwdrPollReqTokenC method to return a controlled channel
	mockTokenC := make(chan *ForwarderReqToken)
	mockForwarder.EXPECT().PollReqTokenC().Return(mockTokenC).AnyTimes()

	// Test pollOrForward for isolated task - poll
	isolatedTask := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, isolationGroup)
	isolatedTaskC <- isolatedTask
	retTask, err := t.matcher.pollOrForward(ctx, startT, isolationGroup, isolatedTaskC, nil, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(isolatedTask, retTask)
}

func (t *MatcherTestSuite) Test_pollOrForward_PollTask() {
	ctx := context.Background()
	startT := time.Now()
	taskC := make(chan *InternalTask, 1)

	t.disableRemoteForwarding()

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	// Mock the fwdrPollReqTokenC method to return a controlled channel
	mockTokenC := make(chan *ForwarderReqToken)
	mockForwarder.EXPECT().PollReqTokenC().Return(mockTokenC).AnyTimes()

	// Test pollOrForward for regular task - poll
	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	taskC <- task
	retTask, err := t.matcher.pollOrForward(ctx, startT, "", nil, taskC, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(task, retTask)
}

func (t *MatcherTestSuite) Test_pollOrForward_PollQueryTask() {
	ctx := context.Background()
	startT := time.Now()
	queryTaskC := make(chan *InternalTask, 1)
	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	// Mock the fwdrPollReqTokenC method to return a controlled channel
	mockTokenC := make(chan *ForwarderReqToken)
	mockForwarder.EXPECT().PollReqTokenC().Return(mockTokenC).AnyTimes()

	// Test pollOrForward for query task - poll
	queryTask := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})
	queryTaskC <- queryTask
	retTask, err := t.matcher.pollOrForward(ctx, startT, "", nil, nil, queryTaskC)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(queryTask, retTask)
}

func (t *MatcherTestSuite) Test_pollOrForward_ForwardTask() {
	ctx := context.Background()
	startT := time.Now()
	isolationGroup := "dca1"

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	// Mock the fwdrPollReqTokenC method to return a controlled channel
	mockTokenC := make(chan *ForwarderReqToken, 1)
	forwardToken := &ForwarderReqToken{
		ch: make(chan *ForwarderReqToken, 1),
	}
	mockTokenC <- forwardToken
	mockForwarder.EXPECT().PollReqTokenC().Return(mockTokenC).AnyTimes()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	mockForwarder.EXPECT().ForwardPoll(ctx).Return(task, nil).Times(1)

	retTask, err := t.matcher.pollOrForward(ctx, startT, isolationGroup, nil, nil, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(task, retTask)
}

func (t *MatcherTestSuite) Test_pollOrForward_ForwardTaskThenPoll() {
	ctx := context.Background()
	startT := time.Now()
	isolationGroup := "dca1"
	taskC := make(chan *InternalTask, 1)

	mockForwarder := NewMockForwarder(t.controller)
	t.matcher.fwdr = mockForwarder

	// Mock the fwdrPollReqTokenC method to return a controlled channel
	mockTokenC := make(chan *ForwarderReqToken, 1)
	forwardToken := &ForwarderReqToken{
		ch: make(chan *ForwarderReqToken, 1),
	}
	mockTokenC <- forwardToken
	mockForwarder.EXPECT().PollReqTokenC().Return(mockTokenC).AnyTimes()

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	mockForwarder.EXPECT().ForwardPoll(ctx).Return(nil, ErrNoParent).Times(1)

	// Add the task after the forwarderReqToken is released
	// It's not a race condition because the PollReqTokenC is mocked and does not use the isolatedCh
	go func() {
		select {
		case <-forwardToken.ch:
			taskC <- task
		}
	}()

	retTask, err := t.matcher.pollOrForward(ctx, startT, isolationGroup, nil, taskC, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(task, retTask)
}

func (t *MatcherTestSuite) Test_poll_IsolatedTask() {
	ctx := context.Background()
	startT := time.Now()
	isolationGroup := "dca1"
	isolatedTaskC := make(chan *InternalTask, 1)

	isolatedTask := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, isolationGroup)
	isolatedTaskC <- isolatedTask
	retTask, err := t.matcher.poll(ctx, startT, isolatedTaskC, nil, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(isolatedTask, retTask)
}

func (t *MatcherTestSuite) Test_poll_Task() {
	ctx := context.Background()
	startT := time.Now()
	taskC := make(chan *InternalTask, 1)

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	taskC <- task
	retTask, err := t.matcher.poll(ctx, startT, nil, taskC, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(task, retTask)
}

func (t *MatcherTestSuite) Test_poll_QueryTask() {
	ctx := context.Background()
	startT := time.Now()
	queryTaskC := make(chan *InternalTask, 1)

	queryTask := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})
	queryTaskC <- queryTask
	retTask, err := t.matcher.poll(ctx, startT, nil, nil, queryTaskC)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(queryTask, retTask)
}

func (t *MatcherTestSuite) Test_poll_ContextExpired() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	cancel()

	startT := time.Now()

	retTask, err := t.matcher.poll(ctx, startT, nil, nil, nil)

	t.ErrorIs(err, ErrNoTasks)
	t.Nil(retTask)
}

func (t *MatcherTestSuite) Test_pollNonBlocking_IsolatedTask() {
	t.matcher.config.LocalPollWaitTime = func() time.Duration { return 0 }

	ctx := context.Background()
	isolationGroup := "dca1"
	isolatedTaskC := make(chan *InternalTask, 1)

	isolatedTask := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, isolationGroup)
	isolatedTaskC <- isolatedTask
	retTask, err := t.matcher.pollNonBlocking(ctx, isolatedTaskC, nil, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(isolatedTask, retTask)
}

func (t *MatcherTestSuite) Test_pollNonBlocking_Task() {
	t.matcher.config.LocalPollWaitTime = func() time.Duration { return 0 }

	ctx := context.Background()
	taskC := make(chan *InternalTask, 1)

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	taskC <- task
	retTask, err := t.matcher.pollNonBlocking(ctx, nil, taskC, nil)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(task, retTask)
}

func (t *MatcherTestSuite) Test_pollNonBlocking_QueryTask() {
	t.matcher.config.LocalPollWaitTime = func() time.Duration { return 0 }

	ctx := context.Background()
	queryTaskC := make(chan *InternalTask, 1)

	queryTask := newInternalQueryTask(uuid.New(), &types.MatchingQueryWorkflowRequest{})
	queryTaskC <- queryTask
	retTask, err := t.matcher.pollNonBlocking(ctx, nil, nil, queryTaskC)
	t.NoError(err)
	t.NotNil(retTask)
	t.Equal(queryTask, retTask)
}

func (t *MatcherTestSuite) Test_pollNonBlocking_NoTasks() {
	t.matcher.config.LocalPollWaitTime = func() time.Duration { return 0 }

	ctx := context.Background()

	retTask, err := t.matcher.pollNonBlocking(ctx, nil, nil, nil)

	t.ErrorIs(err, ErrNoTasks)
	t.Nil(retTask)
}

func (t *MatcherTestSuite) Test_fwdrPollReqTokenC() {
	t.matcher.fwdr = nil
	t.Equal(noopForwarderTokenC, t.matcher.fwdrPollReqTokenC())
}

func (t *MatcherTestSuite) disableRemoteForwarding() {
	// force disable remote forwarding
	<-t.fwdr.AddReqTokenC()
	<-t.fwdr.PollReqTokenC()
}

func (t *MatcherTestSuite) newDomainCache() cache.DomainCache {
	domainName := testDomainName
	dc := cache.NewMockDomainCache(t.controller)
	dc.EXPECT().GetDomainName(gomock.Any()).Return(domainName, nil).AnyTimes()
	return dc
}

func (t *MatcherTestSuite) newTaskInfo() *persistence.TaskInfo {
	return &persistence.TaskInfo{
		DomainID:                      uuid.New(),
		WorkflowID:                    uuid.New(),
		RunID:                         uuid.New(),
		TaskID:                        rand.Int63(),
		ScheduleID:                    rand.Int63(),
		ScheduleToStartTimeoutSeconds: rand.Int31(),
	}
}

func TestRatelimitBehavior(t *testing.T) {
	// NOT t.Parallel() to avoid noise from cpu-heavy tests

	const (
		granularity = 100 * time.Millisecond
		precision   = float64(granularity / 10) // for elapsed-time delta checks
	)

	// code prior to ~june 2024, kept around until new behavior is thoroughly validated,
	// largely to make regression tests easier to build if we encounter problems
	orig := func(ctx context.Context, limiter *rate.Limiter) (*rate.Reservation, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		deadline, ok := ctx.Deadline()
		if !ok {
			if err := limiter.Wait(ctx); err != nil {
				return nil, err
			}
			return nil, nil
		}

		rsv := limiter.Reserve()
		// If we have to wait too long for reservation, give up and return
		if !rsv.OK() || rsv.Delay() > time.Until(deadline) {
			if rsv.OK() { // if we were indeed given a reservation, return it before we bail out
				rsv.Cancel()
			}
			return nil, ErrTasklistThrottled
		}

		time.Sleep(rsv.Delay())
		return rsv, nil
	}
	tests := map[string]struct {
		run         func(allowable func(ctx context.Context) (*rate.Reservation, error)) error
		err         error
		duration    time.Duration
		beginTokens int
		endTokens   int
	}{
		"no deadline returns immediately if available": {
			run: func(allowable func(ctx context.Context) (*rate.Reservation, error)) error {
				_, err := allowable(context.Background())
				return err
			},
			duration:    0,
			beginTokens: 1,
		},
		"no deadline waits until available": {
			run: func(allowable func(ctx context.Context) (*rate.Reservation, error)) error {
				_, err := allowable(context.Background())
				return err
			},
			duration: granularity,
		},
		"canceling no deadline while waiting returns ctx err": {
			run: func(allowable func(ctx context.Context) (*rate.Reservation, error)) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				go func() {
					time.Sleep(granularity / 2)
					cancel()
				}()
				_, err := allowable(ctx)
				return err
			},
			err:      context.Canceled,
			duration: granularity / 2,
		},
		"returns immediately if canceled": {
			run: func(allowable func(ctx context.Context) (*rate.Reservation, error)) error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := allowable(ctx)
				return err
			},
			err:         context.Canceled,
			duration:    0,
			beginTokens: 1,
			endTokens:   1,
		},
		"sufficient deadline returns immediately if available": {
			run: func(allowable func(ctx context.Context) (*rate.Reservation, error)) error {
				ctx, cancel := context.WithTimeout(context.Background(), granularity*2)
				defer cancel()
				_, err := allowable(ctx)
				return err
			},
			duration:    0,
			beginTokens: 1,
		},
		"sufficient deadline waits": {
			run: func(allowable func(ctx context.Context) (*rate.Reservation, error)) error {
				ctx, cancel := context.WithTimeout(context.Background(), granularity*2)
				defer cancel()
				_, err := allowable(ctx)
				return err
			},
			duration: granularity,
		},
		"insufficient deadline stops immediately": {
			run: func(allowable func(ctx context.Context) (*rate.Reservation, error)) error {
				ctx, cancel := context.WithTimeout(context.Background(), granularity/2)
				defer cancel()
				_, err := allowable(ctx)
				return err
			},
			err:         ErrTasklistThrottled,
			duration:    0,
			beginTokens: 0,
		},
	}
	for name, test := range tests {
		test := test // closure copy
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			check := func(t *testing.T, allow func() bool, allowable func(ctx context.Context) (*rate.Reservation, error)) {
				for allow() {
					// drain tokens to start
				}
				if test.beginTokens > 0 {
					time.Sleep(time.Duration(test.beginTokens) * granularity)
				}
				start := time.Now()
				err := test.run(allowable)
				dur := time.Since(start).Round(time.Millisecond)

				if test.err != nil {
					assert.ErrorIs(t, err, test.err, "wrong err returned, got %v", err)
				} else {
					assert.NoError(t, err)
				}
				assert.InDeltaf(t, test.duration, dur, precision, "duration should be within %v of %v, was %v", time.Duration(precision), test.duration, dur)
				availableTokens := 0
				for allow() {
					availableTokens++
				}
				assert.Equal(t, test.endTokens, availableTokens, "should have %v tokens available after the test runs", test.endTokens)

			}
			t.Run("orig", func(t *testing.T) {
				t.Parallel()
				perSecond := time.Second / granularity
				limiter := rate.NewLimiter(rate.Limit(perSecond), int(perSecond))
				check(t, limiter.Allow, func(ctx context.Context) (*rate.Reservation, error) {
					return orig(ctx, limiter)
				})
			})
			t.Run("current", func(t *testing.T) {
				t.Parallel()
				perSecond := float64(time.Second / granularity)
				limiter := clock.NewRatelimiter(rate.Limit(perSecond), int(perSecond))
				check(t, limiter.Allow, func(ctx context.Context) (*rate.Reservation, error) {
					return nil, (&taskMatcherImpl{limiter: limiter}).ratelimit(ctx)
				})
			})
		})
	}

	t.Run("known misleading behavior", func(t *testing.T) {
		t.Parallel()
		run := func(t *testing.T, limit *rate.Limiter) time.Duration {
			// more than enough time to wait
			ctx, cancel := context.WithTimeout(context.Background(), granularity*2)
			defer cancel()

			start := time.Now()
			t.Logf("tokens before wait: %0.5f", limit.Tokens())
			res, err := orig(ctx, limit)
			t.Logf("tokens after wait: %0.5f", limit.Tokens())
			elapsed := time.Since(start).Round(time.Millisecond)

			require.NoError(t, err, "should have succeeded")

			tokensBefore := limit.Tokens()
			t.Logf("tokens before cancel: %0.5f", tokensBefore)
			res.Cancel()
			tokensAfter := limit.Tokens()
			t.Logf("tokens after cancel: %0.5f", tokensAfter)

			assert.Lessf(t, math.Abs(tokensBefore-tokensAfter), 0.1, "canceling a delay-passed reservation does not return any tokens.  had %0.5f, cancel(), now have %0.5f", tokensBefore, tokensAfter)
			assert.Lessf(t, tokensAfter, 0.1, "near zero tokens should remain")

			return elapsed // varies per test
		}
		t.Run("orig returned tokens are un-cancel-able if available immediately", func(t *testing.T) {
			t.Parallel()
			perSecond := time.Second / granularity
			limit := rate.NewLimiter(rate.Limit(perSecond), int(perSecond))
			for limit.Allow() {
				// drain tokens to start
			}

			time.Sleep(granularity) // restore one full token
			elapsed := run(t, limit)
			require.Lessf(t, elapsed, granularity/10, "should have returned basically immediately")
		})
		t.Run("orig returned tokens are un-cancel-able if it had to wait", func(t *testing.T) {
			t.Parallel()
			perSecond := time.Second / granularity
			limit := rate.NewLimiter(rate.Limit(perSecond), int(perSecond))
			for limit.Allow() {
				// drain tokens to start
			}
			time.Sleep(granularity / 2) // restore only half of one token so it waits
			elapsed := run(t, limit)
			require.InDeltaf(t, elapsed, granularity/2, precision, "should have waited %v for the remaining half token", granularity/2)
		})
	})

	t.Run("known different behavior", func(t *testing.T) {
		t.Parallel()
		t.Run("canceling while waiting with a deadline", func(t *testing.T) {
			t.Parallel()
			cancelWhileWaiting := func(cb func(context.Context) error) (time.Duration, error) {
				// sufficient deadline, but canceled after half a token recovers
				ctx, cancel := context.WithTimeout(context.Background(), granularity*2)
				defer cancel()
				go func() {
					time.Sleep(granularity / 2)
					cancel()
				}()

				start := time.Now()
				err := cb(ctx)
				elapsed := time.Since(start)
				return elapsed, err
			}
			t.Run("orig does not stop", func(t *testing.T) {
				t.Parallel()
				perSecond := time.Second / granularity
				limit := rate.NewLimiter(rate.Limit(perSecond), int(perSecond))
				for limit.Allow() {
					// drain tokens to start
				}

				elapsed, err := cancelWhileWaiting(func(ctx context.Context) error {
					_, err := orig(ctx, limit)
					return err
				})

				assert.NoError(t, err, "continues waiting and returns successfully despite being canceled")
				assert.InDeltaf(t, granularity, elapsed, precision, "waits until a full token would recover")
				assert.False(t, limit.Allow(), "consumes the token that was recovered")
				time.Sleep(granularity / 2)
				assert.False(t, limit.Allow(), "did not have 0.5 tokens available after waiting") // should be 0.5 now though
			})
			t.Run("new code stops quickly and returns unused tokens", func(t *testing.T) {
				t.Parallel()
				perSecond := float64(time.Second / granularity)
				limit := clock.NewRatelimiter(rate.Limit(perSecond), int(perSecond))
				for limit.Allow() {
					// drain tokens to start
				}

				elapsed, err := cancelWhileWaiting(func(ctx context.Context) error {
					return (&taskMatcherImpl{limiter: limit}).ratelimit(ctx)
				})

				assert.ErrorIs(t, err, context.Canceled, "gives up and returns context error")
				assert.InDeltaf(t, granularity/2, elapsed, precision, "waits only until canceled")
				assert.False(t, limit.Allow(), "does not yet have a full token, only 0.5")
				time.Sleep(granularity / 2)
				// note this is false in the original code
				assert.True(t, limit.Allow(), "0.5 -> 1 tokens recovered shows 0.5 were available after waiting")
			})
		})
	})
}

// Try to ensure a blocking callback in a goroutine is not running until the thing immediately
// after `ready()` has blocked, so tests can ensure that the callback contents happen last.
//
// Try to delay calling `ready()` until *immediately* before the blocking call for best results.
//
// This is a best-effort technique, as there is no way to reliably synchronize this kind of thing
// without exposing internal latches or having a more sophisticated locking library than Go offers.
// In case of flakiness, increase the time.Sleep and hope for the best.
//
// Note that adding fmt.Println() calls touches synchronization code (for I/O), so it may change behavior.
func ensureAsyncAfterReady(ctxTimeout time.Duration, cb func(ctx context.Context)) (ready func(), wait func()) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	ready = func() {
		go func() {
			defer cancel()

			// since `go func()` is non-blocking, the ready()-ing goroutine should generally continue,
			// and read whatever blocking point is relevant before this goroutine runs.
			// in many cases this sleep is unnecessary (especially with -cpu=1), but it does help.
			time.Sleep(1 * time.Millisecond)

			cb(ctx)

		}()
	}
	wait = func() {
		<-ctx.Done()
	}
	return ready, wait
}

// Try to ensure a blocking callback is actively blocked in a goroutine before returning, so tests can
// ensure that the callback contents happen first.
// Do NOT access shared variables in the callback without mutex, as it may cause data races.
//
// This is a best-effort technique, as there is no way to reliably synchronize this kind of thing
// without exposing internal latches or having a more sophisticated locking library than Go offers.
// In case of flakiness, increase the time.Sleep and hope for the best.
//
// Note that adding fmt.Println() calls touches synchronization code (for I/O), so it may change behavior.
func ensureAsyncReady(ctxTimeout time.Duration, cb func(ctx context.Context)) (wait func()) {
	running := make(chan struct{})
	closed := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	go func() {
		// Defers are stacked. The last one added is the first one to run.
		// We want to cancel the context which will make the callback return because it's a Poll,
		// and then close the closed channel.
		// This way the returned wait function will block until the callback has returned.
		defer close(closed)
		defer cancel()

		close(running)
		cb(ctx)
	}()
	<-running // ensure the goroutine is alive

	// `close(running)` is non-blocking, so it should generally begin polling before yielding control to other goroutines,
	// but there is still a race to reach whatever blocking sync point exists between the code being tested.
	// In many cases this sleep is completely unnecessary (especially with -cpu=1), but it does help.
	time.Sleep(1 * time.Millisecond)

	return func() {
		<-closed
	}
}

func (t *MatcherTestSuite) Test_LocalSyncMatch_AutoConfigHint() {
	t.disableRemoteForwarding()

	wait := ensureAsyncReady(time.Second, func(ctx context.Context) {
		task, err := t.matcher.Poll(ctx, "")
		t.NotNil(task.AutoConfigHint)
		t.Equal(false, task.AutoConfigHint.EnableAutoConfig) // disabled by default
		if err == nil {
			task.Finish(nil)
		}
	})

	task := newInternalTask(t.newTaskInfo(), nil, types.TaskSourceHistory, "", true, nil, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err := t.matcher.Offer(ctx, task)
	cancel()
	wait()
	t.NoError(err)
}
