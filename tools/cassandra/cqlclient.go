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
	"context"
	"fmt"
	"log"
	"time"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/tools/common/schema"
)

type (
	CqlClient interface {
		CreateDatabase(name string) error
		DropDatabase(name string) error
		CreateKeyspace(name string) error
		CreateNTSKeyspace(name string, datacenter string) error
		DropKeyspace(name string) error
		DropAllTables() error
		CreateSchemaVersionTables() error
		ReadSchemaVersion() (string, error)
		UpdateSchemaVersion(newVersion string, minCompatibleVersion string) error
		WriteSchemaUpdateLog(oldVersion string, newVersion string, manifestMD5 string, desc string) error
		ExecDDLQuery(stmt string, args ...interface{}) error
		Close()
		ListTables() ([]string, error)
		ListTypes() ([]string, error)
		DropTable(name string) error
		DropType(name string) error
		DropAllTablesTypes() error
	}

	CqlClientImpl struct {
		nReplicas int
		session   gocql.Session
		cfg       *CQLClientConfig
	}

	// CQLClientConfig contains the configuration for cql client
	CQLClientConfig struct {
		Hosts                 string
		Port                  int
		User                  string
		Password              string
		AllowedAuthenticators []string
		Keyspace              string
		Timeout               int
		ConnectTimeout        int
		NumReplicas           int
		ProtoVersion          int
		TLS                   *config.TLS
	}
)

const (
	DefaultTimeout        = 30 // Timeout in seconds
	DefaultConnectTimeout = 2  // Connect timeout in seconds
	DefaultCassandraPort  = 9042
	SystemKeyspace        = "system"
)

const (
	// Retry policy constants
	ClientCreationRetryInitialInterval = time.Second
	ClientCreationRetryMaximumInterval = 10 * time.Second
	ClientCreationRetryMaximumAttempts = 3
)

const (
	readSchemaVersionCQL        = `SELECT curr_version from schema_version where keyspace_name=?`
	listTablesCQL               = `SELECT table_name from system_schema.tables where keyspace_name=?`
	listTypesCQL                = `SELECT type_name from system_schema.types where keyspace_name=?`
	writeSchemaVersionCQL       = `INSERT into schema_version(keyspace_name, creation_time, curr_version, min_compatible_version) VALUES (?,?,?,?)`
	writeSchemaUpdateHistoryCQL = `INSERT into schema_update_history(year, month, update_time, old_version, new_version, manifest_md5, description) VALUES(?,?,?,?,?,?,?)`

	createSchemaVersionTableCQL = `CREATE TABLE schema_version(keyspace_name text PRIMARY KEY, ` +
		`creation_time timestamp, ` +
		`curr_version text, ` +
		`min_compatible_version text);`

	createSchemaUpdateHistoryTableCQL = `CREATE TABLE schema_update_history(` +
		`year int, ` +
		`month int, ` +
		`update_time timestamp, ` +
		`description text, ` +
		`manifest_md5 text, ` +
		`new_version text, ` +
		`old_version text, ` +
		`PRIMARY KEY ((year, month), update_time));`

	createKeyspaceCQL = `CREATE KEYSPACE IF NOT EXISTS %v ` +
		`WITH replication = { 'class' : 'SimpleStrategy', 'replication_factor' : %v};`

	createNTSKeyspaceCQL = `CREATE KEYSPACE IF NOT EXISTS %v ` +
		`WITH replication = { 'class' : 'NetworkTopologyStrategy', '%v' : %v};`
)

var _ schema.SchemaClient = (*CqlClientImpl)(nil)

func NewCQLClient(cfg *CQLClientConfig, expectedConsistency gocql.Consistency) (CqlClient, error) {
	retrier := createClientCreationRetrier()
	cassClient := gocql.GetRegisteredClient()
	return NewCQLClientWithRetry(cfg, expectedConsistency, cassClient, retrier)
}

