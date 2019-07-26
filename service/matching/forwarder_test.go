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

package matching

import (
	"testing"

	"context"

	"math/rand"

	"sync"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	gen "github.com/uber/cadence/.gen/go/matching"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
)

type ForwarderTestSuite struct {
	suite.Suite
	client   *mocks.MatchingClient
	fwdr     *Forwarder
	cfg      *forwarderConfig
	taskList *taskListID
}

func TestForwarderSuite(t *testing.T) {
	suite.Run(t, new(ForwarderTestSuite))
}

func (t *ForwarderTestSuite) SetupTest() {
	t.client = new(mocks.MatchingClient)
	t.cfg = &forwarderConfig{
		ForwarderMaxOutstandingPolls: func() int { return 1 },
		ForwarderMaxRatePerSecond:    func() int { return 2 },
		ForwarderMaxChildrenPerNode:  func() int { return 20 },
		ForwarderMaxOutstandingTasks: func() int { return 1 },
	}
	t.taskList = newTestTaskListID("fwdr", "tl0", persistence.TaskListTypeDecision)
	scope := func() metrics.Scope { return metrics.NoopScope(metrics.Matching) }
	t.fwdr = newForwarder(t.cfg, t.taskList, shared.TaskListKindNormal, t.client, scope)
}

func (t *ForwarderTestSuite) TestForwardTaskError() {
	task := newInternalTask(&persistence.TaskInfo{}, nil, "", false)
	t.Equal(errNoParent, t.fwdr.ForwardTask(context.Background(), task))

	t.usingTasklistPartition(persistence.TaskListTypeActivity)
	t.fwdr.taskListKind = shared.TaskListKindSticky
	t.Equal(errTaskListKind, t.fwdr.ForwardTask(context.Background(), task))
}

func (t *ForwarderTestSuite) TestForwardDecisionTask() {
	t.usingTasklistPartition(persistence.TaskListTypeDecision)

	var request *gen.AddDecisionTaskRequest
	t.client.On("AddDecisionTask", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		request = args.Get(1).(*gen.AddDecisionTaskRequest)
	}).Return(nil).Once()
	taskInfo := t.newTaskInfo()
	task := newInternalTask(taskInfo, nil, "", false)
	t.NoError(t.fwdr.ForwardTask(context.Background(), task))
	mock.AssertExpectationsForObjects(t.T(), t.client)
	t.NotNil(request)
	t.Equal(t.taskList.Parent(20), request.TaskList.GetName())
	t.Equal(t.fwdr.taskListKind, request.TaskList.GetKind())
	t.Equal(taskInfo.DomainID, request.GetDomainUUID())
	t.Equal(taskInfo.WorkflowID, request.GetExecution().GetWorkflowId())
	t.Equal(taskInfo.RunID, request.GetExecution().GetRunId())
	t.Equal(taskInfo.ScheduleID, request.GetScheduleId())
	t.Equal(taskInfo.ScheduleToStartTimeout, request.GetScheduleToStartTimeoutSeconds())
	t.Equal(t.taskList.name, request.GetForwardedFrom())
}

func (t *ForwarderTestSuite) TestForwardActivityTask() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	var request *gen.AddActivityTaskRequest
	t.client.On("AddActivityTask", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		request = args.Get(1).(*gen.AddActivityTaskRequest)
	}).Return(nil).Once()
	taskInfo := t.newTaskInfo()
	task := newInternalTask(taskInfo, nil, "", false)
	t.NoError(t.fwdr.ForwardTask(context.Background(), task))
	mock.AssertExpectationsForObjects(t.T(), t.client)
	t.NotNil(request)
	t.Equal(t.taskList.Parent(20), request.TaskList.GetName())
	t.Equal(t.fwdr.taskListKind, request.TaskList.GetKind())
	t.Equal(taskInfo.DomainID, request.GetDomainUUID())
	t.Equal(taskInfo.WorkflowID, request.GetExecution().GetWorkflowId())
	t.Equal(taskInfo.RunID, request.GetExecution().GetRunId())
	t.Equal(taskInfo.ScheduleID, request.GetScheduleId())
	t.Equal(taskInfo.ScheduleToStartTimeout, request.GetScheduleToStartTimeoutSeconds())
	t.Equal(t.taskList.name, request.GetForwardedFrom())
}

