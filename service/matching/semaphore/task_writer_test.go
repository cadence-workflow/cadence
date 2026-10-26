package semaphore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/matching/config"
)

// newTestQueueManager returns a manager with only the fields its writer and reader are built from.
func newTestQueueManager(t *testing.T, store persistence.SemaphoreTaskManager, timeSource clock.TimeSource) *semaphoreManagerImpl {
	t.Helper()
	logger := testlogger.New(t)
	return &semaphoreManagerImpl{
		id:             testBucketID,
		logger:         logger,
		timeSource:     timeSource,
		config:         &config.SemaphoreConfig{UpdateAckInterval: testUpdateAckInterval},
		taskDB:         newTaskDB(testBucketID, store),
		taskAckManager: messaging.NewAckManager(logger),
	}
}

// newTestTaskWriter returns an unclaimed writer over store, stopped when the test ends.
func newTestTaskWriter(t *testing.T, store persistence.SemaphoreTaskManager) *taskWriter {
	t.Helper()
	w := newTaskWriter(newTestQueueManager(t, store, clock.NewMockedTimeSource()))
	t.Cleanup(w.stop)
	return w
}

// claimRequest is the claim the bucket sends while holding haveRangeID; 0 is the first claim.
func claimRequest(haveRangeID int64) *persistence.ClaimSemaphoreTaskBucketRequest {
	return &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		RangeID:       haveRangeID,
	}
}

// expectClaim expects one claim sent while holding haveRangeID, answered with rangeID and
// ackLevel.
func expectClaim(store *persistence.MockSemaphoreTaskManager, haveRangeID, rangeID, ackLevel int64) {
	store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(haveRangeID)).Times(1).
		Return(&persistence.ClaimSemaphoreTaskBucketResponse{RangeID: rangeID, AckLevel: ackLevel}, nil)
}

// startTestTaskWriter returns a started writer that claimed range 1 with ack level 0, with ranges
// of rangeSize task ids.
func startTestTaskWriter(t *testing.T, store *persistence.MockSemaphoreTaskManager, rangeSize int64) *taskWriter {
	t.Helper()
	expectClaim(store, 0, 1, 0)
	w := newTestTaskWriter(t, store)
	w.rangeSize = rangeSize
	require.NoError(t, w.claim())
	w.start()
	return w
}

// waiter returns a task as an append gets it, with no task id.
func waiter(workflowID string) *persistence.SemaphoreTask {
	return &persistence.SemaphoreTask{WorkflowID: workflowID, RunID: "run-" + workflowID, HoldID: 7}
}

// written returns waiter(workflowID) as the writer writes it, with taskID.
func written(workflowID string, taskID int64) *persistence.SemaphoreTask {
	task := waiter(workflowID)
	task.TaskID = taskID
	return task
}

// createRequest is the write of tasks while holding rangeID.
func createRequest(rangeID int64, tasks ...*persistence.SemaphoreTask) *persistence.CreateSemaphoreTasksRequest {
	return &persistence.CreateSemaphoreTasksRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		RangeID:       rangeID,
		Tasks:         tasks,
	}
}

// testContext bounds a call, so a test whose writer goroutine failed on the mock reports it
// instead of waiting for an answer that never comes.
func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestRangeIDToTaskIDBlock(t *testing.T) {
	tests := []struct {
		name    string
		rangeID int64
		want    taskIDBlock
	}{
		{name: "the first range starts at task id 1", rangeID: 1, want: taskIDBlock{start: 1, end: 100}},
		{name: "the second range starts right after the first ends", rangeID: 2, want: taskIDBlock{start: 101, end: 200}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, rangeIDToTaskIDBlock(tc.rangeID, 100))
		})
	}
}

func TestClaim(t *testing.T) {
	tests := []struct {
		name string
		// rangeID and ackLevel are what the claim returns.
		rangeID  int64
		ackLevel int64
		// wantMaxReadLevel is the last task id of the earlier ranges.
		wantMaxReadLevel int64
		wantBlock        taskIDBlock
	}{
		{
			name:             "first claim of a new bucket starts at task id 1",
			rangeID:          1,
			wantMaxReadLevel: 0,
			wantBlock:        taskIDBlock{start: 1, end: 100},
		},
		{
			name:             "claim after another host held the bucket starts after its ranges",
			rangeID:          3,
			ackLevel:         150,
			wantMaxReadLevel: 200,
			wantBlock:        taskIDBlock{start: 201, end: 300},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
			expectClaim(store, 0, tc.rangeID, tc.ackLevel)
			w := newTestTaskWriter(t, store)
			w.rangeSize = 100

			require.NoError(t, w.claim())
			assert.Equal(t, tc.wantBlock, w.taskIDBlock)
			assert.Equal(t, tc.wantMaxReadLevel, w.getMaxReadLevel())
			assert.Equal(t, tc.ackLevel, w.taskAckManager.GetAckLevel())
			assert.Equal(t, tc.ackLevel, w.taskAckManager.GetReadLevel())
		})
	}
}

func TestClaimFails(t *testing.T) {
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(0)).Return(nil, assert.AnError)
	w := newTestTaskWriter(t, store)

	assert.ErrorIs(t, w.claim(), assert.AnError)
	assert.Equal(t, int64(0), w.getMaxReadLevel())
}

func TestAppendTask(t *testing.T) {
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	w := startTestTaskWriter(t, store, 100)
	gomock.InOrder(
		store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-a", 1))),
		store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-b", 2))),
	)

	task := waiter("wf-a")
	require.NoError(t, w.appendTask(testContext(t), task))
	assert.Equal(t, waiter("wf-a"), task, "the caller's task is not changed")
	require.NoError(t, w.appendTask(testContext(t), waiter("wf-b")))
	assert.Equal(t, int64(2), w.getMaxReadLevel())
}

