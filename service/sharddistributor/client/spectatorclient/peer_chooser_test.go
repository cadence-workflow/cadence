package spectatorclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/grpc"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

// Test helpers

func newTestChooser(t *testing.T) *SpectatorPeerChooser {
	return &SpectatorPeerChooser{
		transport:       grpc.NewTransport(),
		logger:          testlogger.New(t),
		peers:           make(map[string]peer.Peer),
		grpcAddressToNs: make(map[string]map[string]struct{}),
		stopCh:          make(chan struct{}),
	}
}

func newTestChooserWithSpectators(t *testing.T, spectators map[string]Spectator) *SpectatorPeerChooser {
	chooser := newTestChooser(t)
	chooser.spectators = &Spectators{spectators: spectators}
	return chooser
}

func newTestChooserWithTransport(t *testing.T, ctrl *gomock.Controller) (*SpectatorPeerChooser, peer.Transport) {
	peerTransport := grpc.NewTransport()
	require.NoError(t, peerTransport.Start())
	t.Cleanup(func() { _ = peerTransport.Stop() })

	chooser := NewSpectatorPeerChooser(SpectatorPeerChooserParams{
		Transport: peerTransport,
		Logger:    testlogger.New(t),
	}).(*SpectatorPeerChooser)

	return chooser, peerTransport
}

func newTestRequest(shardKey, namespace string) *transport.Request {
	return &transport.Request{
		ShardKey: shardKey,
		Headers:  transport.NewHeaders().With(NamespaceHeader, namespace),
	}
}

func newShardOwner(executorID, grpcAddress string) *ShardOwner {
	return &ShardOwner{
		ExecutorID: executorID,
		Metadata:   map[string]string{clientcommon.GrpcAddressMetadataKey: grpcAddress},
	}
}

func TestSpectatorPeerChooser_Choose_MissingShardKey(t *testing.T) {
	chooser := newTestChooser(t)
	req := &transport.Request{ShardKey: "", Headers: transport.NewHeaders()}

	p, onFinish, err := chooser.Choose(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Nil(t, onFinish)
	assert.Contains(t, err.Error(), "ShardKey")
}

func TestSpectatorPeerChooser_Choose_MissingNamespaceHeader(t *testing.T) {
	chooser := newTestChooser(t)
	req := &transport.Request{ShardKey: "shard-1", Headers: transport.NewHeaders()}

	p, onFinish, err := chooser.Choose(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Nil(t, onFinish)
	assert.Contains(t, err.Error(), "x-shard-distributor-namespace")
}

func TestSpectatorPeerChooser_Choose_SpectatorNotFound(t *testing.T) {
	chooser := newTestChooserWithSpectators(t, make(map[string]Spectator))
	req := newTestRequest("shard-1", "unknown-namespace")

	p, onFinish, err := chooser.Choose(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Nil(t, onFinish)
	assert.Contains(t, err.Error(), "spectator not found")
}

func TestSpectatorPeerChooser_StartStop(t *testing.T) {
	chooser := newTestChooser(t)

	err := chooser.Start()
	require.NoError(t, err)
	assert.True(t, chooser.IsRunning())

	err = chooser.Stop()
	assert.NoError(t, err)
}

func TestSpectatorPeerChooser_SetSpectators(t *testing.T) {
	chooser := newTestChooser(t)
	spectators := &Spectators{spectators: make(map[string]Spectator)}

	chooser.SetSpectators(spectators)

	assert.Equal(t, spectators, chooser.spectators)
}

func TestSpectatorPeerChooser_Start_WithSubscriptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	updateCh := make(chan struct{}, 1)
	chooser := newTestChooserWithSpectators(t, map[string]Spectator{"test-namespace": mockSpectator})

	mockSpectator.EXPECT().Subscribe(peerChooserSubscriberName).Return(updateCh, nil)
	err := chooser.Start()
	require.NoError(t, err)

	mockSpectator.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(nil)
	err = chooser.Stop()
	assert.NoError(t, err)
}

func TestSpectatorPeerChooser_Start_SubscriptionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser := newTestChooserWithSpectators(t, map[string]Spectator{"test-namespace": mockSpectator})

	mockSpectator.EXPECT().Subscribe(peerChooserSubscriberName).Return(nil, assert.AnError)
	mockSpectator.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(nil) // cleanup partial subscriptions
	err := chooser.Start()
	require.Error(t, err) // Start should fail fast on subscription error
	assert.Contains(t, err.Error(), "failed to subscribe to spectator for namespace test-namespace")
}

func TestSpectatorPeerChooser_Stop_WithPeers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("executor-1", "127.0.0.1:7953"), nil)
	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	require.NoError(t, err)

	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1)
	chooser.peersMutex.RUnlock()

	mockSpectator.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(nil)
	err = chooser.Stop()
	assert.NoError(t, err)

	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 0)
	chooser.peersMutex.RUnlock()
}

