package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/uber/cadence/service/sharddistributor/leader/store"
)

type shardStore struct {
	session   *concurrency.Session
	prefix    string
	leaderKey string
	leaderRev int64
}

func (s *shardStore) GetState(ctx context.Context) (map[string]store.HeartbeatState, map[string]store.AssignedState, error) {
	client := s.session.Client()

	heartbeatStates := make(map[string]store.HeartbeatState)
	assignedStates := make(map[string]store.AssignedState)

	// Get all executor data for this namespace
	executorPrefix := s.buildExecutorPrefix()
	resp, err := client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get executor data from etcd: %w", err)
	}

	// Process all keys and values
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)

		executorID, keyType := s.parseExecutorKey(key)
		if executorID == "" {
			continue // Skip invalid keys
		}

		switch keyType {
		case "heartbeat":
			timestamp, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				// If parse fails, use current time
				timestamp = time.Now().Unix()
			}

			// Get or update existing heartbeat state
			heartbeat, exists := heartbeatStates[executorID]
			if !exists {
				heartbeat = store.HeartbeatState{ExecutorID: executorID}
			}
			heartbeat.LastHeartbeat = timestamp
			heartbeatStates[executorID] = heartbeat

		case "state":
			state := store.ExecutorState(value)

			// Get or update existing heartbeat state
			heartbeat, exists := heartbeatStates[executorID]
			if !exists {
				heartbeat = store.HeartbeatState{ExecutorID: executorID}
			}
			heartbeat.State = state
			heartbeatStates[executorID] = heartbeat

		case "reported_shards":
			// Get or create assigned state entry
			assigned, exists := assignedStates[executorID]
			if !exists {
				assigned = store.AssignedState{
					ExecutorID:     executorID,
					ReportedShards: make(map[string]store.ShardState),
					AssignedShards: make(map[string]store.ShardAssignment),
				}
			}

			// Parse reported shards map
			var reportedShards map[string]store.ShardState
			if err := json.Unmarshal(kv.Value, &reportedShards); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal reported shards for executor %s: %w", executorID, err)
			}
			assigned.ReportedShards = reportedShards
			assignedStates[executorID] = assigned

		case "assigned_shards":
			// Get or create assigned state entry
			assigned, exists := assignedStates[executorID]
			if !exists {
				assigned = store.AssignedState{
					ExecutorID:     executorID,
					ReportedShards: make(map[string]store.ShardState),
					AssignedShards: make(map[string]store.ShardAssignment),
				}
			}

			// Parse assigned shards map
			var assignedShards map[string]store.ShardAssignment
			if err := json.Unmarshal(kv.Value, &assignedShards); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal assigned shards for executor %s: %w", executorID, err)
			}
			assigned.AssignedShards = assignedShards
			assignedStates[executorID] = assigned
		}
	}

	return heartbeatStates, assignedStates, nil
}

func (s *shardStore) AssignShards(ctx context.Context, newState map[string]store.AssignedState) error {
	client := s.session.Client()

	// Build the operations for shard assignments
	var ops []clientv3.Op

	for executorID, state := range newState {
		key := s.buildExecutorKey(executorID, "assigned_shards")

		// Update the timestamp
		state.LastUpdated = time.Now().Unix()

		// Serialize the assigned shards map
		value, err := json.Marshal(state.AssignedShards)
		if err != nil {
			return fmt.Errorf("failed to marshal assigned shards for executor %s: %w", executorID, err)
		}

		ops = append(ops, clientv3.OpPut(key, string(value)))
	}

	// Execute with leader key revision check to ensure we're still the leader
	if len(ops) > 0 {
		// Create transaction with condition that leader key revision hasn't changed
		txn := client.Txn(ctx).
			If(clientv3.Compare(clientv3.ModRevision(s.leaderKey), "=", s.leaderRev)).
			Then(ops...)

		txnResp, err := txn.Commit()
		if err != nil {
			return fmt.Errorf("failed to commit shard assignments: %w", err)
		}

		if !txnResp.Succeeded {
			return fmt.Errorf("transaction failed: leadership may have changed (leader key revision mismatch)")
		}
	}

	return nil
}

// buildExecutorPrefix returns the etcd prefix for all executors in this namespace
func (s *shardStore) buildExecutorPrefix() string {
	return fmt.Sprintf("%s/executors/", s.prefix)
}

// parseExecutorKey extracts executor ID and key type from etcd key
// Examples:
//
//	/prefix/namespaces/my-ns/executors/executor-1/heartbeat -> ("executor-1", "heartbeat")
//	/prefix/namespaces/my-ns/executors/executor-2/state -> ("executor-2", "state")
func (s *shardStore) parseExecutorKey(key string) (executorID, keyType string) {
	prefix := s.buildExecutorPrefix()
	if !strings.HasPrefix(key, prefix) {
		return "", ""
	}

	// Remove prefix: {namespace-prefix}/executors/
	remainder := strings.TrimPrefix(key, prefix)

	// Split by / to get [executorID, keyType]
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}

// buildExecutorKey constructs an etcd key for an executor
func (s *shardStore) buildExecutorKey(executorID, keyType string) string {
	return fmt.Sprintf("%s%s/%s", s.buildExecutorPrefix(), executorID, keyType)
}
