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
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

type ForwarderTestSuite struct {
	suite.Suite
	controller      *gomock.Controller
	client          *matching.MockClient
	fwdr            *forwarderImpl
	cfg             *config.ForwarderConfig
	taskList        *Identifier
	isolationGroups []string
}

func TestForwarderSuite(t *testing.T) {
	suite.Run(t, new(ForwarderTestSuite))
}

func (t *ForwarderTestSuite) SetupTest() {
	t.controller = gomock.NewController(t.T())
	t.client = matching.NewMockClient(t.controller)
	t.cfg = &config.ForwarderConfig{
		ForwarderMaxOutstandingPolls: func() int { return 1 },
		ForwarderMaxRatePerSecond:    func() int { return 2 },
		ForwarderMaxChildrenPerNode:  func() int { return 20 },
		ForwarderMaxOutstandingTasks: func() int { return 1 },
	}
	id, err := NewIdentifier("fwdr", "tl0", persistence.TaskListTypeDecision)
	t.NoError(err)
	t.taskList = id
	t.isolationGroups = []string{"abc", "xyz"}
	t.fwdr = newForwarder(t.cfg, t.taskList, types.TaskListKindNormal, t.client, t.isolationGroups, metrics.NoopScope(metrics.Matching)).(*forwarderImpl)
}

func (t *ForwarderTestSuite) TearDownTest() {
	t.controller.Finish()
}

func (t *ForwarderTestSuite) TestForwardTaskError() {
	task := newInternalTask(&persistence.TaskInfo{}, nil, types.TaskSourceHistory, "", false, nil, "")
	t.Equal(ErrNoParent, t.fwdr.ForwardTask(context.Background(), task))

	t.usingTasklistPartition(persistence.TaskListTypeActivity)
	t.fwdr.taskListKind = types.TaskListKindSticky
	t.Equal(ErrTaskListKind, t.fwdr.ForwardTask(context.Background(), task))
}

func (t *ForwarderTestSuite) TestForwardDecisionTask() {
	t.usingTasklistPartition(persistence.TaskListTypeDecision)

	var request *types.AddDecisionTaskRequest
	t.client.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.AddDecisionTaskRequest, option ...yarpc.CallOption) {
			request = arg1
		},
	).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)

	taskInfo := t.newTaskInfo()
	task := newInternalTask(taskInfo, nil, types.TaskSourceHistory, "", false, nil, "")
	t.NoError(t.fwdr.ForwardTask(context.Background(), task))
	t.NotNil(request)
	t.Equal(t.taskList.Parent(20), request.TaskList.GetName())
	t.Equal(t.fwdr.taskListKind, request.TaskList.GetKind())
	t.Equal(taskInfo.DomainID, request.GetDomainUUID())
	t.Equal(taskInfo.WorkflowID, request.GetExecution().GetWorkflowID())
	t.Equal(taskInfo.RunID, request.GetExecution().GetRunID())
	t.Equal(taskInfo.ScheduleID, request.GetScheduleID())
	t.Equal(taskInfo.ScheduleToStartTimeoutSeconds, request.GetScheduleToStartTimeoutSeconds())
	t.Equal(t.taskList.name, request.GetForwardedFrom())
}

func (t *ForwarderTestSuite) TestForwardActivityTask() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	var request *types.AddActivityTaskRequest
	t.client.EXPECT().AddActivityTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.AddActivityTaskRequest, option ...yarpc.CallOption) {
			request = arg1
		},
	).Return(&types.AddActivityTaskResponse{}, nil).Times(1)

	taskInfo := t.newTaskInfo()
	task := newInternalTask(taskInfo, nil, types.TaskSourceHistory, "", false, nil, "")
	t.NoError(t.fwdr.ForwardTask(context.Background(), task))
	t.NotNil(request)
	t.Equal(t.taskList.Parent(20), request.TaskList.GetName())
	t.Equal(t.fwdr.taskListKind, request.TaskList.GetKind())
	t.Equal(t.taskList.domainID, request.GetDomainUUID())
	t.Equal(taskInfo.DomainID, request.GetSourceDomainUUID())
	t.Equal(taskInfo.WorkflowID, request.GetExecution().GetWorkflowID())
	t.Equal(taskInfo.RunID, request.GetExecution().GetRunID())
	t.Equal(taskInfo.ScheduleID, request.GetScheduleID())
	t.Equal(taskInfo.ScheduleToStartTimeoutSeconds, request.GetScheduleToStartTimeoutSeconds())
	t.Equal(t.taskList.name, request.GetForwardedFrom())
}

