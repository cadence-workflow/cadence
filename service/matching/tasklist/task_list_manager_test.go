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

package tasklist

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/stats"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/poller"
)

type mockDeps struct {
	mockDomainCache    *cache.MockDomainCache
	mockTaskManager    *persistence.MockTaskManager
	mockIsolationState *isolationgroup.MockState
	mockMatchingClient *matching.MockClient
	mockTimeSource     clock.MockedTimeSource
	dynamicClient      dynamicconfig.Client
}

var (
	testIsolationGroups = []string{"datacenterA", "datacenterB"}
)

func setupMocksForTaskListManager(t *testing.T, taskListID *Identifier, taskListKind types.TaskListKind) (*taskListManagerImpl, *mockDeps) {
	ctrl := gomock.NewController(t)
	dynamicClient := dynamicconfig.NewInMemoryClient()
	logger := testlogger.New(t)
	metricsClient := metrics.NewNoopMetricsClient()
	clusterMetadata := cluster.GetTestClusterMetadata(true)
	deps := &mockDeps{
		mockDomainCache:    cache.NewMockDomainCache(ctrl),
		mockTaskManager:    persistence.NewMockTaskManager(ctrl),
		mockIsolationState: isolationgroup.NewMockState(ctrl),
		mockMatchingClient: matching.NewMockClient(ctrl),
		mockTimeSource:     clock.NewMockedTimeSource(),
		dynamicClient:      dynamicClient,
	}
	deps.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("domainName", nil).Times(1)
	config := config.NewConfig(dynamicconfig.NewCollection(dynamicClient, logger), "hostname", getIsolationgroupsHelper)
	mockHistoryService := history.NewMockClient(ctrl)

	tlm, err := NewManager(
		deps.mockDomainCache,
		logger,
		metricsClient,
		deps.mockTaskManager,
		clusterMetadata,
		deps.mockIsolationState,
		deps.mockMatchingClient,
		func(Manager) {},
		taskListID,
		taskListKind,
		config,
		deps.mockTimeSource,
		deps.mockTimeSource.Now(),
		mockHistoryService,
	)
	require.NoError(t, err)
	return tlm.(*taskListManagerImpl), deps
}

func defaultTestConfig() *config.Config {
	config := config.NewConfig(dynamicconfig.NewNopCollection(), "some random hostname", getIsolationgroupsHelper)
	config.LongPollExpirationInterval = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(100 * time.Millisecond)
	config.MaxTaskDeleteBatchSize = dynamicproperties.GetIntPropertyFilteredByTaskListInfo(1)
	config.AllIsolationGroups = getIsolationgroupsHelper
	config.GetTasksBatchSize = dynamicproperties.GetIntPropertyFilteredByTaskListInfo(10)
	config.AsyncTaskDispatchTimeout = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)
	config.LocalTaskWaitTime = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(time.Millisecond)
	return config
}

func TestDeliverBufferTasks(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)

	tests := []func(tlm *taskListManagerImpl){
		func(tlm *taskListManagerImpl) { tlm.taskReader.cancelFunc() },
		func(tlm *taskListManagerImpl) {
			tlm.limiter.ReportLimit(0.1)
			tlm.taskReader.taskBuffers[defaultTaskBufferIsolationGroup] <- &persistence.TaskInfo{}
			err := tlm.matcher.(*taskMatcherImpl).ratelimit(context.Background()) // consume the token
			assert.NoError(t, err)
			tlm.taskReader.cancelFunc()
		},
	}
	for _, test := range tests {
		tlm := createTestTaskListManager(t, logger, controller)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tlm.taskReader.dispatchBufferedTasks(defaultTaskBufferIsolationGroup)
		}()
		test(tlm)
		// dispatchBufferedTasks should stop after invocation of the test function
		wg.Wait()
	}
}

func TestTaskListString(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)
	tlm := createTestTaskListManager(t, logger, controller)
	got := tlm.String()
	want := "Activity task list tl\nRangeID=0\nTaskIDBlock={start:-99999 end:0}\nAckLevel=-1\nMaxReadLevel=-1\n"
	assert.Equal(t, want, got)
}

func TestDeliverBufferTasks_NoPollers(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)

	tlm := createTestTaskListManager(t, logger, controller)
	tlm.taskReader.taskBuffers[defaultTaskBufferIsolationGroup] <- &persistence.TaskInfo{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		tlm.taskReader.dispatchBufferedTasks("")
		wg.Done()
	}()
	time.Sleep(100 * time.Millisecond) // let go routine run first and block on tasksForPoll
	tlm.taskReader.cancelFunc()
	wg.Wait()
}

func TestReadLevelForAllExpiredTasksInBatch(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)

	tlm := createTestTaskListManager(t, logger, controller)
	tlm.db.rangeID = int64(1)
	tlm.taskAckManager.SetAckLevel(0)
	tlm.taskAckManager.SetReadLevel(0)
	require.Equal(t, int64(0), tlm.taskAckManager.GetAckLevel())
	require.Equal(t, int64(0), tlm.taskAckManager.GetReadLevel())

	// Add all expired tasks
	tasks := []*persistence.TaskInfo{
		{
			TaskID:      11,
			Expiry:      time.Now().Add(-time.Minute),
			CreatedTime: time.Now().Add(-time.Hour),
		},
		{
			TaskID:      12,
			Expiry:      time.Now().Add(-time.Minute),
			CreatedTime: time.Now().Add(-time.Hour),
		},
	}

	require.True(t, tlm.taskReader.addTasksToBuffer(tasks))
	require.Equal(t, int64(0), tlm.taskAckManager.GetAckLevel())
	require.Equal(t, int64(12), tlm.taskAckManager.GetReadLevel())

	// Now add a mix of valid and expired tasks
	require.True(t, tlm.taskReader.addTasksToBuffer([]*persistence.TaskInfo{
		{
			TaskID:      13,
			Expiry:      time.Now().Add(-time.Minute),
			CreatedTime: time.Now().Add(-time.Hour),
		},
		{
			TaskID:      14,
			Expiry:      time.Now().Add(time.Hour),
			CreatedTime: time.Now().Add(time.Minute),
		},
	}))
	require.Equal(t, int64(0), tlm.taskAckManager.GetAckLevel())
	require.Equal(t, int64(14), tlm.taskAckManager.GetReadLevel())
}

func createTestTaskListManager(t *testing.T, logger log.Logger, controller *gomock.Controller) *taskListManagerImpl {
	return createTestTaskListManagerWithConfig(t, logger, controller, defaultTestConfig(), clock.NewMockedTimeSource())
}

func createTestTaskListManagerWithConfig(t *testing.T, logger log.Logger, controller *gomock.Controller, cfg *config.Config, timeSource clock.TimeSource) *taskListManagerImpl {
	tm := NewTestTaskManager(t, logger, timeSource)
	mockIsolationState := isolationgroup.NewMockState(controller)
	mockIsolationState.EXPECT().IsDrained(gomock.Any(), "domainName", gomock.Any()).Return(false, nil).AnyTimes()
	mockDomainCache := cache.NewMockDomainCache(controller)
	mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.CreateDomainCacheEntry("domainName"), nil).AnyTimes()
	mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("domainName", nil).AnyTimes()
	mockHistoryService := history.NewMockClient(controller)
	tl := "tl"
	dID := "domain"
	tlID, err := NewIdentifier(dID, tl, persistence.TaskListTypeActivity)
	if err != nil {
		panic(err)
	}
	tlMgr, err := NewManager(
		mockDomainCache,
		logger,
		metrics.NewClient(tally.NoopScope, metrics.Matching),
		tm,
		cluster.GetTestClusterMetadata(true),
		mockIsolationState,
		nil,
		func(Manager) {},
		tlID,
		types.TaskListKindNormal,
		cfg,
		timeSource,
		timeSource.Now(),
		mockHistoryService,
	)
	if err != nil {
		logger.Fatal("error when createTestTaskListManager", tag.Error(err))
	}
	return tlMgr.(*taskListManagerImpl)
}

