package membershipfx

import (
	"fmt"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

// Module provides membership components for fx app.
var Module = fx.Module("membership", fx.Provide(buildMembership))

type buildMembershipParams struct {
	fx.In

	Clock         clock.TimeSource
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
		rings,
	)
	if err != nil {
		return buildMembershipResult{}, fmt.Errorf("create resolver: %w", err)
	}

	params.Lifecycle.Append(fx.StartStopHook(resolver.Start, resolver.Stop))

	return buildMembershipResult{
		Rings:    rings,
		Resolver: resolver,
	}, nil
}
