package processorephemeral

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/sharddistributor/canary/pinger"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

//go:generate mockgen -package $GOPACKAGE -destination canary_client_mock_test.go github.com/uber/cadence/.gen/proto/sharddistributor/v1 ShardDistributorExecutorCanaryAPIYARPCClient

const (
	shardCreationInterval = 1 * time.Second

	// maxAssignedShardsLimit defines the maximum number of assigned shards
	// an executor can have before the shard creator stops creating new shards
	maxAssignedShardsLimit = 1000
)

// ShardCreator creates shards at regular intervals for ephemeral canary testing
type ShardCreator struct {
	logger       *zap.Logger
	timeSource   clock.TimeSource
	canaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient
	namespaces   []string
	executors    map[string]executorclient.Executor[*ShardProcessor]

	stopChan    chan struct{}
	goRoutineWg sync.WaitGroup
}

// ShardCreatorParams contains the dependencies needed to create a ShardCreator
type ShardCreatorParams struct {
	fx.In

	Logger       *zap.Logger
	TimeSource   clock.TimeSource
	CanaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient

	ExecutorsEphemeral []executorclient.Executor[*ShardProcessor] `group:"executor-ephemeral-proc"`
}

// NewShardCreator creates a new ShardCreator instance with the given parameters and namespace
func NewShardCreator(params ShardCreatorParams, namespaces []string) *ShardCreator {
	executors := make(map[string]executorclient.Executor[*ShardProcessor])
	for _, executor := range params.ExecutorsEphemeral {
		executors[executor.GetNamespace()] = executor
	}

	return &ShardCreator{
		logger:       params.Logger,
		timeSource:   params.TimeSource,
		canaryClient: params.CanaryClient,
		stopChan:     make(chan struct{}),
		goRoutineWg:  sync.WaitGroup{},
		namespaces:   namespaces,
		executors:    executors,
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
				if !s.isAllowed(namespace) {
					continue
				}

				shardKey := uuid.New().String()
				s.logger.Info("Creating shard", zap.String("shardKey", shardKey), zap.String("namespace", namespace))

				pinger.PingShard(ctx, s.canaryClient, s.logger, namespace, shardKey)
			}
		}
	}
}

// isAllowed checks if a new shard can be created for the given namespace
// based on the max assigned shards limit
func (s *ShardCreator) isAllowed(namespace string) bool {
	executor, ok := s.executors[namespace]
	if !ok {
		return true
	}

	assignedShardsCount := executor.GetAssignedShardsCount()
	if assignedShardsCount < maxAssignedShardsLimit {
		return true
	}

	s.logger.Info("Shard creator exceeds max assigned shards limit",
		zap.String("namespace", namespace),
		zap.Int64("assignedShards", assignedShardsCount),
		zap.Int64("maxAssignedShardsLimit", maxAssignedShardsLimit),
	)

	return false
}
