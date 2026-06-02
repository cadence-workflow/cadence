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
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/mocks"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	transmissionTaskSuite struct {
		suite.Suite
		domainReplicator *domainReplicatorImpl
		kafkaProducer    *mocks.KafkaProducer
	}
)

func TestTransmissionTaskSuite(t *testing.T) {
	s := new(transmissionTaskSuite)
	suite.Run(t, s)
}

func (s *transmissionTaskSuite) SetupSuite() {
}

func (s *transmissionTaskSuite) TearDownSuite() {

}

func (s *transmissionTaskSuite) SetupTest() {
	s.kafkaProducer = &mocks.KafkaProducer{}
	s.domainReplicator = NewDomainReplicator(
		s.kafkaProducer,
		testlogger.New(s.Suite.T()),
	).(*domainReplicatorImpl)
}

func (s *transmissionTaskSuite) TearDownTest() {
	s.kafkaProducer.AssertExpectations(s.T())
}

func (s *transmissionTaskSuite) TestHandleTransmissionTask_RegisterDomainTask_IsGlobalDomain() {
	taskType := types.ReplicationTaskTypeDomain
	id := uuid.New()
	name := "some random domain test name"
	status := types.DomainStatusRegistered
	description := "some random test description"
	ownerEmail := "some random test owner"
	data := map[string]string{"k": "v"}
	retention := int32(10)
	emitMetric := true
	historyArchivalStatus := types.ArchivalStatusEnabled
	historyArchivalURI := "some random history archival uri"
	visibilityArchivalStatus := types.ArchivalStatusEnabled
	visibilityArchivalURI := "some random visibility archival uri"
	clusterActive := "some random active cluster name"
	clusterStandby := "some random standby cluster name"
	configVersion := int64(0)
	failoverVersion := int64(59)
	previousFailoverVersion := int64(55)
	clusters := []*p.ClusterReplicationConfig{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	domainOperation := types.DomainOperationCreate
	info := &p.DomainInfo{
		ID:          id,
		Name:        name,
		Status:      p.DomainStatusRegistered,
		Description: description,
		OwnerEmail:  ownerEmail,
		Data:        data,
	}
	config := &p.DomainConfig{
		Retention:                retention,
		EmitMetric:               emitMetric,
		HistoryArchivalStatus:    historyArchivalStatus,
		HistoryArchivalURI:       historyArchivalURI,
		VisibilityArchivalStatus: visibilityArchivalStatus,
		VisibilityArchivalURI:    visibilityArchivalURI,
		BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups:          types.IsolationGroupConfiguration{},
		AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{},
	}
	replicationConfig := &p.DomainReplicationConfig{
		ActiveClusterName: clusterActive,
		Clusters:          clusters,
	}
	isGlobalDomain := true

	s.kafkaProducer.On("Publish", mock.Anything, &types.ReplicationTask{
		TaskType: &taskType,
		DomainTaskAttributes: &types.DomainTaskAttributes{
			DomainOperation: &domainOperation,
			ID:              id,
			Info: &types.DomainInfo{
				Name:        name,
				Status:      &status,
				Description: description,
				OwnerEmail:  ownerEmail,
				Data:        data,
			},
			Config: &types.DomainConfiguration{
				WorkflowExecutionRetentionPeriodInDays: retention,
				EmitMetric:                             emitMetric,
				HistoryArchivalStatus:                  historyArchivalStatus.Ptr(),
				HistoryArchivalURI:                     historyArchivalURI,
				VisibilityArchivalStatus:               visibilityArchivalStatus.Ptr(),
				VisibilityArchivalURI:                  visibilityArchivalURI,
				BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
				IsolationGroups:                        &types.IsolationGroupConfiguration{},
				AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
			},
			ReplicationConfig: &types.DomainReplicationConfiguration{
				ActiveClusterName: clusterActive,
				Clusters:          s.domainReplicator.convertClusterReplicationConfigToThrift(clusters),
			},
			ConfigVersion:           configVersion,
			FailoverVersion:         failoverVersion,
			PreviousFailoverVersion: previousFailoverVersion,
		},
	}).Return(nil).Once()

	err := s.domainReplicator.HandleTransmissionTask(
		context.Background(),
		domainOperation,
		info,
		config,
		replicationConfig,
		configVersion,
		failoverVersion,
		previousFailoverVersion,
		isGlobalDomain,
	)
	s.Nil(err)
}

func (s *transmissionTaskSuite) TestHandleTransmissionTask_RegisterDomainTask_NotGlobalDomain() {
	id := uuid.New()
	name := "some random domain test name"
	description := "some random test description"
	ownerEmail := "some random test owner"
	data := map[string]string{"k": "v"}
	retention := int32(10)
	emitMetric := true
	historyArchivalStatus := types.ArchivalStatusEnabled
	historyArchivalURI := "some random history archival uri"
	visibilityArchivalStatus := types.ArchivalStatusEnabled
	visibilityArchivalURI := "some random visibility archival uri"
	clusterActive := "some random active cluster name"
	clusterStandby := "some random standby cluster name"
	configVersion := int64(0)
	failoverVersion := int64(59)
	previousFailoverVersion := int64(55)
	clusters := []*p.ClusterReplicationConfig{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	domainOperation := types.DomainOperationCreate
	info := &p.DomainInfo{
		ID:          id,
		Name:        name,
		Status:      p.DomainStatusRegistered,
		Description: description,
		OwnerEmail:  ownerEmail,
		Data:        data,
	}
	config := &p.DomainConfig{
		Retention:                retention,
		EmitMetric:               emitMetric,
		HistoryArchivalStatus:    historyArchivalStatus,
		HistoryArchivalURI:       historyArchivalURI,
		VisibilityArchivalStatus: visibilityArchivalStatus,
		VisibilityArchivalURI:    visibilityArchivalURI,
		BadBinaries:              types.BadBinaries{},
	}
	replicationConfig := &p.DomainReplicationConfig{
		ActiveClusterName: clusterActive,
		Clusters:          clusters,
	}
	isGlobalDomain := false

	err := s.domainReplicator.HandleTransmissionTask(
		context.Background(),
		domainOperation,
		info,
		config,
		replicationConfig,
		configVersion,
		failoverVersion,
		previousFailoverVersion,
		isGlobalDomain,
	)
	s.Nil(err)
}

func (s *transmissionTaskSuite) TestHandleTransmissionTask_UpdateDomainTask_IsGlobalDomain() {
	taskType := types.ReplicationTaskTypeDomain
	id := uuid.New()
	name := "some random domain test name"
	status := types.DomainStatusDeprecated
	description := "some random test description"
	ownerEmail := "some random test owner"
	data := map[string]string{"k": "v"}
	retention := int32(10)
	emitMetric := true
	historyArchivalStatus := types.ArchivalStatusEnabled
	historyArchivalURI := "some random history archival uri"
	visibilityArchivalStatus := types.ArchivalStatusEnabled
	visibilityArchivalURI := "some random visibility archival uri"
	clusterActive := "some random active cluster name"
	clusterStandby := "some random standby cluster name"
	configVersion := int64(0)
	failoverVersion := int64(59)
	previousFailoverVersion := int64(55)
	clusters := []*p.ClusterReplicationConfig{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	domainOperation := types.DomainOperationUpdate
	info := &p.DomainInfo{
		ID:          id,
		Name:        name,
		Status:      p.DomainStatusDeprecated,
		Description: description,
		OwnerEmail:  ownerEmail,
		Data:        data,
	}
	config := &p.DomainConfig{
		Retention:                retention,
		EmitMetric:               emitMetric,
		HistoryArchivalStatus:    historyArchivalStatus,
		HistoryArchivalURI:       historyArchivalURI,
		VisibilityArchivalStatus: visibilityArchivalStatus,
		VisibilityArchivalURI:    visibilityArchivalURI,
		BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		IsolationGroups: types.IsolationGroupConfiguration{
			"zone-1": {Name: "zone-1", State: types.IsolationGroupStateDrained},
		},
		AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
			Enabled:             true,
			PredefinedQueueName: "queue1",
			QueueType:           "kafka",
			QueueConfig: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         []byte(`{"cluster": "queue1"}`),
			},
		},
	}
	replicationConfig := &p.DomainReplicationConfig{
		ActiveClusterName: clusterActive,
		Clusters:          clusters,
	}
	isGlobalDomain := true

	s.kafkaProducer.On("Publish", mock.Anything, &types.ReplicationTask{
		TaskType: &taskType,
		DomainTaskAttributes: &types.DomainTaskAttributes{
			DomainOperation: &domainOperation,
			ID:              id,
			Info: &types.DomainInfo{
				Name:        name,
				Status:      &status,
				Description: description,
				OwnerEmail:  ownerEmail,
				Data:        data,
			},
			Config: &types.DomainConfiguration{
				WorkflowExecutionRetentionPeriodInDays: retention,
				EmitMetric:                             emitMetric,
				HistoryArchivalStatus:                  historyArchivalStatus.Ptr(),
				HistoryArchivalURI:                     historyArchivalURI,
				VisibilityArchivalStatus:               visibilityArchivalStatus.Ptr(),
				VisibilityArchivalURI:                  visibilityArchivalURI,
				BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
				IsolationGroups: &types.IsolationGroupConfiguration{
					"zone-1": {Name: "zone-1", State: types.IsolationGroupStateDrained},
				},
				AsyncWorkflowConfig: &types.AsyncWorkflowConfiguration{
					Enabled:             true,
					PredefinedQueueName: "queue1",
					QueueType:           "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte(`{"cluster": "queue1"}`),
					},
				},
			},
			ReplicationConfig: &types.DomainReplicationConfiguration{
				ActiveClusterName: clusterActive,
				Clusters:          s.domainReplicator.convertClusterReplicationConfigToThrift(clusters),
			},
			ConfigVersion:           configVersion,
			FailoverVersion:         failoverVersion,
			PreviousFailoverVersion: previousFailoverVersion,
		},
	}).Return(nil).Once()

	err := s.domainReplicator.HandleTransmissionTask(
		context.Background(),
		domainOperation,
		info,
		config,
		replicationConfig,
		configVersion,
		failoverVersion,
		previousFailoverVersion,
		isGlobalDomain,
	)
	s.Nil(err)
}

