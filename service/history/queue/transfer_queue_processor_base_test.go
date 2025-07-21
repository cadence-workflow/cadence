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
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

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
	transferQueueProcessorBaseSuite struct {
		suite.Suite
		*require.Assertions

		controller           *gomock.Controller
		mockShard            *shard.TestContext
		mockTaskProcessor    *task.MockProcessor
		mockQueueSplitPolicy *MockProcessingQueueSplitPolicy

		logger        log.Logger
		metricsClient metrics.Client
		metricsScope  metrics.Scope
	}
)

func TestTransferQueueProcessorBaseSuite(t *testing.T) {
	s := new(transferQueueProcessorBaseSuite)
	suite.Run(t, s)
}

func (s *transferQueueProcessorBaseSuite) SetupTest() {
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

	s.logger = testlogger.New(s.Suite.T())
	s.metricsClient = metrics.NewClient(tally.NoopScope, metrics.History)
	s.metricsScope = s.metricsClient.Scope(metrics.TransferQueueProcessorScope)
}

func (s *transferQueueProcessorBaseSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *transferQueueProcessorBaseSuite) TestProcessQueueCollections_NoNextPage_WithNextQueue() {
	queueLevel := 0
	ackLevel := newTransferTaskKey(0)
	maxLevel := newTransferTaskKey(1000)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
		NewProcessingQueueState(
			queueLevel,
			newTransferTaskKey(1000),
			newTransferTaskKey(10000),
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(10000)
	}
	taskInfos := []persistence.Task{
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain1",
			},
			TaskData: persistence.TaskData{
				TaskID: 1,
			},
		},
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain2",
			},
			TaskData: persistence.TaskData{
				TaskID: 10,
			},
		},
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain1",
			},
			TaskData: persistence.TaskData{
				TaskID: 100,
			},
		},
	}
	mockExecutionManager := s.mockShard.Resource.ExecutionMgr
	mockExecutionManager.On("GetHistoryTasks", mock.Anything, &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(ackLevel.(transferTaskKey).taskID + 1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(maxLevel.(transferTaskKey).taskID + 1),
		PageSize:            s.mockShard.GetConfig().TransferTaskBatchSize(),
	}).Return(&persistence.GetHistoryTasksResponse{
		Tasks:         taskInfos,
		NextPageToken: nil,
	}, nil).Once()

	s.mockTaskProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).AnyTimes()

	processorBase := s.newTestTransferQueueProcessorBase(
		processingQueueStates,
		updateMaxReadLevel,
		nil,
		nil,
		nil,
	)

	processorBase.processQueueCollections()

	queueCollection := processorBase.processingQueueCollections[0]
	s.NotNil(queueCollection.ActiveQueue())
	s.True(taskKeyEquals(maxLevel, queueCollection.Queues()[0].State().ReadLevel()))

	s.True(processorBase.shouldProcess[queueLevel])
	select {
	case <-processorBase.processCh:
	default:
		s.Fail("processCh should be unblocked")
	}
}

func (s *transferQueueProcessorBaseSuite) TestProcessQueueCollections_NoNextPage_NoNextQueue() {
	queueLevel := 0
	ackLevel := newTransferTaskKey(0)
	maxLevel := newTransferTaskKey(1000)
	shardMaxLevel := newTransferTaskKey(500)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return shardMaxLevel
	}
	taskInfos := []persistence.Task{
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain1",
			},
			TaskData: persistence.TaskData{
				TaskID: 1,
			},
		},
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain2",
			},
			TaskData: persistence.TaskData{
				TaskID: 10,
			},
		},
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain1",
			},
			TaskData: persistence.TaskData{
				TaskID: 100,
			},
		},
	}
	mockExecutionManager := s.mockShard.Resource.ExecutionMgr
	mockExecutionManager.On("GetHistoryTasks", mock.Anything, &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(ackLevel.(transferTaskKey).taskID + 1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(shardMaxLevel.(transferTaskKey).taskID + 1),
		PageSize:            s.mockShard.GetConfig().TransferTaskBatchSize(),
	}).Return(&persistence.GetHistoryTasksResponse{
		Tasks:         taskInfos,
		NextPageToken: nil,
	}, nil).Once()

	s.mockTaskProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).AnyTimes()

	processorBase := s.newTestTransferQueueProcessorBase(
		processingQueueStates,
		updateMaxReadLevel,
		nil,
		nil,
		nil,
	)

	processorBase.processQueueCollections()

	queueCollection := processorBase.processingQueueCollections[0]
	s.NotNil(queueCollection.ActiveQueue())
	s.True(taskKeyEquals(shardMaxLevel, queueCollection.Queues()[0].State().ReadLevel()))

	shouldProcess, ok := processorBase.shouldProcess[queueLevel]
	if ok {
		s.False(shouldProcess)
	}
	select {
	case <-processorBase.processCh:
		s.Fail("processCh should be blocked")
	default:
	}
}

