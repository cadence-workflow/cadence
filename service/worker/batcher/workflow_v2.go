// Copyright (c) 2017 Uber Technologies, Inc.
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

package batcher

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

const (
	// BatchWFV2TypeName is the workflow type for the V2 batch workflow with signal-based tuning.
	BatchWFV2TypeName = "cadence-sys-batch-workflow-v2"

	batchActivityV2Name = "cadence-sys-batch-activity-v2"

	// SignalNameTune is the signal name for tuning workflow parameters at runtime.
	SignalNameTune = "cadence-sys-batch-tune-signal"
)

// TuneSignal is the payload for the tune signal.
// Zero values are ignored (no change to the corresponding parameter).
type TuneSignal struct {
	// RPS overrides the current RPS. Zero means no change.
	RPS int
	// Concurrency overrides the current concurrency. Zero means no change.
	Concurrency int
}

// batchParamsV2 extends BatchParams with V2-specific fields.
// It is used only by the V2 activity and is not part of the public API.
type batchParamsV2 struct {
	BatchParams

	// Progress carries forward HeartBeatDetails from a cancelled activity
	// so the next activity invocation can resume where the previous left off.
	// nil means no prior progress (use heartbeat recovery or start fresh).
	Progress *HeartBeatDetails
}

func init() {
	workflow.RegisterWithOptions(BatchWorkflowV2, workflow.RegisterOptions{Name: BatchWFV2TypeName})
	activity.RegisterWithOptions(batchActivityV2, activity.RegisterOptions{Name: batchActivityV2Name})
}

// BatchWorkflowV2 is a batch workflow that supports runtime tuning via signals.
// It launches a single long-running activity that iterates over all pages.
// Tune signals (SignalNameTune) cancel the running activity and restart it
// with updated RPS/Concurrency; progress is preserved via heartbeat details.
func BatchWorkflowV2(ctx workflow.Context, batchParams BatchParams) (HeartBeatDetails, error) {
	batchParams = setDefaultParams(batchParams)
	if err := validateParams(batchParams); err != nil {
		return HeartBeatDetails{}, err
	}

	tuneCh := workflow.GetSignalChannel(ctx, SignalNameTune)
	v2Params := batchParamsV2{BatchParams: batchParams}

	for {
		retryPolicy := BatchActivityRetryPolicy
		retryPolicy.MaximumAttempts = int32(v2Params.MaxActivityRetries)
		actOpts := workflow.ActivityOptions{
			ScheduleToStartTimeout: 5 * time.Minute,
			StartToCloseTimeout:    InfiniteDuration,
			HeartbeatTimeout:       v2Params.ActivityHeartBeatTimeout,
			RetryPolicy:            &retryPolicy,
			WaitForCancellation:    true,
		}
		actCtx, cancel := workflow.WithCancel(workflow.WithActivityOptions(ctx, actOpts))

		var result HeartBeatDetails
		future := workflow.ExecuteActivity(actCtx, batchActivityV2Name, v2Params)

		selector := workflow.NewSelector(ctx)
		var actErr error
		activityDone := false

		selector.AddFuture(future, func(f workflow.Future) {
			actErr = f.Get(ctx, &result)
			activityDone = true
		})

		selector.AddReceive(tuneCh, func(ch workflow.Channel, more bool) {
			var sig TuneSignal
			ch.Receive(ctx, &sig)
			if sig.RPS > 0 {
				v2Params.RPS = sig.RPS
			}
			if sig.Concurrency > 0 {
				v2Params.Concurrency = sig.Concurrency
			}
			// Drain any additional pending signals.
			for ch.ReceiveAsync(&sig) {
				if sig.RPS > 0 {
					v2Params.RPS = sig.RPS
				}
				if sig.Concurrency > 0 {
					v2Params.Concurrency = sig.Concurrency
				}
			}
			// Cancel the running activity so it returns with current progress.
			cancel()
		})

		selector.Select(ctx)

		if activityDone {
			return result, actErr
		}

		// Activity was cancelled by tune signal — wait for it to finish
		// and extract progress from the CanceledError.
		err := future.Get(ctx, nil)
		cancel() // release resources

		if err != nil {
			if cadence.IsCanceledError(err) {
				if ce, ok := err.(*cadence.CanceledError); ok {
					var hbd HeartBeatDetails
					if ce.Details(&hbd) == nil {
						v2Params.Progress = &hbd
					}
				}
			} else {
				// Non-cancellation error (e.g. transient RPC failure) — surface it
				// rather than silently retrying a potentially broken activity.
				return HeartBeatDetails{}, err
			}
		}
	}
}

