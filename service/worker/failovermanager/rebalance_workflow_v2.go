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

const (
	// RebalanceWorkflowV2TypeName is the registered workflow type for RebalanceWorkflowV2.
	RebalanceWorkflowV2TypeName = "cadence-sys-rebalance-v2-workflow"
	// RebalanceWorkflowV2ID is the fixed workflow ID, reused so only one run is active at a time.
	RebalanceWorkflowV2ID = "cadence-rebalance-v2"
	// getDomainsForRebalanceV2ActivityName is the registered name of the rebalance collection activity.
	getDomainsForRebalanceV2ActivityName = "cadence-sys-getDomainsForRebalanceV2-activity"
)

type (
	// RebalanceV2Params is the arg for RebalanceWorkflowV2.
	RebalanceV2Params struct {
		// BatchSize is the number of domains rebalanced per batch.
		BatchSize int
		// WaitBetweenBatchSeconds is the pause between successive batches.
		WaitBetweenBatchSeconds int
	}

	// RebalanceV2Result is the result of RebalanceWorkflowV2.
	RebalanceV2Result struct {
		SuccessDomains []string
		FailedDomains  []string
	}
)

// errNoRebalanceRequiredV2 is returned for a domain whose live config already matches its preferences.
var errNoRebalanceRequiredV2 = errors.New("no rebalance required")

// errUnmarshalClusterAttributePreferencesV2 wraps a malformed ClusterAttributePreferences value.
var errUnmarshalClusterAttributePreferencesV2 = errors.New("failed to unmarshal ClusterAttributePreferences")

// RebalanceWorkflowV2 corrects every managed domain whose live configuration has drifted from its
// stored preferences — domain-level PreferredCluster and/or per-attribute ClusterAttributePreferences
// — moving each back to its preferred cluster in batches, with pause/resume support.
// Returns lists of successfully and unsuccessfully rebalanced domains.
func RebalanceWorkflowV2(ctx workflow.Context, params *RebalanceV2Params) (*RebalanceV2Result, error) {
	var (
		successDomains []string
		failedDomains  []string
		wfState        = WorkflowInitialized
		operator       = getOperator(ctx)
	)
	err := workflow.SetQueryHandler(ctx, QueryType, func(input []byte) (*QueryResult, error) {
		return &QueryResult{
			Success:        len(successDomains),
			Failed:         len(failedDomains),
			State:          wfState,
			SuccessDomains: successDomains,
			FailedDomains:  failedDomains,
			Operator:       operator,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	var prefs []DomainFailoverPreferences
	if err := workflow.ExecuteActivity(ao, GetDomainsForRebalanceV2Activity).Get(ctx, &prefs); err != nil {
		return nil, err
	}

	checkPause := newPauseHandler(ctx, func(s string) { wfState = s })
	waitBetween := time.Duration(params.WaitBetweenBatchSeconds) * time.Second
	successDomains, failedDomains = processInBatches(
		ctx,
		prefs,
		params.BatchSize,
		waitBetween,
		checkPause,
		executeFailoverBatch(),
	)

	wfState = WorkflowCompleted
	return &RebalanceV2Result{
		SuccessDomains: successDomains,
		FailedDomains:  failedDomains,
	}, nil
}

// GetDomainsForRebalanceV2Activity collects the domains that have drifted from their stored
// preferences. A domain is included when its domain-level active cluster differs from PreferredCluster,
// or one or more cluster attributes differ from the ClusterAttributePreferences stored in domain data.
func GetDomainsForRebalanceV2Activity(ctx context.Context) ([]DomainFailoverPreferences, error) {
	logger := activity.GetLogger(ctx)
	domains, err := getAllDomains(ctx, nil)
	if err != nil {
		return nil, err
	}
	var res []DomainFailoverPreferences
	for _, domain := range domains {
		prefs, err := rebalancePreferencesForDomain(domain)
		if err != nil {
			logger.Info("skipping domain: no rebalance required",
				zap.String("domain", domain.GetDomainInfo().GetName()), zap.Error(err))
			continue
		}
		res = append(res, prefs)
	}
	return res, nil
}

// rebalancePreferencesForDomain builds the preferences for one domain, or returns an error when the
// domain is ineligible or already balanced. A malformed ClusterAttributePreferences value is treated
// as "no attribute preferences" so a single bad domain does not block the rest.
func rebalancePreferencesForDomain(domain *types.DescribeDomainResponse) (DomainFailoverPreferences, error) {
	if !isEligibleForFailover(domain) {
		return DomainFailoverPreferences{}, errors.New("domain is not eligible for rebalance")
	}

	name := domain.GetDomainInfo().GetName()
	prefs := DomainFailoverPreferences{DomainName: name}

	// Domain-level: preferred cluster differs from the live active cluster and is a valid target.
	preferred := getPreferredClusterName(domain)
	if len(preferred) != 0 &&
		preferred != domain.ReplicationConfiguration.GetActiveClusterName() &&
		isPreferredClusterInClusterListForDomain(preferred, domain) {
		prefs.PreferredCluster = preferred
	}

	// Attribute-level: any attribute whose live active cluster differs from its stored preference.
	// A malformed preferences value yields nil here and is silently skipped.
	attrPrefs, _ := getClusterAttributePreferences(domain)
	prefs.ClusterAttributeUpdates = clusterAttributeUpdatesNeeded(domain, attrPrefs)

	if prefs.PreferredCluster == "" && len(prefs.ClusterAttributeUpdates) == 0 {
		return DomainFailoverPreferences{}, errNoRebalanceRequiredV2
	}
	return prefs, nil
}

// getClusterAttributePreferences reads and JSON-decodes ClusterAttributePreferences from domain data.
// Returns nil, nil when the key is absent; nil, error when the value is malformed.
func getClusterAttributePreferences(domain *types.DescribeDomainResponse) ([]ClusterAttributePreference, error) {
	raw := domain.GetDomainInfo().GetData()[constants.DomainDataKeyForClusterAttributePreferences]
	if raw == "" {
		return nil, nil
	}
	var prefs []ClusterAttributePreference
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		return nil, errors.Join(errUnmarshalClusterAttributePreferencesV2, err)
	}
	return prefs, nil
}

// clusterAttributeUpdatesNeeded returns the subset of attribute preferences whose live active
// cluster differs from the preference. Attributes not configured on the domain are skipped.
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
