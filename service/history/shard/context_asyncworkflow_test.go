package shard

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
)

func newEnqueueRequest(shardID int) *persistence.EnqueueAsyncWorkflowMessageRequest {
	return &persistence.EnqueueAsyncWorkflowMessageRequest{
		ShardID:         shardID,
		SourceCluster:   "cluster0",
		DomainName:      "domain",
		WorkflowID:      "workflow",
		RequestID:       "request",
		RequestType:     persistence.AsyncWorkflowRequestTypeStartWorkflow,
		Payload:         []byte("payload"),
		PayloadEncoding: constants.EncodingTypeThriftRW,
	}
}

func TestContext_EnqueueAsyncWorkflowMessage(t *testing.T) {
	errManager := errors.New("manager failed")
	tests := []struct {
		name       string
		closeShard bool
		managerErr error
		wantCall   bool
		wantErr    bool
	}{
		{name: "row written with the allocated id", wantCall: true},
		{name: "manager error is returned unchanged", managerErr: errManager, wantCall: true, wantErr: true},
		{name: "closed shard is rejected before allocating an id", closeShard: true, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			s := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
			t.Cleanup(func() { s.Finish(t) })

			manager := persistence.NewMockAsyncWorkflowQueueManager(ctrl)
			s.Resource.PersistenceBean.EXPECT().GetAsyncWorkflowQueueManager().Return(manager).AnyTimes()
			var persisted *persistence.EnqueueAsyncWorkflowMessageRequest
			if tc.wantCall {
				manager.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.EnqueueAsyncWorkflowMessageRequest) error {
						persisted = req
						return tc.managerErr
					}).Times(1)
			}
			if tc.closeShard {
				s.closeCallback = func(int, *historyShardsItem) {}
				s.closeShard()
			}

			req := newEnqueueRequest(s.GetShardID())
			id, err := s.EnqueueAsyncWorkflowMessage(context.Background(), req)
			if tc.wantErr {
				require.Error(t, err)
				if tc.managerErr != nil {
					require.ErrorIs(t, err, tc.managerErr)
				}
				assert.Zero(t, id)
				return
			}
			require.NoError(t, err)
			require.Greater(t, id, int64(0))
			require.NotNil(t, persisted)
			assert.Equal(t, id, persisted.MessageID, "the persisted row carries the returned id")
			assert.Equal(t, id, req.MessageID, "request.MessageID is overwritten with the allocated id")
		})
	}
}

// TestContext_EnqueueAsyncWorkflowMessage_ClosedDuringAllocation proves the closure re-check runs under the
// shard lock after the id is allocated. The task-id range is exhausted so allocation must renew it through
// ShardManager.UpdateShard; the mocked UpdateShard closes the shard before returning success. Allocation
// then succeeds, the re-check must reject, and the queue manager must never be called.
func TestContext_EnqueueAsyncWorkflowMessage_ClosedDuringAllocation(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
	t.Cleanup(func() { s.Finish(t) })

	manager := persistence.NewMockAsyncWorkflowQueueManager(ctrl)
	s.Resource.PersistenceBean.EXPECT().GetAsyncWorkflowQueueManager().Return(manager).AnyTimes()
	s.closeCallback = func(int, *historyShardsItem) {}

	s.Lock()
	s.taskSequenceNumber = s.maxTaskSequenceNumber
	s.Unlock()
	s.Resource.ShardMgr.On("UpdateShard", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
		s.closeShard()
	}).Return(nil).Once()

	id, err := s.EnqueueAsyncWorkflowMessage(context.Background(), newEnqueueRequest(s.GetShardID()))

	var closed *ErrShardClosed
	require.ErrorAs(t, err, &closed)
	assert.Zero(t, id)
}

// TestContext_EnqueueAsyncWorkflowMessage_ShardLockFreeDuringWrite proves the shard lock is not held while
// the queue manager writes: the manager mock takes and releases the shard lock itself, which would deadlock
// if the enqueue path still held it.
func TestContext_EnqueueAsyncWorkflowMessage_ShardLockFreeDuringWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
	t.Cleanup(func() { s.Finish(t) })

	manager := persistence.NewMockAsyncWorkflowQueueManager(ctrl)
	s.Resource.PersistenceBean.EXPECT().GetAsyncWorkflowQueueManager().Return(manager).AnyTimes()
	manager.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *persistence.EnqueueAsyncWorkflowMessageRequest) error {
			locked := make(chan struct{})
			go func() {
				s.Lock()
				s.Unlock()
				close(locked)
			}()
			select {
			case <-locked:
				return nil
			case <-time.After(5 * time.Second):
				return errors.New("shard lock is held during the queue write")
			}
		}).Times(1)

	id, err := s.EnqueueAsyncWorkflowMessage(context.Background(), newEnqueueRequest(s.GetShardID()))
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))
}

// TestContext_EnqueueAsyncWorkflowMessage_OrderedUnderEnqueueLock proves the per-shard enqueue lock
// keeps message ids in allocation order: with many concurrent enqueues, the manager must observe ids in
// strictly increasing order. The mock yields between reading the id and returning so an implementation
// that released the enqueue lock between allocation and write would interleave.
func TestContext_EnqueueAsyncWorkflowMessage_OrderedUnderEnqueueLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
	t.Cleanup(func() { s.Finish(t) })

	const goroutines = 64
	var (
		mu       sync.Mutex
		observed []int64
	)
	manager := persistence.NewMockAsyncWorkflowQueueManager(ctrl)
	s.Resource.PersistenceBean.EXPECT().GetAsyncWorkflowQueueManager().Return(manager).AnyTimes()
	manager.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.EnqueueAsyncWorkflowMessageRequest) error {
			id := req.MessageID
			for i := 0; i < 10; i++ {
				runtime.Gosched()
			}
			mu.Lock()
			observed = append(observed, id)
			mu.Unlock()
			return nil
		}).Times(goroutines)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.EnqueueAsyncWorkflowMessage(context.Background(), newEnqueueRequest(s.GetShardID()))
			assert.NoError(t, err)
		}()
	}
	close(start)
	wg.Wait()

	require.Len(t, observed, goroutines)
	for i := 1; i < len(observed); i++ {
		require.Greater(t, observed[i], observed[i-1], "message ids must reach the store in allocation order (index %d)", i)
	}
}
