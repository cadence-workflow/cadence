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
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/uber-common/bark"

	"github.com/uber-go/tally"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/logging"
	"github.com/uber/cadence/common/metrics"
)

const (
	testWorkflowClusterHosts = "127.0.0.1"
	testPort                 = 0
	testUser                 = ""
	testPassword             = ""
	testDatacenter           = ""
	testSchemaDir            = "../.."
)

type (
	// TestBaseOptions options to configure workflow test base.
	TestBaseOptions struct {
		ClusterHost     string
		ClusterPort     int
		ClusterUser     string
		ClusterPassword string
		KeySpace        string
		Datacenter      string
		DropKeySpace    bool
		SchemaDir       string
	}

	// TestBase wraps the base setup needed to create workflows over engine layer.
	TestBase struct {
		ShardMgr            ShardManager
		ExecutionMgrFactory ExecutionManagerFactory
		WorkflowMgr         ExecutionManager
		TaskMgr             TaskManager
		HistoryMgr          HistoryManager
		MetadataManager     MetadataManager
		VisibilityMgr       VisibilityManager
		ShardInfo           *ShardInfo
		ShardContext        *TestShardContext
		readLevel           int64
		CassandraTestCluster
	}

	// CassandraTestCluster allows executing cassandra operations in testing.
	CassandraTestCluster struct {
		keyspace string
		cluster  *gocql.ClusterConfig
		session  *gocql.Session
	}

	// TestShardContext shard context for testing.
	// TODO: Cleanup, move this out of persistence
	TestShardContext struct {
		sync.RWMutex
		shardInfo              *ShardInfo
		transferSequenceNumber int64
		historyMgr             HistoryManager
		executionMgr           ExecutionManager
		logger                 bark.Logger
		metricsClient          metrics.Client
	}

	testExecutionMgrFactory struct {
		options   TestBaseOptions
		cassandra CassandraTestCluster
		logger    bark.Logger
	}
)

func newTestShardContext(shardInfo *ShardInfo, transferSequenceNumber int64, historyMgr HistoryManager,
	executionMgr ExecutionManager, logger bark.Logger) *TestShardContext {
	return &TestShardContext{
		shardInfo:              shardInfo,
		transferSequenceNumber: transferSequenceNumber,
		historyMgr:             historyMgr,
		executionMgr:           executionMgr,
		logger:                 logger,
		metricsClient:          metrics.NewClient(tally.NoopScope, metrics.History),
	}
}

// GetExecutionManager test implementation
func (s *TestShardContext) GetExecutionManager() ExecutionManager {
	return s.executionMgr
}

// GetHistoryManager test implementation
func (s *TestShardContext) GetHistoryManager() HistoryManager {
	return s.historyMgr
}

// GetNextTransferTaskID test implementation
func (s *TestShardContext) GetNextTransferTaskID() (int64, error) {
	return atomic.AddInt64(&s.transferSequenceNumber, 1), nil
}

// GetTransferMaxReadLevel test implementation
func (s *TestShardContext) GetTransferMaxReadLevel() int64 {
	return atomic.LoadInt64(&s.transferSequenceNumber)
}

// GetTransferAckLevel test implementation
func (s *TestShardContext) GetTransferAckLevel() int64 {
	return atomic.LoadInt64(&s.shardInfo.TransferAckLevel)
}

// UpdateTransferAckLevel test implementation
func (s *TestShardContext) UpdateTransferAckLevel(ackLevel int64) error {
	atomic.StoreInt64(&s.shardInfo.TransferAckLevel, ackLevel)
	return nil
}

// GetTransferSequenceNumber test implementation
func (s *TestShardContext) GetTransferSequenceNumber() int64 {
	return atomic.LoadInt64(&s.transferSequenceNumber)
}

// GetTimerAckLevel test implementation
func (s *TestShardContext) GetTimerAckLevel() time.Time {
	s.RLock()
	defer s.RLock()
	return s.shardInfo.TimerAckLevel
}

// UpdateTimerAckLevel test implementation
func (s *TestShardContext) UpdateTimerAckLevel(ackLevel time.Time) error {
	s.Lock()
	defer s.Unlock()
	s.shardInfo.TimerAckLevel = ackLevel
	return nil
}

// CreateWorkflowExecution test implementation
func (s *TestShardContext) CreateWorkflowExecution(request *CreateWorkflowExecutionRequest) (
	*CreateWorkflowExecutionResponse, error) {
	return s.executionMgr.CreateWorkflowExecution(request)
}

