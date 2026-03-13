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
	"math"

	"github.com/uber/cadence/common/types"
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
// Within the batch the least-loaded ACTIVE executor is chosen per shard, with
// the in-batch running count updated after each pick so load is spread evenly.
func (h *handlerImpl) assignEphemeralBatch(ctx context.Context, namespace string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error) {
	state, err := h.storage.GetState(ctx, namespace)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("get namespace state: %v", err)}
	}

	assignedCounts, err := buildAssignedCounts(state)
	if err != nil {
		return nil, err
	}

	chosenExecutors, err := pickExecutors(namespace, shardKeys, assignedCounts)
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

// buildAssignedCounts returns a map of executorID -> current shard count for
// all ACTIVE executors in the given namespace state.
func buildAssignedCounts(state *store.NamespaceState) (map[string]int, error) {
	counts := make(map[string]int, len(state.ShardAssignments))
	for executorID, assignment := range state.ShardAssignments {
		executorState, ok := state.Executors[executorID]
		if !ok || executorState.Status != types.ExecutorStatusACTIVE {
			continue
		}
		counts[executorID] = len(assignment.AssignedShards)
	}
	return counts, nil
}

// pickExecutors assigns each shard key to the least-loaded active executor,
// updating the in-batch running count after every pick to spread load evenly.
// It returns a map of shardKey -> chosen executorID.
func pickExecutors(namespace string, shardKeys []string, assignedCounts map[string]int) (map[string]string, error) {
	chosenExecutors := make(map[string]string, len(shardKeys))
	for _, shardKey := range shardKeys {
		chosenExecutor := ""
		minCount := math.MaxInt
		for executorID, count := range assignedCounts {
			if count < minCount {
				minCount = count
				chosenExecutor = executorID
			}
		}
		if chosenExecutor == "" {
			return nil, &types.InternalServiceError{Message: "no active executors available for namespace: " + namespace}
		}
		chosenExecutors[shardKey] = chosenExecutor
		assignedCounts[chosenExecutor]++
	}
	return chosenExecutors, nil
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
