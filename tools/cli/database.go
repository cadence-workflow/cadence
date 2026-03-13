// Copyright (c) 2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:generate mockgen -package $GOPACKAGE -destination mock_manager_factory.go -self_package github.com/uber/cadence/tools/cli github.com/uber/cadence/tools/cli ManagerFactory

package cli

import (
	"fmt"
	"net"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/client"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra"
	"github.com/uber/cadence/common/persistence/sql"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/tools/common/flag"
)

var supportedDBs = append(sql.GetRegisteredPluginNames(), "cassandra")

func getDBFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    FlagServiceConfigDir,
			Aliases: []string{"scd"},
			Value:   "config",
			Usage:   "service configuration dir",
			EnvVars: []string{config.EnvKeyConfigDir},
		},
		&cli.StringFlag{
			Name:    FlagServiceEnv,
			Aliases: []string{"se"},
			Usage:   "service env for loading service configuration",
			EnvVars: []string{config.EnvKeyEnvironment},
		},
		&cli.StringFlag{
			Name:    FlagServiceZone,
			Aliases: []string{"sz"},
			Usage:   "service zone for loading service configuration",
			EnvVars: []string{config.EnvKeyAvailabilityZone},
		},
		&cli.StringFlag{
			Name:  FlagDBType,
			Value: "cassandra",
			Usage: fmt.Sprintf("persistence type. Current supported options are %v", supportedDBs),
		},
		&cli.StringFlag{
			Name:  FlagDBAddress,
			Value: "127.0.0.1",
			Usage: "persistence address (right now only cassandra is fully supported)",
		},
		&cli.IntFlag{
			Name:  FlagDBPort,
			Value: 9042,
			Usage: "persistence port",
		},
		&cli.StringFlag{
			Name:  FlagDBRegion,
			Usage: "persistence region",
		},
		&cli.IntFlag{
			Name:  FlagDBShard,
			Usage: "number of db shards in a sharded SQL database",
		},
		&cli.StringFlag{
			Name:  FlagUsername,
			Usage: "persistence username",
		},
		&cli.StringFlag{
			Name:  FlagPassword,
			Usage: "persistence password",
		},
		&cli.StringFlag{
			Name:  FlagKeyspace,
			Value: "cadence",
			Usage: "cassandra keyspace",
		},
		&cli.StringFlag{
			Name:  FlagDatabaseName,
			Value: "cadence",
			Usage: "sql database name",
		},
		&cli.StringFlag{
			Name:  FlagEncodingType,
			Value: "thriftrw",
			Usage: "sql database encoding type",
		},
		&cli.StringSliceFlag{
			Name:  FlagDecodingTypes,
			Value: cli.NewStringSlice("thriftrw"),
			Usage: "sql database decoding types",
		},
		&cli.IntFlag{
			Name:  FlagProtoVersion,
			Value: 4,
			Usage: "cassandra protocol version",
		},
		&cli.BoolFlag{
			Name:  FlagEnableTLS,
			Usage: "enable TLS over cassandra connection",
		},
		&cli.StringFlag{
			Name:  FlagTLSCertPath,
			Usage: "cassandra tls client cert path (tls must be enabled)",
		},
		&cli.StringFlag{
			Name:  FlagTLSKeyPath,
			Usage: "cassandra tls client key path (tls must be enabled)",
		},
		&cli.StringFlag{
			Name:  FlagTLSCaPath,
			Usage: "cassandra tls client ca path (tls must be enabled)",
		},
		&cli.BoolFlag{
			Name:  FlagTLSEnableHostVerification,
			Usage: "cassandra tls verify hostname and server cert (tls must be enabled)",
		},
		&cli.GenericFlag{
			Name:  FlagConnectionAttributes,
			Usage: "a key-value set of sql database connection attributes (must be in key1=value1,key2=value2,...,keyN=valueN format, e.g. cluster=dca or cluster=dca,instance=cadence)",
			Value: &flag.StringMap{},
		},
		&cli.IntFlag{
			Name:  FlagRPS,
			Usage: "target rps of database queries",
			Value: 100,
		},
	}
}

type ManagerFactory interface {
	initializeExecutionManager(c *cli.Context, shardID int) (persistence.ExecutionManager, error)
	initializeHistoryManager(c *cli.Context) (persistence.HistoryManager, error)
	initializeShardManager(c *cli.Context) (persistence.ShardManager, error)
	initializeDomainManager(c *cli.Context) (persistence.DomainManager, error)
	initPersistenceFactory(c *cli.Context) (client.Factory, error)
	initializeInvariantManager(ivs []invariant.Invariant) (invariant.Manager, error)
}

