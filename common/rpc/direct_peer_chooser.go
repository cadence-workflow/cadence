// Copyright (c) 2021 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package rpc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer/direct"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
)

var (
	noOpSubscriberInstance = &noOpSubscriber{}
)

// directPeerChooser is a peer.Chooser that chooses a peer based on the shard key.
// Peers are managed by the peerList and peers are reused across multiple requests.
type directPeerChooser struct {
	status               int32
	serviceName          string
	logger               log.Logger
	scope                metrics.Scope
	t                    peer.Transport
	enableConnRetainMode dynamicconfig.BoolPropertyFn
	legacyChooser        peer.Chooser
	legacyChooserErr     error
	mu                   sync.RWMutex
	peers                map[string]peer.Peer
}

func newDirectChooser(
	serviceName string,
	t peer.Transport,
	logger log.Logger,
	metricsCl metrics.Client,
	enableConnRetainMode dynamicconfig.BoolPropertyFn,
) *directPeerChooser {
	dpc := &directPeerChooser{
		serviceName:          serviceName,
		logger:               logger.WithTags(tag.DestService(serviceName)),
		scope:                metricsCl.Scope(metrics.P2PRPCPeerChooserScope).Tagged(metrics.DestServiceTag(serviceName)),
		t:                    t,
		enableConnRetainMode: enableConnRetainMode,
		peers:                make(map[string]peer.Peer),
	}

	if dpc.enableConnRetainMode == nil {
		dpc.enableConnRetainMode = func(opts ...dynamicconfig.FilterOption) bool { return false }
	}

	return dpc
}

// Start statisfies the peer.Chooser interface.
func (g *directPeerChooser) Start() (err error) {
	if !atomic.CompareAndSwapInt32(&g.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return nil
	}

	defer func() {
		if err != nil {
			g.logger.Error("direct peer chooser failed to start", tag.Error(err))
			return
		}
		g.logger.Info("direct peer chooser started")
	}()

	if !g.enableConnRetainMode() {
		c, ok := g.getLegacyChooser()
		if ok {
			return c.Start()
		}

		return fmt.Errorf("failed to start direct peer chooser because direct peer chooser initialization failed, err: %v", g.legacyChooserErr)
	}

	return nil
}

// Stop statisfies the peer.Chooser interface.
func (g *directPeerChooser) Stop() error {
	if !atomic.CompareAndSwapInt32(&g.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return nil
	}

	var err error
	// Stop legacy chooser if it was initialized
	if g.legacyChooser != nil {
		err = g.legacyChooser.Stop()
	}

	// Release all peers if there's any
	g.updatePeersInternal(nil)

	g.logger.Info("direct peer chooser stopped", tag.Error(err))
	return err
}

// IsRunning statisfies the peer.Chooser interface.
func (g *directPeerChooser) IsRunning() bool {
	if atomic.LoadInt32(&g.status) != common.DaemonStatusStarted {
		return false
	}

	if !g.enableConnRetainMode() {
		c, ok := g.getLegacyChooser()
		if ok {
			return c.IsRunning()
		}
		return false
	}

	return true // no-op
}

// Choose returns an existing peer for the shard key.
// ShardKey is {host}:{port} of the peer. It could be tchannel or grpc address.
func (g *directPeerChooser) Choose(ctx context.Context, req *transport.Request) (peer peer.Peer, onFinish func(error), err error) {
	if !g.enableConnRetainMode() {
		return g.chooseFromLegacyDirectPeerChooser(ctx, req)
	}

	if req.ShardKey == "" {
		return nil, nil, yarpcerrors.InvalidArgumentErrorf("chooser requires ShardKey to be non-empty")
	}

	g.mu.RLock()
	p, ok := g.peers[req.ShardKey]
	if ok {
		g.mu.RUnlock()
		return p, func(error) {}, nil
	}
	g.mu.RUnlock()

	// peer is not cached, add new peer
	p, err = g.addPeer(req.ShardKey)
	if err != nil {
		return nil, nil, yarpcerrors.InternalErrorf("failed to add peer for shard key %v, err: %v", req.ShardKey, err)
	}

	return p, func(error) {}, nil
}

