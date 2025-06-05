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
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	commonConfig "github.com/uber/cadence/common/config"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/testing/testdatagen"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/query"
	"github.com/uber/cadence/service/history/shard"
	shardCtx "github.com/uber/cadence/service/history/shard"
)

type (
	mutableStateSuite struct {
		suite.Suite
		*require.Assertions

		controller      *gomock.Controller
		mockShard       *shard.TestContext
		mockEventsCache *events.MockCache

		msBuilder *mutableStateBuilder
		logger    log.Logger
		testScope tally.TestScope
	}
)

func TestMutableStateSuite(t *testing.T) {
	s := new(mutableStateSuite)
	suite.Run(t, s)
}

func (s *mutableStateSuite) SetupSuite() {

}

func (s *mutableStateSuite) TearDownSuite() {

}

func (s *mutableStateSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())

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
	// set the checksum probabilities to 100% for exercising during test
	s.mockShard.GetConfig().MutableStateChecksumGenProbability = func(domain string) int { return 100 }
	s.mockShard.GetConfig().MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
	s.mockShard.GetConfig().EnableRetryForChecksumFailure = func(domain string) bool { return true }

	s.mockEventsCache = s.mockShard.GetEventsCache().(*events.MockCache)

	s.testScope = s.mockShard.Resource.MetricsScope
	s.logger = s.mockShard.GetLogger()

	s.mockShard.Resource.DomainCache.EXPECT().GetDomainID(constants.TestDomainName).Return(constants.TestDomainID, nil).AnyTimes()

	s.msBuilder = newMutableStateBuilder(s.mockShard, s.logger, constants.TestLocalDomainEntry)
}

func (s *mutableStateSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *mutableStateSuite) TestErrorReturnedWhenSchedulingTooManyPendingActivities() {
	for i := 0; i < s.msBuilder.config.PendingActivitiesCountLimitError(); i++ {
		s.msBuilder.pendingActivityInfoIDs[int64(i)] = &persistence.ActivityInfo{}
	}

	_, _, _, _, _, err := s.msBuilder.AddActivityTaskScheduledEvent(nil, 1, &types.ScheduleActivityTaskDecisionAttributes{}, false)
	assert.Equal(s.T(), "Too many pending activities", err.Error())
}

func (s *mutableStateSuite) TestTransientDecisionCompletionFirstBatchReplicated_ReplicateDecisionCompleted() {
	version := int64(12)
	runID := uuid.New()
	s.msBuilder = NewMutableStateBuilderWithVersionHistoriesWithEventV2(
		s.mockShard,
		s.logger,
		version,
		runID,
		constants.TestGlobalDomainEntry,
	).(*mutableStateBuilder)

	newDecisionScheduleEvent, newDecisionStartedEvent := s.prepareTransientDecisionCompletionFirstBatchReplicated(version, runID)

	newDecisionCompletedEvent := &types.HistoryEvent{
		Version:   version,
		ID:        newDecisionStartedEvent.ID + 1,
		Timestamp: common.Int64Ptr(time.Now().UnixNano()),
		EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
		DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
			ScheduledEventID: newDecisionScheduleEvent.ID,
			StartedEventID:   newDecisionStartedEvent.ID,
			Identity:         "some random identity",
		},
	}
	err := s.msBuilder.ReplicateDecisionTaskCompletedEvent(newDecisionCompletedEvent)
	s.NoError(err)
	s.Equal(0, len(s.msBuilder.GetHistoryBuilder().transientHistory))
	s.Equal(0, len(s.msBuilder.GetHistoryBuilder().history))
}

func (s *mutableStateSuite) TestTransientDecisionCompletionFirstBatchReplicated_FailoverDecisionTimeout() {
	version := int64(12)
	runID := uuid.New()
	s.msBuilder = NewMutableStateBuilderWithVersionHistoriesWithEventV2(
		s.mockShard,
		s.logger,
		version,
		runID,
		constants.TestGlobalDomainEntry,
	).(*mutableStateBuilder)

	newDecisionScheduleEvent, newDecisionStartedEvent := s.prepareTransientDecisionCompletionFirstBatchReplicated(version, runID)

	s.NotNil(s.msBuilder.AddDecisionTaskTimedOutEvent(newDecisionScheduleEvent.ID, newDecisionStartedEvent.ID))
	s.Equal(0, len(s.msBuilder.GetHistoryBuilder().transientHistory))
	s.Equal(1, len(s.msBuilder.GetHistoryBuilder().history))
}

func (s *mutableStateSuite) TestTransientDecisionCompletionFirstBatchReplicated_FailoverDecisionFailed() {
	version := int64(12)
	runID := uuid.New()
	s.msBuilder = NewMutableStateBuilderWithVersionHistoriesWithEventV2(
		s.mockShard,
		s.logger,
		version,
		runID,
		constants.TestGlobalDomainEntry,
	).(*mutableStateBuilder)

	newDecisionScheduleEvent, newDecisionStartedEvent := s.prepareTransientDecisionCompletionFirstBatchReplicated(version, runID)

	s.NotNil(s.msBuilder.AddDecisionTaskFailedEvent(
		newDecisionScheduleEvent.ID,
		newDecisionStartedEvent.ID,
		types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure,
		[]byte("some random decision failure details"),
		"some random decision failure identity",
		"", "", "", "", 0, "",
	))
	s.Equal(0, len(s.msBuilder.GetHistoryBuilder().transientHistory))
	s.Equal(1, len(s.msBuilder.GetHistoryBuilder().history))
}

func (s *mutableStateSuite) TestShouldBufferEvent() {
	// workflow status events will be assign event ID immediately
	workflowEvents := map[types.EventType]bool{
		types.EventTypeWorkflowExecutionStarted:        true,
		types.EventTypeWorkflowExecutionCompleted:      true,
		types.EventTypeWorkflowExecutionFailed:         true,
		types.EventTypeWorkflowExecutionTimedOut:       true,
		types.EventTypeWorkflowExecutionTerminated:     true,
		types.EventTypeWorkflowExecutionContinuedAsNew: true,
		types.EventTypeWorkflowExecutionCanceled:       true,
	}

	// decision events will be assign event ID immediately
	decisionTaskEvents := map[types.EventType]bool{
		types.EventTypeDecisionTaskScheduled: true,
		types.EventTypeDecisionTaskStarted:   true,
		types.EventTypeDecisionTaskCompleted: true,
		types.EventTypeDecisionTaskFailed:    true,
		types.EventTypeDecisionTaskTimedOut:  true,
	}

	// events corresponding to decisions from client will be assign event ID immediately
	decisionEvents := map[types.EventType]bool{
		types.EventTypeWorkflowExecutionCompleted:                      true,
		types.EventTypeWorkflowExecutionFailed:                         true,
		types.EventTypeWorkflowExecutionCanceled:                       true,
		types.EventTypeWorkflowExecutionContinuedAsNew:                 true,
		types.EventTypeActivityTaskScheduled:                           true,
		types.EventTypeActivityTaskCancelRequested:                     true,
		types.EventTypeTimerStarted:                                    true,
		types.EventTypeTimerCanceled:                                   true,
		types.EventTypeCancelTimerFailed:                               true,
		types.EventTypeRequestCancelExternalWorkflowExecutionInitiated: true,
		types.EventTypeMarkerRecorded:                                  true,
		types.EventTypeStartChildWorkflowExecutionInitiated:            true,
		types.EventTypeSignalExternalWorkflowExecutionInitiated:        true,
		types.EventTypeUpsertWorkflowSearchAttributes:                  true,
	}

	// other events will not be assign event ID immediately
	otherEvents := map[types.EventType]bool{}
OtherEventsLoop:
	for _, eventType := range types.EventTypeValues() {
		if _, ok := workflowEvents[eventType]; ok {
			continue OtherEventsLoop
		}
		if _, ok := decisionTaskEvents[eventType]; ok {
			continue OtherEventsLoop
		}
		if _, ok := decisionEvents[eventType]; ok {
			continue OtherEventsLoop
		}
		otherEvents[eventType] = true
	}

	// test workflowEvents, decisionTaskEvents, decisionEvents will return true
	for eventType := range workflowEvents {
		s.False(s.msBuilder.shouldBufferEvent(eventType))
	}
	for eventType := range decisionTaskEvents {
		s.False(s.msBuilder.shouldBufferEvent(eventType))
	}
	for eventType := range decisionEvents {
		s.False(s.msBuilder.shouldBufferEvent(eventType))
	}
	// other events will return false
	for eventType := range otherEvents {
		s.True(s.msBuilder.shouldBufferEvent(eventType))
	}

	// +1 is because DecisionTypeCancelTimer will be mapped
	// to either types.EventTypeTimerCanceled, or types.EventTypeCancelTimerFailed.
	s.Equal(len(types.DecisionTypeValues())+1, len(decisionEvents),
		"This assertaion will be broken a new decision is added and no corresponding logic added to shouldBufferEvent()")
}

func (s *mutableStateSuite) TestReorderEvents() {
	domainID := constants.TestDomainID
	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	activityID := "activity_id"
	activityResult := []byte("activity_result")

	info := &persistence.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  we.GetWorkflowID(),
		RunID:                       we.GetRunID(),
		TaskList:                    tl,
		WorkflowTypeName:            "wType",
		WorkflowTimeout:             200,
		DecisionStartToCloseTimeout: 100,
		State:                       persistence.WorkflowStateRunning,
		CloseStatus:                 persistence.WorkflowCloseStatusNone,
		NextEventID:                 int64(8),
		LastProcessedEvent:          int64(3),
		LastUpdatedTimestamp:        time.Now(),
		DecisionVersion:             commonconstants.EmptyVersion,
		DecisionScheduleID:          commonconstants.EmptyEventID,
		DecisionStartedID:           commonconstants.EmptyEventID,
		DecisionTimeout:             100,
	}

	activityInfos := map[int64]*persistence.ActivityInfo{
		5: {
			Version:                int64(1),
			ScheduleID:             int64(5),
			ScheduledTime:          time.Now(),
			StartedID:              commonconstants.EmptyEventID,
			StartedTime:            time.Now(),
			ActivityID:             activityID,
			ScheduleToStartTimeout: 100,
			ScheduleToCloseTimeout: 200,
			StartToCloseTimeout:    300,
			HeartbeatTimeout:       50,
		},
	}

	bufferedEvents := []*types.HistoryEvent{
		{
			ID:        commonconstants.BufferedEventID,
			EventType: types.EventTypeActivityTaskCompleted.Ptr(),
			Version:   1,
			ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
				Result:           []byte(activityResult),
				ScheduledEventID: 5,
				StartedEventID:   commonconstants.BufferedEventID,
			},
		},
		{
			ID:        commonconstants.BufferedEventID,
			EventType: types.EventTypeActivityTaskStarted.Ptr(),
			Version:   1,
			ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
				ScheduledEventID: 5,
			},
		},
	}

	dbState := &persistence.WorkflowMutableState{
		ExecutionInfo:  info,
		ActivityInfos:  activityInfos,
		BufferedEvents: bufferedEvents,
	}

	err := s.msBuilder.Load(context.Background(), dbState)
	s.Nil(err)

	s.Equal(types.EventTypeActivityTaskStarted, s.msBuilder.bufferedEvents[0].GetEventType())
	s.Equal(int64(-123), s.msBuilder.bufferedEvents[0].ID)
	s.Equal(int64(5), s.msBuilder.bufferedEvents[0].ActivityTaskStartedEventAttributes.GetScheduledEventID())
	s.Equal(types.EventTypeActivityTaskCompleted, s.msBuilder.bufferedEvents[1].GetEventType())
	s.Equal(int64(-123), s.msBuilder.bufferedEvents[1].ID)
	s.Equal(int64(-123), s.msBuilder.bufferedEvents[1].ActivityTaskCompletedEventAttributes.GetStartedEventID())
	s.Equal(int64(5), s.msBuilder.bufferedEvents[1].ActivityTaskCompletedEventAttributes.GetScheduledEventID())

	err = s.msBuilder.FlushBufferedEvents()
	s.Nil(err)

	s.Equal(types.EventTypeActivityTaskStarted, s.msBuilder.hBuilder.history[0].GetEventType())
	s.Equal(int64(8), s.msBuilder.hBuilder.history[0].ID)
	s.Equal(int64(5), s.msBuilder.hBuilder.history[0].ActivityTaskStartedEventAttributes.GetScheduledEventID())
	s.Equal(types.EventTypeActivityTaskCompleted, s.msBuilder.hBuilder.history[1].GetEventType())
	s.Equal(int64(9), s.msBuilder.hBuilder.history[1].ID)
	s.Equal(int64(8), s.msBuilder.hBuilder.history[1].ActivityTaskCompletedEventAttributes.GetStartedEventID())
	s.Equal(int64(5), s.msBuilder.hBuilder.history[1].ActivityTaskCompletedEventAttributes.GetScheduledEventID())
}

