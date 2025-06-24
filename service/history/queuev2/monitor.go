//go:generate mockgen -package $GOPACKAGE -destination monitor_mock.go github.com/uber/cadence/service/history/queuev2 Monitor
package queuev2

import (
	"sync"

	"github.com/uber/cadence/common/persistence"
)

type (
	Monitor interface {
		GetTotalPendingTaskCount() int
		GetSlicePendingTaskCount(VirtualSlice) int
		SetSlicePendingTaskCount(VirtualSlice, int)
		RemoveSlice(VirtualSlice)
	}

	monitorImpl struct {
		sync.Mutex

		category persistence.HistoryTaskCategory

		totalPendingTaskCount int
		slicePendingTaskCount map[VirtualSlice]int
	}
)

func NewMonitor(category persistence.HistoryTaskCategory) Monitor {
	return &monitorImpl{
		category: category,

		totalPendingTaskCount: 0,
		slicePendingTaskCount: make(map[VirtualSlice]int),
	}
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
}

func (m *monitorImpl) RemoveSlice(slice VirtualSlice) {
	m.Lock()
	defer m.Unlock()

	if currentSliceCount, ok := m.slicePendingTaskCount[slice]; ok {
		m.totalPendingTaskCount -= currentSliceCount
		delete(m.slicePendingTaskCount, slice)
	}
}
