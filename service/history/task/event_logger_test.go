// Copyright (c) 2017-2021 Uber Technologies, Inc.
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

package task

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
)

type (
	eventLoggerSuite struct {
		*require.Assertions
		suite.Suite

		mockLogger *log.MockLogger

		eventLogger *eventLoggerImpl
	}
)

func TestEventLoggerSuite(t *testing.T) {
	s := new(eventLoggerSuite)
	suite.Run(t, s)
}

func (s *eventLoggerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.mockLogger = log.NewMockLogger(gomock.NewController(s.T()))

	s.eventLogger = newEventLogger(
		s.mockLogger,
		clock.NewRealTimeSource(),
		defaultTaskEventLoggerSize,
	).(*eventLoggerImpl)
}

func (s *eventLoggerSuite) TestAddEvent() {
	for i := 0; i != defaultTaskEventLoggerSize*2; i++ {
		s.eventLogger.AddEvent("some random event", i)
		s.Equal((i+1)%defaultTaskEventLoggerSize, s.eventLogger.nextEventIdx)
	}

	for i := 0; i != defaultTaskEventLoggerSize; i++ {
		// check if old events got overwritten
		s.Equal(i+defaultTaskEventLoggerSize, s.eventLogger.events[i].details[0])
	}

	s.Len(s.eventLogger.events, defaultTaskEventLoggerSize)
}

func (s *eventLoggerSuite) TestFlushEvents() {
	for _, numEvents := range []int{0, defaultTaskEventLoggerSize / 2, defaultTaskEventLoggerSize, defaultTaskEventLoggerSize * 2} {
		for i := 0; i != numEvents; i++ {
			s.eventLogger.AddEvent("some random event")
		}

		expectedEventsFlushed := min(numEvents, defaultTaskEventLoggerSize)
		s.mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		s.Equal(expectedEventsFlushed, s.eventLogger.FlushEvents("some random message"))
	}
}
