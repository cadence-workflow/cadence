// Copyright (c) 2017-2021 Uber Technologies Inc.

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
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/bench"
	"github.com/uber/cadence/bench/lib"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/tools/common/commoncli"
)

const (
	flagRoot   = "root"
	flagConfig = "config"
	flagEnv    = "env"
	flagZone   = "zone"
)

const (
	defaultRoot   = "."
	defaultConfig = "config/bench"
	defaultEnv    = "development"
	defaultZone   = ""
)

func startHandler(c *cli.Context) error {
	env := getEnvironment(c)
	zone := getZone(c)
	configDir := getConfigDir(c)

	log.Printf("Loading config; env=%v,zone=%v,configDir=%v\n", env, zone, configDir)

	var cfg lib.Config
	if err := config.Load(env, configDir, zone, &cfg); err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	benchWorker, err := bench.NewWorker(&cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize bench worker: %w", err)
	}

	if err := benchWorker.Run(); err != nil {
		return fmt.Errorf("failed to run bench worker: %w", err)
	}
	return nil
}

func getRootDir(c *cli.Context) string {
	rootDir := c.String(flagRoot)
	if len(rootDir) == 0 {
		var err error
		if rootDir, err = os.Getwd(); err != nil {
			rootDir = "."
		}
	}
	return rootDir
}

func getConfigDir(c *cli.Context) string {
	rootDir := getRootDir(c)
	configDir := c.String(flagConfig)
	return path.Join(rootDir, configDir)
}

func getEnvironment(c *cli.Context) string {
	return strings.TrimSpace(c.String(flagEnv))
}

func getZone(c *cli.Context) string {
	return strings.TrimSpace(c.String(flagZone))
}

func buildCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "cadence-bench"
	app.Usage = "Cadence bench"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    flagRoot,
			Aliases: []string{"r"},
			Value:   defaultRoot,
			Usage:   "root directory of execution environment",
			EnvVars: []string{lib.EnvKeyRoot},
		},
		&cli.StringFlag{
			Name:    flagConfig,
			Aliases: []string{"c"},
			Value:   defaultConfig,
			Usage:   "config dir path relative to root",
			EnvVars: []string{lib.EnvKeyConfigDir},
		},
		&cli.StringFlag{
			Name:    flagEnv,
			Aliases: []string{"e"},
			Value:   defaultEnv,
			Usage:   "runtime environment",
			EnvVars: []string{lib.EnvKeyEnvironment},
		},
		&cli.StringFlag{
			Name:    flagZone,
			Aliases: []string{"z"},
			Value:   defaultZone,
			Usage:   "availability zone",
			EnvVars: []string{lib.EnvKeyAvailabilityZone},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "start",
			Usage: "start cadence bench worker",
			Action: func(c *cli.Context) error {
				return startHandler(c)
			},
		},
	}

	return app
}

func main() {
	app := buildCLI()
	commoncli.ExitHandler(app.Run(os.Args))
}
