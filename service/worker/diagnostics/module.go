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

package diagnostics

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
)

type DiagnosticsWorkflow interface {
	Start() error
	Stop()
}

type dw struct {
	svcClient     workflowserviceclient.Interface
	clientBean    client.Bean
	metricsClient metrics.Client
	logger        log.Logger
	tallyScope    tally.Scope
	worker        worker.Worker
	kafkaCfg      config.KafkaConfig
}

type Params struct {
	ServiceClient workflowserviceclient.Interface
	ClientBean    client.Bean
	MetricsClient metrics.Client
	Logger        log.Logger
	TallyScope    tally.Scope
	KafkaCfg      config.KafkaConfig
}

// New creates a new diagnostics workflow.
func New(params Params) DiagnosticsWorkflow {
	return &dw{
		svcClient:     params.ServiceClient,
		metricsClient: params.MetricsClient,
		tallyScope:    params.TallyScope,
		clientBean:    params.ClientBean,
		logger:        params.Logger,
		kafkaCfg:      params.KafkaCfg,
	}
}

// Start starts the worker
func (w *dw) Start() error {
	workerOpts := worker.Options{
		MetricsScope:                     w.tallyScope,
		BackgroundActivityContext:        context.Background(),
		Tracer:                           opentracing.GlobalTracer(),
		MaxConcurrentActivityTaskPollers: 10,
		MaxConcurrentDecisionTaskPollers: 10,
	}
	newWorker := worker.New(w.svcClient, common.SystemLocalDomainName, tasklist, workerOpts)
	newWorker.RegisterWorkflowWithOptions(w.DiagnosticsWorkflow, workflow.RegisterOptions{Name: diagnosticsWorkflow})
	newWorker.RegisterWorkflowWithOptions(w.DiagnosticsStarterWorkflow, workflow.RegisterOptions{Name: diagnosticsStarterWorkflow})
	newWorker.RegisterActivityWithOptions(w.retrieveExecutionHistory, activity.RegisterOptions{Name: retrieveWfExecutionHistoryActivity})
	newWorker.RegisterActivityWithOptions(w.identifyIssues, activity.RegisterOptions{Name: identifyIssuesActivity})
	newWorker.RegisterActivityWithOptions(w.rootCauseIssues, activity.RegisterOptions{Name: rootCauseIssuesActivity})
	newWorker.RegisterActivityWithOptions(w.emitUsageLogs, activity.RegisterOptions{Name: emitUsageLogsActivity})
	w.worker = newWorker
	return newWorker.Start()
}

func (w *dw) Stop() {
	w.worker.Stop()
}
