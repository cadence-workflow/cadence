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

package metricsfx

import (
	"github.com/uber-go/tally"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

// Module provides metrics client for fx application.
var Module = fx.Module("metricsfx",
	fx.Provide(buildClient))

// ModuleForExternalScope provides metrics client for fx application when tally.Scope is created outside.
var ModuleForExternalScope = fx.Module("metricsfx",
	fx.Provide(func(params serviceIdxParams) metrics.ServiceIdx {
		return service.GetMetricsServiceIdx(params.ServiceFullName, params.Logger)
	}),
	fx.Provide(buildClientFromTally))

type clientParams struct {
	fx.In

	Logger          log.Logger
	ServiceFullName string `name:"service-full-name"`
	SvcCfg          config.Service
}

type clientResult struct {
	fx.Out

	Scope  tally.Scope
	Client metrics.Client
}

func buildClient(params clientParams) clientResult {
	scope := params.SvcCfg.Metrics.NewScope(params.Logger, params.ServiceFullName)
	return clientResult{
		Scope:  scope,
		Client: buildClientFromTally(scope, service.GetMetricsServiceIdx(params.ServiceFullName, params.Logger)),
	}
}

type serviceIdxParams struct {
	fx.In

	Logger          log.Logger
	ServiceFullName string `name:"service-full-name"`
}

func buildClientFromTally(scope tally.Scope, serviceID metrics.ServiceIdx) metrics.Client {
	return metrics.NewClient(scope, serviceID)
}
