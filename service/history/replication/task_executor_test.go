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
	"fmt"
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	historyClient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/shard"
)

type (
	taskExecutorSuite struct {
		suite.Suite
		*require.Assertions
		controller *gomock.Controller

		sourceCluster      string
		mockShard          *shard.TestContext
		mockEngine         *engine.MockEngine
		historyClient      *historyClient.MockClient
		config             *config.Config
		mockDomainCache    *cache.MockDomainCache
		mockClientBean     *client.MockBean
		adminClient        *admin.MockClient
		executionManager   *mocks.ExecutionManager
		nDCHistoryResender *ndc.MockHistoryResender

		taskHandler *taskExecutorImpl
	}
)

func TestTaskExecutorSuite(t *testing.T) {
	s := new(taskExecutorSuite)
	suite.Run(t, s)
}

func (s *taskExecutorSuite) SetupSuite() {

}

func (s *taskExecutorSuite) TearDownSuite() {

}

func (s *taskExecutorSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.config = config.NewForTestByShardNumber(2)
	s.controller = gomock.NewController(s.T())
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:                0,
			RangeID:                1,
			ReplicationAckLevel:    0,
			ReplicationDLQAckLevel: map[string]int64{"test": -1},
		},
		s.config,
	)

	s.sourceCluster = "source-cluster"
	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockClientBean = s.mockShard.Resource.ClientBean
	s.adminClient = s.mockShard.Resource.RemoteAdminClient
	s.executionManager = s.mockShard.Resource.ExecutionMgr
	s.nDCHistoryResender = ndc.NewMockHistoryResender(s.controller)
	s.mockEngine = engine.NewMockEngine(s.controller)
	s.historyClient = s.mockShard.Resource.HistoryClient

	s.taskHandler = NewTaskExecutor(
		s.sourceCluster,
		s.mockShard,
		s.mockDomainCache,
		s.nDCHistoryResender,
		s.mockEngine,
		metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{}),
		s.mockShard.GetLogger(),
	).(*taskExecutorImpl)
}

func (s *taskExecutorSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *taskExecutorSuite) TestConvertRetryTaskV2Error_OK() {
	err := &types.RetryTaskV2Error{}
	_, ok := toRetryTaskV2Error(err)
	s.True(ok)
}

func (s *taskExecutorSuite) TestConvertRetryTaskV2Error_NotOK() {
	err := &types.BadRequestError{}
	_, ok := toRetryTaskV2Error(err)
	s.False(ok)
}

func (s *taskExecutorSuite) TestFilterTask() {
	domainID := uuid.New()
	s.mockDomainCache.EXPECT().
		GetDomainByID(domainID).
		Return(cache.NewGlobalDomainCacheEntryForTest(
			nil,
			nil,
			&persistence.DomainReplicationConfig{
				Clusters: []*persistence.ClusterReplicationConfig{
					{
						ClusterName: "active",
					},
				}},
			0,
		), nil)
	ok, err := s.taskHandler.filterTask(domainID, false)
	s.NoError(err)
	s.True(ok)
}

func (s *taskExecutorSuite) TestFilterTask_Error() {
	domainID := uuid.New()
	s.mockDomainCache.EXPECT().
		GetDomainByID(domainID).
		Return(nil, fmt.Errorf("test"))
	ok, err := s.taskHandler.filterTask(domainID, false)
	s.Error(err)
	s.False(ok)
}

func (s *taskExecutorSuite) TestFilterTask_EnforceApply() {
	domainID := uuid.New()
	ok, err := s.taskHandler.filterTask(domainID, true)
	s.NoError(err)
	s.True(ok)
}

