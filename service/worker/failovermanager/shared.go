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
// the domains to act on, so this file includes the shared FailoverActivity as well.

import (
	"context"
	"math"
	"slices"
	"strconv"
	"time"

	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
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

	// DomainFailoverPreferences carries the failover instruction for a single domain. It defines all the changes to be applied to the domain.
	DomainFailoverPreferences struct {
		// DomainName identifies the domain this instruction applies to.
		DomainName string
		// TargetCluster, when non-empty, is the destination cluster set as the domain-level
		// ActiveClusterName. During a failover it is the cluster being failed over to; during a
		// rebalance it is the domain's stored preferred cluster.
		TargetCluster string
		// ClusterAttributeUpdates lists all ClusterAttribute level changes.
		ClusterAttributeUpdates []ClusterAttributePreference
		// FailoverTimeoutSeconds, when > 0, triggers a graceful failover with this timeout.
		// When zero, force failover is used.
		FailoverTimeoutSeconds int32
	}

	// FailoverActivityV2Params is the arg for the shared FailoverActivityV2.
	FailoverActivityV2Params struct {
		DomainPreferences []DomainFailoverPreferences
	}

	// DomainFailoverSuccess records a domain that was successfully failed over.
	DomainFailoverSuccess struct {
		DomainName string
	}

	// DomainFailoverFailure records a domain that failed to fail over, along with the error that
	// caused it. Error is a string (not error) so it survives JSON serialization across the activity
	// boundary and through the workflow query/result.
	DomainFailoverFailure struct {
		DomainName string
		Error      string
	}

	// FailoverActivityV2Result is the result of the shared FailoverActivityV2.
	FailoverActivityV2Result struct {
		SuccessDomains []DomainFailoverSuccess
		FailedDomains  []DomainFailoverFailure
	}
)

// FailoverActivityV2 is the single apply activity shared by FailoverWorkflowV2 and
// RebalanceWorkflowV2. It applies each DomainFailoverPreferences entry via FailoverDomain.
func FailoverActivityV2(ctx context.Context, params *FailoverActivityV2Params) (*FailoverActivityV2Result, error) {
	return failoverDomains(ctx, params.DomainPreferences)
}

// executeFailoverBatch returns a batchExecutor that invokes the shared FailoverActivityV2. On
// activity error every domain in the batch is reported failed with that error (false-positive semantics).
func executeFailoverBatch() batchExecutor {
	return func(ctx workflow.Context, batch []DomainFailoverPreferences) (success []DomainFailoverSuccess, failed []DomainFailoverFailure) {
		ao := workflow.WithActivityOptions(ctx, getFailoverActivityOptions())
		actParams := &FailoverActivityV2Params{DomainPreferences: batch}
		var actResult FailoverActivityV2Result
		if err := workflow.ExecuteActivity(ao, FailoverActivityV2, actParams).Get(ctx, &actResult); err != nil {
			return nil, failuresFromBatch(batch, err)
		}
		return actResult.SuccessDomains, actResult.FailedDomains
	}
}

// failoverDomains is the shared per-domain loop. For each entry it issues a single FailoverDomain request carrying the
// preferred ActiveClusterName and any per-attribute ActiveClusters overrides.
// Returns lists of domains delineated by success or failure, failures carrying the error.
func failoverDomains(ctx context.Context, prefs []DomainFailoverPreferences) (*FailoverActivityV2Result, error) {
	frontendClient := getClient(ctx)
	var successDomains []DomainFailoverSuccess
	var failedDomains []DomainFailoverFailure
	for _, p := range prefs {
		failoverRequest := &types.FailoverDomainRequest{DomainName: p.DomainName}
		if p.TargetCluster != "" {
			failoverRequest.DomainActiveClusterName = common.StringPtr(p.TargetCluster)
		}
		if len(p.ClusterAttributeUpdates) > 0 {
			failoverRequest.ActiveClusters = buildActiveClustersFromUpdates(p.ClusterAttributeUpdates)
		}
		if p.FailoverTimeoutSeconds > 0 {
			failoverRequest.FailoverTimeoutInSeconds = common.Int32Ptr(p.FailoverTimeoutSeconds)
		}

		if _, err := frontendClient.FailoverDomain(ctx, failoverRequest); err != nil {
			failedDomains = append(failedDomains, DomainFailoverFailure{DomainName: p.DomainName, Error: err.Error()})
		} else {
			successDomains = append(successDomains, DomainFailoverSuccess{DomainName: p.DomainName})
		}
	}
	return &FailoverActivityV2Result{
		SuccessDomains: successDomains,
		FailedDomains:  failedDomains,
	}, nil
}

