package executorstore

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=executorstore_mock.go ExecutorStore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdclient"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore/common"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore/shardcache"
)

var (
	_executorStatusRunningJSON = fmt.Sprintf(`"%s"`, types.ExecutorStatusACTIVE)
)

type executorStoreImpl struct {
	client       etcdclient.Client
	prefix       string
	logger       log.Logger
	shardCache   *shardcache.ShardToExecutorCache
	timeSource   clock.TimeSource
	recordWriter *common.RecordWriter
}

// shardStatisticsUpdate holds the staged statistics for a shard so we can write them
// to etcd after the main AssignShards transaction commits.
type shardStatisticsUpdate struct {
	executorID string
	stats      map[string]etcdtypes.ShardStatistics
}

// ExecutorStoreParams defines the dependencies for the etcd store, for use with fx.
type ExecutorStoreParams struct {
	fx.In

	Client     etcdclient.Client `name:"executorstore"`
	Cfg        ETCDConfig
	Lifecycle  fx.Lifecycle
	Logger     log.Logger
	TimeSource clock.TimeSource
}

// NewStore creates a new etcd-backed store and provides it to the fx application.
func NewStore(p ExecutorStoreParams) (store.Store, error) {
	shardCache := shardcache.NewShardToExecutorCache(p.Cfg.Prefix, p.Client, p.Logger)

	timeSource := p.TimeSource
	if timeSource == nil {
		timeSource = clock.NewRealTimeSource()
	}

	recordWriter, err := common.NewRecordWriter(p.Cfg.Compression)
	if err != nil {
		return nil, fmt.Errorf("create record writer: %w", err)
	}
	store := &executorStoreImpl{
		client:       p.Client,
		prefix:       p.Cfg.Prefix,
		logger:       p.Logger,
		shardCache:   shardCache,
		timeSource:   timeSource,
		recordWriter: recordWriter,
	}

	p.Lifecycle.Append(fx.StartStopHook(store.Start, store.Stop))

	return store, nil
}

func (s *executorStoreImpl) Start() {
	s.shardCache.Start()
}

func (s *executorStoreImpl) Stop() {
	s.shardCache.Stop()
}

// --- HeartbeatStore Implementation ---

func (s *executorStoreImpl) RecordHeartbeat(ctx context.Context, namespace, executorID string, request store.HeartbeatState) error {
	heartbeatKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorHeartbeatKey)
	stateKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorStatusKey)
	reportedShardsKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorReportedShardsKey)

	reportedShardsData, err := json.Marshal(request.ReportedShards)
	if err != nil {
		return fmt.Errorf("marshal reported shards: %w", err)
	}

	jsonState, err := json.Marshal(request.Status)
	if err != nil {
		return fmt.Errorf("marshal assinged state: %w", err)
	}

	// Compress data before writing to etcd
	compressedReportedShards, err := s.recordWriter.Write(reportedShardsData)
	if err != nil {
		return fmt.Errorf("compress reported shards: %w", err)
	}

	compressedState, err := s.recordWriter.Write(jsonState)
	if err != nil {
		return fmt.Errorf("compress assigned state: %w", err)
	}

	// Build all operations including metadata
	ops := []clientv3.Op{
		clientv3.OpPut(heartbeatKey, etcdtypes.FormatTime(request.LastHeartbeat)),
		clientv3.OpPut(stateKey, string(compressedState)),
		clientv3.OpPut(reportedShardsKey, string(compressedReportedShards)),
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

	err = s.recordShardStatistics(ctx, namespace, executorID, request.ReportedShards)
	if err != nil {
		return err
	}

	return nil
}

