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

package failure

import (
	"context"
	"strings"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

// Failure is an invariant that will be used to identify the different failures in the workflow execution history
type Failure invariant.Invariant

type failure struct{}

func NewInvariant() Failure {
	return &failure{}
}

func (f *failure) Check(ctx context.Context, params invariant.InvariantCheckInput) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)
	events := params.WorkflowExecutionHistory.GetHistory().GetEvents()
	issueID := 1
	for _, event := range events {
		if event.GetWorkflowExecutionFailedEventAttributes() != nil && event.WorkflowExecutionFailedEventAttributes.Reason != nil {
			attr := event.WorkflowExecutionFailedEventAttributes
			reason := attr.Reason
			identity := fetchIdentity(attr, events)
			if *reason == common.FailureReasonDecisionBlobSizeExceedsLimit {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: DecisionCausedFailure.String(),
					Reason:        DecisionBlobSizeLimit.String(),
					Metadata:      invariant.MarshalData(FailureIssuesMetadata{Identity: identity}),
				})
				issueID++
			} else {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: WorkflowFailed.String(),
					Reason:        ErrorTypeFromReason(*reason).String(),
					Metadata:      invariant.MarshalData(FailureIssuesMetadata{Identity: identity}),
				})
				issueID++
			}

		}
		if event.GetActivityTaskFailedEventAttributes() != nil && event.ActivityTaskFailedEventAttributes.Reason != nil {
			attr := event.ActivityTaskFailedEventAttributes
			reason := attr.Reason
			scheduled := fetchScheduledEvent(attr, events)
			if *reason == common.FailureReasonHeartbeatExceedsLimit {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: ActivityFailed.String(),
					Reason:        HeartBeatBlobSizeLimit.String(),
					Metadata: invariant.MarshalData(FailureIssuesMetadata{
						Identity:            attr.Identity,
						ActivityType:        scheduled.ActivityType.GetName(),
						ActivityScheduledID: attr.ScheduledEventID,
						ActivityStartedID:   attr.StartedEventID,
					}),
				})
				issueID++
			} else if *reason == common.FailureReasonCompleteResultExceedsLimit {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: ActivityFailed.String(),
					Reason:        ActivityOutputBlobSizeLimit.String(),
					Metadata: invariant.MarshalData(FailureIssuesMetadata{
						Identity:            attr.Identity,
						ActivityType:        scheduled.ActivityType.GetName(),
						ActivityScheduledID: attr.ScheduledEventID,
						ActivityStartedID:   attr.StartedEventID,
					}),
				})
				issueID++
			} else {
				result = append(result, invariant.InvariantCheckResult{
					IssueID:       issueID,
					InvariantType: ActivityFailed.String(),
					Reason:        ErrorTypeFromReason(*reason).String(),
					Metadata: invariant.MarshalData(FailureIssuesMetadata{
						Identity:            attr.Identity,
						ActivityType:        scheduled.ActivityType.GetName(),
						ActivityScheduledID: attr.ScheduledEventID,
						ActivityStartedID:   attr.StartedEventID,
					}),
				})
				issueID++
			}

		}
	}
	return result, nil
}

func ErrorTypeFromReason(reason string) ErrorType {
	if strings.Contains(reason, "Generic") {
		return GenericError
	}
	if strings.Contains(reason, "Panic") {
		return PanicError
	}
	if strings.Contains(reason, "Timeout") {
		return TimeoutError
	}
	return CustomError
}

func fetchScheduledEvent(attr *types.ActivityTaskFailedEventAttributes, events []*types.HistoryEvent) *types.ActivityTaskScheduledEventAttributes {
	for _, event := range events {
		if event.ID == attr.GetScheduledEventID() {
			return event.GetActivityTaskScheduledEventAttributes()
		}
	}
	return nil
}

func fetchIdentity(attr *types.WorkflowExecutionFailedEventAttributes, events []*types.HistoryEvent) string {
	for _, event := range events {
		if event.ID == attr.DecisionTaskCompletedEventID {
			return event.GetDecisionTaskCompletedEventAttributes().Identity
		}
	}
	return ""
}

func (f *failure) RootCause(ctx context.Context, params invariant.InvariantRootCauseInput) ([]invariant.InvariantRootCauseResult, error) {
	result := make([]invariant.InvariantRootCauseResult, 0)
	for _, issue := range params.Issues {
		switch issue.Reason {
		case GenericError.String():
			result = append(result, invariant.InvariantRootCauseResult{
				IssueID:   issue.IssueID,
				RootCause: invariant.RootCauseTypeServiceSideIssue,
				Metadata:  issue.Metadata,
			})
		case PanicError.String():
			result = append(result, invariant.InvariantRootCauseResult{
				IssueID:   issue.IssueID,
				RootCause: invariant.RootCauseTypeServiceSidePanic,
				Metadata:  issue.Metadata,
			})
		case CustomError.String():
			result = append(result, invariant.InvariantRootCauseResult{
				IssueID:   issue.IssueID,
				RootCause: invariant.RootCauseTypeServiceSideCustomError,
				Metadata:  issue.Metadata,
			})
		}
	}
	return result, nil
}
