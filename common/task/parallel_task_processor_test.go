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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

type (
	parallelTaskProcessorSuite struct {
		*require.Assertions
		suite.Suite

		controller *gomock.Controller

		processor *parallelTaskProcessorImpl
	}
)

var (
	errRetryable    = errors.New("retryable error")
	errNonRetryable = errors.New("non-retryable error")
)

func TestParallelTaskProcessorSuite(t *testing.T) {
	s := new(parallelTaskProcessorSuite)
	suite.Run(t, s)
}

func (s *parallelTaskProcessorSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())

	s.processor = NewParallelTaskProcessor(
		testlogger.New(s.Suite.T()),
		metrics.NewClient(tally.NoopScope, metrics.Common),
		&ParallelTaskProcessorOptions{
			QueueSize:   0,
			WorkerCount: dynamicproperties.GetIntPropertyFn(1),
			RetryPolicy: backoff.NewExponentialRetryPolicy(time.Millisecond),
		},
	).(*parallelTaskProcessorImpl)
}

func (s *parallelTaskProcessorSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *parallelTaskProcessorSuite) TestSubmit_Success() {
	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().Execute().Return(nil).MaxTimes(1)
	mockTask.EXPECT().Ack().MaxTimes(1)
	s.processor.Start()
	err := s.processor.Submit(mockTask)
	s.NoError(err)
	s.processor.Stop()
}

func (s *parallelTaskProcessorSuite) TestSubmit_Fail() {
	mockTask := NewMockTask(s.controller)
	s.processor.Start()
	s.processor.Stop()
	err := s.processor.Submit(mockTask)
	s.Equal(ErrTaskProcessorClosed, err)
}

func (s *parallelTaskProcessorSuite) TestTaskWorker() {
	numTasks := 5

	done := make(chan struct{})

	s.processor.shutdownWG.Add(1)
	workerShutdownCh := make(chan struct{})
	s.processor.workerShutdownCh = append(s.processor.workerShutdownCh, workerShutdownCh)

	go func() {
		for i := 0; i != numTasks; i++ {
			mockTask := NewMockTask(s.controller)
			mockTask.EXPECT().Execute().Return(nil).Times(1)
			mockTask.EXPECT().Ack().Times(1)
			err := s.processor.Submit(mockTask)
			s.NoError(err)
		}
		close(workerShutdownCh)
		close(done)
	}()

	s.processor.taskWorker(workerShutdownCh)
	<-done
}

func (s *parallelTaskProcessorSuite) TestExecuteTask_RetryableError() {
	mockTask := NewMockTask(s.controller)
	gomock.InOrder(
		mockTask.EXPECT().Execute().Return(errRetryable),
		mockTask.EXPECT().HandleErr(errRetryable).Return(errRetryable),
		mockTask.EXPECT().RetryErr(errRetryable).Return(true),
		mockTask.EXPECT().Execute().Return(errRetryable),
		mockTask.EXPECT().HandleErr(errRetryable).Return(errRetryable),
		mockTask.EXPECT().RetryErr(errRetryable).Return(true),
		mockTask.EXPECT().Execute().Return(nil),
		mockTask.EXPECT().Ack(),
	)

	s.processor.executeTask(mockTask, make(chan struct{}))
}

func (s *parallelTaskProcessorSuite) TestExecuteTask_NonRetryableError() {
	mockTask := NewMockTask(s.controller)
	gomock.InOrder(
		mockTask.EXPECT().Execute().Return(errNonRetryable),
		mockTask.EXPECT().HandleErr(errNonRetryable).Return(errNonRetryable),
		mockTask.EXPECT().RetryErr(errNonRetryable).Return(false).AnyTimes(),
		mockTask.EXPECT().Nack(gomock.Any()),
	)

	s.processor.executeTask(mockTask, make(chan struct{}))
}

