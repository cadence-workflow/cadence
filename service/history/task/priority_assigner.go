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

package task

import (
	"sync"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	dynamicquotas "github.com/uber/cadence/common/dynamicconfig/quotas"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

var (
	highTaskPriority    = constants.GetTaskPriority(constants.HighPriorityClass, constants.DefaultPrioritySubclass)
	defaultTaskPriority = constants.GetTaskPriority(constants.DefaultPriorityClass, constants.DefaultPrioritySubclass)
	lowTaskPriority     = constants.GetTaskPriority(constants.LowPriorityClass, constants.DefaultPrioritySubclass)
)

type priorityAssignerImpl struct {
	sync.RWMutex

	currentClusterName string
	domainCache        cache.DomainCache
	config             *config.Config
	logger             log.Logger
	scope              metrics.Scope
	rateLimiters       *quotas.Collection
}

// NewPriorityAssigner creates a new task priority assigner
func NewPriorityAssigner(
	currentClusterName string,
	domainCache cache.DomainCache,
	logger log.Logger,
	metricClient metrics.Client,
	config *config.Config,
) PriorityAssigner {
	return &priorityAssignerImpl{
		currentClusterName: currentClusterName,
		domainCache:        domainCache,
		config:             config,
		logger:             logger,
		scope:              metricClient.Scope(metrics.TaskPriorityAssignerScope),
		rateLimiters: quotas.NewCollection(dynamicquotas.NewSimpleDynamicRateLimiterFactory(
			config.TaskProcessRPS,
		)),
	}
}

func (a *priorityAssignerImpl) Assign(queueTask Task) error {
	if priority := queueTask.Priority(); priority != noPriority {
		if priority != lowTaskPriority && queueTask.GetAttempt() > a.config.TaskCriticalRetryCount() {
			// automatically lower the priority if task attempt exceeds certain threshold
			queueTask.SetPriority(lowTaskPriority)
		}
		return nil
	}

	queueType := queueTask.GetQueueType()

	if queueType == QueueTypeReplication {
		queueTask.SetPriority(lowTaskPriority)
		return nil
	}

	// timer, transfer or cross cluster task, first check if task is active or not and if domain is active or not
	isActiveTask := queueType == QueueTypeActiveTimer || queueType == QueueTypeActiveTransfer
	domainName, isActiveDomain, err := a.getDomainInfo(queueTask.GetDomainID())
	if err != nil {
		return err
	}

	// there are four cases here:
	// 1. active task for active domain
	// 2. active task for standby domain
	// 3. standby task for active domain
	// 4. standby task for standby domain

	if !isActiveTask && !isActiveDomain {
		// only assign low priority to tasks in the fourth case
		queueTask.SetPriority(lowTaskPriority)
		return nil
	}

	// for case 1 we should give the task a high priority
	// for case 2 and 3 the task will be a no-op in most cases, also give it a high priority so that
	// it can be quickly verified/acked and won't prevent the ack level in the processor from advancing
	// (especially for active processor)
	if !a.rateLimiters.For(domainName).Allow() {
		queueTask.SetPriority(defaultTaskPriority)
		taggedScope := a.scope.Tagged(metrics.DomainTag(domainName))
		switch queueType {
		case QueueTypeActiveTransfer, QueueTypeStandbyTransfer:
			taggedScope.IncCounter(metrics.TransferTaskThrottledCounter)
		case QueueTypeActiveTimer, QueueTypeStandbyTimer:
			taggedScope.IncCounter(metrics.TimerTaskThrottledCounter)
		}
		return nil
	}

	queueTask.SetPriority(highTaskPriority)
	return nil
}

// getDomainInfo returns three pieces of information:
//  1. domain name
//  2. if domain is active
//  3. error, if any
func (a *priorityAssignerImpl) getDomainInfo(domainID string) (string, bool, error) {
	domainEntry, err := a.domainCache.GetDomainByID(domainID)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			a.logger.Warn("Cannot find domain", tag.WorkflowDomainID(domainID))
			return "", false, err
		}
		// it is possible that the domain is deleted
		// we should treat that domain as active
		a.logger.Warn("Cannot find domain, treat as active ", tag.WorkflowDomainID(domainID))
		return "", true, nil
	}

	if domainEntry.GetReplicationConfig().IsActiveActive() {
		active := domainEntry.IsActiveIn(a.currentClusterName)
		return domainEntry.GetInfo().Name, active, nil
	}

	// TODO(active-active): The logic below ignores pending active case for active-passive domains.
	// However IsActiveIn() that is used above for active-active domains returns false for pending active domains.
	// What should be the behavior for pending active domains?

	if domainEntry.IsGlobalDomain() && a.currentClusterName != domainEntry.GetReplicationConfig().ActiveClusterName {
		return domainEntry.GetInfo().Name, false, nil
	}
	return domainEntry.GetInfo().Name, true, nil
}
