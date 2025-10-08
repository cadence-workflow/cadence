package domain

import (
	"github.com/uber/cadence/common/types"
)

func mergeActiveActiveScopes(existingDomain *types.ActiveClusters, incomingTask *types.ActiveClusters) (*types.ActiveClusters, bool) {

	if existingDomain == nil && incomingTask == nil {
		return nil, false
	}

	// new active clusters are being added
	if existingDomain == nil && incomingTask != nil {
		return incomingTask, true
	}

	// existing data is not being mutated
	if incomingTask == nil && existingDomain != nil {
		return existingDomain, false
	}

	isChanged := false
	mergedActiveClusters := &types.ActiveClusters{
		AttributeScopes: make(map[string]types.ClusterAttributeScope),
	}

	for scope, existingScopeData := range existingDomain.AttributeScopes {
		incomingScope, presentInIncoming := incomingTask.AttributeScopes[scope]
		if presentInIncoming {
			merged, isCh := mergeScope(existingScopeData, incomingScope)
			isChanged = isChanged || isCh
			mergedActiveClusters.AttributeScopes[scope] = merged
		} else {
			mergedActiveClusters.AttributeScopes[scope] = existingScopeData
		}
	}

	for newScope, newScopeData := range incomingTask.AttributeScopes {
		_, existsAlready := mergedActiveClusters.AttributeScopes[newScope]
		if !existsAlready {
			mergedActiveClusters.AttributeScopes[newScope] = newScopeData
		}
	}

	return mergedActiveClusters, isChanged
}

func mergeScope(existing types.ClusterAttributeScope, incoming types.ClusterAttributeScope) (merged types.ClusterAttributeScope, isChanged bool) {

	merged.ClusterAttributes = make(map[string]types.ActiveClusterInfo)

	for existingAttr := range existing.ClusterAttributes {
		incomingAtt, presentInNew := incoming.ClusterAttributes[existingAttr]
		if presentInNew {
			if incomingAtt.FailoverVersion > existing.ClusterAttributes[existingAttr].FailoverVersion {
				merged.ClusterAttributes[existingAttr] = incomingAtt
				isChanged = true
			} else {
				merged.ClusterAttributes[existingAttr] = existing.ClusterAttributes[existingAttr]
			}
		} else {
			merged.ClusterAttributes[existingAttr] = existing.ClusterAttributes[existingAttr]
		}
	}

	for newAttr, newAttrClusterInfo := range incoming.ClusterAttributes {
		_, existsInOld := existing.ClusterAttributes[newAttr]
		if !existsInOld {
			merged.ClusterAttributes[newAttr] = newAttrClusterInfo
			isChanged = true
		}
	}

	return merged, isChanged
}
