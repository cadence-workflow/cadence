package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestNewMitigator(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	logger := testlogger.New(t)
	metricsScope := metrics.NoopScope
	options := &MitigatorOptions{
		MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
	}

	mitigator := NewMitigator(
		mockVirtualQueueManager,
		mockMonitor,
		logger,
		metricsScope,
		options,
	)

	require.NotNil(t, mitigator)

	// Verify internal structure
	impl, ok := mitigator.(*mitigatorImpl)
	require.True(t, ok)
	assert.Equal(t, mockMonitor, impl.monitor)
	assert.Equal(t, logger, impl.logger)
	assert.Equal(t, metricsScope, impl.metricsScope)
	assert.Equal(t, options, impl.options)

	// Verify handlers are properly initialized
	assert.NotNil(t, impl.handlers)
	assert.Len(t, impl.handlers, 1)
	_, exists := impl.handlers[AlertTypeQueuePendingTaskCount]
	assert.True(t, exists)
}

func TestMitigator_Mitigate_KnownAlertType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	logger := testlogger.New(t)
	metricsScope := metrics.NoopScope
	options := &MitigatorOptions{
		MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
	}

	mitigator := NewMitigator(
		mockVirtualQueueManager,
		mockMonitor,
		logger,
		metricsScope,
		options,
	)
	impl, ok := mitigator.(*mitigatorImpl)
	require.True(t, ok)
	handlerCalled := false
	impl.handlers[AlertTypeQueuePendingTaskCount] = func(alert Alert) {
		handlerCalled = true
	}

	alert := Alert{
		AlertType: AlertTypeQueuePendingTaskCount,
		AlertAttributesQueuePendingTaskCount: &AlertAttributesQueuePendingTaskCount{
			CurrentPendingTaskCount:  150,
			CriticalPendingTaskCount: 100,
		},
	}

	// Expect ResolveAlert to be called on the monitor
	mockMonitor.EXPECT().ResolveAlert(AlertTypeQueuePendingTaskCount).Times(1)

	mitigator.Mitigate(alert)
	assert.True(t, handlerCalled)
}

func TestMitigator_Mitigate_UnknownAlertType(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockVirtualQueueManager := NewMockVirtualQueueManager(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	logger := testlogger.New(t)
	metricsScope := metrics.NoopScope
	options := &MitigatorOptions{
		MaxVirtualQueueCount: dynamicproperties.GetIntPropertyFn(10),
	}

	mitigator := NewMitigator(
		mockVirtualQueueManager,
		mockMonitor,
		logger,
		metricsScope,
		options,
	)

	// Create an alert with an unknown/unhandled alert type
	unknownAlertType := AlertType(999)
	alert := Alert{
		AlertType: unknownAlertType,
	}

	mitigator.Mitigate(alert)
}
