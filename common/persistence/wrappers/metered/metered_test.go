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

package metered

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var _staticMethods = map[string]bool{
	"Close":      true,
	"GetName":    true,
	"GetShardID": true,
}

func TestWrappersAgainstPreviousImplementation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		prepareMock func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any)
	}{
		{
			name: "ConfigStoreManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockConfigStoreManager(ctrl)

				newObj := NewConfigStoreManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true}, "test-host", "test")

				return newObj, wrapped
			},
		},
		{
			name: "DomainManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockDomainManager(ctrl)

				newObj := NewDomainManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true}, "test-host", "test")

				return newObj, wrapped
			},
		},
		{
			name: "HistoryManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockHistoryManager(ctrl)

				newObj := NewHistoryManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true}, "test-host", "test")

				return newObj, wrapped
			},
		},
		{
			name: "QueueManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockQueueManager(ctrl)

				newObj := NewQueueManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true}, "test-host", "test")

				return newObj, wrapped
			},
		},
		{
			name: "ShardManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockShardManager(ctrl)

				newObj := NewShardManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true}, "test-host", "test")

				return newObj, wrapped
			},
		},
		{
			name: "TaskManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockTaskManager(ctrl)

				newObj := NewTaskManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true}, "test-host", "test")

				return newObj, wrapped
			},
		},
		{
			name: "VisibilityManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockVisibilityManager(ctrl)

				newObj := NewVisibilityManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true}, "test-host", "test")

				return newObj, wrapped
			},
		},
		{
			name: "ExecutionManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockExecutionManager(ctrl)

				wrapped.EXPECT().GetShardID().Return(0).AnyTimes()

				newObj := NewExecutionManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true},
					dynamicproperties.GetIntPropertyFn(1), dynamicproperties.GetBoolPropertyFn(true), "test-host", "test")

				return newObj, wrapped
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("without error", func(t *testing.T) {
				ctrl := gomock.NewController(t)

				zapLogger, logs := setupLogsCapture()
				metricScope := tally.NewTestScope("", nil)
				metricsClient := metrics.NewClient(metricScope, metrics.ServiceIdx(0), metrics.HistogramMigration{})
				logger := log.NewLogger(zapLogger)

				wrapper, mocked := tc.prepareMock(t, ctrl, metricsClient, logger)
				prepareMockForTest(t, mocked, nil)

				runScenario(t, wrapper, logs, metricScope)
			})
			t.Run("with error", func(t *testing.T) {
				for _, errorType := range []error{
					&types.DomainAlreadyExistsError{},
					&types.BadRequestError{},
					&types.EntityNotExistsError{},
					&types.ServiceBusyError{},
					&persistence.WorkflowExecutionAlreadyStartedError{},
					&persistence.ConditionFailedError{},
					&persistence.CurrentWorkflowConditionFailedError{},
					&persistence.ShardAlreadyExistError{},
					&persistence.ShardOwnershipLostError{},
					&persistence.DuplicateRequestError{},
					&persistence.TimeoutError{},
					&persistence.DBUnavailableError{},
					errors.New("persistence error"),
				} {
					t.Run(reflect.TypeOf(errorType).String(), func(t *testing.T) {
						ctrl := gomock.NewController(t)

						zapLogger, logs := setupLogsCapture()
						metricScope := tally.NewTestScope("", nil)
						metricsClient := metrics.NewClient(metricScope, metrics.ServiceIdx(0), metrics.HistogramMigration{})
						logger := log.NewLogger(zapLogger)

						newObj, mocked := tc.prepareMock(t, ctrl, metricsClient, logger)
						prepareMockForTest(t, mocked, errorType)

						runScenario(t, newObj, logs, metricScope)
					})
				}
			})
		})
	}
}