func (s *parallelTaskProcessorSuite) TestExecuteTask_WorkerStopped() {
	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().Execute().Return(errRetryable).AnyTimes()
	mockTask.EXPECT().HandleErr(errRetryable).Return(errRetryable).AnyTimes()
	mockTask.EXPECT().RetryErr(errRetryable).Return(true).AnyTimes()
	mockTask.EXPECT().Nack(gomock.Any()).Times(1)

	done := make(chan struct{})
	workerShutdownCh := make(chan struct{})
	go func() {
		s.processor.executeTask(mockTask, workerShutdownCh)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	close(workerShutdownCh)
	<-done
}

func (s *parallelTaskProcessorSuite) TestAddWorker() {
	newWorkerCount := 10

	s.processor.addWorker(newWorkerCount)
	s.Len(s.processor.workerShutdownCh, newWorkerCount)

	s.processor.addWorker(-1)
	s.Len(s.processor.workerShutdownCh, newWorkerCount)

	for _, shutdownCh := range s.processor.workerShutdownCh {
		close(shutdownCh)
	}

	s.processor.shutdownWG.Wait()
}

func (s *parallelTaskProcessorSuite) TestRemoveWorker() {
	currentWorkerCount := 10
	removeWorkerCount := 4

	s.processor.addWorker(currentWorkerCount)

	s.processor.removeWorker(removeWorkerCount)
	s.Len(s.processor.workerShutdownCh, currentWorkerCount-removeWorkerCount)

	s.processor.removeWorker(-1)
	s.Len(s.processor.workerShutdownCh, currentWorkerCount-removeWorkerCount)

	for _, shutdownCh := range s.processor.workerShutdownCh {
		close(shutdownCh)
	}

	s.processor.shutdownWG.Wait()
}

func (s *parallelTaskProcessorSuite) TestMonitor() {
	workerCount := 5

	s.processor.shutdownWG.Add(1) // for monitor
	dcClient := dynamicconfig.NewInMemoryClient()
	err := dcClient.UpdateValue(dynamicproperties.TaskSchedulerWorkerCount, workerCount)
	s.NoError(err)
	dcCollection := dynamicconfig.NewCollection(dcClient, s.processor.logger)
	s.processor.options.WorkerCount = dcCollection.GetIntProperty(dynamicproperties.TaskSchedulerWorkerCount)

	testMonitorTickerDuration := 100 * time.Millisecond
	go s.processor.workerMonitor(testMonitorTickerDuration)

	time.Sleep(2 * testMonitorTickerDuration)
	// note we can't check the length of the workerShutdownCh directly
	// as that will lead to race condition. Instead we check the current
	// size of shutdownWG chan, which should be workerCount + 1
	for i := 0; i != workerCount+1; i++ {
		s.processor.shutdownWG.Done()
	}
	s.processor.shutdownWG.Wait()
	s.processor.shutdownWG.Add(workerCount + 1)

	newWorkerCount := 3
	err = dcClient.UpdateValue(dynamicproperties.TaskSchedulerWorkerCount, newWorkerCount)
	s.NoError(err)

	time.Sleep(2 * testMonitorTickerDuration)
	for i := 0; i != newWorkerCount+1; i++ {
		s.processor.shutdownWG.Done()
	}
	s.processor.shutdownWG.Wait()
	s.processor.shutdownWG.Add(newWorkerCount + 1)

	close(s.processor.shutdownCh)

	time.Sleep(2 * testMonitorTickerDuration)
	s.processor.shutdownWG.Wait()
}

func (s *parallelTaskProcessorSuite) TestProcessorContract() {
	numTasks := 10000
	var taskWG sync.WaitGroup

	tasks := []Task{}
	taskStatusLock := &sync.Mutex{}
	taskStatus := make(map[Task]State)
	for i := 0; i != numTasks; i++ {
		mockTask := NewMockTask(s.controller)
		taskStatus[mockTask] = TaskStatePending
		mockTask.EXPECT().Execute().Return(nil).MaxTimes(1)
		mockTask.EXPECT().Ack().Do(func() {
			taskStatusLock.Lock()
			defer taskStatusLock.Unlock()

			s.Equal(TaskStatePending, taskStatus[mockTask])
			taskStatus[mockTask] = TaskStateAcked
			taskWG.Done()
		}).MaxTimes(1)
		mockTask.EXPECT().Nack(gomock.Any()).Do(func(err error) {
			taskStatusLock.Lock()
			defer taskStatusLock.Unlock()

			s.Equal(TaskStatePending, taskStatus[mockTask])
			taskStatus[mockTask] = State(-1) // set it to whatever state that is not TaskStatePending
			taskWG.Done()
		}).MaxTimes(1)
		tasks = append(tasks, mockTask)
		taskWG.Add(1)
	}

	processor := NewParallelTaskProcessor(
		testlogger.New(s.Suite.T()),
		metrics.NewClient(tally.NoopScope, metrics.Common),
		&ParallelTaskProcessorOptions{
			QueueSize:   100,
			WorkerCount: dynamicproperties.GetIntPropertyFn(10),
			RetryPolicy: backoff.NewExponentialRetryPolicy(time.Millisecond),
		},
	).(*parallelTaskProcessorImpl)

	processor.Start()
	go func() {
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
		processor.Stop()
	}()

	for _, task := range tasks {
		if err := processor.Submit(task); err != nil {
			taskWG.Done()
			taskStatusLock.Lock()
			delete(taskStatus, task)
			taskStatusLock.Unlock()
		}
	}
	s.True(common.AwaitWaitGroup(&taskWG, 10*time.Second))
	<-processor.shutdownCh

	for _, status := range taskStatus {
		s.NotEqual(TaskStatePending, status)
	}
}

func (s *parallelTaskProcessorSuite) TestExecuteTask_PanicHandling() {
	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().Execute().Do(func() {
		panic("A panic occurred")
	})
	mockTask.EXPECT().HandleErr(gomock.Any()).Return(errRetryable).AnyTimes()
	mockTask.EXPECT().Nack(gomock.Any()).Times(1)
	done := make(chan struct{})
	workerShutdownCh := make(chan struct{})
	go func() {
		s.processor.executeTask(mockTask, workerShutdownCh)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	close(workerShutdownCh)
	<-done
}
