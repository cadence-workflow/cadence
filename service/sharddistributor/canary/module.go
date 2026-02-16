package canary

import (
	"go.uber.org/fx"
	"go.uber.org/yarpc"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/service/sharddistributor/canary/config"
	"github.com/uber/cadence/service/sharddistributor/canary/executors"
	"github.com/uber/cadence/service/sharddistributor/canary/factory"
	"github.com/uber/cadence/service/sharddistributor/canary/handler"
	"github.com/uber/cadence/service/sharddistributor/canary/pinger"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/canary/sharddistributorclient"
	"github.com/uber/cadence/service/sharddistributor/canary/sharddistributorexecutorclient"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

type NamespacesNames struct {
	fx.In
	FixedNamespace              string
	EphemeralNamespace          string
	ExternalAssignmentNamespace string
	SharddistributorServiceName string

	Config config.Config
}

func Module(namespacesNames NamespacesNames) fx.Option {
	return fx.Module("shard-distributor-canary", opts(namespacesNames))
}

func opts(names NamespacesNames) fx.Option {
	return fx.Options(
		fx.Supply(names.Config),

		fx.Provide(sharddistributorv1.NewFxShardDistributorExecutorAPIYARPCClient(names.SharddistributorServiceName)),
		fx.Provide(sharddistributorv1.NewFxShardDistributorAPIYARPCClient(names.SharddistributorServiceName)),

		fx.Provide(sharddistributorclient.NewShardDistributorClient),
		fx.Provide(sharddistributorexecutorclient.NewShardDistributorExecutorClient),

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

		spectatorclient.Module(),
		fx.Provide(spectatorclient.NewSpectatorPeerChooser),
		fx.Invoke(func(chooser spectatorclient.SpectatorPeerChooserInterface, lc fx.Lifecycle) {
			lc.Append(fx.StartStopHook(chooser.Start, chooser.Stop))
		}),

		// Create canary client using the dispatcher's client config
		fx.Provide(func(dispatcher *yarpc.Dispatcher) sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient {
			config := dispatcher.ClientConfig("shard-distributor-canary")
			return sharddistributorv1.NewShardDistributorExecutorCanaryAPIYARPCClient(config)
		}),

		fx.Provide(func(params pinger.Params) *pinger.Pinger {
			return pinger.NewPinger(params, names.FixedNamespace, 32)
		}),
		fx.Invoke(func(p *pinger.Pinger, lc fx.Lifecycle) {
			lc.Append(fx.StartStopHook(p.Start, p.Stop))
		}),

		// Register canary ping handler to receive ping requests from other executors
		fx.Provide(handler.NewPingHandler),
		fx.Provide(fx.Annotate(
			func(h *handler.PingHandler) sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCServer {
				return h
			},
		)),
		fx.Provide(sharddistributorv1.NewFxShardDistributorExecutorCanaryAPIYARPCProcedures()),

		// There is a circular dependency between the spectator client and the peer chooser, since
		// the yarpc dispatcher needs the peer chooser and the peer chooser needs the spectators, which needs the yarpc dispatcher.
		// To break the circular dependency, we set the spectators on the peer chooser here.
		fx.Invoke(func(chooser spectatorclient.SpectatorPeerChooserInterface, spectators *spectatorclient.Spectators) {
			chooser.SetSpectators(spectators)
		}),
	)
}
