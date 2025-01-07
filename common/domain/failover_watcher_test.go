// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

package domain

import (
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
)

type (
	failoverWatcherSuite struct {
		suite.Suite

		*require.Assertions
		controller *gomock.Controller

		mockDomainCache *cache.MockDomainCache
		timeSource      clock.TimeSource
		mockMetadataMgr *mocks.MetadataManager
		watcher         *failoverWatcherImpl
	}
)

func TestFailoverWatcherSuite(t *testing.T) {
	s := new(failoverWatcherSuite)
	suite.Run(t, s)
}

func (s *failoverWatcherSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

func (s *failoverWatcherSuite) TearDownSuite() {
}

func (s *failoverWatcherSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())

	s.mockDomainCache = cache.NewMockDomainCache(s.controller)
	s.timeSource = clock.NewMockedTimeSource()
	s.mockMetadataMgr = &mocks.MetadataManager{}

	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: 1,
	}, nil)

	logger := testlogger.New(s.T())
	scope := tally.NewTestScope("failover_test", nil)
	metricsClient := metrics.NewClient(scope, metrics.Frontend)
	s.watcher = NewFailoverWatcher(
		s.mockDomainCache,
		s.mockMetadataMgr,
		s.timeSource,
		dynamicconfig.GetDurationPropertyFn(10*time.Second),
		dynamicconfig.GetFloatPropertyFn(0.2),
		metricsClient,
		logger,
	).(*failoverWatcherImpl)
}

func (s *failoverWatcherSuite) TearDownTest() {
	s.controller.Finish()
	s.watcher.Stop()
}

func (s *failoverWatcherSuite) TestCleanPendingActiveState() {
	domainName := uuid.New()
	info := &persistence.DomainInfo{
		ID:          domainName,
		Name:        domainName,
		Status:      persistence.DomainStatusRegistered,
		Description: "some random description",
		OwnerEmail:  "some random email",
		Data:        nil,
	}
	domainConfig := &persistence.DomainConfig{
		Retention:  1,
		EmitMetric: true,
	}
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: "active",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: "active"},
		},
	}

	s.mockMetadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{
		ID: domainName,
	}).Return(&persistence.GetDomainResponse{
		Info:                        info,
		Config:                      domainConfig,
		ReplicationConfig:           replicationConfig,
		IsGlobalDomain:              true,
		ConfigVersion:               1,
		FailoverVersion:             1,
		FailoverNotificationVersion: 1,
		FailoverEndTime:             nil,
		NotificationVersion:         1,
	}, nil).Times(1)

	// does not have failover end time
	err := CleanPendingActiveState(s.mockMetadataMgr, domainName, 1, s.watcher.retryPolicy)
	s.NoError(err)

	s.mockMetadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{
		ID: domainName,
	}).Return(&persistence.GetDomainResponse{
		Info:                        info,
		Config:                      domainConfig,
		ReplicationConfig:           replicationConfig,
		IsGlobalDomain:              true,
		ConfigVersion:               1,
		FailoverVersion:             1,
		FailoverNotificationVersion: 1,
		FailoverEndTime:             common.Int64Ptr(1),
		NotificationVersion:         1,
	}, nil).Times(1)

	// does not match failover versions
	err = CleanPendingActiveState(s.mockMetadataMgr, domainName, 5, s.watcher.retryPolicy)
	s.NoError(err)

	s.mockMetadataMgr.On("UpdateDomain", mock.Anything, &persistence.UpdateDomainRequest{
		Info:                        info,
		Config:                      domainConfig,
		ReplicationConfig:           replicationConfig,
		ConfigVersion:               1,
		FailoverVersion:             2,
		FailoverNotificationVersion: 2,
		FailoverEndTime:             nil,
		NotificationVersion:         1,
	}).Return(nil).Times(1)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{
		ID: domainName,
	}).Return(&persistence.GetDomainResponse{
		Info:                        info,
		Config:                      domainConfig,
		ReplicationConfig:           replicationConfig,
		IsGlobalDomain:              true,
		ConfigVersion:               1,
		FailoverVersion:             2,
		FailoverNotificationVersion: 2,
		FailoverEndTime:             common.Int64Ptr(1),
		NotificationVersion:         1,
	}, nil).Times(1)

	err = CleanPendingActiveState(s.mockMetadataMgr, domainName, 2, s.watcher.retryPolicy)
	s.NoError(err)
}

