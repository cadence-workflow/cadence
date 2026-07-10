// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package openfeatureprovider is a plugin registry for OpenFeature providers.
// Each provider (flagd, unleash, etc.) lives in its own subpackage, defines its own
// config struct, and self-registers a Constructor via init() - see the unleash
// subpackage for the reference implementation. Binaries opt into a provider with a
// blank import (e.g. cmd/server/main.go), same convention as
// common/asyncworkflow/queue/provider and common/archiver/provider.
package openfeatureprovider

import (
	"fmt"

	"github.com/open-feature/go-sdk/openfeature"

	"github.com/uber/cadence/common/syncmap"
)

type (
	// Decoder decodes a provider's own configuration. *config.YamlNode satisfies this
	// structurally, so this package never needs to import common/config.
	Decoder interface {
		Decode(out any) error
	}

	// Constructor builds an OpenFeature provider from its own config, decoded via cfg.
	Constructor func(cfg Decoder) (openfeature.FeatureProvider, error)
)

var constructors = syncmap.New[string, Constructor]()

// Register registers a named OpenFeature provider constructor. Intended to be called
// from a provider's own package init(), e.g.
// common/dynamicconfig/openfeatureprovider/unleash.
func Register(name string, constructor Constructor) error {
	if !constructors.Put(name, constructor) {
		return fmt.Errorf("openfeature provider %q already registered", name)
	}
	return nil
}

// Get returns the constructor registered for name, if any.
func Get(name string) (Constructor, bool) {
	return constructors.Get(name)
}
