package executorstore

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=executorstore_mock.go ExecutorStore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore/shardcache"
)

var (
	_executorStatusRunningJSON = fmt.Sprintf(`"%s"`, types.ExecutorStatusACTIVE)
)

type executorStoreImpl struct {
	client     *clientv3.Client
	prefix     string
	logger     log.Logger
	shardCache *shardcache.ShardToExecutorCache
}

// shardMetricsUpdate tracks the etcd key, revision, and metrics used to update a shard
// after the main transaction in AssignShards for exec state.
// Retains metrics to safely merge concurrent updates before retrying.
type shardMetricsUpdate struct {
	key               string
	shardID           string
	metrics           store.ShardMetrics
	modRevision       int64
	desiredLastMove   int64 // intended LastMoveTime for this update
	defaultLastUpdate int64
}

// ExecutorStoreParams defines the dependencies for the etcd store, for use with fx.
type ExecutorStoreParams struct {
	fx.In

	Client    *clientv3.Client `optional:"true"`
	Cfg       config.ShardDistribution
	Lifecycle fx.Lifecycle
	Logger    log.Logger
}

// NewStore creates a new etcd-backed store and provides it to the fx application.
func NewStore(p ExecutorStoreParams) (store.Store, error) {
	if !p.Cfg.Enabled {
		return nil, nil
	}

	var err error
	var etcdCfg struct {
		Endpoints   []string      `yaml:"endpoints"`
		DialTimeout time.Duration `yaml:"dialTimeout"`
		Prefix      string        `yaml:"prefix"`
	}

	if err := p.Cfg.Store.StorageParams.Decode(&etcdCfg); err != nil {
		return nil, fmt.Errorf("bad config for etcd store: %w", err)
	}

	etcdClient := p.Client
	if etcdClient == nil {
		etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   etcdCfg.Endpoints,
			DialTimeout: etcdCfg.DialTimeout,
		})
		if err != nil {
			return nil, err
		}
	}

	shardCache := shardcache.NewShardToExecutorCache(etcdCfg.Prefix, etcdClient, p.Logger)

	store := &executorStoreImpl{
		client:     etcdClient,
		prefix:     etcdCfg.Prefix,
		logger:     p.Logger,
		shardCache: shardCache,
	}

	p.Lifecycle.Append(fx.StartStopHook(store.Start, store.Stop))

	return store, nil
}

func (s *executorStoreImpl) Start() {
	s.shardCache.Start()
}

func (s *executorStoreImpl) Stop() {
	s.shardCache.Stop()
	s.client.Close()
}

// --- HeartbeatStore Implementation ---

func (s *executorStoreImpl) RecordHeartbeat(ctx context.Context, namespace, executorID string, request store.HeartbeatState) error {
	heartbeatETCDKey, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorHeartbeatKey)
	if err != nil {
		return fmt.Errorf("build executor heartbeat key: %w", err)
	}
	stateETCDKey, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorStatusKey)
	if err != nil {
		return fmt.Errorf("build executor status key: %w", err)
	}
	reportedShardsETCDKey, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorReportedShardsKey)
	if err != nil {
		return fmt.Errorf("build executor reported shards key: %w", err)
	}

	reportedShardsData, err := json.Marshal(request.ReportedShards)
	if err != nil {
		return fmt.Errorf("marshal assinged shards: %w", err)
	}

	jsonState, err := json.Marshal(request.Status)
	if err != nil {
		return fmt.Errorf("marshal assinged shards: %w", err)
	}

	// Atomically update both the timestamp and the state.
	_, err = s.client.Txn(ctx).Then(
		clientv3.OpPut(heartbeatETCDKey, strconv.FormatInt(request.LastHeartbeat, 10)),
		clientv3.OpPut(stateETCDKey, string(jsonState)),
		clientv3.OpPut(reportedShardsETCDKey, string(reportedShardsData)),
	).Commit()

	if err != nil {
		return fmt.Errorf("record heartbeat: %w", err)
	}
	return nil
}

