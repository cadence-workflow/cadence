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

package engineimpl

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

func TestGenerateFailoverTasksForDomainCallback(t *testing.T) {

	domainInfo := persistence.DomainInfo{
		ID:         "83b48dab-68cb-4f73-8752-c75d9271977f",
		Name:       "samples-domain",
		OwnerEmail: "test4@test.com",
		Data:       map[string]string{},
	}
	domainConfig := persistence.DomainConfig{
		Retention:             3,
		EmitMetric:            true,
		HistoryArchivalURI:    "file:///tmp/cadence_archival/development",
		VisibilityArchivalURI: "file:///tmp/cadence_vis_archival/development",
		BadBinaries:           types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:       types.IsolationGroupConfiguration{},
	}
	replicationConfig := persistence.DomainReplicationConfig{
		ActiveClusterName: "cluster0",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: "cluster0"},
			{ClusterName: "cluster1"},
			{ClusterName: "cluster2"},
		},
	}

	t1 := cache.NewDomainCacheEntryForTest(&domainInfo,
		&domainConfig,
		true,
		&replicationConfig,
		1,
		nil,
		2,
		common.InitialPreviousFailoverVersion,
		15,
	)

	t2 := cache.NewDomainCacheEntryForTest(&domainInfo,
		&domainConfig,
		true,
		&replicationConfig,
		10,
		nil,
		3,
		common.InitialPreviousFailoverVersion,
		15,
	)

	t3 := cache.NewDomainCacheEntryForTest(&domainInfo,
		&domainConfig,
		true,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster1",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "cluster0"},
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
		},
		10,
		common.Ptr(time.Now().Add(time.Second).UnixNano()),
		4,
		1,
		15,
	)

	nonFailoverDomainUpdateT1 := []*cache.DomainCacheEntry{
		t1,
	}

	failoverUpdateT2 := []*cache.DomainCacheEntry{
		t2,
	}

	failoverUpdateT3 := []*cache.DomainCacheEntry{
		t3,
	}

	tests := map[string]struct {
		domainUpdates              []*cache.DomainCacheEntry
		currentNotificationVersion int64
		expectedRes                []*persistence.FailoverMarkerTask
		expectedErr                error
	}{
		"non-failover domain update. This should generate no tasks as there's no failover taking place": {
			domainUpdates:              nonFailoverDomainUpdateT1,
			currentNotificationVersion: 1,
			expectedRes:                []*persistence.FailoverMarkerTask{},
		},
		"non-graceful failover domain update. This should generate no tasks as there's no failover taking place": {
			domainUpdates:              failoverUpdateT2,
			currentNotificationVersion: 2,
			expectedRes:                []*persistence.FailoverMarkerTask{
				// no failoverMarker tasks are created for non-graceful failovers, they just occur immediately
			},
		},
		"graceful failover domain update. This should generate failover marker tasks": {
			domainUpdates:              failoverUpdateT3,
			currentNotificationVersion: 3,
			expectedRes: []*persistence.FailoverMarkerTask{
				{
					TaskData: persistence.TaskData{
						Version:             10,
						TaskID:              0,
						VisibilityTimestamp: time.Time{},
					},
					DomainID: "83b48dab-68cb-4f73-8752-c75d9271977f",
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			cluster := cluster.NewMetadata(
				10,
				"cluster0",
				"cluster0",
				map[string]config.ClusterInformation{
					"cluster0": config.ClusterInformation{
						Enabled:                true,
						InitialFailoverVersion: 1,
					},
					"cluster1": config.ClusterInformation{
						Enabled:                true,
						InitialFailoverVersion: 0,
					},
					"cluster2": config.ClusterInformation{
						Enabled:                true,
						InitialFailoverVersion: 2,
					},
				},
				func(string) bool { return false },
				metrics.NewNoopMetricsClient(),
				loggerimpl.NewNopLogger(),
			)

			he := historyEngineImpl{
				logger:             loggerimpl.NewNopLogger(),
				clusterMetadata:    cluster,
				currentClusterName: "cluster0",
				metricsClient:      metrics.NewNoopMetricsClient(),
			}
			res := he.generateGracefulFailoverTasksForDomainUpdateCallback(td.currentNotificationVersion, td.domainUpdates)
			assert.Equal(t, td.expectedRes, res)
		})
	}
}

