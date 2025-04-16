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
	"context"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
)

// Module provides fx options for dynamic config initialization
var Module = fx.Options(fx.Provide(New))

// New creates dynamicconfig.Client from the configuration
func New(cfg config.Config, logger log.Logger, lifecycle fx.Lifecycle) dynamicconfig.Client {
	stopped := make(chan struct{})

	lifecycle.Append(fx.Hook{OnStop: func(_ context.Context) error {
		close(stopped)
		return nil
	}})

	var res dynamicconfig.Client

	var err error
	if cfg.DynamicConfig.Client == "" {
		logger.Warn("falling back to legacy file based dynamicClientConfig")
		res, err = dynamicconfig.NewFileBasedClient(&cfg.DynamicConfigClient, logger, stopped)
	} else {
		switch cfg.DynamicConfig.Client {
		case dynamicconfig.ConfigStoreClient:
			logger.Info("initialising ConfigStore dynamic config client")
			res, err = configstore.NewConfigStoreClient(
				&cfg.DynamicConfig.ConfigStore,
				&cfg.Persistence,
				logger,
				persistence.DynamicConfig,
			)
		case dynamicconfig.FileBasedClient:
			logger.Info("initialising File Based dynamic config client")
			res, err = dynamicconfig.NewFileBasedClient(&cfg.DynamicConfig.FileBased, logger, stopped)
		}
	}

	if res == nil {
		logger.Info("initialising NOP dynamic config client")
		res = dynamicconfig.NewNopClient()
	} else if err != nil {
		logger.Error("creating dynamic config client failed, using no-op config client instead", tag.Error(err))
		res = dynamicconfig.NewNopClient()
	}

	return res
}
