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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	historyConstants "github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a simple mock that returns parent-domain-name for any domain ID
			mockDomainCache := &testDomainCache{}

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

// TODO: Use a real domain cache mock if it exists elsewhere
// Simple test mock for DomainCache
type testDomainCache struct{}

func (t *testDomainCache) GetDomainName(domainID string) (string, error) {
	return "parent-domain-name", nil
}

func (t *testDomainCache) GetDomainID(domainName string) (string, error) {
	return "test-domain-id", nil
}

func (t *testDomainCache) GetDomain(domainName string) (*cache.DomainCacheEntry, error) {
	return nil, nil
}

func (t *testDomainCache) GetDomainByID(domainID string) (*cache.DomainCacheEntry, error) {
	return nil, nil
}

func (t *testDomainCache) GetAllDomain() map[string]*cache.DomainCacheEntry {
	return nil
}

func (t *testDomainCache) GetCacheSize() (sizeOfCacheByName int64, sizeOfCacheByID int64) {
	return 0, 0
}

func (t *testDomainCache) RegisterDomainChangeCallback(id string, catchUpFn cache.CatchUpFn, prepareCallback cache.PrepareCallbackFn, callback cache.CallbackFn) {
	// No-op for testing
}

func (t *testDomainCache) UnregisterDomainChangeCallback(id string) {
	// No-op for testing
}

func (t *testDomainCache) Start() {
	// No-op for testing
}

func (t *testDomainCache) Stop() {
	// No-op for testing
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

// TODO: Test mapPendingChildExecutionInfo and mapPendingDecisionInfo
// TODO: Test createDescribeWorkflowExecutionResponse - particularly the error cases
