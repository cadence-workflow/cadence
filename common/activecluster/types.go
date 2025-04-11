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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -destination manager_mock.go -self_package github.com/uber/cadence/common/activecluster github.com/uber/cadence/common/activecluster Manager

type LookupResult struct {
	Region          string
	ClusterName     string
	FailoverVersion int64
}

type ChangeType string

const (
	ChangeTypeEntityMap ChangeType = "ChangeTypeEntityMap"
)

type Manager interface {
	common.Daemon

	// LookupExternalEntity returns active cluster, cluster name and failover version of given external entity.
	// Only active-active global domains can have external entities.
	// For each entity type, there must be a corresponding watcher populating EntityActiveRegion table.
	// LookupExternalEntity will return the active cluster name and failover version by checking EntityActiveRegion table.
	LookupExternalEntity(ctx context.Context, entityType, entityKey string) (*LookupResult, error)

	// LookupExternalEntityOfNewWorkflow returns active cluster, cluster name and failover version of given new workflow.
	// Exactly same as LookupExternalEntity except it extracts entityType and entityKey from the start request.
	LookupExternalEntityOfNewWorkflow(ctx context.Context, req *types.HistoryStartWorkflowExecutionRequest) (*LookupResult, error)

	// LookupWorkflow returns active cluster, cluster name and failover version of given workflow.
	//  1. If domain is local:
	//     	Returns current cluster name and domain entry's failover version.
	//  2. If domain is active-passive global:
	//     	Returns domain entry's ActiveClusterName and domain entry's failover version.
	//  3. If domain is active-active global:
	//     	Returns corresponding active cluster name and failover version by checking workflow's activeness metadata and EntityActiveRegion lookup table.
	LookupWorkflow(ctx context.Context, domainID, wfID, rID string) (*LookupResult, error)

	// LookupFailoverVersion returns active cluster, cluster name and failover version of given failover version.
	// For local and active-passive global domains, it returns the value based on cluster metadata.
	// For active-active global domains, it returns the value based on region metadata and domain's cluster list.
	LookupFailoverVersion(failoverVersion int64, domainID string) (*LookupResult, error)

	// RegisterChangeCallback registers a callback that will be called for change events such as entity map changes.
	RegisterChangeCallback(shardID int, callback func(ChangeType))

	// UnregisterChangeCallback unregisters a callback that will be called for change events.
	UnregisterChangeCallback(shardID int)
}
