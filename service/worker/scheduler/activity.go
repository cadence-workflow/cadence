// Copyright (c) 2026 Uber Technologies, Inc.
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

package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
)

// schedulerRequestIDNamespace is a stable UUID namespace used to derive
// deterministic RequestIDs. Cassandra's schema stores create_request_id as
// a uuid column, so plain strings are rejected by the gocql driver.
var schedulerRequestIDNamespace = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("cadence.scheduler"))

type contextKey string

const schedulerContextKey contextKey = "schedulerContext"

// schedulerContext is the context passed to activities via BackgroundActivityContext.
type schedulerContext struct {
	FrontendClient frontend.Client
}

// startWorkflowActivity starts a workflow execution as specified by the schedule's action.
// It generates a deterministic workflow ID from the schedule ID and scheduled time
// to ensure idempotent execution.
func startWorkflowActivity(ctx context.Context, req StartWorkflowRequest) (*StartWorkflowResult, error) {
	sc, ok := ctx.Value(schedulerContextKey).(schedulerContext)
	if !ok {
		return nil, fmt.Errorf("scheduler context not found in activity context")
	}

	workflowID := generateWorkflowID(req.Action.WorkflowIDPrefix, req.ScheduleID, req.ScheduledTime)

	reusePolicy := types.WorkflowIDReusePolicyAllowDuplicate
	startReq := &types.StartWorkflowExecutionRequest{
		Domain:                              req.Domain,
		WorkflowID:                          workflowID,
		WorkflowType:                        req.Action.WorkflowType,
		TaskList:                            req.Action.TaskList,
		Input:                               req.Action.Input,
		ExecutionStartToCloseTimeoutSeconds: req.Action.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      req.Action.TaskStartToCloseTimeoutSeconds,
		RequestID:                           generateRequestID(req.ScheduleID, req.ScheduledTime.UnixNano(), req.TriggerSource),
		WorkflowIDReusePolicy:               &reusePolicy,
		RetryPolicy:                         req.Action.RetryPolicy,
		Memo:                                req.Action.Memo,
		SearchAttributes:                    req.Action.SearchAttributes,
	}

	resp, err := sc.FrontendClient.StartWorkflowExecution(ctx, startReq)
	if err != nil {
		if isAlreadyStartedError(err) {
			return &StartWorkflowResult{
				WorkflowID: workflowID,
				Skipped:    true,
			}, nil
		}
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	return &StartWorkflowResult{
		WorkflowID: workflowID,
		RunID:      resp.GetRunID(),
		Started:    true,
	}, nil
}

// generateWorkflowID creates a deterministic workflow ID from the
// schedule's prefix (or schedule ID) and the scheduled time.
// Example: "my-prefix-2026-01-15T10:00:00Z"
func generateWorkflowID(prefix, scheduleID string, scheduledTime time.Time) string {
	if prefix == "" {
		prefix = scheduleID
	}
	return fmt.Sprintf("%s-%s", prefix, scheduledTime.UTC().Format(time.RFC3339))
}

// generateRequestID produces a deterministic UUID from the schedule ID,
// scheduled time, and trigger source. Including the trigger source ensures
// that a backfill for the same timestamp as a normal schedule fire produces
// a distinct RequestID, avoiding unintended server-side deduplication.
func generateRequestID(scheduleID string, scheduledTimeNanos int64, source TriggerSource) string {
	name := fmt.Sprintf("%s-%d-%s", scheduleID, scheduledTimeNanos, source)
	return uuid.NewSHA1(schedulerRequestIDNamespace, []byte(name)).String()
}

func isAlreadyStartedError(err error) bool {
	_, ok := err.(*types.WorkflowExecutionAlreadyStartedError)
	return ok
}
