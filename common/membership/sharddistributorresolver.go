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

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type modeKey string

var (
	modeKeyHashRing                       modeKey = "hash_ring"
	modeKeyShardDistributor               modeKey = "shard_distributor"
	modeKeyHashRingShadowShardDistributor modeKey = "hash_ring-shadow-shard_distributor"
	modeKeyShardDistributorShadowHashRing modeKey = "shard_distributor-shadow-hash_ring"
)

type shardDistributorResolver struct {
	namespace             string
	shardDistributionMode dynamicconfig.StringPropertyFn
	client                sharddistributor.Client
	ring                  SingleProvider
	logger                log.Logger
}

func (s shardDistributorResolver) AddressToHost(owner string) (HostInfo, error) {
	return s.ring.AddressToHost(owner)
}

func NewShardDistributorResolver(
	namespace string,
	client sharddistributor.Client,
	shardDistributionMode dynamicconfig.StringPropertyFn,
	ring SingleProvider,
	logger log.Logger,
) SingleProvider {
	return &shardDistributorResolver{
		namespace:             namespace,
		client:                client,
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

func (s shardDistributorResolver) LookupRaw(key string) (string, error) {
	switch modeKey(s.shardDistributionMode()) {
	case modeKeyHashRing:
		return s.ring.LookupRaw(key)
	case modeKeyShardDistributor:
		return s.lookUpInShardDistributor(key)
	case modeKeyHashRingShadowShardDistributor:
		hashRingResult, err := s.ring.LookupRaw(key)
		if err != nil {
			return "", err
		}
		shardDistributorResult, err := s.lookUpInShardDistributor(key)
		if err != nil {
			s.logger.Warn("Failed to lookup in shard distributor shadow", tag.Error(err))
		} else if hashRingResult != shardDistributorResult {
			s.logger.Warn("Shadow lookup mismatch", tag.HashRingResult(hashRingResult), tag.ShardDistributorResult(shardDistributorResult))
		}

		return hashRingResult, nil
	case modeKeyShardDistributorShadowHashRing:
		shardDistributorResult, err := s.lookUpInShardDistributor(key)
		if err != nil {
			return "", err
		}
		hashRingResult, err := s.ring.LookupRaw(key)
		if err != nil {
			s.logger.Warn("Failed to lookup in hash ring shadow", tag.Error(err))
		} else if hashRingResult != shardDistributorResult {
			s.logger.Warn("Shadow lookup mismatch", tag.HashRingResult(hashRingResult), tag.ShardDistributorResult(shardDistributorResult))
		}

		return shardDistributorResult, nil
	}

	// Default to hash ring
	s.logger.Warn("Unknown shard distribution mode, defaulting to hash ring", tag.Value(s.shardDistributionMode()))

	return s.ring.LookupRaw(key)
}

func (s shardDistributorResolver) Lookup(key string) (HostInfo, error) {
	owner, err := s.LookupRaw(key)
	if err != nil {
		return HostInfo{}, err
	}

	return s.ring.AddressToHost(owner)
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

func (s shardDistributorResolver) lookUpInShardDistributor(key string) (string, error) {
	request := &types.GetShardOwnerRequest{
		ShardKey:  key,
		Namespace: s.namespace,
	}
	response, err := s.client.GetShardOwner(context.Background(), request)
	if err != nil {
		return "", err
	}

	return response.Owner, nil
}
