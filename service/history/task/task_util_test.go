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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

func TestInitializeLoggerForTask(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()

	testCases := []struct {
		name string
		task persistence.Task
	}{
		{
			name: "UserTimerTask",
			task: &persistence.UserTimerTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				},
				TaskData: persistence.TaskData{
					TaskID:              1,
					VisibilityTimestamp: timeSource.Now(),
					Version:             10,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := InitializeLoggerForTask(1, tc.task, log.NewLogger(zap.NewNop()))
			assert.NotNil(t, logger)
		})
	}
}

func TestGetTransferTaskMetricsScope(t *testing.T) {
	testCases := []struct {
		name          string
		taskType      int
		isActive      bool
		expectedScope metrics.ScopeIdx
	}{
		{
			name:          "TransferTaskTypeActivityTask - active",
			taskType:      persistence.TransferTaskTypeActivityTask,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskActivityScope,
		},
		{
			name:          "TransferTaskTypeActivityTask - standby",
			taskType:      persistence.TransferTaskTypeActivityTask,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskActivityScope,
		},
		{
			name:          "TransferTaskTypeDecisionTask - active",
			taskType:      persistence.TransferTaskTypeDecisionTask,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskDecisionScope,
		},
		{
			name:          "TransferTaskTypeDecisionTask - standby",
			taskType:      persistence.TransferTaskTypeDecisionTask,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskDecisionScope,
		},
		{
			name:          "TransferTaskTypeCloseExecution - active",
			taskType:      persistence.TransferTaskTypeCloseExecution,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskCloseExecutionScope,
		},
		{
			name:          "TransferTaskTypeCloseExecution - standby",
			taskType:      persistence.TransferTaskTypeCloseExecution,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskCloseExecutionScope,
		},
		{
			name:          "TransferTaskTypeCancelExecution - active",
			taskType:      persistence.TransferTaskTypeCancelExecution,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskCancelExecutionScope,
		},
		{
			name:          "TransferTaskTypeCancelExecution - standby",
			taskType:      persistence.TransferTaskTypeCancelExecution,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskCancelExecutionScope,
		},
		{
			name:          "TransferTaskTypeSignalExecution - active",
			taskType:      persistence.TransferTaskTypeSignalExecution,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskSignalExecutionScope,
		},
		{
			name:          "TransferTaskTypeSignalExecution - standby",
			taskType:      persistence.TransferTaskTypeSignalExecution,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskSignalExecutionScope,
		},
		{
			name:          "TransferTaskTypeStartChildExecution - active",
			taskType:      persistence.TransferTaskTypeStartChildExecution,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskStartChildExecutionScope,
		},
		{
			name:          "TransferTaskTypeStartChildExecution - standby",
			taskType:      persistence.TransferTaskTypeStartChildExecution,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskStartChildExecutionScope,
		},
		{
			name:          "TransferTaskTypeRecordWorkflowStarted - active",
			taskType:      persistence.TransferTaskTypeRecordWorkflowStarted,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskRecordWorkflowStartedScope,
		},
		{
			name:          "TransferTaskTypeRecordWorkflowStarted - standby",
			taskType:      persistence.TransferTaskTypeRecordWorkflowStarted,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskRecordWorkflowStartedScope,
		},
		{
			name:          "TransferTaskTypeResetWorkflow - active",
			taskType:      persistence.TransferTaskTypeResetWorkflow,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskResetWorkflowScope,
		},
		{
			name:          "TransferTaskTypeResetWorkflow - standby",
			taskType:      persistence.TransferTaskTypeResetWorkflow,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskResetWorkflowScope,
		},
		{
			name:          "TransferTaskTypeUpsertWorkflowSearchAttributes - active",
			taskType:      persistence.TransferTaskTypeUpsertWorkflowSearchAttributes,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskUpsertWorkflowSearchAttributesScope,
		},
		{
			name:          "TransferTaskTypeUpsertWorkflowSearchAttributes - standby",
			taskType:      persistence.TransferTaskTypeUpsertWorkflowSearchAttributes,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskUpsertWorkflowSearchAttributesScope,
		},
		{
			name:          "TransferTaskTypeRecordWorkflowClosed - active",
			taskType:      persistence.TransferTaskTypeRecordWorkflowClosed,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskRecordWorkflowClosedScope,
		},
		{
			name:          "TransferTaskTypeRecordWorkflowClosed - standby",
			taskType:      persistence.TransferTaskTypeRecordWorkflowClosed,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskRecordWorkflowClosedScope,
		},
		{
			name:          "TransferTaskTypeRecordChildExecutionCompleted - active",
			taskType:      persistence.TransferTaskTypeRecordChildExecutionCompleted,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskRecordChildExecutionCompletedScope,
		},
		{
			name:          "TransferTaskTypeRecordChildExecutionCompleted - standby",
			taskType:      persistence.TransferTaskTypeRecordChildExecutionCompleted,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskRecordChildExecutionCompletedScope,
		},
		{
			name:          "TransferTaskTypeApplyParentClosePolicy - active",
			taskType:      persistence.TransferTaskTypeApplyParentClosePolicy,
			isActive:      true,
			expectedScope: metrics.TransferActiveTaskApplyParentClosePolicyScope,
		},
		{
			name:          "TransferTaskTypeApplyParentClosePolicy - standby",
			taskType:      persistence.TransferTaskTypeApplyParentClosePolicy,
			isActive:      false,
			expectedScope: metrics.TransferStandbyTaskApplyParentClosePolicyScope,
		},
		{
			name:          "TransferTaskType not caught - active",
			taskType:      -100,
			isActive:      true,
			expectedScope: metrics.TransferActiveQueueProcessorScope,
		},
		{
			name:          "TransferTaskType not caught - standby",
			taskType:      -100,
			isActive:      false,
			expectedScope: metrics.TransferStandbyQueueProcessorScope,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scope := GetTransferTaskMetricsScope(tc.taskType, tc.isActive)
			assert.Equal(t, tc.expectedScope, scope)
		})
	}
}

