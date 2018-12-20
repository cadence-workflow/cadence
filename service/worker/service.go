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

package worker

import (
	"context"
	"time"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"

	"github.com/uber-common/bark"
	"github.com/uber-go/tally"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	persistencefactory "github.com/uber/cadence/common/persistence/persistence-factory"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/service/dynamicconfig"
	"github.com/uber/cadence/service/worker/indexer"
	"github.com/uber/cadence/service/worker/replicator"
	"github.com/uber/cadence/service/worker/sysworkflow"
	"go.uber.org/cadence/.gen/go/shared"
)

const (
	// FrontendRetryLimit is the number of times frontend will try to be connected to before giving up
	FrontendRetryLimit = 5

	// PollingDelay is the amount of time to wait between polling frontend
	PollingDelay = time.Second
)

type (
	// Service represents the cadence-worker service.  This service host all background processing which needs to happen
	// for a Cadence cluster.  This service runs the replicator which is responsible for applying replication tasks
	// generated by remote clusters.
	Service struct {
		stopC         chan struct{}
		params        *service.BootstrapParams
		config        *Config
		metricsClient metrics.Client
		metadataV2Mgr persistence.MetadataManager
		domainCache   cache.DomainCache
	}

	// Config contains all the service config for worker
	Config struct {
		ReplicationCfg *replicator.Config
		SysWorkflowCfg *sysworkflow.Config
		IndexerCfg     *indexer.Config
	}
)

// NewService builds a new cadence-worker service
func NewService(params *service.BootstrapParams) common.Daemon {
	params.UpdateLoggerWithServiceName(common.WorkerServiceName)
	return &Service{
		params: params,
		config: NewConfig(dynamicconfig.NewCollection(params.DynamicConfig, params.Logger)),
		stopC:  make(chan struct{}),
	}
}

// NewConfig builds the new Config for cadence-worker service
func NewConfig(dc *dynamicconfig.Collection) *Config {
	return &Config{
		ReplicationCfg: &replicator.Config{
			EnableHistoryRereplication: dc.GetBoolProperty(dynamicconfig.EnableHistoryRereplication, false),
			PersistenceMaxQPS:          dc.GetIntProperty(dynamicconfig.WorkerPersistenceMaxQPS, 500),
			ReplicatorConcurrency:      dc.GetIntProperty(dynamicconfig.WorkerReplicatorConcurrency, 1000),
			ReplicatorBufferRetryCount: dc.GetIntProperty(dynamicconfig.WorkerReplicatorBufferRetryCount, 8),
			ReplicationTaskMaxRetry:    dc.GetIntProperty(dynamicconfig.WorkerReplicationTaskMaxRetry, 50),
		},
		SysWorkflowCfg: &sysworkflow.Config{},
		IndexerCfg: &indexer.Config{
			EnableIndexer:            dc.GetBoolProperty(dynamicconfig.EnableVisibilityToKafka, dynamicconfig.DefaultEnableVisibilityToKafka),
			IndexerConcurrency:       dc.GetIntProperty(dynamicconfig.WorkerIndexerConcurrency, 1000),
			ESProcessorNumOfWorkers:  dc.GetIntProperty(dynamicconfig.WorkerESProcessorNumOfWorkers, 1),
			ESProcessorBulkActions:   dc.GetIntProperty(dynamicconfig.WorkerESProcessorBulkActions, 1000),
			ESProcessorBulkSize:      dc.GetIntProperty(dynamicconfig.WorkerESProcessorBulkSize, 2<<24), // 16MB
			ESProcessorFlushInterval: dc.GetDurationProperty(dynamicconfig.WorkerESProcessorFlushInterval, 10*time.Second),
		},
	}
}

// Start is called to start the service
func (s *Service) Start() {
	var err error
	params := s.params
	base := service.New(params)

	log := base.GetLogger()
	log.Infof("%v starting", common.WorkerServiceName)
	base.Start()

	s.metricsClient = base.GetMetricsClient()

	pConfig := params.PersistenceConfig
	pConfig.SetMaxQPS(pConfig.DefaultStore, s.config.ReplicationCfg.PersistenceMaxQPS())
	pFactory := persistencefactory.New(&pConfig, params.ClusterMetadata.GetCurrentClusterName(), s.metricsClient, log)
	s.metadataV2Mgr, err = pFactory.NewMetadataManager(persistencefactory.MetadataV2)
	if err != nil {
		log.Fatalf("failed to create metadata manager: %v", err)
	}
	s.domainCache = cache.NewDomainCache(s.metadataV2Mgr, params.ClusterMetadata, s.metricsClient, log)
	s.domainCache.Start()

	if params.ClusterMetadata.IsGlobalDomainEnabled() {
		s.startReplicator(params, base, log)
	}

	if params.ClusterMetadata.IsArchivalEnabled() {
		s.startSysWorker(base, log, params.MetricScope)
	}

	if s.config.IndexerCfg.EnableIndexer() {
		s.startIndexer(params, base, log)
	}

	log.Infof("%v started", common.WorkerServiceName)
	<-s.stopC
	base.Stop()
}

// Stop is called to stop the service
func (s *Service) Stop() {
	select {
	case s.stopC <- struct{}{}:
	default:
	}
	s.params.Logger.Infof("%v stopped", common.WorkerServiceName)
}

func (s *Service) startReplicator(params *service.BootstrapParams, base service.Service, log bark.Logger) {

	replicator := replicator.NewReplicator(params.ClusterMetadata, s.metadataV2Mgr, s.domainCache, base.GetClientBean(),
		s.config.ReplicationCfg, params.MessagingClient, log, s.metricsClient)
	if err := replicator.Start(); err != nil {
		replicator.Stop()
		log.Fatalf("fail to start replicator: %v", err)
	}
}

func (s *Service) startIndexer(params *service.BootstrapParams, base service.Service, log bark.Logger) {
	indexer := indexer.NewIndexer(s.config.IndexerCfg, params.MessagingClient, params.ESClient, params.ESConfig, log, s.metricsClient)
	if err := indexer.Start(); err != nil {
		indexer.Stop()
		log.Fatalf("fail to start indexer: %v", err)
	}
}

func (s *Service) startSysWorker(base service.Service, log bark.Logger, scope tally.Scope) {

	frontendClient := frontend.NewRetryableClient(
		base.GetClientBean().GetFrontendClient(),
		common.CreateFrontendServiceRetryPolicy(),
		common.IsWhitelistServiceTransientError,
	)

	s.waitForFrontendStart(frontendClient, log)
	sysWorker := sysworkflow.NewSysWorker(frontendClient, scope, s.params.BlobstoreClient)
	if err := sysWorker.Start(); err != nil {
		sysWorker.Stop()
		log.Fatalf("failed to start sysworker: %v", err)
	}
}

func (s *Service) waitForFrontendStart(frontendClient frontend.Client, log bark.Logger) {
	name := sysworkflow.Domain
	request := &shared.DescribeDomainRequest{
		Name: &name,
	}

	for i := 0; i < FrontendRetryLimit; i++ {
		if _, err := frontendClient.DescribeDomain(context.Background(), request); err == nil {
			return
		}
		<-time.After(PollingDelay)
	}
	log.Fatal("failed to connect to frontend client")
}
