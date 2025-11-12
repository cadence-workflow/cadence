// Copyright (c) 2020 Uber Technologies, Inc.
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

package constants

import (
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var (
	// TestVersion is the workflow version for test
	TestVersion = cluster.TestCurrentClusterInitialFailoverVersion + (cluster.TestFailoverVersionIncrement * 5)
	// TestDomainID is the domainID for test
	TestDomainID = "deadbeef-0123-4567-890a-bcdef0123456"
	// TestActiveActiveDomainID is the active active domainID for test
	TestActiveActiveDomainID = "965c78b7-1f6f-4122-9ba7-af6b7a55b27f"
	// TestActiveActiveDomainName is the active active domain name for test
	TestActiveActiveDomainName = "active-active-domain"
	// TestDomainName is the domainName for test
	TestDomainName = "some random domain name"
	// TestRateLimitedDomainName is the domain name for testing task processing rate limits
	TestRateLimitedDomainName = "rate-limited-domain-0123-4567-890a-bcdef0123456"
	// TestRateLimitedDomainID is the domain ID for testing task processing rate limits
	TestRateLimitedDomainID = "rate-limited-domain"
	// TestUnknownDomainID is the domain ID for testing unknown domains
	TestUnknownDomainID = "unknown-domain-id"
	// TestWorkflowID is the workflowID for test
	TestWorkflowID = "random-workflow-id"
	// TestRunID is the workflow runID for test
	TestRunID = "0d00698f-08e1-4d36-a3e2-3bf109f5d2d6"
	// TestRequestID is the request ID for test
	TestRequestID = "143b22cd-dfac-4d59-9398-893f89d89df6"
	// CronSkip
	CronSkip = types.CronOverlapPolicySkipped

	// TestClusterMetadata is the cluster metadata for test
	TestClusterMetadata = cluster.GetTestClusterMetadata(true)

	// TestActiveClusterSelectionPolicy is the active cluster selection policy for test
	TestActiveClusterSelectionPolicy = types.ActiveClusterSelectionPolicy{
		ClusterAttribute: &types.ClusterAttribute{
			Scope: "region",
			Name:  "us-east",
		},
	}

	// TestLocalDomainEntry is the local domain cache entry for test
	TestLocalDomainEntry = cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestDomainID, Name: TestDomainName},
		&persistence.DomainConfig{Retention: 1},
		cluster.TestCurrentClusterName,
	)

	// TestGlobalDomainEntry is the global domain cache entry for test
	TestGlobalDomainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestDomainID, Name: TestDomainName},
		&persistence.DomainConfig{
			Retention:                1,
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    "test:///visibility/archival",
		},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		TestVersion,
	)

	TestActiveActiveDomainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestActiveActiveDomainID, Name: TestActiveActiveDomainName},
		&persistence.DomainConfig{
			Retention:                1,
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    "test:///visibility/archival",
		},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-east": {
								ActiveClusterName: cluster.TestCurrentClusterName,
							},
							"us-west": {
								ActiveClusterName: cluster.TestAlternativeClusterName,
							},
						},
					},
				},
			},
		},
		TestVersion,
	)

	// TestRateLimitedDomainEntry is the global domain cache entry for test
	TestRateLimitedDomainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestRateLimitedDomainID, Name: TestRateLimitedDomainName},
		&persistence.DomainConfig{
			Retention:                1,
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    "test:///visibility/archival",
		},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		TestVersion,
	)

	// TestGlobalParentDomainEntry is the global parent domain cache entry for test
	TestGlobalParentDomainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestDomainID, Name: TestDomainName},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		TestVersion,
	)

	// TestGlobalTargetDomainEntry is the global target domain cache entry for test
	TestGlobalTargetDomainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestDomainID, Name: TestDomainName},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		TestVersion,
	)

	// TestGlobalChildDomainEntry is the global child domain cache entry for test
	TestGlobalChildDomainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestDomainID, Name: TestDomainName},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		TestVersion,
	)

	// TestGlobalStandbyDomainEntry is the global standby domain cache entry for test
	TestGlobalStandbyDomainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: TestDomainID, Name: TestDomainName},
		&persistence.DomainConfig{
			Retention:                1,
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    "test:///visibility/archival",
		},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		TestVersion,
	)
)
