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

package execution

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
)

type (
	mutableStateTaskGeneratorSuite struct {
		suite.Suite
		*require.Assertions

		controller       *gomock.Controller
		mockDomainCache  *cache.MockDomainCache
		mockMutableState *MockMutableState

		taskGenerator *mutableStateTaskGeneratorImpl
	}
)

func TestMutableStateTaskGeneratorSuite(t *testing.T) {
	s := new(mutableStateTaskGeneratorSuite)
	suite.Run(t, s)
}

func (s *mutableStateTaskGeneratorSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)
	s.mockMutableState = NewMockMutableState(s.controller)

	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()

	s.taskGenerator = NewMutableStateTaskGenerator(
		log.NewNoop(),
		constants.TestClusterMetadata,
		s.mockDomainCache,
		s.mockMutableState,
	).(*mutableStateTaskGeneratorImpl)
}

func (s *mutableStateTaskGeneratorSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateWorkflowCloseTasks_JitteredDeletion() {
	now := time.Now()
	version := int64(123)
	closeEvent := &types.HistoryEvent{
		EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
		Timestamp: common.Int64Ptr(now.UnixNano()),
		Version:   version,
	}
	domainEntry, err := s.mockDomainCache.GetDomainByID(constants.TestDomainID)
	s.NoError(err)
	retention := time.Duration(domainEntry.GetRetentionDays(constants.TestWorkflowID)) * time.Hour * 24
	testCases := GenerateWorkflowCloseTasksTestCases(retention, closeEvent, now)

	for _, tc := range testCases {
		// create new mockMutableState so can we can setup separete mock for each test case
		mockMutableState := NewMockMutableState(s.controller)
		taskGenerator := NewMutableStateTaskGenerator(
			log.NewNoop(),
			constants.TestClusterMetadata,
			s.mockDomainCache,
			mockMutableState,
		)

		var transferTasks []persistence.Task
		var timerTasks []persistence.Task
		mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry).AnyTimes()
		mockMutableState.EXPECT().AddTransferTasks(gomock.Any()).Do(func(tasks ...persistence.Task) {
			transferTasks = tasks
		}).MaxTimes(1)
		mockMutableState.EXPECT().AddTimerTasks(gomock.Any()).Do(func(tasks ...persistence.Task) {
			timerTasks = tasks
		}).MaxTimes(1)

		tc.setupFn(mockMutableState)
		err := taskGenerator.GenerateWorkflowCloseTasks(closeEvent, 60)
		s.NoError(err)

		actualGeneratedTasks := transferTasks
		for _, task := range actualGeneratedTasks {
			// force set visibility timestamp since that field is not assigned
			// for transfer and cross cluster in GenerateWorkflowCloseTasks
			// it will be set by shard context
			// set it to now so that we can easily test if other fields are equal
			task.SetVisibilityTimestamp(now)
		}
		for _, task := range timerTasks {
			// force set timer tasks because with jittering the timertask visibility time stamp
			// is not consistent for each run.
			// as long as code doesn't break during generation we should be ok
			task.SetVisibilityTimestamp(time.Unix(0, closeEvent.GetTimestamp()).Add(retention))
		}
		actualGeneratedTasks = append(actualGeneratedTasks, timerTasks...)
		s.Equal(tc.generatedTasks, actualGeneratedTasks)
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateWorkflowCloseTasks() {
	now := time.Now()
	version := int64(123)
	closeEvent := &types.HistoryEvent{
		EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
		Timestamp: common.Int64Ptr(now.UnixNano()),
		Version:   version,
	}
	domainEntry, err := s.mockDomainCache.GetDomainByID(constants.TestDomainID)
	s.NoError(err)
	retention := time.Duration(domainEntry.GetRetentionDays(constants.TestWorkflowID)) * time.Hour * 24
	testCases := GenerateWorkflowCloseTasksTestCases(retention, closeEvent, now)

	for _, tc := range testCases {
		// create new mockMutableState so can we can setup separete mock for each test case
		mockMutableState := NewMockMutableState(s.controller)
		taskGenerator := NewMutableStateTaskGenerator(
			log.NewNoop(),
			constants.TestClusterMetadata,
			s.mockDomainCache,
			mockMutableState,
		)

		var transferTasks []persistence.Task
		var timerTasks []persistence.Task
		mockMutableState.EXPECT().GetDomainEntry().Return(domainEntry).AnyTimes()
		mockMutableState.EXPECT().AddTransferTasks(gomock.Any()).Do(func(tasks ...persistence.Task) {
			transferTasks = tasks
		}).MaxTimes(1)
		mockMutableState.EXPECT().AddTimerTasks(gomock.Any()).Do(func(tasks ...persistence.Task) {
			timerTasks = tasks
		}).MaxTimes(1)

		tc.setupFn(mockMutableState)
		err := taskGenerator.GenerateWorkflowCloseTasks(closeEvent, 1)
		s.NoError(err)

		actualGeneratedTasks := transferTasks
		for _, task := range actualGeneratedTasks {
			// force set visibility timestamp since that field is not assigned
			// for transfer and cross cluster in GenerateWorkflowCloseTasks
			// it will be set by shard context
			// set it to now so that we can easily test if other fields are equal
			task.SetVisibilityTimestamp(now)
		}
		actualGeneratedTasks = append(actualGeneratedTasks, timerTasks...)
		s.Equal(tc.generatedTasks, actualGeneratedTasks)
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateWorkflowCloseTasks_NotActive() {
	closeEvent := &types.HistoryEvent{
		Version:   constants.TestVersion,
		Timestamp: common.Ptr(time.Unix(1719224698, 0).UnixNano()),
	}

	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   "some-domain-id",
		WorkflowID: "wf-id",
		RunID:      "rid",
		TaskList:   "task-list",
	}).Times(1)
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{},
		&persistence.DomainConfig{
			Retention: defaultWorkflowRetentionInDays,
		},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1,
	)

	s.mockDomainCache.EXPECT().GetDomainByID("some-domain-id").Return(domainEntry, nil).Times(1)

	var transferTasks []persistence.Task
	transferTasks = append(transferTasks, &persistence.CloseExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   "some-domain-id",
			WorkflowID: "wf-id",
			RunID:      "rid",
		},
		TaskData: persistence.TaskData{
			Version: constants.TestVersion,
		},
		TaskList: "task-list",
	})

	s.mockMutableState.EXPECT().AddTransferTasks(transferTasks).Times(1)

	expectedDeletionTS := time.Unix(0, closeEvent.GetTimestamp()).
		Add(time.Duration(defaultWorkflowRetentionInDays) * time.Hour * 24)

	s.mockMutableState.EXPECT().AddTimerTasks(&persistence.DeleteHistoryEventTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   "some-domain-id",
			WorkflowID: "wf-id",
			RunID:      "rid",
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			VisibilityTimestamp: expectedDeletionTS,
			Version:             closeEvent.Version,
		},
		TaskList: "task-list",
	})

	err := s.taskGenerator.GenerateWorkflowCloseTasks(closeEvent, 1)

	s.NoError(err)
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateFromTransferTask() {
	now := time.Now()
	testCases := []struct {
		transferTask *persistence.TransferTaskInfo
		expectError  bool
	}{
		{
			transferTask: &persistence.TransferTaskInfo{
				TaskType: persistence.TransferTaskTypeActivityTask,
			},
			expectError: true,
		},
		{
			transferTask: &persistence.TransferTaskInfo{
				TaskType:                persistence.TransferTaskTypeCancelExecution,
				TargetDomainID:          constants.TestDomainID,
				TargetWorkflowID:        constants.TestWorkflowID,
				TargetRunID:             constants.TestRunID,
				TargetChildWorkflowOnly: false,
				ScheduleID:              int64(123),
			},
			expectError: false,
		},
		{
			transferTask: &persistence.TransferTaskInfo{
				TaskType:                persistence.TransferTaskTypeSignalExecution,
				TargetDomainID:          constants.TestDomainID,
				TargetWorkflowID:        constants.TestWorkflowID,
				TargetRunID:             constants.TestRunID,
				TargetChildWorkflowOnly: false,
				ScheduleID:              int64(123),
			},
			expectError: false,
		},
		{
			transferTask: &persistence.TransferTaskInfo{
				TaskType:         persistence.TransferTaskTypeStartChildExecution,
				TargetDomainID:   constants.TestDomainID,
				TargetWorkflowID: constants.TestWorkflowID,
				ScheduleID:       int64(123),
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		if !tc.expectError {
			tc.transferTask.Version = int64(101)
			tc.transferTask.VisibilityTimestamp = now
		}
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGetNextDecisionTimeout() {
	defaultStartToCloseTimeout := 10 * time.Second
	expectedResult := []time.Duration{
		defaultStartToCloseTimeout,
		defaultStartToCloseTimeout,
		defaultInitIntervalForDecisionRetry,
		defaultInitIntervalForDecisionRetry * 2,
		defaultInitIntervalForDecisionRetry * 4,
		defaultMaxIntervalForDecisionRetry,
		defaultMaxIntervalForDecisionRetry,
		defaultMaxIntervalForDecisionRetry,
	}
	for i := 0; i < len(expectedResult); i++ {
		next := getNextDecisionTimeout(int64(i), defaultStartToCloseTimeout)
		expected := expectedResult[i]
		min, max := getNextBackoffRange(expected)
		s.True(next >= min, "NextBackoff too low: actual: %v, expected: %v", next, expected)
		s.True(next <= max, "NextBackoff too high: actual: %v, expected: %v", next, expected)
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateWorkflowStartTasks() {
	startTime := time.Now()
	expirationTime := startTime.Add(5 * time.Second)

	testCases := []struct {
		name                string
		startEvent          *types.HistoryEvent
		workflowTimeout     int32
		visibilityTimestamp time.Time
	}{
		{
			name: "Success case - first attempt",
			startEvent: &types.HistoryEvent{
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{Attempt: 0},
				Version:                                 constants.TestVersion,
			},
			workflowTimeout:     10,
			visibilityTimestamp: startTime.Add(time.Duration(10) * time.Second),
		},
		{
			name: "Success case - second attempt and workflowTimeoutTimestamp before expirationTime",
			startEvent: &types.HistoryEvent{
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{Attempt: 1},
				Version:                                 constants.TestVersion,
			},
			workflowTimeout:     1,
			visibilityTimestamp: startTime.Add(time.Duration(1) * time.Second),
		},
		{
			name: "Success case - second attempt and workflowTimeoutTimestamp after expirationTime",
			startEvent: &types.HistoryEvent{
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{Attempt: 1},
				Version:                                 constants.TestVersion,
			},
			workflowTimeout:     6,
			visibilityTimestamp: expirationTime,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{WorkflowTimeout: tc.workflowTimeout, ExpirationTime: expirationTime, TaskList: "task-list"}).Times(1)
			s.mockMutableState.EXPECT().AddTimerTasks(&persistence.WorkflowTimeoutTask{
				TaskData: persistence.TaskData{
					VisibilityTimestamp: tc.visibilityTimestamp,
					Version:             tc.startEvent.Version,
				},
				TaskList: "task-list",
			}).Times(1)

			err := s.taskGenerator.GenerateWorkflowStartTasks(startTime, tc.startEvent)

			s.NoError(err)
		})
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateDelayedDecisionTasks() {
	timestamp := common.Int64Ptr(time.Now().UnixNano())
	firstDecisionTaskBackoffSeconds := common.Int32Ptr(1)

	testCases := []struct {
		name       string
		startEvent *types.HistoryEvent
		setupMock  func()
		err        error
	}{
		{
			name: "Success case - nil initiator",
			startEvent: &types.HistoryEvent{
				Version: constants.TestVersion,
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					Initiator:                       nil,
					FirstDecisionTaskBackoffSeconds: firstDecisionTaskBackoffSeconds,
				},
				Timestamp: timestamp,
			},
			setupMock: func() {
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				}).Times(1)
				s.mockMutableState.EXPECT().AddTimerTasks(&persistence.WorkflowBackoffTimerTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, *timestamp).Add(time.Duration(*firstDecisionTaskBackoffSeconds) * time.Second),
						Version:             constants.TestVersion,
					},
					TimeoutType: persistence.WorkflowBackoffTimeoutTypeCron,
					TaskList:    "task-list",
				}).Times(1)
			},
		},
		{
			name: "Success case - retry policy initiator",
			startEvent: &types.HistoryEvent{
				Version: constants.TestVersion,
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					Initiator:                       types.ContinueAsNewInitiatorRetryPolicy.Ptr(),
					FirstDecisionTaskBackoffSeconds: firstDecisionTaskBackoffSeconds,
				},
				Timestamp: timestamp,
			},
			setupMock: func() {
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				}).Times(1)
				s.mockMutableState.EXPECT().AddTimerTasks(&persistence.WorkflowBackoffTimerTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, *timestamp).Add(time.Duration(*firstDecisionTaskBackoffSeconds) * time.Second),
						Version:             constants.TestVersion,
					},
					TimeoutType: persistence.WorkflowBackoffTimeoutTypeRetry,
					TaskList:    "task-list",
				}).Times(1)
			},
		},
		{
			name: "Success case - cron initiator",
			startEvent: &types.HistoryEvent{
				Version: constants.TestVersion,
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					Initiator:                       types.ContinueAsNewInitiatorCronSchedule.Ptr(),
					FirstDecisionTaskBackoffSeconds: firstDecisionTaskBackoffSeconds,
				},
				Timestamp: timestamp,
			},
			setupMock: func() {
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				}).Times(1)
				s.mockMutableState.EXPECT().AddTimerTasks(&persistence.WorkflowBackoffTimerTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, *timestamp).Add(time.Duration(*firstDecisionTaskBackoffSeconds) * time.Second),
						Version:             constants.TestVersion,
					},
					TimeoutType: persistence.WorkflowBackoffTimeoutTypeCron,
					TaskList:    "task-list",
				}).Times(1)
			},
		},
		{
			name: "Error case - decider initiator",
			startEvent: &types.HistoryEvent{
				Version: constants.TestVersion,
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					Initiator:                       types.ContinueAsNewInitiatorDecider.Ptr(),
					FirstDecisionTaskBackoffSeconds: firstDecisionTaskBackoffSeconds,
				},
				Timestamp: timestamp,
			},
			setupMock: func() {},
			err:       &types.InternalServiceError{Message: "encounter continue as new iterator & first decision delay not 0"},
		},
		{
			name: "Error case - unknown initiator",
			startEvent: &types.HistoryEvent{
				Version: constants.TestVersion,
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					Initiator:                       types.ContinueAsNewInitiator(3).Ptr(),
					FirstDecisionTaskBackoffSeconds: firstDecisionTaskBackoffSeconds,
				},
				Timestamp: timestamp,
			},
			setupMock: func() {},
			err:       &types.InternalServiceError{Message: fmt.Sprintf("unknown iterator retry policy: %v", types.ContinueAsNewInitiator(3).Ptr())},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			err := s.taskGenerator.GenerateDelayedDecisionTasks(tc.startEvent)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateRecordWorkflowStartedTasks() {
	startEvent := &types.HistoryEvent{
		Version: constants.TestVersion,
	}

	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   "domain-id",
		WorkflowID: "wf-id",
		RunID:      "rid",
		TaskList:   "task-list",
	}).Times(1)
	s.mockMutableState.EXPECT().AddTransferTasks(&persistence.RecordWorkflowStartedTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   "domain-id",
			WorkflowID: "wf-id",
			RunID:      "rid",
		},
		TaskData: persistence.TaskData{
			Version: startEvent.Version,
		},
		TaskList: "task-list",
	}).Times(1)

	err := s.taskGenerator.GenerateRecordWorkflowStartedTasks(startEvent)

	s.NoError(err)
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateDecisionScheduleTasks() {
	decisionScheduleID := int64(123)

	executionInfo := &persistence.WorkflowExecutionInfo{
		DomainID:   constants.TestDomainID,
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
		TaskList:   "task-list",
	}

	decision := &DecisionInfo{
		TaskList:           "taskList",
		ScheduleID:         123,
		Version:            constants.TestVersion,
		ScheduledTimestamp: time.Now().UnixNano(),
		Attempt:            1,
	}

	testCases := []struct {
		name      string
		setupMock func()
		err       error
	}{
		{
			name: "Success case - scheduleToStartTimeout 0",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetDecisionInfo(decisionScheduleID).Return(decision, true).Times(1)
				s.mockMutableState.EXPECT().AddTransferTasks(&persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						Version: decision.Version,
					},
					TargetDomainID:   executionInfo.DomainID,
					TaskList:         decision.TaskList,
					ScheduleID:       decision.ScheduleID,
					OriginalTaskList: "task-list",
				}).Times(1)
				s.mockMutableState.EXPECT().GetDecisionScheduleToStartTimeout().Return(time.Duration(0)).Times(1)
			},
		},
		{
			name: "Success case - scheduleToStartTimeout not 0",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetDecisionInfo(decisionScheduleID).Return(decision, true).Times(1)
				s.mockMutableState.EXPECT().AddTransferTasks(&persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						Version: decision.Version,
					},
					TargetDomainID:   executionInfo.DomainID,
					TaskList:         decision.TaskList,
					ScheduleID:       decision.ScheduleID,
					OriginalTaskList: "task-list",
				}).Times(1)
				scheduleToStartTimeout := time.Duration(1)
				s.mockMutableState.EXPECT().GetDecisionScheduleToStartTimeout().Return(scheduleToStartTimeout).Times(1)
				s.mockMutableState.EXPECT().AddTimerTasks(&persistence.DecisionTimeoutTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, decision.ScheduledTimestamp).Add(scheduleToStartTimeout),
						Version:             decision.Version,
					},
					TimeoutType:     int(TimerTypeScheduleToStart),
					EventID:         decision.ScheduleID,
					ScheduleAttempt: decision.Attempt,
					TaskList:        "task-list",
				}).Times(1)
			},
		},
		{
			name: "Error case - GetDecisionInfo error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetDecisionInfo(decisionScheduleID).Return(nil, false).Times(1)
			},
			err: &types.InternalServiceError{
				Message: fmt.Sprintf("it could be a bug, cannot get pending decision: %v", decisionScheduleID),
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).Times(1)
			tc.setupMock()

			err := s.taskGenerator.GenerateDecisionScheduleTasks(decisionScheduleID)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})

	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateDecisionStartTasks() {
	seed := int64(1)
	rand.Seed(seed)
	decisionScheduleID := int64(123)
	getDecision := func() *DecisionInfo {
		return &DecisionInfo{
			Version:          constants.TestVersion,
			ScheduleID:       123,
			StartedTimestamp: time.Now().UnixNano(),
		}
	}

	testCases := []struct {
		name      string
		setupMock func()
		err       error
	}{
		{
			name: "Success case - attempt greater than 1",
			setupMock: func() {
				decision := getDecision()
				decision.Attempt = 2
				s.mockMutableState.EXPECT().GetDecisionInfo(decisionScheduleID).Return(decision, true).Times(1)
				defaultStartToCloseTimeout := int32(1)
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:                    "domain-id",
					WorkflowID:                  "wf-id",
					RunID:                       "rid",
					DecisionStartToCloseTimeout: defaultStartToCloseTimeout,
					TaskList:                    "task-list",
				}).Times(1)
				startToCloseTimeout := getNextDecisionTimeout(decision.Attempt, time.Duration(defaultStartToCloseTimeout)*time.Second)
				decision.DecisionTimeout = int32(startToCloseTimeout.Seconds())
				s.mockMutableState.EXPECT().UpdateDecision(decision).Times(1)
				s.mockMutableState.EXPECT().AddTimerTasks(&persistence.DecisionTimeoutTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, decision.StartedTimestamp).Add(startToCloseTimeout),
						Version:             decision.Version,
					},
					TimeoutType:     int(TimerTypeStartToClose),
					EventID:         decision.ScheduleID,
					ScheduleAttempt: decision.Attempt,
					TaskList:        "task-list",
				})
				rand.Seed(seed)
			},
		},
		{
			name: "Success case - attempt less or equal to 1",
			setupMock: func() {
				decision := getDecision()
				decision.DecisionTimeout = 1
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				})
				s.mockMutableState.EXPECT().GetDecisionInfo(decisionScheduleID).Return(decision, true).Times(1)
				s.mockMutableState.EXPECT().AddTimerTasks(&persistence.DecisionTimeoutTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, decision.StartedTimestamp).Add(time.Duration(decision.DecisionTimeout) * time.Second),
						Version:             decision.Version,
					},
					TimeoutType:     int(TimerTypeStartToClose),
					EventID:         decision.ScheduleID,
					ScheduleAttempt: decision.Attempt,
					TaskList:        "task-list",
				})
			},
		},
		{
			name: "Error case - GetDecisionInfo error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetDecisionInfo(decisionScheduleID).Return(nil, false).Times(1)
			},
			err: &types.InternalServiceError{
				Message: fmt.Sprintf("it could be a bug, cannot get pending decision: %v", decisionScheduleID),
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			err := s.taskGenerator.GenerateDecisionStartTasks(decisionScheduleID)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})

	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateActivityTransferTasks() {
	domain := constants.TestDomainName
	event := &types.HistoryEvent{
		ID: 123,
		ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
			Domain: &domain,
		},
	}

	getActivityInfo := func() *persistence.ActivityInfo {
		return &persistence.ActivityInfo{
			Version:    constants.TestVersion,
			TaskList:   "taskList",
			ScheduleID: 456,
		}
	}

	testCases := []struct {
		name      string
		setupMock func()
		err       error
	}{
		{
			name: "Success case - DomainID is not empty",
			setupMock: func() {
				activityInfo := getActivityInfo()
				activityInfo.DomainID = constants.TestDomainID
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				})
				s.mockMutableState.EXPECT().GetActivityInfo(event.ID).Return(activityInfo, true).Times(1)
				s.mockMutableState.EXPECT().AddTransferTasks(&persistence.ActivityTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						Version: activityInfo.Version,
					},
					TargetDomainID: activityInfo.DomainID,
					TaskList:       activityInfo.TaskList,
					ScheduleID:     activityInfo.ScheduleID,
				}).Times(1)
			},
		},
		{
			name: "Success case - DomainID is empty",
			setupMock: func() {
				activityInfo := getActivityInfo()
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				})
				s.mockMutableState.EXPECT().GetActivityInfo(event.ID).Return(activityInfo, true).Times(1)
				s.mockDomainCache.EXPECT().GetDomainID(event.ActivityTaskScheduledEventAttributes.GetDomain()).Return(event.ActivityTaskScheduledEventAttributes.GetDomain(), nil).Times(1)
				s.mockMutableState.EXPECT().AddTransferTasks(&persistence.ActivityTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						Version: activityInfo.Version,
					},
					TargetDomainID: event.ActivityTaskScheduledEventAttributes.GetDomain(),
					TaskList:       activityInfo.TaskList,
					ScheduleID:     activityInfo.ScheduleID,
				}).Times(1)
			},
		},
		{
			name: "Error case - GetActivityInfo error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetActivityInfo(event.ID).Return(nil, false).Times(1)
			},
			err: &types.InternalServiceError{
				Message: fmt.Sprintf("it could be a bug, cannot get pending activity: %v", event.ID),
			},
		},
		{
			name: "Error case - getTargetDomainID error",
			setupMock: func() {
				activityInfo := getActivityInfo()
				s.mockMutableState.EXPECT().GetActivityInfo(event.ID).Return(activityInfo, true).Times(1)
				s.mockDomainCache.EXPECT().GetDomainID(event.ActivityTaskScheduledEventAttributes.GetDomain()).
					Return("", errors.New("get-target-domain-id-error")).Times(1)
			},
			err: errors.New("get-target-domain-id-error"),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			err := s.taskGenerator.GenerateActivityTransferTasks(event)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateActivityRetryTasks() {
	activityScheduleID := int64(123)

	testCases := []struct {
		name      string
		setupMock func()
		err       error
	}{
		{
			name: "Success case",
			setupMock: func() {
				ai := &persistence.ActivityInfo{
					Version:       constants.TestVersion,
					ScheduledTime: time.Now(),
					ScheduleID:    activityScheduleID,
					Attempt:       1,
					TaskList:      "task-list2",
				}
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				})
				s.mockMutableState.EXPECT().GetActivityInfo(activityScheduleID).Return(ai, true).Times(1)
				s.mockMutableState.EXPECT().AddTimerTasks(&persistence.ActivityRetryTimerTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						Version:             ai.Version,
						VisibilityTimestamp: ai.ScheduledTime,
					},
					EventID:  ai.ScheduleID,
					Attempt:  int64(ai.Attempt),
					TaskList: "task-list2",
				}).Times(1)
			},
		},
		{
			name: "Error case - GetActivityInfo error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetActivityInfo(activityScheduleID).Return(nil, false).Times(1)
			},
			err: &types.InternalServiceError{
				Message: fmt.Sprintf("it could be a bug, cannot get pending activity: %v", activityScheduleID),
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			err := s.taskGenerator.GenerateActivityRetryTasks(activityScheduleID)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})

	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateChildWorkflowTasks() {
	eventID := int64(123)

	getChildWorkflowInfo := func() *persistence.ChildExecutionInfo {
		return &persistence.ChildExecutionInfo{
			Version:           constants.TestVersion,
			InitiatedID:       123,
			StartedWorkflowID: constants.TestWorkflowID,
		}
	}

	parentDomain := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{
			ID:   constants.TestDomainID,
			Name: constants.TestDomainName,
		},
		&persistence.DomainConfig{},
		nil,
		0,
	)

	testCases := []struct {
		name       string
		setupMock  func()
		domainName string
		err        error
	}{
		{
			name: "Success case - targetDomainID is not empty and isCrossClusterTask is false",
			setupMock: func() {
				childWorkflowInfo := getChildWorkflowInfo()
				childWorkflowInfo.DomainID = constants.TestDomainID
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				})
				s.mockMutableState.EXPECT().GetDomainEntry().Return(parentDomain).Times(1)
				s.mockMutableState.EXPECT().GetChildExecutionInfo(eventID).Return(childWorkflowInfo, true).Times(1)
				s.mockMutableState.EXPECT().AddTransferTasks(&persistence.StartChildExecutionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						Version: childWorkflowInfo.Version,
					},
					TargetDomainID:   childWorkflowInfo.DomainID,
					TargetWorkflowID: childWorkflowInfo.StartedWorkflowID,
					InitiatedID:      childWorkflowInfo.InitiatedID,
					TaskList:         "task-list",
				}).Times(1)
			},
		},
		{
			name:       "Success case - targetDomainID is empty - the expectations is that this should default to the existing parent workflow's domain",
			domainName: "",
			setupMock: func() {
				childWorkflowInfo := getChildWorkflowInfo()

				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				})
				s.mockMutableState.EXPECT().GetDomainEntry().Return(parentDomain).Times(1)
				s.mockMutableState.EXPECT().GetChildExecutionInfo(eventID).Return(childWorkflowInfo, true).Times(1)
				s.mockMutableState.EXPECT().AddTransferTasks(&persistence.StartChildExecutionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						Version: childWorkflowInfo.Version,
					},
					TargetDomainID:   constants.TestDomainID,
					TargetWorkflowID: childWorkflowInfo.StartedWorkflowID,
					InitiatedID:      childWorkflowInfo.InitiatedID,
					TaskList:         "task-list",
				}).Times(1)
			},
		},
		{
			name:       "targetDomainID different to child's - this is invalid and should be an error",
			domainName: "child-domain-B",
			setupMock: func() {

				childWorkflowInfo := &persistence.ChildExecutionInfo{
					Version:           constants.TestVersion,
					InitiatedID:       123,
					StartedWorkflowID: constants.TestWorkflowID,
					DomainID:          "child-domain-B",
				}

				cacheEntry := cache.NewGlobalDomainCacheEntryForTest(nil, nil, nil, 0)

				s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cacheEntry, nil).Times(1)
				s.mockMutableState.EXPECT().GetChildExecutionInfo(eventID).Return(childWorkflowInfo, true).Times(1)
				s.mockMutableState.EXPECT().GetDomainEntry().Return(parentDomain).Times(2)
			},
			err: &types.BadRequestError{
				Message: "The child workflow is trying to use domain child-domain-B but it's running in domain deadbeef-0123-4567-890a-bcdef0123456. Cross-cluster and cross domain child workflows are not supported for global domains",
			},
		},
		{
			name: "Error case - GetChildExecutionInfo error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetChildExecutionInfo(eventID).Return(nil, false).Times(1)
			},
			err: &types.InternalServiceError{
				Message: fmt.Sprintf("it could be a bug, cannot get pending child workflow: %v", eventID),
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			event := &types.HistoryEvent{
				StartChildWorkflowExecutionInitiatedEventAttributes: &types.StartChildWorkflowExecutionInitiatedEventAttributes{
					Domain: tc.domainName,
				},
				ID: eventID,
			}

			err := s.taskGenerator.GenerateChildWorkflowTasks(event)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateRequestCancelExternalTasks() {
	event := &types.HistoryEvent{
		RequestCancelExternalWorkflowExecutionInitiatedEventAttributes: &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
			Domain: constants.TestDomainName,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
		},
		ID:      123,
		Version: constants.TestVersion,
	}

	testCases := []struct {
		name      string
		setupMock func()
		err       error
	}{
		{
			name: "Error case - GetRequestCancelInfo error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetRequestCancelInfo(event.ID).Return(nil, false).Times(1)
			},
			err: &types.InternalServiceError{
				Message: fmt.Sprintf("it could be a bug, cannot get pending request cancel external workflow: %v", event.ID),
			},
		},
		{
			name: "Error case - getTargetDomainID error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetRequestCancelInfo(event.ID).Return(&persistence.RequestCancelInfo{}, true).Times(1)
				s.mockDomainCache.EXPECT().GetDomainID(event.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes.GetDomain()).
					Return("", errors.New("get-target-domain-id-error")).Times(1)
			},
			err: errors.New("get-target-domain-id-error"),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			err := s.taskGenerator.GenerateRequestCancelExternalTasks(event)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateSignalExternalTasks() {
	event := &types.HistoryEvent{
		SignalExternalWorkflowExecutionInitiatedEventAttributes: &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
			Domain: constants.TestDomainName,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
		},
		ID:      123,
		Version: constants.TestVersion,
	}

	testCases := []struct {
		name      string
		setupMock func()
		err       error
	}{
		{
			name: "Success case",
			setupMock: func() {
				targetDomainID := "target-domain-id"
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "rid",
					TaskList:   "task-list",
				})
				s.mockMutableState.EXPECT().GetSignalInfo(event.ID).Return(nil, true).Times(1)
				s.mockDomainCache.EXPECT().GetDomainID(event.SignalExternalWorkflowExecutionInitiatedEventAttributes.GetDomain()).
					Return(targetDomainID, nil).Times(1)
				s.mockMutableState.EXPECT().AddTransferTasks(&persistence.SignalExecutionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "domain-id",
						WorkflowID: "wf-id",
						RunID:      "rid",
					},
					TaskData: persistence.TaskData{
						Version: event.Version,
					},
					TargetDomainID:   targetDomainID,
					TargetWorkflowID: event.SignalExternalWorkflowExecutionInitiatedEventAttributes.WorkflowExecution.GetWorkflowID(),
					TargetRunID:      event.SignalExternalWorkflowExecutionInitiatedEventAttributes.WorkflowExecution.GetRunID(),
					InitiatedID:      event.ID,
					TaskList:         "task-list",
				}).Times(1)
			},
		},
		{
			name: "Error case - GetSignalInfo error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetSignalInfo(event.ID).Return(nil, false).Times(1)
			},
			err: &types.InternalServiceError{
				Message: fmt.Sprintf("it could be a bug, cannot get pending signal external workflow: %v", event.ID),
			},
		},
		{
			name: "Error case - getTargetDomainID error",
			setupMock: func() {
				s.mockMutableState.EXPECT().GetSignalInfo(event.ID).Return(&persistence.SignalInfo{}, true).Times(1)
				s.mockDomainCache.EXPECT().GetDomainID(event.SignalExternalWorkflowExecutionInitiatedEventAttributes.GetDomain()).
					Return("", errors.New("get-target-domain-id-error")).Times(1)
			},
			err: errors.New("get-target-domain-id-error"),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			err := s.taskGenerator.GenerateSignalExternalTasks(event)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateWorkflowSearchAttrTasks() {
	version := int64(123)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   "domain-id",
		WorkflowID: "wf-id",
		RunID:      "run-id",
		TaskList:   "task-list",
	}).Times(1)
	s.mockMutableState.EXPECT().GetCurrentVersion().Return(version).Times(1)
	s.mockMutableState.EXPECT().AddTransferTasks(&persistence.UpsertWorkflowSearchAttributesTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   "domain-id",
			WorkflowID: "wf-id",
			RunID:      "run-id",
		},
		TaskData: persistence.TaskData{
			Version: version,
		},
		TaskList: "task-list",
	}).Times(1)

	err := s.taskGenerator.GenerateWorkflowSearchAttrTasks()

	s.NoError(err)
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateWorkflowResetTasks() {
	version := int64(123)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   "domain-id",
		WorkflowID: "wf-id",
		RunID:      "run-id",
		TaskList:   "task-list",
	}).Times(1)
	s.mockMutableState.EXPECT().GetCurrentVersion().Return(version).Times(1)
	s.mockMutableState.EXPECT().AddTransferTasks(&persistence.ResetWorkflowTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   "domain-id",
			WorkflowID: "wf-id",
			RunID:      "run-id",
		},
		TaskData: persistence.TaskData{
			Version: version,
		},
		TaskList: "task-list",
	}).Times(1)

	err := s.taskGenerator.GenerateWorkflowResetTasks()

	s.NoError(err)
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateActivityTimerTasks() {
	s.mockMutableState.EXPECT().GetPendingActivityInfos().Return(nil).Times(1)

	err := s.taskGenerator.GenerateActivityTimerTasks()

	s.NoError(err)
}

