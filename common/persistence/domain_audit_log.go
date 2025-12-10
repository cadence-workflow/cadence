package persistence

import (
	"sort"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

// ToFailoverEvent maps a DomainAuditLog to a FailoverEvent
// by looking at the failover changes and only listing the differences.
// Active clusters or cluster-attributes which are unchanged will not be listed as a failover event.
// Since one DomainAuditLog can contain multiple failover changes, this function will return a
// list of clusterFailver entries for each change. However, it repressents a single atomic
// change in the domain
func (auditLog *DomainAuditLog) ToFailoverEvents() *types.FailoverEvent {

	var clusterFailovers []*types.ClusterFailover

	// we're comparing the active cluster name rather than the failvoer version ()
	// because cluster-attribute updates are a noop bump, to filter out some noise the domain-level active
	// cluster will only be reported if it changes
	if auditLog.StateBefore.GetReplicationConfig().GetActiveClusterName() != auditLog.StateAfter.GetReplicationConfig().GetActiveClusterName() {
		clusterFailovers = append(clusterFailovers, &types.ClusterFailover{
			FromCluster: &types.ActiveClusterInfo{
				ActiveClusterName: auditLog.StateBefore.GetReplicationConfig().GetActiveClusterName(),
				FailoverVersion:   auditLog.StateBefore.GetFailoverVersion(),
			},
			ToCluster: &types.ActiveClusterInfo{
				ActiveClusterName: auditLog.StateAfter.GetReplicationConfig().GetActiveClusterName(),
				FailoverVersion:   auditLog.StateAfter.GetFailoverVersion(),
			},
		})
	}

	clusterAttrBefore := auditLog.StateBefore.GetReplicationConfig().GetClusterAttributeScopes()
	clusterAttrAfter := auditLog.StateAfter.GetReplicationConfig().GetClusterAttributeScopes()

	// Sort keys to ensure deterministic ordering
	sortedScopeKeysAfter := make([]string, 0, len(clusterAttrAfter))
	for scopeKey := range clusterAttrAfter {
		sortedScopeKeysAfter = append(sortedScopeKeysAfter, scopeKey)
	}
	sort.Strings(sortedScopeKeysAfter)

	// a scope is thing like 'region' or 'city'
	for _, scopeKeyAfter := range sortedScopeKeysAfter {
		vAfter := clusterAttrAfter[scopeKeyAfter]
		vBefore, ok := clusterAttrBefore[scopeKeyAfter]
		if !ok {
			// if the scope is not even defined in the before state, it's entirely new, this is its introduction
			clusterFailovers = append(clusterFailovers, auditLog.mapNewClusterAttributeToClusterFailover(scopeKeyAfter, &vAfter)...)
			continue
		}

		clusterFailovers = append(clusterFailovers, auditLog.mapClusterAttributeChangeToClusterFailover(scopeKeyAfter, &vBefore, &vAfter)...)
	}

	// Sort keys for deterministic ordering
	sortedScopeKeysBefore := make([]string, 0, len(clusterAttrBefore))
	for scopeKey := range clusterAttrBefore {
		sortedScopeKeysBefore = append(sortedScopeKeysBefore, scopeKey)
	}
	sort.Strings(sortedScopeKeysBefore)

	for _, scopeKeyBefore := range sortedScopeKeysBefore {
		vBefore := clusterAttrBefore[scopeKeyBefore]
		_, ok := clusterAttrAfter[scopeKeyBefore]
		if !ok {
			// if the scope is not even defined in the after state, it's entirely removed, this is its removal
			clusterFailovers = append(clusterFailovers, auditLog.mapRemovedClusterAttributeToClusterFailover(scopeKeyBefore, &vBefore)...)
			continue
		}
	}

	failoverType := types.FailoverTypeForce
	if auditLog.StateAfter.FailoverEndTime != nil {
		failoverType = types.FailoverTypeGraceful
	}

	return &types.FailoverEvent{
		ID:               &auditLog.EventID,
		CreatedTime:      common.Ptr(auditLog.CreatedTime.UnixNano()),
		ClusterFailovers: clusterFailovers,
		FailoverType:     &failoverType,
	}
}

// for when there's only a new cluster attribute introduced
func (auditLog *DomainAuditLog) mapNewClusterAttributeToClusterFailover(scopeKey string, vAfter *types.ClusterAttributeScope) []*types.ClusterFailover {
	if vAfter == nil && scopeKey == "" {
		return nil
	}
	var clusterFailovers []*types.ClusterFailover

	sortedNames := make([]string, 0, len(vAfter.ClusterAttributes))
	for name := range vAfter.ClusterAttributes {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		v := vAfter.ClusterAttributes[name]
		clusterFailovers = append(clusterFailovers, &types.ClusterFailover{
			ClusterAttribute: &types.ClusterAttribute{
				Scope: scopeKey,
				Name:  name,
			},
			ToCluster: &types.ActiveClusterInfo{
				ActiveClusterName: v.ActiveClusterName,
				FailoverVersion:   v.FailoverVersion,
			},
			FromCluster: nil,
		})
	}
	return clusterFailovers
}

// for when a cluster attribute is removed
func (auditLog *DomainAuditLog) mapRemovedClusterAttributeToClusterFailover(scopeKey string, vBefore *types.ClusterAttributeScope) []*types.ClusterFailover {
	if vBefore == nil && scopeKey == "" {
		return nil
	}
	var clusterFailovers []*types.ClusterFailover

	sortedNames := make([]string, 0, len(vBefore.ClusterAttributes))
	for name := range vBefore.ClusterAttributes {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		v := vBefore.ClusterAttributes[name]
		clusterFailovers = append(clusterFailovers, &types.ClusterFailover{
			ClusterAttribute: &types.ClusterAttribute{
				Scope: scopeKey,
				Name:  name,
			},
			FromCluster: &types.ActiveClusterInfo{
				ActiveClusterName: v.ActiveClusterName,
				FailoverVersion:   v.FailoverVersion,
			},
			ToCluster: nil,
		})
	}
	return clusterFailovers
}

// for comparing if any cluster attributes have changed within a scope
// will return nil if none are changed
// assumes that the cluster-attr scope is already checked to exist in both before and after states
func (auditLog *DomainAuditLog) mapClusterAttributeChangeToClusterFailover(scopeKey string, vBefore, vAfter *types.ClusterAttributeScope) []*types.ClusterFailover {

	var clusterFailovers []*types.ClusterFailover

	// Sort keys to ensure deterministic ordering
	sortedNamesAfter := make([]string, 0, len(vAfter.ClusterAttributes))
	for name := range vAfter.ClusterAttributes {
		sortedNamesAfter = append(sortedNamesAfter, name)
	}
	sort.Strings(sortedNamesAfter)

	for _, name := range sortedNamesAfter {
		vAfterAttr := vAfter.ClusterAttributes[name]
		vBeforeAttr, ok := vBefore.ClusterAttributes[name]
		if !ok {
			// new cluster attribute
			clusterFailovers = append(clusterFailovers, &types.ClusterFailover{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: scopeKey,
					Name:  name,
				},
				ToCluster: &types.ActiveClusterInfo{
					ActiveClusterName: vAfterAttr.ActiveClusterName,
					FailoverVersion:   vAfterAttr.FailoverVersion,
				},
				FromCluster: nil,
			})
			continue
		}

		// if the cluster attribute has changed
		if vBeforeAttr.FailoverVersion != vAfterAttr.FailoverVersion {
			clusterFailovers = append(clusterFailovers, &types.ClusterFailover{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: scopeKey,
					Name:  name,
				},
				FromCluster: &types.ActiveClusterInfo{
					ActiveClusterName: vBeforeAttr.ActiveClusterName,
					FailoverVersion:   vBeforeAttr.FailoverVersion,
				},
				ToCluster: &types.ActiveClusterInfo{
					ActiveClusterName: vAfterAttr.ActiveClusterName,
					FailoverVersion:   vAfterAttr.FailoverVersion,
				},
			})
		}
	}

	sortedNamesBefore := make([]string, 0, len(vBefore.ClusterAttributes))
	for name := range vBefore.ClusterAttributes {
		sortedNamesBefore = append(sortedNamesBefore, name)
	}
	sort.Strings(sortedNamesBefore)

	for _, name := range sortedNamesBefore {
		vBeforeAttr := vBefore.ClusterAttributes[name]
		_, ok := vAfter.ClusterAttributes[name]
		if !ok {
			// removed cluster attribute
			clusterFailovers = append(clusterFailovers, &types.ClusterFailover{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: scopeKey,
					Name:  name,
				},
				FromCluster: &types.ActiveClusterInfo{
					ActiveClusterName: vBeforeAttr.ActiveClusterName,
					FailoverVersion:   vBeforeAttr.FailoverVersion,
				},
				ToCluster: nil,
			})
			continue
		}
	}
	return clusterFailovers
}
