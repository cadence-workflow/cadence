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

package ndc

import (
	"context"
	"flag"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	adminClient "github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	test "github.com/uber/cadence/common/testing"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/host"
)

var (
	clusterName              = []string{"active", "standby", "other"}
	clusterReplicationConfig = []*types.ClusterReplicationConfiguration{
		{ClusterName: clusterName[0]},
		{ClusterName: clusterName[1]},
		{ClusterName: clusterName[2]},
	}
)

func TestNDCIntegrationTestSuite(t *testing.T) {
	flag.Parse()

	clusterConfigs, err := host.GetTestClusterConfigs("../testdata/ndc_integration_test_clusters.yaml")
	if err != nil {
		panic(err)
	}
	clusterConfigs[0].WorkerConfig = &host.WorkerConfig{}
	clusterConfigs[1].WorkerConfig = &host.WorkerConfig{}
	testCluster := host.NewPersistenceTestCluster(t, clusterConfigs[0])
	params := NDCIntegrationTestSuiteParams{
		ClusterConfigs:        clusterConfigs,
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
	}
	s := NewNDCIntegrationTestSuite(params)
	suite.Run(t, s)
}

func (s *NDCIntegrationTestSuite) SetupSuite() {
	s.serializer = persistence.NewPayloadSerializer()
	s.logger = testlogger.New(s.T())

	s.standByReplicationTasksChan = make(chan *types.ReplicationTask, 100)

	s.standByTaskID = 0
	s.mockAdminClient = make(map[string]adminClient.Client)
	controller := gomock.NewController(s.T())
	mockStandbyClient := adminClient.NewMockClient(controller)
	mockStandbyClient.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any()).DoAndReturn(s.GetReplicationMessagesMock).AnyTimes()
	mockOtherClient := adminClient.NewMockClient(controller)
	mockOtherClient.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any()).Return(
		&types.GetReplicationMessagesResponse{
			MessagesByShard: make(map[int32]*types.ReplicationMessages),
		}, nil).AnyTimes()
	s.mockAdminClient["standby"] = mockStandbyClient
	s.mockAdminClient["other"] = mockOtherClient
	s.clusterConfigs[0].MockAdminClient = s.mockAdminClient

	clusterMetadata := host.NewClusterMetadata(s.T(), s.clusterConfigs[0])
	dc := persistence.DynamicConfiguration{
		EnableSQLAsyncTransaction:                dynamicproperties.GetBoolPropertyFn(false),
		EnableCassandraAllConsistencyLevelDelete: dynamicproperties.GetBoolPropertyFn(true),
		EnableHistoryTaskDualWriteMode:           dynamicproperties.GetBoolPropertyFn(true),
		ReadNoSQLHistoryTaskFromDataBlob:         dynamicproperties.GetBoolPropertyFn(false),
		SerializationEncoding:                    dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		ReadNoSQLShardFromDataBlob:               dynamicproperties.GetBoolPropertyFn(true),
		HistoryNodeDeleteBatchSize:               dynamicproperties.GetIntPropertyFn(1000),
	}
	params := pt.TestBaseParams{
		DefaultTestCluster:    s.defaultTestCluster,
		VisibilityTestCluster: s.VisibilityTestCluster,
		ClusterMetadata:       clusterMetadata,
		DynamicConfiguration:  dc,
	}
	cluster, err := host.NewCluster(s.T(), s.clusterConfigs[0], s.logger.WithTags(tag.ClusterName(clusterName[0])), params)
	s.Require().NoError(err)
	s.active = cluster

	s.RegisterDomain()

	s.version = s.clusterConfigs[1].ClusterGroupMetadata.ClusterGroup[s.clusterConfigs[1].ClusterGroupMetadata.CurrentClusterName].InitialFailoverVersion
	s.versionIncrement = s.clusterConfigs[0].ClusterGroupMetadata.FailoverVersionIncrement
	s.generator = test.InitializeHistoryEventGenerator(s.domainName, s.version)
}

func (s *NDCIntegrationTestSuite) GetReplicationMessagesMock(
	ctx context.Context,
	request *types.GetReplicationMessagesRequest,
	opts ...yarpc.CallOption,
) (*types.GetReplicationMessagesResponse, error) {
	select {
	case task := <-s.standByReplicationTasksChan:
		taskID := atomic.AddInt64(&s.standByTaskID, 1)
		task.SourceTaskID = taskID
		tasks := []*types.ReplicationTask{task}
		for len(s.standByReplicationTasksChan) > 0 {
			task = <-s.standByReplicationTasksChan
			taskID := atomic.AddInt64(&s.standByTaskID, 1)
			task.SourceTaskID = taskID
			tasks = append(tasks, task)
		}

		replicationMessage := &types.ReplicationMessages{
			ReplicationTasks:       tasks,
			LastRetrievedMessageID: tasks[len(tasks)-1].SourceTaskID,
			HasMore:                true,
		}

		return &types.GetReplicationMessagesResponse{
			MessagesByShard: map[int32]*types.ReplicationMessages{0: replicationMessage},
		}, nil
	default:
		return &types.GetReplicationMessagesResponse{
			MessagesByShard: make(map[int32]*types.ReplicationMessages),
		}, nil
	}
}

func (s *NDCIntegrationTestSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
	s.generator = test.InitializeHistoryEventGenerator(s.domainName, s.version)
}

func (s *NDCIntegrationTestSuite) TearDownSuite() {
	if s.generator != nil {
		s.generator.Reset()
	}
	s.active.TearDownCluster()
}

