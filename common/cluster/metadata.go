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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/service/config"
	"github.com/uber/cadence/common/service/dynamicconfig"
)

type (
	// Metadata provides information about clusters
	Metadata interface {
		// IsGlobalDomainEnabled whether the global domain is enabled,
		// this attr should be discarded when cross DC is made public
		IsGlobalDomainEnabled() bool
		// IsMasterCluster whether current cluster is master cluster
		IsMasterCluster() bool
		// GetNextFailoverVersion return the next failover version for domain failover
		GetNextFailoverVersion(string, int64) int64
		// IsVersionFromSameCluster return true if 2 version are used for the same cluster
		IsVersionFromSameCluster(version1 int64, version2 int64) bool
		// GetMasterClusterName return the master cluster name
		GetMasterClusterName() string
		// GetCurrentClusterName return the current cluster name
		GetCurrentClusterName() string
		// GetAllClusterInfo return the all cluster name -> corresponding info
		GetAllClusterInfo() map[string]config.ClusterInformation
		// ClusterNameForFailoverVersion return the corresponding cluster name for a given failover version
		ClusterNameForFailoverVersion(failoverVersion int64) string

		// HistoryArchivalConfig returns the history archival config of the cluster
		HistoryArchivalConfig() *ArchivalConfig
		// VisibilityArchivalConfig returns the visibility archival config of the cluster
		VisibilityArchivalConfig() *ArchivalConfig
	}

	metadataImpl struct {
		logger log.Logger
		// EnableGlobalDomain whether the global domain is enabled,
		// this attr should be discarded when cross DC is made public
		enableGlobalDomain dynamicconfig.BoolPropertyFn
		// failoverVersionIncrement is the increment of each cluster's version when failover happen
		failoverVersionIncrement int64
		// masterClusterName is the name of the master cluster, only the master cluster can register / update domain
		// all clusters can do domain failover
		masterClusterName string
		// currentClusterName is the name of the current cluster
		currentClusterName string
		// clusterInfo contains all cluster name -> corresponding information
		clusterInfo map[string]config.ClusterInformation
		// versionToClusterName contains all initial version -> corresponding cluster name
		versionToClusterName map[int64]string

		// historyArchivalConfig is cluster's history archival config
		historyArchivalConfig *ArchivalConfig
		// visibilityArchivalConfig is cluster's visibility archival config
		visibilityArchivalConfig *ArchivalConfig
	}
)

// NewMetadata create a new instance of Metadata
func NewMetadata(
	logger log.Logger,
	dc *dynamicconfig.Collection,
	enableGlobalDomain bool,
	failoverVersionIncrement int64,
	masterClusterName string,
	currentClusterName string,
	clusterInfo map[string]config.ClusterInformation,
	archivalClusterConfig config.Archival,
	archivalDomainDefault config.ArchivalDomainDefaults,
) Metadata {

	if len(clusterInfo) == 0 {
		panic("Empty cluster information")
	} else if len(masterClusterName) == 0 {
		panic("Master cluster name is empty")
	} else if len(currentClusterName) == 0 {
		panic("Current cluster name is empty")
	} else if failoverVersionIncrement == 0 {
		panic("Version increment is 0")
	}

	versionToClusterName := make(map[int64]string)
	for clusterName, info := range clusterInfo {
		if failoverVersionIncrement <= info.InitialFailoverVersion || info.InitialFailoverVersion < 0 {
			panic(fmt.Sprintf(
				"Version increment %v is smaller than initial version: %v.",
				failoverVersionIncrement,
				info.InitialFailoverVersion,
			))
		}
		if len(clusterName) == 0 {
			panic("Cluster name in all cluster names is empty")
		}
		versionToClusterName[info.InitialFailoverVersion] = clusterName

		if info.Enabled && (len(info.RPCName) == 0 || len(info.RPCAddress) == 0) {
			panic(fmt.Sprintf("Cluster %v: rpc name / address is empty", clusterName))
		}
	}

	if _, ok := clusterInfo[currentClusterName]; !ok {
		panic("Current cluster is not specified in cluster info")
	}
	if _, ok := clusterInfo[masterClusterName]; !ok {
		panic("Master cluster is not specified in cluster info")
	}
	if len(versionToClusterName) != len(clusterInfo) {
		panic("Cluster info initial versions have duplicates")
	}

	clusterHistoryArchivalStatus := dc.GetStringProperty(dynamicconfig.HistoryArchivalStatus, archivalClusterConfig.History.Status)()
	enableReadFromHistoryArchival := dc.GetBoolProperty(dynamicconfig.EnableReadFromHistoryArchival, archivalClusterConfig.History.EnableRead)()
	clusterStatus, err := getClusterArchivalStatus(clusterHistoryArchivalStatus)
	if err != nil {
		panic(err)
	}
	domainStatus, err := getDomainArchivalStatus(archivalDomainDefault.History.Status)
	if err != nil {
		panic(err)
	}
	historyArchivalConfig := NewArchivalConfig(clusterStatus, enableReadFromHistoryArchival, domainStatus, archivalDomainDefault.History.URI)

	clusterVisibilityArchivalStatus := dc.GetStringProperty(dynamicconfig.VisibilityArchivalStatus, archivalClusterConfig.Visibility.Status)()
	enableReadFromVisibilityArchival := dc.GetBoolProperty(dynamicconfig.EnableReadFromVisibilityArchival, archivalClusterConfig.Visibility.EnableRead)()
	clusterStatus, err = getClusterArchivalStatus(clusterVisibilityArchivalStatus)
	if err != nil {
		panic(err)
	}
	domainStatus, err = getDomainArchivalStatus(archivalDomainDefault.Visibility.Status)
	if err != nil {
		panic(err)
	}
	visibilityArchivalConfig := NewArchivalConfig(clusterStatus, enableReadFromVisibilityArchival, domainStatus, archivalDomainDefault.Visibility.URI)

	return &metadataImpl{
		logger:                   logger,
		enableGlobalDomain:       dc.GetBoolProperty(dynamicconfig.EnableGlobalDomain, enableGlobalDomain),
		failoverVersionIncrement: failoverVersionIncrement,
		masterClusterName:        masterClusterName,
		currentClusterName:       currentClusterName,
		clusterInfo:              clusterInfo,
		versionToClusterName:     versionToClusterName,
		historyArchivalConfig:    historyArchivalConfig,
		visibilityArchivalConfig: visibilityArchivalConfig,
	}
}

