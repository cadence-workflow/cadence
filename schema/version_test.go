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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"

	"github.com/uber/cadence/schema/cassandra"
	"github.com/uber/cadence/schema/mysql"
	"github.com/uber/cadence/schema/postgres"
	"github.com/uber/cadence/schema/sqlite"
)

// TestSchemaVersionsMatchDirectories verifies that the schema version constants
// match the latest versioned directory for each database type.
// This prevents regressions where new migration directories are added but the
// version constants are not updated.
func TestSchemaVersionsMatchDirectories(t *testing.T) {
	tests := []struct {
		name             string
		schemaDir        string
		versionedSubPath string // path from schemaDir to versioned directory
		declaredVersion  string
		versionType      string // "main" or "visibility"
	}{
		// Cassandra
		{
			name:             "cassandra_main",
			schemaDir:        "cassandra",
			versionedSubPath: "cadence/versioned",
			declaredVersion:  cassandra.Version,
			versionType:      "main",
		},
		{
			name:             "cassandra_visibility",
			schemaDir:        "cassandra",
			versionedSubPath: "visibility/versioned",
			declaredVersion:  cassandra.VisibilityVersion,
			versionType:      "visibility",
		},
		// MySQL
		{
			name:             "mysql_main",
			schemaDir:        "mysql",
			versionedSubPath: "v8/cadence/versioned",
			declaredVersion:  mysql.Version,
			versionType:      "main",
		},
		{
			name:             "mysql_visibility",
			schemaDir:        "mysql",
			versionedSubPath: "v8/visibility/versioned",
			declaredVersion:  mysql.VisibilityVersion,
			versionType:      "visibility",
		},
		// Postgres
		{
			name:             "postgres_main",
			schemaDir:        "postgres",
			versionedSubPath: "cadence/versioned",
			declaredVersion:  postgres.Version,
			versionType:      "main",
		},
		{
			name:             "postgres_visibility",
			schemaDir:        "postgres",
			versionedSubPath: "visibility/versioned",
			declaredVersion:  postgres.VisibilityVersion,
			versionType:      "visibility",
		},
		// SQLite
		{
			name:             "sqlite_main",
			schemaDir:        "sqlite",
			versionedSubPath: "cadence/versioned",
			declaredVersion:  sqlite.Version,
			versionType:      "main",
		},
		{
			name:             "sqlite_visibility",
			schemaDir:        "sqlite",
			versionedSubPath: "visibility/versioned",
			declaredVersion:  sqlite.VisibilityVersion,
			versionType:      "visibility",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionedPath := filepath.Join(tt.schemaDir, tt.versionedSubPath)
			latestVersion, err := getLatestVersionFromDirectory(versionedPath)
			if err != nil {
				t.Fatalf("Failed to get latest version from %s: %v", versionedPath, err)
			}

			if latestVersion != tt.declaredVersion {
				// Map versionType to actual constant name
				constantName := "Version"
				if tt.versionType == "visibility" {
					constantName = "VisibilityVersion"
				}

				t.Errorf(
					"%s %s schema version mismatch:\n"+
						"  Declared version in version.go: %s\n"+
						"  Latest versioned directory:     v%s\n"+
						"  Please update the %s constant in schema/%s/version.go to \"%s\"",
					tt.schemaDir,
					tt.versionType,
					tt.declaredVersion,
					latestVersion,
					constantName,
					tt.schemaDir,
					latestVersion,
				)
			}
		})
	}
}

// getLatestVersionFromDirectory scans a versioned directory and returns the highest version number.
// It expects directories named like "v0.1", "v0.2", etc.
func getLatestVersionFromDirectory(versionedPath string) (string, error) {
	entries, err := os.ReadDir(versionedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", versionedPath, err)
	}

	var versions []*version.Version
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "v") {
			continue
		}
		// Extract version number (e.g., "v0.9" -> "0.9")
		versionStr := strings.TrimPrefix(name, "v")
		v, err := version.NewVersion(versionStr)
		if err != nil {
			// Skip directories that don't follow semantic versioning
			continue
		}
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versioned directories found in %s", versionedPath)
	}

	// Find the highest version
	latest := versions[0]
	for _, v := range versions[1:] {
		if v.GreaterThan(latest) {
			latest = v
		}
	}

	return latest.Original(), nil
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