func (s *NDCIntegrationTestSuite) TestSingleBranch() {

	s.setupRemoteFrontendClients()
	workflowID := "ndc-single-branch-test" + uuid.New()

	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	versions := []int64{101, 1, 201, 301, 401, 601, 501, 801, 1001, 901, 701, 1101}
	for _, version := range versions {
		runID := uuid.New()
		historyBatch := []*types.History{}
		s.generator = test.InitializeHistoryEventGenerator(s.domainName, version)

		for s.generator.HasNextVertex() {
			events := s.generator.GetNextVertices()
			historyEvents := &types.History{}
			for _, event := range events {
				historyEvents.Events = append(historyEvents.Events, event.GetData().(*types.HistoryEvent))
			}
			historyBatch = append(historyBatch, historyEvents)
		}

		versionHistory := s.eventBatchesToVersionHistory(nil, historyBatch)
		s.applyEvents(
			workflowID,
			runID,
			workflowType,
			tasklist,
			versionHistory,
			historyBatch,
			historyClient,
		)

		err := s.verifyEventHistory(workflowID, runID, historyBatch)
		s.Require().NoError(err)
	}
}

func (s *NDCIntegrationTestSuite) verifyEventHistory(
	workflowID string,
	runID string,
	historyBatch []*types.History,
) error {
	// get replicated history events from passive side
	passiveClient := s.active.GetFrontendClient()
	ctx, cancel := s.createContext()
	defer cancel()
	replicatedHistory, err := passiveClient.GetWorkflowExecutionHistory(
		ctx,
		&types.GetWorkflowExecutionHistoryRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
			MaximumPageSize:        1000,
			NextPageToken:          nil,
			WaitForNewEvent:        false,
			HistoryEventFilterType: types.HistoryEventFilterTypeAllEvent.Ptr(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to get history event from passive side: %v", err)
	}

	// compare origin events with replicated events
	batchIndex := 0
	batch := historyBatch[batchIndex].Events
	eventIndex := 0
	for _, event := range replicatedHistory.GetHistory().GetEvents() {
		if eventIndex >= len(batch) {
			batchIndex++
			batch = historyBatch[batchIndex].Events
			eventIndex = 0
		}
		originEvent := batch[eventIndex]
		eventIndex++
		if originEvent.GetEventType() != event.GetEventType() {
			return fmt.Errorf("the replicated event (%v) and the origin event (%v) are not the same",
				originEvent.GetEventType().String(), event.GetEventType().String())
		}
	}

	return nil
}

func (s *NDCIntegrationTestSuite) TestMultipleBranches() {

	s.setupRemoteFrontendClients()
	workflowID := "ndc-multiple-branches-test" + uuid.New()

	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	versions := []int64{101, 1, 201}
	for _, version := range versions {
		runID := uuid.New()

		baseBranch := []*types.History{}
		baseGenerator := test.InitializeHistoryEventGenerator(s.domainName, version)
		baseGenerator.SetVersion(version)

		for i := 0; i < 10 && baseGenerator.HasNextVertex(); i++ {
			events := baseGenerator.GetNextVertices()
			historyEvents := &types.History{}
			for _, event := range events {
				historyEvents.Events = append(historyEvents.Events, event.GetData().(*types.HistoryEvent))
			}
			baseBranch = append(baseBranch, historyEvents)
		}
		baseVersionHistory := s.eventBatchesToVersionHistory(nil, baseBranch)

		branch1 := []*types.History{}
		branchVersionHistory1 := baseVersionHistory.Duplicate()
		branchGenerator1 := baseGenerator.DeepCopy()
		for i := 0; i < 10 && branchGenerator1.HasNextVertex(); i++ {
			events := branchGenerator1.GetNextVertices()
			historyEvents := &types.History{}
			for _, event := range events {
				historyEvents.Events = append(historyEvents.Events, event.GetData().(*types.HistoryEvent))
			}
			branch1 = append(branch1, historyEvents)
		}
		branchVersionHistory1 = s.eventBatchesToVersionHistory(branchVersionHistory1, branch1)

		branch2 := []*types.History{}
		branchVersionHistory2 := baseVersionHistory.Duplicate()
		branchGenerator2 := baseGenerator.DeepCopy()
		branchGenerator2.SetVersion(branchGenerator2.GetVersion() + 1)
		for i := 0; i < 10 && branchGenerator2.HasNextVertex(); i++ {
			events := branchGenerator2.GetNextVertices()
			historyEvents := &types.History{}
			for _, event := range events {
				historyEvents.Events = append(historyEvents.Events, event.GetData().(*types.HistoryEvent))
			}
			branch2 = append(branch2, historyEvents)
		}
		branchVersionHistory2 = s.eventBatchesToVersionHistory(branchVersionHistory2, branch2)

		s.applyEvents(
			workflowID,
			runID,
			workflowType,
			tasklist,
			baseVersionHistory,
			baseBranch,
			historyClient,
		)
		s.applyEvents(
			workflowID,
			runID,
			workflowType,
			tasklist,
			branchVersionHistory1,
			branch1,
			historyClient,
		)
		s.applyEvents(
			workflowID,
			runID,
			workflowType,
			tasklist,
			branchVersionHistory2,
			branch2,
			historyClient,
		)
	}
}

func (s *NDCIntegrationTestSuite) TestHandcraftedMultipleBranches() {

	s.setupRemoteFrontendClients()
	workflowID := "ndc-handcrafted-multiple-branches-test" + uuid.New()
	runID := uuid.New()

	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"
	identity := "worker-identity"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	eventsBatch1 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        1,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: tasklist},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
					FirstDecisionTaskBackoffSeconds:     common.Int32Ptr(100),
				},
			},
			{
				ID:        2,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        3,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 2,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        4,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 2,
					StartedEventID:   3,
					Identity:         identity,
				},
			},
			{
				ID:        5,
				Version:   21,
				EventType: types.EventTypeMarkerRecorded.Ptr(),
				MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{
					MarkerName:                   "some marker name",
					Details:                      []byte("some marker details"),
					DecisionTaskCompletedEventID: 4,
				},
			},
			{
				ID:        6,
				Version:   21,
				EventType: types.EventTypeActivityTaskScheduled.Ptr(),
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					DecisionTaskCompletedEventID:  4,
					ActivityID:                    "0",
					ActivityType:                  &types.ActivityType{Name: "activity-type"},
					TaskList:                      &types.TaskList{Name: tasklist},
					Input:                         nil,
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(20),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(20),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(20),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        7,
				Version:   21,
				EventType: types.EventTypeActivityTaskStarted.Ptr(),
				ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 6,
					Identity:         identity,
					RequestID:        uuid.New(),
					Attempt:          0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        8,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 1",
					Input:      []byte("some signal details 1"),
					Identity:   identity,
				},
			},
			{
				ID:        9,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        10,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 9,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        11,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
				},
			},
			{
				ID:        12,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 2",
					Input:      []byte("some signal details 2"),
					Identity:   identity,
				},
			},
			{
				ID:        13,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
			{
				ID:        14,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 13,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
	}

	eventsBatch2 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   31,
				EventType: types.EventTypeWorkflowExecutionTimedOut.Ptr(),
				WorkflowExecutionTimedOutEventAttributes: &types.WorkflowExecutionTimedOutEventAttributes{
					TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
				},
			},
		}},
	}

	eventsBatch3 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   30,
				EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
				DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
					ScheduledEventID: 13,
					StartedEventID:   14,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        16,
				Version:   30,
				EventType: types.EventTypeActivityTaskTimedOut.Ptr(),
				ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 6,
					StartedEventID:   7,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        17,
				Version:   30,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        18,
				Version:   30,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 17,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        19,
				Version:   30,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 8,
					StartedEventID:   9,
					Identity:         identity,
				},
			},
			{
				ID:        20,
				Version:   30,
				EventType: types.EventTypeWorkflowExecutionFailed.Ptr(),
				WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{
					DecisionTaskCompletedEventID: 19,
					Reason:                       common.StringPtr("some random reason"),
					Details:                      nil,
				},
			},
		}},
	}

	versionHistory1 := s.eventBatchesToVersionHistory(nil, eventsBatch1)

	versionHistory2, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory2 = s.eventBatchesToVersionHistory(versionHistory2, eventsBatch2)

	versionHistory3, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory3 = s.eventBatchesToVersionHistory(versionHistory3, eventsBatch3)

	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory1,
		eventsBatch1,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory3,
		eventsBatch3,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory2,
		eventsBatch2,
		historyClient,
	)
}

