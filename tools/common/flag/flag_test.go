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

package flag

import (
	"os"
	"reflect"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestParseMultiStringMap(t *testing.T) {
	_ = (&cli.App{
		Flags: []cli.Flag{
			&cli.GenericFlag{Name: "serve", Aliases: []string{"s"}, Value: &StringMap{}},
		},
		Action: func(ctx *cli.Context) error {
			if !reflect.DeepEqual(ctx.Generic("serve"), &StringMap{"a": "b", "c": "d"}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &StringMap{"a": "b", "c": "d"}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "a=b", "-s", "c=d"})
}

func TestParseMultiStringMapFromEnv(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("APP_SERVE", "x=y,w=v")
	_ = (&cli.App{
		Flags: []cli.Flag{
			&cli.GenericFlag{Name: "serve", Aliases: []string{"s"}, Value: &StringMap{}, EnvVars: []string{"APP_SERVE"}},
		},
		Action: func(ctx *cli.Context) error {
			if !reflect.DeepEqual(ctx.Generic("serve"), &StringMap{"x": "y", "w": "v"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &StringMap{"x": "y", "w": "v"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiStringMapFromEnvCascade(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("APP_FOO", "u=t,r=s")
	_ = (&cli.App{
		Flags: []cli.Flag{
			&cli.GenericFlag{Name: "foos", Value: &StringMap{}, EnvVars: []string{"COMPAT_FOO", "APP_FOO"}},
		},
		Action: func(ctx *cli.Context) error {
			if !reflect.DeepEqual(ctx.Generic("foos"), &StringMap{"u": "t", "r": "s"}) {
				t.Errorf("value not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}
