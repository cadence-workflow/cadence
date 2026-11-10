package sqlite

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"

	_ "github.com/ncruces/go-sqlite3/driver" // register sqlite3 driver for tests
	_ "github.com/ncruces/go-sqlite3/embed"  // embed sqlite db for tests
)

func TestIsSchemaElementExistsError(t *testing.T) {
	tests := []struct {
		name     string
		setup    []string
		stmt     string
		expected bool
	}{
		{
			name:     "duplicate table",
			setup:    []string{"CREATE TABLE t (id INT PRIMARY KEY);"},
			stmt:     "CREATE TABLE t (id INT PRIMARY KEY);",
			expected: true,
		},
		{
			name:     "duplicate column",
			setup:    []string{"CREATE TABLE t (id INT PRIMARY KEY, c INT);"},
			stmt:     "ALTER TABLE t ADD c INT;",
			expected: true,
		},
		{
			name:     "duplicate index",
			setup:    []string{"CREATE TABLE t (id INT PRIMARY KEY, c INT);", "CREATE INDEX t_c ON t (c);"},
			stmt:     "CREATE INDEX t_c ON t (c);",
			expected: true,
		},
		{
			name:     "missing table",
			stmt:     "ALTER TABLE missing ADD c INT;",
			expected: false,
		},
		{
			name:     "duplicate row",
			setup:    []string{"CREATE TABLE t (id INT PRIMARY KEY);", "INSERT INTO t (id) VALUES (1);"},
			stmt:     "INSERT INTO t (id) VALUES (1);",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &plugin{}
			db, err := p.CreateAdminDB(&config.SQL{DatabaseName: path.Join(os.TempDir(), uuid.New().String())})
			require.NoError(t, err)
			defer db.Close()

			for _, stmt := range tt.setup {
				require.NoError(t, db.ExecSchemaOperationQuery(t.Context(), stmt))
			}
			err = db.ExecSchemaOperationQuery(t.Context(), tt.stmt)
			require.Error(t, err)
			assert.Equal(t, tt.expected, db.IsSchemaElementExistsError(err), "error: %v", err)
		})
	}
}

func TestIsSchemaElementExistsError_NonSQLiteError(t *testing.T) {
	db := &DB{}
	assert.False(t, db.IsSchemaElementExistsError(nil))
	assert.False(t, db.IsSchemaElementExistsError(errors.New("table t already exists")))
}
