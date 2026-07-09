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

// Package config holds only the YAML-decodable Config for the
// OpenFeature-backed dynamicconfig.Client, so common/config can embed it
// directly. Deliberately kept free of any OpenFeature SDK import:
// github.com/open-feature/go-sdk/openfeature starts a background goroutine
// unconditionally from its own package init() the moment anything imports
// it, whether or not a provider is ever configured. common/config is
// imported by nearly every binary and test package in this repo, so if it
// pulled that SDK in transitively, every one of them would carry that
// goroutine permanently - breaking any test using goleak.VerifyNone. The
// actual client implementation (openfeatureclient.NewOpenFeatureClient et
// al., which does need the SDK) lives in the parent
// common/dynamicconfig/openfeatureclient package instead, imported only by
// dynamicconfigfx.New.
package config

import "gopkg.in/yaml.v2" // CAUTION: go.uber.org/config does not support yaml.v3

// Config configures the dynamicconfig.Client backed by OpenFeature.
// ProviderName selects a provider plugin self-registered under
// common/dynamicconfig/openfeatureprovider/<name> (blank-imported by the
// binary that wants it, e.g. cmd/server/main.go). Provider holds that
// plugin's own config shape, decoded lazily so this package - and
// common/config, which embeds this struct directly - never import any
// provider-specific config type.
type Config struct {
	ProviderName string   `yaml:"providerName"`
	Provider     *RawYAML `yaml:"provider"`
}

// RawYAML is a lazy YAML unmarshaler, deferring decode until the selected
// provider plugin knows what shape to decode into. It mirrors
// common/config.YamlNode; that type can't be reused here directly since
// common/config already imports common/dynamicconfig, and this package must
// stay import-cycle-free with respect to common/config.
type RawYAML struct {
	unmarshal func(out interface{}) error
}

var _ yaml.Unmarshaler = (*RawYAML)(nil)

func (r *RawYAML) UnmarshalYAML(unmarshal func(interface{}) error) error {
	r.unmarshal = unmarshal
	return nil
}

// Decode satisfies openfeatureprovider.Decoder.
func (r *RawYAML) Decode(out any) error {
	if r == nil {
		return nil
	}
	return r.unmarshal(out)
}
