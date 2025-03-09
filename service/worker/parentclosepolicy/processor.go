// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package parentclosepolicy

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/worker"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type (
	// BootstrapParams contains the set of params needed to bootstrap
	// the sub-system
	BootstrapParams struct {
		// Config contains the configuration for scanner
		// ServiceClient is an instance of cadence service client
		ServiceClient workflowserviceclient.Interface
		// MetricsClient is an instance of metrics object for emitting stats
		MetricsClient metrics.Client
		Logger        log.Logger
		// TallyScope is an instance of tally metrics scope
		TallyScope tally.Scope
		// ClientBean is the collection of clients
		ClientBean client.Bean
		// DomainCache is the cache for domain information and configuration
		DomainCache cache.DomainCache
		// NumWorkflows is the total number of workflows for processing parent close policy
		NumWorkflows int
	}

	// Processor is the background sub-system that execute workflow for ParentClosePolicy
	Processor struct {
		svcClient     workflowserviceclient.Interface
		clientBean    client.Bean
		domainCache   cache.DomainCache
		numWorkflows  int
		metricsClient metrics.Client
		tallyScope    tally.Scope
		logger        log.Logger
	}
)

// New returns a new instance as daemon
func New(params *BootstrapParams) *Processor {
	return &Processor{
		svcClient:     params.ServiceClient,
		clientBean:    params.ClientBean,
		domainCache:   params.DomainCache,
		numWorkflows:  params.NumWorkflows,
		metricsClient: params.MetricsClient,
		tallyScope:    params.TallyScope,
		logger:        params.Logger.WithTags(tag.ComponentBatcher),
	}
}

// Start starts the scanner
func (s *Processor) Start() error {
	ctx := context.WithValue(context.Background(), processorContextKey, s)
	workerOpts := worker.Options{
		MetricsScope:              s.tallyScope,
		BackgroundActivityContext: ctx,
		Tracer:                    opentracing.GlobalTracer(),
	}
	processorWorker := worker.New(s.svcClient, constants.SystemLocalDomainName, processorTaskListName, workerOpts)
	return processorWorker.Start()
}
