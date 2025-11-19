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

package domain

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	dc "github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	persistencetests "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	"github.com/uber/cadence/common/types"
)

type (
	domainHandlerCommonSuite struct {
		*persistencetests.TestBase

		minRetentionDays       int
		maxBadBinaryCount      int
		failoverHistoryMaxSize int

		domainManager          persistence.DomainManager
		mockProducer           *mocks.KafkaProducer
		mockDomainReplicator   Replicator
		archivalMetadata       archiver.ArchivalMetadata
		mockArchiverProvider   *provider.MockArchiverProvider
		mockDomainAuditManager persistence.DomainAuditManager

		handler *handlerImpl
	}
)

var nowInt64 = time.Now().UnixNano()

func TestDomainHandlerCommonSuite(t *testing.T) {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}

	s := new(domainHandlerCommonSuite)
	s.setupTestBase(t)
	suite.Run(t, s)
}

func (s *domainHandlerCommonSuite) setupTestBase(t *testing.T) {
	sqliteTestBaseOptions := sqlite.GetTestClusterOption()
	sqliteTestBaseOptions.ClusterMetadata = cluster.GetTestClusterMetadata(true)
	s.TestBase = persistencetests.NewTestBaseWithSQL(t, sqliteTestBaseOptions)
	s.Setup()
}

func (s *domainHandlerCommonSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

func (s *domainHandlerCommonSuite) SetupTest() {
	s.setupTestBase(s.T())

	logger := s.Logger
	dcCollection := dc.NewCollection(dc.NewNopClient(), logger)
	s.minRetentionDays = 1
	s.maxBadBinaryCount = 10
	s.failoverHistoryMaxSize = 5
	s.domainManager = s.TestBase.DomainManager
	s.mockProducer = &mocks.KafkaProducer{}
	s.mockDomainReplicator = NewDomainReplicator(s.mockProducer, logger)
	s.mockDomainAuditManager = persistence.NewMockDomainAuditManager(s.Controller)
	s.archivalMetadata = archiver.NewArchivalMetadata(
		dcCollection,
		"",
		false,
		"",
		false,
		&config.ArchivalDomainDefaults{},
	)
	s.mockArchiverProvider = provider.NewMockArchiverProvider(s.Controller)
	domainConfig := Config{
		MinRetentionDays:         dynamicproperties.GetIntPropertyFn(s.minRetentionDays),
		MaxBadBinaryCount:        dynamicproperties.GetIntPropertyFilteredByDomain(s.maxBadBinaryCount),
		FailoverCoolDown:         dynamicproperties.GetDurationPropertyFnFilteredByDomain(0 * time.Second),
		FailoverHistoryMaxSize:   dynamicproperties.GetIntPropertyFilteredByDomain(s.failoverHistoryMaxSize),
		EnableDomainAuditLogging: dynamicproperties.GetBoolPropertyFn(false),
	}
	s.handler = NewHandler(
		domainConfig,
		logger,
		s.domainManager,
		s.mockDomainAuditManager,
		s.ClusterMetadata,
		s.mockDomainReplicator,
		s.archivalMetadata,
		s.mockArchiverProvider,
		clock.NewRealTimeSource(),
	).(*handlerImpl)
}

func (s *domainHandlerCommonSuite) TearDownTest() {
	s.mockProducer.AssertExpectations(s.T())
}

func (s *domainHandlerCommonSuite) TestMergeDomainData_Overriding() {
	out := s.handler.mergeDomainData(
		map[string]string{
			"k0": "v0",
		},
		map[string]string{
			"k0": "v2",
		},
	)

	assert.Equal(s.T(), map[string]string{
		"k0": "v2",
	}, out)
}

func (s *domainHandlerCommonSuite) TestMergeDomainData_Adding() {
	out := s.handler.mergeDomainData(
		map[string]string{
			"k0": "v0",
		},
		map[string]string{
			"k1": "v2",
		},
	)

	assert.Equal(s.T(), map[string]string{
		"k0": "v0",
		"k1": "v2",
	}, out)
}

func (s *domainHandlerCommonSuite) TestMergeDomainData_Merging() {
	out := s.handler.mergeDomainData(
		map[string]string{
			"k0": "v0",
		},
		map[string]string{
			"k0": "v1",
			"k1": "v2",
		},
	)

	assert.Equal(s.T(), map[string]string{
		"k0": "v1",
		"k1": "v2",
	}, out)
}

func (s *domainHandlerCommonSuite) TestMergeDomainData_Nil() {
	out := s.handler.mergeDomainData(
		nil,
		map[string]string{
			"k0": "v1",
			"k1": "v2",
		},
	)

	assert.Equal(s.T(), map[string]string{
		"k0": "v1",
		"k1": "v2",
	}, out)
}

// test merging bad binaries
func (s *domainHandlerCommonSuite) TestMergeBadBinaries_Overriding() {
	out := s.handler.mergeBadBinaries(
		map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason0"},
		},
		map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason2"},
		}, nowInt64,
	)

	assert.Equal(s.T(), types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason2", CreatedTimeNano: common.Int64Ptr(nowInt64)},
		},
	}, out)
}