func (t *ForwarderTestSuite) TestForwardTaskMaxOutstanding() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	count := 0

	var wg sync.WaitGroup
	wg.Add(1)

	t.client.On("AddActivityTask", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		count++
		if count == 1 {
			wg.Done()
			time.Sleep(time.Millisecond * 500)
		}
	}).Return(nil).Twice()

	taskInfo := t.newTaskInfo()
	task := newInternalTask(taskInfo, nil, "", false)
	go func() {
		t.NoError(t.fwdr.ForwardTask(context.Background(), task))
	}()
	t.True(common.AwaitWaitGroup(&wg, time.Second))
	err := t.fwdr.ForwardTask(context.Background(), task)
	t.Equal(errForwarderRateLimit, err)
}

func (t *ForwarderTestSuite) TestForwardTaskRateExceeded() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	rps := 2
	t.client.On("AddActivityTask", mock.Anything, mock.Anything).Return(nil).Times(rps + 1)
	taskInfo := t.newTaskInfo()
	task := newInternalTask(taskInfo, nil, "", false)
	for i := 0; i < rps; i++ {
		t.NoError(t.fwdr.ForwardTask(context.Background(), task))
	}
	t.Equal(errForwarderRateLimit, t.fwdr.ForwardTask(context.Background(), task))
}

func (t *ForwarderTestSuite) TestForwardQueryTaskError() {
	task := newInternalQueryTask("id1", &gen.QueryWorkflowRequest{})
	_, err := t.fwdr.ForwardQueryTask(context.Background(), task)
	t.Equal(errNoParent, err)

	t.usingTasklistPartition(persistence.TaskListTypeDecision)
	t.fwdr.taskListKind = shared.TaskListKindSticky
	_, err = t.fwdr.ForwardQueryTask(context.Background(), task)
	t.Equal(errTaskListKind, err)
}

func (t *ForwarderTestSuite) TestForwardQueryTask() {
	t.usingTasklistPartition(persistence.TaskListTypeDecision)
	task := newInternalQueryTask("id1", &gen.QueryWorkflowRequest{})
	resp := &shared.QueryWorkflowResponse{}
	var request *gen.QueryWorkflowRequest
	t.client.On("QueryWorkflow", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		request = args.Get(1).(*gen.QueryWorkflowRequest)
	}).Return(resp, nil).Once()
	gotResp, err := t.fwdr.ForwardQueryTask(context.Background(), task)
	t.NoError(err)
	t.Equal(t.taskList.Parent(20), request.TaskList.GetName())
	t.Equal(t.fwdr.taskListKind, request.TaskList.GetKind())
	t.True(task.query.request.QueryRequest == request.QueryRequest)
	t.True(resp == gotResp)
}

func (t *ForwarderTestSuite) TestForwardQueryTaskRateExceeded() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)
	task := newInternalQueryTask("id1", &gen.QueryWorkflowRequest{})
	resp := &shared.QueryWorkflowResponse{}
	rps := 2
	t.client.On("QueryWorkflow", mock.Anything, mock.Anything).Return(resp, nil).Times(rps + 1)
	for i := 0; i < rps; i++ {
		_, err := t.fwdr.ForwardQueryTask(context.Background(), task)
		t.NoError(err)
	}
	_, err := t.fwdr.ForwardQueryTask(context.Background(), task)
	t.Equal(errForwarderRateLimit, err)
}

func (t *ForwarderTestSuite) TestForwardPollError() {
	_, err := t.fwdr.ForwardPoll(context.Background())
	t.Equal(errNoParent, err)

	t.usingTasklistPartition(persistence.TaskListTypeActivity)
	t.fwdr.taskListKind = shared.TaskListKindSticky
	_, err = t.fwdr.ForwardPoll(context.Background())
	t.Equal(errTaskListKind, err)

}

func (t *ForwarderTestSuite) TestForwardPollForDecision() {
	t.usingTasklistPartition(persistence.TaskListTypeDecision)

	pollerID := uuid.New()
	ctx := context.WithValue(context.Background(), pollerIDKey, pollerID)
	ctx = context.WithValue(ctx, identityKey, "id1")
	resp := &gen.PollForDecisionTaskResponse{}

	var request *gen.PollForDecisionTaskRequest
	t.client.On("PollForDecisionTask", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		request = args.Get(1).(*gen.PollForDecisionTaskRequest)
	}).Return(resp, nil).Once()

	task, err := t.fwdr.ForwardPoll(ctx)
	t.NoError(err)
	t.NotNil(task)
	t.NotNil(request)
	t.Equal(pollerID, request.GetPollerID())
	t.Equal(t.taskList.domainID, request.GetDomainUUID())
	t.Equal("id1", request.GetPollRequest().GetIdentity())
	t.Equal(t.taskList.Parent(20), request.GetPollRequest().GetTaskList().GetName())
	t.Equal(t.fwdr.taskListKind, request.GetPollRequest().GetTaskList().GetKind())
	t.True(resp == task.pollForDecisionResponse())
	t.Nil(task.pollForActivityResponse())
}

