// Copyright (c) 2017 Uber Technologies, Inc.
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

package cadence

import (
	"context"
	"fmt"
	stdLog "log"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/logfx"
	"github.com/uber/cadence/common/service"
)

// validServices is the list of all valid cadence services
var validServices = service.ShortNames(service.List)

func isValidService(in string) bool {
	for _, s := range validServices {
		if s == in {
			return true
		}
	}
	return false
}

// BuildCLI is the main entry point for the cadence server
func BuildCLI(releaseVersion string, gitRevision string) *cli.App {
	version := fmt.Sprintf(" Release version: %v \n"+
		"   Build commit: %v\n"+
		"   Max Support CLI feature version: %v \n"+
		"   Max Support GoSDK feature version: %v \n"+
		"   Max Support JavaSDK feature version: %v \n"+
		"   Note:  Feature version is for compatibility checking between server and clients if enabled feature checking. Server is always backward compatible to older CLI versions, but not accepting newer than it can support.",
		releaseVersion, gitRevision, client.SupportedCLIVersion, client.SupportedGoSDKVersion, client.SupportedJavaSDKVersion)

	app := cli.NewApp()
	app.Name = "cadence"
	app.Usage = "Cadence server"
	app.Version = version
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "root",
			Aliases: []string{"r"},
			Value:   ".",
			Usage:   "root directory of execution environment",
			EnvVars: []string{config.EnvKeyRoot},
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "config",
			Usage:   "config dir is a path relative to root, or an absolute path",
			EnvVars: []string{config.EnvKeyConfigDir},
		},
		&cli.StringFlag{
			Name:    "env",
			Aliases: []string{"e"},
			Value:   "development",
			Usage:   "runtime environment",
			EnvVars: []string{config.EnvKeyEnvironment},
		},
		&cli.StringFlag{
			Name:    "zone",
			Aliases: []string{"az"},
			Value:   "",
			Usage:   "availability zone",
			EnvVars: []string{config.EnvKeyAvailabilityZone},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:    "start",
			Aliases: []string{""},
			Usage:   "start cadence server",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "services",
					Aliases: []string{"s"},
					Value:   strings.Join(validServices, ","),
					Usage:   "list of services to start",
				},
			},
			Action: func(c *cli.Context) error {
				fxApp := fx.New(
					config.Module,
					logfx.Module,
					fx.Provide(func() appContext {
						return appContext{
							CfgContext: config.Context{
								Environment: getEnvironment(c),
								Zone:        getZone(c),
							},
							ConfigDir: getConfigDir(c),
							RootDir:   getRootDir(c),
							Services:  getServices(c),
						}
					}),
					Module,
				)

				ctx := context.Background()
				if err := fxApp.Start(ctx); err != nil {
					return err
				}

				// Block until FX receives a shutdown signal
				<-fxApp.Done()

				// Stop the application
				return fxApp.Stop(ctx)
			},
		},
	}

	return app

}

type appContext struct {
	fx.Out

	CfgContext config.Context
	ConfigDir  string   `name:"config-dir"`
	RootDir    string   `name:"root-dir"`
	Services   []string `name:"services"`
}

func getEnvironment(c *cli.Context) string {
	return strings.TrimSpace(c.String("env"))
}

func getZone(c *cli.Context) string {
	return strings.TrimSpace(c.String("zone"))
}

// getServices parses the services arg from cli
// and returns a list of services to start
func getServices(c *cli.Context) []string {
	val := strings.TrimSpace(c.String("services"))
	tokens := strings.Split(val, ",")

	if len(tokens) == 0 {
		stdLog.Fatal("list of services is empty")
	}

	for _, t := range tokens {
		if !isValidService(t) {
			stdLog.Fatalf("invalid service `%v` in service list [%v]", t, val)
		}
	}

	return tokens
}

func getConfigDir(c *cli.Context) string {
	return constructPathIfNeed(getRootDir(c), c.String("config"))
}

func getRootDir(c *cli.Context) string {
	dirpath := c.String("root")
	if len(dirpath) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			stdLog.Fatalf("os.Getwd() failed, err=%v", err)
		}
		return cwd
	}
	return dirpath
}

// constructPathIfNeed would append the dir as the root dir
// when the file wasn't absolute path.
func constructPathIfNeed(dir string, file string) string {
	if !filepath.IsAbs(file) {
		return dir + "/" + file
	}
	return file
}
