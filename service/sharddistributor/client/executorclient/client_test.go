package executorclient

import (
	"testing"
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

func TestModule(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := log.NewNoop()

	mockShardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)
	shardDistributorExecutorClient := NewMockClient(ctrl)
	shardDistributorExecutorClient.EXPECT().
		Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&types.ExecutorHeartbeatResponse{}, nil).
		AnyTimes()
	// Example config
	config := clientcommon.Config{
		Namespaces: []clientcommon.NamespaceConfig{
			{
				Namespace:         "test-namespace",
				HeartBeatInterval: 5 * time.Second,
			},
		},
	}

	// Create a test app with the library, check that it starts and stops
	fxtest.New(t,
		fx.Provide(func() Client {
			return shardDistributorExecutorClient
		}),
		fx.Supply(
			fx.Annotate(tally.NoopScope, fx.As(new(tally.Scope))),
			fx.Annotate(mockLogger, fx.As(new(log.Logger))),
			fx.Annotate(mockShardProcessorFactory, fx.As(new(ShardProcessorFactory[*MockShardProcessor]))),
			fx.Annotate(clock.NewMockedTimeSource(), fx.As(new(clock.TimeSource))),
			config,
		),
		Module[*MockShardProcessor](),
	).RequireStart().RequireStop()
}

// Create distinct mock processor types for testing multiple namespaces
type MockShardProcessor1 struct {
	*MockShardProcessor
}

type MockShardProcessor2 struct {
	*MockShardProcessor
}

func TestModuleWithNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := log.NewNoop()

	// Create executor yarpc client mock
	mockYARPCClient := NewMockShardDistributorExecutorAPIYARPCClient(ctrl)
	mockYARPCClient.EXPECT().
		Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&sharddistributorv1.HeartbeatResponse{}, nil).
		AnyTimes()

	shardDistributorExecutorClient := NewMockClient(ctrl)
	shardDistributorExecutorClient.EXPECT().
		Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&types.ExecutorHeartbeatResponse{}, nil).
		AnyTimes()

	mockFactory1 := NewMockShardProcessorFactory[*MockShardProcessor1](ctrl)
	mockFactory2 := NewMockShardProcessorFactory[*MockShardProcessor2](ctrl)

	// Multi-namespace config
	config := clientcommon.Config{
		Namespaces: []clientcommon.NamespaceConfig{
			{
				Namespace:         "namespace1",
				HeartBeatInterval: 5 * time.Second,
			},
			{
				Namespace:         "namespace2",
				HeartBeatInterval: 10 * time.Second,
			},
		},
	}

	// Create a test app with two namespace-specific modules using different processor types
	fxtest.New(t,
		fx.Provide(func() Client {
			return shardDistributorExecutorClient
		}),
		fx.Supply(
			fx.Annotate(tally.NoopScope, fx.As(new(tally.Scope))),
			fx.Annotate(mockLogger, fx.As(new(log.Logger))),
			fx.Annotate(clock.NewMockedTimeSource(), fx.As(new(clock.TimeSource))),
			fx.Annotate(mockFactory1, fx.As(new(ShardProcessorFactory[*MockShardProcessor1]))),
			fx.Annotate(mockFactory2, fx.As(new(ShardProcessorFactory[*MockShardProcessor2]))),
			config,
		),
		// Two namespace-specific modules with different processor types
		ModuleWithNamespace[*MockShardProcessor1]("namespace1"),
		ModuleWithNamespace[*MockShardProcessor2]("namespace2"),
	).RequireStart().RequireStop()
}
