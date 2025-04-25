package ringpopfx

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/peerprovider/ringpopprovider"
	"github.com/uber/cadence/common/rpc"
)

// Module provides a peer resolver based on ringpop for fx app.
var Module = fx.Module("ringpop", fx.Provide(buildRingpopProvider))

type Params struct {
	fx.In

	Service       string `name:"service"`
	Config        config.Config
	ServiceConfig config.Service
	Logger        log.Logger
	RPCFactory    rpc.Factory
	Lifecycle     fx.Lifecycle
}

func buildRingpopProvider(params Params) (membership.PeerProvider, error) {
	provider, err := ringpopprovider.New(params.Service, &params.Config.Ringpop, params.RPCFactory.GetTChannel(), membership.PortMap{
		membership.PortGRPC:     params.ServiceConfig.RPC.GRPCPort,
		membership.PortTchannel: params.ServiceConfig.RPC.Port,
	}, params.Logger)
	if err != nil {
		return nil, err
	}
	params.Lifecycle.Append(fx.StartStopHook(provider.Start, provider.Stop))
	return provider, nil
}
