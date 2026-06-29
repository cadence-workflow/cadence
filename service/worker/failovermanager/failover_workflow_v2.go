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
	"errors"
	"slices"
	"strings"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/types"
)

const (
	// FailoverWorkflowV2TypeName is the registered workflow type for FailoverWorkflowV2.
	FailoverWorkflowV2TypeName = "cadence-sys-failover-v2-workflow"
	// FailoverWorkflowV2ID is the fixed workflow ID, reused so only one run is active at a time.
	FailoverWorkflowV2ID = "cadence-failover-v2"
	// failoverActivityV2Name is the registered name of the shared apply activity.
	failoverActivityV2Name = "cadence-sys-failover-v2-activity"
	// getDomainsForFailoverV2ActivityName is the registered name of the failover collection activity.
	getDomainsForFailoverV2ActivityName = "cadence-sys-getDomainsForFailoverV2-activity"

	defaultBatchSizeV2               = 20
	defaultWaitBetweenBatchSecondsV2 = 30

	errMsgV2ParamsNil          = "params is nil"
	errMsgV2SourceClusterEmpty = "sourceClusters is empty"
	errMsgV2TargetClusterEmpty = "targetCluster is empty"
	errMsgV2SameCluster        = "targetCluster is also listed as a sourceCluster"
)

type (
	// FailoverV2Params is the arg for FailoverWorkflowV2.
	FailoverV2Params struct {
		// SourceClusters are the clusters being evacuated; only domains active in one of these are moved.
		SourceClusters []string
		// TargetCluster is where evacuated domains and attributes are moved to.
		TargetCluster string
		// BatchSize is the number of domains failed over per batch.
		BatchSize int
		// WaitBetweenBatchSeconds is the pause between successive batches.
		WaitBetweenBatchSeconds int
		// Domains optionally restricts the run to a specific subset of domain names.
		Domains []string
		// ClusterAttributes specifies which cluster attributes should be included for failover.
		// If empty, cluster attributes are not included.
		ClusterAttributes []types.ClusterAttribute
	}

	// DomainSnapshot records a single domain's pre-failover state so a later restore can put it
	// back exactly. Only the parts that were changed are populated.
	DomainSnapshot struct {
		// DomainName identifies the domain.
		DomainName string
		// PreviousActiveCluster is the domain-level ActiveClusterName before failover; empty when
		// the domain-level active cluster was not changed.
		PreviousActiveCluster string
		// PreviousClusterAttributes lists each changed attribute with the cluster it was on before
		// failover, suitable for replaying through FailoverActivityV2 to restore.
		PreviousClusterAttributes []ClusterAttributePreference
	}

	// FailoverV2Result is the result of FailoverWorkflowV2.
	FailoverV2Result struct {
		SuccessDomains []DomainFailoverSuccess
		FailedDomains  []DomainFailoverFailure
		// Snapshots holds the pre-failover state of every moved domain. It is queryable while the
		// workflow runs and readable from history afterwards, and is the input a future restore
		// workflow would replay.
		Snapshots []DomainSnapshot
	}

	// GetDomainsForFailoverV2Params
	GetDomainsForFailoverV2Params struct {
		// SourceClusters are the clusters being evacuated; only domains active in one of these are moved.
		SourceClusters []string
		// TargetCluster will be used as the new cluster for all domains that are failed over.
		TargetCluster string
		// Domains optionally restricts the run to a specific subset of domain names.
		Domains []string
		// ClusterAttributes specifies which cluster attributes should be included for failover.
		// If empty, cluster attributes are not included.
		ClusterAttributes []types.ClusterAttribute
	}

	// GetDomainsForFailoverV2Result is what GetDomainsForFailoverV2Activity returns: the per-domain
	// preferences to apply plus the snapshots of what is being changed.
	GetDomainsForFailoverV2Result struct {
		Preferences []DomainFailoverPreferences
		Snapshots   []DomainSnapshot
	}
)

