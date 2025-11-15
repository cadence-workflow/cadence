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

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	persistencetests "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	"github.com/uber/cadence/common/types"
)

type (
	domainReplicationTaskExecutorSuite struct {
		*persistencetests.TestBase
		domainReplicator *domainReplicationTaskExecutorImpl
	}
)

func TestDomainReplicationTaskExecutorSuite(t *testing.T) {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}

	s := new(domainReplicationTaskExecutorSuite)
	s.setupTestBase(t)
	suite.Run(t, s)
}

func (s *domainReplicationTaskExecutorSuite) setupTestBase(t *testing.T) {
	sqliteTestBaseOptions := sqlite.GetTestClusterOption()
	s.TestBase = persistencetests.NewTestBaseWithSQL(t, sqliteTestBaseOptions)
	s.Setup()
}

func (s *domainReplicationTaskExecutorSuite) SetupTest() {
	s.setupTestBase(s.T())

	domainAuditManager, err := s.ExecutionMgrFactory.NewDomainAuditManager()
	if err != nil {
		s.T().Fatalf("Failed to create domain audit manager: %v", err)
	}

	// Disable audit logging for integration tests as SQLite doesn't fully support the audit manager
	enableAuditLogging := func(...dynamicproperties.FilterOption) bool { return false }

	s.domainReplicator = NewReplicationTaskExecutor(
		s.DomainManager,
		domainAuditManager,
		clock.NewRealTimeSource(),
		s.Logger,
		enableAuditLogging,
	).(*domainReplicationTaskExecutorImpl)
}

func (s *domainReplicationTaskExecutorSuite) TearDownTest() {
	s.TearDownWorkflowStore()
}

func (s *domainReplicationTaskExecutorSuite) TestExecute_RegisterDomainTask_NameUUIDCollision() {
	operation := types.DomainOperationCreate
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
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	task := &types.DomainTaskAttributes{
		DomainOperation: &operation,
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
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: clusterActive,
			Clusters:          clusters,
		},
		ConfigVersion:   configVersion,
		FailoverVersion: failoverVersion,
	}

	err := s.domainReplicator.Execute(task)
	s.Nil(err)

	task.ID = uuid.New()
	task.Info.Name = name
	err = s.domainReplicator.Execute(task)
	s.NotNil(err)
	s.IsType(&types.BadRequestError{}, err)

	task.ID = id
	task.Info.Name = "other random domain test name"
	err = s.domainReplicator.Execute(task)
	s.NotNil(err)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *domainReplicationTaskExecutorSuite) TestExecute_RegisterDomainTask() {
	operation := types.DomainOperationCreate
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
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	task := &types.DomainTaskAttributes{
		DomainOperation: &operation,
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
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: clusterActive,
			Clusters:          clusters,
		},
		ConfigVersion:   configVersion,
		FailoverVersion: failoverVersion,
	}

	metadata, err := s.DomainManager.GetMetadata(context.Background())
	s.Nil(err)
	notificationVersion := metadata.NotificationVersion
	err = s.domainReplicator.Execute(task)
	s.Nil(err)

	resp, err := s.DomainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{ID: id})
	s.Nil(err)
	s.NotNil(resp)
	s.Equal(id, resp.Info.ID)
	s.Equal(name, resp.Info.Name)
	s.Equal(persistence.DomainStatusRegistered, resp.Info.Status)
	s.Equal(description, resp.Info.Description)
	s.Equal(ownerEmail, resp.Info.OwnerEmail)
	s.Equal(data, resp.Info.Data)
	s.Equal(retention, resp.Config.Retention)
	s.Equal(emitMetric, resp.Config.EmitMetric)
	s.Equal(historyArchivalStatus, resp.Config.HistoryArchivalStatus)
	s.Equal(historyArchivalURI, resp.Config.HistoryArchivalURI)
	s.Equal(visibilityArchivalStatus, resp.Config.VisibilityArchivalStatus)
	s.Equal(visibilityArchivalURI, resp.Config.VisibilityArchivalURI)
	s.Equal(clusterActive, resp.ReplicationConfig.ActiveClusterName)
	s.Equal(s.domainReplicator.convertClusterReplicationConfigFromThrift(clusters), resp.ReplicationConfig.Clusters)
	s.Equal(configVersion, resp.ConfigVersion)
	s.Equal(failoverVersion, resp.FailoverVersion)
	s.Equal(int64(0), resp.FailoverNotificationVersion)
	s.Equal(notificationVersion, resp.NotificationVersion)

	// handle duplicated task
	err = s.domainReplicator.Execute(task)
	s.Nil(err)
}

