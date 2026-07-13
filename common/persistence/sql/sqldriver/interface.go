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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/common/persistence/sql/sqldriver

package sqldriver

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type (
	// Driver interface is an abstraction to query SQL.
	// The layer is added so that we can have a adapter to support multiple SQL databases behind a single Cadence cluster
	Driver interface {

		// shared methods are for both non-transactional (using sqlx.DB) and transactional (using sqlx.Tx) operation --
		// if a transaction is started(using BeginTxx), then query are executed in the transaction mode. Otherwise executed in normal mode.
		commonOfDbAndTx

		// BeginTxx starts a new transaction in the shard of dbShardID
		BeginTxx(ctx context.Context, dbShardID int, opts *sql.TxOptions) (*sqlx.Tx, error)
		// Commit commits the current transaction(started by BeginTxx)
		Commit() error
		// Rollback rollbacks the current transaction(started by BeginTxx)
		Rollback() error
		// Close closes this driver(and underlying connections)
		Close() error

		// ExecDDL executes a DDL query
		ExecDDL(ctx context.Context, dbShardID int, query string, args ...interface{}) (sql.Result, error)
		// SelectForSchemaQuery executes a select query for schema(returning multiple rows).
		SelectForSchemaQuery(dbShardID int, dest interface{}, query string, args ...interface{}) error
		// GetForSchemaQuery executes a get query for schema(returning single row).
		GetForSchemaQuery(dbShardID int, dest interface{}, query string, args ...interface{}) error
	}

	// the methods can be executed from either a started or transaction(then need to call Commit/Rollback), or without a transaction
	commonOfDbAndTx interface {
		// ExecContext executes a query that doesn't return rows.
		// For example, INSERT, DELETE or UPDATE.
		ExecContext(ctx context.Context, dbShardID int, query string, args ...interface{}) (sql.Result, error)

		// NamedExecContext works same as ExecContext but it binds named query parameters like (:field_name).
		NamedExecContext(ctx context.Context, dbShardID int, query string, arg interface{}) (sql.Result, error)

		// GetContext fetches exactly one row into dest.
		// dest parameter must be a pointer to a struct.
		GetContext(ctx context.Context, dbShardID int, dest interface{}, query string, args ...interface{}) error

		// SelectContext fetches all matching rows into dest.
		// dest parameter must be a pointer to a slice.
		SelectContext(ctx context.Context, dbShardID int, dest interface{}, query string, args ...interface{}) error
	}
)
