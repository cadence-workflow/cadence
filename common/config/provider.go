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
	"flag"
	"fmt"
)

// Provider defines the interface for configuration access
type Provider interface {
	// GetConfig returns the loaded configuration
	GetConfig() *Config
	// GetLogger returns the logger configuration
	GetLogger() Logger
	// GetMetrics returns the metrics configuration
	GetMetrics() Metrics
	// GetPersistence returns the persistence configuration
	GetPersistence() Persistence
}

// provider implements the Provider interface
type provider struct {
	config *Config
}

// NewProvider creates a new configuration provider
func NewProvider() (Provider, error) {
	var params struct {
		Env       string
		Zone      string
		ConfigDir string
	}

	flag.StringVar(&params.Env, "env", "", "Env for the config")
	flag.StringVar(&params.Zone, "zone", "", "Zone for the config")
	flag.StringVar(&params.ConfigDir, "configDir", "", "Config directory")
	flag.Parse()

	if params.Env == "" {
		return nil, fmt.Errorf("env is required")
	}
	if params.ConfigDir == "" {
		return nil, fmt.Errorf("configDir is required")
	}

	var cfg Config
	if err := Load(params.Env, params.ConfigDir, params.Zone, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config (for env %q zone %q configDir %q): %w",
			params.Env, params.Zone, params.ConfigDir, err)
	}

	return &provider{
		config: &cfg,
	}, nil
}

func (p *provider) GetConfig() *Config {
	return p.config
}

func (p *provider) GetLogger() Logger {
	return p.config.Log
}

func (p *provider) GetMetrics() Metrics {
	return p.config.Services["cadence"].Metrics
}

func (p *provider) GetPersistence() Persistence {
	return p.config.Persistence
}
