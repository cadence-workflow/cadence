package spectatorclient

import (
	"context"
	"errors"
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
	defer goleak.VerifyNone(t)

	stateSignal := csync.NewResettableSignal()
	timeSource := clock.NewMockedTimeSource()

	spectator := &spectatorImpl{
		firstStateSignal: stateSignal,
		timeSource:       timeSource,
		logger:           log.NewNoop(),
		enabled:          func() bool { return false },
	}

	err := spectator.Start(context.Background())
	assert.NoError(t, err)

	// Disabled state enters a sleep loop, verify it sleeps periodically
	timeSource.BlockUntil(1)
	timeSource.Advance(1200 * time.Millisecond)

	timeSource.BlockUntil(1)
	timeSource.Advance(1200 * time.Millisecond)

	// Stop exits cleanly and calls Done() on the signal
	spectator.Stop()

	// After Stop(), Done() has been called so Wait returns nil
	err = stateSignal.Wait(context.Background())
	assert.NoError(t, err)
}

func TestSubscribe_Success(t *testing.T) {
	spectator := &spectatorImpl{
		namespace:        "test-ns",
		logger:           log.NewNoop(),
		subscribers:      make(map[string]chan<- struct{}),
		firstStateSignal: csync.NewResettableSignal(),
	}

	// Subscribe with a unique name
	ch, err := spectator.Subscribe("subscriber-1")
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Verify subscription was registered
	spectator.subscribersMu.RLock()
	_, exists := spectator.subscribers["subscriber-1"]
	spectator.subscribersMu.RUnlock()
	assert.True(t, exists)
}

func TestSubscribe_DuplicateName(t *testing.T) {
	spectator := &spectatorImpl{
		namespace:        "test-ns",
		logger:           log.NewNoop(),
		subscribers:      make(map[string]chan<- struct{}),
		firstStateSignal: csync.NewResettableSignal(),
	}

	// First subscription succeeds
	_, err := spectator.Subscribe("subscriber-1")
	require.NoError(t, err)

	// Second subscription with same name should fail
	_, err = spectator.Subscribe("subscriber-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subscriber with name \"subscriber-1\" already exists")
}

func TestUnsubscribe(t *testing.T) {
	spectator := &spectatorImpl{
		namespace:        "test-ns",
		logger:           log.NewNoop(),
		subscribers:      make(map[string]chan<- struct{}),
		firstStateSignal: csync.NewResettableSignal(),
	}

	// Subscribe first
	_, err := spectator.Subscribe("subscriber-1")
	require.NoError(t, err)

	// Verify subscription exists
	spectator.subscribersMu.RLock()
	_, exists := spectator.subscribers["subscriber-1"]
	spectator.subscribersMu.RUnlock()
	assert.True(t, exists)

	// Unsubscribe
	spectator.Unsubscribe("subscriber-1")

	// Verify subscription was removed
	spectator.subscribersMu.RLock()
	_, exists = spectator.subscribers["subscriber-1"]
	spectator.subscribersMu.RUnlock()
	assert.False(t, exists)
}

func TestUnsubscribe_NonExistentSubscriber(t *testing.T) {
	spectator := &spectatorImpl{
		namespace:        "test-ns",
		logger:           log.NewNoop(),
		subscribers:      make(map[string]chan<- struct{}),
		firstStateSignal: csync.NewResettableSignal(),
	}

	// Unsubscribe non-existent subscriber should be idempotent (no-op, no error)
	spectator.Unsubscribe("non-existent")

	// Verify no panic occurred and subscribers map is still empty
	spectator.subscribersMu.RLock()
	assert.Len(t, spectator.subscribers, 0)
	spectator.subscribersMu.RUnlock()
}

func TestGetExecutors(t *testing.T) {
	spectator := &spectatorImpl{
		namespace: "test-ns",
		logger:    log.NewNoop(),
		executorToOwner: map[string]*ShardOwner{
			"executor-1": {
				ExecutorID: "executor-1",
				Metadata: map[string]string{
					"grpc_address": "127.0.0.1:7953",
				},
			},
			"executor-2": {
				ExecutorID: "executor-2",
				Metadata: map[string]string{
					"grpc_address": "127.0.0.1:7954",
				},
			},
		},
		firstStateSignal: csync.NewResettableSignal(),
	}

	executors := spectator.GetExecutors()

	assert.Len(t, executors, 2)
	assert.Equal(t, "executor-1", executors["executor-1"].ExecutorID)
	assert.Equal(t, "127.0.0.1:7953", executors["executor-1"].Metadata["grpc_address"])
	assert.Equal(t, "executor-2", executors["executor-2"].ExecutorID)
	assert.Equal(t, "127.0.0.1:7954", executors["executor-2"].Metadata["grpc_address"])
}

