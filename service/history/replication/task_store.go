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
	"errors"
	"fmt"
	"time"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

// ErrUnknownCluster is returned when given cluster is not defined in cluster metadata
var ErrUnknownCluster = errors.New("unknown cluster")

// TaskStore is a component that hydrates and caches replication messages so that they can be reused across several polling source clusters.
// It also exposes public Put method. This allows pre-store already hydrated messages at the end of successful transaction, saving a DB call to fetch history events.
//
// TaskStore uses a separate cache per each source cluster allowing messages to be fetched at different rates.
// Once a cache becomes full it will not accept further messages for that cluster. Later those messages be fetched from DB and hydrated again.
// A cache stores only a pointer to the message. It is hydrates once and shared across caches. Cluster acknowledging the message will remove it from that corresponding cache.
// Once all clusters acknowledge it, no more references will be held, and GC will eventually pick it up.
type TaskStore struct {
	clusters      map[string]cache.AckCache[*types.ReplicationTask]
	domains       domainCache
	hydrator      taskHydrator
	rateLimiter   quotas.Limiter
	throttleRetry *backoff.ThrottleRetry

	scope  metrics.Scope
	logger log.Logger

	lastLogTime time.Time
}

type (
	domainCache interface {
		GetDomainByID(id string) (*cache.DomainCacheEntry, error)
	}
	taskHydrator interface {
		Hydrate(ctx context.Context, task persistence.Task) (*types.ReplicationTask, error)
	}
)

// NewTaskStore create new instance of TaskStore
func NewTaskStore(
	config *config.Config,
	clusterMetadata cluster.Metadata,
	domains domainCache,
	metricsClient metrics.Client,
	logger log.Logger,
	hydrator taskHydrator,
	budgetManager cache.Manager,
	shardID int,
) *TaskStore {
	cacheName := fmt.Sprintf("replication-cache-%d", shardID)

	clusters := map[string]cache.AckCache[*types.ReplicationTask]{}
	for clusterName := range clusterMetadata.GetRemoteClusterInfo() {
		clusters[clusterName] = cache.NewBoundedAckCache[*types.ReplicationTask](config.ReplicatorCacheCapacity, config.ReplicatorCacheMaxSize, logger, budgetManager, cacheName)
	}

	retryPolicy := backoff.NewExponentialRetryPolicy(100 * time.Millisecond)
	retryPolicy.SetMaximumAttempts(config.ReplicatorReadTaskMaxRetryCount())
	retryPolicy.SetBackoffCoefficient(1)

	return &TaskStore{
		clusters: clusters,
		domains:  domains,
		hydrator: hydrator,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(retryPolicy),
			backoff.WithRetryableError(persistence.IsTransientError),
		),
		scope:       metricsClient.Scope(metrics.ReplicatorCacheManagerScope),
		logger:      logger.WithTags(tag.ComponentReplicationCacheManager),
		rateLimiter: quotas.NewDynamicRateLimiter(config.ReplicationTaskGenerationQPS.AsFloat64()),
	}
}