func (s *domainReplicationTaskExecutorSuite) TestExecute_UpdateDomainTask_DomainNotExist() {
	operation := types.DomainOperationUpdate
	id := uuid.New()
	name := "some random domain test name"
	status := types.DomainStatusRegistered
	description := "some random test description"
	ownerEmail := "some random test owner"
	retention := int32(10)
	emitMetric := true
	historyArchivalStatus := types.ArchivalStatusEnabled
	historyArchivalURI := "some random history archival uri"
	visibilityArchivalStatus := types.ArchivalStatusEnabled
	visibilityArchivalURI := "some random visibility archival uri"
	clusterActive := "some random active cluster name"
	clusterStandby := "some random standby cluster name"
	configVersion := int64(12)
	failoverVersion := int64(59)
	domainData := map[string]string{"k1": "v1", "k2": "v2"}
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	updateTask := &types.DomainTaskAttributes{
		DomainOperation: &operation,
		ID:              id,
		Info: &types.DomainInfo{
			Name:        name,
			Status:      &status,
			Description: description,
			OwnerEmail:  ownerEmail,
			Data:        domainData,
		},
		Config: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: retention,
			EmitMetric:                             emitMetric,
			HistoryArchivalStatus:                  historyArchivalStatus.Ptr(),
			HistoryArchivalURI:                     historyArchivalURI,
			VisibilityArchivalStatus:               visibilityArchivalStatus.Ptr(),
			VisibilityArchivalURI:                  visibilityArchivalURI,
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: clusterActive,
			Clusters:          clusters,
		},
		ConfigVersion:   configVersion,
		FailoverVersion: failoverVersion,
	}

	metadata, err := s.DomainManager.GetMetadata(context.Background())
	s.Nil(err)
	notificationVersion := metadata.NotificationVersion
	err = s.domainReplicator.Execute(updateTask)
	s.Nil(err)

	resp, err := s.DomainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{Name: name})
	s.Nil(err)
	s.NotNil(resp)
	s.Equal(id, resp.Info.ID)
	s.Equal(name, resp.Info.Name)
	s.Equal(persistence.DomainStatusRegistered, resp.Info.Status)
	s.Equal(description, resp.Info.Description)
	s.Equal(ownerEmail, resp.Info.OwnerEmail)
	s.Equal(domainData, resp.Info.Data)
	s.Equal(retention, resp.Config.Retention)
	s.Equal(emitMetric, resp.Config.EmitMetric)
	s.Equal(historyArchivalStatus, resp.Config.HistoryArchivalStatus)
	s.Equal(historyArchivalURI, resp.Config.HistoryArchivalURI)
	s.Equal(visibilityArchivalStatus, resp.Config.VisibilityArchivalStatus)
	s.Equal(visibilityArchivalURI, resp.Config.VisibilityArchivalURI)
	s.Equal(clusterActive, resp.ReplicationConfig.ActiveClusterName)
	s.Equal(s.domainReplicator.convertClusterReplicationConfigFromThrift(clusters), resp.ReplicationConfig.Clusters)
	s.Equal(configVersion, resp.ConfigVersion)
	s.Equal(failoverVersion, resp.FailoverVersion)
	s.Equal(int64(0), resp.FailoverNotificationVersion)
	s.Equal(notificationVersion, resp.NotificationVersion)
}