func TestGetTimerTaskMetricScope(t *testing.T) {
	testCases := []struct {
		name          string
		taskType      int
		isActive      bool
		expectedScope metrics.ScopeIdx
	}{
		{
			name:          "TimerTaskTypeDecisionTimeout - active",
			taskType:      persistence.TaskTypeDecisionTimeout,
			isActive:      true,
			expectedScope: metrics.TimerActiveTaskDecisionTimeoutScope,
		},
		{
			name:          "TimerTaskTypeDecisionTimeout - standby",
			taskType:      persistence.TaskTypeDecisionTimeout,
			isActive:      false,
			expectedScope: metrics.TimerStandbyTaskDecisionTimeoutScope,
		},
		{
			name:          "TimerTaskTypeActivityTimeout - active",
			taskType:      persistence.TaskTypeActivityTimeout,
			isActive:      true,
			expectedScope: metrics.TimerActiveTaskActivityTimeoutScope,
		},
		{
			name:          "TimerTaskTypeActivityTimeout - standby",
			taskType:      persistence.TaskTypeActivityTimeout,
			isActive:      false,
			expectedScope: metrics.TimerStandbyTaskActivityTimeoutScope,
		},
		{
			name:          "TimerTaskTypeUserTimer - active",
			taskType:      persistence.TaskTypeUserTimer,
			isActive:      true,
			expectedScope: metrics.TimerActiveTaskUserTimerScope,
		},
		{
			name:          "TimerTaskTypeUserTimer - standby",
			taskType:      persistence.TaskTypeUserTimer,
			isActive:      false,
			expectedScope: metrics.TimerStandbyTaskUserTimerScope,
		},
		{
			name:          "TimerTaskTypeWorkflowTimeout - active",
			taskType:      persistence.TaskTypeWorkflowTimeout,
			isActive:      true,
			expectedScope: metrics.TimerActiveTaskWorkflowTimeoutScope,
		},
		{
			name:          "TimerTaskTypeWorkflowTimeout - standby",
			taskType:      persistence.TaskTypeWorkflowTimeout,
			isActive:      false,
			expectedScope: metrics.TimerStandbyTaskWorkflowTimeoutScope,
		},
		{
			name:          "TimerTaskTypeActivityRetryTimer - active",
			taskType:      persistence.TaskTypeActivityRetryTimer,
			isActive:      true,
			expectedScope: metrics.TimerActiveTaskActivityRetryTimerScope,
		},
		{
			name:          "TimerTaskTypeActivityRetryTimer - standby",
			taskType:      persistence.TaskTypeActivityRetryTimer,
			isActive:      false,
			expectedScope: metrics.TimerStandbyTaskActivityRetryTimerScope,
		},
		{
			name:          "TimerTaskTypeWorkflowBackoffTimer - active",
			taskType:      persistence.TaskTypeWorkflowBackoffTimer,
			isActive:      true,
			expectedScope: metrics.TimerActiveTaskWorkflowBackoffTimerScope,
		},
		{
			name:          "TimerTaskTypeWorkflowBackoffTimer - standby",
			taskType:      persistence.TaskTypeWorkflowBackoffTimer,
			isActive:      false,
			expectedScope: metrics.TimerStandbyTaskWorkflowBackoffTimerScope,
		},
		{
			name:          "TimerTaskTypeDeleteHistoryEvent - active",
			taskType:      persistence.TaskTypeDeleteHistoryEvent,
			isActive:      true,
			expectedScope: metrics.TimerActiveTaskDeleteHistoryEventScope,
		},
		{
			name:          "TimerTaskTypeDeleteHistoryEvent - standby",
			taskType:      persistence.TaskTypeDeleteHistoryEvent,
			isActive:      false,
			expectedScope: metrics.TimerStandbyTaskDeleteHistoryEventScope,
		},
		{
			name:          "TimerTaskType not caught - active",
			taskType:      -100,
			isActive:      true,
			expectedScope: metrics.TimerActiveQueueProcessorScope,
		},
		{
			name:          "TimerTaskType not caught - standby",
			taskType:      -100,
			isActive:      false,
			expectedScope: metrics.TimerStandbyQueueProcessorScope,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scope := GetTimerTaskMetricScope(tc.taskType, tc.isActive)
			assert.Equal(t, tc.expectedScope, scope)
		})
	}
}