func (s *transferQueueProcessorBaseSuite) TestProcessQueueCollections_WithNextPage_NotThrottled() {
	queueLevel := 0
	ackLevel := newTransferTaskKey(0)
	maxLevel := newTransferTaskKey(1000)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(10000)
	}
	taskInfos := []persistence.Task{
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain1",
			},
			TaskData: persistence.TaskData{
				TaskID: 1,
			},
		},
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain2",
			},
			TaskData: persistence.TaskData{
				TaskID: 10,
			},
		},
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain1",
			},
			TaskData: persistence.TaskData{
				TaskID: 100,
			},
		},
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: "testDomain2",
			},
			TaskData: persistence.TaskData{
				TaskID: 500,
			},
		},
	}
	mockExecutionManager := s.mockShard.Resource.ExecutionMgr
	mockExecutionManager.On("GetHistoryTasks", mock.Anything, &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(ackLevel.(transferTaskKey).taskID + 1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(maxLevel.(transferTaskKey).taskID + 1),
		PageSize:            s.mockShard.GetConfig().TransferTaskBatchSize(),
	}).Return(&persistence.GetHistoryTasksResponse{
		Tasks:         taskInfos,
		NextPageToken: []byte{1, 2, 3},
	}, nil).Once()

	s.mockTaskProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).AnyTimes()

	processorBase := s.newTestTransferQueueProcessorBase(
		processingQueueStates,
		updateMaxReadLevel,
		nil,
		nil,
		nil,
	)

	processorBase.processQueueCollections()

	queueCollection := processorBase.processingQueueCollections[0]
	s.NotNil(queueCollection.ActiveQueue())
	s.True(taskKeyEquals(newTransferTaskKey(500), queueCollection.Queues()[0].State().ReadLevel()))

	s.True(processorBase.shouldProcess[queueLevel])
	select {
	case <-processorBase.processCh:
	default:
		s.Fail("processCh should be unblocked")
	}
}