func (s *transmissionTaskSuite) TestHandleTransmissionTask_UpdateDomainTask_NotGlobalDomain() {
	id := uuid.New()
	name := "some random domain test name"
	description := "some random test description"
	ownerEmail := "some random test owner"
	data := map[string]string{"k": "v"}
	retention := int32(10)
	emitMetric := true
	historyArchivalStatus := types.ArchivalStatusEnabled
	historyArchivalURI := "some random history archival uri"
	visibilityArchivalStatus := types.ArchivalStatusEnabled
	visibilityArchivalURI := "some random visibility archival uri"
	clusterActive := "some random active cluster name"
	clusterStandby := "some random standby cluster name"
	configVersion := int64(0)
	failoverVersion := int64(59)
	previousFailoverVersion := int64(55)
	clusters := []*p.ClusterReplicationConfig{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	domainOperation := types.DomainOperationUpdate
	info := &p.DomainInfo{
		ID:          id,
		Name:        name,
		Status:      p.DomainStatusDeprecated,
		Description: description,
		OwnerEmail:  ownerEmail,
		Data:        data,
	}
	config := &p.DomainConfig{
		Retention:                retention,
		EmitMetric:               emitMetric,
		HistoryArchivalStatus:    historyArchivalStatus,
		HistoryArchivalURI:       historyArchivalURI,
		VisibilityArchivalStatus: visibilityArchivalStatus,
		VisibilityArchivalURI:    visibilityArchivalURI,
	}
	replicationConfig := &p.DomainReplicationConfig{
		ActiveClusterName: clusterActive,
		Clusters:          clusters,
	}
	isGlobalDomain := false

	err := s.domainReplicator.HandleTransmissionTask(
		context.Background(),
		domainOperation,
		info,
		config,
		replicationConfig,
		configVersion,
		failoverVersion,
		previousFailoverVersion,
		isGlobalDomain,
	)
	s.Nil(err)
}
