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

package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

type fakePlugin struct{}

var _ sqlplugin.Plugin = (*fakePlugin)(nil)

func (f *fakePlugin) CreateDB(cfg *config.SQL) (sqlplugin.DB, error) {
	return nil, nil
}

func (f *fakePlugin) CreateAdminDB(cfg *config.SQL) (sqlplugin.AdminDB, error) {
	return nil, nil
}

func (f *fakePlugin) GetSchema(dbType persistence.DBType) (persistence.Schema, error) {
	return nil, nil
}

func TestPluginRegistration(t *testing.T) {
	assert.NotPanics(t, func() {
		RegisterPlugin("fake", &fakePlugin{})
	}, "RegisterPlugin failed to register a plugin")

	assert.Panics(t, func() {
		RegisterPlugin("fake", &fakePlugin{})
	}, "RegisterPlugin failed to panic when registering a plugin with the same name")

	assert.NotPanics(t, func() {
		RegisterPluginIfNotExists("fake", &fakePlugin{})
	}, "RegisterPluginIfNotExists failed to register a plugin")

	assert.NotPanics(t, func() {
		RegisterPluginIfNotExists("fake2", &fakePlugin{})
	}, "RegisterPluginIfNotExists failed to register a plugin")

	assert.True(t, PluginRegistered("fake"), "PluginRegistered failed to return true for a registered plugin")
	assert.True(t, PluginRegistered("fake2"), "PluginRegistered failed to return true for a registered plugin")
	assert.False(t, PluginRegistered("fake3"), "PluginRegistered failed to return false for an unregistered plugin")

	assert.Equal(t, []string{"fake", "fake2", "shared"}, GetRegisteredPluginNames(), "GetRegisteredPluginNames failed to return the correct list of registered plugins")

	_, err := NewSQLDB(&config.SQL{PluginName: "fake"})
	assert.NoError(t, err, "NewSQLDB failed to create a DB with a registered plugin")
	_, err = NewSQLDB(&config.SQL{PluginName: "fake2"})
	assert.NoError(t, err, "NewSQLDB failed to create a DB with a registered plugin")

	_, err = NewSQLDB(&config.SQL{PluginName: "fake3"})
	assert.Error(t, err, "NewSQLDB failed to return an error with an unregistered plugin")
}
