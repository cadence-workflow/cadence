package election

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/leaderstore"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=election_mock.go Factory,Elector

var errSelfResign = fmt.Errorf("self-resigned")

// Module provides election factory for fx app.
var Module = fx.Module(
	"leader-election",
	fx.Provide(NewElectionFactory),
)

type ProcessFunc func(ctx context.Context) error

// Elector handles leader election for a specific namespace
type Elector interface {
	Run(ctx context.Context, OnLeader, OnResign ProcessFunc) <-chan bool
}

// Factory creates elector instances
type Factory interface {
	CreateElector(ctx context.Context, namespace string) (Elector, error)
}

type electionFactory struct {
	hostname  string
	cfg       config.Election
	store     leaderstore.Store
	logger    log.Logger
	serviceID string
	clock     clock.TimeSource
}

type elector struct {
	hostname      string
	namespace     string
	store         leaderstore.Store
	logger        log.Logger
	cfg           config.Election
	leaderStarted time.Time
	clock         clock.TimeSource
}

type FactoryParams struct {
	fx.In

	HostName string `name:"hostname"`
	Cfg      config.LeaderElection
	Store    leaderstore.Store
	Logger   log.Logger
	Clock    clock.TimeSource
}

// NewElectionFactory creates a new election factory
func NewElectionFactory(p FactoryParams) Factory {
	return &electionFactory{
		cfg:      p.Cfg.Election,
		store:    p.Store,
		logger:   p.Logger,
		clock:    p.Clock,
		hostname: p.HostName,
	}
}

// CreateElector creates a new elector for the given namespace
func (f *electionFactory) CreateElector(ctx context.Context, namespace string) (Elector, error) {
	return &elector{
		namespace: namespace,
		store:     f.store,
		logger:    f.logger.WithTags(tag.ComponentLeaderElection, tag.ShardNamespace(namespace)),
		cfg:       f.cfg,
		clock:     f.clock,
		hostname:  f.hostname,
	}, nil
}

// Run starts the leader election process it returns a channel that will return the value if the current instance becomes the leader or resigns from leadership.
// OnLeader will be called once leadership is acquired. OnResign will be called once leadership is lost.
func (e *elector) Run(ctx context.Context, OnLeader, OnResign ProcessFunc) <-chan bool {
	leaderCh := make(chan bool, 1)

	go func() {
		defer close(leaderCh)

		for {
			if err := e.runElection(ctx, leaderCh, OnLeader, OnResign); err != nil {
				// Self resign, immediately retry, otherwise, wait
				if !errors.Is(err, errSelfResign) {
					e.logger.Error("Error in election, retrying", tag.Error(err))
					sleepErr := e.clock.SleepWithContext(ctx, e.cfg.FailedElectionCooldown)
					if sleepErr != nil {
						e.logger.Warn("sleep error", tag.Error(sleepErr))
					}
				}
			}
			if ctx.Err() != nil {
				break
			}
		}
	}()

	return leaderCh
}

// runElection runs a single election attempt
func (e *elector) runElection(ctx context.Context, leaderCh chan<- bool, OnLeader, OnResign ProcessFunc) (err error) {
	// Add random delay before campaigning to spread load across instances
	delay := time.Duration(rand.Intn(int(e.cfg.MaxRandomDelay)))

	e.logger.Debug("Adding random delay before campaigning", tag.ElectionDelay(delay))

	select {
	case <-e.clock.After(delay):
		// Continue after delay
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during pre-campaign delay: %w", ctx.Err())
	}

	election, err := e.store.CreateElection(ctx, e.namespace)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer func() {
		resignErr := e.resign(election, OnResign)
		if resignErr != nil {
			if err == nil {
				err = resignErr
			} else {
				// Something already went wrong, so the process is most likely not started or stopped.
				// It should be safe to report leadership lost.
				e.logger.Error("Error resigning leader", tag.Error(resignErr))
			}
		}

		// We are no longer a leader and OnResign is called - notify the manager that leader elected process
		leaderCh <- false
	}()

	// Campaign to become leader
	if err := election.Campaign(ctx, e.hostname); err != nil {
		return fmt.Errorf("failed to campaign: %w", err)
	}

	err = OnLeader(ctx)
	if err != nil {
		return fmt.Errorf("onLeader: %w", err)
	}

	// Successfully became leader
	e.leaderStarted = e.clock.Now()
	leaderCh <- true

	e.logger.Info("Became leader")

	// Start a timer to voluntarily resign after the leadership period
	leaderTimer := e.clock.NewTimer(e.cfg.LeaderPeriod)
	defer leaderTimer.Stop()

	// Watch for session expiration, context cancellation, or timer expiration
	select {
	case <-ctx.Done():
		e.logger.Info("Context cancelled while leader")
		return nil

	case <-election.Done():
		e.logger.Info("Session expired while leader")
		return fmt.Errorf("session expired")

	case <-leaderTimer.Chan():
		e.logger.Info("Leadership period ended, voluntarily resigning")

		return errSelfResign
	}
}

func (e *elector) resign(election leaderstore.Election, onResign ProcessFunc) error {
	ctx, cancel := e.clock.ContextWithTimeout(context.Background(), 3*time.Second) // TODO: this should be configurable.
	defer cancel()

	// first - ensure that OnResign is called.
	// that means that there can be a time frame between previos owner resigned and a new leader elected when leader processes are not running.
	resignErr := onResign(ctx)
	if resignErr != nil {
		return fmt.Errorf("OnResign: %w", resignErr)
	}

	// When the leader process is stopped it is safe to resign and pass leadership to other instances.
	err := election.Resign(ctx)
	if err != nil {
		return fmt.Errorf("election resign: %w", err)
	}

	return nil
}
