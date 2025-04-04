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

package resource

import (
	"testing"

	oldgomock "github.com/golang/mock/gomock" // client library cannot change from the old gomock
	"github.com/stretchr/testify/mock"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	publicservicetest "go.uber.org/cadence/.gen/go/cadence/workflowservicetest"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
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
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
	"github.com/uber/cadence/common/quotas/global/rpc"
	"github.com/uber/cadence/common/taskvalidator"
)

type (
	// Test is the test implementation used for testing
	Test struct {
		MetricsScope    tally.TestScope
		ClusterMetadata cluster.Metadata

		// other common resources

		DomainCache             *cache.MockDomainCache
		DomainMetricsScopeCache cache.DomainMetricsScopeCache
		DomainReplicationQueue  *domain.MockReplicationQueue
		TimeSource              clock.TimeSource
		PayloadSerializer       persistence.PayloadSerializer
		MetricsClient           metrics.Client
		ArchivalMetadata        *archiver.MockArchivalMetadata
		ArchiverProvider        *provider.MockArchiverProvider
		BlobstoreClient         *blobstore.MockClient
		MockPayloadSerializer   *persistence.MockPayloadSerializer

		// membership infos
		MembershipResolver *membership.MockResolver

		// internal services clients

		SDKClient            *publicservicetest.MockClient
		FrontendClient       *frontend.MockClient
		MatchingClient       *matching.MockClient
		HistoryClient        *history.MockClient
		RemoteAdminClient    *admin.MockClient
		RemoteFrontendClient *frontend.MockClient
		ClientBean           *client.MockBean

		// persistence clients

		MetadataMgr     *mocks.MetadataManager
		TaskMgr         *mocks.TaskManager
		VisibilityMgr   *mocks.VisibilityManager
		ShardMgr        *mocks.ShardManager
		HistoryMgr      *mocks.HistoryV2Manager
		ExecutionMgr    *mocks.ExecutionManager
		PersistenceBean *persistenceClient.MockBean

		IsolationGroups     *isolationgroup.MockState
		IsolationGroupStore *configstore.MockClient
		HostName            string
		Logger              log.Logger
		taskvalidator       taskvalidator.Checker

		AsyncWorkflowQueueProvider *queue.MockProvider

		RatelimiterAggregatorClient rpc.Client
	}
)

var _ Resource = (*Test)(nil)

const (
	testHostName = "test_host"
)

var (
	testHostInfo = membership.NewHostInfo(testHostName)
)

