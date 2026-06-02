// Copyright (c) 2017-2020 Uber Technologies Inc.
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
	"fmt"
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

type (
	contextKey string

	// ClusterAttributePreference declares the preferred active cluster for a single scope:name
	// cluster attribute pair. Used both as the stored form in domain data
	// (DomainDataKeyForClusterAttributePreferences) and as the unit returned by
	// clusterAttributeUpdatesNeeded when a mismatch is detected.
	ClusterAttributePreference struct {
		Scope            string `json:"scope"`
		Name             string `json:"name"`
		PreferredCluster string `json:"preferredCluster"`
	}

	// DomainFailoverPreferences carries the failover instruction for a single domain. It is
	// the unit of work that both FailoverActivity and RebalanceActivity consume — workflows
	// build a slice of these, the batch helper slices the slice, and the activity iterates.
	DomainFailoverPreferences struct {
		// DomainName identifies the domain this instruction applies to.
		DomainName string
		// PreferredCluster, when non-empty, sets the domain-level ActiveClusterName.
		PreferredCluster string
		// ClusterAttributeUpdates lists attribute-level mismatches that need correction.
		// Each entry is a ClusterAttributePreference whose current active cluster differs
		// from the preferred one.
		ClusterAttributeUpdates []ClusterAttributePreference
	}
)

const (
	failoverManagerContextKey contextKey = "failoverManagerContext"
	// TaskListName tasklist
	TaskListName = "cadence-sys-failoverManager-tasklist"
	// FailoverWorkflowTypeName workflow type name
	FailoverWorkflowTypeName = "cadence-sys-failoverManager-workflow"
	// RebalanceWorkflowTypeName is rebalance workflow type name
	RebalanceWorkflowTypeName = "cadence-sys-rebalance-workflow"
	// WorkflowID will be reused to ensure only one workflow running
	FailoverWorkflowID              = "cadence-failover-manager"
	RebalanceWorkflowID             = "cadence-rebalance-workflow"
	DrillWorkflowID                 = FailoverWorkflowID + "-drill"
	failoverActivityName            = "cadence-sys-failover-activity"
	rebalanceActivityName           = "cadence-sys-rebalance-activity"
	getDomainsActivityName          = "cadence-sys-getDomains-activity"
	getRebalanceDomainsActivityName = "cadence-sys-getRebalanceDomains-activity"

	defaultBatchFailoverSize              = 20
	defaultBatchFailoverWaitTimeInSeconds = 30

	errMsgParamsIsNil                 = "params is nil"
	errMsgTargetClusterIsEmpty        = "targetCluster is empty"
	errMsgSourceClusterIsEmpty        = "sourceCluster is empty"
	errMsgTargetClusterIsSameAsSource = "targetCluster is same as sourceCluster"

	// QueryType for failover workflow
	QueryType = "state"
	// PauseSignal signal name for pause
	PauseSignal = "pause"
	// ResumeSignal signal name for resume
	ResumeSignal = "resume"

	// workflow states for query

	// WorkflowInitialized state
	WorkflowInitialized = "initialized"
	// WorkflowRunning state
	WorkflowRunning = "running"
	// WorkflowPaused state
	WorkflowPaused = "paused"
	// WorkflowCompleted state
	WorkflowCompleted = "complete"
	// WorkflowAborted state
	WorkflowAborted = "aborted"

	unknownOperator = "unknown"
)

