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
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/shard"
)

type (
	stateBuilderSuite struct {
		suite.Suite
		*require.Assertions

		controller       *gomock.Controller
		mockShard        *shard.TestContext
		mockEventsCache  *events.MockCache
		mockDomainCache  *cache.MockDomainCache
		mockMutableState *MockMutableState
		logger           log.Logger

		sourceCluster string
		stateBuilder  *stateBuilderImpl
	}
)

func TestStateBuilderSuite(t *testing.T) {
	s := new(stateBuilderSuite)
	suite.Run(t, s)
}

func (s *stateBuilderSuite) SetupSuite() {

}

func (s *stateBuilderSuite) TearDownSuite() {

}

func (s *stateBuilderSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockMutableState = NewMockMutableState(s.controller)

	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          0,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest(),
	)

	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockEventsCache = s.mockShard.MockEventsCache
	s.mockEventsCache.EXPECT().PutEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	s.logger = s.mockShard.GetLogger()

	s.mockMutableState.EXPECT().GetVersionHistories().Return(persistence.NewVersionHistories(&persistence.VersionHistory{})).AnyTimes()

	s.stateBuilder = NewStateBuilder(
		s.mockShard,
		s.logger,
		s.mockMutableState,
	).(*stateBuilderImpl)
	s.sourceCluster = "some random source cluster"
}

func (s *stateBuilderSuite) TearDownTest() {
	s.stateBuilder = nil
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *stateBuilderSuite) mockUpdateVersion(events ...*types.HistoryEvent) {
	for _, event := range events {
		s.mockMutableState.EXPECT().UpdateCurrentVersion(event.Version, true).Times(1)
	}
	s.mockMutableState.EXPECT().SetHistoryBuilder(NewHistoryBuilderFromEvents(events, s.mockMutableState)).Times(1)
}

func (s *stateBuilderSuite) toHistory(events ...*types.HistoryEvent) []*types.HistoryEvent {
	return events
}