// NewCQLClient returns a new instance of CQLClient
func NewCQLClientWithRetry(cfg *CQLClientConfig, expectedConsistency gocql.Consistency, cassClient gocql.Client, retrier *backoff.ThrottleRetry) (CqlClient, error) {
	cqlClient := new(CqlClientImpl)
	cqlClient.cfg = cfg
	cqlClient.nReplicas = cfg.NumReplicas

	err := retrier.Do(context.Background(), func(ctx context.Context) error {
		var err error
		cqlClient.session, err = cassClient.CreateSession(gocql.ClusterConfig{
			Hosts:                 cfg.Hosts,
			Port:                  cfg.Port,
			User:                  cfg.User,
			Password:              cfg.Password,
			AllowedAuthenticators: cfg.AllowedAuthenticators,
			Keyspace:              cfg.Keyspace,
			TLS:                   cfg.TLS,
			Timeout:               time.Duration(cfg.Timeout) * time.Second,
			ConnectTimeout:        time.Duration(cfg.ConnectTimeout) * time.Second,
			ProtoVersion:          cfg.ProtoVersion,
			Consistency:           expectedConsistency,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return cqlClient, nil
}

func (client *CqlClientImpl) CreateDatabase(name string) error {
	return client.CreateKeyspace(name)
}

func (client *CqlClientImpl) DropDatabase(name string) error {
	return client.DropKeyspace(name)
}

// CreateNTSKeyspace creates a cassandra Keyspace if it doesn't exist using network topology strategy
func (client *CqlClientImpl) CreateKeyspace(name string) error {
	return client.ExecDDLQuery(fmt.Sprintf(createKeyspaceCQL, name, client.nReplicas))
}

// CreateNTSKeyspace creates a cassandra Keyspace if it doesn't exist using network topology strategy
func (client *CqlClientImpl) CreateNTSKeyspace(name string, datacenter string) error {
	return client.ExecDDLQuery(fmt.Sprintf(createNTSKeyspaceCQL, name, datacenter, client.nReplicas))
}

// DropKeyspace drops a Keyspace
func (client *CqlClientImpl) DropKeyspace(name string) error {
	return client.ExecDDLQuery(fmt.Sprintf("DROP KEYSPACE %v", name))
}

func (client *CqlClientImpl) DropAllTables() error {
	return client.DropAllTablesTypes()
}

// CreateSchemaVersionTables sets up the schema version tables
func (client *CqlClientImpl) CreateSchemaVersionTables() error {
	if err := client.ExecDDLQuery(createSchemaVersionTableCQL); err != nil {
		return err
	}
	return client.ExecDDLQuery(createSchemaUpdateHistoryTableCQL)
}

// ReadSchemaVersion returns the current schema version for the Keyspace
func (client *CqlClientImpl) ReadSchemaVersion() (string, error) {
	query := client.session.Query(readSchemaVersionCQL, client.cfg.Keyspace)
	iter := query.Iter()
	var version string
	if !iter.Scan(&version) {
		err := iter.Close()
		return "", fmt.Errorf("reading schema version: %w", err)
	}
	if err := iter.Close(); err != nil {
		return "", err
	}
	return version, nil
}

// UpdateSchemaVersion updates the schema version for the Keyspace
func (client *CqlClientImpl) UpdateSchemaVersion(newVersion string, minCompatibleVersion string) error {
	query := client.session.Query(writeSchemaVersionCQL, client.cfg.Keyspace, time.Now(), newVersion, minCompatibleVersion)
	return query.Exec()
}

// WriteSchemaUpdateLog adds an entry to the schema update history table
func (client *CqlClientImpl) WriteSchemaUpdateLog(oldVersion string, newVersion string, manifestMD5 string, desc string) error {
	now := time.Now().UTC()
	query := client.session.Query(writeSchemaUpdateHistoryCQL)
	query.Bind(now.Year(), int(now.Month()), now, oldVersion, newVersion, manifestMD5, desc)
	return query.Exec()
}

// ExecDDLQuery executes a cql statement
func (client *CqlClientImpl) ExecDDLQuery(stmt string, args ...interface{}) error {
	return client.session.Query(stmt, args...).Exec()
}

// Close closes the cql client
func (client *CqlClientImpl) Close() {
	if client.session != nil {
		client.session.Close()
	}
}

// ListTables lists the table names in a Keyspace
func (client *CqlClientImpl) ListTables() ([]string, error) {
	query := client.session.Query(listTablesCQL, client.cfg.Keyspace)
	iter := query.Iter()
	var names []string
	var name string
	for iter.Scan(&name) {
		names = append(names, name)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return names, nil
}

// ListTypes lists the User defined types in a Keyspace.
func (client *CqlClientImpl) ListTypes() ([]string, error) {
	qry := client.session.Query(listTypesCQL, client.cfg.Keyspace)
	iter := qry.Iter()
	var names []string
	var name string
	for iter.Scan(&name) {
		names = append(names, name)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return names, nil
}

// DropTable drops a given table from the Keyspace
func (client *CqlClientImpl) DropTable(name string) error {
	return client.ExecDDLQuery(fmt.Sprintf("DROP TABLE %v", name))
}

// DropType drops a given type from the Keyspace
func (client *CqlClientImpl) DropType(name string) error {
	return client.ExecDDLQuery(fmt.Sprintf("DROP TYPE %v", name))
}

// DropAllTablesTypes deletes all tables/types in the
// Keyspace without deleting the Keyspace
func (client *CqlClientImpl) DropAllTablesTypes() error {
	tables, err := client.ListTables()
	if err != nil {
		return err
	}
	log.Printf("Dropping following tables: %v\n", tables)
	for _, table := range tables {
		err1 := client.DropTable(table)
		if err1 != nil {
			log.Printf("Error dropping table %v, err=%v\n", table, err1)
		}
	}

	types, err := client.ListTypes()
	if err != nil {
		return err
	}
	log.Printf("Dropping following types: %v\n", types)
	numOfTypes := len(types)
	for i := 0; i < numOfTypes && len(types) > 0; i++ {
		var erroredTypes []string
		for _, t := range types {
			err = client.DropType(t)
			if err != nil {
				log.Printf("Error dropping type %v, err=%v\n", t, err)
				erroredTypes = append(erroredTypes, t)
			}
		}
		types = erroredTypes
	}
	if len(types) > 0 {
		return err
	}
	return nil
}

func createClientCreationRetrier() *backoff.ThrottleRetry {
	policy := backoff.NewExponentialRetryPolicy(ClientCreationRetryInitialInterval)
	policy.SetMaximumInterval(ClientCreationRetryMaximumInterval)
	policy.SetMaximumAttempts(ClientCreationRetryMaximumAttempts)

	retrier := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(policy),
		backoff.WithRetryableError(func(err error) bool { return true }),
	)
	return retrier
}