// batchActivityV2 is the V2 activity for processing batch operations.
// Compared to V1 (BatchActivity), it:
//   - Accepts progress from a prior cancelled activity via batchParamsV2.Progress
//   - Uses context.WithoutCancel so in-flight operations complete on cancellation
//   - Returns current HeartBeatDetails on scan errors for resumability
//   - Returns CanceledError with progress when context is cancelled
func batchActivityV2(ctx context.Context, params batchParamsV2) (HeartBeatDetails, error) {
	batcher := ctx.Value(BatcherContextKey).(*Batcher)
	client := batcher.clientBean.GetFrontendClient()
	var adminClient admin.Client
	if params.BatchType == BatchTypeReplicate {
		currentCluster := batcher.cfg.ClusterMetadata.GetCurrentClusterName()
		if currentCluster != params.ReplicateParams.SourceCluster {
			return HeartBeatDetails{}, cadence.NewCustomError(_nonRetriableReason, fmt.Sprintf("the activity must run in the source cluster, current cluster is %s", currentCluster))
		}
		var err error
		adminClient, err = batcher.clientBean.GetRemoteAdminClient(params.ReplicateParams.TargetCluster)
		if err != nil {
			return HeartBeatDetails{}, cadence.NewCustomError(_nonRetriableReason, err.Error())
		}
	}

	domainResp, err := client.DescribeDomain(ctx, &types.DescribeDomainRequest{
		Name: &params.DomainName,
	})
	if err != nil {
		return HeartBeatDetails{}, err
	}
	domainID := domainResp.GetDomainInfo().GetUUID()

	hbd := getHeartBeatDetailsV2(ctx)
	// Workflow-provided progress (from a prior cancelled activity) takes precedence
	// over heartbeat recovery, which only works within the same activity ID.
	if params.Progress != nil {
		hbd = *params.Progress
	}

	if hbd.TotalEstimate == 0 {
		resp, err := client.CountWorkflowExecutions(ctx, &types.CountWorkflowExecutionsRequest{
			Domain: params.DomainName,
			Query:  params.Query,
		})
		if err != nil {
			return HeartBeatDetails{}, err
		}
		hbd.TotalEstimate = resp.GetCount()
	}

	rateLimiter := rate.NewLimiter(rate.Limit(params.RPS), params.RPS)
	taskCh := make(chan taskDetail, params.PageSize)
	respCh := make(chan error, params.PageSize)
	for i := 0; i < params.Concurrency; i++ {
		go startTaskProcessorV2(ctx, params.BatchParams, domainID, taskCh, respCh, rateLimiter, client, adminClient)
	}

	for {
		resp, err := client.ScanWorkflowExecutions(ctx, &types.ListWorkflowExecutionsRequest{
			Domain:        params.DomainName,
			PageSize:      int32(params.PageSize),
			NextPageToken: hbd.PageToken,
			Query:         params.Query,
		})
		if err != nil {
			return hbd, err
		}
		batchCount := len(resp.Executions)

		if batchCount > 0 {
			for _, wf := range resp.Executions {
				taskCh <- taskDetail{
					execution: *wf.Execution,
					attempts:  0,
					hbd:       hbd,
				}
			}

			succCount := 0
			errCount := 0
		Loop:
			for {
				select {
				case err := <-respCh:
					if err == nil {
						succCount++
					} else {
						errCount++
					}
					if succCount+errCount == batchCount {
						break Loop
					}
				case <-ctx.Done():
					return hbd, cadence.NewCanceledError(hbd)
				}
			}
			hbd.SuccessCount += succCount
			hbd.ErrorCount += errCount
			hbd.CurrentPage++
		}
		hbd.PageToken = resp.NextPageToken
		activity.RecordHeartbeat(ctx, hbd)

		if ctx.Err() != nil {
			return hbd, cadence.NewCanceledError(hbd)
		}

		if len(hbd.PageToken) == 0 {
			return hbd, nil
		}
	}
}

