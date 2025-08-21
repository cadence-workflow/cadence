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

package execution

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/shard"
)

var currentTime = time.Unix(0, 1)

func testMutableStateBuilder(t *testing.T) *mutableStateBuilder {
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t,
		ctrl,
		&persistence.ShardInfo{
			ShardID:          0,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest(),
	)
	mockShard.Resource.TimeSource = clock.NewMockedTimeSourceAt(currentTime)
	// set the checksum probabilities to 100% for exercising during test
	mockShard.GetConfig().MutableStateChecksumGenProbability = func(domain string) int { return 100 }
	mockShard.GetConfig().MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
	mockShard.GetConfig().EnableRetryForChecksumFailure = func(domain string) bool { return true }
	logger := log.NewNoop()

	mockShard.Resource.MatchingClient.EXPECT().AddActivityTask(gomock.Any(), gomock.Any()).Return(&types.AddActivityTaskResponse{}, nil).AnyTimes()
	mockShard.Resource.DomainCache.EXPECT().GetDomainID(constants.TestDomainName).Return(constants.TestDomainID, nil).AnyTimes()
	mockShard.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(&cache.DomainCacheEntry{}, nil).AnyTimes()
	return newMutableStateBuilder(mockShard, logger, constants.TestLocalDomainEntry)
}

