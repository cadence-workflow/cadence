package retry

import (
	"context"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

// Retry is an invariant that will be used to identify the issues regarding retries in the workflow execution history
type Retry invariant.Invariant

type retry struct {
}

func NewInvariant() Retry {
	return &retry{}
}

func (r *retry) Check(ctx context.Context, params invariant.InvariantCheckInput) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)
	events := params.WorkflowExecutionHistory.GetHistory().GetEvents()
	issueID := 0
	startedEvent := fetchWfStartedEvent(events)

	if issue := checkRetryPolicy(startedEvent.RetryPolicy); issue != "" {
		result = append(result, invariant.InvariantCheckResult{
			IssueID:       issueID,
			InvariantType: WorkflowRetryIssue.String(),
			Reason:        issue.String(),
			Metadata: invariant.MarshalData(RetryMetadata{
				EventID:     1,
				RetryPolicy: startedEvent.RetryPolicy,
			}),
		})
		issueID++
	}

	for _, event := range events {
		if event.GetActivityTaskScheduledEventAttributes() != nil {
			attr := event.GetActivityTaskScheduledEventAttributes()
			if issue := checkRetryPolicy(attr.RetryPolicy); issue != "" {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: ActivityRetryIssue.String(),
					Reason:        issue.String(),
					Metadata: invariant.MarshalData(RetryMetadata{
						EventID:     event.ID,
						RetryPolicy: attr.RetryPolicy,
					}),
				})
				issueID++
			}
			if attr.GetStartToCloseTimeoutSeconds() <= attr.GetHeartbeatTimeoutSeconds() {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: ActivityHeartbeatIssue.String(),
					Reason:        HeartBeatTimeoutEqualToStartToCloseTimeout.String(),
					Metadata: invariant.MarshalData(RetryMetadata{
						EventID: event.ID,
					}),
				})
				issueID++
			}
		}
	}

	return result, nil
}

func fetchWfStartedEvent(events []*types.HistoryEvent) *types.WorkflowExecutionStartedEventAttributes {
	for _, event := range events {
		if event.GetWorkflowExecutionStartedEventAttributes() != nil {
			return event.GetWorkflowExecutionStartedEventAttributes()
		}
	}
	return nil
}

func checkRetryPolicy(policy *types.RetryPolicy) IssueType {
	if policy == nil {
		return ""
	}
	if policy.GetExpirationIntervalInSeconds() == 0 && policy.GetMaximumAttempts() == 1 {
		return RetryPolicyValidationMaxAttempts
	}
	if policy.GetMaximumAttempts() == 0 && policy.GetExpirationIntervalInSeconds() < policy.GetInitialIntervalInSeconds() {
		return RetryPolicyValidationExpInterval
	}
	return ""
}

func (r *retry) RootCause(ctx context.Context, params invariant.InvariantRootCauseInput) ([]invariant.InvariantRootCauseResult, error) {
	// Not implemented since this invariant does not have any root cause.
	// Issue identified in Check() are the root cause.
	result := make([]invariant.InvariantRootCauseResult, 0)
	return result, nil
}
