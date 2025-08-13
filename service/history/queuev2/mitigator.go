//go:generate mockgen -package $GOPACKAGE -destination mitigator_mock.go github.com/uber/cadence/service/history/queuev2 Mitigator
package queuev2

import (
	"maps"
	"slices"
	"time"

	"github.com/uber/cadence/common/collection"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

const (
	targetLoadFactor           = 0.8
	clearSliceThrottleDuration = 10 * time.Second
)

type (
	Mitigator interface {
		Mitigate(Alert)
	}

	MitigatorOptions struct {
		MaxVirtualQueueCount dynamicproperties.IntPropertyFn
	}

	mitigatorImpl struct {
		virtualQueueManager VirtualQueueManager
		monitor             Monitor
		logger              log.Logger
		metricsScope        metrics.Scope
		options             *MitigatorOptions

		handlers map[AlertType]func(Alert)
	}

	pendingTaskStats struct {
		totalPendingTaskCount             int
		pendingTaskCountPerDomain         map[string]int
		pendingTaskCountPerDomainPerSlice map[VirtualSlice]map[string]int
		slicesPerDomain                   map[string][]VirtualSlice
	}
)

func NewMitigator(
	virtualQueueManager VirtualQueueManager,
	monitor Monitor,
	logger log.Logger,
	metricsScope metrics.Scope,
	options *MitigatorOptions,
) Mitigator {
	m := &mitigatorImpl{
		monitor:      monitor,
		logger:       logger,
		metricsScope: metricsScope,
		options:      options,
	}
	m.handlers = map[AlertType]func(Alert){
		AlertTypeQueuePendingTaskCount: m.handleQueuePendingTaskCount,
	}
	return m
}

func (m *mitigatorImpl) Mitigate(alert Alert) {
	handler, ok := m.handlers[alert.AlertType]
	if !ok {
		m.logger.Error("unknown queue alert type", tag.AlertType(int(alert.AlertType)))
		return
	}
	handler(alert)

	m.monitor.ResolveAlert(alert.AlertType)
	m.logger.Info("mitigated queue alert", tag.AlertType(int(alert.AlertType)))
}

func (m *mitigatorImpl) handleQueuePendingTaskCount(alert Alert) {
	// First, try cleaning up tasks that has already been acknowledged to see if we can reduce the pending task count
	virtualQueues := m.virtualQueueManager.VirtualQueues()
	for _, virtualQueue := range virtualQueues {
		virtualQueue.UpdateAndGetState()
	}
	if m.monitor.GetTotalPendingTaskCount() <= alert.AlertAttributesQueuePendingTaskCount.CriticalPendingTaskCount {
		return
	}
	// Second, getting the stats of pending tasks. We need:
	stats := m.collectPendingTaskStats()

	// Third, find virtual slices to split given the target pending task count and the stats of pending tasks
	targetPendingTaskCount := int(float64(alert.AlertAttributesQueuePendingTaskCount.CriticalPendingTaskCount) * targetLoadFactor)
	domainsToClearPerSlice := m.findDomainsToClear(stats, targetPendingTaskCount)

	// Finally, split and clear the slices
	m.processQueueSplitsAndClear(virtualQueues, domainsToClearPerSlice)
}

// The stats of pending tasks are used to calculate the domains to clear. We need:
// 1. The total number of pending tasks per domain
// 2. The number of pending tasks per domain per slice
// 3. The slices that contains the tasks for each domain
func (m *mitigatorImpl) collectPendingTaskStats() pendingTaskStats {
	stats := pendingTaskStats{
		pendingTaskCountPerDomain:         make(map[string]int),
		pendingTaskCountPerDomainPerSlice: make(map[VirtualSlice]map[string]int),
		slicesPerDomain:                   make(map[string][]VirtualSlice),
	}

	for _, virtualQueue := range m.virtualQueueManager.VirtualQueues() {
		virtualQueue.IterateSlices(func(slice VirtualSlice) {
			perDomain := slice.PendingTaskStats().PendingTaskCountPerDomain
			stats.pendingTaskCountPerDomainPerSlice[slice] = perDomain
			for domain, count := range perDomain {
				stats.totalPendingTaskCount += count
				stats.pendingTaskCountPerDomain[domain] += count
				stats.slicesPerDomain[domain] = append(stats.slicesPerDomain[domain], slice)
			}
		})
	}

	for _, slicesList := range stats.slicesPerDomain {
		slices.SortFunc(slicesList, func(a, b VirtualSlice) int {
			return b.GetState().Range.InclusiveMinTaskKey.Compare(a.GetState().Range.InclusiveMinTaskKey)
		})
	}
	return stats
}

func (m *mitigatorImpl) findDomainsToClear(stats pendingTaskStats, targetCount int) map[VirtualSlice][]string {
	domainsToClear := make(map[VirtualSlice][]string)

	pq := collection.NewPriorityQueue(
		func(a, b string) bool {
			return stats.pendingTaskCountPerDomain[a] > stats.pendingTaskCountPerDomain[b]
		},
		slices.Collect(maps.Keys(stats.pendingTaskCountPerDomain))...,
	)

	for stats.totalPendingTaskCount > targetCount && !pq.IsEmpty() {
		domain, err := pq.Remove()
		if err != nil {
			// this should never happen because we check the priority queue is not empty before calling Remove
			// but just want to be future proof
			m.logger.Error("failed to remove domain from priority queue with unexpected error", tag.Error(err))
			panic(err)
		}
		if len(stats.slicesPerDomain[domain]) == 0 {
			continue
		}
		slice := stats.slicesPerDomain[domain][0]
		stats.slicesPerDomain[domain] = stats.slicesPerDomain[domain][1:]

		taskCount := stats.pendingTaskCountPerDomainPerSlice[slice][domain]
		stats.totalPendingTaskCount -= taskCount
		stats.pendingTaskCountPerDomain[domain] -= taskCount
		if stats.pendingTaskCountPerDomain[domain] > 0 {
			pq.Add(domain)
		}
		domainsToClear[slice] = append(domainsToClear[slice], domain)
	}
	return domainsToClear
}

func (m *mitigatorImpl) processQueueSplitsAndClear(virtualQueues map[int64]VirtualQueue, domainsToClear map[VirtualSlice][]string) {
	maxQueueID := m.options.MaxVirtualQueueCount() - 1
	for queueID, vq := range virtualQueues {
		if queueID >= int64(maxQueueID) {
			// Clear slices in the last queue
			cleared := false
			vq.ClearSlices(func(slice VirtualSlice) bool {
				_, ok := domainsToClear[slice]
				cleared = cleared || ok
				return ok
			})
			if cleared {
				vq.Pause(clearSliceThrottleDuration)
			}
			continue
		}

		var slicesToMove []VirtualSlice
		vq.SplitSlices(func(slice VirtualSlice) ([]VirtualSlice, bool) {
			domains := domainsToClear[slice]
			if len(domains) == 0 {
				return nil, false
			}
			predicate := NewDomainIDPredicate(domains, false)
			splitSlice, remainingSlice, ok := slice.TrySplitByPredicate(predicate)
			if !ok {
				slice.Clear()
				slicesToMove = append(slicesToMove, slice)
				return nil, true
			}
			splitSlice.Clear()
			slicesToMove = append(slicesToMove, splitSlice)
			return []VirtualSlice{remainingSlice}, true
		})

		if len(slicesToMove) > 0 {
			nextQueue := m.virtualQueueManager.GetOrCreateVirtualQueue(queueID + 1)
			nextQueue.Pause(clearSliceThrottleDuration)
			nextQueue.MergeSlices(slicesToMove...)
		}
	}
}
