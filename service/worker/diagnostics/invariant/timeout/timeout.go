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

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

type Timeout invariant.Invariant

type timeout struct {
	workflowExecutionHistory *types.GetWorkflowExecutionHistoryResponse
	domain                   string
	clientBean               client.Bean
}

type NewTimeoutParams struct {
	WorkflowExecutionHistory *types.GetWorkflowExecutionHistoryResponse
	Domain                   string
	ClientBean               client.Bean
}

func NewInvariant(p NewTimeoutParams) invariant.Invariant {
	return &timeout{
		workflowExecutionHistory: p.WorkflowExecutionHistory,
		domain:                   p.Domain,
		clientBean:               p.ClientBean,
	}
}

func (t *timeout) Check(context.Context) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)
	events := t.workflowExecutionHistory.GetHistory().GetEvents()
	for _, event := range events {
		if event.WorkflowExecutionTimedOutEventAttributes != nil {
			timeoutLimit := getWorkflowExecutionConfiguredTimeout(events)
			data := ExecutionTimeoutMetadata{
				ExecutionTime:     getExecutionTime(1, event.ID, events),
				ConfiguredTimeout: time.Duration(timeoutLimit) * time.Second,
				LastOngoingEvent:  events[len(events)-2],
				Tasklist:          getWorkflowExecutionTasklist(events),
			}
			result = append(result, invariant.InvariantCheckResult{
				InvariantType: TimeoutTypeExecution.String(),
				Reason:        event.GetWorkflowExecutionTimedOutEventAttributes().GetTimeoutType().String(),
				Metadata:      invariant.MarshalData(data),
			})
		}
		if event.ActivityTaskTimedOutEventAttributes != nil {
			metadata, err := getActivityTaskMetadata(event, events)
			if err != nil {
				return nil, err
			}
			result = append(result, invariant.InvariantCheckResult{
				InvariantType: TimeoutTypeActivity.String(),
				Reason:        event.GetActivityTaskTimedOutEventAttributes().GetTimeoutType().String(),
				Metadata:      invariant.MarshalData(metadata),
			})
		}
		if event.DecisionTaskTimedOutEventAttributes != nil {
			reason, metadata := reasonForDecisionTaskTimeouts(event, events)
			result = append(result, invariant.InvariantCheckResult{
				InvariantType: TimeoutTypeDecision.String(),
				Reason:        reason,
				Metadata:      invariant.MarshalData(metadata),
			})
		}
		if event.ChildWorkflowExecutionTimedOutEventAttributes != nil {
			timeoutLimit := getChildWorkflowExecutionConfiguredTimeout(event, events)
			data := ChildWfTimeoutMetadata{
				ExecutionTime:     getExecutionTime(event.GetChildWorkflowExecutionTimedOutEventAttributes().StartedEventID, event.ID, events),
				ConfiguredTimeout: time.Duration(timeoutLimit) * time.Second,
				Execution:         event.GetChildWorkflowExecutionTimedOutEventAttributes().WorkflowExecution,
			}
			result = append(result, invariant.InvariantCheckResult{
				InvariantType: TimeoutTypeChildWorkflow.String(),
				Reason:        event.GetChildWorkflowExecutionTimedOutEventAttributes().TimeoutType.String(),
				Metadata:      invariant.MarshalData(data),
			})
		}
	}
	return result, nil
}

func (t *timeout) RootCause(ctx context.Context, issues []invariant.InvariantCheckResult) ([]invariant.InvariantRootCauseResult, error) {
	result := make([]invariant.InvariantRootCauseResult, 0)
	for _, issue := range issues {
		if issue.InvariantType == TimeoutTypeActivity.String() || issue.InvariantType == TimeoutTypeExecution.String() {
			pollerStatus, err := t.checkTasklist(ctx, issue)
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

func (t *timeout) checkTasklist(ctx context.Context, issue invariant.InvariantCheckResult) (invariant.InvariantRootCauseResult, error) {
	var taskList *types.TaskList
	var tasklistType *types.TaskListType
	switch issue.InvariantType {
	case TimeoutTypeExecution.String():
		var metadata ExecutionTimeoutMetadata
		err := json.Unmarshal(issue.Metadata, &metadata)
		if err != nil {
			return invariant.InvariantRootCauseResult{}, err
		}
		taskList = metadata.Tasklist
		tasklistType = types.TaskListTypeDecision.Ptr()
	case TimeoutTypeActivity.String():
		var metadata ActivityTimeoutMetadata
		err := json.Unmarshal(issue.Metadata, &metadata)
		if err != nil {
			return invariant.InvariantRootCauseResult{}, err
		}
		taskList = metadata.Tasklist
		tasklistType = types.TaskListTypeActivity.Ptr()
	}
	if taskList == nil {
		return invariant.InvariantRootCauseResult{}, fmt.Errorf("tasklist not set")
	}

	frontendClient := t.clientBean.GetFrontendClient()
	resp, err := frontendClient.DescribeTaskList(ctx, &types.DescribeTaskListRequest{
		Domain:       t.domain,
		TaskList:     taskList,
		TaskListType: tasklistType,
	})
	if err != nil {
		return invariant.InvariantRootCauseResult{}, err
	}

	tasklistBacklog := resp.GetTaskListStatus().GetBacklogCountHint()
	polllersMetadataInBytes := invariant.MarshalData(PollersMetadata{TaskListBacklog: tasklistBacklog})
	if len(resp.GetPollers()) == 0 {
		return invariant.InvariantRootCauseResult{
			RootCause: invariant.RootCauseTypeMissingPollers,
			Metadata:  polllersMetadataInBytes,
		}, nil
	}
	return invariant.InvariantRootCauseResult{
		RootCause: invariant.RootCauseTypePollersStatus,
		Metadata:  polllersMetadataInBytes,
	}, nil

}

func checkHeartbeatStatus(issue invariant.InvariantCheckResult) ([]invariant.InvariantRootCauseResult, error) {
	var metadata ActivityTimeoutMetadata
	err := json.Unmarshal(issue.Metadata, &metadata)
	if err != nil {
		return nil, err
	}

	heartbeatingMetadataInBytes := invariant.MarshalData(HeartbeatingMetadata{TimeElapsed: metadata.TimeElapsed, RetryPolicy: metadata.RetryPolicy})

	if metadata.HeartBeatTimeout == 0 && activityStarted(metadata) {
		if metadata.RetryPolicy != nil {
			return []invariant.InvariantRootCauseResult{
				{
					RootCause: invariant.RootCauseTypeHeartBeatingNotEnabledWithRetryPolicy,
					Metadata:  heartbeatingMetadataInBytes,
				},
			}, nil
		}
		return []invariant.InvariantRootCauseResult{
			{
				RootCause: invariant.RootCauseTypeNoHeartBeatTimeoutNoRetryPolicy,
				Metadata:  heartbeatingMetadataInBytes,
			},
		}, nil
	}

	if metadata.HeartBeatTimeout > 0 && metadata.TimeoutType.String() == types.TimeoutTypeHeartbeat.String() {
		if metadata.RetryPolicy == nil {
			return []invariant.InvariantRootCauseResult{
				{
					RootCause: invariant.RootCauseTypeHeartBeatingEnabledWithoutRetryPolicy,
					Metadata:  heartbeatingMetadataInBytes,
				},
			}, nil
		}
		return []invariant.InvariantRootCauseResult{
			{
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
