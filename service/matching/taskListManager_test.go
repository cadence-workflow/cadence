// Copyright (c) 2017 Uber Technologies, Inc.
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

package matching

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/mocks"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uber-common/bark"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service/dynamicconfig"
)

const _minBurst = 10000

func TestDeliverBufferTasks(t *testing.T) {
	tests := []func(tlm *taskListManagerImpl){
		func(tlm *taskListManagerImpl) { close(tlm.taskBuffer) },
		func(tlm *taskListManagerImpl) { close(tlm.deliverBufferShutdownCh) },
		func(tlm *taskListManagerImpl) { tlm.cancelFunc() },
	}
	for _, test := range tests {
		tlm := createTestTaskListManager()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tlm.deliverBufferTasksForPoll()
		}()
		test(tlm)
		// deliverBufferTasksForPoll should stop after invocation of the test function
		wg.Wait()
	}
}

func TestDeliverBufferTasks_NoPollers(t *testing.T) {
	tlm := createTestTaskListManager()
	tlm.taskBuffer <- &persistence.TaskInfo{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		tlm.deliverBufferTasksForPoll()
		wg.Done()
	}()
	time.Sleep(100 * time.Millisecond) // let go routine run first and block on tasksForPoll
	close(tlm.deliverBufferShutdownCh)
	wg.Wait()
}

func TestNewRateLimiter(t *testing.T) {
	maxDispatch := float64(0.01)
	rl := newRateLimiter(&maxDispatch, time.Second, _minBurst)
	limiter := rl.globalLimiter.Load().(*rate.Limiter)
	assert.Equal(t, _minBurst, limiter.Burst())
}

func createTestTaskListManager() *taskListManagerImpl {
	return createTestTaskListManagerWithConfig(defaultTestConfig())
}

func createTestTaskListManagerWithConfig(cfg *Config) *taskListManagerImpl {
	logger := bark.NewLoggerFromLogrus(log.New())
	tm := newTestTaskManager(logger)
	mockDomainCache := &cache.DomainCacheMock{}
	mockDomainCache.On("GetDomainByID", mock.Anything).Return(cache.CreateDomainCacheEntry("domainName"), nil)
	me := newMatchingEngine(
		cfg, tm, &mocks.HistoryClient{}, logger, mockDomainCache,
	)
	tl := "tl"
	dID := "domain"
	tlID := &taskListID{domainID: dID, taskListName: tl, taskType: persistence.TaskListTypeActivity}
	tlKind := common.TaskListKindPtr(workflow.TaskListKindNormal)
	tlMgr, err := newTaskListManager(me, tlID, tlKind, cfg)
	if err != nil {
		logger.Fatalf("error when createTestTaskListManager: %v", err)
	}
	return tlMgr.(*taskListManagerImpl)
}

func TestIsTaskAddedRecently(t *testing.T) {
	tlm := createTestTaskListManager()
	require.True(t, tlm.isTaskAddedRecently(time.Now()))
	require.False(t, tlm.isTaskAddedRecently(time.Now().Add(-tlm.config.MaxTasklistIdleTime())))
	require.True(t, tlm.isTaskAddedRecently(time.Now().Add(1*time.Second)))
	require.False(t, tlm.isTaskAddedRecently(time.Time{}))
}

