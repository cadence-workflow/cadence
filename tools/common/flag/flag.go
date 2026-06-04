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
	"sort"
	"strings"
)

type (
	StringMap   map[string]string
	StringSlice []string
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

func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func (s *StringSlice) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, "\n")
}

func (s *StringSlice) Value() []string {
	if s == nil {
		return nil
	}
	return []string(*s)
}

// RepeatedStringFlag is a cli.Flag for repeatable string values that does NOT
// split on commas. Each Apply creates a fresh StringSlice to prevent cross-run
// accumulation when the flag is defined as a package-level var.
type RepeatedStringFlag struct {
	Name  string
	Usage string
}

func (f *RepeatedStringFlag) String() string {
	return fmt.Sprintf("--%s value\t%s", f.Name, f.Usage)
}

func (f *RepeatedStringFlag) Names() []string {
	return []string{f.Name}
}

// IsSet always returns false; c.IsSet() uses fs.Visit which is the
// authoritative check for whether the flag appeared on the command line.
func (f *RepeatedStringFlag) IsSet() bool {
	return false
}

// Apply registers a fresh StringSlice with the flag set on every call,
// preventing cross-run accumulation when the flag is a package-level var.
func (f *RepeatedStringFlag) Apply(set *goflag.FlagSet) error {
	set.Var(&StringSlice{}, f.Name, f.Usage)
	return nil
}
