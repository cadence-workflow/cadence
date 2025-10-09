package executorstore

import (
	"context"
	"fmt"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/log"
)

type NamespaceToShards map[string]*namespaceShardToExecutor
type ShardToExecutorCache struct {
	sync.RWMutex
	namespaceToShards NamespaceToShards
	client            *clientv3.Client
	stopC             chan struct{}
	logger            log.Logger
	prefix            string
	wg                sync.WaitGroup
}

type ShardToExecutorCacheParams struct {
	fx.In

	Logger log.Logger
}

func NewShardToExecutorCache(p ShardToExecutorCacheParams) *ShardToExecutorCache {
	shardCache := &ShardToExecutorCache{
		namespaceToShards: make(NamespaceToShards),
		stopC:             make(chan struct{}),
		logger:            p.Logger,
		wg:                sync.WaitGroup{},
	}

	return shardCache
}

func (s *ShardToExecutorCache) Start() {}

func (s *ShardToExecutorCache) Stop() {
	close(s.stopC)
	s.wg.Wait()
}

func (s *ShardToExecutorCache) GetShardOwner(ctx context.Context, namespace, shardID string) (string, error) {
	namespaceShardToExecutor, err := s.getNamespaceShardToExecutor(ctx, namespace)
	if err != nil {
		return "", fmt.Errorf("get namespace shard to executor: %w", err)
	}
	return namespaceShardToExecutor.GetShardOwner(ctx, shardID)
}

func (s *ShardToExecutorCache) getNamespaceShardToExecutor(ctx context.Context, namespace string) (*namespaceShardToExecutor, error) {
	s.RLock()
	namespaceShardToExecutor, ok := s.namespaceToShards[namespace]
	s.RUnlock()

	if ok {
		return namespaceShardToExecutor, nil
	}

	s.Lock()
	defer s.Unlock()

	namespaceShardToExecutor, err := newNamespaceShardToExecutor(s.prefix, namespace, s.client, s.stopC, s.logger)
	if err != nil {
		return nil, fmt.Errorf("new namespace shard to executor: %w", err)
	}
	namespaceShardToExecutor.Start(&s.wg)

	s.namespaceToShards[namespace] = namespaceShardToExecutor
	return namespaceShardToExecutor, nil
}
