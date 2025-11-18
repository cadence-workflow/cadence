// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package shard

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/resource"
)

const (
	testShardID                   = 123
	testRangeID                   = 1
	testTransferMaxReadLevel      = 10
	testMaxTransferSequenceNumber = 100
	testCluster                   = "test-cluster"
	testDomain                    = "test-domain"
	testDomainID                  = "test-domain-id"
	testWorkflowID                = "test-workflow-id"
)

type (
	contextTestSuite struct {
		suite.Suite
		*require.Assertions

		controller       *gomock.Controller
		mockResource     *resource.Test
		mockShardManager *mocks.ShardManager

		metricsClient metrics.Client
		logger        log.Logger

		context *contextImpl
	}
)

func TestContextSuite(t *testing.T) {
	s := new(contextTestSuite)
	suite.Run(t, s)
}

func (s *contextTestSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.History)
	s.mockShardManager = s.mockResource.ShardMgr

	s.metricsClient = metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{})
	s.logger = testlogger.New(s.T())

	s.context = s.newContext()
}

func (s *contextTestSuite) newContext() *contextImpl {
	eventsCache := events.NewMockCache(s.controller)
	config := config.NewForTest()
	shardInfo := &persistence.ShardInfo{
		ShardID: testShardID,
		RangeID: testRangeID,
		// the following fields will be initialized
		// when acquiring the shard if they are nil
		ClusterTransferAckLevel: make(map[string]int64),
		ClusterTimerAckLevel:    make(map[string]time.Time),
		ClusterReplicationLevel: make(map[string]int64),
		TransferProcessingQueueStates: &types.ProcessingQueueStates{
			StatesByCluster: make(map[string][]*types.ProcessingQueueState),
		},
		TimerProcessingQueueStates: &types.ProcessingQueueStates{
			StatesByCluster: make(map[string][]*types.ProcessingQueueState),
		},
		QueueStates: make(map[int32]*types.QueueState),
	}
	context := &contextImpl{
		Resource:                     s.mockResource,
		shardID:                      shardInfo.ShardID,
		rangeID:                      shardInfo.RangeID,
		shardInfo:                    shardInfo,
		executionManager:             s.mockResource.ExecutionMgr,
		activeClusterManager:         s.mockResource.ActiveClusterMgr,
		closeCallback:                func(i int, item *historyShardsItem) {},
		config:                       config,
		logger:                       s.logger,
		throttledLogger:              s.logger,
		taskSequenceNumber:           1,
		immediateTaskMaxReadLevel:    testTransferMaxReadLevel,
		maxTaskSequenceNumber:        testMaxTransferSequenceNumber,
		scheduledTaskMaxReadLevelMap: make(map[string]time.Time),
		remoteClusterCurrentTime:     make(map[string]time.Time),
		failoverLevels:               make(map[persistence.HistoryTaskCategory]map[string]persistence.FailoverLevel),
		eventsCache:                  eventsCache,
	}

	s.Require().True(testMaxTransferSequenceNumber < (1<<context.config.RangeSizeBits), "bad config value")

	return context
}

func (s *contextTestSuite) TearDownTest() {
	s.controller.Finish()
}

// test various setters and getters
func (s *contextTestSuite) TestAccessorMethods() {
	s.Assert().EqualValues(testShardID, s.context.GetShardID())
	s.Assert().Equal(s.mockResource, s.context.GetService())
	s.Assert().Equal(s.mockResource.ExecutionMgr, s.context.GetExecutionManager())
	s.Assert().EqualValues(testTransferMaxReadLevel, s.context.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTransfer, cluster.TestCurrentClusterName).GetTaskID())
	s.Assert().Equal(s.logger, s.context.GetLogger())
	s.Assert().Equal(s.logger, s.context.GetThrottledLogger())

	mockEngine := engine.NewMockEngine(s.controller)
	s.context.SetEngine(mockEngine)
	s.Assert().Equal(mockEngine, s.context.GetEngine())
}

func (s *contextTestSuite) TestTransferQueueState() {
	s.context.shardInfo.TransferAckLevel = 5
	queueState, err := s.context.GetQueueState(persistence.HistoryTaskCategoryTransfer)
	s.NoError(err)
	s.EqualValues(6, queueState.ExclusiveMaxReadLevel.TaskID)

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	err = s.context.UpdateQueueState(persistence.HistoryTaskCategoryTransfer, &types.QueueState{
		VirtualQueueStates: map[int64]*types.VirtualQueueState{
			0: {
				VirtualSliceStates: []*types.VirtualSliceState{
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{TaskID: 7},
							ExclusiveMax: &types.TaskKey{TaskID: 10},
						},
					},
				},
			},
		},
		ExclusiveMaxReadLevel: &types.TaskKey{TaskID: 10},
	})
	s.NoError(err)
	queueState, err = s.context.GetQueueState(persistence.HistoryTaskCategoryTransfer)
	s.NoError(err)
	s.EqualValues(10, queueState.ExclusiveMaxReadLevel.TaskID)
	s.Assert().Equal(0, s.context.shardInfo.StolenSinceRenew)

	s.EqualValues(6, s.context.GetQueueAckLevel(persistence.HistoryTaskCategoryTransfer).GetTaskID())
	s.EqualValues(6, s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTransfer, cluster.TestCurrentClusterName).GetTaskID())
	s.Equal([]*types.ProcessingQueueState{
		{
			Level:    common.Int32Ptr(0),
			AckLevel: common.Int64Ptr(6),
			MaxLevel: common.Int64Ptr(math.MaxInt64),
			DomainFilter: &types.DomainFilter{
				ReverseMatch: true,
			},
		},
	}, s.context.shardInfo.TransferProcessingQueueStates.StatesByCluster[cluster.TestCurrentClusterName])
}

func (s *contextTestSuite) TestTimerQueueState() {
	now := time.Now()
	s.context.shardInfo.TimerAckLevel = now
	queueState, err := s.context.GetQueueState(persistence.HistoryTaskCategoryTimer)
	s.NoError(err)
	s.Equal(now.UnixNano(), queueState.ExclusiveMaxReadLevel.ScheduledTimeNano)

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	err = s.context.UpdateQueueState(persistence.HistoryTaskCategoryTimer, &types.QueueState{
		VirtualQueueStates: map[int64]*types.VirtualQueueState{
			0: {
				VirtualSliceStates: []*types.VirtualSliceState{
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{ScheduledTimeNano: now.Add(time.Second).UnixNano()},
							ExclusiveMax: &types.TaskKey{ScheduledTimeNano: now.Add(time.Second * 2).UnixNano()},
						},
					},
				},
			},
		},
		ExclusiveMaxReadLevel: &types.TaskKey{ScheduledTimeNano: now.Add(time.Second * 2).UnixNano()},
	})
	s.NoError(err)
	queueState, err = s.context.GetQueueState(persistence.HistoryTaskCategoryTimer)
	s.NoError(err)
	s.Equal(now.Add(time.Second*2).UnixNano(), queueState.ExclusiveMaxReadLevel.ScheduledTimeNano)
	s.Assert().Equal(0, s.context.shardInfo.StolenSinceRenew)

	s.EqualValues(now.Add(time.Second).UnixNano(), s.context.GetQueueAckLevel(persistence.HistoryTaskCategoryTimer).GetScheduledTime().UnixNano())
	s.EqualValues(now.Add(time.Second).UnixNano(), s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTimer, cluster.TestCurrentClusterName).GetScheduledTime().UnixNano())
	s.Equal([]*types.ProcessingQueueState{
		{
			Level:    common.Int32Ptr(0),
			AckLevel: common.Int64Ptr(now.Add(time.Second).UnixNano()),
			MaxLevel: common.Int64Ptr(math.MaxInt64),
			DomainFilter: &types.DomainFilter{
				ReverseMatch: true,
			},
		},
	}, s.context.shardInfo.TimerProcessingQueueStates.StatesByCluster[cluster.TestCurrentClusterName])
}

