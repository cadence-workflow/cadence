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

package testdata

import (
	"time"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

func NewDomainRow(ts time.Time) *nosqlplugin.DomainRow {
	return &nosqlplugin.DomainRow{
		Info: &persistence.DomainInfo{
			ID:          "test-domain-id",
			Name:        "test-domain-name",
			Status:      persistence.DomainStatusRegistered,
			Description: "test-domain-description",
			OwnerEmail:  "test-domain-owner-email",
			Data:        map[string]string{"k1": "v1"},
		},
		Config: &persistence.InternalDomainConfig{
			Retention:                7 * 24 * time.Hour,
			EmitMetric:               true,
			ArchivalBucket:           "test-archival-bucket",
			ArchivalStatus:           types.ArchivalStatusEnabled,
			HistoryArchivalStatus:    types.ArchivalStatusEnabled,
			HistoryArchivalURI:       "test-history-archival-uri",
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    "test-visibility-archival-uri",
			BadBinaries:              &persistence.DataBlob{Encoding: "thriftrw", Data: []byte("bad-binaries")},
			IsolationGroups:          &persistence.DataBlob{Encoding: "thriftrw", Data: []byte("isolation-group")},
			AsyncWorkflowsConfig:     &persistence.DataBlob{Encoding: "thriftrw", Data: []byte("async-workflows-config")},
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "test-active-cluster-name",
			Clusters: []*persistence.ClusterReplicationConfig{
				{
					ClusterName: "test-cluster-name",
				},
			},
		},
		IsGlobalDomain:      true,
		ConfigVersion:       3,
		FailoverVersion:     4,
		FailoverEndTime:     &ts,
		LastUpdatedTime:     ts,
		NotificationVersion: 5,
	}
}
