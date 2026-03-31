package spectatorclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

const (
	NamespaceHeader = "x-shard-distributor-namespace"
)

type trackedPeer struct {
	peer     peer.Peer
	lastUsed time.Time
}

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

	peersMutex sync.RWMutex
	peers      map[string]*trackedPeer // grpc_address -> peer
	timeSource clock.TimeSource
	peerTTL    time.Duration
	stopCh     chan struct{}
	stopWG     sync.WaitGroup
}

type SpectatorPeerChooserParams struct {
	fx.In
	Transport  peer.Transport
	Logger     log.Logger
	Config     clientcommon.Config
	TimeSource clock.TimeSource
}

// NewSpectatorPeerChooser creates a new peer chooser that routes based on shard distributor ownership
func NewSpectatorPeerChooser(
	params SpectatorPeerChooserParams,
) SpectatorPeerChooserInterface {
	return &SpectatorPeerChooser{
		transport:  params.Transport,
		logger:     params.Logger,
		peers:      make(map[string]*trackedPeer),
		peerTTL:    params.Config.GetPeerTTL(),
		timeSource: params.TimeSource,
	}
}

// Start satisfies the peer.Chooser interface
func (c *SpectatorPeerChooser) Start() error {
	c.logger.Info("Starting shard distributor peer chooser", tag.ShardNamespace(c.namespace))
	c.startEvictionLoop()
	return nil
}

// Stop satisfies the peer.Chooser interface
func (c *SpectatorPeerChooser) Stop() error {
	c.logger.Info("Stopping shard distributor peer chooser", tag.ShardNamespace(c.namespace))

	if c.stopCh != nil {
		close(c.stopCh)
		c.stopWG.Wait()
		c.stopCh = nil
	}

	c.peersMutex.Lock()
	defer c.peersMutex.Unlock()

	for addr, tp := range c.peers {
		if err := c.transport.ReleasePeer(tp.peer, &noOpSubscriber{}); err != nil {
			c.logger.Error("Failed to release peer", tag.Error(err), tag.Address(addr))
		}
	}
	c.peers = make(map[string]*trackedPeer)

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
// The ShardKey in the request is the shard key (e.g., shard ID)
// The function returns
// peer: the peer to use for the request
// onFinish: a function to call when the request is finished (currently no-op)
// err: the error if the request failed
func (c *SpectatorPeerChooser) Choose(ctx context.Context, req *transport.Request) (peer peer.Peer, onFinish func(error), err error) {
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
		return nil, nil, yarpcerrors.InvalidArgumentErrorf("get spectator for namespace %s: %w", namespace, err)
	}

	// Query spectator for shard owner
	owner, err := spectator.GetShardOwner(ctx, req.ShardKey)
	if err != nil {
		return nil, nil, yarpcerrors.UnavailableErrorf("get shard owner for key %s: %v", req.ShardKey, err)
	}

	// Extract GRPC address from owner metadata
	grpcAddress, ok := owner.Metadata[clientcommon.GrpcAddressMetadataKey]
	if !ok || grpcAddress == "" {
		return nil, nil, yarpcerrors.InternalErrorf("no grpc_address in metadata for executor %s owning shard %s", owner.ExecutorID, req.ShardKey)
	}

	// Get peer for this address
	p, err := c.getOrCreatePeer(grpcAddress)
	if err != nil {
		return nil, nil, yarpcerrors.InternalErrorf("get or create peer for address %s: %v", grpcAddress, err)
	}

	return p, func(error) {}, nil
}

func (c *SpectatorPeerChooser) SetSpectators(spectators *Spectators) {
	c.spectators = spectators
}

func (c *SpectatorPeerChooser) getOrCreatePeer(grpcAddress string) (peer.Peer, error) {
	c.peersMutex.Lock()
	defer c.peersMutex.Unlock()

	if tp, ok := c.peers[grpcAddress]; ok {
		tp.lastUsed = c.timeSource.Now()
		return tp.peer, nil
	}

	p, err := c.transport.RetainPeer(hostport.Identify(grpcAddress), &noOpSubscriber{})
	if err != nil {
		return nil, fmt.Errorf("retain peer: %w", err)
	}

	tp := &trackedPeer{peer: p, lastUsed: c.timeSource.Now()}
	c.peers[grpcAddress] = tp
	return p, nil
}

func (c *SpectatorPeerChooser) evictStalePeers() {
	now := c.timeSource.Now()

	c.peersMutex.Lock()
	defer c.peersMutex.Unlock()

	for addr, tp := range c.peers {
		if now.Sub(tp.lastUsed) > c.peerTTL {
			if err := c.transport.ReleasePeer(tp.peer, &noOpSubscriber{}); err != nil {
				c.logger.Error("Failed to release stale peer", tag.Error(err), tag.Address(addr))
			}
			delete(c.peers, addr)
		}
	}
}

func (c *SpectatorPeerChooser) startEvictionLoop() {
	if c.stopCh != nil {
		return // already started
	}
	c.stopCh = make(chan struct{})
	c.stopWG.Add(1)
	go func() {
		defer c.stopWG.Done()
		// Tick at half the TTL so a peer is evicted within at most 1.5x the TTL
		// after its last use (worst case: eviction check just missed, then TTL passes,
		// then next tick fires at TTL/2 later).
		ticker := c.timeSource.NewTicker(c.peerTTL / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.Chan():
				c.evictStalePeers()
			case <-c.stopCh:
				return
			}
		}
	}()
}

// noOpSubscriber is a no-op implementation of peer.Subscriber
type noOpSubscriber struct{}

func (*noOpSubscriber) NotifyStatusChanged(peer.Identifier) {}
