package semaphore

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
)

// testUpdateAckInterval is how often the readers in these tests save the ack level.
func testUpdateAckInterval() time.Duration { return time.Minute }

// newTestTaskReader returns a reader whose writer claimed range 1 with ackLevel, stopped when the
// test ends.
func newTestTaskReader(t *testing.T, store *persistence.MockSemaphoreTaskManager, ackLevel int64, timeSource clock.TimeSource) *taskReader {
	t.Helper()
	expectClaim(store, 0, 1, ackLevel)
	m := newTestQueueManager(t, store, timeSource)
	m.taskWriter = newTaskWriter(m)
	t.Cleanup(m.taskWriter.stop)
	require.NoError(t, m.taskWriter.claim())
	r := newTaskReader(m)
	t.Cleanup(r.stop)
	return r
}

// getTasksRequest is the read of (readLevel, maxReadLevel].
func getTasksRequest(readLevel, maxReadLevel int64, batchSize int) *persistence.GetSemaphoreTasksRequest {
	return &persistence.GetSemaphoreTasksRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		ReadLevel:     readLevel,
		MaxReadLevel:  maxReadLevel,
		BatchSize:     batchSize,
	}
}

// tasksWithIDs returns tasks with the given task ids.
func tasksWithIDs(taskIDs ...int64) []*persistence.SemaphoreTask {
	tasks := make([]*persistence.SemaphoreTask, len(taskIDs))
	for i, taskID := range taskIDs {
		tasks[i] = &persistence.SemaphoreTask{TaskID: taskID}
	}
	return tasks
}

func TestGetTaskBatch(t *testing.T) {
	const windowSize, batchSize = 10, 5
	tests := []struct {
		name                    string
		readLevel, maxReadLevel int64
		// reads stubs the store, in order. Nil expects no read.
		reads         func(store *persistence.MockSemaphoreTaskManager)
		wantTasks     []*persistence.SemaphoreTask
		wantReadLevel int64
		wantDone      bool
	}{
		{
			name:          "read level already at maxReadLevel",
			readLevel:     20,
			maxReadLevel:  20,
			wantReadLevel: 20,
			wantDone:      true,
		},
		{
			name:         "returns the first window's waiters without reading further",
			maxReadLevel: 35,
			reads: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(0, 10, batchSize)).
					Return(&persistence.GetSemaphoreTasksResponse{Tasks: tasksWithIDs(3, 5)}, nil)
			},
			wantTasks:     tasksWithIDs(3, 5),
			wantReadLevel: 10,
			wantDone:      true,
		},
		{
			name:         "skips an empty window",
			maxReadLevel: 35,
			reads: func(store *persistence.MockSemaphoreTaskManager) {
				gomock.InOrder(
					store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(0, 10, batchSize)).
						Return(&persistence.GetSemaphoreTasksResponse{}, nil),
					store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(10, 20, batchSize)).
						Return(&persistence.GetSemaphoreTasksResponse{Tasks: tasksWithIDs(15)}, nil),
				)
			},
			wantTasks:     tasksWithIDs(15),
			wantReadLevel: 20,
			wantDone:      true,
		},
		{
			name:         "last window ends at maxReadLevel",
			maxReadLevel: 15,
			reads: func(store *persistence.MockSemaphoreTaskManager) {
				gomock.InOrder(
					store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(0, 10, batchSize)).
						Return(&persistence.GetSemaphoreTasksResponse{}, nil),
					store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(10, 15, batchSize)).
						Return(&persistence.GetSemaphoreTasksResponse{}, nil),
				)
			},
			wantReadLevel: 15,
			wantDone:      true,
		},
		{
			name:         "stops after maxReadWindows empty windows",
			maxReadLevel: 1000,
			reads: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).Times(maxReadWindows).
					Return(&persistence.GetSemaphoreTasksResponse{}, nil)
			},
			wantReadLevel: maxReadWindows * windowSize,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
			r := newTestTaskReader(t, store, 0, clock.NewMockedTimeSource())
			r.readRangeSize = windowSize
			if tc.reads != nil {
				tc.reads(store)
			}

			tasks, readLevel, done, err := r.getTaskBatch(tc.readLevel, tc.maxReadLevel, batchSize)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantTasks, tasks)
			assert.Equal(t, tc.wantReadLevel, readLevel)
			assert.Equal(t, tc.wantDone, done)
		})
	}
}

