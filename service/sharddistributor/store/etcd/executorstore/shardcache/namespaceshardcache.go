package shardcache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
)

type namespaceShardToExecutor struct {
	sync.RWMutex

	shardToExecutor     map[string]*store.ShardOwner
	executorRevision    map[string]int64
	namespace           string
	etcdPrefix          string
	changeUpdateChannel clientv3.WatchChan
	stopCh              chan struct{}
	logger              log.Logger
	client              *clientv3.Client
}

func newNamespaceShardToExecutor(etcdPrefix, namespace string, client *clientv3.Client, stopCh chan struct{}, logger log.Logger) (*namespaceShardToExecutor, error) {
	// Start listening
	watchPrefix := etcdkeys.BuildExecutorPrefix(etcdPrefix, namespace)
	watchChan := client.Watch(context.Background(), watchPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

	return &namespaceShardToExecutor{
		shardToExecutor:     make(map[string]*store.ShardOwner),
		executorRevision:    make(map[string]int64),
		namespace:           namespace,
		etcdPrefix:          etcdPrefix,
		changeUpdateChannel: watchChan,
		stopCh:              stopCh,
		logger:              logger,
		client:              client,
	}, nil
}

func (n *namespaceShardToExecutor) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.nameSpaceRefreashLoop()
	}()
}

func (n *namespaceShardToExecutor) GetShardOwner(ctx context.Context, shardID string) (*store.ShardOwner, error) {
	n.RLock()
	shardOwner, ok := n.shardToExecutor[shardID]
	n.RUnlock()

	if ok {
		return shardOwner, nil
	}

	// Force refresh the cache
	err := n.refresh(ctx)
	if err != nil {
		return nil, fmt.Errorf("refresh for namespace %s: %w", n.namespace, err)
	}

	// Check the cache again after refresh
	n.RLock()
	shardOwner, ok = n.shardToExecutor[shardID]
	n.RUnlock()
	if ok {
		return shardOwner, nil
	}

	return nil, store.ErrShardNotFound
}

func (n *namespaceShardToExecutor) GetExecutorModRevisionCmp() ([]clientv3.Cmp, error) {
	n.RLock()
	defer n.RUnlock()
	comparisons := []clientv3.Cmp{}
	for executor, revision := range n.executorRevision {
		executorAssignedStateKey, err := etcdkeys.BuildExecutorKey(n.etcdPrefix, n.namespace, executor, etcdkeys.ExecutorAssignedStateKey)
		if err != nil {
			return nil, fmt.Errorf("build executor assigned state key: %w", err)
		}
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(executorAssignedStateKey), "=", revision))
	}

	return comparisons, nil
}

func (n *namespaceShardToExecutor) nameSpaceRefreashLoop() {
	for {
		select {
		case <-n.stopCh:
			return
		case watchResp := <-n.changeUpdateChannel:
			shouldRefresh := false
			for _, event := range watchResp.Events {
				_, keyType, keyErr := etcdkeys.ParseExecutorKey(n.etcdPrefix, n.namespace, string(event.Kv.Key))
				if keyErr == nil && (keyType == etcdkeys.ExecutorAssignedStateKey || keyType == etcdkeys.ExecutorMetadataKey) {
					// Check if value actually changed (skip if same value written again)
					if event.PrevKv != nil && string(event.Kv.Value) == string(event.PrevKv.Value) {
						continue
					}
					shouldRefresh = true
					break
				}
			}
			if shouldRefresh {
				err := n.refresh(context.Background())
				if err != nil {
					n.logger.Error("failed to refresh namespace shard to executor", tag.ShardNamespace(n.namespace), tag.Error(err))
				}
			}

		}
	}
}

func (n *namespaceShardToExecutor) refresh(ctx context.Context) error {

	executorPrefix := etcdkeys.BuildExecutorPrefix(n.etcdPrefix, n.namespace)

	resp, err := n.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("get executor prefix for namespace %s: %w", n.namespace, err)
	}

	n.Lock()
	defer n.Unlock()
	// Clear the cache, so we don't have any stale data
	n.shardToExecutor = make(map[string]*store.ShardOwner)
	n.executorRevision = make(map[string]int64)

	shardOwners := make(map[string]*store.ShardOwner)

	for _, kv := range resp.Kvs {
		executorID, keyType, keyErr := etcdkeys.ParseExecutorKey(n.etcdPrefix, n.namespace, string(kv.Key))
		if keyErr != nil {
			continue
		}
		if keyType == etcdkeys.ExecutorAssignedStateKey {
			shardOwner := getOrCreateShardOwner(shardOwners, executorID)

			var assignedState store.AssignedState
			err = json.Unmarshal(kv.Value, &assignedState)
			if err != nil {
				return fmt.Errorf("unmarshal assigned state: %w", err)
			}
			for shardID := range assignedState.AssignedShards {
				n.shardToExecutor[shardID] = shardOwner
				n.executorRevision[executorID] = kv.ModRevision
			}
		}
		if keyType == etcdkeys.ExecutorMetadataKey {
			shardOwner := getOrCreateShardOwner(shardOwners, executorID)
			metadataKey := strings.TrimPrefix(string(kv.Key), etcdkeys.BuildMetadataKey(n.etcdPrefix, n.namespace, executorID, ""))
			shardOwner.Metadata[metadataKey] = string(kv.Value)
		}
	}

	return nil
}

// getOrCreateShardOwner retrieves an existing ShardOwner from the map or creates a new one if it doesn't exist
func getOrCreateShardOwner(shardOwners map[string]*store.ShardOwner, executorID string) *store.ShardOwner {
	shardOwner, ok := shardOwners[executorID]
	if !ok {
		shardOwner = &store.ShardOwner{
			ExecutorID: executorID,
			Metadata:   make(map[string]string),
		}
		shardOwners[executorID] = shardOwner
	}
	return shardOwner
}