// NewTest returns a new test resource instance
func NewTest(
	t *testing.T,
	controller *gomock.Controller,
	serviceMetricsIndex metrics.ServiceIdx,
) *Test {
	logger := testlogger.New(t)

	frontendClient := frontend.NewMockClient(controller)
	matchingClient := matching.NewMockClient(controller)
	historyClient := history.NewMockClient(controller)
	remoteAdminClient := admin.NewMockClient(controller)
	remoteFrontendClient := frontend.NewMockClient(controller)
	clientBean := client.NewMockBean(controller)
	clientBean.EXPECT().GetFrontendClient().Return(frontendClient).AnyTimes()
	clientBean.EXPECT().GetMatchingClient(gomock.Any()).Return(matchingClient, nil).AnyTimes()
	clientBean.EXPECT().GetHistoryClient().Return(historyClient).AnyTimes()
	clientBean.EXPECT().GetRemoteAdminClient(gomock.Any()).Return(remoteAdminClient).AnyTimes()
	clientBean.EXPECT().GetRemoteFrontendClient(gomock.Any()).Return(remoteFrontendClient).AnyTimes()

	metadataMgr := &mocks.MetadataManager{}
	taskMgr := &mocks.TaskManager{}
	visibilityMgr := &mocks.VisibilityManager{}
	shardMgr := &mocks.ShardManager{}
	historyMgr := &mocks.HistoryV2Manager{}
	executionMgr := &mocks.ExecutionManager{}
	domainReplicationQueue := domain.NewMockReplicationQueue(controller)
	domainReplicationQueue.EXPECT().Start().AnyTimes()
	domainReplicationQueue.EXPECT().Stop().AnyTimes()
	persistenceBean := persistenceClient.NewMockBean(controller)
	persistenceBean.EXPECT().GetDomainManager().Return(metadataMgr).AnyTimes()
	persistenceBean.EXPECT().GetTaskManager().Return(taskMgr).AnyTimes()
	persistenceBean.EXPECT().GetVisibilityManager().Return(visibilityMgr).AnyTimes()
	persistenceBean.EXPECT().GetHistoryManager().Return(historyMgr).AnyTimes()
	persistenceBean.EXPECT().GetShardManager().Return(shardMgr).AnyTimes()
	persistenceBean.EXPECT().GetExecutionManager(gomock.Any()).Return(executionMgr, nil).AnyTimes()

	isolationGroupMock := isolationgroup.NewMockState(controller)
	isolationGroupMock.EXPECT().Stop().AnyTimes()

	scope := tally.NewTestScope("test", nil)

	asyncWorkflowQueueProvider := queue.NewMockProvider(controller)

	return &Test{
		MetricsScope: scope,

		// By default tests will run on active cluster unless overridden otherwise
		ClusterMetadata: cluster.TestActiveClusterMetadata,

		// other common resources

		DomainCache:             cache.NewMockDomainCache(controller),
		DomainMetricsScopeCache: cache.NewDomainMetricsScopeCache(),
		DomainReplicationQueue:  domainReplicationQueue,
		TimeSource:              clock.NewRealTimeSource(),
		PayloadSerializer:       persistence.NewPayloadSerializer(),
		MetricsClient:           metrics.NewClient(scope, serviceMetricsIndex),
		ArchivalMetadata:        &archiver.MockArchivalMetadata{},
		ArchiverProvider:        &provider.MockArchiverProvider{},
		BlobstoreClient:         &blobstore.MockClient{},
		MockPayloadSerializer:   persistence.NewMockPayloadSerializer(controller),

		// membership infos
		MembershipResolver: membership.NewMockResolver(controller),

		// internal services clients

		SDKClient:            publicservicetest.NewMockClient(oldgomock.NewController(t)),
		FrontendClient:       frontendClient,
		MatchingClient:       matchingClient,
		HistoryClient:        historyClient,
		RemoteAdminClient:    remoteAdminClient,
		RemoteFrontendClient: remoteFrontendClient,
		ClientBean:           clientBean,

		// persistence clients

		MetadataMgr:     metadataMgr,
		TaskMgr:         taskMgr,
		VisibilityMgr:   visibilityMgr,
		ShardMgr:        shardMgr,
		HistoryMgr:      historyMgr,
		ExecutionMgr:    executionMgr,
		PersistenceBean: persistenceBean,
		IsolationGroups: isolationGroupMock,
		// logger

		Logger: logger,

		AsyncWorkflowQueueProvider: asyncWorkflowQueueProvider,

		RatelimiterAggregatorClient: nil, // TODO: not currently used
	}
}

// Start for testing
func (s *Test) Start() {

}

// Stop for testing
func (s *Test) Stop() {

}

// static infos

// GetServiceName for testing
func (s *Test) GetServiceName() string {
	panic("user should implement this method for test")
}

// GetHostInfo for testing
func (s *Test) GetHostInfo() membership.HostInfo {
	return testHostInfo
}

// GetClusterMetadata for testing
func (s *Test) GetClusterMetadata() cluster.Metadata {
	return s.ClusterMetadata
}

// other common resources

// GetDomainCache for testing
func (s *Test) GetDomainCache() cache.DomainCache {
	return s.DomainCache
}

// GetDomainMetricsScopeCache for testing
func (s *Test) GetDomainMetricsScopeCache() cache.DomainMetricsScopeCache {
	return s.DomainMetricsScopeCache
}

// GetDomainReplicationQueue for testing
func (s *Test) GetDomainReplicationQueue() domain.ReplicationQueue {
	// user should implement this method for test
	return s.DomainReplicationQueue
}

// GetTimeSource for testing
func (s *Test) GetTimeSource() clock.TimeSource {
	return s.TimeSource
}

// GetPayloadSerializer for testing
func (s *Test) GetPayloadSerializer() persistence.PayloadSerializer {
	return s.PayloadSerializer
}

// GetPayloadSerializer for testing
func (s *Test) GetTaskValidator() taskvalidator.Checker {
	return s.taskvalidator
}

// GetMetricsClient for testing
func (s *Test) GetMetricsClient() metrics.Client {
	return s.MetricsClient
}

// GetMessagingClient for testing
func (s *Test) GetMessagingClient() messaging.Client {
	panic("user should implement this method for test")
}

// GetBlobstoreClient for testing
func (s *Test) GetBlobstoreClient() blobstore.Client {
	return s.BlobstoreClient
}

