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
func NewPluginExecutor(
	plugin p.Plugin,
	config Config,
	logger *zap.Logger,
	storage cache.Storage,
	computer cache.IDComputer,
) *PluginExecutor {
	return &PluginExecutor{
		plugin:        plugin,
		logger:        logger.With(zap.String("plugin", plugin.Name())),
		storage:       storage,
		idComputer:    computer,
		cacheDisabled: config.CacheDisabled,
	}
}

// Execute runs the plugin with the provided arguments and caches the result
func (r *PluginExecutor) Execute(ctx context.Context, args []string) error {
	logger := r.logger.With(zap.Strings("args", args))
	logger.Debug("Starting plugin execution")

	cacheID, isExist, err := r.checkCache(ctx, args)
	if err != nil {
		logger.Debug("Couldn't use cache, executing plugin without caching", zap.Error(err))
		if err := r.plugin.Execute(ctx, args); err != nil {
			return fmt.Errorf("plugin failed: %w", err)
		}
		return nil
	}

	if isExist {
		logger.Debug("Plugin already executed, skipping")
		return nil
	}

	logger = logger.With(zap.String("cache id", string(cacheID)))
	logger.Debug("Cache not found, executing plugin")

	if err = r.plugin.Execute(ctx, args); err != nil {
		return fmt.Errorf("plugin failed: %w", err)
	}

	logger.Debug("Plugin executed successfully, saving cache")
	if err := r.storage.Save(cacheID); err != nil {
		logger.Warn("Failed to save cache", zap.Error(err))
	}

	logger.Debug("Cache saved successfully")
	return nil
}

func (r *PluginExecutor) checkCache(_ context.Context, args []string) (cache.ID, bool, error) {
	if r.cacheDisabled {
		return "", false, fmt.Errorf("cache is disabled")
	}

	info, err := r.plugin.ComputeInfo(args)
	if err != nil {
		return "", false, fmt.Errorf("failed to compute info: %w", err)
	}

	cacheID, err := r.idComputer.Compute(info)
	if err != nil {
		return "", false, fmt.Errorf("failed to compute cache ID: %w", err)
	}

	isExist, err := r.storage.IsExist(cacheID)
	if err != nil {
		return "", false, fmt.Errorf("failed to check cache existence: %w", err)
	}

	return cacheID, isExist, nil
}
