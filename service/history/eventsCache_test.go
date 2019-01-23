// Copyright (c) 2017 Uber Technologies, Inc.
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

package history

import (
	"errors"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/uber-go/tally"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"

	"github.com/uber/cadence/.gen/go/shared"
)

type (
	eventsCacheSuite struct {
		suite.Suite
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
		logger bark.Logger

		mockEventsMgr   *mocks.HistoryManager
		mockEventsV2Mgr *mocks.HistoryV2Manager

		cache *eventsCacheImpl
	}
)

func TestEventsCacheSuite(t *testing.T) {
	s := new(eventsCacheSuite)
	suite.Run(t, s)
}

func (s *eventsCacheSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

func (s *eventsCacheSuite) TearDownSuite() {

}

func (s *eventsCacheSuite) SetupTest() {
	s.logger = bark.NewLoggerFromLogrus(log.New())
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
	s.mockEventsMgr = &mocks.HistoryManager{}
	s.mockEventsV2Mgr = &mocks.HistoryV2Manager{}
	s.cache = s.newTestEventsCache()
}

func (s *eventsCacheSuite) TearDownTest() {
	s.mockEventsMgr.AssertExpectations(s.T())
	s.mockEventsV2Mgr.AssertExpectations(s.T())
}

func (s *eventsCacheSuite) newTestEventsCache() *eventsCacheImpl {
	return newEventsCacheWithOptions(16, 32, time.Minute, s.mockEventsMgr, s.mockEventsV2Mgr, false, s.logger,
		metrics.NewClient(tally.NoopScope, metrics.History))
}

func (s *eventsCacheSuite) TestEventsCacheHitSuccess() {
	domainID := "events-cache-hit-success-domain"
	workflowID := "events-cache-hit-success-workflow-id"
	runID := "events-cache-hit-success-run-id"
	eventID := int64(23)
	event := &shared.HistoryEvent{
		EventId:                            &eventID,
		EventType:                          shared.EventTypeActivityTaskStarted.Ptr(),
		ActivityTaskStartedEventAttributes: &shared.ActivityTaskStartedEventAttributes{},
	}

	s.cache.putEvent(domainID, workflowID, runID, eventID, event)
	actualEvent, err := s.cache.getEvent(domainID, workflowID, runID, eventID, eventID, 0, nil)
	s.Nil(err)
	s.Equal(event, actualEvent)
}

func (s *eventsCacheSuite) TestEventsCacheMissSuccess() {
	domainID := "events-cache-miss-success-domain"
	workflowID := "events-cache-miss-success-workflow-id"
	runID := "events-cache-miss-success-run-id"
	event1ID := int64(23)
	event1 := &shared.HistoryEvent{
		EventId:                            &event1ID,
		EventType:                          shared.EventTypeActivityTaskStarted.Ptr(),
		ActivityTaskStartedEventAttributes: &shared.ActivityTaskStartedEventAttributes{},
	}
	event2ID := int64(32)
	event2 := &shared.HistoryEvent{
		EventId:                            &event2ID,
		EventType:                          shared.EventTypeActivityTaskStarted.Ptr(),
		ActivityTaskStartedEventAttributes: &shared.ActivityTaskStartedEventAttributes{},
	}

	s.mockEventsMgr.On("GetWorkflowExecutionHistory", &persistence.GetWorkflowExecutionHistoryRequest{
		DomainID:      domainID,
		Execution:     shared.WorkflowExecution{WorkflowId: common.StringPtr(workflowID), RunId: common.StringPtr(runID)},
		FirstEventID:  event2ID,
		NextEventID:   event2ID + 1,
		PageSize:      1,
		NextPageToken: nil,
	}).Return(&persistence.GetWorkflowExecutionHistoryResponse{
		History:          &shared.History{Events: []*shared.HistoryEvent{event2}},
		NextPageToken:    nil,
		LastFirstEventID: event2ID,
	}, nil)

	s.cache.putEvent(domainID, workflowID, runID, event1ID, event1)
	actualEvent, err := s.cache.getEvent(domainID, workflowID, runID, event2ID, event2ID, 0, nil)
	s.Nil(err)
	s.Equal(event2, actualEvent)
}

func (s *eventsCacheSuite) TestEventsCacheMissMultiEventsBatchSuccess() {
	domainID := "events-cache-miss-success-domain"
	workflowID := "events-cache-miss-success-workflow-id"
	runID := "events-cache-miss-success-run-id"
	event1 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(11),
		EventType:                            shared.EventTypeDecisionTaskCompleted.Ptr(),
		DecisionTaskCompletedEventAttributes: &shared.DecisionTaskCompletedEventAttributes{},
	}
	event2 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(12),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event3 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(13),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event4 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(14),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event5 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(15),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event6 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(16),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}

	s.mockEventsMgr.On("GetWorkflowExecutionHistory", &persistence.GetWorkflowExecutionHistoryRequest{
		DomainID:      domainID,
		Execution:     shared.WorkflowExecution{WorkflowId: common.StringPtr(workflowID), RunId: common.StringPtr(runID)},
		FirstEventID:  event1.GetEventId(),
		NextEventID:   event6.GetEventId() + 1,
		PageSize:      1,
		NextPageToken: nil,
	}).Return(&persistence.GetWorkflowExecutionHistoryResponse{
		History:          &shared.History{Events: []*shared.HistoryEvent{event1, event2, event3, event4, event5, event6}},
		NextPageToken:    nil,
		LastFirstEventID: event1.GetEventId(),
	}, nil)

	s.cache.putEvent(domainID, workflowID, runID, event2.GetEventId(), event2)
	actualEvent, err := s.cache.getEvent(domainID, workflowID, runID, event1.GetEventId(), event6.GetEventId(), 0, nil)
	s.Nil(err)
	s.Equal(event6, actualEvent)
}

