package processorephemeral

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/yarpc"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

//go:generate mockgen -package $GOPACKAGE -destination canary_client_mock_test.go github.com/uber/cadence/.gen/proto/sharddistributor/v1 ShardDistributorExecutorCanaryAPIYARPCClient

const (
	shardCreationInterval = 1 * time.Second
)

// ShardCreator creates shards at regular intervals for ephemeral canary testing
type ShardCreator struct {
	logger       *zap.Logger
	timeSource   clock.TimeSource
	spectators   *spectatorclient.Spectators
	canaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient
	namespaces   []string

	stopChan    chan struct{}
	goRoutineWg sync.WaitGroup
}

// ShardCreatorParams contains the dependencies needed to create a ShardCreator
type ShardCreatorParams struct {
	fx.In

	Logger       *zap.Logger
	TimeSource   clock.TimeSource
	Spectators   *spectatorclient.Spectators
	CanaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient
}

// NewShardCreator creates a new ShardCreator instance with the given parameters and namespace
func NewShardCreator(params ShardCreatorParams, namespaces []string) *ShardCreator {
	return &ShardCreator{
		logger:       params.Logger,
		timeSource:   params.TimeSource,
		spectators:   params.Spectators,
		canaryClient: params.CanaryClient,
		stopChan:     make(chan struct{}),
		goRoutineWg:  sync.WaitGroup{},
		namespaces:   namespaces,
	}
}

// Start begins the shard creation process in a background goroutine
func (s *ShardCreator) Start() {
	s.goRoutineWg.Add(1)
	go s.process(context.Background())
	s.logger.Info("Shard creator started")
}

// Stop stops the shard creation process and waits for the goroutine to finish
func (s *ShardCreator) Stop() {
	close(s.stopChan)
	s.goRoutineWg.Wait()
	s.logger.Info("Shard creator stopped")
}

// ShardCreatorModule creates an fx module for the shard creator with the given namespace
func ShardCreatorModule(namespace []string) fx.Option {
	return fx.Module("shard-creator",
		fx.Provide(func(params ShardCreatorParams) *ShardCreator {
			return NewShardCreator(params, namespace)
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle, shardCreator *ShardCreator) {
			lifecycle.Append(fx.StartStopHook(shardCreator.Start, shardCreator.Stop))
		}),
	)
}

func (s *ShardCreator) process(ctx context.Context) {
	defer s.goRoutineWg.Done()

	ticker := s.timeSource.NewTicker(shardCreationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.Chan():
			for _, namespace := range s.namespaces {
				shardKey := uuid.New().String()
				s.logger.Info("Creating shard", zap.String("shardKey", shardKey), zap.String("namespace", namespace))

				// Get spectator for this namespace
				spectator, err := s.spectators.ForNamespace(namespace)
				if err != nil {
					s.logger.Warn("No spectator for namespace, skipping shard creation",
						zap.String("namespace", namespace),
						zap.String("shardKey", shardKey),
						zap.Error(err))
					continue
				}

				// Create shard and get owner via spectator (spectator will create if not exists)
				owner, err := spectator.GetShardOwner(ctx, shardKey)
				if err != nil {
					s.logger.Error("Failed to get/create shard owner",
						zap.Error(err),
						zap.String("namespace", namespace),
						zap.String("shardKey", shardKey))
					continue
				}

				s.logger.Info("Shard created, got owner from spectator",
					zap.String("shardKey", shardKey),
					zap.String("namespace", namespace),
					zap.String("executor_id", owner.ExecutorID))

				// Now ping the owner to verify
				if err := s.pingShardOwner(ctx, owner, namespace, shardKey); err != nil {
					s.logger.Error("Failed to ping shard owner",
						zap.Error(err),
						zap.String("namespace", namespace),
						zap.String("shardKey", shardKey))
				}
			}
		}
	}
}

func (s *ShardCreator) pingShardOwner(ctx context.Context, owner *spectatorclient.ShardOwner, namespace, shardKey string) error {
	s.logger.Debug("Pinging shard owner after creation",
		zap.String("namespace", namespace),
		zap.String("shardKey", shardKey),
		zap.String("expected_executor_id", owner.ExecutorID))

	// Create ping request
	request := &sharddistributorv1.PingRequest{
		ShardKey:  shardKey,
		Namespace: namespace,
	}

	// SIMPLE CANARY CODE: Just pass the shard key and namespace!
	// The SpectatorPeerChooser library code handles routing to the right executor
	response, err := s.canaryClient.Ping(ctx, request, yarpc.WithShardKey(shardKey), yarpc.WithHeader(spectatorclient.NamespaceHeader, namespace))
	if err != nil {
		return fmt.Errorf("ping rpc failed: %w", err)
	}

	// Verify response matches the owner we got from spectator
	if response.GetExecutorId() != owner.ExecutorID {
		s.logger.Warn("Executor ID mismatch",
			zap.String("namespace", namespace),
			zap.String("shardKey", shardKey),
			zap.String("expected", owner.ExecutorID),
			zap.String("actual", response.GetExecutorId()))
	}

	if !response.GetOwnsShard() {
		s.logger.Warn("Executor does not own shard",
			zap.String("namespace", namespace),
			zap.String("shardKey", shardKey),
			zap.String("executor_id", response.GetExecutorId()))
		return fmt.Errorf("executor %s does not own shard %s", response.GetExecutorId(), shardKey)
	}

	s.logger.Info("Successfully verified shard owner after creation",
		zap.String("namespace", namespace),
		zap.String("shardKey", shardKey),
		zap.String("executor_id", response.GetExecutorId()))

	return nil
}
