package spectatorclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/grpc"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

func TestSpectatorPeerChooser_Choose_MissingShardKey(t *testing.T) {
	chooser := &SpectatorPeerChooser{
		logger: testlogger.New(t),
		peers:  make(map[string]peer.Peer),
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
		logger: testlogger.New(t),
		peers:  make(map[string]peer.Peer),
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
		peers:      make(map[string]peer.Peer),
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
		logger: testlogger.New(t),
		peers:  make(map[string]peer.Peer),
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

	chooser := &SpectatorPeerChooser{
		transport: peerTransport,
		logger:    testlogger.New(t),
		peers:     make(map[string]peer.Peer),
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
}

func TestSpectatorPeerChooser_Choose_ReusesPeer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSpectator := NewMockSpectator(ctrl)
	peerTransport := grpc.NewTransport()

	chooser := &SpectatorPeerChooser{
		transport: peerTransport,
		logger:    testlogger.New(t),
		peers:     make(map[string]peer.Peer),
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