// Get will return a hydrated replication message for a given cluster based on raw task info.
// It will either return it immediately from cache or hydrate it, store in cache and then return.
//
// Returned task may be nil. This may be due domain not existing in a given cluster or replication message is not longer relevant.
// Either case is valid and such replication message should be ignored and not returned in the response.
func (m *TaskStore) Get(ctx context.Context, cluster string, info persistence.Task) (*types.ReplicationTask, error) {
	cacheForTargetCluster, ok := m.clusters[cluster]
	if !ok {
		return nil, ErrUnknownCluster
	}

	domain, err := m.domains.GetDomainByID(info.GetDomainID())
	if err != nil {
		return nil, fmt.Errorf("resolving domain: %w", err)
	}

	// Domain does not exist in this cluster, do not replicate the task
	if !domain.HasReplicationCluster(cluster) {
		return nil, nil
	}

	scope := m.scope.Tagged(metrics.SourceClusterTag(cluster))

	scope.IncCounter(metrics.CacheRequests)
	// Keep timer (backwards compatible), dual-emit exponential histogram for migration.
	cacheLatencyStart := time.Now()
	sw := scope.StartTimer(metrics.CacheLatency)
	defer sw.Stop()
	defer func() {
		scope.ExponentialHistogram(metrics.ExponentialCacheLatency, time.Since(cacheLatencyStart))
	}()

	task := cacheForTargetCluster.Get(info.GetTaskID())

	if task != nil {
		scope.IncCounter(metrics.CacheHitCounter)
		return task, nil
	}

	m.scope.IncCounter(metrics.CacheMissCounter)

	// Rate limit to not kill the database
	m.rateLimiter.Wait(ctx)

	op := func(ctx context.Context) error {
		var err error
		task, err = m.hydrator.Hydrate(ctx, info)
		return err
	}

	err = m.throttleRetry.Do(ctx, op)

	if err != nil {
		m.scope.IncCounter(metrics.CacheFailures)
		return nil, err
	}

	m.Put(task)

	return task, nil
}

// Put will try to store hydrated replication to all cluster caches.
// Tasks may not be relevant, as domain is not enabled in some clusters. Ignore task for that cluster.
// Some clusters may already have full cache. Ignore the task, it will be fetched and hydrated again later.
// Some clusters may have already acknowledged such task. Ignore task, it is no longer relevant for such cluster.
func (m *TaskStore) Put(task *types.ReplicationTask) {
	// Do not store nil tasks
	if task == nil {
		return
	}

	domain, err := m.getDomain(task)
	if err != nil {
		m.logger.Error("failed to resolve domain", tag.Error(err))
		return
	}

	for targetCluster, cacheByCluster := range m.clusters {
		if domain != nil && !domain.HasReplicationCluster(targetCluster) {
			continue
		}

		scope := m.scope.Tagged(metrics.SourceClusterTag(targetCluster))

		err = cacheByCluster.Put(task, task.ByteSize())
		switch {
		case errors.Is(err, cache.ErrAckCacheFull):
			scope.IncCounter(metrics.CacheFullCounter)

			// This will help debug which shard is full. Logger already has ShardID tag attached.
			// Log only once a minute to not flood the logs.
			if time.Since(m.lastLogTime) > time.Minute {
				m.logger.Warn("Replication cache is full")
				m.lastLogTime = time.Now()
			}
		case errors.Is(err, cache.ErrAlreadyAcked):
			// No action, this is expected.
			// Some cluster(s) may be already past this, due to different fetch rates.
		}

		scope.RecordTimer(metrics.CacheSize, time.Duration(cacheByCluster.Count()))
	}
}

// Ack will acknowledge replication message for a given cluster.
// This will result in all messages removed from the cache up to a given lastTaskID.
func (m *TaskStore) Ack(cluster string, lastTaskID int64) error {
	cache, ok := m.clusters[cluster]
	if !ok {
		return ErrUnknownCluster
	}

	_, _ = cache.Ack(lastTaskID)

	scope := m.scope.Tagged(metrics.SourceClusterTag(cluster))
	scope.RecordTimer(metrics.CacheSize, time.Duration(cache.Count()))

	return nil
}

func (m *TaskStore) getDomain(task *types.ReplicationTask) (*cache.DomainCacheEntry, error) {
	if domainID := task.GetHistoryTaskV2Attributes().GetDomainID(); domainID != "" {
		return m.domains.GetDomainByID(domainID)
	}
	if domainID := task.GetSyncActivityTaskAttributes().GetDomainID(); domainID != "" {
		return m.domains.GetDomainByID(domainID)
	}
	if domainID := task.GetFailoverMarkerAttributes().GetDomainID(); domainID != "" {
		return m.domains.GetDomainByID(domainID)
	}

	return nil, nil
}
