// Copyright (c) 2019 Uber Technologies, Inc.
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

package ndc

import (
	"context"
	"fmt"
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
)

type (
	eventReapplicationSuite struct {
		suite.Suite
		*require.Assertions

		controller *gomock.Controller

		reapplication EventsReapplier
	}
)

func TestEventReapplicationSuite(t *testing.T) {
	s := new(eventReapplicationSuite)
	suite.Run(t, s)
}

func (s *eventReapplicationSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())

	logger := testlogger.New(s.Suite.T())
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{})
	s.reapplication = NewEventsReapplier(
		metricsClient,
		logger,
	)
}

func (s *eventReapplicationSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *eventReapplicationSuite) TestReapplyEvents_AppliedEvent() {
	runID := uuid.New()
	workflowExecution := &persistence.WorkflowExecutionInfo{
		DomainID: uuid.New(),
	}
	event := &types.HistoryEvent{
		ID:        1,
		EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
			Identity:   "test",
			SignalName: "signal",
			Input:      []byte{},
			RequestID:  "b90794a5-e9f4-4f41-9ebf-38aeedefd4ef",
		},
	}
	attr := event.WorkflowExecutionSignaledEventAttributes

	msBuilderCurrent := execution.NewMockMutableState(s.controller)
	msBuilderCurrent.EXPECT().IsWorkflowExecutionRunning().Return(true)
	msBuilderCurrent.EXPECT().GetLastWriteVersion().Return(int64(1), nil).AnyTimes()
	msBuilderCurrent.EXPECT().GetExecutionInfo().Return(workflowExecution).AnyTimes()
	msBuilderCurrent.EXPECT().AddWorkflowExecutionSignaled(
		attr.GetSignalName(),
		attr.GetInput(),
		attr.GetIdentity(),
		"",
	).Return(event, nil).Times(1)
	dedupResource := definition.NewEventReappliedID(runID, event.ID, event.Version)
	msBuilderCurrent.EXPECT().IsResourceDuplicated(dedupResource).Return(false).Times(1)
	msBuilderCurrent.EXPECT().UpdateDuplicatedResource(dedupResource).Times(1)
	events := []*types.HistoryEvent{
		{EventType: types.EventTypeWorkflowExecutionStarted.Ptr()},
		event,
	}
	appliedEvent, err := s.reapplication.ReapplyEvents(context.Background(), msBuilderCurrent, events, runID)
	s.NoError(err)
	s.Equal(1, len(appliedEvent))
}

func (s *eventReapplicationSuite) TestReapplyEvents_Noop() {
	runID := uuid.New()
	event := &types.HistoryEvent{
		ID:        1,
		EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
			Identity:   "test",
			SignalName: "signal",
			Input:      []byte{},
		},
	}

	msBuilderCurrent := execution.NewMockMutableState(s.controller)
	dedupResource := definition.NewEventReappliedID(runID, event.ID, event.Version)
	msBuilderCurrent.EXPECT().IsResourceDuplicated(dedupResource).Return(true).Times(1)
	events := []*types.HistoryEvent{
		{EventType: types.EventTypeWorkflowExecutionStarted.Ptr()},
		event,
	}
	appliedEvent, err := s.reapplication.ReapplyEvents(context.Background(), msBuilderCurrent, events, runID)
	s.NoError(err)
	s.Equal(0, len(appliedEvent))
}

func (s *eventReapplicationSuite) TestReapplyEvents_PartialAppliedEvent() {
	runID := uuid.New()
	workflowExecution := &persistence.WorkflowExecutionInfo{
		DomainID: uuid.New(),
	}
	event1 := &types.HistoryEvent{
		ID:        1,
		EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
			Identity:   "test",
			SignalName: "signal",
			Input:      []byte{},
			RequestID:  "3eb0594e-82dd-4335-8284-855c99d61c74",
		},
	}
	event2 := &types.HistoryEvent{
		ID:        2,
		EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
			Identity:   "test",
			SignalName: "signal",
			Input:      []byte{},
			RequestID:  "2d2cae90-1ae4-4bcb-99a8-30b1cce64e3e",
		},
	}
	attr1 := event1.WorkflowExecutionSignaledEventAttributes

	msBuilderCurrent := execution.NewMockMutableState(s.controller)
	msBuilderCurrent.EXPECT().IsWorkflowExecutionRunning().Return(true)
	msBuilderCurrent.EXPECT().GetLastWriteVersion().Return(int64(1), nil).AnyTimes()
	msBuilderCurrent.EXPECT().GetExecutionInfo().Return(workflowExecution).AnyTimes()
	msBuilderCurrent.EXPECT().AddWorkflowExecutionSignaled(
		attr1.GetSignalName(),
		attr1.GetInput(),
		attr1.GetIdentity(),
		"",
	).Return(event1, nil).Times(1)
	dedupResource1 := definition.NewEventReappliedID(runID, event1.ID, event1.Version)
	msBuilderCurrent.EXPECT().IsResourceDuplicated(dedupResource1).Return(false).Times(1)
	dedupResource2 := definition.NewEventReappliedID(runID, event2.ID, event2.Version)
	msBuilderCurrent.EXPECT().IsResourceDuplicated(dedupResource2).Return(true).Times(1)
	msBuilderCurrent.EXPECT().UpdateDuplicatedResource(dedupResource1).Times(1)
	events := []*types.HistoryEvent{
		{EventType: types.EventTypeWorkflowExecutionStarted.Ptr()},
		event1,
		event2,
	}
	appliedEvent, err := s.reapplication.ReapplyEvents(context.Background(), msBuilderCurrent, events, runID)
	s.NoError(err)
	s.Equal(1, len(appliedEvent))
}

func (s *eventReapplicationSuite) TestReapplyEvents_Error() {
	runID := uuid.New()
	workflowExecution := &persistence.WorkflowExecutionInfo{
		DomainID: uuid.New(),
	}
	event := &types.HistoryEvent{
		ID:        1,
		EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
		WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
			Identity:   "test",
			SignalName: "signal",
			Input:      []byte{},
			RequestID:  "1ece6551-27ac-4a9f-a086-5a780dea10f7",
		},
	}
	attr := event.WorkflowExecutionSignaledEventAttributes

	msBuilderCurrent := execution.NewMockMutableState(s.controller)
	msBuilderCurrent.EXPECT().IsWorkflowExecutionRunning().Return(true)
	msBuilderCurrent.EXPECT().GetLastWriteVersion().Return(int64(1), nil).AnyTimes()
	msBuilderCurrent.EXPECT().GetExecutionInfo().Return(workflowExecution).AnyTimes()
	msBuilderCurrent.EXPECT().AddWorkflowExecutionSignaled(
		attr.GetSignalName(),
		attr.GetInput(),
		attr.GetIdentity(),
		"",
	).Return(nil, fmt.Errorf("test")).Times(1)
	dedupResource := definition.NewEventReappliedID(runID, event.ID, event.Version)
	msBuilderCurrent.EXPECT().IsResourceDuplicated(dedupResource).Return(false).Times(1)
	events := []*types.HistoryEvent{
		{EventType: types.EventTypeWorkflowExecutionStarted.Ptr()},
		event,
	}
	appliedEvent, err := s.reapplication.ReapplyEvents(context.Background(), msBuilderCurrent, events, runID)
	s.Error(err)
	s.Equal(0, len(appliedEvent))
}