func (s *NDCIntegrationTestSuite) TestHandcraftedResetWorkflow_Zombie() {

	s.setupRemoteFrontendClients()
	workflowID := "ndc-handcrafted-reset-workflow-test" + uuid.New()
	runID := uuid.New()
	newRunID := uuid.New()

	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"
	identity := "worker-identity"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	eventsBatch1 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        1,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: tasklist},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
					FirstDecisionTaskBackoffSeconds:     common.Int32Ptr(100),
				},
			},
			{
				ID:        2,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        3,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 2,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        4,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 2,
					StartedEventID:   3,
					Identity:         identity,
				},
			},
			{
				ID:        5,
				Version:   21,
				EventType: types.EventTypeMarkerRecorded.Ptr(),
				MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{
					MarkerName:                   "some marker name",
					Details:                      []byte("some marker details"),
					DecisionTaskCompletedEventID: 4,
				},
			},
			{
				ID:        6,
				Version:   21,
				EventType: types.EventTypeActivityTaskScheduled.Ptr(),
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					DecisionTaskCompletedEventID:  4,
					ActivityID:                    "0",
					ActivityType:                  &types.ActivityType{Name: "activity-type"},
					TaskList:                      &types.TaskList{Name: tasklist},
					Input:                         nil,
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(20),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(20),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(20),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        7,
				Version:   21,
				EventType: types.EventTypeActivityTaskStarted.Ptr(),
				ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 6,
					Identity:         identity,
					RequestID:        uuid.New(),
					Attempt:          0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        8,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 1",
					Input:      []byte("some signal details 1"),
					Identity:   identity,
				},
			},
			{
				ID:        9,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        10,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 9,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        11,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
				},
			},
			{
				ID:        12,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 2",
					Input:      []byte("some signal details 2"),
					Identity:   identity,
				},
			},
			{
				ID:        13,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
			{
				ID:        14,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 13,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
	}

	eventsBatch2 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   30,
				EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
				DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
					ScheduledEventID: 13,
					StartedEventID:   14,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        16,
				Version:   30,
				EventType: types.EventTypeActivityTaskTimedOut.Ptr(),
				ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 6,
					StartedEventID:   7,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        17,
				Version:   30,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        18,
				Version:   30,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 17,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
	}

	resetCause := types.DecisionTaskFailedCauseResetWorkflow
	dtFailedReason := "events-reapplication"
	eventsBatch3 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        11,
				Version:   22,
				EventType: types.EventTypeDecisionTaskFailed.Ptr(),
				DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
					Cause:            &resetCause,
					Reason:           &dtFailedReason,
					BaseRunID:        runID,
					NewRunID:         newRunID,
					ForkEventVersion: 21,
				},
			},
		}},
	}

	versionHistory1 := s.eventBatchesToVersionHistory(nil, eventsBatch1)

	versionHistory2, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory2 = s.eventBatchesToVersionHistory(versionHistory2, eventsBatch2)

	versionHistory3, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(10, 21),
	)
	s.NoError(err)
	versionHistory3 = s.eventBatchesToVersionHistory(versionHistory3, eventsBatch3)

	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory1,
		eventsBatch1,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory2,
		eventsBatch2,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		newRunID,
		workflowType,
		tasklist,
		versionHistory3,
		eventsBatch3,
		historyClient,
	)
}
func (s *NDCIntegrationTestSuite) TestHandcraftedResetWorkflow() {

	s.setupRemoteFrontendClients()
	workflowID := "ndc-handcrafted-reset-workflow-test" + uuid.New()
	runID := uuid.New()
	newRunID := uuid.New()

	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"
	identity := "worker-identity"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	eventsBatch1 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        1,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: tasklist},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
					FirstDecisionTaskBackoffSeconds:     common.Int32Ptr(100),
				},
			},
			{
				ID:        2,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        3,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 2,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        4,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 2,
					StartedEventID:   3,
					Identity:         identity,
				},
			},
			{
				ID:        5,
				Version:   21,
				EventType: types.EventTypeMarkerRecorded.Ptr(),
				MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{
					MarkerName:                   "some marker name",
					Details:                      []byte("some marker details"),
					DecisionTaskCompletedEventID: 4,
				},
			},
			{
				ID:        6,
				Version:   21,
				EventType: types.EventTypeActivityTaskScheduled.Ptr(),
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					DecisionTaskCompletedEventID:  4,
					ActivityID:                    "0",
					ActivityType:                  &types.ActivityType{Name: "activity-type"},
					TaskList:                      &types.TaskList{Name: tasklist},
					Input:                         nil,
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(20),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(20),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(20),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        7,
				Version:   21,
				EventType: types.EventTypeActivityTaskStarted.Ptr(),
				ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 6,
					Identity:         identity,
					RequestID:        uuid.New(),
					Attempt:          0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        8,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 1",
					Input:      []byte("some signal details 1"),
					Identity:   identity,
				},
			},
			{
				ID:        9,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        10,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 9,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        11,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
				},
			},
			{
				ID:        12,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 2",
					Input:      []byte("some signal details 2"),
					Identity:   identity,
				},
			},
			{
				ID:        13,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
			{
				ID:        14,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 13,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
	}

	eventsBatch2 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   30,
				EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
				DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
					ScheduledEventID: 13,
					StartedEventID:   14,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        16,
				Version:   30,
				EventType: types.EventTypeActivityTaskTimedOut.Ptr(),
				ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 6,
					StartedEventID:   7,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        17,
				Version:   30,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        18,
				Version:   30,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 17,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        19,
				Version:   30,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 8,
					StartedEventID:   9,
					Identity:         identity,
				},
			},
			{
				ID:        20,
				Version:   30,
				EventType: types.EventTypeWorkflowExecutionFailed.Ptr(),
				WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{
					DecisionTaskCompletedEventID: 19,
					Reason:                       common.StringPtr("some random reason"),
					Details:                      nil,
				},
			},
		}},
	}

	resetCause := types.DecisionTaskFailedCauseResetWorkflow
	dtFailedReason := "events-reapplication"
	eventsBatch3 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        11,
				Version:   22,
				EventType: types.EventTypeDecisionTaskFailed.Ptr(),
				DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
					Cause:            &resetCause,
					Reason:           &dtFailedReason,
					BaseRunID:        runID,
					NewRunID:         newRunID,
					ForkEventVersion: 21,
				},
			},
		}},
	}

	versionHistory1 := s.eventBatchesToVersionHistory(nil, eventsBatch1)

	versionHistory2, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory2 = s.eventBatchesToVersionHistory(versionHistory2, eventsBatch2)

	versionHistory3, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(10, 21),
	)
	s.NoError(err)
	versionHistory3 = s.eventBatchesToVersionHistory(versionHistory3, eventsBatch3)

	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory1,
		eventsBatch1,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory2,
		eventsBatch2,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		newRunID,
		workflowType,
		tasklist,
		versionHistory3,
		eventsBatch3,
		historyClient,
	)
}

