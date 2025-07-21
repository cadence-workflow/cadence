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

import "time"

// Sustain tracks whether a boolean value is consistently true over a dynamic duration. It does this by recording the earliest
// time that it received a true value, and clearing that timestamp any time that a false value is encountered.
// The timestamp is only initialized when a true datapoint is accepted.
type Sustain struct {
	started  time.Time
	source   TimeSource
	duration func() time.Duration
}

func NewSustain(source TimeSource, duration func() time.Duration) Sustain {
	return Sustain{
		source:   source,
		duration: duration,
	}
}

// Check accepts a datapoint and returns true if the condition has been sustained. For example, if the duration was
// 60s, and a true datapoint was accepted every second, it would return false until 60s had elapsed from the first datapoint
// and then subsequently return true.
func (s *Sustain) Check(value bool) bool {
	if value {
		now := s.source.Now()
		if s.started.IsZero() {
			s.started = now
		}
		if now.Sub(s.started) >= s.duration() {
			return true
		}
	} else {
		s.Reset()
	}
	return false
}

// CheckAndReset accepts a datapoint and returns true if the condition has been sustained.
// If the condition has been sustained the timestamp is set to the current time so that it will be considered sustained
// again until after the duration again elapses.
// For example, if the duration was 60s, and a true datapoint was accepted every second, it would return true once every 60s and
// otherwise return false
func (s *Sustain) CheckAndReset(value bool) bool {
	if value {
		now := s.source.Now()
		if s.started.IsZero() {
			s.started = now
		}
		if now.Sub(s.started) >= s.duration() {
			s.started = now
			return true
		}
	} else {
		s.Reset()
	}
	return false
}

// Reset clears the datapoints that the Sustain has received. It is equivalent to providing a false datapoint
func (s *Sustain) Reset() {
	s.started = time.Time{}
}
