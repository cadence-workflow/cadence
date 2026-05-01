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
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	mmocks "github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/types"
)

type workflowSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	workflowEnv *testsuite.TestWorkflowEnvironment
	activityEnv *testsuite.TestActivityEnvironment
}

func TestWorkflowSuite(t *testing.T) {
	suite.Run(t, new(workflowSuite))
}

func (s *workflowSuite) SetupTest() {
	s.workflowEnv = s.NewTestWorkflowEnvironment()
	s.workflowEnv.RegisterWorkflow(BatchWorkflow)

	s.activityEnv = s.NewTestActivityEnvironment()
	s.activityEnv.RegisterActivity(BatchActivity)

	batcher, mockResource := setuptest(s.T())

	metricsMock := &mmocks.Client{}
	metricsMock.On("IncCounter", metrics.BatcherScope, metrics.BatcherProcessorSuccess).Once()
	batcher.metricsClient = metricsMock

	mockResource.FrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(&types.DescribeDomainResponse{}, nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.ListWorkflowExecutionsResponse{
		Executions:    []*types.WorkflowExecutionInfo{{Execution: &types.WorkflowExecution{WorkflowID: "wid", RunID: "rid"}}},
		NextPageToken: nil,
	}, nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{}, nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mockResource.RemoteAdminClient.EXPECT().ResendReplicationTasks(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	ctx := context.WithValue(context.Background(), BatcherContextKey, batcher)
	workerOpts := worker.Options{
		MetricsScope:              tally.TestScope(nil),
		BackgroundActivityContext: ctx,
		Tracer:                    opentracing.GlobalTracer(),
	}
	s.activityEnv.SetWorkerOptions(workerOpts)

}

func (s *workflowSuite) TestWorkflow() {
	params := createParams(BatchTypeCancel)
	activityHeartBeatDeatils := HeartBeatDetails{}
	s.workflowEnv.OnActivity(batchActivityName, mock.Anything, mock.Anything).Return(activityHeartBeatDeatils, nil)
	s.workflowEnv.ExecuteWorkflow(BatchWorkflow, params)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
}

func (s *workflowSuite) TestActivity_BatchCancel() {
	params := createParams(BatchTypeCancel)
	_, err := s.activityEnv.ExecuteActivity(BatchActivity, params)
	s.NoError(err)
}

func (s *workflowSuite) TestActivity_BatchTerminate() {
	params := createParams(BatchTypeTerminate)
	_, err := s.activityEnv.ExecuteActivity(BatchActivity, params)
	s.NoError(err)
}

func (s *workflowSuite) TestActivity_BatchSignal() {
	params := createParams(BatchTypeSignal)
	_, err := s.activityEnv.ExecuteActivity(BatchActivity, params)
	s.NoError(err)
}

func (s *workflowSuite) TestActivity_BatchReplicate() {
	params := createParams(BatchTypeReplicate)
	_, err := s.activityEnv.ExecuteActivity(BatchActivity, params)
	s.NoError(err)
}

func (s *workflowSuite) TestWorkflow_BatchTypeCancelValidationError() {
	params := createParams(BatchTypeCancel)
	params.Query = ""
	s.workflowEnv.ExecuteWorkflow(BatchWorkflow, params)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), "must provide required parameters: BatchType/Reason/DomainName/Query")
}

func (s *workflowSuite) TestWorkflow_UnsupportedBatchType() {
	params := createParams("invalid-batch-type")
	s.workflowEnv.ExecuteWorkflow(BatchWorkflow, params)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), "not supported batch type")
}

func (s *workflowSuite) TestWorkflow_BatchTypeSignalValidation() {
	params := createParams(BatchTypeSignal)
	params.SignalParams.SignalName = ""
	s.workflowEnv.ExecuteWorkflow(BatchWorkflow, params)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), "must provide signal name")
}

func (s *workflowSuite) TestWorkflow_BatchTypeReplicateSourceCLusterValidation() {
	params := createParams(BatchTypeReplicate)
	params.ReplicateParams.SourceCluster = ""
	s.workflowEnv.ExecuteWorkflow(BatchWorkflow, params)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), "must provide source cluster")
}

func (s *workflowSuite) TestWorkflow_BatchTypeReplicateTargetCLusterValidation() {
	params := createParams(BatchTypeReplicate)
	params.ReplicateParams.TargetCluster = ""
	s.workflowEnv.ExecuteWorkflow(BatchWorkflow, params)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), "must provide target cluster")
}

func (s *workflowSuite) TearDownTest() {
	s.workflowEnv.AssertExpectations(s.T())
}

func createParams(batchType string) BatchParams {
	return BatchParams{
		DomainName: "test-domain",
		Query:      "Closetime=missing",
		Reason:     "unit-test",
		BatchType:  batchType,
		TerminateParams: TerminateParams{
			TerminateChildren: common.BoolPtr(true),
		},
		CancelParams: CancelParams{
			CancelChildren: common.BoolPtr(true),
		},
		SignalParams: SignalParams{
			SignalName: "test-signal-name",
		},
		ReplicateParams: ReplicateParams{
			SourceCluster: "test-primary-cluster",
			TargetCluster: "test-secondary-cluster",
		},
		RPS:                      5,
		Concurrency:              5,
		PageSize:                 10,
		AttemptsOnRetryableError: 0,
		ActivityHeartBeatTimeout: 0,
		MaxActivityRetries:       0,
		NonRetryableErrors:       []string{"HeartbeatTimeoutError"},
		_nonRetryableErrors:      nil,
	}
}