func (s *contextTestSuite) TestTransferAckLevel() {
	// validate default value returned
	s.context.shardInfo.TransferAckLevel = 5
	s.Assert().EqualValues(5, s.context.GetQueueAckLevel(persistence.HistoryTaskCategoryTransfer).GetTaskID())

	// update and validate it's returned
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	s.context.UpdateQueueAckLevel(persistence.HistoryTaskCategoryTransfer, persistence.NewImmediateTaskKey(20))
	s.Assert().EqualValues(20, s.context.GetQueueAckLevel(persistence.HistoryTaskCategoryTransfer).GetTaskID())
	s.Assert().Equal(0, s.context.shardInfo.StolenSinceRenew)
}

func (s *contextTestSuite) TestClusterTransferAckLevel() {
	// update and validate cluster transfer ack level
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	s.context.UpdateQueueClusterAckLevel(persistence.HistoryTaskCategoryTransfer, cluster.TestCurrentClusterName, persistence.NewImmediateTaskKey(5))
	s.Assert().EqualValues(5, s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTransfer, cluster.TestCurrentClusterName).GetTaskID())
	s.Assert().Equal(0, s.context.shardInfo.StolenSinceRenew)

	// get cluster transfer ack level for non existing cluster
	s.context.shardInfo.TransferAckLevel = 10
	s.Assert().EqualValues(10, s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTransfer, "non-existing-cluster").GetTaskID())
}

func (s *contextTestSuite) TestTimerAckLevel() {
	// validate default value returned
	now := time.Now()
	s.context.shardInfo.TimerAckLevel = now
	s.Assert().Equal(now.UnixNano(), s.context.GetQueueAckLevel(persistence.HistoryTaskCategoryTimer).GetScheduledTime().UnixNano())

	// update and validate it's returned
	newTime := time.Now()
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	s.context.UpdateQueueAckLevel(persistence.HistoryTaskCategoryTimer, persistence.NewHistoryTaskKey(newTime, 0))
	s.Assert().EqualValues(newTime.UnixNano(), s.context.GetQueueAckLevel(persistence.HistoryTaskCategoryTimer).GetScheduledTime().UnixNano())
	s.Assert().Equal(0, s.context.shardInfo.StolenSinceRenew)
}

func (s *contextTestSuite) TestClusterTimerAckLevel() {
	// update and validate cluster timer ack level
	now := time.Now()
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	s.context.UpdateQueueClusterAckLevel(persistence.HistoryTaskCategoryTimer, cluster.TestCurrentClusterName, persistence.NewHistoryTaskKey(now, 0))
	s.Assert().EqualValues(now.UnixNano(), s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTimer, cluster.TestCurrentClusterName).GetScheduledTime().UnixNano())
	s.Assert().Equal(0, s.context.shardInfo.StolenSinceRenew)

	// get cluster timer ack level for non existing cluster
	s.context.shardInfo.TimerAckLevel = now
	s.Assert().EqualValues(now.UnixNano(), s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTimer, "non-existing-cluster").GetScheduledTime().UnixNano())
}

func (s *contextTestSuite) TestUpdateTransferFailoverLevel() {
	failoverLevel1 := persistence.FailoverLevel{
		StartTime:    time.Now(),
		MinLevel:     persistence.NewImmediateTaskKey(1),
		CurrentLevel: persistence.NewImmediateTaskKey(10),
		MaxLevel:     persistence.NewImmediateTaskKey(100),
		DomainIDs:    map[string]struct{}{"testDomainID": {}},
	}
	failoverLevel2 := persistence.FailoverLevel{
		StartTime:    time.Now(),
		MinLevel:     persistence.NewImmediateTaskKey(2),
		CurrentLevel: persistence.NewImmediateTaskKey(20),
		MaxLevel:     persistence.NewImmediateTaskKey(200),
		DomainIDs:    map[string]struct{}{"testDomainID2": {}},
	}

	err := s.context.UpdateFailoverLevel(persistence.HistoryTaskCategoryTransfer, "id1", failoverLevel1)
	s.NoError(err)
	err = s.context.UpdateFailoverLevel(persistence.HistoryTaskCategoryTransfer, "id2", failoverLevel2)
	s.NoError(err)

	gotLevels := s.context.GetAllFailoverLevels(persistence.HistoryTaskCategoryTransfer)
	s.Len(gotLevels, 2)
	assert.Equal(s.T(), failoverLevel1, gotLevels["id1"])
	assert.Equal(s.T(), failoverLevel2, gotLevels["id2"])

	err = s.context.DeleteFailoverLevel(persistence.HistoryTaskCategoryTransfer, "id1")
	s.NoError(err)
	gotLevels = s.context.GetAllFailoverLevels(persistence.HistoryTaskCategoryTransfer)
	s.Len(gotLevels, 1)
	assert.Equal(s.T(), failoverLevel2, gotLevels["id2"])
}

