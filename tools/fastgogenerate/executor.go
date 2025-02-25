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

	"go.uber.org/zap"

	"github.com/uber/cadence/tools/fastgogenerate/cache"
	p "github.com/uber/cadence/tools/fastgogenerate/plugins"
)

// PluginExecutor is responsible for executing a plugin and caching its result
// If provided args already exist in the cache, the plugin will not be executed
type PluginExecutor struct {
	plugin        p.Plugin
	logger        *zap.Logger
	storage       cache.Storage
	idComputer    cache.IDComputer
	cacheDisabled bool
}

// NewPluginExecutor creates a new PluginExecutor
func NewPluginExecutor(plugin p.Plugin, logger *zap.Logger, storage cache.Storage, computer cache.IDComputer) *PluginExecutor {
	return &PluginExecutor{
		plugin:     plugin,
		logger:     logger.With(zap.String("plugin", plugin.Name())),
		storage:    storage,
		idComputer: computer,
	}
}

// Execute runs the plugin with the provided arguments and caches the result
func (r *PluginExecutor) Execute(ctx context.Context, args []string) error {
	r.logger.Debug("Starting plugin execution", zap.Strings("args", args))

	if r.cacheDisabled {
		r.logger.Debug("Cache is disabled, executing plugin without caching")
		if err := r.plugin.Execute(ctx, args); err != nil {
			return fmt.Errorf("plugin failed: %w", err)
		}
		return nil
	}

	info, err := r.plugin.ComputeInfo(args)
	if err != nil {
		return fmt.Errorf("failed to compute info: %w", err)
	}

	cacheID, err := r.idComputer.Compute(info)
	if err != nil {
		return fmt.Errorf("failed to compute cache ID: %w", err)
	}

	isExist, err := r.storage.IsExist(cacheID)
	if err != nil {
		return fmt.Errorf("failed to check cache existence: %w", err)
	}
	if isExist {
		r.logger.Debug("Plugin already executed, skipping")
		return nil
	}

	r.logger.Debug("Cache not found, executing plugin", zap.String("cache id", string(cacheID)))
	if err = r.plugin.Execute(ctx, args); err != nil {
		return fmt.Errorf("plugin failed: %w", err)
	}

	r.logger.Debug("Plugin executed successfully, saving cache", zap.String("cache id", string(cacheID)))
	if err := r.storage.Save(cacheID); err != nil {
		r.logger.Warn("Failed to save cache", zap.Error(err))
	}

	r.logger.Debug("Cache saved successfully", zap.String("cache id", string(cacheID)))
	return nil
}
