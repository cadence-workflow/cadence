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
	"time"

	p8s "github.com/m3db/prometheus_client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	tallyp8s "github.com/uber-go/tally/prometheus"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var _staticMethods = map[string]bool{
	"Close":                                true,
	"GetName":                              true,
	"GetShardID":                           true,
	"GetActivityMapDeleteRewriteThreshold": true,
	"GetTimerMapDeleteRewriteThreshold":    true,
}

// TestPersistenceMetricsLabelConsistency exercises every method of every metered
// wrapper against a single, shared Prometheus-backed metrics client. Prometheus
// rejects registering the same metric name twice with a different label set, so
// running all wrappers/methods through one registry surfaces label inconsistencies
// between methods and between wrapper types (e.g. shard-enabled vs shard-disabled
// ExecutionManager, or ExecutionManager vs HistoryManager sharing a metric name).
//
// Functional errors returned by the wrapped calls are intentionally ignored: the
// mocks below return zero-value responses that aren't semantically valid, so the
// only thing worth asserting on is the absence of Prometheus registration errors.
func TestPersistenceMetricsLabelConsistency(t *testing.T) {
	var registrationErrors []error
	promCfg := &tallyp8s.Configuration{
		OnError:   "none",
		TimerType: "histogram",
	}
	reporter, err := promCfg.NewReporter(tallyp8s.ConfigurationOptions{
		Registry: p8s.NewRegistry(),
		OnError: func(err error) {
			registrationErrors = append(registrationErrors, err)
		},
	})
	require.NoError(t, err)
	rootScope, closer := tally.NewRootScope(tally.ScopeOptions{
		Tags: map[string]string{
			metrics.CadenceServiceTagName: "history",
		},
		CachedReporter: reporter,
		Separator:      tallyp8s.DefaultSeparator,
	}, time.Second)

	metricsClient := metrics.NewClient(rootScope, metrics.History, metrics.MigrationConfig{})
	logger := log.NewNoop()

	for _, tc := range persistenceWrapperTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			newObj, mocked := tc.prepareMock(t, ctrl, metricsClient, logger)
			prepareMockForTest(t, mocked, nil)

			invokeAllMethods(t, newObj)
		})
	}

	// The calls above all return zero-value (empty, Len()==0) responses, so they
	// only ever exercise the PersistenceEmptyResponseCounter branch of
	// emitRowCountMetrics. Call every emptyCountedMethods entry once more with a
	// non-empty response so the PersistenceResponseRowSize histogram branch gets
	// registered too, and any label inconsistency there is caught as well.
	for _, tc := range nonEmptyRowCountTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tc.invoke(t, ctrl, metricsClient, logger)
		})
	}

	// Metric registration happens as metrics are emitted, and reporting/flushing
	// happens on Close(); force it here so registrationErrors is fully populated.
	require.NoError(t, closer.Close())

	// Log every registration error for diagnosability, then fail: these indicate a
	// metric name being registered with inconsistent labels across persistence
	// operations, which silently drops data in production (Prometheus keeps
	// whichever registration won the race and rejects the rest).
	for _, regErr := range registrationErrors {
		t.Logf("Prometheus registration error (label inconsistency): %v", regErr)
	}
	assert.Empty(t, registrationErrors, "Prometheus registration errors must not be emitted")
}

func TestGetRetryCountFromContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	wrapped := persistence.NewMockExecutionManager(ctrl)
	wrapped.EXPECT().GetShardID().Return(1).AnyTimes()
	request := &persistence.GetHistoryTasksRequest{}

	gomock.InOrder(
		wrapped.EXPECT().GetHistoryTasks(gomock.Any(), request).Return(nil, &types.InternalServiceError{}),
		wrapped.EXPECT().GetHistoryTasks(gomock.Any(), request).Return(&persistence.GetHistoryTasksResponse{}, nil),
	)

	metricScope := tally.NewTestScope("", nil)
	meteredManager := NewExecutionManager(
		wrapped,
		metrics.NewClient(metricScope, metrics.ServiceIdx(0), metrics.MigrationConfig{}),
		log.NewLogger(zap.NewNop()),
		&config.Persistence{},
		dynamicproperties.GetBoolPropertyFn(false),
	)

	policy := backoff.NewExponentialRetryPolicy(time.Nanosecond)
	policy.SetMaximumAttempts(2)

	retryer := persistence.NewPersistenceRetryer(meteredManager, persistence.NewMockHistoryManager(ctrl), policy)
	_, err := retryer.GetHistoryTasks(context.Background(), request)
	require.NoError(t, err)

	countsByIsRetry := map[string]int64{}
	for _, counter := range metricScope.Snapshot().Counters() {
		if counter.Name() == "persistence_requests" {
			countsByIsRetry[counter.Tags()["is_retry"]] += counter.Value()
		}
	}
	require.Equal(t, map[string]int64{"false": 1, "true": 1}, countsByIsRetry)
}

// persistenceWrapperTestCases enumerates every metered wrapper constructor so that
// tests can exercise all of them generically. ExecutionManager is included twice,
// once per shard-reporting mode, since that flag changes which metric labels are
// emitted for the same metric names.
func persistenceWrapperTestCases() []struct {
	name        string
	prepareMock func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any)
} {
	return []struct {
		name        string
		prepareMock func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any)
	}{
		{
			name: "ConfigStoreManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockConfigStoreManager(ctrl)

				newObj := NewConfigStoreManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})

				return newObj, wrapped
			},
		},
		{
			name: "DomainManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockDomainManager(ctrl)

				newObj := NewDomainManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})

				return newObj, wrapped
			},
		},
		{
			name: "HistoryManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockHistoryManager(ctrl)

				newObj := NewHistoryManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})

				return newObj, wrapped
			},
		},
		{
			name: "QueueManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockQueueManager(ctrl)

				newObj := NewQueueManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})

				return newObj, wrapped
			},
		},
		{
			name: "ShardManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockShardManager(ctrl)

				newObj := NewShardManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})

				return newObj, wrapped
			},
		},
		{
			name: "TaskManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockTaskManager(ctrl)

				newObj := NewTaskManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})

				return newObj, wrapped
			},
		},
		{
			name: "VisibilityManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockVisibilityManager(ctrl)

				newObj := NewVisibilityManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})

				return newObj, wrapped
			},
		},
		{
			name: "ExecutionManager",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockExecutionManager(ctrl)

				wrapped.EXPECT().GetShardID().Return(0).AnyTimes()

				newObj := NewExecutionManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true},
					dynamicproperties.GetBoolPropertyFn(true))

				return newObj, wrapped
			},
		},
		{
			name: "ExecutionManagerShardReportingDisabled",
			prepareMock: func(t *testing.T, ctrl *gomock.Controller, newMetricsClient metrics.Client, newLogger log.Logger) (newManager any, mocked any) {
				wrapped := persistence.NewMockExecutionManager(ctrl)

				wrapped.EXPECT().GetShardID().Return(0).AnyTimes()

				newObj := NewExecutionManager(wrapped, newMetricsClient, newLogger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true},
					dynamicproperties.GetBoolPropertyFn(false))

				return newObj, wrapped
			},
		},
	}
}

