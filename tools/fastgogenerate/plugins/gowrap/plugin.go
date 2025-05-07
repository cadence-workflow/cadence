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

package gowrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"go.uber.org/zap"

	"github.com/uber/cadence/tools/fastgogenerate/plugins"
)

const Name = "gowrap"

// Plugin is a plugin for generating mocks using gowrap
// Support a change detection of $GOFILE file content. In case of changes in other dependency files,
// the plugin will not detect them.
type Plugin struct {
	logger  *zap.Logger
	binPath string
}

func NewPlugin(logger *zap.Logger, cfg Config) *Plugin {
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = Name
	}

	return &Plugin{
		logger:  logger.With(zap.String("plugin", Name)),
		binPath: cfg.BinaryPath,
	}
}

func (p *Plugin) Name() string {
	return Name
}

func (p *Plugin) ComputeInfo(args []string) (plugins.ComputeInfo, error) {
	if len(args) == 0 {
		return plugins.ComputeInfo{}, fmt.Errorf("no arguments provided")
	}

	var nextIsDestination bool
	var info = plugins.ComputeInfo{
		PluginName: p.Name(),
		Args:       args,

		// Plugin can determine only $GOFILE as input file name
		// In case of changes in other dependency files, the plugin will not detect them
		InputFileNames: []string{
			os.Getenv("GOFILE"),
		},
	}

	// Parse the arguments to find output file names in --destination or -destination flags
	for _, arg := range args {
		if arg == "-o" {
			nextIsDestination = true
			continue
		}

		if nextIsDestination {
			info.OutputFileNames = append(info.OutputFileNames, arg)
			nextIsDestination = false
		}
	}

	p.logger.Debug("Prepared compute info",
		zap.Strings("args", args),
		zap.Strings("inputFileNames", info.InputFileNames),
		zap.Strings("outputFileNames", info.OutputFileNames),
	)
	return info, nil
}

func (p *Plugin) Execute(ctx context.Context, args []string) error {

	cmd := exec.CommandContext(ctx, p.binPath, args...)
	p.logger.Debug("Executing command", zap.Any("cmd", cmd))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