// workflow operations

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionStarted_NoCronSchedule() {
	cronSchedule := ""
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	executionInfo := &persistence.WorkflowExecutionInfo{
		WorkflowTimeout: 100,
		CronSchedule:    cronSchedule,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionStarted
	startWorkflowAttribute := &types.WorkflowExecutionStartedEventAttributes{
		ParentWorkflowDomain: common.StringPtr(constants.TestDomainName),
	}

	event := &types.HistoryEvent{
		Version:                                 version,
		ID:                                      1,
		Timestamp:                               common.Int64Ptr(now.UnixNano()),
		EventType:                               &evenType,
		WorkflowExecutionStartedEventAttributes: startWorkflowAttribute,
	}

	s.mockDomainCache.EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestGlobalParentDomainEntry, nil).Times(1)
	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionStartedEvent(&constants.TestDomainID, workflowExecution, requestID, event, true).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)
	s.mockMutableState.EXPECT().SetHistoryTree(constants.TestRunID).Return(nil).Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionStarted_WithCronSchedule() {
	cronSchedule := "* * * * *"
	parsedSchedule, err := backoff.ValidateSchedule(cronSchedule)
	require.NoError(s.T(), err)
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	executionInfo := &persistence.WorkflowExecutionInfo{
		WorkflowTimeout: 100,
		CronSchedule:    cronSchedule,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionStarted
	next, err := backoff.GetBackoffForNextSchedule(parsedSchedule, now, now, 0)
	require.NoError(s.T(), err)
	startWorkflowAttribute := &types.WorkflowExecutionStartedEventAttributes{
		ParentWorkflowDomainID: common.StringPtr(constants.TestDomainID),
		ParentWorkflowDomain:   common.StringPtr(constants.TestDomainName),
		Initiator:              types.ContinueAsNewInitiatorCronSchedule.Ptr(),
		FirstDecisionTaskBackoffSeconds: common.Int32Ptr(
			int32(next.Seconds()),
		),
	}

	event := &types.HistoryEvent{
		Version:                                 version,
		ID:                                      1,
		Timestamp:                               common.Int64Ptr(now.UnixNano()),
		EventType:                               &evenType,
		WorkflowExecutionStartedEventAttributes: startWorkflowAttribute,
	}

	s.mockDomainCache.EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestGlobalParentDomainEntry, nil).Times(0)
	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionStartedEvent(&constants.TestDomainID, workflowExecution, requestID, event, true).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)
	s.mockMutableState.EXPECT().SetHistoryTree(constants.TestRunID).Return(nil).Times(1)

	_, err = s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionTimedOut() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionTimedOut
	event := &types.HistoryEvent{
		Version:                                  version,
		ID:                                       130,
		Timestamp:                                common.Int64Ptr(now.UnixNano()),
		EventType:                                &evenType,
		WorkflowExecutionTimedOutEventAttributes: &types.WorkflowExecutionTimedOutEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionTimedoutEvent(event.ID, event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionTerminated() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionTerminated
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		WorkflowExecutionTerminatedEventAttributes: &types.WorkflowExecutionTerminatedEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionTerminatedEvent(event.ID, event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)
	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionFailed() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionFailed
	event := &types.HistoryEvent{
		Version:                                version,
		ID:                                     130,
		Timestamp:                              common.Int64Ptr(now.UnixNano()),
		EventType:                              &evenType,
		WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionFailedEvent(event.ID, event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionCompleted() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionCompleted
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionCompletedEvent(event.ID, event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionCanceled() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionCanceled
	event := &types.HistoryEvent{
		Version:                                  version,
		ID:                                       130,
		Timestamp:                                common.Int64Ptr(now.UnixNano()),
		EventType:                                &evenType,
		WorkflowExecutionCanceledEventAttributes: &types.WorkflowExecutionCanceledEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionCanceledEvent(event.ID, event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).AnyTimes()
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionContinuedAsNew() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}
	parentWorkflowID := "some random parent workflow ID"
	parentRunID := uuid.New()
	parentInitiatedEventID := int64(144)

	now := time.Now()
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeoutSecond := int32(110)
	decisionTimeoutSecond := int32(11)
	newRunID := uuid.New()

	continueAsNewEvent := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
		WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
			NewExecutionRunID: newRunID,
		},
	}

	newRunStartedEvent := &types.HistoryEvent{
		Version:   version,
		ID:        1,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
			ParentWorkflowDomainID: common.StringPtr(constants.TestDomainID),
			ParentWorkflowDomain:   common.StringPtr(constants.TestDomainName),
			ParentWorkflowExecution: &types.WorkflowExecution{
				WorkflowID: parentWorkflowID,
				RunID:      parentRunID,
			},
			ParentInitiatedEventID:              common.Int64Ptr(parentInitiatedEventID),
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(workflowTimeoutSecond),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(decisionTimeoutSecond),
			TaskList:                            &types.TaskList{Name: tasklist},
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
		},
	}

	newRunSignalEvent := &types.HistoryEvent{
		Version:   version,
		ID:        2,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
			SignalName: "some random signal name",
			Input:      []byte("some random signal input"),
			Identity:   "some random identity",
		},
	}

	newRunDecisionAttempt := int64(123)
	newRunDecisionEvent := &types.HistoryEvent{
		Version:   version,
		ID:        3,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
		DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
			TaskList:                   &types.TaskList{Name: tasklist},
			StartToCloseTimeoutSeconds: common.Int32Ptr(decisionTimeoutSecond),
			Attempt:                    newRunDecisionAttempt,
		},
	}
	newRunEvents := []*types.HistoryEvent{
		newRunStartedEvent, newRunSignalEvent, newRunDecisionEvent,
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionContinuedAsNewEvent(
		continueAsNewEvent.ID,
		constants.TestDomainID,
		continueAsNewEvent,
	).Return(nil).Times(1)
	s.mockMutableState.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).AnyTimes()
	s.mockUpdateVersion(continueAsNewEvent)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	// new workflow domain
	s.mockDomainCache.EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestGlobalParentDomainEntry, nil).AnyTimes()

	newRunStateBuilder, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(continueAsNewEvent), newRunEvents)
	s.Nil(err)
	s.NotNil(newRunStateBuilder)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionContinuedAsNew_EmptyNewRunHistory() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	newRunID := uuid.New()

	continueAsNewEvent := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
		WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
			NewExecutionRunID: newRunID,
		},
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionContinuedAsNewEvent(
		continueAsNewEvent.ID,
		constants.TestDomainID,
		continueAsNewEvent,
	).Return(nil).Times(1)
	s.mockMutableState.EXPECT().GetDomainEntry().Return(constants.TestGlobalDomainEntry).AnyTimes()
	s.mockUpdateVersion(continueAsNewEvent)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	// new workflow domain
	s.mockDomainCache.EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestGlobalParentDomainEntry, nil).AnyTimes()
	newRunStateBuilder, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(continueAsNewEvent), nil)
	s.Nil(err)
	s.Nil(newRunStateBuilder)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionSignaled() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionSignaled
	event := &types.HistoryEvent{
		Version:                                  version,
		ID:                                       130,
		Timestamp:                                common.Int64Ptr(now.UnixNano()),
		EventType:                                &evenType,
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{},
	}
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionSignaled(event).Return(nil).Times(1)
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeWorkflowExecutionCancelRequested() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}
	now := time.Now()
	evenType := types.EventTypeWorkflowExecutionCancelRequested
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		WorkflowExecutionCancelRequestedEventAttributes: &types.WorkflowExecutionCancelRequestedEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateWorkflowExecutionCancelRequestedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeUpsertWorkflowSearchAttributes() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeUpsertWorkflowSearchAttributes
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		UpsertWorkflowSearchAttributesEventAttributes: &types.UpsertWorkflowSearchAttributesEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateUpsertWorkflowSearchAttributesEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeMarkerRecorded() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeMarkerRecorded
	event := &types.HistoryEvent{
		Version:                       version,
		ID:                            130,
		Timestamp:                     common.Int64Ptr(now.UnixNano()),
		EventType:                     &evenType,
		MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{},
	}
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

