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
	"github.com/iancoleman/strcase"
	"github.com/jmoiron/sqlx"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	PluginName = "sqlite"
)

// SQLite plugin provides an sql persistence storage implementation for sqlite database
// Mostly the implementation reuses the mysql implementation
// The plugin supports only in-memory sqlite database for now
type plugin struct{}

var _ sqlplugin.Plugin = (*plugin)(nil)

// CreateDB wraps createDB to return an instance of sqlplugin.DB
func (p *plugin) CreateDB(cfg *config.SQL) (sqlplugin.DB, error) {
	return p.createDB(cfg)
}

// CreateAdminDB wraps createDB to return an instance of sqlplugin.AdminDB
func (p *plugin) CreateAdminDB(cfg *config.SQL) (sqlplugin.AdminDB, error) {
	return p.createDB(cfg)
}

// createDB create a new instance of DB
func (p *plugin) createDB(cfg *config.SQL) (*DB, error) {
	conns, err := sqldriver.CreateDBConnections(cfg, func(cfg *config.SQL) (*sqlx.DB, error) {
		return p.createSingleDBConn(cfg)
	})
	if err != nil {
		return nil, err
	}
	return NewDB(conns, nil, sqlplugin.DbShardUndefined, cfg.NumShards)
}

// createSingleDBConn creates a single database connection for sqlite
func (p *plugin) createSingleDBConn(cfg *config.SQL) (*sqlx.DB, error) {
	db, err := sqlx.Connect(PluginName, buildDSN(cfg))
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

// buildDSN builds the data source name for sqlite from config.SQL
func buildDSN(_ *config.SQL) string {
	// only in-memory sqlite is supported for now
	// permanent sqlite file will be supported in the future
	return ":memory:"
}