func (t *ForwarderTestSuite) TestForwardPollForActivity() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	pollerID := uuid.New()
	ctx := context.WithValue(context.Background(), pollerIDKey, pollerID)
	ctx = context.WithValue(ctx, identityKey, "id1")
	resp := &shared.PollForActivityTaskResponse{}

	var request *gen.PollForActivityTaskRequest
	t.client.On("PollForActivityTask", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		request = args.Get(1).(*gen.PollForActivityTaskRequest)
	}).Return(resp, nil).Once()

	task, err := t.fwdr.ForwardPoll(ctx)
	t.NoError(err)
	t.NotNil(task)
	t.NotNil(request)
	t.Equal(pollerID, request.GetPollerID())
	t.Equal(t.taskList.domainID, request.GetDomainUUID())
	t.Equal("id1", request.GetPollRequest().GetIdentity())
	t.Equal(t.taskList.Parent(20), request.GetPollRequest().GetTaskList().GetName())
	t.Equal(t.fwdr.taskListKind, request.GetPollRequest().GetTaskList().GetKind())
	t.True(resp == task.pollForActivityResponse())
	t.Nil(task.pollForDecisionResponse())
}

func (t *ForwarderTestSuite) TestForwardPollMaxOutstanding() {
	t.usingTasklistPartition(persistence.TaskListTypeActivity)

	count := 0
	resp := &shared.PollForActivityTaskResponse{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	t.client.On("PollForActivityTask", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		count++
		if count == 1 {
			wg.Done()
			time.Sleep(time.Millisecond * 500)
		}
	}).Return(resp, nil).Twice()

	go func() {
		_, err := t.fwdr.ForwardPoll(context.Background())
		t.NoError(err)
	}()

	t.True(common.AwaitWaitGroup(&wg, time.Second))
	_, err := t.fwdr.ForwardPoll(context.Background())
	t.Equal(errForwarderRateLimit, err)
}

func (t *ForwarderTestSuite) TestRatelimiting() {
	testCases := []struct {
		name  string
		token forwarderToken
	}{
		{"maxRate", newForwarderToken(func() int { return 50 }, func() float64 { return 10 })},
		{"maxOutstanding", newForwarderToken(func() int { return 1 }, func() float64 { return 10 })},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func() {
			wg := sync.WaitGroup{}
			startC := make(chan struct{})
			success := 0
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func() {
					<-startC
					if tc.token.acquire() {
						success++
						tc.token.release()
					}
					wg.Done()
				}()
			}
			close(startC)
			t.True(common.AwaitWaitGroup(&wg, time.Second))
			if tc.name == "maxRate" {
				t.Equal(int(tc.token.rpsFunc()), success)
				return
			}
			t.True(success >= tc.token.maxOutstanding() && success <= int(tc.token.rpsFunc()))
		})
	}
}

func (t *ForwarderTestSuite) TestMaxOutstanding() {
	token := newForwarderToken(func() int { return 1 }, func() float64 { return 10 })
	token.acquire()
	wg := sync.WaitGroup{}
	success := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			if token.acquire() {
				success++
				token.release()
			}
			wg.Done()
		}()
	}
	t.True(common.AwaitWaitGroup(&wg, time.Second))
	t.Equal(0, success)
}

func (t *ForwarderTestSuite) usingTasklistPartition(taskType int) {
	t.taskList = newTestTaskListID("fwdr", taskListPartitionPrefix+"tl0/1", taskType)
	t.fwdr.taskListID = t.taskList
}

func (t *ForwarderTestSuite) newTaskInfo() *persistence.TaskInfo {
	return &persistence.TaskInfo{
		DomainID:               uuid.New(),
		WorkflowID:             uuid.New(),
		RunID:                  uuid.New(),
		TaskID:                 rand.Int63(),
		ScheduleID:             rand.Int63(),
		ScheduleToStartTimeout: rand.Int31(),
	}
}
