package ringpopfx

import (
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/rpc"
)

func TestFxApp(t *testing.T) {
	app := fxtest.New(t,
		fx.Provide(
			func() testSetupParams {
				ctrl := gomock.NewController(t)
				factory := rpc.NewMockFactory(ctrl)
				factory.EXPECT().GetTChannel().Return(nil)

				return testSetupParams{
					Service:    "test",
					Logger:     testlogger.New(t),
					RPCFactory: factory,
				}
			}),
		Module,
	)
	app.RequireStart().RequireStop()
}

type testSetupParams struct {
	fx.Out

	Service       string `name:"service"`
	Config        config.Config
	ServiceConfig config.Service
	Logger        log.Logger
	RPCFactory    rpc.Factory
}
