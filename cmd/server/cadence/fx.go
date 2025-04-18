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

	"go.uber.org/fx"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/tools/cassandra"
	"github.com/uber/cadence/tools/sql"
)

// Module provides a cadence server initialization with root components.
// AppParams allows to provide optional/overrides for implementation specific dependencies.
var Module = fx.Options(
	fx.Provide(NewApp),
	// empty invoke so fx won't drop the application from the dependencies.
	fx.Invoke(func(a *App) {}),
)

type AppParams struct {
	fx.In

	Services      []string `name:"services"`
	AppContext    config.Context
	Config        config.Config
	Logger        log.Logger
	LifeCycle     fx.Lifecycle
	DynamicConfig dynamicconfig.Client
}

// NewApp created a new Application from pre initalized config and logger.
func NewApp(params AppParams) *App {
	app := &App{
		cfg:           params.Config,
		logger:        params.Logger,
		services:      params.Services,
		dynamicConfig: params.DynamicConfig,
	}
	params.LifeCycle.Append(fx.Hook{OnStart: app.Start, OnStop: app.Stop})
	return app
}

// App is a fx application that registers itself into fx.Lifecycle and runs.
// It is done implicitly, since it provides methods Start and Stop which are picked up by fx.
type App struct {
	cfg           config.Config
	rootDir       string
	logger        log.Logger
	dynamicConfig dynamicconfig.Client

	daemons  []common.Daemon
	services []string
}

func (a *App) Start(_ context.Context) error {
	if err := a.cfg.ValidateAndFillDefaults(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	// cassandra schema version validation
	if err := cassandra.VerifyCompatibleVersion(a.cfg.Persistence, gocql.Quorum); err != nil {
		return fmt.Errorf("cassandra schema version compatibility check failed: %w", err)
	}
	// sql schema version validation
	if err := sql.VerifyCompatibleVersion(a.cfg.Persistence); err != nil {
		return fmt.Errorf("sql schema version compatibility check failed: %w", err)
	}

	var daemons []common.Daemon
	for _, svc := range a.services {
		server := newServer(svc, a.cfg, a.logger, a.dynamicConfig)
		daemons = append(daemons, server)
		server.Start()
	}

	return nil
}

func (a *App) Stop(ctx context.Context) error {
	for _, daemon := range a.daemons {
		daemon.Stop()
	}
	return nil
}
