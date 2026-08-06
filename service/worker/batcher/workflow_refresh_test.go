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
	"errors"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestRefreshBatchActivity(t *testing.T) {
	tests := []struct {
		name       string
		activity   interface{}
		wantTuning bool
	}{
		{
			name:     "V1",
			activity: BatchActivity,
		},
		{
			name:       "V2",
			activity:   batchActivityV2,
			wantTuning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			activityEnv := suite.NewTestActivityEnvironment()
			activityEnv.RegisterActivity(tt.activity)

			batcher, mockResource := setuptest(t)
			setRefreshActivityWorkerOptions(activityEnv, batcher)

			params := createParams(BatchTypeRefresh)
			params.Concurrency = 1
			params.RPS = 100
			params.PageSize = 10
			expectedExecution := &types.WorkflowExecution{
				WorkflowID: "workflow-id",
				RunID:      "run-id",
			}

			mockResource.FrontendClient.EXPECT().
				DescribeDomain(gomock.Any(), gomock.Any()).
				Return(&types.DescribeDomainResponse{}, nil)
			mockResource.FrontendClient.EXPECT().
				CountWorkflowExecutions(gomock.Any(), &types.CountWorkflowExecutionsRequest{
					Domain: params.DomainName,
					Query:  params.Query,
				}).
				Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
			mockResource.FrontendClient.EXPECT().
				ScanWorkflowExecutions(gomock.Any(), &types.ListWorkflowExecutionsRequest{
					Domain:   params.DomainName,
					PageSize: int32(params.PageSize),
					Query:    params.Query,
				}).
				Return(&types.ListWorkflowExecutionsResponse{
					Executions: []*types.WorkflowExecutionInfo{{Execution: expectedExecution}},
				}, nil)
			mockResource.FrontendClient.EXPECT().
				RefreshWorkflowTasks(gomock.Any(), &types.RefreshWorkflowTasksRequest{
					Domain:    params.DomainName,
					Execution: expectedExecution,
				}).
				Return(nil)
			mockResource.FrontendClient.EXPECT().
				DescribeWorkflowExecution(gomock.Any(), &types.DescribeWorkflowExecutionRequest{
					Domain:    params.DomainName,
					Execution: expectedExecution,
				}).
				Return(&types.DescribeWorkflowExecutionResponse{
					PendingChildren: []*types.PendingChildExecutionInfo{{
						WorkflowID: "child-workflow-id",
						RunID:      "child-run-id",
					}},
				}, nil)

			value, err := activityEnv.ExecuteActivity(tt.activity, params)
			if err != nil {
				t.Fatalf("ExecuteActivity() error = %v", err)
			}

			var result HeartBeatDetails
			if err := value.Get(&result); err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if result.SuccessCount != 1 {
				t.Errorf("SuccessCount = %d, want 1", result.SuccessCount)
			}
			if result.ErrorCount != 0 {
				t.Errorf("ErrorCount = %d, want 0", result.ErrorCount)
			}
			if result.CurrentPage != 1 {
				t.Errorf("CurrentPage = %d, want 1", result.CurrentPage)
			}
			if tt.wantTuning && (result.RPS != params.RPS || result.Concurrency != params.Concurrency) {
				t.Errorf("V2 tuning details = (%d, %d), want (%d, %d)", result.RPS, result.Concurrency, params.RPS, params.Concurrency)
			}
		})
	}
}

func TestRefreshBatchActivityEntityNotExistsIsSuccess(t *testing.T) {
	tests := []struct {
		name     string
		activity interface{}
	}{
		{
			name:     "V1",
			activity: BatchActivity,
		},
		{
			name:     "V2",
			activity: batchActivityV2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			activityEnv := suite.NewTestActivityEnvironment()
			activityEnv.RegisterActivity(tt.activity)

			batcher, mockResource := setuptest(t)
			setRefreshActivityWorkerOptions(activityEnv, batcher)

			params := createParams(BatchTypeRefresh)
			params.Concurrency = 1
			params.RPS = 100
			expectedExecution := &types.WorkflowExecution{
				WorkflowID: "workflow-id",
				RunID:      "run-id",
			}

			mockResource.FrontendClient.EXPECT().
				DescribeDomain(gomock.Any(), gomock.Any()).
				Return(&types.DescribeDomainResponse{}, nil)
			mockResource.FrontendClient.EXPECT().
				CountWorkflowExecutions(gomock.Any(), gomock.Any()).
				Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
			mockResource.FrontendClient.EXPECT().
				ScanWorkflowExecutions(gomock.Any(), gomock.Any()).
				Return(&types.ListWorkflowExecutionsResponse{
					Executions: []*types.WorkflowExecutionInfo{{Execution: expectedExecution}},
				}, nil)
			mockResource.FrontendClient.EXPECT().
				RefreshWorkflowTasks(gomock.Any(), &types.RefreshWorkflowTasksRequest{
					Domain:    params.DomainName,
					Execution: expectedExecution,
				}).
				Return(&types.EntityNotExistsError{})

			value, err := activityEnv.ExecuteActivity(tt.activity, params)
			if err != nil {
				t.Fatalf("ExecuteActivity() error = %v", err)
			}

			var result HeartBeatDetails
			if err := value.Get(&result); err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if result.SuccessCount != 1 || result.ErrorCount != 0 {
				t.Errorf("progress = %+v, want one successful no-op", result)
			}
		})
	}
}

