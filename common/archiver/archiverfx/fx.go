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

package archiverfx

import (
	"fmt"

	uconfig "go.uber.org/config"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/dynamicconfig"
)

// Module provides archival components for fx application.
var Module = fx.Module("archiverfx",
	fx.Provide(New),
)

// Params are the dependencies for creating archival components.
type Params struct {
	fx.In

	Provider          uconfig.Provider
	DynamicCollection *dynamicconfig.Collection
}

// Result contains the archival components provided by this module.
type Result struct {
	fx.Out

	Metadata archiver.ArchivalMetadata
	Provider provider.ArchiverProvider
}

// New creates archival components via dependency injection.
func New(p Params) (Result, error) {
	var archival Archival
	if err := p.Provider.Get("archival").Populate(&archival); err != nil {
		return Result{}, fmt.Errorf("failed to decode archival config: %w", err)
	}

	var domainDefaults archiver.ArchivalDomainDefaults
	if err := p.Provider.Get("domainDefaults.archival").Populate(&domainDefaults); err != nil {
		return Result{}, fmt.Errorf("failed to decode domain archival defaults: %w", err)
	}

	if err := archival.Validate(&domainDefaults); err != nil {
		return Result{}, fmt.Errorf("archival config validation failed: %w", err)
	}

	metadata := archiver.NewArchivalMetadata(
		p.DynamicCollection,
		archival.History.Status,
		archival.History.EnableRead,
		archival.Visibility.Status,
		archival.Visibility.EnableRead,
		&domainDefaults,
	)

	archiverProvider := provider.NewArchiverProvider(
		archival.History.Provider,
		archival.Visibility.Provider,
	)

	return Result{
		Metadata: metadata,
		Provider: archiverProvider,
	}, nil
}
