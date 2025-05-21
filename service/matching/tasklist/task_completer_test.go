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

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
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
		scope:           metrics.NoopScope,
		logger:          log.NewNoop(),
	}
	tc := newTaskCompleter(tlMgr, retryPolicy)

	return tc.(*taskCompleterImpl)
}

func TestCompleteTaskIfStarted(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2020, 8, 1, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name           string
		setupMock      func(*types.HistoryDescribeWorkflowExecutionRequest, *cache.MockDomainCache, *history.MockClient, clock.MockedTimeSource)
		task           func(chan bool) *InternalTask
		taskType       int
		isTaskComplete bool
		err            error
	}{
		{
			name: "error - could not get domain by ID from cache",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(nil, errors.New("error-getting-domain-by-id")).Times(retryPolicyMaxAttempts + 1)
			},
			isTaskComplete: false,
			err:            errors.New("error-getting-domain-by-id"),
		},
		{
			name: "error - domain is active",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil).Times(1)
			},
			isTaskComplete: false,
			err:            errDomainIsActive,
		},
		{
			name: "error - could not fetch workflow execution from history service",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(nil, errors.New("error-describing-workflow-execution")).Times(retryPolicyMaxAttempts + 1)
			},
			isTaskComplete: false,
			err:            errors.New("error-describing-workflow-execution"),
		},
		{
			name: "error - no WorkflowExecutionInfo in workflow execution response",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			isTaskComplete: false,
			err:            errWorkflowExecutionInfoIsNil,
		},
		{
			name: "error - task type not supported",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType:       999,
			isTaskComplete: false,
			err:            errTaskTypeNotSupported,
		},
		{
			name: "error - decision task not started - scheduleID greater than PendingDecision scheduleID",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 2,
						State:      types.PendingDecisionStateScheduled.Ptr(),
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType:       persistence.TaskListTypeDecision,
			isTaskComplete: false,
			err:            errTaskNotStarted,
		},
		{
			name: "error - decision task not started - scheduleID equal to PendingDecision scheduleID but PendingDecision state is not PendingDecisionStateStarted",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 3,
						State:      types.PendingDecisionStateScheduled.Ptr(),
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType:       persistence.TaskListTypeDecision,
			isTaskComplete: false,
			err:            errTaskNotStarted,
		},
		{
			name: "error - activity task not started - activity matching scheduleID is in PendingActivities but its state is not PendingActivityStateStarted",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
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
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(retryPolicyMaxAttempts + 1)
			},
			taskType:       persistence.TaskListTypeActivity,
			isTaskComplete: false,
			err:            errTaskNotStarted,
		},
		{
			name: "error - workflow not found and task created less than 24 hours before completion attempt",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(retryPolicyMaxAttempts + 1)
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(nil, &types.EntityNotExistsError{}).Times(retryPolicyMaxAttempts + 1)
			},
			isTaskComplete: false,
			err:            errWaitTimeNotReachedForEntityNotExists,
		},
		{
			name: "complete task - workflow not found and task created more than 24 hours before completion attempt",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(nil, &types.EntityNotExistsError{}).Times(1)
				timeSource.Advance(time.Hour*24 + time.Second)
			},
			isTaskComplete: true,
			err:            nil,
		},
		{
			name: "complete task - workflow closed",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
						CloseStatus: types.WorkflowExecutionCloseStatusCompleted.Ptr(),
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(1)
			},
			taskType:       persistence.TaskListTypeDecision,
			isTaskComplete: true,
			err:            nil,
		},
		{
			name: "complete decision task - no pending decision",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(1)
			},
			taskType:       persistence.TaskListTypeDecision,
			isTaskComplete: true,
			err:            nil,
		},
		{
			name: "complete decision task - scheduleID is less than PendingDecision scheduleID",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  2,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 3,
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(1)
			},
			taskType:       persistence.TaskListTypeDecision,
			isTaskComplete: true,
			err:            nil,
		},
		{
			name: "complete decision task - scheduleID is equal to PendingDecision scheduleID and PendingDecision state is PendingDecisionStateStarted",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
				mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalStandbyDomainEntry, nil).Times(1)
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					PendingDecision: &types.PendingDecisionInfo{
						ScheduleID: 3,
						State:      types.PendingDecisionStateStarted.Ptr(),
					},
				}
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(1)
			},
			taskType:       persistence.TaskListTypeDecision,
			isTaskComplete: true,
			err:            nil,
		},
		{
			name: "complete activity task - no activity matching scheduleID in PendingActivities",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
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
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(1)
			},
			taskType:       persistence.TaskListTypeActivity,
			isTaskComplete: true,
			err:            nil,
		},
		{
			name: "complete activity task - activity matching scheduleID is in PendingActivities and its state is PendingActivityStateStarted",
			task: func(isComplete chan bool) *InternalTask {
				return &InternalTask{
					Event: &genericTaskInfo{
						TaskInfo: &persistence.TaskInfo{
							DomainID:    constants.TestDomainID,
							WorkflowID:  constants.TestWorkflowID,
							RunID:       constants.TestRunID,
							ScheduleID:  3,
							CreatedTime: createdAt,
						},
						completionFunc: func(_ *persistence.TaskInfo, _ error) { isComplete <- true },
					},
				}
			},
			setupMock: func(req *types.HistoryDescribeWorkflowExecutionRequest, mockDomainCache *cache.MockDomainCache, mockHistoryService *history.MockClient, timeSource clock.MockedTimeSource) {
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
				mockHistoryService.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(resp, nil).Times(1)
			},
			taskType:       persistence.TaskListTypeActivity,
			isTaskComplete: true,
			err:            nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tCmp := createTestTaskCompleter(ctrl, tc.taskType)

			defer ctrl.Finish()

			isTaskComplete := make(chan bool, 1)

			task := tc.task(isTaskComplete)

			req := &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: task.Event.TaskInfo.DomainID,
				Request: &types.DescribeWorkflowExecutionRequest{
					Domain: task.domainName,
					Execution: &types.WorkflowExecution{
						WorkflowID: task.Event.WorkflowID,
						RunID:      task.Event.RunID,
					},
				},
			}

			mockedTimeSource := clock.NewMockedTimeSourceAt(createdAt)
			tCmp.timeSource = mockedTimeSource

			tc.setupMock(req, tCmp.domainCache.(*cache.MockDomainCache), tCmp.historyService.(*history.MockClient), mockedTimeSource)

			err := tCmp.CompleteTaskIfStarted(ctx, task)

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

			var val bool
			select {
			case val = <-isTaskComplete:
			default:
			}

			assert.Equal(t, tc.isTaskComplete, val)
		})
	}
}