func TestDescribeTaskList(t *testing.T) {
	// Magic values hardcoded in matching/config.go. Not much of a config :(
	defaultRps := 100000.0
	defaultRangeSize := 100000
	startedID := int64(1)
	firstIDBlock := &types.TaskIDBlock{
		StartID: startedID,
		EndID:   int64(defaultRangeSize),
	}

	cases := []struct {
		name           string
		includeStatus  bool
		pollers        map[string]poller.Info
		allowance      func(ctrl *gomock.Controller, impl *taskListManagerImpl)
		expectedStatus *types.TaskListStatus
		expectedConfig *types.TaskListPartitionConfig
	}{
		{
			name: "no status, pollers, or config",
		},
		{
			name: "no status, with config",
			allowance: func(_ *gomock.Controller, impl *taskListManagerImpl) {
				err := impl.RefreshTaskListPartitionConfig(context.Background(), &types.TaskListPartitionConfig{
					Version:         1,
					ReadPartitions:  partitions(3),
					WritePartitions: partitions(2),
				})
				require.NoError(t, err)
			},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(2),
			},
		},
		{
			name: "no status, with pollers",
			pollers: map[string]poller.Info{
				"pollerID": {
					Identity:       "pollerIdentity",
					RatePerSecond:  1.0,
					IsolationGroup: "a",
				},
			},
		},
		{
			name:          "with status",
			includeStatus: true,
			expectedStatus: &types.TaskListStatus{
				RatePerSecond: defaultRps,
				TaskIDBlock:   firstIDBlock,
				IsolationGroupMetrics: map[string]*types.IsolationGroupMetrics{
					"datacenterA": {},
					"datacenterB": {},
				},
				Empty: true,
			},
		},
		{
			name:          "with status, tasks completed",
			includeStatus: true,
			allowance: func(_ *gomock.Controller, tlm *taskListManagerImpl) {
				for i := startedID; i < 11; i++ {
					err := tlm.taskAckManager.ReadItem(i)
					require.NoError(t, err)
				}
				for i := startedID; i < 5; i++ {
					tlm.taskAckManager.AckItem(i)
				}
			},
			expectedStatus: &types.TaskListStatus{
				BacklogCountHint: 6,
				ReadLevel:        10,
				AckLevel:         4,
				RatePerSecond:    defaultRps,
				TaskIDBlock:      firstIDBlock,
				IsolationGroupMetrics: map[string]*types.IsolationGroupMetrics{
					"datacenterA": {},
					"datacenterB": {},
				},
				Empty: false,
			},
		},
		{
			name:          "with status, pollers and metrics",
			includeStatus: true,
			pollers: map[string]poller.Info{
				"a-1": {
					Identity:       "a1Identity",
					RatePerSecond:  1.0,
					IsolationGroup: "datacenterA",
				},
			},
			allowance: func(ctrl *gomock.Controller, impl *taskListManagerImpl) {
				mockQPS := stats.NewMockQPSTrackerGroup(ctrl)
				mockQPS.EXPECT().GroupQPS("datacenterA").Return(float64(75.0))
				mockQPS.EXPECT().GroupQPS("datacenterB").Return(float64(25.0))
				mockQPS.EXPECT().QPS().Return(float64(100.0))
				impl.qpsTracker = mockQPS
				impl.limiter.ReportLimit(1.0)
			},
			expectedStatus: &types.TaskListStatus{
				RatePerSecond:     1.0, // From poller
				TaskIDBlock:       firstIDBlock,
				NewTasksPerSecond: 100,
				IsolationGroupMetrics: map[string]*types.IsolationGroupMetrics{
					"datacenterA": {
						PollerCount:       1,
						NewTasksPerSecond: 75.0,
					},
					"datacenterB": {
						PollerCount:       0,
						NewTasksPerSecond: 25.0,
					},
				},
				Empty: true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectedTl := &types.TaskList{Name: "tl", Kind: types.TaskListKindNormal.Ptr()}
			controller := gomock.NewController(t)
			logger := testlogger.New(t)
			tlm := createTestTaskListManager(t, logger, controller)
			tlm.db.rangeID = int64(1)
			tlm.taskAckManager.SetAckLevel(0)
			tlm.startWG.Done()

			expectedPollers := make([]*types.PollerInfo, 0, len(tc.pollers))
			for id, info := range tc.pollers {
				tlm.pollers.StartPoll(id, func() {}, &info)
				expectedPollers = append(expectedPollers, &types.PollerInfo{
					LastAccessTime: common.Int64Ptr(tlm.timeSource.Now().UnixNano()),
					Identity:       info.Identity,
					RatePerSecond:  info.RatePerSecond,
				})
			}
			if tc.allowance != nil {
				tc.allowance(controller, tlm)
			}
			result := tlm.DescribeTaskList(tc.includeStatus)
			assert.Equal(t, expectedTl, result.TaskList)
			assert.Equal(t, tc.expectedStatus, result.TaskListStatus)
			assert.Equal(t, tc.expectedConfig, result.PartitionConfig)
			assert.ElementsMatch(t, expectedPollers, result.Pollers)
		})
	}
}

func TestCheckIdleTaskList(t *testing.T) {
	defer goleak.VerifyNone(t)
	cfg := config.NewConfig(dynamicconfig.NewNopCollection(), "some random hostname", getIsolationgroupsHelper)
	cfg.IdleTasklistCheckInterval = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)

	t.Run("Idle task-list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tlm := createTestTaskListManagerWithConfig(t, testlogger.New(t), ctrl, cfg, clock.NewRealTimeSource())
		require.NoError(t, tlm.Start())

		require.EqualValues(t, 0, atomic.LoadInt32(&tlm.stopped), "idle check interval had not passed yet")
		time.Sleep(20 * time.Millisecond)
		require.EqualValues(t, 1, atomic.LoadInt32(&tlm.stopped), "idle check interval should have pass")
	})

	t.Run("Active poll-er", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tlm := createTestTaskListManagerWithConfig(t, testlogger.New(t), ctrl, cfg, clock.NewRealTimeSource())
		require.NoError(t, tlm.Start())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, _ = tlm.GetTask(ctx, nil)
		cancel()

		// task list manager should have been stopped,
		// but GetTask extends auto-stop until the next check-idle-task-list-interval
		time.Sleep(6 * time.Millisecond)
		require.EqualValues(t, 0, atomic.LoadInt32(&tlm.stopped))

		time.Sleep(20 * time.Millisecond)
		require.EqualValues(t, 1, atomic.LoadInt32(&tlm.stopped), "idle check interval should have pass")
	})

	t.Run("Active adding task", func(t *testing.T) {
		domainID := uuid.New()
		workflowID := uuid.New()
		runID := uuid.New()

		addTaskParam := AddTaskParams{
			TaskInfo: &persistence.TaskInfo{
				DomainID:                      domainID,
				WorkflowID:                    workflowID,
				RunID:                         runID,
				ScheduleID:                    2,
				ScheduleToStartTimeoutSeconds: 5,
				CreatedTime:                   time.Now(),
			},
		}

		ctrl := gomock.NewController(t)
		tlm := createTestTaskListManagerWithConfig(t, testlogger.New(t), ctrl, cfg, clock.NewRealTimeSource())
		require.NoError(t, tlm.Start())

		time.Sleep(8 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := tlm.AddTask(ctx, addTaskParam)
		require.NoError(t, err)
		cancel()

		// task list manager should have been stopped,
		// but AddTask extends auto-stop until the next check-idle-task-list-interval
		time.Sleep(6 * time.Millisecond)
		require.EqualValues(t, 0, atomic.LoadInt32(&tlm.stopped))

		time.Sleep(20 * time.Millisecond)
		require.EqualValues(t, 1, atomic.LoadInt32(&tlm.stopped), "idle check interval should have pass")
	})
}