func (s *mutableStateSuite) TestChecksum() {
	testCases := []struct {
		name                 string
		enableBufferedEvents bool
		closeTxFunc          func(ms *mutableStateBuilder) (checksum.Checksum, error)
	}{
		{
			name: "closeTransactionAsSnapshot",
			closeTxFunc: func(ms *mutableStateBuilder) (checksum.Checksum, error) {
				snapshot, _, err := ms.CloseTransactionAsSnapshot(time.Now(), TransactionPolicyPassive)
				if err != nil {
					return checksum.Checksum{}, err
				}
				return snapshot.Checksum, err
			},
		},
		{
			name:                 "closeTransactionAsMutation",
			enableBufferedEvents: true,
			closeTxFunc: func(ms *mutableStateBuilder) (checksum.Checksum, error) {
				mutation, _, err := ms.CloseTransactionAsMutation(time.Now(), TransactionPolicyPassive)
				if err != nil {
					return checksum.Checksum{}, err
				}
				return mutation.Checksum, err
			},
		},
	}

	loadErrorsFunc := func() int64 {
		counter := s.testScope.Snapshot().Counters()["test.mutable_state_checksum_mismatch+operation=WorkflowContext"]
		if counter != nil {
			return counter.Value()
		}
		return 0
	}

	var loadErrors int64

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			dbState := s.buildWorkflowMutableState()
			if !tc.enableBufferedEvents {
				dbState.BufferedEvents = nil
			}

			// create mutable state and verify checksum is generated on close
			loadErrors = loadErrorsFunc()
			s.msBuilder.Load(context.Background(), dbState)
			s.Equal(loadErrors, loadErrorsFunc()) // no errors expected
			s.EqualValues(dbState.Checksum, s.msBuilder.checksum)
			s.msBuilder.domainEntry = s.newDomainCacheEntry()
			csum, err := tc.closeTxFunc(s.msBuilder)
			s.Nil(err)
			s.NotNil(csum.Value)
			s.Equal(checksum.FlavorIEEECRC32OverThriftBinary, csum.Flavor)
			s.Equal(mutableStateChecksumPayloadV1, csum.Version)
			s.EqualValues(csum, s.msBuilder.checksum)

			// verify checksum is verified on Load
			dbState.Checksum = csum
			err = s.msBuilder.Load(context.Background(), dbState)
			s.NoError(err)
			s.Equal(loadErrors, loadErrorsFunc())

			// generate checksum again and verify its the same
			csum, err = tc.closeTxFunc(s.msBuilder)
			s.Nil(err)
			s.NotNil(csum.Value)
			s.Equal(dbState.Checksum.Value, csum.Value)

			// modify checksum and verify Load fails
			dbState.Checksum.Value[0]++
			err = s.msBuilder.Load(context.Background(), dbState)
			s.Error(err)
			s.Equal(loadErrors+1, loadErrorsFunc())
			s.EqualValues(dbState.Checksum, s.msBuilder.checksum)

			// test checksum is invalidated
			loadErrors = loadErrorsFunc()
			s.mockShard.GetConfig().MutableStateChecksumInvalidateBefore = func(...dynamicproperties.FilterOption) float64 {
				return float64((s.msBuilder.executionInfo.LastUpdatedTimestamp.UnixNano() / int64(time.Second)) + 1)
			}
			err = s.msBuilder.Load(context.Background(), dbState)
			s.NoError(err)
			s.Equal(loadErrors, loadErrorsFunc())
			s.EqualValues(checksum.Checksum{}, s.msBuilder.checksum)

			// revert the config value for the next test case
			s.mockShard.GetConfig().MutableStateChecksumInvalidateBefore = func(...dynamicproperties.FilterOption) float64 {
				return float64(0)
			}
		})
	}
}

func (s *mutableStateSuite) TestChecksumProbabilities() {
	for _, prob := range []int{0, 100} {
		s.mockShard.GetConfig().MutableStateChecksumGenProbability = func(domain string) int { return prob }
		s.mockShard.GetConfig().MutableStateChecksumVerifyProbability = func(domain string) int { return prob }
		for i := 0; i < 100; i++ {
			shouldGenerate := s.msBuilder.shouldGenerateChecksum()
			shouldVerify := s.msBuilder.shouldVerifyChecksum()
			s.Equal(prob == 100, shouldGenerate)
			s.Equal(prob == 100, shouldVerify)
		}
	}
}

func (s *mutableStateSuite) TestChecksumShouldInvalidate() {
	s.mockShard.GetConfig().MutableStateChecksumInvalidateBefore = func(...dynamicproperties.FilterOption) float64 { return 0 }
	s.False(s.msBuilder.shouldInvalidateChecksum())
	s.msBuilder.executionInfo.LastUpdatedTimestamp = time.Now()
	s.mockShard.GetConfig().MutableStateChecksumInvalidateBefore = func(...dynamicproperties.FilterOption) float64 {
		return float64((s.msBuilder.executionInfo.LastUpdatedTimestamp.UnixNano() / int64(time.Second)) + 1)
	}
	s.True(s.msBuilder.shouldInvalidateChecksum())
	s.mockShard.GetConfig().MutableStateChecksumInvalidateBefore = func(...dynamicproperties.FilterOption) float64 {
		return float64((s.msBuilder.executionInfo.LastUpdatedTimestamp.UnixNano() / int64(time.Second)) - 1)
	}
	s.False(s.msBuilder.shouldInvalidateChecksum())
}

func (s *mutableStateSuite) TestTrimEvents() {
	var input []*types.HistoryEvent
	output := s.msBuilder.trimEventsAfterWorkflowClose(input)
	s.Equal(input, output)

	input = []*types.HistoryEvent{}
	output = s.msBuilder.trimEventsAfterWorkflowClose(input)
	s.Equal(input, output)

	input = []*types.HistoryEvent{
		{
			EventType: types.EventTypeActivityTaskCanceled.Ptr(),
		},
		{
			EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		},
	}
	output = s.msBuilder.trimEventsAfterWorkflowClose(input)
	s.Equal(input, output)

	input = []*types.HistoryEvent{
		{
			EventType: types.EventTypeActivityTaskCanceled.Ptr(),
		},
		{
			EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
		},
	}
	output = s.msBuilder.trimEventsAfterWorkflowClose(input)
	s.Equal(input, output)

	input = []*types.HistoryEvent{
		{
			EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
		},
		{
			EventType: types.EventTypeActivityTaskCanceled.Ptr(),
		},
	}
	output = s.msBuilder.trimEventsAfterWorkflowClose(input)
	s.Equal([]*types.HistoryEvent{
		{
			EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
		},
	}, output)
}

func (s *mutableStateSuite) TestMergeMapOfByteArray() {
	var currentMap map[string][]byte
	var newMap map[string][]byte
	resultMap := mergeMapOfByteArray(currentMap, newMap)
	s.Equal(make(map[string][]byte), resultMap)

	newMap = map[string][]byte{"key": []byte("val")}
	resultMap = mergeMapOfByteArray(currentMap, newMap)
	s.Equal(newMap, resultMap)

	currentMap = map[string][]byte{"number": []byte("1")}
	resultMap = mergeMapOfByteArray(currentMap, newMap)
	s.Equal(2, len(resultMap))
}

func (s *mutableStateSuite) TestEventReapplied() {
	runID := uuid.New()
	eventID := int64(1)
	version := int64(2)
	dedupResource := definition.NewEventReappliedID(runID, eventID, version)
	isReapplied := s.msBuilder.IsResourceDuplicated(dedupResource)
	s.False(isReapplied)
	s.msBuilder.UpdateDuplicatedResource(dedupResource)
	isReapplied = s.msBuilder.IsResourceDuplicated(dedupResource)
	s.True(isReapplied)
}

func (s *mutableStateSuite) TestTransientDecisionTaskSchedule_CurrentVersionChanged() {
	version := int64(2000)
	runID := uuid.New()
	s.msBuilder = NewMutableStateBuilderWithVersionHistoriesWithEventV2(
		s.mockShard,
		s.logger,
		version,
		runID,
		constants.TestGlobalDomainEntry,
	).(*mutableStateBuilder)
	decisionScheduleEvent, decisionStartedEvent := s.prepareTransientDecisionCompletionFirstBatchReplicated(version, runID)
	decisionFailedEvent := &types.HistoryEvent{
		Version:   version,
		ID:        3,
		Timestamp: common.Int64Ptr(time.Now().UnixNano()),
		EventType: types.EventTypeDecisionTaskFailed.Ptr(),
		DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
			ScheduledEventID: decisionScheduleEvent.ID,
			StartedEventID:   decisionStartedEvent.ID,
		},
	}
	err := s.msBuilder.ReplicateDecisionTaskFailedEvent(decisionFailedEvent)
	s.NoError(err)

	err = s.msBuilder.UpdateCurrentVersion(version+1, true)
	s.NoError(err)
	versionHistories := s.msBuilder.GetVersionHistories()
	versionHistory, err := versionHistories.GetCurrentVersionHistory()
	s.NoError(err)
	versionHistory.AddOrUpdateItem(&persistence.VersionHistoryItem{
		EventID: 3,
		Version: version,
	})

	now := time.Now()
	di, err := s.msBuilder.AddDecisionTaskScheduledEventAsHeartbeat(true, now.UnixNano())
	s.NoError(err)
	s.NotNil(di)

	s.Equal(int64(0), s.msBuilder.GetExecutionInfo().DecisionAttempt)
	s.Equal(0, len(s.msBuilder.GetHistoryBuilder().transientHistory))
	s.Equal(1, len(s.msBuilder.GetHistoryBuilder().history))
}

func (s *mutableStateSuite) TestTransientDecisionTaskStart_CurrentVersionChanged() {
	version := int64(2000)
	runID := uuid.New()
	s.msBuilder = NewMutableStateBuilderWithVersionHistoriesWithEventV2(
		s.mockShard,
		s.logger,
		version,
		runID,
		constants.TestGlobalDomainEntry,
	).(*mutableStateBuilder)
	decisionScheduleEvent, decisionStartedEvent := s.prepareTransientDecisionCompletionFirstBatchReplicated(version, runID)
	decisionFailedEvent := &types.HistoryEvent{
		Version:   version,
		ID:        3,
		Timestamp: common.Int64Ptr(time.Now().UnixNano()),
		EventType: types.EventTypeDecisionTaskFailed.Ptr(),
		DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
			ScheduledEventID: decisionScheduleEvent.ID,
			StartedEventID:   decisionStartedEvent.ID,
		},
	}
	err := s.msBuilder.ReplicateDecisionTaskFailedEvent(decisionFailedEvent)
	s.NoError(err)

	decisionScheduleID := int64(4)
	now := time.Now()
	tasklist := "some random tasklist"
	decisionTimeoutSecond := int32(11)
	decisionAttempt := int64(2)
	newDecisionScheduleEvent := &types.HistoryEvent{
		Version:   version,
		ID:        decisionScheduleID,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
		DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
			TaskList:                   &types.TaskList{Name: tasklist},
			StartToCloseTimeoutSeconds: common.Int32Ptr(decisionTimeoutSecond),
			Attempt:                    decisionAttempt,
		},
	}
	di, err := s.msBuilder.ReplicateDecisionTaskScheduledEvent(
		newDecisionScheduleEvent.Version,
		newDecisionScheduleEvent.ID,
		newDecisionScheduleEvent.DecisionTaskScheduledEventAttributes.TaskList.GetName(),
		newDecisionScheduleEvent.DecisionTaskScheduledEventAttributes.GetStartToCloseTimeoutSeconds(),
		newDecisionScheduleEvent.DecisionTaskScheduledEventAttributes.GetAttempt(),
		0,
		0,
		false,
	)
	s.NoError(err)
	s.NotNil(di)

	err = s.msBuilder.UpdateCurrentVersion(version+1, true)
	s.NoError(err)
	versionHistories := s.msBuilder.GetVersionHistories()
	versionHistory, err := versionHistories.GetCurrentVersionHistory()
	s.NoError(err)
	versionHistory.AddOrUpdateItem(&persistence.VersionHistoryItem{
		EventID: 3,
		Version: version,
	})

	_, _, err = s.msBuilder.AddDecisionTaskStartedEvent(
		decisionScheduleID,
		uuid.New(),
		&types.PollForDecisionTaskRequest{
			Identity: IdentityHistoryService,
		},
	)
	s.NoError(err)

	s.Equal(0, len(s.msBuilder.GetHistoryBuilder().transientHistory))
	s.Equal(2, len(s.msBuilder.GetHistoryBuilder().history))
}

func (s *mutableStateSuite) prepareTransientDecisionCompletionFirstBatchReplicated(version int64, runID string) (*types.HistoryEvent, *types.HistoryEvent) {
	domainID := constants.TestDomainID
	execution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      runID,
	}

	now := time.Now()
	workflowType := "some random workflow type"
	tasklist := "some random tasklist"
	workflowTimeoutSecond := int32(222)
	decisionTimeoutSecond := int32(11)
	decisionAttempt := int64(0)
	partitionConfig := map[string]string{
		"zone": "dca",
	}

	eventID := int64(1)
	workflowStartEvent := &types.HistoryEvent{
		Version:   version,
		ID:        eventID,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: tasklist},
			Input:                               nil,
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(workflowTimeoutSecond),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(decisionTimeoutSecond),
			PartitionConfig:                     partitionConfig,
		},
	}
	eventID++

	decisionScheduleEvent := &types.HistoryEvent{
		Version:   version,
		ID:        eventID,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
		DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
			TaskList:                   &types.TaskList{Name: tasklist},
			StartToCloseTimeoutSeconds: common.Int32Ptr(decisionTimeoutSecond),
			Attempt:                    decisionAttempt,
		},
	}
	eventID++

	decisionStartedEvent := &types.HistoryEvent{
		Version:   version,
		ID:        eventID,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeDecisionTaskStarted.Ptr(),
		DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
			ScheduledEventID: decisionScheduleEvent.ID,
			RequestID:        uuid.New(),
		},
	}
	eventID++

	decisionFailedEvent := &types.HistoryEvent{
		Version:   version,
		ID:        eventID,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeDecisionTaskFailed.Ptr(),
		DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
			ScheduledEventID: decisionScheduleEvent.ID,
			StartedEventID:   decisionStartedEvent.ID,
		},
	}
	eventID++

	s.mockEventsCache.EXPECT().PutEvent(
		domainID, execution.GetWorkflowID(), execution.GetRunID(),
		workflowStartEvent.ID, workflowStartEvent,
	).Times(1)
	err := s.msBuilder.ReplicateWorkflowExecutionStartedEvent(
		nil,
		execution,
		uuid.New(),
		workflowStartEvent,
		false,
	)
	s.Nil(err)

	// setup transient decision
	di, err := s.msBuilder.ReplicateDecisionTaskScheduledEvent(
		decisionScheduleEvent.Version,
		decisionScheduleEvent.ID,
		decisionScheduleEvent.DecisionTaskScheduledEventAttributes.TaskList.GetName(),
		decisionScheduleEvent.DecisionTaskScheduledEventAttributes.GetStartToCloseTimeoutSeconds(),
		decisionScheduleEvent.DecisionTaskScheduledEventAttributes.GetAttempt(),
		0,
		0,
		false,
	)
	s.Nil(err)
	s.NotNil(di)

	di, err = s.msBuilder.ReplicateDecisionTaskStartedEvent(nil,
		decisionStartedEvent.Version,
		decisionScheduleEvent.ID,
		decisionStartedEvent.ID,
		decisionStartedEvent.DecisionTaskStartedEventAttributes.GetRequestID(),
		decisionStartedEvent.GetTimestamp(),
	)
	s.Nil(err)
	s.NotNil(di)

	err = s.msBuilder.ReplicateDecisionTaskFailedEvent(decisionFailedEvent)
	s.Nil(err)

	decisionAttempt = int64(123)
	newDecisionScheduleEvent := &types.HistoryEvent{
		Version:   version,
		ID:        eventID,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
		DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
			TaskList:                   &types.TaskList{Name: tasklist},
			StartToCloseTimeoutSeconds: common.Int32Ptr(decisionTimeoutSecond),
			Attempt:                    decisionAttempt,
		},
	}
	eventID++

	newDecisionStartedEvent := &types.HistoryEvent{
		Version:   version,
		ID:        eventID,
		Timestamp: common.Int64Ptr(now.UnixNano()),
		EventType: types.EventTypeDecisionTaskStarted.Ptr(),
		DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
			ScheduledEventID: decisionScheduleEvent.ID,
			RequestID:        uuid.New(),
		},
	}
	eventID++ //nolint:ineffassign

	di, err = s.msBuilder.ReplicateDecisionTaskScheduledEvent(
		newDecisionScheduleEvent.Version,
		newDecisionScheduleEvent.ID,
		newDecisionScheduleEvent.DecisionTaskScheduledEventAttributes.TaskList.GetName(),
		newDecisionScheduleEvent.DecisionTaskScheduledEventAttributes.GetStartToCloseTimeoutSeconds(),
		newDecisionScheduleEvent.DecisionTaskScheduledEventAttributes.GetAttempt(),
		0,
		0,
		false,
	)
	s.Nil(err)
	s.NotNil(di)

	di, err = s.msBuilder.ReplicateDecisionTaskStartedEvent(nil,
		newDecisionStartedEvent.Version,
		newDecisionScheduleEvent.ID,
		newDecisionStartedEvent.ID,
		newDecisionStartedEvent.DecisionTaskStartedEventAttributes.GetRequestID(),
		newDecisionStartedEvent.GetTimestamp(),
	)
	s.Nil(err)
	s.NotNil(di)

	return newDecisionScheduleEvent, newDecisionStartedEvent
}

