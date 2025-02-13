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

package persistence

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
)

type (
	statsComputerSuite struct {
		sc *statsComputer
		suite.Suite
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

func TestStatsComputerSuite(t *testing.T) {
	s := new(statsComputerSuite)
	suite.Run(t, s)
}

// TODO need to add more tests
func (s *statsComputerSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
	s.sc = &statsComputer{}
}

func (s *statsComputerSuite) createRequest() *InternalUpdateWorkflowExecutionRequest {
	return &InternalUpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: InternalWorkflowMutation{
			ExecutionInfo: &InternalWorkflowExecutionInfo{},
		},
	}
}

func (s *statsComputerSuite) TestStatsWithStartedEvent() {
	ms := s.createRequest()
	domainID := "A"
	execution := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("test-workflow-id"),
		RunId:      common.StringPtr("run_id"),
	}
	workflowType := &workflow.WorkflowType{
		Name: common.StringPtr("test-workflow-type-name"),
	}
	taskList := &workflow.TaskList{
		Name: common.StringPtr("test-tasklist"),
	}

	ms.UpdateWorkflowMutation.ExecutionInfo.DomainID = domainID
	ms.UpdateWorkflowMutation.ExecutionInfo.WorkflowID = execution.GetWorkflowId()
	ms.UpdateWorkflowMutation.ExecutionInfo.RunID = execution.GetRunId()
	ms.UpdateWorkflowMutation.ExecutionInfo.WorkflowTypeName = workflowType.GetName()
	ms.UpdateWorkflowMutation.ExecutionInfo.TaskList = taskList.GetName()

	expectedSize := len(execution.GetWorkflowId()) + len(workflowType.GetName()) + len(taskList.GetName())

	stats := s.sc.computeMutableStateUpdateStats(ms)
	s.Equal(stats.ExecutionInfoSize, expectedSize)
}

func (s *statsComputerSuite) TestComputeWorkflowMutationStats() {
	a1 := &InternalActivityInfo{}
	t1 := &TimerInfo{}
	c1 := &InternalChildExecutionInfo{}
	s1 := &SignalInfo{}
	r1 := &RequestCancelInfo{}
	ms := &InternalWorkflowMutation{
		ExecutionInfo:             &InternalWorkflowExecutionInfo{},
		UpsertActivityInfos:       []*InternalActivityInfo{a1, a1},
		UpsertTimerInfos:          []*TimerInfo{t1, t1, t1},
		UpsertChildExecutionInfos: []*InternalChildExecutionInfo{c1, c1, c1, c1},
		UpsertSignalInfos:         []*SignalInfo{s1},
		UpsertRequestCancelInfos:  []*RequestCancelInfo{r1},
		NewBufferedEvents:         &DataBlob{Data: []byte("asdfsaf")},
		DeleteActivityInfos:       []int64{1, 2, 3, 4},
		DeleteTimerInfos:          []string{"asdfa"},
		DeleteChildExecutionInfos: []int64{0},
		DeleteSignalInfos:         nil,
		DeleteRequestCancelInfos:  []int64{},
		TasksByCategory: map[HistoryTaskCategory][]Task{
			HistoryTaskCategoryTransfer: []Task{
				&ActivityTask{}, &DecisionTask{},
			},
			HistoryTaskCategoryTimer: []Task{
				&UserTimerTask{},
			},
		},
	}
	stats := s.sc.computeWorkflowMutationStats(ms)
	s.Equal(computeExecutionInfoSize(ms.ExecutionInfo), stats.ExecutionInfoSize)
	s.Equal(computeActivityInfoSize(a1)*len(ms.UpsertActivityInfos), stats.ActivityInfoSize)
	s.Equal(len(ms.UpsertActivityInfos), stats.ActivityInfoCount)
	s.Equal(computeTimerInfoSize(t1)*len(ms.UpsertTimerInfos), stats.TimerInfoSize)
	s.Equal(len(ms.UpsertTimerInfos), stats.TimerInfoCount)
	s.Equal(computeChildInfoSize(c1)*len(ms.UpsertChildExecutionInfos), stats.ChildInfoSize)
	s.Equal(len(ms.UpsertChildExecutionInfos), stats.ChildInfoCount)
	s.Equal(computeSignalInfoSize(s1)*len(ms.UpsertSignalInfos), stats.SignalInfoSize)
	s.Equal(len(ms.UpsertSignalInfos), stats.SignalInfoCount)
	s.Equal(len(ms.UpsertRequestCancelInfos), stats.RequestCancelInfoCount)
	s.Equal(len(ms.NewBufferedEvents.Data), stats.BufferedEventsSize)
	s.Equal(len(ms.DeleteActivityInfos), stats.DeleteActivityInfoCount)
	s.Equal(len(ms.DeleteTimerInfos), stats.DeleteTimerInfoCount)
	s.Equal(len(ms.DeleteChildExecutionInfos), stats.DeleteChildInfoCount)
	s.Equal(len(ms.DeleteSignalInfos), stats.DeleteSignalInfoCount)
	s.Equal(len(ms.DeleteRequestCancelInfos), stats.DeleteRequestCancelInfoCount)
	s.Equal(stats.ExecutionInfoSize+stats.ActivityInfoSize+stats.TimerInfoSize+stats.ChildInfoSize+stats.SignalInfoSize+stats.BufferedEventsSize, stats.MutableStateSize)
	for c, tasks := range ms.TasksByCategory {
		s.Equal(len(tasks), stats.TaskCountByCategory[c])
	}
}

