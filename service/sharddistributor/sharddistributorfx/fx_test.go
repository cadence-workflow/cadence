package sharddistributorfx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/store"
)

func TestFxServiceStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	testDispatcher := yarpc.NewDispatcher(yarpc.Config{Name: "test"})
	ctrl := gomock.NewController(t)
	app := fxtest.New(t,
		testlogger.Module(t),
		fx.Provide(
			func() metrics.Client { return metrics.NewNoopMetricsClient() },
			func() rpc.Factory {
				factory := rpc.NewMockFactory(ctrl)
				factory.EXPECT().GetDispatcher().Return(testDispatcher)
				return factory
			},
			func() *dynamicconfig.Collection {
				return dynamicconfig.NewNopCollection()
			},
			fx.Annotated{Target: func() string { return "testHost" }, Name: "hostname"},
			func() store.Elector {
				return store.NewMockElector(ctrl)
			},
			func() map[string]membership.SingleProvider { return make(map[string]membership.SingleProvider) },
			func() config.LeaderElection {
				return config.LeaderElection{
					Enabled: false,
				}
			},
			func() clock.TimeSource {
				return clock.NewMockedTimeSource()
			},
		),
		Module)
	app.RequireStart().RequireStop()
	// API should be registered inside dispatcher.
	assert.True(t, len(testDispatcher.Introspect().Procedures) > 1)
}
