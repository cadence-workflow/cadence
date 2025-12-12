package shardcache

import (
	"context"
	"fmt"
	"strings"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdclient"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore/common"
)

type namespaceShardToExecutor struct {
	sync.RWMutex

	shardToExecutor        map[string]*store.ShardOwner   // shardID -> shardOwner
	shardOwners            map[string]*store.ShardOwner   // executorID -> shardOwner
	executorState          map[*store.ShardOwner][]string // executor -> shardIDs
	executorRevision       map[string]int64
	executorStatisticsLock sync.RWMutex
	executorStatistics     map[string]map[string]etcdtypes.ShardStatistics
	namespace              string
	etcdPrefix             string
	changeUpdateChannel    clientv3.WatchChan
	stopCh                 chan struct{}
	logger                 log.Logger
	client                 etcdclient.Client
	pubSub                 *executorStatePubSub
}

func newNamespaceShardToExecutor(etcdPrefix, namespace string, client etcdclient.Client, stopCh chan struct{}, logger log.Logger) (*namespaceShardToExecutor, error) {
	// Start listening
	watchPrefix := etcdkeys.BuildExecutorsPrefix(etcdPrefix, namespace)
	watchChan := client.Watch(context.Background(), watchPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

	return &namespaceShardToExecutor{
		shardToExecutor:     make(map[string]*store.ShardOwner),
		executorState:       make(map[*store.ShardOwner][]string),
		executorRevision:    make(map[string]int64),
		shardOwners:         make(map[string]*store.ShardOwner),
		executorStatistics:  make(map[string]map[string]etcdtypes.ShardStatistics),
		namespace:           namespace,
		etcdPrefix:          etcdPrefix,
		changeUpdateChannel: watchChan,
		stopCh:              stopCh,
		logger:              logger,
		client:              client,
		pubSub:              newExecutorStatePubSub(logger, namespace),
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

func (n *namespaceShardToExecutor) GetExecutorStatistics(ctx context.Context, executorID string) (map[string]etcdtypes.ShardStatistics, error) {
	n.executorStatisticsLock.RLock()
	stats, ok := n.executorStatistics[executorID]
	cloned := cloneStatisticsMap(stats)
	n.executorStatisticsLock.RUnlock()
	if ok {
		return cloned, nil
	}

	return n.getExecutorStatistics(ctx, executorID)
}

// getExecutorStatistics fetches executor statistics from etcd and caches them.
// It is called when there's a cache miss.
func (n *namespaceShardToExecutor) getExecutorStatistics(ctx context.Context, executorID string) (map[string]etcdtypes.ShardStatistics, error) {
	statsKey := etcdkeys.BuildExecutorKey(n.etcdPrefix, n.namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
	resp, err := n.client.Get(ctx, statsKey)
	if err != nil {
		return nil, fmt.Errorf("get executor shard statistics: %w", err)
	}

	stats := make(map[string]etcdtypes.ShardStatistics)
	if len(resp.Kvs) == 0 {
		return stats, nil
	}

	if err := common.DecompressAndUnmarshal(resp.Kvs[0].Value, &stats); err != nil {
		return nil, fmt.Errorf("parse executor shard statistics: %w", err)
	}

	n.executorStatisticsLock.Lock()
	defer n.executorStatisticsLock.Unlock()
	n.executorStatistics[executorID] = stats
	return cloneStatisticsMap(stats), nil
}

func (n *namespaceShardToExecutor) Subscribe(ctx context.Context) (<-chan map[*store.ShardOwner][]string, func()) {
	subCh, unSub := n.pubSub.subscribe(ctx)

	// The go routine sends the initial state to the subscriber.
	go func() {
		initialState := n.getExecutorState()

		select {
		case <-ctx.Done():
			n.logger.Warn("context finnished before initial state was sent", tag.ShardNamespace(n.namespace))
		case subCh <- initialState:
			n.logger.Info("initial state sent to subscriber", tag.ShardNamespace(n.namespace), tag.Value(initialState))
		}

	}()

	return subCh, unSub
}

func (n *namespaceShardToExecutor) nameSpaceRefreashLoop() {
	for {
		select {
		case <-n.stopCh:
			return
		case watchResp := <-n.changeUpdateChannel:
			n.handlePotentialRefresh(watchResp)
		}
	}
}

func (n *namespaceShardToExecutor) handlePotentialRefresh(watchResp clientv3.WatchResponse) {
	shouldRefresh := false
	for _, event := range watchResp.Events {
		executorID, keyType, keyErr := etcdkeys.ParseExecutorKey(n.etcdPrefix, n.namespace, string(event.Kv.Key))
		if keyErr != nil {
			continue
		}

		// Check if value actually changed (skip if same value written again)
		if event.PrevKv != nil && string(event.Kv.Value) == string(event.PrevKv.Value) {
			continue
		}

		switch keyType {
		case etcdkeys.ExecutorShardStatisticsKey:
			n.handleExecutorStatisticsEvent(executorID, event)
		case etcdkeys.ExecutorAssignedStateKey, etcdkeys.ExecutorMetadataKey:
			shouldRefresh = true
		}
	}
	if shouldRefresh {
		err := n.refresh(context.Background())
		if err != nil {
			n.logger.Error("failed to refresh namespace shard to executor", tag.ShardNamespace(n.namespace), tag.Error(err))
		}
	}
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
	n.executorStatisticsLock.Lock()
	defer n.executorStatisticsLock.Unlock()
	// Clear the cache, so we don't have any stale data
	n.shardToExecutor = make(map[string]*store.ShardOwner)
	n.executorState = make(map[*store.ShardOwner][]string)
	n.executorRevision = make(map[string]int64)
	n.shardOwners = make(map[string]*store.ShardOwner)
	n.executorStatistics = make(map[string]map[string]etcdtypes.ShardStatistics)

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

		case etcdkeys.ExecutorShardStatisticsKey:
			var stats map[string]etcdtypes.ShardStatistics
			err = common.DecompressAndUnmarshal(kv.Value, &stats)
			if err != nil {
				return fmt.Errorf("parse shard statistics: %w", err)
			}
			n.executorStatistics[executorID] = stats
		default:
			continue
		}
	}

	return nil
}

// handleExecutorStatisticsEvent processes incoming watch events for executor shard statistics.
// It updates the in-memory statistics map directly from the event without triggering a full refresh.
func (n *namespaceShardToExecutor) handleExecutorStatisticsEvent(executorID string, event *clientv3.Event) {
	if event == nil || event.Type == clientv3.EventTypeDelete || event.Kv == nil || len(event.Kv.Value) == 0 {
		n.setExecutorStatistics(executorID, nil)
		return
	}

	stats := make(map[string]etcdtypes.ShardStatistics)
	if err := common.DecompressAndUnmarshal(event.Kv.Value, &stats); err != nil {
		n.logger.Error(
			"failed to parse executor statistics from watch event",
			tag.ShardNamespace(n.namespace),
			tag.ShardExecutor(executorID),
			tag.Error(err),
		)
		return
	}

	n.setExecutorStatistics(executorID, stats)
}

func (n *namespaceShardToExecutor) setExecutorStatistics(executorID string, stats map[string]etcdtypes.ShardStatistics) {
	n.executorStatisticsLock.Lock()
	defer n.executorStatisticsLock.Unlock()
	if len(stats) == 0 {
		delete(n.executorStatistics, executorID)
		return
	}
	n.executorStatistics[executorID] = cloneStatisticsMap(stats)
}

// Clone to prevent concurrent map access
func cloneStatisticsMap(src map[string]etcdtypes.ShardStatistics) map[string]etcdtypes.ShardStatistics {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]etcdtypes.ShardStatistics, len(src))
	for shardID, stat := range src {
		dst[shardID] = stat
	}
	return dst
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
