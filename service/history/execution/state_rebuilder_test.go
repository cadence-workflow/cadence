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
	events := []*types.HistoryEvent{
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
		events,
		[]*types.HistoryEvent(nil),
	).Return(nil, nil).Times(1)

	err := s.nDCStateRebuilder.applyEvents(workflowIdentifier, mockStateBuilder, events, requestID)
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
	)
}