func TestAddTaskStandby(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)

	cfg := config.NewConfig(dynamicconfig.NewNopCollection(), "some random hostname", getIsolationgroupsHelper)
	cfg.IdleTasklistCheckInterval = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)

	tlm := createTestTaskListManagerWithConfig(t, logger, controller, cfg, clock.NewMockedTimeSource())
	require.NoError(t, tlm.Start())

	// stop taskWriter so that we can check if there's any call to it
	// otherwise the task persist process is async and hard to test
	tlm.taskWriter.Stop()

	domainID := uuid.New()
	workflowID := "some random workflowID"
	runID := "some random runID"

	addTaskParam := AddTaskParams{
		TaskInfo: &persistence.TaskInfo{
			DomainID:                      domainID,
			WorkflowID:                    workflowID,
			RunID:                         runID,
			ScheduleID:                    2,
			ScheduleToStartTimeoutSeconds: 5,
			CreatedTime:                   time.Now(),
		},
	}

	testStandbyDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: domainID, Name: "some random domain name"},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	)
	mockDomainCache := tlm.domainCache.(*cache.MockDomainCache)
	mockDomainCache.EXPECT().GetDomainByID(domainID).Return(testStandbyDomainEntry, nil).AnyTimes()

	syncMatch, err := tlm.AddTask(context.Background(), addTaskParam)
	require.Equal(t, errShutdown, err) // task writer was stopped above
	require.False(t, syncMatch)

	addTaskParam.ForwardedFrom = "from child partition"
	syncMatch, err = tlm.AddTask(context.Background(), addTaskParam)
	require.Error(t, err) // should not persist the task
	require.False(t, syncMatch)
}

// return a client side tasklist throttle error from the rate limiter.
// The expected behaviour is to retry
func TestRateLimitErrorsFromTasklistDispatch(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)

	config := defaultTestConfig()
	config.EnableTasklistIsolation = func(domain string) bool { return true }
	tlm := createTestTaskListManagerWithConfig(t, logger, controller, config, clock.NewMockedTimeSource())

	tlm.taskReader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
		return ErrTasklistThrottled
	}
	tlm.taskReader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
		return "datacenterA", -1
	}

	breakDispatcher, breakRetryLoop := tlm.taskReader.dispatchSingleTaskFromBuffer(&persistence.TaskInfo{})
	assert.False(t, breakDispatcher)
	assert.False(t, breakRetryLoop)
}

// This is a bit of a strange unit-test as it's
// ensuring that invalid behaviour is handled defensively.
// It *should never be the case* that the isolation group tries to
// dispatch to a buffer that isn't there, however, if it does, we want to just
// log this, emit a metric and fallback to the default isolation group.
func TestMisconfiguredZoneDoesNotBlock(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)

	config := defaultTestConfig()
	config.EnableTasklistIsolation = func(domain string) bool { return true }
	tlm := createTestTaskListManagerWithConfig(t, logger, controller, config, clock.NewMockedTimeSource())
	invalidIsolationGroup := "invalid"
	dispatched := make(map[string]int)

	// record dispatched isolation group
	tlm.taskReader.dispatchTask = func(ctx context.Context, task *InternalTask) error {
		dispatched[task.isolationGroup] = dispatched[task.isolationGroup] + 1
		return nil
	}
	tlm.taskReader.getIsolationGroupForTask = func(ctx context.Context, info *persistence.TaskInfo) (string, time.Duration) {
		return invalidIsolationGroup, -1
	}

	maxBufferSize := config.GetTasksBatchSize("", "", 0) - 1

	for i := 0; i < maxBufferSize; i++ {
		breakDispatcher, breakRetryLoop := tlm.taskReader.dispatchSingleTaskFromBuffer(&persistence.TaskInfo{})
		assert.False(t, breakDispatcher, "dispatch isn't shutting down")
		assert.True(t, breakRetryLoop, "should be able to successfully dispatch all these tasks to the default isolation group")
	}
	// We should see them all being redirected
	assert.Equal(t, dispatched[""], maxBufferSize)
	// we should see none in the returned isolation group
	assert.Equal(t, dispatched[invalidIsolationGroup], 0)

	// ok, and here we try and ensure that this *does not block
	// and instead complains and live-retries
	breakDispatcher, breakRetryLoop := tlm.taskReader.dispatchSingleTaskFromBuffer(&persistence.TaskInfo{})
	assert.False(t, breakDispatcher, "dispatch isn't shutting down")
	assert.True(t, breakRetryLoop, "task should be dispatched to default channel")
}

