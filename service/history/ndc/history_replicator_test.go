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

package ndc

import (
	ctx "context"
	"fmt"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	commonConfig "github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/resource"
	"github.com/uber/cadence/service/history/shard"
)

var (
	testShardID = 1234
)

func createTestHistoryReplicator(t *testing.T, domainID string) historyReplicatorImpl {
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetConfig().Return(&config.Config{
		NumberOfShards:                       0,
		IsAdvancedVisConfigExist:             false,
		MaxResponseSize:                      0,
		HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
		HostName:                             "test-host",
		EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
	}).Times(1)

	// before going into NewHistoryReplicator
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).AnyTimes()
	mockShard.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	mockShard.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	mockShard.EXPECT().GetShardID().Return(testShardID).AnyTimes()

	testExecutionCache := execution.NewCache(mockShard)
	mockEventsReapplier := NewMockEventsReapplier(ctrl)

	// going into NewHistoryReplicator -> newTransactionManager()
	clusterMetadata := cluster.Metadata{}
	mockShard.EXPECT().GetClusterMetadata().Return(clusterMetadata).Times(2)
	mockActiveClusterManager := activecluster.NewMockManager(ctrl)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager).Times(2)
	mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
	mockShard.EXPECT().GetHistoryManager().Return(mockHistoryManager).Times(3)

	mockHistoryResource := resource.NewMockResource(ctrl)
	mockShard.EXPECT().GetService().Return(mockHistoryResource).Times(2)
	mockPayloadSerializer := persistence.NewMockPayloadSerializer(ctrl)
	mockHistoryResource.EXPECT().GetPayloadSerializer().Return(mockPayloadSerializer).Times(1)

	// going into NewHistoryReplicator -> newTransactionManager -> reset.NewWorkflowResetter
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).Times(2)

	// going back to NewHistoryReplicator
	mockHistoryResource.EXPECT().GetClusterMetadata().Return(clusterMetadata).Times(1)

	replicator := NewHistoryReplicator(mockShard, testExecutionCache, mockEventsReapplier, log.NewNoop())
	replicatorImpl := replicator.(*historyReplicatorImpl)
	return *replicatorImpl
}

func TestNewHistoryReplicator(t *testing.T) {
	assert.NotNil(t, createTestHistoryReplicator(t, uuid.New()))
}

func TestNewHistoryReplicator_newBranchManager(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetConfig().Return(&config.Config{
		NumberOfShards:                       0,
		IsAdvancedVisConfigExist:             false,
		MaxResponseSize:                      0,
		HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
		HostName:                             "test-host",
		EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
	}).Times(1)

	// before going into NewHistoryReplicator
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).Times(1)
	mockShard.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	mockShard.EXPECT().GetShardID().Return(testShardID).AnyTimes()

	testExecutionCache := execution.NewCache(mockShard)
	mockEventsReapplier := NewMockEventsReapplier(ctrl)

	// going into NewHistoryReplicator -> newTransactionManager()
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)
	mockActiveClusterManager := activecluster.NewMockManager(ctrl)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager).Times(2)
	mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
	mockShard.EXPECT().GetHistoryManager().Return(mockHistoryManager).Times(4)

	mockHistoryResource := resource.NewMockResource(ctrl)
	mockShard.EXPECT().GetService().Return(mockHistoryResource).Times(3)
	mockPayloadSerializer := persistence.NewMockPayloadSerializer(ctrl)
	mockHistoryResource.EXPECT().GetPayloadSerializer().Return(mockPayloadSerializer).Times(1)

	// going into NewHistoryReplicator -> newTransactionManager -> reset.NewWorkflowResetter
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).Times(2)

	// going back to NewHistoryReplicator
	mockHistoryResource.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)

	testReplicator := NewHistoryReplicator(mockShard, testExecutionCache, mockEventsReapplier, log.NewNoop())
	testReplicatorImpl := testReplicator.(*historyReplicatorImpl)

	// test newBranchManagerFn function in history replicator
	mockExecutionContext := execution.NewMockContext(ctrl)
	mockExecutionMutableState := execution.NewMockMutableState(ctrl)
	assert.NotNil(t, testReplicatorImpl.newBranchManagerFn(mockExecutionContext, mockExecutionMutableState, log.NewNoop()))
}

func TestNewHistoryReplicator_newConflictResolver(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetConfig().Return(&config.Config{
		NumberOfShards:                       0,
		IsAdvancedVisConfigExist:             false,
		MaxResponseSize:                      0,
		HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
		HostName:                             "test-host",
		EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
	}).Times(2)

	// before going into NewHistoryReplicator
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).Times(1)
	mockShard.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	mockShard.EXPECT().GetShardID().Return(testShardID).AnyTimes()

	testExecutionCache := execution.NewCache(mockShard)
	mockEventsReapplier := NewMockEventsReapplier(ctrl)

	// going into NewHistoryReplicator -> newTransactionManager()
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(3)
	mockActiveClusterManager := activecluster.NewMockManager(ctrl)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager).Times(2)
	mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
	mockShard.EXPECT().GetHistoryManager().Return(mockHistoryManager).Times(4)

	mockHistoryResource := resource.NewMockResource(ctrl)
	mockShard.EXPECT().GetService().Return(mockHistoryResource).Times(3)
	mockPayloadSerializer := persistence.NewMockPayloadSerializer(ctrl)
	mockHistoryResource.EXPECT().GetPayloadSerializer().Return(mockPayloadSerializer).Times(1)

	// going into NewHistoryReplicator -> newTransactionManager -> reset.NewWorkflowResetter
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).Times(4)

	// going back to NewHistoryReplicator
	mockHistoryResource.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)

	testReplicator := NewHistoryReplicator(mockShard, testExecutionCache, mockEventsReapplier, log.NewNoop())
	testReplicatorImpl := testReplicator.(*historyReplicatorImpl)

	// test newConflictResolverFn function in history replicator
	mockEventsCache := events.NewMockCache(ctrl)
	mockShard.EXPECT().GetEventsCache().Return(mockEventsCache).Times(1)

	mockExecutionContext := execution.NewMockContext(ctrl)
	mockExecutionMutableState := execution.NewMockMutableState(ctrl)
	assert.NotNil(t, testReplicatorImpl.newConflictResolverFn(mockExecutionContext, mockExecutionMutableState, log.NewNoop()))
}

func TestNewHistoryReplicator_newWorkflowResetter(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetConfig().Return(&config.Config{
		NumberOfShards:                       0,
		IsAdvancedVisConfigExist:             false,
		MaxResponseSize:                      0,
		HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
		HostName:                             "test-host",
		EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
	}).Times(2)

	// before going into NewHistoryReplicator
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).Times(1)
	mockShard.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	mockShard.EXPECT().GetShardID().Return(testShardID).AnyTimes()

	testExecutionCache := execution.NewCache(mockShard)
	mockEventsReapplier := NewMockEventsReapplier(ctrl)

	// going into NewHistoryReplicator -> newTransactionManager()
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(3)
	mockActiveClusterManager := activecluster.NewMockManager(ctrl)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager).Times(2)
	mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
	mockShard.EXPECT().GetHistoryManager().Return(mockHistoryManager).Times(5)

	mockHistoryResource := resource.NewMockResource(ctrl)
	mockShard.EXPECT().GetService().Return(mockHistoryResource).Times(3)
	mockPayloadSerializer := persistence.NewMockPayloadSerializer(ctrl)
	mockHistoryResource.EXPECT().GetPayloadSerializer().Return(mockPayloadSerializer).Times(1)

	// going into NewHistoryReplicator -> newTransactionManager -> reset.NewWorkflowResetter
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).Times(4)

	// going back to NewHistoryReplicator
	mockHistoryResource.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)

	testReplicator := NewHistoryReplicator(mockShard, testExecutionCache, mockEventsReapplier, log.NewNoop())
	testReplicatorImpl := testReplicator.(*historyReplicatorImpl)

	// test newWorkflowResetterFn function in history replicator
	mockEventsCache := events.NewMockCache(ctrl)
	mockShard.EXPECT().GetEventsCache().Return(mockEventsCache).Times(1)
	mockShard.EXPECT().GetShardID().Return(testShardID).AnyTimes()

	mockExecutionContext := execution.NewMockContext(ctrl)
	assert.NotNil(t, testReplicatorImpl.newWorkflowResetterFn(
		"test-domain-id",
		"test-workflow-id",
		"test-base-run-id",
		mockExecutionContext,
		"test-run-id",
		log.NewNoop(),
	))
}

