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
	"strings"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type (
	multiLoadBalancer struct {
		defaultLoadBalancer  LoadBalancer
		loadBalancers        map[string]LoadBalancer
		domainIDToName       func(string) (string, error)
		loadbalancerStrategy dynamicproperties.StringPropertyFnWithTaskListInfoFilters
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
		loadbalancerStrategy: dc.GetStringPropertyFilteredByTaskListInfo(dynamicproperties.TasklistLoadBalancerStrategy),
		logger:               logger,
	}
}

func (lb *multiLoadBalancer) PickWritePartition(
	taskListType int,
	req WriteRequest,
) string {
	if !lb.canRedirectToPartition(req) {
		return req.GetTaskList().GetName()
	}
	domainName, err := lb.domainIDToName(req.GetDomainUUID())
	if err != nil {
		return lb.defaultLoadBalancer.PickWritePartition(taskListType, req)
	}
	strategy := lb.loadbalancerStrategy(domainName, req.GetTaskList().GetName(), taskListType)
	loadBalancer, ok := lb.loadBalancers[strategy]
	if !ok {
		lb.logger.Warn("unsupported load balancer strategy", tag.Value(strategy))
		return lb.defaultLoadBalancer.PickWritePartition(taskListType, req)
	}
	return loadBalancer.PickWritePartition(taskListType, req)
}

func (lb *multiLoadBalancer) PickReadPartition(
	taskListType int,
	req ReadRequest,
	isolationGroup string,
) string {
	if !lb.canRedirectToPartition(req) {
		return req.GetTaskList().GetName()
	}
	domainName, err := lb.domainIDToName(req.GetDomainUUID())
	if err != nil {
		return lb.defaultLoadBalancer.PickReadPartition(taskListType, req, isolationGroup)
	}
	strategy := lb.loadbalancerStrategy(domainName, req.GetTaskList().GetName(), taskListType)
	loadBalancer, ok := lb.loadBalancers[strategy]
	if !ok {
		lb.logger.Warn("unsupported load balancer strategy", tag.Value(strategy))
		return lb.defaultLoadBalancer.PickReadPartition(taskListType, req, isolationGroup)
	}
	return loadBalancer.PickReadPartition(taskListType, req, isolationGroup)
}

func (lb *multiLoadBalancer) UpdateWeight(
	taskListType int,
	req ReadRequest,
	partition string,
	info *types.LoadBalancerHints,
) {
	if !lb.canRedirectToPartition(req) {
		return
	}
	domainName, err := lb.domainIDToName(req.GetDomainUUID())
	if err != nil {
		return
	}
	strategy := lb.loadbalancerStrategy(domainName, req.GetTaskList().GetName(), taskListType)
	loadBalancer, ok := lb.loadBalancers[strategy]
	if !ok {
		lb.logger.Warn("unsupported load balancer strategy", tag.Value(strategy))
		return
	}
	loadBalancer.UpdateWeight(taskListType, req, partition, info)
}

func (lb *multiLoadBalancer) canRedirectToPartition(req ReadRequest) bool {
	return req.GetForwardedFrom() == "" && req.GetTaskList().GetKind() == types.TaskListKindNormal && !strings.HasPrefix(req.GetTaskList().GetName(), constants.ReservedTaskListPrefix)
}
