// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package tasklist

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
)

var retryPolicyMaxAttempts = 3

func createTestTaskCompleter(controller *gomock.Controller, taskType int) *taskCompleterImpl {
	mockDomainCache := cache.NewMockDomainCache(controller)
	mockHistoryService := history.NewMockClient(controller)

	retryPolicy := backoff.NewExponentialRetryPolicy(100 * time.Millisecond)
	retryPolicy.SetMaximumAttempts(retryPolicyMaxAttempts)

	tlMgr := &taskListManagerImpl{
		domainCache:     mockDomainCache,
		historyService:  mockHistoryService,
		taskListID:      &Identifier{domainID: constants.TestDomainID, taskType: taskType},
		clusterMetadata: cluster.GetTestClusterMetadata(true),
		scope:           metrics.NoopScope(1),
		logger:          log.NewNoop(),
	}
	tc := newTaskCompleter(tlMgr, retryPolicy)

	return tc.(*taskCompleterImpl)
}

func TestCompleteTaskIfStarted(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name      string
		setupMock func(*types.HistoryDescribeWorkflowExecutionRequest, *cache.MockDomainCache, *history.MockClient)
		task      *InternalTask
		taskType  int
		err       error
	}{
		{
			name: "error - could not get domain by ID from cache",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID: constants.TestDomainID,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(nil, errors.New("error-getting-domain-by-id")).Times(retryPolicyMaxAttempts + 1)
			},
			err: errors.New("error-getting-domain-by-id"),
		},
		{
			name: "error - domain is active",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID: constants.TestDomainID,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil).Times(1)
			},
			err: errDomainIsActive,
		},
		{
			name: "error - could not fetch workflow execution from history service",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(nil, errors.New("error-describing-workflow-execution")).Times(retryPolicyMaxAttempts + 1)
			},
			err: errors.New("error-describing-workflow-execution"),
		},
		{
			name: "error - no WorkflowExecutionInfo in workflow execution response",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			err: errWorkflowExecutionInfoIsNil,
		},
		{
			name: "error - task type not supported",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType: 999,
			err:      errTaskTypeNotSupported,
		},
		{
			name: "error - decision task not started - scheduleID greater than PendingDecision scheduleID",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 2,
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType: persistence.TaskListTypeDecision,
			err:      errTaskNotStarted,
		},
		{
			name: "error - decision task not started - scheduleID equal to PendingDecision scheduleID but PendingDecision state is not PendingDecisionStateStarted",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 3,
						State:      types.PendingDecisionStateScheduled.Ptr(),
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType: persistence.TaskListTypeDecision,
			err:      errTaskNotStarted,
		},
		{
			name: "error - activity task not started - activity matching scheduleID is in PendingActivities but its state is not PendingActivityStateStarted",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingActivities: []*types.PendingActivityInfo{
						{
							ScheduleID: 3,
							State:      types.PendingActivityStateScheduled.Ptr(),
						},
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType: persistence.TaskListTypeActivity,
			err:      errTaskNotStarted,
		},
		{
			name: "complete task - workflow not found",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(nil, &types.EntityNotExistsError{}).Times(1)
			},
			err: nil,
		},
		{
			name: "complete task - workflow closed",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
						CloseStatus: types.WorkflowExecutionCloseStatusCompleted.Ptr(),
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(1)
			},
			taskType: persistence.TaskListTypeDecision,
			err:      nil,
		},
		{
			name: "complete decision task - no pending decision",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(1)
			},
			taskType: persistence.TaskListTypeDecision,
			err:      nil,
		},
		{
			name: "complete decision task - scheduleID is less than PendingDecision scheduleID",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 2,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 3,
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(1)
			},
			taskType: persistence.TaskListTypeDecision,
			err:      nil,
		},
		{
			name: "complete decision task - scheduleID is equal to PendingDecision scheduleID and PendingDecision state is PendingDecisionStateStarted",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 3,
						State:      types.PendingDecisionStateStarted.Ptr(),
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(1)
			},
			taskType: persistence.TaskListTypeDecision,
			err:      nil,
		},
		{
			name: "complete activity task - no activity matching scheduleID in PendingActivities",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingActivities: []*types.PendingActivityInfo{
						{
							ScheduleID: 2,
							State:      types.PendingActivityStateScheduled.Ptr(),
						},
						{
							ScheduleID: 4,
							State:      types.PendingActivityStateScheduled.Ptr(),
						},
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(1)
			},
			taskType: persistence.TaskListTypeActivity,
			err:      nil,
		},
		{
			name: "complete activity task - activity matching scheduleID is in PendingActivities and its state is PendingActivityStateStarted",
			task: &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
						ScheduleID: 3,
					},
				},
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingActivities: []*types.PendingActivityInfo{
						{
							ScheduleID: 3,
							State:      types.PendingActivityStateStarted.Ptr(),
						},
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(ctx, req).Return(resp, nil).Times(1)
			},
			taskType: persistence.TaskListTypeActivity,
			err:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tCmp := createTestTaskCompleter(ctrl, tc.taskType)

			defer ctrl.Finish()

			req := &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: tc.task.Event.TaskInfo.DomainID,
				Request: &types.DescribeWorkflowExecutionRequest{
					Domain: tc.task.domainName,
					Execution: &types.WorkflowExecution{
						WorkflowID: tc.task.Event.WorkflowID,
						RunID:      tc.task.Event.RunID,
					},
				},
			}

			tc.setupMock(req, tCmp.domainCache.(*cache.MockDomainCache), tCmp.historyService.(*history.MockClient))

			err := tCmp.CompleteTaskIfStarted(ctx, tc.task)

			if tc.err != nil {
				assert.Error(t, err)
				if errors.Unwrap(err) != nil {
					assert.ErrorContains(t, errors.Unwrap(err), tc.err.Error())
				} else {
					assert.ErrorContains(t, err, tc.err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