func Test__AddActivityTaskScheduledEvent(t *testing.T) {
	activityType := &types.ActivityType{Name: "activityType"}
	retryPolicy := &types.RetryPolicy{
		InitialIntervalInSeconds:    11,
		BackoffCoefficient:          12,
		MaximumIntervalInSeconds:    13,
		MaximumAttempts:             14,
		NonRetriableErrorReasons:    []string{"no retries please"},
		ExpirationIntervalInSeconds: 15,
	}
	header := &types.Header{Fields: map[string][]byte{
		"key": []byte("value"),
	}}
	cases := []struct {
		name               string
		workflowTaskList   *types.TaskList
		attr               *types.ScheduleActivityTaskDecisionAttributes
		dispatch           bool
		expectedAttributes *types.ActivityTaskScheduledEventAttributes
		expectedInfo       *persistence.ActivityInfo
		expectedDispatch   *types.ActivityLocalDispatchInfo
		expectedDispatched bool
		expectedStarted    bool
		expectedError      error
	}{
		{
			name:             "success",
			workflowTaskList: &types.TaskList{Name: "taskList", Kind: types.TaskListKindNormal.Ptr()},
			attr: &types.ScheduleActivityTaskDecisionAttributes{
				ActivityID:                    "activityID",
				ActivityType:                  activityType,
				Domain:                        constants.TestDomainName,
				TaskList:                      &types.TaskList{Name: "taskList"},
				Input:                         []byte("input"),
				ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
				ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
				StartToCloseTimeoutSeconds:    common.Int32Ptr(3),
				HeartbeatTimeoutSeconds:       common.Int32Ptr(4),
				RetryPolicy:                   retryPolicy,
				Header:                        header,
				RequestLocalDispatch:          false,
			},
			expectedAttributes: &types.ActivityTaskScheduledEventAttributes{
				ActivityID:                    "activityID",
				ActivityType:                  activityType,
				Domain:                        &constants.TestDomainName,
				TaskList:                      &types.TaskList{Name: "taskList"},
				Input:                         []byte("input"),
				ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
				ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
				StartToCloseTimeoutSeconds:    common.Int32Ptr(3),
				HeartbeatTimeoutSeconds:       common.Int32Ptr(4),
				DecisionTaskCompletedEventID:  0,
				RetryPolicy:                   retryPolicy,
				Header:                        header,
			},
			expectedInfo: &persistence.ActivityInfo{
				Version:                commonconstants.EmptyVersion,
				ScheduleID:             1,
				ScheduledEventBatchID:  0,
				ScheduledTime:          currentTime,
				StartedID:              commonconstants.EmptyEventID,
				DomainID:               constants.TestDomainID,
				ActivityID:             "activityID",
				ScheduleToCloseTimeout: 1,
				ScheduleToStartTimeout: 2,
				StartToCloseTimeout:    3,
				HeartbeatTimeout:       4,
				CancelRequestID:        commonconstants.EmptyEventID,
				TaskList:               "taskList",
				TaskListKind:           types.TaskListKindNormal,
				HasRetryPolicy:         true,
				InitialInterval:        retryPolicy.InitialIntervalInSeconds,
				BackoffCoefficient:     retryPolicy.BackoffCoefficient,
				MaximumInterval:        retryPolicy.MaximumIntervalInSeconds,
				ExpirationTime:         currentTime.Add(time.Duration(retryPolicy.ExpirationIntervalInSeconds) * time.Second),
				MaximumAttempts:        retryPolicy.MaximumAttempts,
				NonRetriableErrors:     retryPolicy.NonRetriableErrorReasons,
			},
		},
		{
			name:             "success - normal workflow with ephemeral activity",
			workflowTaskList: &types.TaskList{Name: "taskList", Kind: types.TaskListKindNormal.Ptr()},
			attr: &types.ScheduleActivityTaskDecisionAttributes{
				ActivityID:                    "activityID",
				ActivityType:                  activityType,
				Domain:                        constants.TestDomainName,
				TaskList:                      &types.TaskList{Name: "taskList", Kind: types.TaskListKindEphemeral.Ptr()},
				Input:                         []byte("input"),
				ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
				ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
				StartToCloseTimeoutSeconds:    common.Int32Ptr(3),
				HeartbeatTimeoutSeconds:       common.Int32Ptr(4),
				RetryPolicy:                   retryPolicy,
				Header:                        header,
				RequestLocalDispatch:          false,
			},
			expectedAttributes: &types.ActivityTaskScheduledEventAttributes{
				ActivityID:                    "activityID",
				ActivityType:                  activityType,
				Domain:                        &constants.TestDomainName,
				TaskList:                      &types.TaskList{Name: "taskList", Kind: types.TaskListKindEphemeral.Ptr()},
				Input:                         []byte("input"),
				ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
				ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
				StartToCloseTimeoutSeconds:    common.Int32Ptr(3),
				HeartbeatTimeoutSeconds:       common.Int32Ptr(4),
				DecisionTaskCompletedEventID:  0,
				RetryPolicy:                   retryPolicy,
				Header:                        header,
			},
			expectedInfo: &persistence.ActivityInfo{
				Version:                commonconstants.EmptyVersion,
				ScheduleID:             1,
				ScheduledEventBatchID:  0,
				ScheduledTime:          currentTime,
				StartedID:              commonconstants.EmptyEventID,
				DomainID:               constants.TestDomainID,
				ActivityID:             "activityID",
				ScheduleToCloseTimeout: 1,
				ScheduleToStartTimeout: 2,
				StartToCloseTimeout:    3,
				HeartbeatTimeout:       4,
				CancelRequestID:        commonconstants.EmptyEventID,
				TaskList:               "taskList",
				TaskListKind:           types.TaskListKindEphemeral,
				HasRetryPolicy:         true,
				InitialInterval:        retryPolicy.InitialIntervalInSeconds,
				BackoffCoefficient:     retryPolicy.BackoffCoefficient,
				MaximumInterval:        retryPolicy.MaximumIntervalInSeconds,
				ExpirationTime:         currentTime.Add(time.Duration(retryPolicy.ExpirationIntervalInSeconds) * time.Second),
				MaximumAttempts:        retryPolicy.MaximumAttempts,
				NonRetriableErrors:     retryPolicy.NonRetriableErrorReasons,
			},
		},
		{
			name:             "success - ephemeral workflow dispatches activities as ephemeral",
			workflowTaskList: &types.TaskList{Name: "taskList", Kind: types.TaskListKindEphemeral.Ptr()},
			attr: &types.ScheduleActivityTaskDecisionAttributes{
				ActivityID:                    "activityID",
				ActivityType:                  activityType,
				Domain:                        constants.TestDomainName,
				TaskList:                      &types.TaskList{Name: "taskList", Kind: types.TaskListKindNormal.Ptr()},
				Input:                         []byte("input"),
				ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
				ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
				StartToCloseTimeoutSeconds:    common.Int32Ptr(3),
				HeartbeatTimeoutSeconds:       common.Int32Ptr(4),
				RetryPolicy:                   retryPolicy,
				Header:                        header,
				RequestLocalDispatch:          false,
			},
			expectedAttributes: &types.ActivityTaskScheduledEventAttributes{
				ActivityID:                    "activityID",
				ActivityType:                  activityType,
				Domain:                        &constants.TestDomainName,
				TaskList:                      &types.TaskList{Name: "taskList", Kind: types.TaskListKindEphemeral.Ptr()},
				Input:                         []byte("input"),
				ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
				ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
				StartToCloseTimeoutSeconds:    common.Int32Ptr(3),
				HeartbeatTimeoutSeconds:       common.Int32Ptr(4),
				DecisionTaskCompletedEventID:  0,
				RetryPolicy:                   retryPolicy,
				Header:                        header,
			},
			expectedInfo: &persistence.ActivityInfo{
				Version:                commonconstants.EmptyVersion,
				ScheduleID:             1,
				ScheduledEventBatchID:  0,
				ScheduledTime:          currentTime,
				StartedID:              commonconstants.EmptyEventID,
				DomainID:               constants.TestDomainID,
				ActivityID:             "activityID",
				ScheduleToCloseTimeout: 1,
				ScheduleToStartTimeout: 2,
				StartToCloseTimeout:    3,
				HeartbeatTimeout:       4,
				CancelRequestID:        commonconstants.EmptyEventID,
				TaskList:               "taskList",
				TaskListKind:           types.TaskListKindEphemeral,
				HasRetryPolicy:         true,
				InitialInterval:        retryPolicy.InitialIntervalInSeconds,
				BackoffCoefficient:     retryPolicy.BackoffCoefficient,
				MaximumInterval:        retryPolicy.MaximumIntervalInSeconds,
				ExpirationTime:         currentTime.Add(time.Duration(retryPolicy.ExpirationIntervalInSeconds) * time.Second),
				MaximumAttempts:        retryPolicy.MaximumAttempts,
				NonRetriableErrors:     retryPolicy.NonRetriableErrorReasons,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			mb := testMutableStateBuilder(t)
			mockEventsCache := mb.shard.GetEventsCache().(*events.MockCache)
			mockEventsCache.EXPECT().PutEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			if tc.workflowTaskList != nil {
				mb.executionInfo.TaskList = tc.workflowTaskList.Name
				mb.executionInfo.TaskListKind = tc.workflowTaskList.GetKind()
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			event, activityInfo, dispatchInfo, dispatched, started, err := mb.AddActivityTaskScheduledEvent(ctx, 0, tc.attr, tc.dispatch)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, event)
				assert.Nil(t, activityInfo)
				assert.Nil(t, dispatchInfo)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, types.EventTypeActivityTaskScheduled.Ptr(), event.EventType)
				assert.Equal(t, tc.expectedAttributes, event.ActivityTaskScheduledEventAttributes)
				assert.Equal(t, tc.expectedInfo, activityInfo)
				assert.Equal(t, tc.expectedDispatch, dispatchInfo)
			}
			assert.Equal(t, tc.expectedDispatched, dispatched)
			assert.Equal(t, tc.expectedStarted, started)
		})
	}
}