func TestNotifySubscribers(t *testing.T) {
	spectator := &spectatorImpl{
		namespace:        "test-ns",
		logger:           log.NewNoop(),
		subscribers:      make(map[string]chan<- struct{}),
		firstStateSignal: csync.NewResettableSignal(),
	}

	// Subscribe multiple subscribers
	ch1, err := spectator.Subscribe("subscriber-1")
	require.NoError(t, err)
	ch2, err := spectator.Subscribe("subscriber-2")
	require.NoError(t, err)

	// Notify subscribers
	spectator.notifySubscribers()

	// Verify both subscribers received notification
	select {
	case <-ch1:
		// Received notification
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber-1 did not receive notification")
	}

	select {
	case <-ch2:
		// Received notification
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber-2 did not receive notification")
	}
}

func TestNotifySubscribers_FullChannel(t *testing.T) {
	spectator := &spectatorImpl{
		namespace:        "test-ns",
		logger:           log.NewNoop(),
		subscribers:      make(map[string]chan<- struct{}),
		firstStateSignal: csync.NewResettableSignal(),
	}

	// Create a channel and manually add it to subscribers to control it
	ch := make(chan struct{}, 1)
	spectator.subscribers["subscriber-1"] = ch

	// Fill the channel buffer
	ch <- struct{}{}

	// Notify should not block even if channel is full
	done := make(chan struct{})
	go func() {
		spectator.notifySubscribers()
		close(done)
	}()

	select {
	case <-done:
		// Notification completed without blocking
	case <-time.After(100 * time.Millisecond):
		t.Fatal("notifySubscribers blocked on full channel")
	}
}

func TestDiffExecutors(t *testing.T) {
	tests := []struct {
		name             string
		currentExecutors map[string]*ShardOwner
		newExecutors     map[string]*ShardOwner
		expectChanged    bool
	}{
		{
			name: "NoChange",
			currentExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
				"executor-2": {ExecutorID: "executor-2"},
			},
			newExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
				"executor-2": {ExecutorID: "executor-2"},
			},
			expectChanged: false,
		},
		{
			name: "ExecutorAdded",
			currentExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
			},
			newExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
				"executor-2": {ExecutorID: "executor-2"},
			},
			expectChanged: true,
		},
		{
			name: "ExecutorRemoved",
			currentExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
				"executor-2": {ExecutorID: "executor-2"},
			},
			newExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
			},
			expectChanged: true,
		},
		{
			name: "DifferentExecutors",
			currentExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
			},
			newExecutors: map[string]*ShardOwner{
				"executor-2": {ExecutorID: "executor-2"},
				"executor-3": {ExecutorID: "executor-3"},
			},
			expectChanged: true,
		},
		{
			name:             "EmptyToPopulated",
			currentExecutors: map[string]*ShardOwner{},
			newExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
			},
			expectChanged: true,
		},
		{
			name: "PopulatedToEmpty",
			currentExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
			},
			newExecutors:  map[string]*ShardOwner{},
			expectChanged: true,
		},
		{
			name:             "BothEmpty",
			currentExecutors: map[string]*ShardOwner{},
			newExecutors:     map[string]*ShardOwner{},
			expectChanged:    false,
		},
		{
			name: "SameSizeDifferentExecutors",
			currentExecutors: map[string]*ShardOwner{
				"executor-1": {ExecutorID: "executor-1"},
				"executor-2": {ExecutorID: "executor-2"},
			},
			newExecutors: map[string]*ShardOwner{
				"executor-3": {ExecutorID: "executor-3"},
				"executor-4": {ExecutorID: "executor-4"},
			},
			expectChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spectator := &spectatorImpl{
				namespace:        "test-ns",
				logger:           log.NewNoop(),
				executorToOwner:  tt.currentExecutors,
				firstStateSignal: csync.NewResettableSignal(),
			}

			changed := spectator.diffExecutors(tt.newExecutors)
			assert.Equal(t, tt.expectChanged, changed)
		})
	}
}

