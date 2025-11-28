package canary

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/yarpc"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/service/sharddistributor/canary/executors"
	"github.com/uber/cadence/service/sharddistributor/canary/factory"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/canary/sharddistributorclient"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

type NamespacesNames struct {
	fx.In
	FixedNamespace              string
	EphemeralNamespace          string
	ExternalAssignmentNamespace string
	SharddistributorServiceName string
}

func Module(namespacesNames NamespacesNames) fx.Option {
	return fx.Module("shard-distributor-canary", opts(namespacesNames))
}

func opts(names NamespacesNames) fx.Option {
	return fx.Options(
		fx.Provide(sharddistributorv1.NewFxShardDistributorExecutorAPIYARPCClient(names.SharddistributorServiceName)),
		fx.Provide(sharddistributorv1.NewFxShardDistributorAPIYARPCClient(names.SharddistributorServiceName)),

		fx.Provide(sharddistributorclient.NewShardDistributorClient),

		// Modules for the shard distributor canary
		fx.Provide(
			func(params factory.Params) executorclient.ShardProcessorFactory[*processor.ShardProcessor] {
				return factory.NewShardProcessorFactory(params, processor.NewShardProcessor)
			},
			func(params factory.Params) executorclient.ShardProcessorFactory[*processorephemeral.ShardProcessor] {
				return factory.NewShardProcessorFactory(params, processorephemeral.NewShardProcessor)
			},
		),

		// Simple way to instantiate executor if only one namespace is used
		// executorclient.ModuleWithNamespace[*processor.ShardProcessor](names.FixedNamespace),
		// executorclient.ModuleWithNamespace[*processorephemeral.ShardProcessor](names.EphemeralNamespace),

		// Instantiate executors for multiple namespaces
		executors.Module(names.FixedNamespace, names.EphemeralNamespace, names.ExternalAssignmentNamespace),

		processorephemeral.ShardCreatorModule([]string{names.EphemeralNamespace}),

		fx.Invoke(registerExecutorLifecycle),
	)
}

type lifecycleParams struct {
	fx.In
	Lifecycle          fx.Lifecycle
	Dispatcher         *yarpc.Dispatcher
	FixedExecutors     []executorclient.Executor[*processor.ShardProcessor]          `group:"executor-fixed-proc"`
	EphemeralExecutors []executorclient.Executor[*processorephemeral.ShardProcessor] `group:"executor-ephemeral-proc"`
}

func registerExecutorLifecycle(params lifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := params.Dispatcher.Start(); err != nil {
				return err
			}
			for _, executor := range params.FixedExecutors {
				executor.Start(ctx)
			}
			for _, executor := range params.EphemeralExecutors {
				executor.Start(ctx)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			for _, executor := range params.FixedExecutors {
				executor.Stop()
			}
			for _, executor := range params.EphemeralExecutors {
				executor.Stop()
			}
			return params.Dispatcher.Stop()
		},
	})
}
