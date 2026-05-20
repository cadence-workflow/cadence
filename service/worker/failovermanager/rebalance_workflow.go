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
	"time"

	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

type (
	// ClusterAttributePreference specifies which cadence cluster a given scope:name attribute should prefer.
	ClusterAttributePreference struct {
		Scope            string `json:"scope"`
		Name             string `json:"name"`
		PreferredCluster string `json:"preferredCluster"`
	}

	// GetDomainsForRebalanceActivityParams params for GetDomainsForRebalanceActivity
	GetDomainsForRebalanceActivityParams struct {
		ClusterAttributePreferences []ClusterAttributePreference
	}

	// RebalanceParams contains parameters for rebalance workflow
	RebalanceParams struct {
		// BatchFailoverSize is number of domains to failover in one batch
		BatchFailoverSize int
		// BatchFailoverWaitTimeInSeconds is the waiting time between batch failover
		BatchFailoverWaitTimeInSeconds int
		// ClusterAttributePreferences defines the preferred cluster for each scope:name attribute.
		// When set, AA domains with mismatched attributes are included in the rebalance.
		ClusterAttributePreferences []ClusterAttributePreference
	}

	// RebalanceResult contains the result from the rebalance workflow
	RebalanceResult struct {
		SuccessDomains []string
		FailedDomains  []string
	}

	// DomainRebalanceData contains the result from getRebalanceDomains activity
	DomainRebalanceData struct {
		DomainName string
		// PreferredCluster is the domain-level target (from DomainDataKeyForPreferredCluster).
		// This becomes TargetCluster in FailoverActivity.
		PreferredCluster string
		// ClusterAttributeTargets holds per-attribute updates for AA domains.
		// Only mismatched attrs (current != preferred) are included.
		// Nil for non-AA domains or when all attributes already match.
		ClusterAttributeTargets []ClusterAttributeTarget
	}
)

// RebalanceWorkflow rebalances domains across clusters based on rebalance policy.
func RebalanceWorkflow(ctx workflow.Context, params *RebalanceParams) (*RebalanceResult, error) {
	batchSize := params.BatchFailoverSize
	if batchSize <= 0 {
		batchSize = defaultBatchFailoverSize
	}
	waitTime := params.BatchFailoverWaitTimeInSeconds
	if waitTime <= 0 {
		waitTime = defaultBatchFailoverWaitTimeInSeconds
	}

	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	activityParams := &GetDomainsForRebalanceActivityParams{
		ClusterAttributePreferences: params.ClusterAttributePreferences,
	}
	var domainData []*DomainRebalanceData
	err := workflow.ExecuteActivity(ao, GetDomainsForRebalanceActivity, activityParams).Get(ctx, &domainData)
	if err != nil {
		return nil, err
	}

	result := &RebalanceResult{
		SuccessDomains: []string{},
		FailedDomains:  []string{},
	}

	// Build per-domain preferences for FailoverActivity
	allPrefs := make([]DomainFailoverPreference, len(domainData))
	for i, d := range domainData {
		allPrefs[i] = DomainFailoverPreference{
			DomainName:              d.DomainName,
			TargetCluster:           d.PreferredCluster,
			ClusterAttributeTargets: d.ClusterAttributeTargets,
		}
	}

	// Batch and call FailoverActivity directly (each domain carries its own target)
	failoverAO := workflow.WithActivityOptions(ctx, getFailoverActivityOptions())
	total := len(allPrefs)
	times := total/batchSize + 1
	for i := 0; i < times; i++ {
		batch := allPrefs[i*batchSize : min((i+1)*batchSize, total)]
		if len(batch) == 0 {
			continue
		}
		actParams := &FailoverActivityParams{
			DomainPreferences: batch,
		}
		var actResult FailoverActivityResult
		err := workflow.ExecuteActivity(failoverAO, FailoverActivity, actParams).Get(ctx, &actResult)
		if err != nil {
			for _, p := range batch {
				result.FailedDomains = append(result.FailedDomains, p.DomainName)
			}
		} else {
			result.SuccessDomains = append(result.SuccessDomains, actResult.SuccessDomains...)
			result.FailedDomains = append(result.FailedDomains, actResult.FailedDomains...)
		}

		if i != times-1 {
			workflow.Sleep(ctx, time.Duration(waitTime)*time.Second)
		}
	}

	return result, nil
}

