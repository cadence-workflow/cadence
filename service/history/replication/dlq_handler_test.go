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

package replication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
	workerfixer "github.com/uber/cadence/service/worker/fixer"
)

type (
	dlqHandlerSuite struct {
		suite.Suite
		*require.Assertions
		controller *gomock.Controller

		mockShard        *shard.TestContext
		config           *config.Config
		mockClientBean   *client.MockBean
		adminClient      *admin.MockClient
		executionManager *mocks.ExecutionManager
		shardManager     *mocks.ShardManager
		taskExecutor     *fakeTaskExecutor
		taskExecutors    map[string]TaskExecutor
		sourceCluster    string

		messageHandler *dlqHandlerImpl
	}
)

func TestDLQMessageHandlerSuite(t *testing.T) {
	s := new(dlqHandlerSuite)
	suite.Run(t, s)
}

func (s *dlqHandlerSuite) SetupSuite() {

}

func (s *dlqHandlerSuite) TearDownSuite() {

}

func (s *dlqHandlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.config = config.NewForTest()

	s.controller = gomock.NewController(s.T())
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:                0,
			RangeID:                1,
			ReplicationDLQAckLevel: map[string]int64{"test": -1},
		},
		s.config,
	)

	s.mockClientBean = s.mockShard.Resource.ClientBean
	s.adminClient = s.mockShard.Resource.RemoteAdminClient
	s.executionManager = s.mockShard.Resource.ExecutionMgr
	s.shardManager = s.mockShard.Resource.ShardMgr

	s.taskExecutors = make(map[string]TaskExecutor)
	s.taskExecutor = &fakeTaskExecutor{}
	s.sourceCluster = "test"
	s.taskExecutors[s.sourceCluster] = s.taskExecutor

	s.messageHandler = NewDLQHandler(
		s.mockShard,
		s.taskExecutors,
		nil,
	).(*dlqHandlerImpl)
}

func (s *dlqHandlerSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *dlqHandlerSuite) TestNewDLQHandler_panic() {
	s.Panics(func() { NewDLQHandler(s.mockShard, nil, nil) }, "Failed to initialize replication DLQ handler due to nil task executors")
}

func (s *dlqHandlerSuite) TestStartStop() {
	tests := []struct {
		name   string
		status int32
	}{
		{
			name:   "started",
			status: common.DaemonStatusInitialized,
		},
		{
			name:   "not started",
			status: common.DaemonStatusStopped,
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			defer goleak.VerifyNone(t)

			s.messageHandler.status = tc.status

			s.messageHandler.Start()

			s.messageHandler.Stop()
		})
	}
}

func (s *dlqHandlerSuite) TestGetMessageCount() {
	size := int64(1)
	tests := []struct {
		name         string
		latestCounts map[string]int64
		forceFetch   bool
		err          error
	}{
		{
			name:         "success",
			latestCounts: map[string]int64{s.sourceCluster: size},
		},
		{
			name:       "success with fetchAndEmitMessageCount call",
			forceFetch: true,
		},
		{
			name:       "error",
			forceFetch: true,
			err:        errors.New("fetchAndEmitMessageCount error"),
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			s.messageHandler.latestCounts = tc.latestCounts

			if tc.forceFetch || tc.latestCounts == nil {
				s.executionManager.On("GetReplicationDLQSize", mock.Anything, mock.Anything).Return(&persistence.GetReplicationDLQSizeResponse{Size: size}, tc.err).Times(1)
			}

			counts, err := s.messageHandler.GetMessageCount(context.Background(), tc.forceFetch)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else if tc.latestCounts != nil {
				s.NoError(err)
				s.Equal(size, counts[s.sourceCluster])
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *dlqHandlerSuite) TestFetchAndEmitMessageCount() {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "success",
			err:  nil,
		},
		{
			name: "error",
			err:  errors.New("error"),
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			size := int64(3)
			rets := &persistence.GetReplicationDLQSizeResponse{Size: size}
			s.messageHandler.latestCounts = make(map[string]int64)

			s.executionManager.On("GetReplicationDLQSize", context.Background(), mock.Anything).Return(rets, tc.err).Times(1)

			err := s.messageHandler.fetchAndEmitMessageCount(context.Background())

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(len(s.messageHandler.latestCounts), len(s.taskExecutors))
				s.Equal(size, s.messageHandler.latestCounts[s.sourceCluster])
			}
		})
	}
}

