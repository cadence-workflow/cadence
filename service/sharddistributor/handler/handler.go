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

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

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

	// prevent us from trying to serve requests before shard distributor is started and ready
	handler.startWG.Add(1)
	return handler
}

type handlerImpl struct {
	logger log.Logger

	startWG sync.WaitGroup

	storage              store.Store
	shardDistributionCfg config.ShardDistribution
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

	if !h.shardDistributionCfg.Enabled {
		return nil, fmt.Errorf("shard distributor disabled")
	}

	namespaceIdx := slices.IndexFunc(h.shardDistributionCfg.Namespaces, func(namespace config.Namespace) bool {
		return namespace.Name == request.Namespace
	})
	if namespaceIdx == -1 {
		return nil, &types.NamespaceNotFoundError{
			Namespace: request.Namespace,
		}
	}

	executorID, err := h.storage.GetShardOwner(ctx, request.Namespace, request.ShardKey)
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
		return nil, fmt.Errorf("get shard owner: %w", err)
	}

	resp = &types.GetShardOwnerResponse{
		Owner:     executorID,
		Namespace: request.Namespace,
	}

	return resp, nil
}

func (h *handlerImpl) assignEphemeralShard(ctx context.Context, namespace string, shardID string) (*types.GetShardOwnerResponse, error) {

	// Get the current state of the namespace and evaluate executor load to choose a placement target.
	state, err := h.storage.GetState(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}

	executorID, aggregatedLoad, assignedCount, err := pickLeastLoadedExecutor(state)
	if err != nil {
		h.logger.Error(
			"no eligible executor found for ephemeral assignment",
			tag.ShardNamespace(namespace),
			tag.ShardKey(shardID),
			tag.Error(err),
		)
		return nil, err
	}

	h.logger.Info(
		"selected executor for ephemeral shard assignment",
		tag.AggregateLoad(aggregatedLoad),
		tag.AssignedCount(assignedCount),
		tag.ShardNamespace(namespace),
		tag.ShardKey(shardID),
		tag.ShardExecutor(executorID),
	)

	// Assign the shard to the executor with the least assigned shards
	err = h.storage.AssignShard(ctx, namespace, shardID, executorID)
	if err != nil {
		h.logger.Error(
			"failed to assign ephemeral shard",
			tag.ShardNamespace(namespace),
			tag.ShardKey(shardID),
			tag.ShardExecutor(executorID),
			tag.Error(err),
		)
		return nil, fmt.Errorf("assign ephemeral shard: %w", err)
	}

	return &types.GetShardOwnerResponse{
		Owner:     executorID,
		Namespace: namespace,
	}, nil
}

// pickLeastLoadedExecutor returns the executor with the minimal aggregated smoothed load.
// Ties are broken by fewer assigned shards.
func pickLeastLoadedExecutor(state *store.NamespaceState) (executorID string, aggregatedLoad float64, assignedCount int, err error) {
	if state == nil || len(state.ShardAssignments) == 0 {
		return "", 0, 0, fmt.Errorf("namespace state is nil or has no executors")
	}

	var chosenID string
	var chosenAggregatedLoad float64
	var chosenAssignedCount int
	minAggregatedLoad := math.MaxFloat64
	minAssignedShards := math.MaxInt

	for candidate, assignment := range state.ShardAssignments {
		aggregated := 0.0
		for shard := range assignment.AssignedShards {
			if stats, ok := state.ShardStats[shard]; ok {
				if !math.IsNaN(stats.SmoothedLoad) && !math.IsInf(stats.SmoothedLoad, 0) {
					aggregated += stats.SmoothedLoad
				}
			}
		}

		count := len(assignment.AssignedShards)
		if aggregated < minAggregatedLoad || (aggregated == minAggregatedLoad && count < minAssignedShards) {
			minAggregatedLoad = aggregated
			minAssignedShards = count
			chosenID = candidate
			chosenAggregatedLoad = aggregated
			chosenAssignedCount = count
		}
	}

	if chosenID == "" {
		return "", 0, 0, fmt.Errorf("no executors in namespace state")
	}

	return chosenID, chosenAggregatedLoad, chosenAssignedCount, nil
}
