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

package ndc

import (
	ctx "context"
	"errors"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

type (
	activityReplicatorSuite struct {
		suite.Suite
		*require.Assertions

		controller       *gomock.Controller
		mockShard        *shard.TestContext
		mockEngine       *engine.MockEngine
		mockDomainCache  *cache.MockDomainCache
		mockMutableState *execution.MockMutableState

		mockExecutionMgr *mocks.ExecutionManager

		logger         log.Logger
		executionCache execution.Cache

		activityReplicator ActivityReplicator
	}
)

func TestActivityReplicatorSuite(t *testing.T) {
	s := new(activityReplicatorSuite)
	suite.Run(t, s)
}

func TestSyncActivity_RetryTimerTaskGeneration(t *testing.T) {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	version := int64(100)
	scheduleID := int64(144)
	scheduledTime := time.Now()
	attempt := int32(10)

	testCases := []struct {
		name            string
		startedID       int64
		attempt         int32
		localAttempt    int32
		cancelRequested bool
		wantRetryTimer  bool
	}{
		{
			name:           "unstarted retry generates ActivityRetryTimerTask",
			startedID:      commonconstants.EmptyEventID,
			attempt:        attempt,
			localAttempt:   attempt - 1,
			wantRetryTimer: true,
		},
		{
			name:           "started activity does not generate retry timer",
			startedID:      scheduleID + 1,
			attempt:        attempt,
			localAttempt:   attempt - 1,
			wantRetryTimer: false,
		},
		{
			name:           "first attempt does not generate retry timer",
			startedID:      commonconstants.EmptyEventID,
			attempt:        0,
			localAttempt:   0,
			wantRetryTimer: false,
		},
		{
			name:            "cancel-requested activity does not generate retry timer",
			startedID:       commonconstants.EmptyEventID,
			attempt:         attempt,
			localAttempt:    attempt - 1,
			cancelRequested: true,
			wantRetryTimer:  false,
		},
		{
			name:           "redelivered sync for already-applied attempt does not generate retry timer",
			startedID:      commonconstants.EmptyEventID,
			attempt:        attempt,
			localAttempt:   attempt,
			wantRetryTimer: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mockMutableState := execution.NewMockMutableState(controller)
			mockShard := shard.NewTestContext(
				t,
				controller,
				&persistence.ShardInfo{
					ShardID:          0,
					RangeID:          1,
					TransferAckLevel: 0,
				},
				config.NewForTest(),
			)
			defer mockShard.Finish(t)
			mockDomainCache := mockShard.Resource.DomainCache
			executionCache := execution.NewCache(mockShard)
			mockEngine := engine.NewMockEngine(controller)
			mockEngine.EXPECT().NotifyNewHistoryEvent(gomock.Any()).AnyTimes()
			mockEngine.EXPECT().NotifyNewTransferTasks(gomock.Any()).AnyTimes()
			mockEngine.EXPECT().NotifyNewTimerTasks(gomock.Any()).AnyTimes()
			mockShard.SetEngine(mockEngine)
			activityReplicator := NewActivityReplicator(
				mockShard,
				executionCache,
				mockShard.GetLogger(),
			)

			runID := uuid.New()
			versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
			versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
				versionHistoryItem,
			})
			versionHistories := persistence.NewVersionHistories(versionHistory)

			key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
			workflowContext := execution.NewMockContext(controller)
			workflowContext.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(mockMutableState, nil).Times(1)
			workflowContext.EXPECT().Lock(gomock.Any()).Return(nil)
			workflowContext.EXPECT().Unlock().Times(1)
			workflowContext.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
			_, err := executionCache.PutIfNotExist(key, workflowContext)
			require.NoError(t, err)

			mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
				cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: domainID, Name: domainName},
					&persistence.DomainConfig{Retention: 1},
					&persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
					},
					version,
				), nil,
			).AnyTimes()

			request := &types.SyncActivityRequest{
				DomainID:       domainID,
				WorkflowID:     workflowID,
				RunID:          runID,
				Version:        version,
				ScheduledID:    scheduleID,
				ScheduledTime:  common.Int64Ptr(scheduledTime.UnixNano()),
				StartedID:      tc.startedID,
				Attempt:        tc.attempt,
				VersionHistory: versionHistory.ToInternalType(),
			}

			activityInfo := &persistence.ActivityInfo{
				Version:         version,
				ScheduleID:      scheduleID,
				Attempt:         tc.localAttempt,
				StartedID:       commonconstants.EmptyEventID,
				TaskList:        "some random task list",
				CancelRequested: tc.cancelRequested,
			}
			mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
			mockMutableState.EXPECT().GetNextEventID().Return(scheduleID + 10).AnyTimes()
			mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
			mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
			mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(activityInfo, true).AnyTimes()
			mockMutableState.EXPECT().GetPendingActivityInfos().Return(
				map[int64]*persistence.ActivityInfo{activityInfo.ScheduleID: activityInfo},
			).AnyTimes()
			mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
				DomainID:   domainID,
				WorkflowID: workflowID,
				RunID:      runID,
			}).AnyTimes()
			mockMutableState.EXPECT().GetCurrentVersion().Return(version).AnyTimes()
			mockMutableState.EXPECT().UpdateActivity(gomock.Any()).Return(nil).AnyTimes()
			// simulate applying the request onto the local activity info, as the
			// real ReplicateActivityInfo does for the fields the retry timer reads
			mockMutableState.EXPECT().ReplicateActivityInfo(request, gomock.Any()).DoAndReturn(
				func(req *types.SyncActivityRequest, reset bool) error {
					activityInfo.Version = req.Version
					activityInfo.Attempt = req.Attempt
					activityInfo.StartedID = req.StartedID
					activityInfo.ScheduledTime = time.Unix(0, req.GetScheduledTime())
					return nil
				},
			).Times(1)

			var addedTimerTasks []persistence.Task
			mockMutableState.EXPECT().AddTimerTasks(gomock.Any()).Do(
				func(tasks ...persistence.Task) {
					addedTimerTasks = append(addedTimerTasks, tasks...)
				},
			).AnyTimes()

			workflowContext.EXPECT().UpdateWorkflowExecutionWithNew(
				gomock.Any(),
				gomock.Any(),
				persistence.UpdateWorkflowModeUpdateCurrent,
				nil,
				nil,
				execution.TransactionPolicyPassive,
				nil,
				persistence.CreateWorkflowRequestModeReplicated,
			).Return(nil).Times(1)

			err = activityReplicator.SyncActivity(ctx.Background(), request)
			require.NoError(t, err)

			var retryTasks []*persistence.ActivityRetryTimerTask
			for _, task := range addedTimerTasks {
				if retryTask, ok := task.(*persistence.ActivityRetryTimerTask); ok {
					retryTasks = append(retryTasks, retryTask)
				}
			}
			if !tc.wantRetryTimer {
				require.Empty(t, retryTasks)
				return
			}
			require.Len(t, retryTasks, 1)
			require.Equal(t, scheduleID, retryTasks[0].EventID)
			require.Equal(t, int64(tc.attempt), retryTasks[0].Attempt)
			require.Equal(t, version, retryTasks[0].Version)
			require.True(t, retryTasks[0].VisibilityTimestamp.Equal(time.Unix(0, scheduledTime.UnixNano())))
			require.Equal(t, activityInfo.TaskList, retryTasks[0].TaskList)
		})
	}
}