// getHeartBeatDetailsV2 recovers progress from a previous heartbeat, if any.
func getHeartBeatDetailsV2(ctx context.Context) HeartBeatDetails {
	var hbd HeartBeatDetails
	if activity.HasHeartbeatDetails(ctx) {
		if err := activity.GetHeartbeatDetails(ctx, &hbd); err == nil {
			return hbd
		}
	}
	return hbd
}

func startTaskProcessorV2(
	ctx context.Context,
	batchParams BatchParams,
	domainID string,
	taskCh chan taskDetail,
	respCh chan error,
	limiter *rate.Limiter,
	client frontend.Client,
	adminClient admin.Client,
) {
	batcher := ctx.Value(BatcherContextKey).(*Batcher)
	// opCtx inherits all context values (batcher, activity heartbeat machinery)
	// but is NOT cancelled when the activity context is cancelled. This allows
	// in-flight operations to complete when a tune signal cancels the activity,
	// so no workflows are skipped mid-page.
	opCtx := context.WithoutCancel(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-taskCh:
			var err error
			requestID := uuid.New().String()

			switch batchParams.BatchType {
			case BatchTypeTerminate:
				err = processTask(opCtx, limiter, task, batchParams, client,
					batchParams.TerminateParams.TerminateChildren,
					func(workflowID, runID string) error {
						return client.TerminateWorkflowExecution(opCtx, &types.TerminateWorkflowExecutionRequest{
							Domain: batchParams.DomainName,
							WorkflowExecution: &types.WorkflowExecution{
								WorkflowID: workflowID,
								RunID:      runID,
							},
							Reason:   batchParams.Reason,
							Identity: BatchWFV2TypeName,
						})
					})
			case BatchTypeCancel:
				err = processTask(opCtx, limiter, task, batchParams, client,
					batchParams.CancelParams.CancelChildren,
					func(workflowID, runID string) error {
						return client.RequestCancelWorkflowExecution(opCtx, &types.RequestCancelWorkflowExecutionRequest{
							Domain: batchParams.DomainName,
							WorkflowExecution: &types.WorkflowExecution{
								WorkflowID: workflowID,
								RunID:      runID,
							},
							Identity:  BatchWFV2TypeName,
							RequestID: requestID,
						})
					})
			case BatchTypeSignal:
				err = processTask(opCtx, limiter, task, batchParams, client, common.BoolPtr(false),
					func(workflowID, runID string) error {
						return client.SignalWorkflowExecution(opCtx, &types.SignalWorkflowExecutionRequest{
							Domain: batchParams.DomainName,
							WorkflowExecution: &types.WorkflowExecution{
								WorkflowID: workflowID,
								RunID:      runID,
							},
							Identity:   BatchWFV2TypeName,
							RequestID:  requestID,
							SignalName: batchParams.SignalParams.SignalName,
							Input:      []byte(batchParams.SignalParams.Input),
						})
					})
			case BatchTypeReplicate:
				err = processTask(opCtx, limiter, task, batchParams, client, common.BoolPtr(false),
					func(workflowID, runID string) error {
						return adminClient.ResendReplicationTasks(opCtx, &types.ResendReplicationTasksRequest{
							DomainID:      domainID,
							WorkflowID:    workflowID,
							RunID:         runID,
							RemoteCluster: batchParams.ReplicateParams.SourceCluster,
						})
					})
			}
			if err != nil {
				batcher.metricsClient.IncCounter(metrics.BatcherScope, metrics.BatcherProcessorFailures)
				getActivityLogger(ctx).Error("Failed to process batch operation task", tag.Error(err))

				_, ok := batchParams._nonRetryableErrors[err.Error()]
				if ok || task.attempts >= batchParams.AttemptsOnRetryableError {
					respCh <- err
				} else {
					// put back to the channel if less than attemptsOnError
					task.attempts++
					taskCh <- task
				}
			} else {
				batcher.metricsClient.IncCounter(metrics.BatcherScope, metrics.BatcherProcessorSuccess)
				respCh <- nil
			}
		}
	}
}