func (s *transferQueueProcessorBaseSuite) TestProcessQueueCollections_WithNextPage_Throttled() {
	queueLevel := defaultProcessingQueueLevel + 1
	ackLevel := newTransferTaskKey(0)
	maxLevel := newTransferTaskKey(1000)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(10000)
	}
	taskInfos := []persistence.Task{
		&persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   "testDomain1",
				WorkflowID: "testWorkflowID",
				RunID:      "testRunID",
			},
			TaskData: persistence.TaskData{
				TaskID: 500,
			},
		},
	}
	mockExecutionManager := s.mockShard.Resource.ExecutionMgr
	mockExecutionManager.On("GetHistoryTasks", mock.Anything, &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(ackLevel.(transferTaskKey).taskID + 1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(maxLevel.(transferTaskKey).taskID + 1),
		PageSize:            s.mockShard.GetConfig().TransferTaskBatchSize(),
	}).Return(&persistence.GetHistoryTasksResponse{
		Tasks:         taskInfos,
		NextPageToken: []byte{1, 2, 3},
	}, nil).Once()

	// return false to indicate that taskCh is full
	// so the queue should backoff
	s.mockTaskProcessor.EXPECT().TrySubmit(gomock.Any()).Return(false, nil).AnyTimes()

	processorBase := s.newTestTransferQueueProcessorBase(
		processingQueueStates,
		updateMaxReadLevel,
		nil,
		nil,
		nil,
	)
	processorBase.options.PollBackoffInterval = dynamicproperties.GetDurationPropertyFn(time.Millisecond * 100)

	processorBase.processQueueCollections()

	queueCollection := processorBase.processingQueueCollections[0]
	s.NotNil(queueCollection.ActiveQueue())
	s.True(taskKeyEquals(newTransferTaskKey(500), queueCollection.Queues()[0].State().ReadLevel()))

	processorBase.processingLock.Lock()
	s.False(processorBase.shouldProcess[queueLevel])
	_, ok := processorBase.backoffTimer[queueLevel]
	s.True(ok)
	processorBase.processingLock.Unlock()
	select {
	case <-processorBase.processCh:
		s.Fail("processCh should be blocked before the backoff timer fires")
	default:
	}

	time.Sleep(time.Millisecond * 300)
	processorBase.processingLock.Lock()
	s.True(processorBase.shouldProcess[queueLevel])
	_, ok = processorBase.backoffTimer[queueLevel]
	s.False(ok)
	processorBase.processingLock.Unlock()
	select {
	case <-processorBase.processCh:
	default:
		s.Fail("processCh should be unblocked after the backoff timer fires")
	}
}