func (s *activityReplicatorSuite) SetupSuite() {

}

func (s *activityReplicatorSuite) TearDownSuite() {

}

func (s *activityReplicatorSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockMutableState = execution.NewMockMutableState(s.controller)

	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          0,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest(),
	)

	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockExecutionMgr = s.mockShard.Resource.ExecutionMgr
	s.logger = s.mockShard.GetLogger()

	s.executionCache = execution.NewCache(s.mockShard)
	s.mockEngine = engine.NewMockEngine(s.controller)
	s.mockEngine.EXPECT().NotifyNewHistoryEvent(gomock.Any()).AnyTimes()
	s.mockEngine.EXPECT().NotifyNewTransferTasks(gomock.Any()).AnyTimes()
	s.mockEngine.EXPECT().NotifyNewTimerTasks(gomock.Any()).AnyTimes()
	s.mockShard.SetEngine(s.mockEngine)

	s.activityReplicator = NewActivityReplicator(
		s.mockShard,
		s.executionCache,
		s.logger,
	)
}

func (s *activityReplicatorSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *activityReplicatorSuite) TestSyncActivity_WorkflowNotFound() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	version := int64(100)

	request := &types.SyncActivityRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, &persistence.GetWorkflowExecutionRequest{
		ShardID:  common.Ptr(0),
		DomainID: domainID,
		Execution: types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		DomainName: domainName,
		RangeID:    1,
	}).Return(nil, &types.EntityNotExistsError{})
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			version,
		), nil,
	).AnyTimes()

	err := s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_WorkflowClosed() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	version := int64(100)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(false).AnyTimes()
	versionHistories := &persistence.VersionHistories{}
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateCompleted, persistence.WorkflowCloseStatusCompleted)
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			version,
		), nil,
	).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_IncomingScheduleIDLarger_IncomingVersionSmaller() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(144)
	version := int64(100)
	lastWriteVersion := version + 100
	nextEventID := scheduleID - 10
	versionHistoryItem0 := persistence.NewVersionHistoryItem(1, 1)
	versionHistoryItem1 := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem0,
		versionHistoryItem1,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)

	versionHistoryItem2 := persistence.NewVersionHistoryItem(scheduleID+1, version-1)
	versionHistory2 := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem0,
		versionHistoryItem2,
	})
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: versionHistory2.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_IncomingScheduleIDLarger_IncomingVersionLarger() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(144)
	version := int64(100)
	lastWriteVersion := version - 100
	nextEventID := scheduleID - 10

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:    domainID,
		WorkflowID:  workflowID,
		RunID:       runID,
		Version:     version,
		ScheduledID: scheduleID,
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(lastWriteVersion, nil).AnyTimes()
	var versionHistories *persistence.VersionHistories
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()

	_ = s.activityReplicator.SyncActivity(ctx.Background(), request)
}