func Test__UpdateActivityProgress(t *testing.T) {
	mb := testMutableStateBuilder(t)
	ai := &persistence.ActivityInfo{
		Version:    1,
		ScheduleID: 1,
	}
	request := &types.RecordActivityTaskHeartbeatRequest{
		TaskToken: nil,
		Details:   []byte{10, 0},
		Identity:  "",
	}
	assert.Equal(t, int64(1), ai.Version)
	mb.UpdateActivityProgress(ai, request)
	assert.Equal(t, commonconstants.EmptyVersion, ai.Version)
	assert.Equal(t, request.Details, ai.Details)
	assert.Equal(t, ai, mb.updateActivityInfos[ai.ScheduleID])
	assert.NotNil(t, mb.syncActivityTasks[ai.ScheduleID])
}

func Test__ReplicateActivityInfo(t *testing.T) {
	mb := testMutableStateBuilder(t)
	now := time.Now()
	nowUnix := now.UnixNano()
	request := &types.SyncActivityRequest{
		ScheduledID:       1,
		Version:           1,
		ScheduledTime:     &nowUnix,
		LastHeartbeatTime: &nowUnix,
	}
	ai := &persistence.ActivityInfo{}

	err := mb.ReplicateActivityInfo(request, true)
	assert.Error(t, err)
	assert.Equal(t, ErrMissingActivityInfo, err)

	mb.pendingActivityInfoIDs[request.ScheduledID] = ai
	err = mb.ReplicateActivityInfo(request, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), ai.Version)
	assert.Equal(t, now.UTC(), ai.ScheduledTime.UTC())
	assert.Equal(t, request.StartedID, ai.StartedID)
	assert.Equal(t, now.UTC(), ai.LastHeartBeatUpdatedTime.UTC())
}

