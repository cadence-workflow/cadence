package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	ctask "github.com/uber/cadence/common/task"
)

func newNoopTransferTaskFromDomainID(domainID string) *noopTask {
	return &noopTask{
		Task: &persistence.DecisionTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: domainID,
			},
		},
		state: ctask.TaskStatePending,
	}
}

func newNoopTimerTaskFromDomainID(domainID string, scheduledTime time.Time) *noopTask {
	return &noopTask{
		Task: &persistence.UserTimerTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID: domainID,
			},
			TaskData: persistence.TaskData{
				VisibilityTimestamp: scheduledTime,
			},
		},
		state: ctask.TaskStatePending,
	}
}

func TestReschedulerLifeCycle(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockScheduler := NewMockProcessor(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()
	logger := testlogger.New(t)
	scope := metrics.NoopScope
	now := mockTimeSource.Now()

	rescheduler := NewRescheduler(mockScheduler, mockTimeSource, logger, scope)

	rescheduler.Start()

	for i := 0; i < 10; i++ {
		var task Task
		if i%2 == 0 {
			task = newNoopTransferTaskFromDomainID("domainID")
		} else {
			task = newNoopTimerTaskFromDomainID("domainID", now)
		}
		rescheduler.RescheduleTask(task, now.Add(time.Second*time.Duration(i+1)))
		assert.Equal(t, rescheduler.Size(), i+1)
	}

	taskCh := make(chan Task, 10)
	mockScheduler.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(task Task) (bool, error) {
		taskCh <- task
		return true, nil
	}).Times(1)
	mockTimeSource.Advance(time.Second)
	<-taskCh
	mockScheduler.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(task Task) (bool, error) {
		taskCh <- task
		return true, nil
	}).Times(4)
	mockTimeSource.Advance(time.Second * 4)
	for i := 0; i < 4; i++ {
		<-taskCh
	}

	assert.Equal(t, 5, rescheduler.Size())
	rescheduler.Stop()
	assert.Equal(t, 0, rescheduler.Size())
}

func TestReschedulerRescheduleDomains(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockScheduler := NewMockProcessor(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()
	logger := testlogger.New(t)
	scope := metrics.NoopScope
	now := mockTimeSource.Now()

	rescheduler := NewRescheduler(mockScheduler, mockTimeSource, logger, scope)

	rescheduler.Start()

	for i := 0; i < 10; i++ {
		var task Task
		task = newNoopTransferTaskFromDomainID("X")
		rescheduler.RescheduleTask(task, now.Add(time.Second*time.Duration(i+1)))
		assert.Equal(t, rescheduler.Size(), i+1)
	}

	for i := 0; i < 10; i++ {
		var task Task
		task = newNoopTransferTaskFromDomainID("Y")
		rescheduler.RescheduleTask(task, now.Add(time.Second*100))
		assert.Equal(t, rescheduler.Size(), i+11)
	}

	for i := 0; i < 10; i++ {
		var scheduledTime time.Time
		if i%2 == 0 {
			scheduledTime = now.Add(time.Second * 1000)
		} else {
			scheduledTime = now
		}
		task := newNoopTimerTaskFromDomainID("Z", scheduledTime)
		rescheduler.RescheduleTask(task, now.Add(time.Second*1000))
		assert.Equal(t, rescheduler.Size(), i+21)
	}

	taskCh := make(chan Task, 30)
	mockScheduler.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(task Task) (bool, error) {
		taskCh <- task
		return true, nil
	}).Times(10)
	// advance time to reschedule tasks from domain X
	mockTimeSource.Advance(time.Second * 10)
	for i := 0; i < 10; i++ {
		task := <-taskCh
		assert.Equal(t, "X", task.GetDomainID())
	}
	assert.Equal(t, 20, rescheduler.Size())

	mockScheduler.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(task Task) (bool, error) {
		taskCh <- task
		return true, nil
	}).Times(10)
	// reschedule tasks from domain Y immediately even though their reschedule time is 100s in the future
	rescheduler.RescheduleDomains(map[string]struct{}{"Y": {}})
	for i := 0; i < 10; i++ {
		task := <-taskCh
		assert.Equal(t, "Y", task.GetDomainID())
	}
	assert.Equal(t, 10, rescheduler.Size())

	mockScheduler.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(task Task) (bool, error) {
		taskCh <- task
		return true, nil
	}).Times(5)
	// reschedule tasks from domain Z immediately and verify that tasks with scheduled time in the future are not rescheduled
	rescheduler.RescheduleDomains(map[string]struct{}{"Z": {}})
	for i := 0; i < 5; i++ {
		task := <-taskCh
		assert.Equal(t, "Z", task.GetDomainID())
		assert.False(t, now.After(task.GetTaskKey().GetScheduledTime()))
	}
	assert.Equal(t, 5, rescheduler.Size())
	rescheduler.Stop()
	assert.Equal(t, 0, rescheduler.Size())
}

func TestReschedulerIgnoreTaskWithStateNotPending(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockScheduler := NewMockProcessor(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()
	logger := testlogger.New(t)
	scope := metrics.NoopScope
	now := mockTimeSource.Now()
	rescheduler := NewRescheduler(mockScheduler, mockTimeSource, logger, scope)

	rescheduler.Start()

	for i := 0; i < 10; i++ {
		var task *noopTask
		if i%2 == 0 {
			task = newNoopTransferTaskFromDomainID("domainID")
		} else {
			task = newNoopTransferTaskFromDomainID("domainID")
			task.state = ctask.TaskStateCanceled
		}
		rescheduler.RescheduleTask(task, now.Add(time.Second))
		assert.Equal(t, rescheduler.Size(), i+1)
	}

	taskCh := make(chan Task, 5)
	mockScheduler.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(task Task) (bool, error) {
		taskCh <- task
		return true, nil
	}).Times(5)
	mockTimeSource.Advance(time.Second)
	for i := 0; i < 5; i++ {
		task := <-taskCh
		assert.Equal(t, ctask.TaskStatePending, task.State())
	}
	assert.Equal(t, 0, rescheduler.Size())
	rescheduler.Stop()
}

func TestReschedulerProcessorThrottled(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockScheduler := NewMockProcessor(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()
	logger := testlogger.New(t)
	scope := metrics.NoopScope
	now := mockTimeSource.Now()

	rescheduler := NewRescheduler(mockScheduler, mockTimeSource, logger, scope)

	rescheduler.Start()

	for i := 0; i < 10; i++ {
		task := newNoopTransferTaskFromDomainID("domainID")
		rescheduler.RescheduleTask(task, now.Add(time.Second))
		assert.Equal(t, rescheduler.Size(), i+1)
	}

	taskCh := make(chan Task, 10)
	mockScheduler.EXPECT().TrySubmit(gomock.Any()).DoAndReturn(func(task Task) (bool, error) {
		taskCh <- task
		return true, nil
	}).Times(4)
	mockScheduler.EXPECT().TrySubmit(gomock.Any()).Return(false, nil).Times(1)
	mockTimeSource.Advance(time.Second)
	for i := 0; i < 4; i++ {
		<-taskCh
	}
	assert.Equal(t, 6, rescheduler.Size())
	rescheduler.Stop()
	assert.Equal(t, 0, rescheduler.Size())
}