func (s *mutableStateSuite) TestLoad_BackwardsCompatibility() {
	mutableState := s.buildWorkflowMutableState()

	s.msBuilder.Load(context.Background(), mutableState)

	s.Equal(constants.TestDomainID, s.msBuilder.pendingChildExecutionInfoIDs[81].DomainID)
}

func (s *mutableStateSuite) TestUpdateCurrentVersion_WorkflowOpen() {
	mutableState := s.buildWorkflowMutableState()

	s.msBuilder.Load(context.Background(), mutableState)
	s.Equal(commonconstants.EmptyVersion, s.msBuilder.GetCurrentVersion())

	version := int64(2000)
	s.msBuilder.UpdateCurrentVersion(version, false)
	s.Equal(version, s.msBuilder.GetCurrentVersion())
}

func (s *mutableStateSuite) TestUpdateCurrentVersion_WorkflowClosed() {
	mutableState := s.buildWorkflowMutableState()
	mutableState.ExecutionInfo.State = persistence.WorkflowStateCompleted
	mutableState.ExecutionInfo.CloseStatus = persistence.WorkflowCloseStatusCompleted

	s.msBuilder.Load(context.Background(), mutableState)
	s.Equal(commonconstants.EmptyVersion, s.msBuilder.GetCurrentVersion())

	versionHistory, err := mutableState.VersionHistories.GetCurrentVersionHistory()
	s.NoError(err)
	lastItem, err := versionHistory.GetLastItem()
	s.NoError(err)
	lastWriteVersion := lastItem.Version

	version := int64(2000)
	s.msBuilder.UpdateCurrentVersion(version, false)
	s.Equal(lastWriteVersion, s.msBuilder.GetCurrentVersion())
}

func (s *mutableStateSuite) newDomainCacheEntry() *cache.DomainCacheEntry {
	return cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: "mutableStateTest"},
		&persistence.DomainConfig{},
		true,
		&persistence.DomainReplicationConfig{},
		1,
		nil,
		0,
		0,
		0,
	)
}

func (s *mutableStateSuite) buildWorkflowMutableState() *persistence.WorkflowMutableState {
	domainID := constants.TestDomainID
	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	failoverVersion := int64(300)
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	info := &persistence.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  we.GetWorkflowID(),
		RunID:                       we.GetRunID(),
		TaskList:                    tl,
		WorkflowTypeName:            "wType",
		WorkflowTimeout:             200,
		DecisionStartToCloseTimeout: 100,
		State:                       persistence.WorkflowStateRunning,
		CloseStatus:                 persistence.WorkflowCloseStatusNone,
		NextEventID:                 int64(101),
		LastProcessedEvent:          int64(99),
		LastUpdatedTimestamp:        time.Now(),
		DecisionVersion:             failoverVersion,
		DecisionScheduleID:          commonconstants.EmptyEventID,
		DecisionStartedID:           commonconstants.EmptyEventID,
		DecisionTimeout:             100,
		PartitionConfig:             partitionConfig,
	}

	activityInfos := map[int64]*persistence.ActivityInfo{
		5: {
			Version:                failoverVersion,
			ScheduleID:             int64(5),
			ScheduledTime:          time.Now(),
			StartedID:              commonconstants.EmptyEventID,
			StartedTime:            time.Now(),
			ActivityID:             "activityID_5",
			ScheduleToStartTimeout: 100,
			ScheduleToCloseTimeout: 200,
			StartToCloseTimeout:    300,
			HeartbeatTimeout:       50,
		},
	}

	timerInfos := map[string]*persistence.TimerInfo{
		"25": {
			Version:    failoverVersion,
			TimerID:    "25",
			StartedID:  85,
			ExpiryTime: time.Now().Add(time.Hour),
		},
	}

	childInfos := map[int64]*persistence.ChildExecutionInfo{
		80: {
			Version:               failoverVersion,
			InitiatedID:           80,
			InitiatedEventBatchID: 20,
			InitiatedEvent:        &types.HistoryEvent{},
			StartedID:             commonconstants.EmptyEventID,
			CreateRequestID:       uuid.New(),
			DomainID:              constants.TestDomainID,
			WorkflowTypeName:      "code.uber.internal/test/foobar",
		},
		81: {
			Version:               failoverVersion,
			InitiatedID:           80,
			InitiatedEventBatchID: 20,
			InitiatedEvent:        &types.HistoryEvent{},
			StartedID:             commonconstants.EmptyEventID,
			CreateRequestID:       uuid.New(),
			DomainNameDEPRECATED:  constants.TestDomainName,
			WorkflowTypeName:      "code.uber.internal/test/foobar",
		},
	}

	signalInfos := map[int64]*persistence.SignalInfo{
		75: {
			Version:               failoverVersion,
			InitiatedID:           75,
			InitiatedEventBatchID: 17,
			SignalRequestID:       uuid.New(),
			SignalName:            "test-signal-75",
			Input:                 []byte("signal-input-75"),
		},
	}

	signalRequestIDs := map[string]struct{}{
		uuid.New(): {},
	}

	bufferedEvents := []*types.HistoryEvent{
		{
			ID:        commonconstants.BufferedEventID,
			EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
			Version:   failoverVersion,
			WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
				SignalName: "test-signal-buffered",
				Input:      []byte("test-signal-buffered-input"),
			},
		},
	}

	versionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte("token#1"),
				Items: []*persistence.VersionHistoryItem{
					{EventID: 1, Version: 300},
				},
			},
		},
	}

	return &persistence.WorkflowMutableState{
		ExecutionInfo:       info,
		ActivityInfos:       activityInfos,
		TimerInfos:          timerInfos,
		ChildExecutionInfos: childInfos,
		SignalInfos:         signalInfos,
		SignalRequestedIDs:  signalRequestIDs,
		BufferedEvents:      bufferedEvents,
		VersionHistories:    versionHistories,
	}
}

func TestNewMutableStateBuilderWithEventV2(t *testing.T) {

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
	domainCache := cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: "mutableStateTest"},
		&persistence.DomainConfig{},
		true,
		&persistence.DomainReplicationConfig{},
		1,
		nil,
		0,
		0,
		0,
	)

	NewMutableStateBuilderWithEventV2(mockShard, log.NewNoop(), "A82146B5-7A5C-4660-9195-E80E5161EC56", domainCache)
}

var (
	domainID = "A6338800-D143-4FEF-8A49-9BBB31386C5F"
	wfID     = "879A361B-B435-491D-8A3B-ACF3BAD30F4B"
	runID    = "81DFCB6B-ACD4-46D1-89C2-804388203880"
	ts1      = int64(1234)
	shardID  = 123
)

// Guiding real data example: ie:
// `select execution from executions where run_id = <run-id> ALLOW FILTERING;`
//
// executions.execution {
// domainID: "A6338800-D143-4FEF-8A49-9BBB31386C5F",
// wfID: "879A361B-B435-491D-8A3B-ACF3BAD30F4B",
// runID: "81DFCB6B-ACD4-46D1-89C2-804388203880",
// initiated_id: -7,
// completion_event: null,
// state: 2,
// close_status: 1,
// next_event_id: 12,
// last_processed_event: 9,
// decision_schedule_id: -23,
// decision_started_id: -23,
// last_first_event_id: 10,
// decision_version: -24,
// completion_event_batch_id: 10,
// last_event_task_id: 4194328,
// }
var exampleMutableStateForClosedWF = &mutableStateBuilder{
	executionInfo: &persistence.WorkflowExecutionInfo{
		WorkflowID:             wfID,
		DomainID:               domainID,
		RunID:                  runID,
		NextEventID:            12,
		State:                  persistence.WorkflowStateCompleted,
		CompletionEventBatchID: 10,
		BranchToken:            []byte("branch-token"),
	},
}

var exampleCompletionEvent = &types.HistoryEvent{
	ID:        11,
	TaskID:    4194328,
	Version:   1,
	Timestamp: &ts1,
	WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{
		Result:                       []byte("some random workflow completion result"),
		DecisionTaskCompletedEventID: 10,
	},
}

var exampleStartEvent = &types.HistoryEvent{
	ID:        1,
	TaskID:    4194328,
	Version:   1,
	Timestamp: &ts1,
	WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
		WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
		TaskList:                            &types.TaskList{Name: "tasklist"},
		Input:                               []byte("some random workflow input"),
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		OriginalExecutionRunID:              runID,
		Identity:                            "123@some-hostname@@uuid",
	},
}

func TestGetCompletionEvent(t *testing.T) {
	tests := map[string]struct {
		currentState *mutableStateBuilder

		historyManagerAffordance func(historyManager *persistence.MockHistoryManager)

		expectedResult *types.HistoryEvent
		expectedErr    error
	}{
		"Getting a completed event from a normal, completed workflow - taken from a real example": {
			currentState: exampleMutableStateForClosedWF,
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {

				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(),
					&persistence.ReadHistoryBranchRequest{
						BranchToken:   []byte("branch-token"),
						MinEventID:    10,
						MaxEventID:    12, // nextEventID +1
						PageSize:      1,
						NextPageToken: nil,
						ShardID:       common.IntPtr(shardID),
						DomainName:    "domain",
					}).Return(&persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{
						exampleCompletionEvent,
					},
				}, nil)

			},

			expectedResult: exampleCompletionEvent,
		},
		"An unexpected error while fetchhing history, such as not found err": {
			currentState: &mutableStateBuilder{
				executionInfo: &persistence.WorkflowExecutionInfo{
					WorkflowID:             wfID,
					DomainID:               domainID,
					RunID:                  runID,
					NextEventID:            12,
					State:                  persistence.WorkflowStateCompleted,
					CompletionEventBatchID: 10,
					BranchToken:            []byte("branch-token"),
				},
			},
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {
				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(nil, errors.New("a transient random error"))
			},

			expectedResult: nil,
			expectedErr:    &types.InternalServiceError{Message: "unable to get workflow completion event"},
		},
		"A 'transient' internal service error, this should be returned to the caller": {
			currentState: &mutableStateBuilder{
				executionInfo: &persistence.WorkflowExecutionInfo{
					WorkflowID:             wfID,
					DomainID:               domainID,
					RunID:                  runID,
					NextEventID:            12,
					State:                  persistence.WorkflowStateCompleted,
					CompletionEventBatchID: 10,
					BranchToken:            []byte("branch-token"),
				},
			},
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {
				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(nil, &types.InternalServiceError{Message: "an err"})
			},

			expectedResult: nil,
			expectedErr:    &types.InternalServiceError{Message: "an err"},
		},
		"initial validation: An invalid starting mutable state should return an error": {
			currentState: &mutableStateBuilder{
				executionInfo: &persistence.WorkflowExecutionInfo{},
			},
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {},
			expectedResult:           nil,
			expectedErr:              &types.InternalServiceError{Message: "unable to get workflow completion event"},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shardCtx.NewMockContext(ctrl)
			shardContext.EXPECT().GetShardID().Return(123).AnyTimes() // this isn't called on a few of the validation failures
			historyManager := persistence.NewMockHistoryManager(ctrl)
			td.historyManagerAffordance(historyManager)

			domainCache := cache.NewMockDomainCache(ctrl)
			domainCache.EXPECT().GetDomainName(gomock.Any()).Return("domain", nil).AnyTimes() // this isn't called on validation

			td.currentState.eventsCache = events.NewCache(shardID, historyManager, config.NewForTest(), log.NewNoop(), metrics.NewNoopMetricsClient(), domainCache)
			td.currentState.shard = shardContext

			res, err := td.currentState.GetCompletionEvent(context.Background())

			assert.Equal(t, td.expectedResult, res)
			if td.expectedErr != nil {
				assert.ErrorAs(t, td.expectedErr, &err)
			}
		})
	}
}