func (s *NDCIntegrationTestSuite) TestHandcraftedMultipleBranchesWithZombieContinueAsNew() {

	s.setupRemoteFrontendClients()
	workflowID := "ndc-handcrafted-multiple-branches-with-continue-as-new-test" + uuid.New()
	runID := uuid.New()

	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"
	identity := "worker-identity"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	eventsBatch1 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        1,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: tasklist},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
					FirstDecisionTaskBackoffSeconds:     common.Int32Ptr(100),
				},
			},
			{
				ID:        2,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        3,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 2,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        4,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 2,
					StartedEventID:   3,
					Identity:         identity,
				},
			},
			{
				ID:        5,
				Version:   21,
				EventType: types.EventTypeMarkerRecorded.Ptr(),
				MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{
					MarkerName:                   "some marker name",
					Details:                      []byte("some marker details"),
					DecisionTaskCompletedEventID: 4,
				},
			},
			{
				ID:        6,
				Version:   21,
				EventType: types.EventTypeActivityTaskScheduled.Ptr(),
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					DecisionTaskCompletedEventID:  4,
					ActivityID:                    "0",
					ActivityType:                  &types.ActivityType{Name: "activity-type"},
					TaskList:                      &types.TaskList{Name: tasklist},
					Input:                         nil,
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(20),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(20),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(20),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        7,
				Version:   21,
				EventType: types.EventTypeActivityTaskStarted.Ptr(),
				ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 6,
					Identity:         identity,
					RequestID:        uuid.New(),
					Attempt:          0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        8,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 1",
					Input:      []byte("some signal details 1"),
					Identity:   identity,
				},
			},
			{
				ID:        9,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        10,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 9,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        11,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
				},
			},
			{
				ID:        12,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 2",
					Input:      []byte("some signal details 2"),
					Identity:   identity,
				},
			},
			{
				ID:        13,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
			{
				ID:        14,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 13,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
	}

	eventsBatch2 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   32,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 8,
					StartedEventID:   9,
					Identity:         identity,
				},
			},
		}},
		// need to keep the workflow open for testing
	}

	eventsBatch3 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 8,
					StartedEventID:   9,
					Identity:         identity,
				},
			},
			{
				ID:        16,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
				WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
					NewExecutionRunID:                   uuid.New(),
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: tasklist},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
					DecisionTaskCompletedEventID:        19,
					Initiator:                           types.ContinueAsNewInitiatorDecider.Ptr(),
				},
			},
		}},
	}

	versionHistory1 := s.eventBatchesToVersionHistory(nil, eventsBatch1)

	versionHistory2, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory2 = s.eventBatchesToVersionHistory(versionHistory2, eventsBatch2)

	versionHistory3, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory3 = s.eventBatchesToVersionHistory(versionHistory3, eventsBatch3)

	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory1,
		eventsBatch1,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory2,
		eventsBatch2,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory3,
		eventsBatch3,
		historyClient,
	)
}

