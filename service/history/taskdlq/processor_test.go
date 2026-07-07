// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package taskdlq

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/constants"
)

const (
	defaultTestProcessingInterval = 15 * time.Second
)

// newMockTask creates a mock persistence.Task whose GetTaskKey returns an immediate key for taskID.
func newMockTask(ctrl *gomock.Controller, taskID int64) *persistence.MockTask {
	t := persistence.NewMockTask(ctrl)
	t.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(taskID)).AnyTimes()
	return t
}

type newProcessorParams struct {
	ShardID           int
	Manager           persistence.HistoryTaskDLQManager
	Reinjector        TaskReinjector
	DomainMode        string
	ProcessingEnabled bool
	TimeSource        clock.TimeSource
}

// newProcessor builds a ProcessorImpl with the given dependencies and sensible test defaults.
func newProcessor(
	t *testing.T,
	params newProcessorParams,
) *ProcessorImpl {
	t.Helper()
	return NewProcessor(ProcessorParams{
		ShardID:       1,
		Manager:       params.Manager,
		Reinjector:    params.Reinjector,
		PageSize:      10,
		Interval:      dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		DomainMode:    dynamicproperties.GetStringPropertyFnFilteredByDomain(params.DomainMode),
		Enabled:       dynamicproperties.GetBoolPropertyFn(params.ProcessingEnabled),
		TimeSource:    params.TimeSource,
		MetricsClient: metrics.NewNoopMetricsClient(),
		Logger:        testlogger.New(t),
	})
}

func setupProcessor(t *testing.T, ctrl *gomock.Controller) (*ProcessorImpl, *persistence.MockHistoryTaskDLQManager, *MockTaskReinjector) {
	t.Helper()
	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	reinjector := NewMockTaskReinjector(ctrl)
	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        reinjector,
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        clock.NewMockedTimeSource(),
	})
	return proc, mgr, reinjector
}

func baseAckLevel(shardID int) persistence.HistoryDLQAckLevel {
	return persistence.HistoryDLQAckLevel{
		ShardID:               shardID,
		DomainID:              "test-domain",
		ClusterAttributeScope: "scope",
		ClusterAttributeName:  "name",
		TaskCategory:          persistence.HistoryTaskCategoryTransfer,
		AckLevelVisibilityTS:  time.Unix(0, 0).UTC(),
		AckLevelTaskID:        -1,
	}
}

func TestProcessShard_WhenNoAckLevels_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return(nil, nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

// TestProcessShard_WhenCategoryUnsupported_SkipsWithoutReinjecting verifies that a stray
// replication ack level is skipped rather than paged and reinjected. Reinjection only supports
// transfer and timer tasks; attempting to reinject replication would fail at the store, leaving
// the ack level unadvanced and retried forever.
func TestProcessShard_WhenCategoryUnsupported_SkipsWithoutReinjecting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	al.TaskCategory = persistence.HistoryTaskCategoryReplication

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	// No GetHistoryDLQTasks / ReinjectHistoryTasks / UpdateHistoryDLQAckLevel expectations:
	// the partition must be skipped entirely.

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestProcessShard_WhenGetAckLevelsFails_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return(nil, errors.New("db error"))

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "db error")
}

func TestProcessShard_WhenPageSucceeds_AdvancesAckLevelToLastTaskKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, reinjector := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0 := newMockTask(ctrl, 0)
	task1 := newMockTask(ctrl, 1)
	tasks := []persistence.Task{task0, task1}

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{Tasks: tasks}, nil)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), tasks).Return(nil)
	mgr.EXPECT().UpdateHistoryDLQAckLevel(gomock.Any(), persistence.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   al.ShardID,
		DomainID:                  al.DomainID,
		ClusterAttributeScope:     al.ClusterAttributeScope,
		ClusterAttributeName:      al.ClusterAttributeName,
		TaskCategory:              al.TaskCategory,
		UpdatedInclusiveReadLevel: persistence.NewImmediateTaskKey(1),
	}).Return(nil)
	mgr.EXPECT().DeleteHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

