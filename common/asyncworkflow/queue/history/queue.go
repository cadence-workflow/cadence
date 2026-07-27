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

package history

import (
	"errors"
	"fmt"

	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

type (
	queueImpl struct {
		config *queueConfig
	}
)

func newQueue(decoder provider.Decoder) (provider.Queue, error) {
	var out queueConfig
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("bad config: %w", err)
	}
	return &queueImpl{
		config: &out,
	}, nil
}

func (q *queueImpl) ID() string {
	return q.config.ID()
}

// CreateConsumer returns a no-op consumer: history-backed async workflow queues are
// consumed inside the history service itself (per-shard consumer daemons), not by the
// worker service. A no-op (rather than an error) keeps the worker's ConsumerManager
// from logging an error for every history-backed domain on every refresh.
func (q *queueImpl) CreateConsumer(p *provider.Params) (provider.Consumer, error) {
	p.Logger.Info("History-backed async wf queue is consumed by the history service; starting no-op worker consumer",
		tag.AsyncWFQueueID(q.config.ID()))
	return &noopConsumer{}, nil
}

// noopConsumer satisfies provider.Consumer without doing any work.
type noopConsumer struct{}

func (*noopConsumer) Start() error { return nil }
func (*noopConsumer) Stop()        {}

func (q *queueImpl) CreateProducer(p *provider.Params) (messaging.Producer, error) {
	if p.HistoryClient == nil {
		return nil, errors.New("history client is required to create a history-backed async queue producer")
	}
	if p.NumHistoryShards <= 0 {
		return nil, fmt.Errorf("invalid number of history shards %d for history-backed async queue producer", p.NumHistoryShards)
	}
	p.Logger.Info("Creating history-backed async wf producer", tag.AsyncWFQueueID(q.config.ID()))
	producer := newProducer(q.config.QueueName, p.HistoryClient, p.NumHistoryShards, p.Logger, p.MetricsClient)
	withMetricsOpt := messaging.WithMetricTags(metrics.TopicTag(q.config.QueueName))
	return messaging.NewMetricProducer(producer, p.MetricsClient, withMetricsOpt), nil
}