func (s *NDCIntegrationTestSuite) TestEventsReapply_ZombieWorkflow() {

	workflowID := "ndc-single-branch-test" + uuid.New()

	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	version := int64(101)
	runID := uuid.New()
	historyBatch := []*types.History{}
	s.generator = test.InitializeHistoryEventGenerator(s.domainName, version)

	for s.generator.HasNextVertex() {
		events := s.generator.GetNextVertices()
		historyEvents := &types.History{}
		for _, event := range events {
			historyEvents.Events = append(historyEvents.Events, event.GetData().(*types.HistoryEvent))
		}
		historyBatch = append(historyBatch, historyEvents)
	}

	versionHistory := s.eventBatchesToVersionHistory(nil, historyBatch)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory,
		historyBatch,
		historyClient,
	)

	version = int64(1)
	runID = uuid.New()
	historyBatch = []*types.History{}
	s.generator = test.InitializeHistoryEventGenerator(s.domainName, version)

	// verify two batches of zombie workflow are call reapply API
	s.mockAdminClient["standby"].(*adminClient.MockClient).EXPECT().ReapplyEvents(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	for i := 0; i < 2 && s.generator.HasNextVertex(); i++ {
		events := s.generator.GetNextVertices()
		historyEvents := &types.History{}
		for _, event := range events {
			historyEvents.Events = append(historyEvents.Events, event.GetData().(*types.HistoryEvent))
		}
		historyBatch = append(historyBatch, historyEvents)
	}

	versionHistory = s.eventBatchesToVersionHistory(nil, historyBatch)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory,
		historyBatch,
		historyClient,
	)
}

func (s *NDCIntegrationTestSuite) TestEventsReapply_UpdateNonCurrentBranch() {

	workflowID := "ndc-single-branch-test" + uuid.New()
	runID := uuid.New()
	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"
	version := int64(101)
	isWorkflowFinished := false

	historyClient := s.active.GetHistoryClient()

	s.generator = test.InitializeHistoryEventGenerator(s.domainName, version)
	baseBranch := []*types.History{}
	var taskID int64
	for i := 0; i < 4 && s.generator.HasNextVertex(); i++ {
		events := s.generator.GetNextVertices()
		historyEvents := &types.History{}
		for _, event := range events {
			historyEvent := event.GetData().(*types.HistoryEvent)
			taskID = historyEvent.TaskID
			historyEvents.Events = append(historyEvents.Events, historyEvent)
			switch historyEvent.GetEventType() {
			case types.EventTypeWorkflowExecutionCompleted,
				types.EventTypeWorkflowExecutionFailed,
				types.EventTypeWorkflowExecutionTimedOut,
				types.EventTypeWorkflowExecutionTerminated,
				types.EventTypeWorkflowExecutionContinuedAsNew,
				types.EventTypeWorkflowExecutionCanceled:
				isWorkflowFinished = true
			}
		}
		baseBranch = append(baseBranch, historyEvents)
	}
	if isWorkflowFinished {
		// cannot proceed since the test below requires workflow not finished
		// this is ok since build kite will run this test several times
		s.logger.Info("Encounter finish workflow history event during randomization test, skip")
		return
	}

	versionHistory := s.eventBatchesToVersionHistory(nil, baseBranch)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory,
		baseBranch,
		historyClient,
	)

	newGenerator := s.generator.DeepCopy()
	newBranch := []*types.History{}
	newVersionHistory := versionHistory.Duplicate()
	newGenerator.SetVersion(newGenerator.GetVersion() + 1) // simulate events from other cluster
	for i := 0; i < 4 && newGenerator.HasNextVertex(); i++ {
		events := newGenerator.GetNextVertices()
		historyEvents := &types.History{}
		for _, event := range events {
			history := event.GetData().(*types.HistoryEvent)
			taskID = history.TaskID
			historyEvents.Events = append(historyEvents.Events, history)
		}
		newBranch = append(newBranch, historyEvents)
	}
	newVersionHistory = s.eventBatchesToVersionHistory(newVersionHistory, newBranch)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		newVersionHistory,
		newBranch,
		historyClient,
	)

	s.mockAdminClient["standby"].(*adminClient.MockClient).EXPECT().ReapplyEvents(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	// Handcraft a stale signal event
	baseBranchLastEventBatch := baseBranch[len(baseBranch)-1].GetEvents()
	baseBranchLastEvent := baseBranchLastEventBatch[len(baseBranchLastEventBatch)-1]
	staleBranch := []*types.History{
		{
			Events: []*types.HistoryEvent{
				{
					ID:        baseBranchLastEvent.ID + 1,
					EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
					Timestamp: common.Int64Ptr(time.Now().UnixNano()),
					Version:   baseBranchLastEvent.Version, // dummy event from other cluster
					TaskID:    taskID,
					WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
						SignalName: "signal",
						Input:      []byte{},
						Identity:   "ndc_integration_test",
					},
				},
			},
		},
	}
	staleVersionHistory := s.eventBatchesToVersionHistory(versionHistory.Duplicate(), staleBranch)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		staleVersionHistory,
		staleBranch,
		historyClient,
	)
}