func (s *domainReplicationTaskExecutorSuite) TestExecute_UpdateDomainTask_UpdateConfig_UpdateActiveCluster() {
	operation := types.DomainOperationCreate
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
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	createTask := &types.DomainTaskAttributes{
		DomainOperation: &operation,
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
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: clusterActive,
			Clusters:          clusters,
		},
		ConfigVersion:   configVersion,
		FailoverVersion: failoverVersion,
	}

	err := s.domainReplicator.Execute(createTask)
	s.Nil(err)

	// success update case
	updateOperation := types.DomainOperationUpdate
	updateStatus := types.DomainStatusDeprecated
	updateDescription := "other random domain test description"
	updateOwnerEmail := "other random domain test owner"
	updatedData := map[string]string{"k": "v1"}
	updateRetention := int32(122)
	updateEmitMetric := true
	updateHistoryArchivalStatus := types.ArchivalStatusDisabled
	updateHistoryArchivalURI := "some updated history archival uri"
	updateVisibilityArchivalStatus := types.ArchivalStatusDisabled
	updateVisibilityArchivalURI := "some updated visibility archival uri"
	updateClusterActive := "other random active cluster name"
	updateClusterStandby := "other random standby cluster name"
	updateConfigVersion := configVersion + 1
	updateFailoverVersion := failoverVersion + 1
	updateClusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: updateClusterActive,
		},
		{
			ClusterName: updateClusterStandby,
		},
	}
	updateTask := &types.DomainTaskAttributes{
		DomainOperation: &updateOperation,
		ID:              id,
		Info: &types.DomainInfo{
			Name:        name,
			Status:      &updateStatus,
			Description: updateDescription,
			OwnerEmail:  updateOwnerEmail,
			Data:        updatedData,
		},
		Config: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: updateRetention,
			EmitMetric:                             updateEmitMetric,
			HistoryArchivalStatus:                  updateHistoryArchivalStatus.Ptr(),
			HistoryArchivalURI:                     updateHistoryArchivalURI,
			VisibilityArchivalStatus:               updateVisibilityArchivalStatus.Ptr(),
			VisibilityArchivalURI:                  updateVisibilityArchivalURI,
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: updateClusterActive,
			Clusters:          updateClusters,
		},
		ConfigVersion:   updateConfigVersion,
		FailoverVersion: updateFailoverVersion,
	}
	metadata, err := s.DomainManager.GetMetadata(context.Background())
	s.Nil(err)
	notificationVersion := metadata.NotificationVersion
	err = s.domainReplicator.Execute(updateTask)
	s.Nil(err)
	resp, err := s.DomainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{Name: name})
	s.Nil(err)
	s.NotNil(resp)
	s.Equal(id, resp.Info.ID)
	s.Equal(name, resp.Info.Name)
	s.Equal(persistence.DomainStatusDeprecated, resp.Info.Status)
	s.Equal(updateDescription, resp.Info.Description)
	s.Equal(updateOwnerEmail, resp.Info.OwnerEmail)
	s.Equal(updatedData, resp.Info.Data)
	s.Equal(updateRetention, resp.Config.Retention)
	s.Equal(updateEmitMetric, resp.Config.EmitMetric)
	s.Equal(updateHistoryArchivalStatus, resp.Config.HistoryArchivalStatus)
	s.Equal(updateHistoryArchivalURI, resp.Config.HistoryArchivalURI)
	s.Equal(updateVisibilityArchivalStatus, resp.Config.VisibilityArchivalStatus)
	s.Equal(updateVisibilityArchivalURI, resp.Config.VisibilityArchivalURI)
	s.Equal(updateClusterActive, resp.ReplicationConfig.ActiveClusterName)
	s.Equal(s.domainReplicator.convertClusterReplicationConfigFromThrift(updateClusters), resp.ReplicationConfig.Clusters)
	s.Equal(updateConfigVersion, resp.ConfigVersion)
	s.Equal(updateFailoverVersion, resp.FailoverVersion)
	s.Equal(notificationVersion, resp.FailoverNotificationVersion)
	s.Equal(notificationVersion, resp.NotificationVersion)
}