func (s *mutableStateTaskGeneratorSuite) TestGenerateUserTimerTasks() {
	s.mockMutableState.EXPECT().GetPendingTimerInfos().Return(nil).Times(1)

	err := s.taskGenerator.GenerateUserTimerTasks()

	s.NoError(err)
}

func (s *mutableStateTaskGeneratorSuite) TestGetTargetDomainID() {
	testCases := []struct {
		name             string
		setupMock        func(targetDomainName string, domainID string)
		targetDomainName string
		domainID         string
		err              error
	}{
		{
			name: "Success case - empty targetDomainName",
			setupMock: func(targetDomainName string, domainID string) {
				s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{DomainID: domainID}).Times(1)
			},
			domainID:         constants.TestDomainID,
			targetDomainName: "",
		},
		{
			name: "Success case - targetDomainName is not empty",
			setupMock: func(targetDomainName string, domainID string) {
				s.mockDomainCache.EXPECT().GetDomainID(targetDomainName).Return(domainID, nil).Times(1)
			},
			domainID:         constants.TestDomainID,
			targetDomainName: constants.TestDomainName,
		},
		{
			name: "Error case - GetDomainID error",
			setupMock: func(targetDomainName string, domainID string) {
				s.mockDomainCache.EXPECT().GetDomainID(targetDomainName).Return("", errors.New("get-domain-id-error")).Times(1)
			},
			targetDomainName: constants.TestDomainName,
			err:              errors.New("get-domain-id-error"),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMock(tc.targetDomainName, tc.domainID)

			targetDomainID, err := s.taskGenerator.getTargetDomainID(tc.targetDomainName)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(tc.domainID, targetDomainID)
			}
		})

	}
}

