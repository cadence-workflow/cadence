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
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/events"
)

func TestRefreshTasksForWorkflowStart(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator)
		wantErr   bool
	}{
		{
			name: "failed to get start event",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetStartEvent(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate start tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetStartEvent(gomock.Any()).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateWorkflowStartTasks(gomock.Any(), &types.HistoryEvent{ID: 1}).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate delayed decision tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				startEvent := &types.HistoryEvent{
					ID: 1,
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Ptr[int32](10),
					},
				}
				ms.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mtg.EXPECT().GenerateWorkflowStartTasks(gomock.Any(), gomock.Any()).Return(nil)
				ms.EXPECT().HasProcessedOrPendingDecision().Return(false)
				mtg.EXPECT().GenerateDelayedDecisionTasks(startEvent).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				startEvent := &types.HistoryEvent{
					ID: 1,
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						FirstDecisionTaskBackoffSeconds: common.Ptr[int32](10),
					},
				}
				ms.EXPECT().GetStartEvent(gomock.Any()).Return(startEvent, nil)
				mtg.EXPECT().GenerateWorkflowStartTasks(gomock.Any(), gomock.Any()).Return(nil)
				ms.EXPECT().HasProcessedOrPendingDecision().Return(false)
				mtg.EXPECT().GenerateDelayedDecisionTasks(startEvent).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			tc.mockSetup(ms, mtg)
			err := refreshTasksForWorkflowStart(context.Background(), time.Now(), ms, mtg)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForWorkflowStart err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForWorkflowClose(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator)
		wantErr   bool
	}{
		{
			name: "failed to get completion event",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{CloseStatus: persistence.WorkflowCloseStatusCompleted})
				ms.EXPECT().GetCompletionEvent(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate close tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{CloseStatus: persistence.WorkflowCloseStatusCompleted})
				ms.EXPECT().GetCompletionEvent(gomock.Any()).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateWorkflowCloseTasks(&types.HistoryEvent{ID: 1}, gomock.Any()).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success - open workflow",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{CloseStatus: persistence.WorkflowCloseStatusNone})
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			tc.mockSetup(ms, mtg)
			err := refreshTasksForWorkflowClose(context.Background(), ms, mtg, 100)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForWorkflowClose err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForRecordWorkflowStarted(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator)
		wantErr   bool
	}{
		{
			name: "failed to get start event",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{CloseStatus: persistence.WorkflowCloseStatusNone})
				ms.EXPECT().GetStartEvent(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate record started tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetStartEvent(gomock.Any()).Return(&types.HistoryEvent{ID: 1}, nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{CloseStatus: persistence.WorkflowCloseStatusNone})
				mtg.EXPECT().GenerateRecordWorkflowStartedTasks(&types.HistoryEvent{ID: 1}).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success - closed workflow",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{CloseStatus: persistence.WorkflowCloseStatusCompleted})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			tc.mockSetup(ms, mtg)
			err := refreshTasksForRecordWorkflowStarted(context.Background(), ms, mtg)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForRecordWorkflowStarted err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForDecision(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator)
		wantErr   bool
	}{
		{
			name: "success - no pending decision",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().HasPendingDecision().Return(false)
			},
			wantErr: false,
		},
		{
			name: "bug - cannot get pending decision",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().HasPendingDecision().Return(true)
				ms.EXPECT().GetPendingDecision().Return(nil, false)
			},
			wantErr: true,
		},
		{
			name: "success - generate decision started task",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().HasPendingDecision().Return(true)
				ms.EXPECT().GetPendingDecision().Return(&DecisionInfo{ScheduleID: 2, StartedID: 3}, true)
				mtg.EXPECT().GenerateDecisionStartTasks(int64(2)).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "success - generate decision started task",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator) {
				ms.EXPECT().HasPendingDecision().Return(true)
				ms.EXPECT().GetPendingDecision().Return(&DecisionInfo{ScheduleID: 2, StartedID: constants.EmptyEventID}, true)
				mtg.EXPECT().GenerateDecisionScheduleTasks(int64(2)).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			tc.mockSetup(ms, mtg)
			err := refreshTasksForDecision(context.Background(), ms, mtg)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForDecision err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForActivity(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator, *events.MockCache, *MockTimerSequence)
		wantErr   bool
	}{
		{
			name: "failed to get current branch token",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache, mt *MockTimerSequence) {
				ms.EXPECT().GetCurrentBranchToken().Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to update activity",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache, mt *MockTimerSequence) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{})
				ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{1: {Version: 1, TimerTaskStatus: TimerTaskStatusCreated}})
				ms.EXPECT().UpdateActivity(&persistence.ActivityInfo{Version: 1, TimerTaskStatus: TimerTaskStatusNone}).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to get event",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache, mt *MockTimerSequence) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{1: {Version: 1, TimerTaskStatus: TimerTaskStatusCreated, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}})
				ms.EXPECT().UpdateActivity(&persistence.ActivityInfo{Version: 1, TimerTaskStatus: TimerTaskStatusNone, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}).Return(nil)
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(11), int64(12), []byte("token")).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate activity tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache, mt *MockTimerSequence) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{1: {Version: 1, TimerTaskStatus: TimerTaskStatusCreated, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}})
				ms.EXPECT().UpdateActivity(&persistence.ActivityInfo{Version: 1, TimerTaskStatus: TimerTaskStatusNone, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}).Return(nil)
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(11), int64(12), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateActivityTransferTasks(&types.HistoryEvent{ID: 1}).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to create activity timer",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache, mt *MockTimerSequence) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{1: {Version: 1, TimerTaskStatus: TimerTaskStatusCreated, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}})
				ms.EXPECT().UpdateActivity(&persistence.ActivityInfo{Version: 1, TimerTaskStatus: TimerTaskStatusNone, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}).Return(nil)
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(11), int64(12), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateActivityTransferTasks(&types.HistoryEvent{ID: 1}).Return(nil)
				mt.EXPECT().CreateNextActivityTimer().Return(false, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache, mt *MockTimerSequence) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{10: {Version: 0, StartedID: 10}, 1: {Version: 1, TimerTaskStatus: TimerTaskStatusCreated, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}})
				ms.EXPECT().UpdateActivity(&persistence.ActivityInfo{Version: 0, StartedID: 10, TimerTaskStatus: TimerTaskStatusNone}).Return(nil)
				ms.EXPECT().UpdateActivity(&persistence.ActivityInfo{Version: 1, TimerTaskStatus: TimerTaskStatusNone, ScheduledEventBatchID: 11, ScheduleID: 12, StartedID: constants.EmptyEventID}).Return(nil)
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(11), int64(12), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateActivityTransferTasks(&types.HistoryEvent{ID: 1}).Return(nil)
				mt.EXPECT().CreateNextActivityTimer().Return(true, nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			mc := events.NewMockCache(ctrl)
			mt := NewMockTimerSequence(ctrl)
			tc.mockSetup(ms, mtg, mc, mt)
			err := refreshTasksForActivity(context.Background(), ms, mtg, 1, mc, func(MutableState) TimerSequence { return mt })
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForActivity err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForTimer(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator, *MockTimerSequence)
		wantErr   bool
	}{
		{
			name: "failed to update user timer",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mt *MockTimerSequence) {
				ms.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{"0": &persistence.TimerInfo{}})
				ms.EXPECT().UpdateUserTimer(gomock.Any()).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to create user timer",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mt *MockTimerSequence) {
				ms.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{"0": &persistence.TimerInfo{}})
				ms.EXPECT().UpdateUserTimer(gomock.Any()).Return(nil)
				mt.EXPECT().CreateNextUserTimer().Return(false, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mt *MockTimerSequence) {
				ms.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{"0": &persistence.TimerInfo{}})
				ms.EXPECT().UpdateUserTimer(gomock.Any()).Return(nil)
				mt.EXPECT().CreateNextUserTimer().Return(false, nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			mt := NewMockTimerSequence(ctrl)
			tc.mockSetup(ms, mtg, mt)
			err := refreshTasksForTimer(context.Background(), ms, mtg, func(MutableState) TimerSequence { return mt })
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForTimer err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForChildWorkflow(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator, *events.MockCache)
		wantErr   bool
	}{
		{
			name: "failed to get current branch token",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to get event",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, StartedID: constants.EmptyEventID, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate child workflow tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, StartedID: constants.EmptyEventID, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateChildWorkflowTasks(&types.HistoryEvent{ID: 1}).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, StartedID: constants.EmptyEventID, Version: 1}, 11: {StartedID: 12}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateChildWorkflowTasks(&types.HistoryEvent{ID: 1}).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			mc := events.NewMockCache(ctrl)
			tc.mockSetup(ms, mtg, mc)
			err := refreshTasksForChildWorkflow(context.Background(), ms, mtg, 1, mc)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForChildWorkflow err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForRequestCancelExternalWorkflow(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator, *events.MockCache)
		wantErr   bool
	}{
		{
			name: "failed to get current branch token",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to get event",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate child workflow tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateRequestCancelExternalTasks(&types.HistoryEvent{ID: 1}).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateRequestCancelExternalTasks(&types.HistoryEvent{ID: 1}).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			mc := events.NewMockCache(ctrl)
			tc.mockSetup(ms, mtg, mc)
			err := refreshTasksForRequestCancelExternalWorkflow(context.Background(), ms, mtg, 1, mc)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForRequestCancelExternalWorkflow err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForSignalExternalWorkflow(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableState, *MockMutableStateTaskGenerator, *events.MockCache)
		wantErr   bool
	}{
		{
			name: "failed to get current branch token",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to get event",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate child workflow tasks",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateSignalExternalTasks(&types.HistoryEvent{ID: 1}).Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(ms *MockMutableState, mtg *MockMutableStateTaskGenerator, mc *events.MockCache) {
				ms.EXPECT().GetCurrentBranchToken().Return([]byte("token"), nil)
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: "domain-id", WorkflowID: "wf-id", RunID: "run-id"})
				ms.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{1: {InitiatedEventBatchID: 1, InitiatedID: 2, Version: 1}})
				mc.EXPECT().GetEvent(gomock.Any(), gomock.Any(), "domain-id", "wf-id", "run-id", int64(1), int64(2), []byte("token")).Return(&types.HistoryEvent{ID: 1}, nil)
				mtg.EXPECT().GenerateSignalExternalTasks(&types.HistoryEvent{ID: 1}).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			mc := events.NewMockCache(ctrl)
			tc.mockSetup(ms, mtg, mc)
			err := refreshTasksForSignalExternalWorkflow(context.Background(), ms, mtg, 1, mc)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForSignalExternalWorkflow err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasksForWorkflowSearchAttr(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*MockMutableStateTaskGenerator)
		wantErr   bool
	}{
		{
			name: "failed to generate workflow search attribute tasks",
			mockSetup: func(mtg *MockMutableStateTaskGenerator) {
				mtg.EXPECT().GenerateWorkflowSearchAttrTasks().Return(errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(mtg *MockMutableStateTaskGenerator) {
				mtg.EXPECT().GenerateWorkflowSearchAttrTasks().Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			tc.mockSetup(mtg)
			err := refreshTasksForWorkflowSearchAttr(context.Background(), nil, mtg)
			if (err != nil) != tc.wantErr {
				t.Errorf("refreshTasksForWorkflowSearchAttr err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRefreshTasks(t *testing.T) {
	testCases := []struct {
		name                                           string
		refreshTasksForWorkflowStartFn                 func(context.Context, time.Time, MutableState, MutableStateTaskGenerator) error
		refreshTasksForWorkflowCloseFn                 func(context.Context, MutableState, MutableStateTaskGenerator, int) error
		refreshTasksForRecordWorkflowStartedFn         func(context.Context, MutableState, MutableStateTaskGenerator) error
		refreshTasksForDecisionFn                      func(context.Context, MutableState, MutableStateTaskGenerator) error
		refreshTasksForActivityFn                      func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache, func(MutableState) TimerSequence) error
		refreshTasksForTimerFn                         func(context.Context, MutableState, MutableStateTaskGenerator, func(MutableState) TimerSequence) error
		refreshTasksForChildWorkflowFn                 func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error
		refreshTasksForRequestCancelExternalWorkflowFn func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error
		refreshTasksForSignalExternalWorkflowFn        func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error
		refreshTasksForWorkflowSearchAttrFn            func(context.Context, MutableState, MutableStateTaskGenerator) error
		wantErr                                        bool
	}{
		{
			name:                                   "success",
			refreshTasksForWorkflowStartFn:         func(context.Context, time.Time, MutableState, MutableStateTaskGenerator) error { return nil },
			refreshTasksForWorkflowCloseFn:         func(context.Context, MutableState, MutableStateTaskGenerator, int) error { return nil },
			refreshTasksForRecordWorkflowStartedFn: func(context.Context, MutableState, MutableStateTaskGenerator) error { return nil },
			refreshTasksForDecisionFn:              func(context.Context, MutableState, MutableStateTaskGenerator) error { return nil },
			refreshTasksForActivityFn: func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache, func(MutableState) TimerSequence) error {
				return nil
			},
			refreshTasksForTimerFn: func(context.Context, MutableState, MutableStateTaskGenerator, func(MutableState) TimerSequence) error {
				return nil
			},
			refreshTasksForChildWorkflowFn:                 func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error { return nil },
			refreshTasksForRequestCancelExternalWorkflowFn: func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error { return nil },
			refreshTasksForSignalExternalWorkflowFn:        func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error { return nil },
			refreshTasksForWorkflowSearchAttrFn:            func(context.Context, MutableState, MutableStateTaskGenerator) error { return nil },
			wantErr:                                        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ms := NewMockMutableState(ctrl)
			mtg := NewMockMutableStateTaskGenerator(ctrl)
			ms.EXPECT().GetDomainEntry().Return(cache.NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{ID: "domain-id"}, nil, "test")).AnyTimes()
			refresher := &mutableStateTaskRefresherImpl{
				config: &config.Config{
					WriteVisibilityStoreName:    dynamicproperties.GetStringPropertyFn(constants.VisibilityModeES),
					WorkflowDeletionJitterRange: dynamicproperties.GetIntPropertyFilteredByDomain(1),
					IsAdvancedVisConfigExist:    true,
				},
				newMutableStateTaskGeneratorFn: func(log.Logger, cluster.Metadata, cache.DomainCache, MutableState) MutableStateTaskGenerator {
					return mtg
				},
				refreshTasksForWorkflowStartFn:                 tc.refreshTasksForWorkflowStartFn,
				refreshTasksForWorkflowCloseFn:                 tc.refreshTasksForWorkflowCloseFn,
				refreshTasksForRecordWorkflowStartedFn:         tc.refreshTasksForRecordWorkflowStartedFn,
				refreshTasksForDecisionFn:                      tc.refreshTasksForDecisionFn,
				refreshTasksForActivityFn:                      tc.refreshTasksForActivityFn,
				refreshTasksForTimerFn:                         tc.refreshTasksForTimerFn,
				refreshTasksForChildWorkflowFn:                 tc.refreshTasksForChildWorkflowFn,
				refreshTasksForRequestCancelExternalWorkflowFn: tc.refreshTasksForRequestCancelExternalWorkflowFn,
				refreshTasksForSignalExternalWorkflowFn:        tc.refreshTasksForSignalExternalWorkflowFn,
				refreshTasksForWorkflowSearchAttrFn:            tc.refreshTasksForWorkflowSearchAttrFn,
			}
			err := refresher.RefreshTasks(context.Background(), time.Now(), ms)
			if (err != nil) != tc.wantErr {
				t.Errorf("RefreshTasks err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