func Test__UpdateActivity(t *testing.T) {
	mb := testMutableStateBuilder(t)
	ai := &persistence.ActivityInfo{ScheduleID: 1}
	t.Run("error missing activity info", func(t *testing.T) {
		err := mb.UpdateActivity(ai)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingActivityInfo, err)
	})
	t.Run("update success", func(t *testing.T) {
		mb.pendingActivityInfoIDs[1] = ai
		err := mb.UpdateActivity(ai)
		assert.NoError(t, err)
		assert.Equal(t, ai, mb.updateActivityInfos[1])
	})
}

func Test__GetActivityScheduledEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	ai := &persistence.ActivityInfo{
		ScheduleID:     1,
		ScheduledEvent: &types.HistoryEvent{},
	}
	t.Run("error missing activity info", func(t *testing.T) {
		_, err := mb.GetActivityScheduledEvent(context.Background(), ai.ScheduleID)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingActivityInfo, err)
	})
	t.Run("scheduled event from activity info", func(t *testing.T) {
		mb.pendingActivityInfoIDs[1] = ai
		result, err := mb.GetActivityScheduledEvent(context.Background(), ai.ScheduleID)
		assert.NoError(t, err)
		assert.Equal(t, ai.ScheduledEvent, result)
	})
	t.Run("scheduled event from events cache", func(t *testing.T) {
		mockEventsCache := mb.shard.GetEventsCache().(*events.MockCache)
		mockEventsCache.EXPECT().GetEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&types.HistoryEvent{}, nil)
		mb.pendingActivityInfoIDs[1] = &persistence.ActivityInfo{
			ScheduleID: 1,
		}
		result, err := mb.GetActivityScheduledEvent(context.Background(), ai.ScheduleID)
		assert.NoError(t, err)
		assert.Equal(t, &types.HistoryEvent{}, result)
	})
	t.Run("error missing scheduled event", func(t *testing.T) {
		mockEventsCache := mb.shard.GetEventsCache().(*events.MockCache)
		mockEventsCache.EXPECT().GetEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
		mb.pendingActivityInfoIDs[1] = &persistence.ActivityInfo{
			ScheduleID: 1,
		}
		_, err := mb.GetActivityScheduledEvent(context.Background(), ai.ScheduleID)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingActivityScheduledEvent, err)
	})
}

func Test__AddActivityTaskCompletedEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	request := &types.RespondActivityTaskCompletedRequest{
		TaskToken: nil,
		Result:    nil,
		Identity:  "",
	}
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, err := mbCompleted.AddActivityTaskCompletedEvent(1, 1, request)
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("error getting activity info", func(t *testing.T) {
		_, err := mb.AddActivityTaskCompletedEvent(1, 1, request)
		assert.Error(t, err)
		assert.Equal(t, "add-activitytask-completed-event operation failed", err.Error())
	})
	t.Run("success", func(t *testing.T) {
		ai := &persistence.ActivityInfo{
			ScheduleID:     1,
			ActivityID:     "1",
			ScheduledEvent: &types.HistoryEvent{},
			StartedID:      1,
		}
		mb.pendingActivityInfoIDs[1] = ai
		mb.pendingActivityIDToEventID["1"] = 1
		mb.updateActivityInfos[1] = ai
		mb.hBuilder = NewHistoryBuilder(mb)
		event, err := mb.AddActivityTaskCompletedEvent(1, 1, request)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), event.ActivityTaskCompletedEventAttributes.ScheduledEventID)
	})
}

func Test__tryDispatchActivityTask(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{}
	ai := &persistence.ActivityInfo{}
	result := mb.tryDispatchActivityTask(context.Background(), event, ai)
	assert.True(t, result)
}

func Test__ReplicateActivityTaskCanceledEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{
		EventType: types.EventTypeActivityTaskCanceled.Ptr(),
		ActivityTaskCanceledEventAttributes: &types.ActivityTaskCanceledEventAttributes{
			ScheduledEventID: 1,
		},
	}
	ai := &persistence.ActivityInfo{
		ActivityID: "1",
	}
	mb.pendingActivityInfoIDs[1] = ai
	mb.pendingActivityIDToEventID["1"] = 1
	err := mb.ReplicateActivityTaskCanceledEvent(event)
	assert.NoError(t, err)
	_, ok := mb.pendingActivityInfoIDs[1]
	assert.False(t, ok)
	_, ok = mb.pendingActivityIDToEventID["1"]
	assert.False(t, ok)
}

func Test__ReplicateActivityTaskCancelRequestedEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{
		EventType: types.EventTypeActivityTaskCanceled.Ptr(),
		ActivityTaskCancelRequestedEventAttributes: &types.ActivityTaskCancelRequestedEventAttributes{
			ActivityID: "1",
		},
	}
	ai := &persistence.ActivityInfo{
		ActivityID: "1",
		ScheduleID: 1,
	}
	mb.pendingActivityInfoIDs[1] = ai
	mb.pendingActivityIDToEventID["1"] = 1
	err := mb.ReplicateActivityTaskCancelRequestedEvent(event)
	assert.NoError(t, err)
	assert.Equal(t, ai, mb.updateActivityInfos[1])
}

func Test__ReplicateActivityTaskTimedOutEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{
		EventType: types.EventTypeActivityTaskTimedOut.Ptr(),
		ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
			ScheduledEventID: 1,
		},
	}
	ai := &persistence.ActivityInfo{
		ActivityID: "1",
	}
	mb.pendingActivityInfoIDs[1] = ai
	mb.pendingActivityIDToEventID["1"] = 1
	err := mb.ReplicateActivityTaskTimedOutEvent(event)
	assert.NoError(t, err)
	_, ok := mb.pendingActivityInfoIDs[1]
	assert.False(t, ok)
	_, ok = mb.pendingActivityIDToEventID["1"]
	assert.False(t, ok)
}

func Test__ReplicateActivityTaskFailedEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{
		EventType: types.EventTypeActivityTaskFailed.Ptr(),
		ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
			ScheduledEventID: 1,
		},
	}
	ai := &persistence.ActivityInfo{
		ActivityID: "1",
	}
	mb.pendingActivityInfoIDs[1] = ai
	mb.pendingActivityIDToEventID["1"] = 1
	err := mb.ReplicateActivityTaskFailedEvent(event)
	assert.NoError(t, err)
	_, ok := mb.pendingActivityInfoIDs[1]
	assert.False(t, ok)
	_, ok = mb.pendingActivityIDToEventID["1"]
	assert.False(t, ok)
}

func Test__ReplicateActivityTaskCompletedEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{
		EventType: types.EventTypeActivityTaskCompleted.Ptr(),
		ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
			ScheduledEventID: 1,
		},
	}
	ai := &persistence.ActivityInfo{
		ActivityID: "1",
	}
	mb.pendingActivityInfoIDs[1] = ai
	mb.pendingActivityIDToEventID["1"] = 1
	err := mb.ReplicateActivityTaskCompletedEvent(event)
	assert.NoError(t, err)
	_, ok := mb.pendingActivityInfoIDs[1]
	assert.False(t, ok)
	_, ok = mb.pendingActivityIDToEventID["1"]
	assert.False(t, ok)
}

func Test__AddActivityTaskCanceledEvent(t *testing.T) {
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, err := mbCompleted.AddActivityTaskCanceledEvent(1, 1, 1, []byte{10}, "test")
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("error getting activity info", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		_, err := mb.AddActivityTaskCanceledEvent(1, 1, 1, []byte{10}, "test")
		assert.Error(t, err)
		assert.Equal(t, "add-activitytask-canceled-event operation failed", err.Error())
	})
	t.Run("error cancel not requested", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.hBuilder = NewHistoryBuilder(mb)
		ai := &persistence.ActivityInfo{
			StartedID:       1,
			CancelRequested: false,
			StartedTime:     time.Now(),
		}
		mb.pendingActivityInfoIDs[1] = ai
		_, err := mb.AddActivityTaskCanceledEvent(1, 1, 1, []byte{10}, "test")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.hBuilder = NewHistoryBuilder(mb)
		ai := &persistence.ActivityInfo{
			StartedID:       1,
			CancelRequested: true,
			StartedTime:     time.Now(),
		}
		mb.pendingActivityInfoIDs[1] = ai
		event, err := mb.AddActivityTaskCanceledEvent(1, 1, 1, []byte{10}, "test")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), event.ActivityTaskCanceledEventAttributes.ScheduledEventID)
		assert.Equal(t, "test", event.ActivityTaskCanceledEventAttributes.Identity)
	})
}

