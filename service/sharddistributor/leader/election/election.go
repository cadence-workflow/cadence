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

package election

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/leader/leaderstore"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=election_mock.go Factory,Elector

var resignError = fmt.Errorf("self-resigned")

// Module provides election factory for fx app.
var Module = fx.Module(
	"leader-election",
	fx.Provide(NewElectionFactory),
)

// Elector handles leader election for a specific namespace
type Elector interface {
	Run(ctx context.Context) <-chan bool
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
		logger:    f.logger.WithTags(tag.ComponentLeaderElection, tag.Namespace(namespace)),
		cfg:       f.cfg,
		clock:     f.clock,
		hostname:  f.hostname,
	}, nil
}

// Run starts the leader election process it returns a channel that will return the value if the current instance becomes the leader or resigns from leadership.
func (e *elector) Run(ctx context.Context) <-chan bool {
	leaderCh := make(chan bool, 1)

	go func() {
		defer close(leaderCh)

		for {
			select {
			case <-ctx.Done():
				e.logger.Info("Context cancelled, stopping election")
				return

			default:
				if err := e.runElection(ctx, leaderCh); err != nil {
					// Self resign, immediately retry, otherwise, wait
					if !errors.Is(err, resignError) {
						e.logger.Error("Error in election, retrying", tag.Error(err))
						e.clock.Sleep(e.cfg.FailedElectionCooldown)
					}
				}
			}
		}
	}()

	return leaderCh
}

// runElection runs a single election attempt
func (e *elector) runElection(ctx context.Context, leaderCh chan<- bool) error {
	// Add random delay before campaigning to spread load across instances
	delay := time.Duration(rand.Intn(int(e.cfg.MaxRandomDelay)))

	e.logger.Debug("Adding random delay before campaigning") //tag.Duration("delay", delay)

	select {
	case <-e.clock.After(delay):
		// Continue after delay
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during pre-campaign delay")
	}

	election, err := e.store.CreateElection(ctx, e.namespace)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Campaign to become leader
	if err := election.Campaign(ctx, e.hostname); err != nil {
		return fmt.Errorf("failed to campaign: %w", err)
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
		leaderCh <- false
		return nil

	case <-election.Done():
		e.logger.Info("Session expired while leader")
		leaderCh <- false
		return fmt.Errorf("session expired")

	case <-leaderTimer.Chan():
		e.logger.Info("Leadership period ended, voluntarily resigning")

		// Attempt to resign leadership
		resignCtx, cancel := clock.WithTimeout(ctx, e.clock, 3*time.Second)
		defer cancel()

		err := election.Resign(resignCtx)
		if err != nil {
			e.logger.Error("Failed to resign leadership", tag.Error(err))

		}

		leaderCh <- false
		return resignError
	}
}
