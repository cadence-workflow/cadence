// Copyright (c) 2017 Uber Technologies, Inc.
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

package membership

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgryski/go-farm"
	"github.com/uber/ringpop-go/hashring"
	"github.com/uber/ringpop-go/membership"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination peerprovider_mock.go github.com/uber/cadence/common/membership PeerProvider

// ErrInsufficientHosts is thrown when there are not enough hosts to serve the request
var ErrInsufficientHosts = &types.InternalServiceError{Message: "Not enough hosts to serve the request"}

const (
	minRefreshInternal     = time.Second
	defaultRefreshInterval = 2 * time.Second
	replicaPoints          = 100
)

// PeerProvider is used to retrieve membership information from provider
type PeerProvider interface {
	common.Daemon

	GetMembers(service string) ([]HostInfo, error)
	WhoAmI() (HostInfo, error)
	SelfEvict() error
	Subscribe(name string, handler func(ChangedEvent)) error
}

type ring struct {
	status       int32
	service      string
	peerProvider PeerProvider
	refreshChan  chan struct{}
	shutdownCh   chan struct{}
	shutdownWG   sync.WaitGroup
	timeSource   clock.TimeSource
	scope        metrics.Scope
	logger       log.Logger

	value atomic.Value // this stores the current hashring

	members struct {
		sync.RWMutex
		refreshed time.Time
		keys      map[string]HostInfo // for mapping ip:port to HostInfo
	}

	subscribers struct {
		sync.RWMutex
		keys map[string]chan<- *ChangedEvent
	}
}

func newHashring(
	service string,
	provider PeerProvider,
	timeSource clock.TimeSource,
	logger log.Logger,
	scope metrics.Scope,
) *ring {
	r := &ring{
		status:       common.DaemonStatusInitialized,
		service:      service,
		peerProvider: provider,
		shutdownCh:   make(chan struct{}),
		refreshChan:  make(chan struct{}, 1),
		timeSource:   timeSource,
		logger:       logger,
		scope:        scope,
	}

	r.members.keys = make(map[string]HostInfo)
	r.subscribers.keys = make(map[string]chan<- *ChangedEvent)

	r.value.Store(emptyHashring())
	return r
}

func emptyHashring() *hashring.HashRing {
	return hashring.New(farm.Fingerprint32, replicaPoints)
}

// Start starts the hashring
func (r *ring) Start() {
	if !atomic.CompareAndSwapInt32(
		&r.status,
		common.DaemonStatusInitialized,
		common.DaemonStatusStarted,
	) {
		return
	}
	if err := r.peerProvider.Subscribe(r.service, r.handleUpdates); err != nil {
		r.logger.Fatal("subscribing to peer provider", tag.Error(err))
	}

	if err := r.refresh(); err != nil {
		r.logger.Fatal("failed to start service resolver", tag.Error(err))
	}

	r.shutdownWG.Add(1)
	go r.refreshRingWorker()
}

// Stop stops the resolver
func (r *ring) Stop() {
	if !atomic.CompareAndSwapInt32(
		&r.status,
		common.DaemonStatusStarted,
		common.DaemonStatusStopped,
	) {
		return
	}

	r.peerProvider.Stop()
	r.value.Store(emptyHashring())

	r.subscribers.Lock()
	r.subscribers.keys = make(map[string]chan<- *ChangedEvent)
	r.subscribers.Unlock()
	close(r.shutdownCh)

	if success := common.AwaitWaitGroup(&r.shutdownWG, time.Minute); !success {
		r.logger.Warn("service resolver timed out on shutdown.")
	}
}

// Lookup finds the host in the ring responsible for serving the given key
func (r *ring) Lookup(
	key string,
) (HostInfo, error) {
	addr, found := r.ring().Lookup(key)
	if !found {
		r.signalSelf()
		return HostInfo{}, ErrInsufficientHosts
	}

	r.members.RLock()
	defer r.members.RUnlock()
	host, ok := r.members.keys[addr]
	if !ok {
		return HostInfo{}, fmt.Errorf("host not found in member keys, host: %q", addr)
	}
	return host, nil
}

// Subscribe registers callback watcher. Services can use this to be informed about membership changes
func (r *ring) Subscribe(watcher string, notifyChannel chan<- *ChangedEvent) error {
	r.subscribers.Lock()
	defer r.subscribers.Unlock()

	_, ok := r.subscribers.keys[watcher]
	if ok {
		return fmt.Errorf("watcher %q is already subscribed", watcher)
	}

	r.subscribers.keys[watcher] = notifyChannel
	return nil
}

func (r *ring) handleUpdates(event ChangedEvent) {
	r.signalSelf()
}

func (r *ring) signalSelf() {
	var event struct{}
	select {
	case r.refreshChan <- event:
	default: // channel already has an event, don't block
	}
}

func (r *ring) notifySubscribers(msg ChangedEvent) {
	r.subscribers.Lock()
	defer r.subscribers.Unlock()

	for name, ch := range r.subscribers.keys {
		select {
		case ch <- &msg:
		default:
			r.logger.Warn("subscriber notification failed", tag.Name(name))
		}
	}
}

// Unsubscribe removes subscriber
func (r *ring) Unsubscribe(name string) error {
	r.subscribers.Lock()
	defer r.subscribers.Unlock()
	delete(r.subscribers.keys, name)
	return nil
}