func (s *taskExecutorSuite) TestProcessTask_SyncActivityReplicationTask_SameShardID() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeSyncActivity.Ptr(),
		SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.SyncActivityRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	s.mockEngine.EXPECT().SyncActivity(gomock.Any(), request).Return(nil).Times(1)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcessTask_SyncActivityReplicationTask_SameShardID_RetryTaskV2Error_Success() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeSyncActivity.Ptr(),
		SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.SyncActivityRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	s.mockEngine.EXPECT().SyncActivity(gomock.Any(), request).Return(&types.RetryTaskV2Error{
		DomainID:          "test-domain-id",
		WorkflowID:        "test-wf-id",
		RunID:             "test-run-id",
		StartEventID:      common.Ptr(int64(11)),
		StartEventVersion: common.Ptr(int64(100)),
		EndEventID:        common.Ptr(int64(19)),
		EndEventVersion:   common.Ptr(int64(102)),
	}).Times(1)
	s.nDCHistoryResender.EXPECT().SendSingleWorkflowHistory(
		s.sourceCluster,
		"test-domain-id",
		"test-wf-id",
		"test-run-id",
		common.Ptr(int64(11)),
		common.Ptr(int64(100)),
		common.Ptr(int64(19)),
		common.Ptr(int64(102))).
		Return(nil)
	s.mockEngine.EXPECT().SyncActivity(gomock.Any(), request).Return(nil).Times(1)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcessTask_SyncActivityReplicationTask_SameShardID_RetryTaskV2Error_SkipTask() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeSyncActivity.Ptr(),
		SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.SyncActivityRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	s.mockEngine.EXPECT().SyncActivity(gomock.Any(), request).Return(&types.RetryTaskV2Error{
		DomainID:          "test-domain-id",
		WorkflowID:        "test-wf-id",
		RunID:             "test-run-id",
		StartEventID:      common.Ptr(int64(11)),
		StartEventVersion: common.Ptr(int64(100)),
		EndEventID:        common.Ptr(int64(19)),
		EndEventVersion:   common.Ptr(int64(102)),
	}).Times(1)
	s.nDCHistoryResender.EXPECT().SendSingleWorkflowHistory(
		s.sourceCluster,
		"test-domain-id",
		"test-wf-id",
		"test-run-id",
		common.Ptr(int64(11)),
		common.Ptr(int64(100)),
		common.Ptr(int64(19)),
		common.Ptr(int64(102))).
		Return(ndc.ErrSkipTask)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcessTask_SyncActivityReplicationTask_SameShardID_RetryTaskV2Error_Error() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeSyncActivity.Ptr(),
		SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.SyncActivityRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	retryErr := &types.RetryTaskV2Error{
		DomainID:          "test-domain-id",
		WorkflowID:        "test-wf-id",
		RunID:             "test-run-id",
		StartEventID:      common.Ptr(int64(11)),
		StartEventVersion: common.Ptr(int64(100)),
		EndEventID:        common.Ptr(int64(19)),
		EndEventVersion:   common.Ptr(int64(102)),
	}
	s.mockEngine.EXPECT().SyncActivity(gomock.Any(), request).Return(retryErr).Times(1)
	s.nDCHistoryResender.EXPECT().SendSingleWorkflowHistory(
		s.sourceCluster,
		"test-domain-id",
		"test-wf-id",
		"test-run-id",
		common.Ptr(int64(11)),
		common.Ptr(int64(100)),
		common.Ptr(int64(19)),
		common.Ptr(int64(102))).
		Return(fmt.Errorf("some error"))
	_, err := s.taskHandler.execute(task, true)
	s.Error(err)
	s.ErrorIs(err, retryErr)
}

