package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/taskdlq"
)

var testTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func TestGetRemoteClusterName(t *testing.T) {
	testDomainID := "test-domain-id"
	testWorkflowID := "test-workflow-id"
	testRunID := "test-run-id"
	currentCluster := "cluster-A"
	remoteCluster := "cluster-B"

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller) activecluster.Manager
		taskInfo       persistence.Task
		expectedResult string
		expectedError  error
	}{
		{
			name: "active-active domain with lookup error",
			setupMocks: func(ctrl *gomock.Controller) activecluster.Manager {
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				mockActiveClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(nil, errors.New("lookup error"))

				return mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: "",
			expectedError:  errors.New("lookup error"),
		},
		{
			name: "active-active domain becomes active",
			setupMocks: func(ctrl *gomock.Controller) activecluster.Manager {
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				mockActiveClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: currentCluster,
					}, nil)

				return mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: "",
			expectedError:  errors.New("domain becomes active when processing task as standby"),
		},
		{
			name: "active-active domain successful lookup",
			setupMocks: func(ctrl *gomock.Controller) activecluster.Manager {
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				mockActiveClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: remoteCluster,
					}, nil)

				return mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: remoteCluster,
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockActiveClusterMgr := tt.setupMocks(ctrl)

			result, err := getRemoteClusterName(
				context.Background(),
				currentCluster,
				mockActiveClusterMgr,
				tt.taskInfo,
			)

			if tt.expectedError != nil {
				assert.ErrorContains(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// mockDLQWriter is a simple in-process test double for TaskDLQWriter.
type mockDLQWriter struct {
	calls []taskdlq.AddTaskRequest
	err   error
}

func (m *mockDLQWriter) AddTask(_ context.Context, req taskdlq.AddTaskRequest) error {
	m.calls = append(m.calls, req)
	return m.err
}

func TestStandbyTaskPostActionWriteToDLQ_NilPostActionInfo_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	writer := &mockDLQWriter{}
	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetShardID().Return(1)
	enabled := func(string) string { return constants.HistoryTaskDLQModeEnabled }

	fn := standbyTaskPostActionWriteToDLQ(writer, mockShard, enabled)
	err := fn(context.Background(), &persistence.DecisionTask{}, nil, testlogger.New(t))
	assert.NoError(t, err)
	assert.Empty(t, writer.calls)
}

func TestStandbyTaskPostActionWriteToDLQ_WritesTaskToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	writer := &mockDLQWriter{}
	mockTask := persistence.NewMockTask(ctrl)
	mockTask.EXPECT().GetDomainID().Return("domain-1").AnyTimes()
	mockTask.EXPECT().GetWorkflowID().Return("wf-1").AnyTimes()
	mockTask.EXPECT().GetRunID().Return("run-1").AnyTimes()
	mockTask.EXPECT().GetTaskID().Return(int64(100)).AnyTimes()
	mockTask.EXPECT().GetTaskType().Return(1).AnyTimes()
	mockTask.EXPECT().GetVersion().Return(int64(5)).AnyTimes()
	mockTask.EXPECT().GetVisibilityTimestamp().Return(testTime).AnyTimes()

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetDomainByID("domain-1").Return(getDomainCacheEntry(true, true), nil)

	mockActiveClusterMgr := activecluster.NewMockManager(ctrl)
	mockActiveClusterMgr.EXPECT().
		GetActiveClusterSelectionPolicyForWorkflow(gomock.Any(), "domain-1", "wf-1", "run-1").
		Return(&types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{Scope: "my-scope", Name: "my-name"},
		}, nil)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetShardID().Return(7)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterMgr)

	enabled := func(string) string { return constants.HistoryTaskDLQModeEnabled }

	fn := standbyTaskPostActionWriteToDLQ(writer, mockShard, enabled)
	err := fn(context.Background(), mockTask, "some-post-action-info", testlogger.New(t))

	assert.NoError(t, err)
	assert.Len(t, writer.calls, 1)
	req := writer.calls[0]
	assert.Equal(t, 7, req.ShardID)
	assert.Equal(t, "domain-1", req.DomainID)
	assert.Equal(t, "my-scope", req.ClusterAttributeScope)
	assert.Equal(t, "my-name", req.ClusterAttributeName)
	assert.Equal(t, mockTask, req.Task)
}

