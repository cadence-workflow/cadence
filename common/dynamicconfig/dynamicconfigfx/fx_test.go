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

package dynamicconfigfx

import (
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	openfeatureclientconfig "github.com/uber/cadence/common/dynamicconfig/openfeatureclient/config"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureprovider"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestModule(t *testing.T) {
	app := fxtest.New(t,
		testlogger.Module(t),
		fx.Provide(
			func() config.Config {
				return config.Config{
					ClusterGroupMetadata: &config.ClusterGroupMetadata{},
				}
			},
			func() fxRoot {
				return fxRoot{
					RootDir: "../../../",
				}
			},
			metrics.NewNoopMetricsClient,
		),
		Module,
		fx.Invoke(func(c dynamicconfig.Client) {}),
	)
	app.RequireStart().RequireStop()
}

type fxRoot struct {
	fx.Out

	RootDir string `name:"root-dir"`
}

// TestNew_OpenFeatureClient_SingletonAcrossCalls guards against a regression where
// Cadence's server binary runs one fx.App per service (frontend/history/matching/
// worker) in the same process, so New is invoked once per service. OpenFeature's
// default provider is process-wide state, not per-fx.App state: registering a
// second provider makes the OpenFeature SDK shut down the "old" one, which for a
// global-singleton client tears down whichever client instance is currently live
// and deadlocks any later flag evaluation. New must construct the OpenFeature
// provider at most once per process, regardless of how many times it's called,
// and every caller must share that one client.
func TestNew_OpenFeatureClient_SingletonAcrossCalls(t *testing.T) {
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
		return memprovider.NewInMemoryProvider(map[string]memprovider.InMemoryFlag{
			"testGetDurationPropertyKey": {
				Key:            "testGetDurationPropertyKey",
				State:          memprovider.Enabled,
				DefaultVariant: "default",
				Variants:       map[string]any{"default": "1s"},
			},
		}), nil
	}))

	newParams := func(lc fx.Lifecycle) Params {
		return Params{
			Cfg: config.Config{
				ClusterGroupMetadata: &config.ClusterGroupMetadata{},
				DynamicConfig: config.DynamicConfig{
					Client: dynamicconfig.OpenFeatureClient,
					OpenFeature: openfeatureclientconfig.Config{
						ProviderName: providerName,
					},
				},
			},
			Logger:        testlogger.New(t),
			MetricsClient: metrics.NewNoopMetricsClient(),
			RootDir:       "../../../",
			Lifecycle:     lc,
		}
	}

	// Simulate two per-service fx.Apps sharing the same process.
	lc1 := fxtest.NewLifecycle(t)
	New(newParams(lc1))
	lc1.RequireStart()
	defer lc1.RequireStop()

	lc2 := fxtest.NewLifecycle(t)
	res2 := New(newParams(lc2))
	lc2.RequireStart()
	defer lc2.RequireStop()

	// Regression check: a flag evaluation must not hang even though the provider
	// was "registered" twice.
	done := make(chan struct{})
	go func() {
		defer close(done)
		res2.Client.GetDurationValue(dynamicproperties.TestGetDurationPropertyKey, nil)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("GetDurationValue did not return within 10s: OpenFeature provider re-registration deadlock regressed")
	}
}
