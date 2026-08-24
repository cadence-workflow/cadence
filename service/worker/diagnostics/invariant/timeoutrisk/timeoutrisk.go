package timeoutrisk

import (
	"context"
	"math"
	"time"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

// TimeoutRisk is an invariant that will be used to identify activity configurations in the workflow
// execution history that put the workflow at risk of timing out, even though no timeout has occurred yet.
type TimeoutRisk invariant.Invariant

type timeoutRisk struct {
}

func NewInvariant() TimeoutRisk {
	return &timeoutRisk{}
}

func (t *timeoutRisk) Check(ctx context.Context, params invariant.InvariantCheckInput) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)
	events := params.WorkflowExecutionHistory.GetHistory().GetEvents()
	issueID := 0

	workflowTimeoutSeconds := fetchWorkflowExecutionTimeoutSeconds(events)

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
		// sync; the standby's activity timeout task is discarded after standbyTaskMissingEventsDiscardDelay
		// (default 25m) with no replacement, so a failover during a retry sequence extending past that
		// delay can orphan the activity until its workflow's tasks are refreshed.
		policy := attr.RetryPolicy
		if policy != nil && (policy.GetMaximumAttempts() == 0 || policy.GetMaximumAttempts() >= 2) {
			estimatedRetryWindow := estimatedRetryWindowSeconds(policy, attr.GetStartToCloseTimeoutSeconds(), workflowTimeoutSeconds)
			if estimatedRetryWindow > failoverOrphanRiskThresholdSeconds {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: ActivityRetryWindowExceedsStandbyDiscardDelay.String(),
					Reason:        RetryWindowExceedsStandbyDiscardDelay.String(),
					Metadata: invariant.MarshalData(ActivityRetryWindowExceedsStandbyDiscardDelayMetadata{
						EventID:              event.ID,
						ActivityID:           activityID,
						ActivityType:         activityType,
						EstimatedRetryWindow: time.Duration(estimatedRetryWindow) * time.Second,
						Threshold:            time.Duration(failoverOrphanRiskThresholdSeconds) * time.Second,
						RetryPolicy:          policy,
					}),
				})
				issueID++
			}
		}
	}

	return result, nil
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
// attempt budget. The result is always capped at the workflow timeout when one is known.
func estimatedRetryWindowSeconds(policy *types.RetryPolicy, startToCloseSeconds int32, workflowTimeoutSeconds int32) int32 {
	if policy == nil {
		return 0
	}

	var windowSeconds int64
	switch {
	case policy.GetExpirationIntervalInSeconds() > 0:
		windowSeconds = int64(policy.GetExpirationIntervalInSeconds())
	case policy.GetMaximumAttempts() == 0:
		if workflowTimeoutSeconds == 0 {
			// No started event to establish a baseline against -- conservatively report no window,
			// mirroring the guard on check 1.
			return 0
		}
		windowSeconds = int64(workflowTimeoutSeconds)
	default:
		windowSeconds = cumulativeRetryWindowSeconds(policy, startToCloseSeconds, workflowTimeoutSeconds)
	}

	if workflowTimeoutSeconds > 0 && windowSeconds > int64(workflowTimeoutSeconds) {
		windowSeconds = int64(workflowTimeoutSeconds)
	}
	if windowSeconds > math.MaxInt32 {
		windowSeconds = math.MaxInt32
	}
	return int32(windowSeconds)
}

// cumulativeRetryWindowSeconds estimates the retry window for a bounded, non-expiring retry policy as the
// sum of the backoff delays between attempts (each capped at MaximumIntervalInSeconds when configured) plus
// one StartToCloseTimeout per attempt. The loop exits early once the running backoff total exceeds the
// workflow timeout (or int32 range), since the estimate is already conclusive at that point.
func cumulativeRetryWindowSeconds(policy *types.RetryPolicy, startToCloseSeconds int32, workflowTimeoutSeconds int32) int64 {
	limit := int64(math.MaxInt32)
	if workflowTimeoutSeconds > 0 {
		limit = int64(workflowTimeoutSeconds)
	}

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
