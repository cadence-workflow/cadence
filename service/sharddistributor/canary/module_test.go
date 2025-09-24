package canary

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport/transporttest"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/yarpctest"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/executorclient"
)

func TestModule(t *testing.T) {
	// Create mocks
	ctrl := gomock.NewController(t)
	mockClientConfig := transporttest.NewMockClientConfig(ctrl)
	outbound := grpc.NewTransport().NewOutbound(yarpctest.NewFakePeerList())

	mockClientConfig.EXPECT().Caller().Return("test-executor").Times(2)
	mockClientConfig.EXPECT().Service().Return("shard-distributor").Times(2)
	mockClientConfig.EXPECT().GetUnaryOutbound().Return(outbound).Times(2)

	mockClientConfigProvider := transporttest.NewMockClientConfigProvider(ctrl)
	mockClientConfigProvider.EXPECT().ClientConfig("cadence-shard-distributor").Return(mockClientConfig).Times(2)

	config := executorclient.Config{
		Namespaces: []executorclient.NamespaceConfig{
			{Namespace: "test-fixed", HeartBeatInterval: 5 * time.Second},
			{Namespace: "test-ephemeral", HeartBeatInterval: 5 * time.Second},
		},
	}

	// Create a test app with the library, check that it starts and stops
	fxtest.New(t,
		fx.Supply(
			fx.Annotate(tally.NoopScope, fx.As(new(tally.Scope))),
			fx.Annotate(clock.NewMockedTimeSource(), fx.As(new(clock.TimeSource))),
			fx.Annotate(log.NewNoop(), fx.As(new(log.Logger))),
			fx.Annotate(mockClientConfigProvider, fx.As(new(yarpc.ClientConfig))),
			zaptest.NewLogger(t),
			config,
		),

		Module("test-fixed", "test-ephemeral", "cadence-shard-distributor"),
	).RequireStart().RequireStop()
}