func TestSpectatorPeerChooser_Stop_UnsubscribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser := newTestChooserWithSpectators(t, map[string]Spectator{"test-namespace": mockSpectator})

	mockSpectator.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(assert.AnError)
	err := chooser.Stop()
	require.Error(t, err) // Stop should return unsubscribe error
	assert.Contains(t, err.Error(), "unsubscribe from namespace test-namespace")
}

func TestSpectatorPeerChooser_Subscribe_FailFast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser := newTestChooser(t)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{
		"test-namespace": mockSpectator,
	}}

	// Simulate subscription failure
	mockSpectator.EXPECT().Subscribe(peerChooserSubscriberName).Return(nil, assert.AnError)

	err := chooser.subscribe()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to subscribe to spectator for namespace test-namespace")
}

func TestSpectatorPeerChooser_Unsubscribe_CombinesErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator1, mockSpectator2 := NewMockSpectator(ctrl), NewMockSpectator(ctrl)
	chooser := newTestChooser(t)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{
		"ns-1": mockSpectator1,
		"ns-2": mockSpectator2,
	}}

	// Both spectators will fail to unsubscribe
	mockSpectator1.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(assert.AnError)
	mockSpectator2.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(assert.AnError)

	err := chooser.unsubscribe()
	require.Error(t, err)
	// Both namespace errors should be present in the combined error
	assert.Contains(t, err.Error(), "unsubscribe from namespace ns-1")
	assert.Contains(t, err.Error(), "unsubscribe from namespace ns-2")
}

func TestSpectatorPeerChooser_Start_UnsubscribesOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser := newTestChooser(t)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{
		"test-namespace": mockSpectator,
	}}

	// Subscription fails
	mockSpectator.EXPECT().Subscribe(peerChooserSubscriberName).Return(nil, assert.AnError)

	// unsubscribe should be called during cleanup (even though subscribe failed)
	mockSpectator.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(nil)

	err := chooser.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to subscribe to spectator for namespace test-namespace")
}

func TestSpectatorPeerChooser_Stop_ReleasePeerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator, mockTransport, mockPeer := NewMockSpectator(ctrl), NewMockTransport(ctrl), NewMockPeer(ctrl)
	chooser := newTestChooser(t)
	chooser.transport = mockTransport
	chooser.peers = map[string]peer.Peer{"127.0.0.1:7953": mockPeer}
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	mockSpectator.EXPECT().Unsubscribe(peerChooserSubscriberName).Return(nil)
	mockTransport.EXPECT().ReleasePeer(mockPeer, gomock.Any()).Return(assert.AnError)

	err := chooser.Stop()
	assert.NoError(t, err)

	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 0)
	chooser.peersMutex.RUnlock()
}

func TestSpectatorPeerChooser_Choose_MissingGrpcAddress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser := newTestChooserWithSpectators(t, map[string]Spectator{"test-namespace": mockSpectator})

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(&ShardOwner{
		ExecutorID: "executor-1",
		Metadata:   map[string]string{},
	}, nil)

	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no grpc_address")
}

func TestSpectatorPeerChooser_Choose_GetShardOwnerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser := newTestChooserWithSpectators(t, map[string]Spectator{"test-namespace": mockSpectator})

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(nil, assert.AnError)

	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	assert.Error(t, err)
}

func TestSpectatorPeerChooser_Choose_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("executor-1", "127.0.0.1:7953"), nil)

	p, onFinish, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))

	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.NotNil(t, onFinish)
	assert.Equal(t, "127.0.0.1:7953", p.Identifier())
	assert.Len(t, chooser.peers, 1)
}

func TestSpectatorPeerChooser_Choose_RetainPeerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator, mockTransport := NewMockSpectator(ctrl), NewMockTransport(ctrl)
	chooser := newTestChooser(t)
	chooser.transport = mockTransport
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("executor-1", "127.0.0.1:7953"), nil)
	mockTransport.EXPECT().RetainPeer(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get or create peer")
}

