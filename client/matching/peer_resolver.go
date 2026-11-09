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

package matching

import (
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/semaphore"
	"github.com/uber/cadence/common/service"
)

// PeerResolver is used to resolve matching peers.
// Those are deployed instances of Cadence matching services that participate in the cluster ring.
// The resulting peer is simply an address of form ip:port where RPC calls can be routed to.
//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination peer_resolver_mock.go -package matching github.com/uber/cadence/client/matching PeerResolver

type PeerResolver interface {
	FromTaskList(taskListName string) (string, error)
	FromSemaphoreBucket(domainID, semaphoreName string, bucket int) (string, error)
	GetAllPeers() ([]string, error)
	FromHostAddress(hostAddress string) (string, error)
}

type peerResolver struct {
	resolver  membership.Resolver
	namedPort string // grpc or tchannel, depends on yarpc configuration
}

// NewPeerResolver creates a new matching peer resolver.
func NewPeerResolver(membership membership.Resolver, namedPort string) PeerResolver {
	return peerResolver{
		resolver:  membership,
		namedPort: namedPort,
	}
}

// FromTaskList resolves the matching peer responsible for the given task list name.
// It uses our membership provider to lookup which instance currently owns the given task list.
// FromHostAddress is used for further resolving.
func (pr peerResolver) FromTaskList(taskListName string) (string, error) {
	host, err := pr.resolver.Lookup(service.Matching, taskListName)
	if err != nil {
		return "", common.ToServiceTransientError(err)
	}

	peer, err := host.GetNamedAddress(pr.namedPort)
	return peer, common.ToServiceTransientError(err)
}

// FromSemaphoreBucket returns the address of the Matching host that owns the given semaphore
// bucket.
func (pr peerResolver) FromSemaphoreBucket(domainID, semaphoreName string, bucket int) (string, error) {
	host, err := pr.resolver.Lookup(service.Matching, semaphore.RingKey(domainID, semaphoreName, bucket))
	if err != nil {
		return "", common.ToServiceTransientError(err)
	}

	peer, err := host.GetNamedAddress(pr.namedPort)
	return peer, common.ToServiceTransientError(err)
}

// GetAllPeers returns all matching service peers in the cluster ring.
func (pr peerResolver) GetAllPeers() ([]string, error) {
	hosts, err := pr.resolver.Members(service.Matching)
	if err != nil {
		return nil, common.ToServiceTransientError(err)
	}
	peers := make([]string, 0, len(hosts))
	for _, host := range hosts {
		peer, err := pr.FromHostAddress(host.GetAddress())
		if err != nil {
			return nil, common.ToServiceTransientError(err)
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

// FromHostAddress resolves the final matching peer responsible for the given host address.
// The address may be used as is, or processed with additional address mapper.
// In case of gRPC transport, the port within the address is replaced with gRPC port.
func (pr peerResolver) FromHostAddress(hostAddress string) (string, error) {
	host, err := pr.resolver.LookupByAddress(service.Matching, hostAddress)
	if err != nil {
		return "", common.ToServiceTransientError(err)
	}

	peer, err := host.GetNamedAddress(pr.namedPort)
	return peer, common.ToServiceTransientError(err)
}
