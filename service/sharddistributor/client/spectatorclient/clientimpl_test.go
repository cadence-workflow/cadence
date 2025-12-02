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
)

func TestWatchLoopBasicFlow(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockStream := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)

	spectator := &spectatorImpl{
		namespace:  "test-ns",
		client:     mockClient,
		logger:     log.NewNoop(),
		scope:      tally.NoopScope,
		timeSource: clock.NewRealTimeSource(),
	}
	spectator.firstStateWG.Add(1)

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
		<-spectator.ctx.Done()
		return nil, spectator.ctx.Err()
	})

	mockStream.EXPECT().CloseSend().Return(nil)

	ctx := context.Background()
	err := spectator.Start(ctx)
	require.NoError(t, err)
	defer spectator.Stop()

	// Wait for first state
	spectator.firstStateWG.Wait()

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

	spectator := &spectatorImpl{
		namespace:  "test-ns",
		client:     mockClient,
		logger:     log.NewNoop(),
		scope:      tally.NoopScope,
		timeSource: clock.NewRealTimeSource(),
	}
	spectator.firstStateWG.Add(1)

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
		<-spectator.ctx.Done()
		return nil, spectator.ctx.Err()
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
	defer spectator.Stop()

	spectator.firstStateWG.Wait()

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

	spectator := &spectatorImpl{
		namespace:  "test-ns",
		client:     mockClient,
		logger:     log.NewNoop(),
		scope:      tally.NoopScope,
		timeSource: mockTimeSource,
	}
	spectator.firstStateWG.Add(1)

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
		<-spectator.ctx.Done()
		return nil, errors.New("shutdown")
	})

	mockStream2.EXPECT().CloseSend().Return(nil)

	spectator.Start(context.Background())
	defer spectator.Stop()

	// Wait for the goroutine to be blocked in Sleep, then advance time
	mockTimeSource.BlockUntil(1) // Wait for 1 goroutine to be blocked in Sleep
	mockTimeSource.Advance(2 * time.Second)

	spectator.firstStateWG.Wait()
}
