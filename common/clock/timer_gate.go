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

//go:generate mockgen -package=$GOPACKAGE -destination=timer_gate_mock.go github.com/uber/cadence/common/clock TimerGate
package clock

import (
	"sync"
	"time"
)

type (
	// TimerGate is a timer interface which fires a timer at a given timestamp.
	TimerGate interface {
		// Chan return the channel which will be fired when time is up
		Chan() <-chan time.Time
		// FireAfter check will the timer get fired after a certain time
		FireAfter(t time.Time) bool
		// Update update the timer gate, return true if update is a success
		// success means timer is idle or timer is set with a sooner time to fire
		Update(t time.Time) bool
		// Close shutdown the timer
		Stop()
	}

	timerGateImpl struct {
		sync.RWMutex
		timeSource TimeSource
		timer      Timer
		fireTime   time.Time
	}
)

func NewTimerGate(timeSource TimeSource) TimerGate {
	t := &timerGateImpl{
		timer:      timeSource.NewTimer(0),
		timeSource: timeSource,
	}
	// the timer should be stopped when initialized
	if !t.timer.Stop() {
		// drain the existing signal if exist
		// TODO: the drain can be removed after go 1.23
		<-t.timer.Chan()
	}
	return t
}

func (t *timerGateImpl) Chan() <-chan time.Time {
	return t.timer.Chan()
}

func (t *timerGateImpl) FireAfter(now time.Time) bool {
	t.RLock()
	defer t.RUnlock()
	return t.fireTime.After(now)
}

func (t *timerGateImpl) Stop() {
	t.Lock()
	defer t.Unlock()
	t.fireTime = time.Time{}
	t.timer.Stop()
	// TODO: the drain can be removed after go 1.23
	select {
	case <-t.timer.Chan():
	default:
	}
}

func (t *timerGateImpl) Update(fireTime time.Time) bool {
	t.Lock()
	defer t.Unlock()
	if t.timer.Stop() {
		if t.fireTime.Before(fireTime) {
			// this means the timer, before stopped, is active && next wake up time do not have to be updated
			t.timer.Reset(t.fireTime.Sub(t.timeSource.Now()))
			return false
		}
	}
	// TODO: the drain can be removed after go 1.23
	select {
	case <-t.timer.Chan():
	default:
	}
	// this means the timer, before stopped, is active && next wake up time has to be updated
	// or this means the timer, before stopped, is already fired / never active
	t.fireTime = fireTime
	t.timer.Reset(fireTime.Sub(t.timeSource.Now()))
	// Notifies caller that next notification is reset to fire at passed in 'next' visibility time
	return true
}
