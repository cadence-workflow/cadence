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

package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"go.uber.org/config"
	"go.uber.org/fx"
	"gopkg.in/yaml.v2"
)

// Module returns a config.Provider that could be used byother components.
var Module = fx.Options(
	fx.Provide(New),
)

type Context struct {
	Environment string
	Zone        string
}

// Params defines the dependencies of the configfx module.
type Params struct {
	fx.In

	Context   Context
	LookupEnv LookupEnvFunc `optional:"true"`

	ConfigDir string `name:"config-dir"`

	Lifecycle fx.Lifecycle `optional:"true"` // required for strict mode
}

// Result defines the objects that the configfx module provides.
type Result struct {
	fx.Out

	Provider config.Provider
	Config   Config
}

// LookupEnvFunc returns the value of the environment variable given by key.
// It should behave the same as `os.LookupEnv`. If a function returns false,
// an environment variable is looked up using `envfx.Context.LookupEnv`.
type LookupEnvFunc func(key string) (string, bool)

// New exports functionality similar to Module, but allows the caller to wrap
// or modify Result. Most users should use Module instead.
func New(p Params) (Result, error) {
	lookupFun := os.LookupEnv
	if p.LookupEnv != nil {
		lookupFun = func(key string) (string, bool) {
			if result, ok := p.LookupEnv(key); ok {
				return result, true
			}
			return lookupFun(key)
		}
	}

	files, err := getConfigFiles(p.Context.Environment, p.ConfigDir, p.Context.Zone)
	if err != nil {
		return Result{}, fmt.Errorf("get config files: %w", err)
	}

	// TODO: permissive can be removed from here once we are no longer using root level config directly.
	cfgProvider, err := NewKeyTrackingProviderFromFiles(files, config.Expand(lookupFun), config.Permissive())
	if err != nil {
		return Result{}, fmt.Errorf("init config provider: %w", err)
	}

	var cfg Config
	err = cfgProvider.Get("").Populate(&cfg)
	if err != nil {
		return Result{}, fmt.Errorf("populage config: %w", err)
	}

	cfgProvider.MarkStructAccess(cfg)

	p.Lifecycle.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		return cfgProvider.VerifyAllKeysAccessed()
	}})

	return Result{
		Provider: cfgProvider,
		Config:   cfg,
	}, nil
}

// KeyTrackingProvider tracks which top-level keys in a YAML config are accessed.
type KeyTrackingProvider struct {
	baseProvider config.Provider
	allKeys      map[string]bool
	accessedKeys map[string]bool
	mu           sync.RWMutex
}

// FileSource represents a single YAML configuration file
type FileSource struct {
	Path    string
	Content []byte
}

// NewKeyTrackingProviderFromFiles creates a Provider from multiple YAML files.
func NewKeyTrackingProviderFromFiles(files []string, options ...config.YAMLOption) (*KeyTrackingProvider, error) {
	sources := make([]FileSource, 0, len(files))

	// Read all files
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		sources = append(sources, FileSource{
			Path:    path,
			Content: content,
		})
	}

	return NewKeyTrackingProviderFromSources(sources, options...)
}

// NewKeyTrackingProviderFromSources creates a Provider from multiple file sources.
func NewKeyTrackingProviderFromSources(sources []FileSource, opts ...config.YAMLOption) (*KeyTrackingProvider, error) {
	for _, source := range sources {
		opts = append(opts, config.Source(bytes.NewReader(source.Content)))
	}

	// Extract all top-level keys from all files
	allKeys := make(map[string]bool)
	for _, source := range sources {
		var topLevelMap map[string]interface{}
		if err := yaml.Unmarshal(source.Content, &topLevelMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", source.Path, err)
		}

		for key := range topLevelMap {
			allKeys[key] = true
		}
	}

	// Create base provider that merges all sources
	baseProvider, err := config.NewYAML(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config provider: %w", err)
	}

	return &KeyTrackingProvider{
		baseProvider: baseProvider,
		allKeys:      allKeys,
		accessedKeys: make(map[string]bool),
	}, nil
}

// Get forwards the call to the base provider and tracks key access.
func (p *KeyTrackingProvider) Get(key string) config.Value {
	if key == "" {
		// Getting root doesn't count as accessing any specific key
		return p.baseProvider.Get("")
	}

	// Extract top-level key
	topLevelKey := strings.Split(key, ".")[0]

	p.mu.Lock()
	p.accessedKeys[topLevelKey] = true
	p.mu.Unlock()

	return p.baseProvider.Get(key)
}

// Name returns the provider name.
func (p *KeyTrackingProvider) Name() string {
	return "KeyTrackingProvider(" + p.baseProvider.Name() + ")"
}

// MarkStructAccess analyzes a struct using reflection to determine which
// top-level keys it would access, and marks them as accessed.
func (p *KeyTrackingProvider) MarkStructAccess(v interface{}) {
	fields := CollectYAMLFields(v)

	p.mu.Lock()
	defer p.mu.Unlock()

	for key := range fields {
		p.accessedKeys[key] = true
	}
}

// GetAllKeys returns a list of all top-level keys in the config.
func (p *KeyTrackingProvider) GetAllKeys() []string {
	keys := make([]string, 0, len(p.allKeys))
	for key := range p.allKeys {
		keys = append(keys, key)
	}
	return keys
}

// GetUnusedKeys returns top-level keys that haven't been accessed.
func (p *KeyTrackingProvider) GetUnusedKeys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var unusedKeys []string
	for key := range p.allKeys {
		if !p.accessedKeys[key] {
			unusedKeys = append(unusedKeys, key)
		}
	}
	return unusedKeys
}

// VerifyAllKeysAccessed returns an error if any top-level keys weren't accessed.
func (p *KeyTrackingProvider) VerifyAllKeysAccessed() error {
	unusedKeys := p.GetUnusedKeys()
	if len(unusedKeys) > 0 {
		return fmt.Errorf("unused configuration keys: %v", unusedKeys)
	}
	return nil
}
