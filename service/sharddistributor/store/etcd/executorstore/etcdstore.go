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

	// Build all operations including metadata
	ops := []clientv3.Op{
		clientv3.OpPut(heartbeatETCDKey, strconv.FormatInt(request.LastHeartbeat, 10)),
		clientv3.OpPut(stateETCDKey, string(jsonState)),
		clientv3.OpPut(reportedShardsETCDKey, string(reportedShardsData)),
	}
	for key, value := range request.Metadata {
		metadataKey := etcdkeys.BuildMetadataKey(s.prefix, namespace, executorID, key)
		ops = append(ops, clientv3.OpPut(metadataKey, value))
	}

	// Atomically update both the timestamp and the state.
	_, err = s.client.Txn(ctx).Then(ops...).Commit()

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

	return &store.NamespaceState{
		Executors:        heartbeatStates,
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

	return nil
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

		var comparisons []clientv3.Cmp

		// 3. Prepare and commit the transaction with three atomic checks.
		// a) Check that the executor's status is ACTIVE.
		comparisons = append(comparisons, clientv3.Compare(clientv3.Value(statusKey), "=", _executorStatusRunningJSON))
		// b) Check that the assigned_state key hasn't been changed by another process.
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(assignedState), "=", modRevision))
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
			Then(clientv3.OpPut(assignedState, string(newStateValue))).
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
