package ringpopfx

import (
	"strings"
	"testing"

	"github.com/uber/tchannel-go"
	uconfig "go.uber.org/config"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/rpc"
)

func TestFxApp(t *testing.T) {
	app := fxtest.New(t,
		fx.Provide(
			func() (testSetupParams, error) {
				ctrl := gomock.NewController(t)
				factory := rpc.NewMockFactory(ctrl)
				tch, err := tchannel.NewChannel("test-ringpop", nil)
				if err != nil {
					return testSetupParams{}, err
				}
				factory.EXPECT().GetTChannel().Return(tch)

				yamlConfig := `
ringpop:
  name: test-ringpop
  bootstrapMode: hosts
  bootstrapHosts:
    - 127.0.0.1:7933
    - 127.0.0.1:7934
    - 127.0.0.1:7935
`
				configProvider, err := uconfig.NewYAML(uconfig.RawSource(strings.NewReader(yamlConfig)))
				if err != nil {
					return testSetupParams{}, err
				}

				return testSetupParams{
					Service:        "test",
					Logger:         testlogger.New(t),
					RPCFactory:     factory,
					ConfigProvider: configProvider,
				}, nil
			}),
		Module, fx.Invoke(func(provider membership.PeerProvider) {}),
	)
	app.RequireStart().RequireStop()
}

type testSetupParams struct {
	fx.Out

	Service        string `name:"service-full-name"`
	ConfigProvider uconfig.Provider
	ServiceConfig  config.Service
	Logger         log.Logger
	RPCFactory     rpc.Factory
}