func TestSpectatorPeerChooser_Choose_ReusesPeer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("executor-1", "127.0.0.1:7953"), nil).Times(2)

	firstPeer, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	require.NoError(t, err)

	secondPeer, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	assert.NoError(t, err)
	assert.Equal(t, firstPeer, secondPeer)
	assert.Len(t, chooser.peers, 1)
}

func TestSpectatorPeerChooser_Choose_ReusesPeer_NilNamespaceMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("executor-1", "127.0.0.1:7953"), nil)

	firstPeer, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	require.NoError(t, err)

	// Simulate the edge case where grpcAddressToNs entry was deleted (e.g., after a failed release attempt)
	chooser.peersMutex.Lock()
	delete(chooser.grpcAddressToNs, "127.0.0.1:7953")
	chooser.peersMutex.Unlock()

	// Now try to reuse the peer with a different namespace
	mockSpectator2 := NewMockSpectator(ctrl)
	chooser.spectators.spectators["test-namespace-2"] = mockSpectator2

	mockSpectator2.EXPECT().GetShardOwner(gomock.Any(), "shard-2").Return(newShardOwner("executor-2", "127.0.0.1:7953"), nil)

	secondPeer, _, err := chooser.Choose(context.Background(), newTestRequest("shard-2", "test-namespace-2"))
	require.NoError(t, err)
	assert.Equal(t, firstPeer, secondPeer, "should reuse existing peer")

	// Verify namespace was tracked even though map was nil
	chooser.peersMutex.RLock()
	assert.Contains(t, chooser.grpcAddressToNs["127.0.0.1:7953"], "test-namespace-2")
	chooser.peersMutex.RUnlock()
}

func TestSpectatorPeerChooser_ReleaseStalePeers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("executor-1", "127.0.0.1:7953"), nil)
	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-2").Return(newShardOwner("executor-2", "127.0.0.1:7954"), nil)

	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	require.NoError(t, err)
	_, _, err = chooser.Choose(context.Background(), newTestRequest("shard-2", "test-namespace"))
	require.NoError(t, err)

	// Verify we have 2 peers
	assert.Len(t, chooser.peers, 2)

	// Now simulate executor-2 being removed (only executor-1 remains)
	mockSpectator.EXPECT().
		GetExecutors().
		Return(map[string]*ShardOwner{
			"executor-1": {
				ExecutorID: "executor-1",
				Metadata: map[string]string{
					clientcommon.GrpcAddressMetadataKey: "127.0.0.1:7953",
				},
			},
		})

	// Call removeStaleExecutors
	chooser.removeStaleExecutors("test-namespace", mockSpectator)

	// Verify that only 1 peer remains (executor-1)
	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1)
	_, exists := chooser.peers["127.0.0.1:7953"]
	assert.True(t, exists)
	_, exists = chooser.peers["127.0.0.1:7954"]
	assert.False(t, exists)
	chooser.peersMutex.RUnlock()
}

