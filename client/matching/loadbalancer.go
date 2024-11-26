// Copyright (c) 2019 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination loadbalancer_mock.go -package matching github.com/uber/cadence/client/matching LoadBalancer

package matching

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

type (
	// LoadBalancer is the interface for implementers of
	// component that distributes add/poll api calls across
	// available task list partitions when possible
	LoadBalancer interface {
		// PickWritePartition returns the task list partition for adding
		// an activity or decision task. The input is the name of the
		// original task list (with no partition info). When forwardedFrom
		// is non-empty, this call is forwardedFrom from a child partition
		// to a parent partition in which case, no load balancing should be
		// performed
		PickWritePartition(
			domainID string,
			taskList types.TaskList,
			taskListType int,
			forwardedFrom string,
		) string

		// PickReadPartition returns the task list partition to send a poller to.
		// Input is name of the original task list as specified by caller. When
		// forwardedFrom is non-empty, no load balancing should be done.
		PickReadPartition(
			domainID string,
			taskList types.TaskList,
			taskListType int,
			forwardedFrom string,
		) string

		// UpdateWeight updates the weight of a task list partition.
		// Input is name of the original task list as specified by caller. When
		// the original task list is a partition, no update should be done.
		UpdateWeight(
			domainID string,
			taskList types.TaskList,
			taskListType int,
			forwardedFrom string,
			partition string,
			info *types.LoadBalancerHints,
		)
	}

	defaultLoadBalancer struct {
		provider PartitionConfigProvider
	}
)

// NewLoadBalancer returns an instance of matching load balancer that
// can help distribute api calls across task list partitions
func NewLoadBalancer(
	provider PartitionConfigProvider,
) LoadBalancer {
	return &defaultLoadBalancer{
		provider: provider,
	}
}

func (lb *defaultLoadBalancer) PickWritePartition(
	domainID string,
	taskList types.TaskList,
	taskListType int,
	forwardedFrom string,
) string {
	nPartitions := lb.provider.GetNumberOfWritePartitions(domainID, taskList, taskListType)
	return lb.pickPartition(taskList, forwardedFrom, nPartitions)

}

func (lb *defaultLoadBalancer) PickReadPartition(
	domainID string,
	taskList types.TaskList,
	taskListType int,
	forwardedFrom string,
) string {
	n := lb.provider.GetNumberOfReadPartitions(domainID, taskList, taskListType)
	return lb.pickPartition(taskList, forwardedFrom, n)

}

func (lb *defaultLoadBalancer) pickPartition(
	taskList types.TaskList,
	forwardedFrom string,
	nPartitions int,
) string {

	if forwardedFrom != "" || taskList.GetKind() == types.TaskListKindSticky {
		return taskList.GetName()
	}

	if strings.HasPrefix(taskList.GetName(), common.ReservedTaskListPrefix) {
		// this should never happen when forwardedFrom is empty
		return taskList.GetName()
	}

	if nPartitions <= 0 {
		return taskList.GetName()
	}

	p := rand.Intn(nPartitions)
	return getPartitionTaskListName(taskList.GetName(), p)
}

func (lb *defaultLoadBalancer) UpdateWeight(
	domainID string,
	taskList types.TaskList,
	taskListType int,
	forwardedFrom string,
	partition string,
	info *types.LoadBalancerHints,
) {
}

func getPartitionTaskListName(root string, partition int) string {
	if partition <= 0 {
		return root
	}
	return fmt.Sprintf("%v%v/%v", common.ReservedTaskListPrefix, root, partition)
}
