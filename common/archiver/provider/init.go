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

package provider

import (
	"fmt"

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/filestore"
	"github.com/uber/cadence/common/archiver/s3store"
	"github.com/uber/cadence/common/config"
)

func init() {
	// TODO: ideally remove this and handle per-instance registration during startup somehow,
	// as globals and inits have consistently caused issues.
	//
	// For now though, it's replacing a hard-coded switch statement, so an init func
	// is the most straightforward and should-be-identical conversion.

	must := func(err error) {
		if err != nil {
			panic(fmt.Errorf("failed to register default provider: %w", err))
		}
	}

	must(RegisterHistoryArchiver(filestore.URIScheme, config.FilestoreConfig, func(cfg *config.YamlNode, container *archiver.HistoryBootstrapContainer) (archiver.HistoryArchiver, error) {
		var out *config.FilestoreArchiver
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return filestore.NewHistoryArchiver(container, out)
	}))
	must(RegisterExecutionArchiver(filestore.URIScheme, config.FilestoreConfig, func(cfg *config.YamlNode, container *archiver.ExecutionBootstrapContainer) (archiver.ExecutionArchiver, error) {
		var out *config.FilestoreArchiver
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return filestore.NewExecutionArchiver(container, out)
	}))
	must(RegisterHistoryArchiver(s3store.URIScheme, config.S3storeConfig, func(cfg *config.YamlNode, container *archiver.HistoryBootstrapContainer) (archiver.HistoryArchiver, error) {
		var out *config.S3Archiver
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return s3store.NewHistoryArchiver(container, out)
	}))

	must(RegisterVisibilityArchiver(filestore.URIScheme, config.FilestoreConfig, func(cfg *config.YamlNode, container *archiver.VisibilityBootstrapContainer) (archiver.VisibilityArchiver, error) {
		var out *config.FilestoreArchiver
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return filestore.NewVisibilityArchiver(container, out)
	}))
	must(RegisterVisibilityArchiver(s3store.URIScheme, config.S3storeConfig, func(cfg *config.YamlNode, container *archiver.VisibilityBootstrapContainer) (archiver.VisibilityArchiver, error) {
		var out *config.S3Archiver
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return s3store.NewVisibilityArchiver(container, out)
	}))
}
