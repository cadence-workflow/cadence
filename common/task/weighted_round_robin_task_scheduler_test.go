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
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

type (
	weightedRoundRobinTaskSchedulerSuite struct {
		*require.Assertions
		suite.Suite

		controller    *gomock.Controller
		mockProcessor *MockProcessor
		realProcessor Processor

		queueSize int

		scheduler *weightedRoundRobinTaskSchedulerImpl[int]
	}

	mockPriorityTaskMatcher struct {
		task *MockPriorityTask
	}
)

var (
	testSchedulerWeights = dynamicconfig.GetMapPropertyFn(
		map[string]interface{}{
			"0": 3,
			"1": 2,
			"2": 1,
		},
	)
)

func TestWeightedRoundRobinTaskSchedulerSuite(t *testing.T) {
	s := new(weightedRoundRobinTaskSchedulerSuite)
	suite.Run(t, s)
}

func (s *weightedRoundRobinTaskSchedulerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockProcessor = NewMockProcessor(s.controller)

	s.queueSize = 1000
	s.realProcessor = NewParallelTaskProcessor(
		testlogger.New(s.Suite.T()),
		metrics.NewClient(tally.NoopScope, metrics.Common),
		&ParallelTaskProcessorOptions{
			QueueSize:   1,
			WorkerCount: dynamicconfig.GetIntPropertyFn(1),
			RetryPolicy: backoff.NewExponentialRetryPolicy(time.Millisecond),
		},
	)
	s.scheduler = s.newTestWeightedRoundRobinTaskScheduler(
		&WeightedRoundRobinTaskSchedulerOptions[int]{
			QueueSize:       s.queueSize,
			DispatcherCount: 3,
			TaskToChannelKeyFn: func(task PriorityTask) int {
				return task.Priority()
			},
			ChannelKeyToWeightFn: func(key int) int {
				return testSchedulerWeights()[fmt.Sprintf("%v", key)].(int)
			},
		},
	)
}

func (s *weightedRoundRobinTaskSchedulerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *weightedRoundRobinTaskSchedulerSuite) TestSubmit_Success() {
	taskPriority := 1
	mockTask := NewMockPriorityTask(s.controller)
	mockTask.EXPECT().Priority().Return(taskPriority)

	err := s.scheduler.Submit(mockTask)
	s.NoError(err)

	weight := s.scheduler.options.ChannelKeyToWeightFn(taskPriority)
	taskCh, releaseFn := s.scheduler.pool.GetOrCreateChannel(taskPriority, weight)
	defer releaseFn()
	task := <-taskCh
	s.Equal(mockTask, task)
	taskChs := s.scheduler.pool.GetAllChannels()
	for _, taskCh := range taskChs {
		s.Empty(taskCh)
	}
}

func (s *weightedRoundRobinTaskSchedulerSuite) TestSubmit_Fail_SchedulerShutDown() {
	// create a new scheduler here with queue size 0, otherwise test is non-deterministic
	scheduler := s.newTestWeightedRoundRobinTaskScheduler(
		&WeightedRoundRobinTaskSchedulerOptions[int]{
			QueueSize:       0,
			DispatcherCount: 3,
		},
	)

	mockTask := NewMockPriorityTask(s.controller)
	scheduler.Start()
	scheduler.Stop()
	err := scheduler.Submit(mockTask)
	s.Equal(ErrTaskSchedulerClosed, err)
}

func (s *weightedRoundRobinTaskSchedulerSuite) TestTrySubmit() {
	taskPriority := 1
	for i := 0; i != s.queueSize; i++ {
		mockTask := NewMockPriorityTask(s.controller)
		mockTask.EXPECT().Priority().Return(taskPriority)
		submitted, err := s.scheduler.TrySubmit(mockTask)
		s.NoError(err)
		s.True(submitted)
	}

	// now the queue is full, submit one more task, should be non-blocking
	mockTask := NewMockPriorityTask(s.controller)
	mockTask.EXPECT().Priority().Return(taskPriority)
	submitted, err := s.scheduler.TrySubmit(mockTask)
	s.NoError(err)
	s.False(submitted)
}

