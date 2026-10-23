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
	"errors"
	"fmt"
	"path/filepath"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	csc "github.com/uber/cadence/common/dynamicconfig/configstore/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureclient"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
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

	Client                   dynamicconfig.Client
	Collection               *dynamicconfig.Collection
	OperationalConfigStore   configstore.Client        `name:"operational-config-store"`
	OperationalDynamicConfig *dynamicconfig.Collection `name:"operational-dynamic-config"`
}

// New creates dynamicconfig.Client from the configuration.
// NOP client is used only when dynamic config is not configured at all or explicitly set to "nop".
// Any failure to initialise an explicitly configured client is returned.
func New(p Params) (Result, error) {
	res, err := newClient(p)
	if err != nil {
		return Result{}, err
	}

	clusterGroupMetadata := p.Cfg.ClusterGroupMetadata
	dc := dynamicconfig.NewCollection(
		res,
		p.Logger,
		dynamicproperties.ClusterNameFilter(clusterGroupMetadata.CurrentClusterName),
	)

	operationalConfigStore, err := createOperationalConfigStore(&p.Cfg.Persistence, dc, p.Logger, p.MetricsClient)
	if err != nil {
		return Result{}, fmt.Errorf("creating operational dynamic config store: %w", err)
	}
	operationalDC := dynamicconfig.NewCollection(
		operationalConfigStore,
		p.Logger,
		dynamicproperties.ClusterNameFilter(clusterGroupMetadata.CurrentClusterName),
	)

	return Result{
		Client:                   res,
		Collection:               dc,
		OperationalConfigStore:   operationalConfigStore,
		OperationalDynamicConfig: operationalDC,
	}, nil
}

func newClient(p Params) (dynamicconfig.Client, error) {
	cfg := p.Cfg.DynamicConfig

	switch cfg.Client {
	case "":
		if p.Cfg.DynamicConfigClient.Filepath == "" {
			p.Logger.Warn("dynamic config is not configured, initialising NOP dynamic config client: all properties use default values")
			return dynamicconfig.NewNopClient(), nil
		}
		p.Logger.Warn("falling back to legacy file based dynamicConfigClient")
		client, err := newFileBasedClient(p, p.Cfg.DynamicConfigClient)
		if err != nil {
			return nil, fmt.Errorf("initialising legacy file based dynamic config client (dynamicConfigClient): %w", err)
		}
		return client, nil
	case dynamicconfig.NopClient:
		p.Logger.Warn("initialising NOP dynamic config client: all properties use default values")
		return dynamicconfig.NewNopClient(), nil
	case dynamicconfig.FileBasedClient:
		p.Logger.Info("initialising File Based dynamic config client")
		client, err := newFileBasedClient(p, cfg.FileBased)
		if err != nil {
			return nil, fmt.Errorf("initialising %q dynamic config client (dynamicconfig.filebased): %w", cfg.Client, err)
		}
		return client, nil
	case dynamicconfig.ConfigStoreClient:
		p.Logger.Info("initialising ConfigStore dynamic config client")
		client, err := configstore.NewConfigStoreClient(
			&cfg.ConfigStore,
			&p.Cfg.Persistence,
			p.Logger,
			p.MetricsClient,
			persistence.DynamicConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("initialising %q dynamic config client (dynamicconfig.configstore): %w", cfg.Client, err)
		}
		return client, nil
	case dynamicconfig.OpenFeatureClient:
		p.Logger.Info("initialising OpenFeature dynamic config client")
		providerName := cfg.OpenFeature.ProviderName
		providerConfig := cfg.OpenFeature.Provider
		p.Lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return openfeatureclient.RegisterProvider(ctx, providerName, providerConfig)
			},
			OnStop: func(ctx context.Context) error {
				openfeatureclient.DeregisterProvider()
				return nil
			},
		})
		return openfeatureclient.NewOpenFeatureClient(p.Logger), nil
	default:
		return nil, fmt.Errorf("unknown dynamic config client %q (dynamicconfig.client), supported: %q, %q, %q, %q",
			cfg.Client,
			dynamicconfig.FileBasedClient,
			dynamicconfig.ConfigStoreClient,
			dynamicconfig.OpenFeatureClient,
			dynamicconfig.NopClient,
		)
	}
}

func newFileBasedClient(p Params, cfg dynamicconfig.FileBasedClientConfig) (dynamicconfig.Client, error) {
	if cfg.Filepath == "" {
		return nil, errors.New("filepath is not set")
	}
	cfg.Filepath = constructPathIfNeed(p.RootDir, cfg.Filepath)

	stopPeriodicUpdate := make(chan struct{})
	client, err := dynamicconfig.NewFileBasedClient(&cfg, p.Logger, stopPeriodicUpdate)
	if err != nil {
		return nil, err
	}
	p.Lifecycle.Append(fx.StopHook(func() {
		close(stopPeriodicUpdate)
	}))
	return client, nil
}

// constructPathIfNeed would append the dir as the root dir
// when the file wasn't absolute path.
func constructPathIfNeed(dir string, file string) string {
	if !filepath.IsAbs(file) {
		return dir + "/" + file
	}
	return file
}

// createOperationalConfigStore returns the primary persistence-backed configstore.Client, or a no-op when persistence
// is not configured. Any other failure is returned: operational config carries rollout knobs that must not silently
// fall back to defaults.
func createOperationalConfigStore(
	persistenceConfig *config.Persistence,
	dc *dynamicconfig.Collection,
	logger log.Logger,
	metricsClient metrics.Client,
) (configstore.Client, error) {
	cscConfig := &csc.ClientConfig{
		PollInterval:        dc.GetDurationProperty(dynamicproperties.OperationalConfigStorePollInterval)(),
		UpdateRetryAttempts: dc.GetIntProperty(dynamicproperties.OperationalConfigStoreUpdateRetryAttempts)(),
		FetchTimeout:        dc.GetDurationProperty(dynamicproperties.OperationalConfigStoreFetchTimeout)(),
		UpdateTimeout:       dc.GetDurationProperty(dynamicproperties.OperationalConfigStoreUpdateTimeout)(),
	}
	client, err := configstore.NewConfigStoreClient(
		cscConfig,
		persistenceConfig,
		logger,
		metricsClient,
		persistence.OperationalDynamicConfig,
	)
	if errors.Is(err, configstore.ErrPersistenceNotConfigured) {
		logger.Warn("not instantiating operational dynamic config store, this feature will not be enabled", tag.Error(err))
		return configstore.NewNopClient(), nil
	}
	if err != nil {
		return nil, err
	}
	return client, nil
}
