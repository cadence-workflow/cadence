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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	executioncache "github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/worker/archiver"
)

type (
	timerQueueTaskExecutorBaseSuite struct {
		suite.Suite
		*require.Assertions

		controller                   *gomock.Controller
		mockShard                    *shard.TestContext
		mockWorkflowExecutionContext *execution.MockContext
		mockMutableState             *execution.MockMutableState

		mockExecutionManager  *mocks.ExecutionManager
		mockVisibilityManager *mocks.VisibilityManager
		mockHistoryV2Manager  *mocks.HistoryV2Manager
		mockArchivalClient    *archiver.MockClient

		timerQueueTaskExecutorBase *timerTaskExecutorBase
	}
)

func TestTimerQueueTaskExecutorBaseSuite(t *testing.T) {
	s := new(timerQueueTaskExecutorBaseSuite)
	suite.Run(t, s)
}

func (s *timerQueueTaskExecutorBaseSuite) SetupSuite() {

}

func (s *timerQueueTaskExecutorBaseSuite) TearDownSuite() {

}

func (s *timerQueueTaskExecutorBaseSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockWorkflowExecutionContext = execution.NewMockContext(s.controller)
	s.mockMutableState = execution.NewMockMutableState(s.controller)

	testConfig := config.NewForTest()
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          0,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		testConfig,
	)

	s.mockExecutionManager = s.mockShard.Resource.ExecutionMgr
	s.mockVisibilityManager = s.mockShard.Resource.VisibilityMgr
	s.mockHistoryV2Manager = s.mockShard.Resource.HistoryMgr
	s.mockArchivalClient = archiver.NewMockClient(s.controller)

	logger := s.mockShard.GetLogger()

	s.timerQueueTaskExecutorBase = newTimerTaskExecutorBase(
		s.mockShard,
		s.mockArchivalClient,
		nil,
		logger,
		s.mockShard.GetMetricsClient(),
		testConfig,
	)
}

func (s *timerQueueTaskExecutorBaseSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
	s.timerQueueTaskExecutorBase.Stop()
}

