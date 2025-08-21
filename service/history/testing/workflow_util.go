// Copyright (c) 2021 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package testing

import (
	"testing"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

// StartWorkflow setup a workflow for testing purpose
func StartWorkflow(
	t *testing.T,
	mockShard *shard.TestContext,
	sourceDomainID string,
) (types.WorkflowExecution, execution.MutableState, error) {
	return StartWorkflowWithTaskList(&types.TaskList{
		Name: "some random task list",
	}, mockShard, sourceDomainID)
}

func StartWorkflowWithTaskList(
	tl *types.TaskList,
	mockShard *shard.TestContext,
	sourceDomainID string,
) (types.WorkflowExecution, execution.MutableState, error) {
	workflowExecution := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	workflowType := "some random workflow type"

	entry, err := mockShard.GetDomainCache().GetDomainByID(sourceDomainID)
	if err != nil {
		return types.WorkflowExecution{}, nil, err
	}
	version := entry.GetFailoverVersion()
	mutableState := execution.NewMutableStateBuilderWithVersionHistoriesWithEventV2(
		mockShard,
		mockShard.GetLogger(),
		version,
		workflowExecution.GetRunID(),
		entry,
	)
	_, err = mutableState.AddWorkflowExecutionStartedEvent(
		workflowExecution,
		&types.HistoryStartWorkflowExecutionRequest{
			DomainUUID: sourceDomainID,
			StartRequest: &types.StartWorkflowExecutionRequest{
				WorkflowType:                        &types.WorkflowType{Name: workflowType},
				TaskList:                            tl,
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(2),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
				Header: &types.Header{Fields: map[string][]byte{
					"context-key":         []byte("contextValue"),
					"123456":              []byte("123456"), // unsanitizable key
					"invalid-context-key": []byte("invalidContextValue"),
				}},
			},
			PartitionConfig: map[string]string{"userid": uuid.New()},
		},
	)
	if err != nil {
		return types.WorkflowExecution{}, nil, err
	}

	return workflowExecution, mutableState, nil
}

// SetupWorkflowWithCompletedDecision setup a workflow with a completed decision task for testing purpose
func SetupWorkflowWithCompletedDecision(
	t *testing.T,
	mockShard *shard.TestContext,
	sourceDomainID string,
) (types.WorkflowExecution, execution.MutableState, int64, error) {
	workflowExecution, mutableState, err := StartWorkflow(t, mockShard, sourceDomainID)
	if err != nil {
		return types.WorkflowExecution{}, nil, 0, err
	}

	di := AddDecisionTaskScheduledEvent(mutableState)
	event := AddDecisionTaskStartedEvent(mutableState, di.ScheduleID, mutableState.GetExecutionInfo().TaskList, uuid.New())
	di.StartedID = event.ID
	event = AddDecisionTaskCompletedEvent(mutableState, di.ScheduleID, di.StartedID, nil, "some random identity")

	return workflowExecution, mutableState, event.ID, nil
}

// CreatePersistenceMutableState generated a persistence representation of the mutable state
// a based on the in memory version
func CreatePersistenceMutableState(
	t *testing.T,
	ms execution.MutableState,
	lastEventID int64,
	lastEventVersion int64,
) (*persistence.WorkflowMutableState, error) {

	if ms.GetVersionHistories() != nil {
		currentVersionHistory, err := ms.GetVersionHistories().GetCurrentVersionHistory()
		if err != nil {
			return nil, err
		}

		err = currentVersionHistory.AddOrUpdateItem(persistence.NewVersionHistoryItem(
			lastEventID,
			lastEventVersion,
		))
		if err != nil {
			return nil, err
		}
	}

	return execution.CreatePersistenceMutableState(t, ms), nil
}
