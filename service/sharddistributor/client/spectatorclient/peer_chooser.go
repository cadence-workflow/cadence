package spectatorclient

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/fx"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

const (
	NamespaceHeader        = "x-shard-distributor-namespace"
	grpcAddressMetadataKey = "grpc_address"
)

// SpectatorPeerChooserInterface extends peer.Chooser with SetSpectators method
type SpectatorPeerChooserInterface interface {
	peer.Chooser
	SetSpectators(spectators *Spectators)
}

// SpectatorPeerChooser is a peer.Chooser that uses the Spectator to route requests
// to the correct executor based on shard ownership.
// This is the shard distributor equivalent of Cadence's RingpopPeerChooser.
//
// Flow:
//  1. Client calls RPC with yarpc.WithShardKey("shard-key")
//  2. Choose() is called with req.ShardKey = "shard-key"
//  3. Query Spectator for shard owner
//  4. Extract grpc_address from owner metadata
//  5. Create/reuse peer for that address
//  6. Return peer to YARPC for connection
type SpectatorPeerChooser struct {
	spectators *Spectators
	transport  peer.Transport
	logger     log.Logger
	namespace  string

	mu    sync.RWMutex
	peers map[string]peer.Peer // grpc_address -> peer
}

type SpectatorPeerChooserParams struct {
	fx.In
	Transport peer.Transport
	Logger    log.Logger
}

// NewSpectatorPeerChooser creates a new peer chooser that routes based on shard distributor ownership
func NewSpectatorPeerChooser(
	params SpectatorPeerChooserParams,
) SpectatorPeerChooserInterface {
	return &SpectatorPeerChooser{
		transport: params.Transport,
		logger:    params.Logger,
		peers:     make(map[string]peer.Peer),
	}
}

// Start satisfies the peer.Chooser interface
func (c *SpectatorPeerChooser) Start() error {
	c.logger.Info("Starting shard distributor peer chooser", tag.ShardNamespace(c.namespace))
	return nil
}

// Stop satisfies the peer.Chooser interface
func (c *SpectatorPeerChooser) Stop() error {
	c.logger.Info("Stopping shard distributor peer chooser", tag.ShardNamespace(c.namespace))

	// Release all peers
	c.mu.Lock()
	defer c.mu.Unlock()

	for addr, p := range c.peers {
		if err := c.transport.ReleasePeer(p, &noOpSubscriber{}); err != nil {
			c.logger.Error("Failed to release peer", tag.Error(err), tag.Address(addr))
		}
	}
	c.peers = make(map[string]peer.Peer)

	return nil
}

// IsRunning satisfies the peer.Chooser interface
func (c *SpectatorPeerChooser) IsRunning() bool {
	return true
}

// Choose returns a peer for the given shard key by:
// 0. Looking up the spectator for the namespace using the x-shard-distributor-namespace header
// 1. Looking up the shard owner via the Spectator
// 2. Extracting the grpc_address from the owner's metadata
// 3. Creating/reusing a peer for that address
//
// The ShardKey in the request is the actual shard key (e.g., workflow ID, shard ID),
// NOT the ip:port address. This is the key distinction from directPeerChooser.
func (c *SpectatorPeerChooser) Choose(ctx context.Context, req *transport.Request) (peer.Peer, func(error), error) {
	if req.ShardKey == "" {
		return nil, nil, yarpcerrors.InvalidArgumentErrorf("chooser requires ShardKey to be non-empty")
	}

	// Get the spectator for the namespace
	namespace, ok := req.Headers.Get(NamespaceHeader)
	if !ok || namespace == "" {
		return nil, nil, yarpcerrors.InvalidArgumentErrorf("chooser requires x-shard-distributor-namespace header to be non-empty")
	}

	spectator, err := c.spectators.ForNamespace(namespace)
	if err != nil {
		return nil, nil, yarpcerrors.InvalidArgumentErrorf("failed to get spectator for namespace %s: %w", namespace, err)
	}

	// Query spectator for shard owner
	owner, err := spectator.GetShardOwner(ctx, req.ShardKey)
	if err != nil {
		return nil, nil, yarpcerrors.UnavailableErrorf("failed to get shard owner for key %s: %v", req.ShardKey, err)
	}

	// Extract GRPC address from owner metadata
	grpcAddress, ok := owner.Metadata[grpcAddressMetadataKey]
	if !ok || grpcAddress == "" {
		return nil, nil, yarpcerrors.InternalErrorf("no grpc_address in metadata for executor %s owning shard %s", owner.ExecutorID, req.ShardKey)
	}

	// Check if we already have a peer for this address
	c.mu.RLock()
	p, ok := c.peers[grpcAddress]
	if ok {
		c.mu.RUnlock()
		return p, func(error) {}, nil
	}
	c.mu.RUnlock()

	// Create new peer for this address
	p, err = c.addPeer(grpcAddress)
	if err != nil {
		return nil, nil, yarpcerrors.InternalErrorf("failed to add peer for address %s: %v", grpcAddress, err)
	}

	return p, func(error) {}, nil
}

func (c *SpectatorPeerChooser) SetSpectators(spectators *Spectators) {
	c.spectators = spectators
}

func (c *SpectatorPeerChooser) addPeer(grpcAddress string) (peer.Peer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check again in case another goroutine added it
	if p, ok := c.peers[grpcAddress]; ok {
		return p, nil
	}

	p, err := c.transport.RetainPeer(hostport.Identify(grpcAddress), &noOpSubscriber{})
	if err != nil {
		return nil, fmt.Errorf("retain peer failed: %w", err)
	}

	c.peers[grpcAddress] = p
	c.logger.Info("Added peer to shard distributor peer chooser", tag.Address(grpcAddress))
	return p, nil
}

// noOpSubscriber is a no-op implementation of peer.Subscriber
type noOpSubscriber struct{}

func (*noOpSubscriber) NotifyStatusChanged(peer.Identifier) {}