func (s *NDCIntegrationTestSuite) TestAdminGetWorkflowExecutionRawHistoryV2() {

	workflowID := "ndc-re-send-test" + uuid.New()
	runID := uuid.New()
	workflowType := "ndc-re-send-workflow-type"
	tasklist := "event-generator-taskList"
	identity := "ndc-re-send-test"

	historyClient := s.active.GetHistoryClient()
	adminClient := s.active.GetAdminClient()
	getHistory := func(
		domain string,
		workflowID string,
		runID string,
		startEventID *int64,
		startEventVersion *int64,
		endEventID *int64,
		endEventVersion *int64,
		pageSize int,
		token []byte,
	) (*types.GetWorkflowExecutionRawHistoryV2Response, error) {

		execution := &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		}
		ctx, cancel := s.createContext()
		defer cancel()
		return adminClient.GetWorkflowExecutionRawHistoryV2(ctx, &types.GetWorkflowExecutionRawHistoryV2Request{
			Domain:            domain,
			Execution:         execution,
			StartEventID:      startEventID,
			StartEventVersion: startEventVersion,
			EndEventID:        endEventID,
			EndEventVersion:   endEventVersion,
			MaximumPageSize:   int32(pageSize),
			NextPageToken:     token,
		})
	}

	eventsBatch1 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        1,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: tasklist},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
					FirstDecisionTaskBackoffSeconds:     common.Int32Ptr(100),
				},
			},
			{
				ID:        2,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        3,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 2,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        4,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 2,
					StartedEventID:   3,
					Identity:         identity,
				},
			},
			{
				ID:        5,
				Version:   21,
				EventType: types.EventTypeMarkerRecorded.Ptr(),
				MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{
					MarkerName:                   "some marker name",
					Details:                      []byte("some marker details"),
					DecisionTaskCompletedEventID: 4,
				},
			},
			{
				ID:        6,
				Version:   21,
				EventType: types.EventTypeActivityTaskScheduled.Ptr(),
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					DecisionTaskCompletedEventID:  4,
					ActivityID:                    "0",
					ActivityType:                  &types.ActivityType{Name: "activity-type"},
					TaskList:                      &types.TaskList{Name: tasklist},
					Input:                         nil,
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(20),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(20),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(20),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        7,
				Version:   21,
				EventType: types.EventTypeActivityTaskStarted.Ptr(),
				ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
					ScheduledEventID: 6,
					Identity:         identity,
					RequestID:        uuid.New(),
					Attempt:          0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        8,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 1",
					Input:      []byte("some signal details 1"),
					Identity:   identity,
				},
			},
			{
				ID:        9,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        10,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 9,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        11,
				Version:   21,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
				},
			},
			{
				ID:        12,
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					SignalName: "some signal name 2",
					Input:      []byte("some signal details 2"),
					Identity:   identity,
				},
			},
			{
				ID:        13,
				Version:   21,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
			{
				ID:        14,
				Version:   21,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 13,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
	}

	eventsBatch2 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   31,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 9,
					StartedEventID:   10,
					Identity:         identity,
				},
			},
			{
				ID:        16,
				Version:   31,
				EventType: types.EventTypeActivityTaskScheduled.Ptr(),
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					DecisionTaskCompletedEventID:  4,
					ActivityID:                    "0",
					ActivityType:                  &types.ActivityType{Name: "activity-type"},
					TaskList:                      &types.TaskList{Name: tasklist},
					Input:                         nil,
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(20),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(20),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(20),
				},
			},
		}},
	}

	eventsBatch3 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        15,
				Version:   30,
				EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
				DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
					ScheduledEventID: 13,
					StartedEventID:   14,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        16,
				Version:   30,
				EventType: types.EventTypeActivityTaskTimedOut.Ptr(),
				ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
					ScheduledEventID: 6,
					StartedEventID:   7,
					TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
				},
			},
			{
				ID:        17,
				Version:   30,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
					TaskList:                   &types.TaskList{Name: tasklist},
					StartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					Attempt:                    0,
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        18,
				Version:   30,
				EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
					ScheduledEventID: 17,
					Identity:         identity,
					RequestID:        uuid.New(),
				},
			},
		}},
		{Events: []*types.HistoryEvent{
			{
				ID:        19,
				Version:   30,
				EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ScheduledEventID: 8,
					StartedEventID:   9,
					Identity:         identity,
				},
			},
			{
				ID:        20,
				Version:   30,
				EventType: types.EventTypeWorkflowExecutionFailed.Ptr(),
				WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{
					DecisionTaskCompletedEventID: 19,
					Reason:                       common.StringPtr("some random reason"),
					Details:                      nil,
				},
			},
		}},
	}

	eventsBatch4 := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        17,
				Version:   32,
				EventType: types.EventTypeWorkflowExecutionTimedOut.Ptr(),
				WorkflowExecutionTimedOutEventAttributes: &types.WorkflowExecutionTimedOutEventAttributes{
					TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
				},
			},
		}},
	}

	versionHistory1 := s.eventBatchesToVersionHistory(nil, eventsBatch1)

	versionHistory2, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory2 = s.eventBatchesToVersionHistory(versionHistory2, eventsBatch2)

	versionHistory3, err := versionHistory1.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(14, 21),
	)
	s.NoError(err)
	versionHistory3 = s.eventBatchesToVersionHistory(versionHistory3, eventsBatch3)

	versionHistory4, err := versionHistory2.DuplicateUntilLCAItem(
		persistence.NewVersionHistoryItem(16, 31),
	)
	s.NoError(err)
	versionHistory4 = s.eventBatchesToVersionHistory(versionHistory4, eventsBatch4)

	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory1,
		eventsBatch1,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory3,
		eventsBatch3,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory2,
		eventsBatch2,
		historyClient,
	)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory4,
		eventsBatch4,
		historyClient,
	)

	// GetWorkflowExecutionRawHistoryV2 start and end
	var token []byte
	batchCount := 0
	for continuePaging := true; continuePaging; continuePaging = len(token) != 0 {
		resp, err := getHistory(
			s.domainName,
			workflowID,
			runID,
			common.Int64Ptr(14),
			common.Int64Ptr(21),
			common.Int64Ptr(20),
			common.Int64Ptr(30),
			1,
			token,
		)
		s.Nil(err)
		s.True(len(resp.HistoryBatches) <= 1)
		batchCount++
		token = resp.NextPageToken
	}
	s.Equal(batchCount, 4)

	// GetWorkflowExecutionRawHistoryV2 start and end not on the same branch
	token = nil
	batchCount = 0
	for continuePaging := true; continuePaging; continuePaging = len(token) != 0 {
		resp, err := getHistory(
			s.domainName,
			workflowID,
			runID,
			common.Int64Ptr(17),
			common.Int64Ptr(30),
			common.Int64Ptr(17),
			common.Int64Ptr(32),
			1,
			token,
		)
		s.Nil(err)
		s.True(len(resp.HistoryBatches) <= 1)
		batchCount++
		token = resp.NextPageToken
	}
	s.Equal(batchCount, 2)

	// GetWorkflowExecutionRawHistoryV2 start boundary
	token = nil
	batchCount = 0
	for continuePaging := true; continuePaging; continuePaging = len(token) != 0 {
		resp, err := getHistory(
			s.domainName,
			workflowID,
			runID,
			common.Int64Ptr(14),
			common.Int64Ptr(21),
			nil,
			nil,
			1,
			token,
		)
		s.Nil(err)
		s.True(len(resp.HistoryBatches) <= 1)
		batchCount++
		token = resp.NextPageToken
	}
	s.Equal(batchCount, 3)

	// GetWorkflowExecutionRawHistoryV2 end boundary
	token = nil
	batchCount = 0
	for continuePaging := true; continuePaging; continuePaging = len(token) != 0 {
		resp, err := getHistory(
			s.domainName,
			workflowID,
			runID,
			nil,
			nil,
			common.Int64Ptr(17),
			common.Int64Ptr(32),
			1,
			token,
		)
		s.Nil(err)
		s.True(len(resp.HistoryBatches) <= 1)
		batchCount++
		token = resp.NextPageToken
	}
	s.Equal(batchCount, 10)
}

