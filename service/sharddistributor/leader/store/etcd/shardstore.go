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

func (s *shardStore) GetState(ctx context.Context) (map[string]store.HeartbeatState, map[string]store.AssignedState, int64, error) {
	client := s.session.Client()

	heartbeatStates := make(map[string]store.HeartbeatState)
	assignedStates := make(map[string]store.AssignedState)

	// Get all executor data for this namespace
	executorPrefix := s.buildExecutorPrefix()
	resp, err := client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get executor data from etcd: %w", err)
	}

	// Process all keys and values
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)

		executorID, keyType, keyErr := s.parseExecutorKey(key)
		if keyErr != nil {
			continue // Skip invalid keys
		}

		// Ensure heartbeat and assigned states exist for the executor
		if _, ok := heartbeatStates[executorID]; !ok {
			heartbeatStates[executorID] = store.HeartbeatState{ExecutorID: executorID}
		}
		if _, ok := assignedStates[executorID]; !ok {
			assignedStates[executorID] = store.AssignedState{
				ExecutorID:     executorID,
				ReportedShards: make(map[string]store.ShardState),
				AssignedShards: make(map[string]store.ShardAssignment),
			}
		}

		heartbeat := heartbeatStates[executorID]
		assigned := assignedStates[executorID]

		switch keyType {
		case "heartbeat":
			timestamp, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				timestamp = 0
			}
			heartbeat.LastHeartbeat = timestamp
		case "state":
			heartbeat.State = store.ExecutorState(value)
		case "reported_shards":
			if err := json.Unmarshal(kv.Value, &assigned.ReportedShards); err != nil {
				return nil, nil, 0, fmt.Errorf("failed to unmarshal reported shards for executor %s: %w", executorID, err)
			}
		case "assigned_shards":
			if err := json.Unmarshal(kv.Value, &assigned.AssignedShards); err != nil {
				return nil, nil, 0, fmt.Errorf("failed to unmarshal assigned shards for executor %s: %w", executorID, err)
			}
		}
		heartbeatStates[executorID] = heartbeat
		assignedStates[executorID] = assigned
	}

	return heartbeatStates, assignedStates, resp.Header.Revision, nil
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

// Subscribe creates a watch on the executor prefix and returns a channel that
// receives the revision number of any change.
func (s *shardStore) Subscribe(ctx context.Context) (<-chan int64, error) {
	client := s.session.Client()
	// Use a buffered channel of size 1.
	revisionChan := make(chan int64, 1)

	watchPrefix := s.buildExecutorPrefix()

	go func() {
		defer close(revisionChan)

		watchChan := client.Watch(ctx, watchPrefix, clientv3.WithPrefix())

		for watchResp := range watchChan {
			if err := watchResp.Err(); err != nil {
				return
			}

			isSignificantChange := false
			for _, event := range watchResp.Events {
				// 1. A deletion is always a significant event (e.g., executor removed).
				if !event.IsCreate() && !event.IsModify() {
					isSignificantChange = true
					break
				}

				// 2. Parse the key to determine what was changed.
				_, keyType, err := s.parseExecutorKey(string(event.Kv.Key))
				if err != nil {
					// If the key can't be parsed, assume it's an unrelated key
					// and skip it without causing a notification.
					continue
				}

				// 3. A change to any key other than "heartbeat" or "assigned_shards" is significant.
				if keyType != "heartbeat" && keyType != "assigned_shards" {
					isSignificantChange = true
					break // Found a significant change, no need to check other events.
				}
			}

			if isSignificantChange {
				// 1. Non-blocking drain: If there's an old revision in the channel, discard it.
				// This ensures we are always working with an empty buffer slot.
				select {
				case <-revisionChan:
				default:
				}

				// 2. Non-blocking send: Send the latest revision. Because we just drained,
				// this will always succeed without blocking.
				revisionChan <- watchResp.Header.Revision
			}
		}
	}()

	return revisionChan, nil
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
func (s *shardStore) parseExecutorKey(key string) (executorID, keyType string, err error) {
	prefix := s.buildExecutorPrefix()
	if !strings.HasPrefix(key, prefix) {
		return "", "", fmt.Errorf("unexpected key: %s", key)
	}

	// Remove prefix: {namespace-prefix}/executors/
	remainder := strings.TrimPrefix(key, prefix)

	// Split by / to get [executorID, keyType]
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected key: %s", key)
	}

	return parts[0], parts[1], nil
}

// buildExecutorKey constructs an etcd key for an executor
func (s *shardStore) buildExecutorKey(executorID, keyType string) string {
	return fmt.Sprintf("%s%s/%s", s.buildExecutorPrefix(), executorID, keyType)
}
