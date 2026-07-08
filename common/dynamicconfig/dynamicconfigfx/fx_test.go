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

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureclient"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"

	_ "github.com/uber/cadence/common/dynamicconfig/openfeatureprovider/unleash"
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
// default provider - and the vendor SDK behind it (unleash-client-go keeps a
// process-wide global client) - is process-wide state, not per-fx.App state:
// registering a second provider makes the OpenFeature SDK shut down the "old" one,
// which for a global-singleton client tears down whichever client instance is
// currently live and deadlocks any later flag evaluation. New must construct the
// OpenFeature provider at most once per process, regardless of how many times it's
// called, and every caller must share that one client.
func TestNew_OpenFeatureClient_SingletonAcrossCalls(t *testing.T) {
	var providerCfg openfeatureclient.Config
	err := yaml.Unmarshal([]byte(`
providerName: unleash
provider:
  url: http://127.0.0.1:1/api
  appName: cadence
`), &providerCfg)
	if err != nil {
		t.Fatal(err)
	}

	newParams := func() Params {
		return Params{
			Cfg: config.Config{
				ClusterGroupMetadata: &config.ClusterGroupMetadata{},
				DynamicConfig: config.DynamicConfig{
					Client:      dynamicconfig.OpenFeatureClient,
					OpenFeature: providerCfg,
				},
			},
			Logger:        testlogger.New(t),
			MetricsClient: metrics.NewNoopMetricsClient(),
			RootDir:       "../../../",
			Lifecycle:     fxtest.NewLifecycle(t),
		}
	}

	// Simulate two per-service fx.Apps sharing the same process.
	res1 := New(newParams())
	res2 := New(newParams())

	if res1.Client != res2.Client {
		t.Fatal("expected New to return the same OpenFeature client instance across calls in the same process")
	}

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
