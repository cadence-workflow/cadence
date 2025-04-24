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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination producer_manager_mock.go -self_package github.com/uber/cadence/service/frontend/api

package api

import (
	"fmt"
	"time"

	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type (
	// ProducerManager is used to create a producer for a domain.
	// Producer is used for Async APIs such as StartWorkflowExecutionAsync
	ProducerManager interface {
		GetProducerByDomain(domain string) (messaging.Producer, error)
	}

	producerManagerImpl struct {
		domainCache   cache.DomainCache
		provider      queue.Provider
		logger        log.Logger
		metricsClient metrics.Client

		producerCache cache.Cache
	}
)

func NewProducerManager(
	domainCache cache.DomainCache,
	provider queue.Provider,
	logger log.Logger,
	metricsClient metrics.Client,
) ProducerManager {
	return &producerManagerImpl{
		domainCache:   domainCache,
		provider:      provider,
		logger:        logger,
		metricsClient: metricsClient,
		producerCache: cache.New(&cache.Options{
			TTL:             time.Minute * 5,
			InitialCapacity: 5,
			MaxCount:        100,
			Pin:             true,
			MetricsScope:    metricsClient.Scope(metrics.Frontend),
			Logger:          logger,
		}),
	}
}

// GetProducerByDomain returns a producer for a domain
func (q *producerManagerImpl) GetProducerByDomain(
	domain string,
) (messaging.Producer, error) {
	domainEntry, err := q.domainCache.GetDomain(domain)
	if err != nil {
		return nil, err
	}
	if !domainEntry.GetConfig().AsyncWorkflowConfig.Enabled {
		return nil, &types.BadRequestError{Message: fmt.Sprintf("async workflow is not enabled for domain %v", domain)}
	}
	queueName := domainEntry.GetConfig().AsyncWorkflowConfig.PredefinedQueueName
	var queue provider.Queue
	if queueName != "" {
		queue, err = q.provider.GetPredefinedQueue(queueName)
		if err != nil {
			return nil, err
		}
	} else {
		queue, err = q.provider.GetQueue(domainEntry.GetConfig().AsyncWorkflowConfig.QueueType, domainEntry.GetConfig().AsyncWorkflowConfig.QueueConfig)
		if err != nil {
			return nil, err
		}
	}

	queueID := queue.ID()
	val := q.producerCache.Get(queueID)
	if val != nil {
		return val.(messaging.Producer), nil
	}

	producer, err := queue.CreateProducer(&provider.Params{Logger: q.logger, MetricsClient: q.metricsClient})
	if err != nil {
		return nil, err
	}
	// PutIfNotExist is thread safe, and will either return the value that was already in the cache or the value we just created
	// another thread might have inserted a value between the Get and PutIfNotExist, but that is ok
	// it should never return an error as we do not use Pin
	val, err = q.producerCache.PutIfNotExist(queueID, producer)
	if err != nil {
		return nil, err
	}
	return val.(messaging.Producer), nil
}