func (s *eventsCacheSuite) TestEventsCacheMissMultiEventsBatchV2Success() {
	domainID := "events-cache-miss-multi-events-batch-v2-success-domain"
	workflowID := "events-cache-miss-multi-events-batch-v2-success-workflow-id"
	runID := "events-cache-miss-multi-events-batch-v2-success-run-id"
	event1 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(11),
		EventType:                            shared.EventTypeDecisionTaskCompleted.Ptr(),
		DecisionTaskCompletedEventAttributes: &shared.DecisionTaskCompletedEventAttributes{},
	}
	event2 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(12),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event3 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(13),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event4 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(14),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event5 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(15),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}
	event6 := &shared.HistoryEvent{
		EventId:                              common.Int64Ptr(16),
		EventType:                            shared.EventTypeActivityTaskScheduled.Ptr(),
		ActivityTaskScheduledEventAttributes: &shared.ActivityTaskScheduledEventAttributes{},
	}

	s.mockEventsV2Mgr.On("ReadHistoryBranch", &persistence.ReadHistoryBranchRequest{
		BranchToken:   []byte("store_token"),
		MinEventID:    event1.GetEventId(),
		MaxEventID:    event6.GetEventId() + 1,
		PageSize:      1,
		NextPageToken: nil,
	}).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents:    []*shared.HistoryEvent{event1, event2, event3, event4, event5, event6},
		NextPageToken:    nil,
		LastFirstEventID: event1.GetEventId(),
	}, nil)

	s.cache.putEvent(domainID, workflowID, runID, event2.GetEventId(), event2)
	actualEvent, err := s.cache.getEvent(domainID, workflowID, runID, event1.GetEventId(), event6.GetEventId(),
		persistence.EventStoreVersionV2, []byte("store_token"))
	s.Nil(err)
	s.Equal(event6, actualEvent)
}

func (s *eventsCacheSuite) TestEventsCacheMissFailure() {
	domainID := "events-cache-miss-failure-domain"
	workflowID := "events-cache-miss-failure-workflow-id"
	runID := "events-cache-miss-failure-run-id"

	expectedErr := errors.New("persistence call failed")
	s.mockEventsMgr.On("GetWorkflowExecutionHistory", &persistence.GetWorkflowExecutionHistoryRequest{
		DomainID:      domainID,
		Execution:     shared.WorkflowExecution{WorkflowId: common.StringPtr(workflowID), RunId: common.StringPtr(runID)},
		FirstEventID:  int64(11),
		NextEventID:   int64(15),
		PageSize:      1,
		NextPageToken: nil,
	}).Return(nil, expectedErr)

	actualEvent, err := s.cache.getEvent(domainID, workflowID, runID, int64(11), int64(14), 0, nil)
	s.Nil(actualEvent)
	s.Equal(expectedErr, err)
}

func (s *eventsCacheSuite) TestEventsCacheMissV2Failure() {
	domainID := "events-cache-miss-failure-domain"
	workflowID := "events-cache-miss-failure-workflow-id"
	runID := "events-cache-miss-failure-run-id"

	expectedErr := errors.New("persistence call failed")
	s.mockEventsV2Mgr.On("ReadHistoryBranch", &persistence.ReadHistoryBranchRequest{
		BranchToken:   []byte("store_token"),
		MinEventID:    int64(11),
		MaxEventID:    int64(15),
		PageSize:      1,
		NextPageToken: nil,
	}).Return(nil, expectedErr)

	actualEvent, err := s.cache.getEvent(domainID, workflowID, runID, int64(11), int64(14),
		persistence.EventStoreVersionV2, []byte("store_token"))
	s.Nil(actualEvent)
	s.Equal(expectedErr, err)
}

func (s *eventsCacheSuite) TestEventsCacheDisableSuccess() {
	domainID := "events-cache-disable-success-domain"
	workflowID := "events-cache-disable-success-workflow-id"
	runID := "events-cache-disable-success-run-id"
	event1 := &shared.HistoryEvent{
		EventId:                            common.Int64Ptr(23),
		EventType:                          shared.EventTypeActivityTaskStarted.Ptr(),
		ActivityTaskStartedEventAttributes: &shared.ActivityTaskStartedEventAttributes{},
	}
	event2 := &shared.HistoryEvent{
		EventId:                            common.Int64Ptr(32),
		EventType:                          shared.EventTypeActivityTaskStarted.Ptr(),
		ActivityTaskStartedEventAttributes: &shared.ActivityTaskStartedEventAttributes{},
	}

	s.mockEventsV2Mgr.On("ReadHistoryBranch", &persistence.ReadHistoryBranchRequest{
		BranchToken:   []byte("store_token"),
		MinEventID:    event2.GetEventId(),
		MaxEventID:    event2.GetEventId() + 1,
		PageSize:      1,
		NextPageToken: nil,
	}).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents:    []*shared.HistoryEvent{event2},
		NextPageToken:    nil,
		LastFirstEventID: event2.GetEventId(),
	}, nil)

	s.cache.putEvent(domainID, workflowID, runID, event1.GetEventId(), event1)
	s.cache.putEvent(domainID, workflowID, runID, event2.GetEventId(), event2)
	s.cache.disabled = true
	actualEvent, err := s.cache.getEvent(domainID, workflowID, runID, event2.GetEventId(), event2.GetEventId(),
		persistence.EventStoreVersionV2, []byte("store_token"))
	s.Nil(err)
	s.Equal(event2, actualEvent)
}
