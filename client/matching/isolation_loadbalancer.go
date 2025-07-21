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

package matching

import (
	"math/rand"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/types"
)

type isolationLoadBalancer struct {
	provider                   PartitionConfigProvider
	fallback                   WeightedLoadBalancer
	domainIDToName             func(string) (string, error)
	isolationAssignmentEnabled func(string) bool
}

func NewIsolationLoadBalancer(fallback WeightedLoadBalancer, provider PartitionConfigProvider, domainIDToName func(string) (string, error), config *dynamicconfig.Collection) LoadBalancer {
	isolationAssignmentEnabled := config.GetBoolPropertyFilteredByDomain(dynamicproperties.EnablePartitionIsolationGroupAssignment)
	return &isolationLoadBalancer{
		provider:                   provider,
		fallback:                   fallback,
		domainIDToName:             domainIDToName,
		isolationAssignmentEnabled: isolationAssignmentEnabled,
	}
}

func (i *isolationLoadBalancer) PickWritePartition(taskListType int, req WriteRequest) string {
	taskList := *req.GetTaskList()

	domainName, err := i.domainIDToName(req.GetDomainUUID())
	if err != nil || !i.isolationAssignmentEnabled(domainName) {
		return i.fallback.PickWritePartition(taskListType, req)
	}

	taskGroup, ok := req.GetPartitionConfig()[isolationgroup.GroupKey]
	if !ok || taskGroup == "" {
		return i.fallback.PickWritePartition(taskListType, req)
	}

	config := i.provider.GetPartitionConfig(req.GetDomainUUID(), taskList, taskListType)
	if len(config.WritePartitions) == 1 {
		return taskList.GetName()
	}

	partitions := getPartitionsForGroup(taskGroup, config.WritePartitions)
	if len(partitions) == 0 {
		return i.fallback.PickWritePartition(taskListType, req)
	}

	p := pickRandom(partitions)

	return getPartitionTaskListName(taskList.GetName(), p)
}

func (i *isolationLoadBalancer) PickReadPartition(taskListType int, req ReadRequest, isolationGroup string) string {
	taskList := *req.GetTaskList()

	domainName, err := i.domainIDToName(req.GetDomainUUID())
	if err != nil || !i.isolationAssignmentEnabled(domainName) || isolationGroup == "" {
		return i.fallback.PickReadPartition(taskListType, req, isolationGroup)
	}

	config := i.provider.GetPartitionConfig(req.GetDomainUUID(), taskList, taskListType)
	if len(config.ReadPartitions) == 1 {
		return taskList.GetName()
	}

	partitions := getPartitionsForGroup(isolationGroup, config.ReadPartitions)
	if len(partitions) == 0 {
		return i.fallback.PickReadPartition(taskListType, req, isolationGroup)
	}

	p := partitions[0]
	if len(partitions) > 1 {
		p = i.fallback.PickBetween(req.GetDomainUUID(), taskList.GetName(), taskListType, partitions)

		if p == -1 {
			p = pickRandom(partitions)
		}
	}

	return getPartitionTaskListName(taskList.GetName(), p)
}

func (i *isolationLoadBalancer) UpdateWeight(taskListType int, req ReadRequest, partition string, info *types.LoadBalancerHints) {
	i.fallback.UpdateWeight(taskListType, req, partition, info)
}

func getPartitionsForGroup(taskGroup string, partitions map[int]*types.TaskListPartition) []int {
	if taskGroup == "" {
		return nil
	}

	var res []int
	for id := range len(partitions) {
		p := partitions[id]
		if partitionAcceptsGroup(p, taskGroup) {
			res = append(res, id)
		}
	}
	return res
}

func pickRandom(partitions []int) int {
	picked := rand.Intn(len(partitions))
	return partitions[picked]
}

func partitionAcceptsGroup(partition *types.TaskListPartition, taskGroup string) bool {
	// Accepts all groups
	if len(partition.IsolationGroups) == 0 {
		return true
	}
	for _, ig := range partition.IsolationGroups {
		if ig == taskGroup {
			return true
		}
	}
	return false
}
