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

package cadence

import (
	"context"
	"fmt"

	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock/clockfx"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicconfigfx"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/logfx"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/metricsfx"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/rpc/rpcfx"
	"github.com/uber/cadence/common/service"
	shardDistributorCfg "github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/sharddistributorfx"
	"github.com/uber/cadence/service/sharddistributor/store/etcd"
	"github.com/uber/cadence/tools/cassandra"
	"github.com/uber/cadence/tools/sql"
)

var _commonModule = fx.Options(
	config.Module,
	dynamicconfigfx.Module,
	logfx.Module,
	metricsfx.Module,
	clockfx.Module)

// Module provides a cadence server initialization with root components.
// AppParams allows to provide optional/overrides for implementation specific dependencies.
func Module(serviceName string) fx.Option {
	if serviceName == service.ShortName(service.ShardDistributor) {
		return fx.Options(
			fx.Supply(serviceContext{
				Name:     serviceName,
				FullName: service.FullName(serviceName),
			}),
			fx.Provide(func(cfg config.Config) shardDistributorCfg.ShardDistribution {
				return cfg.ShardDistribution
			}),
			fx.Provide(func(dynamicCollection *dynamicconfig.Collection) shardDistributorCfg.MigrationConfig {
				return shardDistributorCfg.NewMigrationConfig(dynamicCollection)
			}),
			// Decorate both logger so all components use proper service name.
			fx.Decorate(func(z *zap.Logger, l log.Logger) (*zap.Logger, log.Logger) {
				return z.With(zap.String("service", service.ShardDistributor)), l.WithTags(tag.Service(service.ShardDistributor))
			}),

			etcd.Module,

			rpcfx.Module,
			sharddistributorfx.Module)
	}
	return fx.Options(
		fx.Supply(serviceContext{
			Name:     serviceName,
			FullName: service.FullName(serviceName),
		}),
		fx.Provide(NewApp),
		// empty invoke so fx won't drop the application from the dependencies.
		fx.Invoke(func(a *App) {}),
	)
}

type AppParams struct {
	fx.In

	Service       string `name:"service"`
	AppContext    config.Context
	Config        config.Config
	Logger        log.Logger
	LifeCycle     fx.Lifecycle
	DynamicConfig dynamicconfig.Client
	Scope         tally.Scope
	MetricsClient metrics.Client
}

// NewApp created a new Application from pre initalized config and logger.
func NewApp(params AppParams) *App {
	app := &App{
		cfg:           params.Config,
		logger:        params.Logger,
		service:       params.Service,
		dynamicConfig: params.DynamicConfig,
		scope:         params.Scope,
		metricsClient: params.MetricsClient,
	}

	params.LifeCycle.Append(fx.StartHook(app.verifySchema))
	params.LifeCycle.Append(fx.StartStopHook(app.Start, app.Stop))
	return app
}

// App is a fx application that registers itself into fx.Lifecycle and runs.
// It is done implicitly, since it provides methods Start and Stop which are picked up by fx.
type App struct {
	cfg           config.Config
	rootDir       string
	logger        log.Logger
	dynamicConfig dynamicconfig.Client
	scope         tally.Scope
	metricsClient metrics.Client

	daemon  common.Daemon
	service string
}

func (a *App) Start(_ context.Context) error {
	a.daemon = newServer(a.service, a.cfg, a.logger, a.dynamicConfig, a.scope, a.metricsClient)
	a.daemon.Start()
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	a.daemon.Stop()
	return nil
}

func (a *App) verifySchema(ctx context.Context) error {
	// cassandra schema version validation
	if err := cassandra.VerifyCompatibleVersion(a.cfg.Persistence, gocql.Quorum); err != nil {
		return fmt.Errorf("cassandra schema version compatibility check failed: %w", err)
	}
	// sql schema version validation
	if err := sql.VerifyCompatibleVersion(a.cfg.Persistence); err != nil {
		return fmt.Errorf("sql schema version compatibility check failed: %w", err)
	}
	return nil
}

type serviceContext struct {
	fx.Out

	Name     string `name:"service"`
	FullName string `name:"service-full-name"`
}
