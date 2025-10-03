// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package queue

import (
	"errors"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

type (
	timerQueueProcessorBaseSuite struct {
		suite.Suite
		*require.Assertions

		controller           *gomock.Controller
		mockShard            *shard.TestContext
		mockTaskProcessor    *task.MockProcessor
		mockQueueSplitPolicy *MockProcessingQueueSplitPolicy

		clusterName   string
		logger        log.Logger
		metricsClient metrics.Client
		metricsScope  metrics.Scope
	}
)

func TestTimerQueueProcessorBaseSuite(t *testing.T) {
	s := new(timerQueueProcessorBaseSuite)
	suite.Run(t, s)
}

func (s *timerQueueProcessorBaseSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          10,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest(),
	)
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return(constants.TestDomainName, nil).AnyTimes()
	s.mockQueueSplitPolicy = NewMockProcessingQueueSplitPolicy(s.controller)
	s.mockTaskProcessor = task.NewMockProcessor(s.controller)

	s.clusterName = cluster.TestCurrentClusterName
	s.logger = testlogger.New(s.Suite.T())
	s.metricsClient = metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{})
	s.metricsScope = s.metricsClient.Scope(metrics.TimerQueueProcessorScope)
}

func (s *timerQueueProcessorBaseSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
	defer goleak.VerifyNone(s.T())
}

func (s *timerQueueProcessorBaseSuite) TestIsProcessNow() {
	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	s.True(timerQueueProcessBase.isProcessNow(time.Time{}))

	now := s.mockShard.GetCurrentTime(s.clusterName)
	s.True(timerQueueProcessBase.isProcessNow(now))

	timeBefore := now.Add(-10 * time.Second)
	s.True(timerQueueProcessBase.isProcessNow(timeBefore))

	timeAfter := now.Add(10 * time.Second)
	s.False(timerQueueProcessBase.isProcessNow(timeAfter))

}

func (s *timerQueueProcessorBaseSuite) TestGetTimerTasks_More() {
	readLevel := newTimerTaskKey(time.Now().Add(-10*time.Second), 0)
	maxReadLevel := newTimerTaskKey(time.Now().Add(10*time.Second), 0)
	batchSize := 10

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            batchSize,
		NextPageToken:       []byte("some random input next page token"),
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-5 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: []byte("some random output next page token"),
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	got, err := timerQueueProcessBase.getTimerTasks(readLevel, maxReadLevel, request.NextPageToken, batchSize)
	s.Nil(err)
	s.Equal(response.Tasks, got.Tasks)
	s.Equal(response.NextPageToken, got.NextPageToken)
}

func (s *timerQueueProcessorBaseSuite) TestGetTimerTasks_NoMore() {
	readLevel := newTimerTaskKey(time.Now().Add(-10*time.Second), 0)
	maxReadLevel := newTimerTaskKey(time.Now().Add(10*time.Second), 0)
	batchSize := 10

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            batchSize,
		NextPageToken:       nil,
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-5 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: nil,
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	got, err := timerQueueProcessBase.getTimerTasks(readLevel, maxReadLevel, request.NextPageToken, batchSize)
	s.Nil(err)
	s.Equal(response.Tasks, got.Tasks)
	s.Empty(got.NextPageToken)
}

func (s *timerQueueProcessorBaseSuite) TestReadLookAheadTask() {
	shardMaxReadLevel := s.mockShard.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTimer, s.clusterName).GetScheduledTime()
	readLevel := newTimerTaskKey(shardMaxReadLevel, 0)
	maxReadLevel := newTimerTaskKey(shardMaxReadLevel.Add(10*time.Second), 0)

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            1,
		NextPageToken:       nil,
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: shardMaxReadLevel,
					TaskID:              int64(59),
					Version:             int64(79),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: []byte("some random next page token"),
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	lookAheadTask, err := timerQueueProcessBase.readLookAheadTask(readLevel, maxReadLevel)
	s.Nil(err)
	s.Equal(response.Tasks[0], lookAheadTask)
}

func (s *timerQueueProcessorBaseSuite) TestReadAndFilterTasks_NoLookAhead_NoNextPage() {
	readLevel := newTimerTaskKey(time.Now().Add(-10*time.Second), 0)
	maxReadLevel := newTimerTaskKey(time.Now().Add(1*time.Second), 0)

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       []byte("some random input next page token"),
	}

	lookAheadRequest := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maximumTimerTaskKey.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            1,
		NextPageToken:       nil,
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-5 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: nil,
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, lookAheadRequest).Return(&persistence.GetHistoryTasksResponse{}, nil).Once()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	got, err := timerQueueProcessBase.readAndFilterTasks(readLevel, maxReadLevel, request.NextPageToken)
	s.Nil(err)
	s.Equal(response.Tasks, got.timerTasks)
	s.Zero(got.lookAheadTimestamp)
	s.Nil(got.nextPageToken)
}

