package executorclient

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/sharddistributorexecutor"
	timeoutwrapper "github.com/uber/cadence/client/wrappers/timeout"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient/metricsconstants"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go . ShardProcessorFactory,ShardProcessor,Executor,Client

type Client interface {
	Heartbeat(context.Context, *types.ExecutorHeartbeatRequest, ...yarpc.CallOption) (*types.ExecutorHeartbeatResponse, error)
}

type ExecutorMetadata map[string]string

type ShardReport struct {
	ShardLoad float64
	Status    types.ShardStatus
}

type ShardProcessor interface {
	Start(ctx context.Context) error
	Stop()
	GetShardReport() ShardReport
}

type ShardProcessorFactory[SP ShardProcessor] interface {
	NewShardProcessor(shardID string) (SP, error)
}

type Executor[SP ShardProcessor] interface {
	Start(ctx context.Context)
	Stop()

	GetShardProcess(ctx context.Context, shardID string) (SP, error)

	// Get the namespace this executor is responsible for
	GetNamespace() string

	// Set metadata for the executor
	SetMetadata(metadata map[string]string)
	// Get the current metadata of the executor
	GetMetadata() map[string]string

	// AssignShardsFromLocalLogic is used for the migration during local-passthrough, local-passthrough-shadow, distributed-passthrough
	AssignShardsFromLocalLogic(ctx context.Context, shardAssignment map[string]*types.ShardAssignment) error
	// RemoveShardsFromLocalLogic is used for the migration during local-passthrough, local-passthrough-shadow, distributed-passthrough
	RemoveShardsFromLocalLogic(shardIDs []string) error
}

type Params[SP ShardProcessor] struct {
	fx.In

	ExecutorClient        Client
	MetricsScope          tally.Scope
	Logger                log.Logger
	ShardProcessorFactory ShardProcessorFactory[SP]
	Config                clientcommon.Config
	TimeSource            clock.TimeSource
	Metadata              ExecutorMetadata `optional:"true"`
}

// NewExecutorWithNamespace creates an executor for a specific namespace
func NewExecutorWithNamespace[SP ShardProcessor](params Params[SP], namespace string) (Executor[SP], error) {
	// Validate the config first
	if err := params.Config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Get config for the specified namespace
	namespaceConfig, err := params.Config.GetConfigForNamespace(namespace)
	if err != nil {
		return nil, fmt.Errorf("get config for namespace %s: %w", namespace, err)
	}

	return newExecutorWithConfig(params, namespaceConfig)
}

// NewExecutor creates an executor using auto-selection (single namespace only)
func NewExecutor[SP ShardProcessor](params Params[SP]) (Executor[SP], error) {
	// Validate the config first
	if err := params.Config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Auto-select if there's only one namespace
	namespaceConfig, err := params.Config.GetSingleConfig()
	if err != nil {
		return nil, fmt.Errorf("auto-select namespace: %w", err)
	}

	return newExecutorWithConfig(params, namespaceConfig)
}

func newExecutorWithConfig[SP ShardProcessor](params Params[SP], namespaceConfig *clientcommon.NamespaceConfig) (Executor[SP], error) {
	shardDistributorClient, err := createShardDistributorExecutorClient(params.ExecutorClient, params.MetricsScope, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("create shard distributor executor client: %w", err)
	}

	// TODO: get executor ID from environment
	executorID := uuid.New().String()

	metricsScope := params.MetricsScope.Tagged(map[string]string{
		metrics.OperationTagName: metricsconstants.ShardDistributorExecutorOperationTagName,
		"namespace":              namespaceConfig.Namespace,
	})

	executor := &executorImpl[SP]{
		logger:                 params.Logger,
		shardDistributorClient: shardDistributorClient,
		shardProcessorFactory:  params.ShardProcessorFactory,
		heartBeatInterval:      namespaceConfig.HeartBeatInterval,
		namespace:              namespaceConfig.Namespace,
		executorID:             executorID,
		timeSource:             params.TimeSource,
		stopC:                  make(chan struct{}),
		metrics:                metricsScope,
		metadata: syncExecutorMetadata{
			data: params.Metadata,
		},
	}
	executor.setMigrationMode(namespaceConfig.GetMigrationMode())

	return executor, nil
}

func createShardDistributorExecutorClient(client sharddistributorexecutor.Client, metricsScope tally.Scope, logger log.Logger) (sharddistributorexecutor.Client, error) {

	shardDistributorExecutorClient := timeoutwrapper.NewShardDistributorExecutorClient(client, timeoutwrapper.ShardDistributorExecutorDefaultTimeout)

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

// ModuleWithNamespace creates an executor module for a specific namespace
func ModuleWithNamespace[SP ShardProcessor](namespace string) fx.Option {
	return fx.Module(fmt.Sprintf("shard-distributor-executor-client-%s", namespace),
		fx.Provide(func(params Params[SP]) (Executor[SP], error) {
			return NewExecutorWithNamespace(params, namespace)
		}),
		fx.Invoke(func(executor Executor[SP], lc fx.Lifecycle) {
			lc.Append(fx.StartStopHook(executor.Start, executor.Stop))
		}),
	)
}
