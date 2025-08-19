// Copyright (c) 2017 Uber Technologies, Inc.
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

package clusterredirection

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
)

//go:generate mockgen -package $GOPACKAGE -destination policy_mock.go -self_package github.com/uber/cadence/service/frontend/wrappers/clusterredirection github.com/uber/cadence/service/frontend/wrappers/clusterredirection ClusterRedirectionPolicy

const (
	// DCRedirectionPolicyDefault means no redirection
	DCRedirectionPolicyDefault = ""
	// DCRedirectionPolicyNoop means no redirection
	DCRedirectionPolicyNoop = "noop"
	// DCRedirectionPolicySelectedAPIsForwarding means forwarding the following non-worker APIs based domain
	// 1. StartWorkflowExecution
	// 2. SignalWithStartWorkflowExecution
	// 3. SignalWorkflowExecution
	// 4. RequestCancelWorkflowExecution
	// 5. TerminateWorkflowExecution
	// 6. ResetWorkflow
	// please also reference selectedAPIsForwardingRedirectionPolicyAPIAllowlist and DCRedirectionPolicySelectedAPIsForwardingV2
	DCRedirectionPolicySelectedAPIsForwarding = "selected-apis-forwarding"
	// DCRedirectionPolicySelectedAPIsForwardingV2 forwards everything in DCRedirectionPolicySelectedAPIsForwarding,
	// as well as activity completions (sync and async) and heartbeats.
	// This is done because activity results are generally deemed "useful" and relatively costly to re-do (when it is
	// even possible to redo), but activity workers themselves may be datacenter-specific.
	//
	// This will likely replace DCRedirectionPolicySelectedAPIsForwarding soon.
	//
	// 1-6. from DCRedirectionPolicySelectedAPIsForwarding
	// 7. RecordActivityTaskHeartbeat
	// 8. RecordActivityTaskHeartbeatByID
	// 9. RespondActivityTaskCanceled
	// 10. RespondActivityTaskCanceledByID
	// 11. RespondActivityTaskCompleted
	// 12. RespondActivityTaskCompletedByID
	// 13. RespondActivityTaskFailed
	// 14. RespondActivityTaskFailedByID
	// please also reference selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2
	DCRedirectionPolicySelectedAPIsForwardingV2 = "selected-apis-forwarding-v2"
	// DCRedirectionPolicyAllDomainAPIsForwarding means forwarding all the worker and non-worker APIs based domain,
	// and falling back to DCRedirectionPolicySelectedAPIsForwarding when the current active cluster is not the
	// cluster migration target.
	DCRedirectionPolicyAllDomainAPIsForwarding = "all-domain-apis-forwarding"
	// DCRedirectionPolicyAllDomainAPIsForwardingV2 means forwarding all the worker and non-worker APIs based domain,
	// and falling back to DCRedirectionPolicySelectedAPIsForwardingV2 when the current active cluster is not the
	// cluster migration target.
	DCRedirectionPolicyAllDomainAPIsForwardingV2 = "all-domain-apis-forwarding-v2"
)

type (
	// ClusterRedirectionPolicy is a DC redirection policy interface
	ClusterRedirectionPolicy interface {
		// Redirect redirects applicable API calls to active cluster based on given parameters and configured forwarding policy.
		// domainEntry (required): domain cache entry
		// workflowExecution (optional): workflow execution (only used for existing workflow API calls on active-active domains)
		// actClSelPolicyForNewWF (optional): active cluster selection policy for new workflow (only used for new workflow API calls on active-active domains)
		// apiName (required): API name
		// requestedConsistencyLevel (required): requested consistency level
		// call (required): function to call the API on the target cluster
		Redirect(
			ctx context.Context,
			domainEntry *cache.DomainCacheEntry,
			workflowExecution *types.WorkflowExecution,
			actClSelPolicyForNewWF *types.ActiveClusterSelectionPolicy,
			apiName string,
			requestedConsistencyLevel types.QueryConsistencyLevel,
			call func(string) error,
		) error
	}

	// noopRedirectionPolicy is DC redirection policy which does nothing
	noopRedirectionPolicy struct {
		currentClusterName string
	}

	// selectedOrAllAPIsForwardingRedirectionPolicy is a DC redirection policy
	// which (based on domain) forwards selected APIs calls or all domain APIs to active cluster
	selectedOrAllAPIsForwardingRedirectionPolicy struct {
		currentClusterName   string
		config               *frontendcfg.Config
		allDomainAPIs        bool
		selectedAPIs         map[string]struct{}
		targetCluster        string
		logger               log.Logger
		activeClusterManager activecluster.Manager
	}
)

