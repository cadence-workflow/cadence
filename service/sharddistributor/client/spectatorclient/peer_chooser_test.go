package spectatorclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/transport/grpc"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

func TestSpectatorPeerChooser_Choose_MissingShardKey(t *testing.T) {
	chooser := &SpectatorPeerChooser{
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: clock.NewRealTimeSource(),
	}

	req := &transport.Request{
		ShardKey: "",
		Headers:  transport.NewHeaders(),
	}

	p, onFinish, err := chooser.Choose(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Nil(t, onFinish)
	assert.Contains(t, err.Error(), "ShardKey")
}

func TestSpectatorPeerChooser_Choose_MissingNamespaceHeader(t *testing.T) {
	chooser := &SpectatorPeerChooser{
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: clock.NewRealTimeSource(),
	}

	req := &transport.Request{
		ShardKey: "shard-1",
		Headers:  transport.NewHeaders(),
	}

	p, onFinish, err := chooser.Choose(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Nil(t, onFinish)
	assert.Contains(t, err.Error(), "x-shard-distributor-namespace")
}

func TestSpectatorPeerChooser_Choose_SpectatorNotFound(t *testing.T) {
	chooser := &SpectatorPeerChooser{
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: clock.NewRealTimeSource(),
		spectators: &Spectators{spectators: make(map[string]Spectator)},
	}

	req := &transport.Request{
		ShardKey: "shard-1",
		Headers:  transport.NewHeaders().With(NamespaceHeader, "unknown-namespace"),
	}

	p, onFinish, err := chooser.Choose(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Nil(t, onFinish)
	assert.Contains(t, err.Error(), "spectator not found")
}

func TestSpectatorPeerChooser_StartStop(t *testing.T) {
	chooser := &SpectatorPeerChooser{
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: clock.NewRealTimeSource(),
		peerTTL:    time.Minute,
	}

	err := chooser.Start()
	require.NoError(t, err)

	assert.True(t, chooser.IsRunning())

	err = chooser.Stop()
	assert.NoError(t, err)
}

func TestSpectatorPeerChooser_SetSpectators(t *testing.T) {
	chooser := &SpectatorPeerChooser{
		logger: testlogger.New(t),
	}

	spectators := &Spectators{spectators: make(map[string]Spectator)}
	chooser.SetSpectators(spectators)

	assert.Equal(t, spectators, chooser.spectators)
}

func TestSpectatorPeerChooser_Choose_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	peerTransport := grpc.NewTransport()
	require.NoError(t, peerTransport.Start())
	defer peerTransport.Stop()

	chooser := &SpectatorPeerChooser{
		transport:  peerTransport,
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: clock.NewRealTimeSource(),
		spectators: &Spectators{
			spectators: map[string]Spectator{
				"test-namespace": mockSpectator,
			},
		},
	}

	ctx := context.Background()
	req := &transport.Request{
		ShardKey: "shard-1",
		Headers:  transport.NewHeaders().With(NamespaceHeader, "test-namespace"),
	}

	// Mock spectator to return shard owner with grpc_address
	mockSpectator.EXPECT().
		GetShardOwner(ctx, "shard-1").
		Return(&ShardOwner{
			ExecutorID: "executor-1",
			Metadata: map[string]string{
				clientcommon.GrpcAddressMetadataKey: "127.0.0.1:7953",
			},
		}, nil)

	// Execute
	p, onFinish, err := chooser.Choose(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.NotNil(t, onFinish)
	assert.Equal(t, "127.0.0.1:7953", p.Identifier())
	assert.Len(t, chooser.peers, 1)
	assert.Equal(t, "127.0.0.1:7953", chooser.peers["127.0.0.1:7953"].peer.Identifier())
}

func TestSpectatorPeerChooser_Choose_ReusesPeer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	peerTransport := grpc.NewTransport()
	require.NoError(t, peerTransport.Start())
	defer peerTransport.Stop()

	chooser := &SpectatorPeerChooser{
		transport:  peerTransport,
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: clock.NewRealTimeSource(),
		spectators: &Spectators{
			spectators: map[string]Spectator{
				"test-namespace": mockSpectator,
			},
		},
	}

	req := &transport.Request{
		ShardKey: "shard-1",
		Headers:  transport.NewHeaders().With(NamespaceHeader, "test-namespace"),
	}

	// First call creates the peer
	mockSpectator.EXPECT().
		GetShardOwner(gomock.Any(), "shard-1").
		Return(&ShardOwner{
			ExecutorID: "executor-1",
			Metadata: map[string]string{
				clientcommon.GrpcAddressMetadataKey: "127.0.0.1:7953",
			},
		}, nil).Times(2)

	firstPeer, _, err := chooser.Choose(context.Background(), req)
	require.NoError(t, err)

	// Second call should reuse the same peer
	secondPeer, _, err := chooser.Choose(context.Background(), req)

	// Assert - should reuse existing peer
	assert.NoError(t, err)
	assert.Equal(t, firstPeer, secondPeer)
	assert.Len(t, chooser.peers, 1)
}

func TestSpectatorPeerChooser_Choose_TracksLastUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	peerTransport := grpc.NewTransport()
	require.NoError(t, peerTransport.Start())
	defer peerTransport.Stop()

	mockClock := clock.NewMockedTimeSource()

	chooser := &SpectatorPeerChooser{
		transport:  peerTransport,
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: mockClock,
		spectators: &Spectators{
			spectators: map[string]Spectator{"ns": mockSpectator},
		},
	}

	req := &transport.Request{
		ShardKey: "shard-1",
		Headers:  transport.NewHeaders().With(NamespaceHeader, "ns"),
	}

	// First call — creates peer, sets lastUsed to t0
	mockSpectator.EXPECT().
		GetShardOwner(gomock.Any(), "shard-1").
		Return(&ShardOwner{
			ExecutorID: "exec-1",
			Metadata:   map[string]string{clientcommon.GrpcAddressMetadataKey: "127.0.0.1:7953"},
		}, nil).Times(2)

	_, _, err := chooser.Choose(context.Background(), req)
	require.NoError(t, err)
	firstLastUsed := chooser.peers["127.0.0.1:7953"].lastUsed

	// Advance the clock
	mockClock.Advance(30 * time.Second)

	// Second call — reuses peer, should update lastUsed to t0+30s
	_, _, err = chooser.Choose(context.Background(), req)
	require.NoError(t, err)
	secondLastUsed := chooser.peers["127.0.0.1:7953"].lastUsed

	// lastUsed should have advanced
	assert.True(t, secondLastUsed.After(firstLastUsed), "lastUsed should be updated on reuse")
}

func TestSpectatorPeerChooser_StartStop_WithTTL(t *testing.T) {
	peerTransport := grpc.NewTransport()
	require.NoError(t, peerTransport.Start())
	defer peerTransport.Stop()

	chooser := &SpectatorPeerChooser{
		transport:  peerTransport,
		logger:     testlogger.New(t),
		peers:      make(map[string]*trackedPeer),
		timeSource: clock.NewRealTimeSource(),
		peerTTL:    100 * time.Millisecond,
	}

	require.NoError(t, chooser.Start())
	assert.NotNil(t, chooser.stopCh, "eviction loop should be started")

	require.NoError(t, chooser.Stop())
	// After Stop, the goroutine must have exited (stopWG.Wait() returned)
	// Verify idempotency: a second Stop should not panic
	require.NoError(t, chooser.Stop())
}

func TestSpectatorPeerChooser_EvictStalePeers(t *testing.T) {
	tests := []struct {
		name          string
		peerTTL       time.Duration
		advanceBy     time.Duration
		expectEvicted bool
	}{
		{
			name:          "peer within TTL is kept",
			peerTTL:       2 * time.Minute,
			advanceBy:     1 * time.Minute,
			expectEvicted: false,
		},
		{
			name:          "peer exactly at TTL boundary is kept",
			peerTTL:       2 * time.Minute,
			advanceBy:     2 * time.Minute,
			expectEvicted: false,
		},
		{
			name:          "peer past TTL is evicted",
			peerTTL:       2 * time.Minute,
			advanceBy:     2*time.Minute + time.Millisecond,
			expectEvicted: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			peerTransport := grpc.NewTransport()
			require.NoError(t, peerTransport.Start())
			defer peerTransport.Stop()

			mockClock := clock.NewMockedTimeSource()

			chooser := &SpectatorPeerChooser{
				transport:  peerTransport,
				logger:     testlogger.New(t),
				peers:      make(map[string]*trackedPeer),
				timeSource: mockClock,
				peerTTL:    tc.peerTTL,
			}

			// Manually insert a tracked peer with lastUsed = now
			p, err := peerTransport.RetainPeer(hostport.Identify("127.0.0.1:7953"), &noOpSubscriber{})
			require.NoError(t, err)
			chooser.peers["127.0.0.1:7953"] = &trackedPeer{
				peer:     p,
				lastUsed: mockClock.Now(),
			}

			// Advance clock
			mockClock.Advance(tc.advanceBy)

			// Run eviction directly
			chooser.evictStalePeers()

			if tc.expectEvicted {
				assert.Empty(t, chooser.peers)
			} else {
				assert.Len(t, chooser.peers, 1)
			}
		})
	}
}
