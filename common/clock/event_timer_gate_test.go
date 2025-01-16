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
	eventTimerGateSuite struct {
		suite.Suite
		*require.Assertions

		currentTime    time.Time
		eventTimerGate EventTimerGate
	}
)

func TesteventTimerGeteSuite(t *testing.T) {
	s := new(eventTimerGateSuite)
	suite.Run(t, s)
}

func (s *eventTimerGateSuite) SetupSuite() {

}

func (s *eventTimerGateSuite) TearDownSuite() {

}

func (s *eventTimerGateSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.currentTime = time.Now().Add(-10 * time.Minute)
	s.eventTimerGate = NewEventTimerGate(s.currentTime)
}

func (s *eventTimerGateSuite) TearDownTest() {

}

func (s *eventTimerGateSuite) TestTimerFire() {
	now := s.currentTime
	newTimer := now.Add(1 * time.Second)
	deadline := now.Add(2 * time.Second)
	s.eventTimerGate.Update(newTimer)

	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire when current time not updated")
	case <-time.NewTimer(deadline.Sub(now)).C:
	}

	s.eventTimerGate.SetCurrentTime(deadline)
	select {
	case <-s.eventTimerGate.Chan():
	default:
		s.Fail("timer should fire")
	}
}

func (s *eventTimerGateSuite) TestTimerFireAfterUpdate_Active_Updated_BeforeNow() {
	now := s.currentTime
	newTimer := now.Add(5 * time.Second)
	updatedNewTimer := now.Add(-1 * time.Second)
	deadline := now.Add(3 * time.Second)

	s.eventTimerGate.Update(newTimer)
	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire when current time not updated")
	case <-time.NewTimer(deadline.Sub(now)).C:
	}

	s.True(s.eventTimerGate.Update(updatedNewTimer))
	select {
	case <-s.eventTimerGate.Chan():
	default:
		s.Fail("timer should fire")
	}
}

func (s *eventTimerGateSuite) TestTimerFireAfterUpdate_Active_Updated() {
	now := s.currentTime
	newTimer := now.Add(5 * time.Second)
	updatedNewTimer := now.Add(1 * time.Second)
	deadline := now.Add(3 * time.Second)
	s.eventTimerGate.Update(newTimer)
	s.True(s.eventTimerGate.Update(updatedNewTimer))

	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire when current time not updated")
	case <-time.NewTimer(deadline.Sub(now)).C:
	}

	s.eventTimerGate.SetCurrentTime(updatedNewTimer)
	select {
	case <-s.eventTimerGate.Chan():
	default:
		s.Fail("timer should fire")
	}
}

func (s *eventTimerGateSuite) TestTimerFireAfterUpdate_Active_NotUpdated() {
	now := s.currentTime
	newTimer := now.Add(1 * time.Second)
	updatedNewTimer := now.Add(3 * time.Second)
	deadline := now.Add(2 * time.Second)
	s.eventTimerGate.Update(newTimer)
	s.False(s.eventTimerGate.Update(updatedNewTimer))

	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire when current time not updated")
	case <-time.NewTimer(deadline.Sub(now)).C:
	}

	s.eventTimerGate.SetCurrentTime(updatedNewTimer)
	select {
	case <-s.eventTimerGate.Chan():
	default:
		s.Fail("timer should fire")
	}
}

func (s *eventTimerGateSuite) TestTimerFireAfterUpdate_NotActive_Updated() {
	now := s.currentTime
	newTimer := now.Add(-5 * time.Second)
	updatedNewTimer := now.Add(1 * time.Second)
	deadline := now.Add(2 * time.Second)

	s.eventTimerGate.Update(newTimer)
	// this is to drain existing signal
	<-s.eventTimerGate.Chan()

	// test setup up complete

	s.True(s.eventTimerGate.Update(updatedNewTimer))
	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire when current time not updated")
	case <-time.NewTimer(deadline.Sub(now)).C:
	}

	s.eventTimerGate.SetCurrentTime(updatedNewTimer)
	select {
	case <-s.eventTimerGate.Chan():
	default:
		s.Fail("timer should fire")
	}
}

