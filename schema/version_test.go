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

package schema

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/schema/cassandra"
	"github.com/uber/cadence/schema/mysql"
	"github.com/uber/cadence/schema/postgres"
	"github.com/uber/cadence/schema/sqlite"
)

// TestSchemas verifies that the schema version constants
// match the latest versioned directory for each database type.
// This prevents regressions where new migration directories are added but the
// version constants are not updated.
func TestSchema(t *testing.T) {
	tests := []struct {
		name            string
		schema          persistence.Schema
		declaredVersion string
	}{
		// Cassandra
		{
			name:            "cassandra_main",
			schema:          cassandra.DefaultSchema,
			declaredVersion: cassandra.Version,
		},
		{
			name:            "cassandra_visibility",
			schema:          cassandra.VisibilitySchema,
			declaredVersion: cassandra.VisibilityVersion,
		},
		// MySQL
		{
			name:            "mysql_main",
			schema:          mysql.DefaultSchema,
			declaredVersion: mysql.Version,
		},
		{
			name:            "mysql_visibility",
			schema:          mysql.VisibilitySchema,
			declaredVersion: mysql.VisibilityVersion,
		},
		// Postgres
		{
			name:            "postgres_main",
			schema:          postgres.DefaultSchema,
			declaredVersion: postgres.Version,
		},
		{
			name:            "postgres_visibility",
			schema:          postgres.VisibilitySchema,
			declaredVersion: postgres.VisibilityVersion,
		},
		// SQLite
		{
			name:            "sqlite_main",
			schema:          sqlite.DefaultSchema,
			declaredVersion: sqlite.Version,
		},
		{
			name:            "sqlite_visibility",
			schema:          sqlite.VisibilitySchema,
			declaredVersion: sqlite.VisibilityVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, err := tt.schema.AllUpdates()
			if err != nil {
				t.Fatalf("Failed to read updates: %v", err)
			}
			t.Run("correct latest version", func(t *testing.T) {
				declaredVersion, err := persistence.ParseVersion(tt.declaredVersion)
				if err != nil {
					t.Fatalf("Failed to parse declared version: %v", err)
				}
				lastUpdate := updates[len(updates)-1]

				if lastUpdate.Version != declaredVersion {

					t.Errorf(
						"%s schema version mismatch:\n"+
							"  Declared version in version.go: %s\n"+
							"  Latest versioned directory:     v%s\n"+
							"  Please update the constant  to \"%s\"",
						tt.name,
						tt.declaredVersion,
						lastUpdate.Version.String(),
						lastUpdate.Version.String(),
					)
				}
			})
			// Forbid schemas from incrementing the major/minor version by more than 1
			t.Run("all versions sequential", func(t *testing.T) {
				var previous *persistence.SchemaUpdate
				for _, update := range updates {
					if previous != nil {
						oldVersion := previous.Version
						newVersion := update.Version
						majorIncrement := newVersion.Major == oldVersion.Major+1
						minorIncrement := newVersion.Minor == oldVersion.Minor+1
						// elusive exclusive or
						if !(majorIncrement || minorIncrement) || (majorIncrement && minorIncrement) {
							t.Fatalf("Invalid version increment from %s to %s: must increment either major or minor version by 1, but not both", oldVersion.String(), newVersion.String())
						}
					}
					previous = update
				}
			})
			t.Run("correct first version", func(t *testing.T) {
				first := updates[0]
				assert.Equal(t, "0.1", first.Version.String(), "first version should be 0.1")
			})
			t.Run("found all versions", func(t *testing.T) {
				// Since they can only be incremented by 1, the expected number of versions is equal to the sum of the minor and major
				expected := tt.schema.LatestVersion().Minor + tt.schema.LatestVersion().Major
				assert.Equal(t, expected, len(updates), "unexpected number of updates")
			})
			t.Run("minCompatibleVersion is non-decreasing", func(t *testing.T) {
				for i := 1; i < len(updates); i++ {
					prev := updates[i-1]
					curr := updates[i]
					// MinCompatibleVersion should never decrease across sequential updates
					if curr.MinCompatibleVersion.Compare(prev.MinCompatibleVersion) < 0 {
						t.Errorf(
							"MinCompatibleVersion decreased from %s to %s (version %s to %s)\n"+
								"  Previous update: v%s has MinCompatibleVersion=%s\n"+
								"  Current update:  v%s has MinCompatibleVersion=%s\n"+
								"  MinCompatibleVersion must be non-decreasing across schema updates",
							prev.MinCompatibleVersion.String(),
							curr.MinCompatibleVersion.String(),
							prev.Version.String(),
							curr.Version.String(),
							prev.Version.String(),
							prev.MinCompatibleVersion.String(),
							curr.Version.String(),
							curr.MinCompatibleVersion.String(),
						)
					}
				}
			})
			// Validate each version as a test case
			for _, update := range updates {
				t.Run(update.Version.String(), func(t *testing.T) {
					assert.NotEmptyf(t, update.Description, "description is required")
					assert.NotEmptyf(t, update.ManifestMD5, "manifest hash is required")
					assert.True(t, update.Version.Compare(update.MinCompatibleVersion) >= 0, "version must be greater than or equal to min compatible version")
					assert.NotEmptyf(t, update.DDLStatements, "missing DDL statements")
				})
			}
		})
	}
}

// TestVersionComparison verifies that the version comparison logic works correctly
// for edge cases like comparing 0.9 and 0.10.
func TestVersionComparison(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		expected string
	}{
		{
			name:     "simple_sequential",
			versions: []string{"0.1", "0.2", "0.3"},
			expected: "0.3",
		},
		{
			name:     "double_digit",
			versions: []string{"0.1", "0.9", "0.10"},
			expected: "0.10",
		},
		{
			name:     "cassandra_style",
			versions: []string{"0.1", "0.10", "0.49"},
			expected: "0.49",
		},
		{
			name:     "unordered",
			versions: []string{"0.5", "0.1", "0.10", "0.2"},
			expected: "0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var versions []*version.Version
			for _, v := range tt.versions {
				parsed, err := version.NewVersion(v)
				if err != nil {
					t.Fatalf("Failed to parse version %s: %v", v, err)
				}
				versions = append(versions, parsed)
			}

			latest := versions[0]
			for _, v := range versions[1:] {
				if v.GreaterThan(latest) {
					latest = v
				}
			}

			if latest.Original() != tt.expected {
				t.Errorf("Expected latest version %s, got %s", tt.expected, latest.Original())
			}
		})
	}
}
