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
	"fmt"
	"os"

	"go.uber.org/config"
	"go.uber.org/fx"
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
	LookupEnv LookupEnvFunc

	ConfigDir string `name:"config-dir"`

	Lifecycle fx.Lifecycle `optional:"true"` // required for strict mode
}

// Result defines the objects that the configfx module provides.
type Result struct {
	fx.Out

	Provider config.Provider
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
		return Result{}, fmt.Errorf("unable to get config files: %w", err)
	}

	var options []config.YAMLOption
	for _, f := range files {
		options = append(options, config.File(f))
	}

	// expand env variables declared in .yaml files
	options = append(options, config.Expand(lookupFun))

	yaml, err := config.NewYAML(options...)
	if err != nil {
		return Result{}, fmt.Errorf("create yaml parser: %w", err)
	}

	return Result{
		Provider: yaml,
	}, nil
}
