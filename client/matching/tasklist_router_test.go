package matching

import (
	"testing"

	"github.com/cadence-workflow/shard-manager/service/sharddistributor/client/spectatorclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

func TestTaskListRouter_FromTaskList(t *testing.T) {
	tests := []struct {
		name                 string
		shardTaskListsByHash bool
		wantShardID          string
	}{
		{
			name:                 "shard per task list uses the task list name",
			shardTaskListsByHash: false,
			wantShardID:          "test-key",
		},
		{
			name:                 "shard by hash uses the hashed shard",
			shardTaskListsByHash: true,
			wantShardID:          "7126", // farm.Fingerprint64([]byte("test-key"))%8192
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _, shardDistributorMock := newTestTaskListRouter(t)
			router.taskListDifferentiator = NewTaskListDifferentiator(
				metrics.NewNoopMetricsClient(),
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(tt.shardTaskListsByHash),
				dynamicproperties.GetIntPropertyFn(100),
				config.DefaultNumMatchingShards,
			)

			shardDistributorMock.EXPECT().GetShardOwner(gomock.Any(), tt.wantShardID).
				Return(&spectatorclient.ShardOwner{
					ExecutorID: "test-owner",
					Metadata: map[string]string{
						"hostIP":   "127.0.0.1",
						"tchannel": "7933",
						"grpc":     "7833",
					},
				}, nil)

			host, err := router.FromTaskList("test-key")
			require.NoError(t, err)
			assert.Equal(t, "127.0.0.1:7933", host.GetAddress())
		})
	}
}

func TestTaskListRouter_FromTaskList_RecordsRoutingPath(t *testing.T) {
	tests := []struct {
		name                string
		percentageOnboarded int
		withSpectator       bool
		expectedPath        string
		wantAddress         string
	}{
		{
			name:                "shard distributor",
			percentageOnboarded: 100,
			withSpectator:       true,
			expectedPath:        routingPathShardDistributor,
			wantAddress:         "127.0.0.1:7933",
		},
		{
			name:                "excluded task list",
			percentageOnboarded: 0,
			withSpectator:       true,
			expectedPath:        routingPathHashRing,
			wantAddress:         "hash-ring-addr",
		},
		{
			name:                "missing spectator",
			percentageOnboarded: 100,
			withSpectator:       false,
			expectedPath:        routingPathHashRing,
			wantAddress:         "hash-ring-addr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := membership.NewMockResolver(ctrl)

			var spectator spectatorclient.Spectator
			if tt.withSpectator {
				spectatorMock := spectatorclient.NewMockSpectator(ctrl)
				spectator = spectatorMock
				if tt.percentageOnboarded > 0 {
					spectatorMock.EXPECT().GetShardOwner(gomock.Any(), "test-key").Return(&spectatorclient.ShardOwner{
						ExecutorID: "test-owner",
						Metadata: map[string]string{
							"hostIP":   "127.0.0.1",
							"tchannel": "7933",
						},
					}, nil)
				}
			}
			if tt.percentageOnboarded == 0 || !tt.withSpectator {
				resolver.EXPECT().Lookup(service.Matching, "test-key").Return(membership.NewHostInfo("hash-ring-addr"), nil)
			}

			testScope := tally.NewTestScope("", nil)
			metricsClient := metrics.NewClient(testScope, metrics.Matching, metrics.MigrationConfig{})
			taskListDifferentiator := NewTaskListDifferentiator(
				metrics.NewNoopMetricsClient(),
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
				config.DefaultNumMatchingShards,
			)
			router := NewTaskListRouter(
				resolver,
				taskListDifferentiator,
				spectator,
				log.NewNoop(),
				metricsClient.Scope(metrics.ShardDistributorResolverScope),
			)

			host, err := router.FromTaskList("test-key")
			require.NoError(t, err)
			assert.Equal(t, tt.wantAddress, host.GetAddress())

			counters := testScope.Snapshot().Counters()
			assert.Len(t, counters, 1)
			for _, counter := range counters {
				assert.Equal(t, "shard_distributor_resolver_lookups", counter.Name())
				assert.Equal(t, tt.expectedPath, counter.Tags()["routing_path"])
				assert.Equal(t, int64(1), counter.Value())
			}
		})
	}
}

func newTestTaskListRouter(t *testing.T) (*taskListRouter, *membership.MockResolver, *spectatorclient.MockSpectator) {
	ctrl := gomock.NewController(t)
	spectator := spectatorclient.NewMockSpectator(ctrl)
	excludeShortLivedTaskLists := dynamicproperties.GetBoolPropertyFn(false)
	resolver := membership.NewMockResolver(ctrl)
	logger := log.NewNoop()

	taskListDifferentiator := NewTaskListDifferentiator(
		metrics.NewNoopMetricsClient(),
		excludeShortLivedTaskLists,
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetIntPropertyFn(100),
		config.DefaultNumMatchingShards,
	)
	router := NewTaskListRouter(
		resolver,
		taskListDifferentiator,
		spectator,
		logger,
		metrics.NewNoopMetricsClient().Scope(metrics.ShardDistributorResolverScope),
	).(*taskListRouter)

	return router, resolver, spectator
}
