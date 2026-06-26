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

package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/worker/archiver"
)

func TestTimerQueueProcessor_StartStop_DrainCompletesTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ackTime := time.Now().Truncate(time.Millisecond)
	clusterAckTime := ackTime.Add(time.Hour)

	mockShard := shard.NewTestContext(
		t,
		ctrl,
		&persistence.ShardInfo{
			ShardID:       10,
			RangeID:       1,
			TimerAckLevel: ackTime,
			// per-cluster ack levels are ahead of the processor's initial ackLevel
			// (TimerAckLevel), so the graceful-shutdown drain deterministically advances
			// the ack level once.
			ClusterTimerAckLevel: map[string]time.Time{
				constants.TestClusterMetadata.GetCurrentClusterName(): clusterAckTime,
				"standby": clusterAckTime,
			},
		},
		config.NewForTest(),
	)

	processor := NewTimerQueueProcessor(
		mockShard,
		task.NewMockProcessor(ctrl),
		execution.NewCache(mockShard),
		archiver.NewMockClient(ctrl),
		invariant.NewMockInvariant(ctrl),
	).(*timerQueueProcessor)

	mockShard.Resource.ExecutionMgr.On("RangeCompleteHistoryTask", mock.Anything, mock.Anything).
		Return(&persistence.RangeCompleteHistoryTaskResponse{}, nil).Once()
	mockShard.GetShardManager().(*mocks.ShardManager).On("UpdateShard", mock.Anything, mock.Anything).
		Return(nil).Once()

	assert.Equal(t, common.DaemonStatusInitialized, processor.status)
	processor.Start()
	assert.Equal(t, common.DaemonStatusStarted, processor.status)

	processor.Stop()
	assert.Equal(t, common.DaemonStatusStopped, processor.status)

	mockShard.Resource.ExecutionMgr.AssertExpectations(t)
	mockShard.Resource.ShardMgr.AssertExpectations(t)
}

func setupTimerQueueProcessor(t *testing.T) (*gomock.Controller, *timerQueueProcessor) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t,
		ctrl,
		&persistence.ShardInfo{
			ShardID: 10,
			RangeID: 1,
		},
		config.NewForTest(),
	)

	return ctrl, NewTimerQueueProcessor(
		mockShard,
		task.NewMockProcessor(ctrl),
		execution.NewCache(mockShard),
		archiver.NewMockClient(ctrl),
		invariant.NewMockInvariant(ctrl),
	).(*timerQueueProcessor)
}

