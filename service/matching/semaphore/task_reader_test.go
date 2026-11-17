package semaphore

import (
	"context"
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

// newTestTaskReader returns a reader whose writer claimed range 1 with ackLevel. It is stopped,
// possibly saving the ack level and deleting finished tasks, when the test ends.
func newTestTaskReader(t *testing.T, store *persistence.MockSemaphoreTaskManager, ackLevel int64, timeSource clock.TimeSource) *taskReader {
	t.Helper()
	expectClaim(store, 0, 1, ackLevel)
	m := newTestQueueManager(t, store, timeSource)
	m.taskWriter = newTaskWriter(m)
	t.Cleanup(m.taskWriter.stop)
	require.NoError(t, m.taskWriter.claim())
	r := newTaskReader(m)
	t.Cleanup(func() {
		store.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).MaxTimes(1).
			Return(&persistence.UpdateSemaphoreTaskBucketStateResponse{}, nil)
		store.EXPECT().RangeCompleteSemaphoreTasks(gomock.Any(), gomock.Any()).MaxTimes(1).
			Return(&persistence.RangeCompleteSemaphoreTasksResponse{}, nil)
		r.stop()
	})
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
			name:         "returns the first window's tasks without reading further",
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
		// readFirst, if set, is a task already read and not yet acked.
		readFirst int64
		// buffered is how many tasks are already in the buffer.
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
			name:         "buffers and registers tasks, reading only into free room",
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
			name:         "buffers a task past its deadline too, for the grant loop to settle",
			maxReadLevel: maxReadLevel,
			read: func(store *persistence.MockSemaphoreTaskManager) {
				store.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).
					Return(&persistence.GetSemaphoreTasksResponse{Tasks: []*persistence.SemaphoreTask{
						withDeadline(11, future), withDeadline(12, past),
					}}, nil)
			},
			wantBuffered:  []*persistence.SemaphoreTask{withDeadline(11, future), withDeadline(12, past)},
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
			name:         "empty read keeps the ack level below an unacked task",
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
			name:         "task read out of order stops the reader",
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

			// The prefilled tasks come first, then what this pass read.
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

// Tests that stop saves the ack level, then deletes the finished tasks.
func TestStopDeletesFinishedTasks(t *testing.T) {
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	r := newTestTaskReader(t, store, 0, clock.NewMockedTimeSource())
	require.NoError(t, r.taskAckManager.ReadItem(1))
	r.taskAckManager.AckItem(1)

	gomock.InOrder(
		store.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).Times(1).
			Return(&persistence.UpdateSemaphoreTaskBucketStateResponse{}, nil),
		store.EXPECT().RangeCompleteSemaphoreTasks(gomock.Any(), &persistence.RangeCompleteSemaphoreTasksRequest{
			DomainID:      testBucketID.DomainID,
			SemaphoreName: testBucketID.SemaphoreName,
			Bucket:        testBucketID.Bucket,
			AckLevel:      1,
		}).Times(1).Return(&persistence.RangeCompleteSemaphoreTasksResponse{}, nil),
	)
	r.stop()
}

// Tests that a task counts from the moment the writer saves it until it is acked.
func TestHasTasks(t *testing.T) {
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	r := newTestTaskReader(t, store, 0, clock.NewMockedTimeSource())
	r.writer.start()
	assert.False(t, r.hasTasks(), "nothing saved yet")

	store.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, written("wf-a", 1)))
	require.NoError(t, r.writer.appendTask(testContext(t), newTask("wf-a")))
	assert.True(t, r.hasTasks(), "saved, not read yet")

	store.EXPECT().GetSemaphoreTasks(gomock.Any(), getTasksRequest(0, 1, getTasksBatchSize)).Return(
		&persistence.GetSemaphoreTasksResponse{Tasks: []*persistence.SemaphoreTask{written("wf-a", 1)}}, nil)
	require.True(t, r.readTasks())
	assert.True(t, r.hasTasks(), "in the buffer")

	task := <-r.taskBuffer
	assert.True(t, r.hasTasks(), "taken from the buffer, not acked yet")

	r.taskAckManager.AckItem(task.TaskID)
	assert.False(t, r.hasTasks(), "acked")
}

func TestGetTasksPumpTimerSavesAndWakesGrantLoop(t *testing.T) {
	store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	mockClock := clock.NewMockedTimeSource()
	r := newTestTaskReader(t, store, 0, mockClock)
	grantLoopWoken := make(chan struct{}, 1)
	r.signalGrantLoop = func() { grantLoopWoken <- struct{}{} }
	store.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).Times(1).
		Return(&persistence.UpdateSemaphoreTaskBucketStateResponse{}, nil)

	r.wg.Add(1)
	go r.getTasksPump()
	mockClock.BlockUntil(1)
	mockClock.Advance(testUpdateAckInterval())
	select {
	case <-grantLoopWoken:
	case <-time.After(time.Second):
		t.Fatal("the timer did not wake the grant loop")
	}
}

func TestGrantTasksLoop(t *testing.T) {
	tests := []struct {
		name             string
		grantTaskReturns map[int64]bool
		wantAckLevel     int64
	}{
		{
			name:             "grants tasks in order, acking each once it is finished",
			grantTaskReturns: map[int64]bool{1: true, 2: true},
			wantAckLevel:     2,
		},
		{
			name:             "stops at a task that is not finished, leaving it unacked",
			grantTaskReturns: map[int64]bool{1: true, 2: false},
			wantAckLevel:     1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer goleak.VerifyNone(t)
			store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
			r := newTestTaskReader(t, store, 0, clock.NewMockedTimeSource())
			granted := make(chan int64, 2)
			r.grantTask = func(_ context.Context, task *persistence.SemaphoreTask) bool {
				granted <- task.TaskID
				return tc.grantTaskReturns[task.TaskID]
			}
			for _, task := range tasksWithIDs(1, 2) {
				require.True(t, r.addTaskToBuffer(task))
			}
			// Acks delete finished tasks.
			store.EXPECT().RangeCompleteSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().
				Return(&persistence.RangeCompleteSemaphoreTasksResponse{}, nil)

			r.wg.Add(1)
			go r.grantTasksLoop()
			assert.Equal(t, []int64{1, 2}, []int64{<-granted, <-granted})
			require.Eventually(t, func() bool { return r.taskAckManager.GetAckLevel() == tc.wantAckLevel }, time.Second, time.Millisecond)
			assert.Len(t, r.notifyC, 1, "taking a task makes room, so the read loop is signaled")
			store.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).Times(1).
				DoAndReturn(func(_ context.Context, req *persistence.UpdateSemaphoreTaskBucketStateRequest) (*persistence.UpdateSemaphoreTaskBucketStateResponse, error) {
					assert.Equal(t, tc.wantAckLevel, req.AckLevel, "stop saves the ack level reached")
					return &persistence.UpdateSemaphoreTaskBucketStateResponse{}, nil
				})
			r.stop()
			assert.Equal(t, tc.wantAckLevel, r.taskAckManager.GetAckLevel())
		})
	}
}
