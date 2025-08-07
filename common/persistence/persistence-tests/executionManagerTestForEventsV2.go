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

package persistencetests

import (
	"context"
	"log"
	"os"
	"runtime/debug"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// ExecutionManagerSuiteForEventsV2 contains matching persistence tests
	ExecutionManagerSuiteForEventsV2 struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

func failOnPanic(t *testing.T) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("test panicked: %v %s", r, debug.Stack())
			t.FailNow()
		}
	}()
}

// SetupSuite implementation
func (s *ExecutionManagerSuiteForEventsV2) SetupSuite() {
	defer failOnPanic(s.T())
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// TearDownSuite implementation
func (s *ExecutionManagerSuiteForEventsV2) TearDownSuite() {
	defer failOnPanic(s.T())
	s.TearDownWorkflowStore()
}

// SetupTest implementation
func (s *ExecutionManagerSuiteForEventsV2) SetupTest() {
	defer failOnPanic(s.T())
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
	s.ClearTasks()
}

func (s *ExecutionManagerSuiteForEventsV2) newRandomChecksum() checksum.Checksum {
	return checksum.Checksum{
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Version: 22,
		Value:   []byte(uuid.NewRandom()),
	}
}

func (s *ExecutionManagerSuiteForEventsV2) assertChecksumsEqual(expected checksum.Checksum, actual checksum.Checksum) {
	if !actual.Flavor.IsValid() {
		// not all stores support checksum persistence today
		// if its not supported, assert that everything is zero'd out
		expected = checksum.Checksum{}
	}
	s.EqualValues(expected, actual)
}

// TestWorkflowCreation test
func (s *ExecutionManagerSuiteForEventsV2) TestWorkflowCreation() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	defer failOnPanic(s.T())
	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-eventsv2-workflow",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	domainName := uuid.New()
	csum := s.newRandomChecksum()
	decisionScheduleID := int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{decisionScheduleID, constants.EmptyVersion},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	_, err0 := s.ExecutionManager.CreateWorkflowExecution(ctx, &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowExecution.GetWorkflowID(),
				RunID:                       workflowExecution.GetRunID(),
				FirstExecutionRunID:         workflowExecution.GetRunID(),
				TaskList:                    "taskList",
				WorkflowTypeName:            "wType",
				WorkflowTimeout:             20,
				DecisionStartToCloseTimeout: 13,
				ExecutionContext:            nil,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
				NextEventID:                 3,
				LastProcessedEvent:          0,
				DecisionScheduleID:          decisionScheduleID,
				DecisionStartedID:           constants.EmptyEventID,
				DecisionTimeout:             1,
				BranchToken:                 []byte("branchToken1"),
			},
			ExecutionStats: &p.ExecutionStats{},
			TasksByCategory: map[p.HistoryTaskCategory][]p.Task{
				p.HistoryTaskCategoryTransfer: []p.Task{
					&p.DecisionTask{
						WorkflowIdentifier: p.WorkflowIdentifier{
							DomainID:   domainID,
							WorkflowID: workflowExecution.WorkflowID,
							RunID:      workflowExecution.RunID,
						},
						TaskData: p.TaskData{
							TaskID:              s.GetNextSequenceNumber(),
							VisibilityTimestamp: time.Now(),
						},
						TargetDomainID: domainID,
						TaskList:       "taskList",
						ScheduleID:     2,
					},
				},
			},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID:    s.ShardInfo.RangeID,
		DomainName: domainName,
	})

	s.NoError(err0)

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal([]byte("branchToken1"), info0.BranchToken)
	s.assertChecksumsEqual(csum, state0.Checksum)

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	currentTime := time.Now().UTC()
	timerID := "id_1"
	timerInfos := []*p.TimerInfo{{
		Version:    3345,
		TimerID:    timerID,
		ExpiryTime: currentTime,
		TaskStatus: 2,
		StartedID:  5,
	}}
	updatedInfo.BranchToken = []byte("branchToken2")

	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, int64(3), nil, nil, nil, timerInfos, nil)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.TimerInfos))
	s.Equal(int64(3345), state.TimerInfos[timerID].Version)
	s.Equal(timerID, state.TimerInfos[timerID].TimerID)
	s.EqualTimesWithPrecision(currentTime, state.TimerInfos[timerID].ExpiryTime, time.Millisecond*500)
	s.Equal(int64(2), state.TimerInfos[timerID].TaskStatus)
	s.Equal(int64(5), state.TimerInfos[timerID].StartedID)
	s.assertChecksumsEqual(testWorkflowChecksum, state.Checksum)

	err2 = s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, nil, nil, int64(5), nil, nil, nil, nil, []string{timerID})
	s.NoError(err2)

	state, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.TimerInfos))
	info1 := state.ExecutionInfo
	s.Equal([]byte("branchToken2"), info1.BranchToken)
	s.assertChecksumsEqual(testWorkflowChecksum, state.Checksum)
}