type defaultManagerFactory struct {
	persistenceFactory client.Factory
}

func (f *defaultManagerFactory) initializeExecutionManager(c *cli.Context, shardID int) (persistence.ExecutionManager, error) {
	factory, err := f.getPersistenceFactory(c)
	if err != nil {
		return nil, fmt.Errorf("Failed to get persistence factory: %w", err)
	}
	executionManager, err := factory.NewExecutionManager(shardID)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize history manager %w", err)
	}
	return executionManager, nil
}

func (f *defaultManagerFactory) initializeHistoryManager(c *cli.Context) (persistence.HistoryManager, error) {
	factory, err := f.getPersistenceFactory(c)
	if err != nil {
		return nil, fmt.Errorf("Failed to get persistence factory: %w", err)
	}
	historyManager, err := factory.NewHistoryManager()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize history manager %w", err)
	}
	return historyManager, nil
}

func (f *defaultManagerFactory) initializeShardManager(c *cli.Context) (persistence.ShardManager, error) {
	factory, err := f.getPersistenceFactory(c)
	if err != nil {
		return nil, fmt.Errorf("Failed to get persistence factory: %w", err)
	}
	shardManager, err := factory.NewShardManager()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize shard manager %w", err)
	}
	return shardManager, nil
}

func (f *defaultManagerFactory) initializeDomainManager(c *cli.Context) (persistence.DomainManager, error) {
	factory, err := f.getPersistenceFactory(c)
	if err != nil {
		return nil, fmt.Errorf("Failed to get persistence factory: %w", err)
	}
	domainManager, err := factory.NewDomainManager()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize domain manager: %w", err)
	}
	return domainManager, nil
}

func (f *defaultManagerFactory) getPersistenceFactory(c *cli.Context) (client.Factory, error) {
	var err error
	if f.persistenceFactory == nil {
		f.persistenceFactory, err = getDeps(c).initPersistenceFactory(c)
		if err != nil {
			return f.persistenceFactory, fmt.Errorf("%w", err)
		}
	}
	return f.persistenceFactory, nil
}

func (f *defaultManagerFactory) initPersistenceFactory(c *cli.Context) (client.Factory, error) {
	cfg, err := getDeps(c).ServerConfig(c)

	if err != nil {
		cfg = &config.Config{
			Persistence: config.Persistence{
				DefaultStore: "default",
				DataStores: map[string]config.DataStore{
					"default": {NoSQL: &config.NoSQL{PluginName: cassandra.PluginName}},
				},
			},
			ClusterGroupMetadata: &config.ClusterGroupMetadata{
				CurrentClusterName: "current-cluster",
			},
		}
	}

	// If there are any overrides provided via CLI flags, apply them here
	defaultStore := cfg.Persistence.DataStores[cfg.Persistence.DefaultStore]
	defaultStore, err = overrideDataStore(c, defaultStore)
	if err != nil {
		return nil, fmt.Errorf("Error in init persistence factory: %w", err)
	}
	cfg.Persistence.DataStores[cfg.Persistence.DefaultStore] = defaultStore

	cfg.Persistence.TransactionSizeLimit = dynamicproperties.GetIntPropertyFn(constants.DefaultTransactionSizeLimit)
	cfg.Persistence.ErrorInjectionRate = dynamicproperties.GetFloatPropertyFn(0.0)

	rps := c.Float64(FlagRPS)

	return client.NewFactory(
		&cfg.Persistence,
		func() float64 { return rps },
		cfg.ClusterGroupMetadata.CurrentClusterName,
		metrics.NewNoopMetricsClient(),
		log.NewNoop(),
		&persistence.DynamicConfiguration{
			EnableSQLAsyncTransaction: dynamicproperties.GetBoolPropertyFn(false),
		},
	), nil
}

func (f *defaultManagerFactory) initializeInvariantManager(ivs []invariant.Invariant) (invariant.Manager, error) {
	return invariant.NewInvariantManager(ivs), nil
}

func overrideDataStore(c *cli.Context, ds config.DataStore) (config.DataStore, error) {
	if c.IsSet(FlagDBType) {
		// overriding DBType will wipe out all settings, everything will be set from flags only
		var err error
		ds, err = createDataStore(c)
		if err != nil {
			return config.DataStore{}, fmt.Errorf("Error in overriding data store: %w", err)
		}
	}

	if ds.NoSQL != nil {
		overrideNoSQLDataStore(c, ds.NoSQL)
	}
	if ds.SQL != nil {
		overrideSQLDataStore(c, ds.SQL)
	}

	return ds, nil
}

