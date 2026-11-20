package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	test "github.com/uber/cadence/service/history/testing"
	warchiver "github.com/uber/cadence/service/worker/archiver"
)

type visibilityTaskExecutorTestEnv struct {
	shard          *shard.TestContext
	executor       *visibilityTaskExecutor
	executionCache *execution.MockCache
	wfContext      *execution.MockContext
	visibilityMgr  *persistence.MockVisibilityManager
	archiverClient *warchiver.MockClient
}

func newVisibilityTaskExecutorTestEnv(t *testing.T, ctrl *gomock.Controller) *visibilityTaskExecutorTestEnv {
	testConfig := config.NewForTest()
	testConfig.EnableRecordWorkflowExecutionUninitialized = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
	mockShard := shard.NewTestContext(
		t,
		ctrl,
		&persistence.ShardInfo{
			ShardID: 0,
			RangeID: 1,
		},
		testConfig,
	)
	t.Cleanup(func() { mockShard.Finish(t) })
	mockShard.Resource.TimeSource = clock.NewMockedTimeSource()
	mockShard.SetEventsCache(events.NewCache(
		mockShard.GetShardID(),
		mockShard.GetHistoryManager(),
		mockShard.GetConfig(),
		mockShard.GetLogger(),
		mockShard.GetMetricsClient(),
		mockShard.GetDomainCache(),
	))

	mockDomainCache := mockShard.Resource.DomainCache
	mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()
	mockDomainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()
	mockDomainCache.EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()

	env := &visibilityTaskExecutorTestEnv{
		shard:          mockShard,
		executionCache: execution.NewMockCache(ctrl),
		wfContext:      execution.NewMockContext(ctrl),
		visibilityMgr:  persistence.NewMockVisibilityManager(ctrl),
		archiverClient: warchiver.NewMockClient(ctrl),
	}
	env.executor = NewVisibilityTaskExecutor(
		mockShard,
		env.archiverClient,
		env.executionCache,
		mockShard.GetLogger(),
		testConfig,
	).(*visibilityTaskExecutor)
	env.executor.visibilityMgr = env.visibilityMgr
	return env
}

// expectLoad makes the execution cache return the given mutable state for the workflow.
func (env *visibilityTaskExecutorTestEnv) expectLoad(mutableState execution.MutableState, loadErr error) {
	env.executionCache.EXPECT().
		GetOrCreateWorkflowExecutionWithTimeout(constants.TestDomainID, gomock.Any(), gomock.Any()).
		Return(env.wfContext, func(error) {}, nil).
		Times(1)
	env.wfContext.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(mutableState, loadErr).Times(1)
}

func (env *visibilityTaskExecutorTestEnv) expectVisibilityArchival(enabled bool) {
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	env.shard.Resource.ArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig(
		status,
		dynamicproperties.GetStringPropertyFn(status),
		enabled,
		dynamicproperties.GetBoolPropertyFn(enabled),
		status,
		"random URI",
	))
}

func newVisibilityTask(env *visibilityTaskExecutorTestEnv, info persistence.Task) Task {
	return NewHistoryTaskV2(env.shard, info, QueueTypeVisibility, env.shard.GetLogger(), nil, nil, nil, nil)
}

