// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package replicator

import (
	"time"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
)

type (
	// Replicator is the processor for replication tasks
	Replicator struct {
		clusterMetadata               cluster.Metadata
		domainReplicationTaskExecutor domain.ReplicationTaskExecutor
		clientBean                    client.Bean
		domainProcessors              []*domainReplicationProcessor
		logger                        log.Logger
		metricsClient                 metrics.Client
		hostInfo                      membership.HostInfo
		membershipResolver            membership.Resolver
		domainReplicationQueue        domain.ReplicationQueue
		replicationMaxRetry           time.Duration
	}
)

// NewReplicator creates a new replicator for processing replication tasks
func NewReplicator(
	clusterMetadata cluster.Metadata,
	clientBean client.Bean,
	logger log.Logger,
	metricsClient metrics.Client,
	hostInfo membership.HostInfo,
	membership membership.Resolver,
	domainReplicationQueue domain.ReplicationQueue,
	domainReplicationTaskExecutor domain.ReplicationTaskExecutor,
	replicationMaxRetry time.Duration,
) *Replicator {

	logger = logger.WithTags(tag.ComponentReplicator)
	return &Replicator{
		hostInfo:                      hostInfo,
		membershipResolver:            membership,
		clusterMetadata:               clusterMetadata,
		domainReplicationTaskExecutor: domainReplicationTaskExecutor,
		clientBean:                    clientBean,
		logger:                        logger,
		metricsClient:                 metricsClient,
		domainReplicationQueue:        domainReplicationQueue,
		replicationMaxRetry:           replicationMaxRetry,
	}
}

// Start is called to start replicator
func (r *Replicator) Start() error {
	currentClusterName := r.clusterMetadata.GetCurrentClusterName()
	for clusterName := range r.clusterMetadata.GetRemoteClusterInfo() {
		adminClient, err := r.clientBean.GetRemoteAdminClient(clusterName)
		if err != nil {
			return err
		}
		processor := newDomainReplicationProcessor(
			clusterName,
			currentClusterName,
			r.logger.WithTags(tag.ComponentReplicationTaskProcessor, tag.SourceCluster(clusterName)),
			adminClient,
			r.metricsClient,
			r.domainReplicationTaskExecutor,
			r.hostInfo,
			r.membershipResolver,
			r.domainReplicationQueue,
			r.replicationMaxRetry,
			clock.NewRealTimeSource(),
		)
		r.domainProcessors = append(r.domainProcessors, processor)
	}

	for _, domainProcessor := range r.domainProcessors {
		domainProcessor.Start()
	}

	return nil
}

// Stop is called to stop replicator
func (r *Replicator) Stop() {

	for _, domainProcessor := range r.domainProcessors {
		domainProcessor.Stop()
	}
}
