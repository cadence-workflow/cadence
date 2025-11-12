package namespace

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/election"
)

// Module provides namespace manager component for an fx app.
var Module = fx.Module(
	"namespace-manager",
	fx.Invoke(NewManager),
)

type Manager struct {
	cfg             config.ShardDistribution
	logger          log.Logger
	electionFactory election.Factory
	namespaces      map[string]*namespaceHandler
	ctx             context.Context
	cancel          context.CancelFunc
}

type namespaceHandler struct {
	logger       log.Logger
	elector      election.Elector
	cancel       context.CancelFunc
	namespaceCfg config.Namespace
	cleanupWg    sync.WaitGroup
}

type ManagerParams struct {
	fx.In

	Cfg             config.ShardDistribution
	Logger          log.Logger
	ElectionFactory election.Factory
	Lifecycle       fx.Lifecycle
}

// NewManager creates a new namespace manager
func NewManager(p ManagerParams) *Manager {
	manager := &Manager{
		cfg:             p.Cfg,
		logger:          p.Logger.WithTags(tag.ComponentNamespaceManager),
		electionFactory: p.ElectionFactory,
		namespaces:      make(map[string]*namespaceHandler),
	}

	p.Lifecycle.Append(fx.StartStopHook(manager.Start, manager.Stop))

	return manager
}

// Start initializes the namespace manager and starts handling all namespaces
func (m *Manager) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(context.Background())

	for _, ns := range m.cfg.Namespaces {
		m.logger.Info("Starting namespace handler", tag.ShardNamespace(ns.Name))
		if err := m.handleNamespace(ns); err != nil {
			return err
		}
	}

	return nil
}

// Stop gracefully stops all namespace handlers
func (m *Manager) Stop(ctx context.Context) error {
	if m.cancel == nil {
		return fmt.Errorf("manager was not running")
	}

	m.cancel()

	for ns, handler := range m.namespaces {
		m.logger.Info("Stopping namespace handler", tag.ShardNamespace(ns))
		if handler.cancel != nil {
			handler.cancel()
		}
	}

	return nil
}

// handleNamespace sets up leadership election for a namespace
func (m *Manager) handleNamespace(namespaceCfg config.Namespace) error {
	if _, exists := m.namespaces[namespaceCfg.Name]; exists {
		return fmt.Errorf("namespace %s already running", namespaceCfg.Name)
	}

	m.logger.Info("Setting up namespace handler", tag.ShardNamespace(namespaceCfg.Name))

	ctx, cancel := context.WithCancel(m.ctx)

	// Create elector for this namespace
	elector, err := m.electionFactory.CreateElector(ctx, namespaceCfg)
	if err != nil {
		cancel()
		return err
	}

	handler := &namespaceHandler{
		logger:  m.logger.WithTags(tag.ShardNamespace(namespaceCfg.Name)),
		elector: elector,
	}
	// cancel cancels the context and ensures that electionRunner is stopped.
	handler.cancel = func() {
		cancel()
		handler.cleanupWg.Wait()
	}

	m.namespaces[namespaceCfg.Name] = handler
	handler.cleanupWg.Add(1)
	// Start leadership election
	go handler.runElection(ctx)

	return nil
}

// runElection manages the leadership election for a namespace
func (handler *namespaceHandler) runElection(ctx context.Context) {
	defer handler.cleanupWg.Done()

	handler.logger.Info("Starting election for namespace")

	leaderCh := handler.elector.Run(ctx)

	for {
		select {
		case <-ctx.Done():
			handler.logger.Info("Context cancelled, stopping election")
			return
		case isLeader := <-leaderCh:
			if isLeader {
				handler.logger.Info("Became leader for namespace")
			} else {
				handler.logger.Info("Lost leadership for namespace")
			}
		}
	}
}
