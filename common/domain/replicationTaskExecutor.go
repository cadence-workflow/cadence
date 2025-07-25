// Copyright (c) 2020 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination replicationTaskHandler_mock.go

package domain

import (
	"context"
	"errors"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var (
	// ErrEmptyDomainReplicationTask is the error to indicate empty replication task
	ErrEmptyDomainReplicationTask = &types.BadRequestError{Message: "empty domain replication task"}
	// ErrInvalidDomainOperation is the error to indicate empty domain operation attribute
	ErrInvalidDomainOperation = &types.BadRequestError{Message: "invalid domain operation attribute"}
	// ErrInvalidDomainID is the error to indicate empty rID attribute
	ErrInvalidDomainID = &types.BadRequestError{Message: "invalid domain ID attribute"}
	// ErrInvalidDomainInfo is the error to indicate empty info attribute
	ErrInvalidDomainInfo = &types.BadRequestError{Message: "invalid domain info attribute"}
	// ErrInvalidDomainConfig is the error to indicate empty config attribute
	ErrInvalidDomainConfig = &types.BadRequestError{Message: "invalid domain config attribute"}
	// ErrInvalidDomainReplicationConfig is the error to indicate empty replication config attribute
	ErrInvalidDomainReplicationConfig = &types.BadRequestError{Message: "invalid domain replication config attribute"}
	// ErrInvalidDomainStatus is the error to indicate invalid domain status
	ErrInvalidDomainStatus = &types.BadRequestError{Message: "invalid domain status attribute"}
	// ErrNameUUIDCollision is the error to indicate domain name / UUID collision
	ErrNameUUIDCollision = &types.BadRequestError{Message: "domain replication encounter name / UUID collision"}
)

const (
	defaultDomainRepliationTaskContextTimeout = 5 * time.Second
)

// NOTE: the counterpart of domain replication transmission logic is in service/fropntend package

type (
	// ReplicationTaskExecutor is the interface which is to execute domain replication task
	ReplicationTaskExecutor interface {
		Execute(task *types.DomainTaskAttributes) error
	}

	domainReplicationTaskExecutorImpl struct {
		domainManager persistence.DomainManager
		timeSource    clock.TimeSource
		logger        log.Logger
	}
)

// NewReplicationTaskExecutor create a new instance of domain replicator
func NewReplicationTaskExecutor(
	domainManager persistence.DomainManager,
	timeSource clock.TimeSource,
	logger log.Logger,
) ReplicationTaskExecutor {

	return &domainReplicationTaskExecutorImpl{
		domainManager: domainManager,
		timeSource:    timeSource,
		logger:        logger,
	}
}

// Execute handles receiving of the domain replication task
func (h *domainReplicationTaskExecutorImpl) Execute(task *types.DomainTaskAttributes) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDomainRepliationTaskContextTimeout)
	defer cancel()

	if err := h.validateDomainReplicationTask(task); err != nil {
		return err
	}

	switch task.GetDomainOperation() {
	case types.DomainOperationCreate:
		return h.handleDomainCreationReplicationTask(ctx, task)
	case types.DomainOperationUpdate:
		return h.handleDomainUpdateReplicationTask(ctx, task)
	case types.DomainOperationDelete:
		return h.handleDomainDeleteReplicationTask(ctx, task)
	default:
		return ErrInvalidDomainOperation
	}
}

