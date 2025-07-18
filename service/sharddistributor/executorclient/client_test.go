package executorclient

import (
	"testing"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/yarpctest"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

func TestModule(t *testing.T) {
	// Create mocks
	ctrl := gomock.NewController(t)
	mockLogger := log.NewNoop()

	dcMock := dynamicconfig.NewMockClient(ctrl)
	// We call this when creating the yarpc handler for the executor interface
	dcMock.EXPECT().GetFloatValue(dynamicproperties.ShardDistributorExecutorErrorInjectionRate, gomock.Any()).Return(0.2, nil)
	mockDCCollection := dynamicconfig.NewCollection(dcMock, mockLogger)

	mockMetricsClient := metrics.NewNoopMetricsClient()
	mockShardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)

	// Create simple dispatcher
	outbound := grpc.NewTransport().NewOutbound(yarpctest.NewFakePeerList())

	testDispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: "test-executor",
		Outbounds: yarpc.Outbounds{service.ShardDistributor: {
			Unary: outbound,
		}},
	})

	// Example config
	config := Config{
		Namespace:         "test-namespace",
		HeartBeatInterval: 5 * time.Second,
	}

	// Create a test app with the library, check that it starts and stops
	fxtest.New(t,
		fx.Supply(
			mockDCCollection,
			fx.Annotate(mockMetricsClient, fx.As(new(metrics.Client))),
			fx.Annotate(mockLogger, fx.As(new(log.Logger))),
			fx.Annotate(mockShardProcessorFactory, fx.As(new(ShardProcessorFactory[*MockShardProcessor]))),
			fx.Annotate(clock.NewMockedTimeSource(), fx.As(new(clock.TimeSource))),
			config,
		),
		Module[*MockShardProcessor](),
		fx.Provide(func() *yarpc.Dispatcher {
			return testDispatcher
		}),
	).RequireStart().RequireStop()
}