func TestExecutorChangeNotification(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		name               string
		initialExecutors   []*types.ExecutorShardAssignment
		updatedExecutors   []*types.ExecutorShardAssignment
		expectNotification bool
		expectedFinalCount int
	}{
		{
			name: "ExecutorAdded",
			initialExecutors: []*types.ExecutorShardAssignment{
				{
					ExecutorID: "executor-1",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7953"},
				},
			},
			updatedExecutors: []*types.ExecutorShardAssignment{
				{
					ExecutorID: "executor-1",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7953"},
				},
				{
					ExecutorID: "executor-2",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7954"},
				},
			},
			expectNotification: true,
			expectedFinalCount: 2,
		},
		{
			name: "ExecutorRemoved",
			initialExecutors: []*types.ExecutorShardAssignment{
				{
					ExecutorID: "executor-1",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7953"},
				},
				{
					ExecutorID: "executor-2",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7954"},
				},
			},
			updatedExecutors: []*types.ExecutorShardAssignment{
				{
					ExecutorID: "executor-1",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7953"},
				},
			},
			expectNotification: true,
			expectedFinalCount: 1,
		},
		{
			name: "NoChangeInExecutors",
			initialExecutors: []*types.ExecutorShardAssignment{
				{
					ExecutorID: "executor-1",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7953"},
				},
				{
					ExecutorID: "executor-2",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7954"},
				},
			},
			updatedExecutors: []*types.ExecutorShardAssignment{
				{
					ExecutorID: "executor-1",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7953"},
				},
				{
					ExecutorID: "executor-2",
					Metadata:   map[string]string{"grpc_address": "127.0.0.1:7954"},
				},
			},
			expectNotification: false,
			expectedFinalCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := sharddistributor.NewMockClient(ctrl)
			mockStream := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)

			spectator := &spectatorImpl{
				namespace:        "test-ns",
				client:           mockClient,
				logger:           log.NewNoop(),
				scope:            tally.NoopScope,
				timeSource:       clock.NewRealTimeSource(),
				firstStateSignal: csync.NewResettableSignal(),
				subscribers:      make(map[string]chan<- struct{}),
				enabled:          func() bool { return true },
			}

			// Subscribe before starting
			ch, err := spectator.Subscribe("test-subscriber")
			require.NoError(t, err)

			// Setup stream and capture context
			var watchCtx context.Context
			mockClient.EXPECT().
				WatchNamespaceState(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, req *types.WatchNamespaceStateRequest, opts ...interface{}) (sharddistributor.WatchNamespaceStateClient, error) {
					watchCtx = ctx
					return mockStream, nil
				})

			// First Recv returns initial state
			mockStream.EXPECT().Recv().Return(&types.WatchNamespaceStateResponse{
				Executors: tt.initialExecutors,
			}, nil)

			// Second Recv returns updated state
			mockStream.EXPECT().Recv().Return(&types.WatchNamespaceStateResponse{
				Executors: tt.updatedExecutors,
			}, nil)

			// Third Recv blocks until shutdown
			mockStream.EXPECT().Recv().AnyTimes().DoAndReturn(func(...interface{}) (*types.WatchNamespaceStateResponse, error) {
				<-watchCtx.Done()
				return nil, watchCtx.Err()
			})

			mockStream.EXPECT().CloseSend().Return(nil)

			spectator.Start(context.Background())
			defer spectator.Stop()

			// Drain notification from initial state (initial executors are always a "change" from empty)
			select {
			case <-ch:
				// Expected notification for initial state
			case <-time.After(1 * time.Second):
				t.Fatal("Did not receive notification for initial state")
			}

			// Wait for first state
			err = spectator.firstStateSignal.Wait(context.Background())
			require.NoError(t, err)

			// Check for notification after second update
			if tt.expectNotification {
				select {
				case <-ch:
					// Received expected notification
				case <-time.After(1 * time.Second):
					t.Fatal("Did not receive expected notification about executor change")
				}
			} else {
				select {
				case <-ch:
					t.Fatal("Received unexpected notification when executors did not change")
				case <-time.After(100 * time.Millisecond):
					// Correctly did not receive notification
				}
			}

			// Verify executors were updated
			executors := spectator.GetExecutors()
			assert.Len(t, executors, tt.expectedFinalCount)
		})
	}
}

func TestStopCleansUpSubscribers(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockStream := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)

	spectator := &spectatorImpl{
		namespace:        "test-ns",
		client:           mockClient,
		logger:           log.NewNoop(),
		scope:            tally.NoopScope,
		timeSource:       clock.NewRealTimeSource(),
		firstStateSignal: csync.NewResettableSignal(),
		subscribers:      make(map[string]chan<- struct{}),
		enabled:          func() bool { return true },
	}

	// Subscribe
	_, err := spectator.Subscribe("subscriber-1")
	require.NoError(t, err)

	// Verify subscriber exists
	spectator.subscribersMu.RLock()
	assert.Len(t, spectator.subscribers, 1)
	spectator.subscribersMu.RUnlock()

	// Setup minimal stream expectations
	// Capture the context from WatchNamespaceState call
	var watchCtx context.Context
	mockClient.EXPECT().WatchNamespaceState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *types.WatchNamespaceStateRequest, opts ...interface{}) (sharddistributor.WatchNamespaceStateClient, error) {
			watchCtx = ctx
			return mockStream, nil
		}).AnyTimes()
	mockStream.EXPECT().Recv().AnyTimes().DoAndReturn(func(...interface{}) (*types.WatchNamespaceStateResponse, error) {
		<-watchCtx.Done()
		return nil, watchCtx.Err()
	})
	mockStream.EXPECT().CloseSend().Return(nil).AnyTimes()

	spectator.Start(context.Background())
	spectator.Stop()

	// Verify subscribers were cleaned up
	spectator.subscribersMu.RLock()
	assert.Len(t, spectator.subscribers, 0)
	spectator.subscribersMu.RUnlock()
}