func Test_verifyTaskVersion(t *testing.T) {
	testCases := []struct {
		name      string
		setupMock func(*shard.MockContext, *cache.MockDomainCache)
		version   int64
		err       error
		response  bool
	}{
		{
			name: "error - could not get domain entry",
			setupMock: func(s *shard.MockContext, c *cache.MockDomainCache) {
				c.EXPECT().GetDomainByID(constants.TestDomainID).Return(nil, errors.New("test error")).Times(1)
				s.EXPECT().GetDomainCache().Return(c).Times(1)
			},
			err:      errors.New("test error"),
			response: false,
		},
		{
			name: "true - domain is not global",
			setupMock: func(s *shard.MockContext, c *cache.MockDomainCache) {
				domainResponse := cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{ID: constants.TestDomainID, Name: constants.TestDomainName}, &persistence.DomainConfig{}, false, nil, 0, nil, 0, 0, 0)
				c.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainResponse, nil).Times(1)
				s.EXPECT().GetDomainCache().Return(c).Times(1)
			},
			err:      nil,
			response: true,
		},
		{
			name: "false - version is different from task version",
			setupMock: func(s *shard.MockContext, c *cache.MockDomainCache) {
				domainResponse := cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{ID: constants.TestDomainID, Name: constants.TestDomainName}, &persistence.DomainConfig{}, true, nil, 0, nil, 0, 0, 0)
				c.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainResponse, nil).Times(1)
				s.EXPECT().GetDomainCache().Return(c).Times(1)
			},
			version:  6,
			err:      nil,
			response: false,
		},
		{
			name: "true - version is same as task version",
			setupMock: func(s *shard.MockContext, c *cache.MockDomainCache) {
				domainResponse := cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{ID: constants.TestDomainID, Name: constants.TestDomainName}, &persistence.DomainConfig{}, true, nil, 0, nil, 0, 0, 0)
				c.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainResponse, nil).Times(1)
				s.EXPECT().GetDomainCache().Return(c).Times(1)
			},
			version:  50,
			err:      nil,
			response: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			s := shard.NewMockContext(ctrl)
			l := log.NewNoop()
			task := &persistence.UserTimerTask{
				TaskData: persistence.TaskData{
					Version: constants.TestVersion,
				},
			}
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			tc.setupMock(s, mockDomainCache)
			ok, err := verifyTaskVersion(s, l, constants.TestDomainID, tc.version, task.Version, task)
			assert.Equal(t, tc.err, err)
			assert.Equal(t, tc.response, ok)
		})
	}
}

