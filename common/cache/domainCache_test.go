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

package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/testing/testdatagen"
	"github.com/uber/cadence/common/types"
)

type (
	domainCacheSuite struct {
		suite.Suite
		*require.Assertions

		metadataMgr *mocks.MetadataManager

		domainCache *DefaultDomainCache
		logger      log.Logger
	}
)

func TestDomainCacheSuite(t *testing.T) {
	s := new(domainCacheSuite)
	suite.Run(t, s)
}

func (s *domainCacheSuite) SetupSuite() {
}

func (s *domainCacheSuite) TearDownSuite() {

}

func (s *domainCacheSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.logger = testlogger.New(s.Suite.T())

	s.metadataMgr = &mocks.MetadataManager{}
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{})
	s.domainCache = NewDomainCache(s.metadataMgr, cluster.GetTestClusterMetadata(true), metricsClient, s.logger)

	s.domainCache.timeSource = clock.NewMockedTimeSource()
}

func (s *domainCacheSuite) TearDownTest() {
	s.domainCache.Stop()
	s.metadataMgr.AssertExpectations(s.T())
}

func (s *domainCacheSuite) TestListDomain() {
	domainNotificationVersion := int64(0)
	domainRecord1 := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: uuid.New(), Name: "some random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 1,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
			AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
				Enabled:             true,
				PredefinedQueueName: "test-async-wf-queue",
			},
			IsolationGroups: types.IsolationGroupConfiguration{
				"zone-1": {
					Name:  "zone-1",
					State: types.IsolationGroupStateDrained,
				},
				"zone-2": {
					Name:  "zone-2",
					State: types.IsolationGroupStateHealthy,
				},
			},
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		FailoverNotificationVersion: 0,
		NotificationVersion:         domainNotificationVersion,
	}
	entry1 := s.buildEntryFromRecord(domainRecord1)
	domainNotificationVersion++

	domainRecord2 := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: uuid.New(), Name: "another random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 2,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			}},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		FailoverNotificationVersion: 0,
		NotificationVersion:         domainNotificationVersion,
	}
	entry2 := s.buildEntryFromRecord(domainRecord2)
	domainNotificationVersion++

	domainRecord3 := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: uuid.New(), Name: "yet another random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 3,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			}},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		FailoverNotificationVersion: 0,
		NotificationVersion:         domainNotificationVersion,
	}
	// there is no domainNotificationVersion++ here
	// this is to test that if new domain change event happen during the pagination,
	// new change will not be loaded to domain cache

	pageToken := []byte("some random page token")

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: domainNotificationVersion}, nil)
	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: nil,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecord1},
		NextPageToken: pageToken,
	}, nil).Once()

	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: pageToken,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecord2, domainRecord3},
		NextPageToken: nil,
	}, nil).Once()

	// load domains
	s.domainCache.Start()
	defer s.domainCache.Stop()

	entryByName1, err := s.domainCache.GetDomain(domainRecord1.Info.Name)
	s.Nil(err)
	s.Equal(entry1, entryByName1)
	entryByID1, err := s.domainCache.GetDomainByID(domainRecord1.Info.ID)
	s.Nil(err)
	s.Equal(entry1, entryByID1)

	entryByName2, err := s.domainCache.GetDomain(domainRecord2.Info.Name)
	s.Nil(err)
	s.Equal(entry2, entryByName2)
	entryByID2, err := s.domainCache.GetDomainByID(domainRecord2.Info.ID)
	s.Nil(err)
	s.Equal(entry2, entryByID2)

	allDomains := s.domainCache.GetAllDomain()
	s.Equal(map[string]*DomainCacheEntry{
		entry1.GetInfo().ID: entry1,
		entry2.GetInfo().ID: entry2,
	}, allDomains)
}

func (s *domainCacheSuite) TestGetDomain_NonLoaded_GetByName() {
	domainNotificationVersion := int64(999999) // make this notification version really large for test
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: domainNotificationVersion}, nil)
	domainRecord := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: uuid.New(), Name: "some random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 1,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{
					"abc": {
						Reason:          "test reason",
						Operator:        "test operator",
						CreatedTimeNano: common.Int64Ptr(123),
					},
				},
			}},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	}
	entry := s.buildEntryFromRecord(domainRecord)

	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{Name: entry.info.Name}).Return(domainRecord, nil).Once()
	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: nil,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecord},
		NextPageToken: nil,
	}, nil).Once()

	entryByName, err := s.domainCache.GetDomain(domainRecord.Info.Name)
	s.Nil(err)
	s.Equal(entry, entryByName)
	entryByName, err = s.domainCache.GetDomain(domainRecord.Info.Name)
	s.Nil(err)
	s.Equal(entry, entryByName)
}