// UpdateWorkflowExecution test implementation
func (s *TestShardContext) UpdateWorkflowExecution(request *UpdateWorkflowExecutionRequest) error {
	// assign IDs for the timer tasks. They need to be assigned under shard lock.
	// TODO: This needs to be moved out of persistence.
	for _, task := range request.TimerTasks {
		seqID, err := s.GetNextTransferTaskID()
		if err != nil {
			panic(err)
		}
		task.SetTaskID(seqID)
		s.logger.Infof("%v: TestShardContext: Assigning timer (timestamp: %v, seq: %v)",
			time.Now().UTC(), GetVisibilityTSFrom(task).UTC(), task.GetTaskID())
	}
	return s.executionMgr.UpdateWorkflowExecution(request)
}

// AppendHistoryEvents test implementation
func (s *TestShardContext) AppendHistoryEvents(request *AppendHistoryEventsRequest) error {
	return s.historyMgr.AppendHistoryEvents(request)
}

// GetLogger test implementation
func (s *TestShardContext) GetLogger() bark.Logger {
	return s.logger
}

// GetMetricsClient test implementation
func (s *TestShardContext) GetMetricsClient() metrics.Client {
	return s.metricsClient
}

// Reset test implementation
func (s *TestShardContext) Reset() {
	atomic.StoreInt64(&s.shardInfo.RangeID, 0)
	atomic.StoreInt64(&s.shardInfo.TransferAckLevel, 0)
}

// GetRangeID test implementation
func (s *TestShardContext) GetRangeID() int64 {
	return atomic.LoadInt64(&s.shardInfo.RangeID)
}

// GetTimeSource test implementation
func (s *TestShardContext) GetTimeSource() common.TimeSource {
	return common.NewRealTimeSource()
}

func newTestExecutionMgrFactory(options TestBaseOptions, cassandra CassandraTestCluster,
	logger bark.Logger) ExecutionManagerFactory {
	return &testExecutionMgrFactory{
		options:   options,
		cassandra: cassandra,
		logger:    logger,
	}
}

func (f *testExecutionMgrFactory) CreateExecutionManager(shardID int) (ExecutionManager, error) {
	return NewCassandraWorkflowExecutionPersistence(f.options.ClusterHost, f.options.ClusterPort, f.options.ClusterUser,
		f.options.ClusterPassword, f.options.Datacenter, f.cassandra.keyspace,
		shardID, f.logger)
}

// SetupWorkflowStoreWithOptions to setup workflow test base
func (s *TestBase) SetupWorkflowStoreWithOptions(options TestBaseOptions) {
	log := bark.NewLoggerFromLogrus(log.New())
	// Setup Workflow keyspace and deploy schema for tests
	s.CassandraTestCluster.setupTestCluster(options.KeySpace, options.DropKeySpace, options.SchemaDir)
	shardID := 0
	var err error
	s.ShardMgr, err = NewCassandraShardPersistence(options.ClusterHost, options.ClusterPort, options.ClusterUser,
		options.ClusterPassword, options.Datacenter, s.CassandraTestCluster.keyspace, log)
	if err != nil {
		log.Fatal(err)
	}
	s.ExecutionMgrFactory = newTestExecutionMgrFactory(options, s.CassandraTestCluster, log)
	// Create an ExecutionManager for the shard for use in unit tests
	s.WorkflowMgr, err = s.ExecutionMgrFactory.CreateExecutionManager(shardID)
	if err != nil {
		log.Fatal(err)
	}
	s.TaskMgr, err = NewCassandraTaskPersistence(options.ClusterHost, options.ClusterPort, options.ClusterUser,
		options.ClusterPassword, options.Datacenter, s.CassandraTestCluster.keyspace,
		log)
	if err != nil {
		log.Fatal(err)
	}

	s.HistoryMgr, err = NewCassandraHistoryPersistence(options.ClusterHost, options.ClusterPort, options.ClusterUser,
		options.ClusterPassword, options.Datacenter, s.CassandraTestCluster.keyspace, log)
	if err != nil {
		log.Fatal(err)
	}

	s.MetadataManager, err = NewCassandraMetadataPersistence(options.ClusterHost, options.ClusterPort, options.ClusterUser,
		options.ClusterPassword, options.Datacenter, s.CassandraTestCluster.keyspace, log)
	if err != nil {
		log.Fatal(err)
	}

	s.VisibilityMgr, err = NewCassandraVisibilityPersistence(options.ClusterHost, options.ClusterPort,
		options.ClusterUser, options.ClusterPassword, options.Datacenter, s.CassandraTestCluster.keyspace, log)
	if err != nil {
		log.Fatal(err)
	}

	// Create a shard for test
	s.readLevel = 0
	s.ShardInfo = &ShardInfo{
		ShardID:          shardID,
		RangeID:          0,
		TransferAckLevel: 0,
	}
	s.ShardContext = newTestShardContext(s.ShardInfo, 0, s.HistoryMgr, s.WorkflowMgr, log)
	err1 := s.ShardMgr.CreateShard(&CreateShardRequest{
		ShardInfo: s.ShardInfo,
	})
	if err1 != nil {
		log.Fatal(err1)
	}
}

