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

type (
	// Client is  the interface used to report metrics tally.
	Client interface {
		// IncCounter increments a counter metric
		IncCounter(scope ScopeIdx, counter MetricIdx)
		// AddCounter adds delta to the counter metric
		AddCounter(scope ScopeIdx, counter MetricIdx, delta int64)
		// StartTimer starts a timer for the given
		// metric name. Time will be recorded when stopwatch is stopped.
		StartTimer(scope ScopeIdx, timer MetricIdx) tally.Stopwatch
		// RecordTimer starts a timer for the given
		// metric name
		RecordTimer(scope ScopeIdx, timer MetricIdx, d time.Duration)
		// RecordHistogramDuration records a histogram duration value for the given
		// metric name
		RecordHistogramDuration(scope ScopeIdx, timer MetricIdx, d time.Duration)
		// UpdateGauge reports Gauge type absolute value metric
		UpdateGauge(scope ScopeIdx, gauge MetricIdx, value float64)
		// Scope return an internal scope that can be used to add additional
		// information to metrics
		Scope(scope ScopeIdx, tags ...Tag) Scope
	}

	// Scope is an interface for metrics
	Scope interface {
		// IncCounter increments a counter metric
		IncCounter(counter MetricIdx)
		// AddCounter adds delta to the counter metric
		AddCounter(counter MetricIdx, delta int64)
		// StartTimer starts a timer for the given metric name.
		// Time will be recorded when stopwatch is stopped.
		StartTimer(timer MetricIdx) Stopwatch
		// RecordTimer starts a timer for the given metric name
		RecordTimer(timer MetricIdx, d time.Duration)
		// RecordHistogramDuration records a histogram duration value for the given
		// metric name
		RecordHistogramDuration(timer MetricIdx, d time.Duration)
		// RecordHistogramValue records a histogram value for the given metric name
		RecordHistogramValue(timer MetricIdx, value float64)
		// ExponentialHistogram records a subsettable exponential histogram value for the given metric name
		ExponentialHistogram(hist MetricIdx, d time.Duration)
		// IntExponentialHistogram records a subsettable exponential histogram value for the given metric name
		IntExponentialHistogram(hist MetricIdx, value int)
		// UpdateGauge reports Gauge type absolute value metric
		UpdateGauge(gauge MetricIdx, value float64)
		// Tagged return an internal scope that can be used to add additional
		// information to metrics
		Tagged(tags ...Tag) Scope
	}
)

var sanitizer = tally.NewSanitizer(tally.SanitizeOptions{
	NameCharacters:       tally.ValidCharacters{tally.AlphanumericRange, tally.UnderscoreCharacters},
	KeyCharacters:        tally.ValidCharacters{tally.AlphanumericRange, tally.UnderscoreCharacters},
	ValueCharacters:      tally.ValidCharacters{tally.AlphanumericRange, tally.UnderscoreCharacters},
	ReplacementCharacter: '_',
})
