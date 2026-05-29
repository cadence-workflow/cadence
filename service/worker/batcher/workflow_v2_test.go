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

package batcher

import (
	"context"
	"sync"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/cadence"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/metrics"
	mmocks "github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/types"
)

func TestBatchWorkflowV2(t *testing.T) {
	tests := []struct {
		name        string
		params      BatchParams
		setupEnv    func(env *testsuite.TestWorkflowEnvironment)
		wantErr     string
		checkResult func(t *testing.T, result HeartBeatDetails)
	}{
		{
			name:   "normal completion",
			params: createParams(BatchTypeCancel),
			setupEnv: func(env *testsuite.TestWorkflowEnvironment) {
				env.OnActivity(batchActivityV2Name, mock.Anything, mock.Anything).
					Return(HeartBeatDetails{SuccessCount: 5, CurrentPage: 2}, nil)
			},
			checkResult: func(t *testing.T, result HeartBeatDetails) {
				assert.Equal(t, 5, result.SuccessCount)
				assert.Equal(t, 2, result.CurrentPage)
			},
		},
		{
			name:    "validation error on empty params",
			params:  BatchParams{},
			wantErr: "must provide required parameters",
		},
		{
			name: "validation error missing signal name",
			params: func() BatchParams {
				p := createParams(BatchTypeSignal)
				p.SignalParams.SignalName = ""
				return p
			}(),
			wantErr: "must provide signal name",
		},
		{
			name:   "activity returns non-retriable error",
			params: createParams(BatchTypeTerminate),
			setupEnv: func(env *testsuite.TestWorkflowEnvironment) {
				env.OnActivity(batchActivityV2Name, mock.Anything, mock.Anything).
					Return(HeartBeatDetails{}, cadence.NewCustomError(_nonRetriableReason, "details"))
			},
			wantErr: _nonRetriableReason,
		},
		{
			name:   "activity returns full result",
			params: createParams(BatchTypeTerminate),
			setupEnv: func(env *testsuite.TestWorkflowEnvironment) {
				env.OnActivity(batchActivityV2Name, mock.Anything, mock.Anything).
					Return(HeartBeatDetails{SuccessCount: 10, ErrorCount: 2, CurrentPage: 3, TotalEstimate: 12}, nil)
			},
			checkResult: func(t *testing.T, result HeartBeatDetails) {
				assert.Equal(t, 10, result.SuccessCount)
				assert.Equal(t, 2, result.ErrorCount)
				assert.Equal(t, 3, result.CurrentPage)
				assert.Equal(t, int64(12), result.TotalEstimate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterWorkflow(BatchWorkflowV2)

			if tt.setupEnv != nil {
				tt.setupEnv(env)
			}

			env.ExecuteWorkflow(BatchWorkflowV2, tt.params)
			require.True(t, env.IsWorkflowCompleted())

			err := env.GetWorkflowError()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)

			if tt.checkResult != nil {
				var result HeartBeatDetails
				require.NoError(t, env.GetWorkflowResult(&result))
				tt.checkResult(t, result)
			}
		})
	}
}

// TestBatchWorkflowV2_TuneSignal verifies the tune-signal restart path:
// a TuneSignal cancels the running activity and the workflow restarts it
// with the updated RPS/Concurrency from the signal.
//
// Note: the Cadence test framework cancels activities by invoking the callback
// directly with an empty CanceledError (no heartbeat details), so progress
// forwarding via CanceledError details cannot be exercised in a unit test.
// What can be verified is that the signal is received, the activity is
// restarted, and the updated params are applied.
func TestBatchWorkflowV2_TuneSignal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(BatchWorkflowV2)

	// Track params received by each activity invocation.
	var mu sync.Mutex
	var capturedParams []BatchParams

	// firstActivityStarted is signalled by the first activity mock when it is
	// running, so the tune signal can be sent reliably after the activity has
	// started but before it returns.
	firstActivityStarted := make(chan struct{}, 1)

	// firstActivityDone is closed by t.Cleanup after the workflow completes and
	// assertions pass, unblocking the goroutine for the first activity mock.
	// The Cadence test framework delivers CanceledError to the activity future
	// independently of the goroutine, so the blocked goroutine does not prevent
	// the workflow from completing.
	firstActivityDone := make(chan struct{})
	t.Cleanup(func() { close(firstActivityDone) })

	env.OnActivity(batchActivityV2Name, mock.Anything, mock.Anything).
		Return(func(_ context.Context, p BatchParams) (HeartBeatDetails, error) {
			mu.Lock()
			capturedParams = append(capturedParams, p)
			n := len(capturedParams)
			mu.Unlock()
			if n == 1 {
				// Notify that the first activity has started, then block until
				// the test is done. The workflow can still complete because the
				// Cadence test framework delivers CanceledError to the future
				// independently of this goroutine.
				firstActivityStarted <- struct{}{}
				<-firstActivityDone
				return HeartBeatDetails{}, nil
			}
			// Second invocation: completes normally.
			return HeartBeatDetails{SuccessCount: 8, CurrentPage: 3}, nil
		})

	// Send the tune signal from a separate goroutine as soon as the first
	// activity has confirmed it is running. This avoids relying on the test
	// framework's 0ms timer mechanism (which uses a wall-clock AfterFunc path
	// when runningCount > 0 and can occasionally miss the 3-second deadline).
	//
	// stopSig is closed by t.Cleanup (registered after firstActivityDone's cleanup,
	// so it runs first in LIFO order): the goroutine exits via the stopSig arm,
	// closes sigDone, then firstActivityDone is closed to unblock the activity mock.
	stopSig := make(chan struct{})
	sigDone := make(chan struct{})
	t.Cleanup(func() {
		close(stopSig) // unblock the goroutine if still waiting
		<-sigDone      // wait for it to exit before firstActivityDone is closed
	})
	go func() {
		defer close(sigDone)
		select {
		case <-firstActivityStarted:
			env.SignalWorkflow(SignalNameTune, TuneSignal{RPS: 20, Concurrency: 5})
		case <-stopSig:
			// workflow ended before first activity started — nothing to signal
		}
	}()

	params := createParams(BatchTypeCancel)
	env.ExecuteWorkflow(BatchWorkflowV2, params)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result HeartBeatDetails
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 8, result.SuccessCount)
	assert.Equal(t, 3, result.CurrentPage)

	mu.Lock()
	captured := make([]BatchParams, len(capturedParams))
	copy(captured, capturedParams)
	mu.Unlock()

	require.Len(t, captured, 2, "activity must be invoked twice (cancelled then restarted)")
	// Second invocation must carry the updated RPS and Concurrency from the signal.
	assert.Equal(t, 20, captured[1].RPS, "RPS must be updated by tune signal")
	assert.Equal(t, 5, captured[1].Concurrency, "Concurrency must be updated by tune signal")
}

