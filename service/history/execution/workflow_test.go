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
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	workflowSuite struct {
		suite.Suite
		*require.Assertions

		controller       *gomock.Controller
		mockContext      *MockContext
		mockMutableState *MockMutableState

		domainID   string
		domainName string
		workflowID string
		runID      string
	}
)

func TestWorkflowSuite(t *testing.T) {
	s := new(workflowSuite)
	suite.Run(t, s)
}

func (s *workflowSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockContext = NewMockContext(s.controller)
	s.mockMutableState = NewMockMutableState(s.controller)
	s.domainID = uuid.New()
	s.domainName = "domain-name"
	s.workflowID = "some random workflow ID"
	s.runID = uuid.New()
}

func (s *workflowSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *workflowSuite) TestGetMethods() {
	lastEventTaskID := int64(144)
	lastEventVersion := int64(12)
	startTimestamp := time.Now()
	activeClusterSelectionPolicy := &types.ActiveClusterSelectionPolicy{
		ClusterAttribute: &types.ClusterAttribute{Scope: "region", Name: "region-1"},
	}
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(lastEventVersion, nil).AnyTimes()
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:                     s.domainID,
		WorkflowID:                   s.workflowID,
		RunID:                        s.runID,
		LastEventTaskID:              lastEventTaskID,
		StartTimestamp:               startTimestamp,
		ActiveClusterSelectionPolicy: activeClusterSelectionPolicy,
	}).AnyTimes()

	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)

	s.Equal(s.mockContext, nDCWorkflow.GetContext())
	s.Equal(s.mockMutableState, nDCWorkflow.GetMutableState())
	// NOTE golang does not seem to let people compare functions, easily
	//  link: https://github.com/stretchr/testify/issues/182
	// this is a hack to compare 2 functions, being the same
	expectedReleaseFn := runtime.FuncForPC(reflect.ValueOf(NoopReleaseFn).Pointer()).Name()
	actualReleaseFn := runtime.FuncForPC(reflect.ValueOf(nDCWorkflow.GetReleaseFn()).Pointer()).Name()
	s.Equal(expectedReleaseFn, actualReleaseFn)
	vectorClock, err := nDCWorkflow.GetVectorClock()
	s.NoError(err)
	expectedVectorClock := WorkflowVectorClock{
		ActiveClusterSelectionPolicy: activeClusterSelectionPolicy,
		LastWriteVersion:             lastEventVersion,
		LastEventTaskID:              lastEventTaskID,
		StartTimestamp:               startTimestamp,
		RunID:                        s.runID,
	}
	s.Equal(expectedVectorClock, vectorClock)
}

func (s *workflowSuite) TestSuppressWorkflowBy_Error() {
	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)

	incomingMockContext := NewMockContext(s.controller)
	incomingMockMutableState := NewMockMutableState(s.controller)
	incomingNDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		incomingMockContext,
		incomingMockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)

	// cannot suppress by older workflow
	lastEventTaskID := int64(144)
	lastEventVersion := int64(12)
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(lastEventVersion, nil).AnyTimes()
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:        s.domainID,
		WorkflowID:      s.workflowID,
		RunID:           s.runID,
		LastEventTaskID: lastEventTaskID,
	}).AnyTimes()

	incomingRunID := uuid.New()
	incomingLastEventTaskID := int64(144)
	incomingLastEventVersion := lastEventVersion - 1
	incomingMockMutableState.EXPECT().GetLastWriteVersion().Return(incomingLastEventVersion, nil).AnyTimes()
	incomingMockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:        s.domainID,
		WorkflowID:      s.workflowID,
		RunID:           incomingRunID,
		LastEventTaskID: incomingLastEventTaskID,
	}).AnyTimes()

	_, err := nDCWorkflow.SuppressBy(incomingNDCWorkflow)
	s.Error(err)
}