func (s *dlqHandlerSuite) TestEmitDLQSizeMetricsLoop_FetchesAndEmitsMetricsPeriodically() {
	defer goleak.VerifyNone(s.T())

	emissionNumber := 2

	s.messageHandler.status = common.DaemonStatusStarted
	s.executionManager.On("GetReplicationDLQSize", mock.Anything, mock.Anything).Return(&persistence.GetReplicationDLQSizeResponse{Size: 1}, nil).Times(emissionNumber)
	mockTimeSource := clock.NewMockedTimeSource()
	s.messageHandler.timeSource = mockTimeSource

	s.messageHandler.wg.Add(1)
	go s.messageHandler.emitDLQSizeMetricsLoop()

	for i := 0; i < emissionNumber; i++ {
		mockTimeSource.BlockUntil(1)

		// Advance time to trigger the next emission
		mockTimeSource.Advance(dlqMetricsEmitTimerInterval + time.Duration(int64(float64(dlqMetricsEmitTimerInterval)*(1+dlqMetricsEmitTimerCoefficient))))
	}

	// Wait for the last emission to complete before stopping. The goroutine calls
	// timer.Reset only after fetchAndEmitMessageCount returns, so BlockUntil(1)
	// guarantees the final call has been made.
	mockTimeSource.BlockUntil(1)

	s.messageHandler.Stop()

	s.Equal(common.DaemonStatusStopped, s.messageHandler.status)
}