func (s *domainHandlerCommonSuite) TestMergeBadBinaries_Adding() {
	out := s.handler.mergeBadBinaries(
		map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason0"},
		},
		map[string]*types.BadBinaryInfo{
			"k1": {Reason: "reason2"},
		}, nowInt64,
	)

	expected := types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason0"},
			"k1": {Reason: "reason2", CreatedTimeNano: common.Int64Ptr(nowInt64)},
		},
	}
	assert.Equal(s.T(), expected, out)
}

func (s *domainHandlerCommonSuite) TestMergeBadBinaries_Merging() {
	out := s.handler.mergeBadBinaries(
		map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason0"},
		},
		map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason1"},
			"k1": {Reason: "reason2"},
		}, nowInt64,
	)

	assert.Equal(s.T(), types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason1", CreatedTimeNano: common.Int64Ptr(nowInt64)},
			"k1": {Reason: "reason2", CreatedTimeNano: common.Int64Ptr(nowInt64)},
		},
	}, out)
}

func (s *domainHandlerCommonSuite) TestMergeBadBinaries_Nil() {
	out := s.handler.mergeBadBinaries(
		nil,
		map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason1"},
			"k1": {Reason: "reason2"},
		}, nowInt64,
	)

	assert.Equal(s.T(), types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"k0": {Reason: "reason1", CreatedTimeNano: common.Int64Ptr(nowInt64)},
			"k1": {Reason: "reason2", CreatedTimeNano: common.Int64Ptr(nowInt64)},
		},
	}, out)
}