// selectedAPIsForwardingRedirectionPolicyAPIAllowlist contains a list of non-worker APIs which can be redirected.
// This is paired with DCRedirectionPolicySelectedAPIsForwarding - keep both lists up to date.
var selectedAPIsForwardingRedirectionPolicyAPIAllowlist = map[string]struct{}{
	"StartWorkflowExecution":           {},
	"SignalWithStartWorkflowExecution": {},
	"SignalWorkflowExecution":          {},
	"RequestCancelWorkflowExecution":   {},
	"TerminateWorkflowExecution":       {},
	"ResetWorkflowExecution":           {},
}

// selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2 contains a list of non-worker APIs which can be redirected.
// This is paired with DCRedirectionPolicySelectedAPIsForwardingV2 - keep both lists up to date.
var selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2 = map[string]struct{}{
	// from selectedAPIsForwardingRedirectionPolicyAPIAllowlist
	"StartWorkflowExecution":           {},
	"SignalWithStartWorkflowExecution": {},
	"SignalWorkflowExecution":          {},
	"RequestCancelWorkflowExecution":   {},
	"TerminateWorkflowExecution":       {},
	"ResetWorkflowExecution":           {},
	// additional endpoints
	"RecordActivityTaskHeartbeat":      {},
	"RecordActivityTaskHeartbeatByID":  {},
	"RespondActivityTaskCanceled":      {},
	"RespondActivityTaskCanceledByID":  {},
	"RespondActivityTaskCompleted":     {},
	"RespondActivityTaskCompletedByID": {},
	"RespondActivityTaskFailed":        {},
	"RespondActivityTaskFailedByID":    {},
}

// allowedAPIsForDeprecatedDomains contains a list of APIs that are allowed to be called on deprecated domains
var allowedAPIsForDeprecatedDomains = map[string]struct{}{
	"ListWorkflowExecutions":     {},
	"CountWorkflowExecutions":    {},
	"ScanWorkflowExecutions":     {},
	"TerminateWorkflowExecution": {},
}

// RedirectionPolicyGenerator generate corresponding redirection policy
func RedirectionPolicyGenerator(
	clusterMetadata cluster.Metadata,
	config *frontendcfg.Config,
	policy config.ClusterRedirectionPolicy,
	logger log.Logger,
	activeClusterManager activecluster.Manager,
) ClusterRedirectionPolicy {
	switch policy.Policy {
	case DCRedirectionPolicyDefault:
		// default policy, noop
		return newNoopRedirectionPolicy(clusterMetadata.GetCurrentClusterName())
	case DCRedirectionPolicyNoop:
		return newNoopRedirectionPolicy(clusterMetadata.GetCurrentClusterName())
	case DCRedirectionPolicySelectedAPIsForwarding:
		currentClusterName := clusterMetadata.GetCurrentClusterName()
		return newSelectedOrAllAPIsForwardingPolicy(currentClusterName, config, false, selectedAPIsForwardingRedirectionPolicyAPIAllowlist, "", logger, activeClusterManager)
	case DCRedirectionPolicySelectedAPIsForwardingV2:
		currentClusterName := clusterMetadata.GetCurrentClusterName()
		return newSelectedOrAllAPIsForwardingPolicy(currentClusterName, config, false, selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2, "", logger, activeClusterManager)
	case DCRedirectionPolicyAllDomainAPIsForwarding:
		currentClusterName := clusterMetadata.GetCurrentClusterName()
		return newSelectedOrAllAPIsForwardingPolicy(currentClusterName, config, true, selectedAPIsForwardingRedirectionPolicyAPIAllowlist, policy.AllDomainApisForwardingTargetCluster, logger, activeClusterManager)
	case DCRedirectionPolicyAllDomainAPIsForwardingV2:
		currentClusterName := clusterMetadata.GetCurrentClusterName()
		return newSelectedOrAllAPIsForwardingPolicy(currentClusterName, config, true, selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2, policy.AllDomainApisForwardingTargetCluster, logger, activeClusterManager)

	default:
		panic(fmt.Sprintf("Unknown DC redirection policy %v", policy.Policy))
	}
}

// newNoopRedirectionPolicy is DC redirection policy which does nothing
func newNoopRedirectionPolicy(currentClusterName string) *noopRedirectionPolicy {
	return &noopRedirectionPolicy{
		currentClusterName: currentClusterName,
	}
}