func TestVisibilityTaskExecutor_Execute(t *testing.T) {
	version := constants.TestGlobalDomainEntry.GetFailoverVersion()
	taskData := persistence.TaskData{Version: version, TaskID: 59}

	tests := map[string]struct {
		// setup returns the task to execute after setting expectations on env
		setup   func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task
		wantErr error
	}{
		"record workflow started writes the started record": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				workflowExecution, mutableState, _ := setupWorkflow(t, env.shard)
				info := &persistence.RecordWorkflowStartedTask{
					WorkflowIdentifier: workflowIdentifier(workflowExecution),
					TaskData:           taskData,
				}
				startEvent, err := mutableState.GetStartEvent(context.Background())
				require.NoError(t, err)
				env.expectLoad(mutableState, nil)
				env.visibilityMgr.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), createRecordWorkflowExecutionStartedRequest(
					t, constants.TestDomainName, startEvent, newVisibilityTask(env, info), mutableState, 2, env.shard.GetTimeSource().Now(), false,
				)).Return(nil).Times(1)
				return info
			},
		},
		"upsert writes the upserted record": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				workflowExecution, mutableState, _ := setupWorkflow(t, env.shard)
				info := &persistence.UpsertWorkflowSearchAttributesTask{
					WorkflowIdentifier: workflowIdentifier(workflowExecution),
					TaskData:           taskData,
				}
				startEvent, err := mutableState.GetStartEvent(context.Background())
				require.NoError(t, err)
				env.expectLoad(mutableState, nil)
				env.visibilityMgr.EXPECT().UpsertWorkflowExecution(gomock.Any(), createUpsertWorkflowSearchAttributesRequest(
					t, constants.TestDomainName, startEvent, newVisibilityTask(env, info), mutableState, 2, env.shard.GetTimeSource().Now(), false,
				)).Return(nil).Times(1)
				return info
			},
		},
		"record workflow started is skipped for a closed workflow": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				workflowExecution, mutableState, decisionCompletionID := setupWorkflow(t, env.shard)
				completeWorkflow(t, mutableState, decisionCompletionID)
				env.expectLoad(mutableState, nil)
				return &persistence.RecordWorkflowStartedTask{
					WorkflowIdentifier: workflowIdentifier(workflowExecution),
					TaskData:           taskData,
				}
			},
		},
		"record workflow closed writes the closed record without archival": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				workflowExecution, mutableState, decisionCompletionID := setupWorkflow(t, env.shard)
				completeWorkflow(t, mutableState, decisionCompletionID)
				env.expectLoad(mutableState, nil)
				env.expectVisibilityArchival(false)
				env.visibilityMgr.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, request *persistence.RecordWorkflowExecutionClosedRequest) {
						assert.Equal(t, constants.TestDomainID, request.DomainUUID)
						assert.Equal(t, workflowExecution, request.Execution)
						assert.Equal(t, int64(59), request.TaskID)
						assert.Equal(t, types.WorkflowExecutionCloseStatusCompleted, request.Status)
						assert.Equal(t, types.WorkflowExecutionStatusCompleted, request.ExecutionStatus)
					}).Return(nil).Times(1)
				return &persistence.RecordWorkflowClosedTask{
					WorkflowIdentifier: workflowIdentifier(workflowExecution),
					TaskData:           taskData,
				}
			},
		},
		"record workflow closed archives visibility when enabled": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				workflowExecution, mutableState, decisionCompletionID := setupWorkflow(t, env.shard)
				completeWorkflow(t, mutableState, decisionCompletionID)
				env.expectLoad(mutableState, nil)
				env.expectVisibilityArchival(true)
				env.visibilityMgr.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				env.archiverClient.EXPECT().Archive(gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, request *warchiver.ClientRequest) {
						assert.Equal(t, []warchiver.ArchivalTarget{warchiver.ArchiveTargetVisibility}, request.ArchiveRequest.Targets)
					}).Return(nil, nil).Times(1)
				return &persistence.RecordWorkflowClosedTask{
					WorkflowIdentifier: workflowIdentifier(workflowExecution),
					TaskData:           taskData,
				}
			},
		},
		"record workflow closed is skipped for a running workflow": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				workflowExecution, mutableState, _ := setupWorkflow(t, env.shard)
				env.expectLoad(mutableState, nil)
				return &persistence.RecordWorkflowClosedTask{
					WorkflowIdentifier: workflowIdentifier(workflowExecution),
					TaskData:           taskData,
				}
			},
		},
		"deleted workflow is skipped": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				env.expectLoad(nil, &types.EntityNotExistsError{})
				return &persistence.UpsertWorkflowSearchAttributesTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: taskData,
				}
			},
		},
		"non visibility task is rejected": {
			setup: func(t *testing.T, env *visibilityTaskExecutorTestEnv) persistence.Task {
				return &persistence.ActivityTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					TaskData: taskData,
				}
			},
			wantErr: errUnknownVisibilityTask,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			env := newVisibilityTaskExecutorTestEnv(t, ctrl)
			info := tc.setup(t, env)

			resp, err := env.executor.Execute(newVisibilityTask(env, info))

			assert.ErrorIs(t, err, tc.wantErr)
			assert.True(t, resp.IsActiveTask)
			assert.NotNil(t, resp.Scope)
		})
	}
}

func TestGetVisibilityTaskMetricsScope(t *testing.T) {
	tests := map[string]struct {
		taskType int
		want     metrics.ScopeIdx
	}{
		"record workflow started": {
			taskType: persistence.TransferTaskTypeRecordWorkflowStarted,
			want:     metrics.VisibilityTaskRecordWorkflowStartedScope,
		},
		"upsert search attributes": {
			taskType: persistence.TransferTaskTypeUpsertWorkflowSearchAttributes,
			want:     metrics.VisibilityTaskUpsertWorkflowSearchAttributesScope,
		},
		"record workflow closed": {
			taskType: persistence.TransferTaskTypeRecordWorkflowClosed,
			want:     metrics.VisibilityTaskRecordWorkflowClosedScope,
		},
		"unknown task type": {
			taskType: persistence.TransferTaskTypeActivityTask,
			want:     metrics.VisibilityQueueProcessorV2Scope,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, getVisibilityTaskMetricsScope(tc.taskType))
		})
	}
}

// setupWorkflow starts a workflow with a completed decision, and records the last event in the version history
// as persisting and reloading the mutable state would.
func setupWorkflow(t *testing.T, mockShard *shard.TestContext) (types.WorkflowExecution, execution.MutableState, int64) {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(t, mockShard, constants.TestDomainID)
	require.NoError(t, err)
	currentVersionHistory, err := mutableState.GetVersionHistories().GetCurrentVersionHistory()
	require.NoError(t, err)
	require.NoError(t, currentVersionHistory.AddOrUpdateItem(persistence.NewVersionHistoryItem(decisionCompletionID, mutableState.GetCurrentVersion())))
	return workflowExecution, mutableState, decisionCompletionID
}

// completeWorkflow closes the workflow and records the close event in the version history,
// which the close path needs to read the last write version.
func completeWorkflow(t *testing.T, mutableState execution.MutableState, decisionCompletionID int64) {
	event := test.AddCompleteWorkflowEvent(mutableState, decisionCompletionID, nil)
	currentVersionHistory, err := mutableState.GetVersionHistories().GetCurrentVersionHistory()
	require.NoError(t, err)
	require.NoError(t, currentVersionHistory.AddOrUpdateItem(persistence.NewVersionHistoryItem(event.ID, event.Version)))
}

func workflowIdentifier(workflowExecution types.WorkflowExecution) persistence.WorkflowIdentifier {
	return persistence.WorkflowIdentifier{
		DomainID:   constants.TestDomainID,
		WorkflowID: workflowExecution.GetWorkflowID(),
		RunID:      workflowExecution.GetRunID(),
	}
}
