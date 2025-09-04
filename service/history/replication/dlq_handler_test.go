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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
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
	).(*dlqHandlerImpl)
}

func (s *dlqHandlerSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *dlqHandlerSuite) TestNewDLQHandler_panic() {
	s.Panics(func() { NewDLQHandler(s.mockShard, nil) }, "Failed to initialize replication DLQ handler due to nil task executors")
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

	go s.messageHandler.emitDLQSizeMetricsLoop()

	for i := 0; i < emissionNumber; i++ {
		mockTimeSource.BlockUntil(1)

		// Advance time to trigger the next emission
		mockTimeSource.Advance(dlqMetricsEmitTimerInterval + time.Duration(int64(float64(dlqMetricsEmitTimerInterval)*(1+dlqMetricsEmitTimerCoefficient))))
	}

	s.messageHandler.Stop()

	s.Equal(common.DaemonStatusStopped, s.messageHandler.status)
}

func (s *dlqHandlerSuite) TestReadMessages_OK() {
	ctx := context.Background()
	lastMessageID := int64(1)
	pageSize := 1
	var pageToken []byte

	resp := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.HistoryReplicationTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   uuid.New(),
					WorkflowID: uuid.New(),
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					TaskID: 1,
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
	}).Return(resp, nil).Times(1)

	s.mockClientBean.EXPECT().GetRemoteAdminClient(s.sourceCluster).Return(s.adminClient, nil).AnyTimes()
	s.adminClient.EXPECT().
		GetDLQReplicationMessages(ctx, gomock.Any()).
		Return(&types.GetDLQReplicationMessagesResponse{}, nil)
	tasks, info, token, err := s.messageHandler.ReadMessages(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)
	s.NoError(err)
	s.Nil(token)
	s.Equal(resp.Tasks[0].GetDomainID(), info[0].GetDomainID())
	s.Equal(resp.Tasks[0].GetWorkflowID(), info[0].GetWorkflowID())
	s.Equal(resp.Tasks[0].GetRunID(), info[0].GetRunID())
	s.Nil(tasks)
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_OK() {
	replicationTasksResponse := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.HistoryReplicationTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   "domainID",
					WorkflowID: "workflowID",
					RunID:      "runID",
				},
				TaskData: persistence.TaskData{
					TaskID:  123,
					Version: 1,
				},
				FirstEventID: 1,
				NextEventID:  2,
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
	}

	s.executionManager.On("GetReplicationTasksFromDLQ", ctx, req).Return(replicationTasksResponse, nil).Times(1)

	s.adminClient.EXPECT().
		GetDLQReplicationMessages(ctx, gomock.Any()).
		Return(DLQReplicationMessagesResponse, nil).Times(1)

	replicationTasks, taskInfo, nextPageToken, err := s.messageHandler.readMessagesWithAckLevel(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)

	s.NoError(err)
	s.Equal(replicationTasks, DLQReplicationMessagesResponse.ReplicationTasks)
	s.Len(taskInfo, len(replicationTasksResponse.Tasks))
	// testing content of taskInfo because it's assembled in the method using tasks from replicationTasksFromDLQ
	for i, task := range taskInfo {
		t, err := replicationTasksResponse.Tasks[i].ToInternalReplicationTaskInfo()
		s.NoError(err)
		s.Equal(task, t)
	}
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
	}

	s.executionManager.On("GetReplicationTasksFromDLQ", ctx, req).Return(nil, errors.New(errorMessage)).Times(1)

	_, _, _, err := s.messageHandler.readMessagesWithAckLevel(ctx, s.sourceCluster, lastMessageID, pageSize, pageToken)

	s.Error(err)
	s.Equal(err, errors.New(errorMessage))
}

func (s *dlqHandlerSuite) TestReadMessagesWithAckLevel_InvalidCluster() {
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(nil, nil).Times(1)

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
	}

	replicationTasksResponse := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.FailoverMarkerTask{
				DomainID: "domainID",
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

	resp := &persistence.GetHistoryTasksResponse{
		Tasks: []persistence.Task{
			&persistence.HistoryReplicationTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   uuid.New(),
					WorkflowID: uuid.New(),
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					TaskID: 1,
				},
			},
			&persistence.HistoryReplicationTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   uuid.New(),
					WorkflowID: uuid.New(),
					RunID:      uuid.New(),
				},
				TaskData: persistence.TaskData{
					TaskID: 2,
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
	s.executionManager.On("GetReplicationTasksFromDLQ", mock.Anything, mock.Anything).Return(&persistence.GetHistoryTasksResponse{}, nil).Times(1)
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
	}).Return(&persistence.GetHistoryTasksResponse{Tasks: []persistence.Task{&persistence.HistoryReplicationTask{TaskData: persistence.TaskData{TaskID: 1}}}}, nil).Times(1)

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