func (s *timerQueueProcessorBaseSuite) TestReadAndFilterTasks_NoLookAhead_HasNextPage() {
	readLevel := newTimerTaskKey(time.Now().Add(-10*time.Second), 0)
	maxReadLevel := newTimerTaskKey(time.Now().Add(1*time.Second), 0)

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       []byte("some random input next page token"),
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-5 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: []byte("some random next page token"),
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	got, err := timerQueueProcessBase.readAndFilterTasks(readLevel, maxReadLevel, request.NextPageToken)
	s.Nil(err)
	s.Equal(response.Tasks, got.timerTasks)
	s.Zero(got.lookAheadTimestamp)
	s.Equal(response.NextPageToken, got.nextPageToken)
}

func (s *timerQueueProcessorBaseSuite) TestReadAndFilterTasks_HasLookAhead_NoNextPage() {
	readLevel := newTimerTaskKey(time.Now().Add(-10*time.Second), 0)
	maxReadLevel := newTimerTaskKey(time.Now().Add(1*time.Second), 0)

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       []byte("some random input next page token"),
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-5 * time.Second),
				},
				EventID: int64(28),
			},
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(500 * time.Millisecond),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: nil,
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	got, err := timerQueueProcessBase.readAndFilterTasks(readLevel, maxReadLevel, request.NextPageToken)
	s.Nil(err)
	s.Equal([]persistence.Task{response.Tasks[0]}, got.timerTasks)
	s.Equal(response.Tasks[1].GetVisibilityTimestamp(), got.lookAheadTimestamp)
	s.Nil(got.nextPageToken)
}

func (s *timerQueueProcessorBaseSuite) TestReadAndFilterTasks_HasLookAhead_HasNextPage() {
	readLevel := newTimerTaskKey(time.Now().Add(-10*time.Second), 0)
	maxReadLevel := newTimerTaskKey(time.Now().Add(1*time.Second), 0)

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       []byte("some random input next page token"),
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-5 * time.Second),
				},
				EventID: int64(28),
			},
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(500 * time.Millisecond),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: []byte("some random next page token"),
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	got, err := timerQueueProcessBase.readAndFilterTasks(readLevel, maxReadLevel, request.NextPageToken)
	s.Nil(err)
	s.Equal([]persistence.Task{response.Tasks[0]}, got.timerTasks)
	s.Equal(response.Tasks[1].GetVisibilityTimestamp(), got.lookAheadTimestamp)
	s.Equal(response.NextPageToken, got.nextPageToken)
}

func (s *timerQueueProcessorBaseSuite) TestReadAndFilterTasks_LookAheadFailed_NoNextPage() {
	readLevel := newTimerTaskKey(time.Now().Add(-10*time.Second), 0)
	maxReadLevel := newTimerTaskKey(time.Now().Add(1*time.Second), 0)

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(readLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       []byte("some random input next page token"),
	}

	lookAheadRequest := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(maxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maximumTimerTaskKey.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            1,
		NextPageToken:       nil,
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-5 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: time.Now().Add(-500 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: nil,
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, lookAheadRequest).Return(nil, errors.New("some random error")).Times(0)

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()
	got, err := timerQueueProcessBase.readAndFilterTasks(readLevel, maxReadLevel, request.NextPageToken)
	s.Nil(err)
	s.Equal(response.Tasks, got.timerTasks)
	s.Equal(maxReadLevel.(timerTaskKey).visibilityTimestamp, got.lookAheadTimestamp)
	s.Nil(got.nextPageToken)
}

func (s *timerQueueProcessorBaseSuite) TestNotifyNewTimes() {
	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(nil, nil, nil, nil, nil)
	defer done()

	// assert the initial state
	s.True(timerQueueProcessBase.newTime.IsZero())
	select {
	case <-timerQueueProcessBase.newTimerCh:
	default:
	}

	now := time.Now()
	timerQueueProcessBase.notifyNewTimers([]persistence.Task{
		&persistence.UserTimerTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: now.Add(5 * time.Second),
				TaskID:              int64(59),
			},
			EventID: int64(28),
		},
		&persistence.UserTimerTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: now.Add(1 * time.Second),
				TaskID:              int64(59),
			},
			EventID: int64(28),
		},
	})
	select {
	case <-timerQueueProcessBase.newTimerCh:
		s.Equal(now.Add(1*time.Second), timerQueueProcessBase.newTime)
	default:
		s.Fail("should notify new timer")
	}

	timerQueueProcessBase.notifyNewTimers([]persistence.Task{
		&persistence.UserTimerTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: now.Add(10 * time.Second),
				TaskID:              int64(59),
			},
			EventID: int64(28),
		},
	})
	select {
	case <-timerQueueProcessBase.newTimerCh:
		s.Fail("should not notify new timer")
	default:
		s.Equal(now.Add(1*time.Second), timerQueueProcessBase.newTime)
	}
}

