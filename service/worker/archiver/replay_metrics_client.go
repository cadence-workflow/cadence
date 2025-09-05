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

package archiver

import (
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/metrics"
)

type (
	replayMetricsClient struct {
		client metrics.Client
		ctx    workflow.Context
	}

	replayMetricsScope struct {
		scope metrics.Scope
		ctx   workflow.Context
	}
)

// NewReplayMetricsClient creates a metrics client which is aware of cadence's replay mode
func NewReplayMetricsClient(client metrics.Client, ctx workflow.Context) metrics.Client {
	return &replayMetricsClient{
		client: client,
		ctx:    ctx,
	}
}

// IncCounter increments a counter metric
func (r *replayMetricsClient) IncCounter(scope metrics.ScopeIdx, counter metrics.MetricIdx) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.client.IncCounter(scope, counter)
}

// AddCounter adds delta to the counter metric
func (r *replayMetricsClient) AddCounter(scope metrics.ScopeIdx, counter metrics.MetricIdx, delta int64) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.client.AddCounter(scope, counter, delta)
}

// StartTimer starts a timer for the given metric name. Time will be recorded when stopwatch is stopped.
func (r *replayMetricsClient) StartTimer(scope metrics.ScopeIdx, timer metrics.MetricIdx) tally.Stopwatch {
	if workflow.IsReplaying(r.ctx) {
		return metrics.NopStopwatch()
	}
	return r.client.StartTimer(scope, timer)
}

// RecordTimer starts a timer for the given metric name
func (r *replayMetricsClient) RecordTimer(scope metrics.ScopeIdx, timer metrics.MetricIdx, d time.Duration) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.client.RecordTimer(scope, timer, d)
}

// RecordHistogramDuration record and emit a duration
func (r *replayMetricsClient) RecordHistogramDuration(scope metrics.ScopeIdx, timer metrics.MetricIdx, d time.Duration) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.client.RecordHistogramDuration(scope, timer, d)
}

// UpdateGauge reports Gauge type absolute value metric
func (r *replayMetricsClient) UpdateGauge(scope metrics.ScopeIdx, gauge metrics.MetricIdx, value float64) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.client.UpdateGauge(scope, gauge, value)
}

// Scope returns a client that adds the given tags to all metrics
func (r *replayMetricsClient) Scope(scope metrics.ScopeIdx, tags ...metrics.Tag) metrics.Scope {
	return NewReplayMetricsScope(r.client.Scope(scope, tags...), r.ctx)
}

// NewReplayMetricsScope creates a metrics scope which is aware of cadence's replay mode
func NewReplayMetricsScope(scope metrics.Scope, ctx workflow.Context) metrics.Scope {
	return &replayMetricsScope{
		scope: scope,
		ctx:   ctx,
	}
}

// IncCounter increments a counter metric
func (r *replayMetricsScope) IncCounter(counter metrics.MetricIdx) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.IncCounter(counter)
}

// AddCounter adds delta to the counter metric
func (r *replayMetricsScope) AddCounter(counter metrics.MetricIdx, delta int64) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.AddCounter(counter, delta)
}

// StartTimer starts a timer for the given metric name. Time will be recorded when stopwatch is stopped.
func (r *replayMetricsScope) StartTimer(timer metrics.MetricIdx) metrics.Stopwatch {
	if workflow.IsReplaying(r.ctx) {
		return metrics.NewTestStopwatch()
	}
	return r.scope.StartTimer(timer)
}

// RecordTimer starts a timer for the given metric name
func (r *replayMetricsScope) RecordTimer(timer metrics.MetricIdx, d time.Duration) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.RecordTimer(timer, d)
}

// RecordHistogramDuration records a duration value in a histogram
func (r *replayMetricsScope) RecordHistogramDuration(timer metrics.MetricIdx, d time.Duration) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.RecordHistogramDuration(timer, d)
}

// RecordHistogramValue records a value in a histogram
func (r *replayMetricsScope) RecordHistogramValue(timer metrics.MetricIdx, value float64) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.RecordHistogramValue(timer, value)
}

func (r *replayMetricsScope) ExponentialHistogram(hist metrics.MetricIdx, d time.Duration) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.ExponentialHistogram(hist, d)
}

func (r *replayMetricsScope) IntExponentialHistogram(hist metrics.MetricIdx, value int) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.IntExponentialHistogram(hist, value)
}

// UpdateGauge reports Gauge type absolute value metric
func (r *replayMetricsScope) UpdateGauge(gauge metrics.MetricIdx, value float64) {
	if workflow.IsReplaying(r.ctx) {
		return
	}
	r.scope.UpdateGauge(gauge, value)
}

// Tagged return an internal replay aware scope that can be used to add additional information to metrics
func (r *replayMetricsScope) Tagged(tags ...metrics.Tag) metrics.Scope {
	return NewReplayMetricsScope(r.scope.Tagged(tags...), r.ctx)
}