func (s *workflowSuite) TestSuppressWorkflowBy_Terminate() {
	lastEventID := int64(2)
	lastEventTaskID := int64(144)
	lastEventVersion := cluster.TestCurrentClusterInitialFailoverVersion
	s.mockMutableState.EXPECT().GetNextEventID().Return(lastEventID + 1).AnyTimes()
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(lastEventVersion, nil).AnyTimes()
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:        s.domainID,
		WorkflowID:      s.workflowID,
		RunID:           s.runID,
		LastEventTaskID: lastEventTaskID,
	}).AnyTimes()
	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)

	incomingRunID := uuid.New()
	incomingLastEventTaskID := int64(144)
	incomingLastEventVersion := lastEventVersion + 1
	incomingMockContext := NewMockContext(s.controller)
	incomingMockMutableState := NewMockMutableState(s.controller)
	incomingNDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		incomingMockContext,
		incomingMockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	incomingMockMutableState.EXPECT().GetLastWriteVersion().Return(incomingLastEventVersion, nil).AnyTimes()
	incomingMockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:        s.domainID,
		WorkflowID:      s.workflowID,
		RunID:           incomingRunID,
		LastEventTaskID: incomingLastEventTaskID,
	}).AnyTimes()

	s.mockMutableState.EXPECT().UpdateCurrentVersion(lastEventVersion, true).Return(nil).AnyTimes()
	inFlightDecision := &DecisionInfo{
		Version:    1234,
		ScheduleID: 5678,
		StartedID:  9012,
	}
	s.mockMutableState.EXPECT().GetInFlightDecision().Return(inFlightDecision, true).Times(1)
	s.mockMutableState.EXPECT().AddDecisionTaskFailedEvent(
		inFlightDecision.ScheduleID,
		inFlightDecision.StartedID,
		types.DecisionTaskFailedCauseFailoverCloseDecision,
		[]byte(nil),
		IdentityHistoryService,
		"",
		"",
		"",
		"",
		int64(0),
		"",
	).Return(&types.HistoryEvent{}, nil).Times(1)
	s.mockMutableState.EXPECT().FlushBufferedEvents().Return(nil).Times(1)

	s.mockMutableState.EXPECT().AddWorkflowExecutionTerminatedEvent(
		lastEventID+1, WorkflowTerminationReason, gomock.Any(), WorkflowTerminationIdentity,
	).Return(&types.HistoryEvent{}, nil).Times(1)

	// if workflow is in zombie or finished state, keep as is
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(false).Times(1)
	policy, err := nDCWorkflow.SuppressBy(incomingNDCWorkflow)
	s.NoError(err)
	s.Equal(TransactionPolicyPassive, policy)

	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).Times(1)
	policy, err = nDCWorkflow.SuppressBy(incomingNDCWorkflow)
	s.NoError(err)
	s.Equal(TransactionPolicyActive, policy)
}

func (s *workflowSuite) TestSuppressWorkflowBy_Zombiefy() {
	lastEventTaskID := int64(144)
	lastEventVersion := cluster.TestAlternativeClusterInitialFailoverVersion
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(lastEventVersion, nil).AnyTimes()
	executionInfo := &persistence.WorkflowExecutionInfo{
		DomainID:        s.domainID,
		WorkflowID:      s.workflowID,
		RunID:           s.runID,
		LastEventTaskID: lastEventTaskID,
		State:           persistence.WorkflowStateRunning,
		CloseStatus:     persistence.WorkflowCloseStatusNone,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)

	incomingRunID := uuid.New()
	incomingLastEventTaskID := int64(144)
	incomingLastEventVersion := lastEventVersion + 1
	incomingMockContext := NewMockContext(s.controller)
	incomingMockMutableState := NewMockMutableState(s.controller)
	incomingNDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		incomingMockContext,
		incomingMockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	incomingMockMutableState.EXPECT().GetLastWriteVersion().Return(incomingLastEventVersion, nil).AnyTimes()
	incomingMockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:        s.domainID,
		WorkflowID:      s.workflowID,
		RunID:           incomingRunID,
		LastEventTaskID: incomingLastEventTaskID,
	}).AnyTimes()

	// if workflow is in zombie or finished state, keep as is
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(false).Times(1)
	policy, err := nDCWorkflow.SuppressBy(incomingNDCWorkflow)
	s.NoError(err)
	s.Equal(TransactionPolicyPassive, policy)

	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true).Times(1)
	policy, err = nDCWorkflow.SuppressBy(incomingNDCWorkflow)
	s.NoError(err)
	s.Equal(TransactionPolicyPassive, policy)
	s.Equal(persistence.WorkflowStateZombie, executionInfo.State)
	s.Equal(persistence.WorkflowCloseStatusNone, executionInfo.CloseStatus)
}