func (s *statsComputerSuite) TestComputeWorkflowSnapshotStats() {
	a1 := &InternalActivityInfo{}
	t1 := &TimerInfo{}
	c1 := &InternalChildExecutionInfo{}
	s1 := &SignalInfo{}
	r1 := &RequestCancelInfo{}
	ms := &InternalWorkflowSnapshot{
		ExecutionInfo:       &InternalWorkflowExecutionInfo{},
		ActivityInfos:       []*InternalActivityInfo{a1, a1},
		TimerInfos:          []*TimerInfo{t1, t1, t1},
		ChildExecutionInfos: []*InternalChildExecutionInfo{c1, c1, c1, c1},
		SignalInfos:         []*SignalInfo{s1},
		RequestCancelInfos:  []*RequestCancelInfo{r1},
		TasksByCategory: map[HistoryTaskCategory][]Task{
			HistoryTaskCategoryTransfer: []Task{
				&ActivityTask{}, &DecisionTask{},
			},
			HistoryTaskCategoryTimer: []Task{
				&UserTimerTask{},
			},
		},
	}
	stats := s.sc.computeWorkflowSnapshotStats(ms)
	s.Equal(computeExecutionInfoSize(ms.ExecutionInfo), stats.ExecutionInfoSize)
	s.Equal(computeActivityInfoSize(a1)*len(ms.ActivityInfos), stats.ActivityInfoSize)
	s.Equal(len(ms.ActivityInfos), stats.ActivityInfoCount)
	s.Equal(computeTimerInfoSize(t1)*len(ms.TimerInfos), stats.TimerInfoSize)
	s.Equal(len(ms.TimerInfos), stats.TimerInfoCount)
	s.Equal(computeChildInfoSize(c1)*len(ms.ChildExecutionInfos), stats.ChildInfoSize)
	s.Equal(len(ms.ChildExecutionInfos), stats.ChildInfoCount)
	s.Equal(computeSignalInfoSize(s1)*len(ms.SignalInfos), stats.SignalInfoSize)
	s.Equal(len(ms.SignalInfos), stats.SignalInfoCount)
	s.Equal(len(ms.RequestCancelInfos), stats.RequestCancelInfoCount)
	s.Equal(stats.ExecutionInfoSize+stats.ActivityInfoSize+stats.TimerInfoSize+stats.ChildInfoSize+stats.SignalInfoSize+stats.BufferedEventsSize, stats.MutableStateSize)
	for c, tasks := range ms.TasksByCategory {
		s.Equal(len(tasks), stats.TaskCountByCategory[c])
	}
}