func (s *weightedRoundRobinTaskSchedulerSuite) TestDispatcher_SubmitWithNoError() {
	weights, err := common.ConvertDynamicConfigMapPropertyToIntMap(testSchedulerWeights())
	s.NoError(err)

	numPriorities := len(weights)
	tasks := [][]*MockPriorityTask{}
	var taskWG sync.WaitGroup
	for i := 0; i != numPriorities; i++ {
		tasks = append(tasks, []*MockPriorityTask{})
	}

	taskPerPriority := 5
	numSubmittedTask := 0
	tasksPerRound := []int{6, 5, 2, 1, 1}
	round := 0
	mockFn := func(_ Task) error {
		numSubmittedTask++
		if numSubmittedTask == tasksPerRound[round] {
			round++
			numSubmittedTask = 0

			for priority, weight := range weights {
				expectedRemainingTasksNum := taskPerPriority - round*weight
				if expectedRemainingTasksNum < 0 {
					expectedRemainingTasksNum = 0
				}
				taskCh, releaseFn := s.scheduler.pool.GetOrCreateChannel(priority, weight)
				s.Equal(expectedRemainingTasksNum, len(taskCh))
				releaseFn()
			}
		}

		taskWG.Done()
		return nil
	}

	for priority := range weights {
		for i := 0; i != taskPerPriority; i++ {
			mockTask := NewMockPriorityTask(s.controller)
			mockTask.EXPECT().Priority().Return(priority).AnyTimes()
			s.scheduler.Submit(mockTask)
			tasks[priority] = append(tasks[priority], mockTask)
			taskWG.Add(1)
			s.mockProcessor.EXPECT().Submit(newMockPriorityTaskMatcher(mockTask)).DoAndReturn(mockFn)
		}
	}

	s.scheduler.processor = s.mockProcessor

	doneCh := make(chan struct{})
	go func() {
		s.scheduler.dispatcherWG.Add(1)
		s.scheduler.dispatcher()
		close(doneCh)
	}()

	taskWG.Wait()
	close(s.scheduler.shutdownCh)

	<-doneCh
}

func (s *weightedRoundRobinTaskSchedulerSuite) TestDispatcher_FailToSubmit() {
	mockTask := NewMockPriorityTask(s.controller)
	mockTask.EXPECT().Priority().Return(0)
	mockTask.EXPECT().Nack()

	var taskWG sync.WaitGroup
	s.scheduler.Submit(mockTask)
	taskWG.Add(1)

	mockFn := func(_ Task) error {
		taskWG.Done()
		return errors.New("some random error")
	}
	s.mockProcessor.EXPECT().Submit(newMockPriorityTaskMatcher(mockTask)).DoAndReturn(mockFn)
	s.scheduler.processor = s.mockProcessor

	doneCh := make(chan struct{})
	go func() {
		s.scheduler.dispatcherWG.Add(1)
		s.scheduler.dispatcher()
		close(doneCh)
	}()

	taskWG.Wait()
	close(s.scheduler.shutdownCh)

	<-doneCh
}

func (s *weightedRoundRobinTaskSchedulerSuite) TestWRR() {
	numTasks := 1000
	var taskWG sync.WaitGroup

	tasks := []PriorityTask{}
	mockFn := func(_ Task) error {
		taskWG.Done()
		return nil
	}
	for i := 0; i != numTasks; i++ {
		mockTask := NewMockPriorityTask(s.controller)
		mockTask.EXPECT().Priority().Return(rand.Intn(len(testSchedulerWeights()))).Times(1)
		tasks = append(tasks, mockTask)
		taskWG.Add(1)
		s.mockProcessor.EXPECT().Submit(newMockPriorityTaskMatcher(mockTask)).DoAndReturn(mockFn)
	}

	s.scheduler.processor = s.mockProcessor
	s.scheduler.Start()
	for _, task := range tasks {
		if rand.Intn(2) == 0 {
			s.NoError(s.scheduler.Submit(task))
		} else {
			submitted, err := s.scheduler.TrySubmit(task)
			s.NoError(err)
			s.True(submitted)
		}
	}
	taskWG.Wait()
	s.scheduler.Stop()
}

