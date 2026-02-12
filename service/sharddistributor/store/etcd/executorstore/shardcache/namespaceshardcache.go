package shardcache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdclient"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore/common"
)

const (
	// RetryInterval for watch failures is between 50ms to 150ms
	namespaceRefreshLoopWatchJitterCoeff   = 0.5
	namespaceRefreshLoopWatchRetryInterval = 100 * time.Millisecond
)

type namespaceShardToExecutor struct {
	sync.RWMutex

	shardToExecutor  map[string]*store.ShardOwner   // shardID -> shardOwner
	shardOwners      map[string]*store.ShardOwner   // executorID -> shardOwner
	executorState    map[*store.ShardOwner][]string // executor -> shardIDs
	executorRevision map[string]int64
	namespace        string
	etcdPrefix       string
	stopCh           chan struct{}
	logger           log.Logger
	client           etcdclient.Client
	timeSource       clock.TimeSource
	pubSub           *executorStatePubSub
}

func newNamespaceShardToExecutor(etcdPrefix, namespace string, client etcdclient.Client, stopCh chan struct{}, logger log.Logger, timeSource clock.TimeSource) (*namespaceShardToExecutor, error) {
	return &namespaceShardToExecutor{
		shardToExecutor:  make(map[string]*store.ShardOwner),
		executorState:    make(map[*store.ShardOwner][]string),
		executorRevision: make(map[string]int64),
		shardOwners:      make(map[string]*store.ShardOwner),
		namespace:        namespace,
		etcdPrefix:       etcdPrefix,
		stopCh:           stopCh,
		logger:           logger.WithTags(tag.ShardNamespace(namespace)),
		client:           client,
		timeSource:       timeSource,
		pubSub:           newExecutorStatePubSub(logger, namespace),
	}, nil
}

func (n *namespaceShardToExecutor) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.namespaceRefreshLoop()
	}()
}

func (n *namespaceShardToExecutor) GetShardOwner(ctx context.Context, shardID string) (*store.ShardOwner, error) {
	shardOwner, err := n.getShardOwnerInMap(ctx, &n.shardToExecutor, shardID)
	if err != nil {
		return nil, fmt.Errorf("get shard owner in map: %w", err)
	}
	if shardOwner != nil {
		return shardOwner, nil
	}

	return nil, store.ErrShardNotFound
}

func (n *namespaceShardToExecutor) GetExecutor(ctx context.Context, executorID string) (*store.ShardOwner, error) {
	shardOwner, err := n.getShardOwnerInMap(ctx, &n.shardOwners, executorID)
	if err != nil {
		return nil, fmt.Errorf("get shard owner in map: %w", err)
	}
	if shardOwner != nil {
		return shardOwner, nil
	}

	return nil, store.ErrExecutorNotFound
}

func (n *namespaceShardToExecutor) GetExecutorModRevisionCmp() ([]clientv3.Cmp, error) {
	n.RLock()
	defer n.RUnlock()
	comparisons := []clientv3.Cmp{}
	for executor, revision := range n.executorRevision {
		executorAssignedStateKey := etcdkeys.BuildExecutorKey(n.etcdPrefix, n.namespace, executor, etcdkeys.ExecutorAssignedStateKey)
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(executorAssignedStateKey), "=", revision))
	}

	return comparisons, nil
}

func (n *namespaceShardToExecutor) Subscribe(ctx context.Context) (<-chan map[*store.ShardOwner][]string, func()) {
	subCh, unSub := n.pubSub.subscribe(ctx)

	// The go routine sends the initial state to the subscriber.
	go func() {
		initialState := n.getExecutorState()

		select {
		case <-ctx.Done():
			n.logger.Warn("context finished before initial state was sent")
		case subCh <- initialState:
			n.logger.Info("initial state sent to subscriber", tag.Value(initialState))
		}

	}()

	return subCh, unSub
}

func (n *namespaceShardToExecutor) namespaceRefreshLoop() {
	triggerCh := n.runWatchLoop()

	for {
		select {
		case <-n.stopCh:
			n.logger.Info("stop channel closed, exiting namespaceRefreshLoop")
			return

		case _, ok := <-triggerCh:
			if !ok {
				n.logger.Info("trigger channel closed, exiting namespaceRefreshLoop")
				return
			}

			if err := n.refresh(context.Background()); err != nil {
				n.logger.Error("failed to refresh namespace shard to executor", tag.Error(err))
			}
		}
	}
}

func (n *namespaceShardToExecutor) runWatchLoop() <-chan struct{} {
	triggerCh := make(chan struct{}, 1)

	go func() {
		defer close(triggerCh)

		for {
			if err := n.watch(triggerCh); err != nil {
				n.logger.Error("error watching in namespaceRefreshLoop, retrying...", tag.Error(err))
				n.timeSource.Sleep(backoff.JitDuration(
					namespaceRefreshLoopWatchRetryInterval,
					namespaceRefreshLoopWatchJitterCoeff,
				))
				continue
			}

			n.logger.Info("namespaceRefreshLoop is exiting")
			return
		}
	}()

	return triggerCh
}

