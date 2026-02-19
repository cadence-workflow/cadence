package spectatorclient

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
	csync "github.com/uber/cadence/service/sharddistributor/client/spectatorclient/sync"
)

func TestWatchLoopBasicFlow(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockStream := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)

	// Create a context to control when the mock stream should unblock
	streamCtx, cancelStream := context.WithCancel(context.Background())

	spectator := &spectatorImpl{
		namespace:        "test-ns",
		client:           mockClient,
		logger:           log.NewNoop(),
		scope:            tally.NoopScope,
		timeSource:       clock.NewRealTimeSource(),
		firstStateSignal: csync.NewResettableSignal(),
		enabled:          func() bool { return true },
	}

	// Expect stream creation
	mockClient.EXPECT().
		WatchNamespaceState(gomock.Any(), &types.WatchNamespaceStateRequest{Namespace: "test-ns"}).
		Return(mockStream, nil)

	// First Recv returns state
	mockStream.EXPECT().Recv().Return(&types.WatchNamespaceStateResponse{
		Executors: []*types.ExecutorShardAssignment{
			{
				ExecutorID: "executor-1",
				Metadata: map[string]string{
					"grpc_address": "127.0.0.1:7953",
				},
				AssignedShards: []*types.Shard{
					{ShardKey: "shard-1"},
					{ShardKey: "shard-2"},
				},
			},
		},
	}, nil)

	// Second Recv blocks until shutdown
	mockStream.EXPECT().Recv().DoAndReturn(func(...interface{}) (*types.WatchNamespaceStateResponse, error) {
		// Wait for context to be done
		<-streamCtx.Done()
		return nil, streamCtx.Err()
	})

	mockStream.EXPECT().CloseSend().Return(nil)

	ctx := context.Background()
	err := spectator.Start(ctx)
	require.NoError(t, err)
	defer func() {
		cancelStream()
		spectator.Stop()
	}()

	// Wait for first state
	require.NoError(t, spectator.firstStateSignal.Wait(context.Background()))

	// Query shard owner
	owner, err := spectator.GetShardOwner(context.Background(), "shard-1")
	assert.NoError(t, err)
	assert.Equal(t, "executor-1", owner.ExecutorID)
	assert.Equal(t, "127.0.0.1:7953", owner.Metadata["grpc_address"])

	owner, err = spectator.GetShardOwner(context.Background(), "shard-2")
	assert.NoError(t, err)
	assert.Equal(t, "executor-1", owner.ExecutorID)
}

func TestGetShardOwner_CacheMiss_FallbackToRPC(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockStream := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)

	// Create a context to control when the mock stream should unblock
	streamCtx, cancelStream := context.WithCancel(context.Background())

	spectator := &spectatorImpl{
		namespace:        "test-ns",
		client:           mockClient,
		logger:           log.NewNoop(),
		scope:            tally.NoopScope,
		timeSource:       clock.NewRealTimeSource(),
		firstStateSignal: csync.NewResettableSignal(),
		enabled:          func() bool { return true },
	}

	// Setup stream
	mockClient.EXPECT().
		WatchNamespaceState(gomock.Any(), gomock.Any()).
		Return(mockStream, nil)

	// First Recv returns state
	mockStream.EXPECT().Recv().Return(&types.WatchNamespaceStateResponse{
		Executors: []*types.ExecutorShardAssignment{
			{
				ExecutorID: "executor-1",
				Metadata: map[string]string{
					"grpc_address": "127.0.0.1:7953",
				},
				AssignedShards: []*types.Shard{{ShardKey: "shard-1"}},
			},
		},
	}, nil)

	// Second Recv blocks until shutdown
	mockStream.EXPECT().Recv().AnyTimes().DoAndReturn(func(...interface{}) (*types.WatchNamespaceStateResponse, error) {
		// Wait for context to be done
		<-streamCtx.Done()
		return nil, streamCtx.Err()
	})

	mockStream.EXPECT().CloseSend().Return(nil)

	// Expect RPC fallback for unknown shard
	mockClient.EXPECT().
		GetShardOwner(gomock.Any(), &types.GetShardOwnerRequest{
			Namespace: "test-ns",
			ShardKey:  "unknown-shard",
		}).
		Return(&types.GetShardOwnerResponse{
			Owner: "executor-2",
			Metadata: map[string]string{
				"grpc_address": "127.0.0.1:7954",
			},
		}, nil)

	spectator.Start(context.Background())
	defer func() {
		cancelStream()
		spectator.Stop()
	}()

	require.NoError(t, spectator.firstStateSignal.Wait(context.Background()))

	// Cache hit
	owner, err := spectator.GetShardOwner(context.Background(), "shard-1")
	assert.NoError(t, err)
	assert.Equal(t, "executor-1", owner.ExecutorID)

	// Cache miss - should trigger RPC
	owner, err = spectator.GetShardOwner(context.Background(), "unknown-shard")
	assert.NoError(t, err)
	assert.Equal(t, "executor-2", owner.ExecutorID)
	assert.Equal(t, "127.0.0.1:7954", owner.Metadata["grpc_address"])
}

