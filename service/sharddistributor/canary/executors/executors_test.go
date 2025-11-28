package executors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

// mockLifecycle is a simple mock implementation of fx.Lifecycle for testing
func TestNewExecutorsFixedNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := []struct {
		name        string
		params      executorclient.Params[*processor.ShardProcessor]
		newExecutor func(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error)
	}{
		{
			name:   "TestNewExecutorWithFixedNamespace",
			params: createMockParams[*processor.ShardProcessor](ctrl, "shard-distributor-canary"),
			newExecutor: func(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error) {
				return NewExecutorWithFixedNamespace(params, "shard-distributor-canary")
			}},
		{
			name:        "TestNewExecutorLocalPassthroughNamespace",
			params:      createMockParams[*processor.ShardProcessor](ctrl, LocalPassthroughNamespace),
			newExecutor: NewExecutorLocalPassthroughNamespace,
		},
		{
			name:        "TestNewExecutorDistributedPassthroughNamespace",
			params:      createMockParams[*processor.ShardProcessor](ctrl, DistributedPassthroughNamespace),
			newExecutor: NewExecutorDistributedPassthroughNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.newExecutor(tt.params)

			require.NoError(t, err)
			require.NotNil(t, result.Executor)
		})
	}
}

func TestNewExecutorsEphemeralNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := []struct {
		name        string
		params      executorclient.Params[*processorephemeral.ShardProcessor]
		newExecutor func(params executorclient.Params[*processorephemeral.ShardProcessor]) (ExecutorEphemeralResult, error)
	}{
		{
			name:   "TestNewExecutorWithEphemeralNamespace",
			params: createMockParams[*processorephemeral.ShardProcessor](ctrl, "shard-distributor-canary-ephemeral"),
			newExecutor: func(params executorclient.Params[*processorephemeral.ShardProcessor]) (ExecutorEphemeralResult, error) {
				return NewExecutorWithEphemeralNamespace(params, "shard-distributor-canary-ephemeral")
			}},
		{
			name:        "TestNewExecutorLocalPassthroughShadowNamespace",
			params:      createMockParams[*processorephemeral.ShardProcessor](ctrl, LocalPassthroughShadowNamespace),
			newExecutor: NewExecutorLocalPassthroughShadowNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.newExecutor(tt.params)

			require.NoError(t, err)
			require.NotNil(t, result.Executor)
		})
	}
}

func TestNewExecutorExternalAssignmentNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockShardDistributorClient := sharddistributor.NewMockClient(ctrl)

	params := createMockParams[*processorephemeral.ShardProcessor](ctrl, "test-external-assignment")

	result, assigner, err := NewExecutorExternalAssignmentNamespace(params, mockShardDistributorClient, "test-external-assignment")

	require.NoError(t, err)
	require.NotNil(t, result.Executor)
	require.NotNil(t, assigner)
	// Verify that the assigner is properly configured and can start/stop
	assigner.Start()
	defer assigner.Stop()
}

func TestNewExecutor_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[*processor.ShardProcessor](ctrl)

	tests := []struct {
		name        string
		params      executorclient.Params[*processor.ShardProcessor]
		newExecutor func(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error)
		errorString string
	}{
		{
			name: "No namespaces configured",
			params: executorclient.Params[*processor.ShardProcessor]{
				MetricsScope:          tally.NoopScope,
				Logger:                log.NewNoop(),
				ShardProcessorFactory: mockShardProcessorFactory,
				Config: clientcommon.Config{
					Namespaces: []clientcommon.NamespaceConfig{},
				},
				TimeSource: clock.NewMockedTimeSource(),
			},
			errorString: "at least one namespace must be configured",
		},
		{
			name: "No valid namespace",
			params: executorclient.Params[*processor.ShardProcessor]{
				MetricsScope:          tally.NoopScope,
				Logger:                log.NewNoop(),
				ShardProcessorFactory: mockShardProcessorFactory,
				Config: clientcommon.Config{
					Namespaces: []clientcommon.NamespaceConfig{
						{
							Namespace:         "wrong-namespace",
							HeartBeatInterval: 5 * time.Second,
							MigrationMode:     "onboarded",
						},
					},
				},
				TimeSource: clock.NewMockedTimeSource(),
			},
			errorString: "namespace shard-distributor-canary not found in config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewExecutorWithFixedNamespace(tt.params, "shard-distributor-canary")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorString)
		})
	}
}

// Helper functions to create mock parameters
func createMockParams[SP executorclient.ShardProcessor](
	ctrl *gomock.Controller,
	namespace string,
) executorclient.Params[SP] {
	mockShardProcessorFactory := executorclient.NewMockShardProcessorFactory[SP](ctrl)

	return executorclient.Params[SP]{
		MetricsScope:          tally.NoopScope,
		Logger:                log.NewNoop(),
		ShardProcessorFactory: mockShardProcessorFactory,
		Config: clientcommon.Config{
			Namespaces: []clientcommon.NamespaceConfig{
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
