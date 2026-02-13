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

type modeKey string

var (
	modeKeyHashRing                       modeKey = "hash_ring"
	modeKeyShardDistributor               modeKey = "shard_distributor"
	modeKeyHashRingShadowShardDistributor modeKey = "hash_ring-shadow-shard_distributor"
	modeKeyShardDistributorShadowHashRing modeKey = "shard_distributor-shadow-hash_ring"
)

type shardDistributorResolver struct {
	shardDistributionMode dynamicproperties.StringPropertyFn
	spectator             spectatorclient.Spectator
	ring                  SingleProvider
	logger                log.Logger
}

func (s shardDistributorResolver) AddressToHost(owner string) (HostInfo, error) {
	return s.ring.AddressToHost(owner)
}

func NewShardDistributorResolver(
	spectator spectatorclient.Spectator,
	shardDistributionMode dynamicproperties.StringPropertyFn,
	ring SingleProvider,
	logger log.Logger,
) SingleProvider {
	return &shardDistributorResolver{
		spectator:             spectator,
		shardDistributionMode: shardDistributionMode,
		ring:                  ring,
		logger:                logger,
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
	if s.shardDistributionMode() != "hash_ring" && s.spectator == nil {
		// This will avoid panics when the shard distributor is not configured
		s.logger.Warn("No shard distributor client, defaulting to hash ring", tag.Value(s.shardDistributionMode()))

		return s.ring.Lookup(key)
	}

	switch modeKey(s.shardDistributionMode()) {
	case modeKeyHashRing:
		return s.ring.Lookup(key)
	case modeKeyShardDistributor:
		return s.lookUpInShardDistributor(key)
	case modeKeyHashRingShadowShardDistributor:
		hashRingResult, errRing := s.ring.Lookup(key)
		shardDistributorResult, errSD := s.lookUpInShardDistributor(key)

		// Compare if no errors happen
		if errRing == nil && errSD == nil {
			logDifferencesInHostInfo(s.logger, hashRingResult, shardDistributorResult)
		}
		// Log failures form SD
		if errSD != nil {
			s.logger.Warn("Failed to lookup in shard distributor shadow", tag.Error(errSD))
		}
		// Fallback to ShardDistributor if HashRing fails
		if errRing != nil {

			return shardDistributorResult, errSD
		}

		return hashRingResult, errRing
	case modeKeyShardDistributorShadowHashRing:
		shardDistributorResult, errSD := s.lookUpInShardDistributor(key)
		hashRingResult, errRing := s.ring.Lookup(key)

		// Compare if no errors happen
		if errRing == nil && errSD == nil {
			logDifferencesInHostInfo(s.logger, hashRingResult, shardDistributorResult)
		}
		// Log failures form HashRing
		if errRing != nil {
			s.logger.Warn("Failed to lookup in hash ring shadow", tag.Error(errRing))
		}
		// Fallback to HashRing if ShardDistributor fails
		if errSD != nil {
			return hashRingResult, errRing
		}

		return shardDistributorResult, errSD
	}

	// Default to hash ring
	s.logger.Warn("Unknown shard distribution mode, defaulting to hash ring", tag.Value(s.shardDistributionMode()))

	return s.ring.Lookup(key)
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

func logDifferencesInHostInfo(logger log.Logger, hashRingResult, shardDistributorResult HostInfo) {
	for _, portName := range []string{PortTchannel, PortGRPC} {
		hashRingAddr, err := hashRingResult.GetNamedAddress(portName)
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to get hash ring %s address", portName), tag.Error(err))
			continue
		}

		shardDistributorAddr, err := shardDistributorResult.GetNamedAddress(portName)
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to get shard distributor %s address", portName), tag.Error(err))
			continue
		}

		if hashRingAddr != shardDistributorAddr {
			logger.Warn(fmt.Sprintf("Shadow lookup mismatch %s addresses", portName),
				tag.HashRingResult(hashRingAddr),
				tag.ShardDistributorResult(shardDistributorAddr))
		}
	}
}