func TestStandbyTaskPostActionWriteToDLQ_PropagatesWriterError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sentinel := errors.New("dlq write failed")
	writer := &mockDLQWriter{err: sentinel}
	mockTask := persistence.NewMockTask(ctrl)
	mockTask.EXPECT().GetDomainID().Return("d").AnyTimes()
	mockTask.EXPECT().GetWorkflowID().Return("w").AnyTimes()
	mockTask.EXPECT().GetRunID().Return("r").AnyTimes()
	mockTask.EXPECT().GetTaskID().Return(int64(1)).AnyTimes()
	mockTask.EXPECT().GetTaskType().Return(1).AnyTimes()
	mockTask.EXPECT().GetVersion().Return(int64(1)).AnyTimes()
	mockTask.EXPECT().GetVisibilityTimestamp().Return(testTime).AnyTimes()

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetDomainByID("d").Return(getDomainCacheEntry(true, true), nil)

	mockActiveClusterMgr := activecluster.NewMockManager(ctrl)
	mockActiveClusterMgr.EXPECT().
		GetActiveClusterSelectionPolicyForWorkflow(gomock.Any(), "d", "w", "r").
		Return(&types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{Scope: "s", Name: "n"},
		}, nil)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetShardID().Return(1)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterMgr)

	enabled := func(string) string { return constants.HistoryTaskDLQModeEnabled }

	fn := standbyTaskPostActionWriteToDLQ(writer, mockShard, enabled)
	err := fn(context.Background(), mockTask, "info", testlogger.New(t))

	assert.ErrorIs(t, err, sentinel)
}

func TestStandbyTaskPostActionWriteToDLQ_NilWriter_FallsBackToDiscard(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTask := persistence.NewMockTask(ctrl)
	mockTask.EXPECT().GetDomainID().Return("d").AnyTimes()
	mockTask.EXPECT().GetWorkflowID().Return("w").AnyTimes()
	mockTask.EXPECT().GetRunID().Return("r").AnyTimes()
	mockTask.EXPECT().GetTaskID().Return(int64(1)).AnyTimes()
	mockTask.EXPECT().GetTaskType().Return(1).AnyTimes()
	mockTask.EXPECT().GetVersion().Return(int64(1)).AnyTimes()
	mockTask.EXPECT().GetVisibilityTimestamp().Return(testTime).AnyTimes()

	// nil writer returns standbyTaskPostActionTaskDiscarded before GetShardID is called
	mockShard := shard.NewMockContext(ctrl)
	enabled := func(string) string { return constants.HistoryTaskDLQModeEnabled }

	fn := standbyTaskPostActionWriteToDLQ(nil, mockShard, enabled)
	err := fn(context.Background(), mockTask, "info", testlogger.New(t))

	assert.ErrorIs(t, err, ErrTaskDiscarded)
}

func TestStandbyTaskPostActionWriteToDLQ_DisabledMode_FallsBackToDiscard(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	writer := &mockDLQWriter{}
	mockTask := persistence.NewMockTask(ctrl)
	mockTask.EXPECT().GetDomainID().Return("domain-1").AnyTimes()
	mockTask.EXPECT().GetWorkflowID().Return("wf-1").AnyTimes()
	mockTask.EXPECT().GetRunID().Return("run-1").AnyTimes()
	mockTask.EXPECT().GetTaskID().Return(int64(100)).AnyTimes()
	mockTask.EXPECT().GetTaskType().Return(1).AnyTimes()
	mockTask.EXPECT().GetVersion().Return(int64(5)).AnyTimes()
	mockTask.EXPECT().GetVisibilityTimestamp().Return(testTime).AnyTimes()

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetDomainByID("domain-1").Return(getDomainCacheEntry(true, false), nil)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetShardID().Return(1)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)

	enabled := func(string) string { return constants.HistoryTaskDLQModeDisabled }

	fn := standbyTaskPostActionWriteToDLQ(writer, mockShard, enabled)
	err := fn(context.Background(), mockTask, "info", testlogger.New(t))

	assert.ErrorIs(t, err, ErrTaskDiscarded)
	assert.Empty(t, writer.calls)
}

