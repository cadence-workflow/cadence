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
	"github.com/uber/cadence/common/partition"
	"github.com/uber/cadence/common/types"
)

type isolationLoadBalancer struct {
	provider         PartitionConfigProvider
	fallback         LoadBalancer
	domainIDToName   func(string) (string, error)
	isolationEnabled func(string) bool
}

func NewIsolationLoadBalancer(fallback LoadBalancer, provider PartitionConfigProvider, domainIDToName func(string) (string, error), config *dynamicconfig.Collection) LoadBalancer {
	isolationEnabled := config.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableTasklistIsolation)
	return &isolationLoadBalancer{
		provider:         provider,
		fallback:         fallback,
		domainIDToName:   domainIDToName,
		isolationEnabled: isolationEnabled,
	}
}

func (i *isolationLoadBalancer) PickWritePartition(taskListType int, req WriteRequest) string {
	taskList := *req.GetTaskList()

	domainName, err := i.domainIDToName(req.GetDomainUUID())
	if err != nil || !i.isolationEnabled(domainName) {
		return i.fallback.PickWritePartition(taskListType, req)
	}

	taskGroup, ok := req.GetPartitionConfig()[partition.IsolationGroupKey]
	if !ok || taskGroup == "" {
		return i.fallback.PickWritePartition(taskListType, req)
	}

	config := i.provider.GetPartitionConfig(req.GetDomainUUID(), taskList, taskListType)

	partitions := getPartitionsForGroup(taskGroup, config.WritePartitions)
	if len(partitions) == 0 {
		return i.fallback.PickWritePartition(taskListType, req)
	}

	p := pickBetween(partitions)

	return getPartitionTaskListName(taskList.GetName(), p)
}

func (i *isolationLoadBalancer) PickReadPartition(taskListType int, req ReadRequest, isolationGroup string) string {
	taskList := *req.GetTaskList()

	domainName, err := i.domainIDToName(req.GetDomainUUID())
	if err != nil || !i.isolationEnabled(domainName) || isolationGroup == "" {
		return i.fallback.PickReadPartition(taskListType, req, isolationGroup)
	}

	config := i.provider.GetPartitionConfig(req.GetDomainUUID(), taskList, taskListType)

	partitions := getPartitionsForGroup(isolationGroup, config.ReadPartitions)
	if len(partitions) == 0 {
		return i.fallback.PickReadPartition(taskListType, req, isolationGroup)
	}

	p := pickBetween(partitions)

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
	for id, p := range partitions {
		if partitionAcceptsGroup(p, taskGroup) {
			res = append(res, id)
		}
	}
	return res
}

func pickBetween(partitions []int) int {
	// Could alternatively use backlog weights to make a smarter choice
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