func createDataStore(c *cli.Context) (config.DataStore, error) {
	dbType := c.String(FlagDBType)
	switch dbType {
	case cassandra.PluginName:
		return config.DataStore{NoSQL: &config.NoSQL{PluginName: cassandra.PluginName}}, nil
	default:
		if sql.PluginRegistered(dbType) {
			return config.DataStore{SQL: &config.SQL{PluginName: dbType}}, nil
		}
	}
	fmt.Errorf("The DB type is not supported. Options are: %s. Error %v", supportedDBs, nil)
	return config.DataStore{}, nil
}

func overrideNoSQLDataStore(c *cli.Context, cfg *config.NoSQL) {
	if c.IsSet(FlagDBAddress) || cfg.Hosts == "" {
		cfg.Hosts = c.String(FlagDBAddress)
	}
	if c.IsSet(FlagDBPort) || cfg.Port == 0 {
		cfg.Port = c.Int(FlagDBPort)
	}
	if c.IsSet(FlagDBRegion) || cfg.Region == "" {
		cfg.Region = c.String(FlagDBRegion)
	}
	if c.IsSet(FlagUsername) || cfg.User == "" {
		cfg.User = c.String(FlagUsername)
	}
	if c.IsSet(FlagPassword) || cfg.Password == "" {
		cfg.Password = c.String(FlagPassword)
	}
	if c.IsSet(FlagKeyspace) || cfg.Keyspace == "" {
		cfg.Keyspace = c.String(FlagKeyspace)
	}
	if c.IsSet(FlagProtoVersion) || cfg.ProtoVersion == 0 {
		cfg.ProtoVersion = c.Int(FlagProtoVersion)
	}

	if cfg.TLS == nil {
		cfg.TLS = &config.TLS{}
	}
	overrideTLS(c, cfg.TLS)
}

func overrideSQLDataStore(c *cli.Context, cfg *config.SQL) {
	host, port, _ := net.SplitHostPort(cfg.ConnectAddr)
	if c.IsSet(FlagDBAddress) || cfg.ConnectAddr == "" {
		host = c.String(FlagDBAddress)
	}
	if c.IsSet(FlagDBPort) || cfg.ConnectAddr == "" {
		port = c.String(FlagDBPort)
	}
	cfg.ConnectAddr = net.JoinHostPort(host, port)

	if c.IsSet(FlagDBType) || cfg.PluginName == "" {
		cfg.PluginName = c.String(FlagDBType)
	}
	if c.IsSet(FlagDBShard) || cfg.NumShards == 0 {
		cfg.NumShards = c.Int(FlagDBShard)
	}
	if c.IsSet(FlagUsername) || cfg.User == "" {
		cfg.User = c.String(FlagUsername)
	}
	if c.IsSet(FlagPassword) || cfg.Password == "" {
		cfg.Password = c.String(FlagPassword)
	}
	if c.IsSet(FlagDatabaseName) || cfg.DatabaseName == "" {
		cfg.DatabaseName = c.String(FlagDatabaseName)
	}
	if c.IsSet(FlagEncodingType) || cfg.EncodingType == "" {
		cfg.EncodingType = c.String(FlagEncodingType)
	}
	if c.IsSet(FlagDecodingTypes) || len(cfg.DecodingTypes) == 0 {
		cfg.DecodingTypes = c.StringSlice(FlagDecodingTypes)
	}
	if c.IsSet(FlagConnectionAttributes) || cfg.ConnectAttributes == nil {
		cfg.ConnectAttributes = c.Generic(FlagConnectionAttributes).(*flag.StringMap).Value()
	}

	if cfg.TLS == nil {
		cfg.TLS = &config.TLS{}
	}
	overrideTLS(c, cfg.TLS)
}

func overrideTLS(c *cli.Context, tls *config.TLS) {
	tls.Enabled = c.Bool(FlagEnableTLS)

	if c.IsSet(FlagTLSCertPath) {
		tls.CertFile = c.String(FlagTLSCertPath)
	}
	if c.IsSet(FlagTLSKeyPath) {
		tls.KeyFile = c.String(FlagTLSKeyPath)
	}
	if c.IsSet(FlagTLSCaPath) {
		tls.CaFile = c.String(FlagTLSCaPath)
	}
	if c.IsSet(FlagTLSEnableHostVerification) {
		tls.EnableHostVerification = c.Bool(FlagTLSEnableHostVerification)
	}
}
