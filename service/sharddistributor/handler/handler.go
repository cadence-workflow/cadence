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

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func NewHandler(
	logger log.Logger,
	shardDistributionCfg config.ShardDistribution,
	storage store.Store,
	timeSource clock.TimeSource,
) Handler {
	handler := &handlerImpl{
		logger:               logger,
		shardDistributionCfg: shardDistributionCfg,
		storage:              storage,
		timeSource:           timeSource,
	}

	// prevent us from trying to serve requests before shard distributor is started and ready
	handler.startWG.Add(1)
	return handler
}

type handlerImpl struct {
	logger log.Logger

	startWG sync.WaitGroup

	storage              store.Store
	shardDistributionCfg config.ShardDistribution
	timeSource           clock.TimeSource
}

func (h *handlerImpl) Start() {
	h.startWG.Done()
}

func (h *handlerImpl) Stop() {
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
			return h.assignEphemeralShard(ctx, request.Namespace, request.ShardKey)
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

func (h *handlerImpl) assignEphemeralShard(ctx context.Context, namespace string, shardID string) (*types.GetShardOwnerResponse, error) {

	// Get the current state of the namespace and find the best executor for a new ephemeral shard.
	// Prefer ACTIVE, non-stale executors with the lowest current load. Break ties by shard count and ID.
	state, err := h.storage.GetState(ctx, namespace)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("get namespace state: %v", err)}
	}

	now := h.timeSource.Now().UTC()
	ttl := h.shardDistributionCfg.Process.HeartbeatTTL

	candidates := make([]string, 0)
	if len(state.Executors) > 0 {
		for executorID, hb := range state.Executors {
			if hb.Status != types.ExecutorStatusACTIVE {
				continue
			}
			if ttl > 0 && !hb.LastHeartbeat.IsZero() && now.Sub(hb.LastHeartbeat) > ttl {
				continue
			}
			candidates = append(candidates, executorID)
		}
	}

	// Fallback to existing assignments if heartbeats are missing.
	if len(candidates) == 0 {
		for executorID := range state.ShardAssignments {
			candidates = append(candidates, executorID)
		}
	}
	if len(candidates) == 0 {
		return nil, &types.InternalServiceError{Message: "no eligible executors for ephemeral shard"}
	}

	slices.Sort(candidates)

	executorID := ""
	minLoad := math.MaxFloat64
	minAssignedShards := math.MaxInt

	for _, candidate := range candidates {
		assignment, ok := state.ShardAssignments[candidate]
		assignedShards := 0
		if ok {
			assignedShards = len(assignment.AssignedShards)
		}

		load := 0.0
		if ok {
			for sID := range assignment.AssignedShards {
				stats, ok := state.ShardStats[sID]
				if !ok {
					continue
				}
				if ttl > 0 && !stats.LastUpdateTime.IsZero() && now.Sub(stats.LastUpdateTime) > ttl {
					continue
				}
				load += stats.SmoothedLoad
			}
		}

		if load < minLoad ||
			(load == minLoad && (assignedShards < minAssignedShards ||
				(assignedShards == minAssignedShards && (executorID == "" || candidate < executorID)))) {
			minLoad = load
			minAssignedShards = assignedShards
			executorID = candidate
		}
	}

	// Assign the shard to the selected executor.
	err = h.storage.AssignShard(ctx, namespace, shardID, executorID)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("assign ephemeral shard: %v", err)}
	}

	executor, err := h.storage.GetExecutor(ctx, namespace, executorID)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("get executor: %v", err)}
	}

	return &types.GetShardOwnerResponse{
		Owner:     executor.ExecutorID,
		Namespace: namespace,
		Metadata:  executor.Metadata,
	}, nil
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
