package spectatorclient

import (
	"context"
	"fmt"

	"github.com/uber-go/tally"
	"go.uber.org/fx"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go . Spectator

type Spectators struct {
	spectators map[string]Spectator
}

func (s *Spectators) ForNamespace(namespace string) (Spectator, error) {
	spectator, ok := s.spectators[namespace]
	if !ok {
		return nil, fmt.Errorf("spectator not found for namespace %s", namespace)
	}
	return spectator, nil
}

func (s *Spectators) Start(ctx context.Context) error {
	for namespace, spectator := range s.spectators {
		if err := spectator.Start(ctx); err != nil {
			return fmt.Errorf("start spectator for namespace %s: %w", namespace, err)
		}
	}
	return nil
}

func (s *Spectators) Stop() {
	for _, spectator := range s.spectators {
		spectator.Stop()
	}
}

func NewSpectators(params Params) (*Spectators, error) {
	spectators := make(map[string]Spectator)
	for _, namespace := range params.Config.Namespaces {
		spectator, err := NewSpectatorWithNamespace(params, namespace.Namespace)
		if err != nil {
			return nil, fmt.Errorf("create spectator for namespace %s: %w", namespace.Namespace, err)
		}

		spectators[namespace.Namespace] = spectator
	}
	return &Spectators{spectators: spectators}, nil
}

type Spectator interface {
	Start(ctx context.Context) error
	Stop()

	// GetShardOwner returns the owner of a shard
	GetShardOwner(ctx context.Context, shardKey string) (*ShardOwner, error)
}

type Params struct {
	fx.In

	Client       sharddistributor.Client
	MetricsScope tally.Scope
	Logger       log.Logger
	Config       clientcommon.Config
	TimeSource   clock.TimeSource
}

// NewSpectatorWithNamespace creates a spectator for a specific namespace
func NewSpectatorWithNamespace(params Params, namespace string) (Spectator, error) {
	return newSpectatorImpl(params, namespace)
}

// NewSpectator creates a spectator for the single namespace in config
func NewSpectator(params Params) (Spectator, error) {
	cfg, err := params.Config.GetSingleConfig()
	if err != nil {
		return nil, err
	}
	return newSpectatorImpl(params, cfg.Namespace)
}

func newSpectatorImpl(params Params, namespace string) (Spectator, error) {
	// Get config for the specified namespace
	namespaceConfig, err := params.Config.GetConfigForNamespace(namespace)
	if err != nil {
		return nil, fmt.Errorf("get config for namespace %s: %w", namespace, err)
	}

	return newSpectatorWithConfig(params, namespaceConfig)
}

func newSpectatorWithConfig(params Params, namespaceConfig *clientcommon.NamespaceConfig) (Spectator, error) {
	impl := &spectatorImpl{
		namespace:    namespaceConfig.Namespace,
		config:       *namespaceConfig,
		client:       params.Client,
		logger:       params.Logger,
		scope:        params.MetricsScope,
		timeSource:   params.TimeSource,
		firstStateCh: make(chan struct{}),
	}

	return impl, nil
}

// Module creates a spectator module using auto-selection (single namespace only)
func Module() fx.Option {
	return fx.Module("shard-distributor-spectator-client",
		fx.Provide(NewSpectators),
		fx.Invoke(func(spectators *Spectators, lc fx.Lifecycle) {
			lc.Append(fx.StartStopHook(spectators.Start, spectators.Stop))
		}),
	)
}
