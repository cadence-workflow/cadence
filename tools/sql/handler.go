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
	"strconv"
	"strings"

	"github.com/urfave/cli"

	"github.com/uber/cadence/common/service/config"
	"github.com/uber/cadence/schema/mysql"
	"github.com/uber/cadence/tools/common/schema"
)

// VerifyCompatibleVersion ensures that the installed version of cadence and visibility
// is greater than or equal to the expected version.
func VerifyCompatibleVersion(
	cfg config.Persistence,
) error {

	ds, ok := cfg.DataStores[cfg.DefaultStore]
	if ok && ds.SQL != nil {
		err := checkCompatibleVersion(*ds.SQL, mysql.Version)
		if err != nil {
			return err
		}
	}
	ds, ok = cfg.DataStores[cfg.VisibilityStore]
	if ok && ds.SQL != nil {
		err := checkCompatibleVersion(*ds.SQL, mysql.VisibilityVersion)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkCompatibleVersion check the version compatibility
func checkCompatibleVersion(
	cfg config.SQL,
	expectedVersion string,
) error {
	var host string
	var port int
	if strings.Contains(cfg.ConnectAddr, ":") {
		ss := strings.Split(cfg.ConnectAddr, ":")
		if len(ss) != 2 {
			panic("invalid connect address, it must be in host:port format")
		}
		var err error
		host = ss[0]
		port, err = strconv.Atoi(ss[1])
		if err != nil {
			panic("invalid port number:" + ss[1])
		}
	} else {
		host = cfg.ConnectAddr
		port = defaultSQLPort
	}

	connection, err := newConnection(&sqlConnectParams{
		host:       host,
		port:       port,
		user:       cfg.User,
		password:   cfg.Password,
		driverName: cfg.DriverName,
		database:   cfg.DatabaseName,
	})
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
	params, err := parseConnectParams(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	conn, err := newConnection(params)
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
	params, err := parseConnectParams(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	if params.database == schema.DryrunDBName {
		p := *params
		if err := doCreateDatabase(p, p.database); err != nil {
			return handleErr(fmt.Errorf("error creating dryrun database: %v", err))
		}
		defer doDropDatabase(p, p.database)
	}
	conn, err := newConnection(params)
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
	params, err := parseConnectParams(cli)
	if err != nil {
		return handleErr(schema.NewConfigError(err.Error()))
	}
	database := cli.String(schema.CLIOptDatabase)
	if database == "" {
		return handleErr(schema.NewConfigError("missing " + flag(schema.CLIOptDatabase) + " argument "))
	}
	err = doCreateDatabase(*params, database)
	if err != nil {
		return handleErr(fmt.Errorf("error creating database:%v", err))
	}
	return nil
}

func doCreateDatabase(p sqlConnectParams, name string) error {
	p.database = ""
	conn, err := newConnection(&p)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.CreateDatabase(name)
}

func doDropDatabase(p sqlConnectParams, name string) {
	p.database = ""
	conn, err := newConnection(&p)
	if err != nil {
		handleErr(err)
		return
	}
	conn.DropDatabase(name)
	conn.Close()
}

func parseConnectParams(cli *cli.Context) (*sqlConnectParams, error) {
	params := new(sqlConnectParams)
	params.host = cli.GlobalString(schema.CLIOptEndpoint)
	params.port = cli.GlobalInt(schema.CLIOptPort)
	params.user = cli.GlobalString(schema.CLIOptUser)
	params.password = cli.GlobalString(schema.CLIOptPassword)
	params.database = cli.GlobalString(schema.CLIOptDatabase)
	params.driverName = cli.GlobalString(schema.CLIOptDriverName)
	isDryRun := cli.Bool(schema.CLIOptDryrun)
	if err := validateConnectParams(params, isDryRun); err != nil {
		return nil, err
	}
	return params, nil
}

func validateConnectParams(params *sqlConnectParams, isDryRun bool) error {
	if len(params.host) == 0 {
		return schema.NewConfigError("missing sql endpoint argument " + flag(schema.CLIOptEndpoint))
	}
	if params.database == "" {
		if !isDryRun {
			return schema.NewConfigError("missing " + flag(schema.CLIOptDatabase) + " argument ")
		}
		params.database = schema.DryrunDBName
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
