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
		Scope           string `json:"scope"`
		Name            string `json:"name"`
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

	// DomainRebalanceData contains the rebalance target for a single domain.
	// Exactly one of PreferredCluster or ClusterAttributeUpdates will be set.
	DomainRebalanceData struct {
		DomainName string
		// PreferredCluster is set for non-AA domains; it becomes the new ActiveClusterName.
		PreferredCluster string
		// ClusterAttributeUpdates is set for AA domains and contains all mismatched scope updates.
		ClusterAttributeUpdates *types.ActiveClusters
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
// Non-AA domains are included when they have a preferred cluster different from the active one.
// AA domains are included when any scope/attribute in the ClusterAttributeRebalanceMap differs from the current active cluster.
func GetDomainsForRebalanceActivity(ctx context.Context, params *GetDomainsForRebalanceActivityParams) ([]*DomainRebalanceData, error) {
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	var res []*DomainRebalanceData
	for _, domain := range domains {
		if shouldAllowRebalance(domain) {
			res = append(res, &DomainRebalanceData{
				DomainName:       domain.GetDomainInfo().GetName(),
				PreferredCluster: getPreferredClusterName(domain),
			})
			continue
		}
		if params != nil && len(params.ClusterAttributeRebalanceMap) > 0 {
			if entry := rebalanceEntryForScopedDomain(domain, params.ClusterAttributeRebalanceMap); entry != nil {
				res = append(res, entry)
			}
		}
	}
	return res, nil
}

// RebalanceDomainsActivity applies the rebalance updates from DomainRebalanceData to each domain.
// For non-AA domains it sets ActiveClusterName; for AA domains it sets ActiveClusters.
func RebalanceDomainsActivity(ctx context.Context, domainData []*DomainRebalanceData) (*RebalanceResult, error) {
	frontendClient := getClient(ctx)
	var successDomains, failedDomains []string
	for _, d := range domainData {
		activity.RecordHeartbeat(ctx, d.DomainName)
		req := &types.UpdateDomainRequest{Name: d.DomainName}
		if d.ClusterAttributeUpdates != nil {
			req.ActiveClusters = d.ClusterAttributeUpdates
		} else {
			req.ActiveClusterName = common.StringPtr(d.PreferredCluster)
		}
		if _, err := frontendClient.UpdateDomain(ctx, req); err != nil {
			failedDomains = append(failedDomains, d.DomainName)
		} else {
			successDomains = append(successDomains, d.DomainName)
		}
	}
	return &RebalanceResult{SuccessDomains: successDomains, FailedDomains: failedDomains}, nil
}

// rebalanceEntryForScopedDomain checks whether an AA domain has any scope/attribute that
// differs from the ClusterAttributeRebalanceMap preference and returns a DomainRebalanceData
// with the aggregated updates, or nil if nothing needs to change.
func rebalanceEntryForScopedDomain(domain *types.DescribeDomainResponse, clusterAttributeRebalanceMap ClusterAttributeRebalanceMap) *DomainRebalanceData {
	if !isDomainFailoverManagedByCadence(domain) || !domain.IsGlobalDomain {
		return nil
	}
	activeClusters := domain.ReplicationConfiguration.GetActiveClusters()
	if activeClusters == nil || len(activeClusters.AttributeScopes) == 0 {
		return nil
	}
	var updates *types.ActiveClusters
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
			if updates == nil {
				updates = &types.ActiveClusters{AttributeScopes: make(map[string]types.ClusterAttributeScope)}
			}
			if _, exists := updates.AttributeScopes[scopeType]; !exists {
				updates.AttributeScopes[scopeType] = types.ClusterAttributeScope{
					ClusterAttributes: make(map[string]types.ActiveClusterInfo),
				}
			}
			scopeEntry := updates.AttributeScopes[scopeType]
			scopeEntry.ClusterAttributes[attrName] = types.ActiveClusterInfo{ActiveClusterName: preferredCluster}
			updates.AttributeScopes[scopeType] = scopeEntry
		}
	}
	if updates == nil {
		return nil
	}
	return &DomainRebalanceData{
		DomainName:              domain.GetDomainInfo().GetName(),
		ClusterAttributeUpdates: updates,
	}
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
