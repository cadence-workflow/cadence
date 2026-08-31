package cloudsqlmysql

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

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"

	"cloud.google.com/go/cloudsqlconn"
	cloudmysqldriver "cloud.google.com/go/cloudsqlconn/mysql/mysql"
	"cloud.google.com/go/compute/metadata"
	"github.com/iancoleman/strcase"
	"github.com/jmoiron/sqlx"
	"go.uber.org/multierr"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql"
	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	mysqlplugin "github.com/uber/cadence/common/persistence/sql/sqlplugin/mysql"
	"github.com/uber/cadence/environment"
	schemamysql "github.com/uber/cadence/schema/mysql"
)

const (
	// PluginName is the name of the plugin
	PluginName = "cloudsql-mysql"
	// This is the same structure as regular MySQL but with different parameters
	// my-user:mypass@cloudsql-mysql(my-proj:us-central1:my-inst)/my-db
	// Notably:
	// - The password is optional, and if you're using IAM auth you don't need it
	// - cloudsql-mysql is the driver name rather than the connection protocol
	// - the connection address is a CloudSQL instance rather than a host:port
	dsnFmt                       = "%s@%v(%v)/%s"
	isolationLevelAttrName       = "transaction_isolation"
	isolationLevelAttrNameLegacy = "tx_isolation"
	defaultIsolationLevel        = "'READ-COMMITTED'"
	driverFormat                 = "cloudsql-mysql-%d"
)

// Attributes that are used for driver options and should not be included in the DSN
var excludedDSNAttrs = map[string]bool{
	"iamAuthN": true,
	"ipType":   true,
}

var dsnAttrOverrides = map[string]string{
	"parseTime":       "true",
	"clientFoundRows": "true",
	"multiStatements": "true",
}

type plugin struct{}

var _ sqlplugin.Plugin = (*plugin)(nil)

var driverCounter atomic.Int32

func init() {
	sql.RegisterPlugin(PluginName, &plugin{})
}

// CreateDB initialize the DB object
func (p *plugin) CreateDB(cfg *config.SQL) (sqlplugin.DB, error) {
	return createDB(cfg)
}

// CreateAdminDB initialize the adminDb object
func (p *plugin) CreateAdminDB(cfg *config.SQL) (sqlplugin.AdminDB, error) {
	return createDB(cfg)
}

func (p *plugin) GetSchema(dbType persistence.DBType) (persistence.Schema, error) {
	switch dbType {
	case persistence.DBTypeDefault:
		return schemamysql.DefaultSchema, nil
	case persistence.DBTypeVisibility:
		return schemamysql.VisibilitySchema, nil
	default:
		return nil, fmt.Errorf("unknown db type: %v", dbType)
	}
}

// The CloudSQL driver has a different lifecycle compared to typical database/sql connectors:
// 1. We need to create and register the driver. There are many different options that can be specified for the driver
// and these must be set at the time of registration. As a result, you can register the driver with whatever name you'd
// like. If there are two different configurations we need to register two different drivers.
// 2. We then can create connections as normal using sqlx.Connect. Like most connectors, the DSN syntax is very specific.
// 3. When we're done with the conenctions we need to clean up the driver. It has background goroutines.
//
// To address this, when we create a new set of DB connections we use an incrementing identifier in the driver name
// When that set of connections is closed we then close the driver.
func createDB(cfg *config.SQL) (*mysqlplugin.DB, error) {
	driverName := fmt.Sprintf(driverFormat, driverCounter.Add(1))
	closeDriver, err := registerDriver(cfg, driverName)
	if err != nil {
		return nil, err
	}

	driver, err := sqldriver.CreateDBConnections(cfg, func(cfg *config.SQL) (*sqlx.DB, error) {
		return createSingleDBConn(cfg, driverName)
	}, closeDriver)
	if err != nil {
		multierr.AppendFunc(&err, closeDriver)
		return nil, err
	}
	// Once we have a DB connection then everything is the same as MySQL.
	return mysqlplugin.NewDB(driver, cfg.NumShards, mysqlplugin.NewConverter()), nil
}

func registerDriver(cfg *config.SQL, name string) (sqldriver.CloseFunc, error) {
	dialOpt, err := getDialOptions(cfg)
	if err != nil {
		return nil, err
	}
	options, err := getOptions(cfg)
	if err != nil {
		return nil, err
	}
	cleanup, err := cloudmysqldriver.RegisterDriver(name, options, cloudsqlconn.WithDefaultDialOptions(dialOpt))
	if err != nil {
		return nil, err
	}
	return cleanup, nil
}