// GetArchivalMetadata for testing
func (s *Test) GetArchivalMetadata() archiver.ArchivalMetadata {
	return s.ArchivalMetadata
}

// GetArchiverProvider for testing
func (s *Test) GetArchiverProvider() provider.ArchiverProvider {
	return s.ArchiverProvider
}

// GetMembershipResolver for testing
func (s *Test) GetMembershipResolver() membership.Resolver {
	return s.MembershipResolver
}

// internal services clients

// GetSDKClient for testing
func (s *Test) GetSDKClient() workflowserviceclient.Interface {
	return s.SDKClient
}

// GetFrontendRawClient for testing
func (s *Test) GetFrontendRawClient() frontend.Client {
	return s.FrontendClient
}

// GetFrontendClient for testing
func (s *Test) GetFrontendClient() frontend.Client {
	return s.FrontendClient
}

// GetMatchingRawClient for testing
func (s *Test) GetMatchingRawClient() matching.Client {
	return s.MatchingClient
}

// GetMatchingClient for testing
func (s *Test) GetMatchingClient() matching.Client {
	return s.MatchingClient
}

// GetHistoryRawClient for testing
func (s *Test) GetHistoryRawClient() history.Client {
	return s.HistoryClient
}

// GetHistoryClient for testing
func (s *Test) GetHistoryClient() history.Client {
	return s.HistoryClient
}

// GetRemoteAdminClient for testing
func (s *Test) GetRemoteAdminClient(
	cluster string,
) admin.Client {

	return s.RemoteAdminClient
}

// GetRemoteFrontendClient for testing
func (s *Test) GetRemoteFrontendClient(
	cluster string,
) frontend.Client {

	return s.RemoteFrontendClient
}

// GetClientBean for testing
func (s *Test) GetClientBean() client.Bean {
	return s.ClientBean
}

// persistence clients

// GetMetadataManager for testing
func (s *Test) GetDomainManager() persistence.DomainManager {
	return s.MetadataMgr
}

// GetTaskManager for testing
func (s *Test) GetTaskManager() persistence.TaskManager {
	return s.TaskMgr
}

// GetVisibilityManager for testing
func (s *Test) GetVisibilityManager() persistence.VisibilityManager {
	return s.VisibilityMgr
}

// GetShardManager for testing
func (s *Test) GetShardManager() persistence.ShardManager {
	return s.ShardMgr
}

// GetHistoryManager for testing
func (s *Test) GetHistoryManager() persistence.HistoryManager {
	return s.HistoryMgr
}

// GetExecutionManager for testing
func (s *Test) GetExecutionManager(
	shardID int,
) (persistence.ExecutionManager, error) {

	return s.ExecutionMgr, nil
}

// GetPersistenceBean for testing
func (s *Test) GetPersistenceBean() persistenceClient.Bean {
	return s.PersistenceBean
}

// GetHostName for testing
func (s *Test) GetHostName() string {
	return s.HostName
}

// loggers

// GetLogger for testing
func (s *Test) GetLogger() log.Logger {
	return s.Logger
}

// GetThrottledLogger for testing
func (s *Test) GetThrottledLogger() log.Logger {
	return s.Logger
}

// GetDispatcher for testing
func (s *Test) GetDispatcher() *yarpc.Dispatcher {
	panic("user should implement this method for test")
}

// GetIsolationGroupState returns the isolationGroupState for testing
func (s *Test) GetIsolationGroupState() isolationgroup.State {
	return s.IsolationGroups
}

// GetIsolationGroupStore returns the config store for their
// isolation-group stores
func (s *Test) GetIsolationGroupStore() configstore.Client {
	return s.IsolationGroupStore
}

func (s *Test) GetAsyncWorkflowQueueProvider() queue.Provider {
	return s.AsyncWorkflowQueueProvider
}

func (s *Test) GetRatelimiterAggregatorsClient() rpc.Client {
	return s.RatelimiterAggregatorClient
}

// Finish checks whether expectations are met
func (s *Test) Finish(
	t mock.TestingT,
) {
	s.ArchivalMetadata.AssertExpectations(t)
	s.ArchiverProvider.AssertExpectations(t)

	s.MetadataMgr.AssertExpectations(t)
	s.TaskMgr.AssertExpectations(t)
	s.VisibilityMgr.AssertExpectations(t)
	s.ShardMgr.AssertExpectations(t)
	s.HistoryMgr.AssertExpectations(t)
	s.ExecutionMgr.AssertExpectations(t)
}
