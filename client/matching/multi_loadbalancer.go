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
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type (
	multiLoadBalancer struct {
		defaultLoadBalancer  LoadBalancer
		loadBalancers        map[string]LoadBalancer
		domainIDToName       func(string) (string, error)
		loadbalancerStrategy dynamicconfig.StringPropertyFnWithTaskListInfoFilters
		logger               log.Logger
	}
)

func NewMultiLoadBalancer(
	defaultLoadBalancer LoadBalancer,
	loadBalancers map[string]LoadBalancer,
	domainIDToName func(string) (string, error),
	dc *dynamicconfig.Collection,
	logger log.Logger,
) LoadBalancer {
	return &multiLoadBalancer{
		defaultLoadBalancer:  defaultLoadBalancer,
		loadBalancers:        loadBalancers,
		domainIDToName:       domainIDToName,
		loadbalancerStrategy: dc.GetStringPropertyFilteredByTaskListInfo(dynamicconfig.TasklistLoadBalancerStrategy),
		logger:               logger,
	}
}

func (lb *multiLoadBalancer) PickWritePartition(
	domainID string,
	taskList types.TaskList,
	taskListType int,
	forwardedFrom string,
) string {
	domainName, err := lb.domainIDToName(domainID)
	if err != nil {
		return lb.defaultLoadBalancer.PickWritePartition(domainID, taskList, taskListType, forwardedFrom)
	}
	strategy := lb.loadbalancerStrategy(domainName, taskList.GetName(), taskListType)
	loadBalancer, ok := lb.loadBalancers[strategy]
	if !ok {
		lb.logger.Warn("unsupported load balancer strategy", tag.Value(strategy))
		return lb.defaultLoadBalancer.PickWritePartition(domainID, taskList, taskListType, forwardedFrom)
	}
	return loadBalancer.PickWritePartition(domainID, taskList, taskListType, forwardedFrom)
}

func (lb *multiLoadBalancer) PickReadPartition(
	domainID string,
	taskList types.TaskList,
	taskListType int,
	forwardedFrom string,
) string {
	domainName, err := lb.domainIDToName(domainID)
	if err != nil {
		return lb.defaultLoadBalancer.PickReadPartition(domainID, taskList, taskListType, forwardedFrom)
	}
	strategy := lb.loadbalancerStrategy(domainName, taskList.GetName(), taskListType)
	loadBalancer, ok := lb.loadBalancers[strategy]
	if !ok {
		lb.logger.Warn("unsupported load balancer strategy", tag.Value(strategy))
		return lb.defaultLoadBalancer.PickReadPartition(domainID, taskList, taskListType, forwardedFrom)
	}
	return loadBalancer.PickReadPartition(domainID, taskList, taskListType, forwardedFrom)
}

func (lb *multiLoadBalancer) UpdateWeight(
	domainID string,
	taskList types.TaskList,
	taskListType int,
	forwardedFrom string,
	partition string,
	info *types.LoadBalancerHints,
) {
	domainName, err := lb.domainIDToName(domainID)
	if err != nil {
		return
	}
	strategy := lb.loadbalancerStrategy(domainName, taskList.GetName(), taskListType)
	loadBalancer, ok := lb.loadBalancers[strategy]
	if !ok {
		lb.logger.Warn("unsupported load balancer strategy", tag.Value(strategy))
		return
	}
	loadBalancer.UpdateWeight(domainID, taskList, taskListType, forwardedFrom, partition, info)
}
