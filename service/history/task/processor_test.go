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
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/config"
)

type (
	queueTaskProcessorSuite struct {
		suite.Suite
		*require.Assertions

		controller           *gomock.Controller
		mockDomainCache      *cache.MockDomainCache
		mockPriorityAssigner *MockPriorityAssigner

		timeSource    clock.TimeSource
		metricsClient metrics.Client
		logger        log.Logger

		processor *processorImpl
	}
)

func TestQueueTaskProcessorSuite(t *testing.T) {
	s := new(queueTaskProcessorSuite)
	suite.Run(t, s)
}

func (s *queueTaskProcessorSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)
	s.mockPriorityAssigner = NewMockPriorityAssigner(s.controller)

	s.timeSource = clock.NewRealTimeSource()
	s.metricsClient = metrics.NewClient(tally.NoopScope, metrics.History)
	s.logger = testlogger.New(s.Suite.T())

	s.processor = s.newTestQueueTaskProcessor()
}

func (s *queueTaskProcessorSuite) TearDownTest() {}

func (s *queueTaskProcessorSuite) TestStartStop() {
	mockScheduler := task.NewMockScheduler(s.controller)
	mockScheduler.EXPECT().Start().Times(1)
	mockScheduler.EXPECT().Stop().Times(1)
	s.processor.scheduler = mockScheduler

	s.processor.Start()
	s.processor.Stop()
}

func (s *queueTaskProcessorSuite) TestStartStopWithNewScheduler() {
	mockScheduler := task.NewMockScheduler(s.controller)
	mockScheduler.EXPECT().Start().Times(2)
	mockScheduler.EXPECT().Stop().Times(2)
	s.processor.scheduler = mockScheduler
	s.processor.newScheduler = mockScheduler

	s.processor.Start()
	s.processor.Stop()
}

func (s *queueTaskProcessorSuite) TestSubmit() {
	mockTask := NewMockTask(s.controller)
	s.mockPriorityAssigner.EXPECT().Assign(NewMockTaskMatcher(mockTask)).Return(nil).Times(1)

	mockScheduler := task.NewMockScheduler(s.controller)
	mockScheduler.EXPECT().Submit(NewMockTaskMatcher(mockTask)).Return(nil).Times(1)

	s.processor.scheduler = mockScheduler

	err := s.processor.Submit(mockTask)
	s.NoError(err)
}

func (s *queueTaskProcessorSuite) TestSubmitNewScheduler() {
	mockTask := NewMockTask(s.controller)
	s.mockPriorityAssigner.EXPECT().Assign(NewMockTaskMatcher(mockTask)).Return(nil).Times(1)

	mockScheduler := task.NewMockScheduler(s.controller)
	mockScheduler.EXPECT().Submit(NewMockTaskMatcher(mockTask)).Return(nil).Times(1)

	s.processor.newScheduler = mockScheduler
	s.processor.newSchedulerProbabilityFn = func(...dynamicconfig.FilterOption) int { return 100 }

	err := s.processor.Submit(mockTask)
	s.NoError(err)
}

func (s *queueTaskProcessorSuite) TestTrySubmit_AssignPriorityFailed() {
	mockTask := NewMockTask(s.controller)

	errAssignPriority := errors.New("some randome error")
	s.mockPriorityAssigner.EXPECT().Assign(NewMockTaskMatcher(mockTask)).Return(errAssignPriority).Times(1)

	submitted, err := s.processor.TrySubmit(mockTask)
	s.Equal(errAssignPriority, err)
	s.False(submitted)
}

func (s *queueTaskProcessorSuite) TestTrySubmit_Fail() {
	mockTask := NewMockTask(s.controller)
	s.mockPriorityAssigner.EXPECT().Assign(NewMockTaskMatcher(mockTask)).Return(nil).Times(1)

	errTrySubmit := errors.New("some randome error")
	mockScheduler := task.NewMockScheduler(s.controller)
	mockScheduler.EXPECT().TrySubmit(NewMockTaskMatcher(mockTask)).Return(false, errTrySubmit).Times(1)

	s.processor.scheduler = mockScheduler

	submitted, err := s.processor.TrySubmit(mockTask)
	s.Equal(errTrySubmit, err)
	s.False(submitted)
}

func (s *queueTaskProcessorSuite) TestTrySubmit_Fail_NewScheduler() {
	mockTask := NewMockTask(s.controller)
	s.mockPriorityAssigner.EXPECT().Assign(NewMockTaskMatcher(mockTask)).Return(nil).Times(1)

	errTrySubmit := errors.New("some randome error")
	mockScheduler := task.NewMockScheduler(s.controller)
	mockScheduler.EXPECT().TrySubmit(NewMockTaskMatcher(mockTask)).Return(false, errTrySubmit).Times(1)

	s.processor.newScheduler = mockScheduler
	s.processor.newSchedulerProbabilityFn = func(...dynamicconfig.FilterOption) int { return 100 }

	submitted, err := s.processor.TrySubmit(mockTask)
	s.Equal(errTrySubmit, err)
	s.False(submitted)
}
func (s *queueTaskProcessorSuite) TestNewSchedulerOptions_UnknownSchedulerType() {
	options, err := task.NewSchedulerOptions[int](0, 100, dynamicconfig.GetIntPropertyFn(10), 1, func(task.PriorityTask) int { return 1 }, func(int) int { return 1 })
	s.Error(err)
	s.Nil(options)
}

func (s *queueTaskProcessorSuite) newTestQueueTaskProcessor() *processorImpl {
	config := config.NewForTest()
	processor, err := NewProcessor(
		s.mockPriorityAssigner,
		config,
		s.logger,
		s.metricsClient,
		s.timeSource,
		s.mockDomainCache,
	)
	s.NoError(err)
	return processor.(*processorImpl)
}

func TestGetDomainPriorityWeight(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*cache.MockDomainCache, dynamicconfig.Client)
		expected  int
	}{
		{
			name: "success",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, client dynamicconfig.Client) {
				mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain-name", nil).Times(1)
				client.UpdateValue(dynamicconfig.TaskSchedulerDomainRoundRobinWeights, map[string]interface{}{"1": 10})
			},
			expected: 10,
		},
		{
			name: "domain cache error, use default",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, client dynamicconfig.Client) {
				mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("", errors.New("test-error")).Times(1)
				client.UpdateValue(dynamicconfig.TaskSchedulerDomainRoundRobinWeights, map[string]interface{}{"1": 10})
			},
			expected: 500,
		},
		{
			name: "invalid map value, use default",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, client dynamicconfig.Client) {
				mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain-name", nil).Times(1)
				client.UpdateValue(dynamicconfig.TaskSchedulerDomainRoundRobinWeights, map[string]interface{}{"1": "invalid"})
			},
			expected: 500,
		},
		{
			name: "unspecified priority",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, client dynamicconfig.Client) {
				mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain-name", nil).Times(1)
				client.UpdateValue(dynamicconfig.TaskSchedulerDomainRoundRobinWeights, map[string]interface{}{"2": 10})
			},
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			client := dynamicconfig.NewInMemoryClient()
			config := config.New(
				dynamicconfig.NewCollection(
					client,
					testlogger.New(t),
				),
				1024,
				1024,
				false,
				"hostname",
			)
			tc.mockSetup(mockDomainCache, client)

			weight := getDomainPriorityWeight(testlogger.New(t), config, mockDomainCache, DomainPriorityKey{DomainID: "test-domain-id", Priority: 1})
			require.Equal(t, tc.expected, weight)
		})
	}
}
