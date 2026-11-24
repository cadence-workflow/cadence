// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package shard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
)

func TestIsOperationPossiblySuccessfulError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"WorkflowExecutionAlreadyStartedError (types)", &types.WorkflowExecutionAlreadyStartedError{}, false},
		{"WorkflowExecutionAlreadyStartedError (persistence)", &persistence.WorkflowExecutionAlreadyStartedError{}, false},
		{"CurrentWorkflowConditionFailedError", &persistence.CurrentWorkflowConditionFailedError{}, false},
		{"ConditionFailedError", &persistence.ConditionFailedError{}, false},
		{"ServiceBusyError", &types.ServiceBusyError{}, false},
		{"LimitExceededError", &types.LimitExceededError{}, false},
		{"ShardOwnershipLostError", &persistence.ShardOwnershipLostError{}, false},
		// DuplicateRequestError is explicitly false in the shard layer (unlike the execution layer
		// where it falls through to the common base and returns true).
		{"DuplicateRequestError", &persistence.DuplicateRequestError{}, false},
		{"TimeoutError", &persistence.TimeoutError{}, true},
		{"context.DeadlineExceeded", context.DeadlineExceeded, true},
		{"generic error", assert.AnError, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isOperationPossiblySuccessfulError(tc.err))
		})
	}
}

// toInt64s normalizes a task-ID slice recovered from an observed log entry, whose ContextMap
// reconstructs the logged []int64 as []interface{}. Returns nil for absent/empty values so the
// "no IDs of this category" case compares equal to a nil expectation.
func toInt64s(v interface{}) []int64 {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []int64
	for _, e := range arr {
		out = append(out, e.(int64))
	}
	return out
}

func transferTask(id int64) persistence.Task {
	return &persistence.ActivityTask{TaskData: persistence.TaskData{TaskID: id}}
}

func timerTask(id int64) persistence.Task {
	return &persistence.UserTimerTask{TaskData: persistence.TaskData{TaskID: id}}
}

func replicationTask(id int64) persistence.Task {
	return &persistence.HistoryReplicationTask{TaskData: persistence.TaskData{TaskID: id}}
}

func TestLogNotifyTaskDroppedOnPersistenceError(t *testing.T) {
	tests := []struct {
		name            string
		timerMode       string
		transferMode    string
		sources         []map[persistence.HistoryTaskCategory][]persistence.Task
		wantLogged      bool
		wantTimerIDs    []int64
		wantTransferIDs []int64
	}{
		{
			name:         "neither cache in shadow: nothing logged",
			timerMode:    "enabled",
			transferMode: "enabled",
			sources: []map[persistence.HistoryTaskCategory][]persistence.Task{{
				persistence.HistoryTaskCategoryTimer:    {timerTask(1)},
				persistence.HistoryTaskCategoryTransfer: {transferTask(2)},
			}},
			wantLogged: false,
		},
		{
			name:         "timer shadow only: only timer IDs logged",
			timerMode:    "shadow",
			transferMode: "enabled",
			sources: []map[persistence.HistoryTaskCategory][]persistence.Task{{
				persistence.HistoryTaskCategoryTimer:    {timerTask(1), timerTask(2)},
				persistence.HistoryTaskCategoryTransfer: {transferTask(3)},
			}},
			wantLogged:      true,
			wantTimerIDs:    []int64{1, 2},
			wantTransferIDs: nil,
		},
		{
			name:         "transfer shadow only: only transfer IDs logged",
			timerMode:    "disabled",
			transferMode: "shadow",
			sources: []map[persistence.HistoryTaskCategory][]persistence.Task{{
				persistence.HistoryTaskCategoryTimer:    {timerTask(1)},
				persistence.HistoryTaskCategoryTransfer: {transferTask(3), transferTask(4)},
			}},
			wantLogged:      true,
			wantTimerIDs:    nil,
			wantTransferIDs: []int64{3, 4},
		},
		{
			name:         "both shadow: both categories logged across multiple sources",
			timerMode:    "shadow",
			transferMode: "shadow",
			sources: []map[persistence.HistoryTaskCategory][]persistence.Task{
				{
					persistence.HistoryTaskCategoryTimer:    {timerTask(1)},
					persistence.HistoryTaskCategoryTransfer: {transferTask(3)},
				},
				{
					persistence.HistoryTaskCategoryTimer:    {timerTask(2)},
					persistence.HistoryTaskCategoryTransfer: {transferTask(4)},
				},
			},
			wantLogged:      true,
			wantTimerIDs:    []int64{1, 2},
			wantTransferIDs: []int64{3, 4},
		},
		{
			name:         "both shadow but batch has only replication tasks: nothing logged",
			timerMode:    "shadow",
			transferMode: "shadow",
			sources: []map[persistence.HistoryTaskCategory][]persistence.Task{{
				persistence.HistoryTaskCategoryReplication: {replicationTask(5)},
			}},
			wantLogged: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger, obs := testlogger.NewObserved(t)
			n := &taskNotifier{
				shardID: 1,
				logger:  logger,
				config: &config.Config{
					TimerProcessorCachedQueueReaderMode:    func(int) string { return tc.timerMode },
					TransferProcessorCachedQueueReaderMode: func(int) string { return tc.transferMode },
				},
			}

			n.logNotifyTaskDroppedOnPersistenceError(assert.AnError, tc.sources...)

			entries := obs.FilterMessage("notify tasks dropped due to persistence error").All()
			if !tc.wantLogged {
				assert.Empty(t, entries)
				return
			}
			assert.Len(t, entries, 1)
			ctxMap := entries[0].ContextMap()
			assert.Equal(t, tc.wantTimerIDs, toInt64s(ctxMap["droppedTimerTaskIDs"]))
			assert.Equal(t, tc.wantTransferIDs, toInt64s(ctxMap["droppedTransferTaskIDs"]))
		})
	}
}

