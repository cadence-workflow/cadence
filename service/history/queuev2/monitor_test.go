package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
)

func TestMonitorPendingTaskCount(t *testing.T) {
	monitor := NewMonitor(persistence.HistoryTaskCategoryTimer, &MonitorOptions{
		CriticalPendingTaskCount:    dynamicproperties.GetIntPropertyFn(100),
		EnablePendingTaskCountAlert: func() bool { return true },
	})

	assert.Equal(t, 0, monitor.GetTotalPendingTaskCount())

	slice1 := &virtualSliceImpl{}
	slice2 := &virtualSliceImpl{}
	slice3 := &virtualSliceImpl{}

	// set pending task count for slices
	monitor.SetSlicePendingTaskCount(slice1, 10)
	assert.Equal(t, 10, monitor.GetTotalPendingTaskCount())
	assert.Equal(t, 10, monitor.GetSlicePendingTaskCount(slice1))

	monitor.SetSlicePendingTaskCount(slice2, 20)
	assert.Equal(t, 30, monitor.GetTotalPendingTaskCount())
	assert.Equal(t, 20, monitor.GetSlicePendingTaskCount(slice2))

	monitor.SetSlicePendingTaskCount(slice3, 30)
	assert.Equal(t, 60, monitor.GetTotalPendingTaskCount())
	assert.Equal(t, 30, monitor.GetSlicePendingTaskCount(slice3))

	// update pending task count for slices
	monitor.SetSlicePendingTaskCount(slice1, 15)
	assert.Equal(t, 65, monitor.GetTotalPendingTaskCount())
	assert.Equal(t, 15, monitor.GetSlicePendingTaskCount(slice1))

	monitor.SetSlicePendingTaskCount(slice2, 21)
	assert.Equal(t, 66, monitor.GetTotalPendingTaskCount())
	assert.Equal(t, 21, monitor.GetSlicePendingTaskCount(slice2))

	monitor.RemoveSlice(slice1)
	assert.Equal(t, 51, monitor.GetTotalPendingTaskCount())

	monitor.RemoveSlice(slice2)
	assert.Equal(t, 30, monitor.GetTotalPendingTaskCount())

	monitor.RemoveSlice(slice3)
	assert.Equal(t, 0, monitor.GetTotalPendingTaskCount())

	alertCh := make(chan *Alert, alertChSize)
	monitor.Subscribe(alertCh)

	monitor.SetSlicePendingTaskCount(slice1, 101)
	assert.Equal(t, 101, monitor.GetTotalPendingTaskCount())
	assert.Equal(t, 101, monitor.GetSlicePendingTaskCount(slice1))

	alert := <-alertCh
	assert.Equal(t, AlertTypeQueuePendingTaskCount, alert.AlertType)
	assert.Equal(t, 101, alert.AlertAttributesQueuePendingTaskCount.CurrentPendingTaskCount)
	assert.Equal(t, 100, alert.AlertAttributesQueuePendingTaskCount.CriticalPendingTaskCount)

	_, ok := monitor.(*monitorImpl).pendingAlerts[AlertTypeQueuePendingTaskCount]
	assert.True(t, ok)
}

func TestMonitorSubscribeAndUnsubscribe(t *testing.T) {
	monitor := NewMonitor(persistence.HistoryTaskCategoryTimer, &MonitorOptions{})

	alertCh := make(chan *Alert, alertChSize)
	monitor.Subscribe(alertCh)
	monitor.(*monitorImpl).subscriber <- &Alert{AlertType: AlertTypeQueuePendingTaskCount}
	alert := <-alertCh
	assert.Equal(t, AlertTypeQueuePendingTaskCount, alert.AlertType)

	monitor.Unsubscribe()
	assert.Nil(t, monitor.(*monitorImpl).subscriber)
}

func TestMonitorResolveAlert(t *testing.T) {
	monitor := NewMonitor(persistence.HistoryTaskCategoryTimer, &MonitorOptions{})

	monitor.(*monitorImpl).pendingAlerts[AlertTypeQueuePendingTaskCount] = struct{}{}
	assert.Equal(t, 1, len(monitor.(*monitorImpl).pendingAlerts))

	monitor.ResolveAlert(AlertTypeQueuePendingTaskCount)
	assert.Equal(t, 0, len(monitor.(*monitorImpl).pendingAlerts))
}