func TestGetIsolationGroupForTask(t *testing.T) {
	defaultAvailableIsolationGroups := []string{
		"a", "b", "c",
	}
	taskIsolationPollerWindow := time.Second * 10
	testCases := []struct {
		name                     string
		taskIsolationGroup       string
		taskIsolationDuration    time.Duration
		taskLatency              time.Duration
		availableIsolationGroups []string
		recentPollers            []string
		expectedGroup            string
		expectedDuration         time.Duration
		drainedGroups            map[string]bool
		isolationStateErr        error
		disableTaskIsolation     bool
		partitionConfig          *types.TaskListPartition
	}{
		{
			name:                     "success - recent poller allows group",
			taskIsolationGroup:       "a",
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "a",
			expectedDuration:         0,
			recentPollers:            []string{"a"},
		},
		{
			name:                     "success - with isolation duration",
			taskIsolationGroup:       "b",
			taskIsolationDuration:    time.Second,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "b",
			expectedDuration:         time.Second,
			recentPollers:            []string{"b"},
		},
		{
			name:                     "success - low task latency",
			taskIsolationGroup:       "a",
			taskIsolationDuration:    time.Second,
			taskLatency:              time.Millisecond * 300,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "a",
			expectedDuration:         time.Second - (time.Millisecond * 300),
			recentPollers:            []string{"a"},
		},
		{
			name:                     "success - other group drained",
			taskIsolationGroup:       "a",
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "a",
			expectedDuration:         0,
			recentPollers:            []string{"a"},
			drainedGroups: map[string]bool{
				"b": true,
			},
		},
		{
			name:                     "success - right partition",
			taskIsolationGroup:       "a",
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "a",
			expectedDuration:         0,
			recentPollers:            []string{"a"},
			partitionConfig:          &types.TaskListPartition{IsolationGroups: []string{"a", "b", "c", "d"}},
		},
		{
			name:                     "leak - no recent pollers",
			taskIsolationGroup:       "a",
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "",
			expectedDuration:         0,
			recentPollers:            nil,
		},
		{
			name:                     "leak - no matching recent poller",
			taskIsolationGroup:       "a",
			taskIsolationDuration:    time.Second,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "",
			expectedDuration:         0,
			recentPollers:            []string{"b"},
		},
		{
			name:                     "leak - task latency",
			taskIsolationGroup:       "a",
			taskIsolationDuration:    time.Second,
			taskLatency:              time.Second,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			recentPollers:            []string{"a"},
			expectedGroup:            "",
			expectedDuration:         0,
		},
		{
			name:                     "leak - task latency close to taskIsolationDuration",
			taskIsolationGroup:       "a",
			taskIsolationDuration:    time.Second,
			taskLatency:              time.Second - minimumIsolationDuration,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			recentPollers:            []string{"a"},
			expectedGroup:            "",
			expectedDuration:         0,
		},
		{
			name:                     "leak - group drained",
			taskIsolationGroup:       "a",
			taskIsolationDuration:    time.Second,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			recentPollers:            []string{"a"},
			expectedGroup:            "",
			expectedDuration:         0,
			drainedGroups: map[string]bool{
				"a": true,
			},
		},
		{
			name:                     "leak - state error",
			taskIsolationGroup:       "a",
			taskIsolationDuration:    time.Second,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			recentPollers:            []string{"a"},
			isolationStateErr:        errors.New(">:("),
			expectedGroup:            "",
			expectedDuration:         0,
		},
		{
			name:                     "leak - task isolation disabled",
			taskIsolationGroup:       "a",
			taskIsolationDuration:    time.Second,
			availableIsolationGroups: defaultAvailableIsolationGroups,
			recentPollers:            []string{"a"},
			expectedGroup:            "",
			expectedDuration:         0,
			disableTaskIsolation:     true,
		},
		{
			name:                     "leak - wrong partition",
			taskIsolationGroup:       "a",
			availableIsolationGroups: defaultAvailableIsolationGroups,
			expectedGroup:            "",
			expectedDuration:         0,
			recentPollers:            []string{"a"},
			partitionConfig:          &types.TaskListPartition{IsolationGroups: []string{"b", "c", "d"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			logger := testlogger.New(t)

			config := defaultTestConfig()
			config.EnableTasklistIsolation = func(domain string) bool { return !tc.disableTaskIsolation }
			config.TaskIsolationDuration = func(domain string, taskList string, taskType int) time.Duration {
				return tc.taskIsolationDuration
			}
			config.TaskIsolationPollerWindow = func(domain string, taskList string, taskType int) time.Duration {
				return taskIsolationPollerWindow
			}
			config.AllIsolationGroups = func() []string {
				return tc.availableIsolationGroups
			}
			mockClock := clock.NewMockedTimeSource()
			tlm := createTestTaskListManagerWithConfig(t, logger, controller, config, mockClock)

			if tc.partitionConfig != nil {
				tlm.startWG.Done()
				// Add a partition and update the root to the specified config.
				// If we're not adding a new partition it is treated as a no-op since the default is 1 partition
				err := tlm.UpdateTaskListPartitionConfig(context.Background(), &types.TaskListPartitionConfig{
					ReadPartitions: map[int]*types.TaskListPartition{
						0: tc.partitionConfig,
						1: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: tc.partitionConfig,
						1: {},
					},
				})
				require.NoError(t, err)
			}

			mockIsolationGroupState := isolationgroup.NewMockState(controller)
			if tc.isolationStateErr != nil {
				mockIsolationGroupState.EXPECT().IsDrained(gomock.Any(), "domainName", gomock.Any()).Return(false, tc.isolationStateErr).AnyTimes()
			} else {
				mockIsolationGroupState.EXPECT().IsDrained(gomock.Any(), "domainName", gomock.Any()).DoAndReturn(func(ctx context.Context, domainName, group string) (bool, error) {
					return tc.drainedGroups[group], nil
				}).AnyTimes()
			}
			tlm.isolationState = mockIsolationGroupState

			for i, pollerGroup := range tc.recentPollers {
				tlm.pollers.StartPoll(strconv.Itoa(i), func() {}, &poller.Info{Identity: pollerGroup, IsolationGroup: pollerGroup})
			}

			taskInfo := &persistence.TaskInfo{
				DomainID:                      "domainId",
				RunID:                         "run1",
				WorkflowID:                    "workflow1",
				ScheduleID:                    5,
				ScheduleToStartTimeoutSeconds: 1,
				PartitionConfig: map[string]string{
					isolationgroup.GroupKey:      tc.taskIsolationGroup,
					isolationgroup.WorkflowIDKey: "workflow1",
				},
				CreatedTime: mockClock.Now().Add(-1 * tc.taskLatency),
			}

			actual, actualDuration := tlm.getIsolationGroupForTask(context.Background(), taskInfo)

			assert.Equal(t, tc.expectedGroup, actual)
			assert.Equal(t, tc.expectedDuration, actualDuration)
		})
	}
}

func TestTaskWriterShutdown(t *testing.T) {
	controller := gomock.NewController(t)
	logger := testlogger.New(t)
	tlm := createTestTaskListManager(t, logger, controller)
	err := tlm.Start()
	assert.NoError(t, err)

	// stop the task writer explicitly
	tlm.taskWriter.Stop()

	// now attempt to add a task
	addParams := AddTaskParams{
		TaskInfo: &persistence.TaskInfo{
			DomainID:                      "domainId",
			RunID:                         "run1",
			WorkflowID:                    "workflow1",
			ScheduleID:                    5,
			ScheduleToStartTimeoutSeconds: 1,
		},
	}
	_, err = tlm.AddTask(context.Background(), addParams)
	assert.Error(t, err)

	// test race
	tlm.taskWriter.stopped = 0
	_, err = tlm.AddTask(context.Background(), addParams)
	assert.Error(t, err)
	tlm.taskWriter.stopped = 1 // reset it back to old value
	tlm.Stop()
}

func TestTaskListManagerGetTaskBatch(t *testing.T) {
	const taskCount = 1200
	const rangeSize = 10
	controller := gomock.NewController(t)
	mockIsolationState := isolationgroup.NewMockState(controller)
	mockIsolationState.EXPECT().IsDrained(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	mockDomainCache := cache.NewMockDomainCache(controller)
	mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.CreateDomainCacheEntry("domainName"), nil).AnyTimes()
	mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("domainName", nil).AnyTimes()
	mockHistoryService := history.NewMockClient(controller)
	logger := testlogger.New(t)
	timeSource := clock.NewRealTimeSource()
	tm := NewTestTaskManager(t, logger, timeSource)
	taskListID := NewTestTaskListID(t, "domainId", "tl", 0)
	cfg := defaultTestConfig()
	cfg.RangeSize = rangeSize
	cfg.ReadRangeSize = dynamicproperties.GetIntPropertyFn(rangeSize / 2)
	tlMgr, err := NewManager(
		mockDomainCache,
		logger,
		metrics.NewClient(tally.NoopScope, metrics.Matching),
		tm,
		cluster.GetTestClusterMetadata(true),
		mockIsolationState,
		nil,
		func(Manager) {},
		taskListID,
		types.TaskListKindNormal,
		cfg,
		timeSource,
		timeSource.Now(),
		mockHistoryService,
	)
	assert.NoError(t, err)
	tlm := tlMgr.(*taskListManagerImpl)
	err = tlm.Start()
	assert.NoError(t, err)

	// add taskCount tasks
	for i := int64(0); i < taskCount; i++ {
		scheduleID := i * 3
		addParams := AddTaskParams{
			TaskInfo: &persistence.TaskInfo{
				DomainID:                      "domainId",
				RunID:                         "run1",
				WorkflowID:                    "workflow1",
				ScheduleID:                    scheduleID,
				ScheduleToStartTimeoutSeconds: 100,
			},
		}
		_, err = tlm.AddTask(context.Background(), addParams)
		assert.NoError(t, err)
	}
	assert.Equal(t, taskCount, tm.GetTaskCount(taskListID))

	// wait until all tasks are read by the task pump and enqeued into the in-memory buffer
	// at the end of this step, ackManager readLevel will also be equal to the buffer size
	expectedBufSize := min(cap(tlm.taskReader.taskBuffers[defaultTaskBufferIsolationGroup]), taskCount)
	assert.True(t, awaitCondition(func() bool {
		return len(tlm.taskReader.taskBuffers[defaultTaskBufferIsolationGroup]) == expectedBufSize
	}, 10*time.Second))

	// stop all goroutines that read / write tasks in the background
	// remainder of this test works with the in-memory buffer
	tlm.Stop()

	// SetReadLevel should NEVER be called without updating ackManager.outstandingTasks
	// This is only for unit test purpose
	tlm.taskAckManager.SetReadLevel(tlm.taskWriter.GetMaxReadLevel())
	tasks, readLevel, isReadBatchDone, err := tlm.taskReader.getTaskBatch(tlm.taskAckManager.GetReadLevel(), tlm.taskWriter.GetMaxReadLevel())
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
	assert.Equal(t, readLevel, tlm.taskWriter.GetMaxReadLevel())
	assert.True(t, isReadBatchDone)

	tlm.taskAckManager.SetReadLevel(0)
	tasks, readLevel, isReadBatchDone, err = tlm.taskReader.getTaskBatch(tlm.taskAckManager.GetReadLevel(), tlm.taskWriter.GetMaxReadLevel())
	assert.NoError(t, err)
	assert.Equal(t, rangeSize/2, len(tasks))
	assert.Equal(t, rangeSize/2, int(readLevel))
	assert.True(t, isReadBatchDone)

	// reset the ackManager readLevel to the buffer size and consume
	// the in-memory tasks by calling Poll API - assert ackMgr state
	// at the end
	tlm.taskAckManager.SetReadLevel(int64(expectedBufSize))

	// complete rangeSize events
	tlMgr, err = NewManager(
		mockDomainCache,
		logger,
		metrics.NewClient(tally.NoopScope, metrics.Matching),
		tm,
		cluster.GetTestClusterMetadata(true),
		mockIsolationState,
		nil,
		func(Manager) {},
		taskListID,
		types.TaskListKindNormal,
		cfg,
		timeSource,
		timeSource.Now(),
		mockHistoryService,
	)
	assert.NoError(t, err)
	tlm = tlMgr.(*taskListManagerImpl)
	err = tlm.Start()
	assert.NoError(t, err)
	for i := int64(0); i < rangeSize; i++ {
		task, err := tlm.GetTask(context.Background(), nil)
		if errors.Is(err, ErrNoTasks) {
			continue
		}
		assert.NotNil(t, task.AutoConfigHint)
		assert.NoError(t, err)
		assert.NotNil(t, task)
		task.Finish(nil)
	}
	assert.Equal(t, taskCount-rangeSize, tm.GetTaskCount(taskListID))
	tasks, _, isReadBatchDone, err = tlm.taskReader.getTaskBatch(tlm.taskAckManager.GetReadLevel(), tlm.taskWriter.GetMaxReadLevel())
	assert.NoError(t, err)
	assert.True(t, 0 < len(tasks) && len(tasks) <= rangeSize)
	assert.True(t, isReadBatchDone)
	tlm.Stop()
}

func TestTaskListReaderPumpAdvancesAckLevelAfterEmptyReads(t *testing.T) {
	const taskCount = 5
	const rangeSize = 10
	const nLeaseRenewals = 15

	controller := gomock.NewController(t)
	mockIsolationState := isolationgroup.NewMockState(controller)
	mockIsolationState.EXPECT().IsDrained(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	mockDomainCache := cache.NewMockDomainCache(controller)
	mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.CreateDomainCacheEntry("domainName"), nil).AnyTimes()
	mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("domainName", nil).AnyTimes()
	mockHistoryService := history.NewMockClient(controller)

	logger := testlogger.New(t)
	timeSource := clock.NewRealTimeSource()
	tm := NewTestTaskManager(t, logger, timeSource)
	taskListID := NewTestTaskListID(t, "domainId", "tl", 0)
	cfg := defaultTestConfig()
	cfg.RangeSize = rangeSize
	cfg.ReadRangeSize = dynamicproperties.GetIntPropertyFn(rangeSize / 2)

	tlMgr, err := NewManager(
		mockDomainCache,
		logger,
		metrics.NewClient(tally.NoopScope, metrics.Matching),
		tm,
		cluster.GetTestClusterMetadata(true),
		mockIsolationState,
		nil,
		func(Manager) {},
		taskListID,
		types.TaskListKindNormal,
		cfg,
		timeSource,
		timeSource.Now(),
		mockHistoryService,
	)
	require.NoError(t, err)
	tlm := tlMgr.(*taskListManagerImpl)

	// simulate lease renewal multiple times without writing any tasks
	for i := 0; i < nLeaseRenewals; i++ {
		tlm.taskWriter.renewLeaseWithRetry()
	}

	err = tlm.Start() // this call will also renew lease
	require.NoError(t, err)
	defer tlm.Stop()

	// we expect AckLevel to advance and skip all the previously leased ranges
	expectedAckLevel := int64(rangeSize) * nLeaseRenewals

	// wait until task pump will read batches of empty ranges
	assert.True(t, awaitCondition(func() bool {
		return tlm.taskAckManager.GetAckLevel() == expectedAckLevel
	}, 10*time.Second))

	assert.Equal(
		t,
		expectedAckLevel,
		tlm.taskAckManager.GetAckLevel(),
		"we should ack ranges of all the previously acquired leases",
	)

	assert.Equal(
		t,
		tlm.taskWriter.GetMaxReadLevel(),
		tlm.taskAckManager.GetAckLevel(),
		"we should have been acked everything possible",
	)

	maxReadLevelBeforeAddingTasks := tlm.taskWriter.GetMaxReadLevel()

	// verify new task writes go beyond the MaxReadLevel/AckLevel
	for i := int64(0); i < taskCount; i++ {
		addParams := AddTaskParams{
			TaskInfo: &persistence.TaskInfo{
				DomainID:   "domainId",
				RunID:      "run1",
				WorkflowID: "workflow1",
				ScheduleID: i,
			},
		}
		_, err = tlm.AddTask(context.Background(), addParams)
		require.NoError(t, err)
		assert.Equal(t, maxReadLevelBeforeAddingTasks+i+1, tlm.taskWriter.GetMaxReadLevel())
	}
}

func TestTaskListManagerGetTaskBatch_ReadBatchDone(t *testing.T) {
	const rangeSize = 10
	const maxReadLevel = int64(120)
	config := defaultTestConfig()
	config.RangeSize = rangeSize
	config.ReadRangeSize = dynamicproperties.GetIntPropertyFn(rangeSize / 2)
	controller := gomock.NewController(t)
	logger := testlogger.New(t)
	tlm := createTestTaskListManagerWithConfig(t, logger, controller, config, clock.NewMockedTimeSource())

	tlm.taskAckManager.SetReadLevel(0)
	atomic.StoreInt64(&tlm.taskWriter.maxReadLevel, maxReadLevel)
	tasks, readLevel, isReadBatchDone, err := tlm.taskReader.getTaskBatch(tlm.taskAckManager.GetReadLevel(), tlm.taskWriter.GetMaxReadLevel())
	assert.Empty(t, tasks)
	assert.Equal(t, int64(rangeSize/2*10), readLevel)
	assert.False(t, isReadBatchDone)
	assert.NoError(t, err)

	tlm.taskAckManager.SetReadLevel(readLevel)
	atomic.StoreInt64(&tlm.taskWriter.maxReadLevel, maxReadLevel)
	tasks, readLevel, isReadBatchDone, err = tlm.taskReader.getTaskBatch(tlm.taskAckManager.GetReadLevel(), tlm.taskWriter.GetMaxReadLevel())
	assert.Empty(t, tasks)
	assert.Equal(t, 2*int64(rangeSize/2*10), readLevel)
	assert.False(t, isReadBatchDone)
	assert.NoError(t, err)

	tlm.taskAckManager.SetReadLevel(readLevel)
	tasks, readLevel, isReadBatchDone, err = tlm.taskReader.getTaskBatch(tlm.taskAckManager.GetReadLevel(), tlm.taskWriter.GetMaxReadLevel())
	assert.Empty(t, tasks)
	assert.Equal(t, maxReadLevel, readLevel)
	assert.True(t, isReadBatchDone)
	assert.NoError(t, err)
}

func awaitCondition(cond func() bool, timeout time.Duration) bool {
	expiry := time.Now().Add(timeout)
	for !cond() {
		time.Sleep(time.Millisecond * 5)
		if time.Now().After(expiry) {
			return false
		}
	}
	return true
}

func TestTaskExpiryAndCompletion(t *testing.T) {
	const taskCount = 20
	const rangeSize = 10

	testCases := []struct {
		name               string
		batchSize          int
		maxTimeBtwnDeletes time.Duration
	}{
		{"test taskGC deleting due to size threshold", 2, time.Minute},
		{"test taskGC deleting due to time condition", 100, time.Nanosecond},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			controller := gomock.NewController(t)
			mockIsolationState := isolationgroup.NewMockState(controller)
			mockIsolationState.EXPECT().IsDrained(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
			mockDomainCache := cache.NewMockDomainCache(controller)
			mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.CreateDomainCacheEntry("domainName"), nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("domainName", nil).AnyTimes()
			mockHistoryService := history.NewMockClient(controller)
			logger := testlogger.New(t)
			timeSource := clock.NewRealTimeSource()
			tm := NewTestTaskManager(t, logger, timeSource)
			taskListID := NewTestTaskListID(t, "domainId", "tl", 0)
			cfg := defaultTestConfig()
			cfg.RangeSize = rangeSize
			cfg.ReadRangeSize = dynamicproperties.GetIntPropertyFn(rangeSize / 2)
			cfg.MaxTaskDeleteBatchSize = dynamicproperties.GetIntPropertyFilteredByTaskListInfo(tc.batchSize)
			cfg.MaxTimeBetweenTaskDeletes = tc.maxTimeBtwnDeletes
			// set idle timer check to a really small value to assert that we don't accidentally drop tasks while blocking
			// on enqueuing a task to task buffer
			cfg.IdleTasklistCheckInterval = dynamicproperties.GetDurationPropertyFnFilteredByTaskListInfo(20 * time.Millisecond)
			tlMgr, err := NewManager(
				mockDomainCache,
				logger,
				metrics.NewClient(tally.NoopScope, metrics.Matching),
				tm,
				cluster.GetTestClusterMetadata(true),
				mockIsolationState,
				nil,
				func(Manager) {},
				taskListID,
				types.TaskListKindNormal,
				cfg,
				timeSource,
				timeSource.Now(),
				mockHistoryService,
			)
			assert.NoError(t, err)
			tlm := tlMgr.(*taskListManagerImpl)
			err = tlm.Start()
			assert.NoError(t, err)
			for i := int64(0); i < taskCount; i++ {
				scheduleID := i * 3
				addParams := AddTaskParams{
					TaskInfo: &persistence.TaskInfo{
						DomainID:                      "domainId",
						RunID:                         "run1",
						WorkflowID:                    "workflow1",
						ScheduleID:                    scheduleID,
						ScheduleToStartTimeoutSeconds: 100,
					},
				}
				if i%2 == 0 {
					// simulates creating a task whose scheduledToStartTimeout is already expired
					addParams.TaskInfo.ScheduleToStartTimeoutSeconds = -5
				}
				_, err = tlm.AddTask(context.Background(), addParams)
				assert.NoError(t, err)
			}
			assert.Equal(t, taskCount, tm.GetTaskCount(taskListID))

			// wait until all tasks are loaded by into in-memory buffers by task list manager
			// the buffer size should be one less than expected because dispatcher will dequeue the head
			assert.True(t, awaitCondition(func() bool {
				return len(tlm.taskReader.taskBuffers[defaultTaskBufferIsolationGroup]) >= (taskCount/2 - 1)
			}, time.Second))

			remaining := taskCount
			for i := 0; i < 2; i++ {
				// verify that (1) expired tasks are not returned in poll result (2) taskCleaner deletes tasks correctly
				for i := int64(0); i < taskCount/4; i++ {
					task, err := tlm.GetTask(context.Background(), nil)
					assert.NoError(t, err)
					assert.NotNil(t, task)
					task.Finish(nil)
				}
				remaining -= taskCount / 2
				// since every other task is expired, we expect half the tasks to be deleted
				// after poll consumed 1/4th of what is available
				assert.Equal(t, remaining, tm.GetTaskCount(taskListID))
			}
			tlm.Stop()
		})
	}
}

