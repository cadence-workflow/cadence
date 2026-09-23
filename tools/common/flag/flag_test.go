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
	"bytes"
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

func TestRepeatedStringMap(t *testing.T) {
	tests := []struct {
		name            string
		sets            []string
		wantErrContains string
		wantValue       map[string]string
		wantString      string
	}{
		{
			name:       "zero value is empty",
			wantValue:  map[string]string{},
			wantString: "",
		},
		{
			// Each Set call contributes exactly one entry: the value is split on the
			// first "=" and the remainder is kept verbatim, commas included.
			name:       "set splits on the first equals only",
			sets:       []string{"a=b,c=d", "e=f"},
			wantValue:  map[string]string{"a": "b,c=d", "e": "f"},
			wantString: "a=b,c=d,e=f",
		},
		{
			name:       "string sorts pairs for deterministic output",
			sets:       []string{"b=2", "a=1"},
			wantValue:  map[string]string{"a": "1", "b": "2"},
			wantString: "a=1,b=2",
		},
		{
			name:            "value without an equals is an error",
			sets:            []string{"noequals"},
			wantErrContains: "must be in key=value format",
		},
		{
			name:            "repeated key is an error",
			sets:            []string{"a=b", "a=c"},
			wantErrContains: `key "a" specified more than once`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := RepeatedStringMap{}

			var err error
			for _, s := range tt.sets {
				if err = m.Set(s); err != nil {
					break
				}
			}

			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, m.Value())
			assert.Equal(t, tt.wantString, m.String())
		})
	}
}

func TestRepeatedStringMapFlag_MultipleInvocations_CollectsAllValues(t *testing.T) {
	var got map[string]string
	app := &cli.App{
		Flags: []cli.Flag{
			&RepeatedStringMapFlag{Name: "entry", Usage: "test"},
		},
		Action: func(ctx *cli.Context) error {
			got = ctx.Generic("entry").(*RepeatedStringMap).Value()
			return nil
		},
	}
	require.NoError(t, app.Run([]string{"run", "--entry", "k1=v1,v2", "--entry", "k2=v3"}))
	assert.Equal(t, map[string]string{"k1": "v1,v2", "k2": "v3"}, got)
}

func TestRepeatedStringMapFlag_RepeatedKeyOnCommandLine_IsAnError(t *testing.T) {
	app := &cli.App{
		Flags: []cli.Flag{
			&RepeatedStringMapFlag{Name: "entry", Usage: "test"},
		},
		Action: func(*cli.Context) error { return nil },
	}
	err := app.Run([]string{"run", "--entry", "k1=v1", "--entry", "k1=v2"})
	assert.ErrorContains(t, err, `key "k1" specified more than once`)
}

func TestRepeatedStringMapFlag_FreshMapPerRun_NoCrossRunAccumulation(t *testing.T) {
	f := &RepeatedStringMapFlag{Name: "entry", Usage: "test"}
	app := &cli.App{
		Flags:  []cli.Flag{f},
		Action: func(*cli.Context) error { return nil },
	}

	require.NoError(t, app.Run([]string{"run", "--entry", "first=run"}))
	// Second run must not see the value from the first.
	var got map[string]string
	app2 := &cli.App{
		Flags: []cli.Flag{f},
		Action: func(ctx *cli.Context) error {
			got = ctx.Generic("entry").(*RepeatedStringMap).Value()
			return nil
		},
	}
	require.NoError(t, app2.Run([]string{"run", "--entry", "second=run"}))
	assert.Equal(t, map[string]string{"second": "run"}, got, "cross-run accumulation must not occur")
}

func TestRepeatedStringMapFlag_AppearsInHelpOutput(t *testing.T) {
	var help bytes.Buffer
	app := &cli.App{
		Writer: &help,
		Flags: []cli.Flag{
			&RepeatedStringMapFlag{Name: "entry", Usage: "a repeatable entry"},
		},
		Action: func(*cli.Context) error { return nil },
	}

	require.NoError(t, app.Run([]string{"run", "--help"}))
	assert.Contains(t, help.String(), "--entry")
	assert.Contains(t, help.String(), "a repeatable entry")
}

