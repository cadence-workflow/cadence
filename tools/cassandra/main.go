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

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/tools/common/schema"
)

// RunTool runs the cadence-cassandra-tool command line tool
func RunTool(args []string) error {
	app := BuildCLIOptions()
	return app.Run(args) // exits on error
}

// SetupSchema setups the cassandra schema
func SetupSchema(config *SetupSchemaConfig) error {
	if err := validateCQLClientConfig(&config.CQLClientConfig); err != nil {
		return err
	}
	db, err := NewCQLClient(&config.CQLClientConfig, gocql.All)
	if err != nil {
		return err
	}
	return schema.SetupFromConfig(&config.SetupConfig, db)
}

// root handler for all cli commands
func cliHandler(c *cli.Context, handler func(c *cli.Context) error) error {
	quiet := c.Bool(schema.CLIOptQuiet)
	err := handler(c)
	if err != nil {
		if quiet { // if quiet, don't return error
			fmt.Println("fail to run tool: ", err)
			return nil
		}
		return err
	}
	return nil
}

func BuildCLIOptions() *cli.App {

	app := cli.NewApp()
	app.Name = "cadence-cassandra-tool"
	app.Usage = "Command line tool for cadence cassandra operations"
	app.Version = "0.0.1"

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    schema.CLIFlagEndpoint,
			Aliases: []string{"ep"},
			Value:   "127.0.0.1",
			Usage:   "hostname or ip address of cassandra host to connect to",
			EnvVars: []string{"CASSANDRA_HOST"},
		},
		&cli.IntFlag{
			Name:    schema.CLIFlagPort,
			Aliases: []string{"p"},
			Value:   DefaultCassandraPort,
			Usage:   "Port of cassandra host to connect to",
			EnvVars: []string{"CASSANDRA_DB_PORT"},
		},
		&cli.StringFlag{
			Name:    schema.CLIFlagUser,
			Aliases: []string{"u"},
			Value:   "",
			Usage:   "User name used for authentication for connecting to cassandra host",
			EnvVars: []string{"CASSANDRA_USER"},
		},
		&cli.StringFlag{
			Name:    schema.CLIFlagPassword,
			Aliases: []string{"pw"},
			Value:   "",
			Usage:   "Password used for authentication for connecting to cassandra host",
			EnvVars: []string{"CASSANDRA_PASSWORD"},
		},
		&cli.StringSliceFlag{
			Name:    schema.CLIFlagAllowedAuthenticators,
			Aliases: []string{"aa"},
			Value:   cli.NewStringSlice(""),
			Usage:   "Set allowed authenticators for servers with custom authenticators",
		},
		&cli.IntFlag{
			Name:    schema.CLIFlagTimeout,
			Aliases: []string{"t"},
			Value:   DefaultTimeout,
			Usage:   "request Timeout in seconds used for cql client",
			EnvVars: []string{"CASSANDRA_TIMEOUT"},
		},
		&cli.IntFlag{
			Name:  schema.CLIOptConnectTimeout,
			Value: DefaultConnectTimeout,
			Usage: "Connection Timeout in seconds used for cql client",
		},
		&cli.StringFlag{
			Name:    schema.CLIFlagKeyspace,
			Aliases: []string{"k"},
			Value:   "cadence",
			Usage:   "name of the cassandra Keyspace",
			EnvVars: []string{"CASSANDRA_KEYSPACE"},
		},
		&cli.BoolFlag{
			Name:    schema.CLIFlagQuiet,
			Aliases: []string{"q"},
			Usage:   "Don't set exit status to 1 on error",
		},
		&cli.IntFlag{
			Name:    schema.CLIFlagProtoVersion,
			Aliases: []string{"pv"},
			Usage:   "Protocol Version to connect to cassandra host",
			EnvVars: []string{"CASSANDRA_PROTO_VERSION"},
		},

		&cli.BoolFlag{
			Name:    schema.CLIFlagEnableTLS,
			Usage:   "enable TLS",
			EnvVars: []string{"CASSANDRA_ENABLE_TLS"},
		},
		&cli.StringFlag{
			Name:    schema.CLIFlagTLSCertFile,
			Usage:   "TLS cert file",
			EnvVars: []string{"CASSANDRA_TLS_CERT"},
		},
		&cli.StringFlag{
			Name:    schema.CLIFlagTLSKeyFile,
			Usage:   "TLS key file",
			EnvVars: []string{"CASSANDRA_TLS_KEY"},
		},
		&cli.StringFlag{
			Name:    schema.CLIFlagTLSCaFile,
			Usage:   "TLS CA file",
			EnvVars: []string{"CASSANDRA_TLS_CA"},
		},
		&cli.BoolFlag{
			Name:    schema.CLIFlagTLSEnableHostVerification,
			Usage:   "TLS host verification",
			EnvVars: []string{"CASSANDRA_TLS_VERIFY_HOST"},
		},
		&cli.StringFlag{
			Name:    schema.CLIFlagTLSServerName,
			Usage:   "TLS ServerName",
			EnvVars: []string{"CASSANDRA_TLS_SERVER_NAME"},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:    "setup-schema",
			Aliases: []string{"setup"},
			Usage:   "setup initial version of cassandra schema",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    schema.CLIFlagVersion,
					Aliases: []string{"v"},
					Usage:   "initial version of the schema, cannot be used with disable-versioning",
				},
				&cli.StringFlag{
					Name:    schema.CLIFlagSchemaFile,
					Aliases: []string{"f"},
					Usage:   "path to the .cql schema file; if un-specified, will just setup versioning tables",
				},
				&cli.BoolFlag{
					Name:    schema.CLIFlagDisableVersioning,
					Aliases: []string{"d"},
					Usage:   "disable setup of schema versioning",
				},
				&cli.BoolFlag{
					Name:    schema.CLIFlagOverwrite,
					Aliases: []string{"o"},
					Usage:   "drop all existing tables before setting up new schema",
				},
			},
			Action: func(c *cli.Context) error {
				return cliHandler(c, setupSchema)
			},
		},
		{
			Name:    "update-schema",
			Aliases: []string{"update"},
			Usage:   "update cassandra schema to a specific version",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    schema.CLIFlagTargetVersion,
					Aliases: []string{"v"},
					Usage:   "target version for the schema update, defaults to latest",
				},
				&cli.StringFlag{
					Name:    schema.CLIFlagSchemaDir,
					Aliases: []string{"d"},
					Usage:   "path to directory containing versioned schema",
				},
				&cli.BoolFlag{
					Name:  schema.CLIFlagDryrun,
					Usage: "do a dryrun",
				},
			},
			Action: func(c *cli.Context) error {
				return cliHandler(c, updateSchema)
			},
		},
		{
			Name:    "create-Keyspace",
			Aliases: []string{"create"},
			Usage:   "creates a Keyspace with simple strategy. If datacenter is provided, will use network topology strategy",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    schema.CLIFlagKeyspace,
					Aliases: []string{"k"},
					Usage:   "name of the Keyspace",
				},
				&cli.StringFlag{
					Name:    schema.CLIFlagDatacenter,
					Aliases: []string{"dc"},
					Value:   "",
					Usage:   "name of the cassandra datacenter, used when creating the keyspace with network topology strategy",
				},
				&cli.IntFlag{
					Name:    schema.CLIFlagReplicationFactor,
					Aliases: []string{"rf"},
					Value:   1,
					Usage:   "replication factor for the Keyspace",
				},
			},
			Action: func(c *cli.Context) error {
				return cliHandler(c, createKeyspace)
			},
		},
	}

	return app
}