type (
	// FailoverParams is the arg for FailoverWorkflow
	FailoverParams struct {
		// TargetCluster is the destination of failover
		TargetCluster string
		// SourceCluster is from which cluster the domains are active before failover
		SourceCluster string
		// BatchFailoverSize is number of domains to failover in one batch
		BatchFailoverSize int
		// BatchFailoverWaitTimeInSeconds is the waiting time between batch failover
		BatchFailoverWaitTimeInSeconds int
		// Domains candidates to be failover
		Domains []string
		// DrillWaitTime defines the wait time of a failover drill
		DrillWaitTime time.Duration
		// GracefulFailoverTimeoutInSeconds
		// TODO: Remove graceful and use the Domains Failover Preferences instead once that is supported
		GracefulFailoverTimeoutInSeconds *int32
		// ClusterAttributes, when non-empty, triggers failover for active-active domains.
		// Each listed scope+name pair is moved to TargetCluster via UpdateDomain.ActiveClusters
		// instead of ActiveClusterName. GetDomainsActivity also switches to active-active mode
		// when this is set.
		ClusterAttributes []types.ClusterAttribute `json:",omitempty"`
	}

	// FailoverResult is workflow result
	FailoverResult struct {
		SuccessDomains      []string
		FailedDomains       []string
		SuccessResetDomains []string
		FailedResetDomains  []string
	}

	// GetDomainsActivityParams params for activity
	GetDomainsActivityParams struct {
		TargetCluster string
		SourceCluster string
		// Domains can be passed to filter to a specific subset of domains in the cadence environment.
		Domains []string
		// ClusterAttributes, when non-empty, triggers failover for active-active domains.
		// Each listed scope+name pair is moved to TargetCluster via UpdateDomain.ActiveClusters
		// instead of ActiveClusterName. GetDomainsActivity also switches to active-active mode
		// when this is set.
		ClusterAttributes []types.ClusterAttribute `json:",omitempty"`
	}

	// FailoverActivityParams params for FailoverActivity. The activity iterates over
	// DomainPreferences and applies each entry.
	FailoverActivityParams struct {
		DomainPreferences []DomainFailoverPreferences
		// TODO: Remove graceful and use the Domains Failover Preferences instead once that is supported
		GracefulFailoverTimeoutInSeconds *int32
	}

	// FailoverActivityResult result for failover (and rebalance) activities.
	FailoverActivityResult struct {
		SuccessDomains []string
		FailedDomains  []string
	}

	// QueryResult for failover progress
	QueryResult struct {
		TotalDomains        int
		Success             int
		Failed              int
		State               string
		TargetCluster       string
		SourceCluster       string
		SuccessDomains      []string // SuccessDomains are guaranteed succeed processed
		FailedDomains       []string // FailedDomains contains false positive
		SuccessResetDomains []string // SuccessResetDomains are domains successfully reset in drill mode
		FailedResetDomains  []string // FailedResetDomains contains false positive in drill mode
		Operator            string
	}

	// failoverState is the mutable in-workflow state used by both the workflow body and
	// the query handler closure.
	failoverState struct {
		totalDomains        int
		successDomains      []string
		failedDomains       []string
		successResetDomains []string
		failedResetDomains  []string
		wfState             string
		targetCluster       string
		sourceCluster       string
		operator            string
	}
)

// FailoverWorkflow manages failover for all domains with IsManagedByCadence=true. The workflow:
//  1. Sets up a query handler so callers can poll progress.
//  2. Asks GetDomainsActivity to filter the candidate domain list down to those eligible for failover.
//  3. Builds a per-domain DomainFailoverPreferences map from TargetCluster + ClusterAttributes.
//  4. Runs FailoverActivity in batches with pause/resume support.
//  5. If DrillWaitTime > 0, sleeps and then runs again with PreferredCluster reversed to SourceCluster.
func FailoverWorkflow(ctx workflow.Context, params *FailoverParams) (*FailoverResult, error) {
	if err := validateParams(params); err != nil {
		return nil, err
	}

	state := &failoverState{
		wfState:       WorkflowInitialized,
		targetCluster: params.TargetCluster,
		sourceCluster: params.SourceCluster,
		operator:      getOperator(ctx),
	}
	if err := setupFailoverQueryHandler(ctx, state); err != nil {
		return nil, err
	}

	domains, err := executeGetDomainsActivity(ctx, params)
	if err != nil {
		return nil, err
	}
	state.totalDomains = len(domains)

	checkPauseSignal := newPauseHandler(ctx, state)

	waitBetween := time.Duration(params.BatchFailoverWaitTimeInSeconds) * time.Second
	forwardPrefs := buildFailoverPrefsFromParams(domains, params.TargetCluster, params.ClusterAttributes)
	state.successDomains, state.failedDomains = processDomainsInBatches(
		ctx,
		forwardPrefs,
		params.BatchFailoverSize,
		waitBetween,
		checkPauseSignal,
		executeFailoverBatch(params.GracefulFailoverTimeoutInSeconds),
	)

	if params.DrillWaitTime == 0 {
		state.wfState = WorkflowCompleted
		return &FailoverResult{
			SuccessDomains: state.successDomains,
			FailedDomains:  state.failedDomains,
		}, nil
	}

	workflow.Sleep(ctx, params.DrillWaitTime)

	reversedPrefs := buildFailoverPrefsFromParams(domains, params.SourceCluster, params.ClusterAttributes)
	state.successResetDomains, state.failedResetDomains = processDomainsInBatches(
		ctx,
		reversedPrefs,
		params.BatchFailoverSize,
		waitBetween,
		checkPauseSignal,
		executeFailoverBatch(params.GracefulFailoverTimeoutInSeconds),
	)
	state.wfState = WorkflowCompleted

	return &FailoverResult{
		SuccessDomains:      state.successDomains,
		FailedDomains:       state.failedDomains,
		SuccessResetDomains: state.successResetDomains,
		FailedResetDomains:  state.failedResetDomains,
	}, nil
}