func Test_loadMutableState(t *testing.T) {
	testCases := []struct {
		name                   string
		setupMock              func(*execution.MockContext, *execution.MockMutableState)
		task                   persistence.Task
		eventID                int64
		err                    error
		isMutableStateReturned bool
	}{
		{
			name: "error - failed to load workflow execution - entity does not exist",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(nil, &types.EntityNotExistsError{}).Times(1)
			},
			err: nil,
		},
		{
			name: "error - failed to load workflow execution - another error",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(nil, errors.New("other-error")).Times(1)
			},
			err: errors.New("other-error"),
		},
		{
			name: "error - transfer task scheduleID > next eventID and !isDecisionRetry - error loading workflow execution",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				gomock.InOrder(
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1),
					m.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{NextEventID: int64(5), DecisionAttempt: 1, DecisionScheduleID: 1}).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
					w.EXPECT().Clear().Times(1),
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(nil, errors.New("some-other-error")).Times(1),
				)
			},
			task: &persistence.DecisionTask{
				ScheduleID: 11,
			},
			eventID: 11,
			err:     errors.New("some-other-error"),
		},
		{
			name: "no mutable state returned - transfer task scheduleID > next eventID after refresh",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				gomock.InOrder(
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1),
					m.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{NextEventID: int64(5), DecisionAttempt: 1, DecisionScheduleID: 1}).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
					w.EXPECT().Clear().Times(1),
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
					m.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
				)

			},
			task: &persistence.DecisionTask{
				ScheduleID: 11,
			},
			eventID: 11,
			err:     nil,
		},
		{
			name: "success - transfer task scheduleID < next eventID",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1)
				m.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{NextEventID: int64(5), DecisionAttempt: 1, DecisionScheduleID: 1}).Times(1)
				m.EXPECT().GetNextEventID().Return(int64(10)).Times(1)
			},
			task: &persistence.DecisionTask{
				ScheduleID: 3,
			},
			eventID:                3,
			isMutableStateReturned: true,
		},
		{
			name: "error - timer task eventID > next eventID and !isDecisionRetry - error loading workflow execution",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				gomock.InOrder(
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1),
					m.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{NextEventID: int64(5), DecisionAttempt: 1, DecisionScheduleID: 1}).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
					w.EXPECT().Clear().Times(1),
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(nil, errors.New("some-other-error")).Times(1),
				)

			},
			task:    &persistence.DecisionTimeoutTask{},
			eventID: 11,
			err:     errors.New("some-other-error"),
		},
		{
			name: "no mutable state returned - timer task eventID > next eventID after refresh",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				gomock.InOrder(
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1),
					m.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{NextEventID: int64(5), DecisionAttempt: 1, DecisionScheduleID: 1}).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
					w.EXPECT().Clear().Times(1),
					w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
					m.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).Times(1),
					m.EXPECT().GetNextEventID().Return(int64(10)).Times(1),
				)
			},
			task:    &persistence.DecisionTimeoutTask{},
			eventID: 11,
			err:     nil,
		},
		{
			name: "success - timer task eventID < next eventID",
			setupMock: func(w *execution.MockContext, m *execution.MockMutableState) {
				w.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(m, nil).Times(1)
				m.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{NextEventID: int64(5), DecisionAttempt: 1, DecisionScheduleID: 1}).Times(1)
				m.EXPECT().GetNextEventID().Return(int64(10)).Times(1)
			},
			task:                   &persistence.DecisionTimeoutTask{},
			eventID:                3,
			isMutableStateReturned: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			w := execution.NewMockContext(ctrl)
			m := execution.NewMockMutableState(ctrl)
			metricsScope := metrics.NoopScope
			l := log.NewNoop()

			tc.setupMock(w, m)
			ms, err := loadMutableState(context.Background(), w, tc.task, metricsScope, l, tc.eventID)
			assert.Equal(t, tc.err, err)
			if tc.err != nil {
				assert.Nil(t, ms)
			} else if tc.isMutableStateReturned {
				assert.Equal(t, m, ms)
			} else {
				assert.Nil(t, ms)
			}
		})
	}
}