// nonEmptyRowCountTestCases covers every emptyCountedMethods entry with a
// non-empty (Len() > 0) response, one method call each, so the
// PersistenceResponseRowSize histogram branch of emitRowCountMetrics gets
// registered against the shared Prometheus registry at least once.
//
// ExecutionManager.GetReplicationTasksFromDLQ is intentionally omitted: its
// response type, *persistence.GetReplicationDLQTasksResponse, doesn't implement
// the lengther interface, so emitRowCountMetrics silently no-ops for it today
// regardless of response content.
func nonEmptyRowCountTestCases() []struct {
	name   string
	invoke func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger)
} {
	ctx := context.Background()
	return []struct {
		name   string
		invoke func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger)
	}{
		{
			name: "ExecutionManager.ListCurrentExecutions",
			invoke: func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger) {
				wrapped := persistence.NewMockExecutionManager(ctrl)
				wrapped.EXPECT().GetShardID().Return(0).AnyTimes()
				wrapped.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListCurrentExecutionsResponse{
					Executions: []*persistence.CurrentWorkflowExecution{{}},
				}, nil)

				mgr := NewExecutionManager(wrapped, metricsClient, logger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true},
					dynamicproperties.GetBoolPropertyFn(true))
				_, err := mgr.ListCurrentExecutions(ctx, &persistence.ListCurrentExecutionsRequest{})
				require.NoError(t, err)
			},
		},
		{
			name: "ExecutionManager.GetHistoryTasks",
			invoke: func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger) {
				wrapped := persistence.NewMockExecutionManager(ctrl)
				wrapped.EXPECT().GetShardID().Return(0).AnyTimes()
				wrapped.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{
					Tasks: []persistence.Task{&persistence.ActivityTask{}},
				}, nil)

				mgr := NewExecutionManager(wrapped, metricsClient, logger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true},
					dynamicproperties.GetBoolPropertyFn(true))
				_, err := mgr.GetHistoryTasks(ctx, &persistence.GetHistoryTasksRequest{TaskCategory: persistence.HistoryTaskCategoryTransfer})
				require.NoError(t, err)
			},
		},
		{
			name: "TaskManager.GetTasks",
			invoke: func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger) {
				wrapped := persistence.NewMockTaskManager(ctrl)
				wrapped.EXPECT().GetTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetTasksResponse{
					Tasks: []*persistence.TaskInfo{{}},
				}, nil)

				mgr := NewTaskManager(wrapped, metricsClient, logger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})
				_, err := mgr.GetTasks(ctx, &persistence.GetTasksRequest{DomainName: "test-domain"})
				require.NoError(t, err)
			},
		},
		{
			name: "DomainManager.ListDomains",
			invoke: func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger) {
				wrapped := persistence.NewMockDomainManager(ctrl)
				wrapped.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(&persistence.ListDomainsResponse{
					Domains: []*persistence.GetDomainResponse{{}},
				}, nil)

				mgr := NewDomainManager(wrapped, metricsClient, logger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})
				_, err := mgr.ListDomains(ctx, &persistence.ListDomainsRequest{})
				require.NoError(t, err)
			},
		},
		{
			name: "HistoryManager.ReadHistoryBranch",
			invoke: func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger) {
				wrapped := persistence.NewMockHistoryManager(ctrl)
				wrapped.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).Return(&persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{{}},
				}, nil)

				mgr := NewHistoryManager(wrapped, metricsClient, logger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})
				_, err := mgr.ReadHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{DomainName: "test-domain"})
				require.NoError(t, err)
			},
		},
		{
			name: "HistoryManager.GetAllHistoryTreeBranches",
			invoke: func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger) {
				wrapped := persistence.NewMockHistoryManager(ctrl)
				wrapped.EXPECT().GetAllHistoryTreeBranches(gomock.Any(), gomock.Any()).Return(&persistence.GetAllHistoryTreeBranchesResponse{
					Branches: []persistence.HistoryBranchDetail{{}},
				}, nil)

				mgr := NewHistoryManager(wrapped, metricsClient, logger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})
				_, err := mgr.GetAllHistoryTreeBranches(ctx, &persistence.GetAllHistoryTreeBranchesRequest{})
				require.NoError(t, err)
			},
		},
		{
			name: "QueueManager.ReadMessages",
			invoke: func(t *testing.T, ctrl *gomock.Controller, metricsClient metrics.Client, logger log.Logger) {
				wrapped := persistence.NewMockQueueManager(ctrl)
				wrapped.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).Return(&persistence.ReadMessagesResponse{
					Messages: persistence.QueueMessageList{{}},
				}, nil)

				mgr := NewQueueManager(wrapped, metricsClient, logger, &config.Persistence{EnablePersistenceLatencyHistogramMetrics: true})
				_, err := mgr.ReadMessages(ctx, &persistence.ReadMessagesRequest{})
				require.NoError(t, err)
			},
		},
	}
}

