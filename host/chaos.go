// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package host

//go:generate mockgen -package $GOPACKAGE -destination host_controller_mock.go -self_package github.com/uber/cadence/host github.com/uber/cadence/host HostController

import (
	"context"
	"math/rand"
	"time"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/history/simulation"
)

// minSleepMs is a hardcoded safety floor for the tick interval. It prevents
// tight loops when IntervalMs is configured very small.
const minSleepMs = 100

// HostController is implemented by anything that can stop and start individual
// hosts within a cluster. ChaosMonkey drives chaos against any HostController,
// making it reusable across history, matching, and other services.
type HostController interface {
	StopHost(index int)
	StartHost(index int)
	HostIdentity(index int) string
}

// ChaosConfig controls the behaviour of ChaosMonkey.
type ChaosConfig struct {
	Enable      bool
	IntervalMs  int     // average ms between ticks; default 5000
	JitterMs    int     // jitter applied to the interval; 0 = no jitter
	MinHosts    int     // never drop below this many running hosts; default 1
	StopChance  float64 // per-tick probability [0,1] of stopping a running host; default 0.5
	StartChance float64 // per-tick probability [0,1] of starting a stopped host; default 0.5
}

// ChaosMonkey randomly stops and starts hosts managed by a HostController.
// Create one with NewChaosMonkey and call Run in a goroutine; cancel the
// context to stop it.
type ChaosMonkey struct {
	controller HostController
	cfg        ChaosConfig
	numHosts   int
	logger     log.Logger
	running    []bool
}

// NewChaosMonkey creates a ChaosMonkey. cfg is copied by value so the caller
// cannot mutate it after construction. Defaults are applied to the copy.
func NewChaosMonkey(controller HostController, cfg ChaosConfig, numHosts int, logger log.Logger) *ChaosMonkey {
	if cfg.IntervalMs <= 0 {
		cfg.IntervalMs = 5000
	}
	if cfg.MinHosts < 1 {
		cfg.MinHosts = 1
	}
	if cfg.StopChance <= 0 {
		cfg.StopChance = 0.5
	}
	if cfg.StartChance <= 0 {
		cfg.StartChance = 0.5
	}

	running := make([]bool, numHosts)
	for i := range running {
		running[i] = true
	}

	return &ChaosMonkey{
		controller: controller,
		cfg:        cfg,
		numHosts:   numHosts,
		logger:     logger,
		running:    running,
	}
}

// Run executes the chaos loop until ctx is cancelled. It is a no-op if
// cfg.Enable is false. When ctx is cancelled, all stopped hosts are restarted
// before Run returns so the cluster is left in a clean state.
func (c *ChaosMonkey) Run(ctx context.Context) {
	if !c.cfg.Enable {
		return
	}

	for {
		select {
		case <-ctx.Done():
			c.restartAll()
			return
		case <-time.After(c.jitteredSleep()):
		}
		c.tick()
	}
}

// jitteredSleep returns the duration for the next tick sleep.
func (c *ChaosMonkey) jitteredSleep() time.Duration {
	sleepMs := c.cfg.IntervalMs
	if c.cfg.JitterMs > 0 {
		sleepMs += rand.Intn(2*c.cfg.JitterMs) - c.cfg.JitterMs
	}
	if sleepMs < minSleepMs {
		sleepMs = minSleepMs
	}
	return time.Duration(sleepMs) * time.Millisecond
}

// tick performs at most one action per tick (stop or start, never both).
// This avoids rapid concurrent state changes to the cluster that can trigger
// port binding races in cadenceImpl.
//
// Action selection:
//   - When at MinHosts (canStop=false): always start a stopped host (guaranteed recovery).
//   - When only stopping is possible: stop with StopChance probability.
//   - When both are possible: stop with probability StopChance/(StopChance+StartChance),
//     start otherwise — i.e. StopChance and StartChance are relative weights.
func (c *ChaosMonkey) tick() {
	var runningIndices, stoppedIndices []int
	for i, isRunning := range c.running {
		if isRunning {
			runningIndices = append(runningIndices, i)
		} else {
			stoppedIndices = append(stoppedIndices, i)
		}
	}
	canStop := len(runningIndices) > c.cfg.MinHosts
	canStart := len(stoppedIndices) > 0

	switch {
	case !canStop && !canStart:
		// Nothing to do.
	case !canStop:
		// At minimum hosts: always start to guarantee recovery.
		c.startAction(stoppedIndices)
	case !canStart:
		// Only stopping is possible.
		if rand.Float64() < c.cfg.StopChance {
			c.stopAction(runningIndices)
		}
	default:
		// Both are possible: pick one using StopChance as relative weight.
		stopWeight := c.cfg.StopChance / (c.cfg.StopChance + c.cfg.StartChance)
		if rand.Float64() < stopWeight {
			c.stopAction(runningIndices)
		} else {
			c.startAction(stoppedIndices)
		}
	}
}

// stopAction stops a randomly chosen host from runningIndices.
func (c *ChaosMonkey) stopAction(runningIndices []int) {
	idx := runningIndices[rand.Intn(len(runningIndices))]
	hostID := c.controller.HostIdentity(idx)
	c.logger.Info("Chaos monkey: stopping host",
		tag.Dynamic("index", idx),
		tag.Dynamic("host", hostID),
	)
	c.controller.StopHost(idx)
	simulation.LogEvents(simulation.E{
		EventName: simulation.EventNameHostStopped,
		Host:      hostID,
	})
	c.running[idx] = false
}

// startAction starts a randomly chosen host from stoppedIndices.
func (c *ChaosMonkey) startAction(stoppedIndices []int) {
	idx := stoppedIndices[rand.Intn(len(stoppedIndices))]
	hostID := c.controller.HostIdentity(idx)
	c.logger.Info("Chaos monkey: starting host",
		tag.Dynamic("index", idx),
		tag.Dynamic("host", hostID),
	)
	c.controller.StartHost(idx)
	simulation.LogEvents(simulation.E{
		EventName: simulation.EventNameHostStarted,
		Host:      hostID,
	})
	c.running[idx] = true
}

// restartAll starts every stopped host. Called on context cancellation to
// leave the cluster in a clean state for workflow completion.
func (c *ChaosMonkey) restartAll() {
	for i, isRunning := range c.running {
		if !isRunning {
			hostID := c.controller.HostIdentity(i)
			c.logger.Info("Chaos monkey: restarting host before exit",
				tag.Dynamic("index", i),
				tag.Dynamic("host", hostID),
			)
			c.controller.StartHost(i)
			simulation.LogEvents(simulation.E{
				EventName: simulation.EventNameHostStarted,
				Host:      hostID,
			})
			c.running[i] = true
		}
	}
}
