// Copyright (c) 2020 Uber Technologies, Inc.
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

package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	t "github.com/uber/cadence/common/task"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/shard"
)

type (
	taskSuite struct {
		suite.Suite
		*require.Assertions

		controller           *gomock.Controller
		mockShard            *shard.TestContext
		mockTaskExecutor     *MockExecutor
		mockTaskProcessor    *MockProcessor
		mockTaskRedispatcher *MockRedispatcher
		mockTaskInfo         *MockInfo

		logger        log.Logger
		maxRetryCount dynamicconfig.IntPropertyFn
	}
)

func TestTaskSuite(t *testing.T) {
	s := new(taskSuite)
	suite.Run(t, s)
}

func (s *taskSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID: 10,
			RangeID: 1,
		},
		config.NewForTest(),
	)
	s.mockTaskExecutor = NewMockExecutor(s.controller)
	s.mockTaskProcessor = NewMockProcessor(s.controller)
	s.mockTaskRedispatcher = NewMockRedispatcher(s.controller)
	s.mockTaskInfo = NewMockInfo(s.controller)
	s.mockTaskInfo.EXPECT().GetDomainID().Return(constants.TestDomainID).AnyTimes()
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()

	s.logger = testlogger.New(s.Suite.T())
	s.maxRetryCount = dynamicconfig.GetIntPropertyFn(10)
}

func (s *taskSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *taskSuite) TestExecute_TaskFilterErr() {
	taskFilterErr := errors.New("some random error")
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return false, taskFilterErr
	}, nil)
	err := taskBase.Execute()
	s.Equal(taskFilterErr, err)
}

func (s *taskSuite) TestExecute_ExecutionErr() {
	task := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	executionErr := errors.New("some random error")
	s.mockTaskExecutor.EXPECT().Execute(task, true).Return(executionErr).Times(1)

	err := task.Execute()
	s.Equal(executionErr, err)
}

func (s *taskSuite) TestExecute_Success() {
	task := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	s.mockTaskExecutor.EXPECT().Execute(task, true).Return(nil).Times(1)

	err := task.Execute()
	s.NoError(err)
}

func (s *taskSuite) TestHandleErr_ErrEntityNotExists() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	err := &types.EntityNotExistsError{}
	s.NoError(taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrTaskRetry() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	err := &redispatchError{Reason: "random-reason"}
	s.Equal(err, taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrTaskDiscarded() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	err := ErrTaskDiscarded
	s.NoError(taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrTargetDomainNotActive() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	err := &types.DomainNotActiveError{}

	// we should always return the target domain not active error
	// no matter that the submit time is
	taskBase.submitTime = time.Now().Add(-cache.DomainCacheRefreshInterval*time.Duration(5) - time.Second)
	s.Equal(nil, taskBase.HandleErr(err), "should drop errors after a reasonable time")

	taskBase.submitTime = time.Now()
	s.Equal(err, taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrDomainNotActive() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	err := &types.DomainNotActiveError{}

	taskBase.submitTime = time.Now().Add(-cache.DomainCacheRefreshInterval*time.Duration(5) - time.Second)
	s.NoError(taskBase.HandleErr(err))

	taskBase.submitTime = time.Now()
	s.Equal(err, taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrWorkflowRateLimited() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	taskBase.submitTime = time.Now()
	s.Equal(errWorkflowRateLimited, taskBase.HandleErr(errWorkflowRateLimited))
}

func (s *taskSuite) TestHandleErr_ErrShardRecentlyClosed() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	taskBase.submitTime = time.Now()

	shardClosedError := &shard.ErrShardClosed{
		Msg: "shard closed",
		// The shard was closed within the TimeBeforeShardClosedIsError interval
		ClosedAt: time.Now().Add(-shard.TimeBeforeShardClosedIsError / 2),
	}

	s.Equal(shardClosedError, taskBase.HandleErr(shardClosedError))
}

func (s *taskSuite) TestHandleErr_ErrTaskListNotOwnedByHost() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	taskBase.submitTime = time.Now()

	taskListNotOwnedByHost := &cadence_errors.TaskListNotOwnedByHostError{
		OwnedByIdentity: "HostNameOwnedBy",
		MyIdentity:      "HostNameMe",
		TasklistName:    "TaskListName",
	}

	s.Equal(taskListNotOwnedByHost, taskBase.HandleErr(taskListNotOwnedByHost))
}

func (s *taskSuite) TestHandleErr_ErrCurrentWorkflowConditionFailed() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	err := &persistence.CurrentWorkflowConditionFailedError{}
	s.NoError(taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_UnknownErr() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	// Need to mock a return value for function GetTaskType
	// If don't do, there'll be an error when code goes into the defer function:
	// Unexpected call: because: there are no expected calls of the method "GetTaskType" for that receiver
	// But can't put it in the setup function since it may cause other errors
	s.mockTaskInfo.EXPECT().GetTaskType().Return(123)

	// make sure it will go into the defer function.
	// in this case, make it 0 < attempt < stickyTaskMaxRetryCount
	taskBase.attempt = 10

	err := errors.New("some random error")
	s.Equal(err, taskBase.HandleErr(err))
}

func (s *taskSuite) TestTaskState() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	s.Equal(t.TaskStatePending, taskBase.State())

	taskBase.Ack()
	s.Equal(t.TaskStateAcked, taskBase.State())

	s.mockTaskProcessor.EXPECT().TrySubmit(taskBase).Return(true, nil).Times(1)
	taskBase.Nack()
	s.Equal(t.TaskStateNacked, taskBase.State())
}

func (s *taskSuite) TestTaskPriority() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	priority := 10
	taskBase.SetPriority(priority)
	s.Equal(priority, taskBase.Priority())
}

