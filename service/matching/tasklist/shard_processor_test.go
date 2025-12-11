package tasklist

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

func paramsForTaskListManager(t *testing.T, taskListID *Identifier, taskListKind types.TaskListKind) ManagerParams {
	ctrl := gomock.NewController(t)
	dynamicClient := dynamicconfig.NewInMemoryClient()
	logger := testlogger.New(t)
	metricsClient := metrics.NewNoopMetricsClient()
	clusterMetadata := cluster.GetTestClusterMetadata(true)
	deps := &mockDeps{
		mockDomainCache:    cache.NewMockDomainCache(ctrl),
		mockTaskManager:    persistence.NewMockTaskManager(ctrl),
		mockIsolationState: isolationgroup.NewMockState(ctrl),
		mockMatchingClient: matching.NewMockClient(ctrl),
		mockTimeSource:     clock.NewMockedTimeSource(),
		dynamicClient:      dynamicClient,
	}
	deps.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("domainName", nil).Times(1)
	cfg := config.NewConfig(dynamicconfig.NewCollection(dynamicClient, logger), "hostname", getIsolationgroupsHelper)
	mockHistoryService := history.NewMockClient(ctrl)
	params := ManagerParams{
		deps.mockDomainCache,
		logger,
		metricsClient,
		deps.mockTaskManager,
		clusterMetadata,
		deps.mockIsolationState,
		deps.mockMatchingClient,
		func(Manager) {},
		func(Manager) {},
		func(Manager) {},
		taskListID,
		taskListKind,
		cfg,
		deps.mockTimeSource,
		deps.mockTimeSource.Now(),
		mockHistoryService,
	}
	return params
}

func TestNewShardProcessor(t *testing.T) {
	t.Run("NewShardProcessor fails with empty params", func(t *testing.T) {
		params := ManagerParams{}
		sp, err := NewShardProcessor(params)
		require.Nil(t, sp)
		require.Error(t, err)
	})

	t.Run("NewShardProcessor success", func(t *testing.T) {
		tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
		require.NoError(t, err)
		params := paramsForTaskListManager(t, tlID, types.TaskListKindNormal)
		sp, err := NewShardProcessor(params)
		require.NoError(t, err)
		require.NotNil(t, sp)
	})
}

func TestGetShardReport(t *testing.T) {
	t.Run("GetShardReport success", func(t *testing.T) {
		mockManger := NewMockManager(gomock.NewController(t))
		mockManger.EXPECT().LoadBalancerHints().Return(&types.LoadBalancerHints{BacklogCount: 0, RatePerSecond: 10}).Times(1)
		sp := &shardProcessorImpl{
			Manager: mockManger,
		}
		shardReport := sp.GetShardReport()
		require.NotNil(t, shardReport)
		require.Equal(t, float64(10), shardReport.ShardLoad)
		require.Equal(t, types.ShardStatusINVALID, shardReport.Status)
	})
}

func TestSetShardStatus(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("SetShardStatus success", func(t *testing.T) {
		mockManger := NewMockManager(gomock.NewController(t))
		mockManger.EXPECT().LoadBalancerHints().Return(&types.LoadBalancerHints{BacklogCount: 0, RatePerSecond: 10}).Times(1)
		sp := &shardProcessorImpl{
			Manager: mockManger,
		}
		sp.SetShardStatus(types.ShardStatusREADY)
		shardReport := sp.GetShardReport()
		require.NotNil(t, shardReport)
		require.Equal(t, float64(10), shardReport.ShardLoad)
		require.Equal(t, types.ShardStatusREADY, shardReport.Status)
	})
}
