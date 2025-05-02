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

package sharddistributorfx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
)

func TestFxServiceStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	testDispatcher := yarpc.NewDispatcher(yarpc.Config{Name: "test"})
	ctrl := gomock.NewController(t)
	app := fxtest.New(t,
		testlogger.Module(t),
		fx.Supply(
			config.Config{},
			&dynamicconfig.Collection{},
			hostResult{},
			make(map[string]membership.SingleProvider),
		),
		fx.Provide(
			func() metrics.Client { return metrics.NewNoopMetricsClient() },
			func() rpc.Factory {
				factory := rpc.NewMockFactory(ctrl)
				factory.EXPECT().GetDispatcher().Return(testDispatcher)
				return factory
			},
			func() clock.TimeSource { return clock.NewMockedTimeSource() },
			// Stub etcd client to avoid spinning up etcd.
			func() *clientv3.Client {
				return &clientv3.Client{}
			},
		),
		Module)
	app.RequireStart().RequireStop()
	// API should be registered inside dispatcher.
	assert.True(t, len(testDispatcher.Introspect().Procedures) > 1)
}

type hostResult struct {
	fx.Out

	HostName string `name:"hostname"`
}