func (s *dlqHandlerSuite) TestReadMessages_OK() {
	ctx := context.Background()
	lastMessageID := int64(1)
	pageSize := 1
	var pageToken []byte

	domainID := uuid.New()
	workflowID := uuid.New()
	runID := uuid.New()
	resp := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{
					DomainID:   domainID,
					WorkflowID: workflowID,
					RunID:      runID,
					TaskID:     1,
				},
			},
		},
	}
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, &persistence.GetReplicationTasksFromDLQRequest{
		SourceClusterName: s.sourceCluster,
		ReadLevel:         0,
		MaxReadLevel:      lastMessageID + 1,
		BatchSize:         pageSize,
		NextPageToken:     pageToken,
		ShardID:           common.Ptr(0),
	}).Return(resp, nil).Times(1)

	s.mockClientBean.EXPECT().GetRemoteAdminClient(s.sourceCluster).Return(s.adminClient, nil).AnyTimes()
	s.adminClient.EXPECT().
		GetDLQReplicationMessages(ctx, gomock.Any()).
		Return(&types.GetDLQReplicationMessagesResponse{
			ReplicationTasks: []*types.ReplicationTask{{SourceTaskID: 1}},
		}, nil)
	tasks, info, token, err := s.messageHandler.ReadMessages(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)
	s.NoError(err)
	s.Nil(token)
	s.Len(info, 1)
	s.Equal(domainID, info[0].GetDomainID())
	s.Equal(workflowID, info[0].GetWorkflowID())
	s.Equal(runID, info[0].GetRunID())
	s.Len(tasks, 1)
	s.Equal(int64(1), tasks[0].SourceTaskID)
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_OK() {
	replicationTasksResponse := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{
					DomainID:     "domainID",
					WorkflowID:   "workflowID",
					RunID:        "runID",
					TaskID:       123,
					TaskType:     persistence.ReplicationTaskTypeHistory,
					Version:      1,
					FirstEventID: 1,
					NextEventID:  2,
				},
			},
		},
		NextPageToken: []byte("token"),
	}

	DLQReplicationMessagesResponse := &types.GetDLQReplicationMessagesResponse{
		ReplicationTasks: []*types.ReplicationTask{
			{
				SourceTaskID: 123,
			},
		},
	}

	ctx := context.Background()
	lastMessageID := int64(123)
	pageSize := 12
	pageToken := []byte("token")

	req := &persistence.GetReplicationTasksFromDLQRequest{
		SourceClusterName: s.sourceCluster,
		ReadLevel:         defaultBeginningMessageID + 1,
		MaxReadLevel:      lastMessageID + 1,
		BatchSize:         pageSize,
		NextPageToken:     pageToken,
		ShardID:           common.Ptr(0),
	}

	s.executionManager.On("GetReplicationTasksFromDLQ", ctx, req).Return(replicationTasksResponse, nil).Times(1)

	s.adminClient.EXPECT().
		GetDLQReplicationMessages(ctx, gomock.Any()).
		Return(DLQReplicationMessagesResponse, nil).Times(1)

	replicationTasks, taskInfo, nextPageToken, err := s.messageHandler.readMessagesWithAckLevel(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)

	s.NoError(err)
	s.Equal(replicationTasks, DLQReplicationMessagesResponse.ReplicationTasks)
	s.Len(taskInfo, len(replicationTasksResponse.Tasks))
	s.Equal("domainID", taskInfo[0].GetDomainID())
	s.Equal("workflowID", taskInfo[0].GetWorkflowID())
	s.Equal("runID", taskInfo[0].GetRunID())
	s.Equal(nextPageToken, replicationTasksResponse.NextPageToken)
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_GetReplicationTasksFromDLQFailed() {
	errorMessage := "GetReplicationTasksFromDLQFailed"
	ctx := context.Background()
	lastMessageID := int64(123)
	pageSize := 12
	pageToken := []byte("token")

	req := &persistence.GetReplicationTasksFromDLQRequest{
		SourceClusterName: s.sourceCluster,
		ReadLevel:         defaultBeginningMessageID + 1,
		MaxReadLevel:      lastMessageID + 1,
		BatchSize:         pageSize,
		NextPageToken:     pageToken,
		ShardID:           common.Ptr(0),
	}

	s.executionManager.On("GetReplicationTasksFromDLQ", ctx, req).Return(nil, errors.New(errorMessage)).Times(1)

	_, _, _, err := s.messageHandler.readMessagesWithAckLevel(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)

	s.Error(err)
	s.Equal(err, errors.New(errorMessage))
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_InvalidCluster() {
	resp := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{
					DomainID: "domainID",
					TaskID:   1,
				},
			},
		},
	}
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(resp, nil).Times(1)

	s.mockShard.Resource.ClientBean = client.NewMockBean(s.controller)
	s.mockShard.Resource.ClientBean.EXPECT().GetRemoteAdminClient("invalidCluster").Return(nil, errors.New("invalidCluster")).Times(1)

	_, _, _, err := s.messageHandler.readMessagesWithAckLevel(context.Background(), "invalidCluster", 123, 12, []byte("token"))

	s.Error(err)
	s.ErrorContains(err, "invalidCluster")
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_GetDLQReplicationMessagesFailed() {
	errorMessage := "GetDLQReplicationMessagesFailed"
	ctx := context.Background()
	lastMessageID := int64(123)
	pageSize := 12
	pageToken := []byte("token")

	req := &persistence.GetReplicationTasksFromDLQRequest{
		SourceClusterName: s.sourceCluster,
		ReadLevel:         defaultBeginningMessageID + 1,
		MaxReadLevel:      lastMessageID + 1,
		BatchSize:         pageSize,
		NextPageToken:     pageToken,
		ShardID:           common.Ptr(0),
	}

	replicationTasksResponse := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{
					DomainID: "domainID",
					TaskID:   1,
				},
			},
		},
	}

	s.executionManager.On("GetReplicationTasksFromDLQ", ctx, req).Return(replicationTasksResponse, nil).Times(1)

	s.adminClient.EXPECT().
		GetDLQReplicationMessages(ctx, gomock.Any()).
		Return(nil, errors.New(errorMessage)).Times(1)

	_, _, _, err := s.messageHandler.readMessagesWithAckLevel(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)

	s.Error(err)
	s.Equal(err, errors.New(errorMessage))
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_LocalBlob() {
	// When the manager hydrates the task, no cross-cluster call is needed.
	replicationTask := &types.ReplicationTask{
		TaskType:     types.ReplicationTaskTypeHistory.Ptr(),
		SourceTaskID: 123,
	}

	resp := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{DomainID: "domainID", TaskID: 123},
				Task: replicationTask,
			},
		},
	}
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(resp, nil).Times(1)

	// No GetRemoteAdminClient or GetDLQReplicationMessages calls expected.
	replicationTasks, taskInfo, _, err := s.messageHandler.readMessagesWithAckLevel(context.Background(), s.sourceCluster, 123, 12, nil)

	s.NoError(err)
	s.Len(replicationTasks, 1)
	s.Equal(int64(123), replicationTasks[0].SourceTaskID)
	s.Len(taskInfo, 1)
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_CorruptBlob_FallsBackToCrossCluster() {
	// If the manager could not hydrate the payload (corrupt blob), Task is nil
	// and the handler falls back to cross-cluster hydration rather than failing.
	resp := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{DomainID: "domainID", TaskID: 123},
			},
		},
	}
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(resp, nil).Times(1)

	s.adminClient.EXPECT().
		GetDLQReplicationMessages(context.Background(), gomock.Any()).
		Return(&types.GetDLQReplicationMessagesResponse{
			ReplicationTasks: []*types.ReplicationTask{{SourceTaskID: 123}},
		}, nil).Times(1)

	replicationTasks, taskInfo, _, err := s.messageHandler.readMessagesWithAckLevel(context.Background(), s.sourceCluster, 123, 12, nil)

	s.NoError(err)
	s.Len(replicationTasks, 1)
	s.Equal(int64(123), replicationTasks[0].SourceTaskID)
	s.Len(taskInfo, 1)
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_MixedLocalAndRemote() {
	// Task 1 is hydrated by the manager; task 2 has no payload and needs cross-cluster hydration.
	replicationTask1 := &types.ReplicationTask{
		TaskType:     types.ReplicationTaskTypeHistory.Ptr(),
		SourceTaskID: 1,
	}

	resp := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{DomainID: "domainID", TaskID: 1},
				Task: replicationTask1,
			},
			{
				Info: &persistence.ReplicationTaskInfo{DomainID: "domainID", TaskID: 2},
			},
		},
	}
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(resp, nil).Times(1)

	s.adminClient.EXPECT().
		GetDLQReplicationMessages(context.Background(), gomock.Any()).
		Return(&types.GetDLQReplicationMessagesResponse{
			ReplicationTasks: []*types.ReplicationTask{{SourceTaskID: 2}},
		}, nil).Times(1)

	replicationTasks, taskInfo, _, err := s.messageHandler.readMessagesWithAckLevel(context.Background(), s.sourceCluster, 10, 12, nil)
	s.NoError(err)
	s.Len(replicationTasks, 2)
	s.Equal(int64(1), replicationTasks[0].SourceTaskID)
	s.Equal(int64(2), replicationTasks[1].SourceTaskID)
	s.Len(taskInfo, 2)
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_MissingFromRemote() {
	// Task has no blob and is not returned by the remote — replicationTasks[0] is nil, taskInfo[0] is still populated.
	resp := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{DomainID: "domainID", TaskID: 1},
			},
		},
	}
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(resp, nil).Times(1)

	s.adminClient.EXPECT().
		GetDLQReplicationMessages(context.Background(), gomock.Any()).
		Return(&types.GetDLQReplicationMessagesResponse{ReplicationTasks: nil}, nil).Times(1)

	replicationTasks, taskInfo, _, err := s.messageHandler.readMessagesWithAckLevel(context.Background(), s.sourceCluster, 10, 12, nil)

	s.NoError(err)
	s.Len(replicationTasks, 1)
	s.Nil(replicationTasks[0]) // task could not be hydrated
	s.Len(taskInfo, 1)
	s.Equal("domainID", taskInfo[0].GetDomainID())
}