func TestGetStartEvent(t *testing.T) {
	tests := map[string]struct {
		currentState *mutableStateBuilder

		historyManagerAffordance func(historyManager *persistence.MockHistoryManager)

		expectedResult *types.HistoryEvent
		expectedErr    error
	}{
		"Getting a start event from a normal, completed workflow - taken from a real example": {
			currentState: exampleMutableStateForClosedWF,
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {

				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(),
					&persistence.ReadHistoryBranchRequest{
						BranchToken:   []byte("branch-token"),
						MinEventID:    1,
						MaxEventID:    2,
						PageSize:      1,
						NextPageToken: nil,
						ShardID:       common.IntPtr(shardID),
						DomainName:    "domain",
					}).Return(&persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{
						exampleStartEvent,
					},
				}, nil)

			},

			expectedResult: exampleStartEvent,
		},
		"Getting a start event but hitting an error when reaching into history": {
			currentState: exampleMutableStateForClosedWF,
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {
				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(nil, errors.New("an error"))
			},
			expectedErr: types.InternalServiceError{Message: "unable to get workflow start event"},
		},
		"Getting a start event but hitting a 'transient' error when reaching into history. This should be passed back up the call stack": {
			currentState: exampleMutableStateForClosedWF,
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {
				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(nil, &types.InternalServiceError{Message: "an error"})
			},
			expectedErr: types.InternalServiceError{Message: "an error"},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shardCtx.NewMockContext(ctrl)
			shardContext.EXPECT().GetShardID().Return(123).AnyTimes() // this isn't called on a few of the validation failures
			historyManager := persistence.NewMockHistoryManager(ctrl)
			td.historyManagerAffordance(historyManager)

			domainCache := cache.NewMockDomainCache(ctrl)
			domainCache.EXPECT().GetDomainName(gomock.Any()).Return("domain", nil).AnyTimes() // this isn't called on validation

			td.currentState.eventsCache = events.NewCache(shardID, historyManager, config.NewForTest(), log.NewNoop(), metrics.NewNoopMetricsClient(), domainCache)
			td.currentState.shard = shardContext

			res, err := td.currentState.GetStartEvent(context.Background())

			assert.Equal(t, td.expectedResult, res)
			if td.expectedErr != nil {
				assert.ErrorAs(t, err, &td.expectedErr)
			}
		})
	}
}

func TestLoggingNilAndInvalidHandling(t *testing.T) {
	gen := testdatagen.New(t)

	executionInfo := persistence.WorkflowExecutionInfo{}

	gen.Fuzz(&executionInfo)
	msb := mutableStateBuilder{
		executionInfo: &executionInfo,
		logger:        log.NewNoop(),
		metricsClient: metrics.NewNoopMetricsClient(),
	}

	msbInvalid := mutableStateBuilder{logger: log.NewNoop()}

	assert.NotPanics(t, func() {
		msbInvalid.logWarn("test", tag.WorkflowDomainID("test"))
		msbInvalid.logError("test", tag.WorkflowDomainID("test"))
		msbInvalid.logInfo("test", tag.WorkflowDomainID("test"))
		msb.logWarn("test", tag.WorkflowDomainID("test"))
		msb.logError("test", tag.WorkflowDomainID("test"))
		msb.logInfo("test", tag.WorkflowDomainID("test"))
		msb.logDataInconsistency()
	})
}

func TestAssignEventIDToBufferedEvents(t *testing.T) {

	tests := map[string]struct {
		startingEventID                   int64
		pendingActivityInfo               map[int64]*persistence.ActivityInfo
		pendingChildExecutionInfoIDs      map[int64]*persistence.ChildExecutionInfo
		startingHistoryEntries            []*types.HistoryEvent
		expectedUpdateActivityInfos       map[int64]*persistence.ActivityInfo
		expectedEndingHistoryEntries      []*types.HistoryEvent
		expectedNextEventID               int64
		expectedUpdateChildExecutionInfos map[int64]*persistence.ChildExecutionInfo
	}{
		"Timer Fired - this should increment the nextevent ID counter": {
			startingEventID: 12,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID:        "1",
						StartedEventID: 11,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					ID:        12,
					TaskID:    commonconstants.EmptyEventTaskID,
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID:        "1",
						StartedEventID: 11,
					},
				},
			},
			expectedNextEventID:               13,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Activity completed and started - this should update any buffered activities": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
					ID:        4,
					TaskID:    commonconstants.EmptyEventTaskID,
					DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
						ScheduledEventID: 2,
						StartedEventID:   3,
					},
				},
				{
					EventType: types.EventTypeActivityTaskScheduled.Ptr(),
					ID:        5,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						ActivityID: "0",
					},
				},
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 5,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
					ID:        4,
					TaskID:    commonconstants.EmptyEventTaskID,
					DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
						ScheduledEventID: 2,
						StartedEventID:   3,
					},
				},
				{
					EventType: types.EventTypeActivityTaskScheduled.Ptr(),
					ID:        5,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						ActivityID: "0",
					},
				},
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 5,
					},
				},
			},
			expectedNextEventID:               7,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Activity task started and a pending activity is updated - this should be put to the updatedActivityInfos map with all the other counters incremented": {
			startingEventID: 6,
			pendingActivityInfo: map[int64]*persistence.ActivityInfo{
				5: {
					ScheduleID: 5,
					StartedID:  6,
				},
			},
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 5,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 5,
					},
				},
			},
			expectedNextEventID: 7,
			expectedUpdateActivityInfos: map[int64]*persistence.ActivityInfo{
				5: {
					ScheduleID: 5,
					StartedID:  6,
				},
			},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Activity task started and then completed": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskCompleted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
						StartedEventID:   4567,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskCompleted.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
						StartedEventID:   6,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Activity task started and then Cancelled": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskCanceled.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskCanceledEventAttributes: &types.ActivityTaskCanceledEventAttributes{
						StartedEventID:   123,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskCanceled.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskCanceledEventAttributes: &types.ActivityTaskCanceledEventAttributes{
						StartedEventID:   6,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Activity task started and then failed": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskFailed.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
						StartedEventID:   123,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskFailed.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
						StartedEventID:   6,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Activity task started and then timed out": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskTimedOut.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
						StartedEventID:   123,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeActivityTaskStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 3456,
					},
				},
				{
					EventType: types.EventTypeActivityTaskTimedOut.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
						StartedEventID:   6,
						ScheduledEventID: 3456,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Child workflow scheduled and then completed": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionCompleted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionCompletedEventAttributes: &types.ChildWorkflowExecutionCompletedEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionCompleted.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionCompletedEventAttributes: &types.ChildWorkflowExecutionCompletedEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Child workflow scheduled and then Cancelled - where there is a pending execution that requires an update": {
			startingEventID: 6,
			pendingChildExecutionInfoIDs: map[int64]*persistence.ChildExecutionInfo{
				123: {
					InitiatedID: 321,
				},
			},
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionCanceled.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionCanceledEventAttributes: &types.ChildWorkflowExecutionCanceledEventAttributes{
						StartedEventID:   321,
						InitiatedEventID: 123,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionCanceled.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionCanceledEventAttributes: &types.ChildWorkflowExecutionCanceledEventAttributes{
						StartedEventID:   6,
						InitiatedEventID: 123,
					},
				},
			},
			expectedNextEventID:         8,
			expectedUpdateActivityInfos: map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{
				321: {
					InitiatedID: 321,
					StartedID:   6,
				},
			},
		},
		"Child workflow scheduled and then Failed": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionFailed.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionFailedEventAttributes: &types.ChildWorkflowExecutionFailedEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionFailed.Ptr().Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionFailedEventAttributes: &types.ChildWorkflowExecutionFailedEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Child workflow scheduled and then Timed out": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionTimedOut.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionTimedOutEventAttributes: &types.ChildWorkflowExecutionTimedOutEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionTimedOut.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionTimedOutEventAttributes: &types.ChildWorkflowExecutionTimedOutEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
		"Child workflow scheduled and then Terminated": {
			startingEventID: 6,
			startingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionTerminated.Ptr(),
					ID:        commonconstants.BufferedEventID,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionTerminatedEventAttributes: &types.ChildWorkflowExecutionTerminatedEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedEndingHistoryEntries: []*types.HistoryEvent{
				{
					EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
					ID:        6,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 123,
					},
				},
				{
					EventType: types.EventTypeChildWorkflowExecutionTerminated.Ptr(),
					ID:        7,
					TaskID:    commonconstants.EmptyEventTaskID,
					ChildWorkflowExecutionTerminatedEventAttributes: &types.ChildWorkflowExecutionTerminatedEventAttributes{
						StartedEventID:   123,
						InitiatedEventID: 2345,
					},
				},
			},
			expectedNextEventID:               8,
			expectedUpdateActivityInfos:       map[int64]*persistence.ActivityInfo{},
			expectedUpdateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			msb := &mutableStateBuilder{
				pendingChildExecutionInfoIDs: td.pendingChildExecutionInfoIDs,
				pendingActivityInfoIDs:       td.pendingActivityInfo,
				executionInfo: &persistence.WorkflowExecutionInfo{
					NextEventID: td.startingEventID,
				},
				hBuilder: &HistoryBuilder{
					history: td.startingHistoryEntries,
				},
				updateActivityInfos:       make(map[int64]*persistence.ActivityInfo),
				updateChildExecutionInfos: make(map[int64]*persistence.ChildExecutionInfo),
			}

			msb.assignEventIDToBufferedEvents()

			assert.Equal(t, td.expectedEndingHistoryEntries, msb.hBuilder.history)
			assert.Equal(t, td.expectedNextEventID, msb.executionInfo.NextEventID)
			assert.Equal(t, td.expectedUpdateActivityInfos, msb.updateActivityInfos)
			assert.Equal(t, td.expectedUpdateChildExecutionInfos, msb.updateChildExecutionInfos)
		})
	}
}

// This is only for passing the coverage
func TestLog(t *testing.T) {
	var e *mutableStateBuilder
	assert.NotPanics(t, func() { e.logInfo("a") })
	assert.NotPanics(t, func() { e.logWarn("a") })
	assert.NotPanics(t, func() { e.logError("a") })
}

func TestMutableStateBuilder_CopyToPersistence_roundtrip(t *testing.T) {

	for i := 0; i <= 100; i++ {
		ctrl := gomock.NewController(t)

		seed := int64(rand.Int())
		fuzzer := testdatagen.NewWithNilChance(t, seed, 0)

		execution := &persistence.WorkflowMutableState{}
		fuzzer.Fuzz(&execution)

		// checksum is a calculated value, zero it out because
		// it'll be overwridden during the constructor setup
		execution.Checksum = checksum.Checksum{}

		shardContext := shard.NewMockContext(ctrl)
		mockCache := events.NewMockCache(ctrl)
		mockDomainCache := cache.NewMockDomainCache(ctrl)
		mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return("some-domain-id", nil).AnyTimes()

		shardContext.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).Times(2)
		shardContext.EXPECT().GetEventsCache().Return(mockCache)
		shardContext.EXPECT().GetConfig().Return(&config.Config{
			NumberOfShards:                        2,
			IsAdvancedVisConfigExist:              false,
			MaxResponseSize:                       0,
			MutableStateChecksumInvalidateBefore:  dynamicproperties.GetFloatPropertyFn(10),
			MutableStateChecksumVerifyProbability: dynamicproperties.GetIntPropertyFilteredByDomain(0.0),
			HostName:                              "test-host",
		}).Times(1)
		shardContext.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSource())
		shardContext.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient())
		shardContext.EXPECT().GetDomainCache().Return(mockDomainCache).AnyTimes()
		shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()

		activeClusterManager := activecluster.NewMockManager(ctrl)
		shardContext.EXPECT().GetActiveClusterManager().Return(activeClusterManager).AnyTimes()

		msb := newMutableStateBuilder(shardContext, log.NewNoop(), constants.TestGlobalDomainEntry)

		msb.Load(context.Background(), execution)

		out := msb.CopyToPersistence()

		assert.Equal(t, execution.ActivityInfos, out.ActivityInfos, "activityinfos mismatch")
		assert.Equal(t, execution.TimerInfos, out.TimerInfos, "timerinfos mismatch")
		assert.Equal(t, execution.ChildExecutionInfos, out.ChildExecutionInfos, "child executino info mismatches")
		assert.Equal(t, execution.RequestCancelInfos, out.RequestCancelInfos, "request cancellantion info mismatches")
		assert.Equal(t, execution.SignalInfos, out.SignalInfos, "signal info mismatches")
		assert.Equal(t, execution.SignalRequestedIDs, out.SignalRequestedIDs, "signal request ids mismaches")
		assert.Equal(t, execution.ExecutionInfo, out.ExecutionInfo, "execution info mismatches")
		assert.Equal(t, execution.BufferedEvents, out.BufferedEvents, "buffered events mismatch")
		assert.Equal(t, execution.VersionHistories, out.VersionHistories, "version histories")
		assert.Equal(t, execution.Checksum, out.Checksum, "checksum mismatch")
		assert.Equal(t, execution.ReplicationState, out.ReplicationState, "replication state mismatch")
		assert.Equal(t, execution.ExecutionStats, out.ExecutionStats, "execution stats mismatch")

		assert.Equal(t, execution, out)

	}
}