func (s *domainCacheSuite) TestGetDomain_NonLoaded_GetByID() {
	domainNotificationVersion := int64(999999) // make this notification version really large for test
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: domainNotificationVersion}, nil)
	domainRecord := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: uuid.New(), Name: "some random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 1,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	}
	entry := s.buildEntryFromRecord(domainRecord)

	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{ID: entry.info.ID}).Return(domainRecord, nil).Once()
	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: nil,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecord},
		NextPageToken: nil,
	}, nil).Once()

	entryByID, err := s.domainCache.GetDomainByID(domainRecord.Info.ID)
	s.Nil(err)
	s.Equal(entry, entryByID)
	entryByID, err = s.domainCache.GetDomainByID(domainRecord.Info.ID)
	s.Nil(err)
	s.Equal(entry, entryByID)
}

func Test_IsActiveIn(t *testing.T) {
	tests := []struct {
		msg              string
		isGlobalDomain   bool
		currentCluster   string
		activeCluster    string
		activeClusters   *types.ActiveClusters
		failoverDeadline *int64
		expectIsActive   bool
	}{
		{
			msg:            "local domain",
			isGlobalDomain: false,
			expectIsActive: true,
		},
		{
			msg:              "global pending active domain",
			isGlobalDomain:   true,
			failoverDeadline: common.Int64Ptr(time.Now().Unix()),
			expectIsActive:   false,
		},
		{
			msg:            "global domain on active cluster",
			isGlobalDomain: true,
			currentCluster: "A",
			activeCluster:  "A",
			expectIsActive: true,
		},
		{
			msg:            "global domain on passive cluster",
			isGlobalDomain: true,
			currentCluster: "A",
			activeCluster:  "B",
			expectIsActive: false,
		},
		{
			msg:            "active-active domain on active cluster",
			isGlobalDomain: true,
			currentCluster: "A",
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"region0": {ActiveClusterName: "A"},
							"region1": {ActiveClusterName: "B"},
						},
					},
				},
			},
			expectIsActive: true,
		},
		{
			msg:            "active-active domain on domain level active cluster",
			isGlobalDomain: true,
			currentCluster: "A",
			activeCluster:  "A",
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"region0": {ActiveClusterName: "B"},
							"region1": {ActiveClusterName: "B"},
						},
					},
				},
			},
			expectIsActive: true,
		},
		{
			msg:            "active-active domain on passive cluster",
			isGlobalDomain: true,
			currentCluster: "C",
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"region0": {ActiveClusterName: "A"},
							"region1": {ActiveClusterName: "B"},
						},
					},
				},
			},
			expectIsActive: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			domain := NewDomainCacheEntryForTest(
				&persistence.DomainInfo{Name: "test-domain"},
				nil,
				tt.isGlobalDomain,
				&persistence.DomainReplicationConfig{
					ActiveClusterName: tt.activeCluster,
					ActiveClusters:    tt.activeClusters,
				},
				0,
				tt.failoverDeadline,
				0,
				0,
				0,
			)

			isActive := domain.IsActiveIn(tt.currentCluster)
			assert.Equal(t, tt.expectIsActive, isActive)
		})
	}
}

func (s *domainCacheSuite) TestRegisterCallback_CatchUp() {
	prepareCallbackInvoked := false
	callBackInvoked := false
	var entriesNotification []*DomainCacheEntry

	s.domainCache.RegisterDomainChangeCallback(
		"0",
		func(_ DomainCache, prepareCallback PrepareCallbackFn, callback CallbackFn) {
			prepareCallback()
			callback(entriesNotification)
		},
		func() {
			prepareCallbackInvoked = true
		},
		func(nextDomains []*DomainCacheEntry) {
			callBackInvoked = true
		},
	)

	s.True(prepareCallbackInvoked)
	s.True(callBackInvoked)
}

