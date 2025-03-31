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

package logger

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/loggerimpl"
)

// Params defines the parameters for constructing loggers
type Params struct {
	fx.In

	ConfigProvider config.Provider
}

// Result contains the logger instances
type Result struct {
	fx.Out

	ZapLogger    *zap.Logger
	CommonLogger log.Logger
}

// Provide creates and returns both zap and common loggers
func Provide(p Params) (Result, error) {
	loggerConfig := p.ConfigProvider.GetLogger()
	zapLogger, err := loggerConfig.NewZapLogger()
	if err != nil {
		return Result{}, err
	}

	commonLogger := loggerimpl.NewLogger(zapLogger)

	return Result{
		ZapLogger:    zapLogger,
		CommonLogger: commonLogger,
	}, nil
}

// Module provides the logger dependencies
var Module = fx.Options(
	fx.Provide(Provide),
)
