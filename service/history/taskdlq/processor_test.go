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

func setupProcessor(t *testing.T, ctrl *gomock.Controller) (*ProcessorImpl, *persistence.MockHistoryTaskDLQManager, *MockTaskExecutor) {
	t.Helper()
	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	executor := NewMockTaskExecutor(ctrl)
	proc := NewProcessor(
		1,
		mgr,
		map[int]TaskExecutor{
			persistence.HistoryTaskCategoryIDTransfer: executor,
		},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeEnabled),
		dynamicproperties.GetBoolPropertyFn(true),
		clock.NewMockedTimeSource(),
		testlogger.New(t),
	)
	return proc, mgr, executor
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
	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return(nil, nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestProcessShard_WhenGetAckLevelsFails_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)
	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return(nil, errors.New("db error"))

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "db error")
}

func TestProcessShard_WhenAllTasksSucceed_AdvancesAckLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, executor := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0 := newMockTask(ctrl, 0)
	task1 := newMockTask(ctrl, 1)
	tasks := []persistence.Task{task0, task1}

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{Tasks: tasks}, nil)
	executor.EXPECT().Execute(gomock.Any(), tasks[0]).Return(nil)
	executor.EXPECT().Execute(gomock.Any(), tasks[1]).Return(nil)
	mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(nil)
	mgr.EXPECT().DeleteTasks(gomock.Any(), gomock.Any()).Return(nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestProcessShard_WhenTasksSpanMultiplePages_ProcessesAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, executor := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	page1Token := []byte("token1")
	task0 := newMockTask(ctrl, 0)
	task1 := newMockTask(ctrl, 1)

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task0}, NextPageToken: page1Token}, nil,
	)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task1}}, nil,
	)
	executor.EXPECT().Execute(gomock.Any(), task0).Return(nil)
	executor.EXPECT().Execute(gomock.Any(), task1).Return(nil)
	mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(nil)
	mgr.EXPECT().DeleteTasks(gomock.Any(), gomock.Any()).Return(nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestProcessShard_WhenExecutionFailsMidPage_AdvancesAckLevelToLastSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, executor := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	key0 := persistence.NewImmediateTaskKey(0)
	task0 := persistence.NewMockTask(ctrl)
	task0.EXPECT().GetTaskKey().Return(key0)
	task1 := persistence.NewMockTask(ctrl) // GetTaskKey never called: execution fails before it
	execErr := errors.New("execute failed")

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task0, task1}}, nil,
	)
	executor.EXPECT().Execute(gomock.Any(), task0).Return(nil)
	executor.EXPECT().Execute(gomock.Any(), task1).Return(execErr)
	executor.EXPECT().HandleErr(execErr).Return(execErr)
	mgr.EXPECT().UpdateAckLevel(gomock.Any(), persistence.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   al.ShardID,
		DomainID:                  al.DomainID,
		ClusterAttributeScope:     al.ClusterAttributeScope,
		ClusterAttributeName:      al.ClusterAttributeName,
		TaskCategory:              al.TaskCategory,
		UpdatedInclusiveReadLevel: key0,
	}).Return(nil)
	mgr.EXPECT().DeleteTasks(gomock.Any(), gomock.Any()).Return(nil)

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, execErr)
}

func TestProcessShard_WhenFirstTaskFails_DoesNotAdvanceAckLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, executor := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0 := persistence.NewMockTask(ctrl)
	execErr := errors.New("execute failed")

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task0}}, nil,
	)
	executor.EXPECT().Execute(gomock.Any(), task0).Return(execErr)
	executor.EXPECT().HandleErr(execErr).Return(execErr)

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, execErr)
}

func TestProcessShard_WhenTaskIsAckable_SkipsAndAdvancesPastIt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, executor := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0 := newMockTask(ctrl, 0) // will fail with ackable error
	task1 := newMockTask(ctrl, 1) // succeeds
	ackableErr := errors.New("entity not found")

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task0, task1}}, nil,
	)
	executor.EXPECT().Execute(gomock.Any(), task0).Return(ackableErr)
	executor.EXPECT().HandleErr(ackableErr).Return(nil) // ackable: skip and continue
	executor.EXPECT().Execute(gomock.Any(), task1).Return(nil)
	mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(nil)
	mgr.EXPECT().DeleteTasks(gomock.Any(), gomock.Any()).Return(nil)

	assert.NoError(t, proc.ProcessShard(context.Background()))
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

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{ackLevel1, ackLevel2}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, getTasksErr)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)

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
		GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{
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

	transferExecutor := NewMockTaskExecutor(ctrl)
	timerExecutor := NewMockTaskExecutor(ctrl)
	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	proc := NewProcessor(
		1,
		mgr,
		map[int]TaskExecutor{
			persistence.HistoryTaskCategoryIDTransfer: transferExecutor,
			persistence.HistoryTaskCategoryIDTimer:    timerExecutor,
		},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeEnabled),
		dynamicproperties.GetBoolPropertyFn(true),
		clock.NewMockedTimeSource(),
		testlogger.New(t),
	)

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
		GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{
			ShardID: 1, DomainID: "d", ClusterAttributeScope: "s", ClusterAttributeName: "n",
		}).
		Return([]persistence.HistoryDLQAckLevel{transferAL, timerAL}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)

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

			mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(tc.updateErr)
			if tc.expectDeleteCalled {
				mgr.EXPECT().DeleteTasks(gomock.Any(), gomock.Any()).Return(tc.deleteErr)
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

func TestProcessShard_WhenNoExecutorForTaskType_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, _ := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	al.TaskCategory = persistence.HistoryTaskCategoryTimer // no executor registered for timer

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "no executor registered for task type")
}

