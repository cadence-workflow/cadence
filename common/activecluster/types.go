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

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -destination manager_mock.go -self_package github.com/uber/cadence/common/activecluster github.com/uber/cadence/common/activecluster Manager
//go:generate mockgen -package $GOPACKAGE -destination execution_manager_provider_mock.go -self_package github.com/uber/cadence/common/activecluster github.com/uber/cadence/common/activecluster ExecutionManagerProvider

// Manager is the interface for active cluster manager.
// It is used to get active cluster info by cluster attribute or workflow.
type Manager interface {
	// GetActiveClusterInfoByClusterAttribute returns the active cluster info by cluster attribute
	// If clusterAttribute is nil, returns the domain-level active cluster info
	// If clusterAttribute is not nil and exists in the domain metadata, returns the active cluster info of the cluster attribute
	// If clusterAttribute is not nil and does not exist in the domain metadata, returns an error
	GetActiveClusterInfoByClusterAttribute(ctx context.Context, domainID string, clusterAttribute *types.ClusterAttribute) (*types.ActiveClusterInfo, error)

	// GetActiveClusterInfoByWorkflow returns the active cluster info by workflow
	// It will first look up the cluster selection policy for the workflow and then get the active cluster info by cluster attribute from the policy
	GetActiveClusterInfoByWorkflow(ctx context.Context, domainID, wfID, rID string) (*types.ActiveClusterInfo, error)

	// GetActiveClusterSelectionPolicyForWorkflow returns the active cluster selection policy for a workflow
	GetActiveClusterSelectionPolicyForWorkflow(ctx context.Context, domainID, wfID, rID string) (*types.ActiveClusterSelectionPolicy, error)

	// GetActiveClusterSelectionPolicyForCurrentWorkflow returns the active cluster selection policy for the current workflow
	// if the workflow is NOT closed, returns policy and true, otherwise returns nil and false
	GetActiveClusterSelectionPolicyForCurrentWorkflow(ctx context.Context, domainID, wfID string) (*types.ActiveClusterSelectionPolicy, bool, error)
}

type ExecutionManagerProvider interface {
	GetExecutionManager(shardID int) (persistence.ExecutionManager, error)
}

type ClusterAttributeNotFoundError struct {
	DomainID         string
	ClusterAttribute *types.ClusterAttribute
	// ActiveClusterInfo *types.ActiveClusterInfo
}

func (e *ClusterAttributeNotFoundError) Error() string {
	return fmt.Sprintf("could not find cluster attribute %s in the domain %s's active cluster config", e.ClusterAttribute, e.DomainID)
}