// handleDomainCreationReplicationTask handles the domain creation replication task
func (h *domainReplicationTaskExecutorImpl) handleDomainCreationReplicationTask(ctx context.Context, task *types.DomainTaskAttributes) error {
	// task already validated
	status, err := h.convertDomainStatusFromThrift(task.Info.Status)
	if err != nil {
		return err
	}

	request := &persistence.CreateDomainRequest{
		Info: &persistence.DomainInfo{
			ID:          task.GetID(),
			Name:        task.Info.GetName(),
			Status:      status,
			Description: task.Info.GetDescription(),
			OwnerEmail:  task.Info.GetOwnerEmail(),
			Data:        task.Info.Data,
		},
		Config: &persistence.DomainConfig{
			Retention:                task.Config.GetWorkflowExecutionRetentionPeriodInDays(),
			EmitMetric:               task.Config.GetEmitMetric(),
			HistoryArchivalStatus:    task.Config.GetHistoryArchivalStatus(),
			HistoryArchivalURI:       task.Config.GetHistoryArchivalURI(),
			VisibilityArchivalStatus: task.Config.GetVisibilityArchivalStatus(),
			VisibilityArchivalURI:    task.Config.GetVisibilityArchivalURI(),
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: task.ReplicationConfig.GetActiveClusterName(),
			Clusters:          h.convertClusterReplicationConfigFromThrift(task.ReplicationConfig.Clusters),
			ActiveClusters:    task.ReplicationConfig.GetActiveClusters(),
		},
		IsGlobalDomain:  true, // local domain will not be replicated
		ConfigVersion:   task.GetConfigVersion(),
		FailoverVersion: task.GetFailoverVersion(),
		LastUpdatedTime: h.timeSource.Now().UnixNano(),
	}

	_, err = h.domainManager.CreateDomain(ctx, request)
	if err != nil {
		// SQL and Cassandra handle domain UUID collision differently
		// here, whenever seeing a error replicating a domain
		// do a check if there is a name / UUID collision

		recordExists := true
		resp, getErr := h.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{
			Name: task.Info.GetName(),
		})
		switch getErr.(type) {
		case nil:
			if resp.Info.ID != task.GetID() {
				return ErrNameUUIDCollision
			}
		case *types.EntityNotExistsError:
			// no check is necessary
			recordExists = false
		default:
			// return the original err
			return err
		}

		resp, getErr = h.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{
			ID: task.GetID(),
		})
		switch getErr.(type) {
		case nil:
			if resp.Info.Name != task.Info.GetName() {
				return ErrNameUUIDCollision
			}
		case *types.EntityNotExistsError:
			// no check is necessary
			recordExists = false
		default:
			// return the original err
			return err
		}

		if recordExists {
			// name -> id & id -> name check pass, this is duplication request
			return nil
		}
		return err
	}

	return err
}

// handleDomainUpdateReplicationTask handles the domain update replication task
func (h *domainReplicationTaskExecutorImpl) handleDomainUpdateReplicationTask(ctx context.Context, task *types.DomainTaskAttributes) error {
	// task already validated
	status, err := h.convertDomainStatusFromThrift(task.Info.Status)
	if err != nil {
		return err
	}

	// first we need to get the current notification version since we need to it for conditional update
	metadata, err := h.domainManager.GetMetadata(ctx)
	if err != nil {
		h.logger.Error("Error getting metadata while handling replication task", tag.Error(err))
		return err
	}
	notificationVersion := metadata.NotificationVersion

	// plus, we need to check whether the config version is <= the config version set in the input
	// plus, we need to check whether the failover version is <= the failover version set in the input
	resp, err := h.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{
		Name: task.Info.GetName(),
	})
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); ok {
			// this can happen if the create domain replication task is to processed.
			// e.g. new cluster which does not have anything
			return h.handleDomainCreationReplicationTask(ctx, task)
		}
		h.logger.Error("Domain update failed, error in fetching domain", tag.Error(err))
		return err
	}

	recordUpdated := false
	request := &persistence.UpdateDomainRequest{
		Info:                        resp.Info,
		Config:                      resp.Config,
		ReplicationConfig:           resp.ReplicationConfig,
		ConfigVersion:               resp.ConfigVersion,
		FailoverVersion:             resp.FailoverVersion,
		FailoverNotificationVersion: resp.FailoverNotificationVersion,
		PreviousFailoverVersion:     resp.PreviousFailoverVersion,
		NotificationVersion:         notificationVersion,
		LastUpdatedTime:             h.timeSource.Now().UnixNano(),
	}

	if resp.ConfigVersion < task.GetConfigVersion() {
		recordUpdated = true
		request.Info = &persistence.DomainInfo{
			ID:          task.GetID(),
			Name:        task.Info.GetName(),
			Status:      status,
			Description: task.Info.GetDescription(),
			OwnerEmail:  task.Info.GetOwnerEmail(),
			Data:        task.Info.Data,
		}
		request.Config = &persistence.DomainConfig{
			Retention:                task.Config.GetWorkflowExecutionRetentionPeriodInDays(),
			EmitMetric:               task.Config.GetEmitMetric(),
			HistoryArchivalStatus:    task.Config.GetHistoryArchivalStatus(),
			HistoryArchivalURI:       task.Config.GetHistoryArchivalURI(),
			VisibilityArchivalStatus: task.Config.GetVisibilityArchivalStatus(),
			VisibilityArchivalURI:    task.Config.GetVisibilityArchivalURI(),
			IsolationGroups:          task.Config.GetIsolationGroupsConfiguration(),
			AsyncWorkflowConfig:      task.Config.GetAsyncWorkflowConfiguration(),
		}
		if task.Config.GetBadBinaries() != nil {
			request.Config.BadBinaries = *task.Config.GetBadBinaries()
		}
		request.ReplicationConfig.Clusters = h.convertClusterReplicationConfigFromThrift(task.ReplicationConfig.Clusters)
		request.ConfigVersion = task.GetConfigVersion()
	}

	// TODO(active-active): Domain's failover version has to be updated for the below case.
	// However active-active domains don't need that field at all. We still increment it in domain handler whenever ActiveClusters change.
	// Find another mechanism to indicate ActiveClusters changed and don't touch domain's top level failover version for active-active domains.
	if resp.FailoverVersion < task.GetFailoverVersion() {
		recordUpdated = true
		request.ReplicationConfig.ActiveClusterName = task.ReplicationConfig.GetActiveClusterName()
		request.ReplicationConfig.ActiveClusters = task.ReplicationConfig.GetActiveClusters()
		request.FailoverVersion = task.GetFailoverVersion()
		request.FailoverNotificationVersion = notificationVersion
		request.PreviousFailoverVersion = task.GetPreviousFailoverVersion()
	}

	if !recordUpdated {
		return nil
	}

	return h.domainManager.UpdateDomain(ctx, request)
}