func (s *NDCIntegrationTestSuite) TestWorkflowStartTime() {
	s.setupRemoteFrontendClients()
	workflowID := "ndc-workflow-start-time-test" + uuid.New()

	workflowType := "start-time-test-workflow-type"
	tasklist := "start-time-test-tasklist"

	startTime := time.Now().Add(-time.Minute)
	runID := uuid.New()

	historyBatch := []*types.History{
		{Events: []*types.HistoryEvent{
			{
				ID:        1,
				Timestamp: common.Int64Ptr(startTime.UnixNano()),
				Version:   21,
				EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: tasklist},
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
					FirstDecisionTaskBackoffSeconds:     common.Int32Ptr(100),
				},
			},
		},
		},
	}

	versionHistory := s.eventBatchesToVersionHistory(nil, historyBatch)
	s.applyEvents(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory,
		historyBatch,
		s.active.GetHistoryClient(),
	)

	err := s.verifyEventHistory(workflowID, runID, historyBatch)
	s.Require().NoError(err)

	// we are replicating to the `active` cluster, check the comments in
	// registerDomain() method below
	ctx, cancel := s.createContext()
	defer cancel()
	descResp, err := s.active.GetFrontendClient().DescribeWorkflowExecution(
		ctx,
		&types.DescribeWorkflowExecutionRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
		},
	)
	s.NoError(err)
	s.WithinDuration(
		startTime,
		time.Unix(0, descResp.WorkflowExecutionInfo.GetStartTime()),
		time.Millisecond,
	)
}

func (s *NDCIntegrationTestSuite) RegisterDomain() {
	s.domainName = "test-simple-workflow-ndc-" + common.GenerateRandomString(5)
	client1 := s.active.GetFrontendClient() // active
	ctx, cancel := s.createContext()
	defer cancel()
	err := client1.RegisterDomain(ctx, &types.RegisterDomainRequest{
		Name:           s.domainName,
		IsGlobalDomain: true,
		Clusters:       clusterReplicationConfig,
		// make the active cluster `standby` and replicate to `active` cluster
		ActiveClusterName:                      clusterName[1],
		WorkflowExecutionRetentionPeriodInDays: 1,
	})
	s.Require().NoError(err)

	descReq := &types.DescribeDomainRequest{
		Name: common.StringPtr(s.domainName),
	}
	ctx, cancel = s.createContext()
	defer cancel()
	resp, err := client1.DescribeDomain(ctx, descReq)
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.domainID = resp.GetDomainInfo().GetUUID()
	// Wait for domain cache to pick the change
	time.Sleep(2 * cache.DomainCacheRefreshInterval)

	s.logger.Info(fmt.Sprintf("Domain name: %v - ID: %v", s.domainName, s.domainID))
}

func (s *NDCIntegrationTestSuite) generateNewRunHistory(
	event *types.HistoryEvent,
	domain string,
	workflowID string,
	runID string,
	version int64,
	workflowType string,
	taskList string,
) *persistence.DataBlob {

	// TODO temporary code to generate first event & version history
	//  we should generate these as part of modeled based testing

	if event.GetWorkflowExecutionContinuedAsNewEventAttributes() == nil {
		return nil
	}

	event.WorkflowExecutionContinuedAsNewEventAttributes.NewExecutionRunID = uuid.New()

	firstScheduleTime := time.Unix(0, 100)
	newRunFirstEvent := &types.HistoryEvent{
		ID:        constants.FirstEventID,
		Timestamp: common.Int64Ptr(time.Now().UnixNano()),
		EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
		Version:   version,
		TaskID:    1,
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
			WorkflowType:         &types.WorkflowType{Name: workflowType},
			ParentWorkflowDomain: common.StringPtr(domain),
			ParentWorkflowExecution: &types.WorkflowExecution{
				WorkflowID: uuid.New(),
				RunID:      uuid.New(),
			},
			ParentInitiatedEventID: common.Int64Ptr(event.ID),
			TaskList: &types.TaskList{
				Name: taskList,
				Kind: types.TaskListKindNormal.Ptr(),
			},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(10),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
			ContinuedExecutionRunID:             runID,
			Initiator:                           types.ContinueAsNewInitiatorCronSchedule.Ptr(),
			OriginalExecutionRunID:              runID,
			Identity:                            "NDC-test",
			FirstExecutionRunID:                 runID,
			FirstScheduleTime:                   &firstScheduleTime,
			Attempt:                             0,
			ExpirationTimestamp:                 common.Int64Ptr(time.Now().Add(time.Minute).UnixNano()),
		},
	}

	eventBlob, err := s.serializer.SerializeBatchEvents([]*types.HistoryEvent{newRunFirstEvent}, constants.EncodingTypeThriftRW)
	s.NoError(err)

	return eventBlob
}

