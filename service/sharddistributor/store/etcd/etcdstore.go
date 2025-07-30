package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func init() {
	store.Register("etcd", fx.Provide(NewStore))
}

const (
	executorHeartbeatKey      = "heartbeat"
	executorStatusKey         = "status"
	executorReportedShardsKey = "reported_shards"
	executorAssignedStateKey  = "assigned_state"
	shardAssignedKey          = "assigned"
)

var (
	_executorStatusRunningJSON = fmt.Sprintf(`"%s"`, types.ExecutorStatusACTIVE)
)

// Store implements the generic store.Store interface using etcd as the backend.
type Store struct {
	client *clientv3.Client
	prefix string
}

// StoreParams defines the dependencies for the etcd store, for use with fx.
type StoreParams struct {
	fx.In

	Client    *clientv3.Client `optional:"true"`
	Cfg       config.LeaderElection
	Lifecycle fx.Lifecycle
}

// NewStore creates a new etcd-backed store and provides it to the fx application.
func NewStore(p StoreParams) (store.Store, error) {
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

	p.Lifecycle.Append(fx.StopHook(etcdClient.Close))

	return &Store{
		client: etcdClient,
		prefix: etcdCfg.Prefix,
	}, nil
}

// --- HeartbeatStore Implementation ---

func (s *Store) RecordHeartbeat(ctx context.Context, namespace, executorID string, request store.HeartbeatState) error {
	heartbeatETCDKey := s.buildExecutorKey(namespace, executorID, executorHeartbeatKey)
	stateETCDKey := s.buildExecutorKey(namespace, executorID, executorStatusKey)
	reportedShardsETCDKey := s.buildExecutorKey(namespace, executorID, executorReportedShardsKey)

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
func (s *Store) GetHeartbeat(ctx context.Context, namespace string, executorID string) (*store.HeartbeatState, *store.AssignedState, error) {
	// The prefix for all keys related to a single executor.
	executorPrefix := s.buildExecutorKey(namespace, executorID, "")
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
		_, keyType, keyErr := s.parseExecutorKey(namespace, key)
		if keyErr != nil {
			continue // Ignore unexpected keys
		}

		found = true // We found at least one valid key part for the executor.
		switch keyType {
		case executorHeartbeatKey:
			timestamp, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, nil, fmt.Errorf("parse heartbeat timestamp: %w", err)
			}
			heartbeatState.LastHeartbeat = timestamp
		case executorStatusKey:
			err := json.Unmarshal([]byte(value), &heartbeatState.Status)
			if err != nil {
				return nil, nil, fmt.Errorf("parse heartbeat state: %w, value %s", err, value)
			}
		case executorReportedShardsKey:
			err = json.Unmarshal(kv.Value, &heartbeatState.ReportedShards)
			if err != nil {
				return nil, nil, fmt.Errorf("unmarshal reported shards: %w", err)
			}
		case executorAssignedStateKey:
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

func (s *Store) GetState(ctx context.Context, namespace string) (*store.NamespaceState, error) {
	heartbeatStates := make(map[string]store.HeartbeatState)
	assignedStates := make(map[string]store.AssignedState)

	executorPrefix := s.buildExecutorPrefix(namespace)
	resp, err := s.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("get executor data: %w", err)
	}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)
		executorID, keyType, keyErr := s.parseExecutorKey(namespace, key)
		if keyErr != nil {
			continue
		}
		heartbeat := heartbeatStates[executorID]
		assigned := assignedStates[executorID]
		switch keyType {
		case executorHeartbeatKey:
			timestamp, _ := strconv.ParseInt(value, 10, 64)
			heartbeat.LastHeartbeat = timestamp
		case executorStatusKey:
			err := json.Unmarshal([]byte(value), &heartbeat.Status)
			if err != nil {
				return nil, fmt.Errorf("parse heartbeat state: %w, value %s", err, value)
			}
		case executorReportedShardsKey:
			err = json.Unmarshal(kv.Value, &heartbeat.ReportedShards)
			if err != nil {
				return nil, fmt.Errorf("unmarshal reported shards: %w", err)
			}
		case executorAssignedStateKey:
			err = json.Unmarshal(kv.Value, &assigned)
			if err != nil {
				return nil, fmt.Errorf("unmarshal assigned shards: %w, %s", err, value)
			}
		}
		heartbeatStates[executorID] = heartbeat
		assignedStates[executorID] = assigned
	}

	shardStates := make(map[string]store.ShardState)
	shardsPrefix := s.buildShardsPrefix(namespace)
	shardResp, err := s.client.Get(ctx, shardsPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("get shard ownership data: %w", err)
	}
	for _, kv := range shardResp.Kvs {
		key := string(kv.Key)
		remainder := strings.TrimPrefix(key, shardsPrefix)
		parts := strings.Split(remainder, "/")
		if len(parts) < 2 || parts[1] != shardAssignedKey {
			continue
		}
		shardID := parts[0]
		shardStates[shardID] = store.ShardState{
			ExecutorID: string(kv.Value),
			Revision:   kv.ModRevision,
		}
	}

	return &store.NamespaceState{
		Executors:        heartbeatStates,
		Shards:           shardStates,
		ShardAssignments: assignedStates,
		GlobalRevision:   resp.Header.Revision,
	}, nil
}

