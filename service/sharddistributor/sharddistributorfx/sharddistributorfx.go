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

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/sharddistributor/handler"
	"github.com/uber/cadence/service/sharddistributor/leader/election"
	"github.com/uber/cadence/service/sharddistributor/leader/namespace"
	"github.com/uber/cadence/service/sharddistributor/leader/process"
	"github.com/uber/cadence/service/sharddistributor/wrappers/grpc"
	"github.com/uber/cadence/service/sharddistributor/wrappers/metered"
)

var Module = fx.Module("sharddistributor",
	namespace.Module,
	election.Module,
	process.Module,
	fx.Invoke(registerHandlers))

type registerHandlersParams struct {
	fx.In

	Logger            log.Logger
	MetricsClient     metrics.Client
	RPCFactory        rpc.Factory
	DynamicCollection *dynamicconfig.Collection

	MembershipRings map[string]membership.SingleProvider

	Lifecycle fx.Lifecycle
}

func registerHandlers(params registerHandlersParams) error {
	dispatcher := params.RPCFactory.GetDispatcher()

	matchingRing := params.MembershipRings[service.Matching]
	historyRing := params.MembershipRings[service.History]

	rawHandler := handler.NewHandler(params.Logger, params.MetricsClient, matchingRing, historyRing)
	wrappedHandler := metered.NewMetricsHandler(rawHandler, params.Logger, params.MetricsClient)

	grpcHandler := grpc.NewGRPCHandler(wrappedHandler)
	grpcHandler.Register(dispatcher)

	params.Lifecycle.Append(fx.StartStopHook(wrappedHandler.Start, wrappedHandler.Stop))

	return nil
}
