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

package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	readSchemaVersionQuery = `SELECT curr_version from schema_version where db_name=?`

	writeSchemaVersionQuery = `REPLACE into schema_version(db_name, creation_time, curr_version, min_compatible_version) VALUES (?,?,?,?)`

	writeSchemaUpdateHistoryQuery = `INSERT into schema_update_history(year, month, update_time, old_version, new_version, manifest_md5, description) VALUES(?,?,?,?,?,?,?)`

	createSchemaVersionTableQuery = `CREATE TABLE schema_version(db_name VARCHAR(255) not null PRIMARY KEY, ` +
		`creation_time DATETIME(6), ` +
		`curr_version VARCHAR(64), ` +
		`min_compatible_version VARCHAR(64));`

	createSchemaUpdateHistoryTableQuery = `CREATE TABLE schema_update_history(` +
		`year int not null, ` +
		`month int not null, ` +
		`update_time DATETIME(6) not null, ` +
		`description VARCHAR(255), ` +
		`manifest_md5 VARCHAR(64), ` +
		`new_version VARCHAR(64), ` +
		`old_version VARCHAR(64), ` +
		`PRIMARY KEY (year, month, update_time));`

	// NOTE we have to use %v because somehow mysql doesn't work with ? here
	createDatabaseQuery = "CREATE database %v CHARACTER SET UTF8"

	dropDatabaseQuery = "Drop database %v"

	listTablesQuery = "SHOW TABLES FROM %v"

	dropTableQuery = "DROP TABLE %v"
)

// CreateSchemaVersionTables sets up the schema version tables
func (mdb *DB) CreateSchemaVersionTables() error {
	if err := mdb.ExecSchemaOperationQuery(context.Background(), createSchemaVersionTableQuery); err != nil {
		return err
	}
	return mdb.ExecSchemaOperationQuery(context.Background(), createSchemaUpdateHistoryTableQuery)
}

// ReadSchemaVersion returns the current schema version for the keyspace
func (mdb *DB) ReadSchemaVersion(database string) (string, error) {
	var version string
	err := mdb.driver.GetForSchemaQuery(sqlplugin.DbShardUndefined, &version, readSchemaVersionQuery, database)
	return version, err
}

// UpdateSchemaVersion updates the schema version for the keyspace
func (mdb *DB) UpdateSchemaVersion(database string, newVersion string, minCompatibleVersion string) error {
	return mdb.ExecSchemaOperationQuery(context.Background(), writeSchemaVersionQuery, database, time.Now(), newVersion, minCompatibleVersion)
}

// WriteSchemaUpdateLog adds an entry to the schema update history table
func (mdb *DB) WriteSchemaUpdateLog(oldVersion string, newVersion string, manifestMD5 string, desc string) error {
	now := time.Now().UTC()
	return mdb.ExecSchemaOperationQuery(context.Background(), writeSchemaUpdateHistoryQuery, now.Year(), int(now.Month()), now, oldVersion, newVersion, manifestMD5, desc)
}

// ExecSchemaOperationQuery executes a sql statement for schema ONLY. DO NOT use it in other cases, otherwise it will not work for multiple SQL database.
// For Sharded SQL, it will execute the statement for all shards
func (mdb *DB) ExecSchemaOperationQuery(ctx context.Context, stmt string, args ...interface{}) error {
	_, err := mdb.driver.ExecDDL(ctx, sqlplugin.DbShardUndefined, stmt, args...)
	return err
}

// ListTables returns a list of tables in this database
func (mdb *DB) ListTables(database string) ([]string, error) {
	var tables []string
	err := mdb.driver.SelectForSchemaQuery(sqlplugin.DbShardUndefined, &tables, fmt.Sprintf(listTablesQuery, database))
	return tables, err
}

// DropTable drops a given table from the database
func (mdb *DB) DropTable(name string) error {
	return mdb.ExecSchemaOperationQuery(context.Background(), fmt.Sprintf(dropTableQuery, name))
}

// DropAllTables drops all tables from this database
func (mdb *DB) DropAllTables(database string) error {
	tables, err := mdb.ListTables(database)
	if err != nil {
		return err
	}
	for _, tab := range tables {
		if err := mdb.DropTable(tab); err != nil {
			return err
		}
	}
	return nil
}

// CreateDatabase creates a database if it doesn't exist
func (mdb *DB) CreateDatabase(name string) error {
	return mdb.ExecSchemaOperationQuery(context.Background(), fmt.Sprintf(createDatabaseQuery, name))
}

// DropDatabase drops a database
func (mdb *DB) DropDatabase(name string) error {
	return mdb.ExecSchemaOperationQuery(context.Background(), fmt.Sprintf(dropDatabaseQuery, name))
}
