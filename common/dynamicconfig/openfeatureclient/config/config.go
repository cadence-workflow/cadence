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

import "github.com/uber/cadence/common/config/yaml"

// Config configures the dynamicconfig.Client backed by OpenFeature.
// ProviderName selects a provider plugin self-registered under
// common/dynamicconfig/openfeatureprovider/<name> (blank-imported by the
// binary that wants it, e.g. cmd/server/main.go). Provider holds that
// plugin's own config shape, decoded lazily via yaml.Node so this package -
// and common/config, which embeds this struct directly - never import any
// provider-specific config type.
type Config struct {
	ProviderName string     `yaml:"providerName"`
	Provider     *yaml.Node `yaml:"provider"`
}