// GetDomainsActivity fetches all domains that should be failed over.
func GetDomainsActivity(ctx context.Context, params *GetDomainsActivityParams) ([]string, error) {
	err := validateGetDomainsActivityParams(params)
	if err != nil {
		return nil, err
	}
	domains, err := getAllDomains(ctx, params.Domains)
	if err != nil {
		return nil, err
	}
	var res []string
	for _, domain := range domains {
		if shouldFailover(domain, params.SourceCluster, params.ClusterAttributes) {
			res = append(res, domain.GetDomainInfo().GetName())
		}
	}
	return res, nil
}

// FailoverActivity applies the supplied per-domain DomainFailoverPreferences. It is used by
// FailoverWorkflow; GracefulFailoverTimeoutInSeconds is applied to every UpdateDomain call when
// set.
func FailoverActivity(ctx context.Context, params *FailoverActivityParams) (*FailoverActivityResult, error) {
	return applyDomainPreferences(ctx, params.DomainPreferences, params.GracefulFailoverTimeoutInSeconds)
}

// setupFailoverQueryHandler registers the QueryType handler. The handler closes over state so
// QueryWorkflow callers see the latest values as the workflow advances.
func setupFailoverQueryHandler(ctx workflow.Context, state *failoverState) error {
	return workflow.SetQueryHandler(ctx, QueryType, func(input []byte) (*QueryResult, error) {
		return &QueryResult{
			TotalDomains:        state.totalDomains,
			Success:             len(state.successDomains),
			Failed:              len(state.failedDomains),
			State:               state.wfState,
			TargetCluster:       state.targetCluster,
			SourceCluster:       state.sourceCluster,
			SuccessDomains:      state.successDomains,
			FailedDomains:       state.failedDomains,
			SuccessResetDomains: state.successResetDomains,
			FailedResetDomains:  state.failedResetDomains,
			Operator:            state.operator,
		}, nil
	})
}

// newPauseHandler returns a hook that processDomainsInBatches calls at the start of each batch.
// A PauseSignal blocks until ResumeSignal arrives and toggles state.wfState accordingly so the
// query handler reflects the pause.
func newPauseHandler(ctx workflow.Context, state *failoverState) func() {
	pauseCh := workflow.GetSignalChannel(ctx, PauseSignal)
	resumeCh := workflow.GetSignalChannel(ctx, ResumeSignal)
	return func() {
		if pauseCh.ReceiveAsync(nil) {
			state.wfState = WorkflowPaused
			resumeCh.Receive(ctx, nil)
			cleanupChannel(pauseCh)
		}
		state.wfState = WorkflowRunning
	}
}

