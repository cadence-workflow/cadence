//go:generate mockgen -package $GOPACKAGE -destination monitor_mock.go github.com/uber/cadence/service/history/queuev2 Monitor
package queuev2

import (
	"sync"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
)

type (
	Monitor interface {
		Subscribe(chan<- *Alert)
		Unsubscribe()
		GetTotalPendingTaskCount() int
		GetSlicePendingTaskCount(VirtualSlice) int
		SetSlicePendingTaskCount(VirtualSlice, int)
		RemoveSlice(VirtualSlice)
		ResolveAlert(AlertType)
	}

	MonitorOptions struct {
		EnablePendingTaskCountAlert func() bool
		CriticalPendingTaskCount    dynamicproperties.IntPropertyFn
	}

	monitorImpl struct {
		sync.Mutex

		category persistence.HistoryTaskCategory
		options  *MonitorOptions

		subscriber            chan<- *Alert
		pendingAlerts         map[AlertType]struct{}
		totalPendingTaskCount int
		slicePendingTaskCount map[VirtualSlice]int
	}
)

func NewMonitor(category persistence.HistoryTaskCategory, options *MonitorOptions) Monitor {
	return &monitorImpl{
		category: category,
		options:  options,

		pendingAlerts:         make(map[AlertType]struct{}),
		totalPendingTaskCount: 0,
		slicePendingTaskCount: make(map[VirtualSlice]int),
	}
}

func (m *monitorImpl) Subscribe(subscriber chan<- *Alert) {
	m.Lock()
	defer m.Unlock()

	m.subscriber = subscriber
}

func (m *monitorImpl) Unsubscribe() {
	m.Lock()
	defer m.Unlock()

	m.subscriber = nil
}
func (m *monitorImpl) GetTotalPendingTaskCount() int {
	m.Lock()
	defer m.Unlock()
	return m.totalPendingTaskCount
}

func (m *monitorImpl) GetSlicePendingTaskCount(slice VirtualSlice) int {
	m.Lock()
	defer m.Unlock()
	return m.slicePendingTaskCount[slice]
}

func (m *monitorImpl) SetSlicePendingTaskCount(slice VirtualSlice, count int) {
	m.Lock()
	defer m.Unlock()

	currentSliceCount := m.slicePendingTaskCount[slice]
	m.totalPendingTaskCount += count - currentSliceCount
	m.slicePendingTaskCount[slice] = count

	criticalPendingTaskCount := m.options.CriticalPendingTaskCount()
	if m.options.EnablePendingTaskCountAlert() && criticalPendingTaskCount > 0 && m.totalPendingTaskCount > criticalPendingTaskCount {
		m.sendAlertLocked(&Alert{
			AlertType: AlertTypeQueuePendingTaskCount,
			AlertAttributesQueuePendingTaskCount: &AlertAttributesQueuePendingTaskCount{
				CurrentPendingTaskCount:  m.totalPendingTaskCount,
				CriticalPendingTaskCount: criticalPendingTaskCount,
			},
		})
	}
}

func (m *monitorImpl) RemoveSlice(slice VirtualSlice) {
	m.Lock()
	defer m.Unlock()

	if currentSliceCount, ok := m.slicePendingTaskCount[slice]; ok {
		m.totalPendingTaskCount -= currentSliceCount
		delete(m.slicePendingTaskCount, slice)
	}
}

func (m *monitorImpl) ResolveAlert(alertType AlertType) {
	m.Lock()
	defer m.Unlock()

	delete(m.pendingAlerts, alertType)
}

func (m *monitorImpl) sendAlertLocked(alert *Alert) {
	// deduplicate alerts
	if _, ok := m.pendingAlerts[alert.AlertType]; ok {
		return
	}

	select {
	case m.subscriber <- alert:
		m.pendingAlerts[alert.AlertType] = struct{}{}
	default:
		// do not block if subscriber is not ready
	}
}