func Test_timeoutWorkflow(t *testing.T) {
	eventBatchFirstEventID := int64(2)
	testCases := []struct {
		name      string
		setupMock func(*execution.MockMutableState)
		err       error
	}{
		{
			name: "error - execution failDecision error",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, errors.New("failDecision-error")).Times(1)
			},
			err: errors.New("failDecision-error"),
		},
		{
			name: "failDecision and timeout added to mutable state",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, nil).Times(1)
				m.EXPECT().FlushBufferedEvents().Return(nil).Times(1)
				m.EXPECT().AddTimeoutWorkflowEvent(eventBatchFirstEventID).Return(nil, nil).Times(1)
			},
			err: nil,
		},
		{
			name: "error - AddTimeoutWorkflowEvent error",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, nil).Times(1)
				m.EXPECT().FlushBufferedEvents().Return(nil).Times(1)
				m.EXPECT().AddTimeoutWorkflowEvent(eventBatchFirstEventID).Return(nil, errors.New("timeoutWorkflow-error")).Times(1)
			},
			err: errors.New("timeoutWorkflow-error"),
		},
		{
			name: "success - timeoutWorkflow event added",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, nil).Times(1)
				m.EXPECT().FlushBufferedEvents().Return(nil).Times(1)
				m.EXPECT().AddTimeoutWorkflowEvent(eventBatchFirstEventID).Return(nil, nil).Times(1)
			},
			err: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := execution.NewMockMutableState(ctrl)

			tc.setupMock(m)
			err := timeoutWorkflow(m, eventBatchFirstEventID)
			assert.Equal(t, tc.err, err)
		})
	}
}

func Test_retryWorkflow(t *testing.T) {
	eventBatchFirstEventID := int64(2)
	parentDomainName := "parent-domain-name"
	continueAsNewAttributes := &types.ContinueAsNewWorkflowExecutionDecisionAttributes{}

	testCases := []struct {
		name      string
		setupMock func(*execution.MockMutableState)
		err       error
	}{
		{
			name: "error - execution failDecision error",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, errors.New("failDecision-error")).Times(1)
			},
			err: errors.New("failDecision-error"),
		},
		{
			name: "failDecision and continueAsNew added to mutable state",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, nil).Times(1)
				m.EXPECT().FlushBufferedEvents().Return(nil).Times(1)
				m.EXPECT().AddContinueAsNewEvent(
					gomock.Any(),
					eventBatchFirstEventID,
					commonconstants.EmptyEventID,
					parentDomainName,
					continueAsNewAttributes,
				).Return(nil, m, nil).Times(1)
			},
			err: nil,
		},
		{
			name: "error - AddContinueAsNewEvent error",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, nil).Times(1)
				m.EXPECT().FlushBufferedEvents().Return(nil).Times(1)
				m.EXPECT().AddContinueAsNewEvent(
					gomock.Any(),
					eventBatchFirstEventID,
					commonconstants.EmptyEventID,
					parentDomainName,
					continueAsNewAttributes,
				).Return(nil, nil, errors.New("continueAsNew-error")).Times(1)
			},
			err: errors.New("continueAsNew-error"),
		},
		{
			name: "success - continueAsNew event added",
			setupMock: func(m *execution.MockMutableState) {
				decisionInfo := &execution.DecisionInfo{
					ScheduleID: 1,
					StartedID:  2,
				}
				m.EXPECT().GetInFlightDecision().Return(decisionInfo, true).Times(1)
				m.EXPECT().AddDecisionTaskFailedEvent(
					decisionInfo.ScheduleID,
					decisionInfo.StartedID,
					types.DecisionTaskFailedCauseForceCloseDecision,
					nil, execution.IdentityHistoryService,
					"",
					"",
					"",
					"",
					int64(0),
					"").Return(nil, nil).Times(1)
				m.EXPECT().FlushBufferedEvents().Return(nil).Times(1)
				m.EXPECT().AddContinueAsNewEvent(
					gomock.Any(),
					eventBatchFirstEventID,
					commonconstants.EmptyEventID,
					parentDomainName,
					continueAsNewAttributes,
				).Return(nil, m, nil).Times(1)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := execution.NewMockMutableState(ctrl)
			ctx := context.Background()

			tc.setupMock(m)
			resp, err := retryWorkflow(ctx, m, eventBatchFirstEventID, "parent-domain-name", continueAsNewAttributes)
			assert.Equal(t, tc.err, err)
			if tc.err == nil {
				assert.Equal(t, m, resp)
			}
		})
	}
}

