// Copyright (c) 2017 Uber Technologies, Inc.
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

package cassandra

import (
	"fmt"
	"log"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/schema/cassandra"
	"github.com/uber/cadence/tools/common/schema"
)

const defaultNumReplicas = 1

// SetupSchemaConfig contains the configuration params needed to setup schema tables
type SetupSchemaConfig struct {
	CQLClientConfig
	schema.SetupConfig
}

// VerifyCompatibleVersion ensures that the installed version of cadence and visibility keyspaces
// is greater than or equal to the expected version.
// In most cases, the versions should match. However if after a schema upgrade there is a code
// rollback, the code version (expected version) would fall lower than the actual version in
// cassandra.
func VerifyCompatibleVersion(
	cfg config.Persistence, expectedConsistency gocql.Consistency,
) error {

	if ds, ok := cfg.DataStores[cfg.DefaultStore]; ok {
		if err := verifyCompatibleVersion(ds, cassandra.Version, expectedConsistency); err != nil {
			return err
		}
	}

	if ds, ok := cfg.DataStores[cfg.VisibilityStore]; ok {
		if err := verifyCompatibleVersion(ds, cassandra.VisibilityVersion, expectedConsistency); err != nil {
			return err
		}
	}

	return nil
}

func verifyCompatibleVersion(
	ds config.DataStore,
	expectedCassandraVersion string, expectedConsistency gocql.Consistency,
) error {
	if ds.NoSQL != nil {
		return verifyPluginVersion(ds.NoSQL, expectedCassandraVersion, expectedConsistency)
	}
	if ds.ShardedNoSQL != nil {
		for shardName, connection := range ds.ShardedNoSQL.Connections {
			err := verifyPluginVersion(connection.NoSQLPlugin, expectedCassandraVersion, expectedConsistency)
			if err != nil {
				return fmt.Errorf("Failed to verify version for DB shard: %v. Error: %v", shardName, err.Error())
			}
		}
	}

	// not using nosql
	return nil
}

func verifyPluginVersion(plugin *config.NoSQL, expectedCassandraVersion string, expectedConsistency gocql.Consistency) error {
	// Use hardcoded instead of constant because of cycle dependency issue.
	// However, this file will be refactor to support NoSQL soon. After the refactoring, cycle dependency issue
	// should be gone and we can use constant at that time
	if plugin.PluginName != "cassandra" {
		return fmt.Errorf("unknown NoSQL plugin name: %q", plugin.PluginName)
	}

	return CheckCompatibleVersion(*plugin, expectedCassandraVersion, expectedConsistency)
}

// CheckCompatibleVersion check the version compatibility
func CheckCompatibleVersion(
	cfg config.Cassandra,
	expectedVersion string,
	expectedConsistency gocql.Consistency,
) error {

	client, err := NewCQLClient(&CQLClientConfig{
		Hosts:                 cfg.Hosts,
		Port:                  cfg.Port,
		User:                  cfg.User,
		Password:              cfg.Password,
		Keyspace:              cfg.Keyspace,
		AllowedAuthenticators: cfg.AllowedAuthenticators,
		Timeout:               DefaultTimeout,
		ConnectTimeout:        DefaultConnectTimeout,
		TLS:                   cfg.TLS,
		ProtoVersion:          cfg.ProtoVersion,
	}, expectedConsistency)
	if err != nil {
		return fmt.Errorf("creating CQL client: %w", err)
	}
	defer client.Close()

	return schema.VerifyCompatibleVersion(client, cfg.Keyspace, expectedVersion)
}

