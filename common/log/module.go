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

package log

import (
	"fmt"

	"go.uber.org/config"
	"go.uber.org/fx"
)

// Module provides the logger dependencies
var Module = fx.Options(
	fx.Provide(Provide),
)

const (
	_configurationKey = "log"
)

// Params defines the parameters for constructing loggers
type Params struct {
	fx.In

	ConfigProvider config.Provider
}

// Result contains the logger instances
type Result struct {
	fx.Out

	CommonLogger Logger
}

// Provide creates and returns both zap and common loggers
func Provide(p Params) (Result, error) {
	var loggerConfig Config
	err := p.ConfigProvider.Get(_configurationKey).Populate(&loggerConfig)
	if err != nil {
		return Result{}, fmt.Errorf("extract logger config: %w", err)
	}
	zapLogger, err := loggerConfig.NewZapLogger()
	if err != nil {
		return Result{}, fmt.Errorf("build logger with config (%+v): %w", loggerConfig, err)
	}
	return Result{
		CommonLogger: New(zapLogger),
	}, nil
}
