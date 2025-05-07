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

package fastgogenerate

import (
	"go.uber.org/zap"

	p "github.com/uber/cadence/tools/fastgogenerate/plugins"
	"github.com/uber/cadence/tools/fastgogenerate/plugins/gowrap"
	"github.com/uber/cadence/tools/fastgogenerate/plugins/mockgen"
)

func InitializePlugins(logger *zap.Logger, config Config) {
	// Use the original binary name + binary suffix as the default binary path
	if config.BinarySuffix != "" {
		config.Mockgen.BinaryPath = mockgen.Name + "." + config.BinarySuffix
		config.GoWrap.BinaryPath = gowrap.Name + "." + config.BinarySuffix
	}

	RegisterPlugin(mockgen.NewPlugin(logger, config.Mockgen))
	RegisterPlugin(gowrap.NewPlugin(logger, config.GoWrap))
}

var plugins = make(map[string]p.Plugin)

// RegisterPlugin registers a plugin with the given name
func RegisterPlugin(plugin p.Plugin) {
	plugins[plugin.Name()] = plugin
}

// GetPlugin retrieves a plugin by its name
func GetPlugin(name string) (p.Plugin, bool) {
	plugin, ok := plugins[name]
	return plugin, ok
}

// ListPluginNames returns a list of all registered plugin names
func ListPluginNames() []string {
	names := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		names = append(names, plugin.Name())
	}
	return names
}
