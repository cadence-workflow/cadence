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
	"os"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
)

func TestPlugin_CreateDB(t *testing.T) {
	for name, cfg := range map[string]*config.SQL{
		"in-memory": {},
		"temp file": {DatabaseName: path.Join(os.TempDir(), uuid.New().String())},
	} {
		t.Run(name, func(t *testing.T) {
			p := &plugin{}
			db, err := p.CreateDB(cfg)

			assert.NoError(t, err)
			assert.NotNil(t, db)
		})
	}
}

func TestPlugin_CreateAdminDB(t *testing.T) {
	for name, cfg := range map[string]*config.SQL{
		"in-memory": {},
		"temp file": {DatabaseName: path.Join(os.TempDir(), uuid.New().String())},
	} {
		t.Run(name, func(t *testing.T) {
			p := &plugin{}
			db, err := p.CreateAdminDB(cfg)

			assert.NoError(t, err)
			assert.NotNil(t, db)
		})
	}
}
