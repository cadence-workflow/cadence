package ringpopfx

import (
	"fmt"

	uconfig "go.uber.org/config"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/peerprovider/ringpopprovider"
	ringpopconfig "github.com/uber/cadence/common/peerprovider/ringpopprovider/config"
	"github.com/uber/cadence/common/rpc"
)

// Module provides a peer resolver based on ringpop for fx app.
var Module = fx.Module("ringpop", fx.Provide(New))

// Params are the dependencies for creating ringpop peer provider.
type Params struct {
	fx.In

	ServiceFullName string `name:"service-full-name"`
	ConfigProvider  uconfig.Provider
	ServiceConfig   config.Service
	Logger          log.Logger
	RPCFactory      rpc.Factory
	Lifecycle       fx.Lifecycle
}

// Result contains the peer provider provided by this module.
type Result struct {
	fx.Out

	PeerProvider membership.PeerProvider
}

// New creates ringpop peer provider via dependency injection.
func New(params Params) (Result, error) {
	var ringpopCfg ringpopconfig.Config
	if err := params.ConfigProvider.Get("ringpop").Populate(&ringpopCfg); err != nil {
		// This should rarely happen - Populate succeeds even for missing/empty config
		return Result{}, fmt.Errorf("failed to decode ringpop configuration: %w", err)
	}

	// Check if config is empty (all fields are zero values)
	// Empty YAML sections don't cause Populate errors, so we check if config is empty
	if ringpopCfg.IsEmpty() {
		// Config is empty - return successfully with no provider
		// AppParams will fail later due to missing required PeerProvider dependency
		params.Logger.Info("Ringpop configuration is empty, skipping ringpop initialization")
		return Result{}, nil
	}

	provider, err := ringpopprovider.New(params.ServiceFullName, &ringpopCfg, params.RPCFactory.GetTChannel(), membership.PortMap{
		membership.PortGRPC:     params.ServiceConfig.RPC.GRPCPort,
		membership.PortTchannel: params.ServiceConfig.RPC.Port,
	}, params.Logger)
	if err != nil {
		return Result{}, err
	}
	return Result{PeerProvider: provider}, nil
}