func (s *weightedRoundRobinTaskSchedulerSuite) TestSchedulerContract() {
	testSchedulerContract(s.Assertions, s.controller, s.scheduler, s.realProcessor)
}

func (s *weightedRoundRobinTaskSchedulerSuite) newTestWeightedRoundRobinTaskScheduler(
	options *WeightedRoundRobinTaskSchedulerOptions[int],
) *weightedRoundRobinTaskSchedulerImpl[int] {
	scheduler, err := NewWeightedRoundRobinTaskScheduler(
		testlogger.New(s.Suite.T()),
		metrics.NewClient(tally.NoopScope, metrics.Common),
		clock.NewMockedTimeSource(),
		s.realProcessor,
		options,
	)
	s.NoError(err)
	return scheduler.(*weightedRoundRobinTaskSchedulerImpl[int])
}

func testSchedulerContract(
	s *require.Assertions,
	controller *gomock.Controller,
	scheduler Scheduler,
	processor Processor,
) {
	numTasks := 10000
	var taskWG sync.WaitGroup

	tasks := []PriorityTask{}
	taskStatusLock := &sync.Mutex{}
	taskStatus := make(map[PriorityTask]State)
	for i := 0; i != numTasks; i++ {
		mockTask := NewMockPriorityTask(controller)
		taskStatus[mockTask] = TaskStatePending
		mockTask.EXPECT().Priority().Return(rand.Intn(len(testSchedulerWeights()))).MaxTimes(1)
		mockTask.EXPECT().Execute().Return(nil).MaxTimes(1)
		mockTask.EXPECT().Ack().Do(func() {
			taskStatusLock.Lock()
			defer taskStatusLock.Unlock()

			s.Equal(TaskStatePending, taskStatus[mockTask])
			taskStatus[mockTask] = TaskStateAcked
			taskWG.Done()
		}).MaxTimes(1)
		mockTask.EXPECT().Nack().Do(func() {
			taskStatusLock.Lock()
			defer taskStatusLock.Unlock()

			s.Equal(TaskStatePending, taskStatus[mockTask])
			taskStatus[mockTask] = TaskStateNacked
			taskWG.Done()
		}).MaxTimes(1)
		tasks = append(tasks, mockTask)
		taskWG.Add(1)
	}

	if processor != nil {
		processor.Start()
	}
	scheduler.Start()
	go func() {
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
		scheduler.Stop()
		if processor != nil {
			processor.Stop()
		}
	}()

	for _, task := range tasks {
		if rand.Intn(2) == 0 {
			if err := scheduler.Submit(task); err != nil {
				taskWG.Done()
				taskStatusLock.Lock()
				delete(taskStatus, task)
				taskStatusLock.Unlock()
			}
		} else {
			if submitted, _ := scheduler.TrySubmit(task); !submitted {
				taskWG.Done()
				taskStatusLock.Lock()
				delete(taskStatus, task)
				taskStatusLock.Unlock()
			}
		}
	}
	s.True(common.AwaitWaitGroup(&taskWG, 10*time.Second))
	switch schedulerImpl := scheduler.(type) {
	case *fifoTaskSchedulerImpl:
		<-schedulerImpl.shutdownCh
	case *weightedRoundRobinTaskSchedulerImpl[int]:
		<-schedulerImpl.shutdownCh
	default:
		s.Fail("unknown task scheduler type")
	}

	for _, status := range taskStatus {
		s.NotEqual(TaskStatePending, status)
	}
}

func newMockPriorityTaskMatcher(mockTask *MockPriorityTask) gomock.Matcher {
	return &mockPriorityTaskMatcher{
		task: mockTask,
	}
}

func (m *mockPriorityTaskMatcher) Matches(x interface{}) bool {
	taskPtr, ok := x.(*MockPriorityTask)
	if !ok {
		return false
	}
	return taskPtr == m.task
}

func (m *mockPriorityTaskMatcher) String() string {
	return fmt.Sprintf("is equal to %v", m.task)
}