// CreateShard is a utility method to create the shard using persistence layer
func (s *TestBase) CreateShard(shardID int, owner string, rangeID int64) error {
	info := &ShardInfo{
		ShardID: shardID,
		Owner:   owner,
		RangeID: rangeID,
	}

	return s.ShardMgr.CreateShard(&CreateShardRequest{
		ShardInfo: info,
	})
}

// GetShard is a utility method to get the shard using persistence layer
func (s *TestBase) GetShard(shardID int) (*ShardInfo, error) {
	response, err := s.ShardMgr.GetShard(&GetShardRequest{
		ShardID: shardID,
	})

	if err != nil {
		return nil, err
	}

	return response.ShardInfo, nil
}

// UpdateShard is a utility method to update the shard using persistence layer
func (s *TestBase) UpdateShard(updatedInfo *ShardInfo, previousRangeID int64) error {
	return s.ShardMgr.UpdateShard(&UpdateShardRequest{
		ShardInfo:       updatedInfo,
		PreviousRangeID: previousRangeID,
	})
}

// CreateWorkflowExecution is a utility method to create workflow executions
func (s *TestBase) CreateWorkflowExecution(domainID string, workflowExecution workflow.WorkflowExecution, taskList,
	wType string, decisionTimeout int32, executionContext []byte, nextEventID int64, lastProcessedEventID int64,
	decisionScheduleID int64, timerTasks []Task) (string, error) {
	response, err := s.WorkflowMgr.CreateWorkflowExecution(&CreateWorkflowExecutionRequest{
		RequestID:            uuid.New(),
		DomainID:             domainID,
		Execution:            workflowExecution,
		TaskList:             taskList,
		WorkflowTypeName:     wType,
		DecisionTimeoutValue: decisionTimeout,
		ExecutionContext:     executionContext,
		NextEventID:          nextEventID,
		LastProcessedEvent:   lastProcessedEventID,
		RangeID:              s.ShardContext.GetRangeID(),
		TransferTasks: []Task{
			&DecisionTask{
				TaskID:     s.GetNextSequenceNumber(),
				DomainID:   domainID,
				TaskList:   taskList,
				ScheduleID: decisionScheduleID,
			},
		},
		TimerTasks:                  timerTasks,
		DecisionScheduleID:          decisionScheduleID,
		DecisionStartedID:           common.EmptyEventID,
		DecisionStartToCloseTimeout: 1,
	})

	if err != nil {
		return "", err
	}

	return response.TaskID, nil
}

// CreateWorkflowExecutionManyTasks is a utility method to create workflow executions
func (s *TestBase) CreateWorkflowExecutionManyTasks(domainID string, workflowExecution workflow.WorkflowExecution,
	taskList string, executionContext []byte, nextEventID int64, lastProcessedEventID int64,
	decisionScheduleIDs []int64, activityScheduleIDs []int64) (string, error) {

	transferTasks := []Task{}
	for _, decisionScheduleID := range decisionScheduleIDs {
		transferTasks = append(transferTasks,
			&DecisionTask{
				TaskID:     s.GetNextSequenceNumber(),
				DomainID:   domainID,
				TaskList:   taskList,
				ScheduleID: int64(decisionScheduleID),
			})
	}

	for _, activityScheduleID := range activityScheduleIDs {
		transferTasks = append(transferTasks,
			&ActivityTask{
				TaskID:     s.GetNextSequenceNumber(),
				DomainID:   domainID,
				TaskList:   taskList,
				ScheduleID: int64(activityScheduleID),
			})
	}

	response, err := s.WorkflowMgr.CreateWorkflowExecution(&CreateWorkflowExecutionRequest{
		RequestID:                   uuid.New(),
		DomainID:                    domainID,
		Execution:                   workflowExecution,
		TaskList:                    taskList,
		ExecutionContext:            executionContext,
		NextEventID:                 nextEventID,
		LastProcessedEvent:          lastProcessedEventID,
		TransferTasks:               transferTasks,
		RangeID:                     s.ShardContext.GetRangeID(),
		DecisionScheduleID:          common.EmptyEventID,
		DecisionStartedID:           common.EmptyEventID,
		DecisionStartToCloseTimeout: 1,
	})

	if err != nil {
		return "", err
	}

	return response.TaskID, nil
}

