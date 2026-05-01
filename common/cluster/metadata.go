// Copyright (c) 2018 Uber Technologies, Inc.
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

package cluster

import (
	"fmt"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type (
	// Metadata provides information about clusters
	Metadata struct {
		log     log.Logger
		metrics metrics.Scope

		// failoverVersionIncrement is the increment of each cluster's version when failover happen
		failoverVersionIncrement int64
		// primaryClusterName is the name of the primary cluster, only the primary cluster can register / update domain
		// all clusters can do domain failover
		primaryClusterName string
		// currentClusterName is the name of the current cluster
		currentClusterName string
		// allClusters contains all cluster info
		allClusters map[string]config.ClusterInformation
		// enabledClusters contains enabled info
		enabledClusters map[string]config.ClusterInformation
		// remoteClusters contains enabled and remote info
		remoteClusters map[string]config.ClusterInformation
		// versionToClusterName contains all initial version -> corresponding cluster name
		versionToClusterName map[int64]string
		// allows for a new failover version migration
		useNewFailoverVersionOverride dynamicproperties.BoolPropertyFnWithDomainFilter
	}
)

// NewMetadata create a new instance of Metadata
func NewMetadata(
	clusterGroupMetadata config.ClusterGroupMetadata,
	useMinFailoverVersionOverrideConfig dynamicproperties.BoolPropertyFnWithDomainFilter,
	metricsClient metrics.Client,
	logger log.Logger,
) Metadata {
	versionToClusterName := make(map[int64]string)
	for clusterName, info := range clusterGroupMetadata.ClusterGroup {
		versionToClusterName[info.InitialFailoverVersion] = clusterName
	}

	// We never use disable clusters, filter them out on start
	enabledClusters := map[string]config.ClusterInformation{}
	for cluster, info := range clusterGroupMetadata.ClusterGroup {
		if info.Enabled {
			enabledClusters[cluster] = info
		}
	}

	// Precompute remote clusters, they are used in multiple places
	remoteClusters := map[string]config.ClusterInformation{}
	for cluster, info := range enabledClusters {
		if cluster != clusterGroupMetadata.CurrentClusterName {
			remoteClusters[cluster] = info
		}
	}

	m := Metadata{
		log:                           logger,
		metrics:                       metricsClient.Scope(metrics.ClusterMetadataScope),
		failoverVersionIncrement:      clusterGroupMetadata.FailoverVersionIncrement,
		primaryClusterName:            clusterGroupMetadata.PrimaryClusterName,
		currentClusterName:            clusterGroupMetadata.CurrentClusterName,
		allClusters:                   clusterGroupMetadata.ClusterGroup,
		enabledClusters:               enabledClusters,
		remoteClusters:                remoteClusters,
		versionToClusterName:          versionToClusterName,
		useNewFailoverVersionOverride: useMinFailoverVersionOverrideConfig,
	}

	m.log.Info("cluster metadata created",
		tag.Dynamic("primary-cluster-name", m.primaryClusterName),
		tag.Dynamic("current-cluster-name", m.currentClusterName),
		tag.Dynamic("failover-version-increment", m.failoverVersionIncrement),
	)

	return m
}

// GetNextFailoverVersion returns the next valid FailoverVersion for a domain
func (m Metadata) GetNextFailoverVersion(targetClusterName string, currentFailoverVersion int64, domainName string) int64 {
	initialFailoverVersion := m.getInitialFailoverVersion(targetClusterName, domainName)
	failoverVersion := currentFailoverVersion/m.failoverVersionIncrement*m.failoverVersionIncrement + initialFailoverVersion
	if failoverVersion < currentFailoverVersion {
		return failoverVersion + m.failoverVersionIncrement
	}
	return failoverVersion
}

// IsVersionFromSameCluster return true if the new version is used for the same cluster
func (m Metadata) IsVersionFromSameCluster(version1 int64, version2 int64) bool {
	v1Server, err := m.resolveServerName(version1)
	if err != nil {
		// preserving old behaviour however, this should never occur
		m.metrics.IncCounter(metrics.ClusterMetadataFailureToResolveCounter)
		m.log.Error("could not resolve an incoming version", tag.Dynamic("failover-version", version1))
		return false
	}
	v2Server, err := m.resolveServerName(version2)
	if err != nil {
		m.log.Error("could not resolve an incoming version", tag.Dynamic("failover-version", version2))
		return false
	}
	return v1Server == v2Server
}

func (m Metadata) IsPrimaryCluster() bool {
	return m.primaryClusterName == m.currentClusterName
}

// GetCurrentClusterName return the current cluster name
func (m Metadata) GetCurrentClusterName() string {
	return m.currentClusterName
}

// GetAllClusterInfo return all cluster info
func (m Metadata) GetAllClusterInfo() map[string]config.ClusterInformation {
	return m.allClusters
}

// GetEnabledClusterInfo return enabled cluster info
func (m Metadata) GetEnabledClusterInfo() map[string]config.ClusterInformation {
	return m.enabledClusters
}

// GetRemoteClusterInfo return enabled AND remote cluster info
func (m Metadata) GetRemoteClusterInfo() map[string]config.ClusterInformation {
	return m.remoteClusters
}

// ClusterNameForFailoverVersion return the corresponding cluster name for a given failover version
func (m Metadata) ClusterNameForFailoverVersion(failoverVersion int64) (string, error) {
	if failoverVersion == constants.EmptyVersion {
		return m.currentClusterName, nil
	}
	server, err := m.resolveServerName(failoverVersion)
	if err != nil {
		m.metrics.IncCounter(metrics.ClusterMetadataResolvingFailoverVersionCounter)
		return "", fmt.Errorf("failed to resolve failover version to a cluster: %v", err)
	}
	return server, nil
}

// gets the initial failover version for a cluster / domain
// along with some helpers for a migration - should it be necessary
func (m Metadata) getInitialFailoverVersion(cluster string, domainName string) int64 {
	info, ok := m.allClusters[cluster]
	if !ok {
		panic(fmt.Sprintf(
			"Unknown cluster name: %v with given cluster initial failover version map: %v.",
			cluster,
			m.allClusters,
		))
	}

	// if using the minFailover Version during a cluster config, then return this from config
	// (assuming it's safe to do so). This is not the normal state of things and intended only
	// for when migrating versions.
	usingNewFailoverVersion := m.useNewFailoverVersionOverride(domainName)
	if usingNewFailoverVersion && info.NewInitialFailoverVersion != nil {
		m.log.Debug("using new failover version for cluster", tag.ClusterName(cluster), tag.WorkflowDomainName(domainName))
		m.metrics.IncCounter(metrics.ClusterMetadataGettingMinFailoverVersionCounter)
		return *info.NewInitialFailoverVersion
	}
	// default behaviour - return the initial failover version - a marker to
	// identify the cluster for all counters
	m.log.Debug("getting failover version for cluster", tag.ClusterName(cluster), tag.WorkflowDomainName(domainName))
	m.metrics.IncCounter(metrics.ClusterMetadataGettingFailoverVersionCounter)
	return info.InitialFailoverVersion
}

// resolves the server name from a version number. Better to use this
// than to check versionToClusterName directly, as this also falls back to catch
// when there's a migration NewInitialFailoverVersion
func (m Metadata) resolveServerName(originalVersion int64) (string, error) {
	version := originalVersion % m.failoverVersionIncrement
	// attempt a lookup first
	server, ok := m.versionToClusterName[version]
	if ok {
		return server, nil
	}

	// else fall back on checking for new failover versions
	for name, cluster := range m.allClusters {
		if cluster.NewInitialFailoverVersion != nil && *cluster.NewInitialFailoverVersion == version {
			return name, nil
		}
	}

	m.metrics.IncCounter(metrics.ClusterMetadataFailureToResolveCounter)
	return "", fmt.Errorf("could not resolve failover version: %d", originalVersion)
}
