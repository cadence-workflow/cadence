package spectators

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/service/sharddistributor/canary/config"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

func Module(fixedNamespace, ephemeralNamespace string) fx.Option {
	return fx.Module("Spectators",
		fx.Provide(func(p Params) ([]spectatorclient.Spectator, error) {
			return NewSpectators(p, fixedNamespace, ephemeralNamespace)
		}),

		fx.Invoke(func(spectators []spectatorclient.Spectator, lc fx.Lifecycle) {
			for _, sp := range spectators {
				lc.Append(fx.StartStopHook(sp.Start, sp.Stop))
			}
		}),
	)
}

type Params struct {
	fx.In

	Config          config.Config
	SpectatorParams spectatorclient.Params
}

func NewSpectators(p Params, fixedNamespace, ephemeralNamespace string) ([]spectatorclient.Spectator, error) {
	fixed, err := NewSpectatorsWithNamespace(p.SpectatorParams, fixedNamespace, p.Config.Canary.NumFixedSpectators)
	if err != nil {
		return nil, err
	}

	ephemeral, err := NewSpectatorsWithNamespace(p.SpectatorParams, ephemeralNamespace, p.Config.Canary.NumEphemeralSpectators)
	if err != nil {
		return nil, err
	}

	return append(fixed, ephemeral...), nil
}

func NewSpectatorsWithNamespace(params spectatorclient.Params, namespace string, numSpectators int) ([]spectatorclient.Spectator, error) {
	var spectators []spectatorclient.Spectator

	if numSpectators <= 0 {
		numSpectators = 1
	}

	for i := 0; i < numSpectators; i++ {
		spectator, err := spectatorclient.NewSpectatorWithNamespace(params, namespace)
		if err != nil {
			return nil, err
		}
		spectators = append(spectators, spectator)
	}

	return spectators, nil
}
