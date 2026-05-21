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

	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

const versionRebalanceClusterAttributes = "rebalance-cluster-attributes"

type (
	// RebalanceParams contains parameters for rebalance workflow
	RebalanceParams struct {
		// BatchFailoverSize is number of domains to failover in one batch
		BatchFailoverSize int
		// BatchFailoverWaitTimeInSeconds is the waiting time between batch failover
		BatchFailoverWaitTimeInSeconds int
		// ClusterAttributePreferences specifies the preferred cluster for each cluster attribute scope+name pair.
		// Used to rebalance active-active domains whose per-attribute active cluster doesn't match the preference.
		ClusterAttributePreferences []ClusterAttributePreference
	}

	// RebalanceResult contains the result from the rebalance workflow
	RebalanceResult struct {
		SuccessDomains []string
		FailedDomains  []string
	}

	// ClusterAttributePreference specifies the preferred cluster for a given cluster attribute scope+name.
	// This is the CLI input format for per-attribute rebalance preferences.
	ClusterAttributePreference struct {
		Scope            string
		Name             string
		PreferredCluster string
	}

	// GetDomainsForRebalanceActivityParams holds preferences passed from workflow to activity.
	GetDomainsForRebalanceActivityParams struct {
		ClusterAttributePreferences []ClusterAttributePreference
	}

	// DomainRebalanceData contains the result from getRebalanceDomains activity
	DomainRebalanceData struct {
		DomainName string
		// PreferredCluster is the domain-level preferred cluster; empty if no domain-level change is needed.
		PreferredCluster string
		// ClusterAttributeTargets holds per-attribute targets for mismatched active-active attributes.
		ClusterAttributeTargets []ClusterAttributeTarget
	}
)

// RebalanceWorkflow is to rebalance domains across clusters based on rebalance policy.
func RebalanceWorkflow(ctx workflow.Context, params *RebalanceParams) (*RebalanceResult, error) {
	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	var domainData []*DomainRebalanceData
	v := workflow.GetVersion(ctx, versionRebalanceClusterAttributes, workflow.DefaultVersion, 1)
	var err error
	if v >= 1 {
		actParams := &GetDomainsForRebalanceActivityParams{
			ClusterAttributePreferences: params.ClusterAttributePreferences,
		}
		err = workflow.ExecuteActivity(ao, getRebalanceDomainsActivityName, actParams).Get(ctx, &domainData)
	} else {
		err = workflow.ExecuteActivity(ao, getRebalanceDomainsActivityName).Get(ctx, &domainData)
	}
	if err != nil {
		return nil, err
	}

	specs := rebalanceDataToSpecs(domainData)
	rebalanceParams := &FailoverParams{
		BatchFailoverSize:              params.BatchFailoverSize,
		BatchFailoverWaitTimeInSeconds: params.BatchFailoverWaitTimeInSeconds,
	}
	result := &RebalanceResult{
		SuccessDomains: []string{},
		FailedDomains:  []string{},
	}
	result.SuccessDomains, result.FailedDomains = failoverDomainsByBatch(
		ctx,
		specs,
		rebalanceParams,
		func() {},
		false,
	)

	return result, nil
}

// GetDomainsForRebalanceActivity fetches domains eligible for rebalance.
func GetDomainsForRebalanceActivity(ctx context.Context, params *GetDomainsForRebalanceActivityParams) ([]*DomainRebalanceData, error) {
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	prefMap := buildPreferenceMap(params.GetClusterAttributePreferences())
	var res []*DomainRebalanceData
	for _, domain := range domains {
		if shouldAllowRebalance(domain, prefMap) {
			domainName := domain.GetDomainInfo().GetName()
			preferredCluster := getPreferredClusterName(domain)
			// Only set PreferredCluster if it's actually different from the active cluster.
			if preferredCluster == domain.ReplicationConfiguration.GetActiveClusterName() ||
				!isPreferredClusterInClusterListForDomain(preferredCluster, domain) {
				preferredCluster = ""
			}
			res = append(res, &DomainRebalanceData{
				DomainName:              domainName,
				PreferredCluster:        preferredCluster,
				ClusterAttributeTargets: buildMismatchedAttributeTargets(domain, prefMap),
			})
		}
	}
	return res, nil
}

