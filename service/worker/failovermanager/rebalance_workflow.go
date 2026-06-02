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
	"time"

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

	// RebalanceActivityParams params for RebalanceActivity. The activity iterates over
	// DomainPreferences and applies each entry.
	RebalanceActivityParams struct {
		DomainPreferences []DomainFailoverPreferences
	}
)

// RebalanceWorkflow rebalances domains across clusters based on per-domain preferences stored
// in domain data. GetDomainsForRebalanceActivity discovers the mismatched domains and per-domain
// preferences, then the workflow drives RebalanceActivity through the shared batch helper.
func RebalanceWorkflow(ctx workflow.Context, params *RebalanceParams) (*RebalanceResult, error) {
	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	var prefs []DomainFailoverPreferences
	if err := workflow.ExecuteActivity(ao, getRebalanceDomainsActivityName).Get(ctx, &prefs); err != nil {
		return nil, err
	}

	waitBetween := time.Duration(params.BatchFailoverWaitTimeInSeconds) * time.Second
	success, failed := processDomainsInBatches(
		ctx,
		prefs,
		params.BatchFailoverSize,
		waitBetween,
		nil,
		executeRebalanceBatch(),
	)

	return &RebalanceResult{
		SuccessDomains: success,
		FailedDomains:  failed,
	}, nil
}

// executeRebalanceBatch returns a batchExecutor that invokes RebalanceActivity for the given
// batch. On activity error every domain in the batch is reported as failed.
func executeRebalanceBatch() batchExecutor {
	return func(ctx workflow.Context, batch []DomainFailoverPreferences) (success, failed []string) {
		ao := workflow.WithActivityOptions(ctx, getFailoverActivityOptions())
		actParams := &RebalanceActivityParams{DomainPreferences: batch}
		var actResult FailoverActivityResult
		if err := workflow.ExecuteActivity(ao, RebalanceActivity, actParams).Get(ctx, &actResult); err != nil {
			return nil, domainNames(batch)
		}
		return actResult.SuccessDomains, actResult.FailedDomains
	}
}

// RebalanceActivity applies the supplied per-domain DomainFailoverPreferences. It is used by
// RebalanceWorkflow and never sets FailoverTimeoutInSeconds on the resulting UpdateDomain calls.
func RebalanceActivity(ctx context.Context, params *RebalanceActivityParams) (*FailoverActivityResult, error) {
	return applyDomainPreferences(ctx, params.DomainPreferences, nil)
}

// GetDomainsForRebalanceActivity fetches domains that need rebalancing. A domain is included
// when either its domain-level active cluster differs from PreferredCluster, or one or more of
// its cluster attributes differs from the ClusterAttributePreferences stored in domain data.
// Returns a per-domain DomainFailoverPreferences slice ready to feed straight into the batch
// helper and RebalanceActivity.
func GetDomainsForRebalanceActivity(ctx context.Context) ([]DomainFailoverPreferences, error) {
	logger := activity.GetLogger(ctx)
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	var res []DomainFailoverPreferences
	for _, domain := range domains {
		domainPreferences, err := getPreferencesForDomain(ctx, domain)
		if err != nil {
			logger.Error("skipping domain: no rebalance required", zap.String("domain", domain.GetDomainInfo().GetName()), zap.Error(err))
			continue
		}
		res = append(res, domainPreferences)
	}
	return res, nil
}

func getPreferencesForDomain(ctx context.Context, domain *types.DescribeDomainResponse) (DomainFailoverPreferences, error) {
	logger := activity.GetLogger(ctx)
	var prefs DomainFailoverPreferences

	if !isDomainEligibleForFailover(domain) {
		return prefs, errors.New("domain is not eligible for failover")
	}

	domainName := domain.GetDomainInfo().GetName()
	prefs.DomainName = domainName

	// Domain-level: preferred cluster differs from active
	domainPreferredCluster := getPreferredClusterName(domain)
	rebalanceRequired := len(domainPreferredCluster) != 0 &&
		domainPreferredCluster != domain.ReplicationConfiguration.GetActiveClusterName() &&
		isPreferredClusterInClusterListForDomain(domainPreferredCluster, domain)

	if rebalanceRequired {
		prefs.PreferredCluster = domainPreferredCluster
	}

	// Attribute-level: cluster attributes differ from stored preferences
	attrPrefs, err := getClusterAttributePreferences(domain)
	if err != nil {
		logger.Error("skipping domain cluster attribute preferences: malformed ClusterAttributePreferences",
			zap.String("domain", domainName), zap.Error(err))

	}
	attrUpdates := clusterAttributeUpdatesNeeded(domain, attrPrefs)

	if len(attrUpdates) > 0 {
		prefs.ClusterAttributeUpdates = attrUpdates
	}

	if len(prefs.PreferredCluster) == 0 && len(prefs.ClusterAttributeUpdates) == 0 {
		return DomainFailoverPreferences{}, ErrNoRebalanceRequired
	}

	return prefs, nil
}

var ErrNoRebalanceRequired = errors.New("no rebalance required")

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
