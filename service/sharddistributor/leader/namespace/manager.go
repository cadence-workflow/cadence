package namespace

import (
	"context"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/election"
	"github.com/uber/cadence/service/sharddistributor/leader/process"
)

// Module provides namespace manager component for an fx app.
var Module = fx.Module(
	"namespace-manager",
	fx.Provide(
		NewManager,
	),
	// Force use manager to get it into fx.
	fx.Invoke(func(Manager) {}),
)

// Manager handles namespace distribution and leader election
type Manager interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type namespaceManager struct {
	cfg              config.LeaderElection
	logger           log.Logger
	electionFactory  election.Factory
	processorFactory process.Factory
	namespaces       map[string]*namespaceHandler
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

type namespaceHandler struct {
	logger       log.Logger
	elector      election.Elector
	processor    process.Processor
	cancel       context.CancelFunc
	namespaceCfg config.Namespace
	cleanupWg    sync.WaitGroup
}

type ManagerParams struct {
	fx.In

	Cfg              config.LeaderElection
	Logger           log.Logger
	ElectionFactory  election.Factory
	ProcessorFactory process.Factory
	Lifecycle        fx.Lifecycle
}

// NewManager creates a new namespace manager
func NewManager(p ManagerParams) Manager {
	manager := &namespaceManager{
		cfg:              p.Cfg,
		logger:           p.Logger.WithTags(tag.ComponentNamespaceManager),
		electionFactory:  p.ElectionFactory,
		processorFactory: p.ProcessorFactory,
		namespaces:       make(map[string]*namespaceHandler),
	}

	p.Lifecycle.Append(fx.StartStopHook(manager.Start, manager.Stop))

	return manager
}

// Start initializes the namespace manager and starts handling all namespaces
func (m *namespaceManager) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(context.Background())

	for _, ns := range m.cfg.Namespaces {
		if err := m.handleNamespace(ns.Name); err != nil {
			return err
		}
	}

	return nil
}

// Stop gracefully stops all namespace handlers
func (m *namespaceManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	for ns, handler := range m.namespaces {
		m.logger.Info("Stopping namespace handler", tag.Namespace(ns))
		if handler.cancel != nil {
			handler.cancel()
		}
	}

	return nil
}

// handleNamespace sets up leadership election for a namespace
func (m *namespaceManager) handleNamespace(namespace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.namespaces[namespace]; exists {
		return nil // Already handling this namespace
	}

	m.logger.Info("Setting up namespace handler", tag.Namespace(namespace))

	ctx, cancel := context.WithCancel(m.ctx)

	// Create elector for this namespace
	elector, err := m.electionFactory.CreateElector(ctx, namespace)
	if err != nil {
		cancel()
		return err
	}

	// Create processor for this namespace
	createProcessor := m.processorFactory.CreateProcessor(namespace)

	handler := &namespaceHandler{
		logger:    m.logger.WithTags(tag.Namespace(namespace)),
		elector:   elector,
		processor: createProcessor,
	}
	// cancel cancels the context and ensures that electionRunner is stopped.
	handler.cancel = func() {
		cancel()
		handler.cleanupWg.Wait()
	}

	m.namespaces[namespace] = handler
	// Start leadership election
	go handler.runElection(ctx)

	return nil
}

// runElection manages the leadership election for a namespace
func (handler *namespaceHandler) runElection(ctx context.Context) {
	handler.cleanupWg.Add(1)
	defer handler.cleanupWg.Done()

	handler.logger.Info("Starting election for namespace")

	leaderCh := handler.elector.Run(ctx, handler.processor.Start, handler.processor.Stop)

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