func TestDescribeTaskList(t *testing.T) {
	startTaskID := int64(1)
	taskCount := int64(3)
	PollerIdentity := "test-poll"

	// Create taskList Manager and set taskList state
	tlm := createTestTaskListManager()
	tlm.db.rangeID = int64(1)
	tlm.db.ackLevel = int64(0)
	tlm.taskAckManager.setAckLevel(tlm.db.ackLevel)

	for i := int64(0); i < taskCount; i++ {
		tlm.taskAckManager.addTask(startTaskID + i)
	}

	includeTaskStatus := false
	descResp := tlm.DescribeTaskList(includeTaskStatus)
	require.Equal(t, 0, len(descResp.GetPollers()))
	require.Nil(t, descResp.GetTaskListStatus())

	includeTaskStatus = true
	taskListStatus := tlm.DescribeTaskList(includeTaskStatus).GetTaskListStatus()
	require.NotNil(t, taskListStatus)
	require.Zero(t, taskListStatus.GetAckLevel())
	require.Equal(t, taskCount, taskListStatus.GetReadLevel())
	require.Equal(t, taskCount, taskListStatus.GetBacklogCountHint())
	require.True(t, taskListStatus.GetRatePerSecond() > (_defaultTaskDispatchRPS-1))
	require.True(t, taskListStatus.GetRatePerSecond() < (_defaultTaskDispatchRPS+1))
	taskIDBlock := taskListStatus.GetTaskIDBlock()
	require.Equal(t, int64(1), taskIDBlock.GetStartID())
	require.Equal(t, tlm.config.RangeSize, taskIDBlock.GetEndID())

	// Add a poller and complete all tasks
	tlm.pollerHistory.updatePollerInfo(pollerIdentity(PollerIdentity), nil)
	for i := int64(0); i < taskCount; i++ {
		tlm.taskAckManager.completeTask(startTaskID + i)
	}

	descResp = tlm.DescribeTaskList(includeTaskStatus)
	require.Equal(t, 1, len(descResp.GetPollers()))
	require.Equal(t, PollerIdentity, descResp.Pollers[0].GetIdentity())
	require.NotEmpty(t, descResp.Pollers[0].GetLastAccessTime())
	require.True(t, descResp.Pollers[0].GetRatePerSecond() > (_defaultTaskDispatchRPS-1))

	rps := 5.0
	tlm.pollerHistory.updatePollerInfo(pollerIdentity(PollerIdentity), &rps)
	descResp = tlm.DescribeTaskList(includeTaskStatus)
	require.Equal(t, 1, len(descResp.GetPollers()))
	require.Equal(t, PollerIdentity, descResp.Pollers[0].GetIdentity())
	require.True(t, descResp.Pollers[0].GetRatePerSecond() > 4.0 && descResp.Pollers[0].GetRatePerSecond() < 6.0)

	taskListStatus = descResp.GetTaskListStatus()
	require.NotNil(t, taskListStatus)
	require.Equal(t, taskCount, taskListStatus.GetAckLevel())
	require.Zero(t, taskListStatus.GetBacklogCountHint())
}

func tlMgrStartWithoutNotifyEvent(tlm *taskListManagerImpl) {
	// mimic tlm.Start() but avoid calling notifyEvent
	tlm.startWG.Done()
	go tlm.getTasksPump()
}

func TestCheckIdleTaskList(t *testing.T) {
	cfg := NewConfig(dynamicconfig.NewNopCollection())
	cfg.IdleTasklistCheckInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)

	// Idle
	tlm := createTestTaskListManagerWithConfig(cfg)
	tlMgrStartWithoutNotifyEvent(tlm)
	time.Sleep(20 * time.Millisecond)
	require.False(t, atomic.CompareAndSwapInt32(&tlm.stopped, 0, 1))

	// Active poll-er
	tlm = createTestTaskListManagerWithConfig(cfg)
	tlm.pollerHistory.updatePollerInfo(pollerIdentity("test-poll"), nil)
	require.Equal(t, 1, len(tlm.GetAllPollerInfo()))
	tlMgrStartWithoutNotifyEvent(tlm)
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int32(0), tlm.stopped)
	tlm.Stop()
	require.Equal(t, int32(1), tlm.stopped)

	// Active adding task
	tlm = createTestTaskListManagerWithConfig(cfg)
	require.Equal(t, 0, len(tlm.GetAllPollerInfo()))
	tlMgrStartWithoutNotifyEvent(tlm)
	tlm.signalNewTask()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int32(0), tlm.stopped)
	tlm.Stop()
	require.Equal(t, int32(1), tlm.stopped)
}
