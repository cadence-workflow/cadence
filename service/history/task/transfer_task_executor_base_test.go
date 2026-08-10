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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
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