func (s *dlqHandlerSuite) TestPurgeMessages() {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "success",
		},
		{
			name: "error",
			err:  errors.New("error"),
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			lastMessageID := int64(1)
			s.executionManager.On("RangeDeleteReplicationTaskFromDLQ", mock.Anything,
				&persistence.RangeDeleteReplicationTaskFromDLQRequest{
					SourceClusterName:    s.sourceCluster,
					InclusiveBeginTaskID: 0,
					ExclusiveEndTaskID:   lastMessageID + 1,
					ShardID:              common.Ptr(0),
				}).Return(&persistence.RangeDeleteReplicationTaskFromDLQResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, tc.err).Times(1)

			err := s.messageHandler.PurgeMessages(context.Background(), s.sourceCluster, lastMessageID)

			if tc.err != nil {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *dlqHandlerSuite) TestMergeMessages_OK() {
	ctx := context.Background()
	lastMessageID := int64(2)
	pageSize := 1
	var pageToken []byte

	resp := &persistence.GetReplicationDLQTasksResponse{
		Tasks: []*persistence.ReplicationDLQTask{
			{
				Info: &persistence.ReplicationTaskInfo{
					DomainID:   uuid.New(),
					WorkflowID: uuid.New(),
					RunID:      uuid.New(),
					TaskID:     1,
				},
			},
			{
				Info: &persistence.ReplicationTaskInfo{
					DomainID:   uuid.New(),
					WorkflowID: uuid.New(),
					RunID:      uuid.New(),
					TaskID:     2,
				},
			},
		},
	}
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, &persistence.GetReplicationTasksFromDLQRequest{
		SourceClusterName: s.sourceCluster,
		ReadLevel:         0,
		MaxReadLevel:      lastMessageID + 1,
		BatchSize:         pageSize,
		NextPageToken:     pageToken,
		ShardID:           common.Ptr(0),
	}).Return(resp, nil).Times(1)

	s.mockClientBean.EXPECT().GetRemoteAdminClient(s.sourceCluster).Return(s.adminClient, nil).AnyTimes()
	replicationTask := &types.ReplicationTask{
		TaskType:     types.ReplicationTaskTypeHistory.Ptr(),
		SourceTaskID: 1,
	}
	s.adminClient.EXPECT().
		GetDLQReplicationMessages(ctx, gomock.Any()).
		Return(&types.GetDLQReplicationMessagesResponse{
			ReplicationTasks: []*types.ReplicationTask{replicationTask},
		}, nil)
	s.executionManager.On("RangeDeleteReplicationTaskFromDLQ", mock.Anything,
		&persistence.RangeDeleteReplicationTaskFromDLQRequest{
			SourceClusterName:    s.sourceCluster,
			InclusiveBeginTaskID: 0,
			ExclusiveEndTaskID:   lastMessageID + 1,
			ShardID:              common.Ptr(0),
		}).Return(&persistence.RangeDeleteReplicationTaskFromDLQResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, nil).Times(1)

	token, err := s.messageHandler.MergeMessages(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)
	s.NoError(err)
	s.Nil(token)
	s.Equal(1, len(s.taskExecutor.executedTasks))
}

