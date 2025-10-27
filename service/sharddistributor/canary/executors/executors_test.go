package executors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/executorclient"
)

// mockLifecycle is a simple mock implementation of fx.Lifecycle for testing
type mockLifecycle struct {
	hookCount int
}

func (m *mockLifecycle) Append(hook fx.Hook) {
	m.hookCount++
}

func TestNewExecutorWithFixedNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processor.ShardProcessor](ctrl)
	params := createMockParams(ctrl, mockShardProcessorFactory, "shard-distributor-canary")

	result, err := NewExecutorWithFixedNamespace(params)

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
}

func TestNewExecutorWithEphemeralNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processorephemeral.ShardProcessor](ctrl)
	params := createMockParamsEphemeral(ctrl, mockShardProcessorFactory, "shard-distributor-canary-ephemeral")

	result, err := NewExecutorWithEphemeralNamespace(params)

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
}

func TestNewExecutorLocalPassthroughNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processor.ShardProcessor](ctrl)
	params := createMockParams(ctrl, mockShardProcessorFactory, "test-local-passthrough")

	result, err := NewExecutorLocalPassthroughNamespace(params)

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
}

func TestNewExecutorLocalPassthroughShadowNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processorephemeral.ShardProcessor](ctrl)
	params := createMockParamsEphemeral(ctrl, mockShardProcessorFactory, "test-local-passthrough-shadow")

	result, err := NewExecutorLocalPassthroughShadowNamespace(params)

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
}

func TestNewExecutorDistrebutedPassthroughNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processor.ShardProcessor](ctrl)
	params := createMockParams(ctrl, mockShardProcessorFactory, "test-distributed-passthrough")

	result, err := NewExecutorDistrebutedPassthroughNamespace(params)

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
}

func TestNewExecutorExternalAssignmentNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processorephemeral.ShardProcessor](ctrl)
	mockShardDistributorClient := sharddistributor.NewMockClient(ctrl)

	params := createMockParamsEphemeral(ctrl, mockShardProcessorFactory, "test-external-assignment")

	result, assigner, err := NewExecutorExternalAssignmentNamespace(params, mockShardDistributorClient)

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
	require.NotNil(t, assigner)
}

func TestNewExecutorWithFixedNamespace_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processor.ShardProcessor](ctrl)

	// Create params with invalid config (missing namespace)
	params := executorclient.Params[*processor.ShardProcessor]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: mockShardProcessorFactory,
		Config: executorclient.Config{
			Namespaces: []executorclient.NamespaceConfig{},
		},
		TimeSource: clock.NewMockedTimeSource(),
	}

	_, err := NewExecutorWithFixedNamespace(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one namespace must be configured")
}

func TestNewExecutorWithEphemeralNamespace_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processorephemeral.ShardProcessor](ctrl)

	// Create params with invalid config (missing namespace)
	params := executorclient.Params[*processorephemeral.ShardProcessor]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: mockShardProcessorFactory,
		Config: executorclient.Config{
			Namespaces: []executorclient.NamespaceConfig{},
		},
		TimeSource: clock.NewMockedTimeSource(),
	}

	_, err := NewExecutorWithEphemeralNamespace(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one namespace must be configured")
}

func TestNewExecutorExternalAssignmentNamespace_ShardAssignerConfiguration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processorephemeral.ShardProcessor](ctrl)
	mockShardDistributorClient := sharddistributor.NewMockClient(ctrl)

	params := createMockParamsEphemeral(ctrl, mockShardProcessorFactory, "test-external-assignment")

	result, assigner, err := NewExecutorExternalAssignmentNamespace(params, mockShardDistributorClient)

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
	require.NotNil(t, assigner)

	// Verify that the assigner is properly configured and can start/stop
	assigner.Start()
	defer assigner.Stop()
}

func TestExecutorResult_OutputStruct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := executorclient.NewMockExecutor[*processor.ShardProcessor](ctrl)

	result := ExecutorResult{
		Executor: mockExecutor,
	}

	assert.NotNil(t, result.Executor)
	assert.Equal(t, mockExecutor, result.Executor)
}

func TestExecutorEphemeralResult_OutputStruct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := executorclient.NewMockExecutor[*processorephemeral.ShardProcessor](ctrl)

	result := ExecutorEphemeralResult{
		Executor: mockExecutor,
	}

	assert.NotNil(t, result.Executor)
	assert.Equal(t, mockExecutor, result.Executor)
}