func TestNewHistoryReplicator_newStateBuilder(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetConfig().Return(&config.Config{
		NumberOfShards:                       0,
		IsAdvancedVisConfigExist:             false,
		MaxResponseSize:                      0,
		HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
		HostName:                             "test-host",
		EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
	}).Times(1)

	// before going into NewHistoryReplicator
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).Times(1)
	mockShard.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	mockShard.EXPECT().GetShardID().Return(testShardID).AnyTimes()

	testExecutionCache := execution.NewCache(mockShard)
	mockEventsReapplier := NewMockEventsReapplier(ctrl)

	// going into NewHistoryReplicator -> newTransactionManager()
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)
	mockActiveClusterManager := activecluster.NewMockManager(ctrl)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager).Times(2)
	mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
	mockShard.EXPECT().GetHistoryManager().Return(mockHistoryManager).Times(3)

	mockHistoryResource := resource.NewMockResource(ctrl)
	mockShard.EXPECT().GetService().Return(mockHistoryResource).Times(3)
	mockPayloadSerializer := persistence.NewMockPayloadSerializer(ctrl)
	mockHistoryResource.EXPECT().GetPayloadSerializer().Return(mockPayloadSerializer).Times(1)

	// going into NewHistoryReplicator -> newTransactionManager -> reset.NewWorkflowResetter
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).Times(3)

	// going back to NewHistoryReplicator
	mockHistoryResource.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(2)

	testReplicator := NewHistoryReplicator(mockShard, testExecutionCache, mockEventsReapplier, log.NewNoop())
	testReplicatorImpl := testReplicator.(*historyReplicatorImpl)

	// test newStateBuilderFn function in history replicator
	mockExecutionMutableState := execution.NewMockMutableState(ctrl)
	assert.NotNil(t, testReplicatorImpl.newStateBuilderFn(mockExecutionMutableState, log.NewNoop()))
}

func TestNewHistoryReplicator_newMutableState(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetConfig().Return(&config.Config{
		NumberOfShards:                       0,
		IsAdvancedVisConfigExist:             false,
		MaxResponseSize:                      0,
		HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
		HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
		HostName:                             "test-host",
		EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
	}).Times(2)

	// before going into NewHistoryReplicator
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).Times(1)
	mockShard.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	mockShard.EXPECT().GetShardID().Return(testShardID).AnyTimes()

	testExecutionCache := execution.NewCache(mockShard)
	mockEventsReapplier := NewMockEventsReapplier(ctrl)

	// going into NewHistoryReplicator -> newTransactionManager()
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(4)
	mockActiveClusterManager := activecluster.NewMockManager(ctrl)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager).Times(2)
	mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
	mockShard.EXPECT().GetHistoryManager().Return(mockHistoryManager).Times(3)

	mockHistoryResource := resource.NewMockResource(ctrl)
	mockShard.EXPECT().GetService().Return(mockHistoryResource).Times(2)
	mockPayloadSerializer := persistence.NewMockPayloadSerializer(ctrl)
	mockHistoryResource.EXPECT().GetPayloadSerializer().Return(mockPayloadSerializer).Times(1)

	// going into NewHistoryReplicator -> newTransactionManager -> reset.NewWorkflowResetter
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).Times(3)

	// going back to NewHistoryReplicator
	mockHistoryResource.EXPECT().GetClusterMetadata().Return(cluster.Metadata{}).Times(1)

	testReplicator := NewHistoryReplicator(mockShard, testExecutionCache, mockEventsReapplier, log.NewNoop())
	testReplicatorImpl := testReplicator.(*historyReplicatorImpl)

	// test newMutableStateFn function in history replicator
	deadline := int64(0)
	mockShard.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSource()).Times(1)
	mockEventsCache := events.NewMockCache(ctrl)
	mockShard.EXPECT().GetEventsCache().Return(mockEventsCache).Times(1)
	mockDomainCacheEntry := cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: "test-domain"},
		nil,
		true,
		&persistence.DomainReplicationConfig{ActiveClusterName: "test-active-cluster"},
		0,
		&deadline,
		0, 0, 0,
	)
	assert.NotNil(t, testReplicatorImpl.newMutableStateFn(mockDomainCacheEntry, log.NewNoop()))
}

func TestApplyEvents(t *testing.T) {
	replicator := createTestHistoryReplicator(t, uuid.New())
	replicator.newReplicationTaskFn = func(
		clusterMetadata cluster.Metadata,
		historySerializer persistence.PayloadSerializer,
		taskStartTime time.Time,
		logger log.Logger,
		request *types.ReplicateEventsV2Request,
	) (replicationTask, error) {
		return nil, nil
	}

	// Intentionally panic result. Will test applyEvents function seperately
	assert.Panics(t, func() { replicator.ApplyEvents(nil, nil) })
}

func Test_applyEvents_EventTypeWorkflowExecutionStarted(t *testing.T) {
	workflowExecutionStartedType := types.EventTypeWorkflowExecutionStarted

	tests := map[string]struct {
		mockExecutionCacheAffordance  func(mockExecutionCache *execution.MockCache)
		mockReplicationTaskAffordance func(mockReplicationTask *MockreplicationTask)
		expectError                   error
	}{
		"Case1: success case": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionStartedType,
				}).Times(1)
			},
			expectError: nil,
		},
		"Case2: error case": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, func(err error) {}, fmt.Errorf("test error")).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
			},
			expectError: fmt.Errorf("test error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// mock objects
			replicator := createTestHistoryReplicator(t, uuid.New())
			mockReplicationTask := NewMockreplicationTask(ctrl)
			mockExecutionCache := execution.NewMockCache(ctrl)
			replicator.executionCache = mockExecutionCache
			// mock functions
			test.mockReplicationTaskAffordance(mockReplicationTask)
			test.mockExecutionCacheAffordance(mockExecutionCache)

			// parameter functions affordance
			replicator.applyStartEventsFn = func(
				ctx ctx.Context,
				context execution.Context,
				releaseFn execution.ReleaseFunc,
				task replicationTask,
				domainCache cache.DomainCache,
				newMutableState newMutableStateFn,
				newStateBuilder newStateBuilderFn,
				transactionManager transactionManager,
				logger log.Logger,
				shard shard.Context,
				clusterMetadata cluster.Metadata,
			) error {
				return nil
			}

			assert.Equal(t, replicator.applyEvents(ctx.Background(), mockReplicationTask), test.expectError)
		})
	}
}

