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

package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type (
	timerGateSuite struct {
		suite.Suite
		*require.Assertions

		timerGate TimerGate
	}
)

func TestTimerGeteSuite(t *testing.T) {
	s := new(timerGateSuite)
	suite.Run(t, s)
}

func (s *timerGateSuite) SetupSuite() {

}

func (s *timerGateSuite) TearDownSuite() {

}

func (s *timerGateSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.timerGate = NewTimerGate(NewRealTimeSource())
}

func (s *timerGateSuite) TearDownTest() {
	s.timerGate.Stop()
}

func (s *timerGateSuite) TestTimerFire() {
	now := time.Now()
	newTimer := now.Add(1 * time.Second)
	deadline := now.Add(2 * time.Second)
	s.True(s.timerGate.Update(newTimer))

	select {
	case <-s.timerGate.Chan():
	case <-time.NewTimer(deadline.Sub(now)).C:
		s.Fail("timer should fire before test deadline")
	}
}

func (s *timerGateSuite) TestTimerFireAfterUpdate_Active_Updated_BeforeNow() {
	now := time.Now()
	newTimer := now.Add(9 * time.Second)
	updatedNewTimer := now.Add(-1 * time.Second)
	deadline := now.Add(3 * time.Second)

	s.True(s.timerGate.Update(newTimer))
	select {
	case <-s.timerGate.Chan():
		s.Fail("timer should not fire when current time not updated")
	case <-time.NewTimer(deadline.Sub(now)).C:
	}

	s.True(s.timerGate.Update(updatedNewTimer))
	select {
	case <-s.timerGate.Chan():
	case <-time.NewTimer(deadline.Sub(now)).C:
		s.Fail("timer should fire before test deadline")
	}
}

func (s *timerGateSuite) TestTimerFireAfterUpdate_Active_Updated() {
	now := time.Now()
	newTimer := now.Add(5 * time.Second)
	updatedNewTimer := now.Add(1 * time.Second)
	deadline := now.Add(3 * time.Second)
	s.True(s.timerGate.Update(newTimer))
	s.True(s.timerGate.Update(updatedNewTimer))

	select {
	case <-s.timerGate.Chan():
	case <-time.NewTimer(deadline.Sub(now)).C:
		s.Fail("timer should fire before test deadline")
	}
}

func (s *timerGateSuite) TestTimerFireAfterUpdate_Active_NotUpdated() {
	now := time.Now()
	newTimer := now.Add(1 * time.Second)
	updatedNewTimer := now.Add(3 * time.Second)
	deadline := now.Add(2 * time.Second)
	s.True(s.timerGate.Update(newTimer))
	s.False(s.timerGate.Update(updatedNewTimer))

	select {
	case <-s.timerGate.Chan():
	case <-time.NewTimer(deadline.Sub(now)).C:
		s.Fail("timer should fire before test deadline")
	}
}

func (s *timerGateSuite) TestTimerFireAfterUpdate_NotActive_Updated() {
	now := time.Now()
	newTimer := now.Add(-5 * time.Second)
	updatedNewTimer := now.Add(1 * time.Second)
	deadline := now.Add(3 * time.Second)

	s.True(s.timerGate.Update(newTimer))
	// this is to drain existing signal
	<-s.timerGate.Chan()

	// test setup up complete

	s.True(s.timerGate.Update(updatedNewTimer))
	select {
	case <-s.timerGate.Chan():
	case <-time.NewTimer(deadline.Sub(now)).C:
		s.Fail("timer should fire before test deadline")
	}
}

func (s *timerGateSuite) TestTimerWillFire_Zero() {
	// this test is to validate initial notification will trigger a scan of timer
	s.True(s.timerGate.Update(time.Time{}))
	s.False(s.timerGate.FireAfter(time.Now()))

	select { // this is to drain existing signal
	case <-s.timerGate.Chan():
	case <-time.NewTimer(time.Second).C:
		s.Fail("timer should fire")
	}

	now := time.Now()
	newTimer := now.Add(1 * time.Second)
	deadline := now.Add(2 * time.Second)
	s.True(s.timerGate.Update(newTimer))
	select {
	case <-s.timerGate.Chan():
	case <-time.NewTimer(deadline.Sub(now)).C:
		s.Fail("timer should fire")
	}
	s.True(s.timerGate.Update(time.Time{}))
	select { // this is to drain existing signal
	case <-s.timerGate.Chan():
	case <-time.NewTimer(time.Second).C:
		s.Fail("timer should fire")
	}

	now = time.Now()
	newTimer = now.Add(1 * time.Second)
	s.True(s.timerGate.Update(newTimer))
	s.True(s.timerGate.Update(time.Time{}))
	select { // this is to drain existing signal
	case <-s.timerGate.Chan():
	case <-time.NewTimer(time.Second).C:
		s.Fail("timer should fire")
	}
}

func (s *timerGateSuite) TestTimerFireAfterStopAndUpdate() {
	now := time.Now()
	newTimer := now.Add(1 * time.Second)
	deadline := now.Add(2 * time.Second)
	s.True(s.timerGate.Update(newTimer))
	s.True(s.timerGate.FireAfter(newTimer.Add(-1 * time.Nanosecond)))
	s.timerGate.Stop()
	s.False(s.timerGate.FireAfter(newTimer.Add(-1 * time.Nanosecond)))
	select {
	case <-s.timerGate.Chan():
		s.Fail("timer should be cancelled")
	case <-time.NewTimer(deadline.Sub(now)).C:
	}

	now = time.Now()
	newTimer = now.Add(1 * time.Second)
	deadline = now.Add(2 * time.Second)
	s.True(s.timerGate.Update(newTimer))
	s.True(s.timerGate.FireAfter(newTimer.Add(-1 * time.Nanosecond)))
	select {
	case <-s.timerGate.Chan():
	case <-time.NewTimer(deadline.Sub(now)).C:
		s.Fail("timer should fire")
	}
}

func (s *timerGateSuite) TestTimerWillFire() {
	now := time.Now()
	newTimer := now.Add(2 * time.Second)
	timeBeforeNewTimer := now.Add(1 * time.Second)
	timeAfterNewTimer := now.Add(3 * time.Second)
	s.timerGate.Update(newTimer)
	s.True(s.timerGate.FireAfter(timeBeforeNewTimer))
	s.False(s.timerGate.FireAfter(timeAfterNewTimer))
	s.timerGate.Stop()
	s.False(s.timerGate.FireAfter(timeBeforeNewTimer))
}