func (s *activityReplicatorSuite) TestSyncActivity_VersionHistories_IncomingVersionSmaller_DiscardTask() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(144)
	version := int64(99)

	lastWriteVersion := version - 100
	incomingVersionHistory := persistence.VersionHistory{
		BranchToken: []byte{},
		Items: []*persistence.VersionHistoryItem{
			{
				EventID: scheduleID - 1,
				Version: version - 1,
			},
			{
				EventID: scheduleID,
				Version: version,
			},
		},
	}
	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: incomingVersionHistory.ToInternalType(),
	}
	localVersionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte{},
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: scheduleID - 1,
						Version: version - 1,
					},
					{
						EventID: scheduleID + 1,
						Version: version + 1,
					},
				},
			},
		},
	}
	s.mockMutableState.EXPECT().GetVersionHistories().Return(localVersionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateRunning, persistence.WorkflowCloseStatusNone)
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_DifferentVersionHistories_IncomingVersionLarger_ReturnRetryError() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(144)
	version := int64(100)
	lastWriteVersion := version - 100

	incomingVersionHistory := persistence.VersionHistory{
		BranchToken: []byte{},
		Items: []*persistence.VersionHistoryItem{
			{
				EventID: 50,
				Version: 2,
			},
			{
				EventID: scheduleID,
				Version: version,
			},
		},
	}
	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: incomingVersionHistory.ToInternalType(),
	}
	localVersionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte{},
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: 100,
						Version: 2,
					},
				},
			},
		},
	}
	s.mockMutableState.EXPECT().GetVersionHistories().Return(localVersionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateRunning, persistence.WorkflowCloseStatusNone)
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Equal(newNDCRetryTaskErrorWithHint(
		resendHigherVersionMessage,
		domainID,
		workflowID,
		runID,
		common.Int64Ptr(50),
		common.Int64Ptr(2),
		nil,
		nil,
	),
		err,
	)
}

func (s *activityReplicatorSuite) TestSyncActivity_VersionHistories_IncomingScheduleIDLarger_ReturnRetryError() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(99)
	version := int64(100)

	lastWriteVersion := version - 100
	incomingVersionHistory := persistence.VersionHistory{
		BranchToken: []byte{},
		Items: []*persistence.VersionHistoryItem{
			{
				EventID: 50,
				Version: 2,
			},
			{
				EventID: scheduleID,
				Version: version,
			},
			{
				EventID: scheduleID + 100,
				Version: version + 100,
			},
		},
	}
	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: incomingVersionHistory.ToInternalType(),
	}
	localVersionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte{},
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: scheduleID - 10,
						Version: version,
					},
				},
			},
		},
	}
	s.mockMutableState.EXPECT().GetVersionHistories().Return(localVersionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateRunning, persistence.WorkflowCloseStatusNone)
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Equal(newNDCRetryTaskErrorWithHint(
		resendMissingEventMessage,
		domainID,
		workflowID,
		runID,
		common.Int64Ptr(scheduleID-10),
		common.Int64Ptr(version),
		nil,
		nil,
	),
		err,
	)
}