func Test_applyEvents_defaultCase_noErrorBranch(t *testing.T) {
	workflowExecutionType := types.EventTypeWorkflowExecutionCompleted

	tests := map[string]struct {
		mockExecutionCacheAffordance           func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext)
		mockReplicationTaskAffordance          func(mockReplicationTask *MockreplicationTask)
		mockExecutionContextAffordance         func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState)
		mockMutableStateAffordance             func(mockExecutionMutableState *execution.MockMutableState)
		mockApplyNonStartEventsPrepareBranchFn func(ctx ctx.Context,
			context execution.Context,
			mutableState execution.MutableState,
			task replicationTask,
			newBranchManager newBranchManagerFn,
		) (bool, int, error)
		applyNonStartEventsPrepareMutableStateFnAffordance func(mockExecutionMutableState *execution.MockMutableState) func(ctx ctx.Context,
			context execution.Context,
			mutableState execution.MutableState,
			branchIndex int,
			task replicationTask,
			newConflictResolver newConflictResolverFn,
		) (execution.MutableState, bool, error)
		expectError error
	}{
		"Case1: case nil with GetVersionHistories is nil": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, nil).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionMutableState.EXPECT().GetVersionHistories().Return(nil).Times(1)
			},
			mockApplyNonStartEventsPrepareBranchFn: func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				task replicationTask,
				newBranchManager newBranchManagerFn,
			) (bool, int, error) {
				return false, 0, nil
			},
			applyNonStartEventsPrepareMutableStateFnAffordance: func(mockExecutionMutableState *execution.MockMutableState) func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				branchIndex int,
				task replicationTask,
				newConflictResolver newConflictResolverFn,
			) (execution.MutableState, bool, error) {
				fn := func(ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					task replicationTask,
					newConflictResolver newConflictResolverFn,
				) (execution.MutableState, bool, error) {
					return nil, false, fmt.Errorf("test error")
				}
				return fn
			},
			expectError: execution.ErrMissingVersionHistories,
		},
		"Case2: case nil with applyNonStartEventsPrepareBranchFn error": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, nil).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionMutableState.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 1,
					Histories:                  nil,
				}).Times(1)
			},
			mockApplyNonStartEventsPrepareBranchFn: func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				task replicationTask,
				newBranchManager newBranchManagerFn,
			) (bool, int, error) {
				return false, 0, fmt.Errorf("test error")
			},
			applyNonStartEventsPrepareMutableStateFnAffordance: func(mockExecutionMutableState *execution.MockMutableState) func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				branchIndex int,
				task replicationTask,
				newConflictResolver newConflictResolverFn,
			) (execution.MutableState, bool, error) {
				fn := func(ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					task replicationTask,
					newConflictResolver newConflictResolverFn,
				) (execution.MutableState, bool, error) {
					return nil, false, fmt.Errorf("test error")
				}
				return fn
			},
			expectError: fmt.Errorf("test error"),
		},
		"Case3: case nil with applyNonStartEventsPrepareBranchFn no error and doContinue is false": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, nil).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionMutableState.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 1,
					Histories:                  nil,
				}).Times(1)
			},
			mockApplyNonStartEventsPrepareBranchFn: func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				task replicationTask,
				newBranchManager newBranchManagerFn,
			) (bool, int, error) {
				return false, 0, nil
			},
			applyNonStartEventsPrepareMutableStateFnAffordance: func(mockExecutionMutableState *execution.MockMutableState) func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				branchIndex int,
				task replicationTask,
				newConflictResolver newConflictResolverFn,
			) (execution.MutableState, bool, error) {
				fn := func(ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					task replicationTask,
					newConflictResolver newConflictResolverFn,
				) (execution.MutableState, bool, error) {
					return nil, false, fmt.Errorf("test error")
				}
				return fn
			},
			expectError: nil,
		},
		"Case4: case nil with applyNonStartEventsPrepareMutableStateFn error": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, nil).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionMutableState.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 1,
					Histories:                  nil,
				}).Times(1)
			},
			mockApplyNonStartEventsPrepareBranchFn: func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				task replicationTask,
				newBranchManager newBranchManagerFn,
			) (bool, int, error) {
				return true, 5, nil
			},
			applyNonStartEventsPrepareMutableStateFnAffordance: func(mockExecutionMutableState *execution.MockMutableState) func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				branchIndex int,
				task replicationTask,
				newConflictResolver newConflictResolverFn,
			) (execution.MutableState, bool, error) {
				fn := func(ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					task replicationTask,
					newConflictResolver newConflictResolverFn,
				) (execution.MutableState, bool, error) {
					assert.Equal(t, branchIndex, 5)
					return nil, false, fmt.Errorf("test error")
				}
				return fn
			},
			expectError: fmt.Errorf("test error"),
		},
		"Case5: case nil with CurrentVersionHistoryIndex() == branchIndex": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, nil).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionMutableState.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 1,
					Histories:                  nil,
				}).Times(2)
			},
			mockApplyNonStartEventsPrepareBranchFn: func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				task replicationTask,
				newBranchManager newBranchManagerFn,
			) (bool, int, error) {
				return true, 1, nil
			},
			applyNonStartEventsPrepareMutableStateFnAffordance: func(mockExecutionMutableState *execution.MockMutableState) func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				branchIndex int,
				task replicationTask,
				newConflictResolver newConflictResolverFn,
			) (execution.MutableState, bool, error) {
				fn := func(ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					task replicationTask,
					newConflictResolver newConflictResolverFn,
				) (execution.MutableState, bool, error) {
					assert.Equal(t, branchIndex, 1)
					return mockExecutionMutableState, false, nil
				}
				return fn
			},
			expectError: nil,
		},
		"Case6: case nil with CurrentVersionHistoryIndex() != branchIndex": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, nil).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionMutableState.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 1,
					Histories:                  nil,
				}).Times(2)
			},
			mockApplyNonStartEventsPrepareBranchFn: func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				task replicationTask,
				newBranchManager newBranchManagerFn,
			) (bool, int, error) {
				return true, 2, nil
			},
			applyNonStartEventsPrepareMutableStateFnAffordance: func(mockExecutionMutableState *execution.MockMutableState) func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				branchIndex int,
				task replicationTask,
				newConflictResolver newConflictResolverFn,
			) (execution.MutableState, bool, error) {
				fn := func(ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					task replicationTask,
					newConflictResolver newConflictResolverFn,
				) (execution.MutableState, bool, error) {
					assert.Equal(t, branchIndex, 2)
					return mockExecutionMutableState, false, nil
				}
				return fn
			},
			expectError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// mock objects
			replicator := createTestHistoryReplicator(t, uuid.New())
			mockReplicationTask := NewMockreplicationTask(ctrl)
			mockExecutionCache := execution.NewMockCache(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockExecutionMutableState := execution.NewMockMutableState(ctrl)
			mockMetricsClient := metrics.NewNoopMetricsClient()
			replicator.executionCache = mockExecutionCache
			replicator.metricsClient = mockMetricsClient

			// mock functions
			test.mockReplicationTaskAffordance(mockReplicationTask)
			test.mockExecutionCacheAffordance(mockExecutionCache, mockExecutionContext)
			test.mockExecutionContextAffordance(mockExecutionContext, mockExecutionMutableState)
			test.mockMutableStateAffordance(mockExecutionMutableState)

			// parameter functions affordance
			replicator.applyNonStartEventsPrepareBranchFn = test.mockApplyNonStartEventsPrepareBranchFn
			replicator.applyNonStartEventsPrepareMutableStateFn = test.applyNonStartEventsPrepareMutableStateFnAffordance(mockExecutionMutableState)
			replicator.applyNonStartEventsToCurrentBranchFn = func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				isRebuilt bool,
				releaseFn execution.ReleaseFunc,
				task replicationTask,
				newStateBuilder newStateBuilderFn,
				clusterMetadata cluster.Metadata,
				shard shard.Context,
				logger log.Logger,
				transactionManager transactionManager,
			) error {
				return nil
			}
			replicator.applyNonStartEventsToNoneCurrentBranchFn = func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				branchIndex int,
				releaseFn execution.ReleaseFunc,
				task replicationTask,
				r *historyReplicatorImpl,
				logger log.Logger,
			) error {
				return nil
			}

			assert.Equal(t, replicator.applyEvents(ctx.Background(), mockReplicationTask), test.expectError)
		})
	}
}