// IsGlobalDomainEnabled whether the global domain is enabled,
// this attr should be discarded when cross DC is made public
func (metadata *metadataImpl) IsGlobalDomainEnabled() bool {
	return metadata.enableGlobalDomain()
}

// GetNextFailoverVersion return the next failover version based on input
func (metadata *metadataImpl) GetNextFailoverVersion(cluster string, currentFailoverVersion int64) int64 {
	info, ok := metadata.clusterInfo[cluster]
	if !ok {
		panic(fmt.Sprintf(
			"Unknown cluster name: %v with given cluster initial failover version map: %v.",
			cluster,
			metadata.clusterInfo,
		))
	}
	failoverVersion := currentFailoverVersion/metadata.failoverVersionIncrement*metadata.failoverVersionIncrement + info.InitialFailoverVersion
	if failoverVersion < currentFailoverVersion {
		return failoverVersion + metadata.failoverVersionIncrement
	}
	return failoverVersion
}

// IsVersionFromSameCluster return true if 2 version are used for the same cluster
func (metadata *metadataImpl) IsVersionFromSameCluster(version1 int64, version2 int64) bool {
	return (version1-version2)%metadata.failoverVersionIncrement == 0
}

func (metadata *metadataImpl) IsMasterCluster() bool {
	return metadata.masterClusterName == metadata.currentClusterName
}

// GetMasterClusterName return the master cluster name
func (metadata *metadataImpl) GetMasterClusterName() string {
	return metadata.masterClusterName
}

// GetCurrentClusterName return the current cluster name
func (metadata *metadataImpl) GetCurrentClusterName() string {
	return metadata.currentClusterName
}

// GetAllClusterInfo return the all cluster name -> corresponding information
func (metadata *metadataImpl) GetAllClusterInfo() map[string]config.ClusterInformation {
	return metadata.clusterInfo
}

// ClusterNameForFailoverVersion return the corresponding cluster name for a given failover version
func (metadata *metadataImpl) ClusterNameForFailoverVersion(failoverVersion int64) string {
	if failoverVersion == common.EmptyVersion {
		return metadata.currentClusterName
	}

	initialFailoverVersion := failoverVersion % metadata.failoverVersionIncrement
	clusterName, ok := metadata.versionToClusterName[initialFailoverVersion]
	if !ok {
		panic(fmt.Sprintf(
			"Unknown initial failover version %v with given cluster initial failover version map: %v and failover version increment %v.",
			initialFailoverVersion,
			metadata.clusterInfo,
			metadata.failoverVersionIncrement,
		))
	}
	return clusterName
}

// HistoryArchivalConfig returns the history archival config of the cluster.
func (metadata *metadataImpl) HistoryArchivalConfig() *ArchivalConfig {
	return metadata.historyArchivalConfig
}

// VisibilityArchivalConfig returns the visibility archival config of the cluster.
func (metadata *metadataImpl) VisibilityArchivalConfig() *ArchivalConfig {
	return metadata.visibilityArchivalConfig
}
