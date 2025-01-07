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
	"time"

	"go.uber.org/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/matching/event"
)

type (
	// emaFixedWindowQPSTracker is a QPSTracker that uses a fixed time period to calculate QPS and an exponential moving average algorithm to estimate QPS.
	emaFixedWindowQPSTracker struct {
		timeSource            clock.TimeSource
		exp                   float64
		bucketInterval        time.Duration
		bucketIntervalSeconds float64
		wg                    sync.WaitGroup
		done                  chan struct{}
		status                *atomic.Int32
		firstBucket           bool
		baseEvent             event.E
		qps                   *atomic.Float64
		counter               *atomic.Int64
	}
)

func NewEmaFixedWindowQPSTracker(timeSource clock.TimeSource, exp float64, bucketInterval time.Duration, baseEvent event.E) QPSTracker {
	return &emaFixedWindowQPSTracker{
		timeSource:            timeSource,
		exp:                   exp,
		bucketInterval:        bucketInterval,
		bucketIntervalSeconds: float64(bucketInterval) / float64(time.Second),
		done:                  make(chan struct{}),
		status:                atomic.NewInt32(common.DaemonStatusInitialized),
		firstBucket:           true,
		counter:               atomic.NewInt64(0),
		qps:                   atomic.NewFloat64(0),
		baseEvent:             baseEvent,
	}
}

func (r *emaFixedWindowQPSTracker) Start() {
	if !r.status.CompareAndSwap(common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	r.wg.Add(1)
	go r.reportLoop()
}

func (r *emaFixedWindowQPSTracker) reportLoop() {
	defer r.wg.Done()
	ticker := r.timeSource.NewTicker(r.bucketInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			r.report()
		case <-r.done:
			return
		}
	}
}

func (r *emaFixedWindowQPSTracker) report() {
	if r.firstBucket {
		counter := r.counter.Swap(0)
		r.store(float64(counter) / r.bucketIntervalSeconds)
		r.firstBucket = false
		return
	}
	counter := r.counter.Swap(0)
	qps := r.qps.Load()
	r.store(qps*(1-r.exp) + float64(counter)*r.exp/r.bucketIntervalSeconds)
}

func (r *emaFixedWindowQPSTracker) store(qps float64) {
	r.qps.Store(qps)
	e := r.baseEvent
	e.EventName = "QPSTrackerUpdate"
	e.Payload = map[string]any{
		"QPS": qps,
	}
	event.Log(e)
}

func (r *emaFixedWindowQPSTracker) Stop() {
	if !r.status.CompareAndSwap(common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	close(r.done)
	r.wg.Wait()
}

func (r *emaFixedWindowQPSTracker) ReportCounter(delta int64) {
	r.counter.Add(delta)
}

func (r *emaFixedWindowQPSTracker) QPS() float64 {
	return r.qps.Load()
}

type (
	bucket struct {
		counter   int64
		timeIndex int
	}
	rollingWindowQPSTracker struct {
		sync.RWMutex
		timeSource     clock.TimeSource
		bucketInterval time.Duration
		buckets        []bucket

		counter   int64
		timeIndex int
	}
)

func NewRollingWindowQPSTracker(timeSource clock.TimeSource, bucketInterval time.Duration, numBuckets int) QPSTracker {
	return &rollingWindowQPSTracker{
		timeSource:     timeSource,
		bucketInterval: bucketInterval,
		buckets:        make([]bucket, numBuckets),
	}
}

func (r *rollingWindowQPSTracker) Start() {
}

func (r *rollingWindowQPSTracker) Stop() {
}

func (r *rollingWindowQPSTracker) getCurrentTimeIndex() int {
	now := r.timeSource.Now()
	return int(now.UnixNano() / int64(r.bucketInterval))
}

func (r *rollingWindowQPSTracker) ReportCounter(delta int64) {
	r.Lock()
	defer r.Unlock()
	currentIndex := r.getCurrentTimeIndex()
	if currentIndex == r.timeIndex {
		r.counter += delta
		return
	}
	r.buckets[r.timeIndex%len(r.buckets)] = bucket{
		counter:   r.counter,
		timeIndex: r.timeIndex,
	}
	r.timeIndex = currentIndex
	r.counter = delta
}

func (r *rollingWindowQPSTracker) QPS() float64 {
	r.RLock()
	defer r.RUnlock()
	currentIndex := r.getCurrentTimeIndex()
	totalCounter := int64(0)
	for _, b := range r.buckets {
		if currentIndex-b.timeIndex <= len(r.buckets) {
			totalCounter += b.counter
		}
	}
	if currentIndex != r.timeIndex && currentIndex-r.timeIndex <= len(r.buckets) {
		totalCounter += r.counter
	}
	return float64(totalCounter) / float64(r.bucketInterval) / float64(len(r.buckets)) * float64(time.Second)
}
