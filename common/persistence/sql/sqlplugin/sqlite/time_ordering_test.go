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
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestTimeOrderingWithFixedWidthFormat(t *testing.T) {
	dsn := buildDSN(&config.SQL{})

	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_timers (
		id INTEGER PRIMARY KEY,
		visibility_timestamp DATETIME NOT NULL
	)`)
	require.NoError(t, err)

	tests := []struct {
		name     string
		stored   time.Time
		lower    time.Time
		upper    time.Time
		wantRows int
	}{
		{
			name:     "trailing zero boundary includes later time",
			stored:   time.Date(2026, 8, 4, 21, 37, 43, 735000000, time.UTC),
			lower:    time.Date(2026, 8, 4, 21, 37, 43, 700000000, time.UTC),
			upper:    time.Date(2026, 8, 4, 21, 37, 44, 736000000, time.UTC),
			wantRows: 1,
		},
		{
			name:     "trailing zero boundary excludes earlier time",
			stored:   time.Date(2026, 8, 4, 21, 37, 43, 600000000, time.UTC),
			lower:    time.Date(2026, 8, 4, 21, 37, 43, 700000000, time.UTC),
			upper:    time.Date(2026, 8, 4, 21, 37, 44, 736000000, time.UTC),
			wantRows: 0,
		},
		{
			name:     "sub-millisecond precision preserved",
			stored:   time.Date(2026, 8, 4, 21, 37, 48, 968000000, time.UTC),
			lower:    time.Date(2026, 8, 4, 21, 37, 48, 960000000, time.UTC),
			upper:    time.Date(2026, 8, 4, 21, 37, 49, 0, time.UTC),
			wantRows: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.Exec("DELETE FROM test_timers")
			require.NoError(t, err)

			_, err = db.Exec("INSERT INTO test_timers (id, visibility_timestamp) VALUES (1, ?)", tt.stored)
			require.NoError(t, err)

			var count int
			err = db.QueryRow(
				"SELECT COUNT(*) FROM test_timers WHERE visibility_timestamp >= ? AND visibility_timestamp < ?",
				tt.lower, tt.upper,
			).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRows, count)
		})
	}
}

func TestTimeRoundTrip(t *testing.T) {
	dsn := buildDSN(&config.SQL{})

	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_time_rt (
		id INTEGER PRIMARY KEY,
		ts DATETIME NOT NULL
	)`)
	require.NoError(t, err)

	original := time.Date(2026, 8, 4, 21, 37, 43, 735000000, time.UTC)
	_, err = db.Exec("INSERT INTO test_time_rt (id, ts) VALUES (1, ?)", original)
	require.NoError(t, err)

	var retrieved time.Time
	err = db.QueryRow("SELECT ts FROM test_time_rt WHERE id = 1").Scan(&retrieved)
	require.NoError(t, err)
	assert.True(t, original.Equal(retrieved), "expected %v, got %v", original, retrieved)
}