func TestMutableStateBuilder_closeTransactionHandleWorkflowReset(t *testing.T) {
	t1 := time.Unix(123, 0)
	now := time.Unix(500, 0)

	badBinaryID := "bad-binary-id"

	mockDomainEntryWithBadBinary := cache.NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{Name: "domain"}, &persistence.DomainConfig{
		BadBinaries: types.BadBinaries{
			Binaries: map[string]*types.BadBinaryInfo{
				badBinaryID: &types.BadBinaryInfo{
					Reason:          "some-reason",
					Operator:        "",
					CreatedTimeNano: common.Ptr(t1.UnixNano()),
				},
			},
		},
	}, "cluster0")

	mockDomainEntryWithoutBadBinary := cache.NewLocalDomainCacheEntryForTest(nil, &persistence.DomainConfig{
		BadBinaries: types.BadBinaries{
			Binaries: map[string]*types.BadBinaryInfo{},
		},
	}, "cluster0")

	tests := map[string]struct {
		policyIn                         TransactionPolicy
		shardContextExpectations         func(mockCache *events.MockCache, shard *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache)
		mutableStateBuilderStartingState func(m *mutableStateBuilder)

		expectedEndState func(t *testing.T, m *mutableStateBuilder)
		expectedErr      error
	}{
		"a workflow with reset point which is running - the expectation is that this should be able to successfully find the domain to reset and add a transfer task": {

			policyIn: TransactionPolicyActive,
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				// the workflow's running
				m.executionInfo = &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusNone,
					DomainID:    "some-domain-id",
					WorkflowID:  "wf-id",
					AutoResetPoints: &types.ResetPoints{
						Points: []*types.ResetPointInfo{
							{
								BinaryChecksum:           badBinaryID,
								RunID:                    "",
								FirstDecisionCompletedID: 0,
								CreatedTimeNano:          common.Ptr(t1.UnixNano()),
								ExpiringTimeNano:         nil,
								Resettable:               true,
							},
						},
					},
				}
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GetDomainCache().Return(mockDomainCache).Times(1)
				mockDomainCache.EXPECT().GetDomainByID("some-domain-id").Return(mockDomainEntryWithBadBinary, nil)
			},
			expectedEndState: func(t *testing.T, m *mutableStateBuilder) {
				assert.Equal(t, []persistence.Task{
					&persistence.ResetWorkflowTask{
						WorkflowIdentifier: persistence.WorkflowIdentifier{
							DomainID:   "some-domain-id",
							WorkflowID: "wf-id",
						},
						TaskData: persistence.TaskData{
							Version: commonconstants.EmptyVersion,
						},
					},
				}, m.insertTransferTasks)
			},
		},
		"a workflow with reset point which is running for a domain without a bad binary - the expectation is this will not add any transfer tasks": {

			policyIn: TransactionPolicyActive,
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				// the workflow's running
				m.executionInfo = &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusNone,
					DomainID:    "some-domain-id",
					WorkflowID:  "wf-id",
					AutoResetPoints: &types.ResetPoints{
						Points: []*types.ResetPointInfo{
							{
								BinaryChecksum:           badBinaryID,
								RunID:                    "",
								FirstDecisionCompletedID: 0,
								CreatedTimeNano:          common.Ptr(t1.UnixNano()),
								ExpiringTimeNano:         nil,
								Resettable:               true,
							},
						},
					},
				}
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GetDomainCache().Return(mockDomainCache).Times(1)
				mockDomainCache.EXPECT().GetDomainByID("some-domain-id").Return(mockDomainEntryWithoutBadBinary, nil)
			},
			expectedEndState: func(t *testing.T, m *mutableStateBuilder) {
				assert.Equal(t, []persistence.Task(nil), m.insertTransferTasks)
			},
		},
		"a workflow withithout auto-reset point which is running for a domain": {

			policyIn: TransactionPolicyActive,
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				// the workflow's running
				m.executionInfo = &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusNone,
					DomainID:    "some-domain-id",
					WorkflowID:  "wf-id",
				}
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GetDomainCache().Return(mockDomainCache).Times(1)
				mockDomainCache.EXPECT().GetDomainByID("some-domain-id").Return(mockDomainEntryWithoutBadBinary, nil)
			},
			expectedEndState: func(t *testing.T, m *mutableStateBuilder) {
				assert.Equal(t, []persistence.Task(nil), m.insertTransferTasks)
			},
		},
		"a workflow with reset point which is running but which has child workflows": {
			policyIn: TransactionPolicyActive,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
			},
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				// the workflow's running
				m.executionInfo = &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}

				// there's some child workflow that's due to be updated
				m.pendingChildExecutionInfoIDs = map[int64]*persistence.ChildExecutionInfo{
					1: &persistence.ChildExecutionInfo{},
				}
			},
			expectedEndState: func(t *testing.T, m *mutableStateBuilder) {
				assert.Equal(t, []persistence.Task(nil), m.insertTransferTasks)
			},
		},
		"Transaction policy passive - no expected resets": {
			policyIn: TransactionPolicyPassive,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
			},
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				// the workflow's running
				m.executionInfo = &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}

				// there's some child workflow that's due to be updated
				m.pendingChildExecutionInfoIDs = map[int64]*persistence.ChildExecutionInfo{
					1: &persistence.ChildExecutionInfo{},
				}
			},
			expectedEndState: func(t *testing.T, m *mutableStateBuilder) {
				assert.Equal(t, []persistence.Task(nil), m.insertTransferTasks)
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shard.NewMockContext(ctrl)
			mockCache := events.NewMockCache(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()
			td.shardContextExpectations(mockCache, shardContext, mockDomainCache)

			nowClock := clock.NewMockedTimeSourceAt(now)

			msb := createMSBWithMocks(mockCache, shardContext, mockDomainCache)

			td.mutableStateBuilderStartingState(msb)

			msb.timeSource = nowClock
			err := msb.closeTransactionHandleWorkflowReset(td.policyIn)
			assert.Equal(t, td.expectedErr, err)
			td.expectedEndState(t, msb)
		})
	}
}

func TestMutableStateBuilder_GetVersionHistoriesStart(t *testing.T) {
	tests := map[string]struct {
		mutableStateBuilderStartingState func(m *mutableStateBuilder)
		expectedVersion                  int64
		expectedErr                      error
	}{
		"A mutable state with version history": {
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				m.versionHistories = &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte("branch-token1"),
							Items: []*persistence.VersionHistoryItem{
								{
									EventID: 100,
									Version: 23,
								},
								{
									EventID: 401,
									Version: 424,
								},
							},
						},
						{
							BranchToken: []byte("branch-token1"),
							Items: []*persistence.VersionHistoryItem{
								{
									EventID: 200,
									Version: 123,
								},
								{
									EventID: 201,
									Version: 124,
								},
							},
						},
					},
				}
			},
			expectedVersion: 23,
		},
		"invalid / partial version history ": {
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				m.versionHistories = &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte("branch-token1"),
							Items:       []*persistence.VersionHistoryItem{},
						},
						{
							BranchToken: []byte("branch-token2"),
							Items:       []*persistence.VersionHistoryItem{},
						},
					},
				}
			},
			expectedErr:     &types.BadRequestError{Message: "version history is empty."},
			expectedVersion: 0,
		},
		"invalid / partial version history - branch not available": {
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
				m.versionHistories = &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 10,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte("branch-token1"),
							Items:       []*persistence.VersionHistoryItem{},
						},
						{
							BranchToken: []byte("branch-token1"),
							Items:       []*persistence.VersionHistoryItem{},
						},
					},
				}
			},
			expectedErr:     &types.BadRequestError{Message: "getting branch index: 10, available branch count: 2"},
			expectedVersion: 0,
		},
		"nil version history": {
			mutableStateBuilderStartingState: func(m *mutableStateBuilder) {
			},
			expectedVersion: commonconstants.EmptyVersion,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shard.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()

			mockCache := events.NewMockCache(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			msb := createMSBWithMocks(mockCache, shardContext, mockDomainCache)

			td.mutableStateBuilderStartingState(msb)

			res, err := msb.GetStartVersion()
			assert.Equal(t, td.expectedErr, err)
			assert.Equal(t, td.expectedVersion, res)
		})
	}
}

func TestIsCurrentWorkflowGuaranteed(t *testing.T) {
	tests := []struct {
		name           string
		stateInDB      int
		expectedResult bool
	}{
		{
			name:           "Workflow is created",
			stateInDB:      persistence.WorkflowStateCreated,
			expectedResult: true,
		},
		{
			name:           "Workflow is running",
			stateInDB:      persistence.WorkflowStateRunning,
			expectedResult: true,
		},
		{
			name:           "Workflow is completed",
			stateInDB:      persistence.WorkflowStateCompleted,
			expectedResult: false,
		},
		{
			name:           "Workflow is zombie",
			stateInDB:      persistence.WorkflowStateZombie,
			expectedResult: false,
		},
		{
			name:           "Workflow is void",
			stateInDB:      persistence.WorkflowStateVoid,
			expectedResult: false,
		},
		{
			name:           "Workflow is corrupted",
			stateInDB:      persistence.WorkflowStateCorrupted,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msb := mutableStateBuilder{}
			msb.stateInDB = tt.stateInDB
			result := msb.IsCurrentWorkflowGuaranteed()
			assert.Equal(t, tt.expectedResult, result)
		})
	}

	assert.Panics(t, func() {
		msb := mutableStateBuilder{}
		msb.stateInDB = 123
		msb.IsCurrentWorkflowGuaranteed()
	})
}

// this is a pretty poor test, the actual logic is better tested in the
// unit tests for getBackoffInterval()
func TestGetRetryBackoffDuration(t *testing.T) {

	tests := []struct {
		name            string
		retryPolicy     *persistence.WorkflowExecutionInfo
		errorReason     string
		expectedBackoff time.Duration
	}{
		{
			name: "NoRetryPolicy",
			retryPolicy: &persistence.WorkflowExecutionInfo{
				HasRetryPolicy: false,
			},
			errorReason:     "some error reason",
			expectedBackoff: backoff.NoBackoff,
		},
		{
			name: "WithRetryPolicy",
			retryPolicy: &persistence.WorkflowExecutionInfo{
				HasRetryPolicy:     true,
				ExpirationTime:     time.Now().Add(time.Hour),
				Attempt:            1,
				MaximumAttempts:    5,
				BackoffCoefficient: 2.0,
				InitialInterval:    12,
				NonRetriableErrors: []string{"non-retriable-error"},
			},
			errorReason:     "some error reason",
			expectedBackoff: 24 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			t1 := time.Unix(1730247795, 0)

			msb := mutableStateBuilder{}

			msb.executionInfo = tt.retryPolicy
			msb.timeSource = clock.NewMockedTimeSourceAt(t1)

			duration := msb.GetRetryBackoffDuration(tt.errorReason)
			assert.Equal(t, tt.expectedBackoff, duration)
		})
	}
}

func TestGetCronRetryBackoffDuration(t *testing.T) {

	t1 := time.Unix(1730247795, 0)

	sampleVersionHistory := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte("branch-token1"),
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: 100,
						Version: 23,
					},
					{
						EventID: 401,
						Version: 424,
					},
				},
			},
			{
				BranchToken: []byte("branch-token1"),
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: 200,
						Version: 123,
					},
					{
						EventID: 201,
						Version: 124,
					},
				},
			},
		},
	}

	startEvent := &types.HistoryEvent{
		ID:                                      1,
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
	}

	tests := map[string]struct {
		startingExecutionInfo    *persistence.WorkflowExecutionInfo
		expectedErr              bool
		shardContextExpectations func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache)
		expectedBackoff          time.Duration
	}{
		"with simple, valid cron schedule": {
			startingExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:       "domain-id",
				CronSchedule:   "* * * * *",
				RunID:          "run-id",
				WorkflowID:     "wid",
				StartTimestamp: t1,
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GetShardID().Return(12)
				mockCache.EXPECT().GetEvent(gomock.Any(), 12, "domain-id", "wid", "run-id", int64(1), int64(1), []byte("branch-token1")).Return(startEvent, nil)
			},
			expectedBackoff: 45 * time.Second,
		},
		"with no cron schedule": {
			startingExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:       "domain-id",
				RunID:          "run-id",
				WorkflowID:     "wid",
				StartTimestamp: t1,
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
			},
			expectedBackoff: backoff.NoBackoff,
		},
		"with invalid start event": {
			startingExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:       "domain-id",
				RunID:          "run-id",
				WorkflowID:     "wid",
				CronSchedule:   "* * * * *",
				StartTimestamp: t1,
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GetShardID().Return(12)
				mockCache.EXPECT().GetEvent(gomock.Any(), 12, "domain-id", "wid", "run-id", int64(1), int64(1), []byte("branch-token1")).Return(nil, assert.AnError)
			},
			expectedBackoff: backoff.NoBackoff,
			expectedErr:     true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shard.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()
			mockCache := events.NewMockCache(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			td.shardContextExpectations(mockCache, shardContext, mockDomainCache)

			msb := createMSBWithMocks(mockCache, shardContext, mockDomainCache)

			msb.executionInfo = td.startingExecutionInfo
			msb.versionHistories = sampleVersionHistory
			msb.timeSource = clock.NewMockedTimeSourceAt(t1)

			duration, err := msb.GetCronBackoffDuration(context.Background())
			assert.Equal(t, td.expectedBackoff, duration)
			if td.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStartTransactionHandleFailover(t *testing.T) {

	tests := map[string]struct {
		incomingTaskVersion       int64
		currentVersion            int64
		decisionManagerAffordance func(m *MockmutableStateDecisionTaskManager)
		expectFlushBeforeReady    bool
		expectedErr               bool
	}{
		"Failing over from cluster2 to cluster1 - passive -> passive: There's an inflight decision, but it's from an earlier version": {
			incomingTaskVersion: 10,
			currentVersion:      2,
			decisionManagerAffordance: func(m *MockmutableStateDecisionTaskManager) {
				m.EXPECT().GetInFlightDecision().Return(&DecisionInfo{
					Version: 2,
				}, true)
			},
			expectFlushBeforeReady: false,
		},
		// todo: David.porter - look a bit more into why this could occur and write a better description
		// about what the intent is, because this is a unit test without a clear intent or outcome.
		// At the time of writing this test I believe this is a migration case, but I'm not 100% sure and
		// need to do some runtime debugging.
		"empty version": {
			incomingTaskVersion: commonconstants.EmptyVersion,
			currentVersion:      2,
			decisionManagerAffordance: func(m *MockmutableStateDecisionTaskManager) {
				m.EXPECT().GetInFlightDecision().Return(&DecisionInfo{
					Version: 2,
				}, true)
			},
			expectFlushBeforeReady: false,
		},
		"active -> passive - when there's an inflight decision from an earlier version": {
			incomingTaskVersion: 12,
			currentVersion:      11,
			decisionManagerAffordance: func(m *MockmutableStateDecisionTaskManager) {
				m.EXPECT().GetInFlightDecision().Return(&DecisionInfo{
					Version:    2,
					StartedID:  123,
					ScheduleID: 124,
					RequestID:  "requestID",
				}, true)
				m.EXPECT().AddDecisionTaskFailedEvent(int64(124), int64(123), types.DecisionTaskFailedCauseFailoverCloseDecision, gomock.Any(), "history-service", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().HasInFlightDecision().Return(true)
				m.EXPECT().HasInFlightDecision().Return(true)
				m.EXPECT().HasPendingDecision().Return(true)
			},
			expectFlushBeforeReady: true,
		},
		"There's a decision for for the same level as the failover version": {
			incomingTaskVersion: 10,
			currentVersion:      10,
			decisionManagerAffordance: func(m *MockmutableStateDecisionTaskManager) {
				m.EXPECT().GetInFlightDecision().Return(&DecisionInfo{
					Version: 1,
				}, true)
			},
			expectFlushBeforeReady: false,
			expectedErr:            true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			clusterMetadata := cluster.NewMetadata(
				commonConfig.ClusterGroupMetadata{
					FailoverVersionIncrement: 10,
					PrimaryClusterName:       "cluster0",
					CurrentClusterName:       "cluster0",
					ClusterGroup: map[string]commonConfig.ClusterInformation{
						"cluster0": {
							Enabled:                true,
							InitialFailoverVersion: 1,
						},
						"cluster1": {
							Enabled:                true,
							InitialFailoverVersion: 0,
						},
						"cluster2": {
							Enabled:                true,
							InitialFailoverVersion: 2,
						},
					},
				},
				func(string) bool { return false },
				metrics.NewNoopMetricsClient(),
				log.NewNoop(),
			)

			shardContext := shardCtx.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()

			activeClusterManager := activecluster.NewMockManager(ctrl)
			activeClusterManager.EXPECT().ClusterNameForFailoverVersion(gomock.Any(), gomock.Any()).DoAndReturn(func(version int64, domainID string) (string, error) {
				return clusterMetadata.ClusterNameForFailoverVersion(version)
			}).AnyTimes()
			shardContext.EXPECT().GetActiveClusterManager().Return(activeClusterManager).AnyTimes()

			decisionManager := NewMockmutableStateDecisionTaskManager(ctrl)
			td.decisionManagerAffordance(decisionManager)

			shardContext.EXPECT().GetConfig().Return(&config.Config{
				NumberOfShards:                        3,
				IsAdvancedVisConfigExist:              false,
				MaxResponseSize:                       0,
				MutableStateChecksumInvalidateBefore:  dynamicproperties.GetFloatPropertyFn(10),
				MutableStateChecksumVerifyProbability: dynamicproperties.GetIntPropertyFilteredByDomain(0.0),
				EnableReplicationTaskGeneration:       func(_ string, _ string) bool { return true },
				HostName:                              "test-host",
			}).Times(1)

			domainEntry := cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{
				ID:   "domain-id",
				Name: "domain",
			},
				&persistence.DomainConfig{},
				true,
				&persistence.DomainReplicationConfig{
					ActiveClusterName: "cluster0",
					Clusters: []*persistence.ClusterReplicationConfig{
						{ClusterName: "cluster0"},
						{ClusterName: "cluster1"},
						{ClusterName: "cluster2"},
					},
				}, 0, nil, 0, 0, 0)

			msb := mutableStateBuilder{
				decisionTaskManager: decisionManager,
				shard:               shardContext,
				domainEntry:         domainEntry,
				clusterMetadata:     clusterMetadata,
				currentVersion:      td.currentVersion,
				versionHistories: &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte("token"),
							Items: []*persistence.VersionHistoryItem{
								{
									EventID: 3,
									Version: 10,
								},
								{
									EventID: 2,
									Version: 2,
								},
							},
						},
					},
				},
				executionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:               "domainID",
					WorkflowID:             "workflowID",
					RunID:                  "some-example-run",
					FirstExecutionRunID:    "",
					ParentDomainID:         "",
					ParentWorkflowID:       "",
					ParentRunID:            "",
					InitiatedID:            0,
					CompletionEventBatchID: 0,
					CompletionEvent:        nil,
					State:                  0,
					CloseStatus:            0,
					LastFirstEventID:       0,
					LastEventTaskID:        0,
					NextEventID:            0,
					LastProcessedEvent:     0,
				},
				logger: testlogger.New(t),
			}

			msb.hBuilder = NewHistoryBuilder(&msb)

			flushBeforeReady, err := msb.startTransactionHandleDecisionFailover(td.incomingTaskVersion)
			if td.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, td.expectFlushBeforeReady, flushBeforeReady)
		})
	}
}