func (s *workflowSuite) TestRevive_Zombie_Error() {
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateZombie, persistence.WorkflowCloseStatusNone).Times(1)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{State: persistence.WorkflowStateZombie}).Times(1)
	s.mockMutableState.EXPECT().HasProcessedOrPendingDecision().Return(true).Times(1)
	s.mockMutableState.EXPECT().UpdateWorkflowStateCloseStatus(persistence.WorkflowStateRunning, persistence.WorkflowCloseStatusNone).Return(&types.InternalServiceError{Message: "error"}).Times(1)

	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	err := nDCWorkflow.Revive()
	s.Error(err)
}

func (s *workflowSuite) TestRevive_Zombie_Success() {
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateZombie, persistence.WorkflowCloseStatusNone).Times(1)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{State: persistence.WorkflowStateZombie}).Times(1)
	s.mockMutableState.EXPECT().HasProcessedOrPendingDecision().Return(true).Times(1)
	s.mockMutableState.EXPECT().UpdateWorkflowStateCloseStatus(persistence.WorkflowStateRunning, persistence.WorkflowCloseStatusNone).Return(nil).Times(1)

	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	err := nDCWorkflow.Revive()
	s.NoError(err)
}

func (s *workflowSuite) TestRevive_NonZombie_Success() {
	s.mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateCompleted, persistence.WorkflowCloseStatusNone).Times(1)

	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	err := nDCWorkflow.Revive()
	s.NoError(err)
}

func (s *workflowSuite) TestFlushBufferedEvents_Success() {
	lastWriteVersion := cluster.TestCurrentClusterInitialFailoverVersion
	lastEventTaskID := int64(144)
	decision := &DecisionInfo{
		ScheduleID: 1,
		StartedID:  2,
	}

	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true)
	s.mockMutableState.EXPECT().HasBufferedEvents().Return(true)
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(lastWriteVersion, nil)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{LastEventTaskID: lastEventTaskID}).AnyTimes()
	s.mockMutableState.EXPECT().UpdateCurrentVersion(lastWriteVersion, true).Return(nil)
	s.mockMutableState.EXPECT().GetInFlightDecision().Return(decision, true)
	s.mockMutableState.EXPECT().AddDecisionTaskFailedEvent(decision.ScheduleID, decision.StartedID, types.DecisionTaskFailedCauseFailoverCloseDecision, nil, IdentityHistoryService, "", "", "", "", int64(0), "").Return(&types.HistoryEvent{}, nil)
	s.mockMutableState.EXPECT().FlushBufferedEvents().Return(nil)
	s.mockMutableState.EXPECT().HasPendingDecision().Return(false)
	s.mockMutableState.EXPECT().AddDecisionTaskScheduledEvent(false).Return(&DecisionInfo{}, nil)

	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	err := nDCWorkflow.FlushBufferedEvents()
	s.NoError(err)
}

func (s *workflowSuite) TestFlushBufferedEvents_NoBuffer_Success() {
	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true)
	s.mockMutableState.EXPECT().HasBufferedEvents().Return(false)

	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	err := nDCWorkflow.FlushBufferedEvents()
	s.NoError(err)
}

func (s *workflowSuite) TestFlushBufferedEvents_NoDecision_Success() {
	lastWriteVersion := cluster.TestCurrentClusterInitialFailoverVersion
	lastEventTaskID := int64(144)

	s.mockMutableState.EXPECT().IsWorkflowExecutionRunning().Return(true)
	s.mockMutableState.EXPECT().HasBufferedEvents().Return(true)
	s.mockMutableState.EXPECT().GetLastWriteVersion().Return(lastWriteVersion, nil)
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(
		&persistence.WorkflowExecutionInfo{
			DomainID:        s.domainID,
			WorkflowID:      s.workflowID,
			RunID:           s.runID,
			LastEventTaskID: lastEventTaskID,
		},
	).AnyTimes()
	s.mockMutableState.EXPECT().UpdateCurrentVersion(lastWriteVersion, true).Return(nil)
	s.mockMutableState.EXPECT().GetInFlightDecision().Return(nil, false)

	nDCWorkflow := NewWorkflow(
		context.Background(),
		cluster.TestActiveClusterMetadata,
		s.mockContext,
		s.mockMutableState,
		NoopReleaseFn,
		testlogger.New(s.T()),
	)
	err := nDCWorkflow.FlushBufferedEvents()
	s.NoError(err)
}

