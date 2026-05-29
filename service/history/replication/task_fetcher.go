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

package replication

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination task_fetcher_mock.go -self_package github.com/uber/cadence/service/history/replication

// TODO: reuse the interface and implementation defined in history/task package
const (
	fetchTaskRequestTimeout = 60 * time.Second
	requestChanBufferSize   = 1000
)

type (
	// TaskFetcher is responsible for fetching replication messages from remote DC.
	TaskFetcher interface {
		common.Daemon

		GetSourceCluster() string
		GetRequestChan(shardID int) chan<- *request
		GetRateLimiter() quotas.Limiter
	}

	// TaskFetchers is a group of fetchers, one per source DC.
	TaskFetchers interface {
		common.Daemon

		GetFetchers() []TaskFetcher
	}

	// taskFetcherImpl is the implementation of fetching replication messages.
	taskFetcherImpl struct {
		status         int32
		currentCluster string
		sourceCluster  string
		config         *config.Config
		logger         log.Logger
		metricsScope   metrics.Scope
		remotePeer     admin.Client
		rateLimiter    quotas.Limiter
		timeSource     clock.TimeSource
		requestChan    []chan *request
		ctx            context.Context
		cancelCtx      context.CancelFunc
		wg             sync.WaitGroup

		fetchAndDistributeTasksFn func(map[int32]*request) error
	}

	// taskFetchersImpl is a group of fetchers, one per source DC.
	taskFetchersImpl struct {
		status   int32
		logger   log.Logger
		fetchers []TaskFetcher
	}
)

var _ TaskFetcher = (*taskFetcherImpl)(nil)
var _ TaskFetchers = (*taskFetchersImpl)(nil)

// NewTaskFetchers creates an instance of ReplicationTaskFetchers with given configs.
func NewTaskFetchers(
	logger log.Logger,
	config *config.Config,
	clusterMetadata cluster.Metadata,
	clientBean client.Bean,
	metricsClient metrics.Client,
) (TaskFetchers, error) {
	currentCluster := clusterMetadata.GetCurrentClusterName()
	var fetchers []TaskFetcher
	for clusterName := range clusterMetadata.GetRemoteClusterInfo() {
		remoteFrontendClient, err := clientBean.GetRemoteAdminClient(clusterName)
		if err != nil {
			return nil, err
		}
		fetcher := newReplicationTaskFetcher(
			logger,
			clusterName,
			currentCluster,
			config,
			remoteFrontendClient,
			metricsClient,
		)
		fetchers = append(fetchers, fetcher)
	}

	return &taskFetchersImpl{
		fetchers: fetchers,
		status:   common.DaemonStatusInitialized,
		logger:   logger,
	}, nil
}

// Start starts the fetchers
func (f *taskFetchersImpl) Start() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	for _, fetcher := range f.fetchers {
		fetcher.Start()
	}
	f.logger.Info("Replication task fetchers started.", tag.Counter(len(f.fetchers)))
}

// Stop stops the fetchers
func (f *taskFetchersImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	for _, fetcher := range f.fetchers {
		fetcher.Stop()
	}
	f.logger.Info("Replication task fetchers stopped.", tag.Counter(len(f.fetchers)))
}

// GetFetchers returns all the fetchers
func (f *taskFetchersImpl) GetFetchers() []TaskFetcher {
	return f.fetchers
}

// newReplicationTaskFetcher creates a new fetcher.
func newReplicationTaskFetcher(
	logger log.Logger,
	sourceCluster string,
	currentCluster string,
	config *config.Config,
	sourceFrontend admin.Client,
	metricsClient metrics.Client,
) TaskFetcher {
	ctx, cancel := context.WithCancel(context.Background())

	requestChan := make([]chan *request, config.ReplicationTaskFetcherParallelism())
	for i := 0; i < config.ReplicationTaskFetcherParallelism(); i++ {
		requestChan[i] = make(chan *request, requestChanBufferSize)
	}

	fetcher := &taskFetcherImpl{
		status:         common.DaemonStatusInitialized,
		config:         config,
		logger:         logger.WithTags(tag.ClusterName(sourceCluster)),
		metricsScope:   metricsClient.Scope(metrics.ReplicationTaskFetcherScope, metrics.TargetClusterTag(sourceCluster)),
		remotePeer:     sourceFrontend,
		currentCluster: currentCluster,
		sourceCluster:  sourceCluster,
		rateLimiter:    quotas.NewDynamicRateLimiter(config.ReplicationTaskProcessorHostQPS.AsFloat64()),
		timeSource:     clock.NewRealTimeSource(),
		requestChan:    requestChan,
		ctx:            ctx,
		cancelCtx:      cancel,
	}
	fetcher.fetchAndDistributeTasksFn = fetcher.fetchAndDistributeTasks
	return fetcher
}

// Start starts the fetcher
func (f *taskFetcherImpl) Start() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	// NOTE: ReplicationTaskFetcherParallelism > 1 is now supported. Each fetcher goroutine handles a subset of shards
	// (distributed via shardID % parallelism) and runs its own fetch cycle independently.
	for i := 0; i < f.config.ReplicationTaskFetcherParallelism(); i++ {
		i := i
		f.wg.Add(1)
		go f.fetchTasks(i)
	}
	f.logger.Info("Replication task fetcher started.", tag.Counter(f.config.ReplicationTaskFetcherParallelism()))
}