func TestTaskListManagerImpl_HasPollerAfter(t *testing.T) {
	for name, tc := range map[string]struct {
		outstandingPollers []string
		prepareManager     func(*taskListManagerImpl)
	}{
		"has_outstanding_pollers": {
			prepareManager: func(tlm *taskListManagerImpl) {
				tlm.pollers.StartPoll("poller1", func() {}, &poller.Info{Identity: "foo"})
			},
		},
		"no_outstanding_pollers": {
			prepareManager: func(tlm *taskListManagerImpl) {
				tlm.pollers.StartPoll("poller1", func() {}, &poller.Info{Identity: "foo"})
				tlm.pollers.EndPoll("poller1")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			logger := testlogger.New(t)
			tlm := createTestTaskListManager(t, logger, controller)
			err := tlm.Start()
			assert.NoError(t, err)

			if tc.prepareManager != nil {
				tc.prepareManager(tlm)
			}

			assert.True(t, tlm.HasPollerAfter(time.Time{}))
		})
	}
}

func getIsolationgroupsHelper() []string {
	return testIsolationGroups
}

func TestRefreshTaskListPartitionConfig(t *testing.T) {
	testCases := []struct {
		name           string
		req            *types.TaskListPartitionConfig
		originalConfig *types.TaskListPartitionConfig
		setupMocks     func(*mockDeps)
		expectedConfig *types.TaskListPartitionConfig
		expectError    bool
		expectedError  string
	}{
		{
			name: "success - refresh from request",
			req: &types.TaskListPartitionConfig{
				Version:         2,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			setupMocks: func(m *mockDeps) {},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         2,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
		},
		{
			name: "success - ignore older version",
			req: &types.TaskListPartitionConfig{
				Version:         2,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			originalConfig: &types.TaskListPartitionConfig{
				Version:         3,
				ReadPartitions:  partitions(2),
				WritePartitions: partitions(2),
			},
			setupMocks: func(m *mockDeps) {},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         3,
				ReadPartitions:  partitions(2),
				WritePartitions: partitions(2),
			},
		},
		{
			name: "success - refresh from database",
			originalConfig: &types.TaskListPartitionConfig{
				Version:         3,
				ReadPartitions:  partitions(2),
				WritePartitions: partitions(2),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockTaskManager.EXPECT().GetTaskList(gomock.Any(), &persistence.GetTaskListRequest{
					DomainID:   "domain-id",
					DomainName: "domainName",
					TaskList:   "tl",
					TaskType:   persistence.TaskListTypeDecision,
				}).Return(&persistence.GetTaskListResponse{
					TaskListInfo: &persistence.TaskListInfo{
						AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
							Version:         4,
							ReadPartitions:  persistencePartitions(10),
							WritePartitions: persistencePartitions(10),
						},
					},
				}, nil)
			},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         4,
				ReadPartitions:  partitions(10),
				WritePartitions: partitions(10),
			},
		},
		{
			name: "failed to refresh from database",
			originalConfig: &types.TaskListPartitionConfig{
				Version:         3,
				ReadPartitions:  partitions(2),
				WritePartitions: partitions(2),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockTaskManager.EXPECT().GetTaskList(gomock.Any(), &persistence.GetTaskListRequest{
					DomainID:   "domain-id",
					DomainName: "domainName",
					TaskList:   "tl",
					TaskType:   persistence.TaskListTypeDecision,
				}).Return(nil, errors.New("some error"))
			},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         3,
				ReadPartitions:  partitions(2),
				WritePartitions: partitions(2),
			},
			expectError:   true,
			expectedError: "some error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
			require.NoError(t, err)
			tlm, deps := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)
			tc.setupMocks(deps)
			tlm.partitionConfig = tc.originalConfig
			tlm.startWG.Done()

			err = tlm.RefreshTaskListPartitionConfig(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
				assert.Equal(t, tc.expectedConfig, tlm.TaskListPartitionConfig())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedConfig, tlm.TaskListPartitionConfig())
			}
		})
	}
}

