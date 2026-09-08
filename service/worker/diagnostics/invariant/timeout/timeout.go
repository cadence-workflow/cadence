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

package timeout

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/.gen/go/shared"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

type Timeout invariant.Invariant

type timeout struct {
	client workflowserviceclient.Interface
}

type Params struct {
	Client workflowserviceclient.Interface
}

func NewInvariant(p Params) invariant.Invariant {
	return &timeout{
		client: p.Client,
	}
}

func (t *timeout) Check(ctx context.Context, params invariant.InvariantCheckInput) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)
	events := params.WorkflowExecutionHistory.GetHistory().GetEvents()
	issueID := 0
	for _, event := range events {
		if event.WorkflowExecutionTimedOutEventAttributes != nil {
			timeoutLimit := getWorkflowExecutionConfiguredTimeout(events)
			data := TimeoutIssuesMetadata{
				EventID:           event.ID,
				ConfiguredTimeout: time.Duration(timeoutLimit) * time.Second,
				ExecutionTimeout: &ExecutionTimeoutMetadata{
					ExecutionTime:    getExecutionTime(1, event.ID, events),
					LastOngoingEvent: events[len(events)-2],
					Tasklist:         getWorkflowExecutionTasklist(events),
				},
			}
			result = append(result, invariant.InvariantCheckResult{
				IssueID:       issueID,
				InvariantType: TimeoutTypeExecution.String(),
				Reason:        event.GetWorkflowExecutionTimedOutEventAttributes().GetTimeoutType().String(),
				Metadata:      invariant.MarshalData(data),
			})
			issueID++
		}
		if event.ActivityTaskTimedOutEventAttributes != nil {
			metadata, err := getActivityTaskMetadata(event, events)
			if err != nil {
				return nil, err
			}
			result = append(result, invariant.InvariantCheckResult{
				IssueID:       issueID,
				InvariantType: TimeoutTypeActivity.String(),
				Reason:        event.GetActivityTaskTimedOutEventAttributes().GetTimeoutType().String(),
				Metadata:      invariant.MarshalData(metadata),
			})
			issueID++
		}
		if event.DecisionTaskTimedOutEventAttributes != nil {
			reason, metadata := reasonForDecisionTaskTimeouts(event, events)
			result = append(result, invariant.InvariantCheckResult{
				IssueID:       issueID,
				InvariantType: TimeoutTypeDecision.String(),
				Reason:        reason,
				Metadata:      invariant.MarshalData(metadata),
			})
			issueID++
		}
		if event.ChildWorkflowExecutionTimedOutEventAttributes != nil {
			timeoutLimit := getChildWorkflowExecutionConfiguredTimeout(event, events)
			data := TimeoutIssuesMetadata{
				EventID:           event.ID,
				ConfiguredTimeout: time.Duration(timeoutLimit) * time.Second,
				ChildWfTimeout: &ChildWfTimeoutMetadata{
					ExecutionTime: getExecutionTime(event.GetChildWorkflowExecutionTimedOutEventAttributes().StartedEventID, event.ID, events),
					Execution:     event.GetChildWorkflowExecutionTimedOutEventAttributes().WorkflowExecution,
				},
			}
			result = append(result, invariant.InvariantCheckResult{
				IssueID:       issueID,
				InvariantType: TimeoutTypeChildWorkflow.String(),
				Reason:        event.GetChildWorkflowExecutionTimedOutEventAttributes().TimeoutType.String(),
				Metadata:      invariant.MarshalData(data),
			})
			issueID++
		}
	}
	return result, nil
}

func (t *timeout) RootCause(ctx context.Context, params invariant.InvariantRootCauseInput) ([]invariant.InvariantRootCauseResult, error) {
	result := make([]invariant.InvariantRootCauseResult, 0)
	for _, issue := range params.Issues {
		if issue.InvariantType == TimeoutTypeActivity.String() || issue.InvariantType == TimeoutTypeExecution.String() {
			pollerStatus, err := t.checkTasklist(ctx, issue, params.Domain)
			if err != nil {
				return nil, err
			}
			result = append(result, pollerStatus)
		}

		if issue.InvariantType == TimeoutTypeActivity.String() {
			heartbeatStatus, err := checkHeartbeatStatus(issue)
			if err != nil {
				return nil, err
			}
			result = append(result, heartbeatStatus...)
		}
	}
	return result, nil
}