func Test_applyEvents_defaultCase_errorAndDefault(t *testing.T) {
	workflowExecutionType := types.EventTypeWorkflowExecutionCompleted

	tests := map[string]struct {
		mockExecutionCacheAffordance                 func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext)
		mockReplicationTaskAffordance                func(mockReplicationTask *MockreplicationTask)
		mockExecutionContextAffordance               func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState)
		mockMutableStateAffordance                   func(mockExecutionMutableState *execution.MockMutableState)
		mockApplyNonStartEventsMissingMutableStateFn func(ctx ctx.Context,
			newContext execution.Context,
			task replicationTask,
			newWorkflowResetter newWorkflowResetterFn,
		) (execution.MutableState, error)
		expectError error
	}{
		"Case1-1: case EntityNotExistsError + applyNonStartEventsMissingMutableStateFn error": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, &types.EntityNotExistsError{}).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				return
			},
			mockApplyNonStartEventsMissingMutableStateFn: func(ctx ctx.Context,
				newContext execution.Context,
				task replicationTask,
				newWorkflowResetter newWorkflowResetterFn,
			) (execution.MutableState, error) {
				return nil, fmt.Errorf("test error")
			},
			expectError: fmt.Errorf("test error"),
		},
		"Case1-2: case EntityNotExistsError": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, &types.EntityNotExistsError{}).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				return
			},
			mockApplyNonStartEventsMissingMutableStateFn: func(ctx ctx.Context,
				newContext execution.Context,
				task replicationTask,
				newWorkflowResetter newWorkflowResetterFn,
			) (execution.MutableState, error) {
				return nil, nil
			},
			expectError: nil,
		},
		"Case1-3: case other errors": {
			mockExecutionCacheAffordance: func(mockExecutionCache *execution.MockCache, mockExecutionContext *execution.MockContext) {
				mockExecutionCache.EXPECT().GetOrCreateWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockExecutionContext, func(err error) {}, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					EventType: &workflowExecutionType,
				}).Times(1)
				mockReplicationTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockExecutionContextAffordance: func(mockExecutionContext *execution.MockContext, mockExecutionMutableState *execution.MockMutableState) {
				mockExecutionContext.EXPECT().LoadWorkflowExecutionWithTaskVersion(gomock.Any(), gomock.Any()).
					Return(mockExecutionMutableState, fmt.Errorf("test-error")).Times(1)
			},
			mockMutableStateAffordance: func(mockExecutionMutableState *execution.MockMutableState) {
				return
			},
			mockApplyNonStartEventsMissingMutableStateFn: func(ctx ctx.Context,
				newContext execution.Context,
				task replicationTask,
				newWorkflowResetter newWorkflowResetterFn,
			) (execution.MutableState, error) {
				return nil, nil
			},
			expectError: fmt.Errorf("test-error"),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// mock objects
			replicator := createTestHistoryReplicator(t, uuid.New())
			mockReplicationTask := NewMockreplicationTask(ctrl)
			mockExecutionCache := execution.NewMockCache(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockExecutionMutableState := execution.NewMockMutableState(ctrl)
			replicator.executionCache = mockExecutionCache

			// mock functions
			test.mockReplicationTaskAffordance(mockReplicationTask)
			test.mockExecutionCacheAffordance(mockExecutionCache, mockExecutionContext)
			test.mockExecutionContextAffordance(mockExecutionContext, mockExecutionMutableState)
			test.mockMutableStateAffordance(mockExecutionMutableState)

			// parameter functions affordance
			replicator.applyNonStartEventsMissingMutableStateFn = test.mockApplyNonStartEventsMissingMutableStateFn
			replicator.applyNonStartEventsResetWorkflowFn = func(ctx ctx.Context,
				context execution.Context,
				mutableState execution.MutableState,
				task replicationTask,
				newStateBuilder newStateBuilderFn,
				transactionManager transactionManager,
				clusterMetadata cluster.Metadata,
				logger log.Logger,
				shard shard.Context,
			) error {
				return nil
			}

			assert.Equal(t, replicator.applyEvents(ctx.Background(), mockReplicationTask), test.expectError)
		})
	}
}

func Test_applyStartEvents(t *testing.T) {
	tests := map[string]struct {
		mockDomainCacheAffordance        func(mockDomainCache *cache.MockDomainCache)
		mockStateBuilderAffordance       func(mockStateBuilder *execution.MockStateBuilder)
		mockReplicationTaskAffordance    func(mockReplicationTask *MockreplicationTask)
		mockTransactionManagerAffordance func(mockTransactionManager *MocktransactionManager)
		mockShardContextAffordance       func(mockShardContext *shard.MockContext)
		expectError                      error
	}{
		"Case1: success case with no errors": {
			mockDomainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).
					Return(cache.NewGlobalDomainCacheEntryForTest(nil, nil, nil, 1), nil).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder) {
				mockStateBuilder.EXPECT().ApplyEvents(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2)
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(2)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getEvents().Return(nil).Times(1)
				mockReplicationTask.EXPECT().getNewEvents().Return(nil).Times(1)
				mockReplicationTask.EXPECT().getEventTime().Return(time.Now()).Times(2)
				mockReplicationTask.EXPECT().getSourceCluster().Return("test-source-cluster").Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().createWorkflow(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockShardContextAffordance: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					NumberOfShards:                       0,
					IsAdvancedVisConfigExist:             false,
					MaxResponseSize:                      0,
					HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
					HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
					HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
					HostName:                             "test-host",
					StandbyClusterDelay:                  dynamicproperties.GetDurationPropertyFn(10),
					EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
				}).Times(1)
				mockShard.EXPECT().SetCurrentTime(gomock.Any(), gomock.Any()).Times(1)
			},
			expectError: nil,
		},
		"Case2: error when GetDomainByID fails": {
			mockDomainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).
					Return(nil, fmt.Errorf("test error")).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder) {},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {},
			mockShardContextAffordance:       func(mockShard *shard.MockContext) {},
			expectError:                      fmt.Errorf("test error"),
		},
		"Case3: error during createWorkflow": {
			mockDomainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).
					Return(cache.NewGlobalDomainCacheEntryForTest(nil, nil, nil, 1), nil).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder) {
				mockStateBuilder.EXPECT().ApplyEvents(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil, nil).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getLogger().Return(log.NewNoop()).Times(3)
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(2)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getEvents().Return(nil).Times(1)
				mockReplicationTask.EXPECT().getNewEvents().Return(nil).Times(1)
				mockReplicationTask.EXPECT().getEventTime().Return(time.Now()).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().createWorkflow(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("test error")).Times(1)
			},
			mockShardContextAffordance: func(mockShard *shard.MockContext) {},
			expectError:                fmt.Errorf("test error"),
		},
		"Case4: error when calling stateBuilder.ApplyEvents": {
			mockDomainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).
					Return(cache.NewGlobalDomainCacheEntryForTest(nil, nil, nil, 1), nil).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder) {
				mockStateBuilder.EXPECT().ApplyEvents(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil, fmt.Errorf("test error")).Times(1)
			},
			mockReplicationTaskAffordance: func(mockReplicationTask *MockreplicationTask) {
				mockReplicationTask.EXPECT().getLogger().Return(log.NewNoop()).Times(3)
				mockReplicationTask.EXPECT().getDomainID().Return("test-domain-id").Times(2)
				mockReplicationTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockReplicationTask.EXPECT().getEvents().Return(nil).Times(1)
				mockReplicationTask.EXPECT().getNewEvents().Return(nil).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {},
			mockShardContextAffordance:       func(mockShard *shard.MockContext) {},
			expectError:                      fmt.Errorf("test error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl)
			mockStateBuilder := execution.NewMockStateBuilder(ctrl)
			mockReplicationTask := NewMockreplicationTask(ctrl)
			mockTransactionManager := NewMocktransactionManager(ctrl)
			mockShard := shard.NewMockContext(ctrl)
			logger := log.NewNoop()

			// Mock affordances
			test.mockDomainCacheAffordance(mockDomainCache)
			test.mockStateBuilderAffordance(mockStateBuilder)
			test.mockReplicationTaskAffordance(mockReplicationTask)
			test.mockTransactionManagerAffordance(mockTransactionManager)
			test.mockShardContextAffordance(mockShard)

			// Mock functions
			mockNewMutableStateFn := func(
				domainEntry *cache.DomainCacheEntry,
				logger log.Logger,
			) execution.MutableState {
				return mockMutableState
			}

			mockNewStateBuilderFn := func(
				state execution.MutableState,
				logger log.Logger,
			) execution.StateBuilder {
				return mockStateBuilder
			}

			// Call the function under test
			err := applyStartEvents(ctx.Background(), mockExecutionContext, func(err error) {}, mockReplicationTask,
				mockDomainCache, mockNewMutableStateFn, mockNewStateBuilderFn, mockTransactionManager, logger, mockShard, cluster.Metadata{})

			// Assertions
			// can't change it to ErrorIs since ErrorIs need error chains
			assert.Equal(t, test.expectError, err)
		})
	}
}

