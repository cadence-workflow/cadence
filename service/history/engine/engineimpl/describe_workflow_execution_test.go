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

package engineimpl

import (
	ctx "context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	historyConstants "github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
	"github.com/uber/cadence/service/history/execution"
)

func TestDescribeWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name                string
		expirationTimestamp *int64
	}{
		{
			name:                "Success",
			expirationTimestamp: common.Int64Ptr(2004),
		},
		{
			name:                "Success - activity retry without expiration timestamp",
			expirationTimestamp: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)

			childDomainID := "deleted-domain"
			eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainName(historyConstants.TestDomainID).Return(historyConstants.TestDomainName, nil).AnyTimes()
			eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainName(childDomainID).Return("", &types.EntityNotExistsError{}).AnyTimes()
			execution := types.WorkflowExecution{
				WorkflowID: historyConstants.TestWorkflowID,
				RunID:      historyConstants.TestRunID,
			}
			parentWorkflowID := "parentWorkflowID"
			parentRunID := "parentRunID"
			taskList := "taskList"
			executionStartToCloseTimeoutSeconds := int32(1335)
			taskStartToCloseTimeoutSeconds := int32(1336)
			initiatedID := int64(1337)
			typeName := "type"
			startTime := time.UnixMilli(1)
			backOffTime := time.Second
			executionTime := int64(startTime.Nanosecond()) + backOffTime.Nanoseconds()
			historyLength := int64(10)
			state := persistence.WorkflowStateCompleted
			status := persistence.WorkflowCloseStatusCompleted
			closeTime := common.Int64Ptr(1338)
			isCron := true
			autoResetPoints := &types.ResetPoints{Points: []*types.ResetPointInfo{
				{
					BinaryChecksum:           "idk",
					RunID:                    "its a run I guess",
					FirstDecisionCompletedID: 1,
					CreatedTimeNano:          common.Int64Ptr(2),
					ExpiringTimeNano:         common.Int64Ptr(3),
					Resettable:               false,
				},
			}}
			memoFields := map[string][]byte{
				"foo": []byte("bar"),
			}
			searchAttributes := map[string][]byte{
				"search": []byte("attribute"),
			}
			partitionConfig := map[string]string{
				"lots of": "partitions",
			}
			lastUpdatedTime := time.UnixMilli(12345)
			pendingDecisionScheduleID := int64(1000)
			pendingDecisionScheduledTime := int64(1001)
			pendingDecisionAttempt := int64(1002)
			pendingDecisionOriginalScheduledTime := int64(1003)
			pendingDecisionStartedID := int64(1004)
			pendingDecisionStartedTimestamp := int64(1005)
			activity1 := &types.PendingActivityInfo{
				ActivityID: "1",
				ActivityType: &types.ActivityType{
					Name: "activity1Type",
				},
				State:                  types.PendingActivityStateStarted.Ptr(),
				HeartbeatDetails:       []byte("boom boom"),
				LastHeartbeatTimestamp: common.Int64Ptr(2000),
				LastStartedTimestamp:   common.Int64Ptr(2001),
				Attempt:                2002,
				MaximumAttempts:        2003,
				ExpirationTimestamp:    tc.expirationTimestamp,
				LastFailureReason:      common.StringPtr("failure reason"),
				StartedWorkerIdentity:  "StartedWorkerIdentity",
				LastWorkerIdentity:     "LastWorkerIdentity",
				LastFailureDetails:     []byte("failure details"),
				ScheduleID:             1,
			}
			child1 := &types.PendingChildExecutionInfo{
				Domain:            childDomainID,
				WorkflowID:        "childWorkflowID",
				RunID:             "childRunID",
				WorkflowTypeName:  "childWorkflowTypeName",
				InitiatedID:       3000,
				ParentClosePolicy: types.ParentClosePolicyAbandon.Ptr(),
			}

			var expirationTime time.Time
			if tc.expirationTimestamp != nil {
				expirationTime = time.Unix(0, *tc.expirationTimestamp)
			}

			eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ActivityInfos: map[int64]*persistence.ActivityInfo{
						1: {
							ScheduleID: activity1.ScheduleID,
							ScheduledEvent: &types.HistoryEvent{
								ID: 1,
								ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
									ActivityType: activity1.ActivityType,
								},
							},
							StartedID:                2,
							StartedTime:              time.Unix(0, *activity1.LastStartedTimestamp),
							DomainID:                 historyConstants.TestDomainID,
							ActivityID:               activity1.ActivityID,
							Details:                  activity1.HeartbeatDetails,
							LastHeartBeatUpdatedTime: time.Unix(0, *activity1.LastHeartbeatTimestamp),
							Attempt:                  activity1.Attempt,
							StartedIdentity:          activity1.StartedWorkerIdentity,
							TaskList:                 taskList,
							HasRetryPolicy:           true,
							ExpirationTime:           expirationTime,
							MaximumAttempts:          activity1.MaximumAttempts,
							LastFailureReason:        *activity1.LastFailureReason,
							LastWorkerIdentity:       activity1.LastWorkerIdentity,
							LastFailureDetails:       activity1.LastFailureDetails,
						},
					},
					ChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{
						2: {
							InitiatedID:       child1.InitiatedID,
							StartedWorkflowID: child1.WorkflowID,
							StartedRunID:      child1.RunID,
							DomainID:          childDomainID,
							WorkflowTypeName:  child1.WorkflowTypeName,
							ParentClosePolicy: *child1.ParentClosePolicy,
						},
					},
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:               historyConstants.TestDomainID,
						WorkflowID:             historyConstants.TestWorkflowID,
						RunID:                  historyConstants.TestRunID,
						FirstExecutionRunID:    "first",
						ParentDomainID:         historyConstants.TestDomainID,
						ParentWorkflowID:       parentWorkflowID,
						ParentRunID:            parentRunID,
						InitiatedID:            initiatedID,
						CompletionEventBatchID: 1, // Just needs to be set
						CompletionEvent: &types.HistoryEvent{
							Timestamp: closeTime,
						},
						TaskList:                           taskList,
						WorkflowTypeName:                   typeName,
						WorkflowTimeout:                    executionStartToCloseTimeoutSeconds,
						DecisionStartToCloseTimeout:        taskStartToCloseTimeoutSeconds,
						State:                              state,
						CloseStatus:                        status,
						NextEventID:                        historyLength + 1,
						StartTimestamp:                     startTime,
						LastUpdatedTimestamp:               lastUpdatedTime,
						DecisionScheduleID:                 pendingDecisionScheduleID,
						DecisionStartedID:                  pendingDecisionStartedID,
						DecisionAttempt:                    pendingDecisionAttempt,
						DecisionStartedTimestamp:           pendingDecisionStartedTimestamp,
						DecisionScheduledTimestamp:         pendingDecisionScheduledTime,
						DecisionOriginalScheduledTimestamp: pendingDecisionOriginalScheduledTime,
						AutoResetPoints:                    autoResetPoints,
						Memo:                               memoFields,
						SearchAttributes:                   searchAttributes,
						PartitionConfig:                    partitionConfig,
						CronSchedule:                       "yes we are cron",
						IsCron:                             isCron,
					},
					ExecutionStats: &persistence.ExecutionStats{
						HistorySize: historyLength,
					},
				},
			}, nil).Once()
			eft.ShardCtx.Resource.HistoryMgr.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(&persistence.ReadHistoryBranchResponse{
				HistoryEvents: []*types.HistoryEvent{
					{
						ID: 1,
						WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
							FirstDecisionTaskBackoffSeconds: common.Int32Ptr(int32(backOffTime.Seconds())),
						},
					},
				},
				Size:             1,
				LastFirstEventID: 1,
			}, nil)

			eft.Engine.Start()
			result, err := eft.Engine.DescribeWorkflowExecution(ctx.Background(), &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: historyConstants.TestDomainID,
				Request: &types.DescribeWorkflowExecutionRequest{
					Domain:    "why do we even have this field?",
					Execution: &execution,
				},
			})
			eft.Engine.Stop()
			assert.Equal(t, &types.DescribeWorkflowExecutionResponse{
				ExecutionConfiguration: &types.WorkflowExecutionConfiguration{
					TaskList: &types.TaskList{
						Name: taskList,
					},
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionStartToCloseTimeoutSeconds),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(taskStartToCloseTimeoutSeconds),
				},
				WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
					Execution: &execution,
					Type: &types.WorkflowType{
						Name: typeName,
					},
					StartTime:      common.Int64Ptr(startTime.UnixNano()),
					CloseTime:      closeTime,
					CloseStatus:    types.WorkflowExecutionCloseStatusCompleted.Ptr(),
					HistoryLength:  historyLength,
					ParentDomainID: &historyConstants.TestDomainID,
					ParentDomain:   &historyConstants.TestDomainName,
					ParentExecution: &types.WorkflowExecution{
						WorkflowID: parentWorkflowID,
						RunID:      parentRunID,
					},
					ParentInitiatedID: common.Int64Ptr(initiatedID),
					ExecutionTime:     common.Int64Ptr(executionTime),
					Memo:              &types.Memo{Fields: memoFields},
					SearchAttributes:  &types.SearchAttributes{IndexedFields: searchAttributes},
					AutoResetPoints:   autoResetPoints,
					TaskList: &types.TaskList{
						Name: taskList,
						Kind: types.TaskListKindNormal.Ptr(),
					},
					IsCron:                       isCron,
					UpdateTime:                   common.Int64Ptr(lastUpdatedTime.UnixNano()),
					PartitionConfig:              partitionConfig,
					CronOverlapPolicy:            &historyConstants.CronSkip,
					ActiveClusterSelectionPolicy: nil,
				},
				PendingActivities: []*types.PendingActivityInfo{
					activity1,
				},
				PendingChildren: []*types.PendingChildExecutionInfo{
					child1,
				},
				PendingDecision: &types.PendingDecisionInfo{
					State:                      types.PendingDecisionStateStarted.Ptr(),
					ScheduledTimestamp:         common.Int64Ptr(pendingDecisionScheduledTime),
					StartedTimestamp:           common.Int64Ptr(pendingDecisionStartedTimestamp),
					Attempt:                    pendingDecisionAttempt,
					OriginalScheduledTimestamp: common.Int64Ptr(pendingDecisionOriginalScheduledTime),
					ScheduleID:                 pendingDecisionScheduleID,
				},
			}, result)
			assert.Nil(t, err)
		})
	}
}

