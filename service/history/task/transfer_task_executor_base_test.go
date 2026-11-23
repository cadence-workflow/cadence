// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

func TestShouldRedactContextHeader(t *testing.T) {
	cases := []struct {
		key             string
		hiddenValueKeys map[string]interface{}
		expected        bool
	}{
		{
			key: "key1",
			hiddenValueKeys: map[string]interface{}{
				"key1": true,
			},
			expected: true,
		},
		{
			key: "key2",
			hiddenValueKeys: map[string]interface{}{
				"key1": true,
			},
			expected: false,
		},
		{
			key:             "key3",
			hiddenValueKeys: map[string]interface{}{},
			expected:        false,
		},
		{
			key:             "key4",
			hiddenValueKeys: nil,
			expected:        false,
		},
		{
			key: "key5",
			hiddenValueKeys: map[string]interface{}{
				"key5": "true",
			},
			expected: true,
		},
	}

	for _, c := range cases {
		result := shouldRedactContextHeader(c.key, c.hiddenValueKeys)
		if result != c.expected {
			t.Errorf("shouldRedactContextHeader(%s, %v) = %t; expected %t", c.key, c.hiddenValueKeys, result, c.expected)
		}
	}
}

// TestPushDecision covers the standby branch of pushDecision. For an active-active domain whose
// workflow is active in another cluster, shouldPushToMatching returns false. Once the standby
// task has been pending past StandbyTaskMissingEventsDiscardDelay, the task is now written to the
// DLQ (previously it was discarded). Within the delay it is a retryable no-op, and for domains
// that should push it is forwarded to matching.
func TestPushDecision(t *testing.T) {
	const discardDelay = 10 * time.Minute
	visTS := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	newConfig := func() *config.Config {
		cfg := config.NewForTest()
		cfg.StandbyTaskMissingEventsDiscardDelay = func(...dynamicproperties.FilterOption) time.Duration { return discardDelay }
		cfg.HistoryTaskDLQMode = func(string) string { return constants.HistoryTaskDLQModeEnabled }
		return cfg
	}

	activeActiveTask := func() *persistence.DecisionTask {
		return &persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   constants.TestActiveActiveDomainID,
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			TaskData: persistence.TaskData{TaskID: 42, VisibilityTimestamp: visTS},
			TaskList: "tl",
		}
	}

	cases := []struct {
		name           string
		task           *persistence.DecisionTask
		setup          func(t *testing.T, ctrl *gomock.Controller) (*transferTaskExecutorBase, *mockDLQWriter)
		expectDLQWrite bool
		expectErr      func(t *testing.T, err error)
	}{
		{
			name: "active-active passive workflow past discard delay is written to DLQ",
			task: activeActiveTask(),
			setup: func(t *testing.T, ctrl *gomock.Controller) (*transferTaskExecutorBase, *mockDLQWriter) {
				domainCache := cache.NewMockDomainCache(ctrl)
				domainCache.EXPECT().GetDomainByID(constants.TestActiveActiveDomainID).Return(constants.TestActiveActiveDomainEntry, nil).AnyTimes()
				domainCache.EXPECT().GetDomainName(constants.TestActiveActiveDomainID).Return(constants.TestActiveActiveDomainName, nil).AnyTimes()

				activeClusterMgr := activecluster.NewMockManager(ctrl)
				activeClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestActiveActiveDomainID, constants.TestWorkflowID, constants.TestRunID).
					Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestAlternativeClusterName}, nil)
				activeClusterMgr.EXPECT().
					GetActiveClusterSelectionPolicyForWorkflow(gomock.Any(), constants.TestActiveActiveDomainID, constants.TestWorkflowID, constants.TestRunID).
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: &types.ClusterAttribute{Scope: "region", Name: "us-west"},
					}, nil)

				mockShard := shard.NewMockContext(ctrl)
				mockShard.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSourceAt(visTS.Add(discardDelay + time.Second))).AnyTimes()
				mockShard.EXPECT().GetDomainCache().Return(domainCache).AnyTimes()
				mockShard.EXPECT().GetActiveClusterManager().Return(activeClusterMgr).AnyTimes()
				mockShard.EXPECT().GetClusterMetadata().Return(constants.TestClusterMetadata).AnyTimes()
				mockShard.EXPECT().GetShardID().Return(7).AnyTimes()

				writer := &mockDLQWriter{}
				return &transferTaskExecutorBase{
					shard:     mockShard,
					dlqWriter: writer,
					config:    newConfig(),
					logger:    testlogger.New(t),
				}, writer
			},
			expectDLQWrite: true,
			expectErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "active-active passive workflow within discard delay is a retryable no-op",
			task: activeActiveTask(),
			setup: func(t *testing.T, ctrl *gomock.Controller) (*transferTaskExecutorBase, *mockDLQWriter) {
				domainCache := cache.NewMockDomainCache(ctrl)
				domainCache.EXPECT().GetDomainByID(constants.TestActiveActiveDomainID).Return(constants.TestActiveActiveDomainEntry, nil).AnyTimes()

				activeClusterMgr := activecluster.NewMockManager(ctrl)
				activeClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestActiveActiveDomainID, constants.TestWorkflowID, constants.TestRunID).
					Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestAlternativeClusterName}, nil)

				mockShard := shard.NewMockContext(ctrl)
				mockShard.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSourceAt(visTS.Add(discardDelay - time.Second))).AnyTimes()
				mockShard.EXPECT().GetDomainCache().Return(domainCache).AnyTimes()
				mockShard.EXPECT().GetActiveClusterManager().Return(activeClusterMgr).AnyTimes()
				mockShard.EXPECT().GetClusterMetadata().Return(constants.TestClusterMetadata).AnyTimes()

				writer := &mockDLQWriter{}
				return &transferTaskExecutorBase{
					shard:     mockShard,
					dlqWriter: writer,
					config:    newConfig(),
					logger:    testlogger.New(t),
				}, writer
			},
			expectDLQWrite: false,
			expectErr: func(t *testing.T, err error) {
				var re *redispatchError
				assert.ErrorAs(t, err, &re)
			},
		},
		{
			name: "domain that should push is forwarded to matching",
			task: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				},
				TaskData: persistence.TaskData{TaskID: 42, VisibilityTimestamp: visTS},
				TaskList: "tl",
			},
			setup: func(t *testing.T, ctrl *gomock.Controller) (*transferTaskExecutorBase, *mockDLQWriter) {
				domainCache := cache.NewMockDomainCache(ctrl)
				domainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()

				mockShard := shard.NewMockContext(ctrl)
				mockShard.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSourceAt(visTS)).AnyTimes()
				mockShard.EXPECT().GetDomainCache().Return(domainCache).AnyTimes()

				matchingClient := matching.NewMockClient(ctrl)
				matchingClient.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any()).Return(&types.AddDecisionTaskResponse{}, nil)

				writer := &mockDLQWriter{}
				return &transferTaskExecutorBase{
					shard:          mockShard,
					dlqWriter:      writer,
					matchingClient: matchingClient,
					config:         newConfig(),
					logger:         testlogger.New(t),
				}, writer
			},
			expectDLQWrite: false,
			expectErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			base, writer := tc.setup(t, ctrl)

			err := base.pushDecision(context.Background(), tc.task, &pushDecisionToMatchingInfo{
				decisionScheduleToStartTimeout: 10,
				tasklist:                       types.TaskList{Name: tc.task.TaskList},
			})

			tc.expectErr(t, err)
			if tc.expectDLQWrite {
				assert.Len(t, writer.calls, 1)
				assert.Equal(t, tc.task, writer.calls[0].Task)
			} else {
				assert.Empty(t, writer.calls)
			}
		})
	}
}

