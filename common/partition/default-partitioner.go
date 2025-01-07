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

package partition

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

const (
	IsolationGroupKey = "isolation-group"
	WorkflowIDKey     = "wf-id"
)

// ErrNoIsolationGroupsAvailable is returned when there are no available isolation-groups
// this usually indicates a misconfiguration
var ErrNoIsolationGroupsAvailable = errors.New("no isolation-groups are available")

// ErrInvalidPartitionConfig is returned when the required partitioning configuration
// is missing due to misconfiguration
var ErrInvalidPartitionConfig = errors.New("invalid partition config")

// DefaultWorkflowPartitionConfig Is the default dataset expected to be passed around in the
// execution records for workflows which is used for partitioning. It contains the IsolationGroup
// where the workflow was started, and is expected to be pinned, and a workflow ID  for a fallback means
// to partition data deterministically.
type defaultWorkflowPartitionConfig struct {
	WorkflowStartIsolationGroup string
	WFID                        string
}

// defaultPartitioner is a business-agnostic implementation of partitioning
// which is used by the Cadence system to allocate workflows in matching by isolation-group
type defaultPartitioner struct {
	log                 log.Logger
	isolationGroupState isolationgroup.State
}

func NewDefaultPartitioner(
	logger log.Logger,
	isolationGroupState isolationgroup.State,
) Partitioner {
	return &defaultPartitioner{
		log:                 logger,
		isolationGroupState: isolationGroupState,
	}
}

func (r *defaultPartitioner) GetIsolationGroupByDomainID(ctx context.Context, pollerInfo PollerInfo, wfPartitionData PartitionConfig) (string, error) {
	if wfPartitionData == nil {
		return "", ErrInvalidPartitionConfig
	}
	wfPartition := mapPartitionConfigToDefaultPartitionConfig(wfPartitionData)
	if wfPartition.WorkflowStartIsolationGroup == "" || wfPartition.WFID == "" {
		return "", ErrInvalidPartitionConfig
	}

	available, err := r.isolationGroupState.AvailableIsolationGroupsByDomainID(ctx, pollerInfo.DomainID, pollerInfo.TasklistName, pollerInfo.AvailableIsolationGroups)
	if err != nil {
		return "", fmt.Errorf("failed to get available isolation groups: %w", err)
	}

	if len(available) == 0 {
		return "", ErrNoIsolationGroupsAvailable
	}

	ig := r.pickIsolationGroup(wfPartition, available, pollerInfo)
	return ig, nil
}

func mapPartitionConfigToDefaultPartitionConfig(config PartitionConfig) defaultWorkflowPartitionConfig {
	isolationGroup, _ := config[IsolationGroupKey]
	wfID, _ := config[WorkflowIDKey]
	return defaultWorkflowPartitionConfig{
		WorkflowStartIsolationGroup: isolationGroup,
		WFID:                        wfID,
	}
}

// picks an isolation group to run in. if the workflow was started there, it'll attempt to pin it, unless there is an explicit
// drain.
func (r *defaultPartitioner) pickIsolationGroup(wfPartition defaultWorkflowPartitionConfig, available types.IsolationGroupConfiguration, pollerInfo PollerInfo) string {
	_, isAvailable := available[wfPartition.WorkflowStartIsolationGroup]
	if isAvailable {
		return wfPartition.WorkflowStartIsolationGroup
	}
	r.log.Debug("isolation group falling back to any zone",
		tag.IsolationGroup(wfPartition.WorkflowStartIsolationGroup),
		tag.PollerGroupsConfiguration(available),
		tag.WorkflowTaskListName(pollerInfo.TasklistName),
		tag.WorkflowID(wfPartition.WFID),
	)
	return ""
}
