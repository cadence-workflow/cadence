package timeoutrisk

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/.gen/go/shared"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

// TimeoutRisk is an invariant that will be used to identify activity configurations in the workflow
// execution history that put the workflow at risk of timing out, even though no timeout has occurred yet.
type TimeoutRisk invariant.Invariant

type timeoutRisk struct {
	client                      workflowserviceclient.Interface
	failoverOrphanRiskThreshold dynamicproperties.DurationPropertyFn
}

type Params struct {
	Client                      workflowserviceclient.Interface
	FailoverOrphanRiskThreshold dynamicproperties.DurationPropertyFn
}

func NewInvariant(p Params) TimeoutRisk {
	return &timeoutRisk{
		client:                      p.Client,
		failoverOrphanRiskThreshold: p.FailoverOrphanRiskThreshold,
	}
}

func (t *timeoutRisk) Check(ctx context.Context, params invariant.InvariantCheckInput) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)
	events := params.WorkflowExecutionHistory.GetHistory().GetEvents()
	issueID := 0

	workflowTimeoutSeconds := fetchWorkflowExecutionTimeoutSeconds(events)

	var isGlobalDomainResolved, isGlobalDomain bool

	for _, event := range events {
		attr := event.GetActivityTaskScheduledEventAttributes()
		if attr == nil {
			continue
		}

		activityID := attr.GetActivityID()
		activityType := attr.GetActivityType().GetName()

		// Equality with workflow timeout means StartToClose was silently capped
		// (validateActivityScheduleAttributes). ScheduleToStart/ScheduleToClose
		// excluded: retries inflate them to the cap.
		if workflowTimeoutSeconds > 0 && attr.GetStartToCloseTimeoutSeconds() == workflowTimeoutSeconds {
			result = append(result, invariant.InvariantCheckResult{
				IssueID:       issueID,
				InvariantType: ActivityStartToCloseAtWorkflowTimeoutCap.String(),
				Reason:        StartToCloseAtWorkflowTimeoutCap.String(),
				Metadata: invariant.MarshalData(ActivityStartToCloseAtWorkflowTimeoutCapMetadata{
					EventID:             event.ID,
					ActivityID:          activityID,
					ActivityType:        activityType,
					StartToCloseTimeout: time.Duration(attr.GetStartToCloseTimeoutSeconds()) * time.Second,
					WorkflowTimeout:     time.Duration(workflowTimeoutSeconds) * time.Second,
				}),
			})
			issueID++
		}

		if attr.GetStartToCloseTimeoutSeconds() >= longRunningActivityThresholdSeconds && attr.GetHeartbeatTimeoutSeconds() == 0 {
			result = append(result, invariant.InvariantCheckResult{
				IssueID:       issueID,
				InvariantType: ActivityMissingHeartbeatTimeout.String(),
				Reason:        MissingHeartbeatTimeoutForLongActivity.String(),
				Metadata: invariant.MarshalData(ActivityMissingHeartbeatTimeoutMetadata{
					EventID:             event.ID,
					ActivityID:          activityID,
					ActivityType:        activityType,
					StartToCloseTimeout: time.Duration(attr.GetStartToCloseTimeoutSeconds()) * time.Second,
					Threshold:           time.Duration(longRunningActivityThresholdSeconds) * time.Second,
				}),
			})
			issueID++
		}

		// Activity retries are invisible in history and replicate to standby clusters only via activity
		// sync; the standby's activity timeout task is discarded after the configured discard delay with
		// no replacement, so a failover during a retry sequence extending past that delay can orphan the
		// activity until its workflow's tasks are refreshed. Only global (multi-cluster) domains have a
		// standby cluster to fail over to, so single-cluster domains carry no such risk.
		policy := attr.RetryPolicy
		if policy != nil && (policy.GetMaximumAttempts() == 0 || policy.GetMaximumAttempts() >= 2) {
			if !isGlobalDomainResolved {
				var err error
				isGlobalDomain, err = t.isGlobalDomain(ctx, params.Domain)
				if err != nil {
					// Diagnostics are best-effort: a failed domain lookup skips this check for
					// the run rather than failing the activity, which would discard every
					// invariant's findings.
					isGlobalDomain = false
				}
				isGlobalDomainResolved = true
			}

			if isGlobalDomain {
				threshold := int32(t.failoverOrphanRiskThreshold().Seconds())
				estimatedRetryWindow := estimatedRetryWindowSeconds(policy, attr.GetStartToCloseTimeoutSeconds(), workflowTimeoutSeconds)
				if estimatedRetryWindow > threshold {
					result = append(result, invariant.InvariantCheckResult{
						IssueID:       issueID,
						InvariantType: ActivityRetryWindowExceedsStandbyDiscardDelay.String(),
						Reason:        RetryWindowExceedsStandbyDiscardDelay.String(),
						Metadata: invariant.MarshalData(ActivityRetryWindowExceedsStandbyDiscardDelayMetadata{
							EventID:              event.ID,
							ActivityID:           activityID,
							ActivityType:         activityType,
							EstimatedRetryWindow: time.Duration(estimatedRetryWindow) * time.Second,
							Threshold:            time.Duration(threshold) * time.Second,
							RetryPolicy:          policy,
						}),
					})
					issueID++
				}
			}
		}
	}

	return result, nil
}