// MemberCount returns number of hosts in a ring
func (r *ring) MemberCount() int {
	return r.ring().ServerCount()
}

func (r *ring) Members() []HostInfo {
	servers := r.ring().Servers()

	var hosts = make([]HostInfo, 0, len(servers))
	r.members.RLock()
	defer r.members.RUnlock()
	for _, s := range servers {
		host, ok := r.members.keys[s]
		if !ok {
			r.logger.Warn("host not found in hashring keys", tag.Address(s))
			continue
		}
		hosts = append(hosts, host)
	}

	return hosts
}

func (r *ring) refresh() error {
	if r.members.refreshed.After(r.timeSource.Now().Add(-minRefreshInternal)) {
		// refreshed too frequently
		r.logger.Debug("refresh skipped, refreshed too frequently")
		return nil
	}

	members, err := r.peerProvider.GetMembers(r.service)
	if err != nil {
		return fmt.Errorf("getting members from peer provider: %w", err)
	}

	newMembersMap := r.makeMembersMap(members)

	diff := r.diffMembers(newMembersMap)
	if diff.Empty() {
		return nil
	}

	ring := emptyHashring()
	ring.AddMembers(castToMembers(members)...)
	r.value.Store(ring)
	// sort members for deterministic order in the logs
	sort.Slice(members, func(i, j int) bool { return members[i].addr < members[j].addr })
	r.logger.Info("refreshed ring members", tag.Value(members), tag.Counter(len(members)), tag.Service(r.service))

	r.updateMembersMap(newMembersMap)

	r.emitHashIdentifier()
	r.notifySubscribers(diff)

	return nil
}

func (r *ring) updateMembersMap(newMembers map[string]HostInfo) {
	r.members.Lock()
	defer r.members.Unlock()

	r.members.keys = newMembers
	r.members.refreshed = r.timeSource.Now()
}

func (r *ring) refreshRingWorker() {
	defer r.shutdownWG.Done()

	refreshTicker := r.timeSource.NewTicker(defaultRefreshInterval)
	defer refreshTicker.Stop()

	for {
		select {
		case <-r.shutdownCh:
			return
		case <-r.refreshChan: // local signal or signal from provider
			if err := r.refresh(); err != nil {
				r.logger.Error("failed to refresh ring", tag.Error(err))
			}
		case <-refreshTicker.Chan(): // periodically force refreshing membership
			r.signalSelf()
		}
	}
}

func (r *ring) ring() *hashring.HashRing {
	return r.value.Load().(*hashring.HashRing)
}

func (r *ring) emitHashIdentifier() float64 {
	members := r.Members()
	self, err := r.peerProvider.WhoAmI()
	if err != nil {
		r.logger.Error("Observed a problem looking up self from the membership provider while emitting hash identifier metrics", tag.Error(err))
		self = HostInfo{
			identity: "unknown",
		}
	}

	sort.Slice(members, func(i int, j int) bool {
		return members[i].addr > members[j].addr
	})
	var sb strings.Builder
	for i := range members {
		sb.WriteString(members[i].addr)
		sb.WriteString("\n")
	}
	hashedView := farm.Hash32([]byte(sb.String()))
	// Trimming the metric because collisions are unlikely and I didn't want to use the full Float64
	// in-case it overflowed something. The number itself is meaningless, so additional precision
	// doesn't really give any advantage, besides reducing the risk of collision
	trimmedForMetric := float64(hashedView % 1000)
	r.logger.Debug("Hashring view",
		tag.Dynamic("hashring-view", sb.String()),
		tag.Dynamic("trimmed-hash-id", trimmedForMetric),
		tag.Service(r.service),
		tag.Dynamic("self-addr", self.addr),
		tag.Dynamic("self-identity", self.identity),
		tag.Dynamic("self-ip", self.ip),
	)
	r.scope.Tagged(
		metrics.ServiceTag(r.service),
		metrics.HostTag(self.identity),
	).UpdateGauge(metrics.HashringViewIdentifier, trimmedForMetric)
	return trimmedForMetric
}

func (r *ring) makeMembersMap(members []HostInfo) map[string]HostInfo {
	membersMap := make(map[string]HostInfo, len(members))
	for _, m := range members {
		membersMap[m.GetAddress()] = m
	}
	return membersMap
}

func (r *ring) diffMembers(newMembers map[string]HostInfo) ChangedEvent {
	r.members.RLock()
	defer r.members.RUnlock()

	var combinedChange ChangedEvent

	// find newly added hosts
	for addr := range newMembers {
		if _, found := r.members.keys[addr]; !found {
			combinedChange.HostsAdded = append(combinedChange.HostsAdded, addr)
		}
	}
	// find removed hosts
	for addr := range r.members.keys {
		if _, found := newMembers[addr]; !found {
			combinedChange.HostsRemoved = append(combinedChange.HostsRemoved, addr)
		}
	}

	// order since it will most probably used in logs
	slices.Sort(combinedChange.HostsAdded)
	slices.Sort(combinedChange.HostsUpdated)
	slices.Sort(combinedChange.HostsRemoved)
	return combinedChange
}

func castToMembers[T membership.Member](members []T) []membership.Member {
	result := make([]membership.Member, 0, len(members))
	for _, h := range members {
		result = append(result, h)
	}
	return result
}