func TestNewExecutorsModule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock executors
	mockFixedExecutor1 := executorclient.NewMockExecutor[*processor.ShardProcessor](ctrl)
	mockFixedExecutor2 := executorclient.NewMockExecutor[*processor.ShardProcessor](ctrl)
	mockEphemeralExecutor := executorclient.NewMockExecutor[*processorephemeral.ShardProcessor](ctrl)

	// Create a mock lifecycle
	mockLifecycle := &mockLifecycle{}

	// Create params with the mock executors
	params := ExecutorsParams{
		Lc: mockLifecycle,
		ExecutorsFixed: []executorclient.Executor[*processor.ShardProcessor]{
			mockFixedExecutor1,
			mockFixedExecutor2,
		},
		Executorsephemeral: []executorclient.Executor[*processorephemeral.ShardProcessor]{
			mockEphemeralExecutor,
		},
	}

	// Call NewExecutorsModule - it should not panic or error
	// The function doesn't return anything, so we just verify it executes successfully
	require.NotPanics(t, func() {
		NewExecutorsModule(params)
	})

	// Verify that lifecycle hooks were registered for all executors
	assert.Equal(t, 3, mockLifecycle.hookCount)
}

func TestNewExecutorsModule_NoExecutors(t *testing.T) {
	// Create a mock lifecycle
	mockLifecycle := &mockLifecycle{}

	// Test with empty executors lists
	params := ExecutorsParams{
		Lc:                 mockLifecycle,
		ExecutorsFixed:     []executorclient.Executor[*processor.ShardProcessor]{},
		Executorsephemeral: []executorclient.Executor[*processorephemeral.ShardProcessor]{},
	}

	// Should not panic with empty lists
	require.NotPanics(t, func() {
		NewExecutorsModule(params)
	})

	// Verify that no lifecycle hooks were registered
	assert.Equal(t, 0, mockLifecycle.hookCount)
}

func TestNewExecutorWithFixedNamespace_WrongNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processor.ShardProcessor](ctrl)

	// Create params with a namespace that doesn't match what we're requesting
	params := executorclient.Params[*processor.ShardProcessor]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: mockShardProcessorFactory,
		Config: executorclient.Config{
			Namespaces: []executorclient.NamespaceConfig{
				{
					Namespace:         "wrong-namespace",
					HeartBeatInterval: 5 * time.Second,
					MigrationMode:     "onboarded",
				},
			},
		},
		TimeSource: clock.NewMockedTimeSource(),
	}

	_, err := NewExecutorWithFixedNamespace(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace shard-distributor-canary not found in config")
}

func TestNewExecutorWithEphemeralNamespace_WrongNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processorephemeral.ShardProcessor](ctrl)

	// Create params with a namespace that doesn't match what we're requesting
	params := executorclient.Params[*processorephemeral.ShardProcessor]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: mockShardProcessorFactory,
		Config: executorclient.Config{
			Namespaces: []executorclient.NamespaceConfig{
				{
					Namespace:         "wrong-namespace",
					HeartBeatInterval: 5 * time.Second,
					MigrationMode:     "onboarded",
				},
			},
		},
		TimeSource: clock.NewMockedTimeSource(),
	}

	_, err := NewExecutorWithEphemeralNamespace(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace shard-distributor-canary-ephemeral not found in config")
}

func TestNewExecutorLocalPassthroughNamespace_WrongNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processor.ShardProcessor](ctrl)

	// Create params with a namespace that doesn't match what we're requesting
	params := executorclient.Params[*processor.ShardProcessor]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: mockShardProcessorFactory,
		Config: executorclient.Config{
			Namespaces: []executorclient.NamespaceConfig{
				{
					Namespace:         "wrong-namespace",
					HeartBeatInterval: 5 * time.Second,
					MigrationMode:     "onboarded",
				},
			},
		},
		TimeSource: clock.NewMockedTimeSource(),
	}

	_, err := NewExecutorLocalPassthroughNamespace(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get config for namespace test-local-passthrough")
}

// Helper functions to create mock parameters

func createMockParams(
	ctrl *gomock.Controller,
	factory executorclient.ShardProcessorFactory[*processor.ShardProcessor],
	namespace string,
) executorclient.Params[*processor.ShardProcessor] {
	return executorclient.Params[*processor.ShardProcessor]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: factory,
		Config: executorclient.Config{
			Namespaces: []executorclient.NamespaceConfig{
				{
					Namespace:         namespace,
					HeartBeatInterval: 5 * time.Second,
					MigrationMode:     "onboarded",
				},
			},
		},
		TimeSource: clock.NewMockedTimeSource(),
	}
}

func createMockParamsEphemeral(
	ctrl *gomock.Controller,
	factory executorclient.ShardProcessorFactory[*processorephemeral.ShardProcessor],
	namespace string,
) executorclient.Params[*processorephemeral.ShardProcessor] {
	return executorclient.Params[*processorephemeral.ShardProcessor]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: factory,
		Config: executorclient.Config{
			Namespaces: []executorclient.NamespaceConfig{
				{
					Namespace:         namespace,
					HeartBeatInterval: 5 * time.Second,
					MigrationMode:     "onboarded",
				},
			},
		},
		TimeSource: clock.NewMockedTimeSource(),
	}
}