func (s *taskSuite) TestTaskNack_ResubmitSucceeded() {
	task := s.newTestTask(
		func(task Info) (bool, error) {
			return true, nil
		},
		func(task Task) {
			s.mockTaskRedispatcher.AddTask(task)
		},
	)

	s.mockTaskProcessor.EXPECT().TrySubmit(task).Return(true, nil).Times(1)

	task.Nack()
	s.Equal(t.TaskStateNacked, task.State())
}

func (s *taskSuite) TestTaskNack_ResubmitFailed() {
	task := s.newTestTask(
		func(task Info) (bool, error) {
			return true, nil
		},
		func(task Task) {
			s.mockTaskRedispatcher.AddTask(task)
		},
	)

	s.mockTaskProcessor.EXPECT().TrySubmit(task).Return(false, errTaskProcessorNotRunning).Times(1)
	s.mockTaskRedispatcher.EXPECT().AddTask(task).Times(1)

	task.Nack()
	s.Equal(t.TaskStateNacked, task.State())
}

func (s *taskSuite) TestHandleErr_ErrMaxAttempts() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	taskBase.criticalRetryCount = func(i ...dynamicconfig.FilterOption) int { return 0 }
	s.mockTaskInfo.EXPECT().GetTaskType().Return(0)
	assert.NotPanics(s.T(), func() {
		taskBase.HandleErr(errors.New("err"))
	})
}

func (s *taskSuite) TestRetryErr() {
	taskBase := s.newTestTask(func(task Info) (bool, error) {
		return true, nil
	}, nil)

	s.Equal(false, taskBase.RetryErr(&shard.ErrShardClosed{}))
	s.Equal(false, taskBase.RetryErr(errWorkflowBusy))
	s.Equal(false, taskBase.RetryErr(ErrTaskPendingActive))
	s.Equal(false, taskBase.RetryErr(context.DeadlineExceeded))
	s.Equal(false, taskBase.RetryErr(&redispatchError{Reason: "random-reason"}))
	// rate limited errors are retried
	s.Equal(true, taskBase.RetryErr(errWorkflowRateLimited))
}

func (s *taskSuite) newTestTask(
	taskFilter Filter,
	redispatchFn func(task Task),
) *taskImpl {
	if redispatchFn == nil {
		redispatchFn = func(_ Task) {
			// noop
		}
	}
	taskBase := newTask(
		s.mockShard,
		s.mockTaskInfo,
		QueueTypeActiveTransfer,
		0,
		s.logger,
		taskFilter,
		s.mockTaskExecutor,
		s.mockTaskProcessor,
		s.maxRetryCount,
		redispatchFn,
	)
	taskBase.scope = s.mockShard.GetMetricsClient().Scope(0)
	return taskBase
}
