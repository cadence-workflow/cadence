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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
)

func TestPlugin_CreateDB(t *testing.T) {
	p := &plugin{}
	db, err := p.CreateDB(&config.SQL{})

	assert.NoError(t, err)
	assert.NotNil(t, db)
}

func TestPlugin_CreateAdminDB(t *testing.T) {
	p := &plugin{}
	db, err := p.CreateAdminDB(&config.SQL{})

	assert.NoError(t, err)
	assert.NotNil(t, db)
}

func Test_buildDSN(t *testing.T) {
	for name, c := range map[string]struct {
		cfg  *config.SQL
		want string
	}{
		"empty": {
			cfg:  &config.SQL{},
			want: ":memory:",
		},
		"database name only": {
			cfg: &config.SQL{
				DatabaseName: "cadence.db",
			},
			want: "file:cadence.db",
		},
	} {
		t.Run(name, func(t *testing.T) {
			dsn := buildDSN(c.cfg)
			assert.Equal(t, c.want, dsn)
		})
	}
}

func Test_buildDSN_attrs(t *testing.T) {
	for name, c := range map[string]struct {
		cfg *config.SQL

		// wantOneOf contains all possible DSN strings that can be generated because
		// the order of the attributes in the DSN string is not guaranteed
		wantOneOf []string
	}{
		"only connection attrs": {
			cfg: &config.SQL{
				ConnectAttributes: map[string]string{
					"_busy_timeout": "10000",
					"_FK":           "true",
				},
			},
			wantOneOf: []string{
				":memory:?_busy_timeout=10000&_fk=true",
				":memory:?_fk=true&_busy_timeout=10000",
			},
		},
		"database name and connection attrs": {
			cfg: &config.SQL{
				DatabaseName: "cadence.db",
				ConnectAttributes: map[string]string{
					"cache":   "PRIVATE",
					"cache1 ": "NONe",
				},
			},
			wantOneOf: []string{
				"file:cadence.db?cache=private&cache1=none",
				"file:cadence.db?cache1=none&cache=private",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			dsn := buildDSN(c.cfg)
			assert.Contains(t, c.wantOneOf, dsn)
		})
	}
}