// CreateChildWorkflowExecution is a utility method to create child workflow executions
func (s *TestBase) CreateChildWorkflowExecution(domainID string, workflowExecution workflow.WorkflowExecution,
	parentDomainID string, parentExecution *workflow.WorkflowExecution, initiatedID int64, taskList, wType string,
	decisionTimeout int32, executionContext []byte, nextEventID int64, lastProcessedEventID int64,
	decisionScheduleID int64, timerTasks []Task) (string, error) {
	response, err := s.WorkflowMgr.CreateWorkflowExecution(&CreateWorkflowExecutionRequest{
		RequestID:            uuid.New(),
		DomainID:             domainID,
		Execution:            workflowExecution,
		ParentDomainID:       parentDomainID,
		ParentExecution:      parentExecution,
		InitiatedID:          initiatedID,
		TaskList:             taskList,
		WorkflowTypeName:     wType,
		DecisionTimeoutValue: decisionTimeout,
		ExecutionContext:     executionContext,
		NextEventID:          nextEventID,
		LastProcessedEvent:   lastProcessedEventID,
		RangeID:              s.ShardContext.GetRangeID(),
		TransferTasks: []Task{
			&DecisionTask{
				TaskID:     s.GetNextSequenceNumber(),
				DomainID:   domainID,
				TaskList:   taskList,
				ScheduleID: decisionScheduleID,
			},
		},
		TimerTasks:                  timerTasks,
		DecisionScheduleID:          decisionScheduleID,
		DecisionStartedID:           common.EmptyEventID,
		DecisionStartToCloseTimeout: 1,
	})

	if err != nil {
		return "", err
	}

	return response.TaskID, nil
}

// GetWorkflowExecutionInfo is a utility method to retrieve execution info
func (s *TestBase) GetWorkflowExecutionInfo(domainID string, workflowExecution workflow.WorkflowExecution) (
	*WorkflowMutableState, error) {
	response, err := s.WorkflowMgr.GetWorkflowExecution(&GetWorkflowExecutionRequest{
		DomainID:  domainID,
		Execution: workflowExecution,
	})
	if err != nil {
		return nil, err
	}

	return response.State, nil
}

// GetCurrentWorkflow returns the workflow state for the given params
func (s *TestBase) GetCurrentWorkflow(domainID, workflowID string) (string, error) {
	response, err := s.WorkflowMgr.GetCurrentExecution(&GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})

	if err != nil {
		return "", err
	}

	return response.RunID, nil
}

// ContinueAsNewExecution is a utility method to create workflow executions
func (s *TestBase) ContinueAsNewExecution(updatedInfo *WorkflowExecutionInfo, condition int64,
	newExecution workflow.WorkflowExecution, nextEventID, decisionScheduleID int64) error {
	newdecisionTask := &DecisionTask{
		TaskID:     s.GetNextSequenceNumber(),
		DomainID:   updatedInfo.DomainID,
		TaskList:   updatedInfo.TaskList,
		ScheduleID: int64(decisionScheduleID),
	}

	return s.WorkflowMgr.UpdateWorkflowExecution(&UpdateWorkflowExecutionRequest{
		ExecutionInfo:       updatedInfo,
		TransferTasks:       []Task{newdecisionTask},
		TimerTasks:          nil,
		Condition:           condition,
		DeleteTimerTask:     nil,
		RangeID:             s.ShardContext.GetRangeID(),
		UpsertActivityInfos: nil,
		DeleteActivityInfo:  nil,
		UpserTimerInfos:     nil,
		DeleteTimerInfos:    nil,
		ContinueAsNew: &CreateWorkflowExecutionRequest{
			RequestID:                   uuid.New(),
			DomainID:                    updatedInfo.DomainID,
			Execution:                   newExecution,
			TaskList:                    updatedInfo.TaskList,
			WorkflowTypeName:            updatedInfo.WorkflowTypeName,
			DecisionTimeoutValue:        updatedInfo.DecisionTimeoutValue,
			ExecutionContext:            nil,
			NextEventID:                 nextEventID,
			LastProcessedEvent:          common.EmptyEventID,
			RangeID:                     s.ShardContext.GetRangeID(),
			TransferTasks:               nil,
			TimerTasks:                  nil,
			DecisionScheduleID:          decisionScheduleID,
			DecisionStartedID:           common.EmptyEventID,
			DecisionStartToCloseTimeout: 1,
			ContinueAsNew:               true,
		},
	})
}

