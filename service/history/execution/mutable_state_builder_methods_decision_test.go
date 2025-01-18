// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
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
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

func CreateDecisionInfo() *DecisionInfo {
	return &DecisionInfo{
		Version:                    123,
		ScheduleID:                 123,
		StartedID:                  123,
		RequestID:                  "123",
		DecisionTimeout:            123,
		StartedTimestamp:           123,
		ScheduledTimestamp:         123,
		OriginalScheduledTimestamp: 123,
	}
}

func TestGetDecisionInfoMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	scheduleEventID := int64(123)
	rets := CreateDecisionInfo()

	decisionTaskManager.EXPECT().GetDecisionInfo(scheduleEventID).Return(rets, true).Times(1)

	decisionInfo, ok := builder.GetDecisionInfo(scheduleEventID)

	assert.Equal(t, rets, decisionInfo)
	assert.True(t, ok)
}

func TestUpdateDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decision := CreateDecisionInfo()

	decisionTaskManager.EXPECT().UpdateDecision(decision).Times(1)

	builder.UpdateDecision(decision)
}

func TestDeleteDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decisionTaskManager.EXPECT().DeleteDecision().Times(1)

	builder.DeleteDecision()
}

func TestFailDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decisionTaskManager.EXPECT().FailDecision(true).Times(1)

	builder.FailDecision(true)
}

func TestHasProcessedOrPendingDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decisionTaskManager.EXPECT().HasProcessedOrPendingDecision().Return(true).Times(1)

	hasProcessedOrPendingDecision := builder.HasProcessedOrPendingDecision()

	assert.True(t, hasProcessedOrPendingDecision)
}

func TestHasPendingDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decisionTaskManager.EXPECT().HasPendingDecision().Return(true).Times(1)

	hasPendingDecision := builder.HasPendingDecision()

	assert.True(t, hasPendingDecision)
}

func TestGetPendingDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	rets := CreateDecisionInfo()

	decisionTaskManager.EXPECT().GetPendingDecision().Return(rets, true).Times(1)

	pendingDecision, ok := builder.GetPendingDecision()

	assert.Equal(t, rets, pendingDecision)
	assert.True(t, ok)
}

func TestHasInFlightDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decisionTaskManager.EXPECT().HasInFlightDecision().Return(true).Times(1)

	hasInFlightDecision := builder.HasInFlightDecision()

	assert.True(t, hasInFlightDecision)
}

func TestGetInFlightDecisionMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	rets := CreateDecisionInfo()

	decisionTaskManager.EXPECT().GetInFlightDecision().Return(rets, true).Times(1)

	inFlightDecision, ok := builder.GetInFlightDecision()

	assert.Equal(t, rets, inFlightDecision)
	assert.True(t, ok)
}

func TestGetDecisionScheduleToStartTimeoutMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	rets := time.Duration(123)

	decisionTaskManager.EXPECT().GetDecisionScheduleToStartTimeout().Return(rets).Times(1)

	timeout := builder.GetDecisionScheduleToStartTimeout()

	assert.Equal(t, rets, timeout)
}

func TestAddFirstDecisionTaskScheduledMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			startEvent := &types.HistoryEvent{}

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddFirstDecisionTaskScheduled(startEvent).Return(nil).Times(1)
			}

			err := builder.AddFirstDecisionTaskScheduled(startEvent)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddDecisionTaskScheduledEventMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			rets := CreateDecisionInfo()

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskScheduledEvent(true).Return(rets, nil).Times(1)
			}

			decisionInfo, err := builder.AddDecisionTaskScheduledEvent(true)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets, decisionInfo)
			}
		})
	}
}

func TestAddDecisionTaskScheduledEventAsHeartbeatMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			rets := CreateDecisionInfo()
			originalScheduledTimestamp := int64(1)

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskScheduledEventAsHeartbeat(true, originalScheduledTimestamp).Return(rets, nil).Times(1)
			}

			decisionInfo, err := builder.AddDecisionTaskScheduledEventAsHeartbeat(true, originalScheduledTimestamp)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets, decisionInfo)
			}
		})
	}
}

