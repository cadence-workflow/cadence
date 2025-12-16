package shardcache

import (
	"context"
	"fmt"
	"maps"
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
	pubSub           *executorStatePubSub
	timeSource       clock.TimeSource

	executorStatistics namespaceExecutorStatistics
}

type namespaceExecutorStatistics struct {
	lock  sync.RWMutex
	stats map[string]map[string]etcdtypes.ShardStatistics
}

type executorData struct {
	assignedStates etcdtypes.AssignedState
	metadata       map[string]string
	statistics     map[string]etcdtypes.ShardStatistics
	revisions      int64
}

func (n *namespaceShardToExecutor) parseExecutorData(resp *clientv3.GetResponse, etcdPrefix, namespace string) (map[string]executorData, error) {
	data := make(map[string]executorData)

	for _, kv := range resp.Kvs {
		executorID, keyType, err := etcdkeys.ParseExecutorKey(etcdPrefix, namespace, string(kv.Key))
		execData := executorData{
			assignedStates: etcdtypes.AssignedState{},
			metadata:       make(map[string]string),
			statistics:     make(map[string]etcdtypes.ShardStatistics),
			revisions:      0,
		}
		if err != nil {
			n.logger.Warn("error parsing executor key:", tag.Error(err))
			continue
		}

		switch keyType {
		case etcdkeys.ExecutorAssignedStateKey:
			var assignedState etcdtypes.AssignedState
			if err := common.DecompressAndUnmarshal(kv.Value, &assignedState); err != nil {
				return nil, fmt.Errorf("parse assigned state for %s: %w", executorID, err)
			}
			execData.assignedStates = assignedState
			execData.revisions = kv.ModRevision
		case etcdkeys.ExecutorMetadataKey:
			metadataKey := strings.TrimPrefix(string(kv.Key), etcdkeys.BuildMetadataKey(etcdPrefix, namespace, executorID, ""))
			execData.metadata[metadataKey] = string(kv.Value)
		case etcdkeys.ExecutorShardStatisticsKey:
			var stats map[string]etcdtypes.ShardStatistics
			if err := common.DecompressAndUnmarshal(kv.Value, &stats); err != nil {
				return nil, fmt.Errorf("parse shard statistics for %s: %w", executorID, err)
			}
			execData.statistics = stats
		}
		data[executorID] = execData
	}
	return data, nil
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
		executorStatistics: namespaceExecutorStatistics{
			stats: make(map[string]map[string]etcdtypes.ShardStatistics),
		},
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

func (n *namespaceShardToExecutor) GetExecutorStatistics(ctx context.Context, executorID string) (map[string]etcdtypes.ShardStatistics, error) {
	if stats, found := n.readStats(executorID); found {
		return stats, nil
	}

	if err := n.refreshExecutorStatisticsCache(ctx, executorID); err != nil {
		return nil, fmt.Errorf("error from refresh: %w", err)
	}

	// Refreshing cache after cache miss should allow the statistics to be found
	if stats, found := n.readStats(executorID); found {
		return stats, nil
	}

	return nil, fmt.Errorf("could not get executor statistics, even after refresh")
}

func (n *namespaceShardToExecutor) readStats(executorID string) (map[string]etcdtypes.ShardStatistics, bool) {
	n.executorStatistics.lock.RLock()
	defer n.executorStatistics.lock.RUnlock()

	stats, ok := n.executorStatistics.stats[executorID]
	if ok {
		return maps.Clone(stats), true
	}

	return nil, false
}

// refreshExecutorStatisticsCache fetches executor statistics from etcd and caches them.
// It is called when there's a cache miss.
func (n *namespaceShardToExecutor) refreshExecutorStatisticsCache(ctx context.Context, executorID string) error {
	n.executorStatistics.lock.Lock()
	defer n.executorStatistics.lock.Unlock()

	if _, ok := n.executorStatistics.stats[executorID]; ok {
		return nil // Value is already cached. Nothing to do.
	}

	statsKey := etcdkeys.BuildExecutorKey(n.etcdPrefix, n.namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
	resp, err := n.client.Get(ctx, statsKey)
	if err != nil {
		return fmt.Errorf("get executor shard statistics: %w", err)
	}

	stats := make(map[string]etcdtypes.ShardStatistics)
	if len(resp.Kvs) > 0 {
		if err := common.DecompressAndUnmarshal(resp.Kvs[0].Value, &stats); err != nil {
			return fmt.Errorf("parse executor shard statistics: %w", err)
		}
	}

	n.executorStatistics.stats[executorID] = stats
	return nil
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
	for {
		if err := n.watch(); err != nil {
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
}

func (n *namespaceShardToExecutor) watch() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchChan := n.client.Watch(
		// WithRequireLeader ensures that the etcd cluster has a leader
		clientv3.WithRequireLeader(ctx),
		etcdkeys.BuildExecutorsPrefix(n.etcdPrefix, n.namespace),
		clientv3.WithPrefix(), clientv3.WithPrevKV(),
	)

	for {
		select {
		case <-n.stopCh:
			return nil
		case watchResp, ok := <-watchChan:
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			if err := watchResp.Err(); err != nil {
				return fmt.Errorf("watch response: %w", err)
			}

			if n.executorStateChanges(watchResp.Events) {
				if err := n.refresh(context.Background()); err != nil {
					n.logger.Error("failed to refresh namespace shard to executor", tag.ShardNamespace(n.namespace), tag.Error(err))
					return err
				}
			}
		}
	}
}

func (n *namespaceShardToExecutor) executorStateChanges(events []*clientv3.Event) bool {
	for _, event := range events {
		executorID, keyType, keyErr := etcdkeys.ParseExecutorKey(n.etcdPrefix, n.namespace, string(event.Kv.Key))
		if keyErr != nil {
			n.logger.Error("failed to parse executor key", tag.ShardNamespace(n.namespace), tag.Error(keyErr))
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
			return true
		}
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

	parsedData, err := n.parseExecutorData(resp, n.etcdPrefix, n.namespace)
	if err != nil {
		return fmt.Errorf("failed to parse executor data: %w", err)
	}

	n.Lock()
	defer n.Unlock()
	n.executorStatistics.lock.Lock()
	defer n.executorStatistics.lock.Unlock()

	n.applyParsedData(parsedData)
	return nil
}

// This function must be called with write locks held for both shard owners and statistics.
func (n *namespaceShardToExecutor) applyParsedData(data map[string]executorData) {
	// Clear the cache
	n.shardToExecutor = make(map[string]*store.ShardOwner)
	n.executorState = make(map[*store.ShardOwner][]string)
	n.executorRevision = make(map[string]int64)
	n.shardOwners = make(map[string]*store.ShardOwner)
	n.executorStatistics.stats = make(map[string]map[string]etcdtypes.ShardStatistics)

	for executorID, executordata := range data {
		shardOwner := getOrCreateShardOwner(n.shardOwners, executorID)
		shardIDs := make([]string, 0, len(executordata.assignedStates.AssignedShards))
		for shardID := range executordata.assignedStates.AssignedShards {
			n.shardToExecutor[shardID] = shardOwner
			shardIDs = append(shardIDs, shardID)
		}
		n.executorState[shardOwner] = shardIDs
		n.executorRevision[executorID] = executordata.revisions

		maps.Copy(shardOwner.Metadata, executordata.metadata)

		n.executorStatistics.stats[executorID] = executordata.statistics
	}
}

// handleExecutorStatisticsEvent processes incoming watch events for executor shard statistics.
// It updates the in-memory statistics map directly from the event without triggering a full refresh.
func (n *namespaceShardToExecutor) handleExecutorStatisticsEvent(executorID string, event *clientv3.Event) {
	if event == nil || event.Type == clientv3.EventTypeDelete || event.Kv == nil || len(event.Kv.Value) == 0 {
		n.executorStatistics.deleteStatistics(executorID)
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

	n.executorStatistics.assignStatistics(executorID, stats)
}

func (n *namespaceExecutorStatistics) deleteStatistics(executorID string) {
	n.lock.Lock()
	defer n.lock.Unlock()
	delete(n.stats, executorID)
}

func (n *namespaceExecutorStatistics) assignStatistics(executorID string, stats map[string]etcdtypes.ShardStatistics) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.stats[executorID] = maps.Clone(stats)
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