func TestDomainCallback(t *testing.T) {

	// This is a reference set of data from a series of steps, showing a data dump
	// from cluster0 of the domain failover values, counters and guards and how they work:

	// ---- Initial starting position -- active in cluster0
	//
	// Cluster0 has initialFailoverVersion of 1
	// Cluster1 has initialFailoverVersion of 0
	// Cluster2 has initialFailoverVersion of 2
	// failover increments are 10

	// ----------- t0 - initial starting position - the domain is initially active in cluster0
	// domains_partition             | 0
	// name                          | samples-domain
	// config                        | {retention: 3, emit_metric: True, archival_bucket: '', archival_status: 0, bad_binaries: 0x590d000a0b0c0000000000, bad_binaries_encoding: 'thriftrw', history_archival_status: 1, history_archival_uri: 'file:///tmp/cadence_archival/development', visibility_archival_status: 1, visibility_archival_uri: 'file:///tmp/cadence_vis_archival/development', isolation_groups: 0x5900, isolation_groups_encoding: 'thriftrw', async_workflow_config: 0x5902000a000b0014000000000b001e0000000000, async_workflow_config_encoding: 'thriftrw'}
	// config_version                | 0
	// domain                        | {id: ad1002f2-f87d-4128-afce-6d320c8871e7, name: 'samples-domain', status: 0, description: '', owner_email: '', data: null}
	// failover_end_time             | 0
	// failover_notification_version | 0
	// failover_version              | 1
	// is_global_domain              | True
	// last_updated_time             | 1729031237843277000
	// notification_version          | 1
	// previous_failover_version     | -1
	// replication_config            | {active_cluster_name: 'cluster0', clusters: [{cluster_name: 'cluster0'}, {cluster_name: 'cluster1'}, {cluster_name: 'cluster2'}]}

	// ------------ t1 domain update - no failover
	// domains_partition             | 0
	// name                          | samples-domain
	// config                        | {retention: 3, emit_metric: True, archival_bucket: '', archival_status: 0, bad_binaries: 0x590d000a0b0c0000000000, bad_binaries_encoding: 'thriftrw', history_archival_status: 1, history_archival_uri: 'file:///tmp/cadence_archival/development', visibility_archival_status: 1, visibility_archival_uri: 'file:///tmp/cadence_vis_archival/development', isolation_groups: 0x5900, isolation_groups_encoding: 'thriftrw', async_workflow_config: 0x5902000a000b0014000000000b001e0000000000, async_workflow_config_encoding: 'thriftrw'}
	// config_version                | 1
	// domain                        | {id: ad1002f2-f87d-4128-afce-6d320c8871e7, name: 'samples-domain', status: 0, description: '', owner_email: 'some-email@email.com', data: {}}
	// failover_end_time             | 0
	// failover_notification_version | 0
	// failover_version              | 1
	// is_global_domain              | True
	// last_updated_time             | 1729031406189428000
	// notification_version          | 2
	// previous_failover_version     | -1
	// replication_config            | {active_cluster_name: 'cluster0', clusters: [{cluster_name: 'cluster0'}, {cluster_name: 'cluster1'}, {cluster_name: 'cluster2'}]}

	// ------------- t2 Failover to cluster2 - non-graceful
	// domains_partition             | 0
	// name                          | samples-domain
	// config                        | {retention: 3, emit_metric: True, archival_bucket: '', archival_status: 0, bad_binaries: 0x590d000a0b0c0000000000, bad_binaries_encoding: 'thriftrw', history_archival_status: 1, history_archival_uri: 'file:///tmp/cadence_archival/development', visibility_archival_status: 1, visibility_archival_uri: 'file:///tmp/cadence_vis_archival/development', isolation_groups: 0x5900, isolation_groups_encoding: 'thriftrw', async_workflow_config: 0x5902000a000b0014000000000b001e0000000000, async_workflow_config_encoding: 'thriftrw'}
	// config_version                | 1
	// domain                        | {id: ad1002f2-f87d-4128-afce-6d320c8871e7, name: 'samples-domain', status: 0, description: '', owner_email: 'some-email@email.com', data: {'FailoverHistory': '[{"eventTime":"2024-10-15T15:31:52.533009-07:00","fromCluster":"cluster0","toCluster":"cluster2","failoverType":"Force"}]'}}
	// failover_end_time             | 0
	// failover_notification_version | 3
	// failover_version              | 2
	// is_global_domain              | True
	// last_updated_time             | 1729031512533009000
	// notification_version          | 3
	// previous_failover_version     | -1
	// replication_config            | {active_cluster_name: 'cluster2', clusters: [{cluster_name: 'cluster0'}, {cluster_name: 'cluster1'}, {cluster_name: 'cluster2'}]}

	// ------------ t3 Failback to cluster0 - graceful
	// domains_partition             | 0
	// name                          | samples-domain
	// config                        | {retention: 3, emit_metric: True, archival_bucket: '', archival_status: 0, bad_binaries: 0x590d000a0b0c0000000000, bad_binaries_encoding: 'thriftrw', history_archival_status: 1, history_archival_uri: 'file:///tmp/cadence_archival/development', visibility_archival_status: 1, visibility_archival_uri: 'file:///tmp/cadence_vis_archival/development', isolation_groups: 0x5900, isolation_groups_encoding: 'thriftrw', async_workflow_config: 0x5902000a000b0014000000000b001e0000000000, async_workflow_config_encoding: 'thriftrw'}
	// config_version                | 1
	// domain                        | {id: ad1002f2-f87d-4128-afce-6d320c8871e7, name: 'samples-domain', status: 0, description: '', owner_email: 'some-email@email.com', data: {'FailoverHistory': '[{"eventTime":"2024-10-15T15:33:39.83016-07:00","fromCluster":"cluster2","toCluster":"cluster0","failoverType":"Grace"},{"eventTime":"2024-10-15T15:31:52.533009-07:00","fromCluster":"cluster0","toCluster":"cluster2","failoverType":"Force"}]'}}
	// failover_end_time             | 0
	// failover_notification_version | 4
	// failover_version              | 11
	// is_global_domain              | True
	// last_updated_time             | 0
	// notification_version          | 5
	// previous_failover_version     | 0
	// replication_config            | {active_cluster_name: 'cluster0', clusters: [{cluster_name: 'cluster0'}, {cluster_name: 'cluster1'}, {cluster_name: 'cluster2'}]}

	// ------------ t4 Failback to cluster2 - graceful
	// domains_partition             | 0
	// name                          | samples-domain
	// config                        | {retention: 3, emit_metric: True, archival_bucket: '', archival_status: 0, bad_binaries: 0x590d000a0b0c0000000000, bad_binaries_encoding: 'thriftrw', history_archival_status: 1, history_archival_uri: 'file:///tmp/cadence_archival/development', visibility_archival_status: 1, visibility_archival_uri: 'file:///tmp/cadence_vis_archival/development', isolation_groups: 0x5900, isolation_groups_encoding: 'thriftrw', async_workflow_config: 0x5902000a000b0014000000000b001e0000000000, async_workflow_config_encoding: 'thriftrw'}
	// config_version                | 1
	// domain                        | {id: ad1002f2-f87d-4128-afce-6d320c8871e7, name: 'samples-domain', status: 0, description: '', owner_email: 'some-email@email.com', data: {'FailoverHistory': '[{"eventTime":"2024-10-15T15:47:56.967632-07:00","fromCluster":"cluster0","toCluster":"cluster2","failoverType":"Grace"}]'}}
	// failover_end_time             | 0
	// failover_notification_version | 5
	// failover_version              | 12
	// is_global_domain              | True
	// last_updated_time             | 0
	// notification_version          | 6
	// previous_failover_version     | 0
	// replication_config            | {active_cluster_name: 'cluster2', clusters: [{cluster_name: 'cluster0'}, {cluster_name: 'cluster1'}, {cluster_name: 'cluster2'}]}
	clusters := []*persistence.ClusterReplicationConfig{
		{ClusterName: "cluster0"},
		{ClusterName: "cluster1"},
		{ClusterName: "cluster2"},
	}

	domainInfo := persistence.DomainInfo{
		ID:         "83b48dab-68cb-4f73-8752-c75d9271977f",
		Name:       "samples-domain",
		OwnerEmail: "test4@test.com",
		Data:       map[string]string{},
	}
	domainConfig := persistence.DomainConfig{
		Retention:             3,
		EmitMetric:            true,
		HistoryArchivalURI:    "file:///tmp/cadence_archival/development",
		VisibilityArchivalURI: "file:///tmp/cadence_vis_archival/development",
		BadBinaries:           types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:       types.IsolationGroupConfiguration{},
	}
	replicationConfig := persistence.DomainReplicationConfig{
		ActiveClusterName: "cluster0",
		Clusters:          clusters,
	}

	// failover_version              | 1
	// is_global_domain              | True
	// last_updated_time             | 1729031237843277000
	// notification_version          | 1
	// previous_failover_version     | -1

	// A domain update, no failover though
	// failover_end_time             | 0
	// failover_notification_version | 0
	// failover_version              | 1
	// is_global_domain              | True
	// last_updated_time             | 1729031406189428000
	// notification_version          | 2
	// previous_failover_version     | -1
	t1 := cache.NewDomainCacheEntryForTest(&domainInfo,
		&domainConfig,
		true,
		&replicationConfig,
		1,
		nil,
		0,
		common.InitialPreviousFailoverVersion,
		2,
	)

	// failover to cluster2
	// failover_end_time             | 0
	// failover_notification_version | 3
	// failover_version              | 2
	// is_global_domain              | True
	// last_updated_time             | 1729031512533009000
	// notification_version          | 3
	// previous_failover_version     | -1
	t2 := cache.NewDomainCacheEntryForTest(&domainInfo,
		&domainConfig,
		true,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster2",
			Clusters:          clusters,
		},
		2,
		nil,
		3,
		common.InitialPreviousFailoverVersion,
		3,
	)

	// Graceful failover to cluster0 again (remember cluster0 is initialFailover version 1)
	// failover_end_time             | 0
	// failover_notification_version | 4
	// failover_version              | 11
	// is_global_domain              | True
	// last_updated_time             | 0
	// notification_version          | 5
	// previous_failover_version     | 0
	t3 := cache.NewDomainCacheEntryForTest(&domainInfo,
		&domainConfig,
		true,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster0",
			Clusters:          clusters,
		},
		11,
		common.Ptr(time.Now().Add(time.Second).UnixNano()),
		4,
		0,
		5, // this is 4 on all the other clusters, but on cluster0 its 5 because
		// graceful does two events it seems to increment it twice just locally
	)

	nonFailoverDomainUpdateT1 := []*cache.DomainCacheEntry{
		t1,
	}

	failoverUpdateT2 := []*cache.DomainCacheEntry{
		t2,
	}

	failoverUpdateT3 := []*cache.DomainCacheEntry{
		t3,
	}

	invalidDomainUpdate := []*cache.DomainCacheEntry{
		cache.NewDomainCacheEntryForTest(&domainInfo,
			&domainConfig,
			true,
			&persistence.DomainReplicationConfig{
				ActiveClusterName: "cluster0",
				Clusters:          clusters,
			},
			11,
			common.Ptr(time.Now().Add(time.Second).UnixNano()),
			4,
			5, // not a valid initial failover vrsion
			5,
		),
	}

	timeSource := clock.NewMockedTimeSource()

	tests := map[string]struct {
		domainUpdates []*cache.DomainCacheEntry
		asCluster     string
		affordances   func(
			shardCtx *shard.MockContext,
			txProcessor *queue.MockProcessor,
			timerProcessor *queue.MockProcessor,
			taskProcessor *task.MockProcessor,
		)
		expectedErr error
	}{
		"t1 - non-failover domain update. - from the point of view of cluster 0": {
			domainUpdates: nonFailoverDomainUpdateT1,
			asCluster:     "cluster0",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))
				shardCtx.EXPECT().UpdateDomainNotificationVersion(int64(3))

				txProcessor.EXPECT().UnlockTaskProcessing()

				timerProcessor.EXPECT().UnlockTaskProcessing()
			},
		},
		"t1 -> t2: non-graceful failover domain update. cluster0 POV": {
			domainUpdates: failoverUpdateT2,
			asCluster:     "cluster0",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))
				shardCtx.EXPECT().UpdateDomainNotificationVersion(int64(4))

				txProcessor.EXPECT().UnlockTaskProcessing()

				timerProcessor.EXPECT().UnlockTaskProcessing()
			},
		},
		"t1 -> t2: non-graceful failover domain update. - cluster2 POV": {
			domainUpdates: failoverUpdateT2,
			asCluster:     "cluster2",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))
				shardCtx.EXPECT().GetTimeSource().Return(timeSource)
				shardCtx.EXPECT().UpdateDomainNotificationVersion(int64(4))

				txProcessor.EXPECT().UnlockTaskProcessing()
				txProcessor.EXPECT().FailoverDomain(gomock.Any())
				txProcessor.EXPECT().NotifyNewTask("cluster2", gomock.Any())

				timerProcessor.EXPECT().UnlockTaskProcessing()
				timerProcessor.EXPECT().FailoverDomain(gomock.Any())
				timerProcessor.EXPECT().NotifyNewTask("cluster2", gomock.Any())
			},
		},
		"t2 -> t3: graceful failover domain update. cluster0 POV": {
			domainUpdates: failoverUpdateT3,
			asCluster:     "cluster0",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))
				shardCtx.EXPECT().UpdateDomainNotificationVersion(int64(6))
				shardCtx.EXPECT().GetTimeSource().Return(timeSource)

				txProcessor.EXPECT().UnlockTaskProcessing()
				txProcessor.EXPECT().FailoverDomain(map[string]struct{}{"83b48dab-68cb-4f73-8752-c75d9271977f": struct{}{}})
				txProcessor.EXPECT().NotifyNewTask("cluster0", gomock.Any())

				timerProcessor.EXPECT().UnlockTaskProcessing()
				timerProcessor.EXPECT().FailoverDomain(map[string]struct{}{"83b48dab-68cb-4f73-8752-c75d9271977f": struct{}{}})
				timerProcessor.EXPECT().NotifyNewTask("cluster0", gomock.Any())
			},
		},
		"t2 -> t3: graceful failover domain update. - cluster2 POV": {
			domainUpdates: failoverUpdateT3,
			asCluster:     "cluster2",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))
				shardCtx.EXPECT().UpdateDomainNotificationVersion(int64(6))

				txProcessor.EXPECT().UnlockTaskProcessing()

				timerProcessor.EXPECT().UnlockTaskProcessing()
			},
		},
		"graceful failover domain update. cluster1 POV": {
			domainUpdates: failoverUpdateT3,
			asCluster:     "cluster1",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))
				shardCtx.EXPECT().UpdateDomainNotificationVersion(int64(6))

				shardCtx.EXPECT().ReplicateFailoverMarkers(gomock.Any(), []*persistence.FailoverMarkerTask{
					{
						TaskData: persistence.TaskData{
							Version: 11,
						},
						DomainID: "83b48dab-68cb-4f73-8752-c75d9271977f",
					},
				})

				txProcessor.EXPECT().UnlockTaskProcessing()
				timerProcessor.EXPECT().UnlockTaskProcessing()
			},
		},
		"graceful failover domain update. cluster1 POV - error on replication": {
			domainUpdates: failoverUpdateT3,
			asCluster:     "cluster1",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))

				shardCtx.EXPECT().ReplicateFailoverMarkers(gomock.Any(), []*persistence.FailoverMarkerTask{
					{
						TaskData: persistence.TaskData{
							Version: 11,
						},
						DomainID: "83b48dab-68cb-4f73-8752-c75d9271977f",
					},
				}).Return(assert.AnError)

				txProcessor.EXPECT().UnlockTaskProcessing()
				timerProcessor.EXPECT().UnlockTaskProcessing()
			},
		},
		"invalid failover version": {
			domainUpdates: invalidDomainUpdate,
			asCluster:     "cluster1",
			affordances: func(
				shardCtx *shard.MockContext,
				txProcessor *queue.MockProcessor,
				timerProcessor *queue.MockProcessor,
				taskProcessor *task.MockProcessor) {

				shardCtx.EXPECT().GetDomainNotificationVersion().Return(int64(1))

				shardCtx.EXPECT().UpdateDomainNotificationVersion(int64(6))

				txProcessor.EXPECT().UnlockTaskProcessing()
				timerProcessor.EXPECT().UnlockTaskProcessing()
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			cluster := cluster.NewMetadata(
				10,
				"cluster0",
				"cluster0",
				map[string]config.ClusterInformation{
					"cluster0": config.ClusterInformation{
						Enabled:                true,
						InitialFailoverVersion: 1,
					},
					"cluster1": config.ClusterInformation{
						Enabled:                true,
						InitialFailoverVersion: 0,
					},
					"cluster2": config.ClusterInformation{
						Enabled:                true,
						InitialFailoverVersion: 2,
					},
				},
				func(string) bool { return false },
				metrics.NewNoopMetricsClient(),
				loggerimpl.NewNopLogger(),
			)

			ctrl := gomock.NewController(t)

			timeProcessor := queue.NewMockProcessor(ctrl)
			queueTaskProcessor := task.NewMockProcessor(ctrl)
			txProcessor := queue.NewMockProcessor(ctrl)
			shardCtx := shard.NewMockContext(ctrl)

			td.affordances(shardCtx, txProcessor, timeProcessor, queueTaskProcessor)

			he := historyEngineImpl{
				logger:             loggerimpl.NewNopLogger(),
				clusterMetadata:    cluster,
				currentClusterName: td.asCluster,
				metricsClient:      metrics.NewNoopMetricsClient(),
				timerProcessor:     timeProcessor,
				queueTaskProcessor: queueTaskProcessor,
				txProcessor:        txProcessor,
				shard:              shardCtx,
			}
			he.domainChangeCB(td.domainUpdates)
		})
	}
}

