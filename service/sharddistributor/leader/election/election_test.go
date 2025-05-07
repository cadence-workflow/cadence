package election

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/config"
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

	finished := make(chan struct{})
	// once test is done cleanup will be called
	election.EXPECT().Resign(gomock.Any()).DoAndReturn(func(_ context.Context) error {
		close(finished)
		return nil
	})

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

	el, err := factory.CreateElector(context.Background(), _testNamespace)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track callback executions
	onLeaderCalled := false
	onResignCalled := false

	onLeader := func(ctx context.Context) error {
		onLeaderCalled = true
		return nil
	}

	onResign := func(ctx context.Context) error {
		onResignCalled = true
		return nil
	}

	go func() {
		// Wait until run will stop on timer
		timeSource.BlockUntil(1)
		// Advance the time to kick in the election.
		timeSource.Advance(_testMaxRandomDelay)
	}()

	leaderChan := el.Run(ctx, onLeader, onResign)
	assert.True(t, <-leaderChan)
	assert.True(t, onLeaderCalled, "OnLeader callback should have been called")
	assert.False(t, onResignCalled, "OnResign callback should not have been called")
	cancel()
	<-finished
}

func TestElector_Run_Resign(t *testing.T) {
	goleak.VerifyNone(t)
	t.Run("context_canceled", func(t *testing.T) {
		leaderChan, p := prepareRun(t, nil, nil)
		p.election.EXPECT().Resign(gomock.Any()).Return(nil)
		p.cancel()
		assert.False(t, <-leaderChan)
	})
	t.Run("session_expired", func(t *testing.T) {
		leaderChan, p := prepareRun(t, nil, nil)
		p.election.EXPECT().Resign(gomock.Any()).Return(nil)
		defer p.cancel()
		close(p.electionCh)
		assert.False(t, <-leaderChan)
	})
	t.Run("leader_resign", func(t *testing.T) {
		// Verify onResign is called before resignation
		onResignCalled := false
		leaderChan, p := prepareRun(t, nil, func(ctx context.Context) error {
			onResignCalled = true
			return nil
		})
		defer p.cancel()
		// We should be blocked on the timer.
		p.timeSource.BlockUntil(1)

		p.election.EXPECT().Resign(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			assert.True(t, onResignCalled, "OnResign callback should be called before Resign")
			return nil
		})

		p.timeSource.Advance(_testLeaderPeriod + 1)
		p.timeSource.BlockUntil(1)
		assert.False(t, <-leaderChan)
	})

	t.Run("onResign error", func(t *testing.T) {
		// Set onResign to return an error
		onResignCalled := false
		resignErr := errors.New("resign error")
		onResign := func(ctx context.Context) error {
			onResignCalled = true
			return resignErr
		}

		leaderChan, p := prepareRun(t, nil, onResign)
		defer p.cancel()
		// We should be blocked on the timer.
		p.timeSource.BlockUntil(1)

		// The resign function on election should not be called if onResign returns an error
		p.timeSource.Advance(_testLeaderPeriod + 1)
		p.timeSource.BlockUntil(1)
		assert.False(t, <-leaderChan)
		assert.True(t, onResignCalled, "OnResign callback should have been called")
	})
}

type runParams struct {
	ctx        context.Context
	cancel     context.CancelFunc
	timeSource clock.MockedTimeSource
	electionCh chan struct{}
	election   *leaderstore.MockElection
	onLeader   ProcessFunc
	onResign   ProcessFunc
}

func prepareRun(t *testing.T, onLeader, onResign ProcessFunc) (<-chan bool, runParams) {
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

	// Default callbacks
	if onLeader == nil {
		onLeader = func(ctx context.Context) error {
			return nil
		}
	}

	if onResign == nil {
		onResign = func(ctx context.Context) error {
			return nil
		}
	}

	go func() {
		// Wait until run will stop on timer
		timeSource.BlockUntil(1)
		// Advance the time to kick in the election.
		timeSource.Advance(_testMaxRandomDelay)
	}()

	leaderChan := elector.Run(ctx, onLeader, onResign)
	assert.True(t, <-leaderChan)

	return leaderChan, runParams{
		ctx:        ctx,
		cancel:     cancel,
		timeSource: timeSource,
		electionCh: electionCh,
		election:   election,
		onLeader:   onLeader,
		onResign:   onResign,
	}
}

func TestOnLeader_Error(t *testing.T) {
	goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSource()

	election := leaderstore.NewMockElection(ctrl)
	election.EXPECT().Campaign(gomock.Any(), _testHost).Return(nil)
	// Expect resignation after onLeader failure
	election.EXPECT().Resign(gomock.Any()).Return(nil)

	store := leaderstore.NewMockStore(ctrl)
	store.EXPECT().CreateElection(gomock.Any(), _testNamespace).Return(election, nil)

	// Create elector directly for test control
	el := &elector{
		namespace: _testNamespace,
		store:     store,
		logger:    logger,
		cfg: config.Election{
			LeaderPeriod:           _testLeaderPeriod,
			MaxRandomDelay:         _testMaxRandomDelay,
			FailedElectionCooldown: _testFailedElectionCooldown,
		},
		clock:    timeSource,
		hostname: _testHost,
	}

	// Make onLeader return an error
	onLeaderErr := errors.New("leader error")
	onLeader := func(ctx context.Context) error {
		return onLeaderErr
	}

	go func() {
		// Wait until run will stop on timer
		timeSource.BlockUntil(1)
		// Advance the time to kick in the election.
		timeSource.Advance(_testMaxRandomDelay)
	}()

	// Run the test
	leaderCh := make(chan bool, 1)
	err := el.runElection(context.Background(), leaderCh, onLeader, func(ctx context.Context) error { return nil })

	// Error should contain our onLeader error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "onLeader")
	assert.Contains(t, err.Error(), "leader error")
}
