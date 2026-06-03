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

// This file holds the shared building blocks for the V2 failover and rebalance workflows
// (FailoverWorkflowV2 / RebalanceWorkflowV2). The two workflows differ only in how they collect
// the domains to act on; once a per-domain DomainFailoverPreferences slice exists, both apply it
// the same way. That shared apply path — the single FailoverActivityV2 — and the batch/pause helpers
// live here so neither workflow file owns the other's types.
//
// Symbols defined in the (master) workflow.go and rebalance_workflow.go are reused directly:
// getAllDomains, getClient, validateTaskListPollerInfo, isDomainFailoverManagedByCadence,
// getPreferredClusterName, isPreferredClusterInClusterListForDomain, cleanupChannel, the activity
// option helpers, and the Workflow*/PauseSignal/ResumeSignal/QueryType constants.

import (
	"context"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

type (
	// ClusterAttributePreference declares the preferred active cluster for a single scope:name
	// cluster attribute pair. It is the stored form in domain data
	// (constants.DomainDataKeyForClusterAttributePreferences) and the unit of an attribute-level
	// update carried in DomainFailoverPreferences.
	ClusterAttributePreference struct {
		Scope            string `json:"scope"`
		Name             string `json:"name"`
		PreferredCluster string `json:"preferredCluster"`
	}

	// DomainFailoverPreferences carries the failover instruction for a single domain. It is the
	// unit of work the shared FailoverActivityV2 consumes — each workflow builds a slice of these,
	// processInBatches slices the slice, and the activity iterates.
	DomainFailoverPreferences struct {
		// DomainName identifies the domain this instruction applies to.
		DomainName string
		// PreferredCluster, when non-empty, sets the domain-level ActiveClusterName.
		PreferredCluster string
		// ClusterAttributeUpdates lists attribute-level changes; each entry moves one scope:name
		// attribute to its PreferredCluster.
		ClusterAttributeUpdates []ClusterAttributePreference
	}

	// FailoverActivityV2Params is the arg for the shared FailoverActivityV2.
	FailoverActivityV2Params struct {
		DomainPreferences []DomainFailoverPreferences
	}

	// FailoverActivityV2Result is the result of the shared FailoverActivityV2. FailedDomains may
	// contain false positives: a domain whose activity errored is reported failed even if the
	// UpdateDomain partially applied.
	FailoverActivityV2Result struct {
		SuccessDomains []string
		FailedDomains  []string
	}
)

// FailoverActivityV2 is the single apply activity shared by FailoverWorkflowV2 and
// RebalanceWorkflowV2. It applies each DomainFailoverPreferences entry via UpdateDomain.
func FailoverActivityV2(ctx context.Context, params *FailoverActivityV2Params) (*FailoverActivityV2Result, error) {
	return failoverDomain(ctx, params.DomainPreferences)
}

// executeFailoverBatch returns a batchExecutor that invokes the shared FailoverActivityV2. On
// activity error every domain in the batch is reported failed (false-positive semantics).
func executeFailoverBatch() batchExecutor {
	return func(ctx workflow.Context, batch []DomainFailoverPreferences) (success, failed []string) {
		ao := workflow.WithActivityOptions(ctx, getFailoverActivityOptions())
		actParams := &FailoverActivityV2Params{DomainPreferences: batch}
		var actResult FailoverActivityV2Result
		if err := workflow.ExecuteActivity(ao, FailoverActivityV2, actParams).Get(ctx, &actResult); err != nil {
			return nil, domainNames(batch)
		}
		return actResult.SuccessDomains, actResult.FailedDomains
	}
}

// failoverDomain is the shared per-domain loop. For each entry it validates pollers against every
// distinct target cluster the entry references, then issues a single UpdateDomain carrying the
// preferred ActiveClusterName and any per-attribute ActiveClusters overrides.
func failoverDomain(ctx context.Context, prefs []DomainFailoverPreferences) (*FailoverActivityV2Result, error) {
	logger := activity.GetLogger(ctx)
	frontendClient := getClient(ctx)
	var successDomains, failedDomains []string
	for _, p := range prefs {
		failed := false
		for _, cluster := range collectTargetClusters(p) {
			if err := validateTaskListPollerInfo(ctx, cluster, p.DomainName); err != nil {
				logger.Error("Failed to validate task list poller info", zap.Error(err))
				failedDomains = append(failedDomains, p.DomainName)
				failed = true
				break
			}
		}
		if failed {
			continue
		}

		updateRequest := &types.UpdateDomainRequest{Name: p.DomainName}
		if p.PreferredCluster != "" {
			updateRequest.ActiveClusterName = common.StringPtr(p.PreferredCluster)
		}
		if len(p.ClusterAttributeUpdates) > 0 {
			updateRequest.ActiveClusters = buildActiveClustersFromUpdates(p.ClusterAttributeUpdates)
		}

		if _, err := frontendClient.UpdateDomain(ctx, updateRequest); err != nil {
			failedDomains = append(failedDomains, p.DomainName)
		} else {
			successDomains = append(successDomains, p.DomainName)
		}
	}
	return &FailoverActivityV2Result{
		SuccessDomains: successDomains,
		FailedDomains:  failedDomains,
	}, nil
}

// buildActiveClustersFromUpdates constructs an ActiveClusters payload from per-domain
// ClusterAttributePreferences, setting each scope:name attribute to its preferred cluster. The server
// merges this into the domain's existing ActiveClusters config.
func buildActiveClustersFromUpdates(updates []ClusterAttributePreference) *types.ActiveClusters {
	result := &types.ActiveClusters{
		AttributeScopes: make(map[string]types.ClusterAttributeScope),
	}
	for _, u := range updates {
		scope, ok := result.AttributeScopes[u.Scope]
		if !ok {
			scope = types.ClusterAttributeScope{
				ClusterAttributes: make(map[string]types.ActiveClusterInfo),
			}
		}
		scope.ClusterAttributes[u.Name] = types.ActiveClusterInfo{
			ActiveClusterName: u.PreferredCluster,
		}
		result.AttributeScopes[u.Scope] = scope
	}
	return result
}

// collectTargetClusters returns the unique set of target clusters a domain's preferences move to,
// so poller validation runs against every destination.
func collectTargetClusters(prefs DomainFailoverPreferences) []string {
	seen := make(map[string]struct{})
	if prefs.PreferredCluster != "" {
		seen[prefs.PreferredCluster] = struct{}{}
	}
	for _, u := range prefs.ClusterAttributeUpdates {
		seen[u.PreferredCluster] = struct{}{}
	}
	clusters := make([]string, 0, len(seen))
	for c := range seen {
		clusters = append(clusters, c)
	}
	return clusters
}

// isEligibleForFailover checks if a domain meets the eligibility criteria for failover by a central team.
// Returns true if the domain:
//   - is nil or has no DomainInfo
//   - has an explicit Deprecated/Deleted status
//   - is not global
//   - is not opted in via the managed-failover domain-data key
func isEligibleForFailover(domain *types.DescribeDomainResponse) bool {
	if domain == nil || domain.DomainInfo == nil {
		return false
	}
	switch domain.DomainInfo.GetStatus() {
	case types.DomainStatusDeprecated, types.DomainStatusDeleted:
		return false
	}
	if !domain.GetIsGlobalDomain() {
		return false
	}
	return isDomainFailoverManagedByCadence(domain)
}

// batchExecutor runs an activity over a batch of preferences and returns the success/failed subsets.
// It abstracts the activity from processInBatches so both workflows reuse the same batch loop.
type batchExecutor func(ctx workflow.Context, batch []DomainFailoverPreferences) (success, failed []string)

// processInBatches splits prefs into batches of batchSize, calls executeBatch for each, sleeps
// waitBetween between successive batches, and aggregates results. onBatchStart is invoked once before
// each batch (used to honour pause/resume); it may be nil.
func processInBatches(
	ctx workflow.Context,
	prefs []DomainFailoverPreferences,
	batchSize int,
	waitBetween time.Duration,
	onBatchStart func(),
	executeBatch batchExecutor,
) (success, failed []string) {
	if len(prefs) == 0 {
		return nil, nil
	}
	if batchSize <= 0 {
		batchSize = len(prefs)
	}

	total := len(prefs)
	times := total / batchSize
	if total%batchSize != 0 {
		times++
	}

	for i := 0; i < times; i++ {
		if onBatchStart != nil {
			onBatchStart()
		}
		end := (i + 1) * batchSize
		if end > total {
			end = total
		}
		batch := prefs[i*batchSize : end]
		s, f := executeBatch(ctx, batch)
		success = append(success, s...)
		failed = append(failed, f...)
		if i != times-1 {
			workflow.Sleep(ctx, waitBetween)
		}
	}
	return success, failed
}

// newPauseHandler returns a hook that processInBatches calls at the start of each batch. A
// PauseSignal blocks until ResumeSignal arrives; setState toggles the reported workflow state so the
// query handler reflects the pause. setState may be nil.
func newPauseHandler(ctx workflow.Context, setState func(string)) func() {
	pauseCh := workflow.GetSignalChannel(ctx, PauseSignal)
	resumeCh := workflow.GetSignalChannel(ctx, ResumeSignal)
	return func() {
		if pauseCh.ReceiveAsync(nil) {
			if setState != nil {
				setState(WorkflowPaused)
			}
			resumeCh.Receive(ctx, nil)
			cleanupChannel(pauseCh)
		}
		if setState != nil {
			setState(WorkflowRunning)
		}
	}
}

// domainNames extracts DomainName from each entry, used by batch executors to report an all-failed
// list when the underlying activity errors out.
func domainNames(prefs []DomainFailoverPreferences) []string {
	if len(prefs) == 0 {
		return nil
	}
	names := make([]string, len(prefs))
	for i, p := range prefs {
		names[i] = p.DomainName
	}
	return names
}