// UpdateWorkflowExecution is a utility method to update workflow execution
func (s *TestBase) UpdateWorkflowExecution(updatedInfo *WorkflowExecutionInfo, decisionScheduleIDs []int64,
	activityScheduleIDs []int64, condition int64, timerTasks []Task, deleteTimerTask Task,
	upsertActivityInfos []*ActivityInfo, deleteActivityInfo *int64,
	upsertTimerInfos []*TimerInfo, deleteTimerInfos []string) error {
	return s.UpdateWorkflowExecutionWithRangeID(updatedInfo, decisionScheduleIDs, activityScheduleIDs,
		s.ShardContext.GetRangeID(), condition, timerTasks, deleteTimerTask, upsertActivityInfos, deleteActivityInfo,
		upsertTimerInfos, deleteTimerInfos, nil, nil, nil, nil)
}

// UpdateWorkflowExecutionAndDelete is a utility method to update workflow execution
func (s *TestBase) UpdateWorkflowExecutionAndDelete(updatedInfo *WorkflowExecutionInfo, condition int64) error {
	transferTasks := []Task{}
	transferTasks = append(transferTasks, &DeleteExecutionTask{TaskID: s.GetNextSequenceNumber()})
	return s.WorkflowMgr.UpdateWorkflowExecution(&UpdateWorkflowExecutionRequest{
		ExecutionInfo:       updatedInfo,
		TransferTasks:       transferTasks,
		TimerTasks:          nil,
		Condition:           condition,
		DeleteTimerTask:     nil,
		RangeID:             s.ShardContext.GetRangeID(),
		UpsertActivityInfos: nil,
		DeleteActivityInfo:  nil,
		UpserTimerInfos:     nil,
		DeleteTimerInfos:    nil,
		CloseExecution:      true,
	})
}

// UpsertChildExecutionsState is a utility method to update mutable state of workflow execution
func (s *TestBase) UpsertChildExecutionsState(updatedInfo *WorkflowExecutionInfo, condition int64,
	upsertChildInfos []*ChildExecutionInfo) error {
	return s.UpdateWorkflowExecutionWithRangeID(updatedInfo, nil, nil,
		s.ShardContext.GetRangeID(), condition, nil, nil, nil, nil,
		nil, nil, upsertChildInfos, nil, nil, nil)
}

// UpsertRequestCancelState is a utility method to update mutable state of workflow execution
func (s *TestBase) UpsertRequestCancelState(updatedInfo *WorkflowExecutionInfo, condition int64,
	upsertCancelInfos []*RequestCancelInfo) error {
	return s.UpdateWorkflowExecutionWithRangeID(updatedInfo, nil, nil,
		s.ShardContext.GetRangeID(), condition, nil, nil, nil, nil,
		nil, nil, nil, nil, upsertCancelInfos, nil)
}

// DeleteChildExecutionsState is a utility method to delete child execution from mutable state
func (s *TestBase) DeleteChildExecutionsState(updatedInfo *WorkflowExecutionInfo, condition int64,
	deleteChildInfo int64) error {
	return s.UpdateWorkflowExecutionWithRangeID(updatedInfo, nil, nil,
		s.ShardContext.GetRangeID(), condition, nil, nil, nil, nil,
		nil, nil, nil, &deleteChildInfo, nil, nil)
}

// DeleteCancelState is a utility method to delete request cancel state from mutable state
func (s *TestBase) DeleteCancelState(updatedInfo *WorkflowExecutionInfo, condition int64,
	deleteCancelInfo int64) error {
	return s.UpdateWorkflowExecutionWithRangeID(updatedInfo, nil, nil,
		s.ShardContext.GetRangeID(), condition, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, &deleteCancelInfo)
}