func TestReplicateTransientDecisionTaskScheduledMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decisionTaskManager.EXPECT().ReplicateTransientDecisionTaskScheduled().Return(nil).Times(1)

	err := builder.ReplicateTransientDecisionTaskScheduled()

	assert.NoError(t, err)
}

func TestReplicateDecisionTaskScheduledEventMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	var version, scheduleID, attempt, scheduleTimestamp, originalScheduledTimestamp int64 = 1, 2, 3, 4, 5
	startToCloseTimeoutSeconds := int32(6)
	taskList := "taskList"
	rets := CreateDecisionInfo()

	decisionTaskManager.EXPECT().ReplicateDecisionTaskScheduledEvent(version, scheduleID, taskList, startToCloseTimeoutSeconds, attempt, scheduleTimestamp, originalScheduledTimestamp, true).Return(rets, nil).Times(1)

	decisionInfo, err := builder.ReplicateDecisionTaskScheduledEvent(version, scheduleID, taskList, startToCloseTimeoutSeconds, attempt, scheduleTimestamp, originalScheduledTimestamp, true)

	assert.NoError(t, err)
	assert.Equal(t, rets, decisionInfo)
}

func TestAddDecisionTaskStartedEventMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		request       *types.PollForDecisionTaskRequest
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name:    "success",
			request: &types.PollForDecisionTaskRequest{},
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name:    "error due to state",
			request: &types.PollForDecisionTaskRequest{},
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			scheduleEventID := int64(123)
			requestID := "requestID"
			rets0 := &types.HistoryEvent{}
			rets1 := CreateDecisionInfo()

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskStartedEvent(scheduleEventID, requestID, tc.request).Return(rets0, rets1, nil).Times(1)
			}

			historyEvent, decisionInfo, err := builder.AddDecisionTaskStartedEvent(scheduleEventID, requestID, tc.request)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets0, historyEvent)
				assert.Equal(t, rets1, decisionInfo)
			}
		})
	}
}

func TestReplicateDecisionTaskStartedEventMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	var version, scheduleID, startedID, timestamp int64 = 1, 2, 3, 4
	requestID := "requestID"
	rets := CreateDecisionInfo()

	decisionTaskManager.EXPECT().ReplicateDecisionTaskStartedEvent(rets, version, scheduleID, startedID, requestID, timestamp).Return(rets, nil).Times(1)

	decisionInfo, err := builder.ReplicateDecisionTaskStartedEvent(rets, version, scheduleID, startedID, requestID, timestamp)

	assert.NoError(t, err)
	assert.Equal(t, rets, decisionInfo)
}

func TestCreateTransientDecisionEventsMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	decision := CreateDecisionInfo()
	identity := "identity"
	rets := &types.HistoryEvent{}

	decisionTaskManager.EXPECT().CreateTransientDecisionEvents(decision, identity).Return(rets, rets).Times(1)

	historyEvent0, historyEvent1 := builder.CreateTransientDecisionEvents(decision, identity)

	assert.Equal(t, rets, historyEvent0)
	assert.Equal(t, rets, historyEvent1)
}

func TestCheckResettableMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name                         string
		pendingChildExecutionInfoIDs map[int64]*persistence.ChildExecutionInfo
		pendingRequestCancelInfoIDs  map[int64]*persistence.RequestCancelInfo
		pendingSignalInfoIDs         map[int64]*persistence.SignalInfo
		wantErr                      bool
		errMessage                   string
	}{
		{
			name: "success",
		},
		{
			name: "error due to pending child execution",
			pendingChildExecutionInfoIDs: map[int64]*persistence.ChildExecutionInfo{
				1: {},
			},
			wantErr:    true,
			errMessage: "it is not allowed resetting to a point that workflow has pending child types.",
		},
		{
			name: "error due to pending request cancel external",
			pendingRequestCancelInfoIDs: map[int64]*persistence.RequestCancelInfo{
				1: {},
			},
			wantErr:    true,
			errMessage: "it is not allowed resetting to a point that workflow has pending request cancel.",
		},
		{
			name: "error due to pending signal external",
			pendingSignalInfoIDs: map[int64]*persistence.SignalInfo{
				1: {},
			},
			wantErr:    true,
			errMessage: "it is not allowed resetting to a point that workflow has pending signals to send.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := &mutableStateBuilder{
				pendingChildExecutionInfoIDs: tc.pendingChildExecutionInfoIDs,
				pendingRequestCancelInfoIDs:  tc.pendingRequestCancelInfoIDs,
				pendingSignalInfoIDs:         tc.pendingSignalInfoIDs,
			}

			err := builder.CheckResettable()

			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.errMessage, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}

}

func TestAddDecisionTaskCompletedEventMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			var scheduleEventID, startedEventID int64 = 123, 234
			request := &types.RespondDecisionTaskCompletedRequest{}
			maxResetPoints := 1
			rets := &types.HistoryEvent{}

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskCompletedEvent(scheduleEventID, startedEventID, request, maxResetPoints).Return(rets, nil).Times(1)
			}

			historyEvent, err := builder.AddDecisionTaskCompletedEvent(scheduleEventID, startedEventID, request, maxResetPoints)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets, historyEvent)
			}
		})
	}
}

func TestReplicateDecisionTaskCompletedEventMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	event := &types.HistoryEvent{}

	decisionTaskManager.EXPECT().ReplicateDecisionTaskCompletedEvent(event).Return(nil).Times(1)

	err := builder.ReplicateDecisionTaskCompletedEvent(event)

	assert.NoError(t, err)
}

func TestAddDecisionTaskTimedOutEventMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			var scheduleEventID, startedEventID int64 = 123, 234
			rets := &types.HistoryEvent{}

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskTimedOutEvent(scheduleEventID, startedEventID).Return(rets, nil).Times(1)
			}

			historyEvent, err := builder.AddDecisionTaskTimedOutEvent(scheduleEventID, startedEventID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets, historyEvent)
			}
		})
	}
}

func TestReplicateDecisionTaskTimedOutEventMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	event := &types.HistoryEvent{}

	decisionTaskManager.EXPECT().ReplicateDecisionTaskTimedOutEvent(event).Return(nil).Times(1)

	err := builder.ReplicateDecisionTaskTimedOutEvent(event)

	assert.NoError(t, err)
}

func TestAddDecisionTaskScheduleToStartTimeoutEventMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			scheduleEventID := int64(123)
			rets := &types.HistoryEvent{}

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskScheduleToStartTimeoutEvent(scheduleEventID).Return(rets, nil).Times(1)
			}

			historyEvent, err := builder.AddDecisionTaskScheduleToStartTimeoutEvent(scheduleEventID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets, historyEvent)
			}
		})
	}
}

func TestAddDecisionTaskResetTimeoutEventMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			var scheduleEventID, forkEventVersion int64 = 123, 1
			baserRunID, newRunID, reason, resetRequestID := "baseRunID", "newRunID", "reason", "resetRequestID"
			rets := &types.HistoryEvent{}

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskResetTimeoutEvent(scheduleEventID, baserRunID, newRunID, forkEventVersion, reason, resetRequestID).Return(rets, nil).Times(1)
			}

			historyEvent, err := builder.AddDecisionTaskResetTimeoutEvent(scheduleEventID, baserRunID, newRunID, forkEventVersion, reason, resetRequestID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets, historyEvent)
			}
		})
	}
}