// decision operations

func (s *stateBuilderSuite) TestApplyEvents_EventTypeDecisionTaskScheduled() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	tasklist := "some random tasklist"
	timeoutSecond := int32(11)
	evenType := types.EventTypeDecisionTaskScheduled
	decisionAttempt := int64(111)
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
			TaskList:                   &types.TaskList{Name: tasklist},
			StartToCloseTimeoutSeconds: common.Int32Ptr(timeoutSecond),
			Attempt:                    decisionAttempt,
		},
	}
	di := &DecisionInfo{
		Version:         event.Version,
		ScheduleID:      event.ID,
		StartedID:       commonconstants.EmptyEventID,
		RequestID:       commonconstants.EmptyUUID,
		DecisionTimeout: timeoutSecond,
		TaskList:        tasklist,
		Attempt:         decisionAttempt,
	}
	executionInfo := &persistence.WorkflowExecutionInfo{
		TaskList: tasklist,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	s.mockMutableState.EXPECT().ReplicateDecisionTaskScheduledEvent(
		event.Version, event.ID, tasklist, timeoutSecond, decisionAttempt, event.GetTimestamp(), event.GetTimestamp(), false,
	).Return(di, nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}
func (s *stateBuilderSuite) TestApplyEvents_EventTypeDecisionTaskStarted() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	tasklist := "some random tasklist"
	timeoutSecond := int32(11)
	scheduleID := int64(111)
	decisionRequestID := uuid.New()
	evenType := types.EventTypeDecisionTaskStarted
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
			ScheduledEventID: scheduleID,
			RequestID:        decisionRequestID,
		},
	}
	di := &DecisionInfo{
		Version:         event.Version,
		ScheduleID:      scheduleID,
		StartedID:       event.ID,
		RequestID:       decisionRequestID,
		DecisionTimeout: timeoutSecond,
		TaskList:        tasklist,
		Attempt:         0,
	}
	s.mockMutableState.EXPECT().ReplicateDecisionTaskStartedEvent(
		(*DecisionInfo)(nil), event.Version, scheduleID, event.ID, decisionRequestID, event.GetTimestamp(),
	).Return(di, nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeDecisionTaskTimedOut() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	scheduleID := int64(12)
	startedID := int64(28)
	evenType := types.EventTypeDecisionTaskTimedOut
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
			ScheduledEventID: scheduleID,
			StartedEventID:   startedID,
			TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
		},
	}
	s.mockMutableState.EXPECT().ReplicateDecisionTaskTimedOutEvent(event).Return(nil).Times(1)
	tasklist := "some random tasklist"
	executionInfo := &persistence.WorkflowExecutionInfo{
		TaskList: tasklist,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	s.mockMutableState.EXPECT().ReplicateTransientDecisionTaskScheduled().Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeDecisionTaskFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	scheduleID := int64(12)
	startedID := int64(28)
	evenType := types.EventTypeDecisionTaskFailed
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
			ScheduledEventID: scheduleID,
			StartedEventID:   startedID,
		},
	}
	s.mockMutableState.EXPECT().ReplicateDecisionTaskFailedEvent(event).Return(nil).Times(1)
	tasklist := "some random tasklist"
	executionInfo := &persistence.WorkflowExecutionInfo{
		TaskList: tasklist,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	s.mockMutableState.EXPECT().ReplicateTransientDecisionTaskScheduled().Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeDecisionTaskCompleted() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	scheduleID := int64(12)
	startedID := int64(28)
	evenType := types.EventTypeDecisionTaskCompleted
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
			ScheduledEventID: scheduleID,
			StartedEventID:   startedID,
		},
	}
	s.mockMutableState.EXPECT().ReplicateDecisionTaskCompletedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

