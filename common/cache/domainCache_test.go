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
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"

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
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.History)
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
		failoverDeadline *int64
		expectIsActive   bool
		expectedErr      error
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
			expectedErr:      &types.DomainNotActiveError{Message: "Domain: test-domain is pending active in cluster: .", DomainName: "test-domain", CurrentCluster: "", ActiveCluster: ""},
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
			expectedErr:    &types.DomainNotActiveError{Message: "Domain: test-domain is active in cluster: B, while current cluster A is a standby cluster.", DomainName: "test-domain", CurrentCluster: "A", ActiveCluster: "B"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			domain := NewDomainCacheEntryForTest(
				&persistence.DomainInfo{Name: "test-domain"},
				nil,
				tt.isGlobalDomain,
				&persistence.DomainReplicationConfig{ActiveClusterName: tt.activeCluster},
				0,
				tt.failoverDeadline,
				0,
				0,
				0,
			)

			isActive, err := domain.IsActiveIn(tt.currentCluster)

			assert.Equal(t, tt.expectIsActive, isActive)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func (s *domainCacheSuite) TestRegisterCallback_CatchUp() {
	domainNotificationVersion := int64(0)
	domainRecord1 := &persistence.GetDomainResponse{
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
		ConfigVersion:               20,
		FailoverVersion:             21,
		FailoverNotificationVersion: 0,
		NotificationVersion:         domainNotificationVersion,
	}
	entry2 := s.buildEntryFromRecord(domainRecord2)
	domainNotificationVersion++

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: domainNotificationVersion}, nil).Once()
	s.metadataMgr.On("ListDomains", mock.Anything, &persistence.ListDomainsRequest{
		PageSize:      domainCacheRefreshPageSize,
		NextPageToken: nil,
	}).Return(&persistence.ListDomainsResponse{
		Domains:       []*persistence.GetDomainResponse{domainRecord1, domainRecord2},
		NextPageToken: nil,
	}, nil).Once()

	// load domains
	s.Nil(s.domainCache.refreshDomains())

	prepareCallbacckInvoked := false
	entriesNotification := []*DomainCacheEntry{}
	// we are not testing catching up, so make this really large
	currentDomainNotificationVersion := int64(0)
	s.domainCache.RegisterDomainChangeCallback(
		0,
		currentDomainNotificationVersion,
		func() {
			prepareCallbacckInvoked = true
		},
		func(nextDomains []*DomainCacheEntry) {
			s.Equal(2, len(nextDomains))
			entriesNotification = nextDomains
		},
	)

	// the order matters here, should be ordered by notification version
	s.True(prepareCallbacckInvoked)
	s.Equal([]*DomainCacheEntry{entry1, entry2}, entriesNotification)
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

	prepareCallbacckInvoked := false
	entriesNew := []*DomainCacheEntry{}
	// we are not testing catching up, so make this really large
	currentDomainNotificationVersion := int64(9999999)
	s.domainCache.RegisterDomainChangeCallback(
		0,
		currentDomainNotificationVersion,
		func() {
			prepareCallbacckInvoked = true
		},
		func(nextDomains []*DomainCacheEntry) {
			entriesNew = nextDomains
		},
	)
	s.False(prepareCallbacckInvoked)
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
	s.True(prepareCallbacckInvoked)
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
	mockLogger := &log.MockLogger{}
	s.domainCache.logger = mockLogger

	s.Equal(domainCacheInitialized, s.domainCache.status)

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(nil, assert.AnError).Once()
	mockLogger.On("Fatal", "Unable to initialize domain cache", mock.Anything).Once()

	s.domainCache.Start()
}

