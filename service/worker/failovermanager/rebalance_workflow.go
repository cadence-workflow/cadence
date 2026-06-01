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
	"encoding/json"
	"errors"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

type (
	// RebalanceParams contains parameters for rebalance workflow
	RebalanceParams struct {
		// BatchFailoverSize is number of domains to failover in one batch
		BatchFailoverSize int
		// BatchFailoverWaitTimeInSeconds is the waiting time between batch failover
		BatchFailoverWaitTimeInSeconds int
	}

	// RebalanceResult contains the result from the rebalance workflow
	RebalanceResult struct {
		SuccessDomains []string
		FailedDomains  []string
	}

	// DomainRebalanceData contains the result from getRebalanceDomains activity
	DomainRebalanceData struct {
		DomainName string
		// PreferredCluster is non-empty when the domain-level ActiveClusterName differs from the preference specified in DomainDataa.
		PreferredCluster string
		// ClusterAttributeUpdates lists every cluster attribute that has a different active cluster name than the preference specified in DomainData.
		ClusterAttributeUpdates []ClusterAttributePreference
	}
)

// RebalanceWorkflow is to rebalance domains across clusters based on rebalance policy.
func RebalanceWorkflow(ctx workflow.Context, params *RebalanceParams) (*RebalanceResult, error) {
	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	var domainData []*DomainRebalanceData
	err := workflow.ExecuteActivity(ao, getRebalanceDomainsActivityName).Get(ctx, &domainData)
	if err != nil {
		return nil, err
	}

	// Build a unified per-domain preferences map. All domains go through the same
	// batch/activity path — per-domain DomainFailoverPreferences drives the specific rebalance.
	domainPrefs := make(map[string]*DomainFailoverPreferences, len(domainData))
	domainNames := make([]string, 0, len(domainData))
	for _, d := range domainData {
		domainPrefs[d.DomainName] = &DomainFailoverPreferences{
			PreferredCluster:        d.PreferredCluster,
			ClusterAttributeUpdates: d.ClusterAttributeUpdates,
		}
		domainNames = append(domainNames, d.DomainName)
	}

	failoverParams := &FailoverParams{
		BatchFailoverSize:              params.BatchFailoverSize,
		BatchFailoverWaitTimeInSeconds: params.BatchFailoverWaitTimeInSeconds,
		DomainPreferences:              domainPrefs,
	}
	successDomains, failedDomains := failoverDomainsByBatch(ctx, domainNames, failoverParams, func() {}, false)

	return &RebalanceResult{
		SuccessDomains: successDomains,
		FailedDomains:  failedDomains,
	}, nil
}

// GetDomainsForRebalanceActivity fetches domains that need rebalancing.
// A domain is included when either its domain-level active cluster differs from PreferredCluster
// or one or more of its cluster attributes differs from ClusterAttributePreferences stored in domain data.
func GetDomainsForRebalanceActivity(ctx context.Context) ([]*DomainRebalanceData, error) {
	logger := activity.GetLogger(ctx)
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	var res []*DomainRebalanceData
	for _, domain := range domains {
		if !isDomainFailoverManagedByCadence(domain) ||
			!domain.IsGlobalDomain ||
			domain.GetDomainInfo().GetStatus() != types.DomainStatusRegistered {
			continue
		}

		domainName := domain.GetDomainInfo().GetName()

		// Domain-level: preferred cluster differs from active
		var preferredCluster string
		if shouldAllowRebalance(domain) {
			preferredCluster = getPreferredClusterName(domain)
		}

		// Attribute-level: cluster attributes differ from stored preferences
		attrPrefs, err := getClusterAttributePreferences(domain)
		if err != nil {
			logger.Error("skipping domain: malformed ClusterAttributePreferences",
				zap.String("domain", domainName), zap.Error(err))
			continue
		}
		attrUpdates := clusterAttributeUpdatesNeeded(domain, attrPrefs)

		if preferredCluster == "" && len(attrUpdates) == 0 {
			continue
		}

		entry := &DomainRebalanceData{DomainName: domainName}
		if preferredCluster != "" {
			entry.PreferredCluster = preferredCluster
		}
		entry.ClusterAttributeUpdates = attrUpdates
		res = append(res, entry)
	}
	return res, nil
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

func getPreferredClusterName(domain *types.DescribeDomainResponse) string {
	return domain.GetDomainInfo().GetData()[constants.DomainDataKeyForPreferredCluster]
}

// getClusterAttributePreferences reads and JSON-decodes ClusterAttributePreferences from domain data.
// Returns nil, nil when the key is absent. Returns nil, error when the value is malformed.
func getClusterAttributePreferences(domain *types.DescribeDomainResponse) ([]ClusterAttributePreference, error) {
	raw := domain.GetDomainInfo().GetData()[constants.DomainDataKeyForClusterAttributePreferences]
	if raw == "" {
		return nil, nil
	}
	var prefs []ClusterAttributePreference
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		return nil, errors.Join(ErrUnmarshallingClusterAttributePreferences, err)
	}
	return prefs, nil
}

// clusterAttributeUpdatesNeeded returns the subset of cluster attributes that have a different active cluster name
// than the preference specified in DomainData. Attributes not present in the domain's ActiveClusters
// config are skipped.
func clusterAttributeUpdatesNeeded(
	domain *types.DescribeDomainResponse,
	prefs []ClusterAttributePreference,
) []ClusterAttributePreference {
	ac := domain.ReplicationConfiguration.GetActiveClusters()
	if ac == nil {
		return nil
	}
	var updates []ClusterAttributePreference
	for _, pref := range prefs {
		info, err := ac.GetActiveClusterByClusterAttribute(pref.Scope, pref.Name)
		if err != nil {
			continue // attribute not configured on this domain
		}
		if info.ActiveClusterName != pref.PreferredCluster {
			updates = append(updates, pref)
		}
	}
	return updates
}

func isPreferredClusterInClusterListForDomain(preferredCluster string, domain *types.DescribeDomainResponse) bool {
	for _, cluster := range domain.ReplicationConfiguration.Clusters {
		if cluster.ClusterName == preferredCluster {
			return true
		}
	}
	return false
}

var ErrUnmarshallingClusterAttributePreferences = errors.New("failed to unmarshal ClusterAttributePreferences")
