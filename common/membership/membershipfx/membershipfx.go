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

package membershipfx

import (
	"fmt"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
)

// Module provides membership components for fx app.
var Module = fx.Module("membership", fx.Provide(buildMembership))

type buildMembershipParams struct {
	fx.In

	Clock         clock.TimeSource
	RPCFactory    rpc.Factory
	PeerProvider  membership.PeerProvider
	Logger        log.Logger
	MetricsClient metrics.Client
	Lifecycle     fx.Lifecycle
}

type buildMembershipResult struct {
	fx.Out

	Rings    map[string]membership.SingleProvider
	Resolver membership.Resolver
}

func buildMembership(params buildMembershipParams) (buildMembershipResult, error) {
	rings := make(map[string]membership.SingleProvider)
	for _, s := range service.ListWithRing {
		rings[s] = membership.NewHashring(s, params.PeerProvider, params.Clock, params.Logger, params.MetricsClient.Scope(metrics.HashringScope))
	}

	resolver, err := membership.NewResolver(
		params.PeerProvider,
		params.MetricsClient,
		params.Logger,
		rings,
	)
	if err != nil {
		return buildMembershipResult{}, fmt.Errorf("create resolver: %w", err)
	}

	params.Lifecycle.Append(fx.StartStopHook(startResolver(resolver, params.RPCFactory), resolver.Stop))

	return buildMembershipResult{
		Rings:    rings,
		Resolver: resolver,
	}, nil
}

func startResolver(resolver membership.Resolver, rpcFactory rpc.Factory) func() error {
	return func() error {
		err := rpcFactory.Start(resolver)
		if err != nil {
			return fmt.Errorf("start rpc factory: %w", err)
		}
		resolver.Start()
		return nil
	}
}
