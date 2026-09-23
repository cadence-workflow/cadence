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
	goflag "flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

type (
	StringMap map[string]string
	// RepeatedStringMap represents values for a CLI argument where key value pairs are populated by repeating the
	// option. This has to be a distinct type because the parsing behavior is defined as a receiver method on the type
	RepeatedStringMap map[string]string
)

// Set resets the map before parsing. This is intentional: StringMap is used as
// a GenericFlag.Value on package-level vars; without a reset, values from a
// previous app.Run() call accumulate into the next parse. The reset also makes
// Set idempotent when urfave/cli's normalizeFlags copies the flag value to
// aliases via Set(String()) — see String() below.
func (m *StringMap) Set(value string) error {
	if m == nil {
		return fmt.Errorf("StringMap is nil")
	}
	*m = make(StringMap) // reset — see comment above
	if value == "" {
		return nil
	}
	for _, s := range strings.Split(value, ",") {
		kv := strings.Split(s, "=")
		if len(kv) != 2 {
			return fmt.Errorf("should be in 'key1=value1,key2=value2,...,keyN=valueN' format")
		}
		(*m)[kv[0]] = kv[1]
	}
	return nil
}

// String returns the map in key1=v1,key2=v2 format so that it round-trips
// cleanly through Set. urfave/cli's normalizeFlags propagates a flag's value to
// its aliases by calling Set(String()), so returning the Go default map
// representation (map[k:v]) would cause Set to fail and wipe the map.
func (m *StringMap) String() string {
	if m == nil || len(*m) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(*m))
	for k, v := range *m {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs) // deterministic output
	return strings.Join(pairs, ",")
}

func (m *StringMap) Value() map[string]string {
	if m == nil {
		return nil
	}
	return *m
}

func (m *RepeatedStringMap) Set(value string) error {
	if m == nil {
		return fmt.Errorf("RepeatedStringMap is nil")
	}
	kv := strings.SplitN(value, "=", 2)
	if len(kv) != 2 {
		return fmt.Errorf("value %q must be in key=value format", value)
	}
	if _, dup := (*m)[kv[0]]; dup {
		return fmt.Errorf("key %q specified more than once", kv[0])
	}
	(*m)[kv[0]] = kv[1]
	return nil
}

func (m *RepeatedStringMap) String() string {
	if m == nil || len(*m) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(*m))
	for k, v := range *m {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs) // deterministic output
	return strings.Join(pairs, ",")
}

func (m *RepeatedStringMap) Value() map[string]string {
	if m == nil {
		return nil
	}
	return *m
}

// RepeatedStringMapFlag is a cli.Flag for key/value pairs by repeating the option rather than parsing a single value
// into key/value pairs. For example, "--setup-option replication_factor=1 --setup-option cluster=dca" yields
// map[string]string{"replication_factor": "1", "cluster": "dca"}. The flag can also be seeded from environment
// variables with a common prefix, e.g. CADENCE_SETUP_OPTION_<KEY>=<VALUE>. See EnvVarPrefix.
// The only parsing logic is that the value is split on the first `=`, and may contain any other characters after that.
// Each Apply creates a fresh RepeatedStringMap to prevent cross-run accumulation when the flag is defined as a
// package-level var.
type RepeatedStringMapFlag struct {
	Name  string
	Usage string
	// EnvVarPrefix, when non-empty, identifies a family of environment variables
	// that seed the flag's value. Every variable named "<EnvVarPrefix>_<KEY>" is
	// turned into a "<key>=<value>" entry, where only the key is lowercased.
	// For example, with prefix CADENCE_SETUP_OPTION the variable
	// CADENCE_SETUP_OPTION_REPLICATION_FACTOR=1 yields "replication_factor=1".
	// Values given on the command line are appended after the environment
	// derived ones, so consumers see duplicate keys as a conflict.
	EnvVarPrefix string

	// envApplied records whether the most recent Apply seeded any values from
	// the environment, so IsSet can report them as set.
	envApplied bool
}

func (f *RepeatedStringMapFlag) String() string {
	s := fmt.Sprintf("--%s value\t%s", f.Name, f.Usage)
	if f.EnvVarPrefix != "" {
		s += fmt.Sprintf(" [$%s_<KEY>]", f.EnvVarPrefix)
	}
	return s
}

func (f *RepeatedStringMapFlag) Names() []string {
	return []string{f.Name}
}

// IsVisible makes the flag appear in --help output; urfave/cli hides any
// flag that does not implement cli.VisibleFlag.
func (f *RepeatedStringMapFlag) IsVisible() bool {
	return true
}

// IsSet reports whether values were seeded from the environment. Values passed
// on the command line are detected by c.IsSet() via fs.Visit, which is the
// authoritative check for those.
func (f *RepeatedStringMapFlag) IsSet() bool {
	return f.envApplied
}

// Apply registers a fresh RepeatedStringMap with the flag set on every call,
// preventing cross-run accumulation when the flag is a package-level var.
// When EnvVarPrefix is set, matching environment variables seed the slice.
func (f *RepeatedStringMapFlag) Apply(set *goflag.FlagSet) error {
	values, err := f.envValues()
	if err != nil {
		return fmt.Errorf("%s: %w", f.Name, err)
	}
	f.envApplied = len(values) > 0
	result := RepeatedStringMap(values)
	set.Var(&result, f.Name, f.Usage)
	return nil
}

// envValues collects "<key>=<value>" entries derived from environment variables
// matching EnvVarPrefix, sorted for deterministic ordering.
func (f *RepeatedStringMapFlag) envValues() (map[string]string, error) {
	if f.EnvVarPrefix == "" {
		return map[string]string{}, nil
	}
	prefix := f.EnvVarPrefix + "_"
	result := make(map[string]string)
	for _, env := range os.Environ() {
		name, value, found := strings.Cut(env, "=")
		if !found {
			continue
		}
		if name == f.EnvVarPrefix {
			return nil, fmt.Errorf("environment variable %s must be named %s<KEY>", name, prefix)
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		key := strings.ToLower(strings.TrimPrefix(name, prefix))
		if key == "" {
			return nil, fmt.Errorf("environment variable %s must be named %s<KEY>", name, prefix)
		}
		if _, dup := result[key]; dup {
			return nil, fmt.Errorf("duplicate variable: %s", name)
		}
		result[key] = value
	}
	return result, nil
}
