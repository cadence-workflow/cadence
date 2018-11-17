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
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	gen "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	p "github.com/uber/cadence/common/persistence"
)

type (
	// ExecutionManagerSuiteForEventsV2 contains matching persistence tests
	ExecutionManagerSuiteForEventsV2 struct {
		TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

// SetupSuite implementation
func (s *ExecutionManagerSuiteForEventsV2) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// TearDownSuite implementation
func (s *ExecutionManagerSuiteForEventsV2) TearDownSuite() {
	s.TearDownWorkflowStore()
}

// SetupTest implementation
func (s *ExecutionManagerSuiteForEventsV2) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
	s.ClearTasks()
}

// TestWorkflowCreation test
func (s *ExecutionManagerSuiteForEventsV2) TestWorkflowCreation() {
	domainID := uuid.New()
	workflowExecution := gen.WorkflowExecution{
		WorkflowId: common.StringPtr("test-eventsv2-workflow"),
		RunId:      common.StringPtr("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
	}

	_, err0 := s.ExecutionManager.CreateWorkflowExecution(&p.CreateWorkflowExecutionRequest{
		RequestID:            uuid.New(),
		DomainID:             domainID,
		Execution:            workflowExecution,
		TaskList:             "taskList",
		WorkflowTypeName:     "wType",
		WorkflowTimeout:      20,
		DecisionTimeoutValue: 13,
		ExecutionContext:     nil,
		NextEventID:          3,
		LastProcessedEvent:   0,
		RangeID:              s.ShardInfo.RangeID,
		TransferTasks: []p.Task{
			&p.DecisionTask{
				TaskID:              s.GetNextSequenceNumber(),
				DomainID:            domainID,
				TaskList:            "taskList",
				ScheduleID:          2,
				VisibilityTimestamp: time.Now(),
			},
		},
		TimerTasks:                  nil,
		DecisionScheduleID:          2,
		DecisionStartedID:           common.EmptyEventID,
		DecisionStartToCloseTimeout: 1,
		EventStoreVersion:           p.EventStoreVersionV2,
		BranchToken:                 []byte("branchToken1"),
	})

	s.NoError(err0)

	state0, err1 := s.GetWorkflowExecutionInfo(domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal(int32(p.EventStoreVersionV2), info0.EventStoreVersion)
	s.Equal([]byte("branchToken1"), info0.HistoryBranches[0].BranchToken)
	s.Equal(info0.HistorySize, info0.HistoryBranches[0].HistorySize)
	s.Equal(info0.LastFirstEventID, info0.HistoryBranches[0].LastFirstEventID)
	s.Equal(info0.NextEventID, info0.HistoryBranches[0].NextEventID)
	s.Equal(int64(3), info0.HistoryBranches[0].NextEventID)

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	currentTime := time.Now().UTC()
	timerID := "id_1"
	timerInfos := []*p.TimerInfo{{
		Version:    3345,
		TimerID:    timerID,
		ExpiryTime: currentTime,
		TaskID:     2,
		StartedID:  5,
	}}
	updatedInfo.CurrentResetVersion = 1
	updatedInfo.HistoryBranches[1] = &p.HistoryBranch{
		BranchToken:      []byte("branchToken2"),
		NextEventID:      updatedInfo.NextEventID,
		HistorySize:      updatedInfo.HistorySize,
		LastFirstEventID: updatedInfo.LastFirstEventID,
	}

	err2 := s.UpdateWorkflowExecution(updatedInfo, []int64{int64(4)}, nil, int64(3), nil, nil, nil, nil, timerInfos, nil)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.TimerInfos))
	s.Equal(int64(3345), state.TimerInfos[timerID].Version)
	s.Equal(timerID, state.TimerInfos[timerID].TimerID)
	s.EqualTimesWithPrecision(currentTime, state.TimerInfos[timerID].ExpiryTime, time.Millisecond*500)
	s.Equal(int64(2), state.TimerInfos[timerID].TaskID)
	s.Equal(int64(5), state.TimerInfos[timerID].StartedID)

	err2 = s.UpdateWorkflowExecution(updatedInfo, nil, nil, int64(5), nil, nil, nil, nil, nil, []string{timerID})
	s.NoError(err2)

	state, err1 = s.GetWorkflowExecutionInfo(domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.TimerInfos))
	info1 := state.ExecutionInfo
	s.Equal(int32(p.EventStoreVersionV2), info1.EventStoreVersion)
	s.Equal(int32(1), info1.CurrentResetVersion)
	currBr := info1.HistoryBranches[1]
	s.Equal([]byte("branchToken2"), currBr.BranchToken)
	s.Equal(info1.HistorySize, currBr.HistorySize)
	s.Equal(info1.LastFirstEventID, currBr.LastFirstEventID)
	s.Equal(info1.NextEventID, currBr.NextEventID)
	s.Equal(int64(5), currBr.NextEventID)
}

