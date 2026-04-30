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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/metrics"
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
	MetricsClient  metrics.Client
}

// processScheduleFireActivity is the single activity that handles a schedule fire.
// It encapsulates all side effects, checking if the previous workflow is running,
// enforcing the overlap policy (cancel/terminate), and starting the new workflow.
// Keeping all of this in one activity means the workflow history records a single
// activity call per fire, so the internal logic can evolve freely without
// introducing nondeterminism.
func processScheduleFireActivity(ctx context.Context, req ProcessFireRequest) (result *ProcessFireResult, err error) {
	sc, ok := ctx.Value(schedulerContextKey).(schedulerContext)
	if !ok {
		return nil, fmt.Errorf("scheduler context not found in activity context")
	}

	scope := sc.MetricsClient.Scope(metrics.SchedulerActivityScope, metrics.DomainTag(req.Domain))
	defer func() {
		if err != nil {
			scope.IncCounter(metrics.SchedulerFireErrorCountPerDomain)
		}
	}()

	result = &ProcessFireResult{}

	policy := req.OverlapPolicy
	if policy == types.ScheduleOverlapPolicyInvalid {
		policy = types.ScheduleOverlapPolicySkipNew
	}

	// Bounded CONCURRENT: describe each tracked in-flight workflow, prune
	// completed entries, and enforce the cap. When under the cap, falls through
	// to the shared start block; stillRunning is used there to build
	// result.ActiveWorkflows. When at or over the cap, returns early with a skip.
	isBoundedConcurrent := policy == types.ScheduleOverlapPolicyConcurrent && req.ConcurrencyLimit > 0
	var stillRunning []RunningWorkflowInfo
	if isBoundedConcurrent {
		effectiveLimit := effectiveConcurrencyLimit(req.ConcurrencyLimit)
		for _, wf := range req.RunningWorkflows {
			running, err := isWorkflowRunning(ctx, sc.FrontendClient, req.Domain, &wf)
			if err != nil {
				return nil, err
			}
			if running {
				stillRunning = append(stillRunning, wf)
			}
		}
		if len(stillRunning) >= int(effectiveLimit) {
			scope.Tagged(metrics.OverlapPolicyTag(policy.String()), metrics.TriggerSourceTag(string(req.TriggerSource))).
				IncCounter(metrics.SchedulerFireSkippedCountPerDomain)
			result.SkippedDelta = 1
			result.ActiveWorkflows = stillRunning
			return result, nil
		}
	}

	if policy != types.ScheduleOverlapPolicyConcurrent && req.LastStartedWorkflow != nil {
		running, err := isWorkflowRunning(ctx, sc.FrontendClient, req.Domain, req.LastStartedWorkflow)
		if err != nil {
			return nil, err
		}
		if running {
			switch policy {
			case types.ScheduleOverlapPolicySkipNew:
				scope.Tagged(metrics.OverlapPolicyTag(policy.String()), metrics.TriggerSourceTag(string(req.TriggerSource))).IncCounter(metrics.SchedulerFireSkippedCountPerDomain)
				result.SkippedDelta = 1
				result.StartedWorkflow = req.LastStartedWorkflow
				return result, nil
			case types.ScheduleOverlapPolicyBuffer:
				// Defer the fire; the workflow enqueues it in state.BufferedFires.
				scope.Tagged(metrics.OverlapPolicyTag(policy.String()), metrics.TriggerSourceTag(string(req.TriggerSource))).IncCounter(metrics.SchedulerFireBufferedCountPerDomain)
				result.Buffered = true
				result.StartedWorkflow = req.LastStartedWorkflow
				return result, nil
			case types.ScheduleOverlapPolicyCancelPrevious:
				var cancelled bool
				if cancelled, err = cancelWorkflow(ctx, sc.FrontendClient, req.Domain, req.LastStartedWorkflow); err != nil {
					return nil, err
				}
				if cancelled {
					scope.IncCounter(metrics.SchedulerOverlapCancelCountPerDomain)
				}
			case types.ScheduleOverlapPolicyTerminatePrevious:
				var terminated bool
				if terminated, err = terminateWorkflow(ctx, sc.FrontendClient, req.Domain, req.LastStartedWorkflow); err != nil {
					return nil, err
				}
				if terminated {
					scope.IncCounter(metrics.SchedulerOverlapTerminateCountPerDomain)
				}
			}
		}
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
		SearchAttributes:                    buildSearchAttributes(req),
	}

	resp, err := sc.FrontendClient.StartWorkflowExecution(ctx, startReq)
	if err != nil {
		var alreadyStarted *types.WorkflowExecutionAlreadyStartedError
		if errors.As(err, &alreadyStarted) {
			scope.Tagged(metrics.TriggerSourceTag(string(req.TriggerSource))).IncCounter(metrics.SchedulerFireAlreadyRunningCountPerDomain)
			existing := &RunningWorkflowInfo{
				WorkflowID: workflowID,
				RunID:      alreadyStarted.RunID,
			}
			result.SkippedDelta = 1
			result.StartedWorkflow = existing
			if isBoundedConcurrent {
				result.ActiveWorkflows = append(stillRunning, *existing)
			}
			return result, nil
		}
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	startedScope := scope.Tagged(metrics.TriggerSourceTag(string(req.TriggerSource)))
	startedScope.IncCounter(metrics.SchedulerFireStartedCountPerDomain)
	if req.TriggerSource == TriggerSourceSchedule {
		startedScope.ExponentialHistogram(metrics.SchedulerFireLatencyPerDomainHistogram, time.Since(req.ScheduledTime))
	}
	result.TotalDelta = 1
	result.StartedWorkflow = &RunningWorkflowInfo{
		WorkflowID: workflowID,
		RunID:      resp.GetRunID(),
	}
	if isBoundedConcurrent {
		result.ActiveWorkflows = append(stillRunning, *result.StartedWorkflow)
	}
	return result, nil
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

func isWorkflowRunning(ctx context.Context, client frontend.Client, domain string, wf *RunningWorkflowInfo) (bool, error) {
	resp, err := client.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
		Domain: domain,
		Execution: &types.WorkflowExecution{
			WorkflowID: wf.WorkflowID,
			RunID:      wf.RunID,
		},
	})
	if err != nil {
		if isEntityNotExistsError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to describe workflow: %w", err)
	}

	running := resp.WorkflowExecutionInfo != nil &&
		resp.WorkflowExecutionInfo.CloseStatus == nil
	return running, nil
}