// Stop stops the fetcher
func (f *taskFetcherImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	f.cancelCtx()
	if !common.AwaitWaitGroup(&f.wg, 10*time.Second) {
		f.logger.Warn("Replication task fetcher timed out on shutdown.")
	} else {
		f.logger.Info("Replication task fetcher graceful shutdown completed.")
	}
}

// fetchTasks collects getReplicationTasks request from shards and send out aggregated request to source frontend.
func (f *taskFetcherImpl) fetchTasks(chanIdx int) {
	defer f.wg.Done()

	timer := f.timeSource.NewTimer(backoff.JitDuration(
		f.config.ReplicationTaskFetcherAggregationInterval(),
		f.config.ReplicationTaskFetcherTimerJitterCoefficient(),
	))
	defer timer.Stop()

	requestByShard := make(map[int32]*request)
	for {
		select {
		case request := <-f.requestChan[chanIdx]:
			// Here we only add the request to map. We will wait until timer fires to send the request to remote.
			if req, ok := requestByShard[request.token.GetShardID()]; ok && req != request {
				// since this replication task fetcher is per host and replication task processor is per shard
				// during shard movement, duplicated requests can appear, if shard moved from this host to this host.
				f.logger.Info("Get replication task request already exist for shard.", tag.ShardID(int(request.token.GetShardID())))
				close(req.respChan)
			}
			requestByShard[request.token.GetShardID()] = request

		case <-timer.Chan():
			// When timer fires, we collect all the requests we have so far and attempt to send them to remote.
			err := f.fetchAndDistributeTasksFn(requestByShard)
			if err != nil {
				if _, ok := err.(*types.ServiceBusyError); ok {
					// slow down replication when source cluster is busy
					timer.Reset(f.config.ReplicationTaskFetcherServiceBusyWait())
				} else {
					timer.Reset(backoff.JitDuration(
						f.config.ReplicationTaskFetcherErrorRetryWait(),
						f.config.ReplicationTaskFetcherTimerJitterCoefficient(),
					))
				}
			} else {
				timer.Reset(backoff.JitDuration(
					f.config.ReplicationTaskFetcherAggregationInterval(),
					f.config.ReplicationTaskFetcherTimerJitterCoefficient(),
				))
			}
		case <-f.ctx.Done():
			return
		}
	}
}

func (f *taskFetcherImpl) fetchAndDistributeTasks(requestByShard map[int32]*request) error {
	startTime := f.timeSource.Now()
	defer func() {
		fetchLatency := f.timeSource.Now().Sub(startTime)
		f.metricsScope.ExponentialHistogram(metrics.ExponentialReplicationTaskFetchLatency, fetchLatency)
	}()

	if len(requestByShard) == 0 {
		// We don't receive tasks from previous fetch so processors are all sleeping.
		f.logger.Debug("Skip fetching as no processor is asking for tasks.")
		return nil
	}

	messagesByShard, err := f.getMessages(requestByShard)
	if err != nil {
		if _, ok := err.(*types.ServiceBusyError); !ok {
			f.logger.Error("Failed to get replication tasks", tag.Error(err))
		} else {
			f.logger.Debug("Failed to get replication tasks because service busy")
		}

		return err
	}

	totalTasks := 0
	for _, messages := range messagesByShard {
		totalTasks += len(messages.ReplicationTasks)
	}
	f.metricsScope.RecordHistogramValue(metrics.ReplicationTasksFetchedSize, float64(totalTasks))

	f.logger.Debug("Successfully fetched replication tasks.", tag.Counter(len(messagesByShard)))
	for shardID, tasks := range messagesByShard {
		request := requestByShard[shardID]
		request.respChan <- tasks
		close(request.respChan)
		delete(requestByShard, shardID)
	}

	return nil
}

func (f *taskFetcherImpl) getMessages(requestByShard map[int32]*request) (map[int32]*types.ReplicationMessages, error) {
	var tokens []*types.ReplicationToken
	for _, request := range requestByShard {
		tokens = append(tokens, request.token)
	}

	ctx, cancel := context.WithTimeout(f.ctx, fetchTaskRequestTimeout)
	defer cancel()

	request := &types.GetReplicationMessagesRequest{
		Tokens:      tokens,
		ClusterName: f.currentCluster,
	}
	response, err := f.remotePeer.GetReplicationMessages(ctx, request)
	if err != nil {
		return nil, err
	}

	return response.GetMessagesByShard(), nil
}

// GetSourceCluster returns the source cluster for the fetcher
func (f *taskFetcherImpl) GetSourceCluster() string {
	return f.sourceCluster
}

// GetRequestChan returns the request chan for the fetcher
func (f *taskFetcherImpl) GetRequestChan(shardID int) chan<- *request {
	chanIdx := shardID % len(f.requestChan)

	return f.requestChan[chanIdx]
}

// GetRateLimiter returns the host level rate limiter for the fetcher
func (f *taskFetcherImpl) GetRateLimiter() quotas.Limiter {
	return f.rateLimiter
}
