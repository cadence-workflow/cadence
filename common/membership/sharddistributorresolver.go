// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package membership

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

type shardDistributorResolver struct {
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn
	emergencyOffboarding       dynamicproperties.BoolPropertyFn
	percentageOnboarded        dynamicproperties.IntPropertyFn
	spectator                  spectatorclient.Spectator
	ring                       SingleProvider
	logger                     log.Logger
}

func (s shardDistributorResolver) AddressToHost(owner string) (HostInfo, error) {
	return s.ring.AddressToHost(owner)
}

func NewShardDistributorResolver(
	spectator spectatorclient.Spectator,
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn,
	percentageOnboarded dynamicproperties.IntPropertyFn,
	emergencyOffboarding dynamicproperties.BoolPropertyFn,
	ring SingleProvider,
	logger log.Logger,
) SingleProvider {
	return &shardDistributorResolver{
		spectator:                  spectator,
		excludeShortLivedTaskLists: excludeShortLivedTaskLists,
		percentageOnboarded:        percentageOnboarded,
		emergencyOffboarding:       emergencyOffboarding,
		ring:                       ring,
		logger:                     logger,
	}
}

func (s shardDistributorResolver) Start() {
	// We do not need to start anything in the shard distributor, so just start the ring
	s.ring.Start()
}

func (s shardDistributorResolver) Stop() {
	// We do not need to stop anything in the shard distributor, so just stop the ring
	s.ring.Stop()
}

func (s shardDistributorResolver) Lookup(key string) (HostInfo, error) {
	if s.emergencyOffboarding() {
		return s.ring.Lookup(key)
	}
	excludeTaskList := TaskListExcludedFromShardDistributor(key, uint64(s.percentageOnboarded()), s.excludeShortLivedTaskLists())
	if excludeTaskList {
		return s.ring.Lookup(key)
	}

	if s.spectator == nil {
		s.logger.Warn("No shard distributor client, defaulting to hash ring")
		return s.ring.Lookup(key)
	}

	return s.lookUpInShardDistributor(key)
}

// LookupN delegates to the underlying hash ring; the shard distributor does
// not yet support multi-host lookup.
func (s shardDistributorResolver) LookupN(key string, n int) ([]HostInfo, error) {
	return s.ring.LookupN(key, n)
}

func (s shardDistributorResolver) Subscribe(name string, channel chan<- *ChangedEvent) error {
	// Shard distributor does not support subscription yet, so use the ring
	return s.ring.Subscribe(name, channel)
}

func (s shardDistributorResolver) Unsubscribe(name string) error {
	// Shard distributor does not support subscription yet, so use the ring
	return s.ring.Unsubscribe(name)
}

func (s shardDistributorResolver) Members() []HostInfo {
	// Shard distributor does not member tracking yet, so use the ring
	return s.ring.Members()
}

func (s shardDistributorResolver) MemberCount() int {
	// Shard distributor does not member tracking yet, so use the ring
	return s.ring.MemberCount()
}

func (s shardDistributorResolver) Refresh() error {
	// Shard distributor does not need refresh, so propagate to the ring
	return s.ring.Refresh()
}

// TODO: cache the hostinfos, creating them on every request is relatively expensive (we need to do string parsing etc.)
func (s shardDistributorResolver) lookUpInShardDistributor(key string) (HostInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	owner, err := s.spectator.GetShardOwner(ctx, key)
	if err != nil {
		return HostInfo{}, err
	}

	portMap := make(PortMap)
	for _, portType := range []string{PortTchannel, PortGRPC} {
		portString, ok := owner.Metadata[portType]
		if !ok {
			s.logger.Warn(fmt.Sprintf("Port %s not found in metadata", portType), tag.Value(owner))
			continue
		}

		port, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("Failed to convert %s port to int", portType), tag.Error(err), tag.Value(owner))
			continue
		}
		// This is safe because the conversion above ensures the port is within the uint16 range
		portMap[portType] = uint16(port)
	}

	hostaddress := owner.Metadata["hostIP"]
	if hostaddress == "" {
		return HostInfo{}, fmt.Errorf("hostIP not found in metadata")
	}

	address := net.JoinHostPort(hostaddress, strconv.Itoa(int(portMap[PortTchannel])))

	hostInfo := NewDetailedHostInfo(address, owner.ExecutorID, portMap)
	return hostInfo, nil
}