func (s *executorStoreImpl) recordShardStatistics(ctx context.Context, namespace, executorID string, reported map[string]*types.ShardStatusReport) error {
	if len(reported) == 0 {
		return nil
	}

	oldStats, err := s.shardCache.GetExecutorStatistics(ctx, namespace, executorID)
	if err != nil {
		return err
	}

	now := s.timeSource.Now().UTC()

	var statsUpdates []shardStatisticsUpdate
	var statsUpdate shardStatisticsUpdate
	statsUpdate.executorID = executorID
	statsUpdate.stats = make(map[string]etcdtypes.ShardStatistics)

	for shardID, report := range reported {
		if report == nil {
			s.logger.Warn("empty report; skipping EWMA update",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(executorID),
				tag.ShardKey(shardID),
			)
			continue
		}

		load := report.ShardLoad
		if math.IsNaN(load) || math.IsInf(load, 0) {
			s.logger.Warn(
				"invalid shard load reported; skipping EWMA update",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(executorID),
				tag.ShardKey(shardID),
			)
			continue
		}

		var stats etcdtypes.ShardStatistics

		prevStats, ok := oldStats[shardID]
		if ok {
			stats.LastMoveTime = prevStats.LastMoveTime
		}

		prevSmoothed := prevStats.SmoothedLoad
		prevUpdate := prevStats.LastUpdateTime.ToTime()
		newSmoothed := ewmaSmoothedLoad(prevSmoothed, load, prevUpdate, now)

		stats.SmoothedLoad = newSmoothed
		stats.LastUpdateTime = etcdtypes.Time(now)

		statsUpdate.stats[shardID] = stats
	}

	statsUpdates = append(statsUpdates, statsUpdate)

	s.applyShardStatisticsUpdates(ctx, namespace, statsUpdates)

	return nil
}

// GetHeartbeat retrieves the last known heartbeat state for a single executor.
func (s *executorStoreImpl) GetHeartbeat(ctx context.Context, namespace string, executorID string) (*store.HeartbeatState, *store.AssignedState, error) {
	// The prefix for all keys related to a single executor.
	executorIDPrefix := etcdkeys.BuildExecutorIDPrefix(s.prefix, namespace, executorID)
	resp, err := s.client.Get(ctx, executorIDPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, nil, fmt.Errorf("etcd get failed for executor %s: %w", executorID, err)
	}

	if resp.Count == 0 {
		return nil, nil, store.ErrExecutorNotFound
	}

	heartbeatState := &store.HeartbeatState{}
	assignedState := &etcdtypes.AssignedState{}
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
			heartbeatState.LastHeartbeat, err = etcdtypes.ParseTime(value)
			if err != nil {
				return nil, nil, fmt.Errorf("parse heartbeat timestamp: %w", err)
			}
		case etcdkeys.ExecutorStatusKey:
			if err := common.DecompressAndUnmarshal(kv.Value, &heartbeatState.Status); err != nil {
				return nil, nil, fmt.Errorf("parse executor status: %w", err)
			}
		case etcdkeys.ExecutorReportedShardsKey:
			if err := common.DecompressAndUnmarshal(kv.Value, &heartbeatState.ReportedShards); err != nil {
				return nil, nil, fmt.Errorf("parse reported shards: %w", err)
			}
		case etcdkeys.ExecutorAssignedStateKey:
			assignedState.ModRevision = kv.ModRevision
			if err := common.DecompressAndUnmarshal(kv.Value, &assignedState); err != nil {
				return nil, nil, fmt.Errorf("parse assigned shards: %w", err)
			}
		}
	}

	if !found {
		// This case is unlikely if resp.Count > 0, but is a good safeguard.
		return nil, nil, store.ErrExecutorNotFound
	}

	return heartbeatState, assignedState.ToAssignedState(), nil
}

// --- ShardStore Implementation ---