func (s *transferQueueProcessorBaseSuite) TestReadTasks_NoNextPage() {
	readLevel := newTransferTaskKey(3)
	maxReadLevel := newTransferTaskKey(100)

	mockExecutionManager := s.mockShard.Resource.ExecutionMgr
	getTransferTaskResponse := &persistence.GetHistoryTasksResponse{
		Tasks:         []persistence.Task{&persistence.DecisionTask{}, &persistence.DecisionTask{}, &persistence.DecisionTask{}},
		NextPageToken: nil,
	}
	mockExecutionManager.On("GetHistoryTasks", mock.Anything, &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(readLevel.(transferTaskKey).taskID + 1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(maxReadLevel.(transferTaskKey).taskID + 1),
		PageSize:            s.mockShard.GetConfig().TransferTaskBatchSize(),
	}).Return(getTransferTaskResponse, nil).Once()

	processorBase := s.newTestTransferQueueProcessorBase(
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	tasks, more, err := processorBase.readTasks(readLevel, maxReadLevel)
	s.NoError(err)
	s.Len(tasks, len(getTransferTaskResponse.Tasks))
	s.False(more)
}

func (s *transferQueueProcessorBaseSuite) TestReadTasks_WithNextPage() {
	readLevel := newTransferTaskKey(3)
	maxReadLevel := newTransferTaskKey(10)

	mockExecutionManager := s.mockShard.Resource.ExecutionMgr
	getTransferTaskResponse := &persistence.GetHistoryTasksResponse{
		Tasks:         []persistence.Task{&persistence.DecisionTask{}, &persistence.DecisionTask{}, &persistence.DecisionTask{}},
		NextPageToken: []byte{1, 2, 3},
	}
	mockExecutionManager.On("GetHistoryTasks", mock.Anything, &persistence.GetHistoryTasksRequest{
		TaskCategory:        persistence.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(readLevel.(transferTaskKey).taskID + 1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(maxReadLevel.(transferTaskKey).taskID + 1),
		PageSize:            s.mockShard.GetConfig().TransferTaskBatchSize(),
	}).Return(getTransferTaskResponse, nil).Once()

	processorBase := s.newTestTransferQueueProcessorBase(
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	tasks, more, err := processorBase.readTasks(readLevel, maxReadLevel)
	s.NoError(err)
	s.Len(tasks, len(getTransferTaskResponse.Tasks))
	s.True(more)
}

func (s *transferQueueProcessorBaseSuite) TestTransferProcessorPump_ProcessChHasItem() {
	queueLevel := 0
	ackLevel := newTransferTaskKey(0)
	maxLevel := newTransferTaskKey(1000)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
		NewProcessingQueueState(
			queueLevel,
			newTransferTaskKey(1000),
			newTransferTaskKey(10000),
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(10000)
	}

	processorBase := s.newTestTransferQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	processedCh := make(chan struct{}, 1)
	processorBase.processQueueCollectionsFn = func() {
		processedCh <- struct{}{}
	}

	processorBase.Start()
	defer processorBase.Stop()

	processorBase.processCh <- struct{}{}

	select {
	case <-processedCh:
		return
	case <-time.After(100 * time.Millisecond):
		s.Fail("processQueueCollectionsFn not called")
	}
}

func (s *transferQueueProcessorBaseSuite) TestTransferProcessorPump_NotifyChHasItem() {
	queueLevel := 0
	ackLevel := newTransferTaskKey(0)
	maxLevel := newTransferTaskKey(1000)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
		NewProcessingQueueState(
			queueLevel,
			newTransferTaskKey(1000),
			newTransferTaskKey(10000),
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(10000)
	}

	processorBase := s.newTestTransferQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	processedCh := make(chan struct{}, 1)
	processorBase.processQueueCollectionsFn = func() {
		processedCh <- struct{}{}
	}

	processorBase.Start()
	defer processorBase.Stop()

	processorBase.notifyCh <- struct{}{}

	select {
	case <-processedCh:
		return
	case <-time.After(100 * time.Millisecond):
		s.Fail("processQueueCollectionsFn not called")
	}
}

func (s *transferQueueProcessorBaseSuite) TestTransferProcessorPump_UpdateAckLevel() {
	queueLevel := 0
	ackLevel := newTransferTaskKey(0)
	maxLevel := newTransferTaskKey(1000)
	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			queueLevel,
			ackLevel,
			maxLevel,
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
		NewProcessingQueueState(
			queueLevel,
			newTransferTaskKey(1000),
			newTransferTaskKey(10000),
			NewDomainFilter(map[string]struct{}{"testDomain1": {}}, false),
		),
	}
	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(10000)
	}

	processorBase := s.newTestTransferQueueProcessorBase(processingQueueStates, updateMaxReadLevel, nil, nil, nil)
	processorBase.options.UpdateAckInterval = dynamicproperties.GetDurationPropertyFn(1 * time.Millisecond)
	updatedCh := make(chan struct{}, 1)
	processorBase.processQueueCollectionsFn = func() {}
	processorBase.updateAckLevelFn = func() (bool, task.Key, error) {
		updatedCh <- struct{}{}
		return false, nil, nil
	}

	processorBase.Start()
	defer processorBase.Stop()

	select {
	case <-updatedCh:
		return
	case <-time.After(100 * time.Millisecond):
		s.Fail("updateAckLevelFn not called")
	}
}

func (s *transferQueueProcessorBaseSuite) newTestTransferQueueProcessorBase(
	processingQueueStates []ProcessingQueueState,
	maxReadLevel updateMaxReadLevelFn,
	updateClusterAckLevel updateClusterAckLevelFn,
	updateProcessingQueueStates updateProcessingQueueStatesFn,
	transferQueueShutdown queueShutdownFn,
) *transferQueueProcessorBase {
	processorBase := newTransferQueueProcessorBase(
		s.mockShard,
		processingQueueStates,
		s.mockTaskProcessor,
		newTransferQueueProcessorOptions(s.mockShard.GetConfig(), true, false),
		maxReadLevel,
		updateClusterAckLevel,
		updateProcessingQueueStates,
		transferQueueShutdown,
		nil,
		nil,
		s.logger,
		s.metricsClient,
	)
	for _, queueCollections := range processorBase.processingQueueCollections {
		processorBase.shouldProcess[queueCollections.Level()] = true
	}
	return processorBase
}