func (s *eventTimerGateSuite) TestTimerFireAfterUpdate_NotActive_NotUpdated() {
	now := s.currentTime
	newTimer := now.Add(-5 * time.Second)
	updatedNewTimer := now.Add(-1 * time.Second)

	s.eventTimerGate.Update(newTimer)
	// this is to drain existing signal
	<-s.eventTimerGate.Chan()

	// test setup up complete

	s.True(s.eventTimerGate.Update(updatedNewTimer))
	select {
	case <-s.eventTimerGate.Chan():
	default:
		s.Fail("timer should fire when new timer is in the past")
	}
}

func (s *eventTimerGateSuite) TestTimerSetCurrentTime_NoUpdate() {
	now := s.currentTime
	newCurrentTime := now.Add(-1 * time.Second)
	s.False(s.eventTimerGate.SetCurrentTime(newCurrentTime))
	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire")
	default:
	}
}

func (s *eventTimerGateSuite) TestTimerSetCurrentTime_Update_TimerAlreadyFired() {
	now := s.currentTime
	newTimer := now.Add(-1 * time.Second)
	newCurrentTime := now.Add(1 * time.Second)

	s.eventTimerGate.Update(newTimer)
	// this is to drain existing signal
	<-s.eventTimerGate.Chan()

	// test setup up complete

	s.True(s.eventTimerGate.SetCurrentTime(newCurrentTime))
	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire")
	default:
	}
}

func (s *eventTimerGateSuite) TestTimerSetCurrentTime_Update_TimerNotFired() {
	now := s.currentTime
	newTimer := now.Add(2 * time.Second)
	newCurrentTime := now.Add(1 * time.Second)

	s.eventTimerGate.Update(newTimer)
	s.True(s.eventTimerGate.SetCurrentTime(newCurrentTime))
	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire")
	default:
	}
}

func (s *eventTimerGateSuite) TestTimerSetCurrentTime_Update_TimerFired() {
	now := s.currentTime
	newTimer := now.Add(2 * time.Second)
	newCurrentTime := now.Add(2 * time.Second)

	s.eventTimerGate.Update(newTimer)
	s.True(s.eventTimerGate.SetCurrentTime(newCurrentTime))
	select {
	case <-s.eventTimerGate.Chan():
	default:
		s.Fail("timer should fire")
	}

	// should fire only once
	newCurrentTime = newCurrentTime.Add(1 * time.Second)
	s.True(s.eventTimerGate.SetCurrentTime(newCurrentTime))
	select {
	case <-s.eventTimerGate.Chan():
		s.Fail("timer should not fire")
	default:
	}
}

func (s *eventTimerGateSuite) TestTimerWillFire_Zero() {
	// this test is to validate initial notification will trigger a scan of timer
	s.eventTimerGate.Update(time.Time{})
	s.False(s.eventTimerGate.FireAfter(time.Now()))
}

func (s *eventTimerGateSuite) TestTimerWillFire_Active() {
	now := s.currentTime
	newTimer := now.Add(2 * time.Second)
	timeBeforeNewTimer := now.Add(1 * time.Second)
	timeAfterNewimer := now.Add(3 * time.Second)
	s.eventTimerGate.Update(newTimer)
	s.True(s.eventTimerGate.FireAfter(timeBeforeNewTimer))
	s.False(s.eventTimerGate.FireAfter(timeAfterNewimer))
}

func (s *eventTimerGateSuite) TestTimerWillFire_NotActive() {
	now := s.currentTime
	newTimer := now.Add(-2 * time.Second)
	timeBeforeTimer := now.Add(-3 * time.Second)
	timeAfterTimer := now.Add(1 * time.Second)
	s.eventTimerGate.Update(newTimer)
	s.False(s.eventTimerGate.FireAfter(timeBeforeTimer))
	s.False(s.eventTimerGate.FireAfter(timeAfterTimer))
}
