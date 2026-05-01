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

package task

import (
	"context"
	"sync/atomic"

	"github.com/uber/cadence/common"
)

type rateLimitedProcessor struct {
	baseProcessor Processor
	rateLimiter   RateLimiter
	cancelCtx     context.Context
	cancelFn      context.CancelFunc
	status        int32
}

func NewRateLimitedProcessor(
	baseProcessor Processor,
	rateLimiter RateLimiter,
) Processor {
	ctx, cancel := context.WithCancel(context.Background())
	return &rateLimitedProcessor{
		baseProcessor: baseProcessor,
		rateLimiter:   rateLimiter,
		cancelCtx:     ctx,
		cancelFn:      cancel,
		status:        common.DaemonStatusInitialized,
	}
}

func (p *rateLimitedProcessor) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	p.baseProcessor.Start()
}

func (p *rateLimitedProcessor) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	p.cancelFn()
	p.baseProcessor.Stop()
}

func (p *rateLimitedProcessor) Submit(t Task) error {
	if err := p.rateLimiter.Wait(p.cancelCtx, t); err != nil {
		return err
	}
	return p.baseProcessor.Submit(t)
}

func (p *rateLimitedProcessor) TrySubmit(t Task) (bool, error) {
	if ok := p.rateLimiter.Allow(t); !ok {
		return false, nil
	}
	return p.baseProcessor.TrySubmit(t)
}
