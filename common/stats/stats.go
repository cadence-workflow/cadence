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
		groups         sync.Map
		root           *emaFixedWindowState
		timeSource     clock.TimeSource
		exp            float64
		bucketInterval time.Duration
		wg             sync.WaitGroup
		done           chan struct{}
		status         *atomic.Int32
		baseEvent      event.E
	}

	emaFixedWindowState struct {
		exp                   float64
		bucketIntervalSeconds float64
		firstBucket           bool
		baseEvent             event.E
		qps                   *atomic.Float64
		counter               *atomic.Int64
	}
)

func NewEmaFixedWindowQPSTracker(timeSource clock.TimeSource, exp float64, bucketInterval time.Duration, baseEvent event.E) QPSTrackerGroup {
	return &emaFixedWindowQPSTracker{
		root:           newEmaFixedWindowState(exp, bucketInterval, baseEvent),
		timeSource:     timeSource,
		exp:            exp,
		bucketInterval: bucketInterval,
		done:           make(chan struct{}),
		status:         atomic.NewInt32(common.DaemonStatusInitialized),
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
	r.root.report()
	r.groups.Range(func(key, value any) bool {
		state := value.(*emaFixedWindowState)
		state.report()
		return true
	})
}

func (r *emaFixedWindowQPSTracker) Stop() {
	if !r.status.CompareAndSwap(common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	close(r.done)
	r.wg.Wait()
}

func (r *emaFixedWindowQPSTracker) ReportCounter(amount int64) {
	r.root.add(amount)
}

func (r *emaFixedWindowQPSTracker) ReportGroup(group string, amount int64) {
	r.root.add(amount)
	r.getOrCreate(group).add(amount)
}

func (r *emaFixedWindowQPSTracker) GroupQPS(group string) float64 {
	state, ok := r.get(group)
	if !ok {
		return 0
	}
	return state.getQPS()
}

func (r *emaFixedWindowQPSTracker) QPS() float64 {
	return r.root.getQPS()
}

func (r *emaFixedWindowQPSTracker) get(group string) (*emaFixedWindowState, bool) {
	res, ok := r.groups.Load(group)
	if !ok {
		return nil, false
	}
	return res.(*emaFixedWindowState), true
}

func (r *emaFixedWindowQPSTracker) getOrCreate(group string) *emaFixedWindowState {
	res, ok := r.groups.Load(group)
	if !ok {
		e := r.baseEvent
		e.Payload = map[string]any{
			"group": group,
		}
		res, _ = r.groups.LoadOrStore(group, newEmaFixedWindowState(r.exp, r.bucketInterval, e))
	}
	return res.(*emaFixedWindowState)
}

func newEmaFixedWindowState(exp float64, bucketInterval time.Duration, baseEvent event.E) *emaFixedWindowState {
	return &emaFixedWindowState{
		exp:                   exp,
		bucketIntervalSeconds: bucketInterval.Seconds(),
		firstBucket:           true,
		baseEvent:             baseEvent,
		qps:                   atomic.NewFloat64(0),
		counter:               atomic.NewInt64(0),
	}
}

func (s *emaFixedWindowState) report() {
	if s.firstBucket {
		counter := s.counter.Swap(0)
		s.store(float64(counter) / s.bucketIntervalSeconds)
		s.firstBucket = false
		return
	}
	counter := s.counter.Swap(0)
	qps := s.qps.Load()
	s.store(qps*(1-s.exp) + float64(counter)*s.exp/s.bucketIntervalSeconds)
}

func (s *emaFixedWindowState) store(qps float64) {
	s.qps.Store(qps)
	e := s.baseEvent
	e.EventName = "QPSTrackerUpdate"
	payload := make(map[string]any, len(e.Payload)+1)
	for k, v := range e.Payload {
		payload[k] = v
	}
	payload["QPS"] = qps
	e.Payload = payload
	event.Log(e)
}

func (s *emaFixedWindowState) add(delta int64) {
	s.counter.Add(delta)
}

func (s *emaFixedWindowState) getQPS() float64 {
	return s.qps.Load()
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