// isGlobalDomain reports whether the domain is a global (multi-cluster) domain -- only such domains have a
// standby cluster to fail over to, so this gates the retry-window check to where the risk actually applies.
func (t *timeoutRisk) isGlobalDomain(ctx context.Context, domain string) (bool, error) {
	resp, err := t.client.DescribeDomain(ctx, &shared.DescribeDomainRequest{Name: &domain})
	if err != nil {
		return false, fmt.Errorf("failed to describe domain: %w", err)
	}
	return resp.GetIsGlobalDomain(), nil
}

// fetchWorkflowExecutionTimeoutSeconds returns the workflow's configured ExecutionStartToCloseTimeout in
// seconds, or 0 if the WorkflowExecutionStartedEventAttributes could not be found in the history.
func fetchWorkflowExecutionTimeoutSeconds(events []*types.HistoryEvent) int32 {
	for _, event := range events {
		if startedAttr := event.GetWorkflowExecutionStartedEventAttributes(); startedAttr != nil {
			return startedAttr.GetExecutionStartToCloseTimeoutSeconds()
		}
	}
	return 0
}

// estimatedRetryWindowSeconds estimates how long an activity's retry sequence could keep extending, based
// on its retry policy. Retries mutate mutable state and replicate to standby via activity sync without
// emitting new history events, so this window cannot be read off the scheduled event directly -- it is
// derived the same way the server would internally: an explicit expiration wins outright; an unlimited
// attempt budget rides out to the workflow timeout; otherwise it is the cumulative estimate for a bounded
// attempt budget. The result is always capped at the workflow timeout.
func estimatedRetryWindowSeconds(policy *types.RetryPolicy, startToCloseSeconds int32, workflowTimeoutSeconds int32) int32 {
	if policy == nil {
		return 0
	}
	if workflowTimeoutSeconds == 0 {
		// No started event to establish a baseline against -- conservatively report no window,
		// mirroring the guard on check 1. This also keeps the cumulative estimate's loop bounded.
		return 0
	}

	var windowSeconds int64
	switch {
	case policy.GetExpirationIntervalInSeconds() > 0:
		windowSeconds = int64(policy.GetExpirationIntervalInSeconds())
	case policy.GetMaximumAttempts() == 0:
		windowSeconds = int64(workflowTimeoutSeconds)
	default:
		windowSeconds = cumulativeRetryWindowSeconds(policy, startToCloseSeconds, workflowTimeoutSeconds)
	}

	if windowSeconds > int64(workflowTimeoutSeconds) {
		windowSeconds = int64(workflowTimeoutSeconds)
	}
	return int32(windowSeconds)
}

// cumulativeRetryWindowSeconds estimates the retry window for a bounded, non-expiring retry policy as the
// sum of the backoff delays between attempts (each capped at MaximumIntervalInSeconds when configured) plus
// one StartToCloseTimeout per attempt. Callers guarantee a positive workflow timeout; the loop exits early
// once the running backoff total exceeds it, since the estimate is already conclusive at that point.
func cumulativeRetryWindowSeconds(policy *types.RetryPolicy, startToCloseSeconds int32, workflowTimeoutSeconds int32) int64 {
	limit := int64(workflowTimeoutSeconds)

	coefficient := policy.GetBackoffCoefficient()
	if coefficient < 1 {
		coefficient = 1
	}
	maximumIntervalSeconds := int64(policy.GetMaximumIntervalInSeconds())

	interval := float64(policy.GetInitialIntervalInSeconds())
	var backoffTotal int64
	for attempt := int32(1); attempt < policy.GetMaximumAttempts(); attempt++ {
		step := interval
		if maximumIntervalSeconds > 0 && step > float64(maximumIntervalSeconds) {
			step = float64(maximumIntervalSeconds)
		}
		// exponential growth can exceed int64 range, where float-to-int conversion is implementation-defined
		if step > float64(math.MaxInt32) {
			step = float64(math.MaxInt32)
		}
		// retry policy validation enforces a positive initial interval, so a well-formed policy always
		// advances by at least a second; guard anyway so the early exit below is guaranteed to be reached
		if step < 1 {
			step = 1
		}
		backoffTotal += int64(step)
		if backoffTotal > limit {
			return backoffTotal
		}
		interval *= coefficient
	}

	return backoffTotal + int64(policy.GetMaximumAttempts())*int64(startToCloseSeconds)
}

func (t *timeoutRisk) RootCause(ctx context.Context, params invariant.InvariantRootCauseInput) ([]invariant.InvariantRootCauseResult, error) {
	// Not implemented since this invariant does not have any root cause.
	// The issues identified in Check() are static configuration risks that are actionable on their own.
	result := make([]invariant.InvariantRootCauseResult, 0)
	return result, nil
}
