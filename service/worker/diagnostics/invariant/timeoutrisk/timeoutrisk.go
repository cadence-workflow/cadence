package timeoutrisk

import (
	"context"
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

func (t *timeoutRisk) RootCause(ctx context.Context, params invariant.InvariantRootCauseInput) ([]invariant.InvariantRootCauseResult, error) {
	// Not implemented since this invariant does not have any root cause.
	// The issues identified in Check() are static configuration risks that are actionable on their own.
	result := make([]invariant.InvariantRootCauseResult, 0)
	return result, nil
}