func getNextBackoffRange(duration time.Duration) (time.Duration, time.Duration) {
	rangeMin := time.Duration((1 - defaultJitterCoefficient) * float64(duration))
	return rangeMin, duration
}

func TestValidationOfChildWorkflowParameters(t *testing.T) {

	g1 := cache.NewGlobalDomainCacheEntryForTest(&persistence.DomainInfo{ID: "g1", Name: "g1"}, nil, nil, 0)
	g2 := cache.NewGlobalDomainCacheEntryForTest(&persistence.DomainInfo{ID: "g2", Name: "g2"}, nil, nil, 0)
	l1 := cache.NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{ID: "l1", Name: "l1"}, nil, "")
	l2 := cache.NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{ID: "l2", Name: "l2"}, nil, "")

	tests := map[string]struct {
		thisDomain    *cache.DomainCacheEntry
		childWorkflow *cache.DomainCacheEntry
		setupCache    func(mockCache *cache.MockDomainCache)
		expectedError error
	}{
		"Normal case: a child workflow running from the same domain as the parent shouldn't see any errors": {
			thisDomain:    g1,
			childWorkflow: g1,
			setupCache:    func(mockCache *cache.MockDomainCache) {},
			expectedError: nil,
		},
		"local domains, cross-domain call": {
			thisDomain:    l1,
			childWorkflow: l2,
			setupCache: func(cache *cache.MockDomainCache) {
				cache.EXPECT().GetDomainByID(l2.GetInfo().ID).Return(l2, nil).Times(1)
			},
			expectedError: nil,
		},
		"Global domains cross domain call. This is not permitted": {
			thisDomain:    g1,
			childWorkflow: g2,
			setupCache: func(cache *cache.MockDomainCache) {
				cache.EXPECT().GetDomainByID(g2.GetInfo().ID).Return(g2, nil).Times(1)
			},
			expectedError: &types.BadRequestError{Message: "The child workflow is trying to use domain g2 but it's running in domain g1. Cross-cluster and cross domain child workflows are not supported for global domains"},
		},
		"Global domains cross domain call 2. This is not permitted": {
			thisDomain:    l1,
			childWorkflow: g2,
			setupCache: func(cache *cache.MockDomainCache) {
				cache.EXPECT().GetDomainByID(g2.GetInfo().ID).Return(g2, nil).Times(1)
			},
			expectedError: &types.BadRequestError{Message: "The child workflow is trying to use domain g2 but it's running in domain l1. Cross-cluster and cross domain child workflows are not supported for global domains"},
		},
		"Global domains cross domain call 3. This is not permitted": {
			thisDomain:    g1,
			childWorkflow: l2,
			setupCache: func(cache *cache.MockDomainCache) {
				cache.EXPECT().GetDomainByID(l2.GetInfo().ID).Return(g2, nil).Times(1)
			},
			expectedError: &types.BadRequestError{Message: "The child workflow is trying to use domain l2 but it's running in domain g1. Cross-cluster and cross domain child workflows are not supported for global domains"},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			msb := mutableStateBuilder{
				domainEntry: td.thisDomain,
			}

			ctrl := gomock.NewController(t)
			domainCache := cache.NewMockDomainCache(ctrl)
			td.setupCache(domainCache)

			mstb := mutableStateTaskGeneratorImpl{
				mutableState: &msb,
				domainCache:  domainCache,
			}

			err := mstb.validateChildWorkflowParameters(td.thisDomain.GetInfo().ID, td.childWorkflow.GetInfo().ID)
			assert.Equal(t, td.expectedError, err)
		})
	}
}

