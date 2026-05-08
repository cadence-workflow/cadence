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

package sql

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/ncruces/go-sqlite3"

	"github.com/uber/cadence/common/config"
	sqlite_db "github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	sqlite_schema "github.com/uber/cadence/schema/sqlite"
	"github.com/uber/cadence/tools/common/schema"
)

// autoSetupState tracks per-database setup state so concurrent service goroutines
// don't race to initialize the same SQLite database.
var autoSetupState = struct {
	mu   sync.Mutex
	byDB map[string]*dbSetupOnce
}{byDB: make(map[string]*dbSetupOnce)}

type dbSetupOnce struct {
	once sync.Once
	err  error
}

// MaybeAutoSetupSQLiteSchema runs schema setup and migration for any SQLite datastores
// that have AutoSetup enabled. It is a no-op for non-SQLite stores and for SQLite
// stores without AutoSetup. Intended to be called before schema version verification
// on server startup.
func MaybeAutoSetupSQLiteSchema(cfg config.Persistence) error {
	type storeTarget struct {
		storeName    string
		schemaSubdir string
	}
	targets := []storeTarget{
		{cfg.DefaultStore, "cadence/versioned"},
		{cfg.VisibilityStore, "visibility/versioned"},
	}
	for _, t := range targets {
		ds, ok := cfg.DataStores[t.storeName]
		if !ok || ds.SQL == nil {
			continue
		}
		if ds.SQL.PluginName != sqlite_db.PluginName || !ds.SQL.AutoSetup {
			continue
		}
		if err := doSQLiteAutoSetupOnce(*ds.SQL, t.schemaSubdir); err != nil {
			return fmt.Errorf("auto-setup SQLite store %q: %w", t.storeName, err)
		}
	}
	return nil
}

// doSQLiteAutoSetupOnce ensures setup runs exactly once per database file across
// all concurrent service goroutines, and propagates any error to all of them.
func doSQLiteAutoSetupOnce(cfg config.SQL, schemaSubdir string) error {
	autoSetupState.mu.Lock()
	state, ok := autoSetupState.byDB[cfg.DatabaseName]
	if !ok {
		state = &dbSetupOnce{}
		autoSetupState.byDB[cfg.DatabaseName] = state
	}
	autoSetupState.mu.Unlock()

	state.once.Do(func() {
		state.err = doSQLiteAutoSetup(cfg, schemaSubdir)
	})
	return state.err
}

func doSQLiteAutoSetup(cfg config.SQL, schemaSubdir string) error {
	conn, err := NewConnection(&cfg)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	// If the schema version table doesn't exist yet, run initial setup.
	// Only treat a SQLite "no such table" error as a fresh database; any other
	// error (disk full, file locked, corrupt DB) is propagated as-is.
	if _, err := conn.ReadSchemaVersion(); err != nil {
		if !isSQLiteNoSuchTableError(err) {
			return fmt.Errorf("read schema version: %w", err)
		}
		if err2 := schema.SetupFromConfig(&schema.SetupConfig{
			InitialVersion: "0.0",
		}, conn); err2 != nil {
			return fmt.Errorf("schema setup: %w", err2)
		}
	}

	// Apply any outstanding migrations (idempotent: skips already-applied versions).
	subFS, err := fs.Sub(sqlite_schema.SchemaFS, schemaSubdir)
	if err != nil {
		return fmt.Errorf("schema subdir %q: %w", schemaSubdir, err)
	}
	return schema.UpdateFromConfig(&schema.UpdateConfig{
		SchemaFS: subFS,
	}, conn)
}

// isSQLiteNoSuchTableError reports whether err is a SQLite "no such table" error,
// which means the schema version table has not been created yet (fresh database).
// It returns false for all other errors (disk full, file locked, corrupt DB, etc.)
// so callers can propagate them instead of treating them as a fresh-database signal.
//
// SQLite's C API does not provide a specific extended error code for "no such table"
// — it falls under the generic ERROR (code 1) alongside every other SQL logic error.
// Checking the error message is the only reliable way to distinguish this case.
func isSQLiteNoSuchTableError(err error) bool {
	var sqlErr *sqlite3.Error
	if !errors.As(err, &sqlErr) {
		return false
	}
	// Confirm it is a generic SQL logic error (not a timeout, I/O error, etc.)
	// before inspecting the message.
	return sqlErr.Code() == sqlite3.ERROR && strings.Contains(sqlErr.Error(), "no such table")
}