func Test_applyNonStartEventsPrepareBranch(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance          func(mockTask *MockreplicationTask)
		mockBranchManagerAffordance func(mockBranchManager *MockbranchManager)
		mockNewBranchManagerFn      func(context execution.Context, mutableState execution.MutableState, logger log.Logger) branchManager
		expectDoContinue            bool
		expectVersionHistoryIndex   int
		expectError                 error
	}{
		"Case1: success case with no errors": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getVersionHistory().Return(&persistence.VersionHistory{}).Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(1)
				mockTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}).Times(2)
			},
			mockBranchManagerAffordance: func(mockBranchManager *MockbranchManager) {
				mockBranchManager.EXPECT().prepareVersionHistory(
					gomock.Any(),
					gomock.Any(),
					int64(1),
					int64(1),
				).Return(true, 5, nil).Times(1)
			},
			mockNewBranchManagerFn: func(context execution.Context, mutableState execution.MutableState, logger log.Logger) branchManager {
				mockBranchManager := NewMockbranchManager(gomock.NewController(t))
				mockBranchManager.EXPECT().prepareVersionHistory(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, 5, nil).Times(1)
				return mockBranchManager
			},
			expectDoContinue:          true,
			expectVersionHistoryIndex: 5,
			expectError:               nil,
		},
		"Case2: RetryTaskV2Error case": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getVersionHistory().Return(&persistence.VersionHistory{}).Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(1)
				mockTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}).Times(2)
			},
			mockBranchManagerAffordance: func(mockBranchManager *MockbranchManager) {
				mockBranchManager.EXPECT().prepareVersionHistory(
					gomock.Any(),
					gomock.Any(),
					int64(1),
					int64(1),
				).Return(false, 0, &types.RetryTaskV2Error{}).Times(1)
			},
			mockNewBranchManagerFn: func(context execution.Context, mutableState execution.MutableState, logger log.Logger) branchManager {
				mockBranchManager := NewMockbranchManager(gomock.NewController(t))
				mockBranchManager.EXPECT().prepareVersionHistory(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, 0, &types.RetryTaskV2Error{}).Times(1)
				return mockBranchManager
			},
			expectDoContinue:          false,
			expectVersionHistoryIndex: 0,
			expectError:               &types.RetryTaskV2Error{},
		},
		"Case3: unknown error case": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getVersionHistory().Return(&persistence.VersionHistory{}).Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2)
				mockTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}).Times(2)
			},
			mockBranchManagerAffordance: func(mockBranchManager *MockbranchManager) {
				mockBranchManager.EXPECT().prepareVersionHistory(
					gomock.Any(),
					gomock.Any(),
					int64(1),
					int64(1),
				).Return(false, 0, fmt.Errorf("test error")).Times(1)
			},
			mockNewBranchManagerFn: func(context execution.Context, mutableState execution.MutableState, logger log.Logger) branchManager {
				mockBranchManager := NewMockbranchManager(gomock.NewController(t))
				mockBranchManager.EXPECT().prepareVersionHistory(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, 0, fmt.Errorf("test error")).Times(1)
				return mockBranchManager
			},
			expectDoContinue:          false,
			expectVersionHistoryIndex: 0,
			expectError:               fmt.Errorf("test error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockTask := NewMockreplicationTask(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl)

			// Mock affordances
			test.mockTaskAffordance(mockTask)
			mockBranchManager := NewMockbranchManager(ctrl)
			test.mockBranchManagerAffordance(mockBranchManager)

			// Mock newBranchManagerFn
			newBranchManagerFn := func(context execution.Context, mutableState execution.MutableState, logger log.Logger) branchManager {
				return mockBranchManager
			}

			// Call the function under test
			doContinue, versionHistoryIndex, err := applyNonStartEventsPrepareBranch(ctx.Background(), mockExecutionContext, mockMutableState, mockTask, newBranchManagerFn)

			// Assertions
			assert.Equal(t, test.expectDoContinue, doContinue)
			assert.Equal(t, test.expectVersionHistoryIndex, versionHistoryIndex)
			assert.Equal(t, test.expectError, err)
		})
	}
}

func Test_applyNonStartEventsPrepareMutableState(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance             func(mockTask *MockreplicationTask)
		mockConflictResolverAffordance func(mockConflictResolver *MockconflictResolver)
		mockNewConflictResolverFn      func(context execution.Context, mutableState execution.MutableState, logger log.Logger) conflictResolver
		expectMutableState             execution.MutableState
		expectIsRebuilt                bool
		expectError                    error
	}{
		"Case1: success case with no errors": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getVersion().Return(int64(1)).Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(1)
			},
			mockConflictResolverAffordance: func(mockConflictResolver *MockconflictResolver) {
				mockMutableState := execution.NewMockMutableState(gomock.NewController(t)) // Define mockMutableState
				mockConflictResolver.EXPECT().prepareMutableState(
					gomock.Any(),
					5,        // branchIndex
					int64(1), // incomingVersion
				).Return(mockMutableState, true, nil).Times(1)
			},
			mockNewConflictResolverFn: func(context execution.Context, mutableState execution.MutableState, logger log.Logger) conflictResolver {
				mockConflictResolver := NewMockconflictResolver(gomock.NewController(t))
				return mockConflictResolver
			},
			expectMutableState: execution.NewMockMutableState(gomock.NewController(t)), // Use same mock for expectations
			expectIsRebuilt:    true,
			expectError:        nil,
		},
		"Case2: error during prepareMutableState": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getVersion().Return(int64(1)).Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2) // Error logging case
			},
			mockConflictResolverAffordance: func(mockConflictResolver *MockconflictResolver) {
				mockConflictResolver.EXPECT().prepareMutableState(
					gomock.Any(),
					5,        // branchIndex
					int64(1), // incomingVersion
				).Return(nil, false, fmt.Errorf("test error")).Times(1)
			},
			mockNewConflictResolverFn: func(context execution.Context, mutableState execution.MutableState, logger log.Logger) conflictResolver {
				mockConflictResolver := NewMockconflictResolver(gomock.NewController(t))
				return mockConflictResolver
			},
			expectMutableState: nil,
			expectIsRebuilt:    false,
			expectError:        fmt.Errorf("test error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockTask := NewMockreplicationTask(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl) // Define mockMutableState

			// Mock affordances
			test.mockTaskAffordance(mockTask)
			mockConflictResolver := NewMockconflictResolver(ctrl)
			test.mockConflictResolverAffordance(mockConflictResolver)

			// Mock newConflictResolverFn
			newConflictResolverFn := func(context execution.Context, mutableState execution.MutableState, logger log.Logger) conflictResolver {
				return mockConflictResolver
			}

			// Call the function under test
			mutableState, isRebuilt, err := applyNonStartEventsPrepareMutableState(ctx.Background(), mockExecutionContext, mockMutableState, 5, mockTask, newConflictResolverFn)

			// Assertions
			assert.Equal(t, test.expectMutableState, mutableState)
			assert.Equal(t, test.expectIsRebuilt, isRebuilt)
			assert.Equal(t, test.expectError, err)
		})
	}
}

