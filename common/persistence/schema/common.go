package schema

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
)

// defaultConnectTimeout is the default amount of time allowed to connect to all DBs.
const defaultConnectTimeout = 10 * time.Second

type Options struct {
	// Required
	// ClusterName is the name of the current cluster
	ClusterName string
	// Config is the persistence config to use for connecting to the DBs. It must be non-nil.
	Config *config.Persistence
	// Optional
	// SetupOptions are parameters passed to SetupDB.Setup. Defaults to empty
	SetupOptions map[string]string
	// Logger is the logger. Defaults to zap production defaults
	Logger log.Logger
	// ConnectTimeout is the amount of time allowed to spend connecting to all DB instances. Defaults to 10s.
	// During this time period, any errors are retried with a short backoff. This is to tolerate the DBs not yet
	// being available.
	ConnectTimeout time.Duration
	// Time is the time source. Defaults to real time.
	Time clock.TimeSource
}

func OptionsFromConfig(cfg config.Config) Options {
	return Options{
		ClusterName: cfg.ClusterGroupMetadata.CurrentClusterName,
		Config:      &cfg.Persistence,
	}
}

// withDefaults validates the required fields of opts and fills in the documented defaults
// for any optional field that wasn't provided.
func withDefaults(opts Options) (Options, error) {
	if opts.ClusterName == "" {
		return opts, errors.New("ClusterName is required")
	}
	if opts.Config == nil {
		return opts, errors.New("Config is required")
	}
	if opts.SetupOptions == nil {
		opts.SetupOptions = map[string]string{}
	}
	if opts.Logger == nil {
		zapLogger, err := zap.NewProduction()
		if err != nil {
			return opts, fmt.Errorf("create logger: %w", err)
		}
		opts.Logger = log.NewLogger(zapLogger)
	}
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = defaultConnectTimeout
	}
	if opts.Time == nil {
		opts.Time = clock.NewRealTimeSource()
	}
	return opts, nil
}

// newPersistenceFactory constructs a persistence factory from the provided config. It doesn't support dynamic config
// or the more complex features of a full server.
func newPersistenceFactory(clusterName string, cfg *config.Persistence, logger log.Logger) persistenceClient.Factory {
	return persistenceClient.NewFactory(
		cfg,
		func() float64 { return 0 },
		clusterName,
		nil, // no metrics client needed for schema updates
		logger,
		persistence.NewDynamicConfiguration(dynamicconfig.NewNopCollection()),
	)
}
