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
	"slices"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type executorAssignmentLoad struct {
	smoothedLoad float64
	shardCount   int
}

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
// the configured load balancing mode. The in-batch load state is updated after
// each pick so later picks account for earlier picks.

func (h *handlerImpl) assignEphemeralBatch(ctx context.Context, namespace string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error) {
	state, err := h.storage.GetState(ctx, namespace)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("get namespace state: %v", err)}
	}

	loadBalancingMode := h.cfg.GetLoadBalancingMode(namespace)

	executorLoads, averageShardLoad, err := h.computeInitialPlacementLoads(loadBalancingMode, state)
	if err != nil {
		return nil, err
	}

	chosenExecutors, err := pickExecutors(
		namespace,
		shardKeys,
		executorLoads,
		loadBalancingMode,
		averageShardLoad,
	)
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

// computeInitialPlacementLoads returns current shard count for all ACTIVE executors.
// In GREEDY mode it also includes smoothed shard and an average shard load
func (h *handlerImpl) computeInitialPlacementLoads(mode types.LoadBalancingMode, state *store.NamespaceState) (map[string]executorAssignmentLoad, float64, error) {
	var useSmoothedLoad bool
	switch mode {
	case types.LoadBalancingModeGREEDY:
		useSmoothedLoad = true
	case types.LoadBalancingModeNAIVE:
		useSmoothedLoad = false
	default:
		return nil, 0, &types.InternalServiceError{Message: fmt.Sprintf("unsupported load balancing mode: %s", mode)}
	}

	executorsAssignmentLoad := make(map[string]executorAssignmentLoad, len(state.Executors))
	totalSmoothedLoad := 0.0
	totalShardCount := 0

	for executorID, executorState := range state.Executors {
		if executorState.Status != types.ExecutorStatusACTIVE {
			continue
		}

		assignment := state.ShardAssignments[executorID]
		executorLoad := executorAssignmentLoad{shardCount: len(assignment.AssignedShards)}
		totalShardCount += executorLoad.shardCount
		if useSmoothedLoad {
			for shardID := range assignment.AssignedShards {
				stats, ok := state.ShardStats[shardID]
				if !ok {
					continue
				}
				executorLoad.smoothedLoad += stats.SmoothedLoad
				totalSmoothedLoad += stats.SmoothedLoad
			}
		}
		executorsAssignmentLoad[executorID] = executorLoad
	}
	if !useSmoothedLoad || totalShardCount == 0 {
		return executorsAssignmentLoad, 0, nil
	}
	return executorsAssignmentLoad, totalSmoothedLoad / float64(totalShardCount), nil
}

// pickExecutors assigns each shard key to an active executor,
// updating the in-batch running count after every pick to spread load evenly when loads tie.
// It returns a map of shardKey -> chosen executorID.
func pickExecutors(
	namespace string,
	shardKeys []string,
	assignmentLoads map[string]executorAssignmentLoad,
	mode types.LoadBalancingMode,
	averageShardLoad float64,
) (map[string]string, error) {
	executorIDs := make([]string, 0, len(assignmentLoads))
	for executorID := range assignmentLoads {
		executorIDs = append(executorIDs, executorID)
	}
	slices.Sort(executorIDs)

	var pickExecutor func([]string, map[string]executorAssignmentLoad) string
	switch mode {
	case types.LoadBalancingModeNAIVE:
		pickExecutor = pickExecutorByShardCount
	case types.LoadBalancingModeGREEDY:
		pickExecutor = pickExecutorBySmoothedLoad
	default:
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("unsupported load balancing mode: %s", mode)}
	}

	chosenExecutors := make(map[string]string, len(shardKeys))
	for _, shardKey := range shardKeys {
		chosenExecutor := pickExecutor(executorIDs, assignmentLoads)
		if chosenExecutor == "" {
			return nil, &types.InternalServiceError{Message: "no active executors available for namespace: " + namespace}
		}
		chosenExecutors[shardKey] = chosenExecutor
		load := assignmentLoads[chosenExecutor]
		load.shardCount++
		if mode == types.LoadBalancingModeGREEDY {
			// We increase the total load by the average shard load in the namespace
			// This helps avoid placing all shards in the batch on the same executor
			load.smoothedLoad += averageShardLoad
		}
		assignmentLoads[chosenExecutor] = load
	}
	return chosenExecutors, nil
}

// pickExecutorBySmoothedLoad returns the executor with the lowest smoothed load.
// Ties are broken by shard count, then by sorted executorID order.
func pickExecutorBySmoothedLoad(executorIDs []string, assignmentLoads map[string]executorAssignmentLoad) string {
	chosenExecutor := ""
	minLoad := math.MaxFloat64
	minCount := math.MaxInt
	for _, executorID := range executorIDs {
		load := assignmentLoads[executorID]
		if load.smoothedLoad < minLoad || (load.smoothedLoad == minLoad && load.shardCount < minCount) {
			minLoad = load.smoothedLoad
			minCount = load.shardCount
			chosenExecutor = executorID
		}
	}
	return chosenExecutor
}

// pickExecutorByShardCount returns the executor with the fewest assigned shards.
// Ties are broken by sorted executorID order.
func pickExecutorByShardCount(executorIDs []string, assignmentLoads map[string]executorAssignmentLoad) string {
	chosenExecutor := ""
	minCount := math.MaxInt
	for _, executorID := range executorIDs {
		load := assignmentLoads[executorID]
		if load.shardCount < minCount {
			minCount = load.shardCount
			chosenExecutor = executorID
		}
	}
	return chosenExecutor
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