// GetHeartbeat retrieves the last known heartbeat state for a single executor.
func (s *executorStoreImpl) GetHeartbeat(ctx context.Context, namespace string, executorID string) (*store.HeartbeatState, *store.AssignedState, error) {
	// The prefix for all keys related to a single executor.
	executorPrefix, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, "")
	if err != nil {
		return nil, nil, fmt.Errorf("build executor prefix: %w", err)
	}
	resp, err := s.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, nil, fmt.Errorf("etcd get failed for executor %s: %w", executorID, err)
	}

	if resp.Count == 0 {
		return nil, nil, store.ErrExecutorNotFound
	}

	heartbeatState := &store.HeartbeatState{}
	assignedState := &store.AssignedState{}
	found := false

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)
		_, keyType, keyErr := etcdkeys.ParseExecutorKey(s.prefix, namespace, key)
		if keyErr != nil {
			continue // Ignore unexpected keys
		}

		found = true // We found at least one valid key part for the executor.
		switch keyType {
		case etcdkeys.ExecutorHeartbeatKey:
			timestamp, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, nil, fmt.Errorf("parse heartbeat timestamp: %w", err)
			}
			heartbeatState.LastHeartbeat = timestamp
		case etcdkeys.ExecutorStatusKey:
			err := json.Unmarshal([]byte(value), &heartbeatState.Status)
			if err != nil {
				return nil, nil, fmt.Errorf("parse heartbeat state: %w, value %s", err, value)
			}
		case etcdkeys.ExecutorReportedShardsKey:
			err = json.Unmarshal(kv.Value, &heartbeatState.ReportedShards)
			if err != nil {
				return nil, nil, fmt.Errorf("unmarshal reported shards: %w", err)
			}
		case etcdkeys.ExecutorAssignedStateKey:
			err = json.Unmarshal(kv.Value, &assignedState)
			if err != nil {
				return nil, nil, fmt.Errorf("unmarshal assigned shards: %w", err)
			}
		}
	}

	if !found {
		// This case is unlikely if resp.Count > 0, but is a good safeguard.
		return nil, nil, store.ErrExecutorNotFound
	}

	return heartbeatState, assignedState, nil
}

// --- ShardStore Implementation ---

func (s *executorStoreImpl) GetState(ctx context.Context, namespace string) (*store.NamespaceState, error) {
	heartbeatStates := make(map[string]store.HeartbeatState)
	assignedStates := make(map[string]store.AssignedState)
	shardMetrics := make(map[string]store.ShardMetrics)

	executorPrefix := etcdkeys.BuildExecutorPrefix(s.prefix, namespace)
	resp, err := s.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("get executor data: %w", err)
	}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)
		executorID, keyType, keyErr := etcdkeys.ParseExecutorKey(s.prefix, namespace, key)
		if keyErr != nil {
			continue
		}
		heartbeat := heartbeatStates[executorID]
		assigned := assignedStates[executorID]
		switch keyType {
		case etcdkeys.ExecutorHeartbeatKey:
			timestamp, _ := strconv.ParseInt(value, 10, 64)
			heartbeat.LastHeartbeat = timestamp
		case etcdkeys.ExecutorStatusKey:
			err := json.Unmarshal([]byte(value), &heartbeat.Status)
			if err != nil {
				return nil, fmt.Errorf("parse heartbeat state: %w, value %s", err, value)
			}
		case etcdkeys.ExecutorReportedShardsKey:
			err = json.Unmarshal(kv.Value, &heartbeat.ReportedShards)
			if err != nil {
				return nil, fmt.Errorf("unmarshal reported shards: %w", err)
			}
		case etcdkeys.ExecutorAssignedStateKey:
			err = json.Unmarshal(kv.Value, &assigned)
			if err != nil {
				return nil, fmt.Errorf("unmarshal assigned shards: %w, %s", err, value)
			}
			assigned.ModRevision = kv.ModRevision
		}
		heartbeatStates[executorID] = heartbeat
		assignedStates[executorID] = assigned
	}

	// Fetch shard-level metrics stored under shard namespace keys.
	shardPrefix := etcdkeys.BuildShardPrefix(s.prefix, namespace)
	shardResp, err := s.client.Get(ctx, shardPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("get shard data: %w", err)
	}
	for _, kv := range shardResp.Kvs {
		shardID, shardKeyType, err := etcdkeys.ParseShardKey(s.prefix, namespace, string(kv.Key))
		if err != nil {
			continue
		}
		if shardKeyType != etcdkeys.ShardMetricsKey {
			continue
		}
		var shardMetric store.ShardMetrics
		if err := json.Unmarshal(kv.Value, &shardMetric); err != nil {
			continue
		}
		shardMetrics[shardID] = shardMetric
	}

	return &store.NamespaceState{
		Executors:        heartbeatStates,
		ShardMetrics:     shardMetrics,
		ShardAssignments: assignedStates,
		GlobalRevision:   resp.Header.Revision,
	}, nil
}