func (s *timerQueueProcessorBaseSuite) TestProcessQueueCollections_SkipRead() {
	now := time.Now()
	queueLevel := 0
	shardMaxReadLevel := newTimerTaskKey(now, 0)
	ackLevel := newTimerTaskKey(now.Add(50*time.Millisecond), 0)
	maxLevel := newTimerTaskKey(now.Add(10*time.Second), 0)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return shardMaxReadLevel
	}

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	defer done()
	timerQueueProcessBase.processQueueCollections(map[int]struct{}{queueLevel: {}})

	s.Len(timerQueueProcessBase.processingQueueCollections, 1)
	s.Len(timerQueueProcessBase.processingQueueCollections[0].Queues(), 1)
	activeQueue := timerQueueProcessBase.processingQueueCollections[0].ActiveQueue()
	s.NotNil(activeQueue)
	s.Equal(ackLevel, activeQueue.State().AckLevel())
	s.Equal(ackLevel, activeQueue.State().ReadLevel())
	s.Equal(maxLevel, activeQueue.State().MaxLevel())

	s.Empty(timerQueueProcessBase.processingQueueReadProgress)

	s.Empty(timerQueueProcessBase.backoffTimer)
	time.Sleep(100 * time.Millisecond)
	s.True(timerQueueProcessBase.nextPollTime[queueLevel].Before(s.mockShard.GetTimeSource().Now()))
	select {
	case <-timerQueueProcessBase.timerGate.Chan():
	default:
		s.Fail("timer gate should fire")
	}
}

