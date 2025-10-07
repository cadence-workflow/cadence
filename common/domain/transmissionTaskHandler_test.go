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
	"github.com/stretchr/testify/assert"
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

func TestHandleTransmissionTask_ActiveActiveMerge(t *testing.T) {
	tests := map[string]struct {
		domainOperation        types.DomainOperation
		isGlobalDomain         bool
		activeClusters         *types.ActiveClusters
		expectPublish          bool
		expectedActiveClusters *types.ActiveClusters
	}{
		"active-active domain with location and region scopes": {
			domainOperation: types.DomainOperationUpdate,
			isGlobalDomain:  true,
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"Maarstricht": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Sao Paulo":   {ActiveClusterName: "clusterB", FailoverVersion: 1},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
						},
					},
				},
			},
			expectPublish: true,
			expectedActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"Maarstricht": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Sao Paulo":   {ActiveClusterName: "clusterB", FailoverVersion: 1},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
						},
					},
				},
			},
		},
		"active-active domain create operation": {
			domainOperation: types.DomainOperationCreate,
			isGlobalDomain:  true,
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Denver": {ActiveClusterName: "clusterB", FailoverVersion: 1},
						},
					},
				},
			},
			expectPublish: true,
			expectedActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Denver": {ActiveClusterName: "clusterB", FailoverVersion: 1},
						},
					},
				},
			},
		},
		"non-global domain should not publish": {
			domainOperation: types.DomainOperationUpdate,
			isGlobalDomain:  false,
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague": {ActiveClusterName: "clusterA", FailoverVersion: 0},
						},
					},
				},
			},
			expectPublish: false,
		},
		"nil active clusters": {
			domainOperation:        types.DomainOperationUpdate,
			isGlobalDomain:         true,
			activeClusters:         nil,
			expectPublish:          true,
			expectedActiveClusters: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			kafkaProducer := &mocks.KafkaProducer{}
			domainReplicator := NewDomainReplicator(
				kafkaProducer,
				testlogger.New(t),
			).(*domainReplicatorImpl)

			// Setup test data
			taskType := types.ReplicationTaskTypeDomain
			id := uuid.New()
			domainName := "test-active-active-domain"
			status := types.DomainStatusRegistered
			description := "test domain"
			ownerEmail := "test@example.com"
			data := map[string]string{"k": "v"}
			retention := int32(10)
			emitMetric := true
			historyArchivalStatus := types.ArchivalStatusEnabled
			historyArchivalURI := "test-history-uri"
			visibilityArchivalStatus := types.ArchivalStatusEnabled
			visibilityArchivalURI := "test-visibility-uri"
			clusterActive := "clusterA"
			clusterStandby := "clusterB"
			configVersion := int64(5)
			failoverVersion := int64(100)
			previousFailoverVersion := int64(50)
			clusters := []*p.ClusterReplicationConfig{
				{ClusterName: clusterActive},
				{ClusterName: clusterStandby},
			}

			info := &p.DomainInfo{
				ID:          id,
				Name:        domainName,
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
				ActiveClusters:    tc.activeClusters,
			}

			if tc.expectPublish {
				kafkaProducer.On("Publish", mock.Anything, &types.ReplicationTask{
					TaskType: &taskType,
					DomainTaskAttributes: &types.DomainTaskAttributes{
						DomainOperation: &tc.domainOperation,
						ID:              id,
						Info: &types.DomainInfo{
							Name:        domainName,
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
							Clusters:          domainReplicator.convertClusterReplicationConfigToThrift(clusters),
							ActiveClusters:    tc.expectedActiveClusters,
						},
						ConfigVersion:           configVersion,
						FailoverVersion:         failoverVersion,
						PreviousFailoverVersion: previousFailoverVersion,
					},
				}).Return(nil).Once()
			}

			err := domainReplicator.HandleTransmissionTask(
				context.Background(),
				tc.domainOperation,
				info,
				config,
				replicationConfig,
				configVersion,
				failoverVersion,
				previousFailoverVersion,
				tc.isGlobalDomain,
			)

			assert.NoError(t, err)
			kafkaProducer.AssertExpectations(t)
		})
	}
}