func TestMapWorkflowExecutionConfiguration(t *testing.T) {
	testCases := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		expected      *types.WorkflowExecutionConfiguration
	}{
		{
			name: "Success - all fields present",
			executionInfo: &persistence.WorkflowExecutionInfo{
				TaskList:                    "test-task-list",
				WorkflowTimeout:             1800,
				DecisionStartToCloseTimeout: 30,
			},
			expected: &types.WorkflowExecutionConfiguration{
				TaskList:                            &types.TaskList{Name: "test-task-list"},
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1800),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(30),
			},
		},
		{
			name: "Success - zero values",
			executionInfo: &persistence.WorkflowExecutionInfo{
				TaskList:                    "",
				WorkflowTimeout:             0,
				DecisionStartToCloseTimeout: 0,
			},
			expected: &types.WorkflowExecutionConfiguration{
				TaskList:                            &types.TaskList{Name: ""},
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(0),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(0),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := mapWorkflowExecutionConfiguration(tc.executionInfo)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TODO: More test cases with different states of the workflow
func TestMapWorkflowExecutionInfo(t *testing.T) {
	testCases := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		startEvent    *types.HistoryEvent
		setupMock     func(*cache.MockDomainCache)
		expectError   bool
		expected      *types.WorkflowExecutionInfo
	}{
		{
			name: "Success - basic workflow with parent",
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowID:                   "test-workflow-id",
				RunID:                        "test-run-id",
				TaskList:                     "test-task-list",
				TaskListKind:                 types.TaskListKindNormal,
				WorkflowTypeName:             "test-workflow-type",
				StartTimestamp:               time.Unix(0, 1000000),
				LastUpdatedTimestamp:         time.Unix(0, 2000000),
				AutoResetPoints:              &types.ResetPoints{Points: []*types.ResetPointInfo{}},
				Memo:                         map[string][]byte{"key": []byte("value")},
				SearchAttributes:             map[string][]byte{"attr": []byte("val")},
				PartitionConfig:              map[string]string{"partition": "config"},
				CronOverlapPolicy:            historyConstants.CronSkip,
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{},
				ParentWorkflowID:             "parent-workflow-id",
				ParentRunID:                  "parent-run-id",
				ParentDomainID:               "parent-domain-id",
				InitiatedID:                  123,
				State:                        persistence.WorkflowStateCompleted,
				CloseStatus:                  persistence.WorkflowCloseStatusCompleted,
			},
			startEvent: &types.HistoryEvent{
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					FirstDecisionTaskBackoffSeconds: common.Int32Ptr(10),
				},
			},
			setupMock: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainName("parent-domain-id").Return("parent-domain-name", nil)
			},
			expectError: false,
			expected: &types.WorkflowExecutionInfo{
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
				TaskList: &types.TaskList{
					Name: "test-task-list",
					Kind: types.TaskListKindNormal.Ptr(),
				},
				Type:                         &types.WorkflowType{Name: "test-workflow-type"},
				StartTime:                    common.Int64Ptr(1000000),
				HistoryLength:                10,
				AutoResetPoints:              &types.ResetPoints{Points: []*types.ResetPointInfo{}},
				Memo:                         &types.Memo{Fields: map[string][]byte{"key": []byte("value")}},
				IsCron:                       false,
				UpdateTime:                   common.Int64Ptr(2000000),
				SearchAttributes:             &types.SearchAttributes{IndexedFields: map[string][]byte{"attr": []byte("val")}},
				PartitionConfig:              map[string]string{"partition": "config"},
				CronOverlapPolicy:            &historyConstants.CronSkip,
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{},
				ExecutionTime:                common.Int64Ptr(1000000 + (10 * time.Second).Nanoseconds()),
				ParentExecution: &types.WorkflowExecution{
					WorkflowID: "parent-workflow-id",
					RunID:      "parent-run-id",
				},
				ParentDomainID:    common.StringPtr("parent-domain-id"),
				ParentInitiatedID: common.Int64Ptr(123),
				ParentDomain:      common.StringPtr("parent-domain-name"),
				CloseStatus:       types.WorkflowExecutionCloseStatusCompleted.Ptr(),
				CloseTime:         common.Int64Ptr(12345),
			},
		},
		{
			name: "Success - basic workflow without parent",
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowID:           "test-workflow-id",
				RunID:                "test-run-id",
				TaskList:             "test-task-list",
				TaskListKind:         types.TaskListKindSticky,
				WorkflowTypeName:     "test-workflow-type",
				StartTimestamp:       time.Unix(0, 1000000),
				LastUpdatedTimestamp: time.Unix(0, 2000000),
				AutoResetPoints:      &types.ResetPoints{Points: []*types.ResetPointInfo{}},
				Memo:                 map[string][]byte{},
				SearchAttributes:     map[string][]byte{},
				PartitionConfig:      map[string]string{},
				CronSchedule:         "0 0 * * *",
				State:                persistence.WorkflowStateRunning,
			},
			startEvent: &types.HistoryEvent{
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
				},
			},
			setupMock:   func(mockDomainCache *cache.MockDomainCache) {},
			expectError: false,
			expected: &types.WorkflowExecutionInfo{
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
				TaskList: &types.TaskList{
					Name: "test-task-list",
					Kind: types.TaskListKindSticky.Ptr(),
				},
				Type:                         &types.WorkflowType{Name: "test-workflow-type"},
				StartTime:                    common.Int64Ptr(1000000),
				HistoryLength:                10,
				AutoResetPoints:              &types.ResetPoints{Points: []*types.ResetPointInfo{}},
				Memo:                         &types.Memo{Fields: map[string][]byte{}},
				IsCron:                       true,
				UpdateTime:                   common.Int64Ptr(2000000),
				SearchAttributes:             &types.SearchAttributes{IndexedFields: map[string][]byte{}},
				PartitionConfig:              map[string]string{},
				CronOverlapPolicy:            &historyConstants.CronSkip,
				ActiveClusterSelectionPolicy: nil,
				ExecutionTime:                common.Int64Ptr(1000000 + (5 * time.Second).Nanoseconds()),
			},
		},
		{
			name: "Error - parent domain name lookup fails",
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowID:                   "test-workflow-id",
				RunID:                        "test-run-id",
				TaskList:                     "test-task-list",
				TaskListKind:                 types.TaskListKindNormal,
				WorkflowTypeName:             "test-workflow-type",
				StartTimestamp:               time.Unix(0, 1000000),
				LastUpdatedTimestamp:         time.Unix(0, 2000000),
				AutoResetPoints:              &types.ResetPoints{Points: []*types.ResetPointInfo{}},
				Memo:                         map[string][]byte{"key": []byte("value")},
				SearchAttributes:             map[string][]byte{"attr": []byte("val")},
				PartitionConfig:              map[string]string{"partition": "config"},
				CronOverlapPolicy:            historyConstants.CronSkip,
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{},
				ParentWorkflowID:             "parent-workflow-id",
				ParentRunID:                  "parent-run-id",
				ParentDomainID:               "parent-domain-id",
				InitiatedID:                  123,
				State:                        persistence.WorkflowStateRunning,
			},
			startEvent: &types.HistoryEvent{
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					FirstDecisionTaskBackoffSeconds: common.Int32Ptr(10),
				},
			},
			setupMock: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainName("parent-domain-id").Return("", &types.InternalServiceError{Message: "Domain lookup failed"})
			},
			expectError: true,
			expected:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDomainCache := cache.NewMockDomainCache(ctrl)
			tc.setupMock(mockDomainCache)

			result, err := mapWorkflowExecutionInfo(tc.executionInfo, tc.startEvent, mockDomainCache, 10, &types.HistoryEvent{Timestamp: common.Int64Ptr(12345)})

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

