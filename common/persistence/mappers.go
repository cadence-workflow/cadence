package persistence

import "github.com/uber/cadence/common/types"

// ToType converts persistence.GetDomainResponse to types.DescribeDomainResponse. Take care when using this mapper
// as not all the fields between the two types are symmetrical
// todo: Replace the ad-hoc mapping in the DescribeDomain Handler with this one
func (r *GetDomainResponse) ToType() *types.DescribeDomainResponse {
	if r == nil {
		return nil
	}

	var domainInfo *types.DomainInfo
	if r.Info != nil {
		// Validate and clamp Status to valid range (0-2: Registered, Deprecated, Deleted)
		// This prevents issues with invalid or out-of-range Status values
		status := r.Info.Status
		if status < 0 || status > 2 {
			status = 0 // Default to Registered for invalid values
		}
		domainStatus := types.DomainStatus(status)
		domainInfo = &types.DomainInfo{
			Name:        r.Info.Name,
			Status:      &domainStatus,
			Description: r.Info.Description,
			OwnerEmail:  r.Info.OwnerEmail,
			Data:        r.Info.Data,
			UUID:        r.Info.ID,
		}
	}

	var domainConfig *types.DomainConfiguration
	if r.Config != nil {
		domainConfig = &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: r.Config.Retention,
			EmitMetric:                             r.Config.EmitMetric,
			HistoryArchivalStatus:                  r.Config.HistoryArchivalStatus.Ptr(),
			HistoryArchivalURI:                     r.Config.HistoryArchivalURI,
			VisibilityArchivalStatus:               r.Config.VisibilityArchivalStatus.Ptr(),
			VisibilityArchivalURI:                  r.Config.VisibilityArchivalURI,
		}
		// Always set these pointers to maintain symmetry with FromDescribeDomainResponse
		// which always copies them when present
		domainConfig.BadBinaries = &r.Config.BadBinaries
		domainConfig.IsolationGroups = &r.Config.IsolationGroups
		domainConfig.AsyncWorkflowConfig = &r.Config.AsyncWorkflowConfig
	}

	var replicationConfig *types.DomainReplicationConfiguration
	if r.ReplicationConfig != nil {
		var clusters []*types.ClusterReplicationConfiguration
		// Convert clusters, preserving nil entries for consistent roundtrips
		if r.ReplicationConfig.Clusters != nil {
			clusters = make([]*types.ClusterReplicationConfiguration, len(r.ReplicationConfig.Clusters))
			for i, cluster := range r.ReplicationConfig.Clusters {
				if cluster != nil {
					clusters[i] = &types.ClusterReplicationConfiguration{
						ClusterName: cluster.ClusterName,
					}
				}
			}
		}

		replicationConfig = &types.DomainReplicationConfiguration{
			ActiveClusterName: r.ReplicationConfig.ActiveClusterName,
			Clusters:          clusters,
			ActiveClusters:    r.ReplicationConfig.ActiveClusters,
		}
	}

	// Always create FailoverInfo for consistent mapping
	failoverExpireTimestamp := int64(0)
	if r.FailoverEndTime != nil {
		failoverExpireTimestamp = *r.FailoverEndTime
	}

	return &types.DescribeDomainResponse{
		DomainInfo:               domainInfo,
		Configuration:            domainConfig,
		ReplicationConfiguration: replicationConfig,
		FailoverVersion:          r.FailoverVersion,
		IsGlobalDomain:           r.IsGlobalDomain,
		FailoverInfo: &types.FailoverInfo{
			FailoverVersion:         r.FailoverVersion,
			FailoverStartTimestamp:  r.LastUpdatedTime,
			FailoverExpireTimestamp: failoverExpireTimestamp,
			// Fields below are not present at the persistence layer
			CompletedShardCount: 0,
			PendingShards:       nil,
		},
		// note that failoverNotificationVersion is not present because the structs aren't symmetric
	}
}

// FromDescribeDomainResponse converts types.DescribeDomainResponse to persistence.GetDomainResponse
// care should be taken when using this because not all the fields between the two types are symmetrical
func FromDescribeDomainResponse(resp *types.DescribeDomainResponse) *GetDomainResponse {
	if resp == nil {
		return nil
	}

	var domainInfo *DomainInfo
	if resp.DomainInfo != nil {
		var status int
		if resp.DomainInfo.Status != nil {
			status = int(*resp.DomainInfo.Status)
		}
		domainInfo = &DomainInfo{
			ID:          resp.DomainInfo.UUID,
			Name:        resp.DomainInfo.Name,
			Status:      status,
			Description: resp.DomainInfo.Description,
			OwnerEmail:  resp.DomainInfo.OwnerEmail,
			Data:        resp.DomainInfo.Data,
		}
	}

	var domainConfig *DomainConfig
	if resp.Configuration != nil {
		config := &DomainConfig{
			Retention:  resp.Configuration.WorkflowExecutionRetentionPeriodInDays,
			EmitMetric: resp.Configuration.EmitMetric,
		}
		if resp.Configuration.HistoryArchivalStatus != nil {
			config.HistoryArchivalStatus = *resp.Configuration.HistoryArchivalStatus
		}
		config.HistoryArchivalURI = resp.Configuration.HistoryArchivalURI
		if resp.Configuration.VisibilityArchivalStatus != nil {
			config.VisibilityArchivalStatus = *resp.Configuration.VisibilityArchivalStatus
		}
		config.VisibilityArchivalURI = resp.Configuration.VisibilityArchivalURI
		if resp.Configuration.BadBinaries != nil {
			config.BadBinaries = *resp.Configuration.BadBinaries
		}
		if resp.Configuration.IsolationGroups != nil {
			config.IsolationGroups = *resp.Configuration.IsolationGroups
		}
		if resp.Configuration.AsyncWorkflowConfig != nil {
			config.AsyncWorkflowConfig = *resp.Configuration.AsyncWorkflowConfig
		}
		domainConfig = config
	}

	var replicationConfig *DomainReplicationConfig
	if resp.ReplicationConfiguration != nil {
		var clusters []*ClusterReplicationConfig
		// Convert clusters, preserving nil entries for consistent roundtrips
		if resp.ReplicationConfiguration.Clusters != nil {
			clusters = make([]*ClusterReplicationConfig, len(resp.ReplicationConfiguration.Clusters))
			for i, cluster := range resp.ReplicationConfiguration.Clusters {
				if cluster != nil {
					clusters[i] = &ClusterReplicationConfig{
						ClusterName: cluster.ClusterName,
					}
				}
			}
		}

		replicationConfig = &DomainReplicationConfig{
			ActiveClusterName: resp.ReplicationConfiguration.ActiveClusterName,
			Clusters:          clusters,
			ActiveClusters:    resp.ReplicationConfiguration.ActiveClusters,
		}
	}

	result := &GetDomainResponse{
		Info:              domainInfo,
		Config:            domainConfig,
		ReplicationConfig: replicationConfig,
		IsGlobalDomain:    resp.IsGlobalDomain,
		FailoverVersion:   resp.FailoverVersion,
		// FailoverNotificationVersion not mapped - not present in types.DescribeDomainResponse
		// PreviousFailoverVersion not mapped - not present in types.DescribeDomainResponse
		// FailoverEndTime not mapped - not reliably preserved in roundtrips
		// ConfigVersion not mapped - not present in types.DescribeDomainResponse
		// NotificationVersion not mapped - not present in types.DescribeDomainResponse
	}

	if resp.FailoverInfo != nil {
		result.LastUpdatedTime = resp.FailoverInfo.FailoverStartTimestamp
	}

	return result
}