func getOptions(cfg *config.SQL) (cloudsqlconn.Option, error) {
	value, ok := cfg.ConnectAttributes["iamAuthN"]
	if !ok || value == "" {
		return cloudsqlconn.WithIAMAuthN(), nil
	}
	switch value {
	case "true":
		return cloudsqlconn.WithIAMAuthN(), nil
	case "false":
		// Empty options
		return cloudsqlconn.WithOptions(), nil
	default:
		return nil, errors.New("invalid iamAuthN value")
	}
}

func getDialOptions(cfg *config.SQL) (cloudsqlconn.DialOption, error) {
	value, ok := cfg.ConnectAttributes["ipType"]
	if !ok {
		return cloudsqlconn.WithPSC(), nil
	}
	switch value {
	case "private":
		return cloudsqlconn.WithPrivateIP(), nil
	case "public":
		return cloudsqlconn.WithPublicIP(), nil
	case "psc":
		return cloudsqlconn.WithPSC(), nil
	default:
		return nil, errors.New("invalid ipType")
	}

}

func createSingleDBConn(cfg *config.SQL, driverName string) (*sqlx.DB, error) {
	dsn, err := buildDSN(cfg, driverName)
	if err != nil {
		return nil, err
	}
	// Can use either mysql or the DriverName, since the DSN also encodes the DriverName
	db, err := sqlx.Connect(driverName, dsn)
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

	// Maps struct names in CamelCase to snake without need for DB struct tags.
	db.MapperFunc(strcase.ToSnake)
	return db, nil
}

func buildDSN(cfg *config.SQL, driverName string) (string, error) {
	attrs := buildDSNAttrs(cfg)
	userAndPassword := cfg.User
	if userAndPassword == "" {
		serviceAccount, err := getServiceAccountUsername()
		if err != nil {
			return "", err
		}
		userAndPassword = serviceAccount
	}
	if cfg.Password != "" {
		userAndPassword += ":" + cfg.Password
	}
	dsn := fmt.Sprintf(dsnFmt, userAndPassword, driverName, cfg.ConnectAddr, cfg.DatabaseName)
	if attrs != "" {
		dsn = dsn + "?" + attrs
	}
	return dsn, nil
}

// Indirection to allow tests to stub out the GCE metadata server.
var (
	onGCE           = metadata.OnGCE
	getMetadataUser = metadata.EmailWithContext
)

func getServiceAccountUsername() (string, error) {
	if !onGCE() {
		return "", errors.New("missing User")
	}
	serviceAccountEmail, err := getMetadataUser(context.Background(), "default")
	if err != nil {
		return "", err
	}
	username, _, _ := strings.Cut(serviceAccountEmail, "@")
	if username == "" {
		return "", errors.New("missing User")
	}
	return username, nil
}

func buildDSNAttrs(cfg *config.SQL) string {
	attrs := make(map[string]string, len(dsnAttrOverrides)+len(cfg.ConnectAttributes)+1)
	for k, v := range cfg.ConnectAttributes {
		// Skip attributes that are used for driver options
		if excludedDSNAttrs[k] {
			continue
		}
		k1, v1 := sanitizeAttr(k, v)
		attrs[k1] = v1
	}

	// only override isolation level if not specified
	if !hasAttr(attrs, isolationLevelAttrName) &&
		!hasAttr(attrs, isolationLevelAttrNameLegacy) {
		attrs[isolationLevelAttrName] = defaultIsolationLevel
	}

	// these attrs are always overriden
	for k, v := range dsnAttrOverrides {
		attrs[k] = v
	}

	first := true
	var buf bytes.Buffer
	for k, v := range attrs {
		if !first {
			buf.WriteString("&")
		}
		first = false
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v)
	}
	return url.PathEscape(buf.String())
}

func hasAttr(attrs map[string]string, key string) bool {
	_, ok := attrs[key]
	return ok
}

func sanitizeAttr(inkey string, invalue string) (string, string) {
	key := strings.ToLower(strings.TrimSpace(inkey))
	value := strings.ToLower(strings.TrimSpace(invalue))
	switch key {
	case isolationLevelAttrName, isolationLevelAttrNameLegacy:
		if value[0] != '\'' { // mysql sys variable values must be enclosed in single quotes
			value = "'" + value + "'"
		}
		return key, value
	default:
		return inkey, invalue
	}
}

func GetTestConfig() (config.DataStore, error) {
	return config.DataStore{
		SQL: &config.SQL{
			PluginName:   PluginName,
			User:         environment.GetMySQLUser(),
			ConnectAddr:  environment.GetMySQLAddress(),
			DatabaseName: pt.GenerateRandomDBName(),
			NumShards:    4,
		},
	}, nil
}