func Test_applyNonStartEventsToCurrentBranch(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance               func(mockTask *MockreplicationTask)
		mockStateBuilderAffordance       func(mockStateBuilder *execution.MockStateBuilder, mockMutableState *execution.MockMutableState)
		mockTransactionManagerAffordance func(mockTransactionManager *MocktransactionManager)
		mockShardAffordance              func(mockShard *shard.MockContext)
		mockNewStateBuilderFn            func(mutableState execution.MutableState, logger log.Logger) execution.StateBuilder
		expectError                      error
	}{
		"Case1: success case with no errors": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(1)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(2)
				mockTask.EXPECT().getSourceCluster().Return("test-source-cluster").Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder, mockMutableState *execution.MockMutableState) {
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					DomainID:   "test-domain-id",
				}).Times(1)
				mockStateBuilder.EXPECT().ApplyEvents(
					"test-domain-id",
					gomock.Any(),
					types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					nil,
					nil,
				).Return(mockMutableState, nil).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().updateWorkflow(
					gomock.Any(),
					gomock.Any(),
					true,
					gomock.Any(),
					gomock.Any(),
				).Return(nil).Times(1)
			},
			mockShardAffordance: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().GetExecutionManager().Return(nil).Times(1)
				mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).Times(1)
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					NumberOfShards:                       0,
					IsAdvancedVisConfigExist:             false,
					MaxResponseSize:                      0,
					HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
					HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
					HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
					HostName:                             "test-host",
					StandbyClusterDelay:                  dynamicproperties.GetDurationPropertyFn(10),
					EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
				}).Times(1)
				mockShard.EXPECT().SetCurrentTime(gomock.Any(), gomock.Any()).Times(1)
			},
			mockNewStateBuilderFn: func(mutableState execution.MutableState, logger log.Logger) execution.StateBuilder {
				mockStateBuilder := execution.NewMockStateBuilder(gomock.NewController(t))
				return mockStateBuilder
			},
			expectError: nil,
		},
		"Case2: error during ApplyEvents": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder, mockMutableState *execution.MockMutableState) {
				mockStateBuilder.EXPECT().ApplyEvents(
					"test-domain-id",
					gomock.Any(),
					types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					nil,
					nil,
				).Return(nil, fmt.Errorf("test error")).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {},
			mockShardAffordance:              func(mockShard *shard.MockContext) {},
			mockNewStateBuilderFn: func(mutableState execution.MutableState, logger log.Logger) execution.StateBuilder {
				mockStateBuilder := execution.NewMockStateBuilder(gomock.NewController(t))
				return mockStateBuilder
			},
			expectError: fmt.Errorf("test error"),
		},
		"Case3: error during updateWorkflow": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder, mockMutableState *execution.MockMutableState) {
				mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					DomainID:   "test-domain-id",
				}).Times(1)
				mockStateBuilder.EXPECT().ApplyEvents(
					"test-domain-id",
					gomock.Any(),
					types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					nil,
					nil,
				).Return(mockMutableState, nil).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().updateWorkflow(
					gomock.Any(),
					gomock.Any(),
					true,
					gomock.Any(),
					gomock.Any(),
				).Return(fmt.Errorf("test update error")).Times(1)
			},
			mockShardAffordance: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().GetExecutionManager().Return(nil).Times(1)
				mockShard.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).Times(1)
			},
			mockNewStateBuilderFn: func(mutableState execution.MutableState, logger log.Logger) execution.StateBuilder {
				mockStateBuilder := execution.NewMockStateBuilder(gomock.NewController(t))
				return mockStateBuilder
			},
			expectError: fmt.Errorf("test update error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockTask := NewMockreplicationTask(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl)
			mockTransactionManager := NewMocktransactionManager(ctrl)
			mockShard := shard.NewMockContext(ctrl)
			logger := log.NewNoop()

			// Mock affordances
			test.mockTaskAffordance(mockTask)
			mockStateBuilder := execution.NewMockStateBuilder(ctrl)
			test.mockStateBuilderAffordance(mockStateBuilder, mockMutableState)
			test.mockTransactionManagerAffordance(mockTransactionManager)
			test.mockShardAffordance(mockShard)

			// Mock newStateBuilderFn
			newStateBuilderFn := func(mutableState execution.MutableState, logger log.Logger) execution.StateBuilder {
				return mockStateBuilder
			}

			// Call the function under test
			err := applyNonStartEventsToCurrentBranch(ctx.Background(), mockExecutionContext, mockMutableState, true, func(error) {}, mockTask, newStateBuilderFn, cluster.Metadata{}, mockShard, logger, mockTransactionManager)

			// Assertions
			assert.Equal(t, test.expectError, err)
		})
	}
}

func Test_applyNonStartEventsToNoneCurrentBranch(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance                                    func(mockTask *MockreplicationTask)
		mockApplyNonStartEventsWithContinueAsNewAffordance    func(replicator *historyReplicatorImpl)
		mockApplyNonStartEventsWithoutContinueAsNewAffordance func(replicator *historyReplicatorImpl)
		expectError                                           error
	}{
		"Case1: with NewEvents, should call applyNonStartEventsToNoneCurrentBranchWithContinueAsNew": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getNewEvents().Return([]*types.HistoryEvent{{}}).Times(1)
			},
			mockApplyNonStartEventsWithContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {
				replicator.applyNonStartEventsToNoneCurrentBranchWithContinueAsNewFn = func(
					ctx ctx.Context,
					context execution.Context,
					releaseFn execution.ReleaseFunc,
					task replicationTask,
					r *historyReplicatorImpl,
				) error {
					return nil
				}
			},
			mockApplyNonStartEventsWithoutContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {},
			expectError: nil,
		},
		"Case2: without NewEvents, should call applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNew": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
			},
			mockApplyNonStartEventsWithContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {},
			mockApplyNonStartEventsWithoutContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {
				replicator.applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNewFn = func(
					ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					releaseFn execution.ReleaseFunc,
					task replicationTask,
					transactionManager transactionManager,
					clusterMetadata cluster.Metadata,
					shard shard.Context,
					logger log.Logger,
				) error {
					return nil
				}
			},
			expectError: nil,
		},
		"Case3: error case for applyNonStartEventsToNoneCurrentBranchWithContinueAsNew": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getNewEvents().Return([]*types.HistoryEvent{{}}).Times(1)
			},
			mockApplyNonStartEventsWithContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {
				replicator.applyNonStartEventsToNoneCurrentBranchWithContinueAsNewFn = func(
					ctx ctx.Context,
					context execution.Context,
					releaseFn execution.ReleaseFunc,
					task replicationTask,
					r *historyReplicatorImpl,
				) error {
					return fmt.Errorf("test error")
				}
			},
			mockApplyNonStartEventsWithoutContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {},
			expectError: fmt.Errorf("test error"),
		},
		"Case4: error case for applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNew": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
			},
			mockApplyNonStartEventsWithContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {},
			mockApplyNonStartEventsWithoutContinueAsNewAffordance: func(replicator *historyReplicatorImpl) {
				replicator.applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNewFn = func(
					ctx ctx.Context,
					context execution.Context,
					mutableState execution.MutableState,
					branchIndex int,
					releaseFn execution.ReleaseFunc,
					task replicationTask,
					transactionManager transactionManager,
					clusterMetadata cluster.Metadata,
					shard shard.Context,
					logger log.Logger,
				) error {
					return fmt.Errorf("test error")
				}
			},
			expectError: fmt.Errorf("test error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockTask := NewMockreplicationTask(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl)
			mockReleaseFn := func(error) {}

			// Create the replicator using createTestHistoryReplicator
			replicator := createTestHistoryReplicator(t, uuid.New())

			// Mock affordances
			test.mockTaskAffordance(mockTask)
			test.mockApplyNonStartEventsWithContinueAsNewAffordance(&replicator)
			test.mockApplyNonStartEventsWithoutContinueAsNewAffordance(&replicator)

			// Call the function under test
			err := applyNonStartEventsToNoneCurrentBranch(ctx.Background(), mockExecutionContext, mockMutableState, 1, mockReleaseFn, mockTask, &replicator, replicator.logger)

			// Assertions
			assert.Equal(t, test.expectError, err)
		})
	}
}