func (s *domainCacheSuite) TestUpdateCache_TriggerCallBack() {
	domainNotificationVersion := int64(0)
	domainRecord1Old := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: uuid.New(), Name: "some random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 1,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			}},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		ConfigVersion:               10,
		FailoverVersion:             11,
		FailoverNotificationVersion: 0,
		NotificationVersion:         domainNotificationVersion,
	}
	domainNotificationVersion++

	domainRecord2Old := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: uuid.New(), Name: "another random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 2,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			}},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		ConfigVersion:               20,
		FailoverVersion:             21,
		FailoverNotificationVersion: 0,
		NotificationVersion:         domainNotificationVersion,
	}
	domainNotificationVersion++

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: domainNotificationVersion}, nil).Once()
	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: nil,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecord1Old, domainRecord2Old},
		NextPageToken: nil,
	}, nil).Once()

	// load domains
	s.Nil(s.domainCache.refreshDomains())

	domainRecord2New := &persistence.GetDomainResponse{
		Info:   &*domainRecord2Old.Info,
		Config: &*domainRecord2Old.Config,
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName, // only this changed
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		ConfigVersion:               domainRecord2Old.ConfigVersion,
		FailoverVersion:             domainRecord2Old.FailoverVersion + 1,
		FailoverNotificationVersion: domainNotificationVersion,
		NotificationVersion:         domainNotificationVersion,
	}
	entry2New := s.buildEntryFromRecord(domainRecord2New)
	domainNotificationVersion++

	domainRecord1New := &persistence.GetDomainResponse{ // only the description changed
		Info:   &persistence.DomainInfo{ID: domainRecord1Old.Info.ID, Name: domainRecord1Old.Info.Name, Description: "updated description", Data: make(map[string]string)},
		Config: &*domainRecord2Old.Config,
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		ConfigVersion:               domainRecord1Old.ConfigVersion + 1,
		FailoverVersion:             domainRecord1Old.FailoverVersion,
		FailoverNotificationVersion: domainRecord1Old.FailoverNotificationVersion,
		NotificationVersion:         domainNotificationVersion,
	}
	entry1New := s.buildEntryFromRecord(domainRecord1New)
	domainNotificationVersion++

	prepareCallbackInvoked := false
	var entriesNew []*DomainCacheEntry
	s.domainCache.RegisterDomainChangeCallback(
		"0",
		func(domainCache DomainCache, prepareCallback PrepareCallbackFn, callback CallbackFn) {},
		func() {
			prepareCallbackInvoked = true
		},
		func(nextDomains []*DomainCacheEntry) {
			entriesNew = nextDomains
		},
	)
	s.False(prepareCallbackInvoked)
	s.Empty(entriesNew)

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: domainNotificationVersion}, nil).Once()
	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: nil,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecord1New, domainRecord2New},
		NextPageToken: nil,
	}, nil).Once()

	s.domainCache.timeSource.(clock.MockedTimeSource).Advance(domainCacheMinRefreshInterval)
	s.Nil(s.domainCache.refreshDomains())

	// the order matters here: the record 2 got updated first, thus with a lower notification version
	// the record 1 got updated later, thus a higher notification version.
	// making sure notifying from lower to higher version helps the shard to keep track the
	// domain change events
	s.True(prepareCallbackInvoked)
	s.Equal([]*DomainCacheEntry{entry2New, entry1New}, entriesNew)
}

func (s *domainCacheSuite) TestGetTriggerListAndUpdateCache_ConcurrentAccess() {
	domainNotificationVersion := int64(999999) // make this notification version really large for test
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: domainNotificationVersion}, nil)
	id := uuid.New()
	domainRecordOld := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: id, Name: "some random domain name", Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			Retention: 1,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			}},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		ConfigVersion:   0,
		FailoverVersion: 0,
	}
	entryOld := s.buildEntryFromRecord(domainRecordOld)

	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{ID: id}).Return(domainRecordOld, nil).Maybe()
	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: nil,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecordOld},
		NextPageToken: nil,
	}, nil).Once()

	coroutineCountGet := 1000
	waitGroup := &sync.WaitGroup{}
	startChan := make(chan struct{})
	testGetFn := func() {
		<-startChan
		entryNew, err := s.domainCache.GetDomainByID(id)
		s.Nil(err)
		// make the config version the same so we can easily compare those
		entryNew.configVersion = 0
		entryNew.failoverVersion = 0
		s.Equal(entryOld, entryNew)
		waitGroup.Done()
	}

	for i := 0; i < coroutineCountGet; i++ {
		waitGroup.Add(1)
		go testGetFn()
	}
	close(startChan)
	waitGroup.Wait()
}

func (s *domainCacheSuite) TestGetCacheSize() {
	testCache := newDomainCache()
	testCache.Put("testDomainID", &DomainCacheEntry{
		info: &persistence.DomainInfo{ID: "testDomainID", Name: "testDomain"},
	})

	s.domainCache.cacheByID.Store(testCache)

	s.domainCache.cacheNameToID.Store(testCache)

	byName, byID := s.domainCache.GetCacheSize()

	s.Equal(int64(1), byName)
	s.Equal(int64(1), byID)
}