func TestUpdateTaskListPartitionConfig(t *testing.T) {
	testCases := []struct {
		name           string
		req            *types.TaskListPartitionConfig
		originalConfig *types.TaskListPartitionConfig
		setupMocks     func(*mockDeps)
		expectedConfig *types.TaskListPartitionConfig
		expectError    bool
		expectedError  string
	}{
		{
			name: "success - no op",
			req: &types.TaskListPartitionConfig{
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			originalConfig: &types.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			setupMocks: func(m *mockDeps) {},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
		},
		{
			name: "success - no op, nil pointer",
			req: &types.TaskListPartitionConfig{
				ReadPartitions:  partitions(1),
				WritePartitions: partitions(1),
			},
			originalConfig: nil,
			setupMocks:     func(m *mockDeps) {},
			expectedConfig: nil,
		},
		{
			name: "success - update",
			req: &types.TaskListPartitionConfig{
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(1),
			},
			originalConfig: &types.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockTaskManager.EXPECT().UpdateTaskList(gomock.Any(), &persistence.UpdateTaskListRequest{
					DomainName: "domainName",
					TaskListInfo: &persistence.TaskListInfo{
						DomainID: "domain-id",
						Name:     "tl",
						AckLevel: 0,
						RangeID:  0,
						Kind:     persistence.TaskListKindNormal,
						AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
							Version:         2,
							ReadPartitions:  persistencePartitions(3),
							WritePartitions: persistencePartitions(1),
						},
					},
				}).Return(&persistence.UpdateTaskListResponse{}, nil)
				deps.mockMatchingClient.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &types.MatchingRefreshTaskListPartitionConfigRequest{
					DomainUUID:   "domain-id",
					TaskList:     &types.TaskList{Name: "/__cadence_sys/tl/1", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         2,
						ReadPartitions:  partitions(3),
						WritePartitions: partitions(1),
					},
				}).Return(&types.MatchingRefreshTaskListPartitionConfigResponse{}, nil)
				deps.mockMatchingClient.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &types.MatchingRefreshTaskListPartitionConfigRequest{
					DomainUUID:   "domain-id",
					TaskList:     &types.TaskList{Name: "/__cadence_sys/tl/2", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         2,
						ReadPartitions:  partitions(3),
						WritePartitions: partitions(1),
					},
				}).Return(&types.MatchingRefreshTaskListPartitionConfigResponse{}, nil)
			},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         2,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(1),
			},
		},
		{
			name: "success - push failures are ignored",
			req: &types.TaskListPartitionConfig{
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(1),
			},
			originalConfig: &types.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockTaskManager.EXPECT().UpdateTaskList(gomock.Any(), &persistence.UpdateTaskListRequest{
					DomainName: "domainName",
					TaskListInfo: &persistence.TaskListInfo{
						DomainID: "domain-id",
						Name:     "tl",
						AckLevel: 0,
						RangeID:  0,
						Kind:     persistence.TaskListKindNormal,
						AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
							Version:         2,
							ReadPartitions:  persistencePartitions(3),
							WritePartitions: persistencePartitions(1),
						},
					},
				}).Return(&persistence.UpdateTaskListResponse{}, nil)
				deps.mockMatchingClient.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &types.MatchingRefreshTaskListPartitionConfigRequest{
					DomainUUID:   "domain-id",
					TaskList:     &types.TaskList{Name: "/__cadence_sys/tl/1", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         2,
						ReadPartitions:  partitions(3),
						WritePartitions: partitions(1),
					},
				}).Return(nil, errors.New("matching client error"))
				deps.mockMatchingClient.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &types.MatchingRefreshTaskListPartitionConfigRequest{
					DomainUUID:   "domain-id",
					TaskList:     &types.TaskList{Name: "/__cadence_sys/tl/2", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         2,
						ReadPartitions:  partitions(3),
						WritePartitions: partitions(1),
					},
				}).Return(nil, errors.New("matching client error"))
			},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         2,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(1),
			},
		},
		{
			name: "failed to update",
			req: &types.TaskListPartitionConfig{
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(1),
			},
			originalConfig: &types.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockTaskManager.EXPECT().UpdateTaskList(gomock.Any(), &persistence.UpdateTaskListRequest{
					DomainName: "domainName",
					TaskListInfo: &persistence.TaskListInfo{
						DomainID: "domain-id",
						Name:     "tl",
						AckLevel: 0,
						RangeID:  0,
						Kind:     persistence.TaskListKindNormal,
						AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
							Version:         2,
							ReadPartitions:  persistencePartitions(3),
							WritePartitions: persistencePartitions(1),
						},
					},
				}).Return(nil, errors.New("some error"))
			},
			expectedConfig: &types.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  partitions(3),
				WritePartitions: partitions(3),
			},
			expectError:   true,
			expectedError: "some error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
			require.NoError(t, err)
			tlm, deps := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)
			tc.setupMocks(deps)
			tlm.partitionConfig = tc.originalConfig
			tlm.startWG.Done()

			err = tlm.UpdateTaskListPartitionConfig(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
				assert.Equal(t, int32(1), tlm.stopped)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedConfig, tlm.TaskListPartitionConfig())
		})
	}
}