func TestStreamReconnection(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockStream1 := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)
	mockStream2 := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()

	// Create a context to control when the mock stream should unblock
	streamCtx, cancelStream := context.WithCancel(context.Background())

	spectator := &spectatorImpl{
		namespace:        "test-ns",
		client:           mockClient,
		logger:           log.NewNoop(),
		scope:            tally.NoopScope,
		timeSource:       mockTimeSource,
		firstStateSignal: csync.NewResettableSignal(),
		enabled:          func() bool { return true },
	}

	// First stream fails immediately
	mockClient.EXPECT().
		WatchNamespaceState(gomock.Any(), gomock.Any()).
		Return(mockStream1, nil)

	mockStream1.EXPECT().Recv().Return(nil, errors.New("network error"))
	mockStream1.EXPECT().CloseSend().Return(nil)

	// Second stream succeeds
	mockClient.EXPECT().
		WatchNamespaceState(gomock.Any(), gomock.Any()).
		Return(mockStream2, nil)

	// First Recv returns state
	mockStream2.EXPECT().Recv().Return(&types.WatchNamespaceStateResponse{
		Executors: []*types.ExecutorShardAssignment{{ExecutorID: "executor-1"}},
	}, nil)

	// Second Recv blocks until shutdown
	mockStream2.EXPECT().Recv().AnyTimes().DoAndReturn(func(...interface{}) (*types.WatchNamespaceStateResponse, error) {
		// Wait for context to be done
		<-streamCtx.Done()
		return nil, errors.New("shutdown")
	})

	mockStream2.EXPECT().CloseSend().Return(nil)

	spectator.Start(context.Background())
	defer func() {
		cancelStream()
		spectator.Stop()
	}()

	// Wait for the goroutine to be blocked in Sleep, then advance time
	mockTimeSource.BlockUntil(1) // Wait for 1 goroutine to be blocked in Sleep
	mockTimeSource.Advance(2 * time.Second)

	require.NoError(t, spectator.firstStateSignal.Wait(context.Background()))
}

func TestGetShardOwner_TimeoutBeforeFirstState(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)

	spectator := &spectatorImpl{
		namespace:        "test-ns",
		client:           mockClient,
		logger:           log.NewNoop(),
		scope:            tally.NoopScope,
		timeSource:       clock.NewRealTimeSource(),
		firstStateSignal: csync.NewResettableSignal(),
		enabled:          func() bool { return true },
	}

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Try to get shard owner before first state is received
	// Should timeout and return an error
	_, err := spectator.GetShardOwner(ctx, "shard-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wait for first state")
}

func TestWatchLoopDisabled(t *testing.T) {
	t.Skip("Skipping test due to flakiness")
	defer goleak.VerifyNone(t)

	stateSignal := csync.NewResettableSignal()
	timeSource := clock.NewMockedTimeSource()

	spectator := &spectatorImpl{
		firstStateSignal: stateSignal,
		timeSource:       timeSource,
		logger:           log.NewNoop(),
		enabled:          func() bool { return false },
	}

	// Start goroutine that waits on the signal BEFORE starting the spectator
	// This ensures it will observe the reset that happens when entering disabledState
	wg := sync.WaitGroup{}
	wg.Add(1)
	waitStarted := make(chan struct{})
	go func() {
		defer wg.Done()
		close(waitStarted)
		// First wait will observe the reset that happens when entering disabledState
		err := stateSignal.Wait(context.Background())
		assert.ErrorIs(t, err, csync.ErrReset)

		// Second wait will be unblocked when Stop() calls Done()
		err = stateSignal.Wait(context.Background())
		assert.NoError(t, err)
	}()

	// Ensure the goroutine has started waiting
	<-waitStarted
	time.Sleep(100 * time.Millisecond)

	// Now start the spectator, which will enter disabledState and call Reset()
	spectator.Start(context.Background())
	defer spectator.Stop()

	// Wait for the spectator to enter disabledState and be in its first sleep
	timeSource.BlockUntil(1)

	// Advance time to let the sleep in disabledState complete
	timeSource.Advance(1200 * time.Millisecond)

	// Wait for the next sleep iteration
	timeSource.BlockUntil(1)

	// Advance time again
	timeSource.Advance(1200 * time.Millisecond)

	// Stop the spectator to unblock any waiting goroutines
	spectator.Stop()

	wg.Wait()
}
