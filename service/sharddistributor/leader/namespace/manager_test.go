package namespace

import (
	"context"
	"errors"
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

func TestNewManager(t *testing.T) {
	// Setup
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-namespace"},
		},
	}

	// Test
	manager := NewManager(ManagerParams{
		Cfg:             cfg,
		Logger:          logger,
		ElectionFactory: electionFactory,
		Lifecycle:       fxtest.NewLifecycle(t),
	})

	// Assert
	assert.NotNil(t, manager)
	assert.Equal(t, cfg, manager.cfg)
	assert.Equal(t, 0, len(manager.namespaces))
}

func TestStartManager(t *testing.T) {
	// Setup
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)

	leaderCh := make(chan bool)
	elector.EXPECT().Run(gomock.Any()).Return((<-chan bool)(leaderCh))

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

	// Test
	err := manager.Start(context.Background())

	// Try to give goroutine time to start.
	time.Sleep(time.Millisecond)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, manager.ctx)
	assert.NotNil(t, manager.cancel)
	assert.Equal(t, 1, len(manager.namespaces))
	assert.Contains(t, manager.namespaces, "test-namespace")
}

func TestStartManagerWithElectorError(t *testing.T) {
	// Setup
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

	// Test
	err := manager.Start(context.Background())

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, 0, len(manager.namespaces))
}

func TestStopManager(t *testing.T) {
	// Setup
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)

	leaderCh := make(chan bool)
	elector.EXPECT().Run(gomock.Any()).Return((<-chan bool)(leaderCh))

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

	// Start the manager first
	_ = manager.Start(context.Background())

	// Try to give goroutine time to start.
	time.Sleep(time.Millisecond)

	// Test
	err := manager.Stop(context.Background())

	// Assert
	assert.NoError(t, err)
}

func TestHandleNamespaceAlreadyExists(t *testing.T) {
	// Setup
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	mockElector := election.NewMockElector(ctrl)

	manager := &Manager{
		cfg:             config.ShardDistribution{},
		logger:          logger,
		electionFactory: electionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	// Set context
	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// Add existing namespace handler
	manager.namespaces["test-namespace"] = &namespaceHandler{
		elector: mockElector,
	}

	// Test
	err := manager.handleNamespace(config.Namespace{Name: "test-namespace"})

	// Assert
	assert.ErrorContains(t, err, "namespace test-namespace already running")
}

func TestRunElection(t *testing.T) {
	// Setup
	logger := testlogger.New(t)
	ctrl := gomock.NewController(t)
	electionFactory := election.NewMockFactory(ctrl)
	elector := election.NewMockElector(ctrl)

	electionFactory.EXPECT().CreateElector(gomock.Any(), gomock.Any()).Return(elector, nil)

	leaderCh := make(chan bool)
	elector.EXPECT().Run(gomock.Any()).Return((<-chan bool)(leaderCh))

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the test goroutine
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Test becoming leader
	leaderCh <- true
	time.Sleep(10 * time.Millisecond) // Give some time for goroutine to process

	// Test losing leadership
	leaderCh <- false
	time.Sleep(10 * time.Millisecond) // Give some time for goroutine to process

	// Cancel context to end the goroutine
	manager.cancel()
	time.Sleep(10 * time.Millisecond) // Give some time for goroutine to exit
}
