// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ringpopfx

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber/tchannel-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	ringpopproviderconfig "github.com/uber/cadence/common/peerprovider/ringpopprovider/config"
	"github.com/uber/cadence/common/rpc"
)

func TestFxApp(t *testing.T) {
	app := fxtest.New(t,
		fx.Provide(
			func() testSetupParams {
				ctrl := gomock.NewController(t)
				factory := rpc.NewMockFactory(ctrl)
				tch, err := tchannel.NewChannel("test-ringpop", nil)
				require.NoError(t, err)
				factory.EXPECT().GetTChannel().Return(tch)

				return testSetupParams{
					Service:    "test",
					Logger:     testlogger.New(t),
					RPCFactory: factory,
					Config: config.Config{
						Ringpop: ringpopproviderconfig.Config{
							Name:           "test-ringpop",
							BootstrapMode:  ringpopproviderconfig.BootstrapModeHosts,
							BootstrapHosts: []string{"127.0.0.1:7933", "127.0.0.1:7934", "127.0.0.1:7935"},
						},
					},
				}
			}),
		Module, fx.Invoke(func(provider membership.PeerProvider) {}),
	)
	app.RequireStart().RequireStop()
}

type testSetupParams struct {
	fx.Out

	Service       string `name:"service-full-name"`
	Config        config.Config
	ServiceConfig config.Service
	Logger        log.Logger
	RPCFactory    rpc.Factory
}