func TestRefreshTaskListPartitionConfigConcurrency(t *testing.T) {
	tlID, err := NewIdentifier("domain-id", "/__cadence_sys/tl/1", persistence.TaskListTypeDecision)
	require.NoError(t, err)
	tlm, _ := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)
	tlm.startWG.Done()

	var g errgroup.Group
	for i := 0; i < 100; i++ {
		v := i
		g.Go(func() error {
			return tlm.RefreshTaskListPartitionConfig(context.Background(), &types.TaskListPartitionConfig{Version: int64(v), ReadPartitions: partitions(v), WritePartitions: partitions(v)})
		})
	}
	require.NoError(t, g.Wait())
	assert.Equal(t, int64(99), tlm.TaskListPartitionConfig().Version)
}

func TestUpdateTaskListPartitionConfigConcurrency(t *testing.T) {
	tlID, err := NewIdentifier("domain-id", "/__cadence_sys/tl/1", persistence.TaskListTypeDecision)
	require.NoError(t, err)
	tlm, deps := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)
	deps.mockTaskManager.EXPECT().UpdateTaskList(gomock.Any(), gomock.Any()).Return(&persistence.UpdateTaskListResponse{}, nil).AnyTimes()
	deps.mockMatchingClient.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), gomock.Any()).Return(&types.MatchingRefreshTaskListPartitionConfigResponse{}, nil).AnyTimes()
	tlm.startWG.Done()

	var g errgroup.Group
	for i := 2; i < 102; i++ {
		v := i
		g.Go(func() error {
			return tlm.UpdateTaskListPartitionConfig(context.Background(), &types.TaskListPartitionConfig{ReadPartitions: partitions(v), WritePartitions: partitions(v)})
		})
	}
	require.NoError(t, g.Wait())
	assert.Equal(t, int64(100), tlm.TaskListPartitionConfig().Version)
}

func TestManagerStart_RootPartition(t *testing.T) {
	tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
	require.NoError(t, err)
	tlm, deps := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)
	deps.mockTaskManager.EXPECT().LeaseTaskList(gomock.Any(), &persistence.LeaseTaskListRequest{
		DomainID:   "domain-id",
		DomainName: "domainName",
		TaskList:   "tl",
		TaskType:   persistence.TaskListTypeDecision,
	}).Return(&persistence.LeaseTaskListResponse{
		TaskListInfo: &persistence.TaskListInfo{
			DomainID: "domain-id",
			Name:     "tl",
			Kind:     persistence.TaskListKindNormal,
			AckLevel: 0,
			RangeID:  0,
			AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  persistencePartitions(2),
				WritePartitions: persistencePartitions(2),
			},
		},
	}, nil)
	deps.mockMatchingClient.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &types.MatchingRefreshTaskListPartitionConfigRequest{
		DomainUUID:   "domain-id",
		TaskList:     &types.TaskList{Name: "/__cadence_sys/tl/1", Kind: types.TaskListKindNormal.Ptr()},
		TaskListType: types.TaskListTypeDecision.Ptr(),
		PartitionConfig: &types.TaskListPartitionConfig{
			Version:         1,
			ReadPartitions:  partitions(2),
			WritePartitions: partitions(2),
		},
	}).Return(&types.MatchingRefreshTaskListPartitionConfigResponse{}, nil)
	assert.NoError(t, tlm.Start())
	assert.Equal(t, &types.TaskListPartitionConfig{Version: 1, ReadPartitions: partitions(2), WritePartitions: partitions(2)}, tlm.TaskListPartitionConfig())
	tlm.stopWG.Wait()
}