func Test__AddRequestCancelActivityTaskFailedEvent(t *testing.T) {
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, err := mbCompleted.AddRequestCancelActivityTaskFailedEvent(1, "1", "test")
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("success", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.hBuilder = NewHistoryBuilder(mb)
		event, err := mb.AddRequestCancelActivityTaskFailedEvent(1, "1", "test")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), event.RequestCancelActivityTaskFailedEventAttributes.DecisionTaskCompletedEventID)
		assert.Equal(t, "test", event.RequestCancelActivityTaskFailedEventAttributes.Cause)
	})
}

func Test__AddActivityTaskCancelRequestedEvent(t *testing.T) {
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, _, err := mbCompleted.AddActivityTaskCancelRequestedEvent(1, "1", "test")
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("error getting activity info", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.hBuilder = NewHistoryBuilder(mb)
		_, _, err := mb.AddActivityTaskCancelRequestedEvent(1, "1", "test")
		assert.Error(t, err)
		assert.Equal(t, "invalid history builder state for action: add-activitytask-cancel-requested-event", err.Error())
	})
	t.Run("success", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.hBuilder = NewHistoryBuilder(mb)
		ai := &persistence.ActivityInfo{
			StartedID:   1,
			StartedTime: time.Now(),
		}
		mb.pendingActivityInfoIDs[1] = ai
		mb.pendingActivityIDToEventID["1"] = 1
		event, ai, err := mb.AddActivityTaskCancelRequestedEvent(1, "1", "test")
		assert.NoError(t, err)
		assert.Equal(t, "1", event.ActivityTaskCancelRequestedEventAttributes.ActivityID)
	})
}

func Test__AddActivityTaskTimedOutEvent(t *testing.T) {
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, err := mbCompleted.AddActivityTaskTimedOutEvent(1, 1, types.TimeoutTypeHeartbeat, []byte{10})
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("error getting activity info", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		_, err := mb.AddActivityTaskTimedOutEvent(1, 1, types.TimeoutTypeHeartbeat, []byte{10})
		assert.Error(t, err)
		assert.Equal(t, "add-activitytask-timed-event operation failed", err.Error())
	})
	t.Run("success", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		ai := &persistence.ActivityInfo{
			ScheduleID:     1,
			ActivityID:     "1",
			ScheduledEvent: &types.HistoryEvent{},
			StartedID:      1,
		}
		mb.pendingActivityInfoIDs[1] = ai
		mb.hBuilder = NewHistoryBuilder(mb)
		event, err := mb.AddActivityTaskTimedOutEvent(1, 1, types.TimeoutTypeHeartbeat, []byte{10})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), event.ActivityTaskTimedOutEventAttributes.ScheduledEventID)
	})
}

func Test__AddActivityTaskFailedEvent(t *testing.T) {
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, err := mbCompleted.AddActivityTaskFailedEvent(1, 1, &types.RespondActivityTaskFailedRequest{})
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("error getting activity info", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		_, err := mb.AddActivityTaskFailedEvent(1, 1, &types.RespondActivityTaskFailedRequest{})
		assert.Error(t, err)
		assert.Equal(t, "add-activitytask-failed-event operation failed", err.Error())
	})
	t.Run("success", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		ai := &persistence.ActivityInfo{
			ScheduleID:     1,
			ActivityID:     "1",
			ScheduledEvent: &types.HistoryEvent{},
			StartedID:      1,
		}
		mb.pendingActivityInfoIDs[1] = ai
		mb.hBuilder = NewHistoryBuilder(mb)
		event, err := mb.AddActivityTaskFailedEvent(1, 1, &types.RespondActivityTaskFailedRequest{
			Identity: "test",
			Details:  make([]byte, 10),
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), event.ActivityTaskFailedEventAttributes.ScheduledEventID)

	})
}