func (s *Store) Subscribe(ctx context.Context, namespace string) (<-chan int64, error) {
	revisionChan := make(chan int64, 1)
	watchPrefix := s.buildExecutorPrefix(namespace)
	go func() {
		defer close(revisionChan)
		watchChan := s.client.Watch(ctx, watchPrefix, clientv3.WithPrefix())
		for watchResp := range watchChan {
			if err := watchResp.Err(); err != nil {
				return
			}
			isSignificantChange := false
			for _, event := range watchResp.Events {
				if !event.IsCreate() && !event.IsModify() {
					isSignificantChange = true
					break
				}
				_, keyType, err := s.parseExecutorKey(namespace, string(event.Kv.Key))
				if err != nil {
					continue
				}
				if keyType != executorHeartbeatKey && keyType != executorAssignedStateKey {
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

func (s *Store) AssignShards(ctx context.Context, namespace string, newState *store.NamespaceState, guard store.GuardFunc) error {
	var ops []clientv3.Op
	var comparisons []clientv3.Cmp

	// 1. Prepare operations to update executor states and shard ownership,
	// and comparisons to check for concurrent modifications.
	for executorID, state := range newState.ShardAssignments {
		// Update the executor's assigned_state key.
		executorStateKey := s.buildExecutorKey(namespace, executorID, executorAssignedStateKey)
		value, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal assigned shards for executor %s: %w", executorID, err)
		}
		ops = append(ops, clientv3.OpPut(executorStateKey, string(value)))

		// For each shard in the new assignment, add a Put operation and a revision check.
		for shardID := range state.AssignedShards {
			shardOwnerKey := s.buildShardKey(namespace, shardID, shardAssignedKey)
			ops = append(ops, clientv3.OpPut(shardOwnerKey, executorID))

			// Check the revision of the shard from the state we read in GetState.
			previousShardState, ok := newState.Shards[shardID]
			if ok {
				// The shard existed before. Check that its revision has not changed.
				// This handles moves and re-assignments to the same executor.
				comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(shardOwnerKey), "=", previousShardState.Revision))
			} else {
				// The shard is new. Check that it has not been created by another process.
				// A non-existent key has a ModRevision of 0.
				comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(shardOwnerKey), "=", 0))
			}
		}
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

	return nil
}

func (s *Store) GetShardOwner(ctx context.Context, namespace, shardID string) (string, error) {
	key := s.buildShardKey(namespace, shardID, shardAssignedKey)
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("etcd get for shard %s: %w", shardID, err)
	}
	if resp.Count == 0 {
		return "", store.ErrShardNotFound
	}
	return string(resp.Kvs[0].Value), nil
}

