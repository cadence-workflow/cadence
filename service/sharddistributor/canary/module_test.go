package canary

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport/transporttest"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/yarpctest"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
)

func TestModule(t *testing.T) {
	// Create mocks
	ctrl := gomock.NewController(t)
	mockClientConfig := transporttest.NewMockClientConfig(ctrl)
	transport := grpc.NewTransport()
	outbound := transport.NewOutbound(yarpctest.NewFakePeerList())

	mockClientConfig.EXPECT().Caller().Return("test-executor").Times(2)
	mockClientConfig.EXPECT().Service().Return("shard-distributor").Times(2)
	mockClientConfig.EXPECT().GetUnaryOutbound().Return(outbound).Times(2)

	mockClientConfigProvider := transporttest.NewMockClientConfigProvider(ctrl)
	mockClientConfigProvider.EXPECT().ClientConfig("cadence-shard-distributor").Return(mockClientConfig).Times(2)

	config := clientcommon.Config{
		Namespaces: []clientcommon.NamespaceConfig{
			{Namespace: "shard-distributor-canary", HeartBeatInterval: 5 * time.Second, MigrationMode: "onboarded"},
			{Namespace: "shard-distributor-canary-ephemeral", HeartBeatInterval: 5 * time.Second, MigrationMode: "onboarded"},
			{Namespace: "test-local-passthrough", HeartBeatInterval: 1 * time.Second, MigrationMode: "local_pass"},
			{Namespace: "test-local-passthrough-shadow", HeartBeatInterval: 1 * time.Second, MigrationMode: "local_pass_shadow"},
			{Namespace: "test-distributed-passthrough", HeartBeatInterval: 1 * time.Second, MigrationMode: "distributed_pass"},
			{Namespace: "test-external-assignment", HeartBeatInterval: 1 * time.Second, MigrationMode: "distributed_pass"},
		},
	}

	// Create a mock dispatcher with the required outbound
	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: "test-canary",
		Outbounds: yarpc.Outbounds{
			"shard-distributor-canary": {
				Unary: outbound,
			},
		},
	})

	// Create a test app with the library, check that it starts and stops
	fxtest.New(t,
		fx.Supply(
			fx.Annotate(tally.NoopScope, fx.As(new(tally.Scope))),
			fx.Annotate(clock.NewMockedTimeSource(), fx.As(new(clock.TimeSource))),
			fx.Annotate(log.NewNoop(), fx.As(new(log.Logger))),
			fx.Annotate(mockClientConfigProvider, fx.As(new(yarpc.ClientConfig))),
			fx.Annotate(transport, fx.As(new(peer.Transport))),
			zaptest.NewLogger(t),
			config,
			dispatcher,
		),
		Module(NamespacesNames{FixedNamespace: "shard-distributor-canary", EphemeralNamespace: "shard-distributor-canary-ephemeral", ExternalAssignmentNamespace: "test-external-assignment", SharddistributorServiceName: "cadence-shard-distributor"}),
	).RequireStart().RequireStop()
}