func TestSimpleGetters(t *testing.T) {

	msb := createMSB(t)
	assert.Equal(t, msb.versionHistories, msb.GetVersionHistories())

	branchToken, err := msb.GetCurrentBranchToken()
	assert.Equal(t, msb.versionHistories.Histories[0].BranchToken, branchToken)
	assert.NoError(t, err)
	assert.Equal(t, msb.currentVersion, msb.GetCurrentVersion())
	assert.Equal(t, msb.domainEntry, msb.GetDomainEntry())
	assert.Equal(t, msb.executionInfo, msb.GetExecutionInfo())
	assert.Equal(t, msb.hBuilder, msb.GetHistoryBuilder())
	assert.Equal(t, msb.executionStats.HistorySize, msb.GetHistorySize())
	assert.Equal(t, msb.executionInfo.LastFirstEventID, msb.GetLastFirstEventID())
	lastWriteVersion, err := msb.GetLastWriteVersion()

	item, err := msb.versionHistories.Histories[0].GetLastItem()
	assert.NoError(t, err)
	assert.Equal(t, item.Version, lastWriteVersion)

	assert.Equal(t, msb.executionInfo.NextEventID, msb.GetNextEventID())
	assert.Equal(t, msb.pendingRequestCancelInfoIDs, msb.GetPendingRequestCancelExternalInfos())
	assert.Equal(t, msb.executionInfo.LastProcessedEvent, msb.GetPreviousStartedEventID())
	assert.Equal(t, msb.queryRegistry, msb.GetQueryRegistry())

	startVersion, err := msb.GetStartVersion()
	assert.NoError(t, err)
	assert.Equal(t, msb.versionHistories.Histories[0].Items[0].Version, startVersion)
	assert.Equal(t, msb.insertTimerTasks, msb.GetTimerTasks())
	assert.Equal(t, msb.insertTransferTasks, msb.GetTransferTasks())
	assert.Equal(t, msb.nextEventIDInDB, msb.GetUpdateCondition())
	assert.Equal(t, msb.versionHistories, msb.GetVersionHistories())

	state, closeStatus := msb.GetWorkflowStateCloseStatus()
	assert.Equal(t, msb.executionInfo.CloseStatus, closeStatus)
	assert.Equal(t, msb.executionInfo.State, state)
	assert.Equal(t, &types.WorkflowType{Name: msb.executionInfo.WorkflowTypeName}, msb.GetWorkflowType())

	pendingActivityInfo, activityInfoIsPresent := msb.GetActivityInfo(1232)
	assert.Equal(t, msb.pendingActivityInfoIDs[1232], pendingActivityInfo)
	assert.True(t, activityInfoIsPresent)

	assert.Equal(t, msb.pendingActivityInfoIDs, msb.GetPendingActivityInfos())

	pendingRequestCancelledInfo, ok := msb.GetRequestCancelInfo(13)
	assert.Equal(t, msb.pendingRequestCancelInfoIDs[13], pendingRequestCancelledInfo)
	assert.True(t, ok)

	pendingChildExecutions, ok := msb.GetChildExecutionInfo(1)
	assert.Equal(t, msb.pendingChildExecutionInfoIDs[1], pendingChildExecutions)
	assert.True(t, ok)

}

func TestMutableState_IsCurrentWorkflowGuaranteed(t *testing.T) {
	tests := map[string]struct {
		state    int
		expected bool
	}{
		"created": {
			state:    persistence.WorkflowStateCreated,
			expected: true,
		},
		"running": {
			state:    persistence.WorkflowStateCreated,
			expected: true,
		},
		"completed": {
			state:    persistence.WorkflowStateCompleted,
			expected: false,
		},
		"void": {
			state:    persistence.WorkflowStateVoid,
			expected: false,
		},
		"zombie state": {
			state:    persistence.WorkflowStateZombie,
			expected: false,
		},
		"corrupted state": {
			state:    persistence.WorkflowStateCorrupted,
			expected: false,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			msb := mutableStateBuilder{
				stateInDB: td.state,
				logger:    testlogger.New(t),
			}
			assert.Equal(t, td.expected, msb.IsCurrentWorkflowGuaranteed())
		})
	}
}

func createMSB(t *testing.T) mutableStateBuilder {

	sampleDomain := cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{ID: "domain-id", Name: "domain"}, &persistence.DomainConfig{}, true, nil, 0, nil, 0, 0, 0)

	return mutableStateBuilder{
		pendingActivityInfoIDs: map[int64]*persistence.ActivityInfo{
			1232: &persistence.ActivityInfo{ActivityID: "activityID"},
		},
		pendingActivityIDToEventID: map[string]int64{
			"activityID": 6,
		},
		updateActivityInfos: map[int64]*persistence.ActivityInfo{
			7: &persistence.ActivityInfo{DomainID: "domainID"},
		},
		deleteActivityInfos: map[int64]struct{}{
			8: struct{}{},
		},
		syncActivityTasks: map[int64]struct{}{},
		pendingTimerInfoIDs: map[string]*persistence.TimerInfo{
			"testdata-pendingTimerInfoIDs": &persistence.TimerInfo{
				Version: 1,
				TimerID: "1232",
			},
		},
		pendingTimerEventIDToID: map[int64]string{},
		updateTimerInfos: map[string]*persistence.TimerInfo{
			"testdata-updatedtimerinfos": &persistence.TimerInfo{
				Version: 1,
				TimerID: "1232",
			},
		},
		deleteTimerInfos: map[string]struct{}{},
		pendingChildExecutionInfoIDs: map[int64]*persistence.ChildExecutionInfo{
			1: &persistence.ChildExecutionInfo{
				WorkflowTypeName: "sample-workflow",
			},
		},
		updateChildExecutionInfos: map[int64]*persistence.ChildExecutionInfo{
			8: &persistence.ChildExecutionInfo{DomainID: "updateChildInfosDomainID"},
		},
		deleteChildExecutionInfos: map[int64]struct{}{
			12: struct{}{},
		},
		pendingRequestCancelInfoIDs: map[int64]*persistence.RequestCancelInfo{
			13: &persistence.RequestCancelInfo{InitiatedID: 16},
		},
		updateRequestCancelInfos: map[int64]*persistence.RequestCancelInfo{},
		deleteRequestCancelInfos: map[int64]struct{}{
			15: struct{}{},
		},
		pendingSignalInfoIDs:      map[int64]*persistence.SignalInfo{},
		updateSignalInfos:         map[int64]*persistence.SignalInfo{},
		deleteSignalInfos:         map[int64]struct{}{},
		pendingSignalRequestedIDs: map[string]struct{}{},
		updateSignalRequestedIDs:  map[string]struct{}{},
		deleteSignalRequestedIDs:  map[string]struct{}{},
		bufferedEvents:            []*types.HistoryEvent{},
		updateBufferedEvents:      []*types.HistoryEvent{},
		clearBufferedEvents:       false,
		executionInfo: &persistence.WorkflowExecutionInfo{
			DomainID:                           "d9cbf563-3056-4387-b2ac-5fddd868fe4d",
			WorkflowID:                         "53fc235c-093e-4b15-9d9d-045e61354b91",
			RunID:                              "a2901718-ac12-443e-873d-b100f45d55d8",
			FirstExecutionRunID:                "a2901718-ac12-443e-873d-b100f45d55d8",
			InitiatedID:                        -7,
			TaskList:                           "tl",
			WorkflowTypeName:                   "test",
			WorkflowTimeout:                    600000000,
			DecisionStartToCloseTimeout:        10,
			State:                              1,
			LastFirstEventID:                   1,
			LastEventTaskID:                    2097153,
			NextEventID:                        3,
			LastProcessedEvent:                 -23,
			StartTimestamp:                     time.Date(2024, 10, 21, 20, 58, 1, 275000000, time.UTC),
			LastUpdatedTimestamp:               time.Date(2024, 10, 21, 20, 58, 1, 275000000, time.UTC),
			CreateRequestID:                    "f33ee669-9ff6-4221-a2b0-feb2959667b8",
			DecisionVersion:                    1,
			DecisionScheduleID:                 2,
			DecisionStartedID:                  -23,
			DecisionRequestID:                  "emptyUuid",
			DecisionTimeout:                    10,
			DecisionScheduledTimestamp:         1729544281275414000,
			DecisionOriginalScheduledTimestamp: 1729544281275414000,
			AutoResetPoints:                    &types.ResetPoints{},
		},
		versionHistories: &persistence.VersionHistories{
			Histories: []*persistence.VersionHistory{
				{
					BranchToken: []byte("a branch token"),
					Items: []*persistence.VersionHistoryItem{{
						EventID: 2,
						Version: 1,
					}},
				},
			},
		},
		currentVersion:        int64(-24),
		hasBufferedEventsInDB: false,
		stateInDB:             int(1),
		nextEventIDInDB:       int64(3),
		domainEntry:           sampleDomain,
		appliedEvents:         map[string]struct{}{},
		insertTransferTasks: []persistence.Task{
			&persistence.DecisionTask{
				TargetDomainID: "decsion task",
			},
		},
		insertReplicationTasks: []persistence.Task{},
		insertTimerTasks: []persistence.Task{
			&persistence.ActivityRetryTimerTask{
				TaskData: persistence.TaskData{},
				EventID:  123,
				Attempt:  4,
			},
		},
		workflowRequests: map[persistence.WorkflowRequest]struct{}{},
		checksum:         checksum.Checksum{},
		executionStats:   &persistence.ExecutionStats{HistorySize: 403},
		queryRegistry:    query.NewRegistry(),
		logger:           testlogger.New(t),
	}
}

func TestMutableStateBuilder_GetTransferTasks(t *testing.T) {
	msb := &mutableStateBuilder{
		insertTransferTasks: []persistence.Task{
			&persistence.ActivityTask{},
			&persistence.DecisionTask{},
		},
	}
	tasks := msb.GetTransferTasks()
	assert.Equal(t, 2, len(tasks))
	assert.IsType(t, &persistence.ActivityTask{}, tasks[0])
	assert.IsType(t, &persistence.DecisionTask{}, tasks[1])
}