func (s *executorStoreImpl) GetState(ctx context.Context, namespace string) (*store.NamespaceState, error) {
	heartbeatStates := make(map[string]store.HeartbeatState)
	assignedStates := make(map[string]store.AssignedState)
	shardStats := make(map[string]store.ShardStatistics)

	executorPrefix := etcdkeys.BuildExecutorsPrefix(s.prefix, namespace)
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
			heartbeat.LastHeartbeat, err = etcdtypes.ParseTime(value)
			if err != nil {
				return nil, fmt.Errorf("parse heartbeat timestamp: %w", err)
			}
		case etcdkeys.ExecutorStatusKey:
			if err := common.DecompressAndUnmarshal(kv.Value, &heartbeat.Status); err != nil {
				return nil, fmt.Errorf("parse executor status: %w", err)
			}
		case etcdkeys.ExecutorReportedShardsKey:
			if err := common.DecompressAndUnmarshal(kv.Value, &heartbeat.ReportedShards); err != nil {
				return nil, fmt.Errorf("parse reported shards: %w", err)
			}
		case etcdkeys.ExecutorAssignedStateKey:
			var assignedRaw etcdtypes.AssignedState
			if err := common.DecompressAndUnmarshal(kv.Value, &assignedRaw); err != nil {
				return nil, fmt.Errorf("parse assigned shards: %w, %s", err, value)
			}
			assignedRaw.ModRevision = kv.ModRevision
			assigned = *assignedRaw.ToAssignedState()
		case etcdkeys.ExecutorShardStatisticsKey:
			executorShardStats := make(map[string]etcdtypes.ShardStatistics)
			if err := common.DecompressAndUnmarshal(kv.Value, &executorShardStats); err != nil {
				return nil, fmt.Errorf("parse executor shard statistics: %w, %s", err, value)
			}
			for shardID, stat := range executorShardStats {
				shardStats[shardID] = *stat.ToShardStatistics()
			}
		}
		heartbeatStates[executorID] = heartbeat
		assignedStates[executorID] = assigned
	}

	return &store.NamespaceState{
		Executors:        heartbeatStates,
		ShardStats:       shardStats,
		ShardAssignments: assignedStates,
		GlobalRevision:   resp.Header.Revision,
	}, nil
}

func (s *executorStoreImpl) SubscribeToAssignmentChanges(ctx context.Context, namespace string) (<-chan map[*store.ShardOwner][]string, func(), error) {
	return s.shardCache.Subscribe(ctx, namespace)
}

