// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/canary"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/tools/common/commoncli"
)

func startHandler(c *cli.Context) error {
	env := getEnvironment(c)
	zone := getZone(c)
	configDir := getConfigDir(c)

	log.Printf("Loading config; env=%v,zone=%v,configDir=%v\n", env, zone, configDir)

	var cfg canary.Config
	if err := config.Load(env, configDir, zone, &cfg); err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	mode := c.String("mode")
	canary, err := canary.NewCanaryRunner(&cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize canary: %w", err)
	}

	if err := canary.Run(mode); err != nil {
		return fmt.Errorf("failed to run canary: %w", err)
	}
	return nil
}

func getRootDir(c *cli.Context) string {
	rootDir := c.String("root")
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
	configDir := c.String("config")
	return path.Join(rootDir, configDir)
}

func getEnvironment(c *cli.Context) string {
	return strings.TrimSpace(c.String("env"))
}

func getZone(c *cli.Context) string {
	return strings.TrimSpace(c.String("zone"))
}

func buildCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "cadence-canary"
	app.Usage = "Cadence canary"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "root",
			Aliases: []string{"r"},
			Value:   ".",
			Usage:   "root directory of execution environment",
			EnvVars: []string{canary.EnvKeyRoot},
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "config/canary",
			Usage:   "config dir path relative to root",
			EnvVars: []string{canary.EnvKeyConfigDir},
		},
		&cli.StringFlag{
			Name:    "env",
			Aliases: []string{"e"},
			Value:   "development",
			Usage:   "runtime environment",
			EnvVars: []string{canary.EnvKeyEnvironment},
		},
		&cli.StringFlag{
			Name:    "zone",
			Aliases: []string{"az"},
			Value:   "",
			Usage:   "availability zone",
			EnvVars: []string{canary.EnvKeyAvailabilityZone},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "start",
			Usage: "start cadence canary worker or cron, or both",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "mode",
					Aliases: []string{"m"},
					Value:   canary.ModeAll,
					Usage:   fmt.Sprintf("%v, %v or %v", canary.ModeWorker, canary.ModeCronCanary, canary.ModeAll),
					EnvVars: []string{canary.EnvKeyMode},
				},
			},
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