func (s *domainHandlerCommonSuite) TestListDomain() {
	domainName1 := s.getRandomDomainName()
	description1 := "some random description 1"
	email1 := "some random email 1"
	retention1 := int32(1)
	emitMetric1 := true
	data1 := map[string]string{"some random key 1": "some random value 1"}
	isGlobalDomain1 := false
	activeClusterName1 := s.ClusterMetadata.GetCurrentClusterName()
	var cluster1 []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(s.ClusterMetadata.GetCurrentClusterName(), nil) {
		cluster1 = append(cluster1, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}
	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName1,
		Description:                            description1,
		OwnerEmail:                             email1,
		WorkflowExecutionRetentionPeriodInDays: retention1,
		EmitMetric:                             common.BoolPtr(emitMetric1),
		Data:                                   data1,
		IsGlobalDomain:                         isGlobalDomain1,
	})
	s.Nil(err)

	domainName2 := s.getRandomDomainName()
	description2 := "some random description 2"
	email2 := "some random email 2"
	retention2 := int32(2)
	emitMetric2 := false
	data2 := map[string]string{"some random key 2": "some random value 2"}
	isGlobalDomain2 := true
	activeClusterName2 := ""
	var cluster2 []*types.ClusterReplicationConfiguration
	for clusterName := range s.ClusterMetadata.GetEnabledClusterInfo() {
		if clusterName != s.ClusterMetadata.GetCurrentClusterName() {
			activeClusterName2 = clusterName
		}
		cluster2 = append(cluster2, &types.ClusterReplicationConfiguration{
			ClusterName: clusterName,
		})
	}
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	err = s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName2,
		Description:                            description2,
		OwnerEmail:                             email2,
		WorkflowExecutionRetentionPeriodInDays: retention2,
		EmitMetric:                             common.BoolPtr(emitMetric2),
		Clusters:                               cluster2,
		ActiveClusterName:                      activeClusterName2,
		Data:                                   data2,
		IsGlobalDomain:                         isGlobalDomain2,
	})
	s.Nil(err)

	domains := map[string]*types.DescribeDomainResponse{}
	pagesize := int32(1)
	var token []byte
	for doPaging := true; doPaging; doPaging = len(token) > 0 {
		resp, err := s.handler.ListDomains(context.Background(), &types.ListDomainsRequest{
			PageSize:      pagesize,
			NextPageToken: token,
		})
		s.Nil(err)
		token = resp.NextPageToken
		s.True(len(resp.Domains) <= int(pagesize))
		if len(resp.Domains) > 0 {
			s.NotEmpty(resp.Domains[0].DomainInfo.GetUUID())
			resp.Domains[0].DomainInfo.UUID = ""
			domains[resp.Domains[0].DomainInfo.GetName()] = resp.Domains[0]
		}
	}
	delete(domains, constants.SystemLocalDomainName)

	s.Equal(map[string]*types.DescribeDomainResponse{
		domainName1: {
			DomainInfo: &types.DomainInfo{
				Name:        domainName1,
				Status:      types.DomainStatusRegistered.Ptr(),
				Description: description1,
				OwnerEmail:  email1,
				Data:        data1,
				UUID:        "",
			},
			Configuration: &types.DomainConfiguration{
				WorkflowExecutionRetentionPeriodInDays: retention1,
				EmitMetric:                             emitMetric1,
				HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
				HistoryArchivalURI:                     "",
				VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
				VisibilityArchivalURI:                  "",
				BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
				IsolationGroups:                        &types.IsolationGroupConfiguration{},
				AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
			},
			ReplicationConfiguration: &types.DomainReplicationConfiguration{
				ActiveClusterName: activeClusterName1,
				Clusters:          cluster1,
			},
			FailoverVersion: constants.EmptyVersion,
			IsGlobalDomain:  isGlobalDomain1,
		},
		domainName2: {
			DomainInfo: &types.DomainInfo{
				Name:        domainName2,
				Status:      types.DomainStatusRegistered.Ptr(),
				Description: description2,
				OwnerEmail:  email2,
				Data:        data2,
				UUID:        "",
			},
			Configuration: &types.DomainConfiguration{
				WorkflowExecutionRetentionPeriodInDays: retention2,
				EmitMetric:                             emitMetric2,
				HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
				HistoryArchivalURI:                     "",
				VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
				VisibilityArchivalURI:                  "",
				BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
				IsolationGroups:                        &types.IsolationGroupConfiguration{},
				AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
			},
			ReplicationConfiguration: &types.DomainReplicationConfiguration{
				ActiveClusterName: activeClusterName2,
				Clusters:          cluster2,
			},
			FailoverVersion: s.ClusterMetadata.GetNextFailoverVersion(activeClusterName2, 0, "some-domain"),
			IsGlobalDomain:  isGlobalDomain2,
		},
	}, domains)
}

func (s *domainHandlerCommonSuite) TestRegisterDomain_InvalidRetentionPeriod() {
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   "random-domain-name",
		Description:                            "random domain name",
		WorkflowExecutionRetentionPeriodInDays: int32(0),
		IsGlobalDomain:                         false,
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.Equal(errInvalidRetentionPeriod, err)
}

func (s *domainHandlerCommonSuite) TestRegisterDomain_InvalidDomainName() {
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   "random-domain name!^",
		Description:                            "a domain with bad naming",
		WorkflowExecutionRetentionPeriodInDays: int32(3),
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.Equal(errInvalidDomainName, err)
}

func (s *domainHandlerCommonSuite) TestUpdateDomain_InvalidRetentionPeriod() {
	domain := "random-domain-name"
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(10),
		IsGlobalDomain:                         false,
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.NoError(err)

	updateRequest := &types.UpdateDomainRequest{
		Name:                                   domain,
		WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(int32(-1)),
	}
	_, err = s.handler.UpdateDomain(context.Background(), updateRequest)
	s.Equal(errInvalidRetentionPeriod, err)
}