func TestBatchActivityV2_UsesProgress(t *testing.T) {
	var env testsuite.WorkflowTestSuite
	activityEnv := env.NewTestActivityEnvironment()
	activityEnv.RegisterActivity(batchActivityV2)

	batcher, mockResource := setuptest(t)
	metricsMock := &mmocks.Client{}
	metricsMock.On("IncCounter", metrics.BatcherScope, metrics.BatcherProcessorSuccess).Times(1)
	batcher.metricsClient = metricsMock

	mockResource.FrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).
		Return(&types.DescribeDomainResponse{}, nil)
	mockResource.FrontendClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
		Return(&types.DescribeWorkflowExecutionResponse{}, nil).AnyTimes()

	// CountWorkflowExecutions should NOT be called because TotalEstimate > 0 via Progress.

	// Scan with the saved token returns 1 workflow and no next page.
	mockResource.FrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), &types.ListWorkflowExecutionsRequest{
		Domain:        "test-domain",
		PageSize:      int32(10),
		NextPageToken: []byte("saved-token"),
		Query:         "Closetime=missing",
	}).Return(&types.ListWorkflowExecutionsResponse{
		Executions:    []*types.WorkflowExecutionInfo{{Execution: &types.WorkflowExecution{WorkflowID: "wid3", RunID: "rid3"}}},
		NextPageToken: nil,
	}, nil)

	ctx := context.WithValue(context.Background(), BatcherContextKey, batcher)
	activityEnv.SetWorkerOptions(worker.Options{
		MetricsScope:              tally.TestScope(nil),
		BackgroundActivityContext: ctx,
		Tracer:                    opentracing.GlobalTracer(),
	})

	params := createParams(BatchTypeCancel)
	params.Progress = &HeartBeatDetails{
		PageToken:     []byte("saved-token"),
		CurrentPage:   1,
		SuccessCount:  2,
		TotalEstimate: 4,
	}

	val, err := activityEnv.ExecuteActivity(batchActivityV2, params)
	require.NoError(t, err)

	var result HeartBeatDetails
	require.NoError(t, val.Get(&result))
	assert.Equal(t, 3, result.SuccessCount, "should add new success to progress")
	assert.Equal(t, 2, result.CurrentPage, "should increment from progress page")
	assert.Equal(t, int64(4), result.TotalEstimate, "should preserve TotalEstimate from progress")
}

func TestBatchActivityV2_EmptyPageDoesNotIncrementCurrentPage(t *testing.T) {
	var env testsuite.WorkflowTestSuite
	activityEnv := env.NewTestActivityEnvironment()
	activityEnv.RegisterActivity(batchActivityV2)

	batcher, mockResource := setuptest(t)
	metricsMock := &mmocks.Client{}
	batcher.metricsClient = metricsMock

	mockResource.FrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).
		Return(&types.DescribeDomainResponse{}, nil)
	mockResource.FrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.CountWorkflowExecutionsResponse{Count: 0}, nil)

	// Scan returns no results.
	mockResource.FrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.ListWorkflowExecutionsResponse{
			Executions:    nil,
			NextPageToken: nil,
		}, nil)

	ctx := context.WithValue(context.Background(), BatcherContextKey, batcher)
	activityEnv.SetWorkerOptions(worker.Options{
		MetricsScope:              tally.TestScope(nil),
		BackgroundActivityContext: ctx,
		Tracer:                    opentracing.GlobalTracer(),
	})

	params := createParams(BatchTypeCancel)
	val, err := activityEnv.ExecuteActivity(batchActivityV2, params)
	require.NoError(t, err)

	var result HeartBeatDetails
	require.NoError(t, val.Get(&result))
	assert.Equal(t, 0, result.CurrentPage, "empty page should not increment CurrentPage")
	assert.Equal(t, 0, result.SuccessCount)
}
