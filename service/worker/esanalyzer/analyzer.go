// Copyright (c) 2021 Uber Technologies, Inc.
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

package esanalyzer

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/.gen/go/shared"
	cclient "go.uber.org/cadence/client"
	"go.uber.org/cadence/worker"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/service/worker/workercommon"
)

type readMode string

const (
	Pinot readMode = "pinot"
	ES    readMode = "es"
)

type (
	// Analyzer is the background sub-system to query ElasticSearch and execute mitigations
	Analyzer struct {
		svcClient           workflowserviceclient.Interface
		frontendClient      frontend.Client
		clientBean          client.Bean
		esClient            es.GenericClient
		pinotClient         pinot.GenericClient
		readMode            readMode
		logger              log.Logger
		tallyScope          tally.Scope
		visibilityIndexName string
		pinotTableName      string
		resource            resource.Resource
		domainCache         cache.DomainCache
		config              *Config
	}

	// Config contains all configs for ElasticSearch Analyzer
	Config struct {
		ESAnalyzerPause                          dynamicconfig.BoolPropertyFn
		ESAnalyzerTimeWindow                     dynamicconfig.DurationPropertyFn
		ESAnalyzerMaxNumDomains                  dynamicconfig.IntPropertyFn
		ESAnalyzerMaxNumWorkflowTypes            dynamicconfig.IntPropertyFn
		ESAnalyzerLimitToTypes                   dynamicconfig.StringPropertyFn
		ESAnalyzerEnableAvgDurationBasedChecks   dynamicconfig.BoolPropertyFn
		ESAnalyzerLimitToDomains                 dynamicconfig.StringPropertyFn
		ESAnalyzerNumWorkflowsToRefresh          dynamicconfig.IntPropertyFnWithWorkflowTypeFilter
		ESAnalyzerBufferWaitTime                 dynamicconfig.DurationPropertyFnWithWorkflowTypeFilter
		ESAnalyzerMinNumWorkflowsForAvg          dynamicconfig.IntPropertyFnWithWorkflowTypeFilter
		ESAnalyzerWorkflowDurationWarnThresholds dynamicconfig.StringPropertyFn
		ESAnalyzerWorkflowVersionDomains         dynamicconfig.StringPropertyFn
		ESAnalyzerWorkflowTypeDomains            dynamicconfig.StringPropertyFn
	}

	Workflow struct {
		analyzer *Analyzer
	}

	EsAggregateCount struct {
		AggregateKey   string `json:"key"`
		AggregateCount int64  `json:"doc_count"`
	}
)

const (
	taskListName = "cadence-sys-es-analyzer"

	startUpDelay = time.Second * 10
)

// New returns a new instance as daemon
func New(
	svcClient workflowserviceclient.Interface,
	frontendClient frontend.Client,
	clientBean client.Bean,
	esClient es.GenericClient,
	pinotClient pinot.GenericClient,
	esConfig *config.ElasticSearchConfig,
	pinotConfig *config.PinotVisibilityConfig,
	logger log.Logger,
	tallyScope tally.Scope,
	resource resource.Resource,
	domainCache cache.DomainCache,
	config *Config,
) *Analyzer {
	var mode readMode
	var indexName string
	var pinotTableName string

	if esClient != nil {
		mode = ES
		indexName = esConfig.Indices[common.VisibilityAppName]
		pinotTableName = ""
	} else if pinotClient != nil {
		mode = Pinot
		indexName = ""
		pinotTableName = pinotConfig.Table
	}

	return &Analyzer{
		svcClient:           svcClient,
		frontendClient:      frontendClient,
		clientBean:          clientBean,
		esClient:            esClient,
		pinotClient:         pinotClient,
		readMode:            mode,
		logger:              logger,
		tallyScope:          tallyScope,
		visibilityIndexName: indexName,
		pinotTableName:      pinotTableName,
		resource:            resource,
		domainCache:         domainCache,
		config:              config,
	}
}

// Start starts the scanner
func (a *Analyzer) Start() error {
	ctx := context.Background()
	a.StartWorkflow(ctx)
	ctx = context.Background()
	a.StartDomainWFTypeCountWorkflow(ctx)

	workerOpts := worker.Options{
		MetricsScope:              a.tallyScope,
		BackgroundActivityContext: ctx,
		Tracer:                    opentracing.GlobalTracer(),
	}
	esWorker := worker.New(a.svcClient, common.SystemLocalDomainName, taskListName, workerOpts)
	err := esWorker.Start()
	return err
}

func (a *Analyzer) StartWorkflow(ctx context.Context) {
	initWorkflow(a)
	go workercommon.StartWorkflowWithRetry(esanalyzerWFTypeName, startUpDelay, a.resource, func(client cclient.Client) error {
		_, err := client.StartWorkflow(ctx, wfOptions, esanalyzerWFTypeName)
		switch err.(type) {
		case *shared.WorkflowExecutionAlreadyStartedError:
			return nil
		default:
			a.logger.Error("Failed to start ElasticSearch Analyzer", tag.Error(err))
			return err
		}
	})
}

func (a *Analyzer) StartDomainWFTypeCountWorkflow(ctx context.Context) {
	initDomainWorkflowTypeCountWorkflow(a)
	go workercommon.StartWorkflowWithRetry(domainWFTypeCountWorkflowTypeName, startUpDelay, a.resource, func(client cclient.Client) error {
		_, err := client.StartWorkflow(ctx, domainWfTypeCountStartOptions, domainWFTypeCountWorkflowTypeName)
		switch err.(type) {
		case *shared.WorkflowExecutionAlreadyStartedError:
			return nil
		default:
			a.logger.Error("Failed to start domain wf type count workflow", tag.Error(err))
			return err
		}
	})
}
