// Copyright (c) 2020 Uber Technologies, Inc.
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
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

type (
	redispatcherSuite struct {
		suite.Suite
		*require.Assertions

		controller     *gomock.Controller
		mockProcessor  *MockProcessor
		mockTimeSource clock.MockedTimeSource
		options        *RedispatcherOptions

		metricsScope metrics.Scope
		logger       log.Logger

		redispatcher *redispatcherImpl
	}
)

func TestRedispatcherSuite(t *testing.T) {
	s := new(redispatcherSuite)
	suite.Run(t, s)
}

func (s *redispatcherSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockProcessor = NewMockProcessor(s.controller)
	s.mockTimeSource = clock.NewMockedTimeSource()
	s.options = &RedispatcherOptions{
		TaskRedispatchInterval: dynamicconfig.GetDurationPropertyFn(time.Millisecond * 50),
	}

	s.metricsScope = metrics.NewClient(tally.NoopScope, metrics.History).Scope(0)
	s.logger = testlogger.New(s.T())

	s.redispatcher = NewRedispatcher(
		s.mockProcessor,
		s.mockTimeSource,
		s.options,
		s.logger,
		s.metricsScope,
	).(*redispatcherImpl)
}

func (s *redispatcherSuite) TearDownTest() {
	s.redispatcher.Stop()
}

func (s *redispatcherSuite) TestRedispatch_ProcessorShutDown() {
	numTasks := 5

	successfullyRedispatched := 3
	stopDoneCh := make(chan struct{})
	s.mockProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).Times(successfullyRedispatched)
	s.mockProcessor.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(_ interface{}) (bool, error) {
		go func() {
			s.redispatcher.Stop()
			close(stopDoneCh)
		}()

		<-s.redispatcher.shutdownCh
		return false, errors.New("processor shutdown")
	}).Times(1)

	for i := 0; i < numTasks; i++ {
		mockTask := NewMockTask(s.controller)
		mockTask.EXPECT().Priority().Return(rand.Intn(5)).AnyTimes()
		mockTask.EXPECT().GetAttempt().Return(0).Times(1)
		s.redispatcher.AddTask(mockTask)
	}

	s.Equal(numTasks, s.redispatcher.Size())
	s.mockTimeSource.Advance(2 * s.options.TaskRedispatchInterval())
	s.redispatcher.Start()
	<-s.redispatcher.shutdownCh
	<-stopDoneCh

	s.Equal(numTasks-successfullyRedispatched, s.redispatcher.Size())
}

func (s *redispatcherSuite) TestRedispatchWithTargetSize() {
	numTasks := 20
	targetSize := 10

	// First, add tasks to redispatcher
	for i := 0; i < numTasks; i++ {
		mockTask := NewMockTask(s.controller)
		mockTask.EXPECT().GetAttempt().Return(0).Times(1)
		s.redispatcher.AddTask(mockTask)
	}

	s.mockProcessor.EXPECT().TrySubmit(gomock.Any()).Return(false, errors.New("some error")).Times(2)
	s.mockProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil).Times(10)

	s.mockTimeSource.Advance(2 * s.options.TaskRedispatchInterval())
	s.redispatcher.Redispatch(targetSize)

	// implementation can choose to redispatch more tasks than needed
	sz := s.redispatcher.Size()
	s.Equal(numTasks-targetSize, sz)
}

func (s *redispatcherSuite) TestRedispatch_Backoff() {
	numTasks := 50
	numLowAttemptTasks := 0
	numHighAttemptTasks := 0
	for i := 0; i < numTasks; i++ {
		attempt := 100
		if rand.Intn(2) == 0 {
			numLowAttemptTasks++
			attempt = 0
		} else {
			numHighAttemptTasks++
		}

		mockTask := NewMockTask(s.controller)
		mockTask.EXPECT().Priority().Return(rand.Intn(5)).AnyTimes()
		mockTask.EXPECT().GetAttempt().Return(attempt).Times(1)
		s.redispatcher.AddTask(mockTask)
		s.mockProcessor.EXPECT().TrySubmit(NewMockTaskMatcher(mockTask)).Return(true, nil).MaxTimes(1)
	}

	s.redispatcher.Start()
	s.redispatcher.timerGate.Stop()
	s.mockTimeSource.Advance(2 * s.options.TaskRedispatchInterval())
	s.redispatcher.Redispatch(numTasks)

	s.Equal(numHighAttemptTasks, s.redispatcher.Size())
}

func (s *redispatcherSuite) TestRedispatch_Random() {
	numTasks := 100
	dispatched := 0

	for i := 0; i != numTasks; i++ {
		submitted := false
		attempt := 100
		if rand.Intn(2) == 0 {
			submitted = true
			if rand.Intn(2) == 0 {
				dispatched++
				attempt = 0
			}
		}

		mockTask := NewMockTask(s.controller)
		mockTask.EXPECT().Priority().Return(rand.Intn(5)).AnyTimes()
		mockTask.EXPECT().GetAttempt().Return(attempt).Times(1)
		s.redispatcher.AddTask(mockTask)
		s.mockProcessor.EXPECT().TrySubmit(NewMockTaskMatcher(mockTask)).Return(submitted, nil).MaxTimes(1)
	}

	s.redispatcher.Start()
	s.redispatcher.timerGate.Stop()
	s.mockTimeSource.Advance(2 * s.options.TaskRedispatchInterval())
	s.redispatcher.Redispatch(0)

	// implementation can choose to stop redispatch for a certain priority when previous submit has failed
	s.True(s.redispatcher.Size() >= numTasks-dispatched)
}
