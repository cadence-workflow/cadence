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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"
	sqlite_db "github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	sqlite_schema "github.com/uber/cadence/schema/sqlite"
)

func sqliteTestCfg(dbPath string) config.SQL {
	return config.SQL{
		PluginName:   sqlite_db.PluginName,
		DatabaseName: dbPath,
		MaxConns:     1,
		MaxIdleConns: 1,
		AutoSetup:    true,
	}
}

func TestDoSQLiteAutoSetup_FreshDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := sqliteTestCfg(filepath.Join(tmpDir, "cadence.db"))
	require.NoError(t, doSQLiteAutoSetup(cfg, "cadence/versioned"))

	conn, err := NewConnection(&cfg)
	require.NoError(t, err)
	defer conn.Close()

	version, err := conn.ReadSchemaVersion()
	require.NoError(t, err)
	assert.Equal(t, sqlite_schema.Version, version)
}

func TestDoSQLiteAutoSetup_FreshVisibilityDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := sqliteTestCfg(filepath.Join(tmpDir, "cadence_visibility.db"))
	require.NoError(t, doSQLiteAutoSetup(cfg, "visibility/versioned"))

	conn, err := NewConnection(&cfg)
	require.NoError(t, err)
	defer conn.Close()

	version, err := conn.ReadSchemaVersion()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, version, sqlite_schema.VisibilityVersion)
}

func TestDoSQLiteAutoSetup_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := sqliteTestCfg(filepath.Join(tmpDir, "cadence.db"))

	require.NoError(t, doSQLiteAutoSetup(cfg, "cadence/versioned"))
	// Second call on an already-migrated database must not return an error.
	require.NoError(t, doSQLiteAutoSetup(cfg, "cadence/versioned"))
}

func TestMaybeAutoSetupSQLiteSchema_BothStores(t *testing.T) {
	tmpDir := t.TempDir()
	defaultDB := filepath.Join(tmpDir, "cadence.db")
	visibilityDB := filepath.Join(tmpDir, "cadence_visibility.db")

	cfg := config.Persistence{
		DefaultStore:    "default",
		VisibilityStore: "visibility",
		DataStores: map[string]config.DataStore{
			"default": {SQL: &config.SQL{
				PluginName:   sqlite_db.PluginName,
				DatabaseName: defaultDB,
				MaxConns:     1,
				MaxIdleConns: 1,
				AutoSetup:    true,
			}},
			"visibility": {SQL: &config.SQL{
				PluginName:   sqlite_db.PluginName,
				DatabaseName: visibilityDB,
				MaxConns:     1,
				MaxIdleConns: 1,
				AutoSetup:    true,
			}},
		},
	}

	require.NoError(t, MaybeAutoSetupSQLiteSchema(cfg))

	for storeName, wantVersion := range map[string]string{
		"default":    sqlite_schema.Version,
		"visibility": sqlite_schema.VisibilityVersion,
	} {
		sqlCfg := cfg.DataStores[storeName].SQL
		conn, err := NewConnection(sqlCfg)
		require.NoError(t, err, "store %q", storeName)

		version, err := conn.ReadSchemaVersion()
		conn.Close()
		require.NoError(t, err, "store %q", storeName)
		assert.Equal(t, wantVersion, version, "store %q", storeName)
	}
}

func TestMaybeAutoSetupSQLiteSchema_SkipsNonSQLitePlugin(t *testing.T) {
	// Non-SQLite stores must be silently skipped (no connection attempted).
	cfg := config.Persistence{
		DefaultStore: "default",
		DataStores: map[string]config.DataStore{
			"default": {SQL: &config.SQL{
				PluginName: "mysql",
				AutoSetup:  true,
			}},
		},
	}
	require.NoError(t, MaybeAutoSetupSQLiteSchema(cfg))
}

func TestMaybeAutoSetupSQLiteSchema_SkipsWhenAutoSetupDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Persistence{
		DefaultStore: "default",
		DataStores: map[string]config.DataStore{
			"default": {SQL: &config.SQL{
				PluginName:   sqlite_db.PluginName,
				DatabaseName: filepath.Join(tmpDir, "cadence.db"),
				MaxConns:     1,
				MaxIdleConns: 1,
				AutoSetup:    false,
			}},
		},
	}
	require.NoError(t, MaybeAutoSetupSQLiteSchema(cfg))

	// Schema version table must not exist because setup was skipped.
	conn, err := NewConnection(cfg.DataStores["default"].SQL)
	require.NoError(t, err)

	_, err = conn.ReadSchemaVersion()
	conn.Close()
	require.Error(t, err)
	assert.True(t, isSQLiteNoSuchTableError(err))
}

func TestIsSQLiteNoSuchTableError_FreshDB(t *testing.T) {
	// An in-memory database has no tables; ReadSchemaVersion must produce a
	// "no such table" error that isSQLiteNoSuchTableError recognises.
	conn, err := NewConnection(&config.SQL{
		PluginName: sqlite_db.PluginName,
		// Empty DatabaseName → in-memory SQLite.
	})
	require.NoError(t, err)
	defer conn.Close()

	_, readErr := conn.ReadSchemaVersion()
	require.Error(t, readErr)
	assert.True(t, isSQLiteNoSuchTableError(readErr))
}

func TestIsSQLiteNoSuchTableError_OtherErrors(t *testing.T) {
	assert.False(t, isSQLiteNoSuchTableError(errors.New("disk full")))
	assert.False(t, isSQLiteNoSuchTableError(nil))
}