// handleDomainDeleteReplicationTask handles the domain delete replication task
func (h *domainReplicationTaskExecutorImpl) handleDomainDeleteReplicationTask(ctx context.Context, task *types.DomainTaskAttributes) error {
	request := &persistence.DeleteDomainByNameRequest{
		Name: task.Info.GetName(),
	}

	err := h.domainManager.DeleteDomainByName(ctx, request)
	if err != nil {
		_, err := h.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{
			Name: task.Info.GetName(),
		})

		var entityNotExistsError *types.EntityNotExistsError
		if errors.As(err, &entityNotExistsError) {
			return nil
		}
	}
	return err
}

func (h *domainReplicationTaskExecutorImpl) validateDomainReplicationTask(task *types.DomainTaskAttributes) error {
	if task == nil {
		return ErrEmptyDomainReplicationTask
	}

	if task.DomainOperation == nil {
		return ErrInvalidDomainOperation
	} else if task.ID == "" {
		return ErrInvalidDomainID
	} else if task.Info == nil {
		return ErrInvalidDomainInfo
	} else if task.Config == nil {
		return ErrInvalidDomainConfig
	} else if task.ReplicationConfig == nil {
		return ErrInvalidDomainReplicationConfig
	}
	return nil
}

func (h *domainReplicationTaskExecutorImpl) convertClusterReplicationConfigFromThrift(
	input []*types.ClusterReplicationConfiguration) []*persistence.ClusterReplicationConfig {
	output := []*persistence.ClusterReplicationConfig{}
	for _, cluster := range input {
		clusterName := cluster.GetClusterName()
		output = append(output, &persistence.ClusterReplicationConfig{ClusterName: clusterName})
	}
	return output
}

func (h *domainReplicationTaskExecutorImpl) convertDomainStatusFromThrift(input *types.DomainStatus) (int, error) {
	if input == nil {
		return 0, ErrInvalidDomainStatus
	}

	switch *input {
	case types.DomainStatusRegistered:
		return persistence.DomainStatusRegistered, nil
	case types.DomainStatusDeprecated:
		return persistence.DomainStatusDeprecated, nil
	default:
		return 0, ErrInvalidDomainStatus
	}
}