func (s *timerQueueTaskExecutorBaseSuite) TestDeleteWorkflow_NoErr() {
	task := &persistence.DeleteHistoryEventTask{
		TaskData: persistence.TaskData{
			TaskID:              12345,
			VisibilityTimestamp: time.Now(),
		},
	}
	executionInfo := types.WorkflowExecution{
		WorkflowID: task.WorkflowID,
		RunID:      task.RunID,
	}
	wfContext := execution.NewContext(task.DomainID, executionInfo, s.mockShard, s.mockExecutionManager, log.NewNoop())

	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("Sample", nil).AnyTimes()

	s.mockExecutionManager.On("DeleteCurrentWorkflowExecution", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionManager.On("DeleteWorkflowExecution", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionManager.On("DeleteActiveClusterSelectionPolicy", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.mockHistoryV2Manager.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockVisibilityManager.On("DeleteWorkflowExecution", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.mockMutableState.EXPECT().GetCurrentBranchToken().Return([]byte{1, 2, 3}, nil).Times(1)
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(int64(1234), nil).AnyTimes()

	err := s.timerQueueTaskExecutorBase.deleteWorkflow(context.Background(), task, wfContext, s.mockMutableState)
	s.NoError(err)
}

func (s *timerQueueTaskExecutorBaseSuite) TestArchiveHistory_NoErr_InlineArchivalFailed() {
	s.mockWorkflowExecutionContext.EXPECT().LoadExecutionStats(gomock.Any()).Return(&persistence.ExecutionStats{
		HistorySize: 1024,
	}, nil).Times(1)
	s.mockWorkflowExecutionContext.EXPECT().Clear().Times(1)

	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("Sample", nil)

	s.mockMutableState.EXPECT().GetCurrentBranchToken().Return([]byte{1, 2, 3}, nil).Times(1)
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(int64(1234), nil).Times(1)
	s.mockMutableState.EXPECT().GetNextEventID().Return(int64(101)).Times(1)
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("Sample", nil).AnyTimes()
	s.mockExecutionManager.On("DeleteCurrentWorkflowExecution", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionManager.On("DeleteWorkflowExecution", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionManager.On("DeleteActiveClusterSelectionPolicy", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.mockVisibilityManager.On("DeleteWorkflowExecution", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	s.mockArchivalClient.EXPECT().Archive(gomock.Any(), gomock.Cond(func(req *archiver.ClientRequest) bool {
		return req.CallerService == service.History && req.AttemptArchiveInline && req.ArchiveRequest.Targets[0] == archiver.ArchiveTargetHistory
	})).Return(&archiver.ClientResponse{
		HistoryArchivedInline: false,
	}, nil)

	domainCacheEntry := cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{},
		&persistence.DomainConfig{},
		false,
		nil,
		0,
		nil,
		0,
		0,
		0,
	)
	err := s.timerQueueTaskExecutorBase.archiveWorkflow(
		context.Background(),
		&persistence.DeleteHistoryEventTask{},
		s.mockWorkflowExecutionContext,
		s.mockMutableState,
		domainCacheEntry,
	)
	s.NoError(err)
}

func (s *timerQueueTaskExecutorBaseSuite) TestArchiveHistory_SendSignalErr() {
	s.mockWorkflowExecutionContext.EXPECT().LoadExecutionStats(gomock.Any()).Return(&persistence.ExecutionStats{
		HistorySize: 1024 * 1024 * 1024,
	}, nil).Times(1)

	s.mockMutableState.EXPECT().GetCurrentBranchToken().Return([]byte{1, 2, 3}, nil).Times(1)
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(int64(1234), nil).Times(1)
	s.mockMutableState.EXPECT().GetNextEventID().Return(int64(101)).Times(1)

	s.mockArchivalClient.EXPECT().Archive(gomock.Any(), gomock.Cond(func(req *archiver.ClientRequest) bool {
		return req.CallerService == service.History && !req.AttemptArchiveInline && req.ArchiveRequest.Targets[0] == archiver.ArchiveTargetHistory
	})).Return(nil, errors.New("failed to send signal"))

	domainCacheEntry := cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{},
		&persistence.DomainConfig{},
		false,
		nil,
		0,
		nil,
		0,
		0,
		0,
	)
	err := s.timerQueueTaskExecutorBase.archiveWorkflow(
		context.Background(),
		&persistence.DeleteHistoryEventTask{},
		s.mockWorkflowExecutionContext,
		s.mockMutableState, domainCacheEntry,
	)
	s.Error(err)
}

func TestExecuteDeleteHistoryEventTask(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*gomock.Controller) (*timerTaskExecutorBase, *persistence.DeleteHistoryEventTask)
		expectedError error
	}{
		{
			name: "mutable was not able to be loaded",
			setupMocks: func(controller *gomock.Controller) (*timerTaskExecutorBase, *persistence.DeleteHistoryEventTask) {

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

				mockShard.Resource.DomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					false,
					nil,
					0,
					nil,
					0,
					0,
					0,
				), nil)

				timerTask := &persistence.DeleteHistoryEventTask{
					TaskData: persistence.TaskData{
						TaskID:              123,
						VisibilityTimestamp: time.Now(),
					},
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "test-domain",
						WorkflowID: "wf",
						RunID:      "run",
					},
				}

				wfContext := execution.NewContext(
					timerTask.DomainID,
					types.WorkflowExecution{
						WorkflowID: timerTask.WorkflowID,
						RunID:      timerTask.RunID,
					},
					mockShard,
					mockShard.Resource.ExecutionMgr,
					mockShard.GetLogger(),
				)

				executionCache := executioncache.NewMockCache(controller)
				executionCache.EXPECT().GetOrCreateWorkflowExecutionWithTimeout(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wfContext, func(error) {}, nil)

				mockExecutionMgr := mockShard.Resource.ExecutionMgr
				mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

				return newTimerTaskExecutorBase(
					mockShard,
					&archiver.MockClient{},
					executionCache,
					mockShard.GetLogger(),
					mockShard.GetMetricsClient(),
					mockShard.GetConfig(),
				), timerTask
			},
			expectedError: nil,
		},
		{
			name: "mutable showed that the workflow was still running ",
			setupMocks: func(controller *gomock.Controller) (*timerTaskExecutorBase, *persistence.DeleteHistoryEventTask) {

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

				mockShard.Resource.DomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					false,
					nil,
					0,
					nil,
					0,
					0,
					0,
				), nil).AnyTimes()

				timerTask := &persistence.DeleteHistoryEventTask{
					TaskData: persistence.TaskData{
						TaskID:              123,
						VisibilityTimestamp: time.Now(),
					},
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain",
						WorkflowID: "wf",
						RunID:      "run",
					},
				}

				wfContext := execution.NewContext(
					timerTask.DomainID,
					types.WorkflowExecution{
						WorkflowID: timerTask.WorkflowID,
						RunID:      timerTask.RunID,
					},
					mockShard,
					mockShard.Resource.ExecutionMgr,
					mockShard.GetLogger(),
				)

				executionCache := executioncache.NewMockCache(controller)
				executionCache.EXPECT().GetOrCreateWorkflowExecutionWithTimeout(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wfContext, func(error) {}, nil)

				mockExecutionMgr := mockShard.Resource.ExecutionMgr
				mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionStats: &persistence.ExecutionStats{},
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							CloseStatus: 0,
							State:       persistence.WorkflowStateRunning,
						},
					},
				}, nil)

				return newTimerTaskExecutorBase(
					mockShard,
					&archiver.MockClient{},
					executionCache,
					mockShard.GetLogger(),
					mockShard.GetMetricsClient(),
					mockShard.GetConfig(),
				), timerTask
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			defer controller.Finish()

			executor, timerTask := tt.setupMocks(controller)
			err := executor.executeDeleteHistoryEventTask(context.Background(), timerTask)
			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
