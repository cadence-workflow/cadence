package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/persistence"
)

func TestMonitorPendingTaskCount(t *testing.T) {
	monitor := NewMonitor(persistence.HistoryTaskCategoryTimer)

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
}