func (s *activityReplicatorSuite) TestSyncActivity_VersionHistories_SameScheduleID() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(99)
	version := int64(100)

	lastWriteVersion := version - 100
	incomingVersionHistory := persistence.VersionHistory{
		BranchToken: []byte{},
		Items: []*persistence.VersionHistoryItem{
			{
				EventID: 50,
				Version: 2,
			},
			{
				EventID: scheduleID,
				Version: version,
			},
		},
	}
	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: incomingVersionHistory.ToInternalType(),
	}
	localVersionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte{},
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: scheduleID,
						Version: version,
					},
				},
			},
		},
	}
	s.mockMutableState.EXPECT().GetVersionHistories().Return(localVersionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(nil, false).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().
		Return(persistence.WorkflowStateCreated, persistence.WorkflowCloseStatusNone).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_VersionHistories_LocalVersionHistoryWin() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(99)
	version := int64(100)

	lastWriteVersion := version - 100
	incomingVersionHistory := persistence.VersionHistory{
		BranchToken: []byte{},
		Items: []*persistence.VersionHistoryItem{
			{
				EventID: scheduleID,
				Version: version,
			},
		},
	}
	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: incomingVersionHistory.ToInternalType(),
	}
	localVersionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte{},
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: scheduleID,
						Version: version,
					},
					{
						EventID: scheduleID + 1,
						Version: version + 1,
					},
				},
			},
		},
	}
	s.mockMutableState.EXPECT().GetVersionHistories().Return(localVersionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(nil, false).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().
		Return(persistence.WorkflowStateCreated, persistence.WorkflowCloseStatusNone).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_ActivityCompleted() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(144)
	version := int64(100)
	lastWriteVersion := version
	nextEventID := scheduleID + 10
	versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: versionHistory.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(nil, false).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_ActivityRunning_LocalActivityVersionLarger() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	scheduleID := int64(144)
	version := int64(100)
	lastWriteVersion := version + 10
	nextEventID := scheduleID + 10
	versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().AnyTimes()
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:       domainID,
		WorkflowID:     workflowID,
		RunID:          runID,
		Version:        version,
		ScheduledID:    scheduleID,
		VersionHistory: versionHistory.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			lastWriteVersion,
		), nil,
	).AnyTimes()
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(&persistence.ActivityInfo{
		Version: lastWriteVersion - 1,
	}, true).AnyTimes()

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Nil(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_ActivityRunning_Update_SameVersionSameAttempt() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	version := int64(100)
	scheduleID := int64(144)
	scheduledTime := time.Now()
	startedID := scheduleID + 1
	startedTime := scheduledTime.Add(time.Minute)
	heartBeatUpdatedTime := startedTime.Add(time.Minute)
	attempt := int32(0)
	details := []byte("some random activity heartbeat progress")
	nextEventID := scheduleID + 10
	versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().Times(1)
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:          domainID,
		WorkflowID:        workflowID,
		RunID:             runID,
		Version:           version,
		ScheduledID:       scheduleID,
		ScheduledTime:     common.Int64Ptr(scheduledTime.UnixNano()),
		StartedID:         startedID,
		StartedTime:       common.Int64Ptr(startedTime.UnixNano()),
		Attempt:           attempt,
		LastHeartbeatTime: common.Int64Ptr(heartBeatUpdatedTime.UnixNano()),
		Details:           details,
		VersionHistory:    versionHistory.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			version,
		), nil,
	).AnyTimes()
	activityInfo := &persistence.ActivityInfo{
		Version:    version,
		ScheduleID: scheduleID,
		Attempt:    attempt,
	}
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(activityInfo, true).AnyTimes()

	expectedErr := errors.New("this is error is used to by pass lots of mocking")
	s.mockMutableState.EXPECT().ReplicateActivityInfo(request, false).Return(expectedErr).Times(1)

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Equal(expectedErr, err)
}