func (s *dlqHandlerSuite) TestMergeMessages_InvalidCluster() {
	_, err := s.messageHandler.MergeMessages(context.Background(), "invalid", 1, 1, nil)
	s.Error(err)
	s.Equal(errInvalidCluster, err)
}

func (s *dlqHandlerSuite) TestMergeMessages_GetReplicationTasksFromDLQFailed() {
	errorMessage := "GetReplicationTasksFromDLQFailed"
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(nil, errors.New(errorMessage)).Times(1)
	_, err := s.messageHandler.MergeMessages(context.Background(), s.sourceCluster, 1, 1, nil)
	s.Error(err)
	s.Equal(err, errors.New(errorMessage))
}

func (s *dlqHandlerSuite) TestMergeMessages_RangeDeleteReplicationTaskFromDLQFailed() {
	errorMessage := "RangeDeleteReplicationTaskFromDLQFailed"
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(&persistence.GetReplicationDLQTasksResponse{}, nil).Times(1)
	s.executionManager.On("RangeDeleteReplicationTaskFromDLQ", mock.Anything, mock.Anything).Return(nil, errors.New(errorMessage)).Times(1)
	_, err := s.messageHandler.MergeMessages(context.Background(), s.sourceCluster, 1, 1, nil)
	s.Error(err)
	s.Equal(err, errors.New(errorMessage))
}