func TestManagerStart_NonRootPartition(t *testing.T) {
	tlID, err := NewIdentifier("domain-id", "/__cadence_sys/tl/1", persistence.TaskListTypeDecision)
	require.NoError(t, err)
	tlm, deps := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)
	deps.mockTaskManager.EXPECT().GetTaskList(gomock.Any(), &persistence.GetTaskListRequest{
		DomainID:   "domain-id",
		DomainName: "domainName",
		TaskList:   "tl",
		TaskType:   persistence.TaskListTypeDecision,
	}).Return(&persistence.GetTaskListResponse{
		TaskListInfo: &persistence.TaskListInfo{
			DomainID: "domain-id",
			Name:     "tl",
			Kind:     persistence.TaskListKindNormal,
			AckLevel: 0,
			RangeID:  0,
			AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
				Version:         1,
				ReadPartitions:  persistencePartitions(3),
				WritePartitions: persistencePartitions(3),
			},
		},
	}, nil)
	deps.mockTaskManager.EXPECT().LeaseTaskList(gomock.Any(), &persistence.LeaseTaskListRequest{
		DomainID:   "domain-id",
		DomainName: "domainName",
		TaskList:   "/__cadence_sys/tl/1",
		TaskType:   persistence.TaskListTypeDecision,
	}).Return(&persistence.LeaseTaskListResponse{
		TaskListInfo: &persistence.TaskListInfo{
			DomainID: "domain-id",
			Name:     "/__cadence_sys/tl/1",
			Kind:     persistence.TaskListKindNormal,
			AckLevel: 0,
			RangeID:  0,
		},
	}, nil)
	assert.NoError(t, tlm.Start())
	assert.Equal(t, &types.TaskListPartitionConfig{
		Version:         1,
		ReadPartitions:  partitions(3),
		WritePartitions: partitions(3),
	}, tlm.TaskListPartitionConfig())
}

func TestDispatchTask(t *testing.T) {
	testCases := []struct {
		name                        string
		mockSetup                   func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask)
		enableStandByTaskCompletion bool
		activeClusterName           string
		err                         error
	}{
		{
			name: "active cluster - disabled StandByTaskCompletion - task sent to MustOffer and no error returned",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				matcher.EXPECT().MustOffer(ctx, task).Return(nil).Times(1)
			},
			activeClusterName: cluster.TestCurrentClusterName,
			err:               nil,
		},
		{
			name: "active cluster - disabled StandByTaskCompletion - task sent to MustOffer and error returned",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				matcher.EXPECT().MustOffer(ctx, task).Return(errors.New("no-task-completion-must-offer-error")).Times(1)
			},
			activeClusterName: cluster.TestCurrentClusterName,
			err:               errors.New("no-task-completion-must-offer-error"),
		},
		{
			name: "active cluster - enabled StandByTaskCompletion - task sent to MustOffer and no error returned",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				matcher.EXPECT().MustOffer(ctx, task).Return(nil).Times(1)
			},
			enableStandByTaskCompletion: true,
			activeClusterName:           cluster.TestCurrentClusterName,
			err:                         nil,
		},
		{
			name: "active cluster - enabled StandByTaskCompletion - task sent to MustOffer and error returned",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				matcher.EXPECT().MustOffer(ctx, task).Return(errors.New("task-completion-must-offer-error")).Times(1)
			},
			enableStandByTaskCompletion: true,
			activeClusterName:           cluster.TestCurrentClusterName,
			err:                         errors.New("task-completion-must-offer-error"),
		},
		{
			name: "standby cluster - disabled StandByTaskCompletion - task sent to MustOffer and no error returned",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				matcher.EXPECT().MustOffer(ctx, task).Return(nil).Times(1)
			},
			activeClusterName: cluster.TestAlternativeClusterName,
			err:               nil,
		},
		{
			name: "standby cluster - disabled StandByTaskCompletion - task sent to MustOffer and error returned",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				matcher.EXPECT().MustOffer(ctx, task).Return(errors.New("no-task-completion-must-offer-error")).Times(1)
			},
			activeClusterName: cluster.TestAlternativeClusterName,
			err:               errors.New("no-task-completion-must-offer-error"),
		},
		{
			name: "standby cluster - enabled StandByTaskCompletion - task completed",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				taskCompleter.EXPECT().CompleteTaskIfStarted(ctx, task).Return(nil).Times(1)
			},
			enableStandByTaskCompletion: true,
			activeClusterName:           cluster.TestAlternativeClusterName,
			err:                         nil,
		},
		{
			name: "standby cluster - enabled StandByTaskCompletion - task completion error",
			mockSetup: func(matcher *MockTaskMatcher, taskCompleter *MockTaskCompleter, ctx context.Context, task *InternalTask) {
				taskCompleter.EXPECT().CompleteTaskIfStarted(ctx, task).Return(errTaskNotStarted).Times(1)
			},
			enableStandByTaskCompletion: true,
			activeClusterName:           cluster.TestAlternativeClusterName,
			err:                         errTaskNotStarted,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			logger := testlogger.New(t)
			tlm := createTestTaskListManager(t, logger, controller)

			task := &InternalTask{
				Event: &genericTaskInfo{
					TaskInfo: &persistence.TaskInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
				},
			}

			taskMatcher := NewMockTaskMatcher(controller)
			taskCompleter := NewMockTaskCompleter(controller)
			tlm.matcher = taskMatcher
			tlm.taskCompleter = taskCompleter
			tlm.config.EnableStandbyTaskCompletion = func() bool {
				return tc.enableStandByTaskCompletion
			}

			mockDomainCache := cache.NewMockDomainCache(controller)
			cacheEntry := cache.NewGlobalDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: constants.TestDomainID, Name: constants.TestDomainName},
				&persistence.DomainConfig{Retention: 1},
				&persistence.DomainReplicationConfig{
					ActiveClusterName: tc.activeClusterName,
					Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
				},
				1,
			)

			mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(cacheEntry, nil).AnyTimes()
			tlm.domainCache = mockDomainCache

			ctx := context.Background()
			tc.mockSetup(taskMatcher, taskCompleter, ctx, task)

			err := tlm.DispatchTask(ctx, task)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetNumPartitions(t *testing.T) {
	tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
	require.NoError(t, err)
	tlm, deps := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)
	require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.MatchingEnableGetNumberOfPartitionsFromCache, true))
	assert.NotPanics(t, func() { tlm.limiter.ReportLimit(float64(100)) })
}

func TestDisconnectBlockedPollers(t *testing.T) {
	tests := []struct {
		name                 string
		newActiveClusterName *string
		mockSetup            func(mockMatcher *MockTaskMatcher)
		stopped              int32
		expectedErr          error
	}{
		{
			name:                 "disconnect blocked pollers and refresh cancel context",
			newActiveClusterName: common.StringPtr("new-active-cluster"),
			mockSetup: func(mockMatcher *MockTaskMatcher) {
				mockMatcher.EXPECT().DisconnectBlockedPollers().Times(1)
				mockMatcher.EXPECT().RefreshCancelContext().Times(1)
			},
			expectedErr: nil,
		},
		{
			name:                 "tasklist manager is shutting down, noop",
			newActiveClusterName: common.StringPtr("new-active-cluster"),
			mockSetup:            func(mockMatcher *MockTaskMatcher) {},
			stopped:              int32(1),
			expectedErr:          errShutdown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
			require.NoError(t, err)

			tlm, _ := setupMocksForTaskListManager(t, tlID, types.TaskListKindNormal)

			mockMatcher := NewMockTaskMatcher(gomock.NewController(t))
			tlm.matcher = mockMatcher

			tc.mockSetup(mockMatcher)

			tlm.stopped = tc.stopped

			err = tlm.ReleaseBlockedPollers()

			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func partitions(num int) map[int]*types.TaskListPartition {
	result := make(map[int]*types.TaskListPartition, num)
	for i := 0; i < num; i++ {
		result[i] = &types.TaskListPartition{}
	}
	return result
}

func persistencePartitions(num int) map[int]*persistence.TaskListPartition {
	result := make(map[int]*persistence.TaskListPartition, num)
	for i := 0; i < num; i++ {
		result[i] = &persistence.TaskListPartition{}
	}
	return result
}
