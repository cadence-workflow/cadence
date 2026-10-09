package shard

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestContext_EnqueueAsyncWorkflowMessage_ClosedWhileWaitingForLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
	t.Cleanup(func() { s.Finish(t) })

	manager := persistence.NewMockAsyncWorkflowQueueManager(ctrl)
	s.Resource.PersistenceBean.EXPECT().GetAsyncWorkflowQueueManager().Return(manager).AnyTimes()
	s.closeCallback = func(int, *historyShardsItem) {}

	var (
		id  int64
		err error
	)
	started := make(chan struct{})
	done := make(chan struct{})
	s.Lock()
	go func() {
		defer close(done)
		close(started)
		id, err = s.EnqueueAsyncWorkflowMessage(context.Background(), newEnqueueRequest(s.GetShardID()))
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		s.Unlock()
		t.Fatal("enqueue did not start")
	}
	// Give the goroutine a chance to pass the pre-lock closure check and block on Lock().
	runtime.Gosched()
	s.closeShard()
	s.Unlock()
	<-done

	require.Error(t, err)
	assert.Zero(t, id)
}

// TestContext_EnqueueAsyncWorkflowMessage_OrderedUnderLock proves the row is written while the shard
// lock is still held: with many concurrent enqueues, the manager must observe message ids in strictly
// increasing order. The manager mock yields between reading the id and returning so that an
// implementation which releases the lock between allocation and write would interleave.
func TestContext_EnqueueAsyncWorkflowMessage_OrderedUnderLock(t *testing.T) {
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
