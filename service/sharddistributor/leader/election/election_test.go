package election

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/process"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_testHost = "localhost"
)

var (
	_testNamespace = config.Namespace{Name: "test-namespace"}
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

	election := store.NewMockElection(ctrl)
	election.EXPECT().Campaign(gomock.Any(), _testHost).Return(nil)
	election.EXPECT().Done().Return(make(chan struct{}))

	finished := make(chan struct{})
	// once test is done cleanup will be called
	election.EXPECT().Resign(gomock.Any()).DoAndReturn(func(_ context.Context) error {
		close(finished)
		return nil
	})

	leaderStore := store.NewMockElector(ctrl)
	leaderStore.EXPECT().CreateElection(gomock.Any(), _testNamespace.Name).Return(election, nil)

	shardStore := store.NewMockStore(ctrl)

	processFactory := process.NewMockFactory(ctrl)
	processRunner := process.NewMockProcessor(ctrl)
	processFactory.EXPECT().CreateProcessor(_testNamespace, shardStore, election).Return(processRunner)

	factory := NewElectionFactory(FactoryParams{
		HostName: _testHost,
		Cfg: config.ShardDistribution{
			Election: config.Election{
				LeaderPeriod:           _testLeaderPeriod,
				MaxRandomDelay:         _testMaxRandomDelay,
				FailedElectionCooldown: _testFailedElectionCooldown,
			},
		},
		LeaderStore:    leaderStore,
		Store:          shardStore,
		Logger:         logger,
		Clock:          timeSource,
		ProcessFactory: processFactory,
	})

	el, err := factory.CreateElector(context.Background(), _testNamespace)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track callback executions
	onLeaderCalled := false
	onResignCalled := false

	processRunner.EXPECT().Run(gomock.Any()).DoAndReturn(func(_ context.Context) error {
		onLeaderCalled = true
		return nil
	})

	processRunner.EXPECT().Terminate(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		onResignCalled = true
		return nil
	})

	go func() {
		// Wait until run will stop on timer
		timeSource.BlockUntil(1)
		// Advance the time to kick in the election.
		timeSource.Advance(_testMaxRandomDelay)
	}()

	leaderChan := el.Run(ctx)
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
		// Wait for the goroutine to exit
		for range leaderChan {
		}
	})
	t.Run("session_expired", func(t *testing.T) {
		leaderChan, p := prepareRun(t, nil, nil)
		p.election.EXPECT().Resign(gomock.Any()).Return(nil)
		close(p.electionCh)
		assert.False(t, <-leaderChan)
		p.cancel()
		// Wait for the goroutine to exit
		for range leaderChan {
		}
	})
	t.Run("leader_resign", func(t *testing.T) {
		// Verify onResign is called before resignation
		onResignCalled := false
		leaderChan, p := prepareRun(t, nil, func(ctx context.Context) error {
			onResignCalled = true
			return nil
		})

		// We should be blocked on the timer.
		p.timeSource.BlockUntil(1)

		p.election.EXPECT().Resign(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			assert.True(t, onResignCalled, "OnResign callback should be called before Resign")
			return nil
		})

		p.timeSource.Advance(_testLeaderPeriod + 1)
		p.timeSource.BlockUntil(1)
		assert.False(t, <-leaderChan)
		p.cancel()
		// Wait for the goroutine to exit
		for range leaderChan {
		}
	})

	t.Run("onResign_error", func(t *testing.T) {
		// Set onResign to return an error
		onResignCalled := false
		resignErr := errors.New("resign error")
		onResign := func(ctx context.Context) error {
			onResignCalled = true
			return resignErr
		}

		leaderChan, p := prepareRun(t, nil, onResign)
		p.election.EXPECT().Resign(gomock.Any()).Return(nil)
		// We should be blocked on the timer.
		p.timeSource.BlockUntil(1)

		// The resign function on election should not be called if onResign returns an error
		p.timeSource.Advance(_testLeaderPeriod + 1)
		p.timeSource.BlockUntil(1)
		assert.False(t, <-leaderChan)
		assert.True(t, onResignCalled, "OnResign callback should have been called")

		p.cancel()
		// Wait for the goroutine to exit
		for range leaderChan {
		}
	})
	t.Run("OnResign_and_resign_error", func(t *testing.T) {
		// Set onResign to return an error
		onResignCalled := false
		resignErr := errors.New("resign error")
		onResign := func(ctx context.Context) error {
			onResignCalled = true
			return resignErr
		}

		leaderChan, p := prepareRun(t, nil, onResign)
		p.election.EXPECT().Resign(gomock.Any()).Return(fmt.Errorf("failed to resign"))
		// We should be blocked on the timer.
		p.timeSource.BlockUntil(1)

		// The resign function on election should not be called if onResign returns an error
		p.timeSource.Advance(_testLeaderPeriod + 1)
		p.timeSource.BlockUntil(1)
		assert.False(t, <-leaderChan)
		assert.True(t, onResignCalled, "OnResign callback should have been called")

		p.cancel()
		// Wait for the goroutine to exit
		for range leaderChan {
		}
	})

}

