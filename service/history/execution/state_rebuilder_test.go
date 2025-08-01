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

package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/collection"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/shard"
)

type (
	stateRebuilderSuite struct {
		suite.Suite
		*require.Assertions

		controller               *gomock.Controller
		mockShard                *shard.TestContext
		mockEventsCache          *events.MockCache
		mockTaskRefresher        *MockMutableStateTaskRefresher
		mockDomainCache          *cache.MockDomainCache
		mockActiveClusterManager *activecluster.MockManager
		mockHistoryV2Mgr         *mocks.HistoryV2Manager
		logger                   log.Logger

		domainID   string
		workflowID string
		runID      string

		nDCStateRebuilder *stateRebuilderImpl
	}

	testHistoryEvents struct {
		Events []*types.HistoryEvent
		Size   int
	}
)

func TestStateRebuilderSuite(t *testing.T) {
	s := new(stateRebuilderSuite)
	suite.Run(t, s)
}

func (s *stateRebuilderSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockTaskRefresher = NewMockMutableStateTaskRefresher(s.controller)

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

	s.mockHistoryV2Mgr = s.mockShard.Resource.HistoryMgr
	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockEventsCache = s.mockShard.MockEventsCache
	s.mockEventsCache.EXPECT().PutEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	s.logger = s.mockShard.GetLogger()

	s.workflowID = "some random workflow ID"
	s.runID = uuid.New()
	s.nDCStateRebuilder = NewStateRebuilder(
		s.mockShard,
		s.logger,
	).(*stateRebuilderImpl)
	s.nDCStateRebuilder.taskRefresher = s.mockTaskRefresher
}

func (s *stateRebuilderSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *stateRebuilderSuite) TestInitializeBuilders() {
	mutableState, stateBuilder := s.nDCStateRebuilder.initializeBuilders(constants.TestGlobalDomainEntry)
	s.NotNil(mutableState)
	s.NotNil(stateBuilder)
	s.NotNil(mutableState.GetVersionHistories())
}

func (s *stateRebuilderSuite) TestApplyEvents() {

	requestID := uuid.New()
	historyEvents := []*types.HistoryEvent{
		{
			ID:                                      1,
			EventType:                               types.EventTypeWorkflowExecutionStarted.Ptr(),
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
		},
		{
			ID:                                       2,
			EventType:                                types.EventTypeWorkflowExecutionSignaled.Ptr(),
			WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{},
		},
	}

	workflowIdentifier := definition.NewWorkflowIdentifier(s.domainID, s.workflowID, s.runID)

	mockStateBuilder := NewMockStateBuilder(s.controller)
	mockStateBuilder.EXPECT().ApplyEvents(
		s.domainID,
		requestID,
		types.WorkflowExecution{
			WorkflowID: s.workflowID,
			RunID:      s.runID,
		},
		historyEvents,
		[]*types.HistoryEvent(nil),
	).Return(nil, nil).Times(1)

	err := s.nDCStateRebuilder.applyEvents(workflowIdentifier, mockStateBuilder, historyEvents, requestID)
	s.NoError(err)
}

func (s *stateRebuilderSuite) TestPagination() {
	firstEventID := commonconstants.FirstEventID
	nextEventID := int64(101)
	branchToken := []byte("some random branch token")
	domainName := "some random domain name"

	event1 := &types.HistoryEvent{
		ID:                                      1,
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
	}
	event2 := &types.HistoryEvent{
		ID:                                   2,
		DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
	}
	event3 := &types.HistoryEvent{
		ID:                                 3,
		DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{},
	}
	event4 := &types.HistoryEvent{
		ID:                                   4,
		DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{},
	}
	event5 := &types.HistoryEvent{
		ID:                                   5,
		ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{},
	}
	history1 := []*types.History{{Events: []*types.HistoryEvent{event1, event2, event3}}}
	history2 := []*types.History{{Events: []*types.HistoryEvent{event4, event5}}}
	history := append(history1, history2...)
	pageToken := []byte("some random token")
	s.mockDomainCache.EXPECT().GetDomainName(s.domainID).Return(domainName, nil).AnyTimes()
	s.mockHistoryV2Mgr.On("ReadHistoryBranchByBatch", mock.Anything, &persistence.ReadHistoryBranchRequest{
		BranchToken:   branchToken,
		MinEventID:    firstEventID,
		MaxEventID:    nextEventID,
		PageSize:      NDCDefaultPageSize,
		NextPageToken: nil,
		ShardID:       common.IntPtr(s.mockShard.GetShardID()),
		DomainName:    domainName,
	}).Return(&persistence.ReadHistoryBranchByBatchResponse{
		History:       history1,
		NextPageToken: pageToken,
		Size:          12345,
	}, nil).Once()
	s.mockHistoryV2Mgr.On("ReadHistoryBranchByBatch", mock.Anything, &persistence.ReadHistoryBranchRequest{
		BranchToken:   branchToken,
		MinEventID:    firstEventID,
		MaxEventID:    nextEventID,
		PageSize:      NDCDefaultPageSize,
		NextPageToken: pageToken,
		ShardID:       common.IntPtr(s.mockShard.GetShardID()),
		DomainName:    domainName,
	}).Return(&persistence.ReadHistoryBranchByBatchResponse{
		History:       history2,
		NextPageToken: nil,
		Size:          67890,
	}, nil).Once()

	paginationFn := s.nDCStateRebuilder.getPaginationFn(context.Background(), firstEventID, nextEventID, branchToken, s.domainID)
	iter := collection.NewPagingIterator(paginationFn)

	result := []*types.History{}
	for iter.HasNext() {
		item, err := iter.Next()
		s.NoError(err)
		result = append(result, item.(*types.History))
	}

	s.Equal(history, result)
}

