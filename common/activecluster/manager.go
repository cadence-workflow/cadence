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

package activecluster

import (
	"context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

//go:generate mockgen -package $GOPACKAGE -destination manager_mock.go -self_package github.com/uber/cadence/common/activecluster github.com/uber/cadence/common/activecluster Manager

type Manager interface {
	common.Daemon

	ActiveCluster(ctx context.Context, domainID, wfID, rID string) (string, error)
	FailoverVersion(ctx context.Context, domainID, wfID, rID string) (int64, error)
}

type DomainIDToDomainFn func(id string) (*cache.DomainCacheEntry, error)

type manager struct {
	domainIDToDomainFn DomainIDToDomainFn
	clusterMetadata    cluster.Metadata
	metricsCl          metrics.Client
	logger             log.Logger
}

func NewManager(
	domainIDToDomainFn DomainIDToDomainFn,
	clusterMetadata cluster.Metadata,
	metricsCl metrics.Client,
	logger log.Logger,
) Manager {
	return &manager{
		domainIDToDomainFn: domainIDToDomainFn,
		clusterMetadata:    clusterMetadata,
		metricsCl:          metricsCl,
		logger:             logger.WithTags(tag.ComponentActiveRegionManager),
	}
}

func (m *manager) ActiveCluster(ctx context.Context, domainID, wfID, rID string) (string, error) {
	d, err := m.domainIDToDomainFn(domainID)
	if err != nil {
		return "", err
	}

	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return ActiveClusterName from domain entry
		return d.GetReplicationConfig().ActiveClusterName, nil
	}

	// TODO: Remove below fake implementation and implement properly
	// - lookup active region given <domain id, wf id, run id> from executions table RowType=ActiveCluster.
	// - cache this info
	// - add metrics for cache hit/miss
	// - return cluster name
	if wfID == "wf1" {
		return "cluster0", nil
	}
	if wfID == "wf2" {
		return "cluster1", nil
	}

	return d.GetReplicationConfig().ActiveClusterName, nil
}

func (m *manager) FailoverVersion(ctx context.Context, domainID, wfID, rID string) (int64, error) {
	// TODO: implement this
	return 0, nil
}

func (m *manager) Start() {
}

func (m *manager) Stop() {
}