func (s *contextTestSuite) TestUpdateTimerFailoverLevel() {
	t := time.Now()
	failoverLevel1 := persistence.FailoverLevel{
		StartTime:    t,
		MinLevel:     persistence.NewHistoryTaskKey(t.Add(time.Minute), 0),
		CurrentLevel: persistence.NewHistoryTaskKey(t.Add(time.Minute*2), 0),
		MaxLevel:     persistence.NewHistoryTaskKey(t.Add(time.Minute*3), 0),
		DomainIDs:    map[string]struct{}{"testDomainID": {}},
	}
	failoverLevel2 := persistence.FailoverLevel{
		StartTime:    t,
		MinLevel:     persistence.NewHistoryTaskKey(t.Add(time.Minute*2), 0),
		CurrentLevel: persistence.NewHistoryTaskKey(t.Add(time.Minute*4), 0),
		MaxLevel:     persistence.NewHistoryTaskKey(t.Add(time.Minute*6), 0),
		DomainIDs:    map[string]struct{}{"testDomainID2": {}},
	}

	err := s.context.UpdateFailoverLevel(persistence.HistoryTaskCategoryTimer, "id1", failoverLevel1)
	s.NoError(err)
	err = s.context.UpdateFailoverLevel(persistence.HistoryTaskCategoryTimer, "id2", failoverLevel2)
	s.NoError(err)

	gotLevels := s.context.GetAllFailoverLevels(persistence.HistoryTaskCategoryTimer)
	s.Len(gotLevels, 2)
	assert.Equal(s.T(), failoverLevel1, gotLevels["id1"])
	assert.Equal(s.T(), failoverLevel2, gotLevels["id2"])

	err = s.context.DeleteFailoverLevel(persistence.HistoryTaskCategoryTimer, "id1")
	s.NoError(err)
	gotLevels = s.context.GetAllFailoverLevels(persistence.HistoryTaskCategoryTimer)
	s.Len(gotLevels, 1)
	assert.Equal(s.T(), failoverLevel2, gotLevels["id2"])
}

func (s *contextTestSuite) TestDomainNotificationVersion() {
	// test initial value
	s.EqualValues(0, s.context.GetDomainNotificationVersion())

	// test updated value
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	err := s.context.UpdateDomainNotificationVersion(10)
	s.NoError(err)
	s.EqualValues(10, s.context.GetDomainNotificationVersion())
}

func (s *contextTestSuite) TestTimerMaxReadLevel() {
	// this test requires mocked time since we're comparing timestamps
	s.mockResource.TimeSource = clock.NewMockedTimeSource()

	// get current cluster's level
	gotLevel := s.context.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTimer, cluster.TestCurrentClusterName)
	wantLevel := persistence.NewHistoryTaskKey(s.mockResource.TimeSource.Now().Add(s.context.config.TimerProcessorMaxTimeShift()).Truncate(persistence.DBTimestampMinPrecision), 0)
	s.Equal(wantLevel, gotLevel)

	// get remote cluster's level
	remoteCluster := "remote-cluster"
	now := time.Now()
	s.context.SetCurrentTime(remoteCluster, now)
	gotLevel = s.context.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTimer, remoteCluster)
	wantLevel = persistence.NewHistoryTaskKey(now.Add(s.context.config.TimerProcessorMaxTimeShift()).Truncate(persistence.DBTimestampMinPrecision), 0)
	s.Equal(wantLevel, gotLevel)
}

func (s *contextTestSuite) TestGenerateTransferTaskID() {
	taskID, err := s.context.GenerateTaskID()
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), taskID)

	taskID, err = s.context.GenerateTaskID()
	s.Require().NoError(err)
	s.Assert().Equal(int64(2), taskID)
}

func (s *contextTestSuite) TestGenerateTransferTaskIDs() {
	expectedTaskIDs := []int64{1, 2, 3, 4}

	taskIDs, err := s.context.GenerateTaskIDs(4)
	s.Require().NoError(err)
	s.Assert().Equal(expectedTaskIDs, taskIDs)
}

func (s *contextTestSuite) TestGenerateTransferTaskID_RenewsRange() {
	// we acquire task IDs until testMaxTransferSequenceNumber, then next generation should involve
	// renewing range
	_, err := s.context.GenerateTaskIDs(testMaxTransferSequenceNumber - 1)
	s.Require().NoError(err)

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)

	taskID, err := s.context.GenerateTaskID()
	s.Require().NoError(err)

	newRangeID := testRangeID + 1
	s.Assert().EqualValues(newRangeID, s.context.getRangeID(), "RangeID should be incremented when renewing range")
	expectedTransferTaskID := newRangeID << s.context.config.RangeSizeBits

	s.Assert().EqualValues(expectedTransferTaskID, taskID)
}

func (s *contextTestSuite) TestRenewRangeLockedRetriesExceeded() {
	someError := errors.New("some error")
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(someError)

	err := s.context.renewRangeLocked(false)
	s.Error(err)
}

func (s *contextTestSuite) TestUpdateClusterReplicationLevel_Succeeds() {
	lastTaskID := int64(123)

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Once().Return(nil)
	err := s.context.UpdateQueueClusterAckLevel(persistence.HistoryTaskCategoryReplication, testCluster, persistence.NewImmediateTaskKey(lastTaskID))
	s.Require().NoError(err)

	s.Equal(lastTaskID, s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryReplication, testCluster).GetTaskID())
}

func (s *contextTestSuite) TestUpdateClusterReplicationLevel_FailsWhenUpdateShardFail() {
	ownershipLostError := &persistence.ShardOwnershipLostError{ShardID: testShardID, Msg: "testing ownership lost"}
	shardClosed := false
	closeCallbackCalled := make(chan bool)

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(ownershipLostError)
	s.context.closeCallback = func(int, *historyShardsItem) {
		shardClosed = true
		closeCallbackCalled <- true
	}

	err := s.context.UpdateQueueClusterAckLevel(persistence.HistoryTaskCategoryReplication, testCluster, persistence.NewImmediateTaskKey(123))

	select {
	case <-closeCallbackCalled:
		break
	case <-time.NewTimer(time.Second).C:
		s.T().Fatal("close callback is still not called")
	}

	s.Require().ErrorContains(err, ownershipLostError.Msg)
	s.True(shardClosed, "the shard should have been closed on ShardOwnershipLostError")
}

func (s *contextTestSuite) TestReplicateFailoverMarkers() {
	cases := []struct {
		name    string
		markers []*persistence.FailoverMarkerTask
		err     error
		asserts func()
	}{
		{
			name: "Success",
			markers: []*persistence.FailoverMarkerTask{{
				TaskData: persistence.TaskData{},
				DomainID: testDomainID,
			}},
			asserts: func() {
				s.NoError(s.context.closedError())
			},
		},
		{
			name: "Shard ownership lost error",
			err:  &persistence.ShardOwnershipLostError{},
			asserts: func() {
				s.ErrorContains(s.context.closedError(), "shard closed")
			},
		},
		{
			name: "Other error",
			err:  assert.AnError,
			asserts: func() {
				s.NoError(s.context.closedError())
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// Need setup the suite manually, since we are in a subtest
			s.SetupTest()
			s.mockResource.ExecutionMgr.On("CreateFailoverMarkerTasks", mock.Anything, mock.Anything).Once().Return(tc.err)

			err := s.context.ReplicateFailoverMarkers(context.Background(), tc.markers)
			s.Equal(tc.err, err)
		})
	}
}

