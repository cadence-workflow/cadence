package processorephemeral

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

const (
	shardCreationInterval = 1 * time.Second
)

// ShardCreator creates shards at regular intervals for ephemeral canary testing
type ShardCreator struct {
	logger           *zap.Logger
	timeSource       clock.TimeSource
	shardDistributor sharddistributor.Client

	stopChan    chan struct{}
	goRoutineWg sync.WaitGroup
	namespace   string
}

// ShardCreatorParams contains the dependencies needed to create a ShardCreator
type ShardCreatorParams struct {
	fx.In

	Logger           *zap.Logger
	TimeSource       clock.TimeSource
	ShardDistributor sharddistributor.Client
}

// NewShardCreator creates a new ShardCreator instance with the given parameters and namespace
func NewShardCreator(params ShardCreatorParams, namespace string) *ShardCreator {
	return &ShardCreator{
		logger:           params.Logger,
		timeSource:       params.TimeSource,
		shardDistributor: params.ShardDistributor,
		stopChan:         make(chan struct{}),
		goRoutineWg:      sync.WaitGroup{},
		namespace:        namespace,
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
}

// ShardCreatorModule creates an fx module for the shard creator with the given namespace
func ShardCreatorModule(namespace string) fx.Option {
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
			shardKey := uuid.New().String()
			s.logger.Info("Creating shard", zap.String("shardKey", shardKey))
			response, err := s.shardDistributor.GetShardOwner(ctx, &types.GetShardOwnerRequest{
				ShardKey:  shardKey,
				Namespace: s.namespace,
			})
			if err != nil {
				s.logger.Error("create shard failed", zap.Error(err))
				continue
			}
			s.logger.Info("shard created", zap.String("shardKey", shardKey), zap.String("shardOwner", response.Owner))
		}
	}
}