// TestWorkflowCreationWithVersionHistories test
func (s *ExecutionManagerSuiteForEventsV2) TestWorkflowCreationWithVersionHistories() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	defer failOnPanic(s.T())
	domainID := uuid.New()
	domainName := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-eventsv2-workflow-version-history",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	versionHistory := p.NewVersionHistory(
		[]byte{1},
		[]*p.VersionHistoryItem{p.NewVersionHistoryItem(1, 0)},
	)
	versionHistories := p.NewVersionHistories(versionHistory)

	csum := s.newRandomChecksum()

	_, err0 := s.ExecutionManager.CreateWorkflowExecution(ctx, &p.CreateWorkflowExecutionRequest{
		RangeID: s.ShardInfo.RangeID,
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowExecution.GetWorkflowID(),
				RunID:                       workflowExecution.GetRunID(),
				FirstExecutionRunID:         workflowExecution.GetRunID(),
				TaskList:                    "taskList",
				WorkflowTypeName:            "wType",
				WorkflowTimeout:             20,
				DecisionStartToCloseTimeout: 13,
				ExecutionContext:            nil,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
				NextEventID:                 constants.EmptyEventID,
				LastProcessedEvent:          0,
				DecisionScheduleID:          2,
				DecisionStartedID:           constants.EmptyEventID,
				DecisionTimeout:             1,
				BranchToken:                 nil,
			},
			ExecutionStats:   &p.ExecutionStats{},
			VersionHistories: versionHistories,
			TasksByCategory: map[p.HistoryTaskCategory][]p.Task{
				p.HistoryTaskCategoryTransfer: []p.Task{
					&p.DecisionTask{
						WorkflowIdentifier: p.WorkflowIdentifier{
							DomainID:   domainID,
							WorkflowID: workflowExecution.WorkflowID,
							RunID:      workflowExecution.RunID,
						},
						TaskData: p.TaskData{
							TaskID:              s.GetNextSequenceNumber(),
							VisibilityTimestamp: time.Now(),
						},
						TargetDomainID: domainID,
						TaskList:       "taskList",
						ScheduleID:     2,
					},
				},
			},
			Checksum: csum,
		},
		DomainName: domainName,
	})

	s.NoError(err0)

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal(versionHistories, state0.VersionHistories)
	s.assertChecksumsEqual(csum, state0.Checksum)

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.LastProcessedEvent = int64(2)
	currentTime := time.Now().UTC()
	timerID := "id_1"
	timerInfos := []*p.TimerInfo{{
		Version:    3345,
		TimerID:    timerID,
		ExpiryTime: currentTime,
		TaskStatus: 2,
		StartedID:  5,
	}}
	versionHistory, err := versionHistories.GetCurrentVersionHistory()
	s.NoError(err)
	err = versionHistory.AddOrUpdateItem(p.NewVersionHistoryItem(2, 0))
	s.NoError(err)

	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, constants.EmptyEventID, nil, nil, nil, timerInfos, nil)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.TimerInfos))
	s.Equal(int64(3345), state.TimerInfos[timerID].Version)
	s.Equal(timerID, state.TimerInfos[timerID].TimerID)
	s.EqualTimesWithPrecision(currentTime, state.TimerInfos[timerID].ExpiryTime, time.Millisecond*500)
	s.Equal(int64(2), state.TimerInfos[timerID].TaskStatus)
	s.Equal(int64(5), state.TimerInfos[timerID].StartedID)
	s.Equal(state.VersionHistories, versionHistories)
	s.assertChecksumsEqual(testWorkflowChecksum, state.Checksum)
}