func TestRepeatedStringMapFlag_EnvVarPrefix(t *testing.T) {
	const prefix = "CADENCE_SETUP_OPTION"
	tests := []struct {
		name            string
		prefix          string
		env             map[string]string
		args            []string
		want            map[string]string
		wantErrContains string
	}{
		{
			name:   "single matching variable is lowercased by key only",
			prefix: prefix,
			env:    map[string]string{"CADENCE_SETUP_OPTION_REPLICATION_FACTOR": "1"},
			args:   []string{"run"},
			want:   map[string]string{"replication_factor": "1"},
		},
		{
			name:   "value case is preserved",
			prefix: prefix,
			env:    map[string]string{"CADENCE_SETUP_OPTION_CLUSTER": "DCA"},
			args:   []string{"run"},
			want:   map[string]string{"cluster": "DCA"},
		},
		{
			name:   "multiple matching variables are all collected",
			prefix: prefix,
			env: map[string]string{
				"CADENCE_SETUP_OPTION_ZONE":               "z1",
				"CADENCE_SETUP_OPTION_REPLICATION_FACTOR": "1",
			},
			args: []string{"run"},
			want: map[string]string{"zone": "z1", "replication_factor": "1"},
		},
		{
			name:   "non-matching variables are ignored",
			prefix: prefix,
			env: map[string]string{
				"CADENCE_SETUP_OPTION_FOO":  "bar",
				"CADENCE_OTHER_OPTION_BAZ":  "qux",
				"CADENCE_SETUP_OPTIONX_FOO": "nope",
			},
			args: []string{"run"},
			want: map[string]string{"foo": "bar"},
		},
		{
			name:   "empty prefix ignores the environment",
			prefix: "",
			env:    map[string]string{"CADENCE_SETUP_OPTION_FOO": "bar"},
			args:   []string{"run"},
			want:   map[string]string{},
		},
		{
			name:   "command line values are merged with environment values",
			prefix: prefix,
			env:    map[string]string{"CADENCE_SETUP_OPTION_FOO": "bar"},
			args:   []string{"run", "--entry", "baz=qux"},
			want:   map[string]string{"foo": "bar", "baz": "qux"},
		},
		{
			name:            "command line value conflicting with the environment is an error",
			prefix:          prefix,
			env:             map[string]string{"CADENCE_SETUP_OPTION_FOO": "bar"},
			args:            []string{"run", "--entry", "foo=other"},
			wantErrContains: `key "foo" specified more than once`,
		},
		{
			name:            "bare prefix without key is an error",
			prefix:          prefix,
			env:             map[string]string{"CADENCE_SETUP_OPTION": "bar"},
			args:            []string{"run"},
			wantErrContains: "must be named CADENCE_SETUP_OPTION_<KEY>",
		},
		{
			name:            "empty key is an error",
			prefix:          prefix,
			env:             map[string]string{"CADENCE_SETUP_OPTION_": "bar"},
			args:            []string{"run"},
			wantErrContains: "must be named CADENCE_SETUP_OPTION_<KEY>",
		},
		{
			name:   "colliding keys are an error",
			prefix: prefix,
			env: map[string]string{
				"CADENCE_SETUP_OPTION_FOO": "bar",
				"CADENCE_SETUP_OPTION_foo": "baz",
			},
			args:            []string{"run"},
			wantErrContains: "duplicate variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			var got map[string]string
			app := &cli.App{
				Flags: []cli.Flag{
					&RepeatedStringMapFlag{Name: "entry", Usage: "test", EnvVarPrefix: tt.prefix},
				},
				Action: func(ctx *cli.Context) error {
					got = ctx.Generic("entry").(*RepeatedStringMap).Value()
					return nil
				},
			}

			err := app.Run(tt.args)
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRepeatedStringMapFlag_IsSet_ReportsEnvironmentValues(t *testing.T) {
	f := &RepeatedStringMapFlag{Name: "entry", Usage: "test", EnvVarPrefix: "CADENCE_SETUP_OPTION"}
	assert.False(t, f.IsSet(), "not applied yet")

	t.Setenv("CADENCE_SETUP_OPTION_FOO", "bar")
	var isSet bool
	app := &cli.App{
		Flags: []cli.Flag{f},
		Action: func(ctx *cli.Context) error {
			isSet = ctx.IsSet("entry")
			return nil
		},
	}
	require.NoError(t, app.Run([]string{"run"}))
	assert.True(t, isSet)
}

func TestRepeatedStringMapFlag_HelpOutputMentionsEnvVarPrefix(t *testing.T) {
	var help bytes.Buffer
	app := &cli.App{
		Writer: &help,
		Flags: []cli.Flag{
			&RepeatedStringMapFlag{Name: "entry", Usage: "a repeatable entry", EnvVarPrefix: "CADENCE_SETUP_OPTION"},
		},
		Action: func(*cli.Context) error { return nil },
	}

	require.NoError(t, app.Run([]string{"run", "--help"}))
	assert.Contains(t, help.String(), "$CADENCE_SETUP_OPTION_<KEY>")
}