// UpdateWorkflowExecutionWithRangeID is a utility method to update workflow execution
func (s *TestBase) UpdateWorkflowExecutionWithRangeID(updatedInfo *WorkflowExecutionInfo, decisionScheduleIDs []int64,
	activityScheduleIDs []int64, rangeID, condition int64, timerTasks []Task, deleteTimerTask Task,
	upsertActivityInfos []*ActivityInfo, deleteActivityInfo *int64, upsertTimerInfos []*TimerInfo,
	deleteTimerInfos []string, upsertChildInfos []*ChildExecutionInfo, deleteChildInfo *int64,
	upsertCancelInfos []*RequestCancelInfo, deleteCancelInfo *int64) error {
	transferTasks := []Task{}
	for _, decisionScheduleID := range decisionScheduleIDs {
		transferTasks = append(transferTasks, &DecisionTask{
			TaskID:     s.GetNextSequenceNumber(),
			DomainID:   updatedInfo.DomainID,
			TaskList:   updatedInfo.TaskList,
			ScheduleID: int64(decisionScheduleID)})
	}

	for _, activityScheduleID := range activityScheduleIDs {
		transferTasks = append(transferTasks, &ActivityTask{
			TaskID:     s.GetNextSequenceNumber(),
			DomainID:   updatedInfo.DomainID,
			TaskList:   updatedInfo.TaskList,
			ScheduleID: int64(activityScheduleID)})
	}

	return s.WorkflowMgr.UpdateWorkflowExecution(&UpdateWorkflowExecutionRequest{
		ExecutionInfo:             updatedInfo,
		TransferTasks:             transferTasks,
		TimerTasks:                timerTasks,
		Condition:                 condition,
		DeleteTimerTask:           deleteTimerTask,
		RangeID:                   rangeID,
		UpsertActivityInfos:       upsertActivityInfos,
		DeleteActivityInfo:        deleteActivityInfo,
		UpserTimerInfos:           upsertTimerInfos,
		DeleteTimerInfos:          deleteTimerInfos,
		UpsertChildExecutionInfos: upsertChildInfos,
		DeleteChildExecutionInfo:  deleteChildInfo,
		UpsertRequestCancelInfos:  upsertCancelInfos,
		DeleteRequestCancelInfo:   deleteCancelInfo,
	})
}

// UpdateWorkflowExecutionWithTransferTasks is a utility method to update workflow execution
func (s *TestBase) UpdateWorkflowExecutionWithTransferTasks(
	updatedInfo *WorkflowExecutionInfo, condition int64, transferTasks []Task, upsertActivityInfo []*ActivityInfo) error {
	return s.WorkflowMgr.UpdateWorkflowExecution(&UpdateWorkflowExecutionRequest{
		ExecutionInfo:       updatedInfo,
		TransferTasks:       transferTasks,
		Condition:           condition,
		UpsertActivityInfos: upsertActivityInfo,
		RangeID:             s.ShardContext.GetRangeID(),
	})
}

// UpdateWorkflowExecutionForRequestCancel is a utility method to update workflow execution
func (s *TestBase) UpdateWorkflowExecutionForRequestCancel(
	updatedInfo *WorkflowExecutionInfo, condition int64, transferTasks []Task,
	upsertRequestCancelInfo []*RequestCancelInfo) error {
	return s.WorkflowMgr.UpdateWorkflowExecution(&UpdateWorkflowExecutionRequest{
		ExecutionInfo:            updatedInfo,
		TransferTasks:            transferTasks,
		Condition:                condition,
		UpsertRequestCancelInfos: upsertRequestCancelInfo,
		RangeID:                  s.ShardContext.GetRangeID(),
	})
}

// DeleteWorkflowExecution is a utility method to delete a workflow execution
func (s *TestBase) DeleteWorkflowExecution(info *WorkflowExecutionInfo) error {
	return s.WorkflowMgr.DeleteWorkflowExecution(&DeleteWorkflowExecutionRequest{
		ExecutionInfo: info,
	})
}

// GetTransferTasks is a utility method to get tasks from transfer task queue
func (s *TestBase) GetTransferTasks(batchSize int) ([]*TransferTaskInfo, error) {
	response, err := s.WorkflowMgr.GetTransferTasks(&GetTransferTasksRequest{
		ReadLevel:    s.GetReadLevel(),
		MaxReadLevel: int64(math.MaxInt64),
		BatchSize:    batchSize,
	})

	if err != nil {
		return nil, err
	}

	for _, task := range response.Tasks {
		atomic.StoreInt64(&s.readLevel, task.TaskID)
	}

	return response.Tasks, nil
}

// CompleteTransferTask is a utility method to complete a transfer task
func (s *TestBase) CompleteTransferTask(taskID int64) error {

	return s.WorkflowMgr.CompleteTransferTask(&CompleteTransferTaskRequest{
		TaskID: taskID,
	})
}

// GetTimerIndexTasks is a utility method to get tasks from transfer task queue
func (s *TestBase) GetTimerIndexTasks() ([]*TimerTaskInfo, error) {
	response, err := s.WorkflowMgr.GetTimerIndexTasks(&GetTimerIndexTasksRequest{
		MinTimestamp: time.Time{},
		MaxTimestamp: time.Unix(0, math.MaxInt64),
		BatchSize:    10})

	if err != nil {
		return nil, err
	}

	return response.Timers, nil
}

// CompleteTimerTask is a utility method to complete a timer task
func (s *TestBase) CompleteTimerTask(ts time.Time, taskID int64) error {
	return s.WorkflowMgr.CompleteTimerTask(&CompleteTimerTaskRequest{
		VisibilityTimestamp: ts,
		TaskID:              taskID,
	})
}

