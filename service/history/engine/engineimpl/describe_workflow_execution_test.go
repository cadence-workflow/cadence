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

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	historyConstants "github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

func TestDescribeWorkflowExecution(t *testing.T) {
	testDomainUUID := uuid.New()
	testCases := []struct {
		name                  string
		setupMocks            func(*execution.MockCache, *execution.MockContext, *execution.MockMutableState, *shard.MockContext, string)
		enableCorruptionCheck bool
		expectError           bool
		errorContains         string
	}{
		{
			name: "Success - happy path",
			setupMocks: func(mockExecutionCache *execution.MockCache, mockContext *execution.MockContext, mockMutableState *execution.MockMutableState, mockShard *shard.MockContext, domainUUID string) {
				wfExecution := types.WorkflowExecution{WorkflowID: "test-wf", RunID: "test-run"}
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), domainUUID, wfExecution).Return(mockContext, func(error) {}, nil)
				mockContext.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(mockMutableState, nil)

				// Mock for checkForHistoryCorruptions (GetDomainEntry() call)
				domainEntry := cache.NewLocalDomainCacheEntryForTest(
					&persistence.DomainInfo{Name: "test-domain-name"},
					&persistence.DomainConfig{},
					"test-cluster",
				)
				mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry)

				// Mock for createDescribeWorkflowExecutionResponse
				mockShard.EXPECT().GetDomainCache().Return(cache.NewMockDomainCache(gomock.NewController(t)))
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
				})
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(&types.HistoryEvent{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
					},
				}, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(10))
				mockMutableState.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{})
				mockMutableState.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{})
				mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry)
				mockMutableState.EXPECT().GetPendingDecision().Return(nil, false)
			},
			enableCorruptionCheck: false,
			expectError:           false,
		},
		{
			name: "Error - execution cache GetOrCreateWorkflowExecution fails",
			setupMocks: func(mockExecutionCache *execution.MockCache, mockContext *execution.MockContext, mockMutableState *execution.MockMutableState, mockShard *shard.MockContext, domainUUID string) {
				execution := types.WorkflowExecution{WorkflowID: "test-wf", RunID: "test-run"}
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), domainUUID, execution).Return(nil, nil, &types.InternalServiceError{Message: "Cache error"})
			},
			enableCorruptionCheck: false,
			expectError:           true,
			errorContains:         "Cache error",
		},
		{
			name: "Error - LoadWorkflowExecution fails",
			setupMocks: func(mockExecutionCache *execution.MockCache, mockContext *execution.MockContext, mockMutableState *execution.MockMutableState, mockShard *shard.MockContext, domainUUID string) {
				wfExecution := types.WorkflowExecution{WorkflowID: "test-wf", RunID: "test-run"}
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), domainUUID, wfExecution).Return(mockContext, func(error) {}, nil)
				mockContext.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(nil, &types.InternalServiceError{Message: "Load error"})
			},
			enableCorruptionCheck: false,
			expectError:           true,
			errorContains:         "Load error",
		},
		{
			name: "Error - createDescribeWorkflowExecutionResponse fails",
			setupMocks: func(mockExecutionCache *execution.MockCache, mockContext *execution.MockContext, mockMutableState *execution.MockMutableState, mockShard *shard.MockContext, domainUUID string) {
				wfExecution := types.WorkflowExecution{WorkflowID: "test-wf", RunID: "test-run"}
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), domainUUID, wfExecution).Return(mockContext, func(error) {}, nil)
				mockContext.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(mockMutableState, nil)

				// Mock for checkForHistoryCorruptions (GetDomainEntry() call)
				domainEntry := cache.NewLocalDomainCacheEntryForTest(
					&persistence.DomainInfo{Name: "test-domain-name"},
					&persistence.DomainConfig{},
					"test-cluster",
				)
				mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry)

				// Mock for createDescribeWorkflowExecutionResponse failure
				mockShard.EXPECT().GetDomainCache().Return(cache.NewMockDomainCache(gomock.NewController(t)))
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(nil, &types.InternalServiceError{Message: "Start event error"})
			},
			enableCorruptionCheck: false,
			expectError:           true,
			errorContains:         "Start event error",
		},
		{
			name: "Error - history corruption detected (missing start event)",
			setupMocks: func(mockExecutionCache *execution.MockCache, mockContext *execution.MockContext, mockMutableState *execution.MockMutableState, mockShard *shard.MockContext, domainUUID string) {
				wfExecution := types.WorkflowExecution{WorkflowID: "test-wf", RunID: "test-run"}
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), domainUUID, wfExecution).Return(mockContext, func(error) {}, nil)
				mockContext.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(mockMutableState, nil)

				// Mock for checkForHistoryCorruptions
				domainEntry := cache.NewLocalDomainCacheEntryForTest(
					&persistence.DomainInfo{Name: "test-domain-name"},
					&persistence.DomainConfig{},
					"test-cluster",
				)
				mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry)

				// This will trigger corruption detection
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(nil, execution.ErrMissingWorkflowStartEvent)

				// Mock GetExecutionInfo() call for corruption marking
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					WorkflowID:       "test-wf",
					RunID:            "test-run",
					WorkflowTypeName: "test-type",
				})
			},
			enableCorruptionCheck: true,
			expectError:           true,
			errorContains:         "Workflow execution corrupted.",
		},
		{
			name: "Error - history corruption check fails with non-start-event error",
			setupMocks: func(mockExecutionCache *execution.MockCache, mockContext *execution.MockContext, mockMutableState *execution.MockMutableState, mockShard *shard.MockContext, domainUUID string) {
				wfExecution := types.WorkflowExecution{WorkflowID: "test-wf", RunID: "test-run"}
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), domainUUID, wfExecution).Return(mockContext, func(error) {}, nil)
				mockContext.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(mockMutableState, nil)

				// Mock for checkForHistoryCorruptions
				domainEntry := cache.NewLocalDomainCacheEntryForTest(
					&persistence.DomainInfo{Name: "test-domain-name"},
					&persistence.DomainConfig{},
					"test-cluster",
				)
				mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry)

				// This will trigger corruption check failure with different error
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(nil, &types.InternalServiceError{Message: "Database error"})

				// Mock GetExecutionInfo() call for corruption marking
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					WorkflowID:       "test-wf",
					RunID:            "test-run",
					WorkflowTypeName: "test-type",
				})
			},
			enableCorruptionCheck: true,
			expectError:           true,
			errorContains:         "Database error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExecutionCache := execution.NewMockCache(ctrl)
			mockContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl)
			mockShard := shard.NewMockContext(ctrl)

			tc.setupMocks(mockExecutionCache, mockContext, mockMutableState, mockShard, testDomainUUID)

			// Set up config with explicit corruption check setting
			cfg := &config.Config{}
			cfg.EnableHistoryCorruptionCheck = func(domain string) bool {
				return tc.enableCorruptionCheck
			}

			engine := &historyEngineImpl{
				executionCache: mockExecutionCache,
				shard:          mockShard,
				config:         cfg,
				logger:         testlogger.New(t),
			}

			result, err := engine.DescribeWorkflowExecution(ctx.Background(), &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: testDomainUUID,
				Request: &types.DescribeWorkflowExecutionRequest{
					Domain:    "test-domain-name",
					Execution: &types.WorkflowExecution{WorkflowID: "test-wf", RunID: "test-run"},
				},
			})

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCreateDescribeWorkflowExecutionResponse(t *testing.T) {
	testCases := []struct {
		name          string
		setupMocks    func(*execution.MockMutableState, *cache.MockDomainCache)
		expectError   bool
		errorContains string
		verifyResult  func(*testing.T, *types.DescribeWorkflowExecutionResponse)
	}{
		{
			name: "Success - no pending activities, children, or decisions",
			setupMocks: func(mockMutableState *execution.MockMutableState, mockDomainCache *cache.MockDomainCache) {
				executionInfo := &persistence.WorkflowExecutionInfo{
					DomainID:                    "test-domain-id",
					WorkflowID:                  "test-workflow-id",
					RunID:                       "test-run-id",
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
					WorkflowTypeName:            "test-workflow-type",
					State:                       persistence.WorkflowStateRunning,
				}
				startEvent := &types.HistoryEvent{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
					},
				}

				mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo)
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(10))
				mockMutableState.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{})
				mockMutableState.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{})
				mockMutableState.EXPECT().GetDomainEntry().Return(&cache.DomainCacheEntry{})
				mockMutableState.EXPECT().GetPendingDecision().Return(nil, false)
			},
			expectError: false,
			verifyResult: func(t *testing.T, result *types.DescribeWorkflowExecutionResponse) {
				assert.NotNil(t, result.ExecutionConfiguration)
				assert.NotNil(t, result.WorkflowExecutionInfo)
				assert.Empty(t, result.PendingActivities)
				assert.Empty(t, result.PendingChildren)
				assert.Nil(t, result.PendingDecision)
			},
		},
		{
			name: "Error - GetStartEvent fails",
			setupMocks: func(mockMutableState *execution.MockMutableState, mockDomainCache *cache.MockDomainCache) {
				executionInfo := &persistence.WorkflowExecutionInfo{
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
				}

				mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo)
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(nil, &types.InternalServiceError{Message: "Failed to get start event"})
			},
			expectError:   true,
			errorContains: "Failed to get start event",
		},
		{
			name: "Error - GetCompletionEvent fails for completed workflow",
			setupMocks: func(mockMutableState *execution.MockMutableState, mockDomainCache *cache.MockDomainCache) {
				executionInfo := &persistence.WorkflowExecutionInfo{
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
					State:                       persistence.WorkflowStateCompleted,
				}
				startEvent := &types.HistoryEvent{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
					},
				}

				mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo)
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mockMutableState.EXPECT().GetCompletionEvent(gomock.Any()).Return(nil, &types.InternalServiceError{Message: "Failed to get completion event"})
			},
			expectError:   true,
			errorContains: "Failed to get completion event",
		},
		{
			name: "Error - mapWorkflowExecutionInfo fails due to domain cache error",
			setupMocks: func(mockMutableState *execution.MockMutableState, mockDomainCache *cache.MockDomainCache) {
				executionInfo := &persistence.WorkflowExecutionInfo{
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
					ParentDomainID:              "parent-domain-id",
					ParentWorkflowID:            "parent-workflow-id",
					ParentRunID:                 "parent-run-id",
					State:                       persistence.WorkflowStateRunning,
				}
				startEvent := &types.HistoryEvent{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
					},
				}

				mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo)
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(10))
				mockDomainCache.EXPECT().GetDomainName("parent-domain-id").Return("", &types.InternalServiceError{Message: "Domain cache error"})
			},
			expectError:   true,
			errorContains: "Domain cache error",
		},
		{
			name: "Error - GetActivityScheduledEvent fails",
			setupMocks: func(mockMutableState *execution.MockMutableState, mockDomainCache *cache.MockDomainCache) {
				executionInfo := &persistence.WorkflowExecutionInfo{
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
					State:                       persistence.WorkflowStateRunning,
				}
				startEvent := &types.HistoryEvent{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
					},
				}
				activityInfo := &persistence.ActivityInfo{
					ScheduleID: 123,
					ActivityID: "test-activity",
				}

				mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo)
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(10))
				mockMutableState.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{123: activityInfo})
				mockMutableState.EXPECT().GetActivityScheduledEvent(gomock.Any(), int64(123)).Return(nil, &types.InternalServiceError{Message: "Failed to get activity scheduled event"})
			},
			expectError:   true,
			errorContains: "Failed to get activity scheduled event",
		},
		{
			name: "Error - mapPendingChildExecutionInfo fails",
			setupMocks: func(mockMutableState *execution.MockMutableState, mockDomainCache *cache.MockDomainCache) {
				executionInfo := &persistence.WorkflowExecutionInfo{
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
					State:                       persistence.WorkflowStateRunning,
				}
				startEvent := &types.HistoryEvent{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
					},
				}
				childExecutionInfo := &persistence.ChildExecutionInfo{
					InitiatedID:       456,
					DomainID:          "child-domain-id",
					StartedWorkflowID: "child-workflow-id",
					StartedRunID:      "child-run-id",
					WorkflowTypeName:  "child-workflow-type",
				}
				domainEntry := &cache.DomainCacheEntry{}

				mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo)
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(10))
				mockMutableState.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{})
				mockMutableState.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{456: childExecutionInfo})
				mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry)
				mockDomainCache.EXPECT().GetDomainName("child-domain-id").Return("", &types.InternalServiceError{Message: "Child domain cache error"})
			},
			expectError:   true,
			errorContains: "Child domain cache error",
		},
		{
			name: "Success - with pending activities, children, and decision",
			setupMocks: func(mockMutableState *execution.MockMutableState, mockDomainCache *cache.MockDomainCache) {
				executionInfo := &persistence.WorkflowExecutionInfo{
					TaskList:                    "test-task-list",
					WorkflowTimeout:             1800,
					DecisionStartToCloseTimeout: 30,
					State:                       persistence.WorkflowStateRunning,
				}
				startEvent := &types.HistoryEvent{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Int32Ptr(5),
					},
				}
				activityInfo := &persistence.ActivityInfo{
					ScheduleID: 123,
					ActivityID: "test-activity",
				}
				scheduledEvent := &types.HistoryEvent{
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						ActivityType: &types.ActivityType{Name: "test-activity-type"},
					},
				}
				childExecutionInfo := &persistence.ChildExecutionInfo{
					InitiatedID:       456,
					DomainID:          "child-domain-id",
					StartedWorkflowID: "child-workflow-id",
					StartedRunID:      "child-run-id",
					WorkflowTypeName:  "child-workflow-type",
				}
				domainEntry := &cache.DomainCacheEntry{}
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 789,
					StartedID:  constants.EmptyEventID,
				}

				mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo)
				mockMutableState.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(10))
				mockMutableState.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{123: activityInfo})
				mockMutableState.EXPECT().GetActivityScheduledEvent(gomock.Any(), int64(123)).Return(scheduledEvent, nil)
				mockMutableState.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{456: childExecutionInfo})
				mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry)
				mockDomainCache.EXPECT().GetDomainName("child-domain-id").Return("child-domain-name", nil)
				mockMutableState.EXPECT().GetPendingDecision().Return(decisionInfo, true)
			},
			expectError: false,
			verifyResult: func(t *testing.T, result *types.DescribeWorkflowExecutionResponse) {
				assert.NotNil(t, result.ExecutionConfiguration)
				assert.NotNil(t, result.WorkflowExecutionInfo)
				assert.Len(t, result.PendingActivities, 1)
				assert.Len(t, result.PendingChildren, 1)
				assert.NotNil(t, result.PendingDecision)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockMutableState := execution.NewMockMutableState(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			tc.setupMocks(mockMutableState, mockDomainCache)

			result, err := createDescribeWorkflowExecutionResponse(ctx.Background(), mockMutableState, mockDomainCache)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tc.verifyResult != nil {
					tc.verifyResult(t, result)
				}
			}
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
				TaskListKind:                types.TaskListKindSticky,
				WorkflowTimeout:             1800,
				DecisionStartToCloseTimeout: 30,
			},
			expected: &types.WorkflowExecutionConfiguration{
				TaskList: &types.TaskList{
					Name: "test-task-list",
					Kind: types.TaskListKindSticky.Ptr(),
				},
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1800),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(30),
			},
		},
		{
			name:          "Success - zero values",
			executionInfo: &persistence.WorkflowExecutionInfo{},
			expected: &types.WorkflowExecutionConfiguration{
				TaskList:                            &types.TaskList{Name: "", Kind: types.TaskListKindNormal.Ptr()},
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

func TestValidateDescribeWorkflowExecutionRequest(t *testing.T) {
	testCases := []struct {
		name        string
		request     *types.HistoryDescribeWorkflowExecutionRequest
		expectError bool
	}{
		{
			name: "Success - valid UUID",
			request: &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: uuid.New(),
				Request: &types.DescribeWorkflowExecutionRequest{
					Domain: "test-domain",
					Execution: &types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Error - empty UUID",
			request: &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: "",
				Request: &types.DescribeWorkflowExecutionRequest{
					Domain: "test-domain",
					Execution: &types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
				},
			},
			expectError: true,
		},
		{
			name: "Error - invalid UUID format",
			request: &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: "not-a-valid-uuid",
				Request: &types.DescribeWorkflowExecutionRequest{
					Domain: "test-domain",
					Execution: &types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDescribeWorkflowExecutionRequest(tc.request)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