func Test_applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNew(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance               func(mockTask *MockreplicationTask)
		mockMutableStateAffordance       func(mockMutableState *execution.MockMutableState) *persistence.VersionHistories
		mockTransactionManagerAffordance func(mockTransactionManager *MocktransactionManager)
		expectError                      error
	}{
		"Case1: success case with no errors": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLastEvent().Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}).Times(2)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(1)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(2)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
			},
			mockMutableStateAffordance: func(mockMutableState *execution.MockMutableState) *persistence.VersionHistories {
				// Create a VersionHistory and VersionHistories structure
				versionHistory := &persistence.VersionHistory{
					BranchToken: []byte("branch-token"),
					Items:       []*persistence.VersionHistoryItem{},
				}
				versionHistories := &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories:                  []*persistence.VersionHistory{versionHistory},
				}

				mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).Times(1)
				return versionHistories
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().backfillWorkflow(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					&persistence.WorkflowEvents{
						DomainID:    "test-domain-id",
						WorkflowID:  "test-workflow-id",
						RunID:       "test-run-id",
						BranchToken: []byte("branch-token"),
						Events:      nil,
					},
				).Return(nil).Times(1)
			},
			expectError: nil,
		},
		"Case2: missing version histories": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLastEvent().Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}).Times(2)
			},
			mockMutableStateAffordance: func(mockMutableState *execution.MockMutableState) *persistence.VersionHistories {
				mockMutableState.EXPECT().GetVersionHistories().Return(nil).Times(1)
				return nil
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {},
			expectError:                      execution.ErrMissingVersionHistories,
		},
		"Case3: error when adding or updating version history item": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLastEvent().Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}).Times(2)
			},
			mockMutableStateAffordance: func(mockMutableState *execution.MockMutableState) *persistence.VersionHistories {
				// Create a VersionHistory and VersionHistories structure with a higher event ID and version
				versionHistory := &persistence.VersionHistory{
					BranchToken: []byte("branch-token"),
					Items: []*persistence.VersionHistoryItem{
						{
							EventID: 2, // Higher event ID
							Version: 2, // Higher version
						},
					},
				}
				versionHistories := &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories:                  []*persistence.VersionHistory{versionHistory},
				}

				mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).Times(1)
				return versionHistories
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {},
			expectError:                      &types.BadRequestError{Message: "cannot update version history with a lower version 1. Last version: 2"},
		},
		"Case4: error during backfill workflow": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLastEvent().Return(&types.HistoryEvent{
					ID:      1,
					Version: 1,
				}).Times(2)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(1)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(2)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(1)
			},
			mockMutableStateAffordance: func(mockMutableState *execution.MockMutableState) *persistence.VersionHistories {
				versionHistory := &persistence.VersionHistory{
					BranchToken: []byte("branch-token"),
					Items:       []*persistence.VersionHistoryItem{},
				}
				versionHistories := &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories:                  []*persistence.VersionHistory{versionHistory},
				}

				mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).Times(1)
				return versionHistories
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().backfillWorkflow(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					&persistence.WorkflowEvents{
						DomainID:    "test-domain-id",
						WorkflowID:  "test-workflow-id",
						RunID:       "test-run-id",
						BranchToken: []byte("branch-token"),
						Events:      nil,
					},
				).Return(fmt.Errorf("backfill error")).Times(1)
			},
			expectError: fmt.Errorf("backfill error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockTask := NewMockreplicationTask(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl)
			mockTransactionManager := NewMocktransactionManager(ctrl)
			mockReleaseFn := func(error) {}

			// Mock affordances
			test.mockMutableStateAffordance(mockMutableState)
			test.mockTaskAffordance(mockTask)
			test.mockTransactionManagerAffordance(mockTransactionManager)

			mockShard := shard.NewMockContext(ctrl)
			// Call the function under test
			err := applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNew(
				ctx.Background(),
				mockExecutionContext,
				mockMutableState,
				0, // Ensure branchIndex is valid
				mockReleaseFn,
				mockTask,
				mockTransactionManager,
				cluster.Metadata{},
				mockShard,
				testlogger.New(t),
			)

			// Assertions
			assert.Equal(t, test.expectError, err)
		})
	}
}

func Test_applyNonStartEventsMissingMutableState(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance             func(mockTask *MockreplicationTask)
		mockWorkflowResetterAffordance func(mockWorkflowResetter *MockWorkflowResetter)
		mockNewWorkflowResetterFn      func(mockTask *MockreplicationTask, mockWorkflowResetter *MockWorkflowResetter) newWorkflowResetterFn
		expectMutableState             execution.MutableState
		validateError                  func(t *testing.T, err error)
	}{
		"Case1: non-reset workflow, should return retry task error": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().isWorkflowReset().Return(false).Times(1)
				mockTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					ID:      10,
					Version: 1,
				}).Times(1)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getWorkflowID().Return("test-workflow-id").Times(1)
				mockTask.EXPECT().getRunID().Return("test-run-id").Times(1)
			},
			mockWorkflowResetterAffordance: func(mockWorkflowResetter *MockWorkflowResetter) {},
			mockNewWorkflowResetterFn: func(mockTask *MockreplicationTask, mockWorkflowResetter *MockWorkflowResetter) newWorkflowResetterFn {
				return nil
			},
			expectMutableState: nil,
			validateError: func(t *testing.T, err error) {
				assert.IsType(t, &types.RetryTaskV2Error{}, err)
				retryError := err.(*types.RetryTaskV2Error)
				assert.Equal(t, "Resend events due to missing mutable state", retryError.Message)
				assert.Equal(t, "test-domain-id", retryError.DomainID)
				assert.Equal(t, "test-workflow-id", retryError.WorkflowID)
				assert.Equal(t, "test-run-id", retryError.RunID)
			},
		},
		"Case2: reset workflow, should return reset mutable state": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().isWorkflowReset().Return(true).Times(1)
				mockTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					ID:      10,
					Version: 1,
				}).Times(2)
				mockTask.EXPECT().getWorkflowResetMetadata().Return("base-run-id", "new-run-id", int64(1), false).Times(1)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getWorkflowID().Return("test-workflow-id").Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(1)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(1)
				mockTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockWorkflowResetterAffordance: func(mockWorkflowResetter *MockWorkflowResetter) {
				mockMutableState := execution.NewMockMutableState(gomock.NewController(t)) // Ensure consistent MutableState
				mockWorkflowResetter.EXPECT().ResetWorkflow(
					gomock.Any(),
					gomock.Any(),
					int64(9),
					int64(1),
					int64(10),
					int64(1),
				).Return(mockMutableState, nil).Times(1)
			},
			mockNewWorkflowResetterFn: func(mockTask *MockreplicationTask, mockWorkflowResetter *MockWorkflowResetter) newWorkflowResetterFn {
				// Return the already created mockWorkflowResetter
				return func(domainID, workflowID, baseRunID string, newContext execution.Context, newRunID string, logger log.Logger) WorkflowResetter {
					return mockWorkflowResetter
				}
			},
			expectMutableState: execution.NewMockMutableState(gomock.NewController(t)), // Match returned value
			validateError:      func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		"Case3: error during workflow reset": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().isWorkflowReset().Return(true).Times(1)
				mockTask.EXPECT().getFirstEvent().Return(&types.HistoryEvent{
					ID:      10,
					Version: 1,
				}).Times(2)
				mockTask.EXPECT().getWorkflowResetMetadata().Return("base-run-id", "new-run-id", int64(1), false).Times(1)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getWorkflowID().Return("test-workflow-id").Times(1)
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(1)
				mockTask.EXPECT().getVersion().Return(int64(1)).Times(1)
			},
			mockWorkflowResetterAffordance: func(mockWorkflowResetter *MockWorkflowResetter) {
				mockWorkflowResetter.EXPECT().ResetWorkflow(
					gomock.Any(),
					gomock.Any(),
					int64(9),
					int64(1),
					int64(10),
					int64(1),
				).Return(nil, fmt.Errorf("reset error")).Times(1)
			},
			mockNewWorkflowResetterFn: func(mockTask *MockreplicationTask, mockWorkflowResetter *MockWorkflowResetter) newWorkflowResetterFn {
				// Return the already created mockWorkflowResetter
				return func(domainID, workflowID, baseRunID string, newContext execution.Context, newRunID string, logger log.Logger) WorkflowResetter {
					return mockWorkflowResetter
				}
			},
			expectMutableState: nil,
			validateError:      func(t *testing.T, err error) { assert.EqualError(t, err, "reset error") },
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockTask := NewMockreplicationTask(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockWorkflowResetter := NewMockWorkflowResetter(ctrl)

			// Mock affordances
			test.mockTaskAffordance(mockTask)
			test.mockWorkflowResetterAffordance(mockWorkflowResetter)
			newWorkflowResetterFn := test.mockNewWorkflowResetterFn(mockTask, mockWorkflowResetter)

			// Call the function under test
			mutableState, err := applyNonStartEventsMissingMutableState(
				ctx.Background(),
				mockExecutionContext,
				mockTask,
				newWorkflowResetterFn,
			)

			// Assertions
			assert.Equal(t, test.expectMutableState, mutableState)
			test.validateError(t, err)
		})
	}
}