func prepareMockForTest(t *testing.T, input interface{}, expectedErr error) {
	switch mocked := input.(type) {
	case *persistence.MockConfigStoreManager:
		mocked.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().FetchDynamicConfig(gomock.Any(), gomock.Any()).Return(&persistence.FetchDynamicConfigResponse{}, expectedErr).Times(1)
	case *persistence.MockDomainManager:
		mocked.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).Return(&persistence.CreateDomainResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(&persistence.GetDomainResponse{}, expectedErr).Times(1)
		mocked.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteDomain(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteDomainByName(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(&persistence.ListDomainsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{}, expectedErr).Times(1)
	case *persistence.MockHistoryManager:
		mocked.EXPECT().AppendHistoryNodes(gomock.Any(), gomock.Any()).Return(&persistence.AppendHistoryNodesResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(&persistence.ReadHistoryBranchResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ReadHistoryBranchByBatch(gomock.Any(), gomock.Any()).Return(&persistence.ReadHistoryBranchByBatchResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ReadRawHistoryBranch(gomock.Any(), gomock.Any()).Return(&persistence.ReadRawHistoryBranchResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ForkHistoryBranch(gomock.Any(), gomock.Any()).Return(&persistence.ForkHistoryBranchResponse{}, expectedErr).Times(1)
		mocked.EXPECT().DeleteHistoryBranch(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetHistoryTree(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTreeResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetAllHistoryTreeBranches(gomock.Any(), gomock.Any()).Return(&persistence.GetAllHistoryTreeBranchesResponse{}, expectedErr).Times(1)
	case *persistence.MockQueueManager:
		mocked.EXPECT().EnqueueMessage(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().ReadMessages(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*persistence.QueueMessage{}, expectedErr).Times(1)
		mocked.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetAckLevels(gomock.Any()).Return(map[string]int64{}, expectedErr).Times(1)
		mocked.EXPECT().DeleteMessagesBefore(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteMessageFromDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().EnqueueMessageToDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetDLQAckLevels(gomock.Any()).Return(map[string]int64{}, expectedErr).Times(1)
		mocked.EXPECT().GetDLQSize(gomock.Any()).Return(int64(0), expectedErr).Times(1)
		mocked.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().ReadMessagesFromDLQ(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*persistence.QueueMessage{}, nil, expectedErr).Times(1)
		mocked.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
	case *persistence.MockShardManager:
		mocked.EXPECT().GetShard(gomock.Any(), gomock.Any()).Return(&persistence.GetShardResponse{}, expectedErr).Times(1)
		mocked.EXPECT().UpdateShard(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().CreateShard(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
	case *persistence.MockTaskManager:
		mocked.EXPECT().CompleteTasksLessThan(gomock.Any(), gomock.Any()).Return(&persistence.CompleteTasksLessThanResponse{}, expectedErr).Times(1)
		mocked.EXPECT().CompleteTask(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().CreateTasks(gomock.Any(), gomock.Any()).Return(&persistence.CreateTasksResponse{}, expectedErr).Times(1)
		mocked.EXPECT().DeleteTaskList(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetTasksResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetOrphanTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetOrphanTasksResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetTaskListSize(gomock.Any(), gomock.Any()).Return(&persistence.GetTaskListSizeResponse{}, expectedErr).Times(1)
		mocked.EXPECT().LeaseTaskList(gomock.Any(), gomock.Any()).Return(&persistence.LeaseTaskListResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetTaskList(gomock.Any(), gomock.Any()).Return(&persistence.GetTaskListResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListTaskList(gomock.Any(), gomock.Any()).Return(&persistence.ListTaskListResponse{}, expectedErr).Times(1)
		mocked.EXPECT().UpdateTaskList(gomock.Any(), gomock.Any()).Return(&persistence.UpdateTaskListResponse{}, expectedErr).Times(1)
	case *persistence.MockVisibilityManager:
		mocked.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&persistence.CountWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.GetClosedWorkflowExecutionResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListWorkflowExecutionsResponse{}, expectedErr).Times(1)
	case *persistence.MockExecutionManager:
		mocked.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.CreateWorkflowExecutionResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.GetWorkflowExecutionResponse{}, expectedErr).Times(1)
		mocked.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.UpdateWorkflowExecutionResponse{}, expectedErr).Times(1)
		mocked.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Any()).Return(&persistence.GetCurrentExecutionResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.ConflictResolveWorkflowExecutionResponse{}, expectedErr).Times(1)
		mocked.EXPECT().CreateFailoverMarkerTasks(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetReplicationDLQSize(gomock.Any(), gomock.Any()).Return(&persistence.GetReplicationDLQSizeResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetReplicationTasksFromDLQ(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{}, expectedErr).Times(1)
		mocked.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Any()).Return(&persistence.IsWorkflowExecutionExistsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListConcreteExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListCurrentExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().PutReplicationTaskToDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{}, expectedErr).Times(1)
		mocked.EXPECT().CompleteHistoryTask(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(&persistence.RangeCompleteHistoryTaskResponse{}, expectedErr).Times(1)
		mocked.EXPECT().RangeDeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(&persistence.RangeDeleteReplicationTaskFromDLQResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&types.ActiveClusterSelectionPolicy{}, expectedErr).Times(1)
		mocked.EXPECT().DeleteActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
	default:
		t.Errorf("unsupported type %v", reflect.TypeOf(input))
		t.FailNow()
	}
	return
}

func setupLogsCapture() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.InfoLevel)
	return zap.New(core), logs
}

func runScenario(t *testing.T, newObj any, newLogs *observer.ObservedLogs, newMetrics tally.TestScope) {
	newV := reflect.ValueOf(newObj)
	infoT := reflect.TypeOf(newV.Interface())
	for i := 0; i < infoT.NumMethod(); i++ {
		method := infoT.Method(i)
		if _staticMethods[method.Name] {
			// Skip methods that do not use error injection.
			continue
		}
		t.Run(method.Name, func(t *testing.T) {
			vals := make([]reflect.Value, 0, method.Type.NumIn()-1)
			// First argument is always context.Context
			vals = append(vals, reflect.ValueOf(context.Background()))
			for i := 2; i < method.Type.NumIn(); i++ {
				if method.Type.In(i).Kind() == reflect.Ptr {
					vals = append(vals, reflect.New(method.Type.In(i).Elem()))
					continue
				}
				vals = append(vals, reflect.Zero(method.Type.In(i)))
			}

			var newRes []reflect.Value
			assert.NotPanicsf(t, func() {
				newRes = newV.MethodByName(method.Name).Call(vals)
			}, "method does not have tag defined")

			if len(newRes) == 0 {
				// Empty result means that method panicked.
				return
			}
		})
	}
}