func (s *contextTestSuite) TestCreateWorkflowExecution() {
	cases := []struct {
		name            string
		err             error
		domainLookupErr error
		response        *persistence.CreateWorkflowExecutionResponse
		setup           func()
		asserts         func(*persistence.CreateWorkflowExecutionResponse, error)
	}{
		{
			name:     "Success",
			response: &persistence.CreateWorkflowExecutionResponse{},
			asserts: func(response *persistence.CreateWorkflowExecutionResponse, err error) {
				s.NoError(err)
				s.NotNil(response)
			},
		},
		{
			name: "No special handling",
			err:  &types.WorkflowExecutionAlreadyStartedError{},
			asserts: func(resp *persistence.CreateWorkflowExecutionResponse, err error) {
				s.Equal(err, &types.WorkflowExecutionAlreadyStartedError{})
				s.NoError(s.context.closedError())
			},
		},
		{
			name: "Shard ownership lost error",
			err:  &persistence.ShardOwnershipLostError{},
			asserts: func(resp *persistence.CreateWorkflowExecutionResponse, err error) {
				s.Equal(err, &persistence.ShardOwnershipLostError{})
				s.ErrorContains(s.context.closedError(), "shard closed")
			},
		},
		{
			name: "Other error - update shard succeed",
			err:  assert.AnError,
			setup: func() {
				s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
			},
			asserts: func(resp *persistence.CreateWorkflowExecutionResponse, err error) {
				s.Equal(assert.AnError, err)
				s.NoError(s.context.closedError())
			},
		},
		{
			name: "Other error - update shard failed",
			err:  assert.AnError,
			setup: func() {
				s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(assert.AnError)
			},
			asserts: func(resp *persistence.CreateWorkflowExecutionResponse, err error) {
				s.Equal(assert.AnError, err)
				s.ErrorContains(s.context.closedError(), "shard closed")
			},
		},
		{
			name:            "Domain lookup failed",
			domainLookupErr: assert.AnError,
			asserts: func(resp *persistence.CreateWorkflowExecutionResponse, err error) {
				s.ErrorIs(err, assert.AnError)
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// Need setup the suite manually, since we are in a subtest
			s.SetupTest()
			ctx := context.Background()
			request := &persistence.CreateWorkflowExecutionRequest{
				DomainName: testDomain,
				NewWorkflowSnapshot: persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   testDomainID,
						WorkflowID: testWorkflowID,
					},
				},
			}

			domainCacheEntry := cache.NewLocalDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: testDomainID},
				&persistence.DomainConfig{Retention: 7},
				testCluster,
			)
			s.mockResource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(domainCacheEntry, tc.domainLookupErr)
			if tc.setup != nil {
				tc.setup()
			}

			s.mockResource.ExecutionMgr.On("CreateWorkflowExecution", ctx, mock.Anything).Once().Return(tc.response, tc.err)

			resp, err := s.context.CreateWorkflowExecution(ctx, request)
			tc.asserts(resp, err)
		})
	}
}

func (s *contextTestSuite) TestUpdateWorkflowExecution() {
	cases := []struct {
		name            string
		err             error
		domainLookupErr error
		response        *persistence.UpdateWorkflowExecutionResponse
		setup           func()
		asserts         func(*persistence.UpdateWorkflowExecutionResponse, error)
	}{
		{
			name:     "Success",
			response: &persistence.UpdateWorkflowExecutionResponse{},
			asserts: func(response *persistence.UpdateWorkflowExecutionResponse, err error) {
				s.NoError(err)
				s.NotNil(response)
			},
		},
		{
			name: "No special handling",
			err:  &types.ServiceBusyError{},
			asserts: func(resp *persistence.UpdateWorkflowExecutionResponse, err error) {
				s.Equal(err, &types.ServiceBusyError{})
				s.NoError(s.context.closedError())
			},
		},
		{
			name: "Shard ownership lost error",
			err:  &persistence.ShardOwnershipLostError{},
			asserts: func(resp *persistence.UpdateWorkflowExecutionResponse, err error) {
				s.Equal(err, &persistence.ShardOwnershipLostError{})
				s.ErrorContains(s.context.closedError(), "shard closed")
			},
		},
		{
			name: "Other error - update shard succeed",
			err:  assert.AnError,
			setup: func() {
				s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
			},
			asserts: func(resp *persistence.UpdateWorkflowExecutionResponse, err error) {
				s.Equal(assert.AnError, err)
				s.NoError(s.context.closedError())
			},
		},
		{
			name: "Other error - update shard failed",
			err:  assert.AnError,
			setup: func() {
				s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(assert.AnError)
			},
			asserts: func(resp *persistence.UpdateWorkflowExecutionResponse, err error) {
				s.Equal(assert.AnError, err)
				s.ErrorContains(s.context.closedError(), "shard closed")
			},
		},
		{
			name:            "Domain lookup failed",
			domainLookupErr: assert.AnError,
			asserts: func(resp *persistence.UpdateWorkflowExecutionResponse, err error) {
				s.ErrorIs(err, assert.AnError)
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// Need setup the suite manually, since we are in a subtest
			s.SetupTest()
			ctx := context.Background()
			request := &persistence.UpdateWorkflowExecutionRequest{
				RangeID: 123,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   testDomainID,
						WorkflowID: testWorkflowID,
					},
				},
				NewWorkflowSnapshot: &persistence.WorkflowSnapshot{},
				DomainName:          testDomain,
			}

			domainCacheEntry := cache.NewLocalDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: testDomainID},
				&persistence.DomainConfig{Retention: 7},
				testCluster,
			)
			s.mockResource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(domainCacheEntry, tc.domainLookupErr)
			if tc.setup != nil {
				tc.setup()
			}

			s.mockResource.ExecutionMgr.On("UpdateWorkflowExecution", ctx, mock.Anything).Once().Return(tc.response, tc.err)

			resp, err := s.context.UpdateWorkflowExecution(ctx, request)
			tc.asserts(resp, err)
		})
	}
}