func (s *executorStoreImpl) Subscribe(ctx context.Context, namespace string) (<-chan int64, error) {
	revisionChan := make(chan int64, 1)
	watchPrefix := etcdkeys.BuildExecutorPrefix(s.prefix, namespace)
	go func() {
		defer close(revisionChan)
		watchChan := s.client.Watch(ctx, watchPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
		for watchResp := range watchChan {
			if err := watchResp.Err(); err != nil {
				return
			}
			isSignificantChange := false
			for _, event := range watchResp.Events {
				if event.IsModify() && bytes.Equal(event.Kv.Value, event.PrevKv.Value) {
					continue // Value is unchanged, ignore this event.
				}

				if !event.IsCreate() && !event.IsModify() {
					isSignificantChange = true
					break
				}
				_, keyType, err := etcdkeys.ParseExecutorKey(s.prefix, namespace, string(event.Kv.Key))
				if err != nil {
					continue
				}
				if keyType != etcdkeys.ExecutorHeartbeatKey && keyType != etcdkeys.ExecutorAssignedStateKey {
					isSignificantChange = true
					break
				}
			}
			if isSignificantChange {
				select {
				case <-revisionChan:
				default:
				}
				revisionChan <- watchResp.Header.Revision
			}
		}
	}()
	return revisionChan, nil
}

func (s *executorStoreImpl) AssignShards(ctx context.Context, namespace string, request store.AssignShardsRequest, guard store.GuardFunc) error {
	var ops []clientv3.Op
	var comparisons []clientv3.Cmp

	// Compute shard moves to update last_move_time metrics when ownership changes.
	// Read current assignments for the namespace and compare with the new state.
	// Concurrent changes will be caught by the revision comparisons later.
	currentAssignments := make(map[string]string) // shardID -> executorID
	executorPrefix := etcdkeys.BuildExecutorPrefix(s.prefix, namespace)
	resp, err := s.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("get current assignments: %w", err)
	}
	for _, kv := range resp.Kvs {
		executorID, keyType, keyErr := etcdkeys.ParseExecutorKey(s.prefix, namespace, string(kv.Key))
		if keyErr != nil || keyType != etcdkeys.ExecutorAssignedStateKey {
			continue
		}
		var state store.AssignedState
		if err := json.Unmarshal(kv.Value, &state); err != nil {
			return fmt.Errorf("unmarshal current assigned state: %w", err)
		}
		for shardID := range state.AssignedShards {
			currentAssignments[shardID] = executorID
		}
	}

	// Build new owner map and detect moved shards.
	newAssignments := make(map[string]string)
	for executorID, state := range request.NewState.ShardAssignments {
		for shardID := range state.AssignedShards {
			newAssignments[shardID] = executorID
		}
	}
	now := time.Now().Unix()
	// Collect metric updates now so we can apply them after committing the main transaction.
	var metricsUpdates []shardMetricsUpdate
	for shardID, newOwner := range newAssignments {
		if oldOwner, ok := currentAssignments[shardID]; ok && oldOwner == newOwner {
			continue
		}
		// For a new or moved shard, update last_move_time while keeping existing metrics if available.
		shardMetricsKey, err := etcdkeys.BuildShardKey(s.prefix, namespace, shardID, etcdkeys.ShardMetricsKey)
		if err != nil {
			return fmt.Errorf("build shard metrics key: %w", err)
		}
		var shardMetrics store.ShardMetrics
		metricsModRevision := int64(0)
		metricsResp, err := s.client.Get(ctx, shardMetricsKey)
		if err != nil {
			return fmt.Errorf("get shard metrics: %w", err)
		}
		if len(metricsResp.Kvs) > 0 {
			metricsModRevision = metricsResp.Kvs[0].ModRevision
			if err := json.Unmarshal(metricsResp.Kvs[0].Value, &shardMetrics); err != nil {
				return fmt.Errorf("unmarshal shard metrics: %w", err)
			}
		} else {
			shardMetrics.SmoothedLoad = 0
			shardMetrics.LastUpdateTime = now
		}
		// Do not set LastMoveTime here, it will be applied later to avoid overwriting
		// a newer timestamp if a concurrent rebalance has already updated it.
		metricsUpdates = append(metricsUpdates, shardMetricsUpdate{
			key:               shardMetricsKey,
			shardID:           shardID,
			metrics:           shardMetrics,
			modRevision:       metricsModRevision,
			desiredLastMove:   now,
			defaultLastUpdate: shardMetrics.LastUpdateTime,
		})
	}

	// 1. Prepare operations to update executor states and shard ownership,
	// and comparisons to check for concurrent modifications.
	for executorID, state := range request.NewState.ShardAssignments {
		// Update the executor's assigned_state key.
		executorStateKey, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorAssignedStateKey)
		if err != nil {
			return fmt.Errorf("build executor assigned state key: %w", err)
		}
		value, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal assigned shards for executor %s: %w", executorID, err)
		}
		ops = append(ops, clientv3.OpPut(executorStateKey, string(value)))
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(executorStateKey), "=", state.ModRevision))
	}

	if len(ops) == 0 {
		return nil
	}

	// 2. Apply the guard function to get the base transaction, which may already have an 'If' condition for leadership.
	nativeTxn := s.client.Txn(ctx)
	guardedTxn, err := guard(nativeTxn)
	if err != nil {
		return fmt.Errorf("apply transaction guard: %w", err)
	}
	etcdGuardedTxn, ok := guardedTxn.(clientv3.Txn)
	if !ok {
		return fmt.Errorf("guard function returned invalid transaction type")
	}

	// 3. Create a nested transaction operation. This allows us to add our own 'If' (comparisons)
	// and 'Then' (ops) logic that will only execute if the outer guard's 'If' condition passes.
	nestedTxnOp := clientv3.OpTxn(
		comparisons, // Our IF conditions
		ops,         // Our THEN operations
		nil,         // Our ELSE operations
	)

	// 4. Add the nested transaction to the guarded transaction's THEN clause and commit.
	etcdGuardedTxn = etcdGuardedTxn.Then(nestedTxnOp)
	txnResp, err := etcdGuardedTxn.Commit()
	if err != nil {
		return fmt.Errorf("commit shard assignments transaction: %w", err)
	}

	// 5. Check the results of both the outer and nested transactions.
	if !txnResp.Succeeded {
		// This means the guard's condition (e.g., leadership) failed.
		return fmt.Errorf("%w: transaction failed, leadership may have changed", store.ErrVersionConflict)
	}

	// The guard's condition passed. Now check if our nested transaction succeeded.
	// Since we only have one Op in our 'Then', we check the first response.
	if len(txnResp.Responses) == 0 {
		return fmt.Errorf("unexpected empty response from transaction")
	}
	nestedResp := txnResp.Responses[0].GetResponseTxn()
	if !nestedResp.Succeeded {
		// This means our revision checks failed.
		return fmt.Errorf("%w: transaction failed, a shard may have been concurrently assigned", store.ErrVersionConflict)
	}

	// Apply shard metrics updates outside the main transaction to stay within etcd's max operations per txn.
	s.applyShardMetricsUpdates(ctx, namespace, metricsUpdates)

	return nil
}

