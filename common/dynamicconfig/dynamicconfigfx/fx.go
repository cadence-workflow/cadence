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

	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/filebased"
	"github.com/uber/cadence/common/dynamicconfig/provider"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

// Module provides fx options for dynamic config initialization
var Module = fx.Options(fx.Provide(New))

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
	}

	p.Lifecycle.Append(fx.StopHook(func() {
		close(stopped)
	}))

	var res dynamicconfig.Client

	var err error
	if p.Cfg.DynamicConfig.Client == "" {
		p.Logger.Warn("falling back to legacy file based dynamicClientConfig")
		res, err = filebased.NewClient(&p.Cfg.DynamicConfigClient, p.Logger, stopped)
	} else {
		container := &provider.BootstrapContainer{
			Logger:            p.Logger,
			MetricsClient:     p.MetricsClient,
			PersistenceConfig: &p.Cfg.Persistence,
			RootDir:           p.RootDir,
			Stopped:           stopped,
		}
		clientProvider := provider.NewClientProvider(p.Cfg.DynamicConfig.Provider, container)
		p.Logger.Info("initialising dynamic config client", tag.Value(p.Cfg.DynamicConfig.Client))
		res, err = clientProvider.GetClient(p.Cfg.DynamicConfig.Client)
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
