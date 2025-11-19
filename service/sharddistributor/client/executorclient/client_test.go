package executorclient

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	uber_gomock "go.uber.org/mock/gomock"
	"go.uber.org/yarpc/api/transport/transporttest"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/yarpctest"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

func TestModule(t *testing.T) {
	// Create mocks
	ctrl := gomock.NewController(t)
	uberCtrl := uber_gomock.NewController(t)
	mockLogger := log.NewNoop()

	mockShardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](uberCtrl)

	// Create shard distributor yarpc client
	outbound := grpc.NewTransport().NewOutbound(yarpctest.NewFakePeerList())

	mockClientConfig := transporttest.NewMockClientConfig(ctrl)
	mockClientConfig.EXPECT().Caller().Return("test-executor")
	mockClientConfig.EXPECT().Service().Return("shard-distributor")
	mockClientConfig.EXPECT().GetUnaryOutbound().Return(outbound)
	yarpcClient := sharddistributorv1.NewShardDistributorExecutorAPIYARPCClient(mockClientConfig)

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
		fx.Supply(
			fx.Annotate(yarpcClient, fx.As(new(sharddistributorv1.ShardDistributorExecutorAPIYARPCClient))),
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
	// Create mocks
	ctrl := gomock.NewController(t)
	uberCtrl := uber_gomock.NewController(t)
	mockLogger := log.NewNoop()

	mockFactory1 := NewMockShardProcessorFactory[*MockShardProcessor1](uberCtrl)
	mockFactory2 := NewMockShardProcessorFactory[*MockShardProcessor2](uberCtrl)

	// Create shard distributor yarpc client
	outbound := grpc.NewTransport().NewOutbound(yarpctest.NewFakePeerList())

	mockClientConfig := transporttest.NewMockClientConfig(ctrl)
	mockClientConfig.EXPECT().Caller().Return("test-executor").AnyTimes()
	mockClientConfig.EXPECT().Service().Return("shard-distributor").AnyTimes()
	mockClientConfig.EXPECT().GetUnaryOutbound().Return(outbound).AnyTimes()
	yarpcClient := sharddistributorv1.NewShardDistributorExecutorAPIYARPCClient(mockClientConfig)

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
		fx.Supply(
			fx.Annotate(yarpcClient, fx.As(new(sharddistributorv1.ShardDistributorExecutorAPIYARPCClient))),
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