func TestAllocTaskIDsRenewsWhenBlockRunsOut(t *testing.T) {
	renewed := &persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 2}
	tests := []struct {
		name string
		// renew stubs the renewal claim.
		renew func(store *persistence.MockSemaphoreTaskManager) *gomock.Call
	}{
		{
			name: "renewal moves writes to the next range",
			renew: func(store *persistence.MockSemaphoreTaskManager) *gomock.Call {
				return store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(1)).Return(renewed, nil)
			},
		},
		{
			name: "renewal retries a transient error, then moves writes to the next range",
			renew: func(store *persistence.MockSemaphoreTaskManager) *gomock.Call {
				store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(1)).Return(nil, &persistence.TimeoutError{Msg: "timed out"})
				return store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(1)).Return(renewed, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
			w := startTestTaskWriter(t, store, 2)
			gomock.InOrder(
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-a", 1))),
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-b", 2))),
				tc.renew(store),
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(2, written("wf-c", 3))),
			)

			for _, workflowID := range []string{"wf-a", "wf-b", "wf-c"} {
				require.NoError(t, w.appendTask(testContext(t), waiter(workflowID)))
			}
			assert.Equal(t, int64(3), w.getMaxReadLevel())
			assert.False(t, isClosed(w.fatal()))
		})
	}
}

func TestTaskWriterFailures(t *testing.T) {
	conditionFailed := &persistence.ConditionFailedError{Msg: "range_id changed"}
	timedOut := &persistence.TimeoutError{Msg: "timed out"}
	tests := []struct {
		name string
		// rangeSize 1 makes the second append renew.
		rangeSize int64
		// fail sets up the second append to fail with wantErr.
		fail      func(store *persistence.MockSemaphoreTaskManager)
		wantErr   error
		wantFatal bool
		// wantStopped is whether the writer stops itself, so later appends get ErrNotReady.
		wantStopped bool
	}{
		{
			name:      "write fails with ConditionFailedError, so the bucket is lost",
			rangeSize: 100,
			fail: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-b", 2))).Return(nil, conditionFailed)
			},
			wantErr:   conditionFailed,
			wantFatal: true,
		},
		{
			name:      "write fails with another error, so the bucket stays loaded",
			rangeSize: 100,
			fail: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-b", 2))).Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
		{
			name:      "renewal fails with a non-transient error, so the writer stops",
			rangeSize: 1,
			fail: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(1)).Return(nil, assert.AnError)
			},
			wantErr:     assert.AnError,
			wantFatal:   true,
			wantStopped: true,
		},
		{
			// The retry's error is not returned: ThrottleRetry prefers the earlier one.
			name:      "renewal times out, then its retry fails with ConditionFailedError",
			rangeSize: 1,
			fail: func(store *persistence.MockSemaphoreTaskManager) {
				gomock.InOrder(
					store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(1)).Return(nil, timedOut),
					store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), claimRequest(1)).Return(nil, conditionFailed),
				)
			},
			wantErr:     timedOut,
			wantFatal:   true,
			wantStopped: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
			w := startTestTaskWriter(t, store, tc.rangeSize)
			store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-a", 1)))
			require.NoError(t, w.appendTask(testContext(t), waiter("wf-a")))
			tc.fail(store)

			assert.ErrorIs(t, w.appendTask(testContext(t), waiter("wf-b")), tc.wantErr)
			assert.Equal(t, int64(1), w.getMaxReadLevel(), "a failed write is not readable")
			assert.Equal(t, tc.wantFatal, isClosed(w.fatal()))
			if tc.wantStopped {
				// The mock fails the test on another claim.
				assert.ErrorIs(t, w.appendTask(testContext(t), waiter("wf-c")), ErrNotReady)
			}
		})
	}
}

func TestTaskWriterShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	w := startTestTaskWriter(t, store, 100)
	writing, finishWrite := make(chan struct{}), make(chan struct{})
	store.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *persistence.CreateSemaphoreTasksRequest) (*persistence.CreateSemaphoreTasksResponse, error) {
			close(writing)
			<-finishWrite
			return &persistence.CreateSemaphoreTasksResponse{}, nil
		})

	appended := make(chan error, 1)
	go func() { appended <- w.appendTask(testContext(t), waiter("wf-a")) }()
	<-writing
	w.stop()

	assert.ErrorIs(t, <-appended, ErrNotReady)
	assert.ErrorIs(t, w.appendTask(testContext(t), waiter("wf-b")), ErrNotReady)
	close(finishWrite)
}

func TestAppendTaskCallerGivesUp(t *testing.T) {
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	w := startTestTaskWriter(t, store, 100)
	writing, finishWrite := make(chan struct{}), make(chan struct{})
	store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-a", 1))).DoAndReturn(
		func(context.Context, *persistence.CreateSemaphoreTasksRequest) (*persistence.CreateSemaphoreTasksResponse, error) {
			close(writing)
			<-finishWrite
			return &persistence.CreateSemaphoreTasksResponse{}, nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	appended := make(chan error, 1)
	go func() { appended <- w.appendTask(ctx, waiter("wf-a")) }()
	<-writing
	cancel()
	assert.ErrorIs(t, <-appended, context.Canceled)

	close(finishWrite)
	require.Eventually(t, func() bool { return w.getMaxReadLevel() == 1 }, time.Second, time.Millisecond,
		"the write is not canceled with the caller")
}
