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

package domaindeprecation

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
)

type (
	DomainDeprecationWorker interface {
		Start() error
		Stop()
	}

	domainDeprecator struct {
		svcClient workflowserviceclient.Interface
		worker    worker.Worker
		tally     tally.Scope
		logger    log.Logger
	}

	Params struct {
		ServiceClient workflowserviceclient.Interface
		Tally         tally.Scope
		Logger        log.Logger
	}
)

// New creates a new domain deprecation workflow.
func New(params Params) DomainDeprecationWorker {
	return &domainDeprecator{
		svcClient: params.ServiceClient,
		tally:     params.Tally,
		logger:    params.Logger,
	}
}

// Start starts the worker
func (w *domainDeprecator) Start() error {
	workerOpts := worker.Options{
		MetricsScope:                     w.tally,
		BackgroundActivityContext:        context.Background(),
		Tracer:                           opentracing.GlobalTracer(),
		MaxConcurrentActivityTaskPollers: 10,
		MaxConcurrentDecisionTaskPollers: 10,
	}
	newWorker := worker.New(w.svcClient, constants.SystemLocalDomainName, domainDeprecationTaskListName, workerOpts)
	newWorker.RegisterWorkflowWithOptions(w.DomainDeprecationWorkflow, workflow.RegisterOptions{Name: domainDeprecationWorkflowTypeName})
	newWorker.RegisterActivityWithOptions(w.DisableArchivalActivity, activity.RegisterOptions{Name: disableArchivalActivity})
	w.worker = newWorker
	return newWorker.Start()
}

func (w *domainDeprecator) Stop() {
	w.worker.Stop()
}