// GetClusterAttributePreferences returns the preferences, safe to call on a nil receiver.
func (p *GetDomainsForRebalanceActivityParams) GetClusterAttributePreferences() []ClusterAttributePreference {
	if p == nil {
		return nil
	}
	return p.ClusterAttributePreferences
}

func getPreferredClusterName(domain *types.DescribeDomainResponse) string {
	return domain.GetDomainInfo().GetData()[constants.DomainDataKeyForPreferredCluster]
}

// shouldAllowRebalance returns true if the domain should be included in a rebalance run.
// A domain qualifies when it is managed by Cadence, is global, is registered, and has at least one
// of: a domain-level preferred cluster that differs from the active cluster, or an attribute
// whose active cluster doesn't match the preference map.
func shouldAllowRebalance(domain *types.DescribeDomainResponse, prefMap map[string]map[string]string) bool {
	if !isDomainFailoverManagedByCadence(domain) ||
		!domain.IsGlobalDomain ||
		domain.GetDomainInfo().GetStatus() != types.DomainStatusRegistered {
		return false
	}
	// Domain-level check.
	preferredCluster := getPreferredClusterName(domain)
	if len(preferredCluster) > 0 &&
		preferredCluster != domain.ReplicationConfiguration.GetActiveClusterName() &&
		isPreferredClusterInClusterListForDomain(preferredCluster, domain) {
		return true
	}
	// Attribute-level check for active-active domains.
	return len(buildMismatchedAttributeTargets(domain, prefMap)) > 0
}

// buildPreferenceMap converts []ClusterAttributePreference into a nested lookup map:
// scope → name → preferredCluster.
func buildPreferenceMap(prefs []ClusterAttributePreference) map[string]map[string]string {
	m := make(map[string]map[string]string)
	for _, p := range prefs {
		if m[p.Scope] == nil {
			m[p.Scope] = make(map[string]string)
		}
		m[p.Scope][p.Name] = p.PreferredCluster
	}
	return m
}

// buildMismatchedAttributeTargets returns ClusterAttributeTargets for all attributes in prefMap
// whose current active cluster in the domain differs from the preference.
func buildMismatchedAttributeTargets(domain *types.DescribeDomainResponse, prefMap map[string]map[string]string) []ClusterAttributeTarget {
	if len(prefMap) == 0 {
		return nil
	}
	ac := domain.ReplicationConfiguration.GetActiveClusters()
	if ac == nil {
		return nil
	}
	var targets []ClusterAttributeTarget
	for scope, names := range prefMap {
		for name, preferred := range names {
			info, err := ac.GetActiveClusterByClusterAttribute(scope, name)
			if err != nil {
				// Attribute not present in domain — skip.
				continue
			}
			if info.ActiveClusterName != preferred {
				targets = append(targets, ClusterAttributeTarget{
					Attribute:     types.ClusterAttribute{Scope: scope, Name: name},
					TargetCluster: preferred,
				})
			}
		}
	}
	return targets
}

// rebalanceDataToSpecs converts []DomainRebalanceData to []DomainFailoverSpec for use with failoverDomainsByBatch.
func rebalanceDataToSpecs(data []*DomainRebalanceData) []DomainFailoverSpec {
	specs := make([]DomainFailoverSpec, len(data))
	for i, d := range data {
		specs[i] = DomainFailoverSpec{
			DomainName:       d.DomainName,
			TargetCluster:    d.PreferredCluster,
			AttributeTargets: d.ClusterAttributeTargets,
		}
	}
	return specs
}

func isPreferredClusterInClusterListForDomain(preferredCluster string, domain *types.DescribeDomainResponse) bool {
	for _, cluster := range domain.ReplicationConfiguration.Clusters {
		if cluster.ClusterName == preferredCluster {
			return true
		}
	}
	return false
}