func TestActiveActiveDomainUpdates_Merge(t *testing.T) {

	// worked example:
	//
	// Cluster A - Initial Failover Version: 0 - us-east-1
	// Cluster B - Initial Failover Version: 1 - us-west-1
	// Cluster C - Initial Failover Version: 2 - us-west-2
	// Cluster D - Initial Failover Version: 3 - us-west-2

	tests := map[string]struct {
		currentActiveClusters *types.ActiveClusters
		request               *types.ActiveClusters

		expectedResult    *types.ActiveClusters
		expectedIsChanged bool
	}{

		"simple case - incoming location failover out of us-east-1 - failing over some cities to cluster C - the result should indicate the failover has occurred": {
			currentActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Sao Paulo":   {ActiveClusterName: "clusterD", FailoverVersion: 3},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"us-west-2": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
			// incoming location failover out of us-east-1 - failing over some cities to cluster C
			request: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Denver": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
			expectedIsChanged: true,
			expectedResult: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Denver":      {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Sao Paulo":   {ActiveClusterName: "clusterD", FailoverVersion: 3},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"us-west-2": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
		},

		// this simulates a stale domain update being sent through the replication queue
		// in this case, the local data is more recent and should win-out
		"incoming location failover out of us-east-1 - failing over some cities to cluster C - the local data is more recent - the result should drop stale cluster-attributes from the replication message": {
			currentActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 100},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 101},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Sao Paulo":   {ActiveClusterName: "clusterD", FailoverVersion: 3},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"us-west-2": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
			// incoming location failover out of us-east-1 - failing over some cities to cluster C
			request: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Denver": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
			expectedIsChanged: false,
			expectedResult: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 100},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 101},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Sao Paulo":   {ActiveClusterName: "clusterD", FailoverVersion: 3},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"us-west-2": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
		},
		// This simulates the case when there's either been very fast changes or a network split
		// and at least two clusters have updated their local domain-data in the mean time and
		// at for the same amount of times. Ie it's pretty unlikely, though possible.
		// in this case, the arbitrary initial failover version is used as a deterministic tie-break
		"conflict resolution - both incoming and local data have been modified - employ tie-break - the result with the higher failover version should win": {
			currentActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 100},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 101},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Sao Paulo":   {ActiveClusterName: "clusterD", FailoverVersion: 103},
						},
					},
				},
			},
			// incoming location failover out of us-east-1 - failing all locations to cluster C
			request: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterC", FailoverVersion: 102},
							"Denver":      {ActiveClusterName: "clusterC", FailoverVersion: 102},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 102},
							"Sao Paulo":   {ActiveClusterName: "clusterC", FailoverVersion: 102},
						},
					},
				},
			},
			expectedIsChanged: true,
			expectedResult: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							// Prague and Denver are failing over to cluster C - they should be updated to the new cluster
							// the somewhat arbitrary initial failover version is used as a tie-break
							// and clusterC is chosen
							"Prague":      {ActiveClusterName: "clusterC", FailoverVersion: 102},
							"Denver":      {ActiveClusterName: "clusterC", FailoverVersion: 102},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 102},
							// Sao Paulo is not failing over - it should remain at the same cluster
							"Sao Paulo": {ActiveClusterName: "clusterD", FailoverVersion: 103},
						},
					},
				},
			},
		},

		"simple case - no updates - no change": {
			currentActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Sao Paulo":   {ActiveClusterName: "clusterD", FailoverVersion: 3},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"us-west-2": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
			request: &types.ActiveClusters{},
			expectedResult: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"Prague":      {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"Denver":      {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"Maarstricht": {ActiveClusterName: "clusterC", FailoverVersion: 2},
							"Sao Paulo":   {ActiveClusterName: "clusterD", FailoverVersion: 3},
						},
					},
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "clusterA", FailoverVersion: 0},
							"us-west-1": {ActiveClusterName: "clusterB", FailoverVersion: 1},
							"us-west-2": {ActiveClusterName: "clusterC", FailoverVersion: 2},
						},
					},
				},
			},
			expectedIsChanged: false,
		},
	}
	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			result, isChanged := mergeActiveActiveScopes(td.currentActiveClusters, td.request)
			assert.Equal(t, td.expectedResult, result)
			assert.Equal(t, td.expectedIsChanged, isChanged)
		})
	}
}
