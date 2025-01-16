// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package testdata

import (
	"testing"

	oldgomock "github.com/golang/mock/gomock" // client library cannot change from old gomock
	"github.com/stretchr/testify/mock"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/.gen/go/cadence/workflowservicetest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/failover"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/replication"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/history/workflowcache"
)

type EngineForTest struct {
	Engine engine.Engine
	// Add mocks or other fields here
	ShardCtx *shard.TestContext
}

// NewEngineFn is defined as an alias for engineimpl.NewEngineWithShardContext to avoid circular dependency
type NewEngineFn func(
	shard shard.Context,
	visibilityMgr persistence.VisibilityManager,
	matching matching.Client,
	publicClient workflowserviceclient.Interface,
	historyEventNotifier events.Notifier,
	config *config.Config,
	replicationTaskFetchers replication.TaskFetchers,
	rawMatchingClient matching.Client,
	queueTaskProcessor task.Processor,
	failoverCoordinator failover.Coordinator,
	wfIDCache workflowcache.WFCache,
	queueProcessorFactory queue.ProcessorFactory,
) engine.Engine

func NewEngineForTest(t *testing.T, newEngineFn NewEngineFn) *EngineForTest {
	t.Helper()
	controller := gomock.NewController(t)
	historyCfg := config.NewForTest()
	shardCtx := shard.NewTestContext(
		t,
		controller,
		&persistence.ShardInfo{
			RangeID:          1,
			TransferAckLevel: 0,
		},
		historyCfg,
	)

	domainCache := shardCtx.Resource.DomainCache
	domainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestLocalDomainEntry, nil).AnyTimes()
	domainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()
	domainCache.EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil).AnyTimes()
	domainCache.EXPECT().GetDomainID(constants.TestDomainName).Return(constants.TestDomainID, nil).AnyTimes()
	domainCache.EXPECT().RegisterDomainChangeCallback(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	domainCache.EXPECT().UnregisterDomainChangeCallback(gomock.Any()).Times(1)

	executionMgr := shardCtx.Resource.ExecutionMgr
	// RangeCompleteReplicationTask is called by taskProcessorImpl's background loop
	executionMgr.
		On("RangeCompleteReplicationTask", mock.Anything, mock.Anything).
		Return(&persistence.RangeCompleteReplicationTaskResponse{}, nil)

	membershipResolver := shardCtx.Resource.MembershipResolver
	membershipResolver.EXPECT().MemberCount(gomock.Any()).Return(1, nil).AnyTimes()

	eventsCache := events.NewCache(
		shardCtx.GetShardID(),
		shardCtx.GetHistoryManager(),
		historyCfg,
		shardCtx.GetLogger(),
		shardCtx.GetMetricsClient(),
		domainCache,
	)
	shardCtx.SetEventsCache(eventsCache)

	historyEventNotifier := events.NewNotifier(
		clock.NewRealTimeSource(),
		shardCtx.Resource.MetricsClient,
		func(workflowID string) int {
			return len(workflowID)
		},
	)

	// client library is still using the old deprecated gomock, as changing it would be a rather large breaking change.
	// not really an issue, just use a separate controller for it - it'll still require everything via t.Cleanup.
	mockWFServiceClient := workflowservicetest.NewMockClient(oldgomock.NewController(t))

	replicatonTaskFetchers := replication.NewMockTaskFetchers(controller)
	replicationTaskFetcher := replication.NewMockTaskFetcher(controller)
	// TODO: this should probably return another cluster name, not current
	replicationTaskFetcher.EXPECT().GetSourceCluster().Return(constants.TestClusterMetadata.GetCurrentClusterName()).AnyTimes()
	replicationTaskFetcher.EXPECT().GetRateLimiter().Return(quotas.NewDynamicRateLimiter(func() float64 { return 100 })).AnyTimes()
	replicationTaskFetcher.EXPECT().GetRequestChan().Return(nil).AnyTimes()
	replicatonTaskFetchers.EXPECT().GetFetchers().Return([]replication.TaskFetcher{replicationTaskFetcher}).AnyTimes()

	queueTaskProcessor := task.NewMockProcessor(controller)
	queueTaskProcessor.EXPECT().StopShardProcessor(gomock.Any()).Return().Times(1)

	failoverCoordinator := failover.NewMockCoordinator(controller)
	wfIDCache := workflowcache.NewMockWFCache(controller)

	queueProcessorFactory := queue.NewMockProcessorFactory(controller)
	timerQProcessor := queue.NewMockProcessor(controller)
	timerQProcessor.EXPECT().Start().Return().Times(1)
	timerQProcessor.EXPECT().NotifyNewTask(gomock.Any(), gomock.Any()).Return().AnyTimes()
	timerQProcessor.EXPECT().Stop().Return().Times(1)
	queueProcessorFactory.EXPECT().
		NewTimerQueueProcessor(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(timerQProcessor).
		Times(1)

	transferQProcessor := queue.NewMockProcessor(controller)
	transferQProcessor.EXPECT().Start().Return().Times(1)
	transferQProcessor.EXPECT().NotifyNewTask(gomock.Any(), gomock.Any()).Return().AnyTimes()
	transferQProcessor.EXPECT().Stop().Return().Times(1)
	queueProcessorFactory.EXPECT().
		NewTransferQueueProcessor(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(transferQProcessor).
		Times(1)

	engine := newEngineFn(
		shardCtx,
		shardCtx.Resource.VisibilityMgr,
		shardCtx.Resource.MatchingClient,
		mockWFServiceClient,
		historyEventNotifier,
		historyCfg,
		replicatonTaskFetchers,
		shardCtx.Resource.MatchingClient,
		queueTaskProcessor,
		failoverCoordinator,
		wfIDCache,
		queueProcessorFactory,
	)

	shardCtx.SetEngine(engine)

	historyEventNotifier.Start()
	t.Cleanup(historyEventNotifier.Stop)

	return &EngineForTest{
		Engine:   engine,
		ShardCtx: shardCtx,
	}
}
