// Modifications Copyright (c) 2020 Uber Technologies Inc.

// Copyright (c) 2020 Temporal Technologies, Inc.

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

package liveness

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
)

type (
	Liveness struct {
		status     int32
		timeSource clock.TimeSource
		ttl        time.Duration

		// stopCh is used to signal the liveness to stop
		stopCh chan struct{}
		// wg is used to wait for the liveness to stop
		wg sync.WaitGroup

		// broadcast shutdown functions
		broadcastShutdownFn func()

		lastEventTimeNano int64
	}
)

var _ common.Daemon = (*Liveness)(nil)

func NewLiveness(timeSource clock.TimeSource, ttl time.Duration, broadcastShutdownFn func()) *Liveness {
	return &Liveness{
		status:              common.DaemonStatusInitialized,
		timeSource:          timeSource,
		ttl:                 ttl,
		stopCh:              make(chan struct{}),
		broadcastShutdownFn: broadcastShutdownFn,
		lastEventTimeNano:   timeSource.Now().UnixNano(),
	}
}

func (l *Liveness) Start() {
	if !atomic.CompareAndSwapInt32(&l.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	l.wg.Add(1)
	checkTimer := l.timeSource.NewTicker(l.ttl / 2)
	go l.eventLoop(checkTimer)
}

func (l *Liveness) Stop() {
	if !atomic.CompareAndSwapInt32(&l.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	close(l.stopCh)
	l.broadcastShutdownFn()
	l.wg.Wait()
}

func (l *Liveness) eventLoop(ticker clock.Ticker) {
	defer l.wg.Done()
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			if !l.IsAlive() {
				l.Stop()
			}

		case <-l.stopCh:
			return
		}
	}
}

func (l *Liveness) IsAlive() bool {
	now := l.timeSource.Now().UnixNano()
	lastUpdate := atomic.LoadInt64(&l.lastEventTimeNano)
	return now-lastUpdate < l.ttl.Nanoseconds()
}

func (l *Liveness) MarkAlive() {
	now := l.timeSource.Now().UnixNano()
	atomic.StoreInt64(&l.lastEventTimeNano, now)
}
