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
	"strings"
)

type (
	StringMap   map[string]string
	StringSlice []string
)

func (m *StringMap) Set(value string) error {
	if m == nil {
		return fmt.Errorf("StringMap is nil")
	}
	if *m == nil {
		*m = make(map[string]string)
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

func (m *StringMap) String() string {
	if m == nil {
		return fmt.Sprintf("%v", map[string]string(nil))
	}
	return fmt.Sprintf("%v", *m)
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

// NoSepStringSliceFlag is a cli.Flag for repeatable string values that does
// NOT split on commas. Use instead of cli.StringSliceFlag when values may
// contain commas (e.g. JSON arrays). Each Apply call creates a fresh
// StringSlice so package-level flag definitions don't accumulate across runs.
type NoSepStringSliceFlag struct {
	Name  string
	Usage string
}

func (f *NoSepStringSliceFlag) String() string {
	return fmt.Sprintf("--%s value\t%s", f.Name, f.Usage)
}

func (f *NoSepStringSliceFlag) Names() []string {
	return []string{f.Name}
}

// IsSet always returns false; c.IsSet() also uses fs.Visit which is the
// authoritative check for whether the flag appeared on the command line.
func (f *NoSepStringSliceFlag) IsSet() bool {
	return false
}

// Apply registers a fresh StringSlice with the flag set on every call,
// preventing cross-run accumulation when the flag is a package-level var.
func (f *NoSepStringSliceFlag) Apply(set *goflag.FlagSet) error {
	set.Var(&StringSlice{}, f.Name, f.Usage)
	return nil
}

// FreshStringMapFlag is a cli.Flag for key=value pairs (same format as
// GenericFlag+StringMap) that creates a fresh StringMap on each Apply to
// prevent cross-run accumulation when the flag is defined as a package-level var.
type FreshStringMapFlag struct {
	Name    string
	Aliases []string
	Usage   string
}

func (f *FreshStringMapFlag) String() string {
	return fmt.Sprintf("--%s value\t%s", f.Name, f.Usage)
}

func (f *FreshStringMapFlag) Names() []string {
	return append([]string{f.Name}, f.Aliases...)
}

func (f *FreshStringMapFlag) IsSet() bool {
	return false
}

// Apply registers a single fresh StringMap shared across all aliases on every
// call, preventing cross-run accumulation when the flag is a package-level var.
func (f *FreshStringMapFlag) Apply(set *goflag.FlagSet) error {
	fresh := &StringMap{}
	for _, name := range f.Names() {
		set.Var(fresh, name, f.Usage)
	}
	return nil
}
