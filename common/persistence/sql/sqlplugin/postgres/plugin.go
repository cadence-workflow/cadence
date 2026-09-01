// Copyright (c) 2019 Uber Technologies, Inc.
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

package postgres

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"

	"github.com/iancoleman/strcase"
	"github.com/jmoiron/sqlx"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql"
	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/environment"
	"github.com/uber/cadence/schema/postgres"
)

const (
	// PluginName is the name of the plugin
	PluginName = "postgres"
	dsnFmt     = "postgres://%s@%s:%s/%s"
)

type plugin struct{}

var _ sqlplugin.Plugin = (*plugin)(nil)

func init() {
	sql.RegisterPlugin(PluginName, &plugin{})
}

// CreateDB initialize the db object
func (d *plugin) CreateDB(cfg *config.SQL) (sqlplugin.DB, error) {
	driver, err := sqldriver.CreateDBConnections(cfg, d.createSingleDBConn, sqldriver.NoopClose)
	if err != nil {
		return nil, err
	}
	return newDB(driver, cfg.NumShards), nil
}

// CreateAdminDB initialize the adminDB object
func (d *plugin) CreateAdminDB(cfg *config.SQL) (sqlplugin.AdminDB, error) {
	driver, err := sqldriver.CreateDBConnections(cfg, d.createSingleDBConn, sqldriver.NoopClose)
	if err != nil {
		return nil, err
	}
	return newDB(driver, cfg.NumShards), nil
}

func (d *plugin) GetSchema(dbType persistence.DBType) (persistence.Schema, error) {
	switch dbType {
	case persistence.DBTypeDefault:
		return postgres.DefaultSchema, nil
	case persistence.DBTypeVisibility:
		return postgres.VisibilitySchema, nil
	default:
		return nil, fmt.Errorf("unknown db type: %v", dbType)
	}
}

// CreateDBConnection creates a returns a reference to a logical connection to the
// underlying SQL database. The returned object is to tied to a single
// SQL database and the object can be used to perform CRUD operations on
// the tables in the database
func (d *plugin) createSingleDBConn(cfg *config.SQL) (*sqlx.DB, error) {
	params, err := registerTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.ConnectAttributes {
		params.Set(k, v)
	}
	host, port, err := net.SplitHostPort(cfg.ConnectAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid connect address, it must be in host:port format, %v, err: %v", cfg.ConnectAddr, err)
	}

	db, err := sqlx.Connect(PluginName, buildDSN(cfg, host, port, params))
	if err != nil {
		return nil, err
	}
	if cfg.MaxConns > 0 {
		db.SetMaxOpenConns(cfg.MaxConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxConnLifetime > 0 {
		db.SetConnMaxLifetime(cfg.MaxConnLifetime)
	}

	// Maps struct names in CamelCase to snake without need for db struct tags.
	db.MapperFunc(strcase.ToSnake)
	return db, nil
}

func buildDSN(cfg *config.SQL, host string, port string, params url.Values) string {
	dbName := cfg.DatabaseName
	// NOTE: postgres doesn't allow to connect with empty dbName, the admin dbName is "postgres"
	if dbName == "" {
		dbName = "postgres"
	}

	credentialString := generateCredentialString(cfg.User, cfg.Password)
	dsn := fmt.Sprintf(dsnFmt, credentialString, host, port, dbName)
	if attrs := params.Encode(); attrs != "" {
		dsn += "?" + attrs
	}
	return dsn
}

func generateCredentialString(user string, password string) string {
	userPass := url.PathEscape(user)
	if password != "" {
		userPass += ":" + url.PathEscape(password)
	}
	return userPass
}

func registerTLSConfig(cfg *config.SQL) (sslParams url.Values, err error) {
	sslParams = url.Values{}
	if cfg.TLS != nil && cfg.TLS.Enabled {
		sslMode := cfg.TLS.SSLMode
		if sslMode == "" {
			// NOTE: Default to require for backward compatibility for Cadence users.
			sslMode = "require"
		}
		sslParams.Set("sslmode", sslMode)
		sslParams.Set("sslrootcert", cfg.TLS.CaFile)
		sslParams.Set("sslkey", cfg.TLS.KeyFile)
		sslParams.Set("sslcert", cfg.TLS.CertFile)
	} else {
		sslParams.Set("sslmode", "disable")
	}
	return
}

// GetTestConfig return test options
func GetTestConfig() (config.DataStore, error) {
	testUser := "postgres"
	testPassword := "cadence"

	if runtime.GOOS == "darwin" {
		testUser = os.Getenv("USER")
		testPassword = ""
	}

	if os.Getenv("POSTGRES_USER") != "" {
		testUser = os.Getenv("POSTGRES_USER")
	}

	if os.Getenv("POSTGRES_PASSWORD") != "" {
		testPassword = os.Getenv("POSTGRES_PASSWORD")
	}
	dbPort, err := environment.GetPostgresPort()
	if err != nil {
		return config.DataStore{}, err
	}

	return config.DataStore{
		SQL: &config.SQL{
			PluginName:   PluginName,
			User:         testUser,
			Password:     testPassword,
			ConnectAddr:  fmt.Sprintf("%s:%d", environment.GetPostgresAddress(), dbPort),
			DatabaseName: pt.GenerateRandomDBName(),
			NumShards:    4,
		},
	}, nil
}
