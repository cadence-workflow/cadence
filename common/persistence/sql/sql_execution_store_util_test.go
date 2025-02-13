// Copyright (c) 2018 Uber Technologies, Inc.
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

package sql

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

func mockSetupLockAndCheckNextEventID(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	condition int64,
	wantErr bool,
) {
	var nextEventID int
	var err error
	if wantErr {
		err = errors.New("some error")
	} else {
		nextEventID = int(condition)
	}
	mockTx.EXPECT().WriteLockExecutions(gomock.Any(), &sqlplugin.ExecutionsFilter{
		ShardID:    shardID,
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nextEventID, err)
}

func mockCreateExecution(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().WorkflowExecutionInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil)
	mockTx.EXPECT().InsertIntoExecutions(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: 1}, err)
}

func mockUpdateExecution(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().WorkflowExecutionInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil)
	mockTx.EXPECT().UpdateExecutions(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: 1}, err)
}

func mockCreateTransferTasks(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	tasks int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().TransferTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(tasks)
	mockTx.EXPECT().InsertIntoTransferTasks(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: int64(tasks)}, err)
}

func mockCreateReplicationTasks(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	tasks int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(tasks)
	mockTx.EXPECT().InsertIntoReplicationTasks(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: int64(tasks)}, err)
}

func mockCreateTimerTasks(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	tasks int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().TimerTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(tasks)
	mockTx.EXPECT().InsertIntoTimerTasks(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: int64(tasks)}, err)
}

func mockApplyTasks(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	transfer int,
	timer int,
	replication int,
	wantErr bool,
) {
	mockCreateTransferTasks(mockTx, mockParser, transfer, wantErr)
	if wantErr {
		return
	}
	mockCreateTimerTasks(mockTx, mockParser, timer, wantErr)
	mockCreateReplicationTasks(mockTx, mockParser, replication, wantErr)
}

func mockUpdateActivityInfos(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	activityInfos int,
	deleteInfos int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().ActivityInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(activityInfos)
	if activityInfos > 0 {
		mockTx.EXPECT().ReplaceIntoActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, nil)
	}
	if deleteInfos > 0 {
		mockTx.EXPECT().DeleteFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, err)
	}
}

func mockUpdateTimerInfos(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	timerInfos int,
	deleteInfos int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().TimerInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(timerInfos)
	if timerInfos > 0 {
		mockTx.EXPECT().ReplaceIntoTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, nil)
	}
	if deleteInfos > 0 {
		mockTx.EXPECT().DeleteFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, err)
	}
}

func mockUpdateChildExecutionInfos(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	childExecutionInfos int,
	deleteInfos int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().ChildExecutionInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(childExecutionInfos)
	if childExecutionInfos > 0 {
		mockTx.EXPECT().ReplaceIntoChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, nil)
	}
	if deleteInfos > 0 {
		mockTx.EXPECT().DeleteFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, err)
	}
}

func mockUpdateRequestCancelInfos(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	cancelInfos int,
	deleteInfos int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().RequestCancelInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(cancelInfos)
	if cancelInfos > 0 {
		mockTx.EXPECT().ReplaceIntoRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, nil)
	}
	if deleteInfos > 0 {
		mockTx.EXPECT().DeleteFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, err)
	}
}

func mockUpdateSignalInfos(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	signalInfos int,
	deleteInfos int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockParser.EXPECT().SignalInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil).Times(signalInfos)
	if signalInfos > 0 {
		mockTx.EXPECT().ReplaceIntoSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, nil)
	}
	if deleteInfos > 0 {
		mockTx.EXPECT().DeleteFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, err)
	}
}

func mockUpdateSignalRequested(
	mockTx *sqlplugin.MockTx,
	mockParser *serialization.MockParser,
	signalRequested int,
	deleteInfos int,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	if signalRequested > 0 {
		mockTx.EXPECT().InsertIntoSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, nil)
	}
	if deleteInfos > 0 {
		mockTx.EXPECT().DeleteFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, err)
	}
}

func mockDeleteActivityInfoMap(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockTx.EXPECT().DeleteFromActivityInfoMaps(gomock.Any(), &sqlplugin.ActivityInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nil, err)
	if wantErr {
		mockTx.EXPECT().IsNotFoundError(err).Return(true)
	}
}

func mockDeleteTimerInfoMap(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockTx.EXPECT().DeleteFromTimerInfoMaps(gomock.Any(), &sqlplugin.TimerInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nil, err)
	if wantErr {
		mockTx.EXPECT().IsNotFoundError(err).Return(true)
	}
}

func mockDeleteChildExecutionInfoMap(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockTx.EXPECT().DeleteFromChildExecutionInfoMaps(gomock.Any(), &sqlplugin.ChildExecutionInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nil, err)
	if wantErr {
		mockTx.EXPECT().IsNotFoundError(err).Return(true)
	}
}

func mockDeleteRequestCancelInfoMap(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockTx.EXPECT().DeleteFromRequestCancelInfoMaps(gomock.Any(), &sqlplugin.RequestCancelInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nil, err)
	if wantErr {
		mockTx.EXPECT().IsNotFoundError(err).Return(true)
	}
}

func mockDeleteSignalInfoMap(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockTx.EXPECT().DeleteFromSignalInfoMaps(gomock.Any(), &sqlplugin.SignalInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nil, err)
	if wantErr {
		mockTx.EXPECT().IsNotFoundError(err).Return(true)
	}
}

func mockDeleteSignalRequestedSet(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockTx.EXPECT().DeleteFromSignalsRequestedSets(gomock.Any(), &sqlplugin.SignalsRequestedSetsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nil, err)
	if wantErr {
		mockTx.EXPECT().IsNotFoundError(err).Return(true)
	}
}