// TestProcessShard_AdvancesUsingOriginalKey_WhenReinjectionMutatesTaskID guards against
// reading the ack-level cursor from a task whose ID was rewritten by reinjection.
// ReinjectHistoryTasks allocates fresh shard-global IDs and calls SetTaskID in place; if
// the processor captured GetTaskKey() after reinjection it would advance the ack level
// (and delete) using the huge fresh ID instead of the original DLQ row position.
func TestProcessShard_AdvancesUsingOriginalKey_WhenReinjectionMutatesTaskID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, reinjector := setupProcessor(t, ctrl)
	al := baseAckLevel(1)

	const originalID = int64(7)
	const freshID = int64(1_000_000) // simulates a monotonically increasing shard-global ID
	// Real task type whose GetTaskKey() reads the same mutable TaskID that SetTaskID writes.
	task := &persistence.ActivityTask{TaskData: persistence.TaskData{TaskID: originalID}}
	tasks := []persistence.Task{task}

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{Tasks: tasks}, nil)
	// Reinjection rewrites the task ID in place, as the real shard implementation does.
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), tasks).DoAndReturn(
		func(_ context.Context, ts []persistence.Task) error {
			ts[len(ts)-1].SetTaskID(freshID)
			return nil
		},
	)
	// Ack level must advance to the original key, not the mutated one.
	mgr.EXPECT().UpdateHistoryDLQAckLevel(gomock.Any(), persistence.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   al.ShardID,
		DomainID:                  al.DomainID,
		ClusterAttributeScope:     al.ClusterAttributeScope,
		ClusterAttributeName:      al.ClusterAttributeName,
		TaskCategory:              al.TaskCategory,
		UpdatedInclusiveReadLevel: persistence.NewImmediateTaskKey(originalID),
	}).Return(nil)
	// Delete must bound on the original key, not delete the whole partition via the huge fresh ID.
	mgr.EXPECT().DeleteHistoryDLQTasks(gomock.Any(), persistence.HistoryDLQDeleteTasksRequest{
		ShardID:               al.ShardID,
		DomainID:              al.DomainID,
		ClusterAttributeScope: al.ClusterAttributeScope,
		ClusterAttributeName:  al.ClusterAttributeName,
		TaskCategory:          al.TaskCategory,
		ExclusiveMaxTaskKey:   persistence.NewImmediateTaskKey(originalID).Next(),
	}).Return(nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestProcessShard_WhenTasksSpanMultiplePages_ReinjectsEachPageAndAdvances(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, reinjector := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	page1Token := []byte("token1")
	task0 := newMockTask(ctrl, 0)
	task1 := newMockTask(ctrl, 1)
	page0 := []persistence.Task{task0}
	page1 := []persistence.Task{task1}

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: page0, NextPageToken: page1Token}, nil,
	)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: page1}, nil,
	)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), page0).Return(nil)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), page1).Return(nil)
	mgr.EXPECT().UpdateHistoryDLQAckLevel(gomock.Any(), gomock.Any()).Return(nil)
	mgr.EXPECT().DeleteHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestProcessShard_WhenPageReinjectFails_AdvancesToLastGoodPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, reinjector := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	page1Token := []byte("token1")
	task0 := newMockTask(ctrl, 0)
	task1 := newMockTask(ctrl, 1)
	page0 := []persistence.Task{task0}
	page1 := []persistence.Task{task1}
	reinjectErr := errors.New("reinject failed")

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: page0, NextPageToken: page1Token}, nil,
	)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: page1}, nil,
	)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), page0).Return(nil)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), page1).Return(reinjectErr)
	// Ack level advances only to the last task of the last successfully re-injected page (task0).
	mgr.EXPECT().UpdateHistoryDLQAckLevel(gomock.Any(), persistence.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   al.ShardID,
		DomainID:                  al.DomainID,
		ClusterAttributeScope:     al.ClusterAttributeScope,
		ClusterAttributeName:      al.ClusterAttributeName,
		TaskCategory:              al.TaskCategory,
		UpdatedInclusiveReadLevel: persistence.NewImmediateTaskKey(0),
	}).Return(nil)
	mgr.EXPECT().DeleteHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(nil)

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, reinjectErr)
}