func GenerateWorkflowCloseTasksTestCases(retention time.Duration, closeEvent *types.HistoryEvent, now time.Time) []struct {
	setupFn        func(mockMutableState *MockMutableState)
	generatedTasks []persistence.Task
} {
	version := int64(123)
	return []struct {
		setupFn        func(mockMutableState *MockMutableState)
		generatedTasks []persistence.Task
	}{
		{
			// no parent, no children
			setupFn: func(mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					TaskList:   "task-list",
				}).AnyTimes()
				mockMutableState.EXPECT().HasParentExecution().Return(false).AnyTimes()
				mockMutableState.EXPECT().GetPendingChildExecutionInfos().Return(nil).AnyTimes()
			},
			generatedTasks: []persistence.Task{
				&persistence.CloseExecutionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: now,
						Version:             version,
					},
					TaskList: "task-list",
				},
				&persistence.DeleteHistoryEventTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, closeEvent.GetTimestamp()).Add(retention),
						Version:             version,
					},
					TaskList: "task-list",
				},
			},
		},
		{
			// parent and children all active in current cluster
			setupFn: func(mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:         constants.TestDomainID,
					WorkflowID:       constants.TestWorkflowID,
					RunID:            constants.TestRunID,
					ParentDomainID:   constants.TestDomainID,
					ParentWorkflowID: "parent workflowID",
					ParentRunID:      "parent runID",
					InitiatedID:      101,
					CloseStatus:      persistence.WorkflowCloseStatusCompleted,
					TaskList:         "task-list",
				}).AnyTimes()
				mockMutableState.EXPECT().HasParentExecution().Return(true).AnyTimes()
				mockMutableState.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{
					102: {DomainID: constants.TestDomainID, ParentClosePolicy: types.ParentClosePolicyTerminate},
					103: {DomainID: constants.TestDomainID, ParentClosePolicy: types.ParentClosePolicyAbandon},
				}).AnyTimes()
			},
			generatedTasks: []persistence.Task{
				&persistence.CloseExecutionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: now,
						Version:             version,
					},
					TaskList: "task-list",
				},
				&persistence.DeleteHistoryEventTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, closeEvent.GetTimestamp()).Add(retention),
						Version:             version,
					},
					TaskList: "task-list",
				},
			},
		},
		{
			// no parent, no children
			setupFn: func(mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					TaskList:   "task-list",
				}).AnyTimes()
				mockMutableState.EXPECT().HasParentExecution().Return(false).AnyTimes()
				mockMutableState.EXPECT().GetPendingChildExecutionInfos().Return(nil).AnyTimes()
			},
			generatedTasks: []persistence.Task{
				&persistence.CloseExecutionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: now,
						Version:             version,
					},
					TaskList: "task-list",
				},
				&persistence.DeleteHistoryEventTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Unix(0, closeEvent.GetTimestamp()).Add(retention),
						Version:             version,
					},
					TaskList: "task-list",
				},
			},
		},
	}
}