func (s *contextTestSuite) TestConflictResolveWorkflowExecution() {
	cases := []struct {
		name            string
		err             error
		domainLookupErr error
		response        *persistence.ConflictResolveWorkflowExecutionResponse
		setup           func()
		asserts         func(*persistence.ConflictResolveWorkflowExecutionResponse, error)
	}{
		{
			name:     "Success",
			response: &persistence.ConflictResolveWorkflowExecutionResponse{},
			asserts: func(response *persistence.ConflictResolveWorkflowExecutionResponse, err error) {
				s.NoError(err)
				s.NotNil(response)
			},
		},
		{
			name: "No special handling",
			err:  &types.ServiceBusyError{},
			asserts: func(resp *persistence.ConflictResolveWorkflowExecutionResponse, err error) {
				s.Equal(err, &types.ServiceBusyError{})
				s.NoError(s.context.closedError())
			},
		},
		{
			name: "Shard ownership lost error",
			err:  &persistence.ShardOwnershipLostError{},
			asserts: func(resp *persistence.ConflictResolveWorkflowExecutionResponse, err error) {
				s.Equal(err, &persistence.ShardOwnershipLostError{})
				s.ErrorContains(s.context.closedError(), "shard closed")
			},
		},
		{
			name: "Other error - update shard succeed",
			err:  assert.AnError,
			setup: func() {
				s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
			},
			asserts: func(resp *persistence.ConflictResolveWorkflowExecutionResponse, err error) {
				s.Equal(assert.AnError, err)
				s.NoError(s.context.closedError())
			},
		},
		{
			name: "Other error - update shard failed",
			err:  assert.AnError,
			setup: func() {
				s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(assert.AnError)
			},
			asserts: func(resp *persistence.ConflictResolveWorkflowExecutionResponse, err error) {
				s.Equal(assert.AnError, err)
				s.ErrorContains(s.context.closedError(), "shard closed")
			},
		},
		{
			name:            "Domain lookup failed",
			domainLookupErr: assert.AnError,
			asserts: func(resp *persistence.ConflictResolveWorkflowExecutionResponse, err error) {
				s.ErrorIs(err, assert.AnError)
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// Need setup the suite manually, since we are in a subtest
			s.SetupTest()
			ctx := context.Background()
			request := &persistence.ConflictResolveWorkflowExecutionRequest{
				ResetWorkflowSnapshot: persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   testDomainID,
						WorkflowID: testWorkflowID,
					},
				},
				NewWorkflowSnapshot:     &persistence.WorkflowSnapshot{},
				CurrentWorkflowMutation: &persistence.WorkflowMutation{},
				DomainName:              testDomain,
			}

			domainCacheEntry := cache.NewLocalDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: testDomainID},
				&persistence.DomainConfig{Retention: 7},
				testCluster,
			)
			s.mockResource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(domainCacheEntry, tc.domainLookupErr)
			if tc.setup != nil {
				tc.setup()
			}

			s.mockResource.ExecutionMgr.On("ConflictResolveWorkflowExecution", ctx, mock.Anything).Once().Return(tc.response, tc.err)

			resp, err := s.context.ConflictResolveWorkflowExecution(ctx, request)
			tc.asserts(resp, err)
		})
	}
}

func (s *contextTestSuite) TestAppendHistoryV2Events() {
	cases := []struct {
		name            string
		err             error
		domainLookupErr error
		response        *persistence.AppendHistoryNodesResponse
		setup           func()
		asserts         func(*persistence.AppendHistoryNodesResponse, error)
	}{
		{
			name:     "Success",
			response: &persistence.AppendHistoryNodesResponse{},
			asserts: func(response *persistence.AppendHistoryNodesResponse, err error) {
				s.NoError(err)
				s.NotNil(response)
			},
		},
		{
			name:            "Domain lookup failed",
			domainLookupErr: assert.AnError,
			asserts: func(resp *persistence.AppendHistoryNodesResponse, err error) {
				s.ErrorIs(err, assert.AnError)
			},
		},
		{
			name: "History too big",
			response: &persistence.AppendHistoryNodesResponse{
				DataBlob: persistence.DataBlob{
					Data: make([]byte, historySizeLogThreshold+1),
				},
			},
			asserts: func(resp *persistence.AppendHistoryNodesResponse, err error) {
				// We do not err on history too big here, we just log it
				s.NoError(err)
				s.NotNil(resp)
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// Need setup the suite manually, since we are in a subtest
			s.SetupTest()
			ctx := context.Background()
			request := &persistence.AppendHistoryNodesRequest{}
			workflowExecution := types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testWorkflowID,
			}

			s.mockResource.DomainCache.EXPECT().GetDomainName(testDomainID).Return(testDomain, tc.domainLookupErr)
			if tc.setup != nil {
				tc.setup()
			}

			s.mockResource.HistoryMgr.On("AppendHistoryNodes", ctx, mock.Anything).Once().Return(tc.response, tc.err)

			resp, err := s.context.AppendHistoryV2Events(ctx, request, testDomainID, workflowExecution)
			tc.asserts(resp, err)
		})
	}
}

func (s *contextTestSuite) TestValidateAndUpdateFailoverMarkers() {
	// This test verifies that failover markers are processed when a domain becomes active
	domainFailoverVersion := 100
	domainCacheEntryInactiveCluster := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testDomainID},
		&persistence.DomainConfig{Retention: 7},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName, // active is TestCurrentClusterName
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		int64(domainFailoverVersion),
	)
	domainCacheEntryActiveCluster := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testDomainID},
		&persistence.DomainConfig{Retention: 7},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName, // active cluster
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		int64(domainFailoverVersion),
	)
	s.mockResource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(domainCacheEntryInactiveCluster, nil)

	failoverMarker := types.FailoverMarkerAttributes{
		DomainID:        testDomainID,
		FailoverVersion: 101,
	}

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
	s.NoError(s.context.AddingPendingFailoverMarker(&failoverMarker))
	s.Require().Len(s.context.shardInfo.PendingFailoverMarkers, 1, "we should have one failover marker saved since the cluster is not active")

	s.mockResource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(domainCacheEntryActiveCluster, nil)

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)

	pendingFailoverMarkers, err := s.context.ValidateAndUpdateFailoverMarkers()
	s.NoError(err)
	s.Empty(pendingFailoverMarkers, "all pending failover tasks should be cleaned up")
}

func (s *contextTestSuite) TestValidateAndUpdateFailoverMarkers_DeprecatedDomain() {
	// This test verifies that failover markers are dropped (not processed) when a domain is deprecated
	domainFailoverVersion := 100
	domainCacheEntryInactiveCluster := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testDomainID},
		&persistence.DomainConfig{Retention: 7},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName, // active is TestCurrentClusterName
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		int64(domainFailoverVersion),
	)
	domainCacheEntryActiveCluster := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testDomainID},
		&persistence.DomainConfig{Retention: 7},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName, // active cluster
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		int64(domainFailoverVersion),
	)
	domainCacheEntryInactiveCluster.GetInfo().Status = persistence.DomainStatusDeprecated
	domainCacheEntryActiveCluster.GetInfo().Status = persistence.DomainStatusDeprecated

	s.mockResource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(domainCacheEntryInactiveCluster, nil).Times(2)

	failoverMarker := types.FailoverMarkerAttributes{
		DomainID:        testDomainID,
		FailoverVersion: int64(domainFailoverVersion),
	}

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)

	// adding failover marker
	domainFailoverVersion++
	s.NoError(s.context.AddingPendingFailoverMarker(&failoverMarker))
	s.Require().Len(s.context.shardInfo.PendingFailoverMarkers, 1, "we should have one failover marker saved since the cluster is not active")

	// adding more failover markers
	domainFailoverVersion++
	s.NoError(s.context.AddingPendingFailoverMarker(&failoverMarker))

	s.mockResource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(domainCacheEntryActiveCluster, nil).Times(2)

	pendingFailoverMarkers, err := s.context.ValidateAndUpdateFailoverMarkers()
	s.NoError(err)
	s.Empty(pendingFailoverMarkers, "pending failover markers should be dropped when the domain is deprecated")
	s.Empty(s.context.shardInfo.PendingFailoverMarkers, "pending failover markers should be dropped from shard info when domain is deprecated")
}