func TestMutableStateBuilder_GetTimerTasks(t *testing.T) {
	msb := &mutableStateBuilder{
		insertTimerTasks: []persistence.Task{
			&persistence.UserTimerTask{},
		},
	}
	tasks := msb.GetTimerTasks()
	assert.Equal(t, 1, len(tasks))
	assert.IsType(t, &persistence.UserTimerTask{}, tasks[0])
}

func TestMutableStateBuilder_DeleteTransferTasks(t *testing.T) {
	msb := &mutableStateBuilder{
		insertTransferTasks: []persistence.Task{
			&persistence.ActivityTask{},
		},
	}
	msb.DeleteTransferTasks()
	assert.Nil(t, msb.insertTransferTasks)
}

func TestMutableStateBuilder_DeleteTimerTasks(t *testing.T) {
	msb := &mutableStateBuilder{
		insertTimerTasks: []persistence.Task{
			&persistence.UserTimerTask{},
		},
	}
	msb.DeleteTimerTasks()
	assert.Nil(t, msb.insertTimerTasks)
}

func TestMutableStateBuilder_SetUpdateCondition(t *testing.T) {
	msb := &mutableStateBuilder{}
	msb.SetUpdateCondition(123)
	assert.Equal(t, int64(123), msb.nextEventIDInDB)
}

func TestMutableStateBuilder_GetUpdateCondition(t *testing.T) {
	msb := &mutableStateBuilder{
		nextEventIDInDB: 123,
	}
	assert.Equal(t, int64(123), msb.GetUpdateCondition())
}

func TestCheckAndClearTimerFiredEvent(t *testing.T) {
	tests := []struct {
		name                         string
		timerID                      string
		bufferedEvents               []*types.HistoryEvent
		updateBufferedEvents         []*types.HistoryEvent
		history                      []*types.HistoryEvent
		expectedTimerEvent           *types.HistoryEvent
		expectedBufferedEvents       []*types.HistoryEvent
		expectedUpdateBufferedEvents []*types.HistoryEvent
		expectedHistory              []*types.HistoryEvent
	}{
		{
			name:    "TimerFiredEventInBufferedEvents",
			timerID: "timer1",
			bufferedEvents: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer1",
					},
				},
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer2",
					},
				},
			},
			updateBufferedEvents: []*types.HistoryEvent{},
			history:              []*types.HistoryEvent{},
			expectedTimerEvent: &types.HistoryEvent{
				EventType: types.EventTypeTimerFired.Ptr(),
				TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
					TimerID: "timer1",
				},
			},
			expectedBufferedEvents: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer2",
					},
				},
			},
			expectedUpdateBufferedEvents: []*types.HistoryEvent{},
			expectedHistory:              []*types.HistoryEvent{},
		},
		{
			name:           "TimerFiredEventInUpdateBufferedEvents",
			timerID:        "timer2",
			bufferedEvents: []*types.HistoryEvent{},
			updateBufferedEvents: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer1",
					},
				},
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer2",
					},
				},
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer3",
					},
				},
			},
			history: []*types.HistoryEvent{},
			expectedTimerEvent: &types.HistoryEvent{
				EventType: types.EventTypeTimerFired.Ptr(),
				TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
					TimerID: "timer2",
				},
			},
			expectedBufferedEvents: []*types.HistoryEvent{},
			expectedUpdateBufferedEvents: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer1",
					},
				},
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer3",
					},
				},
			},
			expectedHistory: []*types.HistoryEvent{},
		},
		{
			name:                 "TimerFiredEventInHistory",
			timerID:              "timer3",
			bufferedEvents:       []*types.HistoryEvent{},
			updateBufferedEvents: []*types.HistoryEvent{},
			history: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer1",
					},
				},
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer3",
					},
				},
			},
			expectedTimerEvent: &types.HistoryEvent{
				EventType: types.EventTypeTimerFired.Ptr(),
				TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
					TimerID: "timer3",
				},
			},
			expectedBufferedEvents:       []*types.HistoryEvent{},
			expectedUpdateBufferedEvents: []*types.HistoryEvent{},
			expectedHistory: []*types.HistoryEvent{
				{
					EventType: types.EventTypeTimerFired.Ptr(),
					TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
						TimerID: "timer1",
					},
				},
			},
		},
		{
			name:                         "NoTimerFiredEvent",
			timerID:                      "timer4",
			bufferedEvents:               []*types.HistoryEvent{},
			updateBufferedEvents:         []*types.HistoryEvent{},
			history:                      []*types.HistoryEvent{},
			expectedTimerEvent:           nil,
			expectedBufferedEvents:       []*types.HistoryEvent{},
			expectedUpdateBufferedEvents: []*types.HistoryEvent{},
			expectedHistory:              []*types.HistoryEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msb := &mutableStateBuilder{
				bufferedEvents:       tt.bufferedEvents,
				updateBufferedEvents: tt.updateBufferedEvents,
				hBuilder:             &HistoryBuilder{history: tt.history},
			}

			timerEvent := msb.checkAndClearTimerFiredEvent(tt.timerID)

			assert.Equal(t, tt.expectedTimerEvent, timerEvent)
			assert.Equal(t, tt.expectedBufferedEvents, msb.bufferedEvents)
			assert.Equal(t, tt.expectedUpdateBufferedEvents, msb.updateBufferedEvents)
			assert.Equal(t, tt.expectedHistory, msb.hBuilder.history)
		})
	}
}