func (s *domainCacheSuite) TestStart_Stop() {
	mockTimeSource := clock.NewMockedTimeSource()
	s.domainCache.timeSource = mockTimeSource

	s.domainCache.lastRefreshTime = mockTimeSource.Now()

	domainID := uuid.New()
	domainName := "some random domain name"

	s.Equal(domainCacheInitialized, s.domainCache.status)

	s.Equal(0, len(s.domainCache.GetAllDomain()))

	s.domainCache.Start()

	// testing noop
	s.domainCache.Start()

	mockTimeSource.BlockUntil(1)

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: 4}, nil).Once()

	domainResponse := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{ID: domainID, Name: domainName, Data: make(map[string]string)},
		Config: &persistence.DomainConfig{
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
		},
		FailoverVersion:     124,
		NotificationVersion: 3,
	}

	listDomainsResponse := &persistence.ListDomainsResponse{
		Domains: []*persistence.GetDomainResponse{domainResponse},
	}

	s.metadataMgr.On("ListDomains", mock.Anything, mock.Anything).Return(listDomainsResponse, nil).Once()

	mockTimeSource.Advance(DomainCacheRefreshInterval)

	s.Equal(domainCacheStarted, s.domainCache.status)

	// need to wait for the go routine to make progress and update the cache
	time.Sleep(200 * time.Millisecond)

	allDomains := s.domainCache.GetAllDomain()
	s.Equal(1, len(allDomains))
	s.Equal(domainID, allDomains[domainID].GetInfo().ID)
	s.Equal(int64(124), allDomains[domainID].GetFailoverVersion())

	s.domainCache.Stop()
	s.Equal(domainCacheStopped, s.domainCache.status)
}

func (s *domainCacheSuite) TestStart_Error() {
	mockLogger := log.NewMockLogger(gomock.NewController(s.T()))
	s.domainCache.logger = mockLogger

	s.Equal(domainCacheInitialized, s.domainCache.status)

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(nil, assert.AnError).Once()
	mockLogger.EXPECT().Fatal("Unable to initialize domain cache", gomock.Any()).Times(1)

	s.domainCache.Start()
}

func (s *domainCacheSuite) TestUnregisterDomainChangeCallback() {
	s.domainCache.prepareCallbacks = map[string]PrepareCallbackFn{
		"1": func() {},
	}
	s.domainCache.callbacks = map[string]CallbackFn{
		"1": func([]*DomainCacheEntry) {},
	}

	s.domainCache.UnregisterDomainChangeCallback("1")
	s.Empty(s.domainCache.prepareCallbacks)
	s.Empty(s.domainCache.callbacks)
}

func (s *domainCacheSuite) TestGetDomain_Error() {
	entry, err := s.domainCache.GetDomain("")
	s.Nil(entry)
	s.ErrorContains(err, "Domain name is empty")
}

func (s *domainCacheSuite) TestGetDomainByID_Error() {
	entry, err := s.domainCache.GetDomainByID("")
	s.Nil(entry)
	s.ErrorContains(err, "DomainID is empty.")
}

func (s *domainCacheSuite) TestGetDomainID() {
	entry, err := s.domainCache.GetDomainID("")
	s.Empty(entry)
	s.ErrorContains(err, "Domain name is empty")

	domainName := "testDomain"
	domainID := "testDomainID"

	testCache := newDomainCache()
	domainEntry := &DomainCacheEntry{
		info: &persistence.DomainInfo{ID: domainID, Name: domainName},
		config: &persistence.DomainConfig{
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
		},
		replicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
		},
		initialized: true,
	}
	testCache.Put(domainName, domainID)
	testCache.Put(domainID, domainEntry)

	s.domainCache.cacheByID.Store(testCache)

	s.domainCache.cacheNameToID.Store(testCache)

	entry, err = s.domainCache.GetDomainID(domainName)

	s.Nil(err)
	s.Equal(domainID, entry)
}

func (s *domainCacheSuite) TestGetDomainName() {
	domainName := "testDomain"
	domainID := "testDomainID"

	testCache := newDomainCache()
	domainEntry := &DomainCacheEntry{
		info: &persistence.DomainInfo{ID: domainID, Name: domainName},
		config: &persistence.DomainConfig{
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
		},
		replicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
		},
		initialized: true,
	}
	testCache.Put(domainName, domainID)
	testCache.Put(domainID, domainEntry)

	s.domainCache.cacheByID.Store(testCache)

	s.domainCache.cacheNameToID.Store(testCache)

	entry, err := s.domainCache.GetDomainName(domainID)

	s.Nil(err)
	s.Equal(domainName, entry)
}

func (s *domainCacheSuite) TestGetDomainName_Error() {
	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{Name: "", ID: ""}).Return(nil, assert.AnError).Once()

	entry, err := s.domainCache.GetDomainName("")
	s.Empty(entry)
	s.ErrorIs(err, assert.AnError)
}

