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
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
)

func TestDescribeWorkflowExecution(t *testing.T) {
	eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)

	childDomainID := "deleted-domain"
	eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()
	eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainName(childDomainID).Return("", &types.EntityNotExistsError{}).AnyTimes()
	execution := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
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
		ExpirationTimestamp:    common.Int64Ptr(2004),
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
					DomainID:                 constants.TestDomainID,
					ActivityID:               activity1.ActivityID,
					Details:                  activity1.HeartbeatDetails,
					LastHeartBeatUpdatedTime: time.Unix(0, *activity1.LastHeartbeatTimestamp),
					Attempt:                  activity1.Attempt,
					StartedIdentity:          activity1.StartedWorkerIdentity,
					TaskList:                 taskList,
					HasRetryPolicy:           true,
					ExpirationTime:           time.Unix(0, *activity1.ExpirationTimestamp),
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
				DomainID:               constants.TestDomainID,
				WorkflowID:             constants.TestWorkflowID,
				RunID:                  constants.TestRunID,
				FirstExecutionRunID:    "first",
				ParentDomainID:         constants.TestDomainID,
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
		DomainUUID: constants.TestDomainID,
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
			ParentDomainID: &constants.TestDomainID,
			ParentDomain:   &constants.TestDomainName,
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
			IsCron:          isCron,
			UpdateTime:      common.Int64Ptr(lastUpdatedTime.UnixNano()),
			PartitionConfig: partitionConfig,
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

}