func (n *namespaceShardToExecutor) watch(triggerCh chan<- struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchChan := n.client.Watch(
		// WithRequireLeader ensures that the etcd cluster has a leader
		clientv3.WithRequireLeader(ctx),
		etcdkeys.BuildExecutorsPrefix(n.etcdPrefix, n.namespace),
		clientv3.WithPrefix(),
		clientv3.WithPrevKV(),
	)

	for {
		select {
		case <-n.stopCh:
			n.logger.Info("stop channel closed, exiting watch loop")
			return nil

		case watchResp, ok := <-watchChan:
			if err := watchResp.Err(); err != nil {
				return fmt.Errorf("watch response: %w", err)
			}
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			// Only trigger refresh if the change is related to executor assigned state or metadata
			if !n.hasExecutorStateChanged(watchResp) {
				continue
			}

			select {
			case triggerCh <- struct{}{}:
			default:
				n.logger.Info("Cache is being refreshed, skipping trigger")
			}
		}
	}
}

// hasExecutorStateChanged checks if any of the events in the watch response indicate a change to executor assigned state or metadata,
// and if the value actually changed (not just same value written again)
func (n *namespaceShardToExecutor) hasExecutorStateChanged(watchResp clientv3.WatchResponse) bool {
	for _, event := range watchResp.Events {
		_, keyType, keyErr := etcdkeys.ParseExecutorKey(n.etcdPrefix, n.namespace, string(event.Kv.Key))
		if keyErr != nil {
			n.logger.Warn("Received watch event with unrecognized key format", tag.Value(keyErr))
			continue
		}

		// Only refresh on changes to assigned state or metadata, ignore other changes under the executor prefix such as executor heartbeat keys
		if keyType != etcdkeys.ExecutorAssignedStateKey && keyType != etcdkeys.ExecutorMetadataKey {
			continue
		}

		// Check if value actually changed (skip if same value written again)
		if event.PrevKv != nil && string(event.Kv.Value) == string(event.PrevKv.Value) {
			continue
		}

		return true
	}

	return false
}

func (n *namespaceShardToExecutor) refresh(ctx context.Context) error {
	err := n.refreshExecutorState(ctx)
	if err != nil {
		return fmt.Errorf("refresh executor state: %w", err)
	}

	n.pubSub.publish(n.getExecutorState())
	return nil
}

func (n *namespaceShardToExecutor) getExecutorState() map[*store.ShardOwner][]string {
	n.RLock()
	defer n.RUnlock()
	executorState := make(map[*store.ShardOwner][]string)
	for executor, shardIDs := range n.executorState {
		executorState[executor] = make([]string, len(shardIDs))
		copy(executorState[executor], shardIDs)
	}

	return executorState
}

func (n *namespaceShardToExecutor) refreshExecutorState(ctx context.Context) error {
	executorPrefix := etcdkeys.BuildExecutorsPrefix(n.etcdPrefix, n.namespace)

	resp, err := n.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("get executor prefix for namespace %s: %w", n.namespace, err)
	}

	n.Lock()
	defer n.Unlock()
	// Clear the cache, so we don't have any stale data
	n.shardToExecutor = make(map[string]*store.ShardOwner)
	n.executorState = make(map[*store.ShardOwner][]string)
	n.executorRevision = make(map[string]int64)
	n.shardOwners = make(map[string]*store.ShardOwner)

	for _, kv := range resp.Kvs {
		executorID, keyType, keyErr := etcdkeys.ParseExecutorKey(n.etcdPrefix, n.namespace, string(kv.Key))
		if keyErr != nil {
			continue
		}
		switch keyType {
		case etcdkeys.ExecutorAssignedStateKey:
			shardOwner := getOrCreateShardOwner(n.shardOwners, executorID)

			var assignedState etcdtypes.AssignedState
			err = common.DecompressAndUnmarshal(kv.Value, &assignedState)
			if err != nil {
				return fmt.Errorf("parse assigned state: %w", err)
			}

			// Build both shard->executor and executor->shards mappings
			shardIDs := make([]string, 0, len(assignedState.AssignedShards))
			for shardID := range assignedState.AssignedShards {
				n.shardToExecutor[shardID] = shardOwner
				shardIDs = append(shardIDs, shardID)
				n.executorRevision[executorID] = kv.ModRevision
			}
			n.executorState[shardOwner] = shardIDs

		case etcdkeys.ExecutorMetadataKey:
			shardOwner := getOrCreateShardOwner(n.shardOwners, executorID)
			metadataKey := strings.TrimPrefix(string(kv.Key), etcdkeys.BuildMetadataKey(n.etcdPrefix, n.namespace, executorID, ""))
			shardOwner.Metadata[metadataKey] = string(kv.Value)

		default:
			continue
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

// getShardOwnerInMap retrieves a shard owner from the map if it exists, otherwise it refreshes the cache and tries again
// it takes a pointer to the map. When the cache is refreshed, the map is updated, so we need to pass a pointer to the map
func (n *namespaceShardToExecutor) getShardOwnerInMap(ctx context.Context, m *map[string]*store.ShardOwner, key string) (*store.ShardOwner, error) {
	n.RLock()
	shardOwner, ok := (*m)[key]
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
	shardOwner, ok = (*m)[key]
	n.RUnlock()
	if ok {
		return shardOwner, nil
	}
	return nil, nil
}
