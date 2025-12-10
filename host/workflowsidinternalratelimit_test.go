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

package host

import (
	"bytes"
	"encoding/binary"
	"flag"
	"strconv"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/tasklist"
)

func TestWorkflowIDInternalRateLimitIntegrationSuite(t *testing.T) {
	// Loads the flags for persistence etc., if none are given they are set in ./flag.go
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/integration_wfidratelimit_cluster.yaml")
	require.NoError(t, err)

	clusterConfig.TimeSource = clock.NewMockedTimeSource()
	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.WorkflowIDInternalRPS: 2,
	}

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(WorkflowIDInternalRateLimitIntegrationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *WorkflowIDInternalRateLimitIntegrationSuite) SetupSuite() {
	s.SetupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := NewClusterMetadata(s.T(), s.TestClusterConfig)
	dc := persistence.DynamicConfiguration{
		EnableCassandraAllConsistencyLevelDelete: dynamicproperties.GetBoolPropertyFn(true),
		PersistenceSampleLoggingRate:             dynamicproperties.GetIntPropertyFn(100),
		EnableShardIDMetrics:                     dynamicproperties.GetBoolPropertyFn(true),
		EnableHistoryTaskDualWriteMode:           dynamicproperties.GetBoolPropertyFn(true),
		ReadNoSQLHistoryTaskFromDataBlob:         dynamicproperties.GetBoolPropertyFn(false),
		SerializationEncoding:                    dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		ReadNoSQLShardFromDataBlob:               dynamicproperties.GetBoolPropertyFn(true),
		HistoryNodeDeleteBatchSize:               dynamicproperties.GetIntPropertyFn(1000),
	}
	params := pt.TestBaseParams{
		DefaultTestCluster:    s.DefaultTestCluster,
		VisibilityTestCluster: s.VisibilityTestCluster,
		ClusterMetadata:       clusterMetadata,
		DynamicConfiguration:  dc,
	}
	cluster, err := NewCluster(s.T(), s.TestClusterConfig, s.Logger, params)
	s.Require().NoError(err)
	s.TestCluster = cluster
	s.Engine = s.TestCluster.GetFrontendClient()
	s.AdminClient = s.TestCluster.GetAdminClient()

	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	s.domainCacheRefresh()
}

func (s *WorkflowIDInternalRateLimitIntegrationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *WorkflowIDInternalRateLimitIntegrationSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

func (s *WorkflowIDInternalRateLimitIntegrationSuite) TestWorkflowIDSpecificInternalRateLimits() {
	const (
		testWorkflowID   = "integration-workflow-specific-internal-rate-limit-test"
		testWorkflowType = "integration-workflow-specific-internal-rate-limit-test-type"
		testTaskListName = "integration-workflow-specific-internal-rate-limit-test-taskList"
		testIdentity     = "worker1"
	)

	activityName := "test-activity"

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          testWorkflowID,
		WorkflowType:                        &types.WorkflowType{Name: testWorkflowType},
		TaskList:                            &types.TaskList{Name: testTaskListName},
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            testIdentity,
		WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyTerminateIfRunning.Ptr(),
	}

	ctx, cancel := createContext()
	defer cancel()

	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.NoError(err)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowComplete := false
	activityCount := int32(5)
	activityCounter := int32(0)

	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if activityCounter < activityCount {
			activityCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityCounter))

			return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    strconv.Itoa(int(activityCounter)),
					ActivityType:                  &types.ActivityType{Name: activityName},
					TaskList:                      &types.TaskList{Name: testTaskListName},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(5),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(5),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(5),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(1),
				},
			}}, nil
		}

		s.Logger.Info("Completing Workflow.")

		workflowComplete = true
		return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	activityExecutedCount := int32(0)
	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		activityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		s.Equal(testWorkflowID, execution.WorkflowID)
		s.Equal(activityName, activityType.Name)

		activityExecutedCount++
		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        &types.TaskList{Name: testTaskListName},
		Identity:        testIdentity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	for i := int32(0); i < activityCount; i++ {
		_, err = poller.PollAndProcessDecisionTask(false, false)
		s.True(err == nil || err == tasklist.ErrNoTasks)

		err = poller.PollAndProcessActivityTask(false)
		s.True(err == nil || err == tasklist.ErrNoTasks)
	}

	s.Logger.Info("Waiting for workflow to complete", tag.WorkflowRunID(we.RunID))

	s.False(workflowComplete)
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil || err == tasklist.ErrNoTasks)
	s.True(workflowComplete)

	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
		},
	})
	s.NoError(err)
	history := historyResponse.History
	firstEvent := history.Events[0]
	lastEvent := history.Events[len(history.Events)-1]
	// First 7 event ids --> 0 (Workflow start), 1-3 ( Decision scheduled,started,completed) , 4-6 (Activity scheduled,started,completed) post which the tasks will be rate limited since RPS is 2
	eventBeforeRatelimited := history.Events[7]

	timeElapsedBeforeRatelimiting := time.Unix(0, common.Int64Default(eventBeforeRatelimited.Timestamp)).Sub(time.Unix(0, common.Int64Default(firstEvent.Timestamp)))
	s.True(timeElapsedBeforeRatelimiting < 1*time.Second)

	totalTime := time.Unix(0, common.Int64Default(lastEvent.Timestamp)).Sub(time.Unix(0, common.Int64Default(firstEvent.Timestamp)))
	s.True(totalTime > 3*time.Second)

}