func (s *domainCacheSuite) Test_updateIDToDomainCache_Error() {
	domainCacheEntry := &DomainCacheEntry{
		info: &persistence.DomainInfo{ID: "testDomainID", Name: "testDomain"},
	}
	newCache := NewMockCache(gomock.NewController(s.T()))

	newCache.EXPECT().PutIfNotExist("testDomainID", &DomainCacheEntry{}).Return(false, assert.AnError).Times(1)

	triggerCallback, entry, err := s.domainCache.updateIDToDomainCache(newCache, "testDomainID", domainCacheEntry)

	s.False(triggerCallback)
	s.Nil(entry)
	s.ErrorIs(err, assert.AnError)
}

func (s *domainCacheSuite) Test_getDomain_Error_checkDomainExists() {
	domainName := "testDomain"

	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{Name: domainName, ID: ""}).Return(nil, assert.AnError).Once()

	entry, err := s.domainCache.getDomain(domainName)

	s.Nil(entry)
	s.ErrorIs(err, assert.AnError)
}

func (s *domainCacheSuite) Test_getDomain_Error_refreshDomainsLocked() {
	domainName := "testDomain"

	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{Name: domainName, ID: ""}).Return(nil, nil).Once()
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(nil, assert.AnError).Once()

	entry, err := s.domainCache.getDomain(domainName)

	s.Nil(entry)
	s.ErrorIs(err, assert.AnError)
}

func (s *domainCacheSuite) Test_getDomain_cacheHitAfterRefreshLockLocked() {
	domainName := "testDomain"
	domainID := "testDomainID"

	testCache := newDomainCache()
	domainEntry := &DomainCacheEntry{
		info: &persistence.DomainInfo{ID: domainID, Name: domainName, Data: map[string]string{"k1": "v1"}},
		config: &persistence.DomainConfig{
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
		},
		replicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
		},
		initialized: true,
	}

	testCache.Put(domainName, domainID)
	testCache.Put(domainID, domainEntry)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer s.domainCache.refreshLock.Unlock()
		// to test the cache hit after refresh lock is locked, need to ensure that the domain cache is added after the first cache hit check
		// force a lock to ensure that the code will block on the lock, wait for the first cache hit check, add the domain cache
		// and then release the lock
		s.domainCache.refreshLock.Lock()
		wg.Done()
		time.Sleep(200 * time.Millisecond)
		s.domainCache.cacheByID.Store(testCache)
		s.domainCache.cacheNameToID.Store(testCache)
	}()

	wg.Wait()

	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{Name: domainName, ID: ""}).Return(nil, nil).Once()

	entry, err := s.domainCache.getDomain(domainName)

	s.Nil(err)
	// because the code makes deep copies, it's not possible to compare all the pointers directly
	s.Equal(domainEntry.info.Name, entry.info.Name)
	s.Equal(domainEntry.info.ID, entry.info.ID)
	s.Equal(1, len(entry.info.Data))
	s.Equal("v1", entry.info.Data["k1"])
}

func (s *domainCacheSuite) Test_getDomainByID_refreshDomainsLockedError() {
	domainID := "testDomainID"

	s.metadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{Name: "", ID: domainID}).Return(nil, nil).Once()
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(nil, assert.AnError).Once()

	entry, err := s.domainCache.getDomainByID(domainID, false)

	s.Nil(entry)
	s.ErrorIs(err, assert.AnError)
}

func (s *domainCacheSuite) Test_refreshLoop_domainCacheRefreshedError() {
	mockedTimeSource := clock.NewMockedTimeSource()

	s.domainCache.timeSource = mockedTimeSource

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(nil, assert.AnError).Once()

	go func() {
		mockedTimeSource.BlockUntil(1)
		mockedTimeSource.Advance(DomainCacheRefreshInterval)
		s.domainCache.shutdownChan <- struct{}{}
	}()

	s.domainCache.refreshLoop()
}

func (s *domainCacheSuite) Test_refreshDomainsLocked_IntervalTooShort() {
	mockedTimeSource := clock.NewMockedTimeSource()

	s.domainCache.timeSource = mockedTimeSource

	s.domainCache.lastRefreshTime = mockedTimeSource.Now()
	ctx := context.Background()

	err := s.domainCache.refreshDomainsLocked(ctx)
	s.NoError(err)
}

func (s *domainCacheSuite) Test_refreshDomains_ListDomainsNonRetryableError() {
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: 0}, nil).Once()
	s.metadataMgr.On("ListDomains", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

	err := s.domainCache.refreshDomains()
	s.ErrorIs(err, assert.AnError)
}

