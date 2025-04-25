// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