// user timer operations

func (s *stateBuilderSuite) TestApplyEvents_EventTypeTimerStarted() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	timerID := "timer ID"
	timeoutSecond := int64(10)
	evenType := types.EventTypeTimerStarted
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		TimerStartedEventAttributes: &types.TimerStartedEventAttributes{
			TimerID:                   timerID,
			StartToFireTimeoutSeconds: common.Int64Ptr(timeoutSecond),
		},
	}
	ti := &persistence.TimerInfo{
		Version:    event.Version,
		TimerID:    timerID,
		ExpiryTime: time.Unix(0, event.GetTimestamp()).Add(time.Duration(timeoutSecond) * time.Second),
		StartedID:  event.ID,
		TaskStatus: TimerTaskStatusNone,
	}
	s.mockMutableState.EXPECT().ReplicateTimerStartedEvent(event).Return(ti, nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeTimerFired() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeTimerFired
	event := &types.HistoryEvent{
		Version:                   version,
		ID:                        130,
		Timestamp:                 common.Int64Ptr(now.UnixNano()),
		EventType:                 &evenType,
		TimerFiredEventAttributes: &types.TimerFiredEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateTimerFiredEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeCancelTimerFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeCancelTimerFailed
	event := &types.HistoryEvent{
		Version:                          version,
		ID:                               130,
		Timestamp:                        common.Int64Ptr(now.UnixNano()),
		EventType:                        &evenType,
		CancelTimerFailedEventAttributes: &types.CancelTimerFailedEventAttributes{},
	}
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)
	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeTimerCanceled() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()

	evenType := types.EventTypeTimerCanceled
	event := &types.HistoryEvent{
		Version:                      version,
		ID:                           130,
		Timestamp:                    common.Int64Ptr(now.UnixNano()),
		EventType:                    &evenType,
		TimerCanceledEventAttributes: &types.TimerCanceledEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateTimerCanceledEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

// activity operations

func (s *stateBuilderSuite) TestApplyEvents_EventTypeActivityTaskScheduled() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	activityID := "activity ID"
	tasklist := "some random tasklist"
	timeoutSecond := int32(10)
	evenType := types.EventTypeActivityTaskScheduled
	event := &types.HistoryEvent{
		Version:                              version,
		ID:                                   130,
		Timestamp:                            common.Int64Ptr(now.UnixNano()),
		EventType:                            &evenType,
		ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{},
	}

	ai := &persistence.ActivityInfo{
		Version:                  event.Version,
		ScheduleID:               event.ID,
		ScheduledEventBatchID:    event.ID,
		ScheduledEvent:           event,
		ScheduledTime:            time.Unix(0, event.GetTimestamp()),
		StartedID:                commonconstants.EmptyEventID,
		StartedTime:              time.Time{},
		ActivityID:               activityID,
		ScheduleToStartTimeout:   timeoutSecond,
		ScheduleToCloseTimeout:   timeoutSecond,
		StartToCloseTimeout:      timeoutSecond,
		HeartbeatTimeout:         timeoutSecond,
		CancelRequested:          false,
		CancelRequestID:          commonconstants.EmptyEventID,
		LastHeartBeatUpdatedTime: time.Time{},
		TimerTaskStatus:          TimerTaskStatusNone,
		TaskList:                 tasklist,
	}
	executionInfo := &persistence.WorkflowExecutionInfo{
		TaskList: tasklist,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	s.mockMutableState.EXPECT().ReplicateActivityTaskScheduledEvent(event.ID, event, false).Return(ai, nil).Times(1)
	s.mockUpdateVersion(event)
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeActivityTaskStarted() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	tasklist := "some random tasklist"
	evenType := types.EventTypeActivityTaskScheduled
	scheduledEvent := &types.HistoryEvent{
		Version:                              version,
		ID:                                   130,
		Timestamp:                            common.Int64Ptr(now.UnixNano()),
		EventType:                            &evenType,
		ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{},
	}

	evenType = types.EventTypeActivityTaskStarted
	startedEvent := &types.HistoryEvent{
		Version:                            version,
		ID:                                 scheduledEvent.ID + 1,
		Timestamp:                          common.Int64Ptr(scheduledEvent.GetTimestamp() + 1000),
		EventType:                          &evenType,
		ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{},
	}

	executionInfo := &persistence.WorkflowExecutionInfo{
		TaskList: tasklist,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	s.mockMutableState.EXPECT().ReplicateActivityTaskStartedEvent(startedEvent).Return(nil).Times(1)
	s.mockUpdateVersion(startedEvent)
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(startedEvent), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeActivityTaskTimedOut() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeActivityTaskTimedOut
	event := &types.HistoryEvent{
		Version:                             version,
		ID:                                  130,
		Timestamp:                           common.Int64Ptr(now.UnixNano()),
		EventType:                           &evenType,
		ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateActivityTaskTimedOutEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	//	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeActivityTaskFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeActivityTaskFailed
	event := &types.HistoryEvent{
		Version:                           version,
		ID:                                130,
		Timestamp:                         common.Int64Ptr(now.UnixNano()),
		EventType:                         &evenType,
		ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateActivityTaskFailedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeActivityTaskCompleted() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeActivityTaskCompleted
	event := &types.HistoryEvent{
		Version:                              version,
		ID:                                   130,
		Timestamp:                            common.Int64Ptr(now.UnixNano()),
		EventType:                            &evenType,
		ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateActivityTaskCompletedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeActivityTaskCancelRequested() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeActivityTaskCancelRequested
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ActivityTaskCancelRequestedEventAttributes: &types.ActivityTaskCancelRequestedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateActivityTaskCancelRequestedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeRequestCancelActivityTaskFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeRequestCancelActivityTaskFailed
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		RequestCancelActivityTaskFailedEventAttributes: &types.RequestCancelActivityTaskFailedEventAttributes{},
	}
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeActivityTaskCanceled() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeActivityTaskCanceled
	event := &types.HistoryEvent{
		Version:                             version,
		ID:                                  130,
		Timestamp:                           common.Int64Ptr(now.UnixNano()),
		EventType:                           &evenType,
		ActivityTaskCanceledEventAttributes: &types.ActivityTaskCanceledEventAttributes{},
	}

	s.mockMutableState.EXPECT().ReplicateActivityTaskCanceledEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	// assertion on timer generated is in `mockUpdateVersion` function, since activity / user timer
	// need to be refreshed each time
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

// child workflow operations

func (s *stateBuilderSuite) TestApplyEvents_EventTypeStartChildWorkflowExecutionInitiated() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}
	targetWorkflowID := "some random target workflow ID"

	now := time.Now()
	createRequestID := uuid.New()
	evenType := types.EventTypeStartChildWorkflowExecutionInitiated
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		StartChildWorkflowExecutionInitiatedEventAttributes: &types.StartChildWorkflowExecutionInitiatedEventAttributes{
			Domain:     constants.TestDomainName,
			WorkflowID: targetWorkflowID,
		},
	}

	ci := &persistence.ChildExecutionInfo{
		Version:               event.Version,
		InitiatedID:           event.ID,
		InitiatedEventBatchID: event.ID,
		StartedID:             commonconstants.EmptyEventID,
		CreateRequestID:       createRequestID,
		DomainID:              constants.TestDomainID,
		DomainNameDEPRECATED:  constants.TestDomainName,
	}

	// the create request ID is generated inside, cannot assert equal
	s.mockMutableState.EXPECT().ReplicateStartChildWorkflowExecutionInitiatedEvent(
		event.ID, event, gomock.Any(),
	).Return(ci, nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeStartChildWorkflowExecutionFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeStartChildWorkflowExecutionFailed
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		StartChildWorkflowExecutionFailedEventAttributes: &types.StartChildWorkflowExecutionFailedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateStartChildWorkflowExecutionFailedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeChildWorkflowExecutionStarted() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeChildWorkflowExecutionStarted
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateChildWorkflowExecutionStartedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeChildWorkflowExecutionTimedOut() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeChildWorkflowExecutionTimedOut
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ChildWorkflowExecutionTimedOutEventAttributes: &types.ChildWorkflowExecutionTimedOutEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateChildWorkflowExecutionTimedOutEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeChildWorkflowExecutionTerminated() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeChildWorkflowExecutionTerminated
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ChildWorkflowExecutionTerminatedEventAttributes: &types.ChildWorkflowExecutionTerminatedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateChildWorkflowExecutionTerminatedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeChildWorkflowExecutionFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeChildWorkflowExecutionFailed
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ChildWorkflowExecutionFailedEventAttributes: &types.ChildWorkflowExecutionFailedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateChildWorkflowExecutionFailedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeChildWorkflowExecutionCompleted() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeChildWorkflowExecutionCompleted
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ChildWorkflowExecutionCompletedEventAttributes: &types.ChildWorkflowExecutionCompletedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateChildWorkflowExecutionCompletedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

// cancel external workflow operations

func (s *stateBuilderSuite) TestApplyEvents_EventTypeRequestCancelExternalWorkflowExecutionInitiated() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	targetWorkflowID := "some random target workflow ID"
	targetRunID := uuid.New()
	childWorkflowOnly := true

	now := time.Now()
	cancellationRequestID := uuid.New()
	control := []byte("some random control")
	evenType := types.EventTypeRequestCancelExternalWorkflowExecutionInitiated
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		RequestCancelExternalWorkflowExecutionInitiatedEventAttributes: &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
			Domain: constants.TestDomainName,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: targetWorkflowID,
				RunID:      targetRunID,
			},
			ChildWorkflowOnly: childWorkflowOnly,
			Control:           control,
		},
	}
	rci := &persistence.RequestCancelInfo{
		Version:         event.Version,
		InitiatedID:     event.ID,
		CancelRequestID: cancellationRequestID,
	}

	// the cancellation request ID is generated inside, cannot assert equal
	s.mockMutableState.EXPECT().ReplicateRequestCancelExternalWorkflowExecutionInitiatedEvent(
		event.ID, event, gomock.Any(),
	).Return(rci, nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeRequestCancelExternalWorkflowExecutionFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeRequestCancelExternalWorkflowExecutionFailed
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		RequestCancelExternalWorkflowExecutionFailedEventAttributes: &types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateRequestCancelExternalWorkflowExecutionFailedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeExternalWorkflowExecutionCancelRequested() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeExternalWorkflowExecutionCancelRequested
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ExternalWorkflowExecutionCancelRequestedEventAttributes: &types.ExternalWorkflowExecutionCancelRequestedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateExternalWorkflowExecutionCancelRequested(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeChildWorkflowExecutionCanceled() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeChildWorkflowExecutionCanceled
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ChildWorkflowExecutionCanceledEventAttributes: &types.ChildWorkflowExecutionCanceledEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateChildWorkflowExecutionCanceledEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

// signal external workflow operations

func (s *stateBuilderSuite) TestApplyEvents_EventTypeSignalExternalWorkflowExecutionInitiated() {
	version := int64(1)
	requestID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}
	targetWorkflowID := "some random target workflow ID"
	targetRunID := uuid.New()
	childWorkflowOnly := true

	now := time.Now()
	signalRequestID := uuid.New()
	signalName := "some random signal name"
	signalInput := []byte("some random signal input")
	control := []byte("some random control")
	evenType := types.EventTypeSignalExternalWorkflowExecutionInitiated
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		SignalExternalWorkflowExecutionInitiatedEventAttributes: &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
			Domain: constants.TestDomainName,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: targetWorkflowID,
				RunID:      targetRunID,
			},
			SignalName:        signalName,
			Input:             signalInput,
			ChildWorkflowOnly: childWorkflowOnly,
		},
	}
	si := &persistence.SignalInfo{
		Version:         event.Version,
		InitiatedID:     event.ID,
		SignalRequestID: signalRequestID,
		SignalName:      signalName,
		Input:           signalInput,
		Control:         control,
	}

	// the cancellation request ID is generated inside, cannot assert equal
	s.mockMutableState.EXPECT().ReplicateSignalExternalWorkflowExecutionInitiatedEvent(
		event.ID, event, gomock.Any(),
	).Return(si, nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeSignalExternalWorkflowExecutionFailed() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeSignalExternalWorkflowExecutionFailed
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		SignalExternalWorkflowExecutionFailedEventAttributes: &types.SignalExternalWorkflowExecutionFailedEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateSignalExternalWorkflowExecutionFailedEvent(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEvents_EventTypeExternalWorkflowExecutionSignaled() {
	version := int64(1)
	requestID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      constants.TestRunID,
	}

	now := time.Now()
	evenType := types.EventTypeExternalWorkflowExecutionSignaled
	event := &types.HistoryEvent{
		Version:   version,
		ID:        130,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: &evenType,
		ExternalWorkflowExecutionSignaledEventAttributes: &types.ExternalWorkflowExecutionSignaledEventAttributes{},
	}
	s.mockMutableState.EXPECT().ReplicateExternalWorkflowExecutionSignaled(event).Return(nil).Times(1)
	s.mockUpdateVersion(event)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	s.mockMutableState.EXPECT().ClearStickyness().Times(1)

	_, err := s.stateBuilder.ApplyEvents(constants.TestDomainID, requestID, workflowExecution, s.toHistory(event), nil)
	s.Nil(err)
}

func (s *stateBuilderSuite) TestApplyEventsNewEventsNotHandled() {
	eventTypes := types.EventTypeValues()
	s.Equal(42, len(eventTypes), "If you see this error, you are adding new event type. "+
		"Before updating the number to make this test pass, please make sure you update stateBuilderImpl.ApplyEvents method "+
		"to handle the new decision type. Otherwise cross dc will not work on the new event.")
}
