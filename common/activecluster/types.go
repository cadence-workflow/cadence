// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package activecluster

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -destination manager_mock.go -self_package github.com/uber/cadence/common/activecluster github.com/uber/cadence/common/activecluster Manager
//go:generate mockgen -package $GOPACKAGE -destination external_entity_provider_mock.go -self_package github.com/uber/cadence/common/activecluster github.com/uber/cadence/common/activecluster ExternalEntityProvider

// Manager is the interface for active cluster manager.
// It is used to lookup region, active cluster, cluster name and failover version etc.
// This was introduced to support active-active domains.
// It encapsulates the logic to lookup the active cluster for all kinds of domains. Most other components should use this interface instead of cluster metadata directly.
// It is also used to notify components when there's an external entity change. History engine subscribes to these updates similar to domain change notifications.
type Manager interface {
	common.Daemon

	// FailoverVersionOfNewWorkflow returns failover version of given new workflow.
	//  1. If domain is local:
	//     	Returns domain entry's failover version.
	//  2. If domain is active-passive global:
	//     	Returns domain entry's failover version.
	//  3. If domain is active-active global:
	//     	3.1. if workflow is region sticky, returns failover version of current cluster.
	//     	3.2. if workflow has external entity, returns failover version of corresponding row in EntityActiveRegion lookup table.
	FailoverVersionOfNewWorkflow(ctx context.Context, req *types.HistoryStartWorkflowExecutionRequest) (int64, error)

	// LookupWorkflow returns active cluster, region and failover version of given workflow.
	// Returns the info from domain entry for local and active-passive domains
	//
	// Active-active domain logic:
	//  1. Get ActivenessMetadata record of the workflow
	//     1.a. If it's found, continue with step 2
	//     1.b. If it's not found, the domain is migrated from active-passive to active-active and workflow is created before migration.
	//     This case will fallback to active-passive logic and return domain's ActiveClusterName and FailoverVersion.
	//  2. Given ActivenessMetadata, return region and failover version
	//     2.a. If workflow is region sticky (origin=regionA), find active cluster in that region in domain's active cluster config and return its name and failover version.
	//     2.b. If workflow has external entity, locate the entity from EntityActiveRegion table and return that region and it's failover version.
	LookupWorkflow(ctx context.Context, domainID, wfID, rID string) (*LookupResult, error)

	// ClusterNameForFailoverVersion returns cluster name of given failover version.
	// For local domains, it returns current cluster name.
	// For active-passive global domains, it returns the cluster name based on cluster metadata that corresponds to the failover version.
	// For active-active global domains, it returns the cluster name based on cluster & region metadata that corresponds to the failover version.
	ClusterNameForFailoverVersion(failoverVersion int64, domainID string) (string, error)

	// RegisterChangeCallback registers a callback that will be called for change events such as entity map changes.
	RegisterChangeCallback(shardID int, callback func(ChangeType))

	// UnregisterChangeCallback unregisters a callback that will be called for change events.
	UnregisterChangeCallback(shardID int)
}

type LookupResult struct {
	Region          string
	ClusterName     string
	FailoverVersion int64
}

type ChangeType string

const (
	ChangeTypeEntityMap ChangeType = "ChangeTypeEntityMap"
)

type WorkflowActivenessMetadata struct {
	Type WorkflowActivenessType

	// Region is the origin region of the workflow.
	// It's set for region sticky workflows.
	Region string

	// EntityKey is the key of the external entity associated with the workflow.
	// It's set for workflows with external entity associations.
	EntityKey string

	// EntitySource is the source of the external entity.
	// It's set for workflows with external entity associations.
	EntitySource string
}

type WorkflowActivenessType string

const (
	WorkflowActivenessTypeRegionSticky   WorkflowActivenessType = "RegionSticky"
	WorkflowActivenessTypeExternalEntity WorkflowActivenessType = "ExternalEntity"
)

type ExternalEntity struct {
	Source          string
	Key             string
	Region          string
	FailoverVersion int64
}

type ExternalEntityProvider interface {
	SupportedSource() string
	ChangeEvents() <-chan ChangeType
	GetExternalEntity(ctx context.Context, entitySKey string) (*ExternalEntity, error)
}

type RegionNotFoundForDomainError struct {
	Region   string
	DomainID string
}

func newRegionNotFoundForDomainError(region, domainID string) *RegionNotFoundForDomainError {
	return &RegionNotFoundForDomainError{
		Region:   region,
		DomainID: domainID,
	}
}

func (e *RegionNotFoundForDomainError) Error() string {
	return fmt.Sprintf("could not find region %s in the domain %s's active cluster config", e.Region, e.DomainID)
}

type ClusterNotFoundForRegionError struct {
	ClusterName string
	Region      string
}

func newClusterNotFoundForRegionError(clusterName, region string) *ClusterNotFoundForRegionError {
	return &ClusterNotFoundForRegionError{
		ClusterName: clusterName,
		Region:      region,
	}
}

func (e *ClusterNotFoundForRegionError) Error() string {
	return fmt.Sprintf("could not find cluster %s for region %s", e.ClusterName, e.Region)
}