func (s *stateRebuilderSuite) TestRebuild() {
	requestID := uuid.New()
	version := int64(12)
	lastEventID := int64(2)
	branchToken := []byte("other random branch token")
	targetBranchToken := []byte("some other random branch token")
	now := time.Now()
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}

	targetDomainID := uuid.New()
	targetDomainName := "other random domain name"
	targetWorkflowID := "other random workflow ID"
	targetRunID := uuid.New()

	firstEventID := commonconstants.FirstEventID
	nextEventID := lastEventID + 1
	events1 := []*types.HistoryEvent{{
		ID:        1,
		Version:   version,
		EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
			WorkflowType:                        &types.WorkflowType{Name: "some random workflow type"},
			TaskList:                            &types.TaskList{Name: "some random workflow type"},
			Input:                               []byte("some random input"),
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(123),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(233),
			Identity:                            "some random identity",
			PartitionConfig:                     partitionConfig,
		},
	}}
	events2 := []*types.HistoryEvent{{
		ID:        2,
		Version:   version,
		EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
			SignalName: "some random signal name",
			Input:      []byte("some random signal input"),
			Identity:   "some random identity",
		},
	}}
	history1 := []*types.History{{Events: events1}}
	history2 := []*types.History{{Events: events2}}
	pageToken := []byte("some random pagination token")

	historySize1 := 12345
	historySize2 := 67890

	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(targetDomainName, nil).AnyTimes()
	s.mockHistoryV2Mgr.On("ReadHistoryBranchByBatch", mock.Anything, &persistence.ReadHistoryBranchRequest{
		BranchToken:   branchToken,
		MinEventID:    firstEventID,
		MaxEventID:    nextEventID,
		PageSize:      NDCDefaultPageSize,
		NextPageToken: nil,
		ShardID:       common.IntPtr(s.mockShard.GetShardID()),
		DomainName:    targetDomainName,
	}).Return(&persistence.ReadHistoryBranchByBatchResponse{
		History:       history1,
		NextPageToken: pageToken,
		Size:          historySize1,
	}, nil).Once()
	s.mockHistoryV2Mgr.On("ReadHistoryBranchByBatch", mock.Anything, &persistence.ReadHistoryBranchRequest{
		BranchToken:   branchToken,
		MinEventID:    firstEventID,
		MaxEventID:    nextEventID,
		PageSize:      NDCDefaultPageSize,
		NextPageToken: pageToken,
		ShardID:       common.IntPtr(s.mockShard.GetShardID()),
		DomainName:    targetDomainName,
	}).Return(&persistence.ReadHistoryBranchByBatchResponse{
		History:       history2,
		NextPageToken: nil,
		Size:          historySize2,
	}, nil).Once()

	s.mockDomainCache.EXPECT().GetDomainByID(targetDomainID).Return(cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: targetDomainID, Name: targetDomainName},
		&persistence.DomainConfig{},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	), nil).AnyTimes()
	s.mockTaskRefresher.EXPECT().RefreshTasks(gomock.Any(), now, gomock.Any()).Return(nil).Times(1)

	targetBranchTokenFn := func() ([]byte, error) {
		return targetBranchToken, nil
	}

	rebuildMutableState, rebuiltHistorySize, err := s.nDCStateRebuilder.Rebuild(
		context.Background(),
		now,
		definition.NewWorkflowIdentifier(s.domainID, s.workflowID, s.runID),
		branchToken,
		lastEventID,
		version,
		definition.NewWorkflowIdentifier(targetDomainID, targetWorkflowID, targetRunID),
		targetBranchTokenFn,
		requestID,
		nil,
	)
	s.NoError(err)
	s.NotNil(rebuildMutableState)
	rebuildExecutionInfo := rebuildMutableState.GetExecutionInfo()
	s.Equal(targetDomainID, rebuildExecutionInfo.DomainID)
	s.Equal(targetWorkflowID, rebuildExecutionInfo.WorkflowID)
	s.Equal(targetRunID, rebuildExecutionInfo.RunID)
	s.Equal(partitionConfig, rebuildExecutionInfo.PartitionConfig)
	s.Equal(int64(historySize1+historySize2), rebuiltHistorySize)
	s.Equal(persistence.NewVersionHistories(
		persistence.NewVersionHistory(
			targetBranchToken,
			[]*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(lastEventID, version)},
		),
	), rebuildMutableState.GetVersionHistories())
	s.Equal(rebuildMutableState.GetExecutionInfo().StartTimestamp, now)
}