func TestAddDecisionTaskFailedEventMutableStateBuilder(t *testing.T) {
	tests := []struct {
		name          string
		executionInfo *persistence.WorkflowExecutionInfo
		wantErr       bool
	}{
		{
			name: "success",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCreated,
			},
		},
		{
			name: "error due to state",
			executionInfo: &persistence.WorkflowExecutionInfo{
				State: persistence.WorkflowStateCompleted,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
			builder := &mutableStateBuilder{
				decisionTaskManager: decisionTaskManager,
				executionInfo:       tc.executionInfo,
				logger:              loggerimpl.NewNopLogger(),
			}

			var scheduleEventID, startedEventID, forkEventVersion int64 = 123, 234, 1
			cause := types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
			details := []byte("details")
			identity, reason, binChecksum, baseRunID, newRunID, resetRequestID := "identity", "reason", "checksum", "baseRunID", "newRunID", "resetRequestID"
			rets := &types.HistoryEvent{}

			if !tc.wantErr {
				decisionTaskManager.EXPECT().AddDecisionTaskFailedEvent(scheduleEventID, startedEventID, cause, details, identity, reason, binChecksum, baseRunID, newRunID, forkEventVersion, resetRequestID).Return(rets, nil).Times(1)
			}

			historyEvent, err := builder.AddDecisionTaskFailedEvent(scheduleEventID, startedEventID, cause, details, identity, reason, binChecksum, baseRunID, newRunID, forkEventVersion, resetRequestID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, rets, historyEvent)
			}
		})
	}
}

func TestReplicateDecisionTaskFailedEventMutableStateBuilder(t *testing.T) {
	decisionTaskManager := NewMockmutableStateDecisionTaskManager(gomock.NewController(t))
	builder := &mutableStateBuilder{
		decisionTaskManager: decisionTaskManager,
	}

	event := &types.HistoryEvent{}

	decisionTaskManager.EXPECT().ReplicateDecisionTaskFailedEvent(event).Return(nil).Times(1)

	err := builder.ReplicateDecisionTaskFailedEvent(event)

	assert.NoError(t, err)
}