func (s *timerQueueProcessorBaseSuite) TestProcessBatch_HasNextPage() {
	now := time.Now()
	queueLevel := 0
	ackLevel := newTimerTaskKey(now.Add(-5*time.Second), 0)
	shardMaxReadLevel := newTimerTaskKey(now.Add(1*time.Second), 0)
	maxLevel := newTimerTaskKey(now.Add(10*time.Second), 0)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"excludedDomain": {}}, true),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return shardMaxReadLevel
	}

	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(ackLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(shardMaxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       nil,
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: now.Add(-3 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "excludedDomain",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: now.Add(-2 * time.Second),
					TaskID:              int64(60),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: []byte("some random next page token"),
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	s.mockTaskProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).AnyTimes()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	defer done()
	timerQueueProcessBase.processQueueCollections(map[int]struct{}{queueLevel: {}})

	s.Len(timerQueueProcessBase.processingQueueCollections, 1)
	s.Len(timerQueueProcessBase.processingQueueCollections[0].Queues(), 1)
	activeQueue := timerQueueProcessBase.processingQueueCollections[0].ActiveQueue()
	s.NotNil(activeQueue)
	s.Equal(ackLevel, activeQueue.State().AckLevel())
	s.Equal(newTimerTaskKey(response.Tasks[1].GetVisibilityTimestamp(), 0), activeQueue.State().ReadLevel())
	s.Equal(maxLevel, activeQueue.State().MaxLevel())
	s.Len(activeQueue.(*processingQueueImpl).outstandingTasks, 1)

	s.Len(timerQueueProcessBase.processingQueueReadProgress, 1)
	s.Equal(timeTaskReadProgress{
		currentQueue:  activeQueue,
		readLevel:     ackLevel,
		maxReadLevel:  shardMaxReadLevel,
		nextPageToken: response.NextPageToken,
	}, timerQueueProcessBase.processingQueueReadProgress[0])

	s.True(timerQueueProcessBase.nextPollTime[queueLevel].IsZero())
	s.Empty(timerQueueProcessBase.backoffTimer)
	time.Sleep(100 * time.Millisecond)
	select {
	case <-timerQueueProcessBase.timerGate.Chan():
	default:
		s.Fail("timer gate should fire")
	}
}

func (s *timerQueueProcessorBaseSuite) TestProcessBatch_NoNextPage_HasLookAhead() {
	now := time.Now()
	queueLevel := 0
	ackLevel := newTimerTaskKey(now.Add(-5*time.Second), 0)
	shardMaxReadLevel := newTimerTaskKey(now.Add(1*time.Second), 0)
	maxLevel := newTimerTaskKey(now.Add(10*time.Second), 0)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"excludedDomain": {}}, true),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return shardMaxReadLevel
	}

	requestNextPageToken := []byte("some random input next page token")
	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(ackLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(shardMaxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       requestNextPageToken,
	}

	lookAheadTaskTimestamp := now.Add(50 * time.Millisecond)
	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: now.Add(-3 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "excludedDomain",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: lookAheadTaskTimestamp,
					TaskID:              int64(60),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: nil,
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()

	s.mockTaskProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).AnyTimes()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	defer done()
	timerQueueProcessBase.processingQueueReadProgress[0] = timeTaskReadProgress{
		currentQueue:  timerQueueProcessBase.processingQueueCollections[0].ActiveQueue(),
		readLevel:     ackLevel,
		maxReadLevel:  shardMaxReadLevel,
		nextPageToken: requestNextPageToken,
	}
	timerQueueProcessBase.processQueueCollections(map[int]struct{}{queueLevel: {}})

	s.Len(timerQueueProcessBase.processingQueueCollections, 1)
	s.Len(timerQueueProcessBase.processingQueueCollections[0].Queues(), 1)
	activeQueue := timerQueueProcessBase.processingQueueCollections[0].ActiveQueue()
	s.NotNil(activeQueue)
	s.Equal(ackLevel, activeQueue.State().AckLevel())
	s.Equal(newTimerTaskKey(lookAheadTaskTimestamp, 0), activeQueue.State().ReadLevel())
	s.Equal(maxLevel, activeQueue.State().MaxLevel())
	s.Len(activeQueue.(*processingQueueImpl).outstandingTasks, 1)

	s.Empty(timerQueueProcessBase.processingQueueReadProgress)

	s.Empty(timerQueueProcessBase.backoffTimer)
	s.Equal(lookAheadTaskTimestamp, timerQueueProcessBase.nextPollTime[queueLevel])
	time.Sleep(100 * time.Millisecond)
	select {
	case <-timerQueueProcessBase.timerGate.Chan():
	default:
		s.Fail("timer gate should fire")
	}
}

func (s *timerQueueProcessorBaseSuite) TestProcessBatch_NoNextPage_NoLookAhead() {
	now := time.Now()
	queueLevel := 0
	ackLevel := newTimerTaskKey(now.Add(-5*time.Second), 0)
	shardMaxReadLevel := newTimerTaskKey(now.Add(1*time.Second), 0)
	maxLevel := newTimerTaskKey(now.Add(10*time.Second), 0)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"excludedDomain": {}}, true),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return shardMaxReadLevel
	}

	requestNextPageToken := []byte("some random input next page token")
	request := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(ackLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(shardMaxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            s.mockShard.GetConfig().TimerTaskBatchSize(),
		NextPageToken:       requestNextPageToken,
	}

	lookAheadRequest := &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(shardMaxReadLevel.(timerTaskKey).visibilityTimestamp, 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(maximumTimerTaskKey.(timerTaskKey).visibilityTimestamp, 0),
		PageSize:            1,
		NextPageToken:       nil,
	}

	response := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "some random domain ID",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: now.Add(-3 * time.Second),
					TaskID:              int64(59),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
			&persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "excludedDomain",
					WorkflowID: "some random workflow ID",
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					VisibilityTimestamp: now.Add(-2 * time.Second),
					TaskID:              int64(60),
					Version:             int64(1),
				},
				EventID: int64(28),
			},
		},
		NextPageToken: nil,
	}

	mockExecutionMgr := s.mockShard.Resource.ExecutionMgr
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, request).Return(response, nil).Once()
	mockExecutionMgr.On("GetHistoryTasks", mock.Anything, lookAheadRequest).Return(&persistence.GetHistoryTasksResponse{}, nil).Once()

	s.mockTaskProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).AnyTimes()

	timerQueueProcessBase, done := s.newTestTimerQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	defer done()
	timerQueueProcessBase.processingQueueReadProgress[0] = timeTaskReadProgress{
		currentQueue:  timerQueueProcessBase.processingQueueCollections[0].ActiveQueue(),
		readLevel:     ackLevel,
		maxReadLevel:  shardMaxReadLevel,
		nextPageToken: requestNextPageToken,
	}
	timerQueueProcessBase.processQueueCollections(map[int]struct{}{queueLevel: {}})

	s.Len(timerQueueProcessBase.processingQueueCollections, 1)
	s.Len(timerQueueProcessBase.processingQueueCollections[0].Queues(), 1)
	activeQueue := timerQueueProcessBase.processingQueueCollections[0].ActiveQueue()
	s.NotNil(activeQueue)
	s.Equal(ackLevel, activeQueue.State().AckLevel())
	s.Equal(shardMaxReadLevel, activeQueue.State().ReadLevel())
	s.Equal(maxLevel, activeQueue.State().MaxLevel())
	s.Len(activeQueue.(*processingQueueImpl).outstandingTasks, 1)

	s.Empty(timerQueueProcessBase.processingQueueReadProgress)

	_, ok := timerQueueProcessBase.nextPollTime[queueLevel]
	s.True(ok) // this is the poll time for max poll interval
	time.Sleep(100 * time.Millisecond)
	select {
	case <-timerQueueProcessBase.timerGate.Chan():
		s.Fail("timer gate should not fire")
	default:
	}
}

