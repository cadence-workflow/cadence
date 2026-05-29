// Copyright (c) 2017 Uber Technologies, Inc.
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

package domain

import (
	"fmt"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// AttrValidatorImpl is domain attr validator
	AttrValidatorImpl struct {
		clusterMetadata  cluster.Metadata
		minRetentionDays int32
	}
)

// newAttrValidator create a new domain attr validator
func newAttrValidator(
	clusterMetadata cluster.Metadata,
	minRetentionDays int32,
) *AttrValidatorImpl {

	return &AttrValidatorImpl{
		clusterMetadata:  clusterMetadata,
		minRetentionDays: minRetentionDays,
	}
}

func (d *AttrValidatorImpl) validateLocalDomainUpdateRequest(updateRequest *types.UpdateDomainRequest) error {
	if updateRequest.ActiveClusterName != nil || updateRequest.ActiveClusters != nil || updateRequest.Clusters != nil {
		return errLocalDomainsCannotFailover
	}
	if updateRequest.FailoverTimeoutInSeconds != nil {
		return errLocalDomainsCannotFailover
	}
	return nil
}

func (d *AttrValidatorImpl) validateDomainConfig(config *persistence.DomainConfig) error {
	if config.Retention < int32(d.minRetentionDays) {
		return errInvalidRetentionPeriod
	}
	if config.HistoryArchivalStatus == types.ArchivalStatusEnabled && len(config.HistoryArchivalURI) == 0 {
		return errInvalidArchivalConfig
	}
	if config.VisibilityArchivalStatus == types.ArchivalStatusEnabled && len(config.VisibilityArchivalURI) == 0 {
		return errInvalidArchivalConfig
	}
	return nil
}

func (d *AttrValidatorImpl) validateDomainReplicationConfigForLocalDomain(
	replicationConfig *persistence.DomainReplicationConfig,
) error {

	activeCluster := replicationConfig.ActiveClusterName
	clusters := replicationConfig.Clusters

	if err := d.validateClusterName(activeCluster); err != nil {
		return err
	}
	for _, clusterConfig := range clusters {
		if err := d.validateClusterName(clusterConfig.ClusterName); err != nil {
			return err
		}
	}

	if activeCluster != d.clusterMetadata.GetCurrentClusterName() {
		return &types.BadRequestError{Message: "Invalid local domain active cluster"}
	}

	if len(clusters) != 1 || clusters[0].ClusterName != activeCluster {
		return &types.BadRequestError{Message: "Invalid local domain clusters"}
	}

	return nil
}

func (d *AttrValidatorImpl) validateDomainReplicationConfigForGlobalDomain(
	replicationConfig *persistence.DomainReplicationConfig,
) error {
	// TODO: https://github.com/uber/cadence/issues/4345 add checking for "pending active" as well
	// Right now we only have checking if clusters to remove are "current active cluster" in this method.
	// However, there could be edge cases that a cluster is in "pending active" state during graceful failover.
	// It's better to do this check so that people won't make mistake.
	// However, this is not critical -- even this happens, they can add the active cluster back

	activeCluster := replicationConfig.ActiveClusterName
	clusters := replicationConfig.Clusters
	activeClusters := replicationConfig.ActiveClusters

	if activeCluster == "" {
		return errActiveClusterNameRequired
	}

	for _, clusterConfig := range clusters {
		if err := d.validateClusterName(clusterConfig.ClusterName); err != nil {
			return err
		}
	}

	isInClusters := func(clusterName string) bool {
		for _, clusterConfig := range clusters {
			if clusterConfig.ClusterName == clusterName {
				return true
			}
		}
		return false
	}

	if err := d.validateClusterName(activeCluster); err != nil {
		return err
	}

	if !isInClusters(activeCluster) {
		return errActiveClusterNotInClusters
	}
	// For active-active domains, also validate that all clusters in AttributeScopes are valid
	if activeClusters != nil && activeClusters.AttributeScopes != nil {
		for _, scope := range activeClusters.AttributeScopes {
			for _, cluster := range scope.ClusterAttributes {
				if err := d.validateClusterName(cluster.ActiveClusterName); err != nil {
					return err
				}

				if !isInClusters(cluster.ActiveClusterName) {
					return errActiveClusterNotInClusters
				}
			}
		}
	}

	return nil
}

func (d *AttrValidatorImpl) validateDomainReplicationConfigClustersDoesNotRemove(
	clustersOld []*persistence.ClusterReplicationConfig,
	clustersNew []*persistence.ClusterReplicationConfig,
) error {

	clusterNamesOld := make(map[string]bool)
	for _, clusterConfig := range clustersOld {
		clusterNamesOld[clusterConfig.ClusterName] = true
	}
	clusterNamesNew := make(map[string]bool)
	for _, clusterConfig := range clustersNew {
		clusterNamesNew[clusterConfig.ClusterName] = true
	}

	if len(clusterNamesNew) < len(clusterNamesOld) {
		return errCannotRemoveClustersFromDomain
	}

	for clusterName := range clusterNamesOld {
		if _, ok := clusterNamesNew[clusterName]; !ok {
			return errCannotRemoveClustersFromDomain
		}
	}
	return nil
}

func (d *AttrValidatorImpl) validateClusterName(
	clusterName string,
) error {

	if _, ok := d.clusterMetadata.GetEnabledClusterInfo()[clusterName]; !ok {
		return &types.BadRequestError{Message: fmt.Sprintf(
			"Invalid cluster name: %v",
			clusterName,
		)}
	}
	return nil
}

func (d *AttrValidatorImpl) validateActiveActiveDomainReplicationConfig(
	activeClusters *types.ActiveClusters,
) error {

	if activeClusters == nil || activeClusters.AttributeScopes == nil {
		return nil
	}

	clusters := d.clusterMetadata.GetEnabledClusterInfo()

	for _, scopeData := range activeClusters.AttributeScopes {
		for _, activeCluster := range scopeData.ClusterAttributes {
			_, ok := clusters[activeCluster.ActiveClusterName]
			if !ok {
				return &types.BadRequestError{Message: fmt.Sprintf(
					"Invalid active cluster name: %v",
					activeCluster.ActiveClusterName,
				)}
			}
			if activeCluster.FailoverVersion < 0 {
				return &types.BadRequestError{Message: fmt.Sprintf(
					"invalid failover version: %d",
					activeCluster.FailoverVersion,
				)}
			}
		}
	}
	return nil
}
