// Copyright (c) 2021 Uber Technologies, Inc.
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

package nosql

import (
	"fmt"
	"testing"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

var supportedPlugins = map[string]nosqlplugin.Plugin{}

// RegisterPlugin will register a NoSQL plugin
func RegisterPlugin(pluginName string, plugin nosqlplugin.Plugin) {
	if _, ok := supportedPlugins[pluginName]; ok {
		panic("plugin " + pluginName + " already registered")
	}
	supportedPlugins[pluginName] = plugin
}

// RegisterPluginForTest should be used only in tests to register the DB plugin and de-register at the end
func RegisterPluginForTest(t *testing.T, pluginName string, plugin nosqlplugin.Plugin) {
	t.Cleanup(func() { delete(supportedPlugins, pluginName) })
	supportedPlugins[pluginName] = plugin
}

// RegisterPluginIfNotExists will register a NoSQL plugin only if a plugin with same name has not already been registered
func RegisterPluginIfNotExists(pluginName string, plugin nosqlplugin.Plugin) {
	if _, ok := supportedPlugins[pluginName]; !ok {
		supportedPlugins[pluginName] = plugin
	}
}

// PluginRegistered returns true if plugin with given name has been registered, false otherwise
func PluginRegistered(pluginName string) bool {
	_, ok := supportedPlugins[pluginName]
	return ok
}

// GetRegisteredPluginNames returns the list of registered plugin names
func GetRegisteredPluginNames() []string {
	var plugins []string
	for k := range supportedPlugins {
		plugins = append(plugins, k)
	}
	return plugins
}

// NewNoSQLDB creates a returns a reference to a logical connection to the
// underlying NoSQL database. The returned object is to tied to a single
// NoSQL database and the object can be used to perform CRUD operations on
// the tables in the database
func NewNoSQLDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (nosqlplugin.DB, error) {
	plugin, ok := supportedPlugins[cfg.PluginName]

	if !ok {
		return nil, fmt.Errorf("not supported plugin %v, only supported: %v", cfg.PluginName, supportedPlugins)
	}

	return plugin.CreateDB(cfg, logger, dc)
}
