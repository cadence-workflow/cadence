// Copyright (c) 2022 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination sharded_nosql_store_mock.go

package nosql

import (
	"fmt"
	"sync"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
)

type shardedNosqlStore interface {
	GetStoreShardByHistoryShard(shardID int) (*nosqlStore, error)
	GetStoreShardByTaskList(domainID string, taskListName string, taskType int) (*nosqlStore, error)
	GetDefaultShard() nosqlStore
	Close()
	GetName() string
	GetShardingPolicy() shardingPolicy
	GetLogger() log.Logger
}

// shardedNosqlStore is a store that may have one or more shards
type shardedNosqlStoreImpl struct {
	sync.RWMutex

	config config.ShardedNoSQL
	dc     *persistence.DynamicConfiguration
	logger log.Logger

	connectedShards map[string]nosqlStore
	defaultShard    nosqlStore
	shardingPolicy  shardingPolicy
}

func newShardedNosqlStore(cfg config.ShardedNoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (shardedNosqlStore, error) {
	sn := shardedNosqlStoreImpl{
		config: cfg,
		dc:     dc,
		logger: logger,
	}

	// Connect to the default shard
	defaultShardName := cfg.DefaultShard
	store, err := sn.connectToShard(defaultShardName)
	if err != nil {
		return nil, err
	}
	sn.defaultShard = *store
	sn.connectedShards = map[string]nosqlStore{
		defaultShardName: sn.defaultShard,
	}

	// Parse & validate the sharding policy
	sn.shardingPolicy, err = newShardingPolicy(logger, cfg)
	if err != nil {
		return nil, err
	}

	return &sn, nil
}

func (sn *shardedNosqlStoreImpl) GetStoreShardByHistoryShard(shardID int) (*nosqlStore, error) {
	shardName, err := sn.shardingPolicy.getHistoryShardName(shardID)
	if err != nil {
		return nil, err
	}
	return sn.getShard(shardName)
}

func (sn *shardedNosqlStoreImpl) GetStoreShardByTaskList(domainID string, taskListName string, taskType int) (*nosqlStore, error) {
	shardName := sn.shardingPolicy.getTaskListShardName(domainID, taskListName, taskType)
	return sn.getShard(shardName)
}

func (sn *shardedNosqlStoreImpl) GetDefaultShard() nosqlStore {
	return sn.defaultShard
}

func (sn *shardedNosqlStoreImpl) Close() {
	sn.RLock()
	defer sn.RUnlock()
	for name, shard := range sn.connectedShards {
		sn.logger.Warn("Closing store shard", tag.StoreShard(name))
		shard.Close()
	}
}

func (sn *shardedNosqlStoreImpl) GetName() string {
	return "shardedNosql"
}

func (sn *shardedNosqlStoreImpl) GetShardingPolicy() shardingPolicy {
	return sn.shardingPolicy
}

func (sn *shardedNosqlStoreImpl) GetLogger() log.Logger {
	return sn.logger
}

func (sn *shardedNosqlStoreImpl) getShard(shardName string) (*nosqlStore, error) {
	sn.RLock()
	shard, found := sn.connectedShards[shardName]
	sn.RUnlock()

	if found {
		return &shard, nil
	}

	_, ok := sn.config.Connections[shardName]
	if !ok {
		return nil, &ShardingError{
			Message: fmt.Sprintf("Unknown db shard name: %v", shardName),
		}
	}

	sn.Lock()
	defer sn.Unlock()
	if shard, ok := sn.connectedShards[shardName]; ok { // read again to double-check
		return &shard, nil
	}

	s, err := sn.connectToShard(shardName)
	if err != nil {
		return nil, err
	}
	sn.connectedShards[shardName] = *s
	sn.logger.Info("Connected to store shard", tag.StoreShard(shardName))
	return s, nil
}

func (sn *shardedNosqlStoreImpl) connectToShard(shardName string) (*nosqlStore, error) {
	cfg, ok := sn.config.Connections[shardName]
	if !ok {
		return nil, &ShardingError{
			Message: fmt.Sprintf("Unknown db shard name: %v", shardName),
		}
	}

	sn.logger.Info("Connecting to store shard", tag.StoreShard(shardName))
	db, err := NewNoSQLDB(cfg.NoSQLPlugin, sn.logger, sn.dc)
	if err != nil {
		sn.logger.Error("Failed to connect to store shard", tag.StoreShard(shardName), tag.Error(err))
		return nil, err
	}
	shard := nosqlStore{
		db:     db,
		logger: sn.logger,
	}
	return &shard, nil
}