func (s *executorStoreImpl) Subscribe(ctx context.Context, namespace string) (<-chan int64, error) {
	revisionChan := make(chan int64, 1)
	watchPrefix := etcdkeys.BuildExecutorsPrefix(s.prefix, namespace)
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
				// Treat heartbeat, assigned_state and statistics updates as non-significant for rebalancing.
				if keyType != etcdkeys.ExecutorHeartbeatKey &&
					keyType != etcdkeys.ExecutorAssignedStateKey &&
					keyType != etcdkeys.ExecutorShardStatisticsKey {
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
	var opsElse []clientv3.Op
	var comparisons []clientv3.Cmp
	comparisonMaps := make(map[string]int64)

	statsUpdates, err := s.prepareShardStatisticsUpdates(ctx, namespace, request.NewState.ShardAssignments)
	if err != nil {
		return fmt.Errorf("prepare shard statistics: %w", err)
	}

	// 1. Prepare operations to delete stale executors and add comparisons to ensure they haven't been modified
	for executorID, expectedModRevision := range request.ExecutorsToDelete {
		// Build the assigned state key to check for concurrent modifications
		executorStateKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorAssignedStateKey)

		// Add a comparison to ensure the executor's assigned state hasn't changed
		// This prevents deleting an executor that just received a shard assignment
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(executorStateKey), "=", expectedModRevision))
		comparisonMaps[executorStateKey] = expectedModRevision

		// Delete all keys for this executor
		executorPrefix := etcdkeys.BuildExecutorIDPrefix(s.prefix, namespace, executorID)
		ops = append(ops, clientv3.OpDelete(executorPrefix, clientv3.WithPrefix()))
		opsElse = append(opsElse, clientv3.OpGet(executorStateKey))
	}

	// 2. Prepare operations to update executor states and shard ownership,
	// and comparisons to check for concurrent modifications.
	for executorID, state := range request.NewState.ShardAssignments {
		// Update the executor's assigned_state key.
		executorStateKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorAssignedStateKey)
		value, err := json.Marshal(etcdtypes.FromAssignedState(&state))
		if err != nil {
			return fmt.Errorf("marshal assigned shards for executor %s: %w", executorID, err)
		}

		compressedValue, err := s.recordWriter.Write(value)
		if err != nil {
			return fmt.Errorf("compress assigned shards for executor %s: %w", executorID, err)
		}
		ops = append(ops, clientv3.OpPut(executorStateKey, string(compressedValue)))

		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(executorStateKey), "=", state.ModRevision))
		comparisonMaps[executorStateKey] = state.ModRevision
		opsElse = append(opsElse, clientv3.OpGet(executorStateKey))
	}

	if len(ops) == 0 {
		return nil
	}

	// 3. Apply the guard function to get the base transaction, which may already have an 'If' condition for leadership.
	nativeTxn := s.client.Txn(ctx)
	guardedTxn, err := guard(nativeTxn)
	if err != nil {
		return fmt.Errorf("apply transaction guard: %w", err)
	}
	etcdGuardedTxn, ok := guardedTxn.(clientv3.Txn)
	if !ok {
		return fmt.Errorf("guard function returned invalid transaction type")
	}

	// 4. Create a nested transaction operation. This allows us to add our own 'If' (comparisons)
	// and 'Then' (ops) logic that will only execute if the outer guard's 'If' condition passes.
	// we catch what is the state in the else operations so we can identify which part of the condition failed
	nestedTxnOp := clientv3.OpTxn(
		comparisons, // Our IF conditions
		ops,         // Our THEN operations
		opsElse,     // Our ELSE operations
	)

	// 5. Add the nested transaction to the guarded transaction's THEN clause and commit.
	etcdGuardedTxn = etcdGuardedTxn.Then(nestedTxnOp)
	txnResp, err := etcdGuardedTxn.Commit()
	if err != nil {
		return fmt.Errorf("commit shard assignments transaction: %w", err)
	}

	// 6. Check the results of both the outer and nested transactions.
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
		failingRevisionString := ""
		for _, keyValue := range nestedResp.Responses[0].GetResponseRange().Kvs {
			expectedValue, ok := comparisonMaps[string(keyValue.Key)]
			if !ok || expectedValue != keyValue.ModRevision {
				failingRevisionString = failingRevisionString + fmt.Sprintf("{ key: %s, expected:%v, actual: %v }", string(keyValue.Key), expectedValue, keyValue.ModRevision)
			}
		}
		return fmt.Errorf("%w: transaction failed, a shard may have been concurrently assigned, %v", store.ErrVersionConflict, failingRevisionString)
	}

	// Apply shard statistics updates outside the main transaction to stay within etcd's max operations per txn.
	s.applyShardStatisticsUpdates(ctx, namespace, statsUpdates)

	return nil
}

