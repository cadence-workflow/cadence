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

package logfx

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
)

// Module provides fx options to initialize logger from configuration.
var Module = fx.Options(
	fx.Provide(zapBuilder),
	ModuleWithoutZap,
)

// ModuleWithoutZap provides fx options to initialize logger from existing zap logger, which might be provided outside.
// This is useful for monorepos or any application that uses centralize log configuration like Uber monorepo.
var ModuleWithoutZap = fx.Options(
	fx.Provide(log.NewLogger))

type zapBuilderParams struct {
	fx.In

	Cfg config.Config
}

func zapBuilder(p zapBuilderParams) (*zap.Logger, error) {
	logger, err := p.Cfg.Log.NewZapLogger()
	if err != nil {
		return nil, err
	}
	return logger, nil
}