func (s *timerQueueProcessorBaseSuite) TestTimerProcessorPump_HandleAckLevelUpdate() {
	now := time.Now()
	queueLevel := 0
	ackLevel := newTimerTaskKey(now.Add(50*time.Millisecond), 0)
	maxLevel := newTimerTaskKey(now.Add(10*time.Second), 0)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTimerTaskKey(now, 0)
	}

	timerQueueProcessBase, _ := s.newTestTimerQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	timerQueueProcessBase.options.UpdateAckInterval = dynamicproperties.GetDurationPropertyFn(1 * time.Millisecond)
	updatedCh := make(chan struct{}, 1)
	timerQueueProcessBase.updateAckLevelFn = func() (bool, task.Key, error) {
		updatedCh <- struct{}{}
		return false, nil, nil
	}
	timerQueueProcessBase.Start()
	defer timerQueueProcessBase.Stop()

	select {
	case <-updatedCh:
		return
	case <-time.After(100 * time.Millisecond):
		s.Fail("Ack level update not called")
	}
}

func (s *timerQueueProcessorBaseSuite) TestTimerProcessorPump_SplitQueue() {
	now := time.Now()
	queueLevel := 0
	ackLevel := newTimerTaskKey(now.Add(50*time.Millisecond), 0)
	maxLevel := newTimerTaskKey(now.Add(10*time.Second), 0)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTimerTaskKey(now, 0)
	}

	timerQueueProcessBase, _ := s.newTestTimerQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	timerQueueProcessBase.options.SplitQueueInterval = dynamicproperties.GetDurationPropertyFn(1 * time.Millisecond)
	splittedCh := make(chan struct{}, 1)
	timerQueueProcessBase.splitProcessingQueueCollectionFn = func(splitPolicy ProcessingQueueSplitPolicy, upsertPollTimeFn func(int, time.Time)) {
		splittedCh <- struct{}{}
	}
	timerQueueProcessBase.Start()
	defer timerQueueProcessBase.Stop()

	select {
	case <-splittedCh:
		return
	case <-time.After(100 * time.Millisecond):
		s.Fail("splitProcessingQueueCollectionFn not called")
	}
}

func (s *timerQueueProcessorBaseSuite) newTestTimerQueueProcessorBase(
	processingQueueStates []ProcessingQueueState,
	updateMaxReadLevel updateMaxReadLevelFn,
	updateClusterAckLevel updateClusterAckLevelFn,
	updateProcessingQueueStates updateProcessingQueueStatesFn,
	queueShutdown queueShutdownFn,
) (*timerQueueProcessorBase, func()) {
	timerGate := clock.NewTimerGate(s.mockShard.GetTimeSource())

	return newTimerQueueProcessorBase(
			s.clusterName,
			s.mockShard,
			processingQueueStates,
			s.mockTaskProcessor,
			timerGate,
			newTimerQueueProcessorOptions(s.mockShard.GetConfig(), true, false),
			updateMaxReadLevel,
			updateClusterAckLevel,
			updateProcessingQueueStates,
			queueShutdown,
			nil,
			nil,
			s.logger,
			s.metricsClient,
		), func() {
			timerGate.Stop()
		}
}