func (s *dlqHandlerSuite) TestMergeMessages_executeFailed() {
	errorMessage := "executeFailed"
	s.taskExecutors[s.sourceCluster] = &fakeTaskExecutor{err: errors.New(errorMessage)}

	ctx := context.Background()
	lastMessageID := int64(2)
	pageSize := 1
	var pageToken []byte

	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, &persistence.GetReplicationTasksFromDLQRequest{
		SourceClusterName: s.sourceCluster,
		ReadLevel:         0,
		MaxReadLevel:      lastMessageID + 1,
		BatchSize:         pageSize,
		NextPageToken:     pageToken,
		ShardID:           common.Ptr(0),
	}).Return(&persistence.GetReplicationDLQTasksResponse{Tasks: []*persistence.ReplicationDLQTask{{Info: &persistence.ReplicationTaskInfo{TaskID: 1}}}}, nil).Times(1)

	s.adminClient.EXPECT().GetDLQReplicationMessages(ctx, gomock.Any()).
		Return(&types.GetDLQReplicationMessagesResponse{ReplicationTasks: []*types.ReplicationTask{{SourceTaskID: 1}}}, nil)

	_, err := s.messageHandler.MergeMessages(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)

	s.Error(err)
	s.Equal(err, errors.New(errorMessage))
}

type fakeTaskExecutor struct {
	scope metrics.ScopeIdx
	err   error

	executedTasks []*types.ReplicationTask
}

func (e *fakeTaskExecutor) execute(replicationTask *types.ReplicationTask, _ bool) (metrics.ScopeIdx, error) {
	e.executedTasks = append(e.executedTasks, replicationTask)
	return e.scope, e.err
}

// fakeFixer records every Fix call so tests can assert on count and arguments.
type fakeFixer struct {
	calls []fixerCall
}

var _ workerfixer.Fixer = (*fakeFixer)(nil)

type fixerCall struct {
	domainID, workflowID, runID string
}

func (f *fakeFixer) Fix(_ context.Context, domainID, workflowID, runID string) error {
	f.calls = append(f.calls, fixerCall{domainID, workflowID, runID})
	return nil
}