//TestContinueAsNew test
func (s *ExecutionManagerSuiteForEventsV2) TestContinueAsNew() {
	domainID := uuid.New()
	workflowExecution := gen.WorkflowExecution{
		WorkflowId: common.StringPtr("continue-as-new-workflow-test"),
		RunId:      common.StringPtr("551c88d2-d9e6-404f-8131-9eec14f36643"),
	}

	_, err0 := s.CreateWorkflowExecution(domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil)
	s.NoError(err0)

	state0, err1 := s.GetWorkflowExecutionInfo(domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedInfo.State = p.WorkflowStateCompleted
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)

	newWorkflowExecution := gen.WorkflowExecution{
		WorkflowId: common.StringPtr("continue-as-new-workflow-test"),
		RunId:      common.StringPtr("64c7e15a-3fd7-4182-9c6f-6f25a4fa2614"),
	}

	newdecisionTask := &p.DecisionTask{
		TaskID:     s.GetNextSequenceNumber(),
		DomainID:   updatedInfo.DomainID,
		TaskList:   updatedInfo.TaskList,
		ScheduleID: int64(2),
	}

	_, err2 := s.ExecutionManager.UpdateWorkflowExecution(&p.UpdateWorkflowExecutionRequest{
		ExecutionInfo:       updatedInfo,
		TransferTasks:       []p.Task{newdecisionTask},
		TimerTasks:          nil,
		Condition:           info0.NextEventID,
		DeleteTimerTask:     nil,
		RangeID:             s.ShardInfo.RangeID,
		UpsertActivityInfos: nil,
		DeleteActivityInfos: nil,
		UpserTimerInfos:     nil,
		DeleteTimerInfos:    nil,
		ContinueAsNew: &p.CreateWorkflowExecutionRequest{
			RequestID:                   uuid.New(),
			DomainID:                    updatedInfo.DomainID,
			Execution:                   newWorkflowExecution,
			TaskList:                    updatedInfo.TaskList,
			WorkflowTypeName:            updatedInfo.WorkflowTypeName,
			WorkflowTimeout:             updatedInfo.WorkflowTimeout,
			DecisionTimeoutValue:        updatedInfo.DecisionTimeoutValue,
			ExecutionContext:            nil,
			NextEventID:                 info0.NextEventID,
			LastProcessedEvent:          common.EmptyEventID,
			RangeID:                     s.ShardInfo.RangeID,
			TransferTasks:               nil,
			TimerTasks:                  nil,
			DecisionScheduleID:          int64(2),
			DecisionStartedID:           common.EmptyEventID,
			DecisionStartToCloseTimeout: 1,
			CreateWorkflowMode:          p.CreateWorkflowModeContinueAsNew,
			PreviousRunID:               updatedInfo.RunID,
			EventStoreVersion:           p.EventStoreVersionV2,
			BranchToken:                 []byte("branchToken1"),
		},
		Encoding: pickRandomEncoding(),
	})

	s.NoError(err2)

	prevExecutionState, err3 := s.GetWorkflowExecutionInfo(domainID, workflowExecution)
	s.NoError(err3)
	prevExecutionInfo := prevExecutionState.ExecutionInfo
	s.Equal(p.WorkflowStateCompleted, prevExecutionInfo.State)
	s.Equal(int64(5), prevExecutionInfo.NextEventID)
	s.Equal(int64(2), prevExecutionInfo.LastProcessedEvent)

	newExecutionState, err4 := s.GetWorkflowExecutionInfo(domainID, newWorkflowExecution)
	s.NoError(err4)
	newExecutionInfo := newExecutionState.ExecutionInfo
	s.Equal(p.WorkflowStateCreated, newExecutionInfo.State)
	s.Equal(int64(3), newExecutionInfo.NextEventID)
	s.Equal(common.EmptyEventID, newExecutionInfo.LastProcessedEvent)
	s.Equal(int64(2), newExecutionInfo.DecisionScheduleID)
	s.Equal(int32(p.EventStoreVersionV2), newExecutionInfo.EventStoreVersion)
	s.Equal(int32(0), newExecutionInfo.CurrentResetVersion)
	s.Equal(newExecutionInfo.NextEventID, newExecutionInfo.HistoryBranches[0].NextEventID)
	s.Equal([]byte("branchToken1"), newExecutionInfo.HistoryBranches[0].BranchToken)

	newRunID, err5 := s.GetCurrentWorkflowRunID(domainID, *workflowExecution.WorkflowId)
	s.NoError(err5)
	s.Equal(*newWorkflowExecution.RunId, newRunID)
}