func TestSpectatorPeerChooser_WatchExecutorUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	peerTransport := grpc.NewTransport()

	chooser := NewSpectatorPeerChooser(SpectatorPeerChooserParams{
		Transport: peerTransport,
		Logger:    testlogger.New(t),
	}).(*SpectatorPeerChooser)

	chooser.spectators = &Spectators{
		spectators: map[string]Spectator{
			"test-namespace": mockSpectator,
		},
	}

	// Create initial peers
	req1 := &transport.Request{
		ShardKey: "shard-1",
		Headers:  transport.NewHeaders().With(NamespaceHeader, "test-namespace"),
	}
	req2 := &transport.Request{
		ShardKey: "shard-2",
		Headers:  transport.NewHeaders().With(NamespaceHeader, "test-namespace"),
	}

	mockSpectator.EXPECT().
		GetShardOwner(gomock.Any(), "shard-1").
		Return(&ShardOwner{
			ExecutorID: "executor-1",
			Metadata: map[string]string{
				clientcommon.GrpcAddressMetadataKey: "127.0.0.1:7953",
			},
		}, nil)

	mockSpectator.EXPECT().
		GetShardOwner(gomock.Any(), "shard-2").
		Return(&ShardOwner{
			ExecutorID: "executor-2",
			Metadata: map[string]string{
				clientcommon.GrpcAddressMetadataKey: "127.0.0.1:7954",
			},
		}, nil)

	_, _, err := chooser.Choose(context.Background(), req1)
	require.NoError(t, err)
	_, _, err = chooser.Choose(context.Background(), req2)
	require.NoError(t, err)

	assert.Len(t, chooser.peers, 2)

	// Create a notification channel
	updateCh := make(chan struct{}, 1)

	// Start watchExecutorUpdates in a goroutine
	chooser.stopWG.Add(1)
	go chooser.watchExecutorUpdates("test-namespace", mockSpectator, updateCh)

	// Mock GetExecutors to return only executor-1 (executor-2 is removed)
	mockSpectator.EXPECT().
		GetExecutors().
		Return(map[string]*ShardOwner{
			"executor-1": {
				ExecutorID: "executor-1",
				Metadata: map[string]string{
					clientcommon.GrpcAddressMetadataKey: "127.0.0.1:7953",
				},
			},
		})

	// Send update notification
	updateCh <- struct{}{}

	// Wait a bit for the update to be processed
	require.Eventually(t, func() bool {
		chooser.peersMutex.RLock()
		defer chooser.peersMutex.RUnlock()
		return len(chooser.peers) == 1
	}, 1*time.Second, 10*time.Millisecond)

	// Verify that stale peer was released
	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1)
	_, exists := chooser.peers["127.0.0.1:7953"]
	assert.True(t, exists)
	_, exists = chooser.peers["127.0.0.1:7954"]
	assert.False(t, exists)
	chooser.peersMutex.RUnlock()

	// Test stopping the watch goroutine
	close(chooser.stopCh)
	chooser.stopWG.Wait()
}

// TestSpectatorPeerChooser_MultiNamespace_SharedAddress tests that when multiple
// namespaces use the same grpc address, the peer is only removed when all namespaces
// stop using it.
func TestSpectatorPeerChooser_MultiNamespace_SharedAddress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator1, mockSpectator2 := NewMockSpectator(ctrl), NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"ns-1": mockSpectator1, "ns-2": mockSpectator2}}

	sharedAddr := "127.0.0.1:7953"
	mockSpectator1.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("exec-1", sharedAddr), nil)
	mockSpectator2.EXPECT().GetShardOwner(gomock.Any(), "shard-2").Return(newShardOwner("exec-2", sharedAddr), nil)

	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "ns-1"))
	require.NoError(t, err)
	_, _, err = chooser.Choose(context.Background(), newTestRequest("shard-2", "ns-2"))
	require.NoError(t, err)

	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1)
	assert.Len(t, chooser.grpcAddressToNs[sharedAddr], 2)
	chooser.peersMutex.RUnlock()

	mockSpectator1.EXPECT().GetExecutors().Return(map[string]*ShardOwner{})
	chooser.removeStaleExecutors("ns-1", mockSpectator1)

	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1, "peer should remain when ns-2 still uses it")
	assert.Len(t, chooser.grpcAddressToNs[sharedAddr], 1)
	assert.Contains(t, chooser.grpcAddressToNs[sharedAddr], "ns-2")
	chooser.peersMutex.RUnlock()

	mockSpectator2.EXPECT().GetExecutors().Return(map[string]*ShardOwner{})
	chooser.removeStaleExecutors("ns-2", mockSpectator2)

	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 0, "peer should be removed when no namespace uses it")
	chooser.peersMutex.RUnlock()
}