func (s *taskExecutorSuite) TestProcessTask_HistoryV2ReplicationTask_SameShardID() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.ReplicateEventsV2Request{
		DomainUUID: domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}

	s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(nil).Times(1)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcessTask_HistoryV2ReplicationTask_SameShardID_RetryTaskErr_Success() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.ReplicateEventsV2Request{
		DomainUUID: domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}

	retryErr := &types.RetryTaskV2Error{
		DomainID:          "test-domain-id",
		WorkflowID:        "test-wf-id",
		RunID:             "test-run-id",
		StartEventID:      common.Ptr(int64(11)),
		StartEventVersion: common.Ptr(int64(100)),
		EndEventID:        common.Ptr(int64(19)),
		EndEventVersion:   common.Ptr(int64(102)),
	}
	s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(retryErr).Times(1)
	s.nDCHistoryResender.EXPECT().SendSingleWorkflowHistory(
		s.sourceCluster,
		"test-domain-id",
		"test-wf-id",
		"test-run-id",
		common.Ptr(int64(11)),
		common.Ptr(int64(100)),
		common.Ptr(int64(19)),
		common.Ptr(int64(102))).
		Return(nil)
	s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(nil).Times(1)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcessTask_HistoryV2ReplicationTask_SameShardID_RetryTaskErr_SkipTask() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.ReplicateEventsV2Request{
		DomainUUID: domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}

	retryErr := &types.RetryTaskV2Error{
		DomainID:          "test-domain-id",
		WorkflowID:        "test-wf-id",
		RunID:             "test-run-id",
		StartEventID:      common.Ptr(int64(11)),
		StartEventVersion: common.Ptr(int64(100)),
		EndEventID:        common.Ptr(int64(19)),
		EndEventVersion:   common.Ptr(int64(102)),
	}
	s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(retryErr).Times(1)
	s.nDCHistoryResender.EXPECT().SendSingleWorkflowHistory(
		s.sourceCluster,
		"test-domain-id",
		"test-wf-id",
		"test-run-id",
		common.Ptr(int64(11)),
		common.Ptr(int64(100)),
		common.Ptr(int64(19)),
		common.Ptr(int64(102))).
		Return(ndc.ErrSkipTask)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcessTask_HistoryV2ReplicationTask_SameShardID_RetryTaskErr_Error() {
	domainID := uuid.New()
	workflowID := "6d89f939-e6a4-4c26-a0ed-626ce27bcc9c" // belong to shard 0
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.ReplicateEventsV2Request{
		DomainUUID: domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}

	retryErr := &types.RetryTaskV2Error{
		DomainID:          "test-domain-id",
		WorkflowID:        "test-wf-id",
		RunID:             "test-run-id",
		StartEventID:      common.Ptr(int64(11)),
		StartEventVersion: common.Ptr(int64(100)),
		EndEventID:        common.Ptr(int64(19)),
		EndEventVersion:   common.Ptr(int64(102)),
	}
	s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(retryErr).Times(1)
	s.nDCHistoryResender.EXPECT().SendSingleWorkflowHistory(
		s.sourceCluster,
		"test-domain-id",
		"test-wf-id",
		"test-run-id",
		common.Ptr(int64(11)),
		common.Ptr(int64(100)),
		common.Ptr(int64(19)),
		common.Ptr(int64(102))).
		Return(fmt.Errorf("some error"))
	_, err := s.taskHandler.execute(task, true)
	s.Error(err)
	s.ErrorIs(err, retryErr)
}

func (s *taskExecutorSuite) TestProcess_HistoryV2ReplicationTask_DifferentShardID() {
	domainID := uuid.New()
	workflowID := "abc" // belong to shard 1
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.ReplicateEventsV2Request{
		DomainUUID: domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}

	s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(nil).Times(0)
	s.historyClient.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(nil).Times(1)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcess_SyncActivityReplicationTask_DifferentShardID() {
	domainID := uuid.New()
	workflowID := "abc" // belong to shard 1
	runID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeSyncActivity.Ptr(),
		SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	request := &types.SyncActivityRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	s.mockEngine.EXPECT().SyncActivity(gomock.Any(), request).Return(nil).Times(0)
	s.historyClient.EXPECT().SyncActivity(gomock.Any(), request).Return(nil).Times(1)
	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcess_FailoverReplicationTask() {
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeFailoverMarker.Ptr(),
		FailoverMarkerAttributes: &types.FailoverMarkerAttributes{
			DomainID:        "test-domain-id",
			FailoverVersion: 101,
			CreationTime:    common.Ptr(int64(111)),
		},
		CreationTime: common.Ptr(int64(222)),
	}
	s.mockShard.MockAddingPendingFailoverMarker = func(marker *types.FailoverMarkerAttributes) error {
		s.Equal(&types.FailoverMarkerAttributes{
			DomainID:        "test-domain-id",
			FailoverVersion: 101,
			CreationTime:    common.Ptr(int64(222)),
		}, marker)
		return nil
	}

	_, err := s.taskHandler.execute(task, true)
	s.NoError(err)
}

func (s *taskExecutorSuite) TestProcess_UnknownTask() {
	task := &types.ReplicationTask{
		TaskType: common.Ptr(types.ReplicationTaskType(-100)),
	}
	_, err := s.taskHandler.execute(task, true)
	s.Error(err)
	s.ErrorIs(err, ErrUnknownReplicationTask)
}