func TestRefreshTaskProcessorRetriesRetryableErrors(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	activityEnv := suite.NewTestActivityEnvironment()
	activityEnv.RegisterActivity(BatchActivity)

	batcher, mockResource := setuptest(t)
	setRefreshActivityWorkerOptions(activityEnv, batcher)

	params := createParams(BatchTypeRefresh)
	params.Concurrency = 1
	params.RPS = 100
	params.AttemptsOnRetryableError = 1
	params._nonRetryableErrors = make(map[string]struct{})
	execution := types.WorkflowExecution{
		WorkflowID: "workflow-id",
		RunID:      "run-id",
	}
	request := &types.RefreshWorkflowTasksRequest{
		Domain:    params.DomainName,
		Execution: &execution,
	}

	mockResource.FrontendClient.EXPECT().
		DescribeDomain(gomock.Any(), gomock.Any()).
		Return(&types.DescribeDomainResponse{}, nil)
	mockResource.FrontendClient.EXPECT().
		CountWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
	mockResource.FrontendClient.EXPECT().
		ScanWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.ListWorkflowExecutionsResponse{
			Executions: []*types.WorkflowExecutionInfo{{Execution: &execution}},
		}, nil)
	firstRefresh := mockResource.FrontendClient.EXPECT().
		RefreshWorkflowTasks(gomock.Any(), request).
		Return(errors.New("transient error"))
	secondRefresh := mockResource.FrontendClient.EXPECT().
		RefreshWorkflowTasks(gomock.Any(), request).
		Return(nil)
	describe := mockResource.FrontendClient.EXPECT().
		DescribeWorkflowExecution(gomock.Any(), &types.DescribeWorkflowExecutionRequest{
			Domain:    params.DomainName,
			Execution: &execution,
		}).
		Return(&types.DescribeWorkflowExecutionResponse{}, nil)
	gomock.InOrder(firstRefresh, secondRefresh, describe)

	value, err := activityEnv.ExecuteActivity(BatchActivity, params)
	if err != nil {
		t.Fatalf("ExecuteActivity() error = %v", err)
	}

	var result HeartBeatDetails
	if err := value.Get(&result); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.SuccessCount != 1 || result.ErrorCount != 0 {
		t.Errorf("progress = %+v, want one retried success", result)
	}
}

func TestValidateRefreshBatchParams(t *testing.T) {
	tests := []struct {
		name    string
		params  BatchParams
		wantErr string
	}{
		{
			name:   "refresh needs no operation-specific parameters",
			params: createParams(BatchTypeRefresh),
		},
		{
			name:    "unsupported batch type",
			params:  createParams("unsupported"),
			wantErr: "not supported batch type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateParams(tt.params)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateParams() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != "not supported batch type: unsupported" {
				t.Fatalf("validateParams() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func setRefreshActivityWorkerOptions(activityEnv *testsuite.TestActivityEnvironment, batcher *Batcher) {
	setRefreshBatcherMetrics(batcher)
	ctx := context.WithValue(context.Background(), BatcherContextKey, batcher)
	activityEnv.SetWorkerOptions(worker.Options{
		MetricsScope:              tally.TestScope(nil),
		BackgroundActivityContext: ctx,
		Tracer:                    opentracing.GlobalTracer(),
	})
}

func setRefreshBatcherMetrics(batcher *Batcher) {
	batcher.metricsClient = metrics.NewClient(tally.NoopScope, metrics.Worker, metrics.MigrationConfig{})
}
