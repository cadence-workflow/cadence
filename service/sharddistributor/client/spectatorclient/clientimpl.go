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
	csync "github.com/uber/cadence/service/sharddistributor/client/spectatorclient/sync"
)

// stateFn represents a state in the election state machine.
// Each state is a function that blocks until a transition occurs
// and returns the next state function, or nil to stop.
// Note this is a recursive type definition.
type stateFn func(ctx context.Context) stateFn

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
	enabled    EnabledFunc
	config     clientcommon.NamespaceConfig
	client     sharddistributor.Client
	scope      tally.Scope
	logger     log.Logger
	timeSource clock.TimeSource
	stream     sharddistributor.WatchNamespaceStateClient

	cancel context.CancelFunc
	stopWG sync.WaitGroup

	// State storage with lock for thread-safe access
	// Map from shard ID to shard owner (executor ID + metadata)
	stateMu      sync.RWMutex
	shardToOwner map[string]*ShardOwner

	// Signal to notify when first state is received
	firstStateSignal *csync.ResettableSignal
}

func (s *spectatorImpl) Start(ctx context.Context) error {
	// Create a cancellable context for the lifetime of the spectator
	// Use context.WithoutCancel to inherit values but not cancellation from fx lifecycle ctx
	ctx, s.cancel = context.WithCancel(context.WithoutCancel(ctx))

	s.stopWG.Add(1)
	go func() {
		defer s.stopWG.Done()
		s.watchLoop(ctx)
	}()

	return nil
}

func (s *spectatorImpl) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	// Close the firstStateSignal to unblock any goroutines waiting for first state
	s.firstStateSignal.Done()
	s.stopWG.Wait()
}

func (s *spectatorImpl) watchLoop(ctx context.Context) {
	defer s.logger.Info("Shutting down, stopping watch loop", tag.ShardNamespace(s.namespace))
	s.logger.Info("Starting watch loop for namespace", tag.ShardNamespace(s.namespace))

	var state stateFn
	if s.enabled() {
		state = s.connectState
	} else {
		state = s.disabledState
	}

	for state != nil {
		state = state(ctx)
	}
}

func (s *spectatorImpl) connectState(ctx context.Context) stateFn {
	defer s.logger.Info("Exiting connect state", tag.ShardNamespace(s.namespace))
	s.logger.Info("Starting connect state for namespace", tag.ShardNamespace(s.namespace))

	if !s.enabled() {
		return s.disabledState
	}

	stream, err := s.client.WatchNamespaceState(ctx, &types.WatchNamespaceStateRequest{
		Namespace: s.namespace,
	})

	if err != nil {
		if ctx.Err() != nil {
			return nil
		}

		s.logger.Error("Failed to create stream, retrying", tag.Error(err), tag.ShardNamespace(s.namespace))
		if err := s.timeSource.SleepWithContext(ctx, backoff.JitDuration(streamRetryInterval, streamRetryJitterCoeff)); err != nil {
			return nil
		}
		return s.connectState
	}

	s.stream = stream

	return s.enabledState
}

func (s *spectatorImpl) enabledState(ctx context.Context) stateFn {
	defer s.logger.Info("Exiting enabled state", tag.ShardNamespace(s.namespace))
	defer func() {
		if err := s.stream.CloseSend(); err != nil {
			s.logger.Warn("Failed to close stream", tag.Error(err), tag.ShardNamespace(s.namespace))
		}
	}()

	s.logger.Info("Starting enabled state for namespace", tag.ShardNamespace(s.namespace))

	for {
		if !s.enabled() {
			return s.disabledState
		}

		response, err := s.stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				s.logger.Info("Recv interrupted by client shutdown", tag.ShardNamespace(s.namespace))
				return nil
			}

			s.logger.Warn("Stream error (server issue), will reconnect", tag.Error(err), tag.ShardNamespace(s.namespace))
			if err := s.timeSource.SleepWithContext(ctx, backoff.JitDuration(streamRetryInterval, streamRetryJitterCoeff)); err != nil {
				return nil
			}
			return s.connectState

		}

		// Process the response
		s.handleResponse(response)
	}
}

func (s *spectatorImpl) disabledState(ctx context.Context) stateFn {
	defer s.logger.Info("Exiting disabled state", tag.ShardNamespace(s.namespace))
	s.logger.Info("Starting disabled state for namespace", tag.ShardNamespace(s.namespace))
	// We reset the first state signal to ensure we wait for the first state to be received when we re-enable.
	s.firstStateSignal.Reset()

	for {
		// Sleep for a short period of time before checking again.
		// If the context is cancelled, we return nil to exit the loop.
		if err := s.timeSource.SleepWithContext(ctx, backoff.JitDuration(streamRetryInterval, streamRetryJitterCoeff)); err != nil {
			return nil
		}
		if s.enabled() {
			return s.connectState
		}
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

	s.stateMu.Lock()
	s.shardToOwner = shardToOwner
	s.stateMu.Unlock()

	// Signal that first state has been received - this function is free to call
	// multiple times.
	s.firstStateSignal.Done()

	s.logger.Debug("Received namespace state update",
		tag.ShardNamespace(s.namespace),
		tag.Counter(len(response.Executors)))
}

// GetShardOwner returns the full owner information including metadata for a given shard.
// It first waits for the initial state to be received, then checks the cache.
// If not found in cache, it falls back to querying the shard distributor directly.
func (s *spectatorImpl) GetShardOwner(ctx context.Context, shardKey string) (*ShardOwner, error) {
	// Wait for first state to be received to avoid flooding shard distributor on startup
	if err := s.firstStateSignal.Wait(ctx); err != nil {
		return nil, fmt.Errorf("wait for first state: %w", err)
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