func TestProcessShard_WhenFirstPageReinjectFails_DoesNotAdvanceAckLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, reinjector := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0 := newMockTask(ctrl, 0) // key captured before reinjection, but ack level must not advance
	page0 := []persistence.Task{task0}
	reinjectErr := errors.New("reinject failed")

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: page0}, nil,
	)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), page0).Return(reinjectErr)
	// UpdateHistoryDLQAckLevel / DeleteHistoryDLQTasks must NOT be called.

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, reinjectErr)
}

func TestProcessShard_WhenOnePartitionFails_ReturnsErrorButProcessesRemainingPartitions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)
	ackLevel1 := baseAckLevel(1)
	ackLevel2 := persistence.HistoryDLQAckLevel{
		ShardID:               1,
		DomainID:              "other-domain",
		ClusterAttributeScope: "scope",
		ClusterAttributeName:  "name",
		TaskCategory:          persistence.HistoryTaskCategoryTransfer,
		AckLevelVisibilityTS:  time.Unix(0, 0).UTC(),
		AckLevelTaskID:        -1,
	}
	getTasksErr := errors.New("partition error")

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{ackLevel1, ackLevel2}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, getTasksErr)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, getTasksErr)
}

func TestProcessPartition_WhenGetAckLevelsFails_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)
	storeErr := errors.New("partition error")
	mgr.EXPECT().
		GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{
			ShardID: 1, DomainID: "d", ClusterAttributeScope: "s", ClusterAttributeName: "n",
		}).
		Return(nil, storeErr)

	err := proc.ProcessPartition(context.Background(), "d", "s", "n")
	require.Error(t, err)
	assert.ErrorContains(t, err, "partition error")
}

func TestProcessPartition_WhenMultipleTaskTypes_ProcessesAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)

	transferAL := persistence.HistoryDLQAckLevel{
		ShardID: 1, DomainID: "d", ClusterAttributeScope: "s", ClusterAttributeName: "n",
		TaskCategory:         persistence.HistoryTaskCategoryTransfer,
		AckLevelVisibilityTS: time.Unix(0, 0).UTC(), AckLevelTaskID: -1,
	}
	timerAL := persistence.HistoryDLQAckLevel{
		ShardID: 1, DomainID: "d", ClusterAttributeScope: "s", ClusterAttributeName: "n",
		TaskCategory:         persistence.HistoryTaskCategoryTimer,
		AckLevelVisibilityTS: time.Unix(0, 0).UTC(), AckLevelTaskID: -1,
	}

	mgr.EXPECT().
		GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{
			ShardID: 1, DomainID: "d", ClusterAttributeScope: "s", ClusterAttributeName: "n",
		}).
		Return([]persistence.HistoryDLQAckLevel{transferAL, timerAL}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)

	assert.NoError(t, proc.ProcessPartition(context.Background(), "d", "s", "n"))
}

