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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/leader/leaderstore"
)

const (
	_testHost      = "localhost"
	_testNamespace = "test-namespace"
)

var (
	_testLeaderPeriod           = time.Minute
	_testMaxRandomDelay         = time.Second
	_testFailedElectionCooldown = 10 * time.Second
)

func TestElector_Run(t *testing.T) {
	goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSource()

	election := leaderstore.NewMockElection(ctrl)
	election.EXPECT().Campaign(gomock.Any(), _testHost).Return(nil)
	election.EXPECT().Done().Return(make(chan struct{}))

	store := leaderstore.NewMockStore(ctrl)
	store.EXPECT().CreateElection(gomock.Any(), _testNamespace).Return(election, nil)

	factory := NewElectionFactory(FactoryParams{
		HostName: _testHost,
		Cfg: config.LeaderElection{
			Election: config.Election{
				LeaderPeriod:           _testLeaderPeriod,
				MaxRandomDelay:         _testMaxRandomDelay,
				FailedElectionCooldown: _testFailedElectionCooldown,
			},
		},
		Store:  store,
		Logger: logger,
		Clock:  timeSource,
	})

	elector, err := factory.CreateElector(context.Background(), _testNamespace)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// Wait until run will stop on timer
		timeSource.BlockUntil(1)
		// Advance the time to kick in the election.
		timeSource.Advance(_testMaxRandomDelay)
	}()

	leaderChan := elector.Run(ctx)
	assert.True(t, <-leaderChan)
}

func TestElector_Run_Resign(t *testing.T) {
	goleak.VerifyNone(t)
	t.Run("context canceled", func(t *testing.T) {
		leaderChan, p := prepareRun(t)
		p.cancel()
		assert.False(t, <-leaderChan)
	})
	t.Run("session expired", func(t *testing.T) {
		leaderChan, p := prepareRun(t)
		defer p.cancel()
		close(p.electionCh)
		assert.False(t, <-leaderChan)
	})
	t.Run("leader resign", func(t *testing.T) {
		leaderChan, p := prepareRun(t)
		defer p.cancel()
		// We should be blocked on the timer.
		p.timeSource.BlockUntil(1)
		p.election.EXPECT().Resign(gomock.Any()).Return(nil)
		p.timeSource.Advance(_testLeaderPeriod + 1)
		p.timeSource.BlockUntil(1)
		assert.False(t, <-leaderChan)
	})
}

type runParams struct {
	ctx    context.Context
	cancel context.CancelFunc

	timeSource clock.MockedTimeSource
	electionCh chan struct{}
	election   *leaderstore.MockElection
}

func prepareRun(t *testing.T) (<-chan bool, runParams) {
	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSource()

	electionCh := make(chan struct{})

	election := leaderstore.NewMockElection(ctrl)
	election.EXPECT().Campaign(gomock.Any(), _testHost).Return(nil)
	election.EXPECT().Done().Return(electionCh)

	store := leaderstore.NewMockStore(ctrl)
	store.EXPECT().CreateElection(gomock.Any(), _testNamespace).Return(election, nil)

	factory := NewElectionFactory(FactoryParams{
		HostName: _testHost,
		Cfg: config.LeaderElection{
			Election: config.Election{
				LeaderPeriod:           _testLeaderPeriod,
				MaxRandomDelay:         _testMaxRandomDelay,
				FailedElectionCooldown: _testFailedElectionCooldown,
			},
		},
		Store:  store,
		Logger: logger,
		Clock:  timeSource,
	})

	elector, err := factory.CreateElector(context.Background(), _testNamespace)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Wait until run will stop on timer
		timeSource.BlockUntil(1)
		// Advance the time to kick in the election.
		timeSource.Advance(_testMaxRandomDelay)
	}()

	leaderChan := elector.Run(ctx)
	assert.True(t, <-leaderChan)
	return leaderChan, runParams{
		ctx:        ctx,
		cancel:     cancel,
		timeSource: timeSource,
		electionCh: electionCh,
		election:   election,
	}
}
