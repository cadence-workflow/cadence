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
		DomainName       string
		PreferredCluster string
	}
)

// RebalanceWorkflow is to rebalance domains across clusters based on rebalance policy.
func RebalanceWorkflow(ctx workflow.Context, params *RebalanceParams) (*RebalanceResult, error) {
	// get rebalance domains
	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	var domainData []*DomainRebalanceData
	err := workflow.ExecuteActivity(ao, getRebalanceDomainsActivityName).Get(ctx, &domainData)
	if err != nil {
		return nil, err
	}
	domainPerCluster := getDomainsByCluster(domainData)

	result := &RebalanceResult{
		SuccessDomains: []string{},
		FailedDomains:  []string{},
	}
	// failover domains for rebalance
	for cluster, domains := range domainPerCluster {
		failoverParams := &FailoverParams{
			TargetCluster:                  cluster,
			BatchFailoverSize:              params.BatchFailoverSize,
			BatchFailoverWaitTimeInSeconds: params.BatchFailoverWaitTimeInSeconds,
		}
		successDomains, failedDomains := failoverDomainsByBatch(
			ctx,
			domains,
			failoverParams,
			func() {},
			false,
		)
		result.SuccessDomains = append(result.SuccessDomains, successDomains...)
		result.FailedDomains = append(result.FailedDomains, failedDomains...)
	}

	return result, nil
}

// GetDomainsForRebalanceActivity activity fetch domains for rebalance
func GetDomainsForRebalanceActivity(ctx context.Context) ([]*DomainRebalanceData, error) {
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	var res []*DomainRebalanceData
	for _, domain := range domains {
		if shouldAllowRebalance(domain) {
			domainName := domain.GetDomainInfo().GetName()
			res = append(res, &DomainRebalanceData{
				DomainName:       domainName,
				PreferredCluster: getPreferredClusterName(domain),
			})
		}
	}
	return res, nil
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

func getDomainsByCluster(domainData []*DomainRebalanceData) map[string][]string {
	domainPerCluster := make(map[string][]string)
	for _, domain := range domainData {
		domainPerCluster[domain.PreferredCluster] = append(domainPerCluster[domain.PreferredCluster], domain.DomainName)
	}

	return domainPerCluster
}

func isPreferredClusterInClusterListForDomain(preferredCluster string, domain *types.DescribeDomainResponse) bool {
	for _, cluster := range domain.ReplicationConfiguration.Clusters {
		if cluster.ClusterName == preferredCluster {
			return true
		}
	}
	return false
}