func (s *domainHandlerCommonSuite) TestUpdateDomain_GracefulFailover_Success() {
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Twice()
	domain := uuid.New()
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(10),
		IsGlobalDomain:                         true,
		ActiveClusterName:                      "standby",
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: s.ClusterMetadata.GetCurrentClusterName()},
			{ClusterName: "standby"},
		},
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.NoError(err)
	resp1, _ := s.domainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{
		Name: domain,
	})
	s.Equal("standby", resp1.ReplicationConfig.ActiveClusterName)
	s.Equal(cluster.TestAlternativeClusterInitialFailoverVersion, resp1.FailoverVersion)

	updateRequest := &types.UpdateDomainRequest{
		Name:                     domain,
		ActiveClusterName:        common.StringPtr(s.ClusterMetadata.GetCurrentClusterName()),
		FailoverTimeoutInSeconds: common.Int32Ptr(100),
	}
	resp, err := s.handler.UpdateDomain(context.Background(), updateRequest)
	s.NoError(err)
	resp2, err := s.domainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{
		ID: resp.GetDomainInfo().GetUUID(),
	})
	s.NoError(err)
	s.NotNil(resp2.FailoverEndTime)
	s.Equal(cluster.TestFailoverVersionIncrement, resp2.FailoverVersion)
	s.Equal(cluster.TestAlternativeClusterInitialFailoverVersion, resp2.PreviousFailoverVersion)
}

func (s *domainHandlerCommonSuite) TestUpdateDomain_GracefulFailover_NotCurrentActiveCluster() {
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	domain := uuid.New()
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(10),
		IsGlobalDomain:                         true,
		ActiveClusterName:                      "active",
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: "active"},
			{ClusterName: "standby"},
		},
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.NoError(err)

	updateRequest := &types.UpdateDomainRequest{
		Name:                     domain,
		ActiveClusterName:        common.StringPtr("standby"),
		FailoverTimeoutInSeconds: common.Int32Ptr(100),
	}
	_, err = s.handler.UpdateDomain(context.Background(), updateRequest)
	s.Error(err)
}

func (s *domainHandlerCommonSuite) TestUpdateDomain_GracefulFailover_OngoingFailover() {
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Twice()
	domain := uuid.New()
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(10),
		IsGlobalDomain:                         true,
		ActiveClusterName:                      "standby",
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: s.ClusterMetadata.GetCurrentClusterName()},
			{ClusterName: "standby"},
		},
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.NoError(err)

	updateRequest := &types.UpdateDomainRequest{
		Name:                     domain,
		ActiveClusterName:        common.StringPtr(s.ClusterMetadata.GetCurrentClusterName()),
		FailoverTimeoutInSeconds: common.Int32Ptr(100),
	}
	_, err = s.handler.UpdateDomain(context.Background(), updateRequest)
	s.NoError(err)
	_, err = s.handler.UpdateDomain(context.Background(), updateRequest)
	s.Error(err)
}

func (s *domainHandlerCommonSuite) TestUpdateDomain_GracefulFailover_NoUpdateActiveCluster() {
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	domain := uuid.New()
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(10),
		IsGlobalDomain:                         true,
		ActiveClusterName:                      "standby",
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: s.ClusterMetadata.GetCurrentClusterName()},
			{ClusterName: "standby"},
		},
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.NoError(err)

	updateRequest := &types.UpdateDomainRequest{
		Name:                     domain,
		OwnerEmail:               common.StringPtr("test"),
		FailoverTimeoutInSeconds: common.Int32Ptr(100),
	}
	_, err = s.handler.UpdateDomain(context.Background(), updateRequest)
	s.Error(err)
}

