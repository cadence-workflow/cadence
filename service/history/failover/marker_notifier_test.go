// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package failover

import (
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

type (
	markerNotifierSuite struct {
		suite.Suite
		*require.Assertions

		controller      *gomock.Controller
		coordinator     *MockCoordinator
		mockShard       *shard.TestContext
		mockDomainCache *cache.MockDomainCache
		clusterMetadata cluster.Metadata
		markerNotifier  *markerNotifierImpl
	}
)

func TestMarkerNotifierSuite(t *testing.T) {
	s := new(markerNotifierSuite)
	suite.Run(t, s)
}

func (s *markerNotifierSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())

	config := config.NewForTest()
	config.NotifyFailoverMarkerInterval = dynamicproperties.GetDurationPropertyFn(time.Millisecond)
	s.coordinator = NewMockCoordinator(s.controller)
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          10,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config,
	)
	s.clusterMetadata = s.mockShard.Resource.ClusterMetadata
	mockShardManager := s.mockShard.Resource.ShardMgr
	mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
	s.mockDomainCache = s.mockShard.Resource.DomainCache

	s.markerNotifier = NewMarkerNotifier(
		s.mockShard,
		config,
		s.coordinator,
	).(*markerNotifierImpl)
}

func (s *markerNotifierSuite) TearDownTest() {
	s.controller.Finish()
	s.markerNotifier.Stop()
}

func (s *markerNotifierSuite) TestNotifyPendingFailoverMarker_Shutdown() {
	close(s.markerNotifier.shutdownCh)
	s.coordinator.EXPECT().NotifyFailoverMarkers(gomock.Any(), gomock.Any()).Times(0)
	s.markerNotifier.notifyPendingFailoverMarker()
}

func (s *markerNotifierSuite) TestNotifyPendingFailoverMarker() {
	domainID := uuid.New()
	info := &persistence.DomainInfo{
		ID:          domainID,
		Name:        domainID,
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
		ActiveClusterName: s.clusterMetadata.GetCurrentClusterName(),
		Clusters: []*persistence.ClusterReplicationConfig{
			{
				ClusterName: s.clusterMetadata.GetCurrentClusterName(),
			},
		},
	}
	endTime := common.Int64Ptr(time.Now().UnixNano())
	domainEntry := cache.NewDomainCacheEntryForTest(
		info,
		domainConfig,
		true,
		replicationConfig,
		1,
		endTime,
		0, 0, 0,
	)
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil).AnyTimes()
	task := &types.FailoverMarkerAttributes{
		DomainID:        domainID,
		FailoverVersion: 1,
		CreationTime:    common.Int64Ptr(1),
	}
	tasks := []*types.FailoverMarkerAttributes{task}
	respCh := make(chan error, 1)
	err := s.mockShard.AddingPendingFailoverMarker(task)
	s.NoError(err)

	count := 0
	s.coordinator.EXPECT().NotifyFailoverMarkers(
		int32(s.mockShard.GetShardID()),
		tasks,
	).Do(
		func(
			shardID int32,
			markers []*types.FailoverMarkerAttributes,
		) {
			if count == 0 {
				count++
				respCh <- nil
			}
			if count == 1 {
				close(s.markerNotifier.shutdownCh)
			}
		},
	)

	s.markerNotifier.notifyPendingFailoverMarker()
}
