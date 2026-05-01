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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination transmissionTaskHandler_mock.go

package domain

import (
	"context"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// NOTE: the counterpart of domain replication receiving logic is in service/worker package

type (
	// Replicator is the interface which can replicate the domain
	Replicator interface {
		HandleTransmissionTask(
			ctx context.Context,
			domainOperation types.DomainOperation,
			info *persistence.DomainInfo,
			config *persistence.DomainConfig,
			replicationConfig *persistence.DomainReplicationConfig,
			configVersion int64,
			failoverVersion int64,
			previousFailoverVersion int64,
			isGlobalDomainEnabled bool,
		) error
	}

	domainReplicatorImpl struct {
		replicationMessageSink messaging.Producer
		logger                 log.Logger
	}
)

// NewDomainReplicator create a new instance of domain replicator
func NewDomainReplicator(replicationMessageSink messaging.Producer, logger log.Logger) Replicator {
	return &domainReplicatorImpl{
		replicationMessageSink: replicationMessageSink,
		logger:                 logger,
	}
}

// HandleTransmissionTask handle transmission of the domain replication task
func (domainReplicator *domainReplicatorImpl) HandleTransmissionTask(
	ctx context.Context,
	domainOperation types.DomainOperation,
	info *persistence.DomainInfo,
	config *persistence.DomainConfig,
	replicationConfig *persistence.DomainReplicationConfig,
	configVersion int64,
	failoverVersion int64,
	previousFailoverVersion int64,
	isGlobalDomainEnabled bool,
) error {

	if !isGlobalDomainEnabled {
		domainReplicator.logger.Warn("Should not replicate non global domain", tag.WorkflowDomainID(info.ID))
		return nil
	}

	status, err := domainReplicator.convertDomainStatusToThrift(info.Status)
	if err != nil {
		return err
	}

	taskType := types.ReplicationTaskTypeDomain
	task := &types.DomainTaskAttributes{
		DomainOperation: &domainOperation,
		ID:              info.ID,
		Info: &types.DomainInfo{
			Name:        info.Name,
			Status:      status,
			Description: info.Description,
			OwnerEmail:  info.OwnerEmail,
			Data:        info.Data,
		},
		Config: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: config.Retention,
			EmitMetric:                             config.EmitMetric,
			HistoryArchivalStatus:                  config.HistoryArchivalStatus.Ptr(),
			HistoryArchivalURI:                     config.HistoryArchivalURI,
			VisibilityArchivalStatus:               config.VisibilityArchivalStatus.Ptr(),
			VisibilityArchivalURI:                  config.VisibilityArchivalURI,
			BadBinaries:                            &config.BadBinaries,
			IsolationGroups:                        &config.IsolationGroups,
			AsyncWorkflowConfig:                    &config.AsyncWorkflowConfig,
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: replicationConfig.ActiveClusterName,
			Clusters:          domainReplicator.convertClusterReplicationConfigToThrift(replicationConfig.Clusters),
			ActiveClusters:    replicationConfig.ActiveClusters,
		},
		ConfigVersion:           configVersion,
		FailoverVersion:         failoverVersion,
		PreviousFailoverVersion: previousFailoverVersion,
	}

	return domainReplicator.replicationMessageSink.Publish(
		ctx,
		&types.ReplicationTask{
			TaskType:             &taskType,
			DomainTaskAttributes: task,
		})
}

func (domainReplicator *domainReplicatorImpl) convertClusterReplicationConfigToThrift(
	input []*persistence.ClusterReplicationConfig,
) []*types.ClusterReplicationConfiguration {
	output := []*types.ClusterReplicationConfiguration{}
	for _, cluster := range input {
		output = append(output, &types.ClusterReplicationConfiguration{ClusterName: cluster.ClusterName})
	}
	return output
}

func (domainReplicator *domainReplicatorImpl) convertDomainStatusToThrift(input int) (*types.DomainStatus, error) {
	switch input {
	case persistence.DomainStatusRegistered:
		output := types.DomainStatusRegistered
		return &output, nil
	case persistence.DomainStatusDeprecated:
		output := types.DomainStatusDeprecated
		return &output, nil
	default:
		return nil, ErrInvalidDomainStatus
	}
}