// TestContinueAsNew test
func (s *ExecutionManagerSuiteForEventsV2) TestContinueAsNew() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "continue-as-new-workflow-test",
		RunID:      "551c88d2-d9e6-404f-8131-9eec14f36643",
	}
	partitionConfig := map[string]string{
		"userID": uuid.New(),
	}
	decisionScheduleID := int64(2)
	_, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, decisionScheduleID, nil, partitionConfig)
	s.NoError(err0)

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.State = p.WorkflowStateCompleted
	updatedInfo.CloseStatus = p.WorkflowCloseStatusCompleted
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{decisionScheduleID, constants.EmptyVersion},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	newWorkflowExecution := types.WorkflowExecution{
		WorkflowID: "continue-as-new-workflow-test",
		RunID:      "64c7e15a-3fd7-4182-9c6f-6f25a4fa2614",
	}

	newdecisionTask := &p.DecisionTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   domainID,
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		},
		TaskData: p.TaskData{
			TaskID: s.GetNextSequenceNumber(),
		},
		TargetDomainID: updatedInfo.DomainID,
		TaskList:       updatedInfo.TaskList,
		ScheduleID:     int64(2),
	}

	_, err2 := s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:  updatedInfo,
			ExecutionStats: updatedStats,
			TasksByCategory: map[p.HistoryTaskCategory][]p.Task{
				p.HistoryTaskCategoryTransfer: []p.Task{newdecisionTask},
			},
			Condition:           info0.NextEventID,
			UpsertActivityInfos: nil,
			DeleteActivityInfos: nil,
			UpsertTimerInfos:    nil,
			DeleteTimerInfos:    nil,
			VersionHistories:    versionHistories,
		},
		NewWorkflowSnapshot: &p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    updatedInfo.DomainID,
				WorkflowID:                  newWorkflowExecution.GetWorkflowID(),
				RunID:                       newWorkflowExecution.GetRunID(),
				FirstExecutionRunID:         updatedInfo.FirstExecutionRunID,
				TaskList:                    updatedInfo.TaskList,
				TaskListKind:                updatedInfo.TaskListKind,
				WorkflowTypeName:            updatedInfo.WorkflowTypeName,
				WorkflowTimeout:             updatedInfo.WorkflowTimeout,
				DecisionStartToCloseTimeout: updatedInfo.DecisionStartToCloseTimeout,
				ExecutionContext:            nil,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
				NextEventID:                 info0.NextEventID,
				LastProcessedEvent:          constants.EmptyEventID,
				DecisionScheduleID:          int64(2),
				DecisionStartedID:           constants.EmptyEventID,
				DecisionTimeout:             1,
				BranchToken:                 []byte("branchToken1"),
				PartitionConfig:             partitionConfig,
			},
			ExecutionStats:   &p.ExecutionStats{},
			VersionHistories: versionHistories,
		},
		RangeID:  s.ShardInfo.RangeID,
		Encoding: pickRandomEncoding(),
	})

	s.NoError(err2)

	prevExecutionState, err3 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err3)
	prevExecutionInfo := prevExecutionState.ExecutionInfo
	s.Equal("551c88d2-d9e6-404f-8131-9eec14f36643", prevExecutionInfo.FirstExecutionRunID)
	s.Equal(p.WorkflowStateCompleted, prevExecutionInfo.State)
	s.Equal(int64(5), prevExecutionInfo.NextEventID)
	s.Equal(int64(2), prevExecutionInfo.LastProcessedEvent)
	s.Equal(partitionConfig, prevExecutionInfo.PartitionConfig)

	newExecutionState, err4 := s.GetWorkflowExecutionInfo(ctx, domainID, newWorkflowExecution)
	s.NoError(err4)
	newExecutionInfo := newExecutionState.ExecutionInfo
	s.Equal("551c88d2-d9e6-404f-8131-9eec14f36643", newExecutionInfo.FirstExecutionRunID)
	s.Equal(p.WorkflowStateRunning, newExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, newExecutionInfo.CloseStatus)
	s.Equal(int64(3), newExecutionInfo.NextEventID)
	s.Equal(constants.EmptyEventID, newExecutionInfo.LastProcessedEvent)
	s.Equal(int64(2), newExecutionInfo.DecisionScheduleID)
	s.Equal([]byte("branchToken1"), newExecutionInfo.BranchToken)
	s.Equal(partitionConfig, newExecutionInfo.PartitionConfig)

	newRunID, err5 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.WorkflowID)
	s.NoError(err5)
	s.Equal(newWorkflowExecution.RunID, newRunID)
}