func TestStandbyTaskPostActionWriteToDLQ_ShadowMode_WritesToDLQButDiscards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	writer := &mockDLQWriter{}
	mockTask := persistence.NewMockTask(ctrl)
	mockTask.EXPECT().GetDomainID().Return("domain-1").AnyTimes()
	mockTask.EXPECT().GetWorkflowID().Return("wf-1").AnyTimes()
	mockTask.EXPECT().GetRunID().Return("run-1").AnyTimes()
	mockTask.EXPECT().GetTaskID().Return(int64(100)).AnyTimes()
	mockTask.EXPECT().GetTaskType().Return(1).AnyTimes()
	mockTask.EXPECT().GetVersion().Return(int64(5)).AnyTimes()
	mockTask.EXPECT().GetVisibilityTimestamp().Return(testTime).AnyTimes()

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetDomainByID("domain-1").Return(getDomainCacheEntry(true, true), nil)

	mockActiveClusterMgr := activecluster.NewMockManager(ctrl)
	mockActiveClusterMgr.EXPECT().
		GetActiveClusterSelectionPolicyForWorkflow(gomock.Any(), "domain-1", "wf-1", "run-1").
		Return(&types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{Scope: "my-scope", Name: "my-name"},
		}, nil)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetShardID().Return(7)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
	mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterMgr)

	enabled := func(string) string { return constants.HistoryTaskDLQModeShadow }

	fn := standbyTaskPostActionWriteToDLQ(writer, mockShard, enabled)
	err := fn(context.Background(), mockTask, "info", testlogger.New(t))

	assert.ErrorIs(t, err, ErrTaskDiscarded)
	assert.Len(t, writer.calls, 1)
}

func TestStandbyTaskPostActionWriteToDLQ_NonActiveActiveDomain_UsesDefaultAttributes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	writer := &mockDLQWriter{}
	mockTask := persistence.NewMockTask(ctrl)
	mockTask.EXPECT().GetDomainID().Return("domain-1").AnyTimes()
	mockTask.EXPECT().GetWorkflowID().Return("wf-1").AnyTimes()
	mockTask.EXPECT().GetRunID().Return("run-1").AnyTimes()
	mockTask.EXPECT().GetTaskID().Return(int64(100)).AnyTimes()
	mockTask.EXPECT().GetTaskType().Return(1).AnyTimes()
	mockTask.EXPECT().GetVersion().Return(int64(5)).AnyTimes()
	mockTask.EXPECT().GetVisibilityTimestamp().Return(testTime).AnyTimes()

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetDomainByID("domain-1").Return(getDomainCacheEntry(true, false), nil)

	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetShardID().Return(1)
	mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)

	enabled := func(string) string { return constants.HistoryTaskDLQModeEnabled }

	fn := standbyTaskPostActionWriteToDLQ(writer, mockShard, enabled)
	err := fn(context.Background(), mockTask, "info", testlogger.New(t))

	assert.NoError(t, err)
	assert.Len(t, writer.calls, 1)
	assert.Equal(t, taskdlq.DefaultClusterAttributeScope, writer.calls[0].ClusterAttributeScope)
	assert.Equal(t, taskdlq.DefaultClusterAttributeName, writer.calls[0].ClusterAttributeName)
}
