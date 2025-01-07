// Copyright (c) 2017 Uber Technologies, Inc.
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

package messaging

import (
	"context"

	"github.com/uber/cadence/common/metrics"
)

type MetricsProducer struct {
	producer Producer
	scope    metrics.Scope
	tags     []metrics.Tag
}

type MetricProducerOptions func(*MetricsProducer)

func WithMetricTags(tags ...metrics.Tag) MetricProducerOptions {
	return func(p *MetricsProducer) {
		p.tags = tags
	}
}

// NewMetricProducer creates a new instance of producer that emits metrics
func NewMetricProducer(
	producer Producer,
	metricsClient metrics.Client,
	opts ...MetricProducerOptions,
) Producer {
	p := &MetricsProducer{
		producer: producer,
	}

	for _, opt := range opts {
		opt(p)
	}

	p.scope = metricsClient.Scope(metrics.MessagingClientPublishScope, p.tags...)
	return p
}

func (p *MetricsProducer) Publish(ctx context.Context, msg interface{}) error {
	p.scope.IncCounter(metrics.CadenceClientRequests)

	sw := p.scope.StartTimer(metrics.CadenceClientLatency)
	err := p.producer.Publish(ctx, msg)
	sw.Stop()

	if err != nil {
		p.scope.IncCounter(metrics.CadenceClientFailures)
	}
	return err
}

func (p *MetricsProducer) Close() error {
	if closeableProducer, ok := p.producer.(CloseableProducer); ok {
		return closeableProducer.Close()
	}

	return nil
}
