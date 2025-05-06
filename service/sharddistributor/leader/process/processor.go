package process

import (
	"context"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/config"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=process_mock.go Factory,Processor

// Module provides processor factor for fx app.
var Module = fx.Module(
	"leader-process",
	fx.Provide(NewProcessorFactory),
)

// Processor represents a process that runs when the instance is the leader
type Processor interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Factory creates processor instances
type Factory interface {
	CreateProcessor(namespace string) Processor
}

type processorFactory struct {
	logger     log.Logger
	timeSource clock.TimeSource
	cfg        config.LeaderProcess
}

type namespaceProcessor struct {
	namespace  string
	logger     log.Logger
	timeSource clock.TimeSource
	mu         sync.Mutex
	running    bool
	cancel     context.CancelFunc
	cfg        config.LeaderProcess
	wg         sync.WaitGroup
}

// NewProcessorFactory creates a new processor factory
func NewProcessorFactory(
	logger log.Logger,
	timeSource clock.TimeSource,
	cfg config.LeaderElection,
) Factory {
	return &processorFactory{
		logger:     logger,
		timeSource: timeSource,
		cfg:        cfg.Process,
	}
}

// CreateProcessor creates a new processor for the given namespace
func (f *processorFactory) CreateProcessor(namespace string) Processor {
	return &namespaceProcessor{
		namespace:  namespace,
		logger:     f.logger.WithTags(tag.ComponentLeaderProcessor, tag.Namespace(namespace)),
		timeSource: f.timeSource,
		cfg:        f.cfg,
	}
}

// Start begins processing for this namespace
func (p *namespaceProcessor) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil // Already running
	}

	pCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running = true

	p.logger.Info("Starting")

	p.wg.Add(1)
	// Start the process in a goroutine
	go p.runProcess(pCtx)

	return nil
}

// Stop halts processing for this namespace
func (p *namespaceProcessor) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil // Not running
	}

	p.logger.Info("Stopping")

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	p.running = false

	// Ensure that the process has stopped.
	p.wg.Wait()

	return nil
}

// runProcess executes the actual processing logic
func (p *namespaceProcessor) runProcess(ctx context.Context) {
	defer p.wg.Done()

	// TODO: this should be dynamic config.
	ticker := p.timeSource.NewTicker(p.cfg.Period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Process cancelled")
			return
		case <-ticker.Chan():
		}
	}
}
