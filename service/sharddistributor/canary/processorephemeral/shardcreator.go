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
)

//go:generate mockgen -package $GOPACKAGE -destination canary_client_mock_test.go github.com/uber/cadence/.gen/proto/sharddistributor/v1 ShardDistributorExecutorCanaryAPIYARPCClient

const (
	shardCreationInterval = 1 * time.Second
)

// ShardCreator creates shards at regular intervals for ephemeral canary testing
type ShardCreator struct {
	logger       *zap.Logger
	timeSource   clock.TimeSource
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
	CanaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient
}

// NewShardCreator creates a new ShardCreator instance with the given parameters and namespace
func NewShardCreator(params ShardCreatorParams, namespaces []string) *ShardCreator {
	return &ShardCreator{
		logger:       params.Logger,
		timeSource:   params.TimeSource,
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

				pinger.PingShard(ctx, s.canaryClient, s.logger, namespace, shardKey)
			}
		}
	}
}
