// Copyright (c) 2021 Uber Technologies, Inc.
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

package task

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/future"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

type (
	// FetcherOptions configures a Fetcher
	FetcherOptions struct {
		Parallelism                dynamicconfig.IntPropertyFn
		AggregationInterval        dynamicconfig.DurationPropertyFn
		ServiceBusyBackoffInterval dynamicconfig.DurationPropertyFn
		ErrorRetryInterval         dynamicconfig.DurationPropertyFn
		TimerJitterCoefficient     dynamicconfig.FloatPropertyFn
	}

	fetchRequest struct {
		shardID  int
		params   []interface{}
		settable future.Settable
	}

	fetchTaskFunc func(
		clientBean client.Bean,
		currentCluster string,
		requestByShard map[int]fetchRequest,
	) (map[int]interface{}, error)

	fetcherImpl struct {
		status         int32
		currentCluster string
		sourceCluster  string
		clientBean     client.Bean

		options *FetcherOptions
		logger  log.Logger

		shutdownWG  sync.WaitGroup
		shutdownCh  chan struct{}
		requestChan chan fetchRequest

		fetchTaskFunc fetchTaskFunc
	}
)

const (
	defaultFetchTimeout          = 30 * time.Second
	defaultRequestChanBufferSize = 1000
)

var (
	errTaskFetcherShutdown    = errors.New("task fetcher has already shutdown")
	errDuplicatedFetchRequest = errors.New("duplicated task fetch request")
)

// NewCrossClusterTaskFetchers creates a set of task fetchers,
// one for each source cluster
func NewCrossClusterTaskFetchers(
	clusterMetadata cluster.Metadata,
	clientBean client.Bean,
	options *FetcherOptions,
	logger log.Logger,
) Fetchers {
	return newTaskFetchers(
		clusterMetadata,
		clientBean,
		crossClusterTaskFetchFn,
		options,
		logger,
	)
}

func crossClusterTaskFetchFn(
	clientBean client.Bean,
	currentCluster string,
	requestByShard map[int]fetchRequest,
) (map[int]interface{}, error) {
	// TODO: implement the fetch func after the
	// API for fetching tasks is created.
	return nil, errors.New("not implemented")
}

func newTaskFetchers(
	clusterMetadata cluster.Metadata,
	clientBean client.Bean,
	fetchTaskFunc fetchTaskFunc,
	options *FetcherOptions,
	logger log.Logger,
) Fetchers {
	currentClusterName := clusterMetadata.GetCurrentClusterName()
	clusterInfos := clusterMetadata.GetAllClusterInfo()
	fetchers := make([]Fetcher, 0, len(clusterInfos))

	for clusterName, info := range clusterInfos {
		if !info.Enabled || clusterName == currentClusterName {
			continue
		}

		fetchers = append(fetchers, newTaskFetcher(
			currentClusterName,
			clusterName,
			clientBean,
			fetchTaskFunc,
			options,
			logger,
		))
	}

	return fetchers
}

// Start is a util method for starting a group of fetchers
func (fetchers Fetchers) Start() {
	for _, fetcher := range fetchers {
		fetcher.Start()
	}
}

// Stop is a util method for stopping a group of fetchers
func (fetchers Fetchers) Stop() {
	for _, fetcher := range fetchers {
		fetcher.Stop()
	}
}

func newTaskFetcher(
	currentCluster string,
	sourceCluster string,
	clientBean client.Bean,
	fetchTaskFunc fetchTaskFunc,
	options *FetcherOptions,
	logger log.Logger,
) *fetcherImpl {
	return &fetcherImpl{
		status:         common.DaemonStatusInitialized,
		currentCluster: currentCluster,
		sourceCluster:  sourceCluster,
		clientBean:     clientBean,
		options:        options,
		logger:         logger,
		shutdownCh:     make(chan struct{}),
		requestChan:    make(chan fetchRequest, defaultRequestChanBufferSize),
		fetchTaskFunc:  fetchTaskFunc,
	}
}

func (f *fetcherImpl) Start() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	parallelism := f.options.Parallelism()
	f.shutdownWG.Add(parallelism)
	for i := 0; i != parallelism; i++ {
		go f.aggregator()
	}

	f.logger.Info("Task fetcher started.", tag.LifeCycleStarted, tag.Counter(parallelism))
}

func (f *fetcherImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	close(f.shutdownCh)
	if success := common.AwaitWaitGroup(&f.shutdownWG, time.Minute); !success {
		f.logger.Warn("Task fetcher timedout on shutdown.", tag.LifeCycleStopTimedout)
	}

	f.logger.Info("Task fetcher stopped.", tag.LifeCycleStopped)
}

func (f *fetcherImpl) GetSourceCluster() string {
	return f.sourceCluster
}

func (f *fetcherImpl) Fetch(
	shardID int,
	fetchParams ...interface{},
) future.Future {
	future, settable := future.NewFuture()

	f.requestChan <- fetchRequest{
		shardID:  shardID,
		params:   fetchParams,
		settable: settable,
	}

	select {
	case <-f.shutdownCh:
		f.drainRequestCh()
	default:
	}

	return future
}

func (f *fetcherImpl) aggregator() {
	defer f.shutdownWG.Done()

	fetchTimer := time.NewTimer(backoff.JitDuration(
		f.options.AggregationInterval(),
		f.options.TimerJitterCoefficient(),
	))

	outstandingRequests := make(map[int]fetchRequest)

	for {
		select {
		case <-f.shutdownCh:
			fetchTimer.Stop()
			f.drainRequestCh()
			for _, request := range outstandingRequests {
				request.settable.Set(nil, errTaskFetcherShutdown)
			}
			return
		case request := <-f.requestChan:
			if existingRequest, ok := outstandingRequests[request.shardID]; ok {
				existingRequest.settable.Set(nil, errDuplicatedFetchRequest)
			}
			outstandingRequests[request.shardID] = request
		case <-fetchTimer.C:
			var nextFetchInterval time.Duration
			if err := f.fetch(outstandingRequests); err != nil {
				if common.IsServiceBusyError(err) {
					nextFetchInterval = f.options.ServiceBusyBackoffInterval()
				} else {
					nextFetchInterval = f.options.ErrorRetryInterval()
				}
			} else {
				nextFetchInterval = f.options.AggregationInterval()
			}

			fetchTimer.Reset(backoff.JitDuration(
				nextFetchInterval,
				f.options.TimerJitterCoefficient(),
			))
		}
	}
}

func (f *fetcherImpl) fetch(
	outstandingRequests map[int]fetchRequest,
) error {
	if len(outstandingRequests) == 0 {
		return nil
	}

	tasksByShard, err := f.fetchTaskFunc(f.clientBean, f.currentCluster, outstandingRequests)
	if err != nil {
		f.logger.Error("Failed to fetch tasks", tag.Error(err))
		return err
	}

	for shardID, tasks := range tasksByShard {
		if request, ok := outstandingRequests[shardID]; ok {
			request.settable.Set(tasks, nil)
			delete(outstandingRequests, shardID)
		}
	}

	return nil
}

func (f *fetcherImpl) drainRequestCh() {
	for {
		select {
		case request := <-f.requestChan:
			request.settable.Set(nil, errTaskFetcherShutdown)
		default:
			return
		}
	}
}
