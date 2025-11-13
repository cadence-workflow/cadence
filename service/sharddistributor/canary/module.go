package canary

import (
	"go.uber.org/fx"

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

		processorephemeral.ShardCreatorModule([]string{names.EphemeralNamespace, names.ExternalAssignmentNamespace, "test-local-passthrough-shadow"}),
	)
}
