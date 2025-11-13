package externalshardassignment

import (
	"context"
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
	shardAssignmentInterval = 1 * time.Second
)

// ShardAssigneer assigns shards to the executor for canary testing
type ShardAssigner struct {
	logger         log.Logger
	timeSource     clock.TimeSource
	executorclient executorclient.Executor[*processorephemeral.ShardProcessor]
	stopChan       chan struct{}
	goRoutineWg    sync.WaitGroup
	namespace      string
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
	return &ShardAssigner{
		logger:         params.Logger,
		timeSource:     params.TimeSource,
		executorclient: params.Executorclient,
		stopChan:       make(chan struct{}),
		goRoutineWg:    sync.WaitGroup{},
		namespace:      namespace,
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
			newAssignedShard := uuid.New().String()
			s.logger.Info("Assign a new shard from external source", tag.ShardKey(newAssignedShard))
			shardAssignment := map[string]*types.ShardAssignment{
				newAssignedShard: {
					Status: types.AssignmentStatusREADY,
				},
			}
			s.executorclient.AssignShardsFromLocalLogic(context.Background(), shardAssignment)
			sp, err := s.executorclient.GetShardProcess(ctx, newAssignedShard)
			if err != nil {
				s.logger.Error("failed to get shard assigned", tag.ShardKey(newAssignedShard), tag.Error(err))
			} else {
				s.logger.Info("shard assigned", tag.ShardStatus(string(sp.GetShardReport().Status)), tag.ShardLoad(strconv.FormatFloat(sp.GetShardReport().ShardLoad, 'f', -1, 64)))
			}

		}
	}
}