func Test_applyNonStartEventsResetWorkflow(t *testing.T) {
	tests := map[string]struct {
		mockTaskAffordance               func(mockTask *MockreplicationTask)
		mockStateBuilderAffordance       func(mockStateBuilder *execution.MockStateBuilder)
		mockTransactionManagerAffordance func(mockTransactionManager *MocktransactionManager)
		mockShardContextAffordance       func(mockShard *shard.MockContext)
		expectError                      error
	}{
		"Case1: success case with no errors": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(1)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(2)
				mockTask.EXPECT().getSourceCluster().Return("test-source-cluster").Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder) {
				mockStateBuilder.EXPECT().ApplyEvents(
					"test-domain-id",
					gomock.Any(),
					types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					nil,
					nil,
				).Return(nil, nil).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().createWorkflow(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil).Times(1)
			},
			mockShardContextAffordance: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					NumberOfShards:                       0,
					IsAdvancedVisConfigExist:             false,
					MaxResponseSize:                      0,
					HistoryCacheInitialSize:              dynamicproperties.GetIntPropertyFn(10),
					HistoryCacheMaxSize:                  dynamicproperties.GetIntPropertyFn(10),
					HistoryCacheTTL:                      dynamicproperties.GetDurationPropertyFn(10),
					HostName:                             "test-host",
					StandbyClusterDelay:                  dynamicproperties.GetDurationPropertyFn(10),
					EnableSizeBasedHistoryExecutionCache: dynamicproperties.GetBoolPropertyFn(false),
				}).Times(1)
				mockShard.EXPECT().SetCurrentTime(gomock.Any(), gomock.Any()).Times(1)
			},
			expectError: nil,
		},
		"Case2: error during ApplyEvents": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder) {
				mockStateBuilder.EXPECT().ApplyEvents(
					"test-domain-id",
					gomock.Any(),
					types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					nil,
					nil,
				).Return(nil, fmt.Errorf("applyEvents error")).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {},
			mockShardContextAffordance:       func(mockShard *shard.MockContext) {},
			expectError:                      fmt.Errorf("applyEvents error"),
		},
		"Case3: error during createWorkflow": {
			mockTaskAffordance: func(mockTask *MockreplicationTask) {
				mockTask.EXPECT().getLogger().Return(log.NewNoop()).Times(2)
				mockTask.EXPECT().getDomainID().Return("test-domain-id").Times(1)
				mockTask.EXPECT().getExecution().Return(&types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				}).Times(1)
				mockTask.EXPECT().getEvents().Return(nil).Times(1)
				mockTask.EXPECT().getNewEvents().Return(nil).Times(1)
				mockTask.EXPECT().getEventTime().Return(time.Now()).Times(1)
			},
			mockStateBuilderAffordance: func(mockStateBuilder *execution.MockStateBuilder) {
				mockStateBuilder.EXPECT().ApplyEvents(
					"test-domain-id",
					gomock.Any(),
					types.WorkflowExecution{
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					nil,
					nil,
				).Return(nil, nil).Times(1)
			},
			mockTransactionManagerAffordance: func(mockTransactionManager *MocktransactionManager) {
				mockTransactionManager.EXPECT().createWorkflow(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(fmt.Errorf("createWorkflow error")).Times(1)
			},
			mockShardContextAffordance: func(mockShard *shard.MockContext) {},
			expectError:                fmt.Errorf("createWorkflow error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Mock objects
			mockTask := NewMockreplicationTask(ctrl)
			mockExecutionContext := execution.NewMockContext(ctrl)
			mockMutableState := execution.NewMockMutableState(ctrl)
			mockStateBuilder := execution.NewMockStateBuilder(ctrl)
			mockTransactionManager := NewMocktransactionManager(ctrl)
			mockShard := shard.NewMockContext(ctrl)
			logger := log.NewNoop()

			// Mock affordances
			test.mockTaskAffordance(mockTask)
			test.mockStateBuilderAffordance(mockStateBuilder)
			test.mockTransactionManagerAffordance(mockTransactionManager)
			test.mockShardContextAffordance(mockShard)

			// Mock functions
			mockNewStateBuilderFn := func(mutableState execution.MutableState, logger log.Logger) execution.StateBuilder {
				return mockStateBuilder
			}

			// Call the function under test
			err := applyNonStartEventsResetWorkflow(
				ctx.Background(),
				mockExecutionContext,
				mockMutableState,
				mockTask,
				mockNewStateBuilderFn,
				mockTransactionManager,
				cluster.Metadata{},
				logger,
				mockShard,
			)

			// Assertions
			assert.Equal(t, test.expectError, err)
		})
	}
}

func Test_notify(t *testing.T) {
	tests := map[string]struct {
		clusterName          string
		now                  time.Time
		currentClusterName   string
		primaryClusterName   string
		expectSetCurrentTime bool
	}{
		"Case1: event from current cluster, should log a warning": {
			clusterName:          "current-cluster",
			now:                  time.Now(),
			currentClusterName:   "current-cluster",
			primaryClusterName:   "primary-cluster",
			expectSetCurrentTime: false,
		},
		"Case2: event from different cluster, should update shard time": {
			clusterName:          "other-cluster",
			now:                  time.Now(),
			currentClusterName:   "current-cluster",
			primaryClusterName:   "primary-cluster",
			expectSetCurrentTime: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create ClusterInformation instances
			clusterGroup := map[string]commonConfig.ClusterInformation{
				"current-cluster": {
					Enabled:                true,
					InitialFailoverVersion: 1,
					RPCName:                "current-cluster-rpc",
					RPCAddress:             "127.0.0.1:8080",
					RPCTransport:           "tchannel",
				},
				"other-cluster": {
					Enabled:                true,
					InitialFailoverVersion: 2,
					RPCName:                "other-cluster-rpc",
					RPCAddress:             "127.0.0.1:8081",
					RPCTransport:           "grpc",
				},
			}

			// Create Metadata instance
			clusterMetadata := cluster.NewMetadata(
				commonConfig.ClusterGroupMetadata{
					FailoverVersionIncrement: 1,
					PrimaryClusterName:       test.primaryClusterName,
					CurrentClusterName:       test.currentClusterName,
					ClusterGroup:             clusterGroup,
				},
				dynamicproperties.GetBoolPropertyFnFilteredByDomain(false),
				metrics.NewNoopMetricsClient(),
				log.NewNoop(),
			)

			// Mock Shard Context
			mockShard := shard.NewMockContext(ctrl)
			if test.expectSetCurrentTime {
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					StandbyClusterDelay: dynamicproperties.GetDurationPropertyFn(5 * time.Minute),
				}).Times(1)
				mockShard.EXPECT().SetCurrentTime(test.clusterName, gomock.Any()).Times(1)
			}

			// Use Noop logger
			logger := log.NewNoop()

			// Call the function under test
			notify(
				test.clusterName,
				test.now,
				logger,
				mockShard,
				clusterMetadata,
			)
		})
	}
}
