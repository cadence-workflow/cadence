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

package sysworkflow

import (
	"github.com/uber-go/tally"
	"github.com/uber/cadence/common/metrics"
	"time"
)

type replayMetricsClient struct {
	client   metrics.Client
	isReplay bool
}

// NewReplayMetricsClient creates a metrics client which is aware of cadence's replay mode
func NewReplayMetricsClient(client metrics.Client, isReplay bool) metrics.Client {
	return &replayMetricsClient{
		client:   client,
		isReplay: isReplay,
	}
}

// IncCounter increments a counter metric
func (r *replayMetricsClient) IncCounter(scope int, counter int) {
	if r.isReplay {
		return
	}
	r.client.IncCounter(scope, counter)
}

// AddCounter adds delta to the counter metric
func (r *replayMetricsClient) AddCounter(scope int, counter int, delta int64) {
	if r.isReplay {
		return
	}
	r.client.AddCounter(scope, counter, delta)
}

// StartTimer starts a timer for the given metric name. Time will be recorded when stopwatch is stopped.
func (r *replayMetricsClient) StartTimer(scope int, timer int) tally.Stopwatch {
	if r.isReplay {
		return r.nopStopwatch()
	}
	return r.client.StartTimer(scope, timer)
}

// RecordTimer starts a timer for the given metric name
func (r *replayMetricsClient) RecordTimer(scope int, timer int, d time.Duration) {
	if r.isReplay {
		return
	}
	r.RecordTimer(scope, timer, d)
}

// UpdateGauge reports Gauge type absolute value metric
func (r *replayMetricsClient) UpdateGauge(scope int, gauge int, value float64) {
	if r.isReplay {
		return
	}
	r.UpdateGauge(scope, gauge, value)
}

// Tagged returns a client that adds the given tags to all metrics
func (r *replayMetricsClient) Tagged(tags map[string]string) metrics.Client {
	return &replayMetricsClient{
		client:   r.client.Tagged(tags),
		isReplay: r.isReplay,
	}
}

type nopStopwatchRecorder struct{}

// RecordStopwatch is a nop impl for replay mode
func (n *nopStopwatchRecorder) RecordStopwatch(stopwatchStart time.Time) {}

func (r *replayMetricsClient) nopStopwatch() tally.Stopwatch {
	return tally.NewStopwatch(time.Now(), &nopStopwatchRecorder{})
}
