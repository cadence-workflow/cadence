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

package host

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	retryLimit       = 20
	retryBackoffTime = 200 * time.Millisecond
)

func (s *IntegrationSuite) TestArchival_TimerQueueProcessor() {
	s.True(s.TestCluster.archiverBase.metadata.GetHistoryConfig().ClusterConfiguredForArchival())

	domainID := s.getDomainID(s.ArchivalDomainName)
	WorkflowID := "archival-timer-queue-processor-workflow-id"
	workflowType := "archival-timer-queue-processor-type"
	taskList := "archival-timer-queue-processor-task-list"
	numActivities := 1
	numRuns := 1
	RunID := s.startAndFinishWorkflow(WorkflowID, workflowType, taskList, s.ArchivalDomainName, domainID, numActivities, numRuns)[0]

	execution := &types.WorkflowExecution{
		WorkflowID: WorkflowID,
		RunID:      RunID,
	}
	s.True(s.isHistoryArchived(s.ArchivalDomainName, execution))
	s.True(s.isHistoryDeleted(domainID, execution))
	s.True(s.isMutableStateDeleted(domainID, execution))
}

func (s *IntegrationSuite) TestArchival_ContinueAsNew() {
	s.True(s.TestCluster.archiverBase.metadata.GetHistoryConfig().ClusterConfiguredForArchival())

	domainID := s.getDomainID(s.ArchivalDomainName)
	WorkflowID := "archival-continueAsNew-workflow-id"
	workflowType := "archival-continueAsNew-workflow-type"
	taskList := "archival-continueAsNew-task-list"
	numActivities := 1
	numRuns := 5
	RunIDs := s.startAndFinishWorkflow(WorkflowID, workflowType, taskList, s.ArchivalDomainName, domainID, numActivities, numRuns)

	for _, RunID := range RunIDs {
		execution := &types.WorkflowExecution{
			WorkflowID: WorkflowID,
			RunID:      RunID,
		}
		s.True(s.isHistoryArchived(s.ArchivalDomainName, execution))
		s.True(s.isHistoryDeleted(domainID, execution))
		s.True(s.isMutableStateDeleted(domainID, execution))
	}
}

func (s *IntegrationSuite) TestArchival_ArchiverWorker() {
	s.True(s.TestCluster.archiverBase.metadata.GetHistoryConfig().ClusterConfiguredForArchival())

	domainID := s.getDomainID(s.ArchivalDomainName)
	WorkflowID := "archival-archiver-worker-workflow-id"
	workflowType := "archival-archiver-worker-workflow-type"
	taskList := "archival-archiver-worker-task-list"
	numActivities := 10
	RunID := s.startAndFinishWorkflow(WorkflowID, workflowType, taskList, s.ArchivalDomainName, domainID, numActivities, 1)[0]

	execution := &types.WorkflowExecution{
		WorkflowID: WorkflowID,
		RunID:      RunID,
	}
	s.True(s.isHistoryArchived(s.ArchivalDomainName, execution))
	s.True(s.isHistoryDeleted(domainID, execution))
	s.True(s.isMutableStateDeleted(domainID, execution))
}

func (s *IntegrationSuite) TestVisibilityArchival() {
	s.True(s.TestCluster.archiverBase.metadata.GetVisibilityConfig().ClusterConfiguredForArchival())

	domainID := s.getDomainID(s.ArchivalDomainName)
	WorkflowID := "archival-visibility-workflow-id"
	workflowType := "archival-visibility-workflow-type"
	taskList := "archival-visibility-task-list"
	numActivities := 3
	numRuns := 5
	startTime := time.Now().UnixNano()
	s.startAndFinishWorkflow(WorkflowID, workflowType, taskList, s.ArchivalDomainName, domainID, numActivities, numRuns)
	s.startAndFinishWorkflow("some other WorkflowID", "some other workflow type", taskList, s.ArchivalDomainName, domainID, numActivities, numRuns)
	endTime := time.Now().UnixNano()

	var executions []*types.WorkflowExecutionInfo

	for i := 0; i != retryLimit; i++ {
		executions = []*types.WorkflowExecutionInfo{}
		request := &types.ListArchivedWorkflowExecutionsRequest{
			Domain:   s.ArchivalDomainName,
			PageSize: 2,
			Query:    fmt.Sprintf("CloseTime >= %v and CloseTime <= %v and WorkflowType = '%s'", startTime, endTime, workflowType),
		}
		for len(executions) == 0 || request.NextPageToken != nil {
			ctx, cancel := createContext()
			response, err := s.Engine.ListArchivedWorkflowExecutions(ctx, request)
			cancel()
			s.NoError(err)
			s.NotNil(response)
			executions = append(executions, response.GetExecutions()...)
			request.NextPageToken = response.NextPageToken
		}
		if len(executions) == numRuns {
			break
		}
		time.Sleep(retryBackoffTime)
	}

	for _, execution := range executions {
		s.Equal(WorkflowID, execution.GetExecution().GetWorkflowID())
		s.Equal(workflowType, execution.GetType().GetName())
		s.NotZero(execution.StartTime)
		s.NotZero(execution.ExecutionTime)
		s.NotZero(execution.CloseTime)
	}
}

