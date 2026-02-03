package spectatorclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/uber-go/tally"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

const (
	streamRetryInterval    = 1 * time.Second
	streamRetryJitterCoeff = 0.1 // 10% jitter (900ms - 1100ms)
)

// ShardOwner contains information about the executor that owns a shard
type ShardOwner struct {
	ExecutorID string
	Metadata   map[string]string
}

type spectatorImpl struct {
	namespace  string
	config     clientcommon.NamespaceConfig
	client     sharddistributor.Client
	scope      tally.Scope
	logger     log.Logger
	timeSource clock.TimeSource

	ctx    context.Context
	cancel context.CancelFunc
	stopWG sync.WaitGroup

	// State storage with lock for thread-safe access
	// Map from shard ID to shard owner (executor ID + metadata)
	stateMu      sync.RWMutex
	shardToOwner map[string]*ShardOwner

	// Channel to signal when first state is received
	firstStateCh   chan struct{}
	firstStateOnce sync.Once
}

func (s *spectatorImpl) Start(ctx context.Context) error {
	// Create a cancellable context for the lifetime of the spectator
	// Use context.WithoutCancel to inherit values but not cancellation from fx lifecycle ctx
	s.ctx, s.cancel = context.WithCancel(context.WithoutCancel(ctx))

	s.stopWG.Add(1)
	go func() {
		defer s.stopWG.Done()
		s.watchLoop()
	}()

	return nil
}

func (s *spectatorImpl) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	// Close the firstStateCh to unblock any goroutines waiting for first state
	s.firstStateOnce.Do(func() {
		close(s.firstStateCh)
	})
	s.stopWG.Wait()
}

func (s *spectatorImpl) watchLoop() {
	s.logger.Info("Starting watch loop for namespace", tag.ShardNamespace(s.namespace))

	for {
		if s.ctx.Err() != nil {
			s.logger.Info("Shutting down, stopping watch loop", tag.ShardNamespace(s.namespace))
			return
		}

		// Create new stream
		stream, err := s.client.WatchNamespaceState(s.ctx, &types.WatchNamespaceStateRequest{
			Namespace: s.namespace,
		})
		if err != nil {
			if s.ctx.Err() != nil {
				s.logger.Info("Shutting down during stream creation, exiting watch loop", tag.ShardNamespace(s.namespace))
				return
			}

			s.logger.Error("Failed to create stream, retrying", tag.Error(err), tag.ShardNamespace(s.namespace))
			if err := s.timeSource.SleepWithContext(s.ctx, backoff.JitDuration(streamRetryInterval, streamRetryJitterCoeff)); err != nil {
				s.logger.Info("Shutting down waiting to retry stream creation, exiting watch loop", tag.ShardNamespace(s.namespace))
				return // Context cancelled during sleep
			}
			continue
		}
		s.logger.Info("Stream created, entering receive loop", tag.ShardNamespace(s.namespace))
		s.receiveLoop(stream)

		if s.ctx.Err() != nil {
			s.logger.Info("Shutting down, exiting watch loop", tag.ShardNamespace(s.namespace))
			return
		}

		// Server shutdown or network issue - recreate stream (load balancer will route to new server)
		s.logger.Info("Stream ended, reconnecting", tag.ShardNamespace(s.namespace))
		if err := s.timeSource.SleepWithContext(s.ctx, backoff.JitDuration(streamRetryInterval, streamRetryJitterCoeff)); err != nil {
			return // Context cancelled during sleep
		}
	}
}

func (s *spectatorImpl) receiveLoop(stream sharddistributor.WatchNamespaceStateClient) {
	defer func() {
		if err := stream.CloseSend(); err != nil {
			s.logger.Warn("Failed to close stream", tag.Error(err), tag.ShardNamespace(s.namespace))
		}
	}()

	for {
		response, err := stream.Recv()
		if err != nil {
			if s.ctx.Err() != nil {
				// Client shutdown - Recv() unblocked due to context cancellation
				s.logger.Info("Recv interrupted by client shutdown", tag.ShardNamespace(s.namespace))
			} else {
				// Server error - io.EOF, network error, server shutdown, etc.
				s.logger.Warn("Stream error (server issue), will reconnect", tag.Error(err), tag.ShardNamespace(s.namespace))
			}
			return // Exit receiveLoop, watchLoop will handle reconnection or shutdown
		}

		// Process the response
		s.handleResponse(response)
	}
}

func (s *spectatorImpl) handleResponse(response *types.WatchNamespaceStateResponse) {
	// Build inverted map: shard ID -> shard owner (executor ID + metadata)
	shardToOwner := make(map[string]*ShardOwner)
	for _, executor := range response.Executors {
		owner := &ShardOwner{
			ExecutorID: executor.ExecutorID,
			Metadata:   executor.Metadata,
		}
		for _, shard := range executor.AssignedShards {
			shardToOwner[shard.ShardKey] = owner
		}
	}

	// Check if this is the first state we're receiving
	isFirstState := false
	s.stateMu.Lock()
	if s.shardToOwner == nil {
		isFirstState = true
	}
	s.shardToOwner = shardToOwner
	s.stateMu.Unlock()

	// Signal that first state has been received (only once)
	if isFirstState {
		s.firstStateOnce.Do(func() {
			close(s.firstStateCh)
		})
	}

	s.logger.Debug("Received namespace state update",
		tag.ShardNamespace(s.namespace),
		tag.Counter(len(response.Executors)))
}

// GetShardOwner returns the full owner information including metadata for a given shard.
// It first waits for the initial state to be received, then checks the cache.
// If not found in cache, it falls back to querying the shard distributor directly.
func (s *spectatorImpl) GetShardOwner(ctx context.Context, shardKey string) (*ShardOwner, error) {
	// Wait for first state to be received to avoid flooding shard distributor on startup
	select {
	case <-s.firstStateCh:
		// First state received, continue
	case <-ctx.Done():
		// Context cancelled or timed out before first state received
		return nil, fmt.Errorf("context cancelled while waiting for first state: %w", ctx.Err())
	}

	// Check cache first
	s.stateMu.RLock()
	owner := s.shardToOwner[shardKey]
	s.stateMu.RUnlock()

	if owner != nil {
		return owner, nil
	}

	// Cache miss - fall back to RPC call
	s.logger.Debug("Shard not found in cache, querying shard distributor",
		tag.ShardKey(shardKey),
		tag.ShardNamespace(s.namespace))

	response, err := s.client.GetShardOwner(ctx, &types.GetShardOwnerRequest{
		Namespace: s.namespace,
		ShardKey:  shardKey,
	})
	if err != nil {
		return nil, fmt.Errorf("get shard owner from shard distributor: %w", err)
	}

	return &ShardOwner{
		ExecutorID: response.Owner,
		Metadata:   response.Metadata,
	}, nil
}
