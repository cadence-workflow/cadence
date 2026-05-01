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

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/worker/batcher"
)

type (
	DomainDeprecationWorker interface {
		Start() error
		Stop()
	}

	// Config defines the configuration for domain deprecator
	Config struct {
		// AdminOperationToken is a dynamic config that provides the security token for admin operations
		AdminOperationToken dynamicproperties.StringPropertyFn
	}

	domainDeprecator struct {
		cfg           Config
		svcClient     workflowserviceclient.Interface
		clientBean    client.Bean
		metricsClient metrics.Client
		worker        worker.Worker
		tally         tally.Scope
		logger        log.Logger
	}

	Params struct {
		Config        Config
		ServiceClient workflowserviceclient.Interface
		ClientBean    client.Bean
		MetricsClient metrics.Client
		Tally         tally.Scope
		Logger        log.Logger
	}
)

// New creates a new domain deprecation workflow.
func New(params Params) DomainDeprecationWorker {
	return &domainDeprecator{
		cfg:           params.Config,
		svcClient:     params.ServiceClient,
		clientBean:    params.ClientBean,
		metricsClient: params.MetricsClient,
		tally:         params.Tally,
		logger:        params.Logger,
	}
}

// Start starts the worker
func (w *domainDeprecator) Start() error {
	batcherParams := &batcher.BootstrapParams{
		Config: batcher.Config{
			AdminOperationToken: w.cfg.AdminOperationToken,
		},
		ServiceClient: w.svcClient,
		ClientBean:    w.clientBean,
		MetricsClient: w.metricsClient,
		Logger:        w.logger,
		TallyScope:    w.tally,
	}
	batcherInstance := batcher.New(batcherParams)

	ctx := context.WithValue(context.Background(), batcher.BatcherContextKey, batcherInstance)

	workerOpts := worker.Options{
		MetricsScope:                     w.tally,
		BackgroundActivityContext:        ctx,
		Tracer:                           opentracing.GlobalTracer(),
		MaxConcurrentActivityTaskPollers: 10,
		MaxConcurrentDecisionTaskPollers: 10,
	}
	newWorker := worker.New(w.svcClient, constants.SystemLocalDomainName, DomainDeprecationTaskListName, workerOpts)
	newWorker.RegisterWorkflowWithOptions(w.DomainDeprecationWorkflow, workflow.RegisterOptions{Name: DomainDeprecationWorkflowTypeName})
	newWorker.RegisterActivityWithOptions(w.DisableArchivalActivity, activity.RegisterOptions{Name: disableArchivalActivity, EnableAutoHeartbeat: true})
	newWorker.RegisterActivityWithOptions(w.CheckOpenWorkflowsActivity, activity.RegisterOptions{Name: checkOpenWorkflowsActivity, EnableAutoHeartbeat: true})
	newWorker.RegisterActivityWithOptions(w.DeprecateDomainActivity, activity.RegisterOptions{Name: deprecateDomainActivity, EnableAutoHeartbeat: true})
	w.worker = newWorker
	return newWorker.Start()
}

func (w *domainDeprecator) Stop() {
	w.worker.Stop()
}
