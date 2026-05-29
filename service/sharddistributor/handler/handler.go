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
	"slices"
	"sync"
	"time"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	// ephemeralBatchInterval is the time window over which GetShardOwner calls for
	// ephemeral namespaces are collected before being processed as a single batch.
	ephemeralBatchInterval = 100 * time.Millisecond

	// versionConflictRetryInitialInterval is the starting backoff for retries
	// triggered when a concurrent shard assignment causes a version conflict.
	versionConflictRetryInitialInterval = 50 * time.Millisecond
	// versionConflictRetryMaxInterval caps the per-attempt sleep.
	versionConflictRetryMaxInterval = 1 * time.Second
	// versionConflictRetryMaxAttempts is the maximum number of retry attempts
	// before the error is surfaced to the caller.
	versionConflictRetryMaxAttempts = 3
)

func NewHandler(
	logger log.Logger,
	timeSource clock.TimeSource,
	shardDistributionCfg config.ShardDistribution,
	cfg *config.Config,
	storage store.Store,
) Handler {
	handler := &handlerImpl{
		logger:               logger,
		shardDistributionCfg: shardDistributionCfg,
		cfg:                  cfg,
		storage:              storage,
	}

	handler.batcher = newShardBatcher(timeSource, ephemeralBatchInterval, handler.assignEphemeralBatch)

	// prevent us from trying to serve requests before shard distributor is started and ready
	handler.startWG.Add(1)
	return handler
}

type handlerImpl struct {
	logger log.Logger

	startWG sync.WaitGroup

	storage              store.Store
	shardDistributionCfg config.ShardDistribution
	cfg                  *config.Config

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
			return h.getOrAssignEphemeralShard(ctx, request)
		}

		return nil, &types.ShardNotFoundError{
			Namespace: request.Namespace,
			ShardKey:  request.ShardKey,
		}
	}
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to get shard owner: %v", err)}
	}

	return &types.GetShardOwnerResponse{
		Owner:     shardOwner.ExecutorID,
		Metadata:  shardOwner.Metadata,
		Namespace: request.Namespace,
	}, nil
}

// getOrAssignEphemeralShard assigns an ephemeral shard that does not yet exist
// in storage. It submits the request to the batcher and, on a version conflict
// (concurrent assignment by another goroutine), retries with exponential backoff.
// Each retry re-reads storage first: if the concurrent writer already committed
// the assignment we return it immediately without re-submitting to the batcher.
func (h *handlerImpl) getOrAssignEphemeralShard(ctx context.Context, request *types.GetShardOwnerRequest) (*types.GetShardOwnerResponse, error) {
	retryPolicy := backoff.NewExponentialRetryPolicy(versionConflictRetryInitialInterval)
	retryPolicy.SetMaximumInterval(versionConflictRetryMaxInterval)
	retryPolicy.SetMaximumAttempts(versionConflictRetryMaxAttempts)

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(func(err error) bool {
			return errors.Is(err, store.ErrVersionConflict)
		}),
	)

	var resp *types.GetShardOwnerResponse
	isRetry := false
	err := throttleRetry.Do(ctx, func(ctx context.Context) error {
		if isRetry {
			// A concurrent batch won the race. Re-read storage first: if the
			// winner already committed our shard's assignment we can return
			// immediately without re-submitting to the batcher.
			owner, err := h.storage.GetShardOwner(ctx, request.Namespace, request.ShardKey)
			if err != nil && !errors.Is(err, store.ErrShardNotFound) {
				return &types.InternalServiceError{Message: fmt.Sprintf("failed to get shard owner: %v", err)}
			}
			if err == nil {
				resp = &types.GetShardOwnerResponse{
					Owner:     owner.ExecutorID,
					Metadata:  owner.Metadata,
					Namespace: request.Namespace,
				}
				return nil
			}
		}
		isRetry = true

		// Submit to the batcher to assign the shard.
		var err error
		resp, err = h.batcher.Submit(ctx, request)
		return err
	})

	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to assign ephemeral shard: %v", err)}
	}
	return resp, nil
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