func TestAssignTaskIDToTransientHistoryEvents(t *testing.T) {

	tests := map[string]struct {
		transientHistory         []*types.HistoryEvent
		taskID                   int64
		shardContextExpectations func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache)
		expectedEvents           []*types.HistoryEvent
		expectedErr              error
	}{
		"AssignTaskIDToSingleEvent - transient": {
			transientHistory: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			taskID: 123,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GenerateTaskIDs(1).Return([]int64{123}, nil).Times(1)
			},
			expectedEvents: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    123,
				},
			},
		},
		"AssignTaskIDToMultipleEvents - transient": {
			transientHistory: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
				{
					ID:        2,
					EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			taskID: 456,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GenerateTaskIDs(2).Return([]int64{123, 124}, nil).Times(1)
			},
			expectedEvents: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    123,
				},
				{
					ID:        2,
					EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
					TaskID:    124,
				},
			},
		},
		"NoEvents - transient events": {
			transientHistory: []*types.HistoryEvent{},
			taskID:           789,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
			},
			expectedEvents: []*types.HistoryEvent{},
		},
		"error returned": {
			transientHistory: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			taskID: 456,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GenerateTaskIDs(1).Return(nil, assert.AnError).Times(1)
			},
			expectedEvents: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			expectedErr: assert.AnError,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shard.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()
			mockCache := events.NewMockCache(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			td.shardContextExpectations(mockCache, shardContext, mockDomainCache)

			msb := createMSBWithMocks(mockCache, shardContext, mockDomainCache)

			msb.hBuilder.transientHistory = td.transientHistory

			err := msb.assignTaskIDToEvents()

			assert.Equal(t, td.expectedEvents, msb.hBuilder.transientHistory)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func TestAssignTaskIDToHistoryEvents(t *testing.T) {

	tests := map[string]struct {
		history                  []*types.HistoryEvent
		taskID                   int64
		shardContextExpectations func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache)
		expectedEvents           []*types.HistoryEvent
		expectedErr              error
	}{
		"AssignTaskIDToSingleEvent": {
			history: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			taskID: 123,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GenerateTaskIDs(1).Return([]int64{123}, nil).Times(1)
			},
			expectedEvents: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    123,
				},
			},
		},
		"AssignTaskIDToMultipleEvents": {
			history: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
				{
					ID:        2,
					EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			taskID: 456,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GenerateTaskIDs(2).Return([]int64{123, 124}, nil).Times(1)
			},
			expectedEvents: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    123,
				},
				{
					ID:        2,
					EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
					TaskID:    124,
				},
			},
		},
		"NoEvents - transient events": {
			history: []*types.HistoryEvent{},
			taskID:  789,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
			},
			expectedEvents: []*types.HistoryEvent{},
		},
		"error returned": {
			history: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			taskID: 456,
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GenerateTaskIDs(1).Return(nil, assert.AnError).Times(1)
			},
			expectedEvents: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					TaskID:    commonconstants.EmptyEventTaskID,
				},
			},
			expectedErr: assert.AnError,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shard.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()
			mockCache := events.NewMockCache(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			td.shardContextExpectations(mockCache, shardContext, mockDomainCache)

			msb := createMSBWithMocks(mockCache, shardContext, mockDomainCache)

			msb.hBuilder.history = td.history

			err := msb.assignTaskIDToEvents()

			assert.Equal(t, td.expectedEvents, msb.hBuilder.history)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func TestAddUpsertWorkflowSearchAttributesEvent(t *testing.T) {

	now := time.Unix(1730353941, 0)

	tests := map[string]struct {
		decisionCompletedEventID int64
		request                  *types.UpsertWorkflowSearchAttributesDecisionAttributes
		mutableStateBuilderSetup func(m *mutableStateBuilder)
		expectedEvent            *types.HistoryEvent
		expectedErr              error
	}{
		"successful upsert": {
			decisionCompletedEventID: 123,
			request: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: &types.SearchAttributes{
					IndexedFields: map[string][]byte{
						"CustomKeywordField": []byte("keyword"),
					},
				},
			},
			mutableStateBuilderSetup: func(m *mutableStateBuilder) {
			},
			expectedEvent: &types.HistoryEvent{
				ID:        1,
				EventType: types.EventTypeUpsertWorkflowSearchAttributes.Ptr(),
				UpsertWorkflowSearchAttributesEventAttributes: &types.UpsertWorkflowSearchAttributesEventAttributes{
					DecisionTaskCompletedEventID: 123,
					SearchAttributes: &types.SearchAttributes{
						IndexedFields: map[string][]byte{
							"CustomKeywordField": []byte("keyword"),
						},
					},
				},
				TaskID:    commonconstants.EmptyEventTaskID,
				Version:   commonconstants.EmptyVersion,
				Timestamp: common.Ptr(now.UnixNano()),
			},
			expectedErr: nil,
		},
		"mutability check fails": {
			decisionCompletedEventID: 123,
			request: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: &types.SearchAttributes{
					IndexedFields: map[string][]byte{
						"CustomKeywordField": []byte("keyword"),
					},
				},
			},
			mutableStateBuilderSetup: func(m *mutableStateBuilder) {
				m.executionInfo.State = persistence.WorkflowStateCompleted
			},
			expectedEvent: nil,
			expectedErr:   &types.InternalServiceError{Message: "invalid mutable state action: mutation after finish"},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shard.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()
			mockCache := events.NewMockCache(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			nowClock := clock.NewMockedTimeSourceAt(now)

			msb := createMSBWithMocks(mockCache, shardContext, mockDomainCache)

			msb.hBuilder = &HistoryBuilder{
				history:   []*types.HistoryEvent{},
				msBuilder: msb,
			}

			td.mutableStateBuilderSetup(msb)

			msb.timeSource = nowClock

			event, err := msb.AddUpsertWorkflowSearchAttributesEvent(td.decisionCompletedEventID, td.request)

			assert.Equal(t, td.expectedEvent, event)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func TestCloseTransactionAsMutation(t *testing.T) {

	now := time.Unix(500, 0)

	mockDomain := cache.NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{Name: "domain"}, &persistence.DomainConfig{
		BadBinaries: types.BadBinaries{},
	}, "cluster0")

	tests := map[string]struct {
		mutableStateSetup        func(ms *mutableStateBuilder)
		shardContextExpectations func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache)
		transactionPolicy        TransactionPolicy
		expectedMutation         *persistence.WorkflowMutation
		expectedEvent            []*persistence.WorkflowEvents
		expectedErr              error
	}{
		"no buffered events": {
			mutableStateSetup: func(ms *mutableStateBuilder) {
				ms.executionInfo.DomainID = "some-domain-id"
				ms.executionInfo.NextEventID = 10
				ms.executionInfo.LastProcessedEvent = 5
				ms.executionInfo.State = persistence.WorkflowStateRunning
				ms.executionInfo.CloseStatus = persistence.WorkflowCloseStatusNone
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GetConfig().Return(&config.Config{
					NumberOfShards:                        2,
					IsAdvancedVisConfigExist:              false,
					MaxResponseSize:                       0,
					MutableStateChecksumInvalidateBefore:  dynamicproperties.GetFloatPropertyFn(10),
					MutableStateChecksumVerifyProbability: dynamicproperties.GetIntPropertyFilteredByDomain(0.0),
					HostName:                              "test-host",
					EnableReplicationTaskGeneration:       func(string, string) bool { return true },
					MaximumBufferedEventsBatch:            func(...dynamicproperties.FilterOption) int { return 100 },
				}).Times(2)

				shardContext.EXPECT().GetDomainCache().Return(mockDomainCache).Times(1)
				mockDomainCache.EXPECT().GetDomainByID("some-domain-id").Return(mockDomain, nil)

			},
			expectedMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:             "some-domain-id",
					NextEventID:          10,
					LastProcessedEvent:   5,
					State:                persistence.WorkflowStateRunning,
					CloseStatus:          persistence.WorkflowCloseStatusNone,
					LastUpdatedTimestamp: now,
					DecisionVersion:      commonconstants.EmptyVersion,
					DecisionScheduleID:   commonconstants.EmptyEventID,
					DecisionRequestID:    commonconstants.EmptyUUID,
					DecisionStartedID:    commonconstants.EmptyEventID,
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryTransfer:    nil,
					persistence.HistoryTaskCategoryTimer:       nil,
					persistence.HistoryTaskCategoryReplication: nil,
				},
				UpsertActivityInfos:       []*persistence.ActivityInfo{},
				DeleteActivityInfos:       []int64{},
				UpsertTimerInfos:          []*persistence.TimerInfo{},
				DeleteTimerInfos:          []string{},
				UpsertChildExecutionInfos: []*persistence.ChildExecutionInfo{},
				UpsertRequestCancelInfos:  []*persistence.RequestCancelInfo{},
				DeleteRequestCancelInfos:  []int64{},
				UpsertSignalInfos:         []*persistence.SignalInfo{},
				DeleteSignalInfos:         []int64{},
				UpsertSignalRequestedIDs:  []string{},
				DeleteSignalRequestedIDs:  []string{},
				DeleteChildExecutionInfos: []int64{},
				WorkflowRequests:          []*persistence.WorkflowRequest{},
				Condition:                 0,
			},
			expectedEvent: nil,
			expectedErr:   nil,
		},
		"with buffered events": {
			mutableStateSetup: func(ms *mutableStateBuilder) {
				ms.executionInfo.DomainID = "some-domain-id"
				ms.executionInfo.NextEventID = 10
				ms.executionInfo.LastProcessedEvent = 5
				ms.executionInfo.State = persistence.WorkflowStateRunning
				ms.executionInfo.CloseStatus = persistence.WorkflowCloseStatusNone
				ms.bufferedEvents = []*types.HistoryEvent{
					{
						ID:        1,
						EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
					},
				}
			},
			shardContextExpectations: func(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) {
				shardContext.EXPECT().GetConfig().Return(&config.Config{
					NumberOfShards:                        2,
					IsAdvancedVisConfigExist:              false,
					MaxResponseSize:                       0,
					MutableStateChecksumInvalidateBefore:  dynamicproperties.GetFloatPropertyFn(10),
					MutableStateChecksumVerifyProbability: dynamicproperties.GetIntPropertyFilteredByDomain(0.0),
					HostName:                              "test-host",
					EnableReplicationTaskGeneration:       func(string, string) bool { return true },
					MaximumBufferedEventsBatch:            func(...dynamicproperties.FilterOption) int { return 100 },
				}).Times(3)

				shardContext.EXPECT().GenerateTaskIDs(1).Return([]int64{123}, nil).Times(1)
				shardContext.EXPECT().GetDomainCache().Return(mockDomainCache).Times(1)
				mockDomainCache.EXPECT().GetDomainByID("some-domain-id").Return(mockDomain, nil)

			},
			expectedMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:             "some-domain-id",
					NextEventID:          10,
					LastProcessedEvent:   5,
					State:                persistence.WorkflowStateRunning,
					CloseStatus:          persistence.WorkflowCloseStatusNone,
					LastUpdatedTimestamp: now,
					DecisionVersion:      commonconstants.EmptyVersion,
					DecisionScheduleID:   commonconstants.EmptyEventID,
					DecisionRequestID:    commonconstants.EmptyUUID,
					DecisionStartedID:    commonconstants.EmptyEventID,
					LastFirstEventID:     1,
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryTransfer: nil,
					persistence.HistoryTaskCategoryTimer:    nil,
					persistence.HistoryTaskCategoryReplication: []persistence.Task{
						&persistence.HistoryReplicationTask{
							WorkflowIdentifier: persistence.WorkflowIdentifier{
								DomainID:   "some-domain-id",
								WorkflowID: "",
								RunID:      "",
							},
							FirstEventID: 1,
							NextEventID:  2,
							TaskData: persistence.TaskData{
								Version:             0,
								TaskID:              0,
								VisibilityTimestamp: time.Time{},
							},
						},
					},
				},
				UpsertActivityInfos:       []*persistence.ActivityInfo{},
				DeleteActivityInfos:       []int64{},
				UpsertTimerInfos:          []*persistence.TimerInfo{},
				DeleteTimerInfos:          []string{},
				UpsertChildExecutionInfos: []*persistence.ChildExecutionInfo{},
				UpsertRequestCancelInfos:  []*persistence.RequestCancelInfo{},
				DeleteRequestCancelInfos:  []int64{},
				UpsertSignalInfos:         []*persistence.SignalInfo{},
				DeleteSignalInfos:         []int64{},
				UpsertSignalRequestedIDs:  []string{},
				DeleteSignalRequestedIDs:  []string{},
				DeleteChildExecutionInfos: []int64{},
				WorkflowRequests:          []*persistence.WorkflowRequest{},
				Condition:                 0,
			},
			expectedEvent: []*persistence.WorkflowEvents{
				{
					DomainID: "some-domain-id",
					Events: []*types.HistoryEvent{{
						ID: 1, EventType: types.EventTypeWorkflowExecutionStarted.Ptr()},
					},
				},
			},
			expectedErr: nil,
		},
	}

	for name, td := range tests {

		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			shardContext := shard.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()

			activeClusterManager := activecluster.NewMockManager(ctrl)
			shardContext.EXPECT().GetActiveClusterManager().Return(activeClusterManager).AnyTimes()

			mockCache := events.NewMockCache(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			ms := createMSBWithMocks(mockCache, shardContext, mockDomainCache)
			td.mutableStateSetup(ms)
			td.shardContextExpectations(mockCache, shardContext, mockDomainCache)

			activeClusterManager.EXPECT().ClusterNameForFailoverVersion(gomock.Any(), gomock.Any()).DoAndReturn(func(version int64, domainID string) (string, error) {
				return cluster.TestActiveClusterMetadata.GetCurrentClusterName(), nil
			}).AnyTimes()

			mutation, workflowEvents, err := ms.CloseTransactionAsMutation(now, td.transactionPolicy)
			assert.Equal(t, td.expectedMutation, mutation)
			assert.Equal(t, td.expectedEvent, workflowEvents)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func Test__reorderAndFilterDuplicateEvents(t *testing.T) {
	testCases := []struct {
		name        string
		buildEvents func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent)
		assertions  func(*testing.T, *observer.ObservedLogs)
	}{
		{
			name: "no duplicates",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskStarted)
				event1.ActivityTaskStartedEventAttributes = &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 1,
					Attempt:          1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskCompleted)
				event2.ActivityTaskCompletedEventAttributes = &types.ActivityTaskCompletedEventAttributes{
					ScheduledEventID: 1,
				}

				return []*types.HistoryEvent{event1, event2}, []*types.HistoryEvent{event1, event2}
			},
			assertions: func(t *testing.T, logs *observer.ObservedLogs) {
				assert.Equal(t, 0, logs.FilterMessage("Duplicate activity task event found").Len())
			},
		},
		{
			name: "started event duplicated",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskStarted)
				event1.ActivityTaskStartedEventAttributes = &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 1,
					Attempt:          1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskStarted)
				event2.ActivityTaskStartedEventAttributes = &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 1,
					Attempt:          1,
				}

				return []*types.HistoryEvent{event1, event2}, []*types.HistoryEvent{event1}
			},
			assertions: func(t *testing.T, logs *observer.ObservedLogs) {
				assert.Equal(t, 1, logs.FilterMessage("Duplicate activity task event found").Len())
			},
		},
		{
			name: "completed event duplicated",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskCompleted)
				event1.ActivityTaskCompletedEventAttributes = &types.ActivityTaskCompletedEventAttributes{
					ScheduledEventID: 1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskCompleted)
				event2.ActivityTaskCompletedEventAttributes = &types.ActivityTaskCompletedEventAttributes{
					ScheduledEventID: 1,
				}

				return []*types.HistoryEvent{event1, event2}, []*types.HistoryEvent{event1}
			},
			assertions: func(t *testing.T, logs *observer.ObservedLogs) {
				assert.Equal(t, 1, logs.FilterMessage("Duplicate activity task event found").Len())
			},
		},
		{
			name: "canceled event duplicated",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskCanceled)
				event1.ActivityTaskCanceledEventAttributes = &types.ActivityTaskCanceledEventAttributes{
					ScheduledEventID: 1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskCanceled)
				event2.ActivityTaskCanceledEventAttributes = &types.ActivityTaskCanceledEventAttributes{
					ScheduledEventID: 1,
				}

				return []*types.HistoryEvent{event1, event2}, []*types.HistoryEvent{event1}
			},
			assertions: func(t *testing.T, logs *observer.ObservedLogs) {
				assert.Equal(t, 1, logs.FilterMessage("Duplicate activity task event found").Len())
			},
		},
		{
			name: "failed event duplicated",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskFailed)
				event1.ActivityTaskFailedEventAttributes = &types.ActivityTaskFailedEventAttributes{
					ScheduledEventID: 1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskFailed)
				event2.ActivityTaskFailedEventAttributes = &types.ActivityTaskFailedEventAttributes{
					ScheduledEventID: 1,
				}

				return []*types.HistoryEvent{event1, event2}, []*types.HistoryEvent{event1}
			},
			assertions: func(t *testing.T, logs *observer.ObservedLogs) {
				assert.Equal(t, 1, logs.FilterMessage("Duplicate activity task event found").Len())
			},
		},
		{
			name: "timed out event duplicated",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskTimedOut)
				event1.ActivityTaskTimedOutEventAttributes = &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskTimedOut)
				event2.ActivityTaskTimedOutEventAttributes = &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 1,
				}

				return []*types.HistoryEvent{event1, event2}, []*types.HistoryEvent{event1}
			},
			assertions: func(t *testing.T, logs *observer.ObservedLogs) {
				assert.Equal(t, 1, logs.FilterMessage("Duplicate activity task event found").Len())
			},
		},
		{
			name: "reorder events",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskStarted)
				event1.ActivityTaskStartedEventAttributes = &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 1,
					Attempt:          1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskTimedOut)
				event2.ActivityTaskTimedOutEventAttributes = &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 1,
				}

				return []*types.HistoryEvent{event2, event1}, []*types.HistoryEvent{event1, event2}
			},
		},
		{
			name: "reorder all event types",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskStarted)
				event1.ActivityTaskStartedEventAttributes = &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 1,
					Attempt:          1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskCompleted)
				event2.ActivityTaskCompletedEventAttributes = &types.ActivityTaskCompletedEventAttributes{
					ScheduledEventID: 1,
				}
				event3 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskFailed)
				event3.ActivityTaskFailedEventAttributes = &types.ActivityTaskFailedEventAttributes{
					ScheduledEventID: 1,
				}
				event4 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskCanceled)
				event4.ActivityTaskCanceledEventAttributes = &types.ActivityTaskCanceledEventAttributes{
					ScheduledEventID: 1,
				}
				event5 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskTimedOut)
				event5.ActivityTaskTimedOutEventAttributes = &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 1,
				}
				event6 := msb.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionCompleted)
				event6.ChildWorkflowExecutionCompletedEventAttributes = &types.ChildWorkflowExecutionCompletedEventAttributes{
					StartedEventID: 1,
				}
				event7 := msb.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionFailed)
				event7.ChildWorkflowExecutionFailedEventAttributes = &types.ChildWorkflowExecutionFailedEventAttributes{
					StartedEventID: 1,
				}
				event8 := msb.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionCanceled)
				event8.ChildWorkflowExecutionCanceledEventAttributes = &types.ChildWorkflowExecutionCanceledEventAttributes{
					StartedEventID: 1,
				}
				event9 := msb.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionTimedOut)
				event9.ChildWorkflowExecutionTimedOutEventAttributes = &types.ChildWorkflowExecutionTimedOutEventAttributes{
					StartedEventID: 1,
				}
				event10 := msb.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionTerminated)
				event10.ChildWorkflowExecutionTerminatedEventAttributes = &types.ChildWorkflowExecutionTerminatedEventAttributes{
					StartedEventID: 1,
				}

				return []*types.HistoryEvent{event2, event3, event4, event5, event6, event7, event8, event9, event10, event1}, []*types.HistoryEvent{event1, event2, event3, event4, event5, event6, event7, event8, event9, event10}
			},
		},
		{
			name: "reorder and remove duplicate events",
			buildEvents: func(msb *mutableStateBuilder) ([]*types.HistoryEvent, []*types.HistoryEvent) {
				event1 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskStarted)
				event1.ActivityTaskStartedEventAttributes = &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 1,
					Attempt:          1,
				}
				event2 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskTimedOut)
				event2.ActivityTaskTimedOutEventAttributes = &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 1,
				}
				event3 := msb.CreateNewHistoryEvent(types.EventTypeActivityTaskTimedOut)
				event3.ActivityTaskTimedOutEventAttributes = &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 1,
				}

				return []*types.HistoryEvent{event2, event1, event3}, []*types.HistoryEvent{event1, event2}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			core, observedLogs := observer.New(zap.WarnLevel)

			mockCache := events.NewMockCache(ctrl)
			shardContext := shard.NewMockContext(ctrl)
			shardContext.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			msb := createMSBWithMocks(mockCache, shardContext, mockDomainCache)

			msb.logger = log.NewLogger(zap.New(core))

			msb.executionInfo.DomainID = "some-domain-id"
			msb.executionInfo.WorkflowID = "some-workflow-id"
			msb.executionInfo.RunID = "some-run-id"

			allEvents, nonDuplicate := tc.buildEvents(msb)

			result := msb.reorderAndFilterDuplicateEvents(allEvents, "test")

			assert.Equal(t, nonDuplicate, result)
			if tc.assertions != nil {
				tc.assertions(t, observedLogs)
			}
		})
	}
}

func createMSBWithMocks(mockCache *events.MockCache, shardContext *shardCtx.MockContext, mockDomainCache *cache.MockDomainCache) *mutableStateBuilder {
	// the MSB constructor calls a bunch of endpoints on the mocks, so
	// put them in here as a set of fixed expectations so the actual mocking
	// code can just make expectations on the calls on the returned MSB object
	// and not get cluttered with constructor calls
	shardContext.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).Times(2)
	shardContext.EXPECT().GetEventsCache().Return(mockCache)
	shardContext.EXPECT().GetConfig().Return(&config.Config{
		NumberOfShards:                        2,
		IsAdvancedVisConfigExist:              false,
		MaxResponseSize:                       0,
		MutableStateChecksumInvalidateBefore:  dynamicproperties.GetFloatPropertyFn(10),
		MutableStateChecksumVerifyProbability: dynamicproperties.GetIntPropertyFilteredByDomain(0.0),
		MutableStateChecksumGenProbability:    dynamicproperties.GetIntPropertyFilteredByDomain(0.0),
		HostName:                              "test-host",
		EnableReplicationTaskGeneration:       func(string, string) bool { return true },
		MaximumBufferedEventsBatch:            func(...dynamicproperties.FilterOption) int { return 100 },
	}).Times(1)
	shardContext.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSource())
	shardContext.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient())
	shardContext.EXPECT().GetDomainCache().Return(mockDomainCache).Times(1)

	msb := newMutableStateBuilder(shardContext, log.NewNoop(), constants.TestGlobalDomainEntry)
	return msb
}
