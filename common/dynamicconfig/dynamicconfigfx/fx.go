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
	"path/filepath"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureclient"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
)

// Module provides fx options for dynamic config initialization
var Module = fx.Options(fx.Provide(New))

// Cadence's server binary runs one fx.App per service (frontend/history/matching/worker)
// in the same OS process, so New is invoked once per service. OpenFeature's default
// provider - and vendor SDKs behind it such as unleash-client-go - keep process-wide
// global state, not per-instance state: registering a second provider makes the
// OpenFeature SDK shut down the "old" one, which for a global-singleton client like
// Unleash's tears down whichever client instance is currently live and deadlocks any
// later flag evaluation. Guard with a process-wide once so the OpenFeature provider is
// only ever constructed and registered a single time, no matter how many services share
// this process; every service's dynamicconfig.Client ends up wrapping that same
// singleton, which is correct since the underlying SDK state is shared anyway.
var (
	openFeatureClientOnce sync.Once
	openFeatureClient     dynamicconfig.Client
	openFeatureClientErr  error
)

// Params required to build a new dynamic config.
type Params struct {
	fx.In

	Cfg           config.Config
	Logger        log.Logger
	MetricsClient metrics.Client
	RootDir       string `name:"root-dir"`

	Lifecycle fx.Lifecycle
}

type Result struct {
	fx.Out

	Client     dynamicconfig.Client
	Collection *dynamicconfig.Collection
}

// New creates dynamicconfig.Client from the configuration
func New(p Params) Result {
	stopped := make(chan struct{})

	if p.Cfg.DynamicConfig.Client == "" {
		p.Cfg.DynamicConfigClient.Filepath = constructPathIfNeed(p.RootDir, p.Cfg.DynamicConfigClient.Filepath)
	} else {
		p.Cfg.DynamicConfig.FileBased.Filepath = constructPathIfNeed(p.RootDir, p.Cfg.DynamicConfig.FileBased.Filepath)
	}

	p.Lifecycle.Append(fx.StopHook(func() {
		close(stopped)
	}))

	var res dynamicconfig.Client

	var err error
	if p.Cfg.DynamicConfig.Client == "" {
		p.Logger.Warn("falling back to legacy file based dynamicClientConfig")
		res, err = dynamicconfig.NewFileBasedClient(&p.Cfg.DynamicConfigClient, p.Logger, stopped)
	} else {
		switch p.Cfg.DynamicConfig.Client {
		case dynamicconfig.ConfigStoreClient:
			p.Logger.Info("initialising ConfigStore dynamic config client")
			res, err = configstore.NewConfigStoreClient(
				&p.Cfg.DynamicConfig.ConfigStore,
				&p.Cfg.Persistence,
				p.Logger,
				p.MetricsClient,
				persistence.DynamicConfig,
			)
		case dynamicconfig.FileBasedClient:
			p.Logger.Info("initialising File Based dynamic config client")
			res, err = dynamicconfig.NewFileBasedClient(&p.Cfg.DynamicConfig.FileBased, p.Logger, stopped)
		case dynamicconfig.OpenFeatureClient:
			openFeatureClientOnce.Do(func() {
				p.Logger.Info("initialising OpenFeature dynamic config client")
				openFeatureClient, openFeatureClientErr = openfeatureclient.NewOpenFeatureClient(
					p.Cfg.DynamicConfig.OpenFeature.ProviderName,
					p.Cfg.DynamicConfig.OpenFeature.Provider,
					p.Logger,
				)
			})
			res, err = openFeatureClient, openFeatureClientErr
		}
	}

	if res == nil {
		p.Logger.Info("initialising NOP dynamic config client")
		res = dynamicconfig.NewNopClient()
	} else if err != nil {
		p.Logger.Error("creating dynamic config client failed, using no-op config client instead", tag.Error(err))
		res = dynamicconfig.NewNopClient()
	}

	clusterGroupMetadata := p.Cfg.ClusterGroupMetadata
	dc := dynamicconfig.NewCollection(
		res,
		p.Logger,
		dynamicproperties.ClusterNameFilter(clusterGroupMetadata.CurrentClusterName),
	)

	return Result{
		Client:     res,
		Collection: dc,
	}
}

// constructPathIfNeed would append the dir as the root dir
// when the file wasn't absolute path.
func constructPathIfNeed(dir string, file string) string {
	if !filepath.IsAbs(file) {
		return dir + "/" + file
	}
	return file
}