func (s *contextTestSuite) TestGetAndUpdateProcessingQueueStates() {
	clusterName := cluster.TestCurrentClusterName
	var initialQueueStates [][]*types.ProcessingQueueState
	initialQueueStates = append(initialQueueStates, s.context.GetTransferProcessingQueueStates(clusterName))
	initialQueueStates = append(initialQueueStates, s.context.GetTimerProcessingQueueStates(clusterName))
	for _, queueStates := range initialQueueStates {
		s.Len(queueStates, 1)
		s.Zero(queueStates[0].GetLevel())
		ackLevel := queueStates[0].GetAckLevel()
		if ackLevel != 0 {
			// for timer queue
			s.Equal(time.Time{}.UnixNano(), ackLevel)
		}
		s.Equal(int64(math.MaxInt64), queueStates[0].GetMaxLevel())
		s.Equal(&types.DomainFilter{ReverseMatch: true}, queueStates[0].GetDomainFilter())
	}

	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
	updatedTransferQueueStates := []*types.ProcessingQueueState{
		{
			Level:    common.Int32Ptr(0),
			AckLevel: common.Int64Ptr(123),
			MaxLevel: common.Int64Ptr(math.MaxInt64),
			DomainFilter: &types.DomainFilter{
				ReverseMatch: true,
			},
		},
	}
	updatedTimerQueueStates := []*types.ProcessingQueueState{
		{
			Level:    common.Int32Ptr(0),
			AckLevel: common.Int64Ptr(time.Now().UnixNano()),
			MaxLevel: common.Int64Ptr(math.MaxInt64),
			DomainFilter: &types.DomainFilter{
				ReverseMatch: true,
			},
		},
	}
	err := s.context.UpdateTransferProcessingQueueStates(clusterName, updatedTransferQueueStates)
	s.NoError(err)
	err = s.context.UpdateTimerProcessingQueueStates(clusterName, updatedTimerQueueStates)
	s.NoError(err)

	s.Equal(updatedTransferQueueStates, s.context.GetTransferProcessingQueueStates(clusterName))
	s.Equal(updatedTimerQueueStates, s.context.GetTimerProcessingQueueStates(clusterName))

	// check if cluster ack level for transfer and timer is backfilled for backward compatibility
	s.Equal(updatedTransferQueueStates[0].GetAckLevel(), s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTransfer, clusterName).GetTaskID())
	s.Equal(time.Unix(0, updatedTimerQueueStates[0].GetAckLevel()), s.context.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryTimer, clusterName).GetScheduledTime())
}

func TestGetWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name           string
		request        *persistence.GetWorkflowExecutionRequest
		mockSetup      func(*mocks.ExecutionManager)
		expectedResult *persistence.GetWorkflowExecutionResponse
		expectedError  error
	}{
		{
			name: "Success",
			request: &persistence.GetWorkflowExecutionRequest{
				DomainID:  "testDomain",
				Execution: types.WorkflowExecution{WorkflowID: "testWorkflowID", RunID: "testRunID"},
			},
			mockSetup: func(mgr *mocks.ExecutionManager) {
				mgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "testDomain",
							WorkflowID: "testWorkflowID",
							RunID:      "testRunID",
						},
					},
				}, nil)
			},
			expectedResult: &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "testDomain",
						WorkflowID: "testWorkflowID",
						RunID:      "testRunID",
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "Error",
			request: &persistence.GetWorkflowExecutionRequest{
				DomainID:  "testDomain",
				Execution: types.WorkflowExecution{WorkflowID: "testWorkflowID", RunID: "testRunID"},
			},
			mockSetup: func(mgr *mocks.ExecutionManager) {
				mgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, errors.New("some random error"))
			},
			expectedResult: nil,
			expectedError:  errors.New("some random error"),
		},
	}

	for _, tc := range testCases {
		mockExecutionMgr := &mocks.ExecutionManager{}
		shardContext := &contextImpl{
			executionManager: mockExecutionMgr,
			shardInfo: &persistence.ShardInfo{
				RangeID: 12,
			},
		}
		tc.mockSetup(mockExecutionMgr)

		result, err := shardContext.GetWorkflowExecution(context.Background(), tc.request)
		assert.Equal(t, tc.expectedResult, result)
		assert.Equal(t, tc.expectedError, err)
	}
}

func TestCloseShard(t *testing.T) {
	closeCallback := make(chan struct{})

	shardContext := &contextImpl{
		shardInfo: &persistence.ShardInfo{RangeID: 12},
		closeCallback: func(i int, item *historyShardsItem) {
			close(closeCallback)
		},
		logger: log.NewNoop(),
	}
	shardContext.closeShard()

	select {
	case <-closeCallback:
	case <-time.After(time.Second):
		assert.Fail(t, "closeCallback not called")
	}

	assert.WithinDuration(t, time.Now(), *shardContext.closedAt.Load(), time.Second)
	assert.Equal(t, int64(-1), shardContext.shardInfo.RangeID)
}

func TestCloseShard_AlreadyClosed(t *testing.T) {
	closeTime := time.Unix(123, 456)

	shardContext := &contextImpl{
		closeCallback: func(i int, item *historyShardsItem) {
			assert.Fail(t, "closeCallback should not be called")
		},
	}
	shardContext.closedAt.Store(&closeTime)
	shardContext.closeShard()
	assert.Equal(t, closeTime, *shardContext.closedAt.Load())
}