// setupSchema executes the setupSchemaTask
// using the given command line arguments
// as input
func setupSchema(cli *cli.Context) error {
	config, err := newCQLClientConfig(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	client, err := NewCQLClient(config, gocql.All)
	if err != nil {
		return handleErr(err)
	}
	defer client.Close()
	if err := schema.Setup(cli, client); err != nil {
		return handleErr(err)
	}
	return nil
}

// updateSchema executes the updateSchemaTask
// using the given command lien args as input
func updateSchema(cli *cli.Context) error {
	config, err := newCQLClientConfig(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	client, err := NewCQLClient(config, gocql.All)
	if err != nil {
		return handleErr(err)
	}
	defer client.Close()
	if err := schema.Update(cli, client); err != nil {
		return handleErr(err)
	}
	return nil
}

// createKeyspace creates a cassandra Keyspace
func createKeyspace(cli *cli.Context) error {
	config, err := newCQLClientConfig(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	keyspace := cli.String(schema.CLIOptKeyspace)
	if keyspace == "" {
		return handleErr(schema.NewConfigError("missing " + flag(schema.CLIOptKeyspace) + " argument "))
	}
	datacenter := cli.String(schema.CLIOptDatacenter)
	err = doCreateKeyspace(*config, keyspace, datacenter)
	if err != nil {
		return handleErr(fmt.Errorf("error creating Keyspace:%v", err))
	}
	return nil
}

func doCreateKeyspace(cfg CQLClientConfig, name string, datacenter string) error {
	cfg.Keyspace = SystemKeyspace
	client, err := NewCQLClient(&cfg, gocql.All)
	if err != nil {
		return err
	}
	defer client.Close()
	if datacenter != "" {
		return client.CreateNTSKeyspace(name, datacenter)
	}
	return client.CreateKeyspace(name)
}

func newCQLClientConfig(cli *cli.Context) (*CQLClientConfig, error) {
	cqlConfig := new(CQLClientConfig)
	cqlConfig.Hosts = cli.String(schema.CLIOptEndpoint)
	cqlConfig.Port = cli.Int(schema.CLIOptPort)
	cqlConfig.User = cli.String(schema.CLIOptUser)
	cqlConfig.Password = cli.String(schema.CLIOptPassword)
	cqlConfig.AllowedAuthenticators = cli.StringSlice(schema.CLIOptAllowedAuthenticators)
	cqlConfig.Timeout = cli.Int(schema.CLIOptTimeout)
	cqlConfig.ConnectTimeout = cli.Int(schema.CLIOptConnectTimeout)
	cqlConfig.Keyspace = cli.String(schema.CLIOptKeyspace)
	cqlConfig.NumReplicas = cli.Int(schema.CLIOptReplicationFactor)
	cqlConfig.ProtoVersion = cli.Int(schema.CLIOptProtoVersion)

	if cli.Bool(schema.CLIFlagEnableTLS) {
		cqlConfig.TLS = &config.TLS{
			Enabled:                true,
			CertFile:               cli.String(schema.CLIFlagTLSCertFile),
			KeyFile:                cli.String(schema.CLIFlagTLSKeyFile),
			CaFile:                 cli.String(schema.CLIFlagTLSCaFile),
			EnableHostVerification: cli.Bool(schema.CLIFlagTLSEnableHostVerification),
			ServerName:             cli.String(schema.CLIFlagTLSServerName),
		}
	}

	if err := validateCQLClientConfig(cqlConfig); err != nil {
		return nil, err
	}
	return cqlConfig, nil
}

func validateCQLClientConfig(config *CQLClientConfig) error {
	if len(config.Hosts) == 0 {
		return schema.NewConfigError("missing cassandra endpoint argument " + flag(schema.CLIOptEndpoint))
	}
	if config.Keyspace == "" {
		return schema.NewConfigError("missing " + flag(schema.CLIOptKeyspace) + " argument ")
	}
	if config.Port == 0 {
		config.Port = DefaultCassandraPort
	}
	if config.NumReplicas == 0 {
		config.NumReplicas = defaultNumReplicas
	}

	return nil
}

func flag(opt string) string {
	return "(-" + opt + ")"
}

func handleErr(err error) error {
	log.Println(err)
	return err
}