func (t *timeout) checkTasklist(ctx context.Context, issue invariant.InvariantCheckResult, domain string) (invariant.InvariantRootCauseResult, error) {
	var taskList *types.TaskList
	var tasklistType *shared.TaskListType
	var metadata TimeoutIssuesMetadata
	err := json.Unmarshal(issue.Metadata, &metadata)
	if err != nil {
		return invariant.InvariantRootCauseResult{}, err
	}
	switch issue.InvariantType {
	case TimeoutTypeExecution.String():
		tasklistType = shared.TaskListTypeDecision.Ptr()
		if metadata.ExecutionTimeout != nil {
			taskList = metadata.ExecutionTimeout.Tasklist
		}
	case TimeoutTypeActivity.String():
		tasklistType = shared.TaskListTypeActivity.Ptr()
		if metadata.ActivityTimeout != nil {
			taskList = metadata.ActivityTimeout.Tasklist
		}
	}
	if taskList == nil {
		return invariant.InvariantRootCauseResult{}, fmt.Errorf("tasklist not set")
	}

	resp, err := t.client.DescribeTaskList(ctx, &shared.DescribeTaskListRequest{
		Domain: &domain,
		TaskList: &shared.TaskList{
			Name: &taskList.Name,
			Kind: taskListKind(taskList.GetKind()),
		},
		TaskListType: tasklistType,
	})
	if err != nil {
		return invariant.InvariantRootCauseResult{}, err
	}

	tasklistBacklog := resp.GetTaskListStatus().GetBacklogCountHint()
	polllersMetadataInBytes := invariant.MarshalData(PollersMetadata{
		TaskListName:    taskList.Name,
		TaskListBacklog: tasklistBacklog,
	})
	if len(resp.GetPollers()) == 0 {
		return invariant.InvariantRootCauseResult{
			IssueID:   issue.IssueID,
			RootCause: invariant.RootCauseTypeMissingPollers,
			Metadata:  polllersMetadataInBytes,
		}, nil
	}
	return invariant.InvariantRootCauseResult{
		IssueID:   issue.IssueID,
		RootCause: invariant.RootCauseTypePollersStatus,
		Metadata:  polllersMetadataInBytes,
	}, nil

}

func taskListKind(kind types.TaskListKind) *shared.TaskListKind {
	if kind.String() == shared.TaskListKindNormal.String() {
		return shared.TaskListKindNormal.Ptr()
	}

	return shared.TaskListKindSticky.Ptr()
}

func checkHeartbeatStatus(issue invariant.InvariantCheckResult) ([]invariant.InvariantRootCauseResult, error) {
	var metadata TimeoutIssuesMetadata
	err := json.Unmarshal(issue.Metadata, &metadata)
	if err != nil {
		return nil, err
	}
	if metadata.ActivityTimeout == nil {
		return nil, fmt.Errorf("activity timeout metadata not set")
	}
	act := metadata.ActivityTimeout

	heartbeatingMetadataInBytes := invariant.MarshalData(HeartbeatingMetadata{TimeElapsed: act.TimeElapsed, RetryPolicy: act.RetryPolicy})

	if act.HeartBeatTimeout == 0 && activityStarted(*act) {
		if act.RetryPolicy != nil {
			return []invariant.InvariantRootCauseResult{
				{
					IssueID:   issue.IssueID,
					RootCause: invariant.RootCauseTypeHeartBeatingNotEnabledWithRetryPolicy,
					Metadata:  heartbeatingMetadataInBytes,
				},
			}, nil
		}
		return []invariant.InvariantRootCauseResult{
			{
				IssueID:   issue.IssueID,
				RootCause: invariant.RootCauseTypeNoHeartBeatTimeoutNoRetryPolicy,
				Metadata:  heartbeatingMetadataInBytes,
			},
		}, nil
	}

	if act.HeartBeatTimeout > 0 && act.TimeoutType.String() == types.TimeoutTypeHeartbeat.String() {
		if act.RetryPolicy == nil {
			return []invariant.InvariantRootCauseResult{
				{
					IssueID:   issue.IssueID,
					RootCause: invariant.RootCauseTypeHeartBeatingEnabledWithoutRetryPolicy,
					Metadata:  heartbeatingMetadataInBytes,
				},
			}, nil
		}
		return []invariant.InvariantRootCauseResult{
			{
				IssueID:   issue.IssueID,
				RootCause: invariant.RootCauseTypeHeartBeatingEnabledMissingHeartbeat,
				Metadata:  heartbeatingMetadataInBytes,
			},
		}, nil
	}

	return nil, nil
}

func activityStarted(metadata ActivityTimeoutMetadata) bool {
	return metadata.TimeoutType.String() != types.TimeoutTypeScheduleToStart.String()
}
