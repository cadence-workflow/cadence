// The MIT License (MIT)
//
// Copyright (c) 2022 Uber Technologies Inc.
//
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

package replication

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	hconfig "github.com/uber/cadence/service/history/config"
)

func TestTaskStore(t *testing.T) {
	ctx := context.Background()

	t.Run("Get error on unknown cluster", func(t *testing.T) {
		ts := createTestTaskStore(t, nil, nil)
		_, err := ts.Get(ctx, "unknown cluster", &testTask11)
		assert.Equal(t, ErrUnknownCluster, err)
	})

	t.Run("Get error resolving domain", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{}, nil)
		_, err := ts.Get(ctx, testClusterA, &testTask11)
		assert.EqualError(t, err, "resolving domain: domain does not exist")
	})

	t.Run("Get skips task for domains non belonging to polling cluster", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, nil)
		task, err := ts.Get(ctx, testClusterB, &testTask11)
		assert.NoError(t, err)
		assert.Nil(t, task)
	})

	t.Run("Get returns cached replication task", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, nil)
		ts.Put(&testHydratedTask11)
		task, err := ts.Get(ctx, testClusterA, &testTask11)
		assert.NoError(t, err)
		assert.Equal(t, &testHydratedTask11, task)
	})

	t.Run("Get returns non-cached replication task by hydrating it", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, fakeTaskHydrator{testTask11.TaskID: testHydratedTask11})
		task, err := ts.Get(ctx, testClusterA, &testTask11)
		assert.NoError(t, err)
		assert.Equal(t, &testHydratedTask11, task)
	})

	t.Run("Get fails to hydrate replication task", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, fakeTaskHydrator{testTask11.TaskID: testHydratedTaskErrorNonRecoverable})
		task, err := ts.Get(ctx, testClusterA, &testTask11)
		assert.EqualError(t, err, "error hydrating task")
		assert.Nil(t, task)
	})

	t.Run("Put does not store nil task", func(t *testing.T) {
		ts := createTestTaskStore(t, nil, nil)
		ts.Put(nil)
		for _, cache := range ts.clusters {
			assert.Zero(t, cache.Size())
		}
	})

	t.Run("Put error resolving domain - does not store task", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{}, nil)
		ts.Put(&testHydratedTask11)
		ts.Put(&testHydratedTask12)
		ts.Put(&testHydratedTask14)
		for _, cache := range ts.clusters {
			assert.Zero(t, cache.Size())
		}
	})

	t.Run("Put hydrated task into appropriate cache", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, nil)
		ts.Put(&testHydratedTask11)
		for _, cluster := range testDomain.GetReplicationConfig().Clusters {
			assert.Equal(t, 1, ts.clusters[cluster.ClusterName].Count())
		}
	})

	t.Run("Put hydrated task without domain info - will put it to all caches", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, nil)
		ts.Put(&types.ReplicationTask{SourceTaskID: 123})
		for _, cluster := range testDomain.GetReplicationConfig().Clusters {
			assert.Equal(t, 1, ts.clusters[cluster.ClusterName].Count())
		}
	})

	t.Run("Put full cache error", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, nil)
		ts.Put(&testHydratedTask11)
		ts.Put(&testHydratedTask12)
		ts.Put(&testHydratedTask14)
		for _, cluster := range testDomain.GetReplicationConfig().Clusters {
			assert.Equal(t, 2, ts.clusters[cluster.ClusterName].Count())
		}
	})

	t.Run("Put will not store acked task", func(t *testing.T) {
		ts := createTestTaskStore(t, fakeDomainCache{testDomainID: testDomain}, nil)
		for _, cluster := range testDomain.GetReplicationConfig().Clusters {
			ts.Ack(cluster.ClusterName, testHydratedTask11.SourceTaskID)
			ts.Put(&testHydratedTask11)
			assert.Equal(t, 0, ts.clusters[cluster.ClusterName].Count())
		}
	})

	t.Run("Ack error on unknown cluster", func(t *testing.T) {
		ts := createTestTaskStore(t, nil, nil)
		err := ts.Ack("unknown cluster", 0)
		assert.Equal(t, ErrUnknownCluster, err)
	})
}

func createTestTaskStore(t *testing.T, domains domainCache, hydrator taskHydrator) *TaskStore {
	cfg := hconfig.Config{
		ReplicatorCacheCapacity:         dynamicproperties.GetIntPropertyFn(2),
		ReplicationTaskGenerationQPS:    dynamicproperties.GetFloatPropertyFn(0),
		ReplicatorReadTaskMaxRetryCount: dynamicproperties.GetIntPropertyFn(1),
		ReplicatorCacheMaxSize:          dynamicproperties.GetIntPropertyFn(2000),
	}

	clusterMetadata := cluster.NewMetadata(
		config.ClusterGroupMetadata{
			FailoverVersionIncrement: 0,
			PrimaryClusterName:       testClusterC,
			CurrentClusterName:       testClusterC,
			ClusterGroup: map[string]config.ClusterInformation{
				testClusterA: {Enabled: true},
				testClusterB: {Enabled: true},
				testClusterC: {Enabled: true},
			},
		},
		func(d string) bool { return false },
		metrics.NewNoopMetricsClient(),
		testlogger.New(t),
	)

	return NewTaskStore(&cfg, clusterMetadata, domains, metrics.NewNoopMetricsClient(), log.NewNoop(), hydrator)
}

type fakeDomainCache map[string]*cache.DomainCacheEntry

func (cache fakeDomainCache) GetDomainByID(id string) (*cache.DomainCacheEntry, error) {
	if entry, ok := cache[id]; ok {
		return entry, nil
	}
	return nil, types.EntityNotExistsError{Message: "domain does not exist"}
}
