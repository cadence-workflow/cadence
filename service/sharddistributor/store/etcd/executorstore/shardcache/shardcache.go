package shardcache

import (
	"context"
	"fmt"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdclient"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
)

type NamespaceToShards map[string]*namespaceShardToExecutor
type ShardToExecutorCache struct {
	sync.RWMutex
	namespaceToShards NamespaceToShards
	timeSource        clock.TimeSource
	client            etcdclient.Client
	stopC             chan struct{}
	logger            log.Logger
	prefix            string
	wg                sync.WaitGroup
}

func NewShardToExecutorCache(
	prefix string,
	client etcdclient.Client,
	logger log.Logger,
	timeSource clock.TimeSource,
) *ShardToExecutorCache {
	shardCache := &ShardToExecutorCache{
		namespaceToShards: make(NamespaceToShards),
		timeSource:        timeSource,
		stopC:             make(chan struct{}),
		logger:            logger,
		prefix:            prefix,
		client:            client,
		wg:                sync.WaitGroup{},
	}

	return shardCache
}

func (s *ShardToExecutorCache) Start() {}

func (s *ShardToExecutorCache) Stop() {
	close(s.stopC)
	s.wg.Wait()
}

func (s *ShardToExecutorCache) GetShardOwner(ctx context.Context, namespace, shardID string) (*store.ShardOwner, error) {
	namespaceShardToExecutor, err := s.getNamespaceShardToExecutor(namespace)
	if err != nil {
		return nil, fmt.Errorf("get namespace shard to executor: %w", err)
	}
	return namespaceShardToExecutor.GetShardOwner(ctx, shardID)
}

func (s *ShardToExecutorCache) GetExecutorStatistics(ctx context.Context, namespace, executorID string) (map[string]etcdtypes.ShardStatistics, error) {
	namespaceShardToExecutor, err := s.getNamespaceShardToExecutor(namespace)
	if err != nil {
		return nil, fmt.Errorf("get namespace shard to executor: %w", err)
	}
	return namespaceShardToExecutor.GetExecutorStatistics(ctx, executorID)
}

func (s *ShardToExecutorCache) GetExecutor(ctx context.Context, namespace, executorID string) (*store.ShardOwner, error) {
	namespaceShardToExecutor, err := s.getNamespaceShardToExecutor(namespace)
	if err != nil {
		return nil, fmt.Errorf("get namespace shard to executor: %w", err)
	}
	return namespaceShardToExecutor.GetExecutor(ctx, executorID)
}

func (s *ShardToExecutorCache) GetExecutorModRevisionCmp(namespace string) ([]clientv3.Cmp, error) {
	namespaceShardToExecutor, err := s.getNamespaceShardToExecutor(namespace)
	if err != nil {
		return nil, fmt.Errorf("get namespace shard to executor: %w", err)
	}
	return namespaceShardToExecutor.GetExecutorModRevisionCmp()
}

func (s *ShardToExecutorCache) Subscribe(ctx context.Context, namespace string) (<-chan map[*store.ShardOwner][]string, func(), error) {
	namespaceShardToExecutor, err := s.getNamespaceShardToExecutor(namespace)
	if err != nil {
		return nil, nil, fmt.Errorf("get namespace shard to executor: %w", err)
	}

	ch, unSub := namespaceShardToExecutor.Subscribe(ctx)
	return ch, unSub, nil
}

func (s *ShardToExecutorCache) getNamespaceShardToExecutor(namespace string) (*namespaceShardToExecutor, error) {
	s.RLock()
	namespaceShardToExecutor, ok := s.namespaceToShards[namespace]
	s.RUnlock()

	if ok {
		return namespaceShardToExecutor, nil
	}

	s.Lock()
	defer s.Unlock()

	namespaceShardToExecutor, err := newNamespaceShardToExecutor(s.prefix, namespace, s.client, s.stopC, s.logger, s.timeSource)
	if err != nil {
		return nil, fmt.Errorf("new namespace shard to executor: %w", err)
	}
	namespaceShardToExecutor.Start(&s.wg)

	s.namespaceToShards[namespace] = namespaceShardToExecutor
	return namespaceShardToExecutor, nil
}