func (s *Store) AssignShard(ctx context.Context, namespace, shardID, executorID string) error {
	assignedState := s.buildExecutorKey(namespace, executorID, executorAssignedStateKey)
	statusKey := s.buildExecutorKey(namespace, executorID, executorStatusKey)
	shardOwnerKey := s.buildShardKey(namespace, shardID, shardAssignedKey)

	// Use a read-modify-write loop to handle concurrent updates safely.
	for {
		// 1. Get the current assigned state of the executor.
		resp, err := s.client.Get(ctx, assignedState)
		if err != nil {
			return fmt.Errorf("get executor state: %w", err)
		}

		var state store.AssignedState
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

		// 2. Modify the state in memory, adding the new shard if it's not already there.
		if _, alreadyAssigned := state.AssignedShards[shardID]; !alreadyAssigned {
			state.AssignedShards[shardID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		}

		newStateValue, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal new assigned state: %w", err)
		}

		// 3. Prepare and commit the transaction with three atomic checks.
		// a) Check that the shard is not already assigned (its key revision must be 0).
		cmpShardUnassigned := clientv3.Compare(clientv3.ModRevision(shardOwnerKey), "=", 0)
		// b) Check that the executor's status is ACTIVE.
		cmpStatus := clientv3.Compare(clientv3.Value(statusKey), "=", _executorStatusRunningJSON)
		// c) Check that the assigned_state key hasn't been changed by another process.
		cmpAssignedState := clientv3.Compare(clientv3.ModRevision(assignedState), "=", modRevision)

		opUpdateExecutorState := clientv3.OpPut(assignedState, string(newStateValue))
		opUpdateShardOwner := clientv3.OpPut(shardOwnerKey, executorID)

		txnResp, err := s.client.Txn(ctx).
			If(cmpShardUnassigned, cmpStatus, cmpAssignedState). // All conditions must be true.
			Then(opUpdateExecutorState, opUpdateShardOwner).
			Commit()

		if err != nil {
			return fmt.Errorf("assign shard transaction: %w", err)
		}

		if txnResp.Succeeded {
			return nil
		}

		// If the transaction failed, diagnose the reason to return a specific error.
		// Check for the new failure condition first.
		shardResp, err := s.client.Get(ctx, shardOwnerKey)
		if err != nil {
			return fmt.Errorf("check shard owner after failed transaction: %w", err)
		}
		if len(shardResp.Kvs) > 0 {
			// The shard key exists, meaning it's already assigned. This was the reason for failure.
			return fmt.Errorf("%w: shard is already owned by %s", store.ErrVersionConflict, shardResp.Kvs[0].Value)
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

		// Otherwise, it was a revision mismatch. Loop to retry the operation.
	}
}

func (s *Store) DeleteExecutors(ctx context.Context, namespace string, executorIDs []string, guard store.GuardFunc) error {
	if len(executorIDs) == 0 {
		return nil
	}
	var ops []clientv3.Op

	// For each executor, find all shards it owns and create conditional deletion operations.
	for _, executorID := range executorIDs {
		// 1. Find the shards currently assigned to this executor.
		assignedStateKey := s.buildExecutorKey(namespace, executorID, executorAssignedStateKey)
		resp, err := s.client.Get(ctx, assignedStateKey)
		if err != nil {
			return fmt.Errorf("get assigned state for executor %s: %w", executorID, err)
		}

		if len(resp.Kvs) > 0 {
			var state store.AssignedState
			if err := json.Unmarshal(resp.Kvs[0].Value, &state); err != nil {
				return fmt.Errorf("unmarshal assigned state for executor %s: %w", executorID, err)
			}

			// 2. For each shard, create a nested transaction to conditionally delete it.
			for shardID := range state.AssignedShards {
				shardOwnerKey := s.buildShardKey(namespace, shardID, shardAssignedKey)

				// This is an atomic check-and-delete operation for a single shard.
				// IF the shard owner is still the executor being deleted,
				// THEN delete the shard ownership key.
				conditionalDelete := clientv3.OpTxn(
					[]clientv3.Cmp{clientv3.Compare(clientv3.Value(shardOwnerKey), "=", executorID)}, // IF condition
					[]clientv3.Op{clientv3.OpDelete(shardOwnerKey)},                                  // THEN operations
					nil, // ELSE do nothing
				)
				ops = append(ops, conditionalDelete)
			}
		}

		// 3. Add the unconditional deletion for the executor's entire record.
		executorPrefix := s.buildExecutorKey(namespace, executorID, "")
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

// --- Key Management Utilities ---

func (s *Store) buildNamespacePrefix(namespace string) string {
	return fmt.Sprintf("%s/%s", s.prefix, namespace)
}

func (s *Store) buildShardsPrefix(namespace string) string {
	return fmt.Sprintf("%s/shards/", s.buildNamespacePrefix(namespace))
}

func (s *Store) buildShardKey(namespace, shardID string, keyType string) string {
	return fmt.Sprintf("%s%s/%s", s.buildShardsPrefix(namespace), shardID, keyType)
}

func (s *Store) buildExecutorPrefix(namespace string) string {
	return fmt.Sprintf("%s/executors/", s.buildNamespacePrefix(namespace))
}

func (s *Store) buildExecutorKey(namespace, executorID, keyType string) string {
	return fmt.Sprintf("%s%s/%s", s.buildExecutorPrefix(namespace), executorID, keyType)
}

func (s *Store) parseExecutorKey(namespace, key string) (executorID, keyType string, err error) {
	prefix := s.buildExecutorPrefix(namespace)
	if !strings.HasPrefix(key, prefix) {
		return "", "", fmt.Errorf("key '%s' does not have expected prefix '%s'", key, prefix)
	}
	remainder := strings.TrimPrefix(key, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected key format: %s", key)
	}
	return parts[0], parts[1], nil
}
