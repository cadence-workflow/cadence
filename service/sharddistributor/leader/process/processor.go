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

package process

import (
	"context"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
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

	return nil
}

// runProcess executes the actual processing logic
func (p *namespaceProcessor) runProcess(ctx context.Context) {
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