// Redirect redirect the API call based on domain ID
func (policy *noopRedirectionPolicy) Redirect(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	workflowExecution *types.WorkflowExecution,
	actClSelPolicyForNewWF *types.ActiveClusterSelectionPolicy,
	apiName string,
	requestedConsistencyLevel types.QueryConsistencyLevel,
	call func(string) error,
) error {
	return call(policy.currentClusterName)
}

// newSelectedOrAllAPIsForwardingPolicy creates a forwarding policy for selected APIs based on domain
func newSelectedOrAllAPIsForwardingPolicy(
	currentClusterName string,
	config *frontendcfg.Config,
	allDomainAPIs bool,
	selectedAPIs map[string]struct{},
	targetCluster string,
	logger log.Logger,
	activeClusterManager activecluster.Manager,
) *selectedOrAllAPIsForwardingRedirectionPolicy {
	return &selectedOrAllAPIsForwardingRedirectionPolicy{
		currentClusterName:   currentClusterName,
		config:               config,
		allDomainAPIs:        allDomainAPIs,
		selectedAPIs:         selectedAPIs,
		targetCluster:        targetCluster,
		logger:               logger,
		activeClusterManager: activeClusterManager,
	}
}

func (policy *selectedOrAllAPIsForwardingRedirectionPolicy) Redirect(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	workflowExecution *types.WorkflowExecution,
	actClSelPolicyForNewWF *types.ActiveClusterSelectionPolicy,
	apiName string,
	requestedConsistencyLevel types.QueryConsistencyLevel,
	call func(string) error,
) error {
	if domainEntry.IsDeprecatedOrDeleted() {
		if _, ok := allowedAPIsForDeprecatedDomains[apiName]; !ok {
			return &types.DomainNotActiveError{
				Message:        "domain is deprecated or deleted.",
				DomainName:     domainEntry.GetInfo().Name,
				CurrentCluster: policy.currentClusterName,
				ActiveCluster:  policy.currentClusterName,
			}
		}
	}

	return policy.withRedirect(ctx, domainEntry, workflowExecution, actClSelPolicyForNewWF, apiName, requestedConsistencyLevel, call)
}

func (policy *selectedOrAllAPIsForwardingRedirectionPolicy) withRedirect(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	workflowExecution *types.WorkflowExecution,
	actClSelPolicyForNewWF *types.ActiveClusterSelectionPolicy,
	apiName string,
	requestedConsistencyLevel types.QueryConsistencyLevel,
	call func(string) error,
) error {
	targetDC, enableDomainNotActiveForwarding := policy.getTargetClusterAndIsDomainNotActiveAutoForwarding(ctx, domainEntry, workflowExecution, actClSelPolicyForNewWF, apiName, requestedConsistencyLevel)

	policy.logger.Debugf("Calling API %q on target cluster:%q for domain:%q", apiName, targetDC, domainEntry.GetInfo().Name)
	err := call(targetDC)

	var domainNotActiveErr *types.DomainNotActiveError
	ok := errors.As(err, &domainNotActiveErr)
	if !ok || !enableDomainNotActiveForwarding {
		return err
	}

	// TODO(active-active): emit a metric here including apiName, targetDC and newTargetDC tags
	// This can only happen if there was a failover during the API call.
	// Forward the request the the active cluster specified in the error
	if domainNotActiveErr.ActiveCluster == "" {
		policy.logger.Debugf("No active cluster specified in the error returned from cluster:%q for domain:%q, api: %q so skipping redirect", targetDC, domainEntry.GetInfo().Name, apiName)
		return err
	}

	if domainNotActiveErr.ActiveCluster == targetDC {
		policy.logger.Debugf("No need to redirect to new target cluster:%q for domain:%q, api: %q", targetDC, domainEntry.GetInfo().Name, apiName)
		return err
	}

	policy.logger.Debugf("Calling API %q on new target cluster:%q for domain:%q as indicated by response from cluster:%q", apiName, domainNotActiveErr.ActiveCluster, domainEntry.GetInfo().Name, targetDC)
	return call(domainNotActiveErr.ActiveCluster)
}