func TestProcessShard_WhenExecutionAndAdvanceAckLevelBothFail_ReturnsBothErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	proc, mgr, executor := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0 := newMockTask(ctrl, 0)
	task1 := persistence.NewMockTask(ctrl)
	execErr := errors.New("execute failed")
	updateErr := errors.New("update ack level failed")

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(
		persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task0, task1}}, nil,
	)
	executor.EXPECT().Execute(gomock.Any(), task0).Return(nil)
	executor.EXPECT().Execute(gomock.Any(), task1).Return(execErr)
	executor.EXPECT().HandleErr(execErr).Return(execErr)
	mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(updateErr)

	err := proc.ProcessShard(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, execErr)
	assert.ErrorIs(t, err, updateErr)
}

func TestProcessShard_AndProcessPartition_AreSerializedByMutex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	proc := NewProcessor(
		1,
		mgr,
		map[int]TaskExecutor{},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeEnabled),
		dynamicproperties.GetBoolPropertyFn(true),
		clock.NewMockedTimeSource(),
		testlogger.New(t),
	)

	shardStarted := make(chan struct{})
	shardBlocked := make(chan struct{})
	partitionRan := make(chan struct{})

	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).DoAndReturn(func(ctx context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
		close(shardStarted)
		<-shardBlocked
		return nil, nil
	})
	mgr.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).DoAndReturn(
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
	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).DoAndReturn(func(ctx context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
		select {
		case inGetAckLevels <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}).AnyTimes()

	proc := NewProcessor(
		1,
		mgr,
		map[int]TaskExecutor{},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeEnabled),
		dynamicproperties.GetBoolPropertyFn(true),
		ts,
		testlogger.New(t),
	)

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

	proc, mgr, executor := setupProcessor(t, ctrl)
	al := baseAckLevel(1)
	task0Key := persistence.NewImmediateTaskKey(0)
	task0 := newMockTask(ctrl, 0)

	// First run: task executes, ack level advances, DeleteTasks fails.
	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{Tasks: []persistence.Task{task0}}, nil)
	executor.EXPECT().Execute(gomock.Any(), task0).Return(nil)
	mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(nil)
	mgr.EXPECT().DeleteTasks(gomock.Any(), gomock.Any()).Return(errors.New("delete failed"))

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
	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{ackLevel2}, nil)
	mgr.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(persistence.HistoryDLQGetTasksResponse{}, nil)
	// UpdateAckLevel and DeleteTasks must NOT be called.

	assert.NoError(t, proc.ProcessShard(context.Background()))
}

func TestStartStop_ShouldBeIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := persistence.NewMockHistoryTaskDLQManager(ctrl)
	proc := NewProcessor(
		1,
		mgr,
		map[int]TaskExecutor{},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeEnabled),
		dynamicproperties.GetBoolPropertyFn(true),
		clock.NewMockedTimeSource(),
		testlogger.New(t),
	)

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
	mgr.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).DoAndReturn(func(_ context.Context, _ persistence.HistoryDLQGetAckLevelsRequest) ([]persistence.HistoryDLQAckLevel, error) {
		select {
		case processed <- struct{}{}:
		default:
		}
		return nil, nil
	}).AnyTimes()

	proc := NewProcessor(
		1,
		mgr,
		map[int]TaskExecutor{},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeEnabled),
		dynamicproperties.GetBoolPropertyFn(true),
		ts,
		testlogger.New(t),
	)

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
	store.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).Times(0)
	proc := NewProcessor(
		1,
		store,
		map[int]TaskExecutor{},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeEnabled),
		dynamicproperties.GetBoolPropertyFn(false),
		ts,
		testlogger.New(t),
	)

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
	executor := NewMockTaskExecutor(ctrl)
	proc := NewProcessor(
		1,
		store,
		map[int]TaskExecutor{
			persistence.HistoryTaskCategoryIDTransfer: executor,
		},
		10,
		dynamicproperties.GetDurationPropertyFnFilteredByShardID(defaultTestProcessingInterval),
		dynamicproperties.GetStringPropertyFnFilteredByDomain(constants.HistoryTaskDLQModeDisabled),
		dynamicproperties.GetBoolPropertyFn(true),
		clock.NewMockedTimeSource(),
		testlogger.New(t),
	)

	al := baseAckLevel(1)
	store.EXPECT().GetAckLevels(gomock.Any(), persistence.HistoryDLQGetAckLevelsRequest{ShardID: 1}).Return([]persistence.HistoryDLQAckLevel{al}, nil)
	store.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Times(0)
	executor.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

	assert.NoError(t, proc.ProcessShard(context.Background()))
}
