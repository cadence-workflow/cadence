package externalshardassignment

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

const (
	shardAssignmentInterval = 4 * time.Second
	minimumAssignedShards   = 3
)

// ShardAssigneer assigns shards to the executor for canary testing
type ShardAssigner struct {
	logger          log.Logger
	timeSource      clock.TimeSource
	shardProcessors map[string]*processorephemeral.ShardProcessor
	executorclient  executorclient.Executor[*processorephemeral.ShardProcessor]
	stopChan        chan struct{}
	goRoutineWg     sync.WaitGroup
	namespace       string
}

// ShardAssignerParams contains the dependencies needed to create a ShardParam
type ShardAssignerParams struct {
	Logger           log.Logger
	TimeSource       clock.TimeSource
	ShardDistributor sharddistributor.Client
	Executorclient   executorclient.Executor[*processorephemeral.ShardProcessor]
}

// NewShardCreator creates a new ShardCreator instance with the given parameters and namespace
func NewShardAssigner(params ShardAssignerParams, namespace string) *ShardAssigner {
	sp := make(map[string]*processorephemeral.ShardProcessor)
	return &ShardAssigner{
		logger:          params.Logger,
		timeSource:      params.TimeSource,
		shardProcessors: sp,
		executorclient:  params.Executorclient,
		stopChan:        make(chan struct{}),
		goRoutineWg:     sync.WaitGroup{},
		namespace:       namespace,
	}
}

// Start begins the shard creation process in a background goroutine
func (s *ShardAssigner) Start() {
	s.goRoutineWg.Add(1)
	go s.process(context.Background())

	s.logger.Info("Shard assigner started")
}

// Stop stops the shard creation process and waits for the goroutine to finish
func (s *ShardAssigner) Stop() {
	close(s.stopChan)
	s.goRoutineWg.Wait()
}

// ShardCreatorModule creates an fx module for the shard creator with the given namespace
func ShardAssignerModule(namespace string) fx.Option {
	return fx.Module("shard-assigner",
		fx.Provide(func(params ShardAssignerParams) *ShardAssigner {

			return NewShardAssigner(params, namespace)
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle, shardAssigner *ShardAssigner) {
			lifecycle.Append(fx.StartStopHook(shardAssigner.Start, shardAssigner.Stop))
		}),
	)
}

func (s *ShardAssigner) process(ctx context.Context) {
	defer s.goRoutineWg.Done()

	ticker := s.timeSource.NewTicker(shardAssignmentInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.Chan():
			if len(s.shardProcessors) > minimumAssignedShards {
				var shardToRemove string
				shardToRemoveIndex := rand.Intn(len(s.shardProcessors))
				for shardID := range s.shardProcessors {
					if shardToRemoveIndex == 0 {
						shardToRemove = shardID
						break
					}
					shardToRemoveIndex--
				}
				err := s.executorclient.RemoveShardsFromLocalLogic([]string{shardToRemove})
				if err != nil {
					s.logger.Error("Failed to remove shards", tag.Error(err))
					continue
				}
				delete(s.shardProcessors, shardToRemove)
				s.logger.Info("Removed a shard from external source", tag.ShardKey(shardToRemove))

			}

			// Simulate the assignment of new shards
			newAssignedShard := uuid.New().String()
			s.logger.Info("Assign a new shard from external source", tag.ShardKey(newAssignedShard))
			shardAssignment := map[string]*types.ShardAssignment{
				newAssignedShard: {
					Status: types.AssignmentStatusREADY,
				},
			}
			err := s.executorclient.AssignShardsFromLocalLogic(context.Background(), shardAssignment)
			if err != nil {
				s.logger.Error("Failed to assign shard from external source", tag.Error(err))
				continue
			}
			sp, err := s.executorclient.GetShardProcess(ctx, newAssignedShard)
			if err != nil {
				s.logger.Error("failed to get shard assigned", tag.ShardKey(newAssignedShard), tag.Error(err))
			} else {
				s.logger.Info("shard assigned", tag.ShardStatus(string(sp.GetShardReport().Status)), tag.ShardLoad(strconv.FormatFloat(sp.GetShardReport().ShardLoad, 'f', -1, 64)))
			}
			s.shardProcessors[newAssignedShard] = sp
		}
	}
}
