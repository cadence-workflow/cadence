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

package stats

import (
	"sync"
	"testing"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/matching/event"
)

// Benchmark the ReportCounter function to see how it handles frequent updates
func BenchmarkReportCounter(b *testing.B) {
	timeSource := clock.NewRealTimeSource()
	// Initialize the QPS reporter with a smoothing factor and a 1 second bucket interval
	reporter := NewEmaFixedWindowQPSTracker(timeSource, 0.5, time.Second, event.E{})
	reporter.Start()

	// Run the benchmark for b.N iterations
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reporter.ReportCounter(1)
	}

	// Stop the reporter after the benchmark
	b.StopTimer()
	reporter.Stop()
}

// Benchmark the QPS calculation function under high load
func BenchmarkQPS(b *testing.B) {
	timeSource := clock.NewRealTimeSource()
	// Initialize the QPS reporter
	reporter := NewEmaFixedWindowQPSTracker(timeSource, 0.5, time.Second, event.E{})
	reporter.Start()

	// Simulate a number of report updates before calling QPS
	for i := 0; i < 1000; i++ {
		reporter.ReportCounter(1)
	}

	// Benchmark QPS retrieval
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reporter.QPS()
	}

	// Stop the reporter
	b.StopTimer()
	reporter.Stop()
}

// Benchmark the full reporting loop, simulating a real-time system.
func BenchmarkFullReport(b *testing.B) {
	timeSource := clock.NewRealTimeSource()
	// Initialize the QPS reporter
	reporter := NewEmaFixedWindowQPSTracker(timeSource, 0.5, time.Millisecond*100, event.E{}) // 100ms bucket interval
	reporter.Start()

	var wg sync.WaitGroup
	// Number of goroutines for each task
	numReporters := 10
	numQPSQueries := 10
	b.ResetTimer()

	for i := 0; i < numReporters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < b.N; j++ {
				// Report random counter value (simulate workload)
				reporter.ReportCounter(1)
			}
		}()
	}
	for i := 0; i < numQPSQueries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < b.N; j++ {
				// Query QPS value (simulate workload)
				_ = reporter.QPS()
			}
		}()
	}
	wg.Wait()
	// Stop the reporter after the benchmark
	b.StopTimer()
	reporter.Stop()
}

func BenchmarkReportCounterRollingWindow(b *testing.B) {
	timeSource := clock.NewRealTimeSource()
	// Initialize the QPS reporter with a smoothing factor and a 1 second bucket interval
	reporter := NewRollingWindowQPSTracker(timeSource, time.Second, 10)
	reporter.Start()

	// Run the benchmark for b.N iterations
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reporter.ReportCounter(1)
	}

	// Stop the reporter after the benchmark
	b.StopTimer()
	reporter.Stop()
}

func BenchmarkQPSRollingWindow(b *testing.B) {
	timeSource := clock.NewRealTimeSource()
	// Initialize the QPS reporter
	reporter := NewRollingWindowQPSTracker(timeSource, time.Second, 10)
	reporter.Start()

	// Simulate a number of report updates before calling QPS
	for i := 0; i < 1000; i++ {
		reporter.ReportCounter(1)
	}

	// Benchmark QPS retrieval
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reporter.QPS()
	}

	// Stop the reporter
	b.StopTimer()
	reporter.Stop()
}

// Benchmark the full reporting loop, simulating a real-time system.
func BenchmarkFullReportRollingWindow(b *testing.B) {
	timeSource := clock.NewRealTimeSource()
	// Initialize the QPS reporter
	reporter := NewRollingWindowQPSTracker(timeSource, time.Millisecond*100, 10) // 100ms bucket interval
	reporter.Start()

	var wg sync.WaitGroup
	// Number of goroutines for each task
	numReporters := 10
	numQPSQueries := 10
	b.ResetTimer()

	for i := 0; i < numReporters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < b.N; j++ {
				// Report random counter value (simulate workload)
				reporter.ReportCounter(1)
			}
		}()
	}
	for i := 0; i < numQPSQueries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < b.N; j++ {
				// Query QPS value (simulate workload)
				_ = reporter.QPS()
			}
		}()
	}
	wg.Wait()
	// Stop the reporter after the benchmark
	b.StopTimer()
	reporter.Stop()
}
