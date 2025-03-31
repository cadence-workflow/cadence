package cadence

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
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

	RootDir    string   `name:"root-dir"`
	Services   []string `name:"services"`
	AppContext config.Context
	Config     config.Config
	Logger     log.Logger
}

// NewApp created a new Application from pre initalized config and logger.
func NewApp(params AppParams) *App {
	app := &App{
		cfg:      params.Config,
		rootDir:  params.RootDir,
		logger:   params.Logger,
		services: params.Services,
	}
	return app
}

// App is a fx application that registers itself into fx.Lifecycle and runs.
// It is done implicitly, since it provides methods Start and Stop which are picked up by fx.
type App struct {
	cfg     config.Config
	rootDir string
	logger  log.Logger

	daemons  []common.Daemon
	services []string
}

func (a *App) Start(_ context.Context) error {
	if a.cfg.DynamicConfig.Client == "" {
		a.cfg.DynamicConfigClient.Filepath = constructPathIfNeed(a.rootDir, a.cfg.DynamicConfigClient.Filepath)
	} else {
		a.cfg.DynamicConfig.FileBased.Filepath = constructPathIfNeed(a.rootDir, a.cfg.DynamicConfig.FileBased.Filepath)
	}

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
		server := newServer(svc, a.cfg, a.logger)
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
