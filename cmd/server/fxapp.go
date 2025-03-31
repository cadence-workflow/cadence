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

package main

import (
	"context"
	"os"

	"github.com/urfave/cli/v2"
	"go.uber.org/fx"

	"github.com/uber/cadence/cmd/server/cadence"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

// CadenceApp represents the main Cadence application
type CadenceApp struct {
	cliApp *cli.App
	logger log.Logger
}

// CadenceAppParams defines the parameters for constructing CadenceApp
type CadenceAppParams struct {
	fx.In

	Logger log.Logger
}

// NewCadenceApp creates a new CadenceApp instance
func NewCadenceApp(p CadenceAppParams) *CadenceApp {
	return &CadenceApp{
		logger: p.Logger,
	}
}

// Start initializes and starts the Cadence application
func (ca *CadenceApp) Start(ctx context.Context) error {
	ca.cliApp = cadence.BuildCLI(metrics.ReleaseVersion, metrics.Revision)
	if err := ca.cliApp.Run(os.Args); err != nil {
		ca.logger.Error("Failed to run CLI app", tag.Error(err))
		return err
	}
	return nil
}

// Stop performs cleanup when the application is shutting down
func (ca *CadenceApp) Stop(ctx context.Context) error {
	// Add any cleanup logic here
	return nil
}

// Module provides the fx module for the Cadence application
var Module = fx.Options(
	fx.Provide(
		NewCadenceApp,
	),
	// Ensure CadenceApp is constructed by using it in an empty Invoke
	fx.Invoke(func(*CadenceApp) {}),
	// Register lifecycle hooks
	fx.Invoke(func(lc fx.Lifecycle, app *CadenceApp) {
		lc.Append(fx.Hook{
			OnStart: app.Start,
			OnStop:  app.Stop,
		})
	}),
)
