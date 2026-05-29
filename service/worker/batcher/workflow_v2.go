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

	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/client/admin"
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
	params := batchParams

	for {
		retryPolicy := BatchActivityRetryPolicy
		retryPolicy.MaximumAttempts = int32(params.MaxActivityRetries)
		actOpts := workflow.ActivityOptions{
			ScheduleToStartTimeout: 5 * time.Minute,
			StartToCloseTimeout:    InfiniteDuration,
			HeartbeatTimeout:       params.ActivityHeartBeatTimeout,
			RetryPolicy:            &retryPolicy,
			WaitForCancellation:    true,
		}
		actCtx, cancel := workflow.WithCancel(workflow.WithActivityOptions(ctx, actOpts))

		var result HeartBeatDetails
		future := workflow.ExecuteActivity(actCtx, batchActivityV2Name, params)

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
				params.RPS = sig.RPS
			}
			if sig.Concurrency > 0 {
				params.Concurrency = sig.Concurrency
			}
		})

		selector.Select(ctx)

		if activityDone {
			cancel()
			return result, actErr
		}

		// Tune signal fired — cancel the activity and wait for it to finish.
		cancel()
		err := future.Get(ctx, &result)
		if err == nil {
			// Activity finished before the cancellation was delivered — return its result directly.
			return result, nil
		}
		if cadence.IsCanceledError(err) {
			if ce, ok := err.(*cadence.CanceledError); ok {
				var hbd HeartBeatDetails
				if ce.Details(&hbd) == nil {
					params.Progress = &hbd
				}
			}
		} else if err != nil {
			// Non-cancellation error (e.g. transient RPC failure) — surface it
			// rather than silently retrying a potentially broken activity.
			return HeartBeatDetails{}, err
		}
	}
}

// batchActivityV2 is the V2 activity for processing batch operations.
// Compared to V1 (BatchActivity), it:
//   - Accepts progress from a prior cancelled activity via batchParamsV2.Progress
//   - Returns current HeartBeatDetails on scan errors for resumability
//   - Returns CanceledError with progress when context is cancelled
func batchActivityV2(ctx context.Context, params BatchParams) (HeartBeatDetails, error) {
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

	hbd, ok := getHeartBeatDetails(ctx)
	// Only fall back to workflow-provided progress when heartbeat
	// recovery has no data (first attempt of a re-launched activity).
	if !ok && params.Progress != nil {
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
		go startTaskProcessor(ctx, params, domainID, taskCh, respCh, rateLimiter, client, adminClient, BatchWFV2TypeName)
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
		if batchCount <= 0 {
			break
		}

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
		hbd.PageToken = resp.NextPageToken
		activity.RecordHeartbeat(ctx, hbd)

		if ctx.Err() != nil {
			return hbd, cadence.NewCanceledError(hbd)
		}

		if len(hbd.PageToken) == 0 {
			return hbd, nil
		}
	}

	return hbd, nil
}