func (s *stateRebuilderSuite) TestRebuildResetEventIDNotNil() {
	requestID := uuid.New()
	version := int64(12)
	branchToken := []byte("other random branch token")
	targetBranchToken := []byte("some other random branch token")
	now := time.Now()
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}

	targetDomainID := uuid.New()
	targetDomainName := "other random domain name"
	targetWorkflowID := "other random workflow ID"
	targetRunID := uuid.New()

	firstEventID := commonconstants.FirstEventID

	workflowEvents := []*types.HistoryEvent{
		{
			ID:        1,
			Version:   version,
			EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
				WorkflowType:                        &types.WorkflowType{Name: "some random workflow type"},
				TaskList:                            &types.TaskList{Name: "some random workflow type"},
				Input:                               []byte("some random input"),
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(123),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(233),
				Identity:                            "some random identity",
				PartitionConfig:                     partitionConfig,
			},
		},
		{
			ID:                                   2,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        3,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 2,
			},
		},
		{
			ID:        4,
			Version:   version,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 2,
				StartedEventID:   3,
			},
		},
		{
			ID:        5,
			Version:   version,
			EventType: types.EventTypeActivityTaskScheduled.Ptr(),
			ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
				ActivityID: "1",
			},
		},
		{
			ID:        6,
			Version:   version,
			EventType: types.EventTypeActivityTaskStarted.Ptr(),
			ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
				ScheduledEventID: 5,
			},
		},
		{
			ID:        7,
			Version:   version,
			EventType: types.EventTypeActivityTaskCompleted.Ptr(),
			ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
				ScheduledEventID: 5,
				StartedEventID:   6,
			},
		},
		{
			ID:                                   8,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        9,
			Version:   version,
			EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
			DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
				ScheduledEventID: 8,
			},
		},
		{
			ID:        10,
			Version:   version,
			EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
				TaskList: &types.TaskList{Name: "some random workflow type"},
			},
		},
		{
			ID:        11,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 10,
			},
		},
		{
			ID:        12,
			Version:   version,
			EventType: types.EventTypeDecisionTaskFailed.Ptr(),
			DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
				ScheduledEventID: 10,
				StartedEventID:   11,
			},
		},
		{
			ID:                                   13,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        14,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 13,
			},
		},
		{
			ID:        15,
			Version:   version,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 13,
				StartedEventID:   14,
			},
		},
		{
			ID:                          16,
			Version:                     version,
			EventType:                   types.EventTypeTimerStarted.Ptr(),
			TimerStartedEventAttributes: &types.TimerStartedEventAttributes{},
		},
		{
			ID:                        17,
			Version:                   version,
			EventType:                 types.EventTypeTimerFired.Ptr(),
			TimerFiredEventAttributes: &types.TimerFiredEventAttributes{},
		},
		{
			ID:                                       18,
			Version:                                  version,
			EventType:                                types.EventTypeWorkflowExecutionSignaled.Ptr(),
			WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{},
		},
		{
			ID:                                   19,
			Version:                              version,
			EventType:                            types.EventTypeActivityTaskScheduled.Ptr(),
			ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{},
		},
		{
			ID:        20,
			Version:   version,
			EventType: types.EventTypeActivityTaskStarted.Ptr(),
			ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
				ScheduledEventID: 19,
			},
		},
		{
			ID:        21,
			Version:   version,
			EventType: types.EventTypeActivityTaskFailed.Ptr(),
			ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
				ScheduledEventID: 19,
				StartedEventID:   20,
			},
		},
		{
			ID:                                   22,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        23,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 22,
			},
		},
		{
			ID:        24,
			Version:   version,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 22,
				StartedEventID:   23,
			},
		},
		{
			ID:        25,
			Version:   version,
			EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
			WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{},
		},
	}

	history := []testHistoryEvents{
		{
			Events: workflowEvents[:2],
			Size:   1,
		},
		{
			Events: workflowEvents[2:3],
			Size:   2,
		},
		{
			Events: workflowEvents[3:6],
			Size:   3,
		},
		{
			Events: workflowEvents[6:8],
			Size:   4,
		},
		{
			Events: workflowEvents[8:10],
			Size:   5,
		},
		{
			Events: workflowEvents[10:11],
			Size:   6,
		},
		{
			Events: workflowEvents[11:13],
			Size:   7,
		},
		{
			Events: workflowEvents[13:14],
			Size:   8,
		},
		{
			Events: workflowEvents[14:16],
			Size:   9,
		},
		{
			Events: workflowEvents[16:19],
			Size:   10,
		},
		{
			Events: workflowEvents[19:22],
			Size:   11,
		},
		{
			Events: workflowEvents[22:23],
			Size:   12,
		},
		{
			Events: workflowEvents[23:24],
			Size:   13,
		},
		{
			Events: workflowEvents[24:25],
			Size:   14,
		},
	}

	testCases := []struct {
		name         string
		resetEventID int64
		setupMock    func(mockTaskRefresher *MockMutableStateTaskRefresher)
		err          error
	}{
		{
			name:         "error - reset on decision task scheduled",
			resetEventID: 10,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[9].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task scheduled",
			resetEventID: 5,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[4].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task started",
			resetEventID: 6,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[5].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task completed",
			resetEventID: 7,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[6].EventType.String()),
			},
		},
		{
			name:         "error - reset on timer started",
			resetEventID: 16,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[15].EventType.String()),
			},
		},
		{
			name:         "error - reset on timer fired",
			resetEventID: 17,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[16].EventType.String()),
			},
		},
		{
			name:         "error - reset on workflow execution signaled",
			resetEventID: 18,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[17].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task failed",
			resetEventID: 21,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[20].EventType.String()),
			},
		},
		{
			name:         "error - reset on workflow execution completed",
			resetEventID: 25,
			setupMock:    func(mockTaskRefresher *MockMutableStateTaskRefresher) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[24].EventType.String()),
			},
		},
		{
			name:         "success - reset on decision task started",
			resetEventID: 23,
			setupMock: func(mockTaskRefresher *MockMutableStateTaskRefresher) {
				mockTaskRefresher.EXPECT().RefreshTasks(gomock.Any(), now, gomock.Any()).Return(nil).Times(1)
			},
			err: nil,
		},
		{
			name:         "success - reset on decision task timed out",
			resetEventID: 9,
			setupMock: func(mockTaskRefresher *MockMutableStateTaskRefresher) {
				mockTaskRefresher.EXPECT().RefreshTasks(gomock.Any(), now, gomock.Any()).Return(nil).Times(1)
			},
			err: nil,
		},
		{
			name:         "success - reset on decision task failed",
			resetEventID: 12,
			setupMock: func(mockTaskRefresher *MockMutableStateTaskRefresher) {
				mockTaskRefresher.EXPECT().RefreshTasks(gomock.Any(), now, gomock.Any()).Return(nil).Times(1)
			},
			err: nil,
		},
		{
			name:         "success - reset on decision task completed",
			resetEventID: 24,
			setupMock: func(mockTaskRefresher *MockMutableStateTaskRefresher) {
				mockTaskRefresher.EXPECT().RefreshTasks(gomock.Any(), now, gomock.Any()).Return(nil).Times(1)
			},
			err: nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			pageToken := []byte("some random pagination token")
			nextEventID := tc.resetEventID - 1 // reset passes next event ID as the reset event ID minus 1

			s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(targetDomainName, nil).AnyTimes()

			s.nDCStateRebuilder = NewStateRebuilder(
				s.mockShard,
				s.logger,
			).(*stateRebuilderImpl)

			s.nDCStateRebuilder.taskRefresher = s.mockTaskRefresher

			counter := int64(0)
			historySize := 0
			for index, historyBatch := range history {
				token := pageToken

				if index == 0 {
					token = nil
				}

				counter += int64(len(historyBatch.Events))

				if counter >= tc.resetEventID {
					pageToken = nil
				}

				s.mockHistoryV2Mgr.On("ReadHistoryBranchByBatch", mock.Anything, &persistence.ReadHistoryBranchRequest{
					BranchToken:   branchToken,
					MinEventID:    firstEventID,
					MaxEventID:    tc.resetEventID + 1, // Rebuild adds 1 to nextEventID
					PageSize:      NDCDefaultPageSize,
					NextPageToken: token,
					ShardID:       common.IntPtr(s.mockShard.GetShardID()),
					DomainName:    targetDomainName,
				}).Return(&persistence.ReadHistoryBranchByBatchResponse{
					History:       []*types.History{{Events: historyBatch.Events}},
					NextPageToken: pageToken,
					Size:          historyBatch.Size,
				}, nil).Once()

				historySize += historyBatch.Size

				if counter >= tc.resetEventID {
					break
				}
			}

			s.mockDomainCache.EXPECT().GetDomainByID(targetDomainID).Return(cache.NewGlobalDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: targetDomainID, Name: targetDomainName},
				&persistence.DomainConfig{},
				&persistence.DomainReplicationConfig{
					ActiveClusterName: cluster.TestCurrentClusterName,
					Clusters: []*persistence.ClusterReplicationConfig{
						{ClusterName: cluster.TestCurrentClusterName},
						{ClusterName: cluster.TestAlternativeClusterName},
					},
				},
				1234,
			), nil).AnyTimes()

			targetBranchTokenFn := func() ([]byte, error) {
				return targetBranchToken, nil
			}

			tc.setupMock(s.mockTaskRefresher)

			rebuildMutableState, rebuiltHistorySize, err := s.nDCStateRebuilder.Rebuild(
				context.Background(),
				now,
				definition.NewWorkflowIdentifier(s.domainID, s.workflowID, s.runID),
				branchToken,
				nextEventID,
				version,
				definition.NewWorkflowIdentifier(targetDomainID, targetWorkflowID, targetRunID),
				targetBranchTokenFn,
				requestID,
				&tc.resetEventID,
			)

			if tc.err != nil {
				s.Error(err)
				s.EqualError(err, tc.err.Error())
			} else {
				s.NoError(err)
				s.NotNil(rebuildMutableState)
				rebuildExecutionInfo := rebuildMutableState.GetExecutionInfo()
				s.Equal(targetDomainID, rebuildExecutionInfo.DomainID)
				s.Equal(targetWorkflowID, rebuildExecutionInfo.WorkflowID)
				s.Equal(targetRunID, rebuildExecutionInfo.RunID)
				s.Equal(partitionConfig, rebuildExecutionInfo.PartitionConfig)
				s.Equal(int64(historySize), rebuiltHistorySize)
				s.Equal(persistence.NewVersionHistories(
					persistence.NewVersionHistory(
						targetBranchToken,
						[]*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(tc.resetEventID-1, version)},
					),
				), rebuildMutableState.GetVersionHistories())
				s.Equal(rebuildMutableState.GetExecutionInfo().StartTimestamp, now)
			}
		})
	}
}

func (s *stateRebuilderSuite) TestInvalidStateHandling() {
	requestID := uuid.New()
	version := int64(12)
	lastEventID := int64(2)
	branchToken := []byte("other random branch token")
	targetBranchToken := []byte("some other random branch token")
	now := time.Now()

	targetDomainID := uuid.New()
	targetDomainName := "other random domain name"
	targetWorkflowID := "other random workflow ID"
	targetRunID := uuid.New()

	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(targetDomainName, nil).AnyTimes()
	s.mockHistoryV2Mgr.On("ReadHistoryBranchByBatch", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	s.mockDomainCache.EXPECT().GetDomainByID(targetDomainID).Return(cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: targetDomainID, Name: targetDomainName},
		&persistence.DomainConfig{},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	), nil).AnyTimes()

	targetBranchTokenFn := func() ([]byte, error) {
		return targetBranchToken, nil
	}

	s.nDCStateRebuilder.Rebuild(
		context.Background(),
		now,
		definition.NewWorkflowIdentifier(s.domainID, s.workflowID, s.runID),
		branchToken,
		lastEventID,
		version,
		definition.NewWorkflowIdentifier(targetDomainID, targetWorkflowID, targetRunID),
		targetBranchTokenFn,
		requestID,
		nil,
	)
}
