//go:generate mockgen -package $GOPACKAGE -destination mitigator_mock.go github.com/uber/cadence/service/history/queuev2 Mitigator
package queuev2

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
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
}

func (m *mitigatorImpl) handleQueuePendingTaskCount(alert Alert) {
	// TODO: implement the algorithm to mitigate the queue pending task count
	// the idea is to scan all the virtual queues and get the number of pending tasks in each domain per virtual slice
	// use a greedy algorithm to split virtual slices by the domain with the most number of pending tasks, clear tasks from that domain
	// until the total number of pending tasks is below the critical pending task count
}
