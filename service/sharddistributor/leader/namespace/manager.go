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

package namespace

import (
	"context"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
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
	elector      election.Elector
	processor    process.Processor
	cancel       context.CancelFunc
	namespaceCfg config.Namespace
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
		elector:   elector,
		processor: createProcessor,
		cancel:    cancel,
	}

	m.namespaces[namespace] = handler

	// Start leadership election
	go m.runElection(ctx, namespace, handler)

	return nil
}

// runElection manages the leadership election for a namespace
func (m *namespaceManager) runElection(ctx context.Context, namespace string, handler *namespaceHandler) {
	logger := m.logger.WithTags(tag.Namespace(namespace))

	logger.Info("Starting election for namespace")

	leaderCh := handler.elector.Run(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, stopping election")
			return

		case isLeader := <-leaderCh:
			if isLeader {
				logger.Info("Became leader for namespace")
				// Start the processor when we become leader
				if err := handler.processor.Start(ctx); err != nil {
					logger.Error("Error starting processor", tag.Error(err))
				}
			} else {
				logger.Info("Lost leadership for namespace")
				// Stop the processor when we lose leadership
				if err := handler.processor.Stop(ctx); err != nil {
					logger.Error("Error stopping processor", tag.Error(err))
				}
			}
		}
	}
}