func TestWorkflowHappensAfter(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC-8", -8*60*60))
	utcBaseTime := baseTime.UTC()
	laterTime := baseTime.Add(time.Hour)

	// Helper function to create ActiveClusterSelectionPolicy
	createPolicy := func(scope, name string) *types.ActiveClusterSelectionPolicy {
		return &types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{
				Scope: scope,
				Name:  name,
			},
		}
	}

	// Helper function to create WorkflowVectorClock
	createVectorClock := func(policy *types.ActiveClusterSelectionPolicy, lastWriteVersion, lastEventTaskID int64, startTime time.Time, runID string) WorkflowVectorClock {
		return WorkflowVectorClock{
			ActiveClusterSelectionPolicy: policy,
			LastWriteVersion:             lastWriteVersion,
			LastEventTaskID:              lastEventTaskID,
			StartTimestamp:               startTime,
			RunID:                        runID,
		}
	}

	policy1 := createPolicy("region", "region-1")
	policy2 := createPolicy("region", "region-2")

	tests := []struct {
		name            string
		thisVectorClock WorkflowVectorClock
		thatVectorClock WorkflowVectorClock
		expected        bool
	}{
		{
			name:            "Different policies - this happens after based on start timestamp",
			thisVectorClock: createVectorClock(policy1, 100, 10, laterTime, "run-1"),
			thatVectorClock: createVectorClock(policy2, 200, 20, baseTime, "run-2"),
			expected:        true,
		},
		{
			name:            "Different policies - same start timestamp - this happens after based on RunID",
			thisVectorClock: createVectorClock(policy1, 100, 10, baseTime, "run-z"),
			thatVectorClock: createVectorClock(policy2, 200, 20, baseTime, "run-a"),
			expected:        true,
		},
		{
			name:            "Different policies - same start timestamp in different time zone - this happens after based on RunID",
			thisVectorClock: createVectorClock(policy1, 100, 10, baseTime, "run-z"),
			thatVectorClock: createVectorClock(policy2, 200, 20, utcBaseTime, "run-a"),
			expected:        true,
		},
		{
			name:            "Same policies - this happens after based on LastWriteVersion",
			thisVectorClock: createVectorClock(policy1, 200, 10, baseTime, "run-1"),
			thatVectorClock: createVectorClock(policy1, 100, 20, baseTime, "run-2"),
			expected:        true,
		},
		{
			name:            "Same policies and LastWriteVersion - this happens after based on LastEventTaskID",
			thisVectorClock: createVectorClock(policy1, 100, 20, baseTime, "run-1"),
			thatVectorClock: createVectorClock(policy1, 100, 10, baseTime, "run-2"),
			expected:        true,
		},
		{
			name:            "Nil policies - this happens after based on LastWriteVersion",
			thisVectorClock: createVectorClock(nil, 200, 10, laterTime, "run-1"),
			thatVectorClock: createVectorClock(nil, 100, 20, baseTime, "run-2"),
			expected:        true,
		},
		{
			name:            "One nil policy, one non-nil - this happens after based on start timestamp",
			thisVectorClock: createVectorClock(nil, 100, 10, laterTime, "run-1"),
			thatVectorClock: createVectorClock(policy1, 200, 20, baseTime, "run-2"),
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workflowHappensAfter(tt.thisVectorClock, tt.thatVectorClock)
			assert.Equal(t, tt.expected, result, "workflowHappensAfter result mismatch for test case: %s", tt.name)
		})
	}
}

