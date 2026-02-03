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

package resource

import (
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/blobstore"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
	"github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

type (
	// Params holds the set of parameters needed to initialize common service resources
	Params struct {
		Name               string
		InstanceID         string
		Logger             log.Logger
		ThrottledLogger    log.Logger
		HostName           string
		GetIsolationGroups func() []string

		MetricScope        tally.Scope
		MembershipResolver membership.Resolver
		HashRings          map[string]membership.SingleProvider
		RPCFactory         rpc.Factory
		PProfInitializer   common.PProfInitializer
		PersistenceConfig  config.Persistence
		ClusterMetadata    cluster.Metadata
		ReplicatorConfig   config.Replicator
		MetricsClient      metrics.Client
		MessagingClient    messaging.Client
		BlobstoreClient    blobstore.Client
		ESClient           es.GenericClient
		ESConfig           *config.ElasticSearchConfig

		// RPC configuration
		RPCConfig config.RPC

		DynamicConfig              dynamicconfig.Client
		ClusterRedirectionPolicy   *config.ClusterRedirectionPolicy
		PublicClient               workflowserviceclient.Interface
		ArchivalMetadata           archiver.ArchivalMetadata
		ArchiverProvider           provider.ArchiverProvider
		Authorizer                 authorization.Authorizer // NOTE: this can be nil. If nil, AccessControlledHandlerImpl will initiate one with config.Authorization
		AuthorizationConfig        config.Authorization     // NOTE: empty(default) struct will get a authorization.NoopAuthorizer
		IsolationGroupStore        configstore.Client       // This can be nil, the default config store will be created if so
		IsolationGroupState        isolationgroup.State     // This can be nil, the default state store will be chosen if so
		PinotConfig                *config.PinotVisibilityConfig
		KafkaConfig                config.KafkaConfig
		PinotClient                pinot.GenericClient
		OSClient                   es.GenericClient
		OSConfig                   *config.ElasticSearchConfig
		AsyncWorkflowQueueProvider queue.Provider
		TimeSource                 clock.TimeSource
		// HistoryClientFn is used by integration tests to mock a history client
		HistoryClientFn func() history.Client
		// NewPersistenceBeanFn can be used to override the default persistence bean creation in unit tests to avoid DB setup
		NewPersistenceBeanFn  func(persistenceClient.Factory, *persistenceClient.Params, *service.Config) (persistenceClient.Bean, error)
		DiagnosticsInvariants []invariant.Invariant

		// ShardDistributorMatchingConfig is the config for shard distributor executor client in matching service
		ShardDistributorMatchingConfig clientcommon.Config
	}
)