// clustersTouchedByPreferences returns the deduped set of destination clusters a domain will be
// moved onto.
func clustersTouchedByPreferences(prefs DomainFailoverPreferences) []string {
	var uniqueClusters []string
	seen := make(map[string]struct{})

	if prefs.TargetCluster != "" {
		uniqueClusters = append(uniqueClusters, prefs.TargetCluster)
		seen[prefs.TargetCluster] = struct{}{}
	}

	for _, clusterAttributePreference := range prefs.ClusterAttributeUpdates {
		if clusterAttributePreference.PreferredCluster == "" {
			continue
		}

		if _, ok := seen[clusterAttributePreference.PreferredCluster]; ok {
			continue
		}
		seen[clusterAttributePreference.PreferredCluster] = struct{}{}
		uniqueClusters = append(uniqueClusters, clusterAttributePreference.PreferredCluster)
	}

	slices.Sort(uniqueClusters)
	return uniqueClusters
}

// warnIfMissingPollers runs the poller check against every destination cluster the domain is being
// moved onto. If any tasklists are missing a poller it logs a warning.
func warnIfMissingPollers(ctx context.Context, logger *zap.Logger, prefs DomainFailoverPreferences) {
	for _, cluster := range clustersTouchedByPreferences(prefs) {
		if err := validateTaskListPollerInfo(ctx, cluster, prefs.DomainName); err != nil {
			logger.Warn("domain has no active pollers in destination cluster; proceeding anyway",
				zap.String("domain", prefs.DomainName),
				zap.String("targetCluster", cluster),
				zap.Error(err))
		}
	}
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
type batchExecutor func(ctx workflow.Context, batch []DomainFailoverPreferences) (success []DomainFailoverSuccess, failed []DomainFailoverFailure)

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
) (success []DomainFailoverSuccess, failed []DomainFailoverFailure) {
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

// failuresFromBatch marks every domain in the batch as failed with the given error, used by batch
// executors to report an all-failed list when the underlying activity errors out.
func failuresFromBatch(prefs []DomainFailoverPreferences, err error) []DomainFailoverFailure {
	if len(prefs) == 0 {
		return nil
	}
	msg := err.Error()
	failures := make([]DomainFailoverFailure, len(prefs))
	for i, p := range prefs {
		failures[i] = DomainFailoverFailure{DomainName: p.DomainName, Error: msg}
	}
	return failures
}

// successDomainNames extracts the domain names from a list of successes, for the workflow query
// handler which reports plain name lists.
func successDomainNames(successes []DomainFailoverSuccess) []string {
	if len(successes) == 0 {
		return nil
	}
	names := make([]string, len(successes))
	for i, s := range successes {
		names[i] = s.DomainName
	}
	return names
}

// failedDomainNames extracts the domain names from a list of failures, for the workflow query handler
// which reports plain name lists.
func failedDomainNames(failures []DomainFailoverFailure) []string {
	if len(failures) == 0 {
		return nil
	}
	names := make([]string, len(failures))
	for i, f := range failures {
		names[i] = f.DomainName
	}
	return names
}

func getFailoverTimeoutSeconds(domain *types.DescribeDomainResponse) int32 {
	raw := domain.GetDomainInfo().GetData()[constants.DomainDataKeyForFailoverTimeoutSeconds]
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 || v > math.MaxInt32 {
		return 0
	}
	return int32(v)
}
