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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestParseStringMap(t *testing.T) {
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
	}).Run([]string{"run", "-s", "a=b,c=d"})
}

func TestParseStringMapFromEnv(t *testing.T) {
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

func TestParseStringMapFromEnvCascade(t *testing.T) {
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

// TestStringMap_WhenSetIsCalledTwice_ItShouldResetBetweenCalls documents that
// StringMap.Set resets the map on each call. This prevents cross-run
// accumulation when the map is used as a GenericFlag.Value on a package-level
// var: without the reset, values set during one app.Run() bleed into the next
// parse. See the comment on StringMap.Set for more detail.
func TestStringMap_WhenSetIsCalledTwice_ItShouldResetBetweenCalls(t *testing.T) {
	m := StringMap{}

	require.NoError(t, m.Set("key1=value1,key2=value2"))
	assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, m.Value())

	// Second call simulates a new app.Run() with different flags — stale keys must not survive.
	require.NoError(t, m.Set("key3=value3"))
	assert.Equal(t, map[string]string{"key3": "value3"}, m.Value(),
		"stale keys from a previous parse must not accumulate")
}

// TestStringMap_WhenStringIsCalledAfterSet_ItShouldRoundTrip documents that
// String() produces output that Set() can parse back identically. urfave/cli's
// normalizeFlags propagates a flag value to its aliases via Set(String()), so
// the two methods must round-trip or the map gets wiped by a failed Set.
func TestStringMap_WhenStringIsCalledAfterSet_ItShouldRoundTrip(t *testing.T) {
	m := StringMap{}
	require.NoError(t, m.Set("key1=value1,key2=value2"))

	m2 := StringMap{}
	require.NoError(t, m2.Set(m.String()))
	assert.Equal(t, m.Value(), m2.Value())
}

func TestStringMap_WhenValueContainsNoEntries_StringShouldReturnEmpty(t *testing.T) {
	m := StringMap{}
	assert.Equal(t, "", m.String())
}

func TestStringMap_WhenSetReceivesEmptyString_ItShouldReturnEmptyMap(t *testing.T) {
	m := StringMap{"existing": "value"}
	require.NoError(t, m.Set(""))
	assert.Empty(t, m.Value())
}

func TestStringMap_WhenSetReceivesMalformedInput_ItShouldReturnError(t *testing.T) {
	m := StringMap{}
	err := m.Set("noequals")
	assert.ErrorContains(t, err, "key1=value1")
}

func TestStringSlice_Set_AppendsWithoutSplittingOnCommas(t *testing.T) {
	s := StringSlice{}
	require.NoError(t, s.Set("a,b,c"))
	require.NoError(t, s.Set("d"))
	assert.Equal(t, []string{"a,b,c", "d"}, s.Value())
}

func TestRepeatedStringFlag_MultipleInvocations_CollectsAllValues(t *testing.T) {
	var got []string
	app := &cli.App{
		Flags: []cli.Flag{
			&RepeatedStringFlag{Name: "entry", Usage: "test"},
		},
		Action: func(ctx *cli.Context) error {
			got = ctx.Generic("entry").(*StringSlice).Value()
			return nil
		},
	}
	require.NoError(t, app.Run([]string{"run", "--entry", "k1=v1,v2", "--entry", "k2=v3"}))
	assert.Equal(t, []string{"k1=v1,v2", "k2=v3"}, got)
}

func TestRepeatedStringFlag_FreshSlicePerRun_NoCrossRunAccumulation(t *testing.T) {
	f := &RepeatedStringFlag{Name: "entry", Usage: "test"}
	app := &cli.App{
		Flags:  []cli.Flag{f},
		Action: func(*cli.Context) error { return nil },
	}

	require.NoError(t, app.Run([]string{"run", "--entry", "first=run"}))
	// Second run must not see the value from the first.
	var got []string
	app2 := &cli.App{
		Flags: []cli.Flag{f},
		Action: func(ctx *cli.Context) error {
			got = ctx.Generic("entry").(*StringSlice).Value()
			return nil
		},
	}
	require.NoError(t, app2.Run([]string{"run", "--entry", "second=run"}))
	assert.Equal(t, []string{"second=run"}, got, "cross-run accumulation must not occur")
}