func TestShardClosedGuard(t *testing.T) {
	shardContext := &contextImpl{
		shardInfo: &persistence.ShardInfo{},
	}

	testCases := []struct {
		name string
		call func() error
	}{
		{
			name: "GetWorkflowExecution",
			call: func() error {
				_, err := shardContext.GetWorkflowExecution(
					context.Background(),
					&persistence.GetWorkflowExecutionRequest{RangeID: 0},
				)
				return err
			},
		},
		{
			name: "CreateWorkflowExecution",
			call: func() error {
				_, err := shardContext.CreateWorkflowExecution(context.Background(), nil)
				return err
			},
		},
		{
			name: "UpdateWorkflowExecution",
			call: func() error {
				_, err := shardContext.UpdateWorkflowExecution(context.Background(), nil)
				return err
			},
		},
		{
			name: "ConflictResolveWorkflowExecution",
			call: func() error {
				_, err := shardContext.ConflictResolveWorkflowExecution(context.Background(), nil)
				return err
			},
		},
		{
			name: "AppendHistoryV2Events",
			call: func() error {
				_, err := shardContext.AppendHistoryV2Events(context.Background(), nil, "", types.WorkflowExecution{})
				return err
			},
		},
		{
			name: "renewRangeLocked",
			call: func() error {
				return shardContext.renewRangeLocked(false)
			},
		},
		{
			name: "persistShardInfoLocked",
			call: func() error {
				return shardContext.persistShardInfoLocked(false)
			},
		},
		{
			name: "ReplicateFailoverMarkers",
			call: func() error {
				return shardContext.ReplicateFailoverMarkers(context.Background(), nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			closedAt := time.Unix(123, 456)

			shardContext.closedAt.Store(&closedAt)
			err := tc.call()
			var shardClosedErr *ErrShardClosed
			assert.ErrorAs(t, err, &shardClosedErr)
			assert.Equal(t, closedAt, shardClosedErr.ClosedAt)
			assert.ErrorContains(t, err, "shard closed")
		})
	}
}

// setupAllocateTimerIDsTest creates common test setup for allocateTimerIDsLocked tests
func (s *contextTestSuite) setupAllocateTimerIDsTest() *cache.DomainCacheEntry {
	// Create basic domain cache entry with sensible defaults
	domainInfo := &persistence.DomainInfo{ID: testDomainID}
	domainConfig := &persistence.DomainConfig{}
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: testCluster,
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: testCluster},
		},
	}

	return cache.NewDomainCacheEntryForTest(
		domainInfo,
		domainConfig,
		false, // domainIsActiveActive - default to false
		replicationConfig,
		456, // failover version
		nil, // failover end time
		123, // failover notification version
		0,   // previous failover version
		1,   // notification version
	)
}

type createMockTimerTaskParams struct {
	Version    int64
	Timestamp  time.Time
	DomainID   string
	WorkflowID string
	RunID      string
}

func (s *contextTestSuite) createMockTimerTask(params createMockTimerTaskParams) *persistence.MockTask {
	mockTask := persistence.NewMockTask(s.controller)

	// Use variables to track changes made by the allocateTimerIDsLocked function
	var taskID int64
	var visibilityTimestamp = params.Timestamp

	mockTask.EXPECT().GetDomainID().Return(params.DomainID).AnyTimes()
	mockTask.EXPECT().GetWorkflowID().Return(params.WorkflowID).AnyTimes()
	mockTask.EXPECT().GetRunID().Return(params.RunID).AnyTimes()
	mockTask.EXPECT().GetVersion().Return(params.Version).AnyTimes()
	mockTask.EXPECT().GetTaskCategory().Return(persistence.HistoryTaskCategoryTimer).AnyTimes()
	mockTask.EXPECT().GetTaskType().Return(123).AnyTimes()
	mockTask.EXPECT().ByteSize().Return(uint64(100)).AnyTimes()

	// Mock GetTaskID to return the current task ID
	mockTask.EXPECT().GetTaskID().DoAndReturn(func() int64 {
		return taskID
	}).AnyTimes()

	// Mock SetTaskID to update the task ID variable
	mockTask.EXPECT().SetTaskID(gomock.Any()).DoAndReturn(func(id int64) {
		taskID = id
	}).AnyTimes()

	// Mock GetVisibilityTimestamp to return the current timestamp
	mockTask.EXPECT().GetVisibilityTimestamp().DoAndReturn(func() time.Time {
		return visibilityTimestamp
	}).AnyTimes()

	// Mock SetVisibilityTimestamp to update the timestamp variable
	mockTask.EXPECT().SetVisibilityTimestamp(gomock.Any()).DoAndReturn(func(ts time.Time) {
		visibilityTimestamp = ts
	}).AnyTimes()

	// Mock GetTaskKey to return the current key based on current values
	mockTask.EXPECT().GetTaskKey().DoAndReturn(func() persistence.HistoryTaskKey {
		return persistence.NewHistoryTaskKey(visibilityTimestamp, taskID)
	}).AnyTimes()

	return mockTask
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenNoTasksProvidedReturnsSuccessfully() {
	domainCacheEntry := s.setupAllocateTimerIDsTest()

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{})

	s.NoError(err)
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenTaskHasEmptyVersionAllocatesTaskID() {
	domainCacheEntry := s.setupAllocateTimerIDsTest()

	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    constants.EmptyVersion,
		Timestamp:  time.Now().Add(time.Hour),
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})
	originalTaskID := task.GetTaskID()

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.NoError(err)
	s.NotEqual(originalTaskID, task.GetTaskID(), "Task ID should have been updated")
	s.True(task.GetTaskID() > 0, "Task ID should be positive")
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenTaskHasVersionAllocatesNewTaskID() {
	domainCacheEntry := s.setupAllocateTimerIDsTest()

	// Enable timer queue v2
	s.context.config.EnableTimerQueueV2 = func(int) bool {
		return true
	}

	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    constants.EmptyVersion,
		Timestamp:  time.Now().Add(time.Hour),
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})
	originalTaskID := task.GetTaskID()

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.NoError(err)
	s.NotEqual(originalTaskID, task.GetTaskID(), "Task ID should have been updated")
	s.True(task.GetTaskID() > 0, "Task ID should be positive")
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenTimerQueueV2DisabledUsesReplicationConfigClusterName() {
	domainInfo := &persistence.DomainInfo{ID: testDomainID}
	domainConfig := &persistence.DomainConfig{}
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: "active-cluster",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: testCluster},
		},
	}
	domainCacheEntry := cache.NewDomainCacheEntryForTest(
		domainInfo, domainConfig, false, replicationConfig, 456, nil, 123, 0, 1,
	)

	// Disable timer queue v2
	s.context.config.EnableTimerQueueV2 = func(int) bool {
		return false
	}

	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    constants.EmptyVersion,
		Timestamp:  time.Now().Add(time.Hour),
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})
	originalTaskID := task.GetTaskID()

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.NoError(err)
	s.NotEqual(originalTaskID, task.GetTaskID(), "Task ID should have been updated")
	s.True(task.GetTaskID() > 0, "Task ID should be positive")
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenDomainIsActiveActiveUsesClusterManagerLookup() {

	// Create active-active domain cache entry
	domainInfo := &persistence.DomainInfo{ID: testDomainID}
	domainConfig := &persistence.DomainConfig{}
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: "active-cluster",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: testCluster},
		},
		ActiveClusters: &types.ActiveClusters{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				"region": {
					ClusterAttributes: map[string]types.ActiveClusterInfo{
						"region1": {
							ActiveClusterName: "active-cluster",
							FailoverVersion:   456,
						},
					},
				},
			},
		},
	}
	domainCacheEntry := cache.NewDomainCacheEntryForTest(
		domainInfo, domainConfig, true, replicationConfig, 456, nil, 123, 0, 1,
	)

	// Disable timer queue v2
	s.context.config.EnableTimerQueueV2 = func(int) bool {
		return false
	}

	// Setup active cluster manager mock
	s.mockResource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(
		gomock.Any(), testDomainID, testWorkflowID, gomock.Any(),
	).Return(&types.ActiveClusterInfo{
		ActiveClusterName: "looked-up-cluster",
	}, nil).Times(1)

	// Create task with non-empty version to trigger the lookup logic
	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    123,
		Timestamp:  time.Now().Add(time.Hour),
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})
	originalTaskID := task.GetTaskID()

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.NoError(err)
	s.NotEqual(originalTaskID, task.GetTaskID(), "Task ID should have been updated")
	s.True(task.GetTaskID() > 0, "Task ID should be positive")
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenTaskTimestampBeforeReadCursorAdjustsTimestamp() {
	s.mockResource.TimeSource = clock.NewMockedTimeSourceAt(time.Now())
	testTimeNow := s.mockResource.TimeSource.Now()
	domainCacheEntry := s.setupAllocateTimerIDsTest()

	// Set up scheduled task max read level map with read cursor ahead of task timestamp
	// Use the actual current cluster name from cluster metadata
	currentCluster := s.context.GetClusterMetadata().GetCurrentClusterName()
	readCursor := testTimeNow.Add(time.Second)
	s.context.scheduledTaskMaxReadLevelMap[currentCluster] = readCursor

	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    constants.EmptyVersion,
		Timestamp:  readCursor.Add(-time.Second), // before read cursor
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.NoError(err)

	// Verify timestamp was adjusted to be after read cursor
	s.True(task.GetVisibilityTimestamp().After(readCursor),
		"Task timestamp should be adjusted to be after read cursor")

	// Verify it's the expected adjusted time (readCursor + DBTimestampMinPrecision)
	expectedTime := readCursor.Add(persistence.DBTimestampMinPrecision)
	actualTime := task.GetVisibilityTimestamp()
	s.Equal(expectedTime.Truncate(persistence.DBTimestampMinPrecision),
		actualTime.Truncate(persistence.DBTimestampMinPrecision),
		"Adjusted timestamp should match expected adjusted time")
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenTaskTimestampBeforeNow() {
	s.mockResource.TimeSource = clock.NewMockedTimeSourceAt(time.Now())
	testTimeNow := s.mockResource.TimeSource.Now()
	domainCacheEntry := s.setupAllocateTimerIDsTest()

	// Set up scheduled task max read level map with read cursor ahead of task timestamp
	// Use the actual current cluster name from cluster metadata
	currentCluster := s.context.GetClusterMetadata().GetCurrentClusterName()
	readCursor := testTimeNow.Add(-2 * time.Second) // read cursor is in the past
	s.context.scheduledTaskMaxReadLevelMap[currentCluster] = readCursor

	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    constants.EmptyVersion,
		Timestamp:  readCursor.Add(-time.Second), // before now but after read cursor
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.NoError(err)

	// Verify timestamp was adjusted to be after read cursor
	s.True(task.GetVisibilityTimestamp().After(readCursor),
		"Task timestamp should be adjusted to be after read cursor")

	// Verify it's the expected adjusted time (readCursor + DBTimestampMinPrecision)
	expectedTime := testTimeNow.Add(persistence.DBTimestampMinPrecision)
	actualTime := task.GetVisibilityTimestamp()
	s.Equal(expectedTime.Truncate(persistence.DBTimestampMinPrecision),
		actualTime.Truncate(persistence.DBTimestampMinPrecision),
		"Adjusted timestamp should match expected adjusted time")
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenClusterManagerLookupFailsReturnsError() {
	// Create active-active domain cache entry
	domainInfo := &persistence.DomainInfo{ID: testDomainID}
	domainConfig := &persistence.DomainConfig{}
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: "active-cluster",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: testCluster},
		},
		ActiveClusters: &types.ActiveClusters{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				"region": {
					ClusterAttributes: map[string]types.ActiveClusterInfo{
						"region1": {
							ActiveClusterName: "active-cluster",
							FailoverVersion:   456,
						},
					},
				},
			},
		},
	}
	domainCacheEntry := cache.NewDomainCacheEntryForTest(
		domainInfo, domainConfig, true, replicationConfig, 456, nil, 123, 0, 1,
	)

	// Disable timer queue v2
	s.context.config.EnableTimerQueueV2 = func(int) bool {
		return false
	}

	// Setup active cluster manager mock to return error
	s.mockResource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(
		gomock.Any(), testDomainID, testWorkflowID, gomock.Any(),
	).Return(nil, assert.AnError).Times(1)

	// Create task with non-empty version to trigger the lookup logic
	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    123,
		Timestamp:  time.Now().Add(time.Hour),
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.Error(err)
	s.Equal(assert.AnError, err)
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenTaskIDGenerationFailsReturnsError() {
	domainCacheEntry := s.setupAllocateTimerIDsTest()

	// Force task sequence number to exceed max to trigger error
	originalTaskSequenceNumber := s.context.taskSequenceNumber
	s.context.taskSequenceNumber = s.context.maxTaskSequenceNumber + 1
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(assert.AnError)
	defer func() {
		s.context.taskSequenceNumber = originalTaskSequenceNumber
	}()

	task := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    constants.EmptyVersion,
		Timestamp:  time.Now().Add(time.Hour),
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id",
	})

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task})

	s.Error(err)
	s.Equal(assert.AnError, err)
}

