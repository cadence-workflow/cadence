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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination resolver_mock.go -self_package github.com/uber/cadence/common/membership

// Package membership provides service discovery and membership information mechanism
package membership

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

type (

	// ChangedEvent describes a change in a membership ring
	ChangedEvent struct {
		HostsAdded   []string
		HostsUpdated []string
		HostsRemoved []string
	}

	// Resolver provides membership information for all cadence services.
	Resolver interface {
		common.Daemon

		// WhoAmI returns self host details.
		// To be consistent with peer provider, it is advised to use peer provider
		// to return this information
		WhoAmI() (HostInfo, error)

		// EvictSelf evicts this member from the membership ring. After this method is
		// called, other members should discover that this node is no longer part of the
		// ring.
		// This primitive is useful to carry out graceful host shutdown during deployments.
		EvictSelf() error

		// Lookup will return host which is an owner for provided key.
		Lookup(service, key string) (HostInfo, error)

		// Subscribe adds a subscriber which will get detailed change data on the given
		// channel, whenever membership changes.
		Subscribe(service, name string, notifyChannel chan<- *ChangedEvent) error

		// Unsubscribe removes a subscriber for this service.
		Unsubscribe(service, name string) error

		// MemberCount returns host count in a service specific hashring
		MemberCount(service string) (int, error)

		// Members returns all host addresses in a service specific hashring
		Members(service string) ([]HostInfo, error)

		// LookupByAddress returns Host which owns IP:port tuple
		LookupByAddress(service, address string) (HostInfo, error)
	}
)

// MultiringResolver uses ring-per-service for membership information
type MultiringResolver struct {
	metrics metrics.Client
	status  int32

	provider PeerProvider
	mu       sync.Mutex
	rings    map[string]*ring
}

var _ Resolver = (*MultiringResolver)(nil)

// NewResolver builds hashrings for all services
func NewResolver(
	provider PeerProvider,
	logger log.Logger,
	metrics metrics.Client,
) (*MultiringResolver, error) {
	return NewMultiringResolver(service.ListWithRing, provider, logger.WithTags(tag.ComponentServiceResolver), metrics), nil
}

// NewMultiringResolver creates hashrings for all services
func NewMultiringResolver(
	services []string,
	provider PeerProvider,
	logger log.Logger,
	metricsClient metrics.Client,
) *MultiringResolver {
	rpo := &MultiringResolver{
		status:   common.DaemonStatusInitialized,
		provider: provider,
		rings:    make(map[string]*ring),
		metrics:  metricsClient,
		mu:       sync.Mutex{},
	}

	for _, s := range services {
		rpo.rings[s] = newHashring(s, provider, clock.NewRealTimeSource(), logger, metricsClient.Scope(metrics.HashringScope))
	}
	return rpo
}

// Start starts provider and all rings
func (rpo *MultiringResolver) Start() {
	if !atomic.CompareAndSwapInt32(
		&rpo.status,
		common.DaemonStatusInitialized,
		common.DaemonStatusStarted,
	) {
		return
	}

	rpo.provider.Start()

	rpo.mu.Lock()
	defer rpo.mu.Unlock()
	for _, ring := range rpo.rings {
		ring.Start()
	}
}

// Stop stops all rings and membership provider
func (rpo *MultiringResolver) Stop() {
	if !atomic.CompareAndSwapInt32(
		&rpo.status,
		common.DaemonStatusStarted,
		common.DaemonStatusStopped,
	) {
		return
	}

	rpo.mu.Lock()
	defer rpo.mu.Unlock()
	for _, ring := range rpo.rings {
		ring.Stop()
	}

	rpo.provider.Stop()
}

// WhoAmI asks to provide current instance address
func (rpo *MultiringResolver) WhoAmI() (HostInfo, error) {
	return rpo.provider.WhoAmI()
}

// EvictSelf is used to remove this host from membership ring
func (rpo *MultiringResolver) EvictSelf() error {
	return rpo.provider.SelfEvict()
}

func (rpo *MultiringResolver) getRing(service string) (*ring, error) {
	rpo.mu.Lock()
	defer rpo.mu.Unlock()
	ring, found := rpo.rings[service]
	if !found {
		return nil, fmt.Errorf("service %q is not tracked by Resolver", service)
	}
	return ring, nil
}

func (rpo *MultiringResolver) Lookup(service string, key string) (HostInfo, error) {
	ring, err := rpo.getRing(service)
	if err != nil {
		return HostInfo{}, err
	}
	return ring.Lookup(key)
}

func (rpo *MultiringResolver) Subscribe(service string, name string, notifyChannel chan<- *ChangedEvent) error {
	ring, err := rpo.getRing(service)
	if err != nil {
		return err
	}
	return ring.Subscribe(name, notifyChannel)
}

func (rpo *MultiringResolver) Unsubscribe(service string, name string) error {
	ring, err := rpo.getRing(service)
	if err != nil {
		return err
	}
	return ring.Unsubscribe(name)
}

func (rpo *MultiringResolver) Members(service string) ([]HostInfo, error) {
	ring, err := rpo.getRing(service)
	if err != nil {
		return nil, err
	}
	return ring.Members(), nil
}

func (rpo *MultiringResolver) LookupByAddress(service, address string) (HostInfo, error) {
	members, err := rpo.Members(service)
	if err != nil {
		return HostInfo{}, err
	}
	for _, m := range members {
		if belongs, err := m.Belongs(address); err == nil && belongs {
			return m, nil
		}
	}
	rpo.metrics.Scope(metrics.ResolverHostNotFoundScope).IncCounter(1)
	return HostInfo{}, fmt.Errorf("host not found in service %s: %s", service, address)
}

func (rpo *MultiringResolver) MemberCount(service string) (int, error) {
	ring, err := rpo.getRing(service)
	if err != nil {
		return 0, err
	}
	return ring.MemberCount(), nil
}

func (ce *ChangedEvent) Empty() bool {
	return len(ce.HostsAdded) == 0 && len(ce.HostsUpdated) == 0 && len(ce.HostsRemoved) == 0
}