// applyShardMetricsUpdates updates shard metrics (like last_move_time) after AssignShards.
// Decided to run these writes outside the primary transaction
// so we are less likely to exceed etcd's max txn-op threshold (128?), and we retry
// logs failures instead of failing the main assignment.
func (s *executorStoreImpl) applyShardMetricsUpdates(ctx context.Context, namespace string, updates []shardMetricsUpdate) {
	for i := range updates {
		update := &updates[i]

		for {
			// If a newer rebalance already set a later LastMoveTime, there's nothing left for this iteration.
			if update.metrics.LastMoveTime >= update.desiredLastMove {
				break
			}

			update.metrics.LastMoveTime = update.desiredLastMove

			payload, err := json.Marshal(update.metrics)
			if err != nil {
				// Log and move on. failing metrics formatting should not invalidate the finished assignment.
				s.logger.Warn("failed to marshal shard metrics after assignment", tag.ShardNamespace(namespace), tag.ShardKey(update.shardID), tag.Error(err))
				break
			}

			txnResp, err := s.client.Txn(ctx).
				If(clientv3.Compare(clientv3.ModRevision(update.key), "=", update.modRevision)).
				Then(clientv3.OpPut(update.key, string(payload))).
				Commit()
			if err != nil {
				// log and abandon this shard rather than propagating an error after assignments commit.
				s.logger.Warn("failed to commit shard metrics update after assignment", tag.ShardNamespace(namespace), tag.ShardKey(update.shardID), tag.Error(err))
				break
			}

			if txnResp.Succeeded {
				break
			}

			if ctx.Err() != nil {
				s.logger.Warn("context canceled while updating shard metrics", tag.ShardNamespace(namespace), tag.ShardKey(update.shardID), tag.Error(ctx.Err()))
				return
			}

			// Another writer beat us. pull the latest metrics so we can merge their view and retry.
			metricsResp, err := s.client.Get(ctx, update.key)
			if err != nil {
				// Unable to observe the conflicting write, so we skip this shard and keep the assignment result.
				s.logger.Warn("failed to refresh shard metrics after compare conflict", tag.ShardNamespace(namespace), tag.ShardKey(update.shardID), tag.Error(err))
				break
			}

			update.modRevision = 0
			if len(metricsResp.Kvs) > 0 {
				update.modRevision = metricsResp.Kvs[0].ModRevision
				if err := json.Unmarshal(metricsResp.Kvs[0].Value, &update.metrics); err != nil {
					// If the value is corrupt we cannot safely merge, so we abandon this shard's metrics update.
					s.logger.Warn("failed to unmarshal shard metrics after compare conflict", tag.ShardNamespace(namespace), tag.ShardKey(update.shardID), tag.Error(err))
					break
				}
			} else {
				update.metrics = store.ShardMetrics{
					SmoothedLoad:   0,
					LastUpdateTime: update.defaultLastUpdate,
				}
				update.modRevision = 0
			}
		}
	}
}

