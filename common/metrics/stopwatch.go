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

// Stopwatch is a helper for simpler tracking of elapsed time, use the
// Stop() method to report time elapsed since its created back to the
// timer or histogram.
type Stopwatch struct {
	start     time.Time
	timers    []tally.Timer
	callbacks []func(time.Duration)
}

// NewStopwatch creates a new immutable stopwatch for recording the start
// time to a stopwatch reporter.
func NewStopwatch(timers ...tally.Timer) Stopwatch {
	return Stopwatch{start: time.Now(), timers: timers}
}

// newStopwatchWithCallback creates a stopwatch that records to the given
// timers and also invokes each callback with the elapsed duration on Stop.
//
// This is an unexported migration shim for StartTimerWithExponentialHistogram —
// it lets one Stopwatch fan out to both a timer and a histogram so callsites
// during the timer→histogram migration can keep their existing
// `defer sw.Stop()` patterns. Once the migration is complete and the
// StartTimerWithExponentialHistogram helper is retired, this constructor and
// the callbacks field can be removed.
//
// Kept unexported on purpose: outside the metrics package, callers should use
// Scope.StartTimerWithExponentialHistogram rather than constructing
// callback-bearing stopwatches directly.
func newStopwatchWithCallback(timers []tally.Timer, callbacks ...func(time.Duration)) Stopwatch {
	return Stopwatch{start: time.Now(), timers: timers, callbacks: callbacks}
}

// NewTestStopwatch returns a new test stopwatch
func NewTestStopwatch() Stopwatch {
	return Stopwatch{start: time.Now(), timers: []tally.Timer{}}
}

// Stop reports time elapsed since the stopwatch start to the recorder.
func (sw Stopwatch) Stop() {
	d := time.Since(sw.start)
	for _, timer := range sw.timers {
		timer.Record(d)
	}
	for _, cb := range sw.callbacks {
		cb(d)
	}
}
