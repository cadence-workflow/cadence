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
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

const (
	NamespaceHeader           = "x-shard-distributor-namespace"
	peerChooserSubscriberName = "peer-chooser"
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

	peersMutex      sync.RWMutex
	peers           map[string]peer.Peer           // grpc_address -> peer
	grpcAddressToNs map[string]map[string]struct{} // grpc_address -> set of namespaces that have executors at this address

	stopCh chan struct{}
	stopWG sync.WaitGroup
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
		transport:       params.Transport,
		logger:          params.Logger,
		peers:           make(map[string]peer.Peer),
		grpcAddressToNs: make(map[string]map[string]struct{}),
		stopCh:          make(chan struct{}),
	}
}

// Start satisfies the peer.Chooser interface
func (c *SpectatorPeerChooser) Start() error {
	c.logger.Info("Starting shard distributor peer chooser", tag.ShardNamespace(c.namespace))

	// Subscribe to executor updates from all spectators
	if c.spectators != nil {
		for namespace, spectator := range c.spectators.spectators {
			ch, err := spectator.Subscribe(peerChooserSubscriberName)
			if err != nil {
				c.logger.Error("Failed to subscribe to spectator updates", tag.Error(err), tag.ShardNamespace(namespace))
				continue
			}

			// Start a goroutine to listen for updates
			c.stopWG.Add(1)
			go c.watchExecutorUpdates(namespace, spectator, ch)
		}
	}

	return nil
}

// Stop satisfies the peer.Chooser interface
func (c *SpectatorPeerChooser) Stop() error {
	c.logger.Info("Stopping shard distributor peer chooser", tag.ShardNamespace(c.namespace))

	// Signal all watch goroutines to stop (if stopCh was initialized)
	if c.stopCh != nil {
		close(c.stopCh)
	}

	// Unsubscribe from all spectators
	if c.spectators != nil {
		for namespace, spectator := range c.spectators.spectators {
			if err := spectator.Unsubscribe(peerChooserSubscriberName); err != nil {
				c.logger.Warn("Failed to unsubscribe from spectator", tag.Error(err), tag.ShardNamespace(namespace))
			}
		}
	}

	// Wait for all watch goroutines to finish
	c.stopWG.Wait()

	// Release all peers
	c.peersMutex.Lock()
	defer c.peersMutex.Unlock()

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
	peer, err = c.getOrCreatePeer(grpcAddress, namespace)
	if err != nil {
		return nil, nil, yarpcerrors.InternalErrorf("get or create peer for address %s: %v", grpcAddress, err)
	}

	return peer, func(error) {}, nil
}

func (c *SpectatorPeerChooser) SetSpectators(spectators *Spectators) {
	c.spectators = spectators
}

func (c *SpectatorPeerChooser) getOrCreatePeer(grpcAddress, namespace string) (peer.Peer, error) {
	c.peersMutex.Lock()
	defer c.peersMutex.Unlock()

	// Check if peer already exists
	if p, ok := c.peers[grpcAddress]; ok {
		// Add namespace to the set of namespaces using this address
		if c.grpcAddressToNs[grpcAddress] == nil {
			c.grpcAddressToNs[grpcAddress] = make(map[string]struct{})
		}
		c.grpcAddressToNs[grpcAddress][namespace] = struct{}{}
		return p, nil
	}

	// Create new peer for this address
	p, err := c.transport.RetainPeer(hostport.Identify(grpcAddress), &noOpSubscriber{})
	if err != nil {
		return nil, fmt.Errorf("retain peer: %w", err)
	}

	// Cache the peer for future use
	c.peers[grpcAddress] = p
	c.grpcAddressToNs[grpcAddress] = map[string]struct{}{namespace: {}}

	return p, nil
}

// watchExecutorUpdates listens for executor updates and releases stale peers
func (c *SpectatorPeerChooser) watchExecutorUpdates(namespace string, spectator Spectator, updateCh <-chan struct{}) {
	defer c.stopWG.Done()

	c.logger.Info("Started watching executor updates", tag.ShardNamespace(namespace))

	for {
		select {
		case <-c.stopCh:
			c.logger.Info("Stopped watching executor updates", tag.ShardNamespace(namespace))
			return
		case _, ok := <-updateCh:
			if !ok {
				c.logger.Info("Update channel closed, stopping watch", tag.ShardNamespace(namespace))
				return
			}
			c.logger.Debug("Received executor update notification", tag.ShardNamespace(namespace))
			c.removeStaleExecutors(namespace, spectator)
		}
	}
}

// removeStaleExecutors releases peers for executors that are no longer in any namespace.
// It updates the grpcAddressToNs map based on the current executors in the given namespace,
// and only releases a peer if no namespace is using that address anymore.
func (c *SpectatorPeerChooser) removeStaleExecutors(namespace string, spectator Spectator) {
	newGrpcAddresses := c.getSpectatorGrpcAddresses(spectator)

	c.peersMutex.Lock()
	defer c.peersMutex.Unlock()

	// Update grpcAddressToNs: remove this namespace from addresses it no longer uses
	for addr, namespaces := range c.grpcAddressToNs {
		if _, ok := namespaces[namespace]; !ok {
			// This address was not previously associated with this namespace, so we can skip it
			continue
		}

		if _, ok := newGrpcAddresses[addr]; ok {
			// This address is still used by this namespace, so we can skip it
			continue
		}

		c.logger.Info("Namespace no longer uses this address", tag.Address(addr), tag.ShardNamespace(namespace))
		delete(namespaces, namespace)
	}

	for addr, p := range c.peers {
		// If there are still namespaces using this address, skip releasing the peer
		if len(c.grpcAddressToNs[addr]) > 0 {
			continue
		}

		if err := c.transport.ReleasePeer(p, &noOpSubscriber{}); err != nil {
			c.logger.Warn("Failed to release stale peer", tag.Error(err), tag.Address(addr))

			// If releasing the peer fails, we keep it in the map to avoid losing connectivity to the executor.
			// We will try releasing it again on the next update.
			// If the peer will be returned by getOrCreatePeer, grpcAddressToNs will be updated with the correct namespaces,
			// so we won't leak peers indefinitely.
			continue
		}

		delete(c.peers, addr)
		delete(c.grpcAddressToNs, addr)

		c.logger.Info("Released stale peer (no namespaces using it)", tag.Address(addr))
	}
}

// getSpectatorGrpcAddresses returns the set of grpc addresses for the current executors in the given spectator's namespace.
// This is used to determine which addresses are still in use by the namespace and which ones can potentially be released if they are no longer used by any namespace.
func (c *SpectatorPeerChooser) getSpectatorGrpcAddresses(spectator Spectator) map[string]struct{} {
	// Get current executors for this namespace
	executors := spectator.GetExecutors()

	// Build set of current grpc addresses for this namespace
	grpcAddresses := make(map[string]struct{})
	for _, owner := range executors {
		if grpcAddr, ok := owner.Metadata[clientcommon.GrpcAddressMetadataKey]; ok {
			grpcAddresses[grpcAddr] = struct{}{}
		}
	}

	return grpcAddresses
}

// noOpSubscriber is a no-op implementation of peer.Subscriber
type noOpSubscriber struct{}

func (*noOpSubscriber) NotifyStatusChanged(peer.Identifier) {}
