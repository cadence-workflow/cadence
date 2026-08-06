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
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

type (
	db struct {
		converter   DataConverter
		driver      sqldriver.Driver
		numDBShards int
	}
)

func (pdb *db) GetTotalNumDBShards() int {
	return pdb.numDBShards
}

var _ sqlplugin.DB = (*db)(nil)
var _ sqlplugin.Tx = (*db)(nil)

// ErrDupEntry indicates a duplicate primary key i.e. the row already exists,
// check http://www.postgresql.org/docs/9.3/static/errcodes-appendix.html
const ErrDupEntry = "23505"

const ErrInsufficientResources = "53000"
const ErrTooManyConnections = "53300"

func (pdb *db) IsDupEntryError(err error) bool {
	sqlErr, ok := err.(*pq.Error)
	return ok && sqlErr.Code == ErrDupEntry
}

func (pdb *db) IsNotFoundError(err error) bool {
	return err == sql.ErrNoRows
}

func (pdb *db) IsTimeoutError(err error) bool {
	return err == context.DeadlineExceeded
}

func (pdb *db) IsThrottlingError(err error) bool {
	sqlErr, ok := err.(*pq.Error)
	if ok {
		if sqlErr.Code == ErrTooManyConnections ||
			sqlErr.Code == ErrInsufficientResources {
			return true
		}
	}
	return false
}

// newDB returns an instance of DB, which is a logical
// connection to the underlying postgres database
// dbShardID is needed when tx is not nil
func newDB(driver sqldriver.Driver, numDBShards int) *db {
	return &db{
		converter:   &converter{},
		driver:      driver,
		numDBShards: numDBShards,
	}
}

// BeginTx starts a new transaction and returns a reference to the Tx object
func (pdb *db) BeginTx(ctx context.Context, dbShardID int) (sqlplugin.Tx, error) {
	driver, err := pdb.driver.BeginTransaction(ctx, dbShardID, nil)
	if err != nil {
		return nil, err
	}
	return newDB(driver, pdb.numDBShards), nil
}

// Commit commits a previously started transaction
func (pdb *db) Commit() error {
	return pdb.driver.Commit()
}

// Rollback triggers rollback of a previously started transaction
func (pdb *db) Rollback() error {
	return pdb.driver.Rollback()
}

// Close closes the connection to the mysql db
func (pdb *db) Close() error {
	return pdb.driver.Close()
}

// PluginName returns the name of the mysql plugin
func (pdb *db) PluginName() string {
	return PluginName
}

// SupportsTTL returns weather Postgres supports TTL
func (pdb *db) SupportsTTL() bool {
	return false
}

// MaxAllowedTTL returns the max allowed ttl Postgres supports
func (pdb *db) MaxAllowedTTL() (*time.Duration, error) {
	return nil, sqlplugin.ErrTTLNotSupported
}

// SupportsTTL returns weather Postgre supports Asynchronous transaction
func (pdb *db) SupportsAsyncTransaction() bool {
	return false
}
