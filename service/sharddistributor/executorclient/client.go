package executorclient

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/yarpc"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/client/sharddistributorexecutor"
	"github.com/uber/cadence/client/wrappers/errorinjectors"
	"github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/client/wrappers/metered"
	timeoutwrapper "github.com/uber/cadence/client/wrappers/timeout"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
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

	Dispatcher            *yarpc.Dispatcher
	DC                    *dynamicconfig.Collection
	MetricsClient         metrics.Client
	Logger                log.Logger
	ShardProcessorFactory ShardProcessorFactory[SP]
	Config                Config
	TimeSource            clock.TimeSource
}

func NewExecutor[SP ShardProcessor](params Params[SP]) (Executor[SP], error) {
	shardDistributorClient, err := createShardDistributorExecutorClient(params.Dispatcher, params.DC, params.MetricsClient, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("create shard distributor executor client: %w", err)
	}

	// TODO: get executor ID from environment
	executorID := uuid.New().String()

	return &executorImpl[SP]{
		logger:                 params.Logger,
		shardDistributorClient: shardDistributorClient,
		shardProcessorFactory:  params.ShardProcessorFactory,
		heartBeatInterval:      params.Config.HeartBeatInterval,
		namespace:              params.Config.Namespace,
		executorID:             executorID,
		timeSource:             params.TimeSource,
		stopC:                  make(chan struct{}),
	}, nil
}

func createShardDistributorExecutorClient(dispatcher *yarpc.Dispatcher, dc *dynamicconfig.Collection, metricsClient metrics.Client, logger log.Logger) (sharddistributorexecutor.Client, error) {
	shardDistributorClientConfig, ok := dispatcher.OutboundConfig(service.ShardDistributor)
	if !ok {
		return nil, fmt.Errorf("no outbound config for shard distributor")
	}
	if !rpc.IsGRPCOutbound(shardDistributorClientConfig) {
		return nil, fmt.Errorf("shard distributor client does not support non-GRPC outbound")
	}

	shardDistributorExecutorClient := grpc.NewShardDistributorExecutorClient(
		sharddistributorv1.NewShardDistributorExecutorAPIYARPCClient(shardDistributorClientConfig),
	)

	shardDistributorExecutorClient = timeoutwrapper.NewShardDistributorExecutorClient(shardDistributorExecutorClient, timeoutwrapper.ShardDistributorExecutorDefaultTimeout)
	if errorRate := dc.GetFloat64Property(dynamicproperties.ShardDistributorExecutorErrorInjectionRate)(); errorRate != 0 {
		shardDistributorExecutorClient = errorinjectors.NewShardDistributorExecutorClient(shardDistributorExecutorClient, errorRate, logger)
	}
	if metricsClient != nil {
		shardDistributorExecutorClient = metered.NewShardDistributorExecutorClient(shardDistributorExecutorClient, metricsClient)
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