func (s *executorStoreImpl) AssignShard(ctx context.Context, namespace, shardID, executorID string) error {
	assignedState := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorAssignedStateKey)
	statusKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorStatusKey)
	executorStatsKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)

	// Use a read-modify-write loop to handle concurrent updates safely.
	for {
		// 1. Get the current assigned state of the executor and prepare the shard statistics.
		resp, err := s.client.Get(ctx, assignedState)
		if err != nil {
			return fmt.Errorf("get executor assigned state: %w", err)
		}

		var state etcdtypes.AssignedState
		var shardStats etcdtypes.ShardStatistics
		modRevision := int64(0) // A revision of 0 means the key doesn't exist yet.

		if len(resp.Kvs) > 0 {
			// If the executor already has shards, load its state.
			kv := resp.Kvs[0]
			modRevision = kv.ModRevision
			if err := common.DecompressAndUnmarshal(kv.Value, &state); err != nil {
				return fmt.Errorf("parse assigned state: %w", err)
			}
		} else {
			// If this is the first shard, initialize the state map.
			state.AssignedShards = make(map[string]*types.ShardAssignment)
		}

		statsResp, err := s.client.Get(ctx, executorStatsKey)
		if err != nil {
			return fmt.Errorf("get shard statistics: %w", err)
		}

		now := s.timeSource.Now().UTC()
		executorShardStats := make(map[string]etcdtypes.ShardStatistics)
		if len(statsResp.Kvs) > 0 {
			if err := common.DecompressAndUnmarshal(statsResp.Kvs[0].Value, &executorShardStats); err != nil {
				return fmt.Errorf("parse shard statistics: %w", err)
			}
		}

		shardStats, ok := executorShardStats[shardID]
		if !ok {
			shardStats.SmoothedLoad = 0
			shardStats.LastUpdateTime = etcdtypes.Time(now)
			// Leave LastMoveTime for newly added shards as zero, to not block it from being moved
			// once we have load measurements.
			shardStats.LastMoveTime = etcdtypes.Time(time.Time{})
		} else {
			shardStats.LastMoveTime = etcdtypes.Time(now)
		}
		executorShardStats[shardID] = shardStats

		// 2. Get the executor state.
		statusResp, err := s.client.Get(ctx, statusKey)
		if err != nil {
			return fmt.Errorf("get executor status: %w", err)
		}
		if len(statusResp.Kvs) == 0 {
			return store.ErrExecutorNotFound
		}
		statusValue := string(statusResp.Kvs[0].Value)
		decompressedStatusValue, err := common.Decompress(statusResp.Kvs[0].Value)
		if err != nil {
			return fmt.Errorf("decompress executor status: %w", err)
		}

		if string(decompressedStatusValue) != _executorStatusRunningJSON {
			return fmt.Errorf("%w: executor status is %s", store.ErrVersionConflict, statusValue)
		}
		statusModRev := statusResp.Kvs[0].ModRevision

		// 3. Modify the state in memory, adding the new shard if it's not already there.
		if _, alreadyAssigned := state.AssignedShards[shardID]; !alreadyAssigned {
			state.AssignedShards[shardID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		}

		// Compress new state value
		newStateValue, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal new assigned state: %w", err)
		}
		compressedStateValue, err := s.recordWriter.Write(newStateValue)
		if err != nil {
			return fmt.Errorf("compress new assigned state: %w", err)
		}

		newStatsValue, err := json.Marshal(executorShardStats)
		if err != nil {
			return fmt.Errorf("marshal new shard statistics: %w", err)
		}
		compressedStatsValue, err := s.recordWriter.Write(newStatsValue)
		if err != nil {
			return fmt.Errorf("compress new shard statistics: %w", err)
		}

		var comparisons []clientv3.Cmp

		// 4. Prepare and commit the transaction with four atomic checks.
		// a) Check that the executor's status ACTIVE has not been changed.
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(statusKey), "=", statusModRev))
		// b) Check that the assigned_state has not been modified concurrently.
		comparisons = append(comparisons, clientv3.Compare(clientv3.ModRevision(assignedState), "=", modRevision))
		// c) Check that the cache is up to date.
		cmp, err := s.shardCache.GetExecutorModRevisionCmp(namespace)
		if err != nil {
			return fmt.Errorf("get executor mod revision cmp: %w", err)
		}
		comparisons = append(comparisons, cmp...)

		// We check the shard cache to see if the shard is already assigned to an executor.
		shardOwner, err := s.shardCache.GetShardOwner(ctx, namespace, shardID)
		if err != nil && !errors.Is(err, store.ErrShardNotFound) {
			return fmt.Errorf("checking shard owner: %w", err)
		}
		if err == nil {
			return &store.ErrShardAlreadyAssigned{ShardID: shardID, AssignedTo: shardOwner.ExecutorID}
		}

		txnResp, err := s.client.Txn(ctx).
			If(comparisons...).
			Then(
				clientv3.OpPut(assignedState, string(compressedStateValue)),
				clientv3.OpPut(executorStatsKey, string(compressedStatsValue)),
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
		decompressedStatus, err := common.Decompress(currentStatusResp.Kvs[0].Value)
		if err != nil {
			return fmt.Errorf("decompress executor status %w", err)
		}
		if string(decompressedStatus) != _executorStatusRunningJSON {
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
		executorIDPrefix := etcdkeys.BuildExecutorIDPrefix(s.prefix, namespace, executorID)
		ops = append(ops, clientv3.OpDelete(executorIDPrefix, clientv3.WithPrefix()))
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

// DeleteShardStats deletes shard statistics for the given shard IDs.
// If the operation fails (e.g. due to leadership loss), it returns immediately.
// Partial deletions are acceptable as the periodic cleanup loop will retry remaining keys.
func (s *executorStoreImpl) DeleteShardStats(ctx context.Context, namespace string, shardIDs []string, guard store.GuardFunc) error {
	if len(shardIDs) == 0 {
		return nil
	}

	// Build a lookup for shard IDs to delete.
	toDelete := make(map[string]struct{}, len(shardIDs))
	for _, shardID := range shardIDs {
		toDelete[shardID] = struct{}{}
	}

	// Fetch all statistics at the executor level for this namespace.
	executorPrefix := etcdkeys.BuildExecutorsPrefix(s.prefix, namespace)
	resp, err := s.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("get executor data for shard stats deletion: %w", err)
	}

	ops := make([]clientv3.Op, 0)

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		executorID, keyType, keyErr := etcdkeys.ParseExecutorKey(s.prefix, namespace, key)
		if keyErr != nil || keyType != etcdkeys.ExecutorShardStatisticsKey {
			continue
		}

		executorStats := make(map[string]etcdtypes.ShardStatistics)
		if err := common.DecompressAndUnmarshal(kv.Value, &executorStats); err != nil {
			s.logger.Warn(
				"failed to parse executor shard statistics during cleanup",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(executorID),
				tag.Error(err),
			)
			continue
		}

		changed := false
		for shardID := range executorStats {
			if _, ok := toDelete[shardID]; ok {
				delete(executorStats, shardID)
				changed = true
			}
		}

		if !changed {
			continue
		}

		statsKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
		if len(executorStats) == 0 {
			ops = append(ops, clientv3.OpDelete(statsKey))
			continue
		}

		payload, err := json.Marshal(executorStats)
		if err != nil {
			s.logger.Warn(
				"failed to marshal executor shard statistics during cleanup",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(executorID),
				tag.Error(err),
			)
			continue
		}

		compressedPayload, err := s.recordWriter.Write(payload)
		if err != nil {
			s.logger.Warn(
				"failed to compress executor shard statistics during cleanup",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(executorID),
				tag.Error(err),
			)
			continue
		}

		ops = append(ops, clientv3.OpPut(statsKey, string(compressedPayload)))
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
	txnResp, err := etcdGuardedTxn.Commit()
	if err != nil {
		return fmt.Errorf("commit shard statistics deletion: %w", err)
	}
	if !txnResp.Succeeded {
		return fmt.Errorf("transaction failed, leadership may have changed")
	}

	return nil
}

func (s *executorStoreImpl) GetShardOwner(ctx context.Context, namespace, shardID string) (*store.ShardOwner, error) {
	return s.shardCache.GetShardOwner(ctx, namespace, shardID)
}

func (s *executorStoreImpl) GetExecutor(ctx context.Context, namespace string, executorID string) (*store.ShardOwner, error) {
	return s.shardCache.GetExecutor(ctx, namespace, executorID)
}

// This function calculates the necessary changes to shard statistics based on a new shard assignment plan.
// It determines which shards have moved between executors, which are new. It then prepares a list of
// update operations that will remove a moved shard's stats from its old owner and add them to its new owner, recording the time of the move.
func (s *executorStoreImpl) prepareShardStatisticsUpdates(ctx context.Context, namespace string, newAssignments map[string]store.AssignedState) ([]shardStatisticsUpdate, error) {
	// This map will store the *new, final* state of statistics for any executor whose stats have changed.
	pendingStatChanges := make(map[string]map[string]etcdtypes.ShardStatistics)

	for executorID, state := range newAssignments {
		for shardID := range state.AssignedShards {
			now := s.timeSource.Now().UTC()

			oldOwner, err := s.shardCache.GetShardOwner(ctx, namespace, shardID)
			if err != nil && !errors.Is(err, store.ErrShardNotFound) {
				return nil, fmt.Errorf("lookup cached shard owner: %w", err)
			}

			var shardStatToMove etcdtypes.ShardStatistics

			if err == nil {
				if oldOwner.ExecutorID == executorID {
					continue
				}

				oldOwnerStats, ok := pendingStatChanges[oldOwner.ExecutorID]
				if !ok { // Not yet touched in this loop, get from main cache.
					oldOwnerStats, err = s.shardCache.GetExecutorStatistics(ctx, namespace, oldOwner.ExecutorID)
					if err != nil {
						return nil, err
					}
				}

				mutableOldOwnerStats := make(map[string]etcdtypes.ShardStatistics, len(oldOwnerStats))
				for k, v := range oldOwnerStats {
					mutableOldOwnerStats[k] = v
				}

				if existing, ok := mutableOldOwnerStats[shardID]; ok {
					shardStatToMove = existing
					delete(mutableOldOwnerStats, shardID)
				}
				pendingStatChanges[oldOwner.ExecutorID] = mutableOldOwnerStats
			}

			// If the shard is new or had no previous stats, initialize them.
			if shardStatToMove.LastUpdateTime == etcdtypes.Time(time.Time{}) {
				shardStatToMove.SmoothedLoad = 0
				shardStatToMove.LastUpdateTime = etcdtypes.Time(now)
				// Leave LastMoveTime for newly added shards as zero, to not block it from being moved once we have load measurements
				shardStatToMove.LastMoveTime = etcdtypes.Time(time.Time{})
			} else {
				shardStatToMove.LastMoveTime = etcdtypes.Time(now)
			}

			newOwnerStats, ok := pendingStatChanges[executorID]
			if !ok {
				newOwnerStats, err = s.shardCache.GetExecutorStatistics(ctx, namespace, executorID)
				if err != nil {
					return nil, err
				}
			}

			mutableNewOwnerStats := make(map[string]etcdtypes.ShardStatistics, len(newOwnerStats))
			for k, v := range newOwnerStats {
				mutableNewOwnerStats[k] = v
			}

			mutableNewOwnerStats[shardID] = shardStatToMove
			pendingStatChanges[executorID] = mutableNewOwnerStats
		}
	}

	updates := make([]shardStatisticsUpdate, 0, len(pendingStatChanges))
	for executorID, stats := range pendingStatChanges {
		updates = append(updates, shardStatisticsUpdate{
			executorID: executorID,
			stats:      stats,
		})
	}

	return updates, nil
}

// applyShardStatisticsUpdates updates shard statistics.
// Is intentionally made tolerant of failures since the data is telemetry only.
func (s *executorStoreImpl) applyShardStatisticsUpdates(ctx context.Context, namespace string, updates []shardStatisticsUpdate) {
	for _, update := range updates {
		statsKey := etcdkeys.BuildExecutorKey(s.prefix, namespace, update.executorID, etcdkeys.ExecutorShardStatisticsKey)

		if len(update.stats) == 0 {
			if _, err := s.client.Delete(ctx, statsKey); err != nil {
				s.logger.Warn(
					"failed to delete executor shard statistics",
					tag.ShardNamespace(namespace),
					tag.ShardExecutor(update.executorID),
					tag.Error(err),
				)
			}
			continue
		}

		payload, err := json.Marshal(update.stats)
		if err != nil {
			s.logger.Warn(
				"failed to marshal shard statistics after assignment",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(update.executorID),
				tag.Error(err),
			)
			continue
		}

		compressedPayload, err := s.recordWriter.Write(payload)
		if err != nil {
			s.logger.Warn(
				"failed to compress shard statistics after assignment",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(update.executorID),
				tag.Error(err),
			)
			continue
		}

		if _, err := s.client.Put(ctx, statsKey, string(compressedPayload)); err != nil {
			s.logger.Warn(
				"failed to update shard statistics",
				tag.ShardNamespace(namespace),
				tag.ShardExecutor(update.executorID),
				tag.Error(err),
			)
		}
	}
}

func ewmaSmoothedLoad(prev, current float64, lastUpdate, now time.Time) float64 {
	const tau = 30 * time.Second // smaller = more responsive, larger = smoother
	if lastUpdate.IsZero() || tau <= 0 {
		return current
	}
	if now.Before(lastUpdate) {
		return current
	}
	dt := now.Sub(lastUpdate)
	alpha := 1 - math.Exp(-dt.Seconds()/tau.Seconds())
	return (1-alpha)*prev + alpha*current
}