func (s *domainCacheSuite) Test_refreshDomains_ListDomainsRetryableError() {
	retryableError := &types.ServiceBusyError{
		Message: "Service is busy",
	}

	// We expect the metadataMgr to be called twice, once for the initial attempt and once for the retry
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: 0}, nil).Times(2)

	// First time return retryable error
	s.metadataMgr.On("ListDomains", mock.Anything, mock.Anything).Return(nil, retryableError).Once()

	// Second time return non-retryable error
	s.metadataMgr.On("ListDomains", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

	err := s.domainCache.refreshDomains()

	// We expect the error to be the first error
	s.ErrorIs(err, retryableError)
}

func (s *domainCacheSuite) TestDomainCacheEntry_Getters() {
	gen := testdatagen.New(s.T())

	entry := DomainCacheEntry{}

	gen.Fuzz(&entry)

	s.Equal(entry.info, entry.GetInfo())
	s.Equal(entry.config, entry.GetConfig())
	s.Equal(entry.replicationConfig, entry.GetReplicationConfig())
	s.Equal(entry.failoverNotificationVersion, entry.GetFailoverNotificationVersion())
	s.Equal(entry.notificationVersion, entry.GetNotificationVersion())
	s.Equal(entry.failoverVersion, entry.GetFailoverVersion())
	s.Equal(entry.previousFailoverVersion, entry.GetPreviousFailoverVersion())
	s.Equal(entry.failoverEndTime, entry.GetFailoverEndTime())
	s.Equal(entry.isGlobalDomain, entry.IsGlobalDomain())
	s.Equal(entry.configVersion, entry.GetConfigVersion())
}

func (s *domainCacheSuite) TestDomainCacheEntry_IsDomainPendingActive() {
	//	Local domain
	entry := CreateDomainCacheEntry("domainName")

	s.False(entry.IsDomainPendingActive())

	//	Global domain not pending active

	entry.isGlobalDomain = true

	s.False(entry.IsDomainPendingActive())

	//	Global domain pending active

	entry.failoverEndTime = common.Int64Ptr(time.Now().Unix() + 100)

	s.True(entry.IsDomainPendingActive())
}

func (s *domainCacheSuite) TestDomainCacheEntry_GetReplicationPolicy() {
	entry := &DomainCacheEntry{
		replicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "active",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "active"},
				{ClusterName: "standby"},
			},
		},
	}

	s.Equal(ReplicationPolicyOneCluster, entry.GetReplicationPolicy())

	entry.isGlobalDomain = true

	s.Equal(ReplicationPolicyMultiCluster, entry.GetReplicationPolicy())
}

func (s *domainCacheSuite) TestDomainCacheEntry_HasReplicationCluster() {
	entry := &DomainCacheEntry{
		replicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "active",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "active"},
				{ClusterName: "standby"},
			},
		},
	}

	s.True(entry.HasReplicationCluster("active"))
	s.True(entry.HasReplicationCluster("standby"))
	s.False(entry.HasReplicationCluster("other"))
}

func (s *domainCacheSuite) TestDomainCacheEntry_IsDeprecatedOrDeleted() {
	entry := &DomainCacheEntry{
		info: &persistence.DomainInfo{
			Status: persistence.DomainStatusDeprecated,
		},
	}

	s.True(entry.IsDeprecatedOrDeleted())

	entry.info.Status = persistence.DomainStatusDeleted

	s.True(entry.IsDeprecatedOrDeleted())

	entry.info.Status = persistence.DomainStatusRegistered

	s.False(entry.IsDeprecatedOrDeleted())
}

func (s *domainCacheSuite) buildEntryFromRecord(record *persistence.GetDomainResponse) *DomainCacheEntry {
	newEntry := &DomainCacheEntry{}
	newEntry.info = &*record.Info
	newEntry.config = &*record.Config
	newEntry.replicationConfig = &persistence.DomainReplicationConfig{
		ActiveClusterName: record.ReplicationConfig.ActiveClusterName,
	}
	for _, testCluster := range record.ReplicationConfig.Clusters {
		newEntry.replicationConfig.Clusters = append(newEntry.replicationConfig.Clusters, &*testCluster)
	}
	newEntry.configVersion = record.ConfigVersion
	newEntry.failoverVersion = record.FailoverVersion
	newEntry.isGlobalDomain = record.IsGlobalDomain
	newEntry.failoverNotificationVersion = record.FailoverNotificationVersion
	newEntry.notificationVersion = record.NotificationVersion
	newEntry.initialized = true
	return newEntry
}

