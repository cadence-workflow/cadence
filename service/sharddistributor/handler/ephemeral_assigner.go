// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/handler/loadbalance"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// assignEphemeralBatch is the ephemeralAssignmentBatchFn wired into the shardBatcher.
// It processes a whole batch of unassigned shard keys for a single ephemeral
// namespace using two storage operations:
//  1. GetState     — read current namespace state once for the whole batch.
//  2. AssignShards — write all new assignments atomically in one operation.
//
// After the write, GetExecutor is called once per unique chosen executor (not
// per shard) to fetch metadata for the response, since metadata is stored
// separately in the shard cache and is not returned by GetState.
//
// Within the batch, each shard is assigned to an ACTIVE executor according to
// the configured load balancing mode. The balancer updates its in-batch load
// state after every pick so later picks account for earlier picks.
func (h *handlerImpl) assignEphemeralBatch(ctx context.Context, namespace string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error) {
	state, err := h.storage.GetState(ctx, namespace)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("get namespace state: %v", err)}
	}

	balancer, err := loadbalance.New(h.cfg.GetLoadBalancingMode(namespace), state)
	if err != nil {
		return nil, &types.InternalServiceError{Message: err.Error()}
	}

	chosenExecutors, err := pickExecutors(namespace, balancer, shardKeys)
	if err != nil {
		return nil, err
	}

	mergeAssignments(state, chosenExecutors)

	if err := h.storage.AssignShards(ctx, namespace, store.AssignShardsRequest{NewState: state}, store.NopGuard()); err != nil {
		if errors.Is(err, store.ErrVersionConflict) {
			// Return the version-conflict sentinel unwrapped so callers can
			// detect it with errors.Is and decide whether to retry.
			return nil, fmt.Errorf("assign ephemeral shards: %w", err)
		}
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("assign ephemeral shards: %v", err)}
	}

	executorOwners, err := h.fetchExecutorMetadata(ctx, namespace, chosenExecutors)
	if err != nil {
		return nil, err
	}

	return buildResults(namespace, shardKeys, chosenExecutors, executorOwners), nil
}

// pickExecutors asks the balancer to choose an executor for each shard key.
// Returns a map of shardKey -> chosen executorID.
func pickExecutors(namespace string, balancer loadbalance.Balancer, shardKeys []string) (map[string]string, error) {
	chosen := make(map[string]string, len(shardKeys))
	for _, shardKey := range shardKeys {
		executor, err := balancer.Pick()
		if err != nil {
			if errors.Is(err, loadbalance.ErrNoActiveExecutors) {
				return nil, &types.InternalServiceError{Message: "no active executors available for namespace: " + namespace}
			}
			return nil, &types.InternalServiceError{Message: fmt.Sprintf("pick executor: %v", err)}
		}
		chosen[shardKey] = executor
	}
	return chosen, nil
}

// mergeAssignments folds the chosen shard→executor assignments back into state.
// The AssignedShards maps are copied to avoid mutating the object returned by
// GetState.
func mergeAssignments(state *store.NamespaceState, chosenExecutors map[string]string) {
	for executorID, shardsForExecutor := range invertMap(chosenExecutors) {
		existing := state.ShardAssignments[executorID]
		newShards := make(map[string]*types.ShardAssignment, len(existing.AssignedShards)+len(shardsForExecutor))
		for k, v := range existing.AssignedShards {
			newShards[k] = v
		}
		for _, shardKey := range shardsForExecutor {
			newShards[shardKey] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		}
		existing.AssignedShards = newShards
		state.ShardAssignments[executorID] = existing
	}
}

// fetchExecutorMetadata calls GetExecutor once per unique chosen executor to
// retrieve metadata. Metadata is stored separately from HeartbeatState and is
// not returned by GetState.
func (h *handlerImpl) fetchExecutorMetadata(ctx context.Context, namespace string, chosenExecutors map[string]string) (map[string]*store.ShardOwner, error) {
	executorOwners := make(map[string]*store.ShardOwner, len(chosenExecutors))
	for _, executorID := range chosenExecutors {
		if _, already := executorOwners[executorID]; already {
			continue
		}
		owner, err := h.storage.GetExecutor(ctx, namespace, executorID)
		if err != nil {
			return nil, &types.InternalServiceError{Message: fmt.Sprintf("get executor %q: %v", executorID, err)}
		}
		executorOwners[executorID] = owner
	}
	return executorOwners, nil
}

// buildResults constructs the shardKey -> GetShardOwnerResponse map from the
// chosen executors and their fetched metadata.
func buildResults(namespace string, shardKeys []string, chosenExecutors map[string]string, executorOwners map[string]*store.ShardOwner) map[string]*types.GetShardOwnerResponse {
	results := make(map[string]*types.GetShardOwnerResponse, len(shardKeys))
	for _, shardKey := range shardKeys {
		executorID := chosenExecutors[shardKey]
		owner := executorOwners[executorID]
		results[shardKey] = &types.GetShardOwnerResponse{
			Owner:     owner.ExecutorID,
			Namespace: namespace,
			Metadata:  owner.Metadata,
		}
	}
	return results
}

// invertMap turns map[shardKey]executorID into map[executorID][]shardKey.
func invertMap(m map[string]string) map[string][]string {
	out := make(map[string][]string)
	for shardKey, executorID := range m {
		out[executorID] = append(out[executorID], shardKey)
	}
	return out
}
