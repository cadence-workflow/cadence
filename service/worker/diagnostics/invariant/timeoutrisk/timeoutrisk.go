// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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

		// The server caps activity timeouts at the workflow execution timeout before recording them
		// (validateActivityScheduleAttributes in service/history/decision/checker.go), so a silently
		// capped StartToClose surfaces as equality with the workflow timeout. ScheduleToStart and
		// ScheduleToClose are excluded: the server inflates them to the cap under retry policies.
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

		policy := attr.RetryPolicy
		if attr.GetScheduleToStartTimeoutSeconds() >= longScheduleToStartThresholdSeconds &&
			policy != nil && (policy.GetMaximumAttempts() == 0 || policy.GetMaximumAttempts() >= minRetryAttemptsForFailoverRisk) {
			result = append(result, invariant.InvariantCheckResult{
				IssueID:       issueID,
				InvariantType: ActivityLongScheduleToStartWithRetries.String(),
				Reason:        LongScheduleToStartWithMultipleRetries.String(),
				Metadata: invariant.MarshalData(ActivityLongScheduleToStartWithRetriesMetadata{
					EventID:                event.ID,
					ActivityID:             activityID,
					ActivityType:           activityType,
					ScheduleToStartTimeout: time.Duration(attr.GetScheduleToStartTimeoutSeconds()) * time.Second,
					Threshold:              time.Duration(longScheduleToStartThresholdSeconds) * time.Second,
					RetryPolicy:            policy,
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
