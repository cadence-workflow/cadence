package namespace

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/election"
)

// mockElectorRun returns a DoAndReturn function that simulates an elector:
// it returns a leaderCh and closes it when the context is cancelled.
func mockElectorRun(leaderCh chan bool) func(ctx context.Context) <-chan bool {
	return func(ctx context.Context) <-chan bool {
		go func() {
			<-ctx.Done()
			close(leaderCh)
		}()
		return (<-chan bool)(leaderCh)
	}
}

// closeDrainObserver is a test helper that simulates the close-and-recreate
// semantics of the real DrainSignalObserver. It wraps a mock and manages
// the channel lifecycle.
type closeDrainObserver struct {
	mu        sync.Mutex
	drainCh   chan struct{}
	undrainCh chan struct{}
}

func newCloseDrainObserver() *closeDrainObserver {
	return &closeDrainObserver{
		drainCh:   make(chan struct{}),
		undrainCh: make(chan struct{}),
	}
}

func (o *closeDrainObserver) Drain() <-chan struct{} {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.drainCh
}

func (o *closeDrainObserver) Undrain() <-chan struct{} {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.undrainCh
}

func (o *closeDrainObserver) SignalDrain() {
	o.mu.Lock()
	defer o.mu.Unlock()
	close(o.drainCh)
	o.undrainCh = make(chan struct{})
}

func (o *closeDrainObserver) SignalUndrain() {
	o.mu.Lock()
	defer o.mu.Unlock()
	close(o.undrainCh)
	o.drainCh = make(chan struct{})
}

func TestNewManager(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := NewManager(ManagerParams{
		Cfg:             cfg,
		Logger:          logger,
		ElectionFactory: electionFactory,
		Lifecycle:       fxtest.NewLifecycle(t),
	})

	assert.NotNil(t, manager)
	assert.Equal(t, cfg, manager.cfg)
	assert.Equal(t, 0, len(manager.namespaces))
}

func TestStartManager(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	leaderCh := make(chan bool)
	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)
	elector.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh))

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	err := manager.Start(context.Background())
	time.Sleep(10 * time.Millisecond)

	assert.NoError(t, err)
	assert.NotNil(t, manager.ctx)
	assert.NotNil(t, manager.cancel)
	assert.Equal(t, 1, len(manager.namespaces))
	assert.Contains(t, manager.namespaces, "test-namespace")

	// Cleanup
	manager.cancel()
	manager.namespaces["test-namespace"].cleanupWg.Wait()
}

func TestStartManagerWithElectorError(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	expectedErr := errors.New("elector creation failed")
	electionFactory.EXPECT().CreateElector(gomock.Any(), config.Namespace{Name: "test-namespace"}).Return(nil, expectedErr)

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	err := manager.Start(context.Background())
	assert.NoError(t, err)

	// The goroutine exits on elector creation error
	handler := manager.namespaces["test-namespace"]
	handler.cleanupWg.Wait()

	// Cleanup
	manager.cancel()
}

func TestStopManager(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	leaderCh := make(chan bool)
	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)
	elector.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh))

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	_ = manager.Start(context.Background())
	time.Sleep(10 * time.Millisecond)

	err := manager.Stop(context.Background())
	assert.NoError(t, err)
}

func TestHandleNamespaceAlreadyExists(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)

	manager := &Manager{
		cfg:             config.ShardDistribution{},
		logger:          logger,
		electionFactory: electionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	manager.ctx, manager.cancel = context.WithCancel(context.Background())
	defer manager.cancel()

	manager.namespaces["test-namespace"] = &namespaceHandler{}

	err := manager.handleNamespace(config.Namespace{Name: "test-namespace"})
	assert.ErrorContains(t, err, "namespace test-namespace already running")
}

func TestRunElection_LeadershipEvents(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	leaderCh := make(chan bool)
	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)
	elector.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh))

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	err := manager.Start(context.Background())
	require.NoError(t, err)

	leaderCh <- true
	time.Sleep(10 * time.Millisecond)

	leaderCh <- false
	time.Sleep(10 * time.Millisecond)

	err = manager.Stop(context.Background())
	assert.NoError(t, err)
}

func TestDrainSignal_TriggersResign(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	leaderCh := make(chan bool)
	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)
	elector.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh))

	observer := newCloseDrainObserver()

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		drainObserver:   observer,
		namespaces:      make(map[string]*namespaceHandler),
	}

	err := manager.Start(context.Background())
	require.NoError(t, err)

	// Wait for the elector to be running
	leaderCh <- true
	time.Sleep(10 * time.Millisecond)

	// Close drain channel — all handlers see it
	observer.SignalDrain()
	time.Sleep(50 * time.Millisecond)

	// Handler should be in an idle state
	err = manager.Stop(context.Background())
	assert.NoError(t, err)
}

func TestDrainSignal_NilDrainObserver(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	leaderCh := make(chan bool)
	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)
	elector.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh))

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	err := manager.Start(context.Background())
	require.NoError(t, err)

	assert.Nil(t, manager.drainObserver)

	err = manager.Stop(context.Background())
	assert.NoError(t, err)
}

func TestDrainSignal_ManagerStopsBeforeDrain(t *testing.T) {
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	leaderCh := make(chan bool)
	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)
	elector.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh))

	observer := newCloseDrainObserver()

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		drainObserver:   observer,
		namespaces:      make(map[string]*namespaceHandler),
	}

	err := manager.Start(context.Background())
	require.NoError(t, err)

	// Stop before drain fires
	err = manager.Stop(context.Background())
	assert.NoError(t, err)
}

func TestDrainThenUndrain_ResumesElection(t *testing.T) {
	logger, logs := testlogger.NewObserved(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)

	elector1 := election.NewMockElector(ctrl)
	leaderCh1 := make(chan bool)
	elector2 := election.NewMockElector(ctrl)
	leaderCh2 := make(chan bool)

	gomock.InOrder(
		electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector1, nil),
		electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector2, nil),
	)
	elector1.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh1))
	elector2.EXPECT().Run(gomock.Any()).DoAndReturn(mockElectorRun(leaderCh2))

	observer := newCloseDrainObserver()

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	manager := &Manager{
		cfg:             cfg,
		logger:          logger,
		electionFactory: electionFactory,
		drainObserver:   observer,
		namespaces:      make(map[string]*namespaceHandler),
	}

	err := manager.Start(context.Background())
	require.NoError(t, err)

	// Phase 1: elector1 running, verify it becomes leader
	leaderCh1 <- true
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, logs.FilterMessage("Became leader for namespace").Len(), "expected leader elected in phase 1")

	// Drain - elector1 resigns
	observer.SignalDrain()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, logs.FilterMessage("Drain signal received, resigning from election").Len())

	// Undrain - elector2 created, campaign again
	observer.SignalUndrain()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, logs.FilterMessage("Undrain signal received, resuming election").Len())

	// Phase 2: verify elector2 is running and becomes leader after undrain
	leaderCh2 <- true
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 2, logs.FilterMessage("Became leader for namespace").Len(), "expected leader elected in both phases")

	err = manager.Stop(context.Background())
	assert.NoError(t, err)
}
