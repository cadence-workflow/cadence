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

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/shard"
)

type (
	historyBuilderSuite struct {
		suite.Suite
		*require.Assertions

		controller      *gomock.Controller
		mockShard       *shard.TestContext
		mockEventsCache *events.MockCache
		mockDomainCache *cache.MockDomainCache

		domainID          string
		domainName        string
		domainEntry       *cache.DomainCacheEntry
		targetDomainID    string
		targetDomainName  string
		targetDomainEntry *cache.DomainCacheEntry
		msBuilder         MutableState
		builder           *HistoryBuilder
		logger            log.Logger
	}
)

func TestHistoryBuilderSuite(t *testing.T) {
	s := new(historyBuilderSuite)
	suite.Run(t, s)
}

func (s *historyBuilderSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
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

	s.logger = log.NewNoop()

	s.domainEntry = constants.TestLocalDomainEntry
	s.domainID = s.domainEntry.GetInfo().ID
	s.domainName = s.domainEntry.GetInfo().Name

	s.targetDomainEntry = constants.TestGlobalTargetDomainEntry
	s.targetDomainID = s.targetDomainEntry.GetInfo().ID
	s.targetDomainName = s.targetDomainEntry.GetInfo().Name

	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockEventsCache = s.mockShard.MockEventsCache
	s.mockDomainCache.EXPECT().GetDomainID(s.domainName).Return(s.domainID, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(s.domainID).Return(s.domainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomain(s.targetDomainName).Return(s.targetDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainID(s.targetDomainName).Return(s.targetDomainID, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(s.targetDomainID).Return(s.targetDomainEntry, nil).AnyTimes()
	s.mockEventsCache.EXPECT().PutEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	s.msBuilder = NewMutableStateBuilder(s.mockShard, s.logger, s.domainEntry)
	s.builder = NewHistoryBuilder(s.msBuilder)
}

func (s *historyBuilderSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *historyBuilderSuite) TestHistoryBuilderDynamicSuccess() {
	id := "dynamic-historybuilder-success-test-workflow-id"
	rid := "dynamic-historybuilder-success-test-run-id"
	wt := "dynamic-historybuilder-success-type"
	tl := "dynamic-historybuilder-success-tasklist"
	identity := "dynamic-historybuilder-success-worker"
	input := []byte("dynamic-historybuilder-success-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	di := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di, 2, tl, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	di0, decisionRunning0 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning0)
	s.Equal(commonconstants.EmptyEventID, di0.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tl, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	di1, decisionRunning1 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning1)
	s.NotNil(di1)
	decisionStartedID1 := di1.StartedID
	s.Equal(int64(3), decisionStartedID1)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionContext := []byte("dynamic-historybuilder-success-context")
	decisionCompletedEvent := s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	s.validateDecisionTaskCompletedEvent(decisionCompletedEvent, 4, 2, 3, decisionContext, identity)
	s.Equal(int64(5), s.getNextEventID())
	_, decisionRunning2 := s.msBuilder.GetDecisionInfo(2)
	s.False(decisionRunning2)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activityTaskList := "dynamic-historybuilder-success-activity-tasklist"
	activityTimeout := int32(60)
	queueTimeout := int32(20)
	hearbeatTimeout := int32(10)

	activity1ID := "activity1"
	activity1Type := "dynamic-historybuilder-success-activity1-type"
	activity1Domain := ""
	activity1Input := []byte("dynamic-historybuilder-success-activity1-input")
	activity1Result := []byte("dynamic-historybuilder-success-activity1-result")
	activity1ScheduledEvent, _, activityDispatchInfo := s.addActivityTaskScheduledEvent(4, activity1ID, activity1Type,
		activity1Domain, activityTaskList, activity1Input, activityTimeout, queueTimeout, hearbeatTimeout, nil, false)
	s.validateActivityTaskScheduledEvent(activity1ScheduledEvent, 5, 4, activity1ID, activity1Type,
		activity1Domain, activityTaskList, activity1Input, activityTimeout, queueTimeout, hearbeatTimeout, activityDispatchInfo, false)
	s.Equal(int64(6), s.getNextEventID())
	ai0, activity1Running0 := s.msBuilder.GetActivityInfo(5)
	s.True(activity1Running0)
	s.Equal(commonconstants.EmptyEventID, ai0.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity2ID := "activity2"
	activity2Type := "dynamic-historybuilder-success-activity2-type"
	activity2Domain := ""
	activity2Input := []byte("dynamic-historybuilder-success-activity2-input")
	activity2Reason := "dynamic-historybuilder-success-activity2-failed"
	activity2Details := []byte("dynamic-historybuilder-success-activity2-callstack")
	activity2ScheduledEvent, _, activityDispatchInfo := s.addActivityTaskScheduledEvent(4, activity2ID, activity2Type,
		activity2Domain, activityTaskList, activity2Input, activityTimeout, queueTimeout, hearbeatTimeout, nil, false)
	s.validateActivityTaskScheduledEvent(activity2ScheduledEvent, 6, 4, activity2ID, activity2Type,
		activity2Domain, activityTaskList, activity2Input, activityTimeout, queueTimeout, hearbeatTimeout, activityDispatchInfo, false)
	s.Equal(int64(7), s.getNextEventID())
	ai2, activity2Running0 := s.msBuilder.GetActivityInfo(6)
	s.True(activity2Running0)
	s.Equal(commonconstants.EmptyEventID, ai2.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity3ID := "activity3"
	activity3Type := "dynamic-historybuilder-success-activity3-type"
	activity3Domain := s.targetDomainName
	activity3Input := []byte("dynamic-historybuilder-success-activity3-input")
	activity3RetryPolicy := &types.RetryPolicy{
		InitialIntervalInSeconds:    1,
		MaximumAttempts:             3,
		MaximumIntervalInSeconds:    1,
		NonRetriableErrorReasons:    []string{"bad-bug"},
		BackoffCoefficient:          1,
		ExpirationIntervalInSeconds: 100,
	}
	activity3ScheduledEvent, _, activityDispatchInfo := s.addActivityTaskScheduledEvent(4, activity3ID, activity3Type,
		activity3Domain, activityTaskList, activity3Input, activityTimeout, queueTimeout, hearbeatTimeout, activity3RetryPolicy, false)
	s.validateActivityTaskScheduledEvent(activity3ScheduledEvent, 7, 4, activity3ID, activity3Type,
		activity3Domain, activityTaskList, activity3Input, activityTimeout, queueTimeout, hearbeatTimeout, activityDispatchInfo, false)
	s.Equal(int64(8), s.getNextEventID())
	ai2, activity3Running0 := s.msBuilder.GetActivityInfo(6)
	s.True(activity3Running0)
	s.Equal(commonconstants.EmptyEventID, ai2.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity4ID := "activity4"
	activity4Type := "dynamic-historybuilder-success-activity4-type"
	activity4Domain := s.targetDomainName
	activity4Input := []byte("dynamic-historybuilder-success-activity4-input")
	activity4Result := []byte("dynamic-historybuilder-success-activity4-result")
	activity4ScheduledEvent, _, activityDispatchInfo := s.addActivityTaskScheduledEvent(4, activity4ID, activity4Type,
		activity4Domain, activityTaskList, activity4Input, activityTimeout, queueTimeout, hearbeatTimeout, nil, true)
	s.validateActivityTaskScheduledEvent(activity4ScheduledEvent, 8, 4, activity4ID, activity4Type,
		activity4Domain, activityTaskList, activity4Input, activityTimeout, queueTimeout, hearbeatTimeout, activityDispatchInfo, true)
	s.Equal(int64(9), s.getNextEventID())
	ai4, activity4Running0 := s.msBuilder.GetActivityInfo(8)
	s.True(activity4Running0)
	s.Equal(commonconstants.EmptyEventID, ai4.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity5ID := "activity5"
	activity5Type := "dynamic-historybuilder-success-activity5-type"
	activity5Domain := s.targetDomainName
	activity5Input := []byte("dynamic-historybuilder-success-activity5-input")
	activity5RetryPolicy := &types.RetryPolicy{
		InitialIntervalInSeconds:    1,
		MaximumAttempts:             3,
		MaximumIntervalInSeconds:    1,
		NonRetriableErrorReasons:    []string{"bad-bug"},
		BackoffCoefficient:          1,
		ExpirationIntervalInSeconds: 100,
	}
	activity5ScheduledEvent, _, activityDispatchInfo := s.addActivityTaskScheduledEvent(4, activity5ID, activity5Type,
		activity5Domain, activityTaskList, activity5Input, activityTimeout, queueTimeout, hearbeatTimeout, activity5RetryPolicy, true)
	s.validateActivityTaskScheduledEvent(activity5ScheduledEvent, 9, 4, activity5ID, activity5Type,
		activity5Domain, activityTaskList, activity5Input, activityTimeout, queueTimeout, hearbeatTimeout, activityDispatchInfo, true)
	s.Equal(int64(10), s.getNextEventID())
	ai5, activity5Running0 := s.msBuilder.GetActivityInfo(9)
	s.True(activity5Running0)
	s.Equal(commonconstants.EmptyEventID, ai5.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activityStartedEvent := s.addActivityTaskStartedEvent(5, activityTaskList, identity)
	s.validateActivityTaskStartedEvent(activityStartedEvent, commonconstants.BufferedEventID, 5, identity,
		0, "", nil)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskStartedEvent(activityStartedEvent, 10, 5, identity,
		0, "", nil)
	s.Equal(int64(11), s.getNextEventID())
	ai3, activity1Running1 := s.msBuilder.GetActivityInfo(5)
	s.True(activity1Running1)
	s.Equal(int64(10), ai3.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activityCompletedEvent := s.addActivityTaskCompletedEvent(5, 10, activity1Result, identity)
	s.validateActivityTaskCompletedEvent(activityCompletedEvent, commonconstants.BufferedEventID, 5, 10, activity1Result,
		identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskCompletedEvent(activityCompletedEvent, 11, 5, 10, activity1Result,
		identity)
	s.Equal(int64(12), s.getNextEventID())
	_, activity1Running2 := s.msBuilder.GetActivityInfo(5)
	s.False(activity1Running2)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	di2 := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di2, 12, tl, taskTimeout)
	s.Equal(int64(13), s.getNextEventID())
	di3, decisionRunning3 := s.msBuilder.GetDecisionInfo(12)
	s.True(decisionRunning3)
	s.Equal(commonconstants.EmptyEventID, di3.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity2StartedEvent := s.addActivityTaskStartedEvent(6, activityTaskList, identity)
	s.validateActivityTaskStartedEvent(activity2StartedEvent, commonconstants.BufferedEventID, 6, identity,
		0, "", nil)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskStartedEvent(activity2StartedEvent, 13, 6, identity,
		0, "", nil)
	s.Equal(int64(14), s.getNextEventID())
	ai4, activity2Running1 := s.msBuilder.GetActivityInfo(6)
	s.True(activity2Running1)
	s.Equal(int64(13), ai4.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity2FailedEvent := s.addActivityTaskFailedEvent(6, 13, activity2Reason, activity2Details,
		identity)
	s.validateActivityTaskFailedEvent(activity2FailedEvent, commonconstants.BufferedEventID, 6, 13, activity2Reason,
		activity2Details, identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskFailedEvent(activity2FailedEvent, 14, 6, 13, activity2Reason,
		activity2Details, identity)
	s.Equal(int64(15), s.getNextEventID())
	_, activity2Running3 := s.msBuilder.GetActivityInfo(6)
	s.False(activity2Running3)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity3StartedEvent := s.addActivityTaskStartedEvent(7, activityTaskList, identity)
	s.validateTransientActivityTaskStartedEvent(activity3StartedEvent, commonconstants.TransientEventID, 7, identity)
	s.Equal(int64(15), s.getNextEventID())
	ai5, activity3Running1 := s.msBuilder.GetActivityInfo(7)
	s.True(activity3Running1)
	s.Equal(commonconstants.TransientEventID, ai5.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity3Reason := "dynamic-historybuilder-success-activity3-failed"
	activity3Details := []byte("dynamic-historybuilder-success-activity3-callstack")
	s.msBuilder.RetryActivity(ai5, activity3Reason, activity3Details)
	ai6, activity3Running2 := s.msBuilder.GetActivityInfo(7)
	s.Equal(activity3Reason, ai6.LastFailureReason)
	s.Equal(activity3Details, ai6.LastFailureDetails)
	s.True(activity3Running2)

	activity3StartedEvent2 := s.addActivityTaskStartedEvent(7, activityTaskList, identity)
	s.validateTransientActivityTaskStartedEvent(activity3StartedEvent2, commonconstants.TransientEventID, 7, identity)
	s.Equal(int64(15), s.getNextEventID())
	ai7, activity3Running3 := s.msBuilder.GetActivityInfo(7)
	s.True(activity3Running3)
	s.Equal(commonconstants.TransientEventID, ai7.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity3Result := []byte("dynamic-historybuilder-success-activity1-result")
	activity3CompletedEvent := s.addActivityTaskCompletedEvent(7, commonconstants.TransientEventID, activity3Result, identity)
	s.validateActivityTaskCompletedEvent(activity3CompletedEvent, commonconstants.BufferedEventID, 7, commonconstants.TransientEventID,
		activity3Result, identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskCompletedEvent(activity3CompletedEvent, 16, 7, 15, activity3Result, identity)
	s.Equal(int64(17), s.getNextEventID())
	ai8, activity3Running4 := s.msBuilder.GetActivityInfo(7)
	s.Nil(ai8)
	s.False(activity3Running4)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity4StartedEvent := s.addActivityTaskStartedEvent(8, activityTaskList, identity)
	s.validateActivityTaskStartedEvent(activity4StartedEvent, commonconstants.BufferedEventID, 8, identity,
		0, "", nil)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskStartedEvent(activity4StartedEvent, 17, 8, identity,
		0, "", nil)
	s.Equal(int64(18), s.getNextEventID())
	ai4, activity4Running1 := s.msBuilder.GetActivityInfo(8)
	s.True(activity4Running1)
	s.Equal(int64(17), ai4.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity4CompletedEvent := s.addActivityTaskCompletedEvent(8, 17, activity4Result, identity)
	s.validateActivityTaskCompletedEvent(activity4CompletedEvent, commonconstants.BufferedEventID, 8, 17, activity4Result,
		identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskCompletedEvent(activity4CompletedEvent, 18, 8, 17, activity4Result,
		identity)
	s.Equal(int64(19), s.getNextEventID())
	_, activity4Running2 := s.msBuilder.GetActivityInfo(8)
	s.False(activity4Running2)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity5StartedEvent := s.addActivityTaskStartedEvent(9, activityTaskList, identity)
	s.validateTransientActivityTaskStartedEvent(activity5StartedEvent, commonconstants.TransientEventID, 9, identity)
	s.Equal(int64(19), s.getNextEventID())
	ai5, activity5Running1 := s.msBuilder.GetActivityInfo(9)
	s.True(activity5Running1)
	s.Equal(commonconstants.TransientEventID, ai5.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity5Reason := "dynamic-historybuilder-success-activity5-failed"
	activity5Details := []byte("dynamic-historybuilder-success-activity5-callstack")
	s.msBuilder.RetryActivity(ai5, activity5Reason, activity5Details)
	ai6, activity5Running2 := s.msBuilder.GetActivityInfo(9)
	s.Equal(activity5Reason, ai6.LastFailureReason)
	s.Equal(activity5Details, ai6.LastFailureDetails)
	s.True(activity5Running2)

	activity5StartedEvent2 := s.addActivityTaskStartedEvent(9, activityTaskList, identity)
	s.validateTransientActivityTaskStartedEvent(activity5StartedEvent2, commonconstants.TransientEventID, 9, identity)
	s.Equal(int64(19), s.getNextEventID())
	ai7, activity5Running3 := s.msBuilder.GetActivityInfo(9)
	s.True(activity5Running3)
	s.Equal(commonconstants.TransientEventID, ai7.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activity5Result := []byte("dynamic-historybuilder-success-activity5-result")
	activity5CompletedEvent := s.addActivityTaskCompletedEvent(9, commonconstants.TransientEventID, activity5Result, identity)
	s.validateActivityTaskCompletedEvent(activity5CompletedEvent, commonconstants.BufferedEventID, 9, commonconstants.TransientEventID,
		activity5Result, identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskCompletedEvent(activity5CompletedEvent, 20, 9, 19, activity5Result, identity)
	s.Equal(int64(21), s.getNextEventID())
	ai8, activity5Running4 := s.msBuilder.GetActivityInfo(9)
	s.Nil(ai8)
	s.False(activity5Running4)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// Verify the last ActivityTaskStartedEvent which should show the error from the first attempt
	historyEvents := s.msBuilder.GetHistoryBuilder().GetHistory().GetEvents()
	s.Len(historyEvents, 20)
	s.validateActivityTaskStartedEvent(historyEvents[14], 15, 7, identity, 1, activity3Reason, activity3Details)

	markerDetails := []byte("dynamic-historybuilder-success-marker-details")
	markerHeaderField1 := []byte("dynamic-historybuilder-success-marker-header1")
	markerHeaderField2 := []byte("dynamic-historybuilder-success-marker-header2")
	markerHeader := map[string][]byte{
		"name1": markerHeaderField1,
		"name2": markerHeaderField2,
	}
	markerEvent := s.addMarkerRecordedEvent(4, "testMarker", markerDetails,
		&markerHeader)
	s.validateMarkerRecordedEvent(markerEvent, 21, 4, "testMarker", markerDetails, &markerHeader)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.Equal(int64(22), s.getNextEventID())
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderWorkflowStartFailures() {
	id := "historybuilder-workflowstart-failures-test-workflow-id"
	rid := "historybuilder-workflowstart-failures-test-run-id"
	wt := "historybuilder-workflowstart-failures-type"
	tl := "historybuilder-workflowstart-failures-tasklist"
	identity := "historybuilder-workflowstart-failures-worker"
	input := []byte("historybuilder-workflowstart-failures-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	di := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di, 2, tl, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	di0, decisionRunning0 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning0)
	s.Equal(commonconstants.EmptyEventID, di0.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	_, err := s.msBuilder.AddWorkflowExecutionStartedEvent(
		we,
		&types.HistoryStartWorkflowExecutionRequest{
			DomainUUID: s.domainID,
			StartRequest: &types.StartWorkflowExecutionRequest{
				WorkflowID:                          we.WorkflowID,
				WorkflowType:                        &types.WorkflowType{Name: wt},
				TaskList:                            &types.TaskList{Name: tl},
				Input:                               input,
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(execTimeout),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(taskTimeout),
				Identity:                            identity,
			},
		})
	s.NotNil(err)

	s.Equal(int64(3), s.getNextEventID(), s.printHistory())
	di1, decisionRunning1 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning1)
	s.Equal(commonconstants.EmptyEventID, di1.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderDecisionScheduledFailures() {
	id := "historybuilder-decisionscheduled-failures-test-workflow-id"
	rid := "historybuilder-decisionscheduled-failures-test-run-id"
	wt := "historybuilder-decisionscheduled-failures-type"
	tl := "historybuilder-decisionscheduled-failures-tasklist"
	identity := "historybuilder-decisionscheduled-failures-worker"
	input := []byte("historybuilder-decisionscheduled-failures-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	di := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di, 2, tl, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	di0, decisionRunning0 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning0)
	s.Equal(commonconstants.EmptyEventID, di0.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	_, err := s.msBuilder.AddDecisionTaskScheduledEvent(false)
	s.NotNil(err)
	s.Equal(int64(3), s.getNextEventID())
	di1, decisionRunning1 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning1)
	s.Equal(commonconstants.EmptyEventID, di1.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderDecisionScheduledTimedout() {
	id := "historybuilder-decisionscheduled-failures-test-workflow-id"
	rid := "historybuilder-decisionscheduled-failures-test-run-id"
	wt := "historybuilder-decisionscheduled-failures-type"
	tl := "historybuilder-decisionscheduled-failures-tasklist"
	identity := "historybuilder-decisionscheduled-failures-worker"
	input := []byte("historybuilder-decisionscheduled-failures-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	di := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di, 2, tl, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	di0, decisionRunning0 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning0)
	s.Equal(commonconstants.EmptyEventID, di0.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionTimeoutEvent := s.addDecisionTaskTimedOutEvent(2)
	s.NotNil(decisionTimeoutEvent)
	s.validateDecisionTaskTimedoutEvent(decisionTimeoutEvent, 3, 2, 0, types.TimeoutTypeScheduleToStart, "", "", commonconstants.EmptyVersion, "", types.DecisionTaskTimedOutCauseTimeout)
	s.Equal(int64(4), s.getNextEventID())

	di2 := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di2, 4, tl, taskTimeout)
	s.Equal(int64(4), s.getNextEventID())
	di1, decisionRunning1 := s.msBuilder.GetDecisionInfo(4)
	s.True(decisionRunning1)
	s.Equal(commonconstants.EmptyEventID, di1.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionTimeoutEvent = s.addDecisionTaskTimedOutEvent(4)
	s.Nil(decisionTimeoutEvent)
	s.Equal(int64(4), s.getNextEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderDecisionStartedTimedout() {
	id := "historybuilder-decisionscheduled-failures-test-workflow-id"
	rid := "historybuilder-decisionscheduled-failures-test-run-id"
	wt := "historybuilder-decisionscheduled-failures-type"
	tl := "historybuilder-decisionscheduled-failures-tasklist"
	identity := "historybuilder-decisionscheduled-failures-worker"
	input := []byte("historybuilder-decisionscheduled-failures-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	di := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di, 2, tl, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	di0, decisionRunning0 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning0)
	s.Equal(commonconstants.EmptyEventID, di0.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tl, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	decisionInfo, decisionRunning := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(int64(3), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionTimeoutEvent := s.addDecisionTaskStartedTimedOutEvent(2, 3)
	s.NotNil(decisionTimeoutEvent)
	s.validateDecisionTaskTimedoutEvent(decisionTimeoutEvent, 4, 2, 3, types.TimeoutTypeStartToClose, "", "", commonconstants.EmptyVersion, "", types.DecisionTaskTimedOutCauseTimeout)
	s.Equal(int64(5), s.getNextEventID())

	di2 := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di2, 5, tl, taskTimeout)
	s.Equal(int64(5), s.getNextEventID())
	di1, decisionRunning1 := s.msBuilder.GetDecisionInfo(5)
	s.True(decisionRunning1)
	s.Equal(int64(1), di1.Attempt)
	s.Equal(commonconstants.EmptyEventID, di1.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent = s.addDecisionTaskStartedEvent(5, tl, identity)
	s.Nil(decisionStartedEvent)
	s.Equal(int64(5), s.getNextEventID())

	decisionTimeoutEvent = s.addDecisionTaskStartedTimedOutEvent(5, 6)
	s.Equal(int64(5), s.getNextEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderDecisionStartedFailures() {
	id := "historybuilder-decisionstarted-failures-test-workflow-id"
	rid := "historybuilder-decisionstarted-failures-test-run-id"
	wt := "historybuilder-decisionstarted-failures-type"
	tl := "historybuilder-decisionstarted-failures-tasklist"
	identity := "historybuilder-decisionstarted-failures-worker"
	input := []byte("historybuilder-decisionstarted-failures-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	_, _, err := s.msBuilder.AddDecisionTaskStartedEvent(2, uuid.New(), &types.PollForDecisionTaskRequest{
		TaskList: &types.TaskList{Name: tl},
		Identity: identity,
	})
	s.NotNil(err)
	s.Equal(int64(2), s.getNextEventID())
	_, decisionRunning1 := s.msBuilder.GetDecisionInfo(2)
	s.False(decisionRunning1)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	di := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di, 2, tl, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	di0, decisionRunning0 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning0)
	s.Equal(commonconstants.EmptyEventID, di0.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	_, _, err = s.msBuilder.AddDecisionTaskStartedEvent(100, uuid.New(), &types.PollForDecisionTaskRequest{
		TaskList: &types.TaskList{Name: tl},
		Identity: identity,
	})
	s.NotNil(err)
	s.Equal(int64(3), s.getNextEventID())
	di2, decisionRunning2 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning2)
	s.Equal(commonconstants.EmptyEventID, di2.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent2 := s.addDecisionTaskStartedEvent(2, tl, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent2, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	di3, decisionRunning3 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning3)
	s.Equal(int64(3), di3.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderFlushBufferedEvents() {
	id := "flush-buffered-events-test-workflow-id"
	rid := "flush-buffered-events-test-run-id"
	wt := "flush-buffered-events-type"
	tl := "flush-buffered-events-tasklist"
	identity := "flush-buffered-events-worker"
	input := []byte("flush-buffered-events-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	// 1 execution started
	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	// 2 decision scheduled
	di := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di, 2, tl, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	di0, decisionRunning0 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning0)
	s.Equal(commonconstants.EmptyEventID, di0.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	// 3 decision started
	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tl, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	di1, decisionRunning1 := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning1)
	s.NotNil(di1)
	decisionStartedID1 := di1.StartedID
	s.Equal(int64(3), decisionStartedID1)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	// 4 decision completed
	decisionContext := []byte("flush-buffered-events-context")
	decisionCompletedEvent := s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	s.validateDecisionTaskCompletedEvent(decisionCompletedEvent, 4, 2, 3, decisionContext, identity)
	s.Equal(int64(5), s.getNextEventID())
	_, decisionRunning2 := s.msBuilder.GetDecisionInfo(2)
	s.False(decisionRunning2)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	activityTaskList := "flush-buffered-events-activity-tasklist"
	activityTimeout := int32(60)
	queueTimeout := int32(20)
	hearbeatTimeout := int32(10)

	// 5 activity1 scheduled
	activity1ID := "activity1"
	activity1Type := "flush-buffered-events-activity1-type"
	activity1Domain := ""
	activity1Input := []byte("flush-buffered-events-activity1-input")
	activity1Result := []byte("flush-buffered-events-activity1-result")
	activity1ScheduledEvent, _, activityDispatchInfo := s.addActivityTaskScheduledEvent(4, activity1ID, activity1Type,
		activity1Domain, activityTaskList, activity1Input, activityTimeout, queueTimeout, hearbeatTimeout, nil, false)
	s.validateActivityTaskScheduledEvent(activity1ScheduledEvent, 5, 4, activity1ID, activity1Type,
		activity1Domain, activityTaskList, activity1Input, activityTimeout, queueTimeout, hearbeatTimeout, activityDispatchInfo, false)
	s.Equal(int64(6), s.getNextEventID())
	ai0, activity1Running0 := s.msBuilder.GetActivityInfo(5)
	s.True(activity1Running0)
	s.Equal(commonconstants.EmptyEventID, ai0.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 6 activity 2 scheduled
	activity2ID := "activity2"
	activity2Type := "flush-buffered-events-activity2-type"
	activity2Domain := s.targetDomainName
	activity2Input := []byte("flush-buffered-events-activity2-input")
	activity2ScheduledEvent, _, activityDispatchInfo := s.addActivityTaskScheduledEvent(4, activity2ID, activity2Type,
		activity2Domain, activityTaskList, activity2Input, activityTimeout, queueTimeout, hearbeatTimeout, nil, false)
	s.validateActivityTaskScheduledEvent(activity2ScheduledEvent, 6, 4, activity2ID, activity2Type,
		activity2Domain, activityTaskList, activity2Input, activityTimeout, queueTimeout, hearbeatTimeout, activityDispatchInfo, false)
	s.Equal(int64(7), s.getNextEventID())
	ai2, activity2Running0 := s.msBuilder.GetActivityInfo(6)
	s.True(activity2Running0)
	s.Equal(commonconstants.EmptyEventID, ai2.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 7 activity1 started
	activityStartedEvent := s.addActivityTaskStartedEvent(5, activityTaskList, identity)
	s.validateActivityTaskStartedEvent(activityStartedEvent, commonconstants.BufferedEventID, 5, identity,
		0, "", nil)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskStartedEvent(activityStartedEvent, 7, 5, identity,
		0, "", nil)
	s.Equal(int64(8), s.getNextEventID())
	ai3, activity1Running1 := s.msBuilder.GetActivityInfo(5)
	s.True(activity1Running1)
	s.Equal(int64(7), ai3.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 8 activity1 completed
	activityCompletedEvent := s.addActivityTaskCompletedEvent(5, 7, activity1Result, identity)
	s.validateActivityTaskCompletedEvent(activityCompletedEvent, commonconstants.BufferedEventID, 5, 7, activity1Result, identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateActivityTaskCompletedEvent(activityCompletedEvent, 8, 5, 7, activity1Result, identity)
	s.Equal(int64(9), s.getNextEventID())
	_, activity1Running2 := s.msBuilder.GetActivityInfo(5)
	s.False(activity1Running2)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 9 decision2 scheduled
	di2 := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(di2, 9, tl, taskTimeout)
	s.Equal(int64(10), s.getNextEventID())
	di3, decisionRunning3 := s.msBuilder.GetDecisionInfo(9)
	s.True(decisionRunning3)
	s.Equal(commonconstants.EmptyEventID, di3.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 10 decision2 started
	decision2StartedEvent := s.addDecisionTaskStartedEvent(9, tl, identity)
	s.validateDecisionTaskStartedEvent(decision2StartedEvent, 10, 9, identity)
	s.Equal(int64(11), s.getNextEventID())
	di2, decision2Running := s.msBuilder.GetDecisionInfo(9)
	s.True(decision2Running)
	s.NotNil(di2)
	decision2StartedID := di2.StartedID
	s.Equal(int64(10), decision2StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 11 (buffered) activity2 started
	activity2StartedEvent := s.addActivityTaskStartedEvent(6, activityTaskList, identity)
	s.validateActivityTaskStartedEvent(activity2StartedEvent, commonconstants.BufferedEventID, 6, identity,
		0, "", nil)
	s.Equal(int64(11), s.getNextEventID())
	ai4, activity2Running := s.msBuilder.GetActivityInfo(6)
	s.True(activity2Running)
	s.Equal(commonconstants.BufferedEventID, ai4.StartedID)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 12 (buffered) activity2 failed
	activity2Reason := "flush-buffered-events-activity2-failed"
	activity2Details := []byte("flush-buffered-events-activity2-callstack")
	activity2FailedEvent := s.addActivityTaskFailedEvent(6, commonconstants.BufferedEventID, activity2Reason, activity2Details, identity)
	s.validateActivityTaskFailedEvent(activity2FailedEvent, commonconstants.BufferedEventID, 6, commonconstants.BufferedEventID, activity2Reason,
		activity2Details, identity)
	s.Equal(int64(11), s.getNextEventID())
	_, activity2Running2 := s.msBuilder.GetActivityInfo(6)
	s.False(activity2Running2)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	// 13 (eventId will be 11) decision completed
	decision2Context := []byte("flush-buffered-events-context")
	decision2CompletedEvent := s.addDecisionTaskCompletedEvent(9, 10, decision2Context, identity)
	s.validateDecisionTaskCompletedEvent(decision2CompletedEvent, 11, 9, 10, decision2Context, identity)
	s.Equal(int64(11), decision2CompletedEvent.ID)
	s.Equal(int64(12), s.getNextEventID())
	_, decision2Running2 := s.msBuilder.GetDecisionInfo(2)
	s.False(decision2Running2)
	s.Equal(int64(10), s.getPreviousDecisionStartedEventID())

	// flush buffered events. 12: Activity2Started, 13: Activity2Failed
	s.NoError(s.msBuilder.FlushBufferedEvents())
	s.Equal(int64(14), s.getNextEventID())
	activity2StartedEvent2 := s.msBuilder.GetHistoryBuilder().history[11]
	s.Equal(int64(12), activity2StartedEvent2.ID)
	s.Equal(types.EventTypeActivityTaskStarted, activity2StartedEvent2.GetEventType())

	activity2FailedEvent2 := s.msBuilder.GetHistoryBuilder().history[12]
	s.Equal(int64(13), activity2FailedEvent2.ID)
	s.Equal(types.EventTypeActivityTaskFailed, activity2FailedEvent2.GetEventType())
	s.Equal(int64(12), activity2FailedEvent2.ActivityTaskFailedEventAttributes.GetStartedEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderWorkflowCancellationRequested() {
	workflowType := "some random workflow type"
	tasklist := "some random tasklist"
	identity := "some random identity"
	input := []byte("some random workflow input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(workflowExecution, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)

	s.Equal(int64(2), s.getNextEventID())

	decisionInfo := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 2, tasklist, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	decisionInfo, decisionRunning := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tasklist, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(int64(3), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionContext := []byte("some random decision context")
	decisionCompletedEvent := s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	s.validateDecisionTaskCompletedEvent(decisionCompletedEvent, 4, 2, 3, decisionContext, identity)
	s.Equal(int64(5), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.False(decisionRunning)
	s.Nil(decisionInfo)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	targetExecution := types.WorkflowExecution{
		WorkflowID: "some random target workflow ID",
		RunID:      "some random target run ID",
	}
	cancellationChildWorkflowOnly := true
	cancellationInitiatedEvent := s.addRequestCancelExternalWorkflowExecutionInitiatedEvent(
		4, s.targetDomainName, targetExecution, cancellationChildWorkflowOnly,
	)
	s.validateRequestCancelExternalWorkflowExecutionInitiatedEvent(
		cancellationInitiatedEvent, 5, 4, s.targetDomainName, targetExecution, cancellationChildWorkflowOnly,
	)
	s.Equal(int64(6), s.getNextEventID())

	cancellationRequestedEvent := s.addExternalWorkflowExecutionCancelRequested(
		5, s.targetDomainName, targetExecution.GetWorkflowID(), targetExecution.GetRunID(),
	)
	s.validateExternalWorkflowExecutionCancelRequested(cancellationRequestedEvent, commonconstants.BufferedEventID, 5, s.targetDomainName, targetExecution)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateExternalWorkflowExecutionCancelRequested(cancellationRequestedEvent, 6, 5, s.targetDomainName, targetExecution)
	s.Equal(int64(7), s.getNextEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderWorkflowCancellationFailed() {
	workflowType := "some random workflow type"
	tasklist := "some random tasklist"
	identity := "some random identity"
	input := []byte("some random workflow input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(workflowExecution, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	decisionInfo := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 2, tasklist, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	decisionInfo, decisionRunning := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tasklist, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(int64(3), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionContext := []byte("some random decision context")
	decisionCompletedEvent := s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	s.validateDecisionTaskCompletedEvent(decisionCompletedEvent, 4, 2, 3, decisionContext, identity)
	s.Equal(int64(5), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.False(decisionRunning)
	s.Nil(decisionInfo)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	targetExecution := types.WorkflowExecution{
		WorkflowID: "some random target workflow ID",
		RunID:      "some random target run ID",
	}
	cancellationChildWorkflowOnly := true
	cancellationFailedCause := types.CancelExternalWorkflowExecutionFailedCause(59)
	cancellationInitiatedEvent := s.addRequestCancelExternalWorkflowExecutionInitiatedEvent(
		4, s.targetDomainName, targetExecution, cancellationChildWorkflowOnly,
	)
	s.validateRequestCancelExternalWorkflowExecutionInitiatedEvent(
		cancellationInitiatedEvent, 5, 4, s.targetDomainName, targetExecution, cancellationChildWorkflowOnly,
	)
	s.Equal(int64(6), s.getNextEventID())

	cancellationRequestedEvent := s.addRequestCancelExternalWorkflowExecutionFailedEvent(
		4, 5, s.targetDomainName, targetExecution.GetWorkflowID(), targetExecution.GetRunID(), cancellationFailedCause,
	)
	s.validateRequestCancelExternalWorkflowExecutionFailedEvent(
		cancellationRequestedEvent, commonconstants.BufferedEventID, 4, 5, s.targetDomainName, targetExecution, cancellationFailedCause,
	)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateRequestCancelExternalWorkflowExecutionFailedEvent(
		cancellationRequestedEvent, 6, 4, 5, s.targetDomainName, targetExecution, cancellationFailedCause,
	)
	s.Equal(int64(7), s.getNextEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilder_DecisionTaskResetTimedOut() {
	workflowType := "some random workflow type"
	tasklist := "some random tasklist"
	identity := "some random identity"
	input := []byte("some random workflow input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(workflowExecution, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	decisionInfo := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 2, tasklist, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	decisionInfo, decisionRunning := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tasklist, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(int64(3), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	baseRunID := uuid.New()
	newRunID := uuid.New()
	forkVersion := int64(10)
	resetRequestID := uuid.New()
	decisionFailedEvent, err := s.msBuilder.AddDecisionTaskFailedEvent(2, 3, types.DecisionTaskFailedCauseResetWorkflow, nil, identity, "", "", baseRunID, newRunID, forkVersion, resetRequestID)
	s.NoError(err)
	s.NotNil(decisionFailedEvent.GetDecisionTaskFailedEventAttributes())
	s.Equal(baseRunID, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetBaseRunID())
	s.Equal(newRunID, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetNewRunID())
	s.Equal(forkVersion, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetForkEventVersion())
	s.Equal(resetRequestID, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetRequestID())
	s.Equal(int64(5), s.getNextEventID())

	decisionInfo = s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 5, tasklist, taskTimeout)
	s.Equal(int64(6), s.getNextEventID())

	decisionStartedEvent = s.addDecisionTaskStartedEvent(5, tasklist, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 6, 5, identity)
	s.Equal(int64(7), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(5)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(5), decisionInfo.ScheduleID)
	s.Equal(int64(6), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionFailedEvent, err = s.msBuilder.AddDecisionTaskFailedEvent(5, 6, types.DecisionTaskFailedCauseResetWorkflow, nil, identity, "", "", baseRunID, newRunID, forkVersion, resetRequestID)
	s.NoError(err)
	s.NotNil(decisionFailedEvent.GetDecisionTaskFailedEventAttributes())
	s.Equal(baseRunID, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetBaseRunID())
	s.Equal(newRunID, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetNewRunID())
	s.Equal(forkVersion, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetForkEventVersion())
	s.Equal(resetRequestID, decisionFailedEvent.GetDecisionTaskFailedEventAttributes().GetRequestID())
	s.Equal(int64(8), s.getNextEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilder_DecisionTaskResetFailed() {
	workflowType := "some random workflow type"
	tasklist := "some random tasklist"
	identity := "some random identity"
	input := []byte("some random workflow input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(workflowExecution, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.Equal(int64(2), s.getNextEventID())

	decisionInfo := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 2, tasklist, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	decisionInfo, decisionRunning := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tasklist, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(int64(3), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	baseRunID := uuid.New()
	newRunID := uuid.New()
	forkVersion := int64(10)
	resetRequestID := uuid.New()
	decisionTimedOutEvent := s.addDecisionTaskResetTimedOutEvent(2, baseRunID, newRunID, forkVersion, resetRequestID)
	s.NotNil(decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes())
	s.Equal(int64(2), decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetScheduledEventID())
	s.Equal(baseRunID, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetBaseRunID())
	s.Equal(newRunID, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetNewRunID())
	s.Equal(forkVersion, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetForkEventVersion())
	s.Equal(resetRequestID, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetRequestID())
	s.Equal(int64(5), s.getNextEventID())

	decisionInfo = s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 5, tasklist, taskTimeout)
	s.Equal(int64(6), s.getNextEventID())

	decisionTimedOutEvent = s.addDecisionTaskResetTimedOutEvent(5, baseRunID, newRunID, forkVersion, resetRequestID)
	s.NotNil(decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes())
	s.Equal(int64(5), decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetScheduledEventID())
	s.Equal(baseRunID, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetBaseRunID())
	s.Equal(newRunID, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetNewRunID())
	s.Equal(forkVersion, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetForkEventVersion())
	s.Equal(resetRequestID, decisionTimedOutEvent.GetDecisionTaskTimedOutEventAttributes().GetRequestID())
	s.Equal(int64(7), s.getNextEventID())
}

func (s *historyBuilderSuite) TestHistoryBuilderWorkflowExternalCancellationRequested() {
	workflowType := "some random workflow type"
	tasklist := "some random tasklist"
	identity := "some random identity"
	input := []byte("some random workflow input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(workflowExecution, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)

	s.Equal(int64(2), s.getNextEventID())

	decisionInfo := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 2, tasklist, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	decisionInfo, decisionRunning := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tasklist, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(int64(3), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionContext := []byte("some random decision context")
	decisionCompletedEvent := s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	s.validateDecisionTaskCompletedEvent(decisionCompletedEvent, 4, 2, 3, decisionContext, identity)
	s.Equal(int64(5), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.False(decisionRunning)
	s.Nil(decisionInfo)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	cause := "cancel workflow"
	req := &types.HistoryRequestCancelWorkflowExecutionRequest{
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			Identity:  "identity",
			RequestID: "b071cbe8-3a95-4223-a8ac-f308a42db383",
		},
	}
	cancellationEvent := s.addWorkflowExecutionCancelRequestedEvent(cause, req)
	s.validateWorkflowExecutionCancelRequestedEvent(cancellationEvent, commonconstants.BufferedEventID, "cancel workflow", "identity", nil, types.WorkflowExecution{}, "b071cbe8-3a95-4223-a8ac-f308a42db383")
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateWorkflowExecutionCancelRequestedEvent(cancellationEvent, 5, "cancel workflow", "identity", nil, types.WorkflowExecution{}, "b071cbe8-3a95-4223-a8ac-f308a42db383")
	s.Equal(int64(6), s.getNextEventID())
	_, exists := s.msBuilder.(*mutableStateBuilder).workflowRequests[persistence.WorkflowRequest{
		RequestID:   "b071cbe8-3a95-4223-a8ac-f308a42db383",
		RequestType: persistence.WorkflowRequestTypeCancel,
		Version:     s.msBuilder.GetCurrentVersion(),
	}]
	s.True(exists)
}

func (s *historyBuilderSuite) TestHistoryBuilderWorkflowExternalSignaled() {
	workflowType := "some random workflow type"
	tasklist := "some random tasklist"
	identity := "some random identity"
	input := []byte("some random workflow input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	workflowStartedEvent := s.addWorkflowExecutionStartedEvent(workflowExecution, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)
	s.validateWorkflowExecutionStartedEvent(workflowStartedEvent, workflowType, tasklist, input, execTimeout, taskTimeout, identity, partitionConfig)

	s.Equal(int64(2), s.getNextEventID())

	decisionInfo := s.addDecisionTaskScheduledEvent()
	s.validateDecisionTaskScheduledEvent(decisionInfo, 2, tasklist, taskTimeout)
	s.Equal(int64(3), s.getNextEventID())
	decisionInfo, decisionRunning := s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionStartedEvent := s.addDecisionTaskStartedEvent(2, tasklist, identity)
	s.validateDecisionTaskStartedEvent(decisionStartedEvent, 3, 2, identity)
	s.Equal(int64(4), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.True(decisionRunning)
	s.NotNil(decisionInfo)
	s.Equal(int64(2), decisionInfo.ScheduleID)
	s.Equal(int64(3), decisionInfo.StartedID)
	s.Equal(commonconstants.EmptyEventID, s.getPreviousDecisionStartedEventID())

	decisionContext := []byte("some random decision context")
	decisionCompletedEvent := s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	s.validateDecisionTaskCompletedEvent(decisionCompletedEvent, 4, 2, 3, decisionContext, identity)
	s.Equal(int64(5), s.getNextEventID())
	decisionInfo, decisionRunning = s.msBuilder.GetDecisionInfo(2)
	s.False(decisionRunning)
	s.Nil(decisionInfo)
	s.Equal(int64(3), s.getPreviousDecisionStartedEventID())

	signalName := "test-signal"
	signalInput := []byte("input")
	signalIdentity := "id"
	requestID := "3b8d0ec2-e1ff-4f61-915b-1ffca831361e"
	signalEvent := s.addWorkflowExecutionSignaledEvent(signalName, signalInput, signalIdentity, requestID)
	s.validateWorkflowExecutionSignaledEvent(signalEvent, commonconstants.BufferedEventID, "test-signal", []byte("input"), "id", "3b8d0ec2-e1ff-4f61-915b-1ffca831361e")
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateWorkflowExecutionSignaledEvent(signalEvent, 5, "test-signal", []byte("input"), "id", "3b8d0ec2-e1ff-4f61-915b-1ffca831361e")
	s.Equal(int64(6), s.getNextEventID())
	_, exists := s.msBuilder.(*mutableStateBuilder).workflowRequests[persistence.WorkflowRequest{
		RequestID:   "3b8d0ec2-e1ff-4f61-915b-1ffca831361e",
		RequestType: persistence.WorkflowRequestTypeSignal,
		Version:     s.msBuilder.GetCurrentVersion(),
	}]
	s.True(exists)
}

func (s *historyBuilderSuite) TestHistoryBuilderActivityTaskTimedOut() {
	id := "historybuilder-workflow-id"
	rid := "historybuilder-test-run-id"
	wt := "historybuilder-type"
	tl := "historybuilder-tasklist"
	identity := "historybuilder-worker"
	input := []byte("historybuilder-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	// 1
	s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	// 2
	s.addDecisionTaskScheduledEvent()
	// 3
	s.addDecisionTaskStartedEvent(2, tl, identity)
	decisionContext := []byte("historybuilder-context")
	// 4
	s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	activityID := "activity1"
	activityType := "activity1-type"
	activityDomain := ""
	activityInput := []byte("activity1-input")
	activityTimeout := int32(60)
	queueTimeout := int32(20)
	hearbeatTimeout := int32(10)
	// 5
	s.addActivityTaskScheduledEvent(4, activityID, activityType, activityDomain, tl, activityInput, activityTimeout, queueTimeout, hearbeatTimeout, nil, false)
	// 6
	s.addActivityTaskStartedEvent(5, tl, identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	timeoutType := types.TimeoutTypeHeartbeat
	lastHeartbeat := []byte("last-heartbeat")
	// 7
	timedOut := s.addActivityTaskTimedOutEvent(5, 6, timeoutType, lastHeartbeat)
	s.validateActivityTaskTimedOutEvent(timedOut, commonconstants.BufferedEventID, 5, 6, timeoutType, lastHeartbeat, "", nil)
	// 8
	s.addDecisionTaskScheduledEvent()
	// 9
	s.addDecisionTaskStartedEvent(8, tl, identity)
	// 10
	s.addDecisionTaskCompletedEvent(8, 9, decisionContext, identity)
	failReason := "activity failed :("
	failDetails := []byte("huge failure")
	// 11
	failed := s.addWorkflowExecutionFailed(10, &types.FailWorkflowExecutionDecisionAttributes{
		Reason:  &failReason,
		Details: failDetails,
	})
	s.validateWorkflowExecutionFailed(failed, 11, 10, failReason, failDetails)
}

func (s *historyBuilderSuite) TestHistoryBuilderTimeoutWorkflow() {
	id := "historybuilder-workflow-id"
	rid := "historybuilder-test-run-id"
	wt := "historybuilder-type"
	tl := "historybuilder-tasklist"
	identity := "historybuilder-worker"
	input := []byte("historybuilder-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	// 1
	s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	// 2
	timedOut := s.addWorkflowExecutionTimedOut(1)
	s.validateWorkflowExecutionTimedOut(timedOut, 2)
}

func (s *historyBuilderSuite) TestHistoryBuilderCompleteWorkflow() {
	id := "historybuilder-workflow-id"
	rid := "historybuilder-test-run-id"
	wt := "historybuilder-type"
	tl := "historybuilder-tasklist"
	identity := "historybuilder-worker"
	input := []byte("historybuilder-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	// 1
	s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	// 2
	s.addDecisionTaskScheduledEvent()
	// 3
	s.addDecisionTaskStartedEvent(2, tl, identity)
	decisionContext := []byte("historybuilder-context")
	// 4
	s.addDecisionTaskCompletedEvent(2, 3, decisionContext, identity)
	decisionResult := []byte("great success")
	// 5
	event := s.addWorkflowExecutionCompleted(4, &types.CompleteWorkflowExecutionDecisionAttributes{
		Result: decisionResult,
	})
	s.validateWorkflowExecutionCompleted(event, 5, 4, decisionResult)
}

func (s *historyBuilderSuite) TestHistoryBuilderTerminateWorkflow() {
	id := "historybuilder-workflow-id"
	rid := "historybuilder-test-run-id"
	wt := "historybuilder-type"
	tl := "historybuilder-tasklist"
	identity := "historybuilder-worker"
	input := []byte("historybuilder-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	// 1
	s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	// 2
	terminated := s.addWorkflowExecutionTerminated(1, "reason", []byte("a really good reason"), identity)
	s.validateWorkflowExecutionTerminated(terminated, 2, "reason", []byte("a really good reason"), identity)
}

func (s *historyBuilderSuite) TestHistoryBuilderCancelWorkflow() {
	id := "historybuilder-workflow-id"
	rid := "historybuilder-test-run-id"
	wt := "historybuilder-type"
	tl := "historybuilder-tasklist"
	identity := "historybuilder-worker"
	input := []byte("historybuilder-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	// 1
	s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	req := &types.HistoryRequestCancelWorkflowExecutionRequest{
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			Identity:  "identity",
			RequestID: "b071cbe8-3a95-4223-a8ac-f308a42db383",
		},
	}
	// 2
	s.addWorkflowExecutionCancelRequestedEvent("because", req)
	// 3
	s.addDecisionTaskScheduledEvent()
	// 4
	s.addDecisionTaskStartedEvent(3, tl, identity)
	// 5
	s.addDecisionTaskCompletedEvent(3, 4, []byte("context"), identity)
	// 6
	event := s.addWorkflowExecutionCanceled(5, &types.CancelWorkflowExecutionDecisionAttributes{
		Details: []byte("details"),
	})
	s.validateWorkflowExecutionCanceled(event, 6, 5, []byte("details"))
}

func (s *historyBuilderSuite) TestHistoryBuilderTimers() {
	id := "historybuilder-workflow-id"
	rid := "historybuilder-test-run-id"
	wt := "historybuilder-type"
	tl := "historybuilder-tasklist"
	identity := "historybuilder-worker"
	input := []byte("historybuilder-input")
	execTimeout := int32(60)
	taskTimeout := int32(10)
	we := types.WorkflowExecution{
		WorkflowID: id,
		RunID:      rid,
	}
	partitionConfig := map[string]string{
		"zone": "dca1",
	}

	// 1
	s.addWorkflowExecutionStartedEvent(we, wt, tl, input, execTimeout, taskTimeout, identity, partitionConfig)
	// 2
	s.addDecisionTaskScheduledEvent()
	// 3
	s.addDecisionTaskStartedEvent(2, tl, identity)
	// 4
	s.addDecisionTaskCompletedEvent(2, 3, []byte("context"), identity)
	// 5
	timer1Started := s.addTimerStartedEvent(4, &types.StartTimerDecisionAttributes{
		TimerID:                   "timer1",
		StartToFireTimeoutSeconds: common.Int64Ptr(int64(30)),
	})
	// 6
	timer2Started := s.addTimerStartedEvent(4, &types.StartTimerDecisionAttributes{
		TimerID:                   "timer2",
		StartToFireTimeoutSeconds: common.Int64Ptr(int64(45)),
	})
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateTimerStartedEvent(timer1Started, 5, 4, 30, "timer1")
	s.validateTimerStartedEvent(timer2Started, 6, 4, 45, "timer2")
	// 7
	timerFired := s.addTimerFiredEvent("timer1")
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateTimerFiredEvent(timerFired, 7, 5, "timer1")
	// 8
	s.addDecisionTaskScheduledEvent()
	// 9
	s.addDecisionTaskStartedEvent(8, tl, identity)
	// 10
	s.addDecisionTaskCompletedEvent(8, 9, []byte("context"), identity)
	// 11
	// Cancel timer2
	cancel := s.addTimerCanceledEvent(10, &types.CancelTimerDecisionAttributes{
		TimerID: "timer2",
	}, identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())

	s.validateTimerCanceledEvent(cancel, 11, 6, 10, "timer2", identity)
	// 12
	// Fail to cancel a timer that doesn't exist
	cancelFailed := s.addTimerCancelFailedEvent(10, &types.CancelTimerDecisionAttributes{
		TimerID: "not-a-real-timer",
	}, identity)
	s.Nil(s.msBuilder.FlushBufferedEvents())
	s.validateTimerCancelFailedEvent(cancelFailed, 12, 10, "not-a-real-timer", identity)
}

func (s *historyBuilderSuite) getNextEventID() int64 {
	return s.msBuilder.GetExecutionInfo().NextEventID
}

func (s *historyBuilderSuite) getPreviousDecisionStartedEventID() int64 {
	return s.msBuilder.GetExecutionInfo().LastProcessedEvent
}

func (s *historyBuilderSuite) addWorkflowExecutionStartedEvent(we types.WorkflowExecution, workflowType,
	taskList string, input []byte, executionStartToCloseTimeout, taskStartToCloseTimeout int32,
	identity string, partitionConfig map[string]string) *types.HistoryEvent {

	request := &types.StartWorkflowExecutionRequest{
		WorkflowID:                          we.WorkflowID,
		WorkflowType:                        &types.WorkflowType{Name: workflowType},
		TaskList:                            &types.TaskList{Name: taskList},
		Input:                               input,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(taskStartToCloseTimeout),
		Identity:                            identity,
	}

	event, err := s.msBuilder.AddWorkflowExecutionStartedEvent(
		we,
		&types.HistoryStartWorkflowExecutionRequest{
			DomainUUID:      s.domainID,
			StartRequest:    request,
			PartitionConfig: partitionConfig,
		},
	)
	s.Nil(err)

	return event
}

func (s *historyBuilderSuite) addWorkflowExecutionTimedOut(firstEvent int64) *types.HistoryEvent {
	event, err := s.msBuilder.AddTimeoutWorkflowEvent(firstEvent)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addWorkflowExecutionFailed(decisionCompletedEventID int64, attributes *types.FailWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event, err := s.msBuilder.AddFailWorkflowEvent(decisionCompletedEventID, attributes)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addWorkflowExecutionCompleted(decisionCompletedEventID int64, attributes *types.CompleteWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event, err := s.msBuilder.AddCompletedWorkflowEvent(decisionCompletedEventID, attributes)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addWorkflowExecutionTerminated(firstEventID int64, reason string, details []byte, identity string) *types.HistoryEvent {
	event, err := s.msBuilder.AddWorkflowExecutionTerminatedEvent(firstEventID, reason, details, identity)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addWorkflowExecutionCanceled(decisionTaskCompletedEventID int64,
	attributes *types.CancelWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event, err := s.msBuilder.AddWorkflowExecutionCanceledEvent(decisionTaskCompletedEventID, attributes)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addDecisionTaskScheduledEvent() *DecisionInfo {
	di, err := s.msBuilder.AddDecisionTaskScheduledEvent(false)
	s.Nil(err)
	return di
}

func (s *historyBuilderSuite) addDecisionTaskStartedEvent(
	scheduleID int64,
	taskList string,
	identity string,
) *types.HistoryEvent {

	event, _, err := s.msBuilder.AddDecisionTaskStartedEvent(scheduleID, uuid.New(), &types.PollForDecisionTaskRequest{
		TaskList: &types.TaskList{Name: taskList},
		Identity: identity,
	})
	s.Nil(err)

	return event
}

func (s *historyBuilderSuite) addDecisionTaskCompletedEvent(
	scheduleID int64,
	startedID int64,
	context []byte,
	identity string,
) *types.HistoryEvent {

	event, err := s.msBuilder.AddDecisionTaskCompletedEvent(scheduleID, startedID, &types.RespondDecisionTaskCompletedRequest{
		ExecutionContext: context,
		Identity:         identity,
	}, commonconstants.DefaultHistoryMaxAutoResetPoints)
	s.Nil(err)

	return event
}

func (s *historyBuilderSuite) addDecisionTaskTimedOutEvent(
	scheduleID int64,
) *types.HistoryEvent {
	event, err := s.msBuilder.AddDecisionTaskScheduleToStartTimeoutEvent(scheduleID)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addDecisionTaskStartedTimedOutEvent(
	scheduleID int64,
	startID int64,
) *types.HistoryEvent {
	event, err := s.msBuilder.AddDecisionTaskTimedOutEvent(scheduleID, startID)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addDecisionTaskResetTimedOutEvent(
	scheduleID int64,
	baseRunID string,
	newRunID string,
	newRunVersion int64,
	resetRequestID string,
) *types.HistoryEvent {
	event, err := s.msBuilder.AddDecisionTaskResetTimeoutEvent(
		scheduleID,
		baseRunID,
		newRunID,
		newRunVersion,
		"",
		resetRequestID,
	)
	s.Nil(err)

	return event
}

func (s *historyBuilderSuite) addTimerStartedEvent(decisionCompletedEventID int64, request *types.StartTimerDecisionAttributes) *types.HistoryEvent {
	event, _, err := s.msBuilder.AddTimerStartedEvent(decisionCompletedEventID, request)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addTimerFiredEvent(timerID string) *types.HistoryEvent {
	event, err := s.msBuilder.AddTimerFiredEvent(timerID)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addTimerCanceledEvent(decisionCompletedEventID int64, attributes *types.CancelTimerDecisionAttributes, identity string) *types.HistoryEvent {
	event, err := s.msBuilder.AddTimerCanceledEvent(decisionCompletedEventID, attributes, identity)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addTimerCancelFailedEvent(decisionCompletedEventID int64, attributes *types.CancelTimerDecisionAttributes, identity string) *types.HistoryEvent {
	event, err := s.msBuilder.AddCancelTimerFailedEvent(decisionCompletedEventID, attributes, identity)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addActivityTaskScheduledEvent(
	decisionCompletedID int64,
	activityID, activityType, domain, taskList string,
	input []byte,
	timeout, queueTimeout, hearbeatTimeout int32,
	retryPolicy *types.RetryPolicy,
	requestLocalDispatch bool,
) (*types.HistoryEvent, *persistence.ActivityInfo, *types.ActivityLocalDispatchInfo) {
	event, ai, activityDispatchInfo, _, _, err := s.msBuilder.AddActivityTaskScheduledEvent(nil, decisionCompletedID,
		&types.ScheduleActivityTaskDecisionAttributes{
			ActivityID:                    activityID,
			ActivityType:                  &types.ActivityType{Name: activityType},
			Domain:                        domain,
			TaskList:                      &types.TaskList{Name: taskList},
			Input:                         input,
			ScheduleToCloseTimeoutSeconds: common.Int32Ptr(timeout),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(queueTimeout),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(hearbeatTimeout),
			StartToCloseTimeoutSeconds:    common.Int32Ptr(1),
			RetryPolicy:                   retryPolicy,
			RequestLocalDispatch:          requestLocalDispatch,
		}, false,
	)
	s.Nil(err)
	if domain == "" {
		s.Equal(s.domainID, ai.DomainID)
	} else {
		s.Equal(s.targetDomainID, ai.DomainID)
	}
	return event, ai, activityDispatchInfo
}

func (s *historyBuilderSuite) addActivityTaskStartedEvent(scheduleID int64, taskList,
	identity string) *types.HistoryEvent {
	ai, _ := s.msBuilder.GetActivityInfo(scheduleID)
	event, err := s.msBuilder.AddActivityTaskStartedEvent(ai, scheduleID, uuid.New(), identity)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addActivityTaskCompletedEvent(scheduleID, startedID int64, result []byte,
	identity string) *types.HistoryEvent {
	event, err := s.msBuilder.AddActivityTaskCompletedEvent(scheduleID, startedID, &types.RespondActivityTaskCompletedRequest{
		Result:   result,
		Identity: identity,
	})
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addActivityTaskFailedEvent(scheduleID, startedID int64, reason string, details []byte,
	identity string) *types.HistoryEvent {
	event, err := s.msBuilder.AddActivityTaskFailedEvent(scheduleID, startedID, &types.RespondActivityTaskFailedRequest{
		Reason:   common.StringPtr(reason),
		Details:  details,
		Identity: identity,
	})
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addActivityTaskTimedOutEvent(scheduleEventID,
	startedEventID int64,
	timeoutType types.TimeoutType,
	lastHeartBeatDetails []byte) *types.HistoryEvent {
	event, err := s.msBuilder.AddActivityTaskTimedOutEvent(scheduleEventID, startedEventID, timeoutType, lastHeartBeatDetails)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addMarkerRecordedEvent(decisionCompletedEventID int64, markerName string, details []byte, header *map[string][]byte) *types.HistoryEvent {
	fields := make(map[string][]byte)
	if header != nil {
		for name, value := range *header {
			fields[name] = value
		}
	}
	event, err := s.msBuilder.AddRecordMarkerEvent(decisionCompletedEventID, &types.RecordMarkerDecisionAttributes{
		MarkerName: markerName,
		Details:    details,
		Header: &types.Header{
			Fields: fields,
		},
	})
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addRequestCancelExternalWorkflowExecutionInitiatedEvent(
	decisionCompletedEventID int64, targetDomain string, targetExecution types.WorkflowExecution,
	childWorkflowOnly bool) *types.HistoryEvent {
	event, _, err := s.msBuilder.AddRequestCancelExternalWorkflowExecutionInitiatedEvent(
		decisionCompletedEventID,
		uuid.New(),
		&types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
			Domain:            targetDomain,
			WorkflowID:        targetExecution.WorkflowID,
			RunID:             targetExecution.RunID,
			ChildWorkflowOnly: childWorkflowOnly,
		},
	)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addExternalWorkflowExecutionCancelRequested(
	initiatedID int64, domain, workflowID, runID string) *types.HistoryEvent {

	event, err := s.msBuilder.AddExternalWorkflowExecutionCancelRequested(
		initiatedID, domain, workflowID, runID,
	)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addRequestCancelExternalWorkflowExecutionFailedEvent(
	decisionTaskCompletedEventID, initiatedID int64,
	domain, workflowID, runID string, cause types.CancelExternalWorkflowExecutionFailedCause) *types.HistoryEvent {

	event, err := s.msBuilder.AddRequestCancelExternalWorkflowExecutionFailedEvent(
		decisionTaskCompletedEventID, initiatedID, domain, workflowID, runID, cause,
	)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addWorkflowExecutionCancelRequestedEvent(
	cause string,
	request *types.HistoryRequestCancelWorkflowExecutionRequest,
) *types.HistoryEvent {
	event, err := s.msBuilder.AddWorkflowExecutionCancelRequestedEvent(cause, request)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) addWorkflowExecutionSignaledEvent(
	signalName string,
	input []byte,
	identity string,
	requestID string,
) *types.HistoryEvent {
	event, err := s.msBuilder.AddWorkflowExecutionSignaled(signalName, input, identity, requestID)
	s.Nil(err)
	return event
}

func (s *historyBuilderSuite) validateWorkflowExecutionStartedEvent(event *types.HistoryEvent, workflowType,
	taskList string, input []byte, executionStartToCloseTimeout, taskStartToCloseTimeout int32, identity string, partitionConfig map[string]string) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionStarted, *event.EventType)
	s.Equal(commonconstants.FirstEventID, event.ID)
	attributes := event.WorkflowExecutionStartedEventAttributes
	s.NotNil(attributes)
	s.Equal(workflowType, attributes.WorkflowType.Name)
	s.Equal(taskList, attributes.TaskList.Name)
	s.Equal(input, attributes.Input)
	s.Equal(executionStartToCloseTimeout, *attributes.ExecutionStartToCloseTimeoutSeconds)
	s.Equal(taskStartToCloseTimeout, *attributes.TaskStartToCloseTimeoutSeconds)
	s.Equal(identity, attributes.Identity)
	s.Equal(partitionConfig, attributes.PartitionConfig)
	if attributes.CronSchedule == "" {
		s.Nil(attributes.FirstScheduleTime)
	} else {
		s.NotNil(attributes.FirstScheduleTime)
	}
}

func (s *historyBuilderSuite) validateWorkflowExecutionFailed(event *types.HistoryEvent, eventID, decisionCompletedID int64, failReason string, failDetail []byte) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionFailed, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.WorkflowExecutionFailedEventAttributes
	s.NotNil(attributes)
	s.Equal(decisionCompletedID, attributes.DecisionTaskCompletedEventID)
	s.Equal(failReason, *attributes.Reason)
	s.Equal(failDetail, attributes.Details)
}

func (s *historyBuilderSuite) validateWorkflowExecutionTimedOut(event *types.HistoryEvent, eventID int64) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionTimedOut, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.WorkflowExecutionTimedOutEventAttributes
	s.NotNil(attributes)
	s.Equal(types.TimeoutTypeStartToClose, *attributes.TimeoutType)
}

func (s *historyBuilderSuite) validateWorkflowExecutionCompleted(event *types.HistoryEvent, eventID, decisionCompletedID int64, result []byte) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionCompleted, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.WorkflowExecutionCompletedEventAttributes
	s.NotNil(attributes)
	s.Equal(decisionCompletedID, attributes.DecisionTaskCompletedEventID)
	s.Equal(result, attributes.Result)
}

func (s *historyBuilderSuite) validateWorkflowExecutionTerminated(event *types.HistoryEvent, eventID int64, reason string, details []byte, identity string) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionTerminated, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.WorkflowExecutionTerminatedEventAttributes
	s.NotNil(attributes)
	s.Equal(reason, attributes.Reason)
	s.Equal(details, attributes.Details)
	s.Equal(identity, attributes.Identity)
}

func (s *historyBuilderSuite) validateWorkflowExecutionCanceled(event *types.HistoryEvent, eventID, decisionTaskCompletedID int64, details []byte) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionCanceled, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.WorkflowExecutionCanceledEventAttributes
	s.NotNil(attributes)
	s.Equal(decisionTaskCompletedID, attributes.DecisionTaskCompletedEventID)
	s.Equal(details, attributes.Details)
}

func (s *historyBuilderSuite) validateDecisionTaskScheduledEvent(di *DecisionInfo, eventID int64,
	taskList string, timeout int32) {
	s.NotNil(di)
	s.Equal(eventID, di.ScheduleID)
	s.Equal(taskList, di.TaskList)
}

func (s *historyBuilderSuite) validateDecisionTaskStartedEvent(event *types.HistoryEvent, eventID, scheduleID int64,
	identity string) {
	s.NotNil(event)
	s.Equal(types.EventTypeDecisionTaskStarted, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.DecisionTaskStartedEventAttributes
	s.NotNil(attributes)
	s.Equal(scheduleID, attributes.ScheduledEventID)
	s.Equal(identity, attributes.Identity)
}

func (s *historyBuilderSuite) validateDecisionTaskCompletedEvent(event *types.HistoryEvent, eventID,
	scheduleID, startedID int64, context []byte, identity string) {
	s.NotNil(event)
	s.Equal(types.EventTypeDecisionTaskCompleted, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.DecisionTaskCompletedEventAttributes
	s.NotNil(attributes)
	s.Equal(scheduleID, attributes.ScheduledEventID)
	s.Equal(startedID, attributes.StartedEventID)
	s.Equal(context, attributes.ExecutionContext)
	s.Equal(identity, attributes.Identity)
}

func (s *historyBuilderSuite) validateDecisionTaskTimedoutEvent(
	event *types.HistoryEvent,
	eventID int64,
	scheduleEventID int64,
	startedEventID int64,
	timeoutType types.TimeoutType,
	baseRunID string,
	newRunID string,
	forkEventVersion int64,
	reason string,
	cause types.DecisionTaskTimedOutCause,
) {
	s.NotNil(event)
	s.Equal(types.EventTypeDecisionTaskTimedOut, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.DecisionTaskTimedOutEventAttributes
	s.NotNil(attributes)
	s.Equal(scheduleEventID, attributes.ScheduledEventID)
	s.Equal(startedEventID, attributes.StartedEventID)
	s.Equal(timeoutType, *attributes.TimeoutType)
	s.Equal(baseRunID, attributes.BaseRunID)
	s.Equal(newRunID, attributes.NewRunID)
	s.Equal(forkEventVersion, attributes.ForkEventVersion)
	s.Equal(reason, attributes.Reason)
	s.Equal(cause, *attributes.Cause)
}

func (s *historyBuilderSuite) validateActivityTaskScheduledEvent(
	event *types.HistoryEvent,
	eventID, decisionID int64,
	activityID, activityType, domain, taskList string,
	input []byte, timeout,
	queueTimeout, hearbeatTimeout int32,
	activityDispatchInfo *types.ActivityLocalDispatchInfo,
	requestLocalDispatch bool,
) {
	s.NotNil(event)
	s.Equal(types.EventTypeActivityTaskScheduled, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.ActivityTaskScheduledEventAttributes
	s.NotNil(attributes)
	s.Equal(decisionID, attributes.DecisionTaskCompletedEventID)
	s.Equal(activityID, attributes.ActivityID)
	s.Equal(activityType, attributes.ActivityType.Name)
	s.Equal(taskList, attributes.TaskList.Name)
	s.Equal(input, attributes.Input)
	s.Equal(timeout, *attributes.ScheduleToCloseTimeoutSeconds)
	s.Equal(queueTimeout, *attributes.ScheduleToStartTimeoutSeconds)
	s.Equal(hearbeatTimeout, *attributes.HeartbeatTimeoutSeconds)
	if domain != "" {
		s.Equal(domain, *attributes.Domain)
	} else {
		s.Nil(attributes.Domain)
	}
	if requestLocalDispatch {
		s.NotNil(activityDispatchInfo)
	} else {
		s.Nil(activityDispatchInfo)
	}
}

func (s *historyBuilderSuite) validateActivityTaskStartedEvent(event *types.HistoryEvent, eventID, scheduleID int64,
	identity string, attempt int64, lastFailureReason string, lastFailureDetails []byte) {
	s.NotNil(event)
	s.Equal(types.EventTypeActivityTaskStarted, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.ActivityTaskStartedEventAttributes
	s.NotNil(attributes)
	s.Equal(scheduleID, attributes.ScheduledEventID)
	s.Equal(identity, attributes.Identity)
	s.Equal(lastFailureReason, *attributes.LastFailureReason)
	s.Equal(lastFailureDetails, attributes.LastFailureDetails)
}

func (s *historyBuilderSuite) validateTransientActivityTaskStartedEvent(event *types.HistoryEvent, eventID, scheduleID int64,
	identity string) {
	s.Nil(event)
	ai, ok := s.msBuilder.GetPendingActivityInfos()[scheduleID]
	s.True(ok)
	s.NotNil(ai)
	s.Equal(scheduleID, ai.ScheduleID)
	s.Equal(identity, ai.StartedIdentity)
}

func (s *historyBuilderSuite) validateActivityTaskCompletedEvent(event *types.HistoryEvent, eventID,
	scheduleID, startedID int64, result []byte, identity string) {
	s.NotNil(event)
	s.Equal(types.EventTypeActivityTaskCompleted, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.ActivityTaskCompletedEventAttributes
	s.NotNil(attributes)
	s.Equal(scheduleID, attributes.ScheduledEventID)
	s.Equal(startedID, attributes.StartedEventID)
	s.Equal(result, attributes.Result)
	s.Equal(identity, attributes.Identity)
}

func (s *historyBuilderSuite) validateActivityTaskFailedEvent(event *types.HistoryEvent, eventID,
	scheduleID, startedID int64, reason string, details []byte, identity string) {
	s.NotNil(event)
	s.Equal(types.EventTypeActivityTaskFailed, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.ActivityTaskFailedEventAttributes
	s.NotNil(attributes)
	s.Equal(scheduleID, attributes.ScheduledEventID)
	s.Equal(startedID, attributes.StartedEventID)
	s.Equal(reason, *attributes.Reason)
	s.Equal(details, attributes.Details)
	s.Equal(identity, attributes.Identity)
}

func (s *historyBuilderSuite) validateActivityTaskTimedOutEvent(event *types.HistoryEvent, eventID, scheduleID, startedID int64, timeoutType types.TimeoutType,
	lastHeartBeatDetails []byte, lastFailureReason string, lastFailureDetails []byte) {
	s.NotNil(event)
	s.Equal(types.EventTypeActivityTaskTimedOut, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.ActivityTaskTimedOutEventAttributes
	s.NotNil(attributes)
	s.Equal(scheduleID, attributes.ScheduledEventID)
	s.Equal(startedID, attributes.StartedEventID)
	s.Equal(timeoutType, *attributes.TimeoutType)
	s.Equal(lastHeartBeatDetails, attributes.Details)
	s.Equal(lastFailureReason, *attributes.LastFailureReason)
	s.Equal(lastFailureDetails, attributes.LastFailureDetails)
}

func (s *historyBuilderSuite) validateMarkerRecordedEvent(
	event *types.HistoryEvent, eventID, decisionTaskCompletedEventID int64,
	markerName string, details []byte, header *map[string][]byte) {
	s.NotNil(event)
	s.Equal(types.EventTypeMarkerRecorded, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.MarkerRecordedEventAttributes
	s.NotNil(attributes)
	s.Equal(decisionTaskCompletedEventID, attributes.DecisionTaskCompletedEventID)
	s.Equal(markerName, attributes.GetMarkerName())
	s.Equal(details, attributes.Details)
	if header != nil {
		for name, value := range attributes.Header.Fields {
			s.Equal((*header)[name], value)
		}
	} else {
		s.Nil(attributes.Header)
	}
}

func (s *historyBuilderSuite) validateRequestCancelExternalWorkflowExecutionInitiatedEvent(
	event *types.HistoryEvent, eventID, decisionTaskCompletedEventID int64,
	domain string, execution types.WorkflowExecution, childWorkflowOnly bool) {
	s.NotNil(event)
	s.Equal(types.EventTypeRequestCancelExternalWorkflowExecutionInitiated, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes
	s.NotNil(attributes)
	s.Equal(decisionTaskCompletedEventID, attributes.DecisionTaskCompletedEventID)
	s.Equal(domain, attributes.GetDomain())
	s.Equal(execution.GetWorkflowID(), attributes.WorkflowExecution.GetWorkflowID())
	s.Equal(execution.GetRunID(), attributes.WorkflowExecution.GetRunID())
	s.Equal(childWorkflowOnly, attributes.ChildWorkflowOnly)
}

func (s *historyBuilderSuite) validateExternalWorkflowExecutionCancelRequested(
	event *types.HistoryEvent, eventID, initiatedEventID int64,
	domain string, execution types.WorkflowExecution) {
	s.NotNil(event)
	s.Equal(types.EventTypeExternalWorkflowExecutionCancelRequested, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.ExternalWorkflowExecutionCancelRequestedEventAttributes
	s.NotNil(attributes)
	s.Equal(initiatedEventID, attributes.GetInitiatedEventID())
	s.Equal(domain, attributes.GetDomain())
	s.Equal(execution.GetWorkflowID(), attributes.WorkflowExecution.GetWorkflowID())
	s.Equal(execution.GetRunID(), attributes.WorkflowExecution.GetRunID())
}

func (s *historyBuilderSuite) validateRequestCancelExternalWorkflowExecutionFailedEvent(
	event *types.HistoryEvent, eventID, decisionTaskCompletedEventID, initiatedEventID int64,
	domain string, execution types.WorkflowExecution, cause types.CancelExternalWorkflowExecutionFailedCause) {
	s.NotNil(event)
	s.Equal(types.EventTypeRequestCancelExternalWorkflowExecutionFailed, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.RequestCancelExternalWorkflowExecutionFailedEventAttributes
	s.NotNil(attributes)
	s.Equal(decisionTaskCompletedEventID, attributes.GetDecisionTaskCompletedEventID())
	s.Equal(initiatedEventID, attributes.GetInitiatedEventID())
	s.Equal(domain, attributes.GetDomain())
	s.Equal(execution.GetWorkflowID(), attributes.WorkflowExecution.GetWorkflowID())
	s.Equal(execution.GetRunID(), attributes.WorkflowExecution.GetRunID())
	s.Equal(cause, *attributes.Cause)
}

func (s *historyBuilderSuite) validateWorkflowExecutionCancelRequestedEvent(
	event *types.HistoryEvent,
	eventID int64,
	cause string,
	identity string,
	externalInitiatedEventID *int64,
	execution types.WorkflowExecution,
	requestID string,
) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionCancelRequested, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.WorkflowExecutionCancelRequestedEventAttributes
	s.NotNil(attributes)
	s.Equal(cause, attributes.Cause)
	s.Equal(identity, attributes.Identity)
	s.Equal(externalInitiatedEventID, attributes.ExternalInitiatedEventID)
	s.Equal(execution.GetWorkflowID(), attributes.ExternalWorkflowExecution.GetWorkflowID())
	s.Equal(execution.GetRunID(), attributes.ExternalWorkflowExecution.GetRunID())
	s.Equal(requestID, attributes.RequestID)
}

func (s *historyBuilderSuite) validateWorkflowExecutionSignaledEvent(
	event *types.HistoryEvent,
	eventID int64,
	signalName string,
	input []byte,
	identity string,
	requestID string,
) {
	s.NotNil(event)
	s.Equal(types.EventTypeWorkflowExecutionSignaled, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.WorkflowExecutionSignaledEventAttributes
	s.Equal(signalName, attributes.SignalName)
	s.Equal(input, attributes.Input)
	s.Equal(identity, attributes.Identity)
	s.Equal(requestID, attributes.RequestID)
}

func (s *historyBuilderSuite) validateTimerStartedEvent(event *types.HistoryEvent, eventID, decisionTaskCompleted, startToFireTimeoutSeconds int64, timerID string) {
	s.NotNil(event)
	s.Equal(types.EventTypeTimerStarted, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.TimerStartedEventAttributes
	s.NotNil(attributes)
	s.Equal(timerID, attributes.TimerID)
	s.Equal(decisionTaskCompleted, attributes.DecisionTaskCompletedEventID)
	s.Equal(startToFireTimeoutSeconds, *attributes.StartToFireTimeoutSeconds)
}

func (s *historyBuilderSuite) validateTimerFiredEvent(event *types.HistoryEvent, eventID, startedEventID int64, timerID string) {
	s.NotNil(event)
	s.Equal(types.EventTypeTimerFired, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.TimerFiredEventAttributes
	s.NotNil(attributes)
	s.Equal(timerID, attributes.TimerID)
	s.Equal(startedEventID, attributes.StartedEventID)
}

func (s *historyBuilderSuite) validateTimerCanceledEvent(event *types.HistoryEvent, eventID, startedEventID, decisionTaskCompletedID int64, timerID, identity string) {
	s.NotNil(event)
	s.Equal(types.EventTypeTimerCanceled, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.TimerCanceledEventAttributes
	s.NotNil(attributes)
	s.Equal(timerID, attributes.TimerID)
	s.Equal(startedEventID, attributes.StartedEventID)
	s.Equal(decisionTaskCompletedID, attributes.DecisionTaskCompletedEventID)
	s.Equal(identity, attributes.Identity)
}

func (s *historyBuilderSuite) validateTimerCancelFailedEvent(event *types.HistoryEvent, eventID, decisionTaskCompletedID int64, timerID, identity string) {
	s.NotNil(event)
	s.Equal(types.EventTypeCancelTimerFailed, *event.EventType)
	s.Equal(eventID, event.ID)
	attributes := event.CancelTimerFailedEventAttributes
	s.NotNil(attributes)
	s.Equal(timerID, attributes.TimerID)
	s.Equal(decisionTaskCompletedID, attributes.DecisionTaskCompletedEventID)
	s.Equal(identity, attributes.Identity)
	s.Equal(timerCancellationMsgTimerIDUnknown, attributes.Cause)
}

func (s *historyBuilderSuite) printHistory() string {
	return thrift.FromHistory(s.builder.GetHistory()).String()
}