func (t *ForwarderTestSuite) TestForwardTaskRateExceeded() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	rps := 2
	t.client.EXPECT().AddActivityTask(gomock.Any(), gomock.Any()).Return(&types.AddActivityTaskResponse{}, nil).Times(rps)
	taskInfo := t.newTaskInfo()
	task := newInternalTask(taskInfo, nil, types.TaskSourceHistory, "", false, nil, "")
	for i := 0; i < rps; i++ {
		t.NoError(t.fwdr.ForwardTask(context.Background(), task))
	}
	t.Equal(ErrForwarderSlowDown, t.fwdr.ForwardTask(context.Background(), task))
}

func (t *ForwarderTestSuite) TestForwardQueryTaskError() {
	task := newInternalQueryTask("id1", &types.MatchingQueryWorkflowRequest{})
	_, err := t.fwdr.ForwardQueryTask(context.Background(), task)
	t.Equal(ErrNoParent, err)

	t.usingTasklistPartition(persistence.TaskListTypeDecision)
	t.fwdr.taskListKind = types.TaskListKindSticky
	_, err = t.fwdr.ForwardQueryTask(context.Background(), task)
	t.Equal(ErrTaskListKind, err)
}

func (t *ForwarderTestSuite) TestForwardQueryTask() {
	t.usingTasklistPartition(persistence.TaskListTypeDecision)
	task := newInternalQueryTask("id1", &types.MatchingQueryWorkflowRequest{})
	resp := &types.QueryWorkflowResponse{}
	var request *types.MatchingQueryWorkflowRequest
	t.client.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.MatchingQueryWorkflowRequest, option ...yarpc.CallOption) {
			request = arg1
		},
	).Return(resp, nil).Times(1)

	gotResp, err := t.fwdr.ForwardQueryTask(context.Background(), task)
	t.NoError(err)
	t.Equal(t.taskList.Parent(20), request.TaskList.GetName())
	t.Equal(t.fwdr.taskListKind, request.TaskList.GetKind())
	t.True(task.Query.Request.QueryRequest == request.QueryRequest)
	t.Equal(resp, gotResp)
}

func (t *ForwarderTestSuite) TestForwardQueryTaskRateNotEnforced() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)
	task := newInternalQueryTask("id1", &types.MatchingQueryWorkflowRequest{})
	resp := &types.QueryWorkflowResponse{}
	rps := 2
	t.client.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(resp, nil).Times(rps + 1)
	for i := 0; i < rps; i++ {
		_, err := t.fwdr.ForwardQueryTask(context.Background(), task)
		t.NoError(err)
	}
	_, err := t.fwdr.ForwardQueryTask(context.Background(), task)
	t.NoError(err) // no rateliming should be enforced for query task
}

func (t *ForwarderTestSuite) TestForwardPollError() {
	_, err := t.fwdr.ForwardPoll(context.Background())
	t.Equal(ErrNoParent, err)

	t.usingTasklistPartition(persistence.TaskListTypeActivity)
	t.fwdr.taskListKind = types.TaskListKindSticky
	_, err = t.fwdr.ForwardPoll(context.Background())
	t.Equal(ErrTaskListKind, err)

}

func (t *ForwarderTestSuite) TestForwardPollForDecision() {
	t.usingTasklistPartition(persistence.TaskListTypeDecision)

	pollerID := uuid.New()
	ctx := ContextWithPollerID(context.Background(), pollerID)
	ctx = ContextWithIdentity(ctx, "id1")
	resp := &types.MatchingPollForDecisionTaskResponse{}

	var request *types.MatchingPollForDecisionTaskRequest
	t.client.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.MatchingPollForDecisionTaskRequest, option ...yarpc.CallOption) {
			request = arg1
		},
	).Return(resp, nil).Times(1)

	task, err := t.fwdr.ForwardPoll(ctx)
	t.NoError(err)
	t.NotNil(task)
	t.NotNil(request)
	t.Equal(pollerID, request.GetPollerID())
	t.Equal(t.taskList.domainID, request.GetDomainUUID())
	t.Equal("id1", request.GetPollRequest().GetIdentity())
	t.Equal(t.taskList.Parent(20), request.GetPollRequest().GetTaskList().GetName())
	t.Equal(t.fwdr.taskListKind, request.GetPollRequest().GetTaskList().GetKind())
	t.Equal(resp, task.PollForDecisionResponse())
	t.Nil(task.PollForActivityResponse())
}