func TestNotifyTasks_RoutesByCategory(t *testing.T) {
	visibilityTask := &persistence.RecordWorkflowStartedTask{TaskData: persistence.TaskData{TaskID: 3}}

	tests := map[string]struct {
		tasksByCategory map[persistence.HistoryTaskCategory][]persistence.Task
		expect          func(mockEngine *engine.MockEngine)
	}{
		"transfer tasks notify the transfer processor": {
			tasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
				persistence.HistoryTaskCategoryTransfer: {transferTask(1)},
			},
			expect: func(mockEngine *engine.MockEngine) {
				mockEngine.EXPECT().NotifyNewTransferTasks(gomock.Any()).Do(func(info *hcommon.NotifyTaskInfo) {
					assert.Equal(t, []persistence.Task{transferTask(1)}, info.Tasks)
				}).Times(1)
			},
		},
		"timer tasks notify the timer processor": {
			tasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
				persistence.HistoryTaskCategoryTimer: {timerTask(2)},
			},
			expect: func(mockEngine *engine.MockEngine) {
				mockEngine.EXPECT().NotifyNewTimerTasks(gomock.Any()).Do(func(info *hcommon.NotifyTaskInfo) {
					assert.Equal(t, []persistence.Task{timerTask(2)}, info.Tasks)
				}).Times(1)
			},
		},
		"visibility tasks notify the visibility processor": {
			tasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
				persistence.HistoryTaskCategoryVisibility: {visibilityTask},
			},
			expect: func(mockEngine *engine.MockEngine) {
				mockEngine.EXPECT().NotifyNewVisibilityTasks(gomock.Any()).Do(func(info *hcommon.NotifyTaskInfo) {
					assert.Equal(t, []persistence.Task{visibilityTask}, info.Tasks)
					assert.True(t, info.PersistenceError)
				}).Times(1)
			},
		},
		"empty categories notify nothing": {
			tasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
				persistence.HistoryTaskCategoryVisibility: {},
			},
			expect: func(mockEngine *engine.MockEngine) {},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockEngine := engine.NewMockEngine(ctrl)
			tc.expect(mockEngine)
			n := newTaskNotifier(
				0,
				config.NewForTest(),
				testlogger.New(t),
				func() engine.Engine { return mockEngine },
				func([]persistence.Task) map[string]time.Time { return nil },
			)

			n.notifyTasks(&persistence.WorkflowExecutionInfo{}, tc.tasksByCategory, true)
		})
	}
}