func (s *domainReplicationTaskExecutorSuite) TestExecute_UpdateDomainTask_UpdateConfig_NoUpdateActiveCluster() {
	operation := types.DomainOperationCreate
	id := uuid.New()
	name := "some random domain test name"
	status := types.DomainStatusRegistered
	description := "some random test description"
	ownerEmail := "some random test owner"
	data := map[string]string{"k": "v"}
	retention := int32(10)
	emitMetric := true
	historyArchivalStatus := types.ArchivalStatusDisabled
	historyArchivalURI := ""
	visibilityArchivalStatus := types.ArchivalStatusDisabled
	visibilityArchivalURI := ""
	clusterActive := "some random active cluster name"
	clusterStandby := "some random standby cluster name"
	configVersion := int64(0)
	failoverVersion := int64(59)
	previousFailoverVersion := int64(55)
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	createTask := &types.DomainTaskAttributes{
		DomainOperation: &operation,
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
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: clusterActive,
			Clusters:          clusters,
		},
		ConfigVersion:   configVersion,
		FailoverVersion: failoverVersion,
	}

	err := s.domainReplicator.Execute(createTask)
	s.Nil(err)

	// success update case
	updateOperation := types.DomainOperationUpdate
	updateStatus := types.DomainStatusDeprecated
	updateDescription := "other random domain test description"
	updateOwnerEmail := "other random domain test owner"
	updateData := map[string]string{"k": "v2"}
	updateRetention := int32(122)
	updateEmitMetric := true
	updateHistoryArchivalStatus := types.ArchivalStatusEnabled
	updateHistoryArchivalURI := "some updated history archival uri"
	updateVisibilityArchivalStatus := types.ArchivalStatusEnabled
	updateVisibilityArchivalURI := "some updated visibility archival uri"
	updateClusterActive := "other random active cluster name"
	updateClusterStandby := "other random standby cluster name"
	updateConfigVersion := configVersion + 1
	updateFailoverVersion := failoverVersion - 1
	updateClusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: updateClusterActive,
		},
		{
			ClusterName: updateClusterStandby,
		},
	}
	updateTask := &types.DomainTaskAttributes{
		DomainOperation: &updateOperation,
		ID:              id,
		Info: &types.DomainInfo{
			Name:        name,
			Status:      &updateStatus,
			Description: updateDescription,
			OwnerEmail:  updateOwnerEmail,
			Data:        updateData,
		},
		Config: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: updateRetention,
			EmitMetric:                             updateEmitMetric,
			HistoryArchivalStatus:                  updateHistoryArchivalStatus.Ptr(),
			HistoryArchivalURI:                     updateHistoryArchivalURI,
			VisibilityArchivalStatus:               updateVisibilityArchivalStatus.Ptr(),
			VisibilityArchivalURI:                  updateVisibilityArchivalURI,
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: updateClusterActive,
			Clusters:          updateClusters,
		},
		ConfigVersion:           updateConfigVersion,
		FailoverVersion:         updateFailoverVersion,
		PreviousFailoverVersion: previousFailoverVersion,
	}
	metadata, err := s.DomainManager.GetMetadata(context.Background())
	s.Nil(err)
	notificationVersion := metadata.NotificationVersion
	err = s.domainReplicator.Execute(updateTask)
	s.Nil(err)
	resp, err := s.DomainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{Name: name})
	s.Nil(err)
	s.NotNil(resp)
	s.Equal(id, resp.Info.ID)
	s.Equal(name, resp.Info.Name)
	s.Equal(persistence.DomainStatusDeprecated, resp.Info.Status)
	s.Equal(updateDescription, resp.Info.Description)
	s.Equal(updateOwnerEmail, resp.Info.OwnerEmail)
	s.Equal(updateData, resp.Info.Data)
	s.Equal(updateRetention, resp.Config.Retention)
	s.Equal(updateEmitMetric, resp.Config.EmitMetric)
	s.Equal(updateHistoryArchivalStatus, resp.Config.HistoryArchivalStatus)
	s.Equal(updateHistoryArchivalURI, resp.Config.HistoryArchivalURI)
	s.Equal(updateVisibilityArchivalStatus, resp.Config.VisibilityArchivalStatus)
	s.Equal(updateVisibilityArchivalURI, resp.Config.VisibilityArchivalURI)
	s.Equal(clusterActive, resp.ReplicationConfig.ActiveClusterName)
	s.Equal(s.domainReplicator.convertClusterReplicationConfigFromThrift(updateClusters), resp.ReplicationConfig.Clusters)
	s.Equal(updateConfigVersion, resp.ConfigVersion)
	s.Equal(failoverVersion, resp.FailoverVersion)
	s.Equal(constants.InitialPreviousFailoverVersion, resp.PreviousFailoverVersion)
	s.Equal(int64(0), resp.FailoverNotificationVersion)
	s.Equal(notificationVersion, resp.NotificationVersion)
}

