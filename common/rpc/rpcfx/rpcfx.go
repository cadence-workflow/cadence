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

package rpcfx

import (
	"fmt"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
)

// Module provides rpc.Params and rpc.Factory for fx application.
var Module = fx.Module("rpcfx",
	fx.Provide(paramsBuilder),
	fx.Provide(buildFactory),
)

type paramsBuilderParams struct {
	fx.In

	ServiceFullName   string `name:"service-full-name"`
	Cfg               config.Config
	Logger            log.Logger
	DynamicCollection *dynamicconfig.Collection
	MetricsClient     metrics.Client
}

func paramsBuilder(p paramsBuilderParams) (rpc.Params, error) {
	res, err := rpc.NewParams(p.ServiceFullName, &p.Cfg, p.DynamicCollection, p.Logger, p.MetricsClient)
	if err != nil {
		return rpc.Params{}, fmt.Errorf("create rpc params: %w", err)
	}
	return res, nil
}

type factoryParams struct {
	fx.In

	Logger    log.Logger
	RPCParams rpc.Params

	Lifecycle fx.Lifecycle
}

func buildFactory(p factoryParams) rpc.Factory {
	res := rpc.NewFactory(p.Logger, p.RPCParams)
	p.Lifecycle.Append(fx.StartStopHook(startDispatcher(res), rpcStopper(res)))
	return res
}

func startDispatcher(f rpc.Factory) func() error {
	return func() error {
		return f.GetDispatcher().Start()
	}
}

func rpcStopper(factory rpc.Factory) func() error {
	return func() error {
		err := factory.GetDispatcher().Stop()
		if err != nil {
			return fmt.Errorf("dispatcher stop: %w", err)
		}
		err = factory.Stop()
		if err != nil {
			return fmt.Errorf("factory stop: %w", err)
		}
		return nil
	}
}