func TestAdvanceAckLevel(t *testing.T) {
	tests := []struct {
		name               string
		updateErr          error
		deleteErr          error
		expectDeleteCalled bool
		expectErr          bool
	}{
		{
			name:               "when UpdateAckLevel fails, returns error without calling DeleteTasks",
			updateErr:          errors.New("update failed"),
			expectDeleteCalled: false,
			expectErr:          true,
		},
		{
			name:               "when DeleteTasks fails, logs and returns nil",
			expectDeleteCalled: true,
			deleteErr:          errors.New("delete failed"),
			expectErr:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			proc, mgr, _ := setupProcessor(t, ctrl)
			al := baseAckLevel(1)
			newKey := persistence.NewImmediateTaskKey(5)

			mgr.EXPECT().UpdateHistoryDLQAckLevel(gomock.Any(), gomock.Any()).Return(tc.updateErr)
			if tc.expectDeleteCalled {
				mgr.EXPECT().DeleteHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(tc.deleteErr)
			}

			err := proc.advanceAckLevel(context.Background(), al, newKey)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessShard_WhenReinjectAndAdvanceAckLevelBothFail_ReturnsBothErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, reinjector := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	page1Token := []byte("token1")
	task0 := newMockTask(ctrl, 0)
	task1 := newMockTask(ctrl, 1)
	page0 := []persistence.Task{task0}
	page1 := []persistence.Task{task1}
	reinjectErr := errors.New("reinject failed")
	updateErr := errors.New("update ack level failed")

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: page0, NextPageToken: page1Token}, nil,
	)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: page1}, nil,
	)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), page0).Return(nil)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), page1).Return(reinjectErr)
	mgr.EXPECT().UpdateHistoryDLQAckLevel(gomock.Any(), gomock.Any()).Return(updateErr)

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, reinjectErr)
	assert.ErrorIs(t, err, updateErr)
}

func TestProcessShard_AndProcessPartition_AreSerializedByMutex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        clock.NewMockedTimeSource(),
	})

	shardStarted := make(chan struct{})
	shardBlocked := make(chan struct{})
	partitionRan := make(chan struct{})

	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).DoAndReturn(func(ctx context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
		close(shardStarted)
		<-shardBlocked
		return nil, nil
	})
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
			close(partitionRan)
			return nil, nil
		},
	)

	shardDone := make(chan error, 1)
	go func() { shardDone <- proc.ProcessShard(context.Background()) }()

	<-shardStarted

	partitionDone := make(chan error, 1)
	go func() { partitionDone <- proc.ProcessPartition(context.Background(), "d", "s", "n") }()

	// ProcessPartition must not run while ProcessShard holds the mutex.
	select {
	case <-partitionRan:
		t.Fatal("ProcessPartition ran while ProcessShard held the mutex")
	case <-time.After(10 * time.Millisecond):
	}

	close(shardBlocked)
	require.NoError(t, <-shardDone)

	select {
	case err := <-partitionDone:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("ProcessPartition did not run after ProcessShard released the mutex")
	}
}

func TestStop_WhenStoreRespectsContextCancellation_ReturnsPromptly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ts := clock.NewMockedTimeSource()
	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)

	inGetAckLevels := make(chan struct{}, 1)
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).DoAndReturn(func(ctx context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
		select {
		case inGetAckLevels <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}).AnyTimes()

	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        ts,
	})

	proc.Start()

	ts.BlockUntil(1)
	ts.Advance(defaultTestProcessingInterval)

	select {
	case <-inGetAckLevels:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for GetAckLevels to be called")
	}

	stopDone := make(chan struct{})
	go func() {
		proc.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return promptly after context cancellation")
	}
}

// Documents a known limitation: if DeleteTasks fails and no new tasks arrive,
// the orphaned rows will not be cleaned up until new tasks cause a subsequent
// DeleteTasks call whose range covers the orphaned keys.
func TestProcessShard_WhenDeleteTasksFailsAndDLQBecomesEmpty_OrphanedRowsNotCleaned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, reinjector := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0Key := persistence.NewImmediateTaskKey(0)
	task0 := newMockTask(ctrl, 0)

	// First run: page re-injects, ack level advances, DeleteTasks fails.
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task0}}, nil)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), []persistence.Task{task0}).Return(nil)
	mgr.EXPECT().UpdateHistoryDLQAckLevel(gomock.Any(), gomock.Any()).Return(nil)
	mgr.EXPECT().DeleteHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(errors.New("delete failed"))

	assert.NoError(t, proc.ProcessShard(context.Background()))

	// Second run: ack level is now at task0's key; DLQ is empty beyond that point.
	ackLevel2 := persistence.HistoryDLQAckLevel{
		ShardID:               al.ShardID,
		DomainID:              al.DomainID,
		ClusterAttributeScope: al.ClusterAttributeScope,
		ClusterAttributeName:  al.ClusterAttributeName,
		TaskCategory:          al.TaskCategory,
		AckLevelVisibilityTS:  task0Key.GetScheduledTime(),
		AckLevelTaskID:        task0Key.GetTaskID(),
	}
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{ackLevel2}, nil)
	mgr.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)
	// UpdateAckLevel and DeleteTasks must NOT be called.

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestStartStop_ShouldBeIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        clock.NewMockedTimeSource(),
	})

	proc.Start()
	proc.Start() // second call must be a no-op
	proc.Stop()
	proc.Stop() // second call must be a no-op
}

func TestStart_ShouldCallProcessShardOnInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ts := clock.NewMockedTimeSource()
	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	processed := make(chan struct{}, 1)
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).DoAndReturn(func(_ context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
		select {
		case processed <- struct{}{}:
		default:
		}
		return nil, nil
	}).AnyTimes()

	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        ts,
	})

	proc.Start()
	defer proc.Stop()

	ts.BlockUntil(1)
	ts.Advance(defaultTestProcessingInterval)

	select {
	case <-processed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ProcessShard to be called by the background loop")
	}
}

func TestStart_WhenNotEnabled_SkipsProcessingButContinuesLoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ts := clock.NewMockedTimeSource()
	store := persistence.NewMockHistoryTaskDLQManager(ctrl)
	store.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), gomock.Any()).Times(0)
	proc := newProcessor(t, newProcessorParams{
		Manager:           store,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: false,
		TimeSource:        ts,
	})

	proc.Start()
	defer proc.Stop()

	// The loop always starts; wait for the first timer to be registered.
	ts.BlockUntil(1)
	// Advance past the interval — enabled() returns false, so GetAckLevels must not be called.
	ts.Advance(defaultTestProcessingInterval)
	// Wait for the timer to be reset, confirming the loop ran and continued.
	ts.BlockUntil(1)
	// ctrl.Finish() verifies GetAckLevels was called 0 times.
}

func TestProcessShard_WhenDomainNotEnabled_SkipsProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := persistence.NewMockHistoryTaskDLQManager(ctrl)
	reinjector := NewMockTaskReinjector(ctrl)
	proc := newProcessor(t, newProcessorParams{
		Manager:           store,
		Reinjector:        reinjector,
		DomainMode:        constants.HistoryTaskDLQModeDisabled,
		ProcessingEnabled: true,
		TimeSource:        clock.NewMockedTimeSource(),
	})

	al := baseAckLevel(1)
	store.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	store.EXPECT().GetHistoryDLQTasks(gomock.Any(), gomock.Any()).Times(0)
	reinjector.EXPECT().ReinjectHistoryTasks(gomock.Any(), gomock.Any()).Times(0)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestFailoverPartitions_DispatchesToProcessPartition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	processed := make(chan persistence.HistoryDLQGetAckLevelsRequest, 1)
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
			select {
			case processed <- req:
			default:
			}
			return nil, nil
		}).AnyTimes()

	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        clock.NewMockedTimeSource(),
	})
	proc.Start()
	defer proc.Stop()

	proc.FailoverPartitions([]Partition{{
		DomainID:              "test-domain",
		ClusterAttributeScope: "scope",
		ClusterAttributeName:  "name",
	}})

	select {
	case req := <-processed:
		// The partition keys must be passed through verbatim so the store issues a
		// by-cluster-attribute query for exactly this partition.
		assert.Equal(t, persistence.HistoryDLQGetAckLevelsRequest{
			ShardID:               1,
			DomainID:              "test-domain",
			ClusterAttributeScope: "scope",
			ClusterAttributeName:  "name",
		}, req)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for FailoverPartitions to trigger ProcessPartition")
	}
}