func (s *domainReplicationTaskExecutorSuite) TestExecute_UpdateDomainTask_NoUpdateConfig_UpdateActiveCluster() {
	operation := types.DomainOperationCreate
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
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	createTask := &types.DomainTaskAttributes{
		DomainOperation: &operation,
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
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: clusterActive,
			Clusters:          clusters,
		},
		ConfigVersion:           configVersion,
		FailoverVersion:         failoverVersion,
		PreviousFailoverVersion: previousFailoverVersion,
	}

	err := s.domainReplicator.Execute(createTask)
	s.Nil(err)
	resp1, err := s.DomainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{Name: name})
	s.Nil(err)
	s.NotNil(resp1)
	s.Equal(id, resp1.Info.ID)
	s.Equal(name, resp1.Info.Name)
	s.Equal(persistence.DomainStatusRegistered, resp1.Info.Status)
	s.Equal(description, resp1.Info.Description)
	s.Equal(ownerEmail, resp1.Info.OwnerEmail)
	s.Equal(data, resp1.Info.Data)
	s.Equal(retention, resp1.Config.Retention)
	s.Equal(emitMetric, resp1.Config.EmitMetric)
	s.Equal(historyArchivalStatus, resp1.Config.HistoryArchivalStatus)
	s.Equal(historyArchivalURI, resp1.Config.HistoryArchivalURI)
	s.Equal(visibilityArchivalStatus, resp1.Config.VisibilityArchivalStatus)
	s.Equal(visibilityArchivalURI, resp1.Config.VisibilityArchivalURI)
	s.Equal(clusterActive, resp1.ReplicationConfig.ActiveClusterName)
	s.Equal(s.domainReplicator.convertClusterReplicationConfigFromThrift(clusters), resp1.ReplicationConfig.Clusters)
	s.Equal(configVersion, resp1.ConfigVersion)
	s.Equal(failoverVersion, resp1.FailoverVersion)
	s.Equal(constants.InitialPreviousFailoverVersion, resp1.PreviousFailoverVersion)

	// success update case
	updateOperation := types.DomainOperationUpdate
	updateStatus := types.DomainStatusDeprecated
	updateDescription := "other random domain test description"
	updateOwnerEmail := "other random domain test owner"
	updatedData := map[string]string{"k": "v2"}
	updateRetention := int32(122)
	updateEmitMetric := true
	updateClusterActive := "other random active cluster name"
	updateClusterStandby := "other random standby cluster name"
	updateConfigVersion := configVersion - 1
	updateFailoverVersion := failoverVersion + 1
	updateClusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: updateClusterActive,
		},
		{
			ClusterName: updateClusterStandby,
		},
	}
	updateTask := &types.DomainTaskAttributes{
		DomainOperation: &updateOperation,
		ID:              id,
		Info: &types.DomainInfo{
			Name:        name,
			Status:      &updateStatus,
			Description: updateDescription,
			OwnerEmail:  updateOwnerEmail,
			Data:        updatedData,
		},
		Config: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: updateRetention,
			EmitMetric:                             updateEmitMetric,
			HistoryArchivalStatus:                  historyArchivalStatus.Ptr(),
			HistoryArchivalURI:                     historyArchivalURI,
			VisibilityArchivalStatus:               visibilityArchivalStatus.Ptr(),
			VisibilityArchivalURI:                  visibilityArchivalURI,
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: updateClusterActive,
			Clusters:          updateClusters,
		},
		ConfigVersion:           updateConfigVersion,
		FailoverVersion:         updateFailoverVersion,
		PreviousFailoverVersion: failoverVersion,
	}
	metadata, err := s.DomainManager.GetMetadata(context.Background())
	s.Nil(err)
	notificationVersion := metadata.NotificationVersion
	err = s.domainReplicator.Execute(updateTask)
	s.Nil(err)
	resp, err := s.DomainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{Name: name})
	s.Nil(err)
	s.NotNil(resp)
	s.Equal(id, resp.Info.ID)
	s.Equal(name, resp.Info.Name)
	s.Equal(persistence.DomainStatusRegistered, resp.Info.Status)
	s.Equal(description, resp.Info.Description)
	s.Equal(ownerEmail, resp.Info.OwnerEmail)
	s.Equal(data, resp.Info.Data)
	s.Equal(retention, resp.Config.Retention)
	s.Equal(emitMetric, resp.Config.EmitMetric)
	s.Equal(historyArchivalStatus, resp.Config.HistoryArchivalStatus)
	s.Equal(historyArchivalURI, resp.Config.HistoryArchivalURI)
	s.Equal(visibilityArchivalStatus, resp.Config.VisibilityArchivalStatus)
	s.Equal(visibilityArchivalURI, resp.Config.VisibilityArchivalURI)
	s.Equal(updateClusterActive, resp.ReplicationConfig.ActiveClusterName)
	s.Equal(s.domainReplicator.convertClusterReplicationConfigFromThrift(clusters), resp.ReplicationConfig.Clusters)
	s.Equal(configVersion, resp.ConfigVersion)
	s.Equal(updateFailoverVersion, resp.FailoverVersion)
	s.Equal(notificationVersion, resp.FailoverNotificationVersion)
	s.Equal(notificationVersion, resp.NotificationVersion)
	s.Equal(failoverVersion, resp.PreviousFailoverVersion)
}

