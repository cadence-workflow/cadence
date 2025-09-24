package canary

import (
	"go.uber.org/fx"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/service/sharddistributor/canary/factory"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/canary/sharddistributorclient"
	"github.com/uber/cadence/service/sharddistributor/executorclient"
)

func Module(fixedNamespace, ephemeralNamespace, sharddistributorServiceName string) fx.Option {
	return fx.Module("shard-distributor-canary", opts(fixedNamespace, ephemeralNamespace, sharddistributorServiceName))
}

func opts(fixedNamespace, ephemeralNamespace, sharddistributorServiceName string) fx.Option {
	return fx.Options(
		fx.Provide(sharddistributorv1.NewFxShardDistributorExecutorAPIYARPCClient(sharddistributorServiceName)),
		fx.Provide(sharddistributorv1.NewFxShardDistributorAPIYARPCClient(sharddistributorServiceName)),

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

		executorclient.ModuleWithNamespace[*processor.ShardProcessor](fixedNamespace),
		executorclient.ModuleWithNamespace[*processorephemeral.ShardProcessor](ephemeralNamespace),

		processorephemeral.ShardCreatorModule(ephemeralNamespace),
	)
}