func (s *domainHandlerCommonSuite) TestUpdateDomain_GracefulFailover_After_ForceFailover() {
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Times(3)
	domain := uuid.New()
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(10),
		IsGlobalDomain:                         true,
		ActiveClusterName:                      "standby",
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: s.ClusterMetadata.GetCurrentClusterName()},
			{ClusterName: "standby"},
		},
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.NoError(err)

	// Start graceful failover
	updateRequest := &types.UpdateDomainRequest{
		Name:                     domain,
		ActiveClusterName:        common.StringPtr(s.ClusterMetadata.GetCurrentClusterName()),
		FailoverTimeoutInSeconds: common.Int32Ptr(100),
	}
	resp, err := s.handler.UpdateDomain(context.Background(), updateRequest)
	s.NoError(err)

	// Force failover
	updateRequest = &types.UpdateDomainRequest{
		Name:              domain,
		ActiveClusterName: common.StringPtr(s.ClusterMetadata.GetCurrentClusterName()),
	}
	_, err = s.handler.UpdateDomain(context.Background(), updateRequest)
	s.NoError(err)
	resp2, err := s.domainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{
		ID: resp.GetDomainInfo().GetUUID(),
	})
	s.NoError(err)
	s.Nil(resp2.FailoverEndTime)
}

func (s *domainHandlerCommonSuite) TestUpdateDomain_ForceFailover_SameActiveCluster() {
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Twice()
	domain := uuid.New()
	registerRequest := &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(10),
		IsGlobalDomain:                         true,
		ActiveClusterName:                      "standby",
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: s.ClusterMetadata.GetCurrentClusterName()},
			{ClusterName: "standby"},
		},
	}
	err := s.handler.RegisterDomain(context.Background(), registerRequest)
	s.NoError(err)

	// Start graceful failover
	updateRequest := &types.UpdateDomainRequest{
		Name:              domain,
		ActiveClusterName: common.StringPtr("standby"),
	}
	_, err = s.handler.UpdateDomain(context.Background(), updateRequest)
	s.NoError(err)
}

func (s *domainHandlerCommonSuite) getRandomDomainName() string {
	return "domain" + uuid.New()
}