func Test_GetRetentionDays(t *testing.T) {
	d := &DomainCacheEntry{
		info: &persistence.DomainInfo{
			Data: make(map[string]string),
		},
		config: &persistence.DomainConfig{
			Retention: 7,
		},
	}
	d.info.Data[SampleRetentionKey] = "30"
	d.info.Data[SampleRateKey] = "0"

	wid := uuid.New()
	rd := d.GetRetentionDays(wid)
	require.Equal(t, int32(7), rd)

	d.info.Data[SampleRateKey] = "1"
	rd = d.GetRetentionDays(wid)
	require.Equal(t, int32(30), rd)

	d.info.Data[SampleRetentionKey] = "invalid-value"
	rd = d.GetRetentionDays(wid)
	require.Equal(t, int32(7), rd) // fallback to normal retention

	d.info.Data[SampleRetentionKey] = "30"
	d.info.Data[SampleRateKey] = "invalid-value"
	rd = d.GetRetentionDays(wid)
	require.Equal(t, int32(7), rd) // fallback to normal retention

	wid = "3aef42a8-db0a-4a3b-b8b7-9829d74b4ebf"
	d.info.Data[SampleRetentionKey] = "30"
	d.info.Data[SampleRateKey] = "0.8"
	rd = d.GetRetentionDays(wid)
	require.Equal(t, int32(7), rd) // fallback to normal retention
	d.info.Data[SampleRateKey] = "0.9"
	rd = d.GetRetentionDays(wid)
	require.Equal(t, int32(30), rd)
}

func Test_IsSampledForLongerRetentionEnabled(t *testing.T) {
	d := &DomainCacheEntry{
		info: &persistence.DomainInfo{
			Data: make(map[string]string),
		},
		config: &persistence.DomainConfig{
			Retention: 7,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
		},
	}
	wid := uuid.New()
	require.False(t, d.IsSampledForLongerRetentionEnabled(wid))
	d.info.Data[SampleRetentionKey] = "30"
	d.info.Data[SampleRateKey] = "0"
	require.True(t, d.IsSampledForLongerRetentionEnabled(wid))
}

func Test_IsSampledForLongerRetention(t *testing.T) {
	d := &DomainCacheEntry{
		info: &persistence.DomainInfo{
			Data: make(map[string]string),
		},
		config: &persistence.DomainConfig{
			Retention: 7,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{},
			},
		},
	}
	wid := uuid.New()
	require.False(t, d.IsSampledForLongerRetention(wid))

	d.info.Data[SampleRetentionKey] = "30"
	d.info.Data[SampleRateKey] = "0"
	require.False(t, d.IsSampledForLongerRetention(wid))

	d.info.Data[SampleRateKey] = "1"
	require.True(t, d.IsSampledForLongerRetention(wid))

	d.info.Data[SampleRateKey] = "invalid-value"
	require.False(t, d.IsSampledForLongerRetention(wid))
}

func Test_WithTimeSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metadataMgr := &mocks.MetadataManager{}

	timeSource := clock.NewRealTimeSource()
	domainCache := NewDomainCache(metadataMgr, cluster.GetTestClusterMetadata(true), metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{}), log.NewNoop(), WithTimeSource(timeSource))

	assert.Equal(t, timeSource, domainCache.timeSource)
}

func Test_NewLocalDomainCacheEntryForTest(t *testing.T) {
	domain := NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{Name: "test-domain"}, nil, "targetCluster")
	assert.False(t, domain.IsGlobalDomain())
}

func Test_NewDomainNotActiveError(t *testing.T) {
	tests := []struct {
		msg            string
		domain         *DomainCacheEntry
		currentCluster string
		activeCluster  string
		expectedErr    *types.DomainNotActiveError
	}{
		{
			msg:            "local domain",
			domain:         NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{Name: "test-domain"}, nil, "targetCluster"),
			currentCluster: "currentCluster",
			activeCluster:  "targetCluster",
			expectedErr: &types.DomainNotActiveError{
				Message:        "Domain: test-domain is active in cluster: targetCluster, while current cluster currentCluster is a standby cluster.",
				DomainName:     "test-domain",
				CurrentCluster: "currentCluster",
				ActiveCluster:  "targetCluster",
			},
		},
		{
			msg:            "active-active domain",
			currentCluster: "cluster1",
			activeCluster:  "cluster2",
			domain: NewDomainCacheEntryForTest(
				&persistence.DomainInfo{Name: "test-domain"},
				nil,
				true,
				&persistence.DomainReplicationConfig{
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"region1": {ActiveClusterName: "cluster1"},
									"region2": {ActiveClusterName: "cluster2"},
								},
							},
						},
					},
				},
				0,
				nil,
				0,
				0,
				0,
			),
			expectedErr: &types.DomainNotActiveError{
				Message:        "Domain: test-domain is active in cluster(s): [cluster1 cluster2], while current cluster cluster1 is a standby cluster. Operation active cluster: cluster2",
				DomainName:     "test-domain",
				CurrentCluster: "cluster1",
				ActiveClusters: []string{"cluster1", "cluster2"},
				ActiveCluster:  "cluster2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := tt.domain.NewDomainNotActiveError(tt.currentCluster, tt.activeCluster)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func Test_getActiveClusters(t *testing.T) {
	tests := []struct {
		msg                    string
		replicationConfig      *persistence.DomainReplicationConfig
		expectedActiveClusters []string
	}{
		{
			msg: "active-passive domain",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: "active",
			},
			expectedActiveClusters: nil,
		},
		{
			msg: "active-active domain",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: "active",
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"region1": {ActiveClusterName: "cluster1"},
								"region2": {ActiveClusterName: "cluster2"},
							},
						},
					},
				},
			},
			expectedActiveClusters: []string{"cluster1", "cluster2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			activeClusters := getActiveClusters(tt.replicationConfig)
			assert.Equal(t, tt.expectedActiveClusters, activeClusters)
		})
	}
}

