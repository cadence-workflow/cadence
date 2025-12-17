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

package sharddistributorfx

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/handler"
	"github.com/uber/cadence/service/sharddistributor/leader/election"
	"github.com/uber/cadence/service/sharddistributor/leader/namespace"
	"github.com/uber/cadence/service/sharddistributor/leader/process"
	"github.com/uber/cadence/service/sharddistributor/store"
	meteredStore "github.com/uber/cadence/service/sharddistributor/store/wrappers/metered"
	"github.com/uber/cadence/service/sharddistributor/wrappers/grpc"
	"github.com/uber/cadence/service/sharddistributor/wrappers/metered"
)

var Module = fx.Module("sharddistributor",
	namespace.Module,
	election.Module,
	process.Module,
	fx.Decorate(func(s store.Store, metricsClient metrics.Client, logger log.Logger, timeSource clock.TimeSource) store.Store {
		return meteredStore.NewStore(s, metricsClient, logger, timeSource)
	}),
	fx.Invoke(registerHandlers))

type registerHandlersParams struct {
	fx.In

	ShardDistributionCfg config.ShardDistribution

	Logger            log.Logger
	MetricsClient     metrics.Client
	RPCFactory        rpc.Factory
	DynamicCollection *dynamicconfig.Collection

	TimeSource clock.TimeSource
	Store      store.Store

	Lifecycle fx.Lifecycle
}

func registerHandlers(params registerHandlersParams) error {
	dispatcher := params.RPCFactory.GetDispatcher()
	cfg := config.NewConfig(params.DynamicCollection)

	rawHandler := handler.NewHandler(params.Logger, params.ShardDistributionCfg, cfg, params.Store)
	wrappedHandler := metered.NewMetricsHandler(rawHandler, params.Logger, params.MetricsClient)

	executorHandler := handler.NewExecutorHandler(params.Logger, params.Store, params.TimeSource, params.ShardDistributionCfg, cfg, params.MetricsClient)
	wrappedExecutor := metered.NewExecutorMetricsExecutor(executorHandler, params.Logger, params.MetricsClient)

	grpcHandler := grpc.NewGRPCHandler(wrappedHandler)
	grpcHandler.Register(dispatcher)

	executorGRPCHander := grpc.NewExecutorGRPCExecutor(wrappedExecutor)
	executorGRPCHander.Register(dispatcher)

	params.Lifecycle.Append(fx.StartStopHook(rawHandler.Start, rawHandler.Stop))

	return nil
}