func TestFailoverPartitions_DedupsAndNeverBlocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Not started: nothing drains the pending set, so we can inspect it directly and confirm
	// FailoverPartitions never blocks no matter how many partitions are queued.
	proc := newProcessor(t, newProcessorParams{
		Manager:           persistence.NewMockHistoryTaskDLQManager(ctrl),
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        clock.NewMockedTimeSource(),
	})

	dup := Partition{DomainID: "test-domain", ClusterAttributeScope: "scope", ClusterAttributeName: "name"}
	other := Partition{DomainID: "other-domain"}
	partitions := make([]Partition, 0, 2000)
	for i := 0; i < 1000; i++ {
		partitions = append(partitions, dup, other) // repeatedly the same two partitions
	}

	done := make(chan struct{})
	go func() {
		proc.FailoverPartitions(partitions)
		proc.FailoverPartitions(partitions) // second call while a signal is already pending
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("FailoverPartitions blocked")
	}

	// Deduped down to the two distinct partitions regardless of how many were queued.
	proc.failoverMu.Lock()
	assert.Len(t, proc.pendingFailover, 2)
	assert.Contains(t, proc.pendingFailover, dup.String())
	assert.Contains(t, proc.pendingFailover, other.String())
	proc.failoverMu.Unlock()

	// The wake-up signal is coalesced to a single pending token.
	assert.Equal(t, 1, len(proc.failoverSignal))
}

// TestFailoverPartitions_PreemptsInProgressSweep verifies the Q2 requirement: a failover
// interrupts an in-progress periodic ProcessShard sweep and reprocesses the failed-over
// partition first.
func TestFailoverPartitions_PreemptsInProgressSweep(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ts := clock.NewMockedTimeSource()
	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)

	// Buffered so the restarted sweep after preemption can send again without a second reader.
	sweepStarted := make(chan struct{}, 10)
	partitionRan := make(chan persistence.HistoryDLQGetAckLevelsRequest, 1)

	// Shard-level sweep query: blocks until its context is canceled by the preemption.
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).DoAndReturn(
		func(ctx context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
			sweepStarted <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		}).AnyTimes()
	// Partition-level failover query: must run once the sweep has been preempted.
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{
		ShardID: 1, DomainID: "d", ClusterAttributeScope: "s", ClusterAttributeName: "n",
	}).DoAndReturn(
		func(_ context.Context, req persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
			select {
			case partitionRan <- req:
			default:
			}
			return nil, nil
		}).AnyTimes()

	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: true,
		TimeSource:        ts,
	})
	proc.Start()
	defer proc.Stop()

	// Kick off a periodic sweep and wait for it to block inside the shard-level query.
	ts.BlockUntil(1)
	ts.Advance(defaultTestProcessingInterval)
	select {
	case <-sweepStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("periodic sweep did not start")
	}

	// Fire a failover; it must cancel the in-flight sweep and process the partition first.
	proc.FailoverPartitions([]Partition{{DomainID: "d", ClusterAttributeScope: "s", ClusterAttributeName: "n"}})

	select {
	case req := <-partitionRan:
		assert.Equal(t, "d", req.DomainID)
		assert.Equal(t, "s", req.ClusterAttributeScope)
		assert.Equal(t, "n", req.ClusterAttributeName)
	case <-time.After(5 * time.Second):
		t.Fatal("failover partition was not processed after preempting the sweep")
	}
}

func TestFailoverPartitions_WhenNotEnabled_DoesNotProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	called := make(chan struct{}, 1)
	mgr.EXPECT().GetHistoryDLQAckLevels(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
			select {
			case called <- struct{}{}:
			default:
			}
			return nil, nil
		}).AnyTimes()

	proc := newProcessor(t, newProcessorParams{
		Manager:           mgr,
		Reinjector:        NewMockTaskReinjector(ctrl),
		DomainMode:        constants.HistoryTaskDLQModeEnabled,
		ProcessingEnabled: false,
		TimeSource:        clock.NewMockedTimeSource(),
	})
	proc.Start()
	defer proc.Stop()

	proc.FailoverPartitions([]Partition{{DomainID: "test-domain"}})

	// enabled() is false, so the loop drains the request but must not process it.
	select {
	case <-called:
		t.Fatal("ProcessPartition ran while the processor was disabled")
	case <-time.After(200 * time.Millisecond):
	}
}