// FailoverWorkflowV2 fails all managed domains out of SourceClusters and onto TargetCluster, in
// batches, with pause/resume support. It is N-region safe: domains not currently active in one of
// SourceClusters are left untouched. It records a per-domain snapshot of everything it changes so the
// failover can be reversed later.
func FailoverWorkflowV2(ctx workflow.Context, params *FailoverV2Params) (*FailoverV2Result, error) {
	if err := validateFailoverV2Params(params); err != nil {
		return nil, err
	}

	// Query state, exposed via the shared QueryType handler.
	var (
		totalDomains   int
		successDomains []DomainFailoverSuccess
		failedDomains  []DomainFailoverFailure
		snapshots      []DomainSnapshot
		wfState        = WorkflowInitialized
		operator       = getOperator(ctx)
	)
	err := workflow.SetQueryHandler(ctx, QueryType, func(input []byte) (*QueryResult, error) {
		return &QueryResult{
			TotalDomains:   totalDomains,
			Success:        len(successDomains),
			Failed:         len(failedDomains),
			State:          wfState,
			TargetCluster:  params.TargetCluster,
			SourceCluster:  strings.Join(params.SourceClusters, ","),
			SuccessDomains: successDomainNames(successDomains),
			FailedDomains:  failedDomainNames(failedDomains),
			Operator:       operator,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	collected, err := executeGetDomainsForFailoverV2(ctx, params)
	if err != nil {
		return nil, err
	}
	totalDomains = len(collected.Preferences)
	snapshots = collected.Snapshots

	checkPause := newPauseHandler(ctx, func(s string) { wfState = s })
	waitBetween := time.Duration(params.WaitBetweenBatchSeconds) * time.Second
	successDomains, failedDomains = processInBatches(
		ctx,
		collected.Preferences,
		params.BatchSize,
		waitBetween,
		checkPause,
		executeFailoverBatch(),
	)

	wfState = WorkflowCompleted
	return &FailoverV2Result{
		SuccessDomains: successDomains,
		FailedDomains:  failedDomains,
		Snapshots:      snapshots,
	}, nil
}

// executeGetDomainsForFailoverV2 runs the failover collection activity and returns its result.
func executeGetDomainsForFailoverV2(ctx workflow.Context, params *FailoverV2Params) (*GetDomainsForFailoverV2Result, error) {
	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	actParams := &GetDomainsForFailoverV2Params{
		SourceClusters:    params.SourceClusters,
		TargetCluster:     params.TargetCluster,
		Domains:           params.Domains,
		ClusterAttributes: params.ClusterAttributes,
	}
	var result GetDomainsForFailoverV2Result
	if err := workflow.ExecuteActivity(ao, GetDomainsForFailoverV2Activity, actParams).Get(ctx, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDomainsForFailoverV2Activity collects the domains to fail out of SourceClusters. For each managed
// domain, any domain-level active cluster or cluster attribute currently on one of SourceClusters is
// marked to move to TargetCluster; a snapshot of the prior values is recorded for restore. Domains not
// active in any of SourceClusters are skipped, keeping the operation N-region safe.
func GetDomainsForFailoverV2Activity(ctx context.Context, params *GetDomainsForFailoverV2Params) (*GetDomainsForFailoverV2Result, error) {
	logger := activity.GetLogger(ctx)
	domains, err := getAllDomains(ctx, params.Domains)
	if err != nil {
		return nil, err
	}
	result := &GetDomainsForFailoverV2Result{}
	for _, domain := range domains {
		if !isEligibleForFailover(domain) {
			continue
		}
		prefs, snapshot, ok := failoverPreferencesForDomain(domain, params.SourceClusters, params.TargetCluster, params.ClusterAttributes)
		if !ok {
			continue
		}
		prefs.FailoverTimeoutSeconds = getFailoverTimeoutSeconds(domain)
		warnIfMissingPollers(ctx, logger, prefs)
		result.Preferences = append(result.Preferences, prefs)
		result.Snapshots = append(result.Snapshots, snapshot)
	}
	return result, nil
}

// failoverPreferencesForDomain builds the preferences and snapshot for one domain. It returns ok=false
// when the domain is not active in any of sourceClusters (nothing to move).
func failoverPreferencesForDomain(
	domain *types.DescribeDomainResponse,
	sourceClusters []string,
	targetCluster string,
	clusterAttributeFilter []types.ClusterAttribute,
) (DomainFailoverPreferences, DomainSnapshot, bool) {
	name := domain.GetDomainInfo().GetName()
	prefs := DomainFailoverPreferences{DomainName: name}
	snapshot := DomainSnapshot{DomainName: name}

	// Domain-level: move the active cluster only when it currently sits on one of the sources.
	if activeCluster := domain.ReplicationConfiguration.GetActiveClusterName(); slices.Contains(sourceClusters, activeCluster) {
		prefs.TargetCluster = targetCluster
		snapshot.PreviousActiveCluster = activeCluster
	}

	if len(clusterAttributeFilter) > 0 {
		// Attribute-level: move each attribute currently active on one of the sources.
		if ac := domain.ReplicationConfiguration.GetActiveClusters(); ac != nil {
			for scope, attrScope := range ac.GetAttributeScopes() {
				for attrName, info := range attrScope.ClusterAttributes {
					if !slices.Contains(clusterAttributeFilter, types.ClusterAttribute{Scope: scope, Name: attrName}) {
						continue
					}
					if !slices.Contains(sourceClusters, info.ActiveClusterName) {
						continue
					}
					prefs.ClusterAttributeUpdates = append(prefs.ClusterAttributeUpdates, ClusterAttributePreference{
						Scope:            scope,
						Name:             attrName,
						PreferredCluster: targetCluster,
					})
					snapshot.PreviousClusterAttributes = append(snapshot.PreviousClusterAttributes, ClusterAttributePreference{
						Scope:            scope,
						Name:             attrName,
						PreferredCluster: info.ActiveClusterName,
					})
				}
			}
		}
	}

	if prefs.TargetCluster == "" && len(prefs.ClusterAttributeUpdates) == 0 {
		return DomainFailoverPreferences{}, DomainSnapshot{}, false
	}
	return prefs, snapshot, true
}

func validateFailoverV2Params(params *FailoverV2Params) error {
	if params == nil {
		return errors.New(errMsgV2ParamsNil)
	}
	if params.BatchSize <= 0 {
		params.BatchSize = defaultBatchSizeV2
	}
	if params.WaitBetweenBatchSeconds <= 0 {
		params.WaitBetweenBatchSeconds = defaultWaitBetweenBatchSecondsV2
	}
	if len(params.SourceClusters) == 0 {
		return errors.New(errMsgV2SourceClusterEmpty)
	}
	if params.TargetCluster == "" {
		return errors.New(errMsgV2TargetClusterEmpty)
	}
	for _, sc := range params.SourceClusters {
		if sc == "" {
			return errors.New(errMsgV2SourceClusterEmpty)
		}
		if sc == params.TargetCluster {
			return errors.New(errMsgV2SameCluster)
		}
	}
	return nil
}