func TestNewStartedVisibilityRecord(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	startTime := now.Add(-time.Hour)
	clusterAttribute := &types.ClusterAttribute{Scope: "region", Name: "us-west"}

	tests := map[string]struct {
		executionInfo     *persistence.WorkflowExecutionInfo
		startAttributes   *types.WorkflowExecutionStartedEventAttributes
		advancedVisMode   string
		hasDecision       bool
		wantRecordChanges func(record *visibilityRecord)
	}{
		"workflow without backoff": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowTypeName: "wf-type",
				TaskList:         "tl",
				WorkflowTimeout:  60,
				Memo:             map[string][]byte{"memo": []byte("m")},
				SearchAttributes: map[string][]byte{"CustomKeywordField": []byte(`"v"`)},
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
					ClusterAttribute: clusterAttribute,
				},
			},
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{
				Header: &types.Header{Fields: map[string][]byte{"header": []byte("h")}},
			},
			advancedVisMode: commonconstants.AdvancedVisibilityModeOff,
		},
		"cron workflow waiting for its first decision with advanced visibility is pending": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowTypeName: "wf-type",
				TaskList:         "tl",
				WorkflowTimeout:  60,
				CronSchedule:     "* * * * *",
			},
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{
				FirstDecisionTaskBackoffSeconds: common.Int32Ptr(30),
			},
			advancedVisMode: commonconstants.VisibilityModeES,
			wantRecordChanges: func(record *visibilityRecord) {
				record.isCron = true
				record.cronSchedule = "* * * * *"
				record.executionTimeUnixNano = startTime.Add(30 * time.Second).UnixNano()
				record.scheduledExecutionTimeUnixNano = startTime.Add(30 * time.Second).UnixNano()
				record.executionStatus = types.WorkflowExecutionStatusPending
				record.visibilityMemo = nil
				record.searchAttributes = nil
				record.headers = nil
				record.clusterAttribute = nil
			},
		},
		"cron workflow waiting for its first decision with basic visibility is started": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowTypeName: "wf-type",
				TaskList:         "tl",
				WorkflowTimeout:  60,
				CronSchedule:     "* * * * *",
			},
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{
				FirstDecisionTaskBackoffSeconds: common.Int32Ptr(30),
			},
			advancedVisMode: commonconstants.AdvancedVisibilityModeOff,
			wantRecordChanges: func(record *visibilityRecord) {
				record.isCron = true
				record.cronSchedule = "* * * * *"
				record.executionTimeUnixNano = startTime.Add(30 * time.Second).UnixNano()
				record.scheduledExecutionTimeUnixNano = startTime.Add(30 * time.Second).UnixNano()
				record.visibilityMemo = nil
				record.searchAttributes = nil
				record.headers = nil
				record.clusterAttribute = nil
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(ctrl)
			mockShard.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSourceAt(now)).AnyTimes()
			cfg := config.NewForTest()
			cfg.WriteVisibilityStoreName = dynamicproperties.GetStringPropertyFn(tc.advancedVisMode)
			cfg.IsAdvancedVisConfigExist = true
			executor := &transferTaskExecutorBase{shard: mockShard, config: cfg}

			mutableState := execution.NewMockMutableState(ctrl)
			mutableState.EXPECT().GetExecutionInfo().Return(tc.executionInfo).AnyTimes()
			mutableState.EXPECT().GetStartEvent(gomock.Any()).Return(&types.HistoryEvent{
				Timestamp:                               common.Int64Ptr(startTime.UnixNano()),
				WorkflowExecutionStartedEventAttributes: tc.startAttributes,
			}, nil)
			mutableState.EXPECT().HasPendingDecision().Return(false).AnyTimes()
			mutableState.EXPECT().HasInFlightDecision().Return(false).AnyTimes()
			mutableState.EXPECT().HasProcessedOrPendingDecision().Return(false).AnyTimes()

			record, err := executor.newStartedVisibilityRecord(context.Background(), mutableState, constants.TestGlobalDomainEntry)
			assert.NoError(t, err)

			want := &visibilityRecord{
				workflowTypeName:               "wf-type",
				startTimeUnixNano:              startTime.UnixNano(),
				executionTimeUnixNano:          time.Unix(0, 0).UnixNano(),
				workflowTimeout:                60,
				taskList:                       "tl",
				visibilityMemo:                 &types.Memo{Fields: map[string][]byte{"memo": []byte("m")}},
				numClusters:                    int16(len(constants.TestGlobalDomainEntry.GetReplicationConfig().Clusters)),
				updateTimeUnixNano:             now.UnixNano(),
				searchAttributes:               map[string][]byte{"CustomKeywordField": []byte(`"v"`)},
				headers:                        map[string][]byte{"header": []byte("h")},
				clusterAttribute:               clusterAttribute,
				executionStatus:                types.WorkflowExecutionStatusStarted,
				scheduledExecutionTimeUnixNano: startTime.UnixNano(),
			}
			if tc.wantRecordChanges != nil {
				tc.wantRecordChanges(want)
			}
			assert.Equal(t, want, record)
		})
	}
}