// TestSuppressBy_ImportInProgress checks that an import in progress that loses a run conflict
// becomes a zombie with its close status, on the cluster of its last write and on other clusters.
func TestSuppressBy_ImportInProgress(t *testing.T) {
	tests := map[string]struct {
		lastWriteVersion int64
		closeStatus      int
	}{
		"last write on this cluster": {
			lastWriteVersion: cluster.TestCurrentClusterInitialFailoverVersion,
			closeStatus:      persistence.WorkflowCloseStatusNone,
		},
		"last write on another cluster": {
			lastWriteVersion: cluster.TestAlternativeClusterInitialFailoverVersion,
			closeStatus:      persistence.WorkflowCloseStatusNone,
		},
		"imported history with the close event": {
			lastWriteVersion: cluster.TestCurrentClusterInitialFailoverVersion,
			closeStatus:      persistence.WorkflowCloseStatusCompleted,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mutableState := NewMockMutableState(controller)
			executionInfo := &persistence.WorkflowExecutionInfo{
				RunID:            uuid.New(),
				State:            persistence.WorkflowStatePreparing,
				CloseStatus:      tc.closeStatus,
				ImportInProgress: true,
			}
			mutableState.EXPECT().GetLastWriteVersion().Return(tc.lastWriteVersion, nil).AnyTimes()
			mutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()

			incomingMutableState := NewMockMutableState(controller)
			incomingMutableState.EXPECT().GetLastWriteVersion().Return(tc.lastWriteVersion+1, nil).AnyTimes()
			incomingMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{RunID: uuid.New()}).AnyTimes()

			workflow := NewWorkflow(context.Background(), cluster.TestActiveClusterMetadata, NewMockContext(controller), mutableState, NoopReleaseFn, testlogger.New(t))
			incomingWorkflow := NewWorkflow(context.Background(), cluster.TestActiveClusterMetadata, NewMockContext(controller), incomingMutableState, NoopReleaseFn, testlogger.New(t))

			policy, err := workflow.SuppressBy(incomingWorkflow)
			require.NoError(t, err)
			assert.Equal(t, TransactionPolicyPassive, policy)
			assert.Equal(t, persistence.WorkflowStateZombie, executionInfo.State)
			assert.Equal(t, tc.closeStatus, executionInfo.CloseStatus)
		})
	}
}

// TestRevive_ImportInProgress checks that a zombie import becomes an import in progress again
// with its close status, and that a zombie that is not an import becomes running.
func TestRevive_ImportInProgress(t *testing.T) {
	tests := map[string]struct {
		executionInfo   *persistence.WorkflowExecutionInfo
		wantState       int
		wantCloseStatus int
	}{
		"zombie import without the close event": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				State:            persistence.WorkflowStateZombie,
				CloseStatus:      persistence.WorkflowCloseStatusNone,
				ImportInProgress: true,
			},
			wantState:       persistence.WorkflowStatePreparing,
			wantCloseStatus: persistence.WorkflowCloseStatusNone,
		},
		"zombie import with the close event": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				State:            persistence.WorkflowStateZombie,
				CloseStatus:      persistence.WorkflowCloseStatusCompleted,
				ImportInProgress: true,
			},
			wantState:       persistence.WorkflowStatePreparing,
			wantCloseStatus: persistence.WorkflowCloseStatusCompleted,
		},
		"zombie that is not an import": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				State:       persistence.WorkflowStateZombie,
				CloseStatus: persistence.WorkflowCloseStatusNone,
			},
			wantState:       persistence.WorkflowStateRunning,
			wantCloseStatus: persistence.WorkflowCloseStatusNone,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mutableState := NewMockMutableState(controller)
			mutableState.EXPECT().GetWorkflowStateCloseStatus().Return(tc.executionInfo.State, tc.executionInfo.CloseStatus).AnyTimes()
			mutableState.EXPECT().GetExecutionInfo().Return(tc.executionInfo).AnyTimes()
			mutableState.EXPECT().HasProcessedOrPendingDecision().Return(true).AnyTimes()
			mutableState.EXPECT().UpdateWorkflowStateCloseStatus(gomock.Any(), gomock.Any()).DoAndReturn(tc.executionInfo.UpdateWorkflowStateCloseStatus).Times(1)

			workflow := NewWorkflow(context.Background(), cluster.TestActiveClusterMetadata, NewMockContext(controller), mutableState, NoopReleaseFn, testlogger.New(t))

			require.NoError(t, workflow.Revive())
			assert.Equal(t, tc.wantState, tc.executionInfo.State)
			assert.Equal(t, tc.wantCloseStatus, tc.executionInfo.CloseStatus)
		})
	}
}
