package namespace

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/election"
)

// Module provides namespace manager component for an fx app.
var Module = fx.Module(
	"namespace-manager",
	fx.Invoke(NewManager),
)

// stateFn is a recursive function type representing a state in the election
// state machine.
// Each state function blocks until a transition occurs and returns the next state function,
// or nil to stop the machine.
type stateFn func(ctx context.Context) stateFn

type Manager struct {
	cfg             config.ShardDistribution
	logger          log.Logger
	electionFactory election.Factory
	drainObserver   clientcommon.DrainSignalObserver
	namespaces      map[string]*namespaceHandler
	ctx             context.Context
	cancel          context.CancelFunc
}

type namespaceHandler struct {
	logger          log.Logger
	electionFactory election.Factory
	namespaceCfg    config.Namespace
	drainObserver   clientcommon.DrainSignalObserver
	cleanupWg       sync.WaitGroup
}

type ManagerParams struct {
	fx.In

	Cfg             config.ShardDistribution
	Logger          log.Logger
	ElectionFactory election.Factory
	Lifecycle       fx.Lifecycle
	DrainObserver   clientcommon.DrainSignalObserver `optional:"true"`
}

// NewManager creates a new namespace manager
func NewManager(p ManagerParams) *Manager {
	manager := &Manager{
		cfg:             p.Cfg,
		logger:          p.Logger.WithTags(tag.ComponentNamespaceManager),
		electionFactory: p.ElectionFactory,
		drainObserver:   p.DrainObserver,
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

// Stop gracefully stops all namespace handlers.
// Cancels the manager context which cascades to all handler contexts,
// then waits for all election goroutines to finish.
func (m *Manager) Stop(ctx context.Context) error {
	if m.cancel == nil {
		return fmt.Errorf("manager was not running")
	}

	m.cancel()

	for ns, handler := range m.namespaces {
		m.logger.Info("Waiting for namespace handler to stop", tag.ShardNamespace(ns))
		handler.cleanupWg.Wait()
	}

	return nil
}

// handleNamespace sets up a namespace handler and starts its election goroutine.
func (m *Manager) handleNamespace(namespaceCfg config.Namespace) error {
	if _, exists := m.namespaces[namespaceCfg.Name]; exists {
		return fmt.Errorf("namespace %s already running", namespaceCfg.Name)
	}

	handler := &namespaceHandler{
		logger:          m.logger.WithTags(tag.ShardNamespace(namespaceCfg.Name)),
		electionFactory: m.electionFactory,
		namespaceCfg:    namespaceCfg,
		drainObserver:   m.drainObserver,
	}

	m.namespaces[namespaceCfg.Name] = handler
	handler.cleanupWg.Add(1)

	go handler.runElection(m.ctx)

	return nil
}

// runElection drives the election state machine for a namespace.
// It starts in the campaigning state and follows state transitions
// until a state returns nil (stop).
func (h *namespaceHandler) runElection(ctx context.Context) {
	defer h.cleanupWg.Done()

	for state := h.campaigning; state != nil; {
		state = state(ctx)
	}
}

func (h *namespaceHandler) drainChannel() <-chan struct{} {
	if h.drainObserver != nil {
		return h.drainObserver.Drain()
	}
	return nil
}

func (h *namespaceHandler) startElection(ctx context.Context) (<-chan bool, context.CancelFunc, error) {
	electorCtx, cancel := context.WithCancel(ctx)
	elector, err := h.electionFactory.CreateElector(electorCtx, h.namespaceCfg)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return elector.Run(electorCtx), cancel, nil
}

// campaigning creates an elector and participates in leader election.
// Transitions: h.idle on drain, h.campaigning on recoverable error, nil on stop.
func (h *namespaceHandler) campaigning(ctx context.Context) stateFn {
	h.logger.Info("Entering campaigning state")

	drainCh := h.drainChannel()

	select {
	case <-drainCh:
		h.logger.Info("Drain signal detected before election start")
		return h.idle
	default:
	}

	leaderCh, cancel, err := h.startElection(ctx)
	if err != nil {
		h.logger.Error("Failed to create elector", tag.Error(err))
		return nil
	}
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-drainCh:
			h.logger.Info("Drain signal received, resigning from election")
			return h.idle
		case isLeader, ok := <-leaderCh:
			if !ok {
				h.logger.Error("Election channel closed unexpectedly")
				return h.campaigning
			}
			if isLeader {
				h.logger.Info("Became leader for namespace")
			} else {
				h.logger.Info("Lost leadership for namespace")
			}
		}
	}
}

// idle waits for an undrain signal to resume campaigning.
// Transitions: h.campaigning on undrain, nil on stop.
func (h *namespaceHandler) idle(ctx context.Context) stateFn {
	h.logger.Info("Entering idle state (drained)")

	var undrainCh <-chan struct{}
	if h.drainObserver != nil {
		undrainCh = h.drainObserver.Undrain()
	}

	select {
	case <-ctx.Done():
		return nil
	case <-undrainCh:
		h.logger.Info("Undrain signal received, resuming election")
		return h.campaigning
	}
}