func TestNewClosedVisibilityRecord(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	startTime := now.Add(-time.Hour)
	closeTime := now.Add(-time.Minute)

	tests := map[string]struct {
		executionInfo     *persistence.WorkflowExecutionInfo
		startAttributes   *types.WorkflowExecutionStartedEventAttributes
		completionEvent   *types.HistoryEvent
		wantRecordChanges func(record *visibilityRecord)
	}{
		"completed workflow": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowTypeName: "wf-type",
				TaskList:         "tl",
				CloseStatus:      persistence.WorkflowCloseStatusCompleted,
				Memo:             map[string][]byte{"memo": []byte("m")},
				SearchAttributes: map[string][]byte{"CustomKeywordField": []byte(`"v"`)},
			},
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{
				Header: &types.Header{Fields: map[string][]byte{"header": []byte("h")}},
			},
			completionEvent: &types.HistoryEvent{
				Timestamp: common.Int64Ptr(closeTime.UnixNano()),
				EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
			},
		},
		"failed cron workflow with first decision backoff": {
			executionInfo: &persistence.WorkflowExecutionInfo{
				WorkflowTypeName: "wf-type",
				TaskList:         "tl",
				CloseStatus:      persistence.WorkflowCloseStatusFailed,
				CronSchedule:     "* * * * *",
			},
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{
				FirstDecisionTaskBackoffSeconds: common.Int32Ptr(30),
			},
			completionEvent: &types.HistoryEvent{
				Timestamp: common.Int64Ptr(closeTime.UnixNano()),
				EventType: types.EventTypeWorkflowExecutionFailed.Ptr(),
			},
			wantRecordChanges: func(record *visibilityRecord) {
				record.isCron = true
				record.cronSchedule = "* * * * *"
				record.executionTimeUnixNano = startTime.Add(30 * time.Second).UnixNano()
				record.scheduledExecutionTimeUnixNano = startTime.Add(30 * time.Second).UnixNano()
				record.executionStatus = types.WorkflowExecutionStatusFailed
				record.closeStatus = types.WorkflowExecutionCloseStatusFailed
				record.visibilityMemo = nil
				record.searchAttributes = nil
				record.headers = nil
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(ctrl)
			mockShard.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSourceAt(now)).AnyTimes()
			executor := &transferTaskExecutorBase{shard: mockShard, config: config.NewForTest()}

			mutableState := execution.NewMockMutableState(ctrl)
			mutableState.EXPECT().GetExecutionInfo().Return(tc.executionInfo).AnyTimes()
			mutableState.EXPECT().GetStartEvent(gomock.Any()).Return(&types.HistoryEvent{
				Timestamp:                               common.Int64Ptr(startTime.UnixNano()),
				WorkflowExecutionStartedEventAttributes: tc.startAttributes,
			}, nil)
			mutableState.EXPECT().GetNextEventID().Return(int64(11)).AnyTimes()

			record, err := executor.newClosedVisibilityRecord(context.Background(), mutableState, constants.TestGlobalDomainEntry, tc.completionEvent)
			assert.NoError(t, err)

			want := &visibilityRecord{
				workflowTypeName:               "wf-type",
				startTimeUnixNano:              startTime.UnixNano(),
				executionTimeUnixNano:          time.Unix(0, 0).UnixNano(),
				taskList:                       "tl",
				visibilityMemo:                 &types.Memo{Fields: map[string][]byte{"memo": []byte("m")}},
				numClusters:                    int16(len(constants.TestGlobalDomainEntry.GetReplicationConfig().Clusters)),
				updateTimeUnixNano:             now.UnixNano(),
				searchAttributes:               map[string][]byte{"CustomKeywordField": []byte(`"v"`)},
				headers:                        map[string][]byte{"header": []byte("h")},
				executionStatus:                types.WorkflowExecutionStatusCompleted,
				scheduledExecutionTimeUnixNano: startTime.UnixNano(),
				endTimeUnixNano:                closeTime.UnixNano(),
				closeStatus:                    types.WorkflowExecutionCloseStatusCompleted,
				historyLength:                  10,
			}
			if tc.wantRecordChanges != nil {
				tc.wantRecordChanges(want)
			}
			assert.Equal(t, want, record)
		})
	}
}

func TestNewVisibilityRecord_StartEventError(t *testing.T) {
	ctrl := gomock.NewController(t)
	executor := &transferTaskExecutorBase{shard: shard.NewMockContext(ctrl), config: config.NewForTest()}
	mutableState := execution.NewMockMutableState(ctrl)
	mutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{}).AnyTimes()
	startEventErr := errors.New("start event not found")
	mutableState.EXPECT().GetStartEvent(gomock.Any()).Return(nil, startEventErr).Times(2)

	_, err := executor.newStartedVisibilityRecord(context.Background(), mutableState, constants.TestGlobalDomainEntry)
	assert.ErrorIs(t, err, startEventErr)
	_, err = executor.newClosedVisibilityRecord(context.Background(), mutableState, constants.TestGlobalDomainEntry, &types.HistoryEvent{})
	assert.ErrorIs(t, err, startEventErr)
}
