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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	ctask "github.com/uber/cadence/common/task"
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
		mockTaskInfo         *persistence.MockTask

		logger        log.Logger
		maxRetryCount dynamicproperties.IntPropertyFn
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
	s.mockTaskInfo = persistence.NewMockTask(s.controller)
	s.mockTaskInfo.EXPECT().GetDomainID().Return(constants.TestDomainID).AnyTimes()
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()

	s.logger = testlogger.New(s.Suite.T())
	s.maxRetryCount = dynamicproperties.GetIntPropertyFn(10)
}

func (s *taskSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *taskSuite) TestExecute_TaskFilterErr() {
	taskFilterErr := errors.New("some random error")
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return false, taskFilterErr
	})
	err := taskBase.Execute()
	s.Equal(taskFilterErr, err)
}

func (s *taskSuite) TestExecute_ExecutionErr() {
	task := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	executionErr := errors.New("some random error")
	s.mockTaskExecutor.EXPECT().Execute(task).Return(ExecuteResponse{
		Scope:        metrics.NoopScope,
		IsActiveTask: true,
	}, executionErr).Times(1)

	err := task.Execute()
	s.Equal(executionErr, err)
}

func (s *taskSuite) TestExecute_Success() {
	task := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	s.mockTaskExecutor.EXPECT().Execute(task).Return(ExecuteResponse{
		Scope:        metrics.NoopScope,
		IsActiveTask: true,
	}, nil).Times(1)

	err := task.Execute()
	s.NoError(err)
}

func (s *taskSuite) TestHandleErr_ErrEntityNotExists() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	err := &types.EntityNotExistsError{}
	s.NoError(taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrTaskRetry() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	err := &redispatchError{Reason: "random-reason"}
	s.Equal(err, taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrTaskDiscarded() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	err := ErrTaskDiscarded
	s.NoError(taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrTargetDomainNotActive() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	err := &types.DomainNotActiveError{}

	// we should always return the target domain not active error
	// no matter that the submit time is
	taskBase.initialSubmitTime = time.Now().Add(-cache.DomainCacheRefreshInterval*time.Duration(5) - time.Second)
	s.Equal(nil, taskBase.HandleErr(err), "should drop errors after a reasonable time")

	taskBase.initialSubmitTime = time.Now()
	s.Equal(err, taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrDomainNotActive() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	err := &types.DomainNotActiveError{}

	taskBase.initialSubmitTime = time.Now().Add(-cache.DomainCacheRefreshInterval*time.Duration(5) - time.Second)
	s.NoError(taskBase.HandleErr(err))

	taskBase.initialSubmitTime = time.Now()
	s.Equal(err, taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_ErrWorkflowRateLimited() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	taskBase.initialSubmitTime = time.Now()
	s.Equal(errWorkflowRateLimited, taskBase.HandleErr(errWorkflowRateLimited))
}

func (s *taskSuite) TestHandleErr_ErrShardRecentlyClosed() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	taskBase.initialSubmitTime = time.Now()

	shardClosedError := &shard.ErrShardClosed{
		Msg: "shard closed",
		// The shard was closed within the TimeBeforeShardClosedIsError interval
		ClosedAt: time.Now().Add(-shard.TimeBeforeShardClosedIsError / 2),
	}

	s.Equal(shardClosedError, taskBase.HandleErr(shardClosedError))
}

func (s *taskSuite) TestHandleErr_ErrTaskListNotOwnedByHost() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	taskBase.initialSubmitTime = time.Now()

	taskListNotOwnedByHost := &cadence_errors.TaskListNotOwnedByHostError{
		OwnedByIdentity: "HostNameOwnedBy",
		MyIdentity:      "HostNameMe",
		TasklistName:    "TaskListName",
	}

	s.Equal(taskListNotOwnedByHost, taskBase.HandleErr(taskListNotOwnedByHost))
}

func (s *taskSuite) TestHandleErr_ErrCurrentWorkflowConditionFailed() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	err := &persistence.CurrentWorkflowConditionFailedError{}
	s.NoError(taskBase.HandleErr(err))
}

func (s *taskSuite) TestHandleErr_UnknownErr() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

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

func (s *taskSuite) TestTaskCancel() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	taskBase.Cancel()
	s.Equal(ctask.TaskStateCanceled, taskBase.State())

	s.NoError(taskBase.Execute())

	taskBase.Ack()
	s.Equal(ctask.TaskStateCanceled, taskBase.State())

	taskBase.Nack(nil)
	s.Equal(ctask.TaskStateCanceled, taskBase.State())

	s.False(taskBase.RetryErr(errors.New("some random error")))
}

func (s *taskSuite) TestTaskState() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	s.Equal(ctask.TaskStatePending, taskBase.State())

	taskBase.Ack()
	s.Equal(ctask.TaskStateAcked, taskBase.State())

	taskBase.Nack(nil)
	s.Equal(ctask.TaskStateAcked, taskBase.State())
}

func (s *taskSuite) TestTaskPriority() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	priority := 10
	taskBase.SetPriority(priority)
	s.Equal(priority, taskBase.Priority())
}

func (s *taskSuite) TestTaskNack_ResubmitSucceeded() {
	task := s.newTestTask(
		func(task persistence.Task) (bool, error) {
			return true, nil
		},
	)

	s.mockTaskProcessor.EXPECT().TrySubmit(task).Return(true, nil).Times(1)

	task.Nack(nil)
	s.Equal(ctask.TaskStatePending, task.State())
}

func (s *taskSuite) TestTaskNack_DomainBecomesActive() {
	task := s.newTestTask(
		func(task persistence.Task) (bool, error) {
			return true, nil
		},
	)
	task.queueType = QueueTypeTransfer

	s.mockTaskProcessor.EXPECT().TrySubmit(task).Return(true, nil).Times(1)

	task.Nack(errDomainBecomesActive)
	s.Equal(ctask.TaskStatePending, task.State())
}

func (s *taskSuite) TestTaskNack_ResubmitFailed() {
	task := s.newTestTask(
		func(task persistence.Task) (bool, error) {
			return true, nil
		},
	)

	s.mockTaskProcessor.EXPECT().TrySubmit(task).Return(false, errTaskProcessorNotRunning).Times(1)
	s.mockTaskRedispatcher.EXPECT().RedispatchTask(task, gomock.Any()).Times(1)

	task.Nack(nil)
	s.Equal(ctask.TaskStatePending, task.State())
}

func (s *taskSuite) TestHandleErr_ErrMaxAttempts() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

	taskBase.criticalRetryCount = func(i ...dynamicproperties.FilterOption) int { return 0 }
	s.mockTaskInfo.EXPECT().GetTaskType().Return(0)
	assert.NotPanics(s.T(), func() {
		taskBase.HandleErr(errors.New("err"))
	})
}

func (s *taskSuite) TestRetryErr() {
	taskBase := s.newTestTask(func(task persistence.Task) (bool, error) {
		return true, nil
	})

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
) *taskImpl {
	taskBase := NewHistoryTask(
		s.mockShard,
		s.mockTaskInfo,
		QueueTypeActiveTransfer,
		s.logger,
		taskFilter,
		s.mockTaskExecutor,
		s.mockTaskProcessor,
		s.mockTaskRedispatcher,
		s.maxRetryCount,
	).(*taskImpl)
	taskBase.scope = s.mockShard.GetMetricsClient().Scope(0)
	return taskBase
}