// TestWorkflowWithReplicationState test
func (s *ExecutionManagerSuiteForEventsV2) TestWorkflowWithReplicationState() {
	domainID := uuid.New()
	runID := uuid.New()
	workflowExecution := gen.WorkflowExecution{
		WorkflowId: common.StringPtr("test-workflow-replication-state-test"),
		RunId:      common.StringPtr(runID),
	}

	replicationTasks := []p.Task{&p.HistoryReplicationTask{
		TaskID:       s.GetNextSequenceNumber(),
		FirstEventID: int64(1),
		NextEventID:  int64(3),
		Version:      int64(9),
		LastReplicationInfo: map[string]*p.ReplicationInfo{
			"dc1": {
				Version:     int64(3),
				LastEventID: int64(1),
			},
			"dc2": {
				Version:     int64(5),
				LastEventID: int64(2),
			},
		},
		EventStoreVersion:       p.EventStoreVersionV2,
		BranchToken:             []byte("branchToken1"),
		NewRunEventStoreVersion: p.EventStoreVersionV2,
		NewRunBranchToken:       []byte("branchToken2"),
	}}

	task0, err0 := s.createWorkflowExecutionWithReplication(domainID, workflowExecution, "taskList", "wType", 20, 13, 3,
		0, 2, &p.ReplicationState{
			CurrentVersion:   int64(9),
			StartVersion:     int64(8),
			LastWriteVersion: int64(7),
			LastWriteEventID: int64(6),
			LastReplicationInfo: map[string]*p.ReplicationInfo{
				"dc1": {
					Version:     int64(3),
					LastEventID: int64(1),
				},
				"dc2": {
					Version:     int64(5),
					LastEventID: int64(2),
				},
			},
		}, replicationTasks, []byte("branchToken1"))
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	taskD, err := s.GetTransferTasks(2, false)
	s.Equal(1, len(taskD), "Expected 1 decision task.")
	s.Equal(p.TransferTaskTypeDecisionTask, taskD[0].TaskType)
	err = s.CompleteTransferTask(taskD[0].TaskID)
	s.NoError(err)

	taskR, err := s.GetReplicationTasks(1, false)
	s.Equal(1, len(taskR), "Expected 1 replication task.")
	tsk := taskR[0]
	s.Equal(p.ReplicationTaskTypeHistory, tsk.TaskType)
	s.Equal(domainID, tsk.DomainID)
	s.Equal(*workflowExecution.WorkflowId, tsk.WorkflowID)
	s.Equal(*workflowExecution.RunId, tsk.RunID)
	s.Equal(int64(1), tsk.FirstEventID)
	s.Equal(int64(3), tsk.NextEventID)
	s.Equal(int64(9), tsk.Version)
	s.Equal(int32(p.EventStoreVersionV2), tsk.EventStoreVersion)
	s.Equal(int32(p.EventStoreVersionV2), tsk.NewRunEventStoreVersion)
	s.Equal([]byte("branchToken1"), tsk.BranchToken)
	s.Equal([]byte("branchToken2"), tsk.NewRunBranchToken)
	s.Equal(2, len(tsk.LastReplicationInfo))
	for k, v := range tsk.LastReplicationInfo {
		log.Infof("ReplicationInfo for %v: {Version: %v, LastEventID: %v}", k, v.Version, v.LastEventID)
		switch k {
		case "dc1":
			s.Equal(int64(3), v.Version)
			s.Equal(int64(1), v.LastEventID)
		case "dc2":
			s.Equal(int64(5), v.Version)
			s.Equal(int64(2), v.LastEventID)
		default:
			s.Fail("Unexpected key")
		}
	}
	err = s.CompleteReplicationTask(taskR[0].TaskID)
	s.NoError(err)

	state0, err1 := s.GetWorkflowExecutionInfo(domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	replicationState0 := state0.ReplicationState
	s.NotNil(info0, "Valid Workflow info expected.")
	s.NotNil(replicationState0, "Valid replication state expected.")
	s.Equal(domainID, info0.DomainID)
	s.Equal("taskList", info0.TaskList)
	s.Equal("wType", info0.WorkflowTypeName)
	s.Equal(int32(20), info0.WorkflowTimeout)
	s.Equal(int32(13), info0.DecisionTimeoutValue)
	s.Equal(int64(3), info0.NextEventID)
	s.Equal(int64(0), info0.LastProcessedEvent)
	s.Equal(int64(2), info0.DecisionScheduleID)
	s.Equal(int32(p.EventStoreVersionV2), info0.EventStoreVersion)
	s.Equal(int32(0), info0.CurrentResetVersion)
	s.Equal(info0.NextEventID, info0.HistoryBranches[0].NextEventID)
	s.Equal(int64(9), replicationState0.CurrentVersion)
	s.Equal(int64(8), replicationState0.StartVersion)
	s.Equal(int64(7), replicationState0.LastWriteVersion)
	s.Equal(int64(6), replicationState0.LastWriteEventID)
	s.Equal(2, len(replicationState0.LastReplicationInfo))
	for k, v := range replicationState0.LastReplicationInfo {
		log.Infof("ReplicationInfo for %v: {Version: %v, LastEventID: %v}", k, v.Version, v.LastEventID)
		switch k {
		case "dc1":
			s.Equal(int64(3), v.Version)
			s.Equal(int64(1), v.LastEventID)
		case "dc2":
			s.Equal(int64(5), v.Version)
			s.Equal(int64(2), v.LastEventID)
		default:
			s.Fail("Unexpected key")
		}
	}

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	updatedInfo.CurrentResetVersion = 1
	updatedInfo.HistoryBranches[1] = &p.HistoryBranch{
		BranchToken:      []byte("branchToken3"),
		NextEventID:      updatedInfo.NextEventID,
		HistorySize:      updatedInfo.HistorySize,
		LastFirstEventID: updatedInfo.LastFirstEventID,
	}

	updatedReplicationState := copyReplicationState(replicationState0)
	updatedReplicationState.CurrentVersion = int64(10)
	updatedReplicationState.StartVersion = int64(11)
	updatedReplicationState.LastWriteVersion = int64(12)
	updatedReplicationState.LastWriteEventID = int64(13)
	updatedReplicationState.LastReplicationInfo["dc1"].Version = int64(4)
	updatedReplicationState.LastReplicationInfo["dc1"].LastEventID = int64(2)

	replicationTasks1 := []p.Task{&p.HistoryReplicationTask{
		TaskID:       s.GetNextSequenceNumber(),
		FirstEventID: int64(3),
		NextEventID:  int64(5),
		Version:      int64(10),
		LastReplicationInfo: map[string]*p.ReplicationInfo{
			"dc1": {
				Version:     int64(4),
				LastEventID: int64(2),
			},
			"dc2": {
				Version:     int64(5),
				LastEventID: int64(2),
			},
		},
		EventStoreVersion:       p.EventStoreVersionV2,
		BranchToken:             []byte("branchToken3"),
		NewRunEventStoreVersion: p.EventStoreVersionV2,
		NewRunBranchToken:       []byte("branchToken4"),
	}}
	err2 := s.UpdateWorklowStateAndReplication(updatedInfo, updatedReplicationState, nil, nil, int64(3), replicationTasks1)
	s.NoError(err2)

	taskR1, err := s.GetReplicationTasks(1, false)
	s.Equal(1, len(taskR1), "Expected 1 replication task.")
	tsk1 := taskR1[0]
	s.Equal(p.ReplicationTaskTypeHistory, tsk1.TaskType)
	s.Equal(domainID, tsk1.DomainID)
	s.Equal(*workflowExecution.WorkflowId, tsk1.WorkflowID)
	s.Equal(*workflowExecution.RunId, tsk1.RunID)
	s.Equal(int64(3), tsk1.FirstEventID)
	s.Equal(int64(5), tsk1.NextEventID)
	s.Equal(int64(10), tsk1.Version)
	s.Equal(int32(p.EventStoreVersionV2), tsk1.EventStoreVersion)
	s.Equal(int32(p.EventStoreVersionV2), tsk1.NewRunEventStoreVersion)
	s.Equal([]byte("branchToken3"), tsk1.BranchToken)
	s.Equal([]byte("branchToken4"), tsk1.NewRunBranchToken)

	s.Equal(2, len(tsk1.LastReplicationInfo))
	for k, v := range tsk1.LastReplicationInfo {
		log.Infof("ReplicationInfo for %v: {Version: %v, LastEventID: %v}", k, v.Version, v.LastEventID)
		switch k {
		case "dc1":
			s.Equal(int64(4), v.Version)
			s.Equal(int64(2), v.LastEventID)
		case "dc2":
			s.Equal(int64(5), v.Version)
			s.Equal(int64(2), v.LastEventID)
		default:
			s.Fail("Unexpected key")
		}
	}
	err = s.CompleteReplicationTask(taskR1[0].TaskID)
	s.NoError(err)

	state1, err2 := s.GetWorkflowExecutionInfo(domainID, workflowExecution)
	s.NoError(err2)
	info1 := state1.ExecutionInfo
	replicationState1 := state1.ReplicationState
	s.NotNil(info1, "Valid Workflow info expected.")
	s.Equal(domainID, info1.DomainID)
	s.Equal("taskList", info1.TaskList)
	s.Equal("wType", info1.WorkflowTypeName)
	s.Equal(int32(20), info1.WorkflowTimeout)
	s.Equal(int32(13), info1.DecisionTimeoutValue)
	s.Equal(int64(5), info1.NextEventID)
	s.Equal(int32(1), info1.CurrentResetVersion)
	s.Equal(info1.NextEventID, info1.HistoryBranches[1].NextEventID)
	s.Equal([]byte("branchToken3"), info1.HistoryBranches[1].BranchToken)
	s.Equal(int64(2), info1.LastProcessedEvent)
	s.Equal(int64(2), info1.DecisionScheduleID)
	s.Equal(int64(10), replicationState1.CurrentVersion)
	s.Equal(int64(11), replicationState1.StartVersion)
	s.Equal(int64(12), replicationState1.LastWriteVersion)
	s.Equal(int64(13), replicationState1.LastWriteEventID)
	s.Equal(2, len(replicationState1.LastReplicationInfo))
	for k, v := range replicationState1.LastReplicationInfo {
		log.Infof("ReplicationInfo for %v: {Version: %v, LastEventID: %v}", k, v.Version, v.LastEventID)
		switch k {
		case "dc1":
			s.Equal(int64(4), v.Version)
			s.Equal(int64(2), v.LastEventID)
		case "dc2":
			s.Equal(int64(5), v.Version)
			s.Equal(int64(2), v.LastEventID)
		default:
			s.Fail("Unexpected key")
		}
	}
}

func (s *ExecutionManagerSuiteForEventsV2) createWorkflowExecutionWithReplication(domainID string, workflowExecution gen.WorkflowExecution,
	taskList, wType string, wTimeout int32, decisionTimeout int32, nextEventID int64,
	lastProcessedEventID int64, decisionScheduleID int64, state *p.ReplicationState, txTasks []p.Task, brToken []byte) (*p.CreateWorkflowExecutionResponse, error) {
	var transferTasks []p.Task
	var replicationTasks []p.Task
	for _, task := range txTasks {
		switch t := task.(type) {
		case *p.DecisionTask, *p.ActivityTask, *p.CloseExecutionTask, *p.CancelExecutionTask, *p.StartChildExecutionTask, *p.SignalExecutionTask:
			transferTasks = append(transferTasks, t)
		case *p.HistoryReplicationTask:
			replicationTasks = append(replicationTasks, t)
		default:
			panic("Unknown transfer task type.")
		}
	}

	transferTasks = append(transferTasks, &p.DecisionTask{
		TaskID:     s.GetNextSequenceNumber(),
		DomainID:   domainID,
		TaskList:   taskList,
		ScheduleID: decisionScheduleID,
	})
	response, err := s.ExecutionManager.CreateWorkflowExecution(&p.CreateWorkflowExecutionRequest{
		RequestID:                   uuid.New(),
		DomainID:                    domainID,
		Execution:                   workflowExecution,
		TaskList:                    taskList,
		WorkflowTypeName:            wType,
		WorkflowTimeout:             wTimeout,
		DecisionTimeoutValue:        decisionTimeout,
		NextEventID:                 nextEventID,
		LastProcessedEvent:          lastProcessedEventID,
		RangeID:                     s.ShardInfo.RangeID,
		TransferTasks:               transferTasks,
		ReplicationTasks:            replicationTasks,
		DecisionScheduleID:          decisionScheduleID,
		DecisionStartedID:           common.EmptyEventID,
		DecisionStartToCloseTimeout: 1,
		ReplicationState:            state,
		EventStoreVersion:           p.EventStoreVersionV2,
		BranchToken:                 brToken,
	})

	return response, err
}
