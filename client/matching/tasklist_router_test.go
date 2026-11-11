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
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(tt.shardTaskListsByHash),
				membership.StaticPercentageOnboarded(100),
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

func TestTaskListRouter_FromTaskList_ExcludedFallsBackToHashRing(t *testing.T) {
	router, resolver, _ := newTestTaskListRouter(t)
	// Set percentage to 0 so all task lists are excluded
	router.taskListDifferentiator = NewTaskListDifferentiator(
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetBoolPropertyFn(false),
		membership.StaticPercentageOnboarded(0),
		config.DefaultNumMatchingShards,
	)

	resolver.EXPECT().Lookup(service.Matching, "test-key").Return(membership.NewHostInfo("test-addr"), nil)

	host, err := router.FromTaskList("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-addr", host.GetAddress())
}

func TestTaskListRouter_FromTaskList_NilSpectatorFallsBackToHashRing(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := membership.NewMockResolver(ctrl)
	logger := log.NewNoop()
	pct := membership.NewMockPercentageOnboarded(ctrl)
	pct.EXPECT().Value().Return(100).AnyTimes()

	taskListDifferentiator := NewTaskListDifferentiator(
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetBoolPropertyFn(false),
		pct,
		config.DefaultNumMatchingShards,
	)
	router := NewTaskListRouter(
		resolver,
		taskListDifferentiator,
		nil, // nil spectator
		logger,
		metrics.NewNoopMetricsClient().Scope(metrics.ShardDistributorResolverScope),
	).(*taskListRouter)

	resolver.EXPECT().Lookup(service.Matching, "test-key").Return(membership.NewHostInfo("hash-ring-addr"), nil)

	host, err := router.FromTaskList("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "hash-ring-addr", host.GetAddress())
}

func TestTaskListRouter_FromTaskList_RecordsRoutingPath(t *testing.T) {
	tests := []struct {
		name                string
		percentageOnboarded int
		withSpectator       bool
		expectedPath        string
	}{
		{
			name:                "shard distributor",
			percentageOnboarded: 100,
			withSpectator:       true,
			expectedPath:        routingPathShardDistributor,
		},
		{
			name:                "excluded task list",
			percentageOnboarded: 0,
			withSpectator:       true,
			expectedPath:        routingPathHashRing,
		},
		{
			name:                "missing spectator",
			percentageOnboarded: 100,
			withSpectator:       false,
			expectedPath:        routingPathHashRing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := membership.NewMockResolver(ctrl)
			percentageOnboarded := membership.NewMockPercentageOnboarded(ctrl)
			percentageOnboarded.EXPECT().Value().Return(tt.percentageOnboarded)

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
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(false),
				percentageOnboarded,
				config.DefaultNumMatchingShards,
			)
			router := NewTaskListRouter(
				resolver,
				taskListDifferentiator,
				spectator,
				log.NewNoop(),
				metricsClient.Scope(metrics.ShardDistributorResolverScope),
			)

			_, err := router.FromTaskList("test-key")
			assert.NoError(t, err)

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

func TestTaskListRouter_FromTaskList_ExcludeShortLivedTaskLists(t *testing.T) {
	cases := []struct {
		name                       string
		excludeShortLivedTaskLists bool
		taskListName               string
		expectHashRing             bool
	}{
		{
			name:                       "exclude enabled with UUID tasklist uses hash ring",
			excludeShortLivedTaskLists: true,
			taskListName:               "tasklist-550e8400-e29b-41d4-a716-446655440000",
			expectHashRing:             true,
		},
		{
			name:                       "exclude enabled without UUID tasklist uses shard distributor",
			excludeShortLivedTaskLists: true,
			taskListName:               "my-regular-tasklist",
			expectHashRing:             false,
		},
		{
			name:                       "exclude disabled with UUID tasklist uses shard distributor",
			excludeShortLivedTaskLists: false,
			taskListName:               "tasklist-550e8400-e29b-41d4-a716-446655440000",
			expectHashRing:             false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			spectator := spectatorclient.NewMockSpectator(ctrl)
			resolver := membership.NewMockResolver(ctrl)
			logger := log.NewNoop()
			pct := membership.NewMockPercentageOnboarded(ctrl)
			pct.EXPECT().Value().Return(100).AnyTimes()

			taskListDifferentiator := NewTaskListDifferentiator(
				dynamicproperties.GetBoolPropertyFn(tc.excludeShortLivedTaskLists),
				dynamicproperties.GetBoolPropertyFn(false),
				pct,
				config.DefaultNumMatchingShards,
			)
			router := NewTaskListRouter(
				resolver,
				taskListDifferentiator,
				spectator,
				logger,
				metrics.NewNoopMetricsClient().Scope(metrics.ShardDistributorResolverScope),
			).(*taskListRouter)

			if tc.expectHashRing {
				resolver.EXPECT().Lookup(service.Matching, tc.taskListName).Return(membership.NewHostInfo("hash-ring-addr"), nil)
			} else {
				spectator.EXPECT().GetShardOwner(gomock.Any(), tc.taskListName).
					Return(&spectatorclient.ShardOwner{
						ExecutorID: "test-owner",
						Metadata: map[string]string{
							"hostIP":   "127.0.0.1",
							"tchannel": "7933",
							"grpc":     "7833",
						},
					}, nil)
			}

			host, err := router.FromTaskList(tc.taskListName)
			assert.NoError(t, err)

			if tc.expectHashRing {
				assert.Equal(t, "hash-ring-addr", host.GetAddress())
			} else {
				assert.Equal(t, "127.0.0.1:7933", host.GetAddress())
			}
		})
	}
}

func newTestTaskListRouter(t *testing.T) (*taskListRouter, *membership.MockResolver, *spectatorclient.MockSpectator) {
	ctrl := gomock.NewController(t)
	spectator := spectatorclient.NewMockSpectator(ctrl)
	excludeShortLivedTaskLists := dynamicproperties.GetBoolPropertyFn(false)
	percentageOnboarded := membership.NewMockPercentageOnboarded(ctrl)
	percentageOnboarded.EXPECT().Value().Return(100).AnyTimes()
	resolver := membership.NewMockResolver(ctrl)
	logger := log.NewNoop()

	taskListDifferentiator := NewTaskListDifferentiator(
		excludeShortLivedTaskLists,
		dynamicproperties.GetBoolPropertyFn(false),
		percentageOnboarded,
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