// UpdatePeers removes peers that are not in the members list.
// Do not create actual yarpc peers for the members. They are created lazily when a request comes in (Choose is called).
func (g *directPeerChooser) UpdatePeers(serviceName string, members []membership.HostInfo) {
	if g.serviceName != serviceName {
		g.logger.Debug("This is not the service chooser is created for. Ignore such updates.", tag.Dynamic("members-service", serviceName))
		return
	}

	g.logger.Debug("direct peer chooser got a membership update", tag.Counter(len(members)))

	// If the chooser is not started, do not act on membership changes.
	// If membership updates arrive after chooser is stopped, ignore them.
	if atomic.LoadInt32(&g.status) != common.DaemonStatusStarted {
		return
	}

	g.updatePeersInternal(members)
}

func (g *directPeerChooser) updatePeersInternal(members []membership.HostInfo) {
	// Create a map of valid peer addresses given members list.
	validPeerAddresses := make(map[string]bool)
	for _, member := range members {
		for _, portName := range []string{membership.PortTchannel, membership.PortGRPC} {
			addr, err := member.GetNamedAddress(portName)
			if err != nil {
				g.logger.Error(fmt.Sprintf("failed to get %s address of member", portName), tag.Error(err), tag.Address(member.GetAddress()))
				continue
			}
			validPeerAddresses[addr] = true
		}
	}

	// Take a copy of the current peers to avoid keeping write lock while removing all peers.
	peers := make(map[string]bool)
	g.mu.RLock()
	for addr := range g.peers {
		peers[addr] = true
	}
	g.mu.RUnlock()

	g.logger.Debugf("valid peers: %v, current peers: %v", validPeerAddresses, peers)

	for addr := range peers {
		if !validPeerAddresses[addr] {
			g.removePeer(addr)
		}
	}
}

func (g *directPeerChooser) removePeer(addr string) {
	g.mu.RLock()
	if err := g.t.ReleasePeer(g.peers[addr], noOpSubscriberInstance); err != nil {
		g.logger.Error("failed to release peer", tag.Error(err), tag.Address(addr))
	}
	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.peers, addr)
	g.logger.Info("removed peer from direct peer chooser", tag.Address(addr))
	g.scope.IncCounter(metrics.P2PPeerRemoved)
	g.scope.UpdateGauge(metrics.P2PPeersCount, float64(len(g.peers)))
}

func (g *directPeerChooser) addPeer(addr string) (peer.Peer, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if p, ok := g.peers[addr]; ok {
		return p, nil
	}

	p, err := g.t.RetainPeer(hostport.Identify(addr), noOpSubscriberInstance)
	if err != nil {
		return nil, err
	}
	g.peers[addr] = p
	g.logger.Info("added peer to direct peer chooser", tag.Address(addr))
	g.scope.IncCounter(metrics.P2PPeerAdded)
	g.scope.UpdateGauge(metrics.P2PPeersCount, float64(len(g.peers)))
	return p, nil
}

func (g *directPeerChooser) chooseFromLegacyDirectPeerChooser(ctx context.Context, req *transport.Request) (peer.Peer, func(error), error) {
	c, ok := g.getLegacyChooser()
	if !ok {
		return nil, nil, yarpcerrors.InternalErrorf("failed to get legacy direct peer chooser, err: %v", g.legacyChooserErr)
	}

	return c.Choose(ctx, req)
}

func (g *directPeerChooser) getLegacyChooser() (peer.Chooser, bool) {
	g.mu.RLock()

	if g.legacyChooser != nil {
		// Legacy chooser already created, return it
		g.mu.RUnlock()
		return g.legacyChooser, true
	}

	if g.legacyChooserErr != nil {
		// There was an error creating the legacy chooser, return false
		g.mu.RUnlock()
		return nil, false
	}

	g.mu.RUnlock()

	g.mu.Lock()
	g.legacyChooser, g.legacyChooserErr = direct.New(direct.Configuration{}, g.t)
	g.mu.Unlock()

	if g.legacyChooserErr != nil {
		g.logger.Error("failed to create legacy direct peer chooser", tag.Error(g.legacyChooserErr))
		return nil, false
	}

	if atomic.LoadInt32(&g.status) == common.DaemonStatusStarted {
		// Start the legacy chooser if the current chooser is already started
		if err := g.legacyChooser.Start(); err != nil {
			g.logger.Error("failed to start legacy direct peer chooser", tag.Error(err))
			return nil, false
		}
	}

	return g.legacyChooser, true
}

// noOpSubscriber is a no-op implementation of peer.Subscriber
type noOpSubscriber struct{}

func (*noOpSubscriber) NotifyStatusChanged(peer.Identifier) {}