// return two values: the target cluster name, and whether or not forwarding to the active cluster
func (policy *selectedOrAllAPIsForwardingRedirectionPolicy) getTargetClusterAndIsDomainNotActiveAutoForwarding(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	workflowExecution *types.WorkflowExecution,
	actClSelPolicyForNewWF *types.ActiveClusterSelectionPolicy,
	apiName string,
	requestedConsistencyLevel types.QueryConsistencyLevel,
) (string, bool) {
	if !domainEntry.IsGlobalDomain() {
		// Do not do dc redirection if domain is local domain,
		// for global domains with 1 dc, it's still useful to do auto-forwarding during cluster migration
		return policy.currentClusterName, false
	}

	if !policy.config.EnableDomainNotActiveAutoForwarding(domainEntry.GetInfo().Name) {
		// Do not do dc redirection if auto-forwarding dynamicconfig is not enabled
		return policy.currentClusterName, false
	}

	currentActiveCluster := domainEntry.GetReplicationConfig().ActiveClusterName
	if domainEntry.GetReplicationConfig().IsActiveActive() {
		currentActiveCluster = policy.activeClusterForActiveActiveDomainRequest(ctx, domainEntry, workflowExecution, actClSelPolicyForNewWF, apiName)
	}

	if policy.allDomainAPIs {
		if policy.targetCluster == "" {
			return currentActiveCluster, true
		}
		if policy.targetCluster == currentActiveCluster {
			return currentActiveCluster, true
		}
		// fallback to selected APIs if targetCluster is not empty and not the same as currentActiveCluster
	}

	if requestedConsistencyLevel == types.QueryConsistencyLevelStrong {
		return currentActiveCluster, true
	}

	_, ok := policy.selectedAPIs[apiName]
	if !ok {
		// do not do dc redirection if API is not whitelisted
		return policy.currentClusterName, false
	}

	return currentActiveCluster, true
}

func (policy *selectedOrAllAPIsForwardingRedirectionPolicy) activeClusterForActiveActiveDomainRequest(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	workflowExecution *types.WorkflowExecution,
	actClSelPolicyForNewWF *types.ActiveClusterSelectionPolicy,
	apiName string,
) string {
	policy.logger.Debug("Determining active cluster for active-active domain request", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.Dynamic("execution", workflowExecution), tag.OperationName(apiName))
	if actClSelPolicyForNewWF != nil {
		policy.logger.Debug("Active cluster selection policy for new workflow", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.OperationName(apiName), tag.Dynamic("policy", actClSelPolicyForNewWF))
		lookupRes, err := policy.activeClusterManager.LookupNewWorkflow(ctx, domainEntry.GetInfo().ID, actClSelPolicyForNewWF)
		if err != nil {
			policy.logger.Error("Failed to lookup active cluster of new workflow, using active cluster in the same region", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.OperationName(apiName), tag.Error(err))
			return policy.activeClusterInSameRegion(ctx, domainEntry, policy.currentClusterName)
		}
		return lookupRes.ClusterName
	}

	if workflowExecution == nil {
		policy.logger.Debug("Workflow execution is nil, using active cluster in the same region", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.OperationName(apiName))
		return policy.activeClusterInSameRegion(ctx, domainEntry, policy.currentClusterName)
	}
	if workflowExecution.RunID == "" {
		policy.logger.Debug("Workflow execution run id is empty, using active cluster in the same region", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.OperationName(apiName))
		return policy.activeClusterInSameRegion(ctx, domainEntry, policy.currentClusterName)
	}

	lookupRes, err := policy.activeClusterManager.LookupWorkflow(ctx, domainEntry.GetInfo().ID, workflowExecution.WorkflowID, workflowExecution.RunID)
	if err != nil {
		policy.logger.Error("Failed to lookup active cluster of workflow, using active cluster in the same region", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.WorkflowID(workflowExecution.WorkflowID), tag.WorkflowRunID(workflowExecution.RunID), tag.OperationName(apiName), tag.Error(err))
		return policy.activeClusterInSameRegion(ctx, domainEntry, policy.currentClusterName)
	}

	policy.logger.Debug("Lookup workflow result for active-active domain request", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.WorkflowID(workflowExecution.WorkflowID), tag.WorkflowRunID(workflowExecution.RunID), tag.OperationName(apiName), tag.ActiveClusterName(lookupRes.ClusterName))
	return lookupRes.ClusterName
}

// activeClusterInSameRegion returns the active cluster in the same region as the given cluster name
// If the lookup fails, it returns current cluster name as fallback
// In that case, if current cluster is not the active cluster the caller will return an not-active error
// TODO(active-active): Revisit this logic for strong-consistency requests and potentially reject them instead of serving them from the current cluster
func (policy *selectedOrAllAPIsForwardingRedirectionPolicy) activeClusterInSameRegion(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	clusterName string,
) string {
	lookupRes, err := policy.activeClusterManager.LookupCluster(ctx, domainEntry.GetInfo().ID, clusterName)
	if err != nil {
		policy.logger.Error("Failed to lookup active cluster in same region", tag.WorkflowDomainName(domainEntry.GetInfo().Name), tag.ClusterName(clusterName), tag.Error(err))
		return policy.currentClusterName
	}
	return lookupRes.ClusterName
}
