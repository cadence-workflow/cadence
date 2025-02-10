// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package sqlite

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/mysql"
)

var (
	_ sqlplugin.AdminDB = (*DB)(nil)
	_ sqlplugin.DB      = (*DB)(nil)
	_ sqlplugin.Tx      = (*DB)(nil)
)

// DB contains methods for managing objects in a sqlite database
// It inherits methods from the mysql.DB to reuse the implementation of the methods
// sqlplugin.ErrorChecker is customized for sqlite
type DB struct {
	*mysql.DB
}

// NewDB returns an instance of DB, which contains a new created mysql.DB with sqlite specific methods
func NewDB(xdbs []*sqlx.DB, tx *sqlx.Tx, dbShardID int, numDBShards int) (*DB, error) {
	mysqlDB, err := mysql.NewDB(xdbs, tx, dbShardID, numDBShards, newConverter())
	if err != nil {
		return nil, err
	}
	return &DB{DB: mysqlDB}, nil
}

// PluginName returns the name of the plugin
func (mdb *DB) PluginName() string {
	return PluginName
}

// BeginTX starts a new transaction and returns a new Tx
func (mdb *DB) BeginTX(ctx context.Context, dbShardID int) (sqlplugin.Tx, error) {
	mysqlTX, err := mdb.DB.BeginTx(ctx, dbShardID)
	if err != nil {
		return nil, err
	}

	return &DB{DB: mysqlTX.(*mysql.DB)}, nil
}