// CreateDecisionTask is a utility method to create a task
func (s *TestBase) CreateDecisionTask(domainID string, workflowExecution workflow.WorkflowExecution, taskList string,
	decisionScheduleID int64) (int64, error) {
	leaseResponse, err := s.TaskMgr.LeaseTaskList(&LeaseTaskListRequest{
		DomainID: domainID,
		TaskList: taskList,
		TaskType: TaskListTypeDecision,
	})
	if err != nil {
		return 0, err
	}

	taskID := s.GetNextSequenceNumber()
	tasks := []*CreateTaskInfo{
		{
			TaskID:    taskID,
			Execution: workflowExecution,
			Data: &TaskInfo{
				DomainID:   domainID,
				WorkflowID: workflowExecution.GetWorkflowId(),
				RunID:      workflowExecution.GetRunId(),
				TaskID:     taskID,
				ScheduleID: decisionScheduleID,
			},
		},
	}

	_, err = s.TaskMgr.CreateTasks(&CreateTasksRequest{
		TaskListInfo: leaseResponse.TaskListInfo,
		Tasks:        tasks,
	})

	if err != nil {
		return 0, err
	}

	return taskID, err
}

// CreateActivityTasks is a utility method to create tasks
func (s *TestBase) CreateActivityTasks(domainID string, workflowExecution workflow.WorkflowExecution,
	activities map[int64]string) ([]int64, error) {

	var taskIDs []int64
	var leaseResponse *LeaseTaskListResponse
	var err error
	for activityScheduleID, taskList := range activities {

		leaseResponse, err = s.TaskMgr.LeaseTaskList(
			&LeaseTaskListRequest{DomainID: domainID, TaskList: taskList, TaskType: TaskListTypeActivity})
		if err != nil {
			return []int64{}, err
		}
		taskID := s.GetNextSequenceNumber()
		tasks := []*CreateTaskInfo{
			{
				TaskID:    taskID,
				Execution: workflowExecution,
				Data: &TaskInfo{
					DomainID:   domainID,
					WorkflowID: workflowExecution.GetWorkflowId(),
					RunID:      workflowExecution.GetRunId(),
					TaskID:     taskID,
					ScheduleID: activityScheduleID,
				},
			},
		}
		_, err := s.TaskMgr.CreateTasks(&CreateTasksRequest{
			TaskListInfo: leaseResponse.TaskListInfo,
			Tasks:        tasks,
		})

		if err != nil {
			return nil, err
		}

		taskIDs = append(taskIDs, taskID)
	}

	return taskIDs, nil
}

// GetTasks is a utility method to get tasks from persistence
func (s *TestBase) GetTasks(domainID, taskList string, taskType int, batchSize int) (*GetTasksResponse, error) {
	leaseResponse, err := s.TaskMgr.LeaseTaskList(&LeaseTaskListRequest{
		DomainID: domainID,
		TaskList: taskList,
		TaskType: taskType,
	})
	if err != nil {
		return nil, err
	}

	response, err := s.TaskMgr.GetTasks(&GetTasksRequest{
		DomainID:     domainID,
		TaskList:     taskList,
		TaskType:     taskType,
		BatchSize:    batchSize,
		RangeID:      leaseResponse.TaskListInfo.RangeID,
		MaxReadLevel: math.MaxInt64,
	})

	if err != nil {
		return nil, err
	}

	return &GetTasksResponse{Tasks: response.Tasks}, nil
}

// CompleteTask is a utility method to complete a task
func (s *TestBase) CompleteTask(domainID, taskList string, taskType int, taskID int64, ackLevel int64) error {
	leaseResponse, err := s.TaskMgr.LeaseTaskList(&LeaseTaskListRequest{
		DomainID: domainID,
		TaskList: taskList,
		TaskType: taskType,
	})
	if err != nil {
		return err
	}

	return s.TaskMgr.CompleteTask(&CompleteTaskRequest{
		TaskList: &TaskListInfo{
			DomainID: domainID,
			AckLevel: ackLevel,
			TaskType: taskType,
			Name:     taskList,
			RangeID:  leaseResponse.TaskListInfo.RangeID,
		},
		TaskID: taskID,
	})
}