func (s *contextTestSuite) TestAllocateTimerIDsLocked_WhenMultipleTasksProvidedAllocatesAllTaskIDs() {

	// Create domain cache entry for non-active-active domain
	domainInfo := &persistence.DomainInfo{ID: testDomainID}
	domainConfig := &persistence.DomainConfig{}
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: "active-cluster",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: testCluster},
		},
	}
	domainCacheEntry := cache.NewDomainCacheEntryForTest(
		domainInfo, domainConfig, false, replicationConfig, 456, nil, 123, 0, 1,
	)

	// Disable timer queue v2
	s.context.config.EnableTimerQueueV2 = func(int) bool {
		return false
	}

	task1 := s.createMockTimerTask(createMockTimerTaskParams{
		Version:    constants.EmptyVersion,
		Timestamp:  time.Now().Add(time.Hour),
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id-1",
	})
	task2 := s.createMockTimerTask(createMockTimerTaskParams{
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      "test-run-id-2",
		Version:    456,
		Timestamp:  time.Now().Add(2 * time.Hour),
	})

	originalTaskID1 := task1.GetTaskID()
	originalTaskID2 := task2.GetTaskID()

	err := s.context.allocateTimerIDsLocked(domainCacheEntry, testWorkflowID, []persistence.Task{task1, task2})

	s.NoError(err)
	s.NotEqual(originalTaskID1, task1.GetTaskID(), "Task 1 ID should have been updated")
	s.NotEqual(originalTaskID2, task2.GetTaskID(), "Task 2 ID should have been updated")
	s.True(task1.GetTaskID() > 0, "Task 1 ID should be positive")
	s.True(task2.GetTaskID() > 0, "Task 2 ID should be positive")
}