func historyReplicationTask(taskID int64, domainID, workflowID, runID string) *types.ReplicationTask {
	return &types.ReplicationTask{
		SourceTaskID: taskID,
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
}

func TestProcessTask(t *testing.T) {
	const sourceCluster = "src"
	tests := []struct {
		name           string
		task           *types.ReplicationTask
		executorErr    error
		maxRetries     int
		passNilFixer   bool
		expectExecutes int
		expectDelete   bool
		expectFixCalls []fixerCall
	}{
		{
			name:           "success on first attempt — point-delete, no fixer",
			task:           historyReplicationTask(42, "d", "w", "r"),
			maxRetries:     3,
			expectExecutes: 1,
			expectDelete:   true,
		},
		{
			name:           "retries exhausted — fixer invoked, no delete",
			task:           historyReplicationTask(99, "d", "w", "r"),
			executorErr:    errors.New("boom"),
			maxRetries:     3,
			expectExecutes: 3,
			expectFixCalls: []fixerCall{{"d", "w", "r"}},
		},
		{
			name:           "retries exhausted, fixer is nil — no panic, no calls",
			task:           historyReplicationTask(99, "d", "w", "r"),
			executorErr:    errors.New("boom"),
			maxRetries:     2,
			passNilFixer:   true,
			expectExecutes: 2,
		},
		{
			name:           "nil task — skip, no execute/delete/fix",
			task:           nil,
			maxRetries:     3,
			expectExecutes: 0,
		},
		{
			name: "failover marker (no workflowID/runID) — fixer NOT invoked",
			task: &types.ReplicationTask{
				SourceTaskID:             7,
				FailoverMarkerAttributes: &types.FailoverMarkerAttributes{DomainID: "d"},
			},
			executorErr:    errors.New("boom"),
			maxRetries:     2,
			expectExecutes: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cfg := config.NewForTest()
			cfg.ReplicationDLQProcessorMaxRetries = dynamicproperties.GetIntPropertyFn(tt.maxRetries)

			mockShard := shard.NewTestContext(
				t,
				ctrl,
				&persistence.ShardInfo{
					ShardID:                0,
					RangeID:                1,
					ReplicationDLQAckLevel: map[string]int64{sourceCluster: -1},
				},
				cfg,
			)
			defer mockShard.Finish(t)

			executor := &fakeTaskExecutor{err: tt.executorErr}
			fixer := &fakeFixer{}
			var fixerArg workerfixer.Fixer = fixer
			if tt.passNilFixer {
				fixerArg = nil
			}

			h := NewDLQHandler(mockShard, map[string]TaskExecutor{sourceCluster: executor}, fixerArg).(*dlqHandlerImpl)

			if tt.expectDelete {
				mockShard.Resource.ExecutionMgr.On("DeleteReplicationTaskFromDLQ", mock.Anything,
					&persistence.DeleteReplicationTaskFromDLQRequest{
						SourceClusterName: sourceCluster,
						TaskID:            tt.task.SourceTaskID,
						ShardID:           common.Ptr(0),
					}).Return(nil).Times(1)
			}

			h.processTask(sourceCluster, tt.task)

			assert.Len(t, executor.executedTasks, tt.expectExecutes, "executor.execute call count")
			if !tt.passNilFixer {
				assert.Equal(t, tt.expectFixCalls, fixer.calls, "fixer.Fix calls")
			}
		})
	}
}