// ClearTransferQueue completes all tasks in transfer queue
func (s *TestBase) ClearTransferQueue() {
	log.Infof("Clearing transfer tasks (RangeID: %v, ReadLevel: %v, AckLevel: %v)", s.ShardContext.GetRangeID(),
		s.GetReadLevel(), s.ShardContext.GetTransferAckLevel())
	tasks, err := s.GetTransferTasks(100)
	if err != nil {
		log.Fatalf("Error during cleanup: %v", err)
	}

	counter := 0
	for _, t := range tasks {
		log.Infof("Deleting transfer task with ID: %v", t.TaskID)
		s.CompleteTransferTask(t.TaskID)
		counter++
	}

	log.Infof("Deleted '%v' transfer tasks.", counter)
	s.ShardContext.Reset()
	atomic.StoreInt64(&s.readLevel, 0)
}

// SetupWorkflowStore to setup workflow test base
func (s *TestBase) SetupWorkflowStore() {
	s.SetupWorkflowStoreWithOptions(TestBaseOptions{
		SchemaDir:       testSchemaDir,
		ClusterHost:     testWorkflowClusterHosts,
		ClusterPort:     testPort,
		ClusterUser:     testUser,
		ClusterPassword: testPassword,
		DropKeySpace:    true,
	})
}

// TearDownWorkflowStore to cleanup
func (s *TestBase) TearDownWorkflowStore() {
	s.CassandraTestCluster.tearDownTestCluster()
}

// GetNextSequenceNumber generates a unique sequence number for can be used for transfer queue taskId
func (s *TestBase) GetNextSequenceNumber() int64 {
	taskID, _ := s.ShardContext.GetNextTransferTaskID()
	return taskID
}

// GetReadLevel returns the current read level for shard
func (s *TestBase) GetReadLevel() int64 {
	return atomic.LoadInt64(&s.readLevel)
}

func (s *CassandraTestCluster) setupTestCluster(keySpace string, dropKeySpace bool, schemaDir string) {
	if keySpace == "" {
		keySpace = generateRandomKeyspace(10)
	}
	s.createCluster(testWorkflowClusterHosts, testPort, testUser, testPassword, testDatacenter, gocql.Consistency(1),
		keySpace)
	s.createKeyspace(1, dropKeySpace)
	s.loadSchema([]string{"schema.cql"}, schemaDir)
	s.loadVisibilitySchema([]string{"schema.cql"}, schemaDir)
}

func (s *CassandraTestCluster) tearDownTestCluster() {
	s.dropKeyspace()
	s.session.Close()
}

func (s *CassandraTestCluster) createCluster(clusterHosts string, port int, user, password, dc string,
	cons gocql.Consistency, keyspace string) {
	s.cluster = common.NewCassandraCluster(clusterHosts, port, user, password, dc)
	s.cluster.Consistency = cons
	s.cluster.Keyspace = "system"
	s.cluster.Timeout = 40 * time.Second
	var err error
	s.session, err = s.cluster.CreateSession()
	if err != nil {
		log.WithField(logging.TagErr, err).Fatal(`createSession`)
	}
	s.keyspace = keyspace
}

func (s *CassandraTestCluster) createKeyspace(replicas int, dropKeySpace bool) {
	err := common.CreateCassandraKeyspace(s.session, s.keyspace, replicas, dropKeySpace)
	if err != nil {
		log.Fatal(err)
	}

	s.cluster.Keyspace = s.keyspace
}

func (s *CassandraTestCluster) dropKeyspace() {
	err := common.DropCassandraKeyspace(s.session, s.keyspace)
	if err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		log.Fatal(err)
	}
}

func (s *CassandraTestCluster) loadSchema(fileNames []string, schemaDir string) {
	workflowSchemaDir := "./schema/cadence"
	if schemaDir != "" {
		workflowSchemaDir = schemaDir + "/schema/cadence"
	}

	err := common.LoadCassandraSchema(workflowSchemaDir, fileNames, s.keyspace, true)
	if err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		log.Fatal(err)
	}
}

func (s *CassandraTestCluster) loadVisibilitySchema(fileNames []string, schemaDir string) {
	workflowSchemaDir := "./schema/visibility"
	if schemaDir != "" {
		workflowSchemaDir = schemaDir + "/schema/visibility"
	}

	err := common.LoadCassandraSchema(workflowSchemaDir, fileNames, s.keyspace, false)
	if err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		log.Fatal(err)
	}
}

func validateTimeRange(t time.Time, expectedDuration time.Duration) bool {
	currentTime := time.Now()
	diff := time.Duration(currentTime.UnixNano() - t.UnixNano())
	if diff > expectedDuration {
		log.Infof("Current time: %v, Application time: %v, Differenrce: %v", currentTime, t, diff)
		return false
	}
	return true
}

func generateRandomKeyspace(n int) string {
	rand.Seed(time.Now().UnixNano())
	letterRunes := []rune("workflow")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