// Cancel is cooperative: the previous workflow receives a cancellation signal
// but may continue running while it handles cleanup. A brief overlap with the
// new run is expected. Use TERMINATE_PREVIOUS for a hard guarantee of no
// concurrent execution.
func cancelWorkflow(ctx context.Context, client frontend.Client, domain string, wf *RunningWorkflowInfo) (bool, error) {
	err := client.RequestCancelWorkflowExecution(ctx, &types.RequestCancelWorkflowExecutionRequest{
		Domain: domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: wf.WorkflowID,
			RunID:      wf.RunID,
		},
		Cause: "schedule overlap policy: CANCEL_PREVIOUS",
	})
	if err != nil {
		if isEntityNotExistsError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to cancel workflow: %w", err)
	}
	return true, nil
}

func terminateWorkflow(ctx context.Context, client frontend.Client, domain string, wf *RunningWorkflowInfo) (bool, error) {
	err := client.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain: domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: wf.WorkflowID,
			RunID:      wf.RunID,
		},
		Reason: "schedule overlap policy: TERMINATE_PREVIOUS",
	})
	if err != nil {
		if isEntityNotExistsError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to terminate workflow: %w", err)
	}
	return true, nil
}

func buildSearchAttributes(req ProcessFireRequest) *types.SearchAttributes {
	fields := make(map[string][]byte)

	// Preserve any user-provided search attributes from the action config
	if req.Action.SearchAttributes != nil {
		for k, v := range req.Action.SearchAttributes.IndexedFields {
			fields[k] = v
		}
	}

	// Add scheduler-managed attributes (overwrite user values if keys conflict)
	if v, err := json.Marshal(req.ScheduleID); err == nil {
		fields[SearchAttrScheduleID] = v
	}
	if v, err := json.Marshal(req.ScheduledTime); err == nil {
		fields[SearchAttrScheduleTime] = v
	}
	if v, err := json.Marshal(req.TriggerSource == TriggerSourceBackfill); err == nil {
		fields[SearchAttrIsBackfill] = v
	}

	return &types.SearchAttributes{IndexedFields: fields}
}

func isEntityNotExistsError(err error) bool {
	var notExists *types.EntityNotExistsError
	return errors.As(err, &notExists)
}
