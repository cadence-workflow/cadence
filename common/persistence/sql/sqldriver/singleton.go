// Copyright (c) 2021 Uber Technologies, Inc.
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

package sqldriver

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"go.uber.org/multierr"
)

type (
	// singleton is the driver querying a single SQL database, which is the default driver
	singleton struct {
		db     *sqlx.DB // this is for starting a transaction, or executing any non transaction query
		tx     *sqlx.Tx // this is a reference of a started transaction
		useTx  bool     // if tx is not nil, the methods from commonOfDbAndTx should use tx
		closer CloseFunc
	}
)

// newSingletonSQLDriver returns a driver querying a single SQL database, which is the default driver
// typically dbShardID is needed when tx is not nil, because it means a started transaction in a shard.
// But this singleton doesn't have sharding so omitting it.
func newSingletonSQLDriver(xdb *sqlx.DB, xtx *sqlx.Tx, closer CloseFunc) Driver {
	driver := &singleton{
		db:     xdb,
		tx:     xtx,
		closer: closer,
	}
	if xtx != nil {
		driver.useTx = true
	}
	return driver
}

// below are shared by transactional and non-transactional, if s.tx is not nil then use s.tx, otherwise use s.db

func (s *singleton) ExecContext(ctx context.Context, _ int, query string, args ...interface{}) (sql.Result, error) {
	if s.useTx {
		return s.tx.ExecContext(ctx, query, args...)
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *singleton) NamedExecContext(ctx context.Context, _ int, query string, arg interface{}) (sql.Result, error) {
	if s.useTx {
		return s.tx.NamedExecContext(ctx, query, arg)
	}
	return s.db.NamedExecContext(ctx, query, arg)
}

func (s *singleton) GetContext(ctx context.Context, _ int, dest interface{}, query string, args ...interface{}) error {
	if s.useTx {
		return s.tx.GetContext(ctx, dest, query, args...)
	}
	return s.db.GetContext(ctx, dest, query, args...)
}

func (s *singleton) SelectContext(ctx context.Context, _ int, dest interface{}, query string, args ...interface{}) error {
	if s.useTx {
		return s.tx.SelectContext(ctx, dest, query, args...)
	}
	return s.db.SelectContext(ctx, dest, query, args...)
}

// below are non-transactional methods only

func (s *singleton) ExecDDL(ctx context.Context, _ int, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *singleton) SelectForSchemaQuery(_ int, dest interface{}, query string, args ...interface{}) error {
	return s.db.Select(dest, query, args...)
}

func (s *singleton) GetForSchemaQuery(_ int, dest interface{}, query string, args ...interface{}) error {
	return s.db.Get(dest, query, args...)
}

func (s *singleton) BeginTransaction(ctx context.Context, _ int, opts *sql.TxOptions) (Driver, error) {
	tx, err := s.db.BeginTxx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return newSingletonSQLDriver(s.db, tx, s.closer), nil
}

func (s *singleton) Close() error {
	var errs []error
	err := s.db.Close()
	if err != nil {
		errs = append(errs, err)
	}
	if s.closer != nil {
		err = s.closer()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return multierr.Combine(errs...)
	}
	return nil
}

// below are transactional methods only

func (s *singleton) Commit() error {
	return s.tx.Commit()
}

func (s *singleton) Rollback() error {
	return s.tx.Rollback()
}
