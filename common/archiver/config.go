// Copyright (c) 2019 Uber Technologies, Inc.
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

package archiver

import "github.com/uber/cadence/common/config/yaml"

const (
	// FilestoreConfig is the config key for filestore archiver
	FilestoreConfig = "filestore"
	// S3storeConfig is the config key for S3 archiver
	S3storeConfig = "s3store"
)

type (
	// ArchivalDomainDefaults is the default archival config for each domain
	ArchivalDomainDefaults struct {
		// History is the domain default history archival config for each domain
		History HistoryArchivalDomainDefaults `yaml:"history"`
		// Visibility is the domain default visibility archival config for each domain
		Visibility VisibilityArchivalDomainDefaults `yaml:"visibility"`
	}

	// HistoryArchivalDomainDefaults is the default history archival config for each domain
	HistoryArchivalDomainDefaults struct {
		// Status is the domain default status of history archival: enabled or disabled
		Status string `yaml:"status"`
		// URI is the domain default URI for history archiver
		URI string `yaml:"URI"`
	}

	// VisibilityArchivalDomainDefaults is the default visibility archival config for each domain
	VisibilityArchivalDomainDefaults struct {
		// Status is the domain default status of visibility archival: enabled or disabled
		Status string `yaml:"status"`
		// URI is the domain default URI for visibility archiver
		URI string `yaml:"URI"`
	}

	// HistoryArchiverProvider contains the config for all history archivers.
	//
	// Because archivers support external plugins, so there is no fundamental structure expected,
	// but a top-level key per named store plugin is required, and will be used to select the
	// config for a plugin as it is initialized.
	//
	// Config keys and structures expected in the main default binary include:
	//  - FilestoreConfig: [*filestore.Config], used with provider scheme [github.com/uber/cadence/common/archiver/filestore.URIScheme]
	//  - S3storeConfig: [*s3store.Config], used with provider scheme [github.com/uber/cadence/common/archiver/s3store.URIScheme]
	//  - "gstorage" via [github.com/uber/cadence/common/archiver/gcloud.ConfigKey]: [github.com/uber/cadence/common/archiver/gcloud.Config], used with provider scheme "gs" [github.com/uber/cadence/common/archiver/gcloud.URIScheme]
	//
	// For handling hardcoded config, see yaml.ToNode.
	HistoryArchiverProvider map[string]*yaml.Node

	// VisibilityArchiverProvider contains the config for all visibility archivers.
	//
	// Because archivers support external plugins, so there is no fundamental structure expected,
	// but a top-level key per named store plugin is required, and will be used to select the
	// config for a plugin as it is initialized.
	//
	// Config keys and structures expected in the main default binary include:
	//  - FilestoreConfig: [*filestore.Config], used with provider scheme [github.com/uber/cadence/common/archiver/filestore.URIScheme]
	//  - S3storeConfig: [*s3store.Config], used with provider scheme [github.com/uber/cadence/common/archiver/s3store.URIScheme]
	//  - "gstorage" via [github.com/uber/cadence/common/archiver/gcloud.ConfigKey]: [github.com/uber/cadence/common/archiver/gcloud.Config], used with provider scheme "gs" [github.com/uber/cadence/common/archiver/gcloud.URIScheme]
	//
	// For handling hardcoded config, see yaml.ToNode.
	VisibilityArchiverProvider map[string]*yaml.Node
)
