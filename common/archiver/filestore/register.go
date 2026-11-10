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
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package filestore

import (
	"fmt"

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/config/yaml"
)

func init() {
	// TODO: ideally remove this and handle per-instance registration during startup somehow,
	// as globals and inits have consistently caused issues.
	//
	// For now though, it's replacing a hard-coded switch statement, so an init func
	// is the most straightforward and should-be-identical conversion.

	must := func(err error) {
		if err != nil {
			panic(fmt.Errorf("failed to register filestore archiver: %w", err))
		}
	}

	// Register history archiver
	must(provider.RegisterHistoryArchiver(URIScheme, archiver.FilestoreConfig, func(cfg *yaml.Node, container *archiver.HistoryBootstrapContainer) (archiver.HistoryArchiver, error) {
		var out *Config
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return NewHistoryArchiver(container, out)
	}))

	// Register visibility archiver
	must(provider.RegisterVisibilityArchiver(URIScheme, archiver.FilestoreConfig, func(cfg *yaml.Node, container *archiver.VisibilityBootstrapContainer) (archiver.VisibilityArchiver, error) {
		var out *Config
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return NewVisibilityArchiver(container, out)
	}))
}
