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

package metrics

import (
	"time"

	"github.com/uber-go/tally"
)

var (
	NoopClient    Client = &noopClientImpl{}
	NoopScope     Scope  = &noopScopeImpl{}
	NoopStopwatch        = tally.NewStopwatch(time.Now(), &nopStopwatchRecorder{})
)

type nopStopwatchRecorder struct{}

// RecordStopwatch is a nop impl for replay mode
func (n *nopStopwatchRecorder) RecordStopwatch(stopwatchStart time.Time) {}

// NopStopwatch return a fake tally stop watch
func NopStopwatch() tally.Stopwatch {
	return tally.NewStopwatch(time.Now(), &nopStopwatchRecorder{})
}

type noopClientImpl struct{}

func (n noopClientImpl) IncCounter(scope ScopeIdx, counter MetricIdx) {
}

func (n noopClientImpl) AddCounter(scope ScopeIdx, counter MetricIdx, delta int64) {
}

func (n noopClientImpl) StartTimer(scope ScopeIdx, timer MetricIdx) tally.Stopwatch {
	return NoopStopwatch
}

func (n noopClientImpl) RecordTimer(scope ScopeIdx, timer MetricIdx, d time.Duration) {
}

func (n *noopClientImpl) RecordHistogramDuration(scope ScopeIdx, timer MetricIdx, d time.Duration) {
}

func (n noopClientImpl) UpdateGauge(scope ScopeIdx, gauge MetricIdx, value float64) {
}

func (n noopClientImpl) Scope(scope ScopeIdx, tags ...Tag) Scope {
	return NoopScope
}

// NewNoopMetricsClient initialize new no-op metrics client
func NewNoopMetricsClient() Client {
	return &noopClientImpl{}
}

type noopScopeImpl struct{}

func (n *noopScopeImpl) IncCounter(counter MetricIdx) {
}

func (n *noopScopeImpl) AddCounter(counter MetricIdx, delta int64) {
}

func (n *noopScopeImpl) StartTimer(timer MetricIdx) Stopwatch {
	return NewTestStopwatch()
}

func (n *noopScopeImpl) RecordTimer(timer MetricIdx, d time.Duration) {
}

func (n *noopScopeImpl) RecordHistogramDuration(timer MetricIdx, d time.Duration) {
}

func (n *noopScopeImpl) RecordHistogramValue(timer MetricIdx, value float64) {
}

func (n *noopScopeImpl) UpdateGauge(gauge MetricIdx, value float64) {
}

func (n *noopScopeImpl) Tagged(tags ...Tag) Scope {
	return n
}