func (s *executorStoreImpl) AssignShard(ctx context.Context, namespace, shardID, executorID string) error {
	assignedState, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorAssignedStateKey)
	if err != nil {
		return fmt.Errorf("build executor assigned state key: %w", err)
	}
	statusKey, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorStatusKey)
	if err != nil {
		return fmt.Errorf("build executor status key: %w", err)
	}
	shardMetricsKey, err := etcdkeys.BuildShardKey(s.prefix, namespace, shardID, etcdkeys.ShardMetricsKey)
	if err != nil {
		return fmt.Errorf("build shard metrics key: %w", err)
	}

	// Use a read-modify-write loop to handle concurrent updates safely.
	for {
		// 1. Get the current assigned state of the executor and prepare the shard metrics.
		resp, err := s.client.Get(ctx, assignedState)
		if err != nil {
			return fmt.Errorf("get executor state: %w", err)
		}

		var state store.AssignedState
		var shardMetrics store.ShardMetrics
		modRevision := int64(0) // A revision of 0 means the key doesn't exist yet.

		if len(resp.Kvs) > 0 {
			// If the executor already has shards, load its state.
			kv := resp.Kvs[0]
			modRevision = kv.ModRevision
			if err := json.Unmarshal(kv.Value, &state); err != nil {
				return fmt.Errorf("unmarshal assigned state: %w", err)
			}
		} else {
			// If this is the first shard, initialize the state map.
			state.AssignedShards = make(map[string]*types.ShardAssignment)
		}

		metricsResp, err := s.client.Get(ctx, shardMetricsKey)
		if err != nil {
			return fmt.Errorf("get shard metrics: %w", err)
		}
		now := time.Now().Unix()
		metricsModRevision := int64(0)
		if len(metricsResp.Kvs) > 0 {
			metricsModRevision = metricsResp.Kvs[0].ModRevision
			if err := json.Unmarshal(metricsResp.Kvs[0].Value, &shardMetrics); err != nil {
				return fmt.Errorf("unmarshal shard metrics: %w", err)
			}
			// Metrics already exist, update the last move time.
			// This can happen if the shard was previously assigned to an executor, and a lookup happens after the executor is deleted,
			// AssignShard is then called to assign the shard to a new executor.
			shardMetrics.LastMoveTime = now
		} else {
			// Metrics don't exist, initialize them.
			shardMetrics.SmoothedLoad = 0
			shardMetrics.LastUpdateTime = now
			shardMetrics.LastMoveTime = now
		}

		// 2. Modify the state in memory, adding the new shard if it's not already there.
		if _, alreadyAssigned := state.AssignedShards[shardID]; !alreadyAssigned {
			state.AssignedShards[shardID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		}

		newStateValue, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal new assigned state: %w", err)
		}

		newMetricsValue, err := json.Marshal(shardMetrics)
		if err != nil {
			return fmt.Errorf("marshal new shard metrics: %w", err)
		}

		var comparisons []clientv3.Cmp

		// 3. Prepare and commit the transaction with four atomic checks.
		// a) Check that the executor's status is ACTIVE.
		comparisons = append(comparisons, clientv3.Compare(clientv3.Value(statusKey), "=", _executorStatusRunningJSON))
		// b) Check that neither the assigned_state nor shard metrics were modified concurrently.
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(assignedState), "=", modRevision))
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(shardMetricsKey), "=", metricsModRevision))
		// c) Check that the cache is up to date.
		cmp, err := s.shardCache.GetExecutorModRevisionCmp(namespace)
		if err != nil {
			return fmt.Errorf("get executor mod revision cmp: %w", err)
		}
		comparisons = append(comparisons, cmp...)

		// We check the shard cache to see if the shard is already assigned to an executor.
		owner, err := s.shardCache.GetShardOwner(ctx, namespace, shardID)
		if err != nil && !errors.Is(err, store.ErrShardNotFound) {
			return fmt.Errorf("checking shard owner: %w", err)
		}
		if err == nil {
			return &store.ErrShardAlreadyAssigned{ShardID: shardID, AssignedTo: owner}
		}

		txnResp, err := s.client.Txn(ctx).
			If(comparisons...).
			Then(
				clientv3.OpPut(assignedState, string(newStateValue)),
				clientv3.OpPut(shardMetricsKey, string(newMetricsValue)),
			).
			Commit()

		if err != nil {
			return fmt.Errorf("assign shard transaction: %w", err)
		}

		if txnResp.Succeeded {
			return nil
		}

		// If the transaction failed, another process interfered.
		// Provide a specific error if the status check failed.
		currentStatusResp, err := s.client.Get(ctx, statusKey)
		if err != nil || len(currentStatusResp.Kvs) == 0 {
			return store.ErrExecutorNotFound
		}
		if string(currentStatusResp.Kvs[0].Value) != _executorStatusRunningJSON {
			return fmt.Errorf(`%w: executor status is %s"`, store.ErrVersionConflict, currentStatusResp.Kvs[0].Value)
		}

		s.logger.Info("Assign shard transaction failed due to a conflict. Retrying...", tag.ShardNamespace(namespace), tag.ShardKey(shardID), tag.ShardExecutor(executorID))
		// Otherwise, it was a revision mismatch. Loop to retry the operation.
	}
}

