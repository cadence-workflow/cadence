// Copyright (c) 2020 Uber Technologies, Inc.
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

package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/events"
	shardCtx "github.com/uber/cadence/service/history/shard"
)

func TestAddContinueAsNewEvent(t *testing.T) {

	firstEventID := int64(15)
	decisionCompletedEventID := int64(15)

	var (
		domainID = "5391dbea-5b30-4323-82ca-e1c95339bb3e"
		ts0      = int64(123450)
		ts1      = int64(123451)
		ts2      = int64(123452)
		ts3      = int64(123453)
		shardID  = 123
	)

	// the mutable state builder confusingly both returns a new builder with this fuction
	// as well as mutating its internal state, making it difficult to test repeatedly, since
	// the supplied inputs are muted per invocation. Wrapping them in a factor here to allow
	// for tests to be independent
	var createStartingExecutionInfo = func() *persistence.WorkflowExecutionInfo {
		return &persistence.WorkflowExecutionInfo{
			DomainID:                           "5391dbea-5b30-4323-82ca-e1c95339bb3e",
			WorkflowID:                         "helloworld_b4db8bd0-74b7-4250-ade7-ac72a1efb171",
			RunID:                              "5adce5c5-b7b2-4418-9bf0-4207303f6343",
			FirstExecutionRunID:                "5adce5c5-b7b2-4418-9bf0-4207303f6343",
			InitiatedID:                        -7,
			TaskList:                           "helloWorldGroup",
			WorkflowTypeName:                   "helloWorldWorkflow",
			WorkflowTimeout:                    60,
			DecisionStartToCloseTimeout:        60,
			State:                              1,
			LastFirstEventID:                   14,
			LastEventTaskID:                    15728673,
			NextEventID:                        16,
			LastProcessedEvent:                 14,
			StartTimestamp:                     time.Unix(0, ts0),
			LastUpdatedTimestamp:               time.Unix(0, ts2),
			CreateRequestID:                    "b086d62c-dd2b-4bbc-9143-5940516acbfe",
			DecisionVersion:                    -24,
			DecisionScheduleID:                 -23,
			DecisionStartedID:                  -23,
			DecisionRequestID:                  "emptyUuid",
			DecisionOriginalScheduledTimestamp: 1709872131542474000,
			StickyTaskList:                     "david-porter-DVFG73D710:04be47fa-2381-469f-b2ea-1253271ad116",
			StickyScheduleToStartTimeout:       5,
			ClientLibraryVersion:               "0.18.4",
			ClientFeatureVersion:               "1.7.0",
			ClientImpl:                         "uber-go",
			AutoResetPoints: &types.ResetPoints{
				Points: []*types.ResetPointInfo{{
					BinaryChecksum:           "6df03bf5110d681667852a8456519536",
					RunID:                    "5adce5c5-b7b2-4418-9bf0-4207303f6343",
					FirstDecisionCompletedID: 4,
					CreatedTimeNano:          common.Ptr(int64(ts1)),
					Resettable:               true,
				}},
			},
			SearchAttributes: map[string][]uint8{"BinaryChecksums": {91, 34, 54, 100, 102, 48, 51, 98, 102, 53, 49, 49, 48, 100, 54, 56, 49, 54, 54, 55, 56, 53, 50, 97, 56, 52, 53, 54, 53, 49, 57, 53, 51, 54, 34, 93}}}

	}

	var createValidStartingHistory = func() []*types.HistoryEvent {
		return []*types.HistoryEvent{{
			ID:        15,
			Timestamp: common.Ptr(int64(1709872131580456000)),
			EventType: common.Ptr(types.EventTypeDecisionTaskCompleted),
			Version:   1,
			TaskID:    -1234,
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 13,
				StartedEventID:   14,
				Identity:         "27368@david-porter-DVFG73D710@helloWorldGroup@6027e9ee-048e-4f67-8d88-27883c496901",
				BinaryChecksum:   "6df03bf5110d681667852a8456519536",
			},
		}}
	}

	var createFetchedHistory = func() *types.HistoryEvent {
		return &types.HistoryEvent{
			ID: 1, Timestamp: common.Ptr(int64(1709938156435726000)),
			EventType: common.Ptr(types.EventTypeWorkflowExecutionStarted),
			Version:   1,
			TaskID:    17826364,
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
				WorkflowType:                        &types.WorkflowType{Name: "helloWorldWorkflow"},
				TaskList:                            &types.TaskList{Name: "helloWorldGroup"},
				Input:                               []uint8{110, 117, 108, 108, 10},
				ExecutionStartToCloseTimeoutSeconds: common.Ptr(int32(60)),
				TaskStartToCloseTimeoutSeconds:      common.Ptr(int32(60)),
				ContinuedExecutionRunID:             "96892ca6-975a-44b1-9726-cdb63acd8cda",
				OriginalExecutionRunID:              "befc5b41-fb06-4a99-bec2-91c3e98b17d7",
				FirstExecutionRunID:                 "bcdee7e4-cb21-4bbb-a8d1-43da79e3d252",
				PrevAutoResetPoints: &types.ResetPoints{Points: []*types.ResetPointInfo{
					{
						BinaryChecksum:           "6df03bf5110d681667852a8456519536",
						RunID:                    "bcdee7e4-cb21-4bbb-a8d1-43da79e3d252",
						FirstDecisionCompletedID: 4,
						CreatedTimeNano:          common.Ptr(int64(1709938002170829000)),
						ExpiringTimeNano:         common.Ptr(int64(1710197212347858000)),
						Resettable:               true,
					},
				}},
				Header: &types.Header{},
			},
		}
	}

	expectedEndingReturnExecutionState := &persistence.WorkflowExecutionInfo{
		DomainID:                           "5391dbea-5b30-4323-82ca-e1c95339bb3e",
		WorkflowID:                         "helloworld_b4db8bd0-74b7-4250-ade7-ac72a1efb171",
		RunID:                              "a run id",
		FirstExecutionRunID:                "5adce5c5-b7b2-4418-9bf0-4207303f6343",
		InitiatedID:                        -23,
		TaskList:                           "helloWorldGroup",
		WorkflowTypeName:                   "helloWorldWorkflow",
		WorkflowTimeout:                    60,
		DecisionStartToCloseTimeout:        60,
		State:                              1,
		LastFirstEventID:                   1,
		NextEventID:                        3,
		LastProcessedEvent:                 -23,
		StartTimestamp:                     time.Unix(0, ts3),
		CreateRequestID:                    "4630bf04-5c64-41bf-92d9-576db2d535cb",
		DecisionVersion:                    1,
		DecisionScheduleID:                 2,
		DecisionStartedID:                  -23,
		DecisionRequestID:                  "emptyUuid",
		DecisionTimeout:                    60,
		DecisionScheduledTimestamp:         ts3,
		DecisionOriginalScheduledTimestamp: ts3,
		AutoResetPoints: &types.ResetPoints{
			Points: []*types.ResetPointInfo{{
				BinaryChecksum:           "6df03bf5110d681667852a8456519536",
				RunID:                    "5adce5c5-b7b2-4418-9bf0-4207303f6343",
				FirstDecisionCompletedID: 4,
				CreatedTimeNano:          common.Ptr(int64(ts1)),
				ExpiringTimeNano:         common.Ptr(int64(ts3)),
				Resettable:               true,
			}},
		},
	}

	expectedEndingReturnHistoryState := []*types.HistoryEvent{
		{
			ID:        1,
			Timestamp: common.Ptr(int64(ts3)),
			EventType: common.Ptr(types.EventTypeWorkflowExecutionStarted),
			Version:   1,
			TaskID:    -1234,
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
				WorkflowType: &types.WorkflowType{
					Name: "helloWorldWorkflow",
				},
				TaskList:                            &types.TaskList{Name: "helloWorldGroup"},
				Input:                               []uint8{110, 117, 108, 108, 10},
				ExecutionStartToCloseTimeoutSeconds: common.Ptr(int32(60)),
				TaskStartToCloseTimeoutSeconds:      common.Ptr(int32(60)),
				ContinuedExecutionRunID:             "5adce5c5-b7b2-4418-9bf0-4207303f6343",
				OriginalExecutionRunID:              "a run id",
				FirstExecutionRunID:                 "5adce5c5-b7b2-4418-9bf0-4207303f6343",
				PrevAutoResetPoints: &types.ResetPoints{Points: []*types.ResetPointInfo{{
					BinaryChecksum:           "6df03bf5110d681667852a8456519536",
					RunID:                    "5adce5c5-b7b2-4418-9bf0-4207303f6343",
					FirstDecisionCompletedID: 4,
					CreatedTimeNano:          common.Ptr(int64(ts1)),
					ExpiringTimeNano:         common.Ptr(int64(ts3)),
					Resettable:               true,
				}}},
				Header: nil,
			},
		},
		{
			ID:        2,
			Timestamp: common.Ptr(int64(ts3)),
			EventType: common.Ptr(types.EventTypeDecisionTaskScheduled),
			Version:   1,
			TaskID:    -1234,
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
				TaskList:                   &types.TaskList{Name: "helloWorldGroup"},
				StartToCloseTimeoutSeconds: common.Ptr(int32(60)),
			},
		},
	}

	tests := map[string]struct {
		startingState *persistence.WorkflowExecutionInfo
		// history is a substruct of current state, but because they're both
		// pointing to each other, they're assembled at the test start
		startingHistory []*types.HistoryEvent

		// expectations
		historyManagerAffordance func(historyManager *persistence.MockHistoryManager)
		shardManagerAffordance   func(shardContext *shardCtx.MockContext, msb *mutableStateBuilder, domainCache cache.DomainCache)
		domainCacheAffordance    func(domainCache *cache.MockDomainCache)
		taskgeneratorAffordance  func(taskGenerator *MockMutableStateTaskGenerator, msb *mutableStateBuilder)

		expectedReturnedState   *persistence.WorkflowExecutionInfo // this is returned
		expectedReturnedHistory []*types.HistoryEvent
		expectedErr             error
	}{
		"a continue-as-new event with no errors": {
			startingState:   createStartingExecutionInfo(),
			startingHistory: createValidStartingHistory(),

			// when it goes to fetch the starting event
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {
				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(&persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{
						createFetchedHistory(),
					},
				}, nil)
			},
			shardManagerAffordance: func(shardContext *shardCtx.MockContext, msb *mutableStateBuilder, domainCache cache.DomainCache) {
				shardContext.EXPECT().GetShardID().Return(123)
				shardContext.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)
				shardContext.EXPECT().GetEventsCache().Return(msb.eventsCache)
				shardContext.EXPECT().GetConfig().Return(msb.config)
				shardContext.EXPECT().GetTimeSource().Return(msb.timeSource)
				shardContext.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient())
				shardContext.EXPECT().GetDomainCache().Return(domainCache)
			},
			domainCacheAffordance: func(domainCache *cache.MockDomainCache) {
				domainCache.EXPECT().GetDomainName(gomock.Any()).Return("domain", nil)
			},
			taskgeneratorAffordance: func(taskGenerator *MockMutableStateTaskGenerator, msb *mutableStateBuilder) {
				taskGenerator.EXPECT().GenerateWorkflowCloseTasks(gomock.Any(), msb.config.WorkflowDeletionJitterRange("domain"))
			},

			expectedReturnedState:   expectedEndingReturnExecutionState,
			expectedReturnedHistory: expectedEndingReturnHistoryState,
		},

		"a continue-as-new with failure to get the history event": {
			startingState:   createStartingExecutionInfo(),
			startingHistory: createValidStartingHistory(),
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {
				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(nil, errors.New("an error"))
			},
			shardManagerAffordance: func(shardContext *shardCtx.MockContext, msb *mutableStateBuilder, domainCache cache.DomainCache) {
				shardContext.EXPECT().GetShardID().Return(123)
			},
			domainCacheAffordance: func(domainCache *cache.MockDomainCache) {
				domainCache.EXPECT().GetDomainName(gomock.Any()).Return("domain", nil)
			},
			taskgeneratorAffordance: func(taskGenerator *MockMutableStateTaskGenerator, msb *mutableStateBuilder) {},
			expectedErr:             errors.New("an error"),
		},

		"a continue-as-new with errors in replicating": {
			startingState:   createStartingExecutionInfo(),
			startingHistory: createValidStartingHistory(),
			historyManagerAffordance: func(historyManager *persistence.MockHistoryManager) {
				historyManager.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(&persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{
						createFetchedHistory(),
					},
				}, nil)
			},
			shardManagerAffordance: func(shardContext *shardCtx.MockContext, msb *mutableStateBuilder, domainCache cache.DomainCache) {
				shardContext.EXPECT().GetShardID().Return(123)
				shardContext.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)
				shardContext.EXPECT().GetEventsCache().Return(msb.eventsCache)
				shardContext.EXPECT().GetConfig().Return(msb.config)
				shardContext.EXPECT().GetTimeSource().Return(msb.timeSource)
				shardContext.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient())
				shardContext.EXPECT().GetDomainCache().Return(domainCache)
			},
			domainCacheAffordance: func(domainCache *cache.MockDomainCache) {
				domainCache.EXPECT().GetDomainName(gomock.Any()).Return("domain", nil)
			},
			taskgeneratorAffordance: func(taskGenerator *MockMutableStateTaskGenerator, msb *mutableStateBuilder) {
				taskGenerator.EXPECT().GenerateWorkflowCloseTasks(gomock.Any(), msb.config.WorkflowDeletionJitterRange("domain")).Return(errors.New("an error"))
			},
			expectedErr: errors.New("an error"),
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			msb := &mutableStateBuilder{
				domainEntry: cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: domainID},
					&persistence.DomainConfig{},
					true,
					&persistence.DomainReplicationConfig{},
					1,
					nil,
					0,
					0,
					0,
				),
				executionInfo: td.startingState,
				logger:        log.NewNoop(),
				config:        config.NewForTest(),
			}

			shardContext := shardCtx.NewMockContext(ctrl)
			historyManager := persistence.NewMockHistoryManager(ctrl)
			domainCache := cache.NewMockDomainCache(ctrl)
			taskGenerator := NewMockMutableStateTaskGenerator(ctrl)

			msb.timeSource = clock.NewMockedTimeSourceAt(time.Unix(0, ts3))
			msb.eventsCache = events.NewCache(shardID,
				historyManager,
				config.NewForTest(),
				log.NewNoop(),
				metrics.NewNoopMetricsClient(),
				domainCache)
			msb.shard = shardContext
			msb.executionInfo = td.startingState
			msb.hBuilder = &HistoryBuilder{
				history:   td.startingHistory,
				msBuilder: msb,
			}
			msb.taskGenerator = taskGenerator

			td.historyManagerAffordance(historyManager)
			td.domainCacheAffordance(domainCache)
			td.taskgeneratorAffordance(taskGenerator, msb)
			td.shardManagerAffordance(shardContext, msb, domainCache)

			_, returnedBuilder, err := msb.AddContinueAsNewEvent(context.Background(),
				firstEventID,
				decisionCompletedEventID,
				"",
				&types.ContinueAsNewWorkflowExecutionDecisionAttributes{
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
					WorkflowType: &types.WorkflowType{
						Name: "helloWorldWorkflow",
					},
					TaskList: &types.TaskList{
						Name: "helloWorldGroup",
					},
					Input: []uint8{110, 117, 108, 108, 10},
				})

			if td.expectedErr != nil {
				assert.ErrorAs(t, err, &td.expectedErr)
				return
			}

			resultExecutionInfo := returnedBuilder.GetExecutionInfo()

			assert.Empty(t, cmp.Diff(td.expectedReturnedState, resultExecutionInfo,

				// these are generated nondeterministically, with a plain guid generator
				// todo(david): make this mockable
				cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "OriginalExecutionRunID"),
				cmpopts.IgnoreFields(types.WorkflowExecution{}, "RunID"),
				cmpopts.IgnoreFields(persistence.WorkflowExecutionInfo{}, "RunID", "CreateRequestID"),
			))

			assert.Empty(t, cmp.Diff(td.expectedReturnedHistory, returnedBuilder.GetHistoryBuilder().history,
				cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "OriginalExecutionRunID"),
				cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "RequestID")),
			)
		})
	}
}
