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
	"errors"
	"fmt"
	stdLog "log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"go.uber.org/multierr"

	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/service"

	_ "go.uber.org/automaxprocs" // defines automaxpocs for dockerized usage.
)

// validServices is the list of all valid cadence services
var validServices = service.ShortNames(service.List)
var defaultServices = service.ShortNames(service.ListWithRing)

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
					Value:   strings.Join(defaultServices, ","),
					Usage:   "list of services to start",
				},
			},
			Action: func(c *cli.Context) error {
				host, err := os.Hostname()
				if err != nil {
					return fmt.Errorf("get hostname: %w", err)
				}

				appCtx := appContext{
					CfgContext: config.Context{
						Environment: getEnvironment(c),
						Zone:        getZone(c),
					},
					ConfigDir: getConfigDir(c),
					RootDir:   getRootDir(c),
					HostName:  host,
				}

				services := getServices(c)

				return runServices(
					services,
					func(serviceName string) fxAppInterface {
						return fx.New(
							fx.Module(serviceName,
								_commonModule,
								fx.Provide(
									func() appContext {
										return appCtx
									},
								),
								Module(serviceName),
							),
						)
					},
				)
			},
		},
	}

	return app
}

func runServices(services []string, appBuilder func(serviceName string) fxAppInterface) error {
	stoppedWg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, len(services))

	for _, serv := range services {
		stoppedWg.Add(1)
		go func(s string) {
			defer stoppedWg.Done()
			fxApp := appBuilder(s)

			//  If any of the start hooks return an error, Start short-circuits, calls Stop, and returns the inciting error.
			if err := fxApp.Start(ctx); err != nil {
				// If any of the apps fails to start, immediately cancel the context so others will also stop.
				cancel()
				errChan <- fmt.Errorf("service %s start: %w", s, err)
				return
			}

			select {
			// Block until FX receives a shutdown signal
			case <-fxApp.Done():
			}

			// Stop the application
			err := fxApp.Stop(ctx)
			if err != nil {
				errChan <- fmt.Errorf("service %s stop: %w", s, err)
			}
		}(serv)
	}
	go func() {
		stoppedWg.Wait()
		// After stoppedWg unblocked all services are stopped to we no longer wait for errors.
		close(errChan)
	}()

	var resErrors error
	for err := range errChan {
		// skip canceled errors, since they are caused by context cancelation and only focus on actual errors.
		if err != nil && !errors.Is(err, context.Canceled) {
			resErrors = multierr.Append(resErrors, err)
		}
	}
	if resErrors != nil {
		return resErrors
	}
	return nil
}

type appContext struct {
	fx.Out

	CfgContext config.Context
	ConfigDir  string `name:"config-dir"`
	RootDir    string `name:"root-dir"`
	HostName   string `name:"hostname"`
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

type fxAppInterface interface {
	Start(context.Context) error
	Stop(context.Context) error
	Done() <-chan os.Signal
}