func TestWrappersAgainstPreviousImplementation(t *testing.T) {
	for _, tc := range persistenceWrapperTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("without error", func(t *testing.T) {
				ctrl := gomock.NewController(t)

				zapLogger, logs := setupLogsCapture()
				metricScope := tally.NewTestScope("", nil)
				metricsClient := metrics.NewClient(metricScope, metrics.ServiceIdx(0), metrics.MigrationConfig{})
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
						metricsClient := metrics.NewClient(metricScope, metrics.ServiceIdx(0), metrics.MigrationConfig{})
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
		mocked.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).Return(&persistence.ReadMessagesResponse{Messages: []*persistence.QueueMessage{}}, expectedErr).Times(1)
		mocked.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).Return(&persistence.GetAckLevelsResponse{AckLevels: map[string]int64{}}, expectedErr).Times(1)
		mocked.EXPECT().DeleteMessagesBefore(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteMessageFromDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().EnqueueMessageToDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetDLQAckLevels(gomock.Any(), gomock.Any()).Return(&persistence.GetDLQAckLevelsResponse{AckLevels: map[string]int64{}}, expectedErr).Times(1)
		mocked.EXPECT().GetDLQSize(gomock.Any(), gomock.Any()).Return(&persistence.GetDLQSizeResponse{Size: 0}, expectedErr).Times(1)
		mocked.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().ReadMessagesFromDLQ(gomock.Any(), gomock.Any()).Return(&persistence.ReadMessagesFromDLQResponse{Messages: []*persistence.QueueMessage{}}, expectedErr).Times(1)
		mocked.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
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
		mocked.EXPECT().CreateHistoryTasks(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().DeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetReplicationDLQSize(gomock.Any(), gomock.Any()).Return(&persistence.GetReplicationDLQSizeResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetReplicationTasksFromDLQ(gomock.Any(), gomock.Any()).Return(&persistence.GetReplicationDLQTasksResponse{}, expectedErr).Times(1)
		mocked.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Any()).Return(&persistence.IsWorkflowExecutionExistsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListConcreteExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Any()).Return(&persistence.ListCurrentExecutionsResponse{}, expectedErr).Times(1)
		mocked.EXPECT().PutReplicationTaskToDLQ(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{}, expectedErr).Times(1)
		mocked.EXPECT().CompleteHistoryTask(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(&persistence.RangeCompleteHistoryTaskResponse{}, expectedErr).Times(1)
		mocked.EXPECT().RangeDeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(&persistence.RangeDeleteReplicationTaskFromDLQResponse{}, expectedErr).Times(1)
		mocked.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(&types.ActiveClusterSelectionPolicy{}, expectedErr).Times(1)
		mocked.EXPECT().DeleteActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(expectedErr).Times(1)
		mocked.EXPECT().FetchWorkflowTimerTasksForCleanup(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)
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
	invokeAllMethods(t, newObj)
}

// invokeAllMethods calls every non-static method of newObj exactly once via
// reflection, using zero-value/newly-allocated arguments. It only asserts that
// the call doesn't panic; return values (including errors) are ignored, since
// callers use this to exercise metric-emission paths, not business logic.
func invokeAllMethods(t *testing.T, newObj any) {
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

			assert.NotPanicsf(t, func() {
				newV.MethodByName(method.Name).Call(vals)
			}, "method does not have tag defined")
		})
	}
}