func Test_GetActiveClusterInfoByClusterAttribute(t *testing.T) {
	tests := []struct {
		name                  string
		entry                 *DomainCacheEntry
		clusterAttribute      *types.ClusterAttribute
		expectedActiveCluster *types.ActiveClusterInfo
		expectedFound         bool
	}{
		{
			name: "nil cluster attribute - returns domain-level active cluster info",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
				},
				failoverVersion: 100,
			},
			clusterAttribute: nil,
			expectedActiveCluster: &types.ActiveClusterInfo{
				ActiveClusterName: "domain-active-cluster",
				FailoverVersion:   100,
			},
			expectedFound: true,
		},
		{
			name: "nil active clusters - returns false",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
					ActiveClusters:    nil,
				},
				failoverVersion: 100,
			},
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			expectedActiveCluster: nil,
			expectedFound:         false,
		},
		{
			name: "nil attribute scopes - returns false",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: nil,
					},
				},
				failoverVersion: 100,
			},
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			expectedActiveCluster: nil,
			expectedFound:         false,
		},
		{
			name: "scope not found - returns false",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"datacenter": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"dc1": {
										ActiveClusterName: "cluster1",
										FailoverVersion:   200,
									},
								},
							},
						},
					},
				},
				failoverVersion: 100,
			},
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region", // different scope
				Name:  "us-west",
			},
			expectedActiveCluster: nil,
			expectedFound:         false,
		},
		{
			name: "attribute not found in scope - returns false",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-east": {
										ActiveClusterName: "cluster1",
										FailoverVersion:   200,
									},
								},
							},
						},
					},
				},
				failoverVersion: 100,
			},
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west", // different name
			},
			expectedActiveCluster: nil,
			expectedFound:         false,
		},
		{
			name: "successful lookup - returns cluster attribute info",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-west": {
										ActiveClusterName: "cluster-west",
										FailoverVersion:   300,
									},
									"us-east": {
										ActiveClusterName: "cluster-east",
										FailoverVersion:   250,
									},
								},
							},
						},
					},
				},
				failoverVersion: 100,
			},
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			expectedActiveCluster: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster-west",
				FailoverVersion:   300,
			},
			expectedFound: true,
		},
		{
			name: "multiple scopes - successful lookup in specific scope",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-west": {
										ActiveClusterName: "region-cluster-west",
										FailoverVersion:   300,
									},
								},
							},
							"datacenter": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-west": {
										ActiveClusterName: "dc-cluster-west",
										FailoverVersion:   400,
									},
								},
							},
						},
					},
				},
				failoverVersion: 100,
			},
			clusterAttribute: &types.ClusterAttribute{
				Scope: "datacenter",
				Name:  "us-west",
			},
			expectedActiveCluster: &types.ActiveClusterInfo{
				ActiveClusterName: "dc-cluster-west",
				FailoverVersion:   400,
			},
			expectedFound: true,
		},
		{
			name: "empty cluster attributes map in scope - returns false",
			entry: &DomainCacheEntry{
				replicationConfig: &persistence.DomainReplicationConfig{
					ActiveClusterName: "domain-active-cluster",
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{},
							},
						},
					},
				},
				failoverVersion: 100,
			},
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			expectedActiveCluster: nil,
			expectedFound:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualActiveCluster, actualFound := tt.entry.GetActiveClusterInfoByClusterAttribute(tt.clusterAttribute)

			assert.Equal(t, tt.expectedFound, actualFound)
			assert.Equal(t, tt.expectedActiveCluster, actualActiveCluster)
		})
	}
}