// executeGetDomainsActivity invokes GetDomainsActivity with the workflow's filter inputs and
// returns the eligible domain list.
func executeGetDomainsActivity(ctx workflow.Context, params *FailoverParams) ([]string, error) {
	ao := workflow.WithActivityOptions(ctx, getGetDomainsActivityOptions())
	getDomainsParams := &GetDomainsActivityParams{
		TargetCluster:     params.TargetCluster,
		SourceCluster:     params.SourceCluster,
		Domains:           params.Domains,
		ClusterAttributes: params.ClusterAttributes,
	}
	var domains []string
	if err := workflow.ExecuteActivity(ao, GetDomainsActivity, getDomainsParams).Get(ctx, &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// executeFailoverBatch returns a batchExecutor that invokes FailoverActivity for the given batch.
// On activity error every domain in the batch is reported as failed (false-positive semantics
// match the original behaviour).
// TODO: Remove graceful and use the Domains Failover Preferences instead once that is supported
func executeFailoverBatch(graceful *int32) batchExecutor {
	return func(ctx workflow.Context, batch []DomainFailoverPreferences) (success, failed []string) {
		ao := workflow.WithActivityOptions(ctx, getFailoverActivityOptions())
		actParams := &FailoverActivityParams{
			DomainPreferences:                batch,
			GracefulFailoverTimeoutInSeconds: graceful,
		}
		var actResult FailoverActivityResult
		if err := workflow.ExecuteActivity(ao, FailoverActivity, actParams).Get(ctx, &actResult); err != nil {
			return nil, domainNames(batch)
		}
		return actResult.SuccessDomains, actResult.FailedDomains
	}
}

// buildFailoverPrefsFromParams converts a CLI-shaped failover request (TargetCluster +
// optional ClusterAttributes) into a per-domain DomainFailoverPreferences slice the activity
// consumes.
func buildFailoverPrefsFromParams(domains []string, targetCluster string, attrs []types.ClusterAttribute) []DomainFailoverPreferences {
	if len(domains) == 0 {
		return nil
	}
	var updates []ClusterAttributePreference
	if len(attrs) > 0 {
		updates = make([]ClusterAttributePreference, 0, len(attrs))
		for _, a := range attrs {
			updates = append(updates, ClusterAttributePreference{
				Scope:            a.GetScope(),
				Name:             a.GetName(),
				PreferredCluster: targetCluster,
			})
		}
	}
	out := make([]DomainFailoverPreferences, 0, len(domains))
	for _, d := range domains {
		out = append(out, DomainFailoverPreferences{
			DomainName:              d,
			PreferredCluster:        targetCluster,
			ClusterAttributeUpdates: updates,
		})
	}
	return out
}

func getOperator(ctx workflow.Context) string {
	memo := workflow.GetInfo(ctx).Memo
	if memo == nil || len(memo.Fields) == 0 {
		return unknownOperator
	}
	opBytes, ok := memo.Fields[constants.MemoKeyForOperator]
	if !ok {
		return unknownOperator
	}
	var operator string
	err := json.Unmarshal(opBytes, &operator)
	if err != nil {
		return unknownOperator
	}
	return operator
}

func getGetDomainsActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		ScheduleToStartTimeout: 10 * time.Second,
		StartToCloseTimeout:    20 * time.Second,
		RetryPolicy: &cadence.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    1 * time.Minute,
			ExpirationInterval: 10 * time.Minute,
			NonRetriableErrorReasons: []string{
				errMsgParamsIsNil,
				errMsgTargetClusterIsEmpty,
				errMsgSourceClusterIsEmpty,
				errMsgTargetClusterIsSameAsSource},
		},
	}
}

func getFailoverActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		ScheduleToStartTimeout: 10 * time.Second,
		StartToCloseTimeout:    10 * time.Second,
	}
}

func validateParams(params *FailoverParams) error {
	if params == nil {
		return errors.New(errMsgParamsIsNil)
	}
	if params.BatchFailoverSize <= 0 {
		params.BatchFailoverSize = defaultBatchFailoverSize
	}
	if params.BatchFailoverWaitTimeInSeconds <= 0 {
		params.BatchFailoverWaitTimeInSeconds = defaultBatchFailoverWaitTimeInSeconds
	}
	return validateTargetAndSourceCluster(params.TargetCluster, params.SourceCluster)
}

func validateGetDomainsActivityParams(params *GetDomainsActivityParams) error {
	if params == nil {
		return errors.New(errMsgParamsIsNil)
	}
	return validateTargetAndSourceCluster(params.TargetCluster, params.SourceCluster)
}

func validateTargetAndSourceCluster(targetCluster, sourceCluster string) error {
	if len(targetCluster) == 0 {
		return errors.New(errMsgTargetClusterIsEmpty)
	}
	if len(sourceCluster) == 0 {
		return errors.New(errMsgSourceClusterIsEmpty)
	}
	if sourceCluster == targetCluster {
		return errors.New(errMsgTargetClusterIsSameAsSource)
	}
	return nil
}

// shouldFailover returns true if the domain should be included in a failover run.
//
// A domain is included when either of the following is true:
//  1. Its default active cluster (ActiveClusterName) matches sourceCluster.
//  2. A clusterAttributeFilter is provided and at least one of the listed cluster
//     attributes is currently active on sourceCluster in the domain's ActiveClusters config.
//
// Both conditions are checked regardless of whether the domain has active-active configuration,
// so plain global domains and active-active domains are treated uniformly.
func shouldFailover(domain *types.DescribeDomainResponse, sourceCluster string, clusterAttributeFilter []types.ClusterAttribute) bool {
	if !isDomainEligibleForFailover(domain) {
		return false
	}

	if domain.ReplicationConfiguration.GetActiveClusterName() == sourceCluster {
		return true
	}

	ac := domain.ReplicationConfiguration.GetActiveClusters()
	if ac != nil {
		for _, attr := range clusterAttributeFilter {
			if info, err := ac.GetActiveClusterByClusterAttribute(attr.GetScope(), attr.GetName()); err == nil {
				if info.ActiveClusterName == sourceCluster {
					return true
				}
			}
		}
	}
	return false
}

func getClient(ctx context.Context) frontend.Client {
	manager := ctx.Value(failoverManagerContextKey).(*FailoverManager)
	feClient := manager.clientBean.GetFrontendClient()
	return feClient
}

