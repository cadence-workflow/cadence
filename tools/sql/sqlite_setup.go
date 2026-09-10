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

// autoSetup runs at most once per process; err is retained for later callers.
var autoSetup struct {
	once sync.Once
	err  error
}

// MaybeAutoSetupSQLiteSchema runs schema setup and migration for any SQLite datastores
// that have AutoSetup enabled. It is a no-op for non-SQLite stores and for SQLite
// stores without AutoSetup.
// Called from each service's schema verification; only the first call runs setup.
func MaybeAutoSetupSQLiteSchema(cfg config.Persistence) error {
	autoSetup.once.Do(func() {
		autoSetup.err = autoSetupSQLiteSchema(cfg)
	})
	return autoSetup.err
}

func autoSetupSQLiteSchema(cfg config.Persistence) error {
	datastores := []struct{ storeName, schemaSubdir string }{
		{cfg.DefaultStore, "cadence/versioned"},
		{cfg.VisibilityStore, "visibility/versioned"},
	}

	for _, datastore := range datastores {
		ds, ok := cfg.DataStores[datastore.storeName]
		if !ok || ds.SQL == nil {
			continue
		}
		if ds.SQL.PluginName != sqlite_db.PluginName || !ds.SQL.AutoSetup {
			continue
		}
		if err := doSQLiteAutoSetup(*ds.SQL, datastore.schemaSubdir); err != nil {
			return fmt.Errorf("auto-setup SQLite store %q: %w", datastore.storeName, err)
		}
	}
	return nil
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
func isSQLiteNoSuchTableError(err error) bool {
	var sqlErr *sqlite3.Error
	if !errors.As(err, &sqlErr) {
		return false
	}
	return sqlErr.Code() == sqlite3.ERROR && strings.Contains(sqlErr.Error(), "no such table")
}
