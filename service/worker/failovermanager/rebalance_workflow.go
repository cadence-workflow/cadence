// Copyright (c) 2017-2021 Uber Technologies Inc.
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

package failovermanager

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

type (
	// RebalanceClusterAttribute specifies the preferred cluster for a scope:name pair,
	// mirroring the []ClusterAttribute input of the failover workflow but with an added
	// preferred-cluster dimension.
	RebalanceClusterAttribute struct {
		Scope            string `json:"scope"`
		Name             string `json:"name"`
		PreferredCluster string `json:"preferredCluster"`
	}

	// RebalanceParams contains parameters for rebalance workflow
	RebalanceParams struct {
		// BatchFailoverSize is number of domains to failover in one batch
		BatchFailoverSize int
		// BatchFailoverWaitTimeInSeconds is the waiting time between batch failover
		BatchFailoverWaitTimeInSeconds int
		// ClusterAttributePreferences is the list of preferred clusters for active-active scopes.
		// Duplicate scope:name pairs with the same preferredCluster are silently collapsed;
		// conflicting preferredClusters for the same scope:name cause the workflow to fail.
		ClusterAttributePreferences []RebalanceClusterAttribute `json:",omitempty"`
	}

	// RebalanceResult contains the result from the rebalance workflow
	RebalanceResult struct {
		SuccessDomains []string
		FailedDomains  []string
	}

	// ClusterAttributeUpdate identifies a single scope/attribute/cluster failover target.
	ClusterAttributeUpdate struct {
		Scope            string
		AttributeName    string
		PreferredCluster string
	}

	// DomainRebalanceData contains all rebalance updates for a single domain.
	// At least one of PreferredCluster or ClusterAttributeUpdates will be non-empty.
	// Both may be set when a domain needs its ActiveClusterName updated alongside cluster attribute updates.
	DomainRebalanceData struct {
		DomainName string
		// PreferredCluster is the new ActiveClusterName (if the domain-level active cluster needs updating).
		PreferredCluster string
		// ClusterAttributeUpdates lists per-attribute updates to apply; each has its own preferred cluster.
		ClusterAttributeUpdates []*ClusterAttributeUpdate
	}

	// GetDomainsForRebalanceActivityParams is the params for GetDomainsForRebalanceActivity
	GetDomainsForRebalanceActivityParams struct {
		ClusterAttributeRebalanceMap ClusterAttributeRebalanceMap
	}
)

// RebalanceWorkflow rebalances domains across clusters based on rebalance policy.
// It fetches all domains that need rebalancing in one step, then applies the updates in batches.
func RebalanceWorkflow(ctx workflow.Context, params *RebalanceParams) (*RebalanceResult, error) {
	clusterAttributeRebalanceMap, err := rebalanceClusterAttributesToMap(params.ClusterAttributePreferences)
	if err != nil {
		return nil, err
	}
	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	activityParams := &GetDomainsForRebalanceActivityParams{ClusterAttributeRebalanceMap: clusterAttributeRebalanceMap}
	var domainData []*DomainRebalanceData
	if err := workflow.ExecuteActivity(ao, getRebalanceDomainsActivityName, activityParams).Get(ctx, &domainData); err != nil {
		return nil, err
	}

	result := &RebalanceResult{SuccessDomains: []string{}, FailedDomains: []string{}}

	batchSize := params.BatchFailoverSize
	if batchSize <= 0 {
		batchSize = defaultBatchFailoverSize
	}
	waitTime := time.Duration(params.BatchFailoverWaitTimeInSeconds) * time.Second

	failoverAO := workflow.WithActivityOptions(ctx, getFailoverActivityOptions())
	for i := 0; i < len(domainData); i += batchSize {
		end := i + batchSize
		if end > len(domainData) {
			end = len(domainData)
		}
		batch := domainData[i:end]

		var batchResult RebalanceResult
		if err := workflow.ExecuteActivity(failoverAO, rebalanceDomainsActivityName, batch).Get(ctx, &batchResult); err != nil {
			result.FailedDomains = append(result.FailedDomains, getDomainNames(batch)...)
		} else {
			result.SuccessDomains = append(result.SuccessDomains, batchResult.SuccessDomains...)
			result.FailedDomains = append(result.FailedDomains, batchResult.FailedDomains...)
		}

		if i+batchSize < len(domainData) && waitTime > 0 {
			_ = workflow.Sleep(ctx, waitTime)
		}
	}

	return result, nil
}

// GetDomainsForRebalanceActivity fetches domains eligible for rebalance.
// For each domain, the domain-level ActiveClusterName and per-attribute cluster checks are evaluated
// independently and additively — a domain may need both its ActiveClusterName and cluster attributes updated.
func GetDomainsForRebalanceActivity(ctx context.Context, params *GetDomainsForRebalanceActivityParams) ([]*DomainRebalanceData, error) {
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	var res []*DomainRebalanceData
	for _, domain := range domains {
		var entry *DomainRebalanceData

		if shouldAllowRebalance(domain) {
			entry = &DomainRebalanceData{
				DomainName:       domain.GetDomainInfo().GetName(),
				PreferredCluster: getPreferredClusterName(domain),
			}
		}

		if params != nil && len(params.ClusterAttributeRebalanceMap) > 0 {
			attrUpdates := clusterAttributeUpdatesForActiveActiveDomain(domain, params.ClusterAttributeRebalanceMap)
			if len(attrUpdates) > 0 {
				if entry == nil {
					entry = &DomainRebalanceData{DomainName: domain.GetDomainInfo().GetName()}
				}
				entry.ClusterAttributeUpdates = attrUpdates
			}
		}

		if entry != nil {
			res = append(res, entry)
		}
	}
	return res, nil
}

