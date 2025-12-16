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
	assignedStates map[string]etcdtypes.AssignedState
	metadata       map[string]map[string]string // executorID -> metadata key -> metadata value
	statistics     map[string]map[string]etcdtypes.ShardStatistics
	revisions      map[string]int64
}

func parseExecutorData(resp *clientv3.GetResponse, etcdPrefix, namespace string) (*executorData, error) {
	data := &executorData{
		assignedStates: make(map[string]etcdtypes.AssignedState),
		metadata:       make(map[string]map[string]string),
		statistics:     make(map[string]map[string]etcdtypes.ShardStatistics),
		revisions:      make(map[string]int64),
	}

	for _, kv := range resp.Kvs {
		executorID, keyType, err := etcdkeys.ParseExecutorKey(etcdPrefix, namespace, string(kv.Key))
		if err != nil {
			continue
		}

		switch keyType {
		case etcdkeys.ExecutorAssignedStateKey:
			var assignedState etcdtypes.AssignedState
			if err := common.DecompressAndUnmarshal(kv.Value, &assignedState); err != nil {
				return nil, fmt.Errorf("parse assigned state for %s: %w", executorID, err)
			}
			data.assignedStates[executorID] = assignedState
			data.revisions[executorID] = kv.ModRevision
		case etcdkeys.ExecutorMetadataKey:
			if _, ok := data.metadata[executorID]; !ok {
				data.metadata[executorID] = make(map[string]string)
			}
			metadataKey := strings.TrimPrefix(string(kv.Key), etcdkeys.BuildMetadataKey(etcdPrefix, namespace, executorID, ""))
			data.metadata[executorID][metadataKey] = string(kv.Value)
		case etcdkeys.ExecutorShardStatisticsKey:
			var stats map[string]etcdtypes.ShardStatistics
			if err := common.DecompressAndUnmarshal(kv.Value, &stats); err != nil {
				return nil, fmt.Errorf("parse shard statistics for %s: %w", executorID, err)
			}
			data.statistics[executorID] = stats
		}
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
	n.executorStatistics.lock.RLock()
	stats, ok := n.executorStatistics.stats[executorID]
	if ok {
		clonedStatistics := cloneStatisticsMap(stats)
		n.executorStatistics.lock.RUnlock()
		return clonedStatistics, nil
	}
	n.executorStatistics.lock.RUnlock()

	err := n.refreshExecutorStatisticsCache(ctx, executorID)
	if err != nil {
		return nil, fmt.Errorf("error from refresh: %w", err)
	}

	// After refresh, read from cache again.
	n.executorStatistics.lock.RLock()
	defer n.executorStatistics.lock.RUnlock()
	stats, ok = n.executorStatistics.stats[executorID]
	if !ok {
		return nil, fmt.Errorf("could not get executor statistics, even after refresh")
	}

	return cloneStatisticsMap(stats), nil
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
			return n.handlePotentialRefresh(watchResp)
		}
	}
}

func (n *namespaceShardToExecutor) handlePotentialRefresh(watchResp clientv3.WatchResponse) error {
	if err := watchResp.Err(); err != nil {
		return fmt.Errorf("watch response: %w", err)
	}

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
			return err
		}
	}
	return nil
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

	parsedData, err := parseExecutorData(resp, n.etcdPrefix, n.namespace)
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
func (n *namespaceShardToExecutor) applyParsedData(data *executorData) {
	// Clear the cache
	n.shardToExecutor = make(map[string]*store.ShardOwner)
	n.executorState = make(map[*store.ShardOwner][]string)
	n.executorRevision = make(map[string]int64)
	n.shardOwners = make(map[string]*store.ShardOwner)
	n.executorStatistics.stats = make(map[string]map[string]etcdtypes.ShardStatistics)

	for executorID, assignedState := range data.assignedStates {
		shardOwner := getOrCreateShardOwner(n.shardOwners, executorID)
		shardIDs := make([]string, 0, len(assignedState.AssignedShards))
		for shardID := range assignedState.AssignedShards {
			n.shardToExecutor[shardID] = shardOwner
			shardIDs = append(shardIDs, shardID)
		}
		n.executorState[shardOwner] = shardIDs
		n.executorRevision[executorID] = data.revisions[executorID]
	}

	for executorID, metadata := range data.metadata {
		shardOwner := getOrCreateShardOwner(n.shardOwners, executorID)
		maps.Copy(shardOwner.Metadata, metadata)
	}

	maps.Copy(n.executorStatistics.stats, data.statistics)
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
	n.executorStatistics.lock.Lock()
	defer n.executorStatistics.lock.Unlock()
	if len(stats) == 0 {
		delete(n.executorStatistics.stats, executorID)
		return
	}
	n.executorStatistics.stats[executorID] = cloneStatisticsMap(stats)
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