func (s *IntegrationSuite) getDomainID(domain string) string {
	ctx, cancel := createContext()
	defer cancel()
	domainResp, err := s.Engine.DescribeDomain(ctx, &types.DescribeDomainRequest{
		Name: common.StringPtr(s.ArchivalDomainName),
	})
	s.Nil(err)
	return domainResp.DomainInfo.GetUUID()
}

func (s *IntegrationSuite) isHistoryArchived(domain string, execution *types.WorkflowExecution) bool {
	request := &types.GetWorkflowExecutionHistoryRequest{
		Domain:    s.ArchivalDomainName,
		Execution: execution,
	}

	for i := 0; i < retryLimit; i++ {
		ctx, cancel := createContext()
		getHistoryResp, err := s.Engine.GetWorkflowExecutionHistory(ctx, request)
		cancel()
		if err == nil && getHistoryResp != nil && getHistoryResp.GetArchived() {
			return true
		}
		time.Sleep(retryBackoffTime)
	}
	return false
}

func (s *IntegrationSuite) isHistoryDeleted(domainID string, execution *types.WorkflowExecution) bool {
	shardID := common.WorkflowIDToHistoryShard(execution.WorkflowID, s.TestClusterConfig.HistoryConfig.NumHistoryShards)

	request := &persistence.GetHistoryTreeRequest{
		TreeID:     execution.GetRunID(),
		ShardID:    common.IntPtr(shardID),
		DomainName: s.DomainName,
	}
	for i := 0; i < retryLimit; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
		resp, err := s.TestCluster.testBase.HistoryV2Mgr.GetHistoryTree(ctx, request)
		s.Nil(err)
		cancel()
		if len(resp.Branches) == 0 {
			return true
		}
		time.Sleep(retryBackoffTime)
	}
	return false
}

func (s *IntegrationSuite) isMutableStateDeleted(domainID string, execution *types.WorkflowExecution) bool {
	request := &persistence.GetWorkflowExecutionRequest{
		DomainID:  domainID,
		Execution: *execution,
	}

	for i := 0; i < retryLimit; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
		_, err := s.TestCluster.testBase.ExecutionManager.GetWorkflowExecution(ctx, request)
		cancel()
		if _, ok := err.(*types.EntityNotExistsError); ok {
			return true
		}
		time.Sleep(retryBackoffTime)

	}
	return false
}

func (s *IntegrationSuite) startAndFinishWorkflow(id, wt, tl, domain, domainID string, numActivities, numRuns int) []string {
	identity := "worker1"
	activityName := "activity_type1"
	workflowType := &types.WorkflowType{
		Name: wt,
	}
	taskList := &types.TaskList{
		Name: tl,
	}
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              domain,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)
	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	RunIDs := make([]string, numRuns)

	workflowComplete := false
	activityCount := int32(numActivities)
	activityCounter := int32(0)
	expectedActivityID := int32(1)
	runCounter := 1

	dtHandler := func(
		execution *types.WorkflowExecution,
		wt *types.WorkflowType,
		previousStartedEventID int64,
		startedEventID int64,
		history *types.History,
	) ([]byte, []*types.Decision, error) {
		RunIDs[runCounter-1] = execution.GetRunID()
		if activityCounter < activityCount {
			activityCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityCounter))
			return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    strconv.Itoa(int(activityCounter)),
					ActivityType:                  &types.ActivityType{Name: activityName},
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		}

		if runCounter < numRuns {
			activityCounter = int32(0)
			expectedActivityID = int32(1)
			runCounter++
			return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
				ContinueAsNewWorkflowExecutionDecisionAttributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
					WorkflowType:                        workflowType,
					TaskList:                            &types.TaskList{Name: tl},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
				},
			}}, nil
		}

		workflowComplete = true
		return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	atHandler := func(
		execution *types.WorkflowExecution,
		activityType *types.ActivityType,
		activityID string,
		input []byte,
		taskToken []byte,
	) ([]byte, bool, error) {
		s.Equal(id, execution.WorkflowID)
		s.Equal(activityName, activityType.Name)
		id, _ := strconv.Atoi(activityID)
		s.Equal(int(expectedActivityID), id)
		buf := bytes.NewReader(input)
		var in int32
		binary.Read(buf, binary.LittleEndian, &in)
		s.Equal(expectedActivityID, in)
		expectedActivityID++
		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          domain,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}
	for run := 0; run < numRuns; run++ {
		for i := 0; i < numActivities; i++ {
			_, err := poller.PollAndProcessDecisionTask(false, false)
			s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
			s.Nil(err)
			if i%2 == 0 {
				err = poller.PollAndProcessActivityTask(false)
			} else { // just for testing respondActivityTaskCompleteByID
				err = poller.PollAndProcessActivityTaskWithID(false)
			}
			s.Logger.Info("PollAndProcessActivityTask", tag.Error(err))
			s.Nil(err)
		}

		_, err = poller.PollAndProcessDecisionTask(false, false)
		s.Nil(err)
	}

	s.True(workflowComplete)
	for run := 1; run < numRuns; run++ {
		s.NotEqual(RunIDs[run-1], RunIDs[run])
	}
	return RunIDs
}
