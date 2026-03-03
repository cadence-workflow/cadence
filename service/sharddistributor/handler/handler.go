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
	"sync"
	"time"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// ephemeralBatchInterval is the time window over which GetShardOwner calls for
// ephemeral namespaces are collected before being processed as a single batch.
const ephemeralBatchInterval = 100 * time.Millisecond

func NewHandler(
	logger log.Logger,
	shardDistributionCfg config.ShardDistribution,
	storage store.Store,
) Handler {
	handler := &handlerImpl{
		logger:               logger,
		shardDistributionCfg: shardDistributionCfg,
		storage:              storage,
	}

	handler.batcher = newShardBatcher(ephemeralBatchInterval, handler.assignEphemeralBatch)

	// prevent us from trying to serve requests before shard distributor is started and ready
	handler.startWG.Add(1)
	return handler
}

type handlerImpl struct {
	logger log.Logger

	startWG sync.WaitGroup

	storage              store.Store
	shardDistributionCfg config.ShardDistribution

	batcher *shardBatcher
}

func (h *handlerImpl) Start() {
	h.batcher.Start()
	h.startWG.Done()
}

func (h *handlerImpl) Stop() {
	h.batcher.Stop()
}

func (h *handlerImpl) Health(ctx context.Context) (*types.HealthStatus, error) {
	h.startWG.Wait()
	h.logger.Debug("Shard Distributor service health check endpoint reached.")
	hs := &types.HealthStatus{Ok: true, Msg: "shard distributor good"}
	return hs, nil
}

func (h *handlerImpl) GetShardOwner(ctx context.Context, request *types.GetShardOwnerRequest) (resp *types.GetShardOwnerResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	namespaceIdx := slices.IndexFunc(h.shardDistributionCfg.Namespaces, func(namespace config.Namespace) bool {
		return namespace.Name == request.Namespace
	})
	if namespaceIdx == -1 {
		return nil, &types.NamespaceNotFoundError{
			Namespace: request.Namespace,
		}
	}

	shardOwner, err := h.storage.GetShardOwner(ctx, request.Namespace, request.ShardKey)
	if errors.Is(err, store.ErrShardNotFound) {
		if h.shardDistributionCfg.Namespaces[namespaceIdx].Type == config.NamespaceTypeEphemeral {
			return h.batcher.Submit(ctx, request.Namespace, request.ShardKey)
		}

		return nil, &types.ShardNotFoundError{
			Namespace: request.Namespace,
			ShardKey:  request.ShardKey,
		}
	}
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to get shard owner: %v", err)}
	}

	resp = &types.GetShardOwnerResponse{
		Owner:     shardOwner.ExecutorID,
		Metadata:  shardOwner.Metadata,
		Namespace: request.Namespace,
	}

	return resp, nil
}

// assignEphemeralBatch is the ephemeralBatchFn wired into the shardBatcher.
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

	assignedCounts := make(map[string]int, len(state.ShardAssignments))
	for executorID, assignment := range state.ShardAssignments {
		executorState, ok := state.Executors[executorID]
		if !ok || executorState.Status != types.ExecutorStatusACTIVE {
			continue
		}
		assignedCounts[executorID] = len(assignment.AssignedShards)
	}

	// Assign each shard key to the least-loaded executor, tracking choices so
	// we can build the response map after the single AssignShards call.
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

	// Merge the new shard assignments into the namespace state. We copy the
	// AssignedShards maps to avoid mutating the object returned by GetState.
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

	// Single atomic write for the entire batch.
	if err := h.storage.AssignShards(ctx, namespace, store.AssignShardsRequest{NewState: state}, store.NopGuard()); err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("assign ephemeral shards: %v", err)}
	}

	// Fetch metadata from the shard cache once per unique chosen executor.
	// Metadata is stored separately from HeartbeatState and is not returned
	// by GetState, so GetExecutor is required here.
	executorOwners := make(map[string]*store.ShardOwner, len(assignedCounts))
	for _, executorID := range chosenExecutors {
		if _, already := executorOwners[executorID]; already {
			continue
		}
		owner, execErr := h.storage.GetExecutor(ctx, namespace, executorID)
		if execErr != nil {
			return nil, &types.InternalServiceError{Message: fmt.Sprintf("get executor %q: %v", executorID, execErr)}
		}
		executorOwners[executorID] = owner
	}

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

	return results, nil
}

// invertMap turns map[shardKey]executorID into map[executorID][]shardKey.
func invertMap(m map[string]string) map[string][]string {
	out := make(map[string][]string)
	for shardKey, executorID := range m {
		out[executorID] = append(out[executorID], shardKey)
	}
	return out
}

func (h *handlerImpl) WatchNamespaceState(request *types.WatchNamespaceStateRequest, server WatchNamespaceStateServer) error {
	h.startWG.Wait()

	// Subscribe to state changes from storage
	assignmentChangesChan, unSubscribe, err := h.storage.SubscribeToAssignmentChanges(server.Context(), request.Namespace)
	defer unSubscribe()
	if err != nil {
		return &types.InternalServiceError{Message: fmt.Sprintf("failed to subscribe to namespace state: %v", err)}
	}

	// Stream subsequent updates
	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		case assignmentChanges, ok := <-assignmentChangesChan:
			if !ok {
				return fmt.Errorf("unexpected close of updates channel")
			}
			response := &types.WatchNamespaceStateResponse{
				Executors: make([]*types.ExecutorShardAssignment, 0, len(assignmentChanges)),
			}
			for executor, shardIDs := range assignmentChanges {
				response.Executors = append(response.Executors, &types.ExecutorShardAssignment{
					ExecutorID:     executor.ExecutorID,
					AssignedShards: WrapShards(shardIDs),
					Metadata:       executor.Metadata,
				})
			}

			err = server.Send(response)
			if err != nil {
				return fmt.Errorf("send response: %w", err)
			}
		}
	}
}

func WrapShards(shardIDs []string) []*types.Shard {
	shards := make([]*types.Shard, 0, len(shardIDs))
	for _, shardID := range shardIDs {
		shards = append(shards, &types.Shard{ShardKey: shardID})
	}
	return shards
}
