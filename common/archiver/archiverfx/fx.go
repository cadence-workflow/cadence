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
	"go.uber.org/fx"

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
)

// Module provides archival components for fx application.
var Module = fx.Module("archiverfx",
	fx.Provide(NewArchivalMetadata),
	fx.Provide(NewArchiverProvider),
)

type archivalMetadataParams struct {
	fx.In

	DynamicCollection *dynamicconfig.Collection
	Config            config.Config
}

// NewArchivalMetadata creates an ArchivalMetadata instance via dependency injection.
func NewArchivalMetadata(p archivalMetadataParams) archiver.ArchivalMetadata {
	return archiver.NewArchivalMetadata(
		p.DynamicCollection,
		p.Config.Archival.History.Status,
		p.Config.Archival.History.EnableRead,
		p.Config.Archival.Visibility.Status,
		p.Config.Archival.Visibility.EnableRead,
		&p.Config.DomainDefaults.Archival,
	)
}

type archiverProviderParams struct {
	fx.In

	Config config.Config
}

// NewArchiverProvider creates an ArchiverProvider instance via dependency injection.
func NewArchiverProvider(p archiverProviderParams) provider.ArchiverProvider {
	return provider.NewArchiverProvider(
		p.Config.Archival.History.Provider,
		p.Config.Archival.Visibility.Provider,
	)
}