// GetDomainsForRebalanceActivity fetches domains that need rebalancing.
func GetDomainsForRebalanceActivity(ctx context.Context, params *GetDomainsForRebalanceActivityParams) ([]*DomainRebalanceData, error) {
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	var res []*DomainRebalanceData
	for _, domain := range domains {
		if !shouldAllowRebalance(domain, params.ClusterAttributePreferences) {
			continue
		}
		domainName := domain.GetDomainInfo().GetName()
		preferredCluster := getPreferredClusterName(domain)
		attrTargets := buildMismatchedTargets(domain, params.ClusterAttributePreferences)
		res = append(res, &DomainRebalanceData{
			DomainName:              domainName,
			PreferredCluster:        preferredCluster,
			ClusterAttributeTargets: attrTargets,
		})
	}
	return res, nil
}

func getPreferredClusterName(domain *types.DescribeDomainResponse) string {
	return domain.GetDomainInfo().GetData()[constants.DomainDataKeyForPreferredCluster]
}

// shouldAllowRebalance returns true if the domain should be included in a rebalance run.
// A domain is included when:
//  1. It is managed by Cadence, is global, and has registered status, AND
//  2. Either: its preferred cluster differs from the active cluster (domain-level change needed)
//     Or: it has active-active config and at least one attribute doesn't match the preferences.
func shouldAllowRebalance(domain *types.DescribeDomainResponse, prefs []ClusterAttributePreference) bool {
	if !isDomainFailoverManagedByCadence(domain) ||
		!domain.IsGlobalDomain ||
		domain.GetDomainInfo().GetStatus() == types.DomainStatusDeprecated ||
		domain.GetDomainInfo().GetStatus() == types.DomainStatusDeleted {
		return false
	}

	preferredCluster := getPreferredClusterName(domain)

	// Domain-level rebalance: preferred cluster is set and differs from the current active cluster
	if len(preferredCluster) > 0 &&
		preferredCluster != domain.ReplicationConfiguration.GetActiveClusterName() &&
		isPreferredClusterInClusterListForDomain(preferredCluster, domain) {
		return true
	}

	// Attribute-level rebalance: AA domain with at least one mismatched attribute
	if len(prefs) > 0 {
		return hasAttributesMismatch(domain, prefs)
	}

	return false
}

// hasAttributesMismatch returns true if any preference's attribute is currently on a different cluster.
func hasAttributesMismatch(domain *types.DescribeDomainResponse, prefs []ClusterAttributePreference) bool {
	ac := domain.ReplicationConfiguration.GetActiveClusters()
	if ac == nil {
		return false
	}
	for _, pref := range prefs {
		scope, ok := ac.AttributeScopes[pref.Scope]
		if !ok {
			continue
		}
		if info, ok := scope.ClusterAttributes[pref.Name]; ok {
			if info.ActiveClusterName != pref.PreferredCluster {
				return true
			}
		}
	}
	return false
}

// buildMismatchedTargets returns ClusterAttributeTargets for all attributes that don't match the preferences.
func buildMismatchedTargets(domain *types.DescribeDomainResponse, prefs []ClusterAttributePreference) []ClusterAttributeTarget {
	ac := domain.ReplicationConfiguration.GetActiveClusters()
	if ac == nil || len(prefs) == 0 {
		return nil
	}
	var targets []ClusterAttributeTarget
	for _, pref := range prefs {
		scope, ok := ac.AttributeScopes[pref.Scope]
		if !ok {
			continue
		}
		info, ok := scope.ClusterAttributes[pref.Name]
		if !ok {
			continue
		}
		if info.ActiveClusterName != pref.PreferredCluster {
			targets = append(targets, ClusterAttributeTarget{
				Attribute:     types.ClusterAttribute{Scope: pref.Scope, Name: pref.Name},
				TargetCluster: pref.PreferredCluster,
			})
		}
	}
	return targets
}

func isPreferredClusterInClusterListForDomain(preferredCluster string, domain *types.DescribeDomainResponse) bool {
	for _, cluster := range domain.ReplicationConfiguration.Clusters {
		if cluster.ClusterName == preferredCluster {
			return true
		}
	}
	return false
}