type runParams struct {
	ctx        context.Context
	cancel     context.CancelFunc
	timeSource clock.MockedTimeSource
	electionCh chan struct{}
	election   *store.MockElection
	onLeader   ProcessFunc
	onResign   ProcessFunc
}

func prepareRun(t *testing.T, onLeader, onResign ProcessFunc) (<-chan bool, runParams) {
	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSource()

	electionCh := make(chan struct{})

	election := store.NewMockElection(ctrl)
	election.EXPECT().Campaign(gomock.Any(), _testHost).Return(nil)
	election.EXPECT().Done().Return(electionCh)

	leaderStore := store.NewMockElector(ctrl)
	leaderStore.EXPECT().CreateElection(gomock.Any(), _testNamespace.Name).Return(election, nil)

	shardStore := store.NewMockStore(ctrl)

	processFactory := process.NewMockFactory(ctrl)
	processRunner := process.NewMockProcessor(ctrl)
	processFactory.EXPECT().CreateProcessor(_testNamespace, shardStore, election).Return(processRunner)

	factory := NewElectionFactory(FactoryParams{
		HostName: _testHost,
		Cfg: config.ShardDistribution{
			Election: config.Election{
				LeaderPeriod:           _testLeaderPeriod,
				MaxRandomDelay:         _testMaxRandomDelay,
				FailedElectionCooldown: _testFailedElectionCooldown,
			},
		},
		LeaderStore:    leaderStore,
		Store:          shardStore,
		Logger:         logger,
		Clock:          timeSource,
		ProcessFactory: processFactory,
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

	processRunner.EXPECT().Run(gomock.Any()).DoAndReturn(onLeader)
	processRunner.EXPECT().Terminate(gomock.Any()).DoAndReturn(onResign)

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
		onLeader:   onLeader,
		onResign:   onResign,
	}
}

func TestOnLeader_Error(t *testing.T) {
	goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSource()

	election := store.NewMockElection(ctrl)
	election.EXPECT().Campaign(gomock.Any(), _testHost).Return(nil)
	// Expect resignation after onLeader failure
	election.EXPECT().Resign(gomock.Any()).Return(nil)

	leaderStore := store.NewMockElector(ctrl)
	leaderStore.EXPECT().CreateElection(gomock.Any(), _testNamespace.Name).Return(election, nil)

	shardStore := store.NewMockStore(ctrl)

	processFactory := process.NewMockFactory(ctrl)
	processRunner := process.NewMockProcessor(ctrl)
	processFactory.EXPECT().CreateProcessor(_testNamespace, shardStore, election).Return(processRunner)

	// Create elector directly for test control
	el := &elector{
		namespace:   _testNamespace,
		leaderStore: leaderStore,
		store:       shardStore,
		logger:      logger,
		cfg: config.Election{
			LeaderPeriod:           _testLeaderPeriod,
			MaxRandomDelay:         _testMaxRandomDelay,
			FailedElectionCooldown: _testFailedElectionCooldown,
		},
		clock:          timeSource,
		hostname:       _testHost,
		processFactory: processFactory,
	}

	// Create a cancelable context for the test
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel() // Ensure context is canceled at the end of the test

	// Make onLeader return an error
	onLeaderErr := errors.New("leader error")
	processRunner.EXPECT().Run(gomock.Any()).Return(onLeaderErr)
	processRunner.EXPECT().Terminate(gomock.Any()).Return(nil)

	go func() {
		// Wait until run will stop on timer
		timeSource.BlockUntil(1)
		// Advance the time to kick in the election.
		timeSource.Advance(_testMaxRandomDelay)
	}()

	// Run the test
	leaderCh := make(chan bool, 1)
	err := el.runElection(ctx, leaderCh)

	// Error should contain our onLeader error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "onLeader")
	assert.Contains(t, err.Error(), "leader error")
}

func TestRun_ContinuesAfterCampaignFailure(t *testing.T) {
	goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSource()
	leaderStore := store.NewMockElector(ctrl)
	shardStore := store.NewMockStore(ctrl)
	processFactory := process.NewMockFactory(ctrl)
	processRunner := process.NewMockProcessor(ctrl)

	factory := NewElectionFactory(FactoryParams{
		HostName: _testHost,
		Cfg: config.ShardDistribution{
			Election: config.Election{
				LeaderPeriod:           _testLeaderPeriod,
				MaxRandomDelay:         _testMaxRandomDelay,
				FailedElectionCooldown: _testFailedElectionCooldown,
			},
		},
		LeaderStore:    leaderStore,
		Store:          shardStore,
		Logger:         logger,
		Clock:          timeSource,
		ProcessFactory: processFactory,
	})

	el, err := factory.CreateElector(context.Background(), _testNamespace)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the election process
	leaderChan := el.Run(ctx)

	t.Run("campaign failed, no leader", func(t *testing.T) {
		electionFailed := store.NewMockElection(ctrl)
		campaignErr := errors.New("etcdserver: mvcc: required revision has been compacted")
		electionFailed.EXPECT().Campaign(gomock.Any(), _testHost).Return(campaignErr)
		electionFailed.EXPECT().Resign(gomock.Any()).Return(nil)
		leaderStore.EXPECT().CreateElection(gomock.Any(), _testNamespace.Name).Return(electionFailed, nil)

		// First attempt: advance past random delay for first campaign
		timeSource.BlockUntil(1)
		timeSource.Advance(_testMaxRandomDelay)

		// Should receive false due to campaign failure
		assert.False(t, <-leaderChan, "Should receive false after campaign failure")
	})

	t.Run("campaign succeeds, became leader", func(t *testing.T) {
		finished := make(chan struct{})
		electionSuccess := store.NewMockElection(ctrl)
		electionSuccess.EXPECT().Campaign(gomock.Any(), _testHost).Return(nil)
		electionSuccess.EXPECT().Done().Return(make(chan struct{}))
		electionSuccess.EXPECT().Resign(gomock.Any()).DoAndReturn(func(_ context.Context) error {
			close(finished)
			return nil
		})
		leaderStore.EXPECT().CreateElection(gomock.Any(), _testNamespace.Name).Return(electionSuccess, nil)
		processFactory.EXPECT().CreateProcessor(_testNamespace, shardStore, electionSuccess).Return(processRunner)
		processRunner.EXPECT().Run(gomock.Any()).Return(nil)
		processRunner.EXPECT().Terminate(gomock.Any()).Return(nil)

		// After campaign failure, code enters cooldown - wait for it to block, then advance through it
		timeSource.BlockUntil(1)
		timeSource.Advance(_testFailedElectionCooldown)

		// Second attempt: advance past random delay for second campaign
		timeSource.BlockUntil(1)
		timeSource.Advance(_testMaxRandomDelay)

		// Should receive true after successful campaign
		assert.True(t, <-leaderChan, "Should receive true after successful campaign")

		// Cleanup
		cancel()
		<-finished
	})
}