func TestReadTasks(t *testing.T) {
	const ackLevel, maxReadLevel = 10, 20
	now := time.Unix(1_000, 0)
	past, future := now.Add(-time.Second), now.Add(time.Second)
	withDeadline := func(taskID int64, deadline time.Time) *persistence.SemaphoreTask {
		return &persistence.SemaphoreTask{TaskID: taskID, AcquireDeadline: &deadline}
	}
	tests := []struct {
		name         string
		maxReadLevel int64
		// readFirst, if set, is a waiter already read and not yet acked.
		readFirst int64
		// buffered is how many waiters are already in the buffer.
		buffered int
		// read stubs the store. Nil expects no read.
		read          func(store *persistence.MockSemaphoreTaskManager)
		wantBuffered  []*persistence.SemaphoreTask
		wantReadLevel int64
		wantAckLevel  int64
		// wantSignal is whether the pass asks for another one.
		wantSignal bool
		// wantStop is whether the pass stops the reader and reports losing the bucket.
		wantStop bool
	}{
		{
			name:         "buffers and registers waiters, reading only into free room",
			maxReadLevel: maxReadLevel,
			buffered:     getTasksBatchSize - 2,
			read: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(ackLevel, maxReadLevel, 2)).
					Return(&persistence.GetSemaphoreTasksResponse{Tasks: tasksWithIDs(11, 12)}, nil)
			},
			wantBuffered:  tasksWithIDs(11, 12),
			wantReadLevel: 12,
			wantAckLevel:  ackLevel,
			wantSignal:    true,
		},
		{
			name:         "skips a waiter past its deadline",
			maxReadLevel: maxReadLevel,
			read: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).
					Return(&persistence.GetSemaphoreTasksResponse{Tasks: []*persistence.SemaphoreTask{
						withDeadline(11, future), withDeadline(12, past),
					}}, nil)
			},
			wantBuffered:  []*persistence.SemaphoreTask{withDeadline(11, future)},
			wantReadLevel: 12,
			wantAckLevel:  ackLevel,
			wantSignal:    true,
		},
		{
			name:         "empty read moves the ack level and signals to read the rest",
			maxReadLevel: ackLevel + (maxReadWindows+1)*readRangeSize,
			read: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).Times(maxReadWindows).
					Return(&persistence.GetSemaphoreTasksResponse{}, nil)
			},
			wantReadLevel: ackLevel + maxReadWindows*readRangeSize,
			wantAckLevel:  ackLevel + maxReadWindows*readRangeSize,
			wantSignal:    true,
		},
		{
			name:         "empty read keeps the ack level below an unacked waiter",
			maxReadLevel: maxReadLevel,
			readFirst:    11,
			read: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(11, maxReadLevel, getTasksBatchSize)).
					Return(&persistence.GetSemaphoreTasksResponse{}, nil)
			},
			wantReadLevel: maxReadLevel,
			wantAckLevel:  ackLevel,
		},
		{
			name:         "waiter read out of order stops the reader",
			maxReadLevel: maxReadLevel,
			readFirst:    15,
			read: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(15, maxReadLevel, getTasksBatchSize)).
					Return(&persistence.GetSemaphoreTasksResponse{Tasks: tasksWithIDs(12)}, nil)
			},
			wantReadLevel: 15,
			wantAckLevel:  ackLevel,
			wantStop:      true,
		},
		{
			name:          "full buffer skips the read",
			maxReadLevel:  maxReadLevel,
			buffered:      getTasksBatchSize,
			wantReadLevel: ackLevel,
			wantAckLevel:  ackLevel,
		},
		{
			name:         "failed read signals to try again",
			maxReadLevel: maxReadLevel,
			read: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantReadLevel: ackLevel,
			wantAckLevel:  ackLevel,
			wantSignal:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
			mockClock := clock.NewMockedTimeSourceAt(now)
			r := newTestTaskReader(t, store, ackLevel, mockClock)
			atomic.StoreInt64(&r.writer.maxReadLevel, tc.maxReadLevel)
			if tc.readFirst != 0 {
				require.NoError(t, r.taskAckManager.ReadItem(tc.readFirst))
			}
			for range tc.buffered {
				r.taskBuffer <- &persistence.SemaphoreTask{}
			}
			if tc.read != nil {
				tc.read(store)
			}

			assert.Equal(t, !tc.wantStop, r.readTasks())

			// The prefilled waiters come first; what follows is what this pass read.
			for range tc.buffered {
				<-r.taskBuffer
			}
			var buffered []*persistence.SemaphoreTask
			for len(r.taskBuffer) > 0 {
				buffered = append(buffered, <-r.taskBuffer)
			}
			assert.Equal(t, tc.wantBuffered, buffered)
			assert.Equal(t, tc.wantReadLevel, r.taskAckManager.GetReadLevel())
			assert.Equal(t, tc.wantAckLevel, r.taskAckManager.GetAckLevel())
			assert.Equal(t, tc.wantSignal, len(r.notifyC) == 1)
			assert.Equal(t, tc.wantStop, isClosed(r.fatal()))
		})
	}
}

func TestGetTasksPumpConditionFailedReportsFatal(t *testing.T) {
	defer goleak.VerifyNone(t)
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	mockClock := clock.NewMockedTimeSource()
	r := newTestTaskReader(t, store, 0, mockClock)
	store.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).
		Return(nil, &persistence.ConditionFailedError{})
	r.start()

	mockClock.BlockUntil(1)
	mockClock.Advance(testUpdateAckInterval())
	select {
	case <-r.fatal():
	case <-time.After(time.Second):
		t.Fatal("losing the bucket was not reported")
	}
	r.stop()
}

func TestGetTasksPumpReadsWhenSignaled(t *testing.T) {
	defer goleak.VerifyNone(t)
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	r := newTestTaskReader(t, store, 0, clock.NewMockedTimeSource())
	gomock.InOrder(
		store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(0, 2, getTasksBatchSize)).
			Return(&persistence.GetSemaphoreTasksResponse{Tasks: tasksWithIDs(1, 2)}, nil),
		// Two waiters are still in the buffer, so the read asks for the room left.
		store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(2, 3, getTasksBatchSize-2)).
			Return(&persistence.GetSemaphoreTasksResponse{Tasks: tasksWithIDs(3)}, nil),
	)

	atomic.StoreInt64(&r.writer.maxReadLevel, 2)
	r.start()
	require.Eventually(t, func() bool { return len(r.taskBuffer) == 2 }, time.Second, time.Millisecond,
		"starting did not read the waiters already written")

	atomic.StoreInt64(&r.writer.maxReadLevel, 3)
	r.signal()
	require.Eventually(t, func() bool { return len(r.taskBuffer) == 3 }, time.Second, time.Millisecond,
		"a signal did not read the new waiter")
	r.stop()
	assert.Equal(t, tasksWithIDs(1, 2, 3), []*persistence.SemaphoreTask{<-r.taskBuffer, <-r.taskBuffer, <-r.taskBuffer})
}
