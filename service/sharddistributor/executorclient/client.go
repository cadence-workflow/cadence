package executorclient

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uber-go/tally"
	"go.uber.org/fx"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/client/sharddistributorexecutor"
	"github.com/uber/cadence/client/wrappers/grpc"
	timeoutwrapper "github.com/uber/cadence/client/wrappers/timeout"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/sharddistributor/executorclient/metricsconstants"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go . ShardProcessorFactory,ShardProcessor,Executor

type ShardProcessor interface {
	Start(ctx context.Context)
	Stop()
	GetShardLoad() float64
}

type ShardProcessorFactory[SP ShardProcessor] interface {
	NewShardProcessor(shardID string) (SP, error)
}

type Executor[SP ShardProcessor] interface {
	Start(ctx context.Context)
	Stop()

	GetShardProcess(shardID string) (SP, error)
}

type Params[SP ShardProcessor] struct {
	fx.In

	YarpcClient           sharddistributorv1.ShardDistributorExecutorAPIYARPCClient
	MetricsScope          tally.Scope
	Logger                log.Logger
	ShardProcessorFactory ShardProcessorFactory[SP]
	Config                Config
	TimeSource            clock.TimeSource
}

func NewExecutor[SP ShardProcessor](params Params[SP]) (Executor[SP], error) {
	shardDistributorClient, err := createShardDistributorExecutorClient(params.YarpcClient, params.MetricsScope, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("create shard distributor executor client: %w", err)
	}

	// TODO: get executor ID from environment
	executorID := uuid.New().String()

	metricsScope := params.MetricsScope.Tagged(map[string]string{
		metrics.OperationTagName: metricsconstants.ShardDistributorExecutorOperationTagName,
		"namespace":              params.Config.Namespace,
	})

	return &executorImpl[SP]{
		logger:                 params.Logger,
		shardDistributorClient: shardDistributorClient,
		shardProcessorFactory:  params.ShardProcessorFactory,
		heartBeatInterval:      params.Config.HeartBeatInterval,
		namespace:              params.Config.Namespace,
		executorID:             executorID,
		timeSource:             params.TimeSource,
		stopC:                  make(chan struct{}),
		metrics:                metricsScope,
	}, nil
}

func createShardDistributorExecutorClient(yarpcClient sharddistributorv1.ShardDistributorExecutorAPIYARPCClient, metricsScope tally.Scope, logger log.Logger) (sharddistributorexecutor.Client, error) {
	shardDistributorExecutorClient := grpc.NewShardDistributorExecutorClient(yarpcClient)

	shardDistributorExecutorClient = timeoutwrapper.NewShardDistributorExecutorClient(shardDistributorExecutorClient, timeoutwrapper.ShardDistributorExecutorDefaultTimeout)

	if metricsScope != nil {
		shardDistributorExecutorClient = NewMeteredShardDistributorExecutorClient(shardDistributorExecutorClient, metricsScope)
	}

	return shardDistributorExecutorClient, nil
}

func Module[SP ShardProcessor]() fx.Option {
	return fx.Module("shard-distributor-executor-client",
		fx.Provide(NewExecutor[SP]),
		fx.Invoke(func(executor Executor[SP], lc fx.Lifecycle) {
			lc.Append(fx.StartStopHook(executor.Start, executor.Stop))
		}),
	)
}