func mockDeleteBufferedEvents(
	mockTx *sqlplugin.MockTx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	wantErr bool,
) {
	var err error
	if wantErr {
		err = errors.New("some error")
	}
	mockTx.EXPECT().DeleteFromBufferedEvents(gomock.Any(), &sqlplugin.BufferedEventsFilter{
		ShardID:    shardID,
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).Return(nil, err)
	if wantErr {
		mockTx.EXPECT().IsNotFoundError(err).Return(true)
	}
}

func TestApplyWorkflowMutationTx(t *testing.T) {
	shardID := 1
	testCases := []struct {
		name      string
		workflow  *persistence.InternalWorkflowMutation
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			workflow: &persistence.InternalWorkflowMutation{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
					DomainID:   "8be8a310-7d20-483e-a5d2-48659dc47602",
					WorkflowID: "abc",
					RunID:      "8be8a310-7d20-483e-a5d2-48659dc47603",
				},
				Condition: 9,
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryTransfer: []persistence.Task{
						&persistence.ActivityTask{},
					},
					persistence.HistoryTaskCategoryTimer: []persistence.Task{
						&persistence.DecisionTimeoutTask{},
						&persistence.DecisionTimeoutTask{},
						&persistence.DecisionTimeoutTask{},
					},
					persistence.HistoryTaskCategoryReplication: []persistence.Task{
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
					},
				},
				UpsertActivityInfos: []*persistence.InternalActivityInfo{
					{},
				},
				DeleteActivityInfos: []int64{1, 2},
				UpsertTimerInfos: []*persistence.TimerInfo{
					{},
				},
				DeleteTimerInfos: []string{"a", "b"},
				UpsertChildExecutionInfos: []*persistence.InternalChildExecutionInfo{
					{},
				},
				DeleteChildExecutionInfos: []int64{1, 2},
				UpsertRequestCancelInfos: []*persistence.RequestCancelInfo{
					{},
				},
				DeleteRequestCancelInfos: []int64{1, 2},
				UpsertSignalInfos: []*persistence.SignalInfo{
					{},
				},
				DeleteSignalInfos:        []int64{1, 2},
				UpsertSignalRequestedIDs: []string{"a", "b"},
				DeleteSignalRequestedIDs: []string{"c", "d"},
				ClearBufferedEvents:      true,
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockSetupLockAndCheckNextEventID(mockTx, shardID, serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"), "abc", serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"), 9, false)
				mockUpdateExecution(mockTx, mockParser, false)
				mockApplyTasks(mockTx, mockParser, 1, 3, 4, false)
				mockUpdateActivityInfos(mockTx, mockParser, 1, 2, false)
				mockUpdateTimerInfos(mockTx, mockParser, 1, 2, false)
				mockUpdateChildExecutionInfos(mockTx, mockParser, 1, 2, false)
				mockUpdateRequestCancelInfos(mockTx, mockParser, 1, 2, false)
				mockUpdateSignalInfos(mockTx, mockParser, 1, 2, false)
				mockUpdateSignalRequested(mockTx, mockParser, 1, 2, false)
				mockDeleteBufferedEvents(mockTx, shardID, serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"), "abc", serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"), false)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := applyWorkflowMutationTx(context.Background(), mockTx, shardID, tc.workflow, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestApplyWorkflowSnapshotTxAsReset(t *testing.T) {
	shardID := 1
	testCases := []struct {
		name      string
		workflow  *persistence.InternalWorkflowSnapshot
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			workflow: &persistence.InternalWorkflowSnapshot{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
					DomainID:   "8be8a310-7d20-483e-a5d2-48659dc47602",
					WorkflowID: "abc",
					RunID:      "8be8a310-7d20-483e-a5d2-48659dc47603",
				},
				Condition: 9,
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryTransfer: []persistence.Task{
						&persistence.ActivityTask{},
					},
					persistence.HistoryTaskCategoryTimer: []persistence.Task{
						&persistence.DecisionTimeoutTask{},
						&persistence.DecisionTimeoutTask{},
						&persistence.DecisionTimeoutTask{},
					},
					persistence.HistoryTaskCategoryReplication: []persistence.Task{
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
					},
				},
				ActivityInfos: []*persistence.InternalActivityInfo{
					{},
				},
				TimerInfos: []*persistence.TimerInfo{
					{},
				},
				ChildExecutionInfos: []*persistence.InternalChildExecutionInfo{
					{},
				},
				RequestCancelInfos: []*persistence.RequestCancelInfo{
					{},
				},
				SignalInfos: []*persistence.SignalInfo{
					{},
				},
				SignalRequestedIDs: []string{"a", "b"},
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				domainID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602")
				workflowID := "abc"
				runID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603")
				mockSetupLockAndCheckNextEventID(mockTx, shardID, domainID, workflowID, runID, 9, false)
				mockUpdateExecution(mockTx, mockParser, false)
				mockApplyTasks(mockTx, mockParser, 1, 3, 4, false)
				mockDeleteActivityInfoMap(mockTx, shardID, domainID, workflowID, runID, false)
				mockUpdateActivityInfos(mockTx, mockParser, 1, 0, false)
				mockDeleteTimerInfoMap(mockTx, shardID, domainID, workflowID, runID, false)
				mockUpdateTimerInfos(mockTx, mockParser, 1, 0, false)
				mockDeleteChildExecutionInfoMap(mockTx, shardID, domainID, workflowID, runID, false)
				mockUpdateChildExecutionInfos(mockTx, mockParser, 1, 0, false)
				mockDeleteRequestCancelInfoMap(mockTx, shardID, domainID, workflowID, runID, false)
				mockUpdateRequestCancelInfos(mockTx, mockParser, 1, 0, false)
				mockDeleteSignalInfoMap(mockTx, shardID, domainID, workflowID, runID, false)
				mockUpdateSignalInfos(mockTx, mockParser, 1, 0, false)
				mockDeleteSignalRequestedSet(mockTx, shardID, domainID, workflowID, runID, false)
				mockUpdateSignalRequested(mockTx, mockParser, 1, 0, false)
				mockDeleteBufferedEvents(mockTx, shardID, domainID, workflowID, runID, false)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := applyWorkflowSnapshotTxAsReset(context.Background(), mockTx, shardID, tc.workflow, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestApplyWorkflowSnapshotTxAsNew(t *testing.T) {
	shardID := 1
	testCases := []struct {
		name      string
		workflow  *persistence.InternalWorkflowSnapshot
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			workflow: &persistence.InternalWorkflowSnapshot{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
					DomainID:   "8be8a310-7d20-483e-a5d2-48659dc47602",
					WorkflowID: "abc",
					RunID:      "8be8a310-7d20-483e-a5d2-48659dc47603",
				},
				Condition: 9,
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryTransfer: []persistence.Task{
						&persistence.ActivityTask{},
					},
					persistence.HistoryTaskCategoryTimer: []persistence.Task{
						&persistence.DecisionTimeoutTask{},
						&persistence.DecisionTimeoutTask{},
						&persistence.DecisionTimeoutTask{},
					},
					persistence.HistoryTaskCategoryReplication: []persistence.Task{
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
						&persistence.HistoryReplicationTask{},
					},
				},
				ActivityInfos: []*persistence.InternalActivityInfo{
					{},
				},
				TimerInfos: []*persistence.TimerInfo{
					{},
				},
				ChildExecutionInfos: []*persistence.InternalChildExecutionInfo{
					{},
				},
				RequestCancelInfos: []*persistence.RequestCancelInfo{
					{},
				},
				SignalInfos: []*persistence.SignalInfo{
					{},
				},
				SignalRequestedIDs: []string{"a", "b"},
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockCreateExecution(mockTx, mockParser, false)
				mockApplyTasks(mockTx, mockParser, 1, 3, 4, false)
				mockUpdateActivityInfos(mockTx, mockParser, 1, 0, false)
				mockUpdateTimerInfos(mockTx, mockParser, 1, 0, false)
				mockUpdateChildExecutionInfos(mockTx, mockParser, 1, 0, false)
				mockUpdateRequestCancelInfos(mockTx, mockParser, 1, 0, false)
				mockUpdateSignalInfos(mockTx, mockParser, 1, 0, false)
				mockUpdateSignalRequested(mockTx, mockParser, 1, 0, false)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := applyWorkflowSnapshotTxAsNew(context.Background(), mockTx, shardID, tc.workflow, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestLockAndCheckNextEventID(t *testing.T) {
	shardID := 1
	domainID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602")
	workflowID := "abc"
	runID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603")
	testCases := []struct {
		name      string
		condition int64
		mockSetup func(*sqlplugin.MockTx)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name:      "Success case",
			condition: 10,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().WriteLockExecutions(gomock.Any(), &sqlplugin.ExecutionsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
				}).Return(10, nil)
			},
			wantErr: false,
		},
		{
			name:      "Error case - entity not exists",
			condition: 10,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().WriteLockExecutions(gomock.Any(), &sqlplugin.ExecutionsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
				}).Return(0, sql.ErrNoRows)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.EntityNotExistsError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be EntityNotExistsError")
			},
		},
		{
			name:      "Error case - condition failed",
			condition: 10,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().WriteLockExecutions(gomock.Any(), &sqlplugin.ExecutionsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
				}).Return(11, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.ConditionFailedError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be ConditionFailedError")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)

			err := lockAndCheckNextEventID(context.Background(), mockTx, shardID, domainID, workflowID, runID, tc.condition)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestCreateExecution(t *testing.T) {
	shardID := 1
	testCases := []struct {
		name      string
		workflow  *persistence.InternalWorkflowSnapshot
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			workflow: &persistence.InternalWorkflowSnapshot{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "8be8a310-7d20-483e-a5d2-48659dc47602",
					WorkflowID:  "abc",
					RunID:       "8be8a310-7d20-483e-a5d2-48659dc47603",
					NextEventID: 9,
				},
				VersionHistories: &persistence.DataBlob{},
				StartVersion:     1,
				LastWriteVersion: 2,
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().WorkflowExecutionInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`workflow`),
					Encoding: common.EncodingType("workflow"),
				}, nil)
				mockTx.EXPECT().InsertIntoExecutions(gomock.Any(), &sqlplugin.ExecutionsRow{
					ShardID:          shardID,
					DomainID:         serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID:       "abc",
					RunID:            serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
					NextEventID:      9,
					LastWriteVersion: 2,
					Data:             []byte(`workflow`),
					DataEncoding:     "workflow",
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - already started",
			workflow: &persistence.InternalWorkflowSnapshot{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "8be8a310-7d20-483e-a5d2-48659dc47602",
					WorkflowID:  "abc",
					RunID:       "8be8a310-7d20-483e-a5d2-48659dc47603",
					NextEventID: 9,
				},
				VersionHistories: &persistence.DataBlob{},
				StartVersion:     1,
				LastWriteVersion: 2,
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().WorkflowExecutionInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`workflow`),
					Encoding: common.EncodingType("workflow"),
				}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoExecutions(gomock.Any(), &sqlplugin.ExecutionsRow{
					ShardID:          shardID,
					DomainID:         serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID:       "abc",
					RunID:            serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
					NextEventID:      9,
					LastWriteVersion: 2,
					Data:             []byte(`workflow`),
					DataEncoding:     "workflow",
				}).Return(nil, err)
				mockTx.EXPECT().IsDupEntryError(err).Return(true)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.WorkflowExecutionAlreadyStartedError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be WorkflowExecutionAlreadyStartedError")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := createExecution(context.Background(), mockTx, tc.workflow.ExecutionInfo, tc.workflow.VersionHistories, tc.workflow.ChecksumData, tc.workflow.StartVersion, tc.workflow.LastWriteVersion, shardID, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestUpdateExecution(t *testing.T) {
	shardID := 1
	testCases := []struct {
		name      string
		workflow  *persistence.InternalWorkflowSnapshot
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			workflow: &persistence.InternalWorkflowSnapshot{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "8be8a310-7d20-483e-a5d2-48659dc47602",
					WorkflowID:  "abc",
					RunID:       "8be8a310-7d20-483e-a5d2-48659dc47603",
					NextEventID: 9,
				},
				VersionHistories: &persistence.DataBlob{},
				StartVersion:     1,
				LastWriteVersion: 2,
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().WorkflowExecutionInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`workflow`),
					Encoding: common.EncodingType("workflow"),
				}, nil)
				mockTx.EXPECT().UpdateExecutions(gomock.Any(), &sqlplugin.ExecutionsRow{
					ShardID:          shardID,
					DomainID:         serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID:       "abc",
					RunID:            serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
					NextEventID:      9,
					LastWriteVersion: 2,
					Data:             []byte(`workflow`),
					DataEncoding:     "workflow",
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - already started",
			workflow: &persistence.InternalWorkflowSnapshot{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "8be8a310-7d20-483e-a5d2-48659dc47602",
					WorkflowID:  "abc",
					RunID:       "8be8a310-7d20-483e-a5d2-48659dc47603",
					NextEventID: 9,
				},
				VersionHistories: &persistence.DataBlob{},
				StartVersion:     1,
				LastWriteVersion: 2,
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().WorkflowExecutionInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`workflow`),
					Encoding: common.EncodingType("workflow"),
				}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateExecutions(gomock.Any(), &sqlplugin.ExecutionsRow{
					ShardID:          shardID,
					DomainID:         serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID:       "abc",
					RunID:            serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
					NextEventID:      9,
					LastWriteVersion: 2,
					Data:             []byte(`workflow`),
					DataEncoding:     "workflow",
				}).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := updateExecution(context.Background(), mockTx, tc.workflow.ExecutionInfo, tc.workflow.VersionHistories, tc.workflow.ChecksumData, tc.workflow.StartVersion, tc.workflow.LastWriteVersion, shardID, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestCreateTransferTasks(t *testing.T) {
	shardID := 1
	domainID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602")
	workflowID := "abc"
	runID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603")
	testCases := []struct {
		name      string
		tasks     []persistence.Task
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			tasks: []persistence.Task{
				&persistence.ActivityTask{
					TaskData: persistence.TaskData{
						Version:             1,
						VisibilityTimestamp: time.Unix(1, 1),
						TaskID:              1,
					},
					DomainID:   "8be8a310-7d20-483e-a5d2-48659dc47609",
					TaskList:   "tl",
					ScheduleID: 111,
				},
				&persistence.DecisionTask{
					TaskData: persistence.TaskData{
						Version:             2,
						VisibilityTimestamp: time.Unix(2, 2),
						TaskID:              2,
					},
					DomainID:   "7be8a310-7d20-483e-a5d2-48659dc47609",
					TaskList:   "tl2",
					ScheduleID: 222,
				},
				&persistence.CancelExecutionTask{
					TaskData: persistence.TaskData{
						Version:             3,
						VisibilityTimestamp: time.Unix(3, 3),
						TaskID:              3,
					},
					TargetDomainID:          "6be8a310-7d20-483e-a5d2-48659dc47609",
					TargetWorkflowID:        "acd",
					TargetRunID:             "3be8a310-7d20-483e-a5d2-48659dc47609",
					TargetChildWorkflowOnly: true,
					InitiatedID:             333,
				},
				&persistence.SignalExecutionTask{
					TaskData: persistence.TaskData{
						Version:             5,
						VisibilityTimestamp: time.Unix(5, 5),
						TaskID:              5,
					},
					TargetDomainID:          "5be8a310-7d20-483e-a5d2-48659dc47609",
					TargetWorkflowID:        "zcd",
					TargetRunID:             "4be8a310-7d20-483e-a5d2-48659dc47609",
					TargetChildWorkflowOnly: true,
					InitiatedID:             555,
				},
				&persistence.StartChildExecutionTask{
					TaskData: persistence.TaskData{
						Version:             7,
						VisibilityTimestamp: time.Unix(7, 7),
						TaskID:              7,
					},
					TargetDomainID:   "2be8a310-7d20-483e-a5d2-48659dc47609",
					TargetWorkflowID: "xcd",
					InitiatedID:      777,
				},
				&persistence.RecordChildExecutionCompletedTask{
					TaskData: persistence.TaskData{
						Version:             8,
						VisibilityTimestamp: time.Unix(8, 8),
						TaskID:              8,
					},
					TargetDomainID:   "1be8a310-7d20-483e-a5d2-48659dc47609",
					TargetWorkflowID: "ddd",
					TargetRunID:      "0be8a310-7d20-483e-a5d2-48659dc47609",
				},
				&persistence.ApplyParentClosePolicyTask{
					TaskData: persistence.TaskData{
						Version:             9,
						VisibilityTimestamp: time.Unix(9, 9),
						TaskID:              9,
					},
					TargetDomainIDs: map[string]struct{}{"abe8a310-7d20-483e-a5d2-48659dc47609": struct{}{}},
				},
				&persistence.CloseExecutionTask{
					TaskData: persistence.TaskData{
						Version:             10,
						VisibilityTimestamp: time.Unix(10, 10),
						TaskID:              10,
					},
				},
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:            domainID,
					WorkflowID:          workflowID,
					RunID:               runID,
					TaskType:            int16(persistence.TransferTaskTypeActivityTask),
					TargetDomainID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47609"),
					TargetWorkflowID:    persistence.TransferTaskTransferTargetWorkflowID,
					ScheduleID:          111,
					Version:             1,
					VisibilityTimestamp: time.Unix(1, 1),
					TaskList:            "tl",
				}).Return(persistence.DataBlob{
					Data:     []byte(`1`),
					Encoding: common.EncodingType("1"),
				}, nil)
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:            domainID,
					WorkflowID:          workflowID,
					RunID:               runID,
					TaskType:            int16(persistence.TransferTaskTypeDecisionTask),
					TargetDomainID:      serialization.MustParseUUID("7be8a310-7d20-483e-a5d2-48659dc47609"),
					TargetWorkflowID:    persistence.TransferTaskTransferTargetWorkflowID,
					ScheduleID:          222,
					Version:             2,
					VisibilityTimestamp: time.Unix(2, 2),
					TaskList:            "tl2",
				}).Return(persistence.DataBlob{
					Data:     []byte(`2`),
					Encoding: common.EncodingType("2"),
				}, nil)
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:                domainID,
					WorkflowID:              workflowID,
					RunID:                   runID,
					TaskType:                int16(persistence.TransferTaskTypeCancelExecution),
					TargetDomainID:          serialization.MustParseUUID("6be8a310-7d20-483e-a5d2-48659dc47609"),
					TargetWorkflowID:        "acd",
					TargetRunID:             serialization.MustParseUUID("3be8a310-7d20-483e-a5d2-48659dc47609"),
					ScheduleID:              333,
					Version:                 3,
					VisibilityTimestamp:     time.Unix(3, 3),
					TargetChildWorkflowOnly: true,
				}).Return(persistence.DataBlob{
					Data:     []byte(`3`),
					Encoding: common.EncodingType("3"),
				}, nil)
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:                domainID,
					WorkflowID:              workflowID,
					RunID:                   runID,
					TaskType:                int16(persistence.TransferTaskTypeSignalExecution),
					TargetDomainID:          serialization.MustParseUUID("5be8a310-7d20-483e-a5d2-48659dc47609"),
					TargetWorkflowID:        "zcd",
					TargetRunID:             serialization.MustParseUUID("4be8a310-7d20-483e-a5d2-48659dc47609"),
					ScheduleID:              555,
					Version:                 5,
					VisibilityTimestamp:     time.Unix(5, 5),
					TargetChildWorkflowOnly: true,
				}).Return(persistence.DataBlob{
					Data:     []byte(`5`),
					Encoding: common.EncodingType("5"),
				}, nil)
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:            domainID,
					WorkflowID:          workflowID,
					RunID:               runID,
					TaskType:            int16(persistence.TransferTaskTypeStartChildExecution),
					TargetDomainID:      serialization.MustParseUUID("2be8a310-7d20-483e-a5d2-48659dc47609"),
					TargetWorkflowID:    "xcd",
					ScheduleID:          777,
					Version:             7,
					VisibilityTimestamp: time.Unix(7, 7),
				}).Return(persistence.DataBlob{
					Data:     []byte(`7`),
					Encoding: common.EncodingType("7"),
				}, nil)
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:            domainID,
					WorkflowID:          workflowID,
					RunID:               runID,
					TaskType:            int16(persistence.TransferTaskTypeRecordChildExecutionCompleted),
					TargetDomainID:      serialization.MustParseUUID("1be8a310-7d20-483e-a5d2-48659dc47609"),
					TargetWorkflowID:    "ddd",
					TargetRunID:         serialization.MustParseUUID("0be8a310-7d20-483e-a5d2-48659dc47609"),
					Version:             8,
					VisibilityTimestamp: time.Unix(8, 8),
				}).Return(persistence.DataBlob{
					Data:     []byte(`8`),
					Encoding: common.EncodingType("8"),
				}, nil)
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:            domainID,
					WorkflowID:          workflowID,
					RunID:               runID,
					TaskType:            int16(persistence.TransferTaskTypeApplyParentClosePolicy),
					TargetDomainID:      domainID,
					TargetDomainIDs:     []serialization.UUID{serialization.MustParseUUID("abe8a310-7d20-483e-a5d2-48659dc47609")},
					TargetWorkflowID:    persistence.TransferTaskTransferTargetWorkflowID,
					Version:             9,
					VisibilityTimestamp: time.Unix(9, 9),
				}).Return(persistence.DataBlob{
					Data:     []byte(`9`),
					Encoding: common.EncodingType("9"),
				}, nil)
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:            domainID,
					WorkflowID:          workflowID,
					RunID:               runID,
					TaskType:            int16(persistence.TransferTaskTypeCloseExecution),
					TargetDomainID:      domainID,
					TargetWorkflowID:    persistence.TransferTaskTransferTargetWorkflowID,
					Version:             10,
					VisibilityTimestamp: time.Unix(10, 10),
				}).Return(persistence.DataBlob{
					Data:     []byte(`10`),
					Encoding: common.EncodingType("10"),
				}, nil)
				mockTx.EXPECT().InsertIntoTransferTasks(gomock.Any(), []sqlplugin.TransferTasksRow{
					{
						ShardID:      shardID,
						TaskID:       1,
						Data:         []byte(`1`),
						DataEncoding: "1",
					},
					{
						ShardID:      shardID,
						TaskID:       2,
						Data:         []byte(`2`),
						DataEncoding: "2",
					},
					{
						ShardID:      shardID,
						TaskID:       3,
						Data:         []byte(`3`),
						DataEncoding: "3",
					},
					{
						ShardID:      shardID,
						TaskID:       5,
						Data:         []byte(`5`),
						DataEncoding: "5",
					},
					{
						ShardID:      shardID,
						TaskID:       7,
						Data:         []byte(`7`),
						DataEncoding: "7",
					},
					{
						ShardID:      shardID,
						TaskID:       8,
						Data:         []byte(`8`),
						DataEncoding: "8",
					},
					{
						ShardID:      shardID,
						TaskID:       9,
						Data:         []byte(`9`),
						DataEncoding: "9",
					},
					{
						ShardID:      shardID,
						TaskID:       10,
						Data:         []byte(`10`),
						DataEncoding: "10",
					},
				}).Return(&sqlResult{rowsAffected: 8}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case",
			tasks: []persistence.Task{
				&persistence.ActivityTask{
					TaskData: persistence.TaskData{
						Version:             1,
						VisibilityTimestamp: time.Unix(1, 1),
						TaskID:              1,
					},
					DomainID:   "8be8a310-7d20-483e-a5d2-48659dc47609",
					TaskList:   "tl",
					ScheduleID: 111,
				},
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().TransferTaskInfoToBlob(&serialization.TransferTaskInfo{
					DomainID:            domainID,
					WorkflowID:          workflowID,
					RunID:               runID,
					TaskType:            int16(persistence.TransferTaskTypeActivityTask),
					TargetDomainID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47609"),
					TargetWorkflowID:    persistence.TransferTaskTransferTargetWorkflowID,
					ScheduleID:          111,
					Version:             1,
					VisibilityTimestamp: time.Unix(1, 1),
					TaskList:            "tl",
				}).Return(persistence.DataBlob{
					Data:     []byte(`1`),
					Encoding: common.EncodingType("1"),
				}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoTransferTasks(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := createTransferTasks(context.Background(), mockTx, tc.tasks, shardID, domainID, workflowID, runID, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestCreateTimerTasks(t *testing.T) {
	shardID := 1
	domainID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602")
	workflowID := "abc"
	runID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603")
	testCases := []struct {
		name      string
		tasks     []persistence.Task
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			tasks: []persistence.Task{
				&persistence.DecisionTimeoutTask{
					TaskData: persistence.TaskData{
						Version:             1,
						VisibilityTimestamp: time.Unix(1, 1),
						TaskID:              1,
					},
					EventID:         1,
					ScheduleAttempt: 1,
					TimeoutType:     1,
				},
				&persistence.ActivityTimeoutTask{
					TaskData: persistence.TaskData{
						Version:             2,
						VisibilityTimestamp: time.Unix(2, 2),
						TaskID:              2,
					},
					EventID:     2,
					Attempt:     2,
					TimeoutType: 2,
				},
				&persistence.UserTimerTask{
					TaskData: persistence.TaskData{
						Version:             3,
						VisibilityTimestamp: time.Unix(3, 3),
						TaskID:              3,
					},
					EventID: 3,
				},
				&persistence.ActivityRetryTimerTask{
					TaskData: persistence.TaskData{
						Version:             4,
						VisibilityTimestamp: time.Unix(4, 4),
						TaskID:              4,
					},
					EventID: 4,
					Attempt: 4,
				},
				&persistence.WorkflowBackoffTimerTask{
					TaskData: persistence.TaskData{
						Version:             5,
						VisibilityTimestamp: time.Unix(5, 5),
						TaskID:              5,
					},
					EventID:     5,
					TimeoutType: 5,
				},
				&persistence.WorkflowTimeoutTask{
					TaskData: persistence.TaskData{
						Version:             6,
						VisibilityTimestamp: time.Unix(6, 6),
						TaskID:              6,
					},
				},
				&persistence.DeleteHistoryEventTask{
					TaskData: persistence.TaskData{
						Version:             7,
						VisibilityTimestamp: time.Unix(7, 7),
						TaskID:              7,
					},
				},
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeDecisionTimeout),
					Version:         1,
					EventID:         1,
					ScheduleAttempt: 1,
					TimeoutType:     common.Int16Ptr(1),
				}).Return(persistence.DataBlob{
					Data:     []byte(`1`),
					Encoding: common.EncodingType("1"),
				}, nil)
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeActivityTimeout),
					Version:         2,
					EventID:         2,
					ScheduleAttempt: 2,
					TimeoutType:     common.Int16Ptr(2),
				}).Return(persistence.DataBlob{
					Data:     []byte(`2`),
					Encoding: common.EncodingType("2"),
				}, nil)
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeUserTimer),
					Version:         3,
					EventID:         3,
					ScheduleAttempt: 0,
				}).Return(persistence.DataBlob{
					Data:     []byte(`3`),
					Encoding: common.EncodingType("3"),
				}, nil)
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeActivityRetryTimer),
					Version:         4,
					EventID:         4,
					ScheduleAttempt: 4,
				}).Return(persistence.DataBlob{
					Data:     []byte(`4`),
					Encoding: common.EncodingType("4"),
				}, nil)
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeWorkflowBackoffTimer),
					Version:         5,
					EventID:         5,
					ScheduleAttempt: 0,
					TimeoutType:     common.Int16Ptr(5),
				}).Return(persistence.DataBlob{
					Data:     []byte(`5`),
					Encoding: common.EncodingType("5"),
				}, nil)
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeWorkflowTimeout),
					Version:         6,
					EventID:         common.EmptyEventID,
					ScheduleAttempt: 0,
				}).Return(persistence.DataBlob{
					Data:     []byte(`6`),
					Encoding: common.EncodingType("6"),
				}, nil)
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeDeleteHistoryEvent),
					Version:         7,
					EventID:         common.EmptyEventID,
					ScheduleAttempt: 0,
				}).Return(persistence.DataBlob{
					Data:     []byte(`7`),
					Encoding: common.EncodingType("7"),
				}, nil)
				mockTx.EXPECT().InsertIntoTimerTasks(gomock.Any(), []sqlplugin.TimerTasksRow{
					{
						ShardID:             shardID,
						TaskID:              1,
						VisibilityTimestamp: time.Unix(1, 1),
						Data:                []byte(`1`),
						DataEncoding:        "1",
					},
					{
						ShardID:             shardID,
						TaskID:              2,
						VisibilityTimestamp: time.Unix(2, 2),
						Data:                []byte(`2`),
						DataEncoding:        "2",
					},
					{
						ShardID:             shardID,
						TaskID:              3,
						VisibilityTimestamp: time.Unix(3, 3),
						Data:                []byte(`3`),
						DataEncoding:        "3",
					},
					{
						ShardID:             shardID,
						TaskID:              4,
						VisibilityTimestamp: time.Unix(4, 4),
						Data:                []byte(`4`),
						DataEncoding:        "4",
					},
					{
						ShardID:             shardID,
						TaskID:              5,
						VisibilityTimestamp: time.Unix(5, 5),
						Data:                []byte(`5`),
						DataEncoding:        "5",
					},
					{
						ShardID:             shardID,
						TaskID:              6,
						VisibilityTimestamp: time.Unix(6, 6),
						Data:                []byte(`6`),
						DataEncoding:        "6",
					},
					{
						ShardID:             shardID,
						TaskID:              7,
						VisibilityTimestamp: time.Unix(7, 7),
						Data:                []byte(`7`),
						DataEncoding:        "7",
					},
				}).Return(&sqlResult{rowsAffected: 7}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case",
			tasks: []persistence.Task{
				&persistence.DecisionTimeoutTask{
					TaskData: persistence.TaskData{
						Version:             1,
						VisibilityTimestamp: time.Unix(1, 1),
						TaskID:              1,
					},
					EventID:         1,
					ScheduleAttempt: 1,
					TimeoutType:     1,
				},
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().TimerTaskInfoToBlob(&serialization.TimerTaskInfo{
					DomainID:        domainID,
					WorkflowID:      workflowID,
					RunID:           runID,
					TaskType:        int16(persistence.TaskTypeDecisionTimeout),
					Version:         1,
					EventID:         1,
					ScheduleAttempt: 1,
					TimeoutType:     common.Int16Ptr(1),
				}).Return(persistence.DataBlob{
					Data:     []byte(`1`),
					Encoding: common.EncodingType("1"),
				}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoTimerTasks(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := createTimerTasks(context.Background(), mockTx, tc.tasks, shardID, domainID, workflowID, runID, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestCreateReplicationTasks(t *testing.T) {
	shardID := 1
	domainID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602")
	workflowID := "abc"
	runID := serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603")
	testCases := []struct {
		name      string
		tasks     []persistence.Task
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			tasks: []persistence.Task{
				&persistence.HistoryReplicationTask{
					TaskData: persistence.TaskData{
						TaskID:              1,
						VisibilityTimestamp: time.Unix(1, 1),
						Version:             1,
					},
					FirstEventID:      1,
					NextEventID:       2,
					BranchToken:       []byte{1},
					NewRunBranchToken: []byte{2},
				},
				&persistence.SyncActivityTask{
					TaskData: persistence.TaskData{
						TaskID:              2,
						VisibilityTimestamp: time.Unix(2, 2),
						Version:             2,
					},
					ScheduledID: 2,
				},
				&persistence.FailoverMarkerTask{
					TaskData: persistence.TaskData{
						TaskID:              3,
						VisibilityTimestamp: time.Unix(3, 3),
						Version:             3,
					},
					DomainID: "ddd",
				},
			},
			mockSetup: func(mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ReplicationTaskInfoToBlob(&serialization.ReplicationTaskInfo{
					DomainID:                domainID,
					WorkflowID:              workflowID,
					RunID:                   runID,
					TaskType:                int16(persistence.ReplicationTaskTypeHistory),
					FirstEventID:            1,
					NextEventID:             2,
					Version:                 1,
					ScheduledID:             common.EmptyEventID,
					EventStoreVersion:       persistence.EventStoreVersion,
					NewRunEventStoreVersion: persistence.EventStoreVersion,
					CreationTimestamp:       time.Unix(1, 1),
					BranchToken:             []byte{1},
					NewRunBranchToken:       []byte{2},
				}).Return(persistence.DataBlob{
					Data:     []byte(`1`),
					Encoding: common.EncodingType("1"),
				}, nil)
				mockParser.EXPECT().ReplicationTaskInfoToBlob(&serialization.ReplicationTaskInfo{
					DomainID:                domainID,
					WorkflowID:              workflowID,
					RunID:                   runID,
					TaskType:                int16(persistence.ReplicationTaskTypeSyncActivity),
					FirstEventID:            common.EmptyEventID,
					NextEventID:             common.EmptyEventID,
					Version:                 2,
					ScheduledID:             2,
					EventStoreVersion:       persistence.EventStoreVersion,
					NewRunEventStoreVersion: persistence.EventStoreVersion,
					CreationTimestamp:       time.Unix(2, 2),
				}).Return(persistence.DataBlob{
					Data:     []byte(`2`),
					Encoding: common.EncodingType("2"),
				}, nil)
				mockParser.EXPECT().ReplicationTaskInfoToBlob(&serialization.ReplicationTaskInfo{
					DomainID:                domainID,
					WorkflowID:              workflowID,
					RunID:                   runID,
					TaskType:                int16(persistence.ReplicationTaskTypeFailoverMarker),
					FirstEventID:            common.EmptyEventID,
					NextEventID:             common.EmptyEventID,
					Version:                 3,
					ScheduledID:             common.EmptyEventID,
					EventStoreVersion:       persistence.EventStoreVersion,
					NewRunEventStoreVersion: persistence.EventStoreVersion,
					CreationTimestamp:       time.Unix(3, 3),
				}).Return(persistence.DataBlob{
					Data:     []byte(`3`),
					Encoding: common.EncodingType("3"),
				}, nil)
				mockTx.EXPECT().InsertIntoReplicationTasks(gomock.Any(), []sqlplugin.ReplicationTasksRow{
					{
						ShardID:      shardID,
						TaskID:       1,
						Data:         []byte(`1`),
						DataEncoding: "1",
					},
					{
						ShardID:      shardID,
						TaskID:       2,
						Data:         []byte(`2`),
						DataEncoding: "2",
					},
					{
						ShardID:      shardID,
						TaskID:       3,
						Data:         []byte(`3`),
						DataEncoding: "3",
					},
				}).Return(&sqlResult{rowsAffected: 3}, nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)

			tc.mockSetup(mockTx, mockParser)

			err := createReplicationTasks(context.Background(), mockTx, tc.tasks, shardID, domainID, workflowID, runID, mockParser)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestLockCurrentExecutionIfExists(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*sqlplugin.MockTx)
		wantErr   bool
		want      *sqlplugin.CurrentExecutionsRow
	}{
		{
			name: "Success case",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutionsJoinExecutions(gomock.Any(), gomock.Any()).Return([]sqlplugin.CurrentExecutionsRow{
					{
						ShardID:    1,
						DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
						WorkflowID: "abc",
						RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
					},
				}, nil)
			},
			wantErr: false,
			want: &sqlplugin.CurrentExecutionsRow{
				ShardID:    1,
				DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
				WorkflowID: "abc",
				RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
			},
		},
		{
			name: "Error case",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().LockCurrentExecutionsJoinExecutions(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Empty result",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutionsJoinExecutions(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
			},
			wantErr: false,
		},
		{
			name: "Multiple rows",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutionsJoinExecutions(gomock.Any(), gomock.Any()).Return([]sqlplugin.CurrentExecutionsRow{
					{
						ShardID:    1,
						DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
						WorkflowID: "abc",
						RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
					},
					{
						ShardID:    1,
						DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
						WorkflowID: "def",
						RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47604"),
					},
				}, nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)

			got, err := lockCurrentExecutionIfExists(context.Background(), mockTx, 1, serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"), "abc")
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Expected result to match")
			}
		})
	}
}

func TestCreateOrUpdateCurrentExecution(t *testing.T) {
	testCases := []struct {
		name       string
		createMode persistence.CreateWorkflowMode
		mockSetup  func(*sqlplugin.MockTx)
		wantErr    bool
	}{
		{
			name:       "Brand new workflow - success",
			createMode: persistence.CreateWorkflowModeBrandNew,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().InsertIntoCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:       "Brand new workflow - error",
			createMode: persistence.CreateWorkflowModeBrandNew,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:       "Update current execution - success",
			createMode: persistence.CreateWorkflowModeWorkflowIDReuse,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().UpdateCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			wantErr: false,
		},
		{
			name:       "Update current execution - error",
			createMode: persistence.CreateWorkflowModeWorkflowIDReuse,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().UpdateCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:       "Update current execution - no rows affected",
			createMode: persistence.CreateWorkflowModeContinueAsNew,
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().UpdateCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: 0}, nil)
			},
			wantErr: true,
		},
		{
			name:       "Zombie workflow - success",
			createMode: persistence.CreateWorkflowModeZombie,
			mockSetup:  func(mockTx *sqlplugin.MockTx) {},
			wantErr:    false,
		},
		{
			name:       "Unknown create mode",
			createMode: persistence.CreateWorkflowMode(100),
			mockSetup:  func(mockTx *sqlplugin.MockTx) {},
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)

			err := createOrUpdateCurrentExecution(
				context.Background(),
				mockTx,
				tc.createMode,
				1,
				serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
				"abc",
				serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
				0,
				1,
				"request-id",
				11,
				12,
			)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestAssertNotCurrentExecution(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*sqlplugin.MockTx)
		wantErr   bool
	}{
		{
			name: "Success case",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlplugin.CurrentExecutionsRow{
					ShardID:    1,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Success case - No rows",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
			},
			wantErr: false,
		},
		{
			name: "Error case - run ID match",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlplugin.CurrentExecutionsRow{
					ShardID:    1,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
				}, nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)

			err := assertNotCurrentExecution(
				context.Background(),
				mockTx,
				1,
				serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
				"abc",
				serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
			)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestAssertRunIDAndUpdateCurrentExecution(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*sqlplugin.MockTx)
		wantErr   bool
	}{
		{
			name: "Success case",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlplugin.CurrentExecutionsRow{
					ShardID:    1,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47604"),
				}, nil)
				mockTx.EXPECT().UpdateCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - update current execution",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlplugin.CurrentExecutionsRow{
					ShardID:    1,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47604"),
				}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - run ID mismatch",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(&sqlplugin.CurrentExecutionsRow{
					ShardID:    1,
					DomainID:   serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
					WorkflowID: "abc",
					RunID:      serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
				}, nil)
			},
			wantErr: true,
		},
		{
			name: "Error case - unknown error",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().LockCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)

			err := assertRunIDAndUpdateCurrentExecution(
				context.Background(),
				mockTx,
				1,
				serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47602"),
				"abc",
				serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47603"),
				serialization.MustParseUUID("8be8a310-7d20-483e-a5d2-48659dc47604"),
				"request-id",
				1,
				11,
				12,
				13,
			)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}