func TestScanAndProcess(t *testing.T) {
	const sourceCluster = "src"
	tests := []struct {
		name           string
		pages          []scanPage
		expectExecutes int
	}{
		{
			name: "single page, single task — processed",
			pages: []scanPage{
				{tasks: []*types.ReplicationTask{historyReplicationTask(1, "d", "w", "r")}, nextToken: nil},
			},
			expectExecutes: 1,
		},
		{
			name: "two pages — both processed",
			pages: []scanPage{
				{tasks: []*types.ReplicationTask{historyReplicationTask(1, "d", "w", "r")}, nextToken: []byte("p2")},
				{tasks: []*types.ReplicationTask{historyReplicationTask(2, "d", "w", "r")}, nextToken: nil},
			},
			expectExecutes: 2,
		},
		{
			name: "first-page error — early return, nothing processed",
			pages: []scanPage{
				{getErr: errors.New("scan failed")},
			},
			expectExecutes: 0,
		},
		{
			name: "empty page — clean return",
			pages: []scanPage{
				{tasks: nil, nextToken: nil},
			},
			expectExecutes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cfg := config.NewForTest()
			cfg.ReplicationDLQProcessorMaxRetries = dynamicproperties.GetIntPropertyFn(1)
			cfg.ReplicationDLQProcessorPageSize = dynamicproperties.GetIntPropertyFn(10)

			mockShard := shard.NewTestContext(
				t,
				ctrl,
				&persistence.ShardInfo{
					ShardID:                0,
					RangeID:                1,
					ReplicationDLQAckLevel: map[string]int64{sourceCluster: -1},
				},
				cfg,
			)
			defer mockShard.Finish(t)

			executor := &fakeTaskExecutor{}
			h := NewDLQHandler(mockShard, map[string]TaskExecutor{sourceCluster: executor}, &fakeFixer{}).(*dlqHandlerImpl)

			// Wire one expectation per page; pre-store the raw DLQ task blobs so
			// hydrateDLQTasks finds them locally (no remote admin RPC).
			for _, page := range tt.pages {
				rawTasks := make([]*persistence.ReplicationDLQTask, 0, len(page.tasks))
				for _, task := range page.tasks {
					attr := task.HistoryTaskV2Attributes
					rawTasks = append(rawTasks, &persistence.ReplicationDLQTask{
						Info: &persistence.ReplicationTaskInfo{
							DomainID:   attr.GetDomainID(),
							WorkflowID: attr.GetWorkflowID(),
							RunID:      attr.GetRunID(),
							TaskID:     task.SourceTaskID,
						},
						Task: task,
					})
				}
				resp := &persistence.GetReplicationDLQTasksResponse{
					Tasks:         rawTasks,
					NextPageToken: page.nextToken,
				}
				if page.getErr != nil {
					mockShard.Resource.ExecutionMgr.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).
						Return(nil, page.getErr).Times(1)
				} else {
					mockShard.Resource.ExecutionMgr.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).
						Return(resp, nil).Times(1)
				}
			}
			if tt.expectExecutes > 0 {
				mockShard.Resource.ExecutionMgr.On("DeleteReplicationTaskFromDLQ", mock.Anything, mock.Anything).
					Return(nil).Times(tt.expectExecutes)
			}

			h.scanAndProcess(sourceCluster)

			assert.Len(t, executor.executedTasks, tt.expectExecutes)
		})
	}
}

type scanPage struct {
	tasks     []*types.ReplicationTask
	nextToken []byte
	getErr    error
}

func TestInitialDLQProcessorDelay(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		wantZero bool
		wantMax  time.Duration
	}{
		{name: "zero interval", interval: 0, wantZero: true},
		{name: "negative interval", interval: -time.Second, wantZero: true},
		{name: "positive interval", interval: 5 * time.Minute, wantMax: 5 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initialDLQProcessorDelay(tt.interval)
			if tt.wantZero {
				assert.Equal(t, time.Duration(0), got)
				return
			}
			assert.GreaterOrEqual(t, got, time.Duration(0))
			assert.Less(t, got, tt.wantMax)
		})
	}
}

func TestStart_LaunchesAndStopsProcessorLoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := config.NewForTest()
	cfg.ReplicationDLQProcessorEnabled = dynamicproperties.GetBoolPropertyFn(true)

	mockShard := shard.NewTestContext(t, ctrl, &persistence.ShardInfo{
		ShardID:                0,
		RangeID:                1,
		ReplicationDLQAckLevel: map[string]int64{"src": -1},
	}, cfg)
	defer mockShard.Finish(t)

	h := NewDLQHandler(mockShard, map[string]TaskExecutor{"src": &fakeTaskExecutor{}}, &fakeFixer{}).(*dlqHandlerImpl)
	h.Start()

	done := make(chan struct{})
	go func() {
		h.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not complete in 2s — processorLoop did not exit cleanly")
	}
}
