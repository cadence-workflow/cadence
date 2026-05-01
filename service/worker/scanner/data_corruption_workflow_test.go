// Copyright (c) 2021 Uber Technologies, Inc.
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

package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/metrics"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/service/history/constants"
)

const testWorkflowName = "default-test-workflow-type-name"

var validBranchToken = []byte{89, 11, 0, 10, 0, 0, 0, 12, 116, 101, 115, 116, 45, 116, 114, 101, 101, 45, 105, 100, 11, 0, 20, 0, 0, 0, 14, 116, 101, 115, 116, 45, 98, 114, 97, 110, 99, 104, 45, 105, 100, 0}

type dataCorruptionWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestDataCorruptionWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(dataCorruptionWorkflowTestSuite))
}

func (s *dataCorruptionWorkflowTestSuite) TestWorkflow_SignalWorkflow_Success() {
	signalOnce := false
	env := s.NewTestWorkflowEnvironment()
	env.OnActivity(ExecutionFixerActivity, mock.Anything, mock.Anything).Return([]invariant.FixResult{}, nil).Times(1)
	env.OnActivity(EmitResultMetricsActivity, mock.Anything, mock.Anything).Return(nil).Times(1)
	env.SetOnTimerScheduledListener(func(timerID string, duration time.Duration) {
		if !signalOnce {
			env.SignalWorkflow(reconciliation.CheckDataCorruptionWorkflowSignalName, entity.Execution{
				ShardID:    0,
				DomainID:   uuid.New(),
				WorkflowID: uuid.New(),
				RunID:      uuid.New(),
			})
			signalOnce = true
		}
	})
	env.ExecuteWorkflow(reconciliation.CheckDataCorruptionWorkflowType, nil)
	s.True(env.IsWorkflowCompleted())
	env.AssertExpectations(s.T())
}

func (s *dataCorruptionWorkflowTestSuite) TestWorkflow_TimerFire_Success() {
	isTimerFire := false
	env := s.NewTestWorkflowEnvironment()
	env.SetOnTimerFiredListener(func(timerID string) {
		isTimerFire = true
	})
	env.ExecuteWorkflow(reconciliation.CheckDataCorruptionWorkflowType, nil)
	s.True(env.IsWorkflowCompleted())
	s.True(isTimerFire)
	env.AssertExpectations(s.T())
	env.AssertNotCalled(s.T(), "ExecutionFixerActivity", mock.Anything, mock.Anything)
	env.AssertNotCalled(s.T(), "EmitResultMetricsActivity", mock.Anything, mock.Anything)
}

func (s *dataCorruptionWorkflowTestSuite) TestExecutionFixerActivity_Success() {
	env := s.NewTestActivityEnvironment()
	controller := gomock.NewController(s.T())
	mockResource := resource.NewTest(s.T(), controller, metrics.Worker)
	defer mockResource.Finish(s.T())
	fixList := []entity.Execution{
		{
			ShardID:    0,
			DomainID:   uuid.New(),
			WorkflowID: uuid.New(),
			RunID:      uuid.New(),
		},
	}
	mockResource.ExecutionMgr.On("GetShardID").Return(0)
	mockResource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(&p.GetCurrentExecutionResponse{
		RunID:            fixList[0].RunID,
		State:            2,
		CloseStatus:      1,
		LastWriteVersion: 0,
	}, nil)
	mockResource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&p.GetWorkflowExecutionResponse{
		State: &p.WorkflowMutableState{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				BranchToken: validBranchToken,
			},
			VersionHistories: &p.VersionHistories{
				CurrentVersionHistoryIndex: 0,
				Histories: []*p.VersionHistory{
					{
						BranchToken: validBranchToken,
					},
				},
			},
		},
	}, nil)
	mockResource.ExecutionMgr.On("DeleteWorkflowExecution", mock.Anything, mock.Anything).Return(nil)
	mockResource.ExecutionMgr.On("DeleteCurrentWorkflowExecution", mock.Anything, mock.Anything).Return(nil)
	mockResource.HistoryMgr.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(&p.ReadHistoryBranchResponse{}, nil)
	ctx := context.WithValue(context.Background(), contextKey(testWorkflowName), scannerContext{resource: mockResource})
	env.SetTestTimeout(time.Second * 5)
	env.SetWorkerOptions(worker.Options{
		BackgroundActivityContext: ctx,
	})
	tlScavengerHBInterval = time.Millisecond * 10
	mockResource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain-name", nil).AnyTimes()
	mockResource.DomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()
	_, err := env.ExecuteActivity(ExecutionFixerActivity, fixList)
	s.NoError(err)
}