func TestTimerQueueProcessor_NotifyNewTask(t *testing.T) {
	const standbyCluster = "standby"
	activeCluster := constants.TestClusterMetadata.GetCurrentClusterName()

	// A timer task with a non-zero visibility timestamp.
	timerTask := &persistence.UserTimerTask{
		TaskData: persistence.TaskData{
			VisibilityTimestamp: time.Now().Add(time.Second),
		},
	}

	tests := map[string]struct {
		clusterName       string
		tasks             []persistence.Task
		preFetched        map[string]time.Time // nil means field is nil, not empty map
		checkNotification func(t *testing.T, processor *timerQueueProcessor)
		shouldPanic       bool
	}{
		"no tasks - no notification": {
			clusterName: activeCluster,
			tasks:       []persistence.Task{},
			preFetched:  nil,
			checkNotification: func(t *testing.T, processor *timerQueueProcessor) {
				select {
				case <-processor.activeQueueProcessor.newTimerCh:
					t.Fatal("expected no notification on active queue processor but got one")
				default:
					// expected: channel empty
				}
			},
		},
		"active cluster - notifyNewTimer fires": {
			clusterName: activeCluster,
			tasks:       []persistence.Task{timerTask},
			preFetched:  nil,
			checkNotification: func(t *testing.T, processor *timerQueueProcessor) {
				select {
				case <-processor.activeQueueProcessor.newTimerCh:
					// expected
				case <-time.After(100 * time.Millisecond):
					t.Fatal("timed out waiting for active queue processor notification")
				}
			},
		},
		"standby cluster without pre-fetched times": {
			clusterName: standbyCluster,
			tasks:       []persistence.Task{timerTask},
			preFetched:  nil, // nil → calls shard.GetCurrentTime
			checkNotification: func(t *testing.T, processor *timerQueueProcessor) {
				select {
				case <-processor.standbyQueueProcessors[standbyCluster].newTimerCh:
					// expected
				case <-time.After(100 * time.Millisecond):
					t.Fatal("timed out waiting for standby queue processor notification")
				}
			},
		},
		"standby cluster with pre-fetched times - map hit": {
			clusterName: standbyCluster,
			tasks:       []persistence.Task{timerTask},
			preFetched:  map[string]time.Time{standbyCluster: time.Now()},
			checkNotification: func(t *testing.T, processor *timerQueueProcessor) {
				select {
				case <-processor.standbyQueueProcessors[standbyCluster].newTimerCh:
					// expected
				case <-time.After(100 * time.Millisecond):
					t.Fatal("timed out waiting for standby queue processor notification (pre-fetched map hit)")
				}
			},
		},
		"standby cluster with pre-fetched times - map miss": {
			clusterName: standbyCluster,
			tasks:       []persistence.Task{timerTask},
			// Non-nil map but does not contain standbyCluster → zero value → fallback to time.Now()
			preFetched: map[string]time.Time{},
			checkNotification: func(t *testing.T, processor *timerQueueProcessor) {
				select {
				case <-processor.standbyQueueProcessors[standbyCluster].newTimerCh:
					// expected: fallback to time.Now() still triggers notification
				case <-time.After(100 * time.Millisecond):
					t.Fatal("timed out waiting for standby queue processor notification (pre-fetched map miss)")
				}
			},
		},
		"unknown cluster - panics": {
			clusterName:       "unknown-cluster",
			tasks:             []persistence.Task{timerTask},
			preFetched:        nil,
			shouldPanic:       true,
			checkNotification: func(t *testing.T, processor *timerQueueProcessor) {},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl, processor := setupTimerQueueProcessor(t)
			defer ctrl.Finish()

			info := &hcommon.NotifyTaskInfo{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				},
				Tasks:               tc.tasks,
				ClusterCurrentTimes: tc.preFetched,
			}

			if tc.shouldPanic {
				assert.Panics(t, func() {
					processor.NotifyNewTask(tc.clusterName, info)
				})
				return
			}

			processor.NotifyNewTask(tc.clusterName, info)
			tc.checkNotification(t, processor)
		})
	}
}

func TestTimerQueueProcessor_NotifyNewTask_PreFetchedTimePropagation(t *testing.T) {
	// Verify that when a pre-fetched time is provided, the standby timer gate's
	// SetCurrentTime is called with that value.
	//
	// Observable side-effect: if we first arm the gate with a fireTime in the past
	// (via Update), and then call NotifyNewTask with a pre-fetched time that is >=
	// fireTime, SetCurrentTime will fire the gate's Chan. This proves SetCurrentTime
	// was called with the pre-fetched time rather than the shard's current time.
	const standbyCluster = "standby"

	ctrl, processor := setupTimerQueueProcessor(t)
	defer ctrl.Finish()

	fireTime := time.Now().Add(-10 * time.Second)      // 10 seconds ago
	preFetchedTime := time.Now().Add(-5 * time.Second) // 5 seconds ago (> fireTime)

	// Arm the standby gate with a fireTime in the past.
	gate := processor.standbyQueueTimerGates[standbyCluster]
	gate.Update(fireTime)

	timerTask := &persistence.UserTimerTask{
		TaskData: persistence.TaskData{
			VisibilityTimestamp: time.Now().Add(time.Second),
		},
	}

	info := &hcommon.NotifyTaskInfo{
		ExecutionInfo: &persistence.WorkflowExecutionInfo{
			DomainID:   constants.TestDomainID,
			WorkflowID: constants.TestWorkflowID,
			RunID:      constants.TestRunID,
		},
		Tasks: []persistence.Task{timerTask},
		ClusterCurrentTimes: map[string]time.Time{
			standbyCluster: preFetchedTime,
		},
	}

	processor.NotifyNewTask(standbyCluster, info)

	// SetCurrentTime(preFetchedTime) fires the gate because preFetchedTime >= fireTime.
	select {
	case <-gate.Chan():
		// expected: gate fired because pre-fetched time was applied
	case <-time.After(100 * time.Millisecond):
		t.Fatal("standby timer gate did not fire; pre-fetched time was not propagated to SetCurrentTime")
	}
}
