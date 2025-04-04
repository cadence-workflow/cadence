// Copyright (c) 2019 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination resource_mock.go -self_package github.com/uber/cadence/common/resource

package resource

import (
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/blobstore"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
	qrpc "github.com/uber/cadence/common/quotas/global/rpc"
	"github.com/uber/cadence/common/service"
)

type ResourceFactory interface {
	NewResource(params *Params,
		serviceName string,
		serviceConfig *service.Config,
	) (resource Resource, err error)
}

// Resource is the interface which expose common resources
type Resource interface {
	common.Daemon

	// static infos

	GetServiceName() string
	GetHostInfo() membership.HostInfo
	GetArchivalMetadata() archiver.ArchivalMetadata
	GetClusterMetadata() cluster.Metadata

	// other common resources

	GetDomainCache() cache.DomainCache
	GetDomainMetricsScopeCache() cache.DomainMetricsScopeCache
	GetTimeSource() clock.TimeSource
	GetPayloadSerializer() persistence.PayloadSerializer
	GetMetricsClient() metrics.Client
	GetArchiverProvider() provider.ArchiverProvider
	GetMessagingClient() messaging.Client
	GetBlobstoreClient() blobstore.Client
	GetDomainReplicationQueue() domain.ReplicationQueue

	// membership infos
	GetMembershipResolver() membership.Resolver

	// internal services clients

	GetSDKClient() workflowserviceclient.Interface
	GetFrontendRawClient() frontend.Client
	GetFrontendClient() frontend.Client
	GetMatchingRawClient() matching.Client
	GetMatchingClient() matching.Client
	GetHistoryRawClient() history.Client
	GetHistoryClient() history.Client
	GetRatelimiterAggregatorsClient() qrpc.Client
	GetRemoteAdminClient(cluster string) admin.Client
	GetRemoteFrontendClient(cluster string) frontend.Client
	GetClientBean() client.Bean

	// persistence clients
	GetDomainManager() persistence.DomainManager
	GetTaskManager() persistence.TaskManager
	GetVisibilityManager() persistence.VisibilityManager
	GetShardManager() persistence.ShardManager
	GetHistoryManager() persistence.HistoryManager
	GetExecutionManager(int) (persistence.ExecutionManager, error)
	GetPersistenceBean() persistenceClient.Bean

	// GetHostName get host name
	GetHostName() string

	// loggers
	GetLogger() log.Logger
	GetThrottledLogger() log.Logger

	// for registering handlers
	GetDispatcher() *yarpc.Dispatcher

	// GetIsolationGroupState returns the isolationGroupState
	GetIsolationGroupState() isolationgroup.State
	GetIsolationGroupStore() configstore.Client

	GetAsyncWorkflowQueueProvider() queue.Provider
}