func (s *domainCacheSuite) TestUnregisterDomainChangeCallback() {
	s.domainCache.prepareCallbacks = map[int]PrepareCallbackFn{
		1: func() {},
	}
	s.domainCache.callbacks = map[int]CallbackFn{
		1: func([]*DomainCacheEntry) {},
	}

	s.domainCache.UnregisterDomainChangeCallback(1)
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

	s.metadataMgr.On("GetMetadata", mock.Anything).Return(nil, assert.AnError).Twice()

	go func() {
		mockedTimeSource.BlockUntil(1)
		mockedTimeSource.Advance(DomainCacheRefreshInterval)
		mockedTimeSource.BlockUntil(2)
		mockedTimeSource.Advance(DomainCacheRefreshFailureRetryInterval)
		s.domainCache.shutdownChan <- struct{}{}
	}()

	s.domainCache.refreshLoop()
}

func (s *domainCacheSuite) Test_refreshDomainsLocked_IntervalTooShort() {
	mockedTimeSource := clock.NewMockedTimeSource()

	s.domainCache.timeSource = mockedTimeSource

	s.domainCache.lastRefreshTime = mockedTimeSource.Now()

	err := s.domainCache.refreshDomainsLocked()
	s.NoError(err)
}

func (s *domainCacheSuite) Test_refreshDomainsLocked_ListDomainsError() {
	s.metadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: 0}, nil).Once()
	s.metadataMgr.On("ListDomains", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

	err := s.domainCache.refreshDomainsLocked()
	s.ErrorIs(err, assert.AnError)
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
	for _, cluster := range record.ReplicationConfig.Clusters {
		newEntry.replicationConfig.Clusters = append(newEntry.replicationConfig.Clusters, &*cluster)
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

func Test_GetActiveDomainByID(t *testing.T) {
	nonExistingUUID := uuid.New()
	activeDomainUUID := uuid.New()
	passiveDomainUUID := uuid.New()

	activeDomain := NewGlobalDomainCacheEntryForTest(&persistence.DomainInfo{ID: activeDomainUUID, Name: "active"}, nil, &persistence.DomainReplicationConfig{ActiveClusterName: "A"}, 0)
	passiveDomain := NewGlobalDomainCacheEntryForTest(&persistence.DomainInfo{ID: passiveDomainUUID, Name: "passive"}, nil, &persistence.DomainReplicationConfig{ActiveClusterName: "B"}, 0)

	tests := []struct {
		msg          string
		domainID     string
		expectDomain *DomainCacheEntry
		expectedErr  error
	}{
		{
			msg:         "invalid UUID",
			domainID:    "invalid",
			expectedErr: &types.BadRequestError{Message: "Invalid domain UUID."},
		},
		{
			msg:         "non existing domain",
			domainID:    nonExistingUUID,
			expectedErr: assert.AnError,
		},
		{
			msg:          "active domain",
			domainID:     activeDomainUUID,
			expectDomain: activeDomain,
		},
		{
			msg:          "passive domain",
			domainID:     passiveDomainUUID,
			expectDomain: passiveDomain,
			expectedErr:  &types.DomainNotActiveError{Message: "Domain: passive is active in cluster: B, while current cluster A is a standby cluster.", DomainName: "passive", CurrentCluster: "A", ActiveCluster: "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cache := NewMockDomainCache(ctrl)
			cache.EXPECT().GetDomainByID(nonExistingUUID).Return(nil, assert.AnError).AnyTimes()
			cache.EXPECT().GetDomainByID(activeDomainUUID).Return(activeDomain, nil).AnyTimes()
			cache.EXPECT().GetDomainByID(passiveDomainUUID).Return(passiveDomain, nil).AnyTimes()

			domain, err := GetActiveDomainByID(cache, "A", tt.domainID)

			assert.Equal(t, tt.expectDomain, domain)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func Test_WithTimeSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metadataMgr := &mocks.MetadataManager{}

	timeSource := clock.NewRealTimeSource()
	domainCache := NewDomainCache(metadataMgr, cluster.GetTestClusterMetadata(true), metrics.NewClient(tally.NoopScope, metrics.History), log.NewNoop(), WithTimeSource(timeSource))

	assert.Equal(t, timeSource, domainCache.timeSource)
}

func Test_NewLocalDomainCacheEntryForTest(t *testing.T) {
	domain := NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{Name: "test-domain"}, nil, "targetCluster")
	assert.False(t, domain.IsGlobalDomain())
}