func getRemoteClient(ctx context.Context, clusterName string) (frontend.Client, error) {
	manager := ctx.Value(failoverManagerContextKey).(*FailoverManager)
	return manager.clientBean.GetRemoteFrontendClient(clusterName)
}

func getAllDomains(ctx context.Context, targetDomains []string) ([]*types.DescribeDomainResponse, error) {
	feClient := getClient(ctx)
	var res []*types.DescribeDomainResponse

	isTargetDomainsProvided := len(targetDomains) > 0
	targetDomainsSet := make(map[string]struct{})
	if isTargetDomainsProvided {
		for _, domain := range targetDomains {
			targetDomainsSet[domain] = struct{}{}
		}
	}

	pagesize := int32(200)
	var token []byte
	for more := true; more; more = len(token) > 0 {
		listRequest := &types.ListDomainsRequest{
			PageSize:      pagesize,
			NextPageToken: token,
		}
		listResp, err := feClient.ListDomains(ctx, listRequest)
		if err != nil {
			return nil, err
		}
		token = listResp.GetNextPageToken()

		if isTargetDomainsProvided {
			for _, domain := range listResp.GetDomains() {
				if _, ok := targetDomainsSet[domain.DomainInfo.GetName()]; ok {
					res = append(res, domain)
				}
			}
		} else {
			res = append(res, listResp.GetDomains()...)
		}

		activity.RecordHeartbeat(ctx, len(res))
	}
	return res, nil
}

// applyDomainPreferences is the shared per-domain loop used by both FailoverActivity and
// RebalanceActivity. For each prefs entry it validates pollers against every distinct target
// cluster referenced by that entry, then issues a single UpdateDomain call carrying the preferred
// ActiveClusterName and any per-attribute ActiveClusters overrides. A nil gracefulTimeout means
// "do not set FailoverTimeoutInSeconds on the request".
func applyDomainPreferences(
	ctx context.Context,
	prefs []DomainFailoverPreferences,
	gracefulTimeout *int32,
) (*FailoverActivityResult, error) {
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
		if gracefulTimeout != nil {
			updateRequest.FailoverTimeoutInSeconds = gracefulTimeout
		}

		if _, err := frontendClient.UpdateDomain(ctx, updateRequest); err != nil {
			failedDomains = append(failedDomains, p.DomainName)
		} else {
			successDomains = append(successDomains, p.DomainName)
		}
	}
	return &FailoverActivityResult{
		SuccessDomains: successDomains,
		FailedDomains:  failedDomains,
	}, nil
}

// buildActiveClustersFromUpdates constructs an ActiveClusters payload from per-domain
// ClusterAttributePreferences. Each entry sets one scope:name attribute to its preferred cluster.
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

// collectTargetClusters returns the unique set of target clusters referenced by a domain's preferences.
// This is used to run poller validation against every cluster the domain's attributes or
// ActiveClusterName will move to.
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

func cleanupChannel(channel workflow.Channel) {
	for {
		if hasValue := channel.ReceiveAsync(nil); !hasValue {
			return
		}
	}
}

func validateTaskListPollerInfo(ctx context.Context, targetCluster string, domain string) error {
	remoteFrontendClient, err := getRemoteClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	frontendClient := getClient(ctx)
	localTaskListResponse, err := frontendClient.GetTaskListsByDomain(ctx, &types.GetTaskListsByDomainRequest{Domain: domain})
	if err != nil {
		return fmt.Errorf("failed to get task list for domain %s", domain)
	}

	remoteTaskListRepsonse, err := remoteFrontendClient.GetTaskListsByDomain(ctx, &types.GetTaskListsByDomainRequest{Domain: domain})
	if err != nil {
		return fmt.Errorf("failed to get task list for domain %s", domain)
	}
	for name, tl := range localTaskListResponse.GetDecisionTaskListMap() {
		if len(tl.GetPollers()) != 0 {
			remoteTaskList, ok := remoteTaskListRepsonse.GetDecisionTaskListMap()[name]
			if !ok || len(remoteTaskList.GetPollers()) == 0 {
				return fmt.Errorf("received zero poller in decision task list %s with domain %s", name, domain)
			}
		}
	}
	for name, tl := range localTaskListResponse.GetActivityTaskListMap() {
		if len(tl.GetPollers()) != 0 {
			remoteTaskList, ok := remoteTaskListRepsonse.GetActivityTaskListMap()[name]
			if !ok || len(remoteTaskList.GetPollers()) == 0 {
				return fmt.Errorf("received zero poller in decision task list %s with domain %s", name, domain)
			}
		}
	}
	return nil
}
