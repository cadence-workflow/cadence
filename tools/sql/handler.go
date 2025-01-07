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

package sql

import (
	"fmt"
	"log"
	"net"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/config"
	mysql_db "github.com/uber/cadence/common/persistence/sql/sqlplugin/mysql"
	postgres_db "github.com/uber/cadence/common/persistence/sql/sqlplugin/postgres"
	"github.com/uber/cadence/schema/mysql"
	"github.com/uber/cadence/schema/postgres"
	cliflag "github.com/uber/cadence/tools/common/flag"
	"github.com/uber/cadence/tools/common/schema"
)

// VerifyCompatibleVersion ensures that the installed version of cadence and visibility
// is greater than or equal to the expected version.
func VerifyCompatibleVersion(
	cfg config.Persistence,
) error {

	ds, ok := cfg.DataStores[cfg.DefaultStore]
	if ok && ds.SQL != nil {
		expectedVersion := mysql.Version
		switch ds.SQL.PluginName {
		case mysql_db.PluginName:
			expectedVersion = mysql.Version
		case postgres_db.PluginName:
			expectedVersion = postgres.Version
		}
		err := CheckCompatibleVersion(*ds.SQL, expectedVersion)
		if err != nil {
			return err
		}
	}
	ds, ok = cfg.DataStores[cfg.VisibilityStore]
	if ok && ds.SQL != nil {
		expectedVersion := mysql.VisibilityVersion
		switch ds.SQL.PluginName {
		case mysql_db.PluginName:
			expectedVersion = mysql.VisibilityVersion
		case postgres_db.PluginName:
			expectedVersion = postgres.VisibilityVersion
		}
		err := CheckCompatibleVersion(*ds.SQL, expectedVersion)
		if err != nil {
			return err
		}
	}
	return nil
}

// CheckCompatibleVersion check the version compatibility
func CheckCompatibleVersion(
	cfg config.SQL,
	expectedVersion string,
) error {
	if !cfg.UseMultipleDatabases {
		return doCheckCompatibleVersion(cfg, expectedVersion)
	}
	// recover from the original at the end
	defer func() {
		cfg.User = ""
		cfg.Password = ""
		cfg.DatabaseName = ""
		cfg.ConnectAddr = ""
		cfg.UseMultipleDatabases = true
	}()
	cfg.UseMultipleDatabases = false
	// loop over every database to check schema version
	for idx, entry := range cfg.MultipleDatabasesConfig {
		cfg.User = entry.User
		cfg.Password = entry.Password
		cfg.DatabaseName = entry.DatabaseName
		cfg.ConnectAddr = entry.ConnectAddr

		err := doCheckCompatibleVersion(cfg, expectedVersion)
		if err != nil {
			return fmt.Errorf("shardID %d fails at check schema version: %v", idx, err)
		}
	}
	return nil
}

func doCheckCompatibleVersion(
	cfg config.SQL,
	expectedVersion string,
) error {
	connection, err := NewConnection(&cfg)

	if err != nil {
		return fmt.Errorf("unable to create SQL connection: %v", err.Error())
	}
	defer connection.Close()

	return schema.VerifyCompatibleVersion(connection, cfg.DatabaseName, expectedVersion)
}

// setupSchema executes the setupSchemaTask
// using the given command line arguments
// as input
func setupSchema(cli *cli.Context) error {
	cfg, err := parseConnectConfig(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	conn, err := NewConnection(cfg)
	if err != nil {
		return handleErr(err)
	}
	defer conn.Close()
	if err := schema.Setup(cli, conn); err != nil {
		return handleErr(err)
	}
	return nil
}

// updateSchema executes the updateSchemaTask
// using the given command lien args as input
func updateSchema(cli *cli.Context) error {
	cfg, err := parseConnectConfig(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	conn, err := NewConnection(cfg)
	if err != nil {
		return handleErr(err)
	}
	defer conn.Close()
	if err := schema.Update(cli, conn); err != nil {
		return handleErr(err)
	}
	return nil
}

// createDatabase creates a sql database
func createDatabase(cli *cli.Context) error {
	cfg, err := parseConnectConfig(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	database := cli.String(schema.CLIOptDatabase)
	if database == "" {
		return handleErr(schema.NewConfigError("missing " + flag(schema.CLIOptDatabase) + " argument "))
	}
	err = doCreateDatabase(cfg, database)
	if err != nil {
		return handleErr(fmt.Errorf("error creating database:%v", err))
	}
	return nil
}

func doCreateDatabase(cfg *config.SQL, name string) error {
	cfg.DatabaseName = ""
	conn, err := NewConnection(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.CreateDatabase(name)
}

func parseConnectConfig(cli *cli.Context) (*config.SQL, error) {
	cfg := new(config.SQL)

	host := cli.String(schema.CLIOptEndpoint)
	port := cli.Int(schema.CLIOptPort)
	cfg.ConnectAddr = fmt.Sprintf("%s:%v", host, port)
	cfg.User = cli.String(schema.CLIOptUser)
	cfg.Password = cli.String(schema.CLIOptPassword)
	cfg.DatabaseName = cli.String(schema.CLIOptDatabase)
	cfg.PluginName = cli.String(schema.CLIOptPluginName)

	connectAttributes := cli.Generic(schema.CLIOptConnectAttributes).(*cliflag.StringMap)
	cfg.ConnectAttributes = connectAttributes.Value()

	if cli.Bool(schema.CLIFlagEnableTLS) {
		cfg.TLS = &config.TLS{
			Enabled:                true,
			CertFile:               cli.String(schema.CLIFlagTLSCertFile),
			KeyFile:                cli.String(schema.CLIFlagTLSKeyFile),
			CaFile:                 cli.String(schema.CLIFlagTLSCaFile),
			EnableHostVerification: cli.Bool(schema.CLIFlagTLSEnableHostVerification),
		}
	}

	if err := ValidateConnectConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ValidateConnectConfig validates params
func ValidateConnectConfig(cfg *config.SQL) error {
	host, _, err := net.SplitHostPort(cfg.ConnectAddr)
	if err != nil {
		return schema.NewConfigError("invalid host and port " + cfg.ConnectAddr)
	}
	if len(host) == 0 {
		return schema.NewConfigError("missing sql endpoint argument " + flag(schema.CLIOptEndpoint))
	}
	if cfg.DatabaseName == "" {
		return schema.NewConfigError("missing " + flag(schema.CLIOptDatabase) + " argument")
	}
	if cfg.TLS != nil && cfg.TLS.Enabled {
		enabledCaFile := cfg.TLS.CaFile != ""
		enabledCertFile := cfg.TLS.CertFile != ""
		enabledKeyFile := cfg.TLS.KeyFile != ""

		if (enabledCertFile && !enabledKeyFile) || (!enabledCertFile && enabledKeyFile) {
			return schema.NewConfigError("must have both CertFile and KeyFile set")
		}

		if !enabledCaFile && !enabledCertFile && !enabledKeyFile {
			return schema.NewConfigError("must provide tls certs to use")
		}
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