func (s *activityReplicatorSuite) TestSyncActivity_ActivityRunning_Update_SameVersionLargerAttempt() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	version := int64(100)
	scheduleID := int64(144)
	scheduledTime := time.Now()
	startedID := scheduleID + 1
	startedTime := scheduledTime.Add(time.Minute)
	heartBeatUpdatedTime := startedTime.Add(time.Minute)
	attempt := int32(100)
	details := []byte("some random activity heartbeat progress")
	nextEventID := scheduleID + 10
	versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().Times(1)
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:          domainID,
		WorkflowID:        workflowID,
		RunID:             runID,
		Version:           version,
		ScheduledID:       scheduleID,
		ScheduledTime:     common.Int64Ptr(scheduledTime.UnixNano()),
		StartedID:         startedID,
		StartedTime:       common.Int64Ptr(startedTime.UnixNano()),
		Attempt:           attempt,
		LastHeartbeatTime: common.Int64Ptr(heartBeatUpdatedTime.UnixNano()),
		Details:           details,
		VersionHistory:    versionHistory.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			version,
		), nil,
	).AnyTimes()
	activityInfo := &persistence.ActivityInfo{
		Version:    version,
		ScheduleID: scheduleID,
		Attempt:    attempt - 1,
	}
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(activityInfo, true).AnyTimes()

	expectedErr := errors.New("this is error is used to by pass lots of mocking")
	s.mockMutableState.EXPECT().ReplicateActivityInfo(request, true).Return(expectedErr).Times(1)

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Equal(expectedErr, err)
}

func (s *activityReplicatorSuite) TestSyncActivity_ActivityRunning_Update_LargerVersion() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	version := int64(100)
	scheduleID := int64(144)
	scheduledTime := time.Now()
	startedID := scheduleID + 1
	startedTime := scheduledTime.Add(time.Minute)
	heartBeatUpdatedTime := startedTime.Add(time.Minute)
	attempt := int32(100)
	details := []byte("some random activity heartbeat progress")
	nextEventID := scheduleID + 10
	versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().Clear().Times(1)
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:          domainID,
		WorkflowID:        workflowID,
		RunID:             runID,
		Version:           version,
		ScheduledID:       scheduleID,
		ScheduledTime:     common.Int64Ptr(scheduledTime.UnixNano()),
		StartedID:         startedID,
		StartedTime:       common.Int64Ptr(startedTime.UnixNano()),
		Attempt:           attempt,
		LastHeartbeatTime: common.Int64Ptr(heartBeatUpdatedTime.UnixNano()),
		Details:           details,
		VersionHistory:    versionHistory.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			version,
		), nil,
	).AnyTimes()
	activityInfo := &persistence.ActivityInfo{
		Version:    version - 1,
		ScheduleID: scheduleID,
		Attempt:    attempt + 1,
	}
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(activityInfo, true).AnyTimes()

	expectedErr := errors.New("this is error is used to by pass lots of mocking")
	s.mockMutableState.EXPECT().ReplicateActivityInfo(request, true).Return(expectedErr).Times(1)

	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.Equal(expectedErr, err)
}

func (s *activityReplicatorSuite) TestSyncActivity_ActivityRunning() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	version := int64(100)
	scheduleID := int64(144)
	scheduledTime := time.Now()
	startedID := scheduleID + 1
	startedTime := scheduledTime.Add(time.Minute)
	heartBeatUpdatedTime := startedTime.Add(time.Minute)
	attempt := int32(100)
	details := []byte("some random activity heartbeat progress")
	nextEventID := scheduleID + 10
	versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:          domainID,
		WorkflowID:        workflowID,
		RunID:             runID,
		Version:           version,
		ScheduledID:       scheduleID,
		ScheduledTime:     common.Int64Ptr(scheduledTime.UnixNano()),
		StartedID:         startedID,
		StartedTime:       common.Int64Ptr(startedTime.UnixNano()),
		Attempt:           attempt,
		LastHeartbeatTime: common.Int64Ptr(heartBeatUpdatedTime.UnixNano()),
		Details:           details,
		VersionHistory:    versionHistory.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(1, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			version,
		), nil,
	).AnyTimes()
	activityInfo := &persistence.ActivityInfo{
		Version:    version - 1,
		ScheduleID: scheduleID,
		Attempt:    attempt + 1,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(activityInfo, true).AnyTimes()
	activityInfos := map[int64]*persistence.ActivityInfo{activityInfo.ScheduleID: activityInfo}
	s.mockMutableState.EXPECT().GetPendingActivityInfos().Return(activityInfos).AnyTimes()

	s.mockMutableState.EXPECT().ReplicateActivityInfo(request, true).Return(nil).Times(1)
	s.mockMutableState.EXPECT().UpdateActivity(activityInfo).Return(nil).Times(1)
	s.mockMutableState.EXPECT().GetCurrentVersion().Return(int64(1)).Times(1)
	s.mockMutableState.EXPECT().AddTimerTasks(gomock.Any()).Times(1)
	now := time.Unix(0, request.GetLastHeartbeatTime())
	context.EXPECT().UpdateWorkflowExecutionWithNew(
		gomock.Any(),
		now,
		persistence.UpdateWorkflowModeUpdateCurrent,
		nil,
		nil,
		execution.TransactionPolicyPassive,
		nil,
		persistence.CreateWorkflowRequestModeReplicated,
	).Return(nil).Times(1)
	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.NoError(err)
}