func TestDomainLocking(t *testing.T) {

	cluster := cluster.NewMetadata(
		10,
		"cluster0",
		"cluster0",
		map[string]config.ClusterInformation{
			"cluster0": config.ClusterInformation{
				Enabled:                true,
				InitialFailoverVersion: 1,
			},
		},
		func(string) bool { return false },
		metrics.NewNoopMetricsClient(),
		loggerimpl.NewNopLogger(),
	)

	ctrl := gomock.NewController(t)
	timeProcessor := queue.NewMockProcessor(ctrl)
	queueTaskProcessor := task.NewMockProcessor(ctrl)
	txProcessor := queue.NewMockProcessor(ctrl)
	shardCtx := shard.NewMockContext(ctrl)

	timeProcessor.EXPECT().LockTaskProcessing()
	timeProcessor.EXPECT().UnlockTaskProcessing()

	txProcessor.EXPECT().LockTaskProcessing()
	txProcessor.EXPECT().UnlockTaskProcessing()

	he := historyEngineImpl{
		logger:             loggerimpl.NewNopLogger(),
		clusterMetadata:    cluster,
		currentClusterName: "cluster0",
		metricsClient:      metrics.NewNoopMetricsClient(),
		timerProcessor:     timeProcessor,
		queueTaskProcessor: queueTaskProcessor,
		txProcessor:        txProcessor,
		shard:              shardCtx,
	}

	he.lockProcessingForFailover()
	he.unlockProcessingForFailover()
}