// DeleteExecutors deletes the given executors from the store. It does not delete the shards owned by the executors, this
// should be handled by the namespace processor loop as we want to reassign, not delete the shards.
func (s *executorStoreImpl) DeleteExecutors(ctx context.Context, namespace string, executorIDs []string, guard store.GuardFunc) error {
	if len(executorIDs) == 0 {
		return nil
	}
	var ops []clientv3.Op

	for _, executorID := range executorIDs {
		executorPrefix, err := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, "")
		if err != nil {
			return fmt.Errorf("build executor prefix: %w", err)
		}
		ops = append(ops, clientv3.OpDelete(executorPrefix, clientv3.WithPrefix()))
	}

	if len(ops) == 0 {
		return nil
	}

	nativeTxn := s.client.Txn(ctx)
	guardedTxn, err := guard(nativeTxn)
	if err != nil {
		return fmt.Errorf("apply transaction guard: %w", err)
	}
	etcdGuardedTxn, ok := guardedTxn.(clientv3.Txn)
	if !ok {
		return fmt.Errorf("guard function returned invalid transaction type")
	}

	etcdGuardedTxn = etcdGuardedTxn.Then(ops...)
	resp, err := etcdGuardedTxn.Commit()
	if err != nil {
		return fmt.Errorf("commit executor deletion: %w", err)
	}
	if !resp.Succeeded {
		return fmt.Errorf("transaction failed, leadership may have changed")
	}
	return nil
}

func (s *executorStoreImpl) GetShardOwner(ctx context.Context, namespace, shardID string) (string, error) {
	return s.shardCache.GetShardOwner(ctx, namespace, shardID)
}
