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
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
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

func TestNew_Client(t *testing.T) {
	dir := t.TempDir()
	validFile := filepath.Join(dir, "valid.yaml")
	require.NoError(t, os.WriteFile(validFile, []byte("testGetBoolPropertyKey:\n- value: true\n"), 0644))
	brokenFile := filepath.Join(dir, "broken.yaml")
	require.NoError(t, os.WriteFile(brokenFile, []byte("testGetBoolPropertyKey:\n- value: true\n  constraints: {\n"), 0644))

	tests := []struct {
		name       string
		cfg        config.Config
		wantNop    bool
		wantErrMsg string
	}{
		{
			name:    "not configured falls back to NOP",
			wantNop: true,
		},
		{
			name:    "explicit NOP",
			cfg:     config.Config{DynamicConfig: config.DynamicConfig{Client: dynamicconfig.NopClient}},
			wantNop: true,
		},
		{
			name: "file based",
			cfg: config.Config{DynamicConfig: config.DynamicConfig{
				Client:    dynamicconfig.FileBasedClient,
				FileBased: dynamicconfig.FileBasedClientConfig{Filepath: validFile},
			}},
		},
		{
			name: "legacy file based",
			cfg:  config.Config{DynamicConfigClient: dynamicconfig.FileBasedClientConfig{Filepath: validFile}},
		},
		{
			name: "file based with broken yaml fails",
			cfg: config.Config{DynamicConfig: config.DynamicConfig{
				Client:    dynamicconfig.FileBasedClient,
				FileBased: dynamicconfig.FileBasedClientConfig{Filepath: brokenFile},
			}},
			wantErrMsg: "failed to decode dynamic config file " + brokenFile,
		},
		{
			name: "file based with missing file fails",
			cfg: config.Config{DynamicConfig: config.DynamicConfig{
				Client:    dynamicconfig.FileBasedClient,
				FileBased: dynamicconfig.FileBasedClientConfig{Filepath: filepath.Join(dir, "missing.yaml")},
			}},
			wantErrMsg: "missing.yaml",
		},
		{
			name:       "file based without filepath fails",
			cfg:        config.Config{DynamicConfig: config.DynamicConfig{Client: dynamicconfig.FileBasedClient}},
			wantErrMsg: "filepath is not set",
		},
		{
			name:       "legacy file based with broken yaml fails",
			cfg:        config.Config{DynamicConfigClient: dynamicconfig.FileBasedClientConfig{Filepath: brokenFile}},
			wantErrMsg: "legacy file based dynamic config client",
		},
		{
			name:       "config store without persistence fails",
			cfg:        config.Config{DynamicConfig: config.DynamicConfig{Client: dynamicconfig.ConfigStoreClient}},
			wantErrMsg: `initialising "configstore" dynamic config client`,
		},
		{
			name:       "unknown client fails",
			cfg:        config.Config{DynamicConfig: config.DynamicConfig{Client: "filebsaed"}},
			wantErrMsg: `unknown dynamic config client "filebsaed"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.ClusterGroupMetadata = &config.ClusterGroupMetadata{}
			lc := fxtest.NewLifecycle(t)
			res, err := New(Params{
				Cfg:           tc.cfg,
				Logger:        testlogger.New(t),
				MetricsClient: metrics.NewNoopMetricsClient(),
				RootDir:       dir,
				Lifecycle:     lc,
			})
			if tc.wantErrMsg != "" {
				require.ErrorContains(t, err, tc.wantErrMsg)
				return
			}
			require.NoError(t, err)
			lc.RequireStart().RequireStop()
			if tc.wantNop {
				require.IsType(t, dynamicconfig.NewNopClient(), res.Client)
			} else {
				require.NotEqual(t, reflect.TypeOf(dynamicconfig.NewNopClient()), reflect.TypeOf(res.Client))
			}
		})
	}
}

func TestNew_OperationalConfigStore(t *testing.T) {
	tests := []struct {
		name        string
		persistence config.Persistence
		wantErr     bool
	}{
		{
			name:        "no default store falls back to no-op",
			persistence: config.Persistence{},
		},
		{
			name: "default store without SQL or NoSQL falls back to no-op",
			persistence: config.Persistence{
				DefaultStore: "default",
				DataStores:   map[string]config.DataStore{"default": {}},
			},
		},
		{
			name: "store initialization failure is returned",
			persistence: config.Persistence{
				DefaultStore: "default",
				DataStores: map[string]config.DataStore{
					"default": {SQL: &config.SQL{PluginName: "unknown-plugin"}},
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := New(Params{
				Cfg: config.Config{
					ClusterGroupMetadata: &config.ClusterGroupMetadata{},
					Persistence:          tc.persistence,
				},
				Logger:        testlogger.New(t),
				MetricsClient: metrics.NewNoopMetricsClient(),
				RootDir:       "../../../",
				Lifecycle:     fxtest.NewLifecycle(t),
			})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.IsType(t, configstore.NewNopClient(), res.OperationalConfigStore)
			require.NotNil(t, res.OperationalDynamicConfig)
		})
	}
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
	_, err := New(newParams(lc1))
	require.NoError(t, err)
	lc1.RequireStart()
	defer lc1.RequireStop()

	lc2 := fxtest.NewLifecycle(t)
	res2, err := New(newParams(lc2))
	require.NoError(t, err)
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