func Test_mocks(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockTask := NewMockTask(ctrl)
	resp := NewMockTaskMatcher(mockTask)
	assert.NotNil(t, resp)
	assert.Equal(t, &mockTaskMatcher{task: mockTask}, resp)

	matches := resp.Matches(mockTask)
	assert.True(t, matches)

	matches = resp.Matches(nil)
	assert.False(t, matches)

	assert.Equal(t, fmt.Sprintf("is equal to %v", mockTask), resp.String())
}

func TestShouldPushToMatching(t *testing.T) {
	domainID := "domainID"
	wfid := "wfid"
	rid := "rid"
	currentClusterName := "currentCluster"

	tests := []struct {
		name           string
		setupMock      func(mockDomainCache *cache.MockDomainCache, mockActiveClusterMgr *activecluster.MockManager)
		expectedResult bool
		expectedError  string
	}{
		{
			name: "failed to get domain",
			setupMock: func(mockDomainCache *cache.MockDomainCache, mockActiveClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(nil, errors.New("failed to get domain"))
			},
			expectedResult: false,
			expectedError:  "failed to get domain",
		},
		{
			name: "not an active-active domain so returns true",
			setupMock: func(mockDomainCache *cache.MockDomainCache, mockActiveClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(getDomainCacheEntry(true, false), nil)
			},
			expectedResult: true,
		},
		{
			name: "active-active domain and workflow is active in current cluster so returns true",
			setupMock: func(mockDomainCache *cache.MockDomainCache, mockActiveClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(getDomainCacheEntry(true, true), nil)
				mockActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), domainID, wfid, rid).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: currentClusterName,
					}, nil)
			},
			expectedResult: true,
		},
		{
			name: "active-active domain and workflow is not active in current cluster so returns false",
			setupMock: func(mockDomainCache *cache.MockDomainCache, mockActiveClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(getDomainCacheEntry(true, true), nil)
				mockActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), domainID, wfid, rid).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: "otherCluster",
					}, nil)
			},
			expectedResult: false,
		},
		{
			name: "active-active domain - failed to lookup workflow",
			setupMock: func(mockDomainCache *cache.MockDomainCache, mockActiveClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(getDomainCacheEntry(true, true), nil)
				mockActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), domainID, wfid, rid).
					Return(nil, errors.New("failed to lookup workflow"))
			},
			expectedResult: false,
			expectedError:  "failed to lookup workflow",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			shard := shard.NewMockContext(ctrl)

			domainCache := cache.NewMockDomainCache(ctrl)

			shard.EXPECT().GetClusterMetadata().Return(cluster.NewMetadata(
				config.ClusterGroupMetadata{
					CurrentClusterName: currentClusterName,
				},
				func(d string) bool { return false },
				metrics.NewNoopMetricsClient(),
				log.NewNoop(),
			)).AnyTimes()

			shard.EXPECT().GetDomainCache().Return(domainCache)

			activeClusterMgr := activecluster.NewMockManager(ctrl)
			shard.EXPECT().GetActiveClusterManager().Return(activeClusterMgr).AnyTimes()

			test.setupMock(domainCache, activeClusterMgr)

			mockTask := NewMockTask(ctrl)
			mockTask.EXPECT().GetDomainID().Return(domainID).AnyTimes()
			mockTask.EXPECT().GetWorkflowID().Return(wfid).AnyTimes()
			mockTask.EXPECT().GetRunID().Return(rid).AnyTimes()

			result, err := shouldPushToMatching(context.Background(), shard, mockTask)
			assert.Equal(t, test.expectedResult, result)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func getDomainCacheEntry(isGlobal, isActiveActive bool) *cache.DomainCacheEntry {
	activeClusters := &types.ActiveClusters{
		AttributeScopes: map[string]types.ClusterAttributeScope{
			"region": {
				ClusterAttributes: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
					},
				},
			},
		},
	}
	if !isActiveActive {
		activeClusters = nil
	}
	return cache.NewDomainCacheEntryForTest(
		nil,
		nil,
		isGlobal,
		&persistence.DomainReplicationConfig{
			ActiveClusters: activeClusters,
		},
		1,
		nil,
		1,
		1,
		1,
	)
}