func (s *domainReplicationTaskExecutorSuite) TestExecute_UpdateDomainTask_NoUpdateConfig_NoUpdateActiveCluster() {
	operation := types.DomainOperationCreate
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
	clusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterActive,
		},
		{
			ClusterName: clusterStandby,
		},
	}

	createTask := &types.DomainTaskAttributes{
		DomainOperation: &operation,
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
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: clusterActive,
			Clusters:          clusters,
		},
		ConfigVersion:   configVersion,
		FailoverVersion: failoverVersion,
	}
	metadata, err := s.DomainManager.GetMetadata(context.Background())
	s.Nil(err)
	notificationVersion := metadata.NotificationVersion
	err = s.domainReplicator.Execute(createTask)
	s.Nil(err)

	// success update case
	updateOperation := types.DomainOperationUpdate
	updateStatus := types.DomainStatusDeprecated
	updateDescription := "other random domain test description"
	updateOwnerEmail := "other random domain test owner"
	updatedData := map[string]string{"k": "v2"}
	updateRetention := int32(122)
	updateEmitMetric := true
	updateClusterActive := "other random active cluster name"
	updateClusterStandby := "other random standby cluster name"
	updateConfigVersion := configVersion - 1
	updateFailoverVersion := failoverVersion - 1
	updateClusters := []*types.ClusterReplicationConfiguration{
		{
			ClusterName: updateClusterActive,
		},
		{
			ClusterName: updateClusterStandby,
		},
	}
	updateTask := &types.DomainTaskAttributes{
		DomainOperation: &updateOperation,
		ID:              id,
		Info: &types.DomainInfo{
			Name:        name,
			Status:      &updateStatus,
			Description: updateDescription,
			OwnerEmail:  updateOwnerEmail,
			Data:        updatedData,
		},
		Config: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: updateRetention,
			EmitMetric:                             updateEmitMetric,
			HistoryArchivalStatus:                  historyArchivalStatus.Ptr(),
			HistoryArchivalURI:                     historyArchivalURI,
			VisibilityArchivalStatus:               visibilityArchivalStatus.Ptr(),
			VisibilityArchivalURI:                  visibilityArchivalURI,
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: updateClusterActive,
			Clusters:          updateClusters,
		},
		ConfigVersion:           updateConfigVersion,
		FailoverVersion:         updateFailoverVersion,
		PreviousFailoverVersion: previousFailoverVersion,
	}
	err = s.domainReplicator.Execute(updateTask)
	s.Nil(err)
	resp, err := s.DomainManager.GetDomain(context.Background(), &persistence.GetDomainRequest{Name: name})
	s.Nil(err)
	s.NotNil(resp)
	s.Equal(id, resp.Info.ID)
	s.Equal(name, resp.Info.Name)
	s.Equal(persistence.DomainStatusRegistered, resp.Info.Status)
	s.Equal(description, resp.Info.Description)
	s.Equal(ownerEmail, resp.Info.OwnerEmail)
	s.Equal(data, resp.Info.Data)
	s.Equal(retention, resp.Config.Retention)
	s.Equal(emitMetric, resp.Config.EmitMetric)
	s.Equal(historyArchivalStatus, resp.Config.HistoryArchivalStatus)
	s.Equal(historyArchivalURI, resp.Config.HistoryArchivalURI)
	s.Equal(visibilityArchivalStatus, resp.Config.VisibilityArchivalStatus)
	s.Equal(visibilityArchivalURI, resp.Config.VisibilityArchivalURI)
	s.Equal(clusterActive, resp.ReplicationConfig.ActiveClusterName)
	s.Equal(s.domainReplicator.convertClusterReplicationConfigFromThrift(clusters), resp.ReplicationConfig.Clusters)
	s.Equal(configVersion, resp.ConfigVersion)
	s.Equal(failoverVersion, resp.FailoverVersion)
	s.Equal(constants.InitialPreviousFailoverVersion, resp.PreviousFailoverVersion)
	s.Equal(int64(0), resp.FailoverNotificationVersion)
	s.Equal(notificationVersion, resp.NotificationVersion)
}
