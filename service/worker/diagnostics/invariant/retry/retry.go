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

package retry

import (
	"context"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/failure"
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
	issueID := 1
	lastEvent := fetchContinuedAsNewEvent(events)
	startedEvent := fetchWfStartedEvent(events)
	if lastEvent != nil && startedEvent != nil && startedEvent.RetryPolicy != nil {
		result = append(result, invariant.InvariantCheckResult{
			IssueID:       issueID,
			InvariantType: WorkflowRetryInfo.String(),
			Reason:        failure.ErrorTypeFromReason(*lastEvent.FailureReason).String(),
			Metadata: invariant.MarshalData(RetryMetadata{
				EventID:     1,
				RetryPolicy: startedEvent.RetryPolicy,
			}),
		})
		issueID++
	}

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
		}
	}

	return result, nil
}

func fetchContinuedAsNewEvent(events []*types.HistoryEvent) *types.WorkflowExecutionContinuedAsNewEventAttributes {
	for _, event := range events {
		if event.GetWorkflowExecutionContinuedAsNewEventAttributes() != nil {
			return event.GetWorkflowExecutionContinuedAsNewEventAttributes()
		}
	}
	return nil
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