// TODO: More test cases for any other branches
func TestMapPendingActivityInfo(t *testing.T) {
	testCases := []struct {
		name                   string
		activityInfo           *persistence.ActivityInfo
		activityScheduledEvent *types.HistoryEvent
		expected               *types.PendingActivityInfo
	}{
		{
			name: "Success - started activity with retry policy",
			activityInfo: &persistence.ActivityInfo{
				ActivityID:               "test-activity-id",
				ScheduleID:               123,
				StartedID:                124,
				CancelRequested:          false,
				StartedTime:              time.Unix(0, 2001),
				LastHeartBeatUpdatedTime: time.Unix(0, 2000),
				Details:                  []byte("boom boom"),
				HasRetryPolicy:           true,
				Attempt:                  2002,
				MaximumAttempts:          2003,
				ExpirationTime:           time.Unix(0, 2004),
				LastFailureReason:        "failure reason",
				StartedIdentity:          "StartedWorkerIdentity",
				LastWorkerIdentity:       "LastWorkerIdentity",
				LastFailureDetails:       []byte("failure details"),
				ScheduledTime:            time.Unix(0, 1999),
			},
			activityScheduledEvent: &types.HistoryEvent{
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					ActivityType: &types.ActivityType{
						Name: "test-activity-type",
					},
				},
			},
			expected: &types.PendingActivityInfo{
				ActivityID:             "test-activity-id",
				ScheduleID:             123,
				State:                  types.PendingActivityStateStarted.Ptr(),
				HeartbeatDetails:       []byte("boom boom"),
				LastHeartbeatTimestamp: common.Int64Ptr(2000),
				LastStartedTimestamp:   common.Int64Ptr(2001),
				Attempt:                2002,
				MaximumAttempts:        2003,
				ExpirationTimestamp:    common.Int64Ptr(2004),
				LastFailureReason:      common.StringPtr("failure reason"),
				StartedWorkerIdentity:  "StartedWorkerIdentity",
				LastWorkerIdentity:     "LastWorkerIdentity",
				LastFailureDetails:     []byte("failure details"),
				ActivityType: &types.ActivityType{
					Name: "test-activity-type",
				},
			},
		},
		{
			name: "Success - scheduled activity without retry policy",
			activityInfo: &persistence.ActivityInfo{
				ActivityID:      "test-activity-id-2",
				ScheduleID:      125,
				StartedID:       constants.EmptyEventID,
				CancelRequested: false,
				ScheduledTime:   time.Unix(0, 1999),
				HasRetryPolicy:  false,
			},
			activityScheduledEvent: &types.HistoryEvent{
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					ActivityType: &types.ActivityType{
						Name: "test-activity-type-2",
					},
				},
			},
			expected: &types.PendingActivityInfo{
				ActivityID:         "test-activity-id-2",
				ScheduleID:         125,
				State:              types.PendingActivityStateScheduled.Ptr(),
				ScheduledTimestamp: common.Int64Ptr(1999),
				ActivityType: &types.ActivityType{
					Name: "test-activity-type-2",
				},
			},
		},
		{
			name: "Success - cancel requested activity",
			activityInfo: &persistence.ActivityInfo{
				ActivityID:      "test-activity-id-3",
				ScheduleID:      126,
				StartedID:       127,
				CancelRequested: true,
				StartedTime:     time.Unix(0, 2001),
				HasRetryPolicy:  false,
			},
			activityScheduledEvent: &types.HistoryEvent{
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					ActivityType: &types.ActivityType{
						Name: "test-activity-type-3",
					},
				},
			},
			expected: &types.PendingActivityInfo{
				ActivityID:           "test-activity-id-3",
				ScheduleID:           126,
				State:                types.PendingActivityStateCancelRequested.Ptr(),
				LastStartedTimestamp: common.Int64Ptr(2001),
				ActivityType: &types.ActivityType{
					Name: "test-activity-type-3",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mapPendingActivityInfo(tc.activityInfo, tc.activityScheduledEvent)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMapPendingChildExecutionInfo(t *testing.T) {
	testCases := []struct {
		name           string
		childExecution *persistence.ChildExecutionInfo
		domainEntry    *cache.DomainCacheEntry
		setupMock      func(*cache.MockDomainCache)
		expectError    bool
		expected       *types.PendingChildExecutionInfo
	}{
		{
			name: "Success - child execution with DomainID",
			childExecution: &persistence.ChildExecutionInfo{
				InitiatedID:       123,
				StartedWorkflowID: "child-workflow-id",
				StartedRunID:      "child-run-id",
				DomainID:          "child-domain-id",
				WorkflowTypeName:  "child-workflow-type",
				ParentClosePolicy: types.ParentClosePolicyAbandon,
			},
			domainEntry: &cache.DomainCacheEntry{},
			setupMock: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainName("child-domain-id").Return("child-domain-name", nil)
			},
			expectError: false,
			expected: &types.PendingChildExecutionInfo{
				Domain:            "child-domain-name",
				WorkflowID:        "child-workflow-id",
				RunID:             "child-run-id",
				WorkflowTypeName:  "child-workflow-type",
				InitiatedID:       123,
				ParentClosePolicy: types.ParentClosePolicyAbandon.Ptr(),
			},
		},
		{
			name: "Success - child execution with deprecated domain name",
			childExecution: &persistence.ChildExecutionInfo{
				InitiatedID:          456,
				StartedWorkflowID:    "child-workflow-id-2",
				StartedRunID:         "child-run-id-2",
				DomainID:             "", // Empty DomainID
				DomainNameDEPRECATED: "deprecated-domain-name",
				WorkflowTypeName:     "child-workflow-type-2",
				ParentClosePolicy:    types.ParentClosePolicyTerminate,
			},
			domainEntry: &cache.DomainCacheEntry{},
			setupMock: func(mockDomainCache *cache.MockDomainCache) {
				// No mock expectations needed - uses deprecated domain name directly
			},
			expectError: false,
			expected: &types.PendingChildExecutionInfo{
				Domain:            "deprecated-domain-name",
				WorkflowID:        "child-workflow-id-2",
				RunID:             "child-run-id-2",
				WorkflowTypeName:  "child-workflow-type-2",
				InitiatedID:       456,
				ParentClosePolicy: types.ParentClosePolicyTerminate.Ptr(),
			},
		},
		{
			name: "Success - child execution using parent domain (fallback)",
			childExecution: &persistence.ChildExecutionInfo{
				InitiatedID:          789,
				StartedWorkflowID:    "child-workflow-id-3",
				StartedRunID:         "child-run-id-3",
				DomainID:             "", // Empty DomainID
				DomainNameDEPRECATED: "", // Empty deprecated name
				WorkflowTypeName:     "child-workflow-type-3",
				ParentClosePolicy:    types.ParentClosePolicyRequestCancel,
			},
			domainEntry: cache.NewLocalDomainCacheEntryForTest(
				&persistence.DomainInfo{
					ID:   "parent-domain-id",
					Name: "parent-domain-fallback",
				},
				&persistence.DomainConfig{},
				"test-cluster",
			),
			setupMock: func(mockDomainCache *cache.MockDomainCache) {
				// No mock expectations needed - uses parent domain entry directly
			},
			expectError: false,
			expected: &types.PendingChildExecutionInfo{
				Domain:            "parent-domain-fallback",
				WorkflowID:        "child-workflow-id-3",
				RunID:             "child-run-id-3",
				WorkflowTypeName:  "child-workflow-type-3",
				InitiatedID:       789,
				ParentClosePolicy: types.ParentClosePolicyRequestCancel.Ptr(),
			},
		},
		{
			name: "Success - domain not exists error (uses domainID as fallback)",
			childExecution: &persistence.ChildExecutionInfo{
				InitiatedID:       999,
				StartedWorkflowID: "child-workflow-id-4",
				StartedRunID:      "child-run-id-4",
				DomainID:          "deleted-domain-id",
				WorkflowTypeName:  "child-workflow-type-4",
				ParentClosePolicy: types.ParentClosePolicyAbandon,
			},
			domainEntry: &cache.DomainCacheEntry{},
			setupMock: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainName("deleted-domain-id").Return("", &types.EntityNotExistsError{Message: "Domain not found"})
			},
			expectError: false,
			expected: &types.PendingChildExecutionInfo{
				Domain:            "deleted-domain-id", // Falls back to DomainID
				WorkflowID:        "child-workflow-id-4",
				RunID:             "child-run-id-4",
				WorkflowTypeName:  "child-workflow-type-4",
				InitiatedID:       999,
				ParentClosePolicy: types.ParentClosePolicyAbandon.Ptr(),
			},
		},
		{
			name: "Error - non-EntityNotExists error from domain cache",
			childExecution: &persistence.ChildExecutionInfo{
				InitiatedID:       111,
				StartedWorkflowID: "child-workflow-id-5",
				StartedRunID:      "child-run-id-5",
				DomainID:          "error-domain-id",
				WorkflowTypeName:  "child-workflow-type-5",
				ParentClosePolicy: types.ParentClosePolicyAbandon,
			},
			domainEntry: &cache.DomainCacheEntry{},
			setupMock: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainName("error-domain-id").Return("", &types.InternalServiceError{Message: "Internal error"})
			},
			expectError: true,
			expected:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDomainCache := cache.NewMockDomainCache(ctrl)
			tc.setupMock(mockDomainCache)

			result, err := mapPendingChildExecutionInfo(tc.childExecution, tc.domainEntry, mockDomainCache)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestMapPendingDecisionInfo(t *testing.T) {
	testCases := []struct {
		name         string
		decisionInfo *execution.DecisionInfo
		expected     *types.PendingDecisionInfo
	}{
		{
			name: "Success - scheduled decision (not started)",
			decisionInfo: &execution.DecisionInfo{
				Version:                    1,
				ScheduleID:                 100,
				StartedID:                  constants.EmptyEventID, // Not started
				RequestID:                  "request-id-1",
				DecisionTimeout:            30,
				TaskList:                   "decision-task-list",
				Attempt:                    1,
				ScheduledTimestamp:         1234567890,
				StartedTimestamp:           0, // Not started
				OriginalScheduledTimestamp: 1234567890,
			},
			expected: &types.PendingDecisionInfo{
				State:                      types.PendingDecisionStateScheduled.Ptr(),
				ScheduledTimestamp:         common.Int64Ptr(1234567890),
				Attempt:                    1,
				OriginalScheduledTimestamp: common.Int64Ptr(1234567890),
				ScheduleID:                 100,
				StartedTimestamp:           nil, // Should not be set for scheduled decision
			},
		},
		{
			name: "Success - started decision",
			decisionInfo: &execution.DecisionInfo{
				Version:                    2,
				ScheduleID:                 200,
				StartedID:                  201, // Started
				RequestID:                  "request-id-2",
				DecisionTimeout:            60,
				TaskList:                   "decision-task-list-2",
				Attempt:                    3,
				ScheduledTimestamp:         2345678901,
				StartedTimestamp:           2345678950,
				OriginalScheduledTimestamp: 2345678900,
			},
			expected: &types.PendingDecisionInfo{
				State:                      types.PendingDecisionStateStarted.Ptr(),
				ScheduledTimestamp:         common.Int64Ptr(2345678901),
				StartedTimestamp:           common.Int64Ptr(2345678950),
				Attempt:                    3,
				OriginalScheduledTimestamp: common.Int64Ptr(2345678900),
				ScheduleID:                 200,
			},
		},
		{
			name: "Success - decision with zero values",
			decisionInfo: &execution.DecisionInfo{
				Version:                    0,
				ScheduleID:                 0,
				StartedID:                  constants.EmptyEventID,
				RequestID:                  "",
				DecisionTimeout:            0,
				TaskList:                   "",
				Attempt:                    0,
				ScheduledTimestamp:         0,
				StartedTimestamp:           0,
				OriginalScheduledTimestamp: 0,
			},
			expected: &types.PendingDecisionInfo{
				State:                      types.PendingDecisionStateScheduled.Ptr(),
				ScheduledTimestamp:         common.Int64Ptr(0),
				Attempt:                    0,
				OriginalScheduledTimestamp: common.Int64Ptr(0),
				ScheduleID:                 0,
				StartedTimestamp:           nil,
			},
		},
		{
			name: "Success - decision with high attempt count",
			decisionInfo: &execution.DecisionInfo{
				Version:                    5,
				ScheduleID:                 500,
				StartedID:                  501,
				RequestID:                  "request-id-high-attempt",
				DecisionTimeout:            90,
				TaskList:                   "high-attempt-task-list",
				Attempt:                    99,
				ScheduledTimestamp:         3456789012,
				StartedTimestamp:           3456789100,
				OriginalScheduledTimestamp: 3456789000,
			},
			expected: &types.PendingDecisionInfo{
				State:                      types.PendingDecisionStateStarted.Ptr(),
				ScheduledTimestamp:         common.Int64Ptr(3456789012),
				StartedTimestamp:           common.Int64Ptr(3456789100),
				Attempt:                    99,
				OriginalScheduledTimestamp: common.Int64Ptr(3456789000),
				ScheduleID:                 500,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mapPendingDecisionInfo(tc.decisionInfo)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TODO: Test createDescribeWorkflowExecutionResponse - particularly the error cases
