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
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	dc "github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	persistencetests "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	"github.com/uber/cadence/common/types"
)

type (
	domainHandlerGlobalDomainEnabledPrimaryClusterSuite struct {
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

func TestDomainHandlerGlobalDomainEnabledPrimaryClusterSuite(t *testing.T) {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}

	s := new(domainHandlerGlobalDomainEnabledPrimaryClusterSuite)
	s.setupTestBase(t)
	suite.Run(t, s)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) setupTestBase(t *testing.T) {
	sqliteTestBaseOptions := sqlite.GetTestClusterOption()
	sqliteTestBaseOptions.ClusterMetadata = cluster.GetTestClusterMetadata(true)
	s.TestBase = persistencetests.NewTestBaseWithSQL(t, sqliteTestBaseOptions)
	s.Setup()
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TearDownSuite() {
	s.TestBase.TearDownWorkflowStore()
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) SetupTest() {
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
	s.mockArchiverProvider = &provider.MockArchiverProvider{}
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
		clock.NewMockedTimeSource(),
	).(*handlerImpl)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TearDownTest() {
	s.mockProducer.AssertExpectations(s.T())
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestRegisterGetDomain_LocalDomain_InvalidCluster() {
	domainName := s.getRandomDomainName()
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	activeClusterName := cluster.TestAlternativeClusterName
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: activeClusterName,
		},
	}
	data := map[string]string{"some random key": "some random value"}
	isGlobalDomain := false

	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		Data:                                   data,
		IsGlobalDomain:                         isGlobalDomain,
	})
	s.IsType(&types.BadRequestError{}, err)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestRegisterGetDomain_LocalDomain_AllDefault() {
	domainName := s.getRandomDomainName()
	isGlobalDomain := false
	var clusters []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(s.ClusterMetadata.GetCurrentClusterName(), nil) {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}

	retention := int32(1)
	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		IsGlobalDomain:                         isGlobalDomain,
		WorkflowExecutionRetentionPeriodInDays: retention,
	})
	s.Nil(err)

	resp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)

	s.NotEmpty(resp.DomainInfo.GetUUID())
	resp.DomainInfo.UUID = ""
	s.Equal(&types.DomainInfo{
		Name:        domainName,
		Status:      types.DomainStatusRegistered.Ptr(),
		Description: "",
		OwnerEmail:  "",
		Data:        nil,
		UUID:        "",
	}, resp.DomainInfo)
	s.Equal(&types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             true,
		HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
		HistoryArchivalURI:                     "",
		VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
		VisibilityArchivalURI:                  "",
		BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:                        &types.IsolationGroupConfiguration{},
		AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
	}, resp.Configuration)
	s.Equal(&types.DomainReplicationConfiguration{
		ActiveClusterName: s.ClusterMetadata.GetCurrentClusterName(),
		Clusters:          clusters,
	}, resp.ReplicationConfiguration)
	s.Equal(constants.EmptyVersion, resp.GetFailoverVersion())
	s.Equal(isGlobalDomain, resp.GetIsGlobalDomain())
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestRegisterGetDomain_LocalDomain_NoDefault() {
	domainName := s.getRandomDomainName()
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	activeClusterName := cluster.TestCurrentClusterName
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: activeClusterName,
		},
	}
	data := map[string]string{"some random key": "some random value"}
	isGlobalDomain := false

	var expectedClusters []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(s.ClusterMetadata.GetCurrentClusterName(), nil) {
		expectedClusters = append(expectedClusters, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}

	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		Data:                                   data,
		IsGlobalDomain:                         isGlobalDomain,
	})
	s.Nil(err)

	resp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)

	s.NotEmpty(resp.DomainInfo.GetUUID())
	resp.DomainInfo.UUID = ""
	s.Equal(&types.DomainInfo{
		Name:        domainName,
		Status:      types.DomainStatusRegistered.Ptr(),
		Description: description,
		OwnerEmail:  email,
		Data:        data,
		UUID:        "",
	}, resp.DomainInfo)
	s.Equal(&types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             emitMetric,
		HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
		HistoryArchivalURI:                     "",
		VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
		VisibilityArchivalURI:                  "",
		BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:                        &types.IsolationGroupConfiguration{},
		AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
	}, resp.Configuration)
	s.Equal(&types.DomainReplicationConfiguration{
		ActiveClusterName: s.ClusterMetadata.GetCurrentClusterName(),
		Clusters:          expectedClusters,
	}, resp.ReplicationConfiguration)
	s.Equal(constants.EmptyVersion, resp.GetFailoverVersion())
	s.Equal(isGlobalDomain, resp.GetIsGlobalDomain())
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestUpdateGetDomain_LocalDomain_NoAttrSet() {
	domainName := s.getRandomDomainName()
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	data := map[string]string{"some random key": "some random value"}
	var clusters []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(s.ClusterMetadata.GetCurrentClusterName(), nil) {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}
	isGlobalDomain := false

	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      s.ClusterMetadata.GetCurrentClusterName(),
		Data:                                   data,
		IsGlobalDomain:                         isGlobalDomain,
	})
	s.Nil(err)

	fnTest := func(info *types.DomainInfo, config *types.DomainConfiguration,
		replicationConfig *types.DomainReplicationConfiguration, isGlobalDomain bool, failoverVersion int64) {
		s.NotEmpty(info.GetUUID())
		info.UUID = ""
		s.Equal(&types.DomainInfo{
			Name:        domainName,
			Status:      types.DomainStatusRegistered.Ptr(),
			Description: description,
			OwnerEmail:  email,
			Data:        data,
			UUID:        "",
		}, info)
		s.Equal(&types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: retention,
			EmitMetric:                             emitMetric,
			HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
			HistoryArchivalURI:                     "",
			VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
			VisibilityArchivalURI:                  "",
			BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
			IsolationGroups:                        &types.IsolationGroupConfiguration{},
			AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
		}, config)
		s.Equal(&types.DomainReplicationConfiguration{
			ActiveClusterName: s.ClusterMetadata.GetCurrentClusterName(),
			Clusters:          clusters,
		}, replicationConfig)
		s.Equal(constants.EmptyVersion, failoverVersion)
		s.Equal(isGlobalDomain, isGlobalDomain)
	}

	updateResp, err := s.handler.UpdateDomain(context.Background(), &types.UpdateDomainRequest{
		Name: domainName,
	})
	s.Nil(err)
	fnTest(
		updateResp.DomainInfo,
		updateResp.Configuration,
		updateResp.ReplicationConfiguration,
		updateResp.GetIsGlobalDomain(),
		updateResp.GetFailoverVersion(),
	)

	getResp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	fnTest(
		getResp.DomainInfo,
		getResp.Configuration,
		getResp.ReplicationConfiguration,
		getResp.GetIsGlobalDomain(),
		getResp.GetFailoverVersion(),
	)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestUpdateGetDomain_LocalDomain_AllAttrSet() {
	domainName := s.getRandomDomainName()
	isGlobalDomain := false
	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		IsGlobalDomain:                         isGlobalDomain,
		WorkflowExecutionRetentionPeriodInDays: 1,
	})
	s.Nil(err)

	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	data := map[string]string{"some random key": "some random value"}
	var clusters []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(s.ClusterMetadata.GetCurrentClusterName(), nil) {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}

	fnTest := func(info *types.DomainInfo, config *types.DomainConfiguration,
		replicationConfig *types.DomainReplicationConfiguration, isGlobalDomain bool, failoverVersion int64) {
		s.NotEmpty(info.GetUUID())
		info.UUID = ""
		s.Equal(&types.DomainInfo{
			Name:        domainName,
			Status:      types.DomainStatusRegistered.Ptr(),
			Description: description,
			OwnerEmail:  email,
			Data:        data,
			UUID:        "",
		}, info)
		s.Equal(&types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: retention,
			EmitMetric:                             emitMetric,
			HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
			HistoryArchivalURI:                     "",
			VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
			VisibilityArchivalURI:                  "",
			BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
			IsolationGroups:                        &types.IsolationGroupConfiguration{},
			AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
		}, config)
		s.Equal(&types.DomainReplicationConfiguration{
			ActiveClusterName: s.ClusterMetadata.GetCurrentClusterName(),
			Clusters:          clusters,
		}, replicationConfig)
		s.Equal(constants.EmptyVersion, failoverVersion)
		s.Equal(isGlobalDomain, isGlobalDomain)
	}

	updateResp, err := s.handler.UpdateDomain(context.Background(), &types.UpdateDomainRequest{
		Name:                                   domainName,
		Description:                            common.StringPtr(description),
		OwnerEmail:                             common.StringPtr(email),
		Data:                                   data,
		WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(retention),
		EmitMetric:                             common.BoolPtr(emitMetric),
		HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
		HistoryArchivalURI:                     common.StringPtr(""),
		VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
		VisibilityArchivalURI:                  common.StringPtr(""),
		BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
	})
	s.Nil(err)
	fnTest(
		updateResp.DomainInfo,
		updateResp.Configuration,
		updateResp.ReplicationConfiguration,
		updateResp.GetIsGlobalDomain(),
		updateResp.GetFailoverVersion(),
	)

	getResp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	fnTest(
		getResp.DomainInfo,
		getResp.Configuration,
		getResp.ReplicationConfiguration,
		getResp.GetIsGlobalDomain(),
		getResp.GetFailoverVersion(),
	)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestDeprecateGetDomain_LocalDomain() {
	domainName := s.getRandomDomainName()
	domain := s.setupLocalDomain(domainName)

	err := s.handler.DeprecateDomain(context.Background(), &types.DeprecateDomainRequest{
		Name: domainName,
	})
	s.Nil(err)

	expectedResp := domain
	expectedResp.DomainInfo.Status = types.DomainStatusDeprecated.Ptr()

	getResp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	assertDomainEqual(s.Suite, getResp, expectedResp)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestRegisterGetDomain_GlobalDomain_AllDefault() {
	domainName := s.getRandomDomainName()
	isGlobalDomain := true
	var clusters []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(s.ClusterMetadata.GetCurrentClusterName(), nil) {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}

	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()

	retention := int32(1)
	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		IsGlobalDomain:                         isGlobalDomain,
		WorkflowExecutionRetentionPeriodInDays: retention,
	})
	s.Nil(err)

	resp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)

	s.NotEmpty(resp.DomainInfo.GetUUID())
	resp.DomainInfo.UUID = ""
	s.Equal(&types.DomainInfo{
		Name:        domainName,
		Status:      types.DomainStatusRegistered.Ptr(),
		Description: "",
		OwnerEmail:  "",
		Data:        nil,
		UUID:        "",
	}, resp.DomainInfo)
	s.Equal(&types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             true,
		HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
		HistoryArchivalURI:                     "",
		VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
		VisibilityArchivalURI:                  "",
		BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:                        &types.IsolationGroupConfiguration{},
		AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
	}, resp.Configuration)
	s.Equal(&types.DomainReplicationConfiguration{
		ActiveClusterName: s.ClusterMetadata.GetCurrentClusterName(),
		Clusters:          clusters,
	}, resp.ReplicationConfiguration)
	s.Equal(s.ClusterMetadata.GetNextFailoverVersion(s.ClusterMetadata.GetCurrentClusterName(), 0, "some-domaain"), resp.GetFailoverVersion())
	s.Equal(isGlobalDomain, resp.GetIsGlobalDomain())
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestRegisterGetDomain_GlobalDomain_NoDefault() {
	domainName := s.getRandomDomainName()
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	activeClusterName := ""
	clusters := []*types.ClusterReplicationConfiguration{}
	for clusterName := range s.ClusterMetadata.GetEnabledClusterInfo() {
		if clusterName != s.ClusterMetadata.GetCurrentClusterName() {
			activeClusterName = clusterName
		}
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: clusterName,
		})
	}
	s.True(len(activeClusterName) > 0)
	s.True(len(clusters) > 1)
	data := map[string]string{"some random key": "some random value"}
	isGlobalDomain := true

	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()

	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		Data:                                   data,
		IsGlobalDomain:                         isGlobalDomain,
	})
	s.Nil(err)

	resp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)

	s.NotEmpty(resp.DomainInfo.GetUUID())
	resp.DomainInfo.UUID = ""
	s.Equal(&types.DomainInfo{
		Name:        domainName,
		Status:      types.DomainStatusRegistered.Ptr(),
		Description: description,
		OwnerEmail:  email,
		Data:        data,
		UUID:        "",
	}, resp.DomainInfo)
	s.Equal(&types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             emitMetric,
		HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
		HistoryArchivalURI:                     "",
		VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
		VisibilityArchivalURI:                  "",
		BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:                        &types.IsolationGroupConfiguration{},
		AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
	}, resp.Configuration)
	s.Equal(&types.DomainReplicationConfiguration{
		ActiveClusterName: activeClusterName,
		Clusters:          clusters,
	}, resp.ReplicationConfiguration)
	s.Equal(s.ClusterMetadata.GetNextFailoverVersion(activeClusterName, 0, "some-domain"), resp.GetFailoverVersion())
	s.Equal(isGlobalDomain, resp.GetIsGlobalDomain())
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestUpdateGetDomain_GlobalDomain_NoAttrSet() {
	domainName := s.getRandomDomainName()
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	data := map[string]string{"some random key": "some random value"}
	activeClusterName := ""
	clusters := []*types.ClusterReplicationConfiguration{}
	for clusterName := range s.ClusterMetadata.GetEnabledClusterInfo() {
		if clusterName != s.ClusterMetadata.GetCurrentClusterName() {
			activeClusterName = clusterName
		}
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: clusterName,
		})
	}
	s.True(len(activeClusterName) > 0)
	s.True(len(clusters) > 1)
	isGlobalDomain := true

	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Twice()

	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		Data:                                   data,
		IsGlobalDomain:                         isGlobalDomain,
	})
	s.Nil(err)

	fnTest := func(info *types.DomainInfo, config *types.DomainConfiguration,
		replicationConfig *types.DomainReplicationConfiguration, isGlobalDomain bool, failoverVersion int64) {
		s.NotEmpty(info.GetUUID())
		info.UUID = ""
		s.Equal(&types.DomainInfo{
			Name:        domainName,
			Status:      types.DomainStatusRegistered.Ptr(),
			Description: description,
			OwnerEmail:  email,
			Data:        data,
			UUID:        "",
		}, info)
		s.Equal(&types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: retention,
			EmitMetric:                             emitMetric,
			HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
			HistoryArchivalURI:                     "",
			VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
			VisibilityArchivalURI:                  "",
			BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
			IsolationGroups:                        &types.IsolationGroupConfiguration{},
			AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
		}, config)
		s.Equal(&types.DomainReplicationConfiguration{
			ActiveClusterName: activeClusterName,
			Clusters:          clusters,
		}, replicationConfig)
		s.Equal(s.ClusterMetadata.GetNextFailoverVersion(activeClusterName, 0, "some-domain"), failoverVersion)
		s.Equal(isGlobalDomain, isGlobalDomain)
	}

	updateResp, err := s.handler.UpdateDomain(context.Background(), &types.UpdateDomainRequest{
		Name: domainName,
	})
	s.Nil(err)
	fnTest(
		updateResp.DomainInfo,
		updateResp.Configuration,
		updateResp.ReplicationConfiguration,
		updateResp.GetIsGlobalDomain(),
		updateResp.GetFailoverVersion(),
	)

	getResp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	fnTest(
		getResp.DomainInfo,
		getResp.Configuration,
		getResp.ReplicationConfiguration,
		getResp.GetIsGlobalDomain(),
		getResp.GetFailoverVersion(),
	)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestUpdateGetDomain_GlobalDomain_AllAttrSet() {
	domainName := s.getRandomDomainName()
	activeClusterName := ""
	clusters := []*types.ClusterReplicationConfiguration{}
	for clusterName := range s.ClusterMetadata.GetEnabledClusterInfo() {
		if clusterName != s.ClusterMetadata.GetCurrentClusterName() {
			activeClusterName = clusterName
		}
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: clusterName,
		})
	}
	s.True(len(activeClusterName) > 0)
	s.True(len(clusters) > 1)
	isGlobalDomain := true

	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Twice()

	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		IsGlobalDomain:                         isGlobalDomain,
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		WorkflowExecutionRetentionPeriodInDays: 1,
	})
	s.Nil(err)

	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	data := map[string]string{"some random key": "some random value"}

	fnTest := func(info *types.DomainInfo, config *types.DomainConfiguration,
		replicationConfig *types.DomainReplicationConfiguration, isGlobalDomain bool, failoverVersion int64) {
		s.NotEmpty(info.GetUUID())
		info.UUID = ""
		s.Equal(&types.DomainInfo{
			Name:        domainName,
			Status:      types.DomainStatusRegistered.Ptr(),
			Description: description,
			OwnerEmail:  email,
			Data:        data,
			UUID:        "",
		}, info)
		s.Equal(&types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: retention,
			EmitMetric:                             emitMetric,
			HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
			HistoryArchivalURI:                     "",
			VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
			VisibilityArchivalURI:                  "",
			BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
			IsolationGroups:                        &types.IsolationGroupConfiguration{},
			AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
		}, config)
		s.Equal(&types.DomainReplicationConfiguration{
			ActiveClusterName: activeClusterName,
			Clusters:          clusters,
		}, replicationConfig)
		s.Equal(s.ClusterMetadata.GetNextFailoverVersion(activeClusterName, 0, "some-domain"), failoverVersion)
		s.Equal(isGlobalDomain, isGlobalDomain)
	}

	updateResp, err := s.handler.UpdateDomain(context.Background(), &types.UpdateDomainRequest{
		Name:                                   domainName,
		Description:                            common.StringPtr(description),
		OwnerEmail:                             common.StringPtr(email),
		Data:                                   data,
		WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(retention),
		EmitMetric:                             common.BoolPtr(emitMetric),
		HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
		HistoryArchivalURI:                     common.StringPtr(""),
		VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
		VisibilityArchivalURI:                  common.StringPtr(""),
		BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		ActiveClusterName:                      nil,
		Clusters:                               clusters,
	})
	s.Nil(err)
	fnTest(
		updateResp.DomainInfo,
		updateResp.Configuration,
		updateResp.ReplicationConfiguration,
		updateResp.GetIsGlobalDomain(),
		updateResp.GetFailoverVersion(),
	)

	getResp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	fnTest(
		getResp.DomainInfo,
		getResp.Configuration,
		getResp.ReplicationConfiguration,
		getResp.GetIsGlobalDomain(),
		getResp.GetFailoverVersion(),
	)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestUpdateGetDomain_GlobalDomain_Failover() {
	domainName := s.getRandomDomainName()
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	data := map[string]string{"some random key": "some random value"}
	prevActiveClusterName := ""
	nextActiveClusterName := s.ClusterMetadata.GetCurrentClusterName()
	clusters := []*types.ClusterReplicationConfiguration{}
	for clusterName := range s.ClusterMetadata.GetEnabledClusterInfo() {
		if clusterName != nextActiveClusterName {
			prevActiveClusterName = clusterName
		}
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: clusterName,
		})
	}
	s.True(len(prevActiveClusterName) > 0)
	s.True(len(clusters) > 1)
	isGlobalDomain := true

	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Twice()

	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      prevActiveClusterName,
		Data:                                   data,
		IsGlobalDomain:                         isGlobalDomain,
	})
	s.Nil(err)

	var failoverHistory []FailoverEvent
	failoverHistory = append(failoverHistory, FailoverEvent{
		EventTime:    s.handler.timeSource.Now(),
		FromCluster:  prevActiveClusterName,
		ToCluster:    nextActiveClusterName,
		FailoverType: constants.FailoverType(constants.FailoverTypeForce).String(),
	})
	failoverHistoryJSON, _ := json.Marshal(failoverHistory)
	data[constants.DomainDataKeyForFailoverHistory] = string(failoverHistoryJSON)

	fnTest := func(
		info *types.DomainInfo,
		config *types.DomainConfiguration,
		replicationConfig *types.DomainReplicationConfiguration,
		isGlobalDomain bool,
		failoverVersion int64) {

		s.T().Helper()
		s.NotEmpty(info.GetUUID())
		info.UUID = ""
		s.Equal(&types.DomainInfo{
			Name:        domainName,
			Status:      types.DomainStatusRegistered.Ptr(),
			Description: description,
			OwnerEmail:  email,
			Data:        data,
			UUID:        "",
		}, info)
		s.Equal(&types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: retention,
			EmitMetric:                             emitMetric,
			HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
			HistoryArchivalURI:                     "",
			VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
			VisibilityArchivalURI:                  "",
			BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
			IsolationGroups:                        &types.IsolationGroupConfiguration{},
			AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
		}, config)
		s.Equal(&types.DomainReplicationConfiguration{
			ActiveClusterName: nextActiveClusterName,
			Clusters:          clusters,
		}, replicationConfig)
		s.Equal(s.ClusterMetadata.GetNextFailoverVersion(
			nextActiveClusterName,
			s.ClusterMetadata.GetNextFailoverVersion(prevActiveClusterName, 0, "some-domain"),
			"some-domain"), failoverVersion)
		s.Equal(isGlobalDomain, isGlobalDomain)
	}

	updateResp, err := s.handler.UpdateDomain(context.Background(), &types.UpdateDomainRequest{
		Name:              domainName,
		ActiveClusterName: common.StringPtr(s.ClusterMetadata.GetCurrentClusterName()),
	})
	s.Nil(err)
	fnTest(
		updateResp.DomainInfo,
		updateResp.Configuration,
		updateResp.ReplicationConfiguration,
		updateResp.GetIsGlobalDomain(),
		updateResp.GetFailoverVersion(),
	)

	getResp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	fnTest(
		getResp.DomainInfo,
		getResp.Configuration,
		getResp.ReplicationConfiguration,
		getResp.GetIsGlobalDomain(),
		getResp.GetFailoverVersion(),
	)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestUpdateDomain_CoolDown() {
	domainConfig := Config{
		MinRetentionDays:         dynamicproperties.GetIntPropertyFn(s.minRetentionDays),
		MaxBadBinaryCount:        dynamicproperties.GetIntPropertyFilteredByDomain(s.maxBadBinaryCount),
		FailoverCoolDown:         dynamicproperties.GetDurationPropertyFnFilteredByDomain(10000 * time.Second),
		EnableDomainAuditLogging: dynamicproperties.GetBoolPropertyFn(false),
	}
	s.handler = NewHandler(
		domainConfig,
		s.Logger,
		s.domainManager,
		s.mockDomainAuditManager,
		s.ClusterMetadata,
		s.mockDomainReplicator,
		s.archivalMetadata,
		s.mockArchiverProvider,
		clock.NewRealTimeSource(),
	).(*handlerImpl)

	domainName := s.getRandomDomainName()
	isGlobalDomain := true
	var clusters []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(s.ClusterMetadata.GetCurrentClusterName(), nil) {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}

	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()

	retention := int32(1)
	err := s.handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		IsGlobalDomain:                         isGlobalDomain,
		WorkflowExecutionRetentionPeriodInDays: retention,
	})
	s.Nil(err)

	resp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)

	s.NotEmpty(resp.DomainInfo.GetUUID())
	resp.DomainInfo.UUID = ""
	s.Equal(&types.DomainInfo{
		Name:        domainName,
		Status:      types.DomainStatusRegistered.Ptr(),
		Description: "",
		OwnerEmail:  "",
		Data:        nil,
		UUID:        "",
	}, resp.DomainInfo)
	s.Equal(&types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             true,
		HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
		HistoryArchivalURI:                     "",
		VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
		VisibilityArchivalURI:                  "",
		BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:                        &types.IsolationGroupConfiguration{},
		AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
	}, resp.Configuration)
	s.Equal(&types.DomainReplicationConfiguration{
		ActiveClusterName: s.ClusterMetadata.GetCurrentClusterName(),
		Clusters:          clusters,
	}, resp.ReplicationConfiguration)
	s.Equal(s.ClusterMetadata.GetNextFailoverVersion(s.ClusterMetadata.GetCurrentClusterName(), 0, "some-domain"), resp.GetFailoverVersion())
	s.Equal(isGlobalDomain, resp.GetIsGlobalDomain())

	_, err = s.handler.UpdateDomain(context.Background(), &types.UpdateDomainRequest{
		Name:        domainName,
		Description: common.StringPtr("test1"),
	})
	s.Error(err)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) TestDeprecateGetDomain_GlobalDomain() {
	domainName := s.getRandomDomainName()
	domain := s.setupGlobalDomain(domainName)

	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()

	err := s.handler.DeprecateDomain(context.Background(), &types.DeprecateDomainRequest{
		Name: domainName,
	})
	s.Nil(err)

	expectedResp := domain
	expectedResp.DomainInfo.Status = types.DomainStatusDeprecated.Ptr()

	getResp, err := s.handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	assertDomainEqual(s.Suite, getResp, expectedResp)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) getRandomDomainName() string {
	return "domain" + uuid.New()
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) setupLocalDomain(domainName string) *types.DescribeDomainResponse {
	return setupLocalDomain(s.Suite, s.handler, s.ClusterMetadata, domainName)
}