// TestSpectatorPeerChooser_ReleasePeerFailure tests that ReleasePeer failures
// don't cause errors in subsequent operations and the peer can be released on retry.
func TestSpectatorPeerChooser_ReleasePeerFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator, mockTransport, mockPeer := NewMockSpectator(ctrl), NewMockTransport(ctrl), NewMockPeer(ctrl)
	chooser := newTestChooser(t)
	chooser.transport = mockTransport
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"ns-1": mockSpectator}}

	addr := "127.0.0.1:7953"
	mockTransport.EXPECT().RetainPeer(gomock.Any(), gomock.Any()).Return(mockPeer, nil)
	mockPeer.EXPECT().Identifier().Return(addr).AnyTimes()
	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("exec-1", addr), nil)

	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "ns-1"))
	require.NoError(t, err)

	// Try to release - fails
	mockSpectator.EXPECT().GetExecutors().Return(map[string]*ShardOwner{})
	mockTransport.EXPECT().ReleasePeer(mockPeer, gomock.Any()).Return(assert.AnError)
	chooser.removeStaleExecutors("ns-1", mockSpectator)

	// Peer remains due to failure
	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1)
	assert.Len(t, chooser.grpcAddressToNs[addr], 0, "namespace tracking should be cleaned up")
	chooser.peersMutex.RUnlock()

	// Different namespace can still use the peer
	mockSpectator2 := NewMockSpectator(ctrl)
	chooser.spectators.spectators["ns-2"] = mockSpectator2
	mockSpectator2.EXPECT().GetShardOwner(gomock.Any(), "shard-2").Return(newShardOwner("exec-2", addr), nil)

	p, _, err := chooser.Choose(context.Background(), newTestRequest("shard-2", "ns-2"))
	require.NoError(t, err)
	assert.Equal(t, mockPeer, p, "should reuse existing peer")

	// Retry release - succeeds
	mockSpectator2.EXPECT().GetExecutors().Return(map[string]*ShardOwner{})
	mockTransport.EXPECT().ReleasePeer(mockPeer, gomock.Any()).Return(nil)
	chooser.removeStaleExecutors("ns-2", mockSpectator2)

	// Peer removed
	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 0)
	chooser.peersMutex.RUnlock()
}

// TestSpectatorPeerChooser_WatchChannelClosed tests that watchExecutorUpdates
// exits cleanly when the update channel is closed.
func TestSpectatorPeerChooser_WatchChannelClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser := NewSpectatorPeerChooser(SpectatorPeerChooserParams{
		Transport: grpc.NewTransport(),
		Logger:    testlogger.New(t),
	}).(*SpectatorPeerChooser)

	updateCh := make(chan struct{})
	chooser.stopWG.Add(1)
	go chooser.watchExecutorUpdates("ns-1", mockSpectator, updateCh)

	// Close channel and verify goroutine exits
	close(updateCh)

	done := make(chan struct{})
	go func() {
		chooser.stopWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("watchExecutorUpdates did not exit after channel closed")
	}
}

func TestSpectatorPeerChooser_RemoveStalePeers_AddressStillUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{"test-namespace": mockSpectator}}

	addr := "127.0.0.1:7953"
	mockSpectator.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("executor-1", addr), nil)

	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "test-namespace"))
	require.NoError(t, err)

	// Call removeStaleExecutors with same executor still present
	mockSpectator.EXPECT().GetExecutors().Return(map[string]*ShardOwner{
		"executor-1": newShardOwner("executor-1", addr),
	})

	chooser.removeStaleExecutors("test-namespace", mockSpectator)

	// Peer should still exist (address still used)
	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1)
	assert.Contains(t, chooser.peers, addr)
	chooser.peersMutex.RUnlock()
}

func TestSpectatorPeerChooser_RemoveStalePeers_DifferentNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator1, mockSpectator2 := NewMockSpectator(ctrl), NewMockSpectator(ctrl)
	chooser, _ := newTestChooserWithTransport(t, ctrl)
	chooser.spectators = &Spectators{spectators: map[string]Spectator{
		"ns-1": mockSpectator1,
		"ns-2": mockSpectator2,
	}}

	// ns-1 creates a peer at addr1
	addr1 := "127.0.0.1:7953"
	mockSpectator1.EXPECT().GetShardOwner(gomock.Any(), "shard-1").Return(newShardOwner("exec-1", addr1), nil)
	_, _, err := chooser.Choose(context.Background(), newTestRequest("shard-1", "ns-1"))
	require.NoError(t, err)

	// Now call removeStaleExecutors for ns-2 (which never used this address)
	mockSpectator2.EXPECT().GetExecutors().Return(map[string]*ShardOwner{})
	chooser.removeStaleExecutors("ns-2", mockSpectator2)

	// Peer should still exist (ns-1 still uses it)
	chooser.peersMutex.RLock()
	assert.Len(t, chooser.peers, 1)
	assert.Contains(t, chooser.peers, addr1)
	assert.Contains(t, chooser.grpcAddressToNs[addr1], "ns-1")
	assert.NotContains(t, chooser.grpcAddressToNs[addr1], "ns-2")
	chooser.peersMutex.RUnlock()
}