func (s *failoverWatcherSuite) TestHandleFailoverTimeout() {
	domainName := uuid.New()
	info := &persistence.DomainInfo{
		ID:          domainName,
		Name:        domainName,
		Status:      persistence.DomainStatusRegistered,
		Description: "some random description",
		OwnerEmail:  "some random email",
		Data:        nil,
	}
	domainConfig := &persistence.DomainConfig{
		Retention:  1,
		EmitMetric: true,
	}
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: "active",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: "active"},
		},
	}
	endtime := common.Int64Ptr(s.timeSource.Now().UnixNano() - 1)

	s.mockMetadataMgr.On("GetDomain", mock.Anything, &persistence.GetDomainRequest{
		ID: domainName,
	}).Return(&persistence.GetDomainResponse{
		Info:                        info,
		Config:                      domainConfig,
		ReplicationConfig:           replicationConfig,
		IsGlobalDomain:              true,
		ConfigVersion:               1,
		FailoverVersion:             1,
		FailoverNotificationVersion: 1,
		FailoverEndTime:             endtime,
		NotificationVersion:         1,
	}, nil).Times(1)
	s.mockMetadataMgr.On("UpdateDomain", mock.Anything, &persistence.UpdateDomainRequest{
		Info:                        info,
		Config:                      domainConfig,
		ReplicationConfig:           replicationConfig,
		ConfigVersion:               1,
		FailoverVersion:             1,
		FailoverNotificationVersion: 1,
		FailoverEndTime:             nil,
		NotificationVersion:         1,
	}).Return(nil).Times(1)

	domainEntry := cache.NewDomainCacheEntryForTest(
		info,
		domainConfig,
		true,
		replicationConfig,
		1,
		endtime,
		0, 0, 0,
	)
	s.watcher.handleFailoverTimeout(domainEntry)
}

func (s *failoverWatcherSuite) TestStart() {
	s.Assertions.Equal(common.DaemonStatusInitialized, s.watcher.status)
	s.watcher.Start()
	s.Assertions.Equal(common.DaemonStatusStarted, s.watcher.status)

	// Verify that calling Start again does not change the status
	s.watcher.Start()
	s.Assertions.Equal(common.DaemonStatusStarted, s.watcher.status)
	s.watcher.Stop()
}

func (s *failoverWatcherSuite) TestIsUpdateDomainRetryable() {
	testCases := []struct {
		name      string
		inputErr  error
		wantRetry bool
	}{
		{"nil error", nil, true},
		{"non-nil error", errors.New("some error"), true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			retry := isUpdateDomainRetryable(tc.inputErr)
			s.Equal(tc.wantRetry, retry)
		})
	}
}

func (s *failoverWatcherSuite) TestRefreshDomainLoop() {

	domainName := "testDomain"
	domainID := uuid.New()
	failoverEndTime := common.Int64Ptr(time.Now().Add(-time.Hour).UnixNano()) // 1 hour in the past
	mockTimeSource, _ := s.timeSource.(clock.MockedTimeSource)

	domainInfo := &persistence.DomainInfo{ID: domainID, Name: domainName}
	domainConfig := &persistence.DomainConfig{Retention: 1, EmitMetric: true}
	replicationConfig := &persistence.DomainReplicationConfig{ActiveClusterName: "active", Clusters: []*persistence.ClusterReplicationConfig{{ClusterName: "active"}}}
	domainEntry := cache.NewDomainCacheEntryForTest(domainInfo, domainConfig, true, replicationConfig, 1, failoverEndTime, 0, 0, 0)

	domainsMap := map[string]*cache.DomainCacheEntry{domainID: domainEntry}
	s.mockDomainCache.EXPECT().GetAllDomain().Return(domainsMap).AnyTimes()

	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil).Maybe()

	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.AnythingOfType("*persistence.GetDomainRequest")).Return(&persistence.GetDomainResponse{
		Info:                        domainInfo,
		Config:                      domainConfig,
		ReplicationConfig:           replicationConfig,
		IsGlobalDomain:              true,
		ConfigVersion:               1,
		FailoverVersion:             1,
		FailoverNotificationVersion: 1,
		FailoverEndTime:             failoverEndTime,
		NotificationVersion:         1,
	}, nil).Once()

	s.mockMetadataMgr.On("UpdateDomain", mock.Anything, mock.Anything).Return(nil).Once()

	s.watcher.Start()

	// Delay to allow loop to start
	time.Sleep(1 * time.Second)
	mockTimeSource.Advance(12 * time.Second)
	// Now stop the watcher, which should trigger the shutdown case in refreshDomainLoop
	s.watcher.Stop()

	// Enough time for shutdown process to complete
	time.Sleep(1 * time.Second)

	s.mockMetadataMgr.AssertExpectations(s.T())
}