func (s *domainHandlerGlobalDomainEnabledPrimaryClusterSuite) setupGlobalDomain(domainName string) *types.DescribeDomainResponse {
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	return setupGlobalDomain(s.Suite, s.handler, s.ClusterMetadata, domainName)
}

func setupGlobalDomain(s *suite.Suite, handler *handlerImpl, clusterMetadata cluster.Metadata, domainName string) *types.DescribeDomainResponse {
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	data := map[string]string{"some random key": "some random value"}
	activeClusterName := ""
	clusters := []*types.ClusterReplicationConfiguration{}
	for clusterName := range clusterMetadata.GetEnabledClusterInfo() {
		if clusterName != clusterMetadata.GetCurrentClusterName() {
			activeClusterName = clusterName
		}
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: clusterName,
		})
	}
	s.True(len(activeClusterName) > 0)
	s.True(len(clusters) > 1)
	isGlobalDomain := true

	err := handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		Data:                                   data,
		IsGlobalDomain:                         isGlobalDomain,
	})
	s.Nil(err)

	getResp, err := handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	return getResp
}

func setupLocalDomain(s *suite.Suite, handler *handlerImpl, clusterMetadata cluster.Metadata, domainName string) *types.DescribeDomainResponse {
	description := "some random description"
	email := "some random email"
	retention := int32(7)
	emitMetric := true
	data := map[string]string{"some random key": "some random value"}
	var clusters []*types.ClusterReplicationConfiguration
	for _, replicationConfig := range cluster.GetOrUseDefaultClusters(clusterMetadata.GetCurrentClusterName(), nil) {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: replicationConfig.ClusterName,
		})
	}
	err := handler.RegisterDomain(context.Background(), &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             email,
		WorkflowExecutionRetentionPeriodInDays: retention,
		EmitMetric:                             common.BoolPtr(emitMetric),
		Clusters:                               clusters,
		ActiveClusterName:                      clusterMetadata.GetCurrentClusterName(),
		Data:                                   data,
	})
	s.Nil(err)
	getResp, err := handler.DescribeDomain(context.Background(), &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.Nil(err)
	return getResp
}

func assertDomainEqual(s *suite.Suite, actual, expected *types.DescribeDomainResponse) {
	s.NotEmpty(actual.DomainInfo.GetUUID())
	expected.DomainInfo.UUID = actual.DomainInfo.GetUUID()
	s.Equal(expected, actual)
}
