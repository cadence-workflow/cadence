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
	"context"
	"fmt"
	"path"

	"go.uber.org/zap"

	"github.com/uber/cadence/tools/fastgogenerate/cache"
)

// Run executes the plugin with the provided name and arguments
// If the plugin is not found or the plugin failed, it will return an error
func Run(pluginName string, args []string) error {
	cfg := LoadConfig()
	if pluginName != "" {
		cfg.PluginName = pluginName
	}

	if cfg.PluginName == "" {
		return fmt.Errorf("plugin name is required")
	}

	loggerConfig := zap.NewDevelopmentConfig()
	if !cfg.Debug {
		loggerConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := loggerConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to create logger: %s", err)
	}

	// Initialize plugins
	InitializePlugins(logger, cfg)

	plugin, ok := GetPlugin(cfg.PluginName)
	if !ok {
		return fmt.Errorf("%s plugin not found, supported plugins: %s", cfg.PluginName, ListPluginNames())
	}

	executor := NewPluginExecutor(
		plugin,
		cfg,
		logger,
		cache.NewFileStorage(path.Join(cfg.CacheDirPath, cfg.PluginName), logger),
		cache.NewIDComputer(logger),
	)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	if err := executor.Execute(ctx, args); err != nil {
		return fmt.Errorf("%s returned error: %s, args: %s", plugin.Name(), err, args)
	}

	return nil
}