func (s *NDCIntegrationTestSuite) toInternalDataBlob(
	blob *persistence.DataBlob,
) *types.DataBlob {

	if blob == nil {
		return nil
	}

	var encodingType types.EncodingType
	switch blob.GetEncoding() {
	case constants.EncodingTypeThriftRW:
		encodingType = types.EncodingTypeThriftRW
	case constants.EncodingTypeJSON,
		constants.EncodingTypeGob,
		constants.EncodingTypeUnknown,
		constants.EncodingTypeEmpty:
		panic(fmt.Sprintf("unsupported encoding type: %v", blob.GetEncoding()))
	default:
		panic(fmt.Sprintf("unknown encoding type: %v", blob.GetEncoding()))
	}

	return &types.DataBlob{
		EncodingType: encodingType.Ptr(),
		Data:         blob.Data,
	}
}

func (s *NDCIntegrationTestSuite) generateEventBlobs(
	workflowID string,
	runID string,
	workflowType string,
	tasklist string,
	batch *types.History,
) (*persistence.DataBlob, *persistence.DataBlob) {
	// TODO temporary code to generate next run first event
	//  we should generate these as part of modeled based testing
	lastEvent := batch.Events[len(batch.Events)-1]
	newRunEventBlob := s.generateNewRunHistory(
		lastEvent, s.domainName, workflowID, runID, lastEvent.Version, workflowType, tasklist,
	)
	// must serialize events batch after attempt on continue as new as generateNewRunHistory will
	// modify the NewExecutionRunID attr
	eventBlob, err := s.serializer.SerializeBatchEvents(batch.Events, constants.EncodingTypeThriftRW)
	s.NoError(err)
	return eventBlob, newRunEventBlob
}

func (s *NDCIntegrationTestSuite) applyEvents(
	workflowID string,
	runID string,
	workflowType string,
	tasklist string,
	versionHistory *persistence.VersionHistory,
	eventBatches []*types.History,
	historyClient host.HistoryClient,
) {
	for _, batch := range eventBatches {
		eventBlob, newRunEventBlob := s.generateEventBlobs(workflowID, runID, workflowType, tasklist, batch)
		req := &types.ReplicateEventsV2Request{
			DomainUUID: s.domainID,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
			VersionHistoryItems: s.toInternalVersionHistoryItems(versionHistory),
			Events:              s.toInternalDataBlob(eventBlob),
			NewRunEvents:        s.toInternalDataBlob(newRunEventBlob),
		}

		ctx, cancel := s.createContext()
		err := historyClient.ReplicateEventsV2(ctx, req)
		cancel()
		s.Nil(err, "Failed to replicate history event")
		ctx, cancel = s.createContext()
		err = historyClient.ReplicateEventsV2(ctx, req)
		cancel()
		s.Nil(err, "Failed to dedup replicate history event")
	}
}

func (s *NDCIntegrationTestSuite) applyEventsThroughFetcher(
	workflowID string,
	runID string,
	workflowType string,
	tasklist string,
	versionHistory *persistence.VersionHistory,
	eventBatches []*types.History,
) {
	for _, batch := range eventBatches {
		eventBlob, newRunEventBlob := s.generateEventBlobs(workflowID, runID, workflowType, tasklist, batch)

		taskType := types.ReplicationTaskTypeHistoryV2
		replicationTask := &types.ReplicationTask{
			TaskType:     &taskType,
			SourceTaskID: 1,
			HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
				DomainID:            s.domainID,
				WorkflowID:          workflowID,
				RunID:               runID,
				VersionHistoryItems: s.toInternalVersionHistoryItems(versionHistory),
				Events:              s.toInternalDataBlob(eventBlob),
				NewRunEvents:        s.toInternalDataBlob(newRunEventBlob),
			},
		}

		s.standByReplicationTasksChan <- replicationTask
		// this is to test whether dedup works
		s.standByReplicationTasksChan <- replicationTask
	}
}

func (s *NDCIntegrationTestSuite) eventBatchesToVersionHistory(
	versionHistory *persistence.VersionHistory,
	eventBatches []*types.History,
) *persistence.VersionHistory {

	// TODO temporary code to generate version history
	//  we should generate version as part of modeled based testing
	if versionHistory == nil {
		versionHistory = persistence.NewVersionHistory(nil, nil)
	}
	for _, batch := range eventBatches {
		for _, event := range batch.Events {
			err := versionHistory.AddOrUpdateItem(
				persistence.NewVersionHistoryItem(
					event.ID,
					event.Version,
				))
			s.NoError(err)
		}
	}

	return versionHistory
}

func (s *NDCIntegrationTestSuite) toInternalVersionHistoryItems(
	versionHistory *persistence.VersionHistory,
) []*types.VersionHistoryItem {
	if versionHistory == nil {
		return nil
	}

	return versionHistory.ToInternalType().Items
}

func (s *NDCIntegrationTestSuite) createContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	return ctx, cancel
}

func (s *NDCIntegrationTestSuite) setupRemoteFrontendClients() {
	s.mockAdminClient["standby"].(*adminClient.MockClient).EXPECT().ReapplyEvents(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.mockAdminClient["other"].(*adminClient.MockClient).EXPECT().ReapplyEvents(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}