func TestAddBinaryCheckSumIfNotExistsMutableStateBuilder(t *testing.T) {
	timeNow := time.Now()
	runID := "runID"
	checkSum := "checkSum"
	eventID := int64(1)

	tests := []struct {
		name                                 string
		decisionTaskCompletedEventAttributes *types.DecisionTaskCompletedEventAttributes
		autoResetPoints                      *types.ResetPoints
		shardConfig                          *config.Config
		pendingChildExecutionInfoIDs         map[int64]*persistence.ChildExecutionInfo
		wantWorkFlowExecutionInfo            *persistence.WorkflowExecutionInfo
	}{
		{
			name: "binaryChecksum not added due to empty binChecksum",
			wantWorkFlowExecutionInfo: &persistence.WorkflowExecutionInfo{
				RunID: runID,
			},
		},
		{
			name: "binaryChecksum not added due to existing checksum with AutoResetPoints and AutoResetPoints.Points in executionInfo",
			decisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				BinaryChecksum: checkSum,
			},
			autoResetPoints: &types.ResetPoints{
				Points: []*types.ResetPointInfo{
					{BinaryChecksum: checkSum},
				},
			},
			wantWorkFlowExecutionInfo: &persistence.WorkflowExecutionInfo{
				RunID: runID,
				AutoResetPoints: &types.ResetPoints{
					Points: []*types.ResetPointInfo{
						{BinaryChecksum: checkSum},
					},
				},
			},
		},
		{
			name: "success with existing distinct autoResetPoints",
			decisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				BinaryChecksum: checkSum,
			},
			autoResetPoints: &types.ResetPoints{
				Points: []*types.ResetPointInfo{
					{BinaryChecksum: "anotherCheckSum"},
					{BinaryChecksum: "toBeTrimmed"},
				},
			},
			shardConfig: &config.Config{
				WriteVisibilityStoreName: dynamicconfig.GetStringPropertyFn("off"),
			},
			wantWorkFlowExecutionInfo: &persistence.WorkflowExecutionInfo{
				RunID: runID,
				AutoResetPoints: &types.ResetPoints{
					Points: []*types.ResetPointInfo{
						{
							BinaryChecksum:           checkSum,
							RunID:                    runID,
							FirstDecisionCompletedID: eventID,
							CreatedTimeNano:          common.Int64Ptr(timeNow.UnixNano()),
							Resettable:               true,
						},
					},
				},
				SearchAttributes: map[string][]byte{
					definition.BinaryChecksums: func() []byte {
						bytes, _ := json.Marshal([]string{checkSum})
						return bytes
					}(),
				},
			},
		},
		{
			name: "success with AdvancedVisibilityWritingEnabled",
			decisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				BinaryChecksum: checkSum,
			},
			shardConfig: &config.Config{
				WriteVisibilityStoreName: dynamicconfig.GetStringPropertyFn("es"),
				IsAdvancedVisConfigExist: true,
			},
			wantWorkFlowExecutionInfo: &persistence.WorkflowExecutionInfo{
				RunID: runID,
				AutoResetPoints: &types.ResetPoints{
					Points: []*types.ResetPointInfo{
						{
							BinaryChecksum:           checkSum,
							RunID:                    runID,
							FirstDecisionCompletedID: eventID,
							CreatedTimeNano:          common.Int64Ptr(timeNow.UnixNano()),
							Resettable:               true,
						},
					},
				},
				SearchAttributes: map[string][]byte{
					definition.BinaryChecksums: func() []byte {
						bytes, _ := json.Marshal([]string{checkSum})
						return bytes
					}(),
				},
			},
		},
		{
			name: "success with CheckResettable error",
			decisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				BinaryChecksum: checkSum,
			},
			shardConfig: &config.Config{
				WriteVisibilityStoreName: dynamicconfig.GetStringPropertyFn("off"),
			},
			pendingChildExecutionInfoIDs: map[int64]*persistence.ChildExecutionInfo{
				1: {},
			},
			wantWorkFlowExecutionInfo: &persistence.WorkflowExecutionInfo{
				RunID: runID,
				AutoResetPoints: &types.ResetPoints{
					Points: []*types.ResetPointInfo{
						{
							BinaryChecksum:           checkSum,
							RunID:                    runID,
							FirstDecisionCompletedID: eventID,
							CreatedTimeNano:          common.Int64Ptr(timeNow.UnixNano()),
						},
					},
				},
				SearchAttributes: map[string][]byte{
					definition.BinaryChecksums: func() []byte {
						bytes, _ := json.Marshal([]string{checkSum})
						return bytes
					}(),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			builder := &mutableStateBuilder{
				logger: loggerimpl.NewNopLogger(),
				executionInfo: &persistence.WorkflowExecutionInfo{
					AutoResetPoints: tc.autoResetPoints,
					RunID:           runID,
				},
				timeSource:                   clock.NewMockedTimeSourceAt(timeNow),
				pendingChildExecutionInfoIDs: tc.pendingChildExecutionInfoIDs,
				shard:                        shard.NewMockContext(ctrl),
				taskGenerator:                NewMockMutableStateTaskGenerator(ctrl),
			}

			event := &types.HistoryEvent{
				ID:                                   eventID,
				DecisionTaskCompletedEventAttributes: tc.decisionTaskCompletedEventAttributes,
			}
			maxResetPoints := 1

			if tc.shardConfig != nil {
				builder.shard.(*shard.MockContext).EXPECT().GetConfig().Return(tc.shardConfig).Times(2)
			}

			if tc.shardConfig != nil && tc.shardConfig.IsAdvancedVisConfigExist {
				builder.taskGenerator.(*MockMutableStateTaskGenerator).EXPECT().GenerateWorkflowSearchAttrTasks().Return(nil).Times(1)
			}

			err := builder.addBinaryCheckSumIfNotExists(event, maxResetPoints)

			assert.NoError(t, err)

			if diff := cmp.Diff(tc.wantWorkFlowExecutionInfo, builder.executionInfo); diff != "" {
				t.Fatalf("Mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