func (s *activityReplicatorSuite) TestSyncActivity_ActivityRunning_ZombieWorkflow() {
	domainName := "some random domain name"
	domainID := constants.TestDomainID
	workflowID := "some random workflow ID"
	runID := uuid.New()
	version := int64(100)
	scheduleID := int64(144)
	scheduledTime := time.Now()
	startedID := scheduleID + 1
	startedTime := scheduledTime.Add(time.Minute)
	heartBeatUpdatedTime := startedTime.Add(time.Minute)
	attempt := int32(100)
	details := []byte("some random activity heartbeat progress")
	nextEventID := scheduleID + 10
	versionHistoryItem := persistence.NewVersionHistoryItem(scheduleID, version)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		versionHistoryItem,
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	key := definition.NewWorkflowIdentifier(domainID, workflowID, runID)
	context := execution.NewMockContext(s.controller)
	context.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(s.mockMutableState, nil).Times(1)
	context.EXPECT().Lock(gomock.Any()).Return(nil)
	context.EXPECT().Unlock().Times(1)
	context.EXPECT().ByteSize().Return(uint64(1)).AnyTimes()
	_, err := s.executionCache.PutIfNotExist(key, context)
	s.NoError(err)
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()
	request := &types.SyncActivityRequest{
		DomainID:          domainID,
		WorkflowID:        workflowID,
		RunID:             runID,
		Version:           version,
		ScheduledID:       scheduleID,
		ScheduledTime:     common.Int64Ptr(scheduledTime.UnixNano()),
		StartedID:         startedID,
		StartedTime:       common.Int64Ptr(startedTime.UnixNano()),
		Attempt:           attempt,
		LastHeartbeatTime: common.Int64Ptr(heartBeatUpdatedTime.UnixNano()),
		Details:           details,
		VersionHistory:    versionHistory.ToInternalType(),
	}
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).AnyTimes()
	s.mockMutableState.EXPECT().GetNextEventID().Return(nextEventID).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(3, 0).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(
		cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{ID: domainID, Name: domainName},
			&persistence.DomainConfig{Retention: 1},
			&persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			version,
		), nil,
	).AnyTimes()
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	activityInfo := &persistence.ActivityInfo{
		Version:    version - 1,
		ScheduleID: scheduleID,
		Attempt:    attempt + 1,
	}
	s.mockMutableState.EXPECT().GetActivityInfo(scheduleID).Return(activityInfo, true).AnyTimes()
	activityInfos := map[int64]*persistence.ActivityInfo{activityInfo.ScheduleID: activityInfo}
	s.mockMutableState.EXPECT().GetPendingActivityInfos().Return(activityInfos).AnyTimes()

	s.mockMutableState.EXPECT().ReplicateActivityInfo(request, true).Return(nil).Times(1)
	s.mockMutableState.EXPECT().UpdateActivity(activityInfo).Return(nil).Times(1)
	s.mockMutableState.EXPECT().GetCurrentVersion().Return(int64(1)).Times(1)
	s.mockMutableState.EXPECT().AddTimerTasks(gomock.Any()).Times(1)
	now := time.Unix(0, request.GetLastHeartbeatTime())
	context.EXPECT().UpdateWorkflowExecutionWithNew(
		gomock.Any(),
		now,
		persistence.UpdateWorkflowModeBypassCurrent,
		nil,
		nil,
		execution.TransactionPolicyPassive,
		nil,
		persistence.CreateWorkflowRequestModeReplicated,
	).Return(nil).Times(1)
	err = s.activityReplicator.SyncActivity(ctx.Background(), request)
	s.NoError(err)
}