// RebalanceDomainsActivity applies the rebalance updates from DomainRebalanceData to each domain.
// A single UpdateDomain call is made per domain; it may set ActiveClusterName, ActiveClusters, or both.
func RebalanceDomainsActivity(ctx context.Context, domainData []*DomainRebalanceData) (*RebalanceResult, error) {
	frontendClient := getClient(ctx)
	var successDomains, failedDomains []string
	for _, d := range domainData {
		activity.RecordHeartbeat(ctx, d.DomainName)
		req := &types.UpdateDomainRequest{Name: d.DomainName}
		if d.PreferredCluster != "" {
			req.ActiveClusterName = common.StringPtr(d.PreferredCluster)
		}
		if len(d.ClusterAttributeUpdates) > 0 {
			activeClusters := &types.ActiveClusters{
				AttributeScopes: make(map[string]types.ClusterAttributeScope),
			}
			for _, u := range d.ClusterAttributeUpdates {
				if _, ok := activeClusters.AttributeScopes[u.Scope]; !ok {
					activeClusters.AttributeScopes[u.Scope] = types.ClusterAttributeScope{
						ClusterAttributes: make(map[string]types.ActiveClusterInfo),
					}
				}
				scope := activeClusters.AttributeScopes[u.Scope]
				scope.ClusterAttributes[u.AttributeName] = types.ActiveClusterInfo{ActiveClusterName: u.PreferredCluster}
				activeClusters.AttributeScopes[u.Scope] = scope
			}
			req.ActiveClusters = activeClusters
		}
		if _, err := frontendClient.UpdateDomain(ctx, req); err != nil {
			failedDomains = append(failedDomains, d.DomainName)
		} else {
			successDomains = append(successDomains, d.DomainName)
		}
	}
	return &RebalanceResult{SuccessDomains: successDomains, FailedDomains: failedDomains}, nil
}

// clusterAttributeUpdatesForActiveActiveDomain returns one ClusterAttributeUpdate per active-active domain
// attribute that differs from the ClusterAttributeRebalanceMap preference, or nil if nothing needs to change.
func clusterAttributeUpdatesForActiveActiveDomain(domain *types.DescribeDomainResponse, clusterAttributeRebalanceMap ClusterAttributeRebalanceMap) []*ClusterAttributeUpdate {
	if !isDomainFailoverManagedByCadence(domain) || !domain.IsGlobalDomain {
		return nil
	}
	activeClusters := domain.ReplicationConfiguration.GetActiveClusters()
	if activeClusters == nil || len(activeClusters.AttributeScopes) == 0 {
		return nil
	}
	var updates []*ClusterAttributeUpdate
	for scopeType, scope := range activeClusters.AttributeScopes {
		attrMapping, ok := clusterAttributeRebalanceMap[clusterAttributeScope(scopeType)]
		if !ok {
			continue
		}
		for attrName, clusterInfo := range scope.ClusterAttributes {
			preferredCluster, ok := attrMapping[clusterAttributeName(attrName)]
			if !ok || clusterInfo.ActiveClusterName == preferredCluster {
				continue
			}
			if !isPreferredClusterInClusterListForDomain(preferredCluster, domain) {
				continue
			}
			updates = append(updates, &ClusterAttributeUpdate{
				Scope:            scopeType,
				AttributeName:    attrName,
				PreferredCluster: preferredCluster,
			})
		}
	}
	return updates
}

// rebalanceClusterAttributesToMap converts the slice input to the internal ClusterAttributeRebalanceMap.
// Identical duplicates (same scope, name, and preferredCluster) are silently collapsed.
// An error is returned if the same scope:name pair appears with two different preferredCluster values.
func rebalanceClusterAttributesToMap(attrs []RebalanceClusterAttribute) (ClusterAttributeRebalanceMap, error) {
	if len(attrs) == 0 {
		return nil, nil
	}
	result := make(ClusterAttributeRebalanceMap, len(attrs))
	for _, attr := range attrs {
		scope := clusterAttributeScope(attr.Scope)
		name := clusterAttributeName(attr.Name)
		if _, ok := result[scope]; !ok {
			result[scope] = make(ClusterAttributeToClusterMap)
		}
		if existing, exists := result[scope][name]; exists && existing != attr.PreferredCluster {
			return nil, fmt.Errorf("conflicting preferred cluster for scope=%q name=%q: %q vs %q", attr.Scope, attr.Name, existing, attr.PreferredCluster)
		}
		result[scope][name] = attr.PreferredCluster
	}
	return result, nil
}

func getPreferredClusterName(domain *types.DescribeDomainResponse) string {
	return domain.GetDomainInfo().GetData()[constants.DomainDataKeyForPreferredCluster]
}

func shouldAllowRebalance(domain *types.DescribeDomainResponse) bool {
	preferredCluster := getPreferredClusterName(domain)
	return isDomainFailoverManagedByCadence(domain) &&
		domain.IsGlobalDomain &&
		domain.GetDomainInfo().GetStatus() == types.DomainStatusRegistered &&
		len(preferredCluster) != 0 &&
		preferredCluster != domain.ReplicationConfiguration.GetActiveClusterName() &&
		isPreferredClusterInClusterListForDomain(preferredCluster, domain)
}

func getDomainNames(data []*DomainRebalanceData) []string {
	names := make([]string, len(data))
	for i, d := range data {
		names[i] = d.DomainName
	}
	return names
}

func isPreferredClusterInClusterListForDomain(preferredCluster string, domain *types.DescribeDomainResponse) bool {
	for _, cluster := range domain.ReplicationConfiguration.Clusters {
		if cluster.ClusterName == preferredCluster {
			return true
		}
	}
	return false
}