func TestHandlerImpl_UpdateIsolationGroups(t *testing.T) {

	t0 := int64(1565914445 * time.Second)
	t1 := int64(1685914445 * time.Second)
	mockedTimeSourceAt := clock.NewMockedTimeSourceAt(time.Unix(0, t1))

	info := persistence.DomainInfo{
		ID:   "10CF5859-C5CC-4CCC-888E-631F84D53F57",
		Name: "test",
	}

	domainConfig := persistence.DomainConfig{
		Retention:       10,
		IsolationGroups: nil,
	}
	replicationConfig := persistence.DomainReplicationConfig{
		ActiveClusterName: "cluster-1",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: "cluster-1"},
		},
	}

	ig := types.IsolationGroupConfiguration{
		"zone-1": {Name: "zone-1", State: types.IsolationGroupStateDrained},
	}

	tests := map[string]struct {
		in                types.UpdateDomainIsolationGroupsRequest
		managerAffordance func(m *persistence.MockDomainManager)
		expectedErr       error
	}{
		"successful update": {
			in: types.UpdateDomainIsolationGroupsRequest{
				Domain:          "test",
				IsolationGroups: ig,
			},
			managerAffordance: func(m *persistence.MockDomainManager) {

				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{
					NotificationVersion: 3,
				}, nil)

				m.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: "test"}).Return(&persistence.GetDomainResponse{
					Info:                        &info,
					Config:                      &domainConfig,
					ReplicationConfig:           &replicationConfig,
					IsGlobalDomain:              false,
					ConfigVersion:               1,
					FailoverVersion:             1,
					FailoverNotificationVersion: 1,
					PreviousFailoverVersion:     1,
					FailoverEndTime:             &t0,
					LastUpdatedTime:             t0,
					NotificationVersion:         1,
				}, nil)

				m.EXPECT().UpdateDomain(gomock.Any(), &persistence.UpdateDomainRequest{
					Info: &info,
					Config: &persistence.DomainConfig{
						Retention:       10,
						IsolationGroups: ig,
					},
					ReplicationConfig:           &replicationConfig,
					ConfigVersion:               2,
					FailoverVersion:             1,
					FailoverNotificationVersion: 1,
					PreviousFailoverVersion:     1,
					FailoverEndTime:             &t0,
					LastUpdatedTime:             t1,
					NotificationVersion:         3,
				})
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			domainMgrMock := persistence.NewMockDomainManager(ctrl)
			td.managerAffordance(domainMgrMock)

			producer := messaging.NewNoopProducer()
			replicator := NewDomainReplicator(producer, testlogger.New(t))

			handler := handlerImpl{
				domainManager:       domainMgrMock,
				clusterMetadata:     cluster.Metadata{},
				domainReplicator:    replicator,
				domainAttrValidator: nil,
				archivalMetadata:    nil,
				archiverProvider:    nil,
				timeSource:          mockedTimeSourceAt,
				config: Config{
					MinRetentionDays:  func(opts ...dynamicproperties.FilterOption) int { return 0 },
					MaxBadBinaryCount: func(string) int { return 3 },
					FailoverCoolDown:  func(string) time.Duration { return time.Second },
				},
				logger: testlogger.New(t),
			}

			err := handler.UpdateIsolationGroups(context.TODO(), td.in)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func TestHandlerImpl_UpdateAsyncWorkflowConfiguraton(t *testing.T) {
	t0 := int64(1565914445 * time.Second)
	t1 := int64(1685914445 * time.Second)
	mockedTimeSourceAt := clock.NewMockedTimeSourceAt(time.Unix(0, t1))

	info := persistence.DomainInfo{
		ID:   "10CF5859-C5CC-4CCC-888E-631F84D53F57",
		Name: "test",
	}

	domainConfig := persistence.DomainConfig{
		Retention: 10,
	}
	replicationConfig := persistence.DomainReplicationConfig{
		ActiveClusterName: "cluster-1",
		Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: "cluster-1"},
		},
	}

	asyncWFCfg := types.AsyncWorkflowConfiguration{
		Enabled:             true,
		PredefinedQueueName: "queue-name",
		QueueType:           "kafka",
		QueueConfig: &types.DataBlob{
			EncodingType: types.EncodingTypeJSON.Ptr(),
			Data:         []byte(`{"key":"value"}`),
		},
	}

	tests := map[string]struct {
		in                types.UpdateDomainAsyncWorkflowConfiguratonRequest
		managerAffordance func(m *persistence.MockDomainManager)
		expectedErr       error
	}{
		"successful update": {
			in: types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain:        "test",
				Configuration: &asyncWFCfg,
			},
			managerAffordance: func(m *persistence.MockDomainManager) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{
					NotificationVersion: 3,
				}, nil).Times(1)

				m.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: "test"}).Return(&persistence.GetDomainResponse{
					Info:                        &info,
					Config:                      &domainConfig,
					ReplicationConfig:           &replicationConfig,
					IsGlobalDomain:              false,
					ConfigVersion:               1,
					FailoverVersion:             1,
					FailoverNotificationVersion: 1,
					PreviousFailoverVersion:     1,
					FailoverEndTime:             &t0,
					LastUpdatedTime:             t0,
					NotificationVersion:         1,
				}, nil).Times(1)

				m.EXPECT().UpdateDomain(gomock.Any(), &persistence.UpdateDomainRequest{
					Info: &info,
					Config: &persistence.DomainConfig{
						Retention:           10,
						AsyncWorkflowConfig: asyncWFCfg,
					},
					ReplicationConfig:           &replicationConfig,
					ConfigVersion:               2,
					FailoverVersion:             1,
					FailoverNotificationVersion: 1,
					PreviousFailoverVersion:     1,
					FailoverEndTime:             &t0,
					LastUpdatedTime:             t1,
					NotificationVersion:         3,
				}).Times(1)
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			domainMgrMock := persistence.NewMockDomainManager(ctrl)
			td.managerAffordance(domainMgrMock)

			producer := messaging.NewNoopProducer()
			replicator := NewDomainReplicator(producer, testlogger.New(t))

			handler := handlerImpl{
				domainManager:    domainMgrMock,
				clusterMetadata:  cluster.Metadata{},
				domainReplicator: replicator,
				timeSource:       mockedTimeSourceAt,
				config: Config{
					MinRetentionDays:  func(opts ...dynamicproperties.FilterOption) int { return 0 },
					MaxBadBinaryCount: func(string) int { return 3 },
					FailoverCoolDown:  func(string) time.Duration { return time.Second },
				},
				logger: testlogger.New(t),
			}

			err := handler.UpdateAsyncWorkflowConfiguraton(context.Background(), td.in)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}
