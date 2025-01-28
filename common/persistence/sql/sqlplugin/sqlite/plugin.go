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
	"bytes"
	"fmt"
	"strings"

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
	db, err := sqlx.Connect("sqlite3", buildDSN(cfg))
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
func buildDSN(cfg *config.SQL) string {

	var dsn = ":memory:"

	// if database name is provided, then sqlite will create a file with the name
	if cfg.DatabaseName != "" {
		dsn = fmt.Sprintf("file:%s", cfg.DatabaseName)

	}

	if dsnAttrs := buildDSNAttrs(cfg); dsnAttrs != "" {
		dsn += "?" + dsnAttrs
	}

	return dsn
}

const (
	// if journal mode is not provided, we set it to WAL by default
	// WAL mode allows readers and writers from different processes
	// to access the database concurrently by default
	// https://www.sqlite.org/wal.html
	journalModeAttrName      = "_journal_mode"
	journalModeAttrShortName = "_journal"
	journalModeDefaultValue  = "WAL"
)

var (
	journalModeAttrNames = []string{journalModeAttrName, journalModeAttrShortName}
)

// buildDSNAttrs builds the data source name attributes for sqlite from config.SQL
// available attributes can be found here
// https://github.com/mattn/go-sqlite3?tab=readme-ov-file#connection-string
func buildDSNAttrs(cfg *config.SQL) string {

	sanitizedAttrs := sanitizeDSNAttrs(cfg.ConnectAttributes)

	if cfg.DatabaseName != "" {
		if !hasAttr(sanitizedAttrs, journalModeAttrNames...) {
			sanitizedAttrs[journalModeAttrName] = journalModeDefaultValue
		}
	}

	return joinDSNAttrs(sanitizedAttrs)
}

// hasAttr checks if the attributes map has any of the keys
func hasAttr(attrs map[string]string, keys ...string) bool {
	for key := range attrs {
		for _, k := range keys {
			if key == k {
				return true
			}
		}
	}
	return false
}

// sanitizeDSNAttrs sanitizes the attributes by trimming the keys and values
func sanitizeDSNAttrs(attrs map[string]string) map[string]string {
	sanitized := make(map[string]string, len(attrs))

	for k, v := range attrs {
		k, v = sanitizeDSNAttrElem(k), sanitizeDSNAttrElem(v)
		sanitized[k] = v
	}

	return sanitized
}

// sanitizeDSNAttrElem trims the value, lowercases it
func sanitizeDSNAttrElem(v string) string {
	return strings.TrimSpace(strings.ToLower(v))
}

// joinDSNAttrs joins the attributes into a single string
// with key=value pairs separated by & and escaped
func joinDSNAttrs(attrs map[string]string) string {
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
	return buf.String()
}