func (t *ForwarderTestSuite) TestForwardPollForActivity() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	pollerID := uuid.New()
	ctx := ContextWithPollerID(context.Background(), pollerID)
	ctx = ContextWithIdentity(ctx, "id1")
	resp := &types.MatchingPollForActivityTaskResponse{}

	var request *types.MatchingPollForActivityTaskRequest
	t.client.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any()).Do(
		func(arg0 context.Context, arg1 *types.MatchingPollForActivityTaskRequest, option ...yarpc.CallOption) {
			request = arg1
		},
	).Return(resp, nil).Times(1)

	task, err := t.fwdr.ForwardPoll(ctx)
	t.NoError(err)
	t.NotNil(task)
	t.NotNil(request)
	t.Equal(pollerID, request.GetPollerID())
	t.Equal(t.taskList.domainID, request.GetDomainUUID())
	t.Equal("id1", request.GetPollRequest().GetIdentity())
	t.Equal(t.taskList.Parent(20), request.GetPollRequest().GetTaskList().GetName())
	t.Equal(t.fwdr.taskListKind, request.GetPollRequest().GetTaskList().GetKind())
	t.Equal(resp, task.PollForActivityResponse())
	t.Nil(task.PollForDecisionResponse())
}

func (t *ForwarderTestSuite) TestMaxOutstandingConcurrency() {
	concurrency := 50
	testCases := []struct {
		name          string
		mustLeakToken bool
		output        int32
	}{
		{"contention", false, int32(concurrency)},
		{"token_leak", true, 1},
	}

	var adds int32
	var polls int32
	var wg sync.WaitGroup

	for _, tc := range testCases {
		adds = 0
		polls = 0
		t.Run(tc.name, func() {
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					for i := 0; i < len(t.isolationGroups)+1; i++ {
						select {
						case token := <-t.fwdr.AddReqTokenC():
							if !tc.mustLeakToken {
								token.release("")
							}
							atomic.AddInt32(&adds, 1)
						case <-time.After(time.Millisecond * 100):
							break
						}
					}

					select {
					case token := <-t.fwdr.PollReqTokenC(""):
						if !tc.mustLeakToken {
							token.release("")
						}
						atomic.AddInt32(&polls, 1)
					case <-time.After(time.Millisecond * 100):
						break
					}
					for _, ig := range t.isolationGroups {
						select {
						case token := <-t.fwdr.PollReqTokenC(ig):
							if !tc.mustLeakToken {
								token.release(ig)
							}
							atomic.AddInt32(&polls, 1)
						case <-time.After(time.Millisecond * 100):
							break
						}
					}
					wg.Done()
				}()
			}
			t.True(common.AwaitWaitGroup(&wg, time.Second))
			t.Equal(tc.output*int32(len(t.isolationGroups)+1), adds)
			t.Equal(tc.output*int32(len(t.isolationGroups)+1), polls)
		})
	}
}

func (t *ForwarderTestSuite) TestMaxOutstandingConfigUpdate() {
	maxOutstandingTasks := int32(1)
	maxOutstandingPolls := int32(1)
	t.fwdr.cfg.ForwarderMaxOutstandingTasks = func() int { return int(atomic.LoadInt32(&maxOutstandingTasks)) }
	t.fwdr.cfg.ForwarderMaxOutstandingPolls = func() int { return int(atomic.LoadInt32(&maxOutstandingPolls)) }

	startC := make(chan struct{})
	doneWG := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		doneWG.Add(1)
		go func() {
			<-startC
			token1 := <-t.fwdr.AddReqTokenC()
			token1.release("")
			token2 := <-t.fwdr.PollReqTokenC("")
			token2.release("")
			doneWG.Done()
		}()
	}

	maxOutstandingTasks = 10
	maxOutstandingPolls = 10
	close(startC)
	t.True(common.AwaitWaitGroup(&doneWG, time.Second))

	t.Equal(10*(len(t.isolationGroups)+1), cap(t.fwdr.addReqToken.Load().(*ForwarderReqToken).ch))
	t.Equal(10, cap(t.fwdr.pollReqToken.Load().(*ForwarderReqToken).ch))
}

func (t *ForwarderTestSuite) usingTasklistPartition(taskType int) {
	t.taskList = NewTestTaskListID(t.T(), "fwdr", common.ReservedTaskListPrefix+"tl0/1", taskType)
	t.fwdr.taskListID = t.taskList
}

func (t *ForwarderTestSuite) newTaskInfo() *persistence.TaskInfo {
	return &persistence.TaskInfo{
		DomainID:                      uuid.New(),
		WorkflowID:                    uuid.New(),
		RunID:                         uuid.New(),
		TaskID:                        rand.Int63(),
		ScheduleID:                    rand.Int63(),
		ScheduleToStartTimeoutSeconds: rand.Int31(),
	}
}
