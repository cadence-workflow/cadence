package ndc

import (
	ctx "context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

// TestDispatchForNewWorkflow_CurrentImportInProgress checks that a newer workflow suppresses a
// current import in progress like a running workflow, so the two do not stay current together.
func TestDispatchForNewWorkflow_CurrentImportInProgress(t *testing.T) {
	controller := gomock.NewController(t)
	mockTransactionManager := NewMocktransactionManager(controller)
	createManager := newTransactionManagerForNewWorkflow(mockTransactionManager, testlogger.New(t)).(*transactionManagerForNewWorkflowImpl)
	now := time.Now()

	targetWorkflow := execution.NewMockWorkflow(controller)
	targetContext := execution.NewMockContext(controller)
	targetMutableState := execution.NewMockMutableState(controller)
	targetWorkflow.EXPECT().GetContext().Return(targetContext).AnyTimes()
	targetWorkflow.EXPECT().GetMutableState().Return(targetMutableState).AnyTimes()
	targetWorkflow.EXPECT().GetReleaseFn().Return(execution.NoopReleaseFn).AnyTimes()
	targetMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   "domain-id",
		WorkflowID: "workflow-id",
		RunID:      "target-run-id",
	}).AnyTimes()

	currentWorkflow := execution.NewMockWorkflow(controller)
	currentContext := execution.NewMockContext(controller)
	currentMutableState := execution.NewMockMutableState(controller)
	currentWorkflow.EXPECT().GetContext().Return(currentContext).AnyTimes()
	currentWorkflow.EXPECT().GetMutableState().Return(currentMutableState).AnyTimes()
	currentWorkflow.EXPECT().GetReleaseFn().Return(execution.NoopReleaseFn).AnyTimes()
	currentMutableState.EXPECT().IsWorkflowExecutionRunning().Return(false).AnyTimes()
	currentMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:         "domain-id",
		WorkflowID:       "workflow-id",
		State:            persistence.WorkflowStatePreparing,
		ImportInProgress: true,
	}).AnyTimes()

	mockTransactionManager.EXPECT().getCurrentWorkflowRunID(gomock.Any(), "domain-id", "workflow-id").Return("current-run-id", nil).Times(1)
	mockTransactionManager.EXPECT().loadNDCWorkflow(gomock.Any(), "domain-id", "workflow-id", "current-run-id").Return(currentWorkflow, nil).Times(1)
	targetWorkflow.EXPECT().HappensAfter(currentWorkflow).Return(true, nil)
	currentWorkflow.EXPECT().SuppressBy(targetWorkflow).Return(execution.TransactionPolicyPassive, nil).Times(1)
	targetWorkflow.EXPECT().Revive().Return(nil).Times(1)
	currentContext.EXPECT().UpdateWorkflowExecutionWithNew(
		gomock.Any(),
		now,
		persistence.UpdateWorkflowModeUpdateCurrent,
		targetContext,
		targetMutableState,
		execution.TransactionPolicyPassive,
		execution.TransactionPolicyPassive.Ptr(),
		persistence.CreateWorkflowRequestModeReplicated,
	).Return(nil).Times(1)

	require.NoError(t, createManager.dispatchForNewWorkflow(ctx.Background(), now, targetWorkflow))
}

// TestDispatchForNewWorkflow_CreateImportAsZombie checks that an imported run created as a zombie
// does not reapply its events: the source cluster already handled them.
func TestDispatchForNewWorkflow_CreateImportAsZombie(t *testing.T) {
	controller := gomock.NewController(t)
	mockTransactionManager := NewMocktransactionManager(controller)
	createManager := newTransactionManagerForNewWorkflow(mockTransactionManager, testlogger.New(t)).(*transactionManagerForNewWorkflowImpl)
	now := time.Now()

	targetWorkflow := execution.NewMockWorkflow(controller)
	targetContext := execution.NewMockContext(controller)
	targetMutableState := execution.NewMockMutableState(controller)
	targetWorkflow.EXPECT().GetContext().Return(targetContext).AnyTimes()
	targetWorkflow.EXPECT().GetMutableState().Return(targetMutableState).AnyTimes()
	targetWorkflow.EXPECT().GetReleaseFn().Return(execution.NoopReleaseFn).AnyTimes()
	targetMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:         "domain-id",
		WorkflowID:       "workflow-id",
		RunID:            "target-run-id",
		State:            persistence.WorkflowStateZombie,
		ImportInProgress: true,
	}).AnyTimes()

	currentWorkflow := execution.NewMockWorkflow(controller)
	currentWorkflow.EXPECT().GetReleaseFn().Return(execution.NoopReleaseFn).AnyTimes()

	targetWorkflowSnapshot := &persistence.WorkflowSnapshot{}
	targetWorkflowEventsSeq := []*persistence.WorkflowEvents{{
		Events: []*types.HistoryEvent{
			{EventType: types.EventTypeWorkflowExecutionStarted.Ptr()},
			{EventType: types.EventTypeWorkflowExecutionSignaled.Ptr()},
		},
	}}
	targetWorkflowHistory := events.PersistedBlob{}

	mockTransactionManager.EXPECT().getCurrentWorkflowRunID(gomock.Any(), "domain-id", "workflow-id").Return("current-run-id", nil).Times(1)
	mockTransactionManager.EXPECT().loadNDCWorkflow(gomock.Any(), "domain-id", "workflow-id", "current-run-id").Return(currentWorkflow, nil).Times(1)
	targetWorkflow.EXPECT().HappensAfter(currentWorkflow).Return(false, nil)
	targetWorkflow.EXPECT().SuppressBy(currentWorkflow).Return(execution.TransactionPolicyPassive, nil).Times(1)
	targetMutableState.EXPECT().CloseTransactionAsSnapshot(now, execution.TransactionPolicyPassive).Return(targetWorkflowSnapshot, targetWorkflowEventsSeq, nil).Times(1)
	targetContext.EXPECT().PersistStartWorkflowBatchEvents(gomock.Any(), targetWorkflowEventsSeq[0]).Return(targetWorkflowHistory, nil).Times(1)
	// no ReapplyEvents call is expected
	targetContext.EXPECT().CreateWorkflowExecution(
		gomock.Any(),
		targetWorkflowSnapshot,
		targetWorkflowHistory,
		persistence.CreateWorkflowModeZombie,
		"",
		int64(0),
		persistence.CreateWorkflowRequestModeReplicated,
	).Return(nil).Times(1)

	require.NoError(t, createManager.dispatchForNewWorkflow(ctx.Background(), now, targetWorkflow))
}

// TestSuppressCurrentAndUpdateAsCurrent_CurrentImportInProgress checks that a current import in
// progress is suppressed like a running workflow when an existing workflow becomes current.
func TestSuppressCurrentAndUpdateAsCurrent_CurrentImportInProgress(t *testing.T) {
	controller := gomock.NewController(t)
	updateManager := newTransactionManagerForExistingWorkflow(NewMocktransactionManager(controller), testlogger.New(t)).(*transactionManagerForExistingWorkflowImpl)
	now := time.Now()

	targetWorkflow := execution.NewMockWorkflow(controller)
	targetContext := execution.NewMockContext(controller)
	targetMutableState := execution.NewMockMutableState(controller)
	targetWorkflow.EXPECT().GetContext().Return(targetContext).AnyTimes()
	targetWorkflow.EXPECT().GetMutableState().Return(targetMutableState).AnyTimes()

	currentWorkflow := execution.NewMockWorkflow(controller)
	currentContext := execution.NewMockContext(controller)
	currentMutableState := execution.NewMockMutableState(controller)
	currentWorkflow.EXPECT().GetContext().Return(currentContext).AnyTimes()
	currentWorkflow.EXPECT().GetMutableState().Return(currentMutableState).AnyTimes()
	currentMutableState.EXPECT().IsWorkflowExecutionRunning().Return(false).AnyTimes()
	currentMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		State:            persistence.WorkflowStatePreparing,
		ImportInProgress: true,
	}).AnyTimes()

	currentWorkflow.EXPECT().SuppressBy(targetWorkflow).Return(execution.TransactionPolicyPassive, nil).Times(1)
	targetWorkflow.EXPECT().Revive().Return(nil).Times(1)
	targetContext.EXPECT().ConflictResolveWorkflowExecution(
		gomock.Any(),
		now,
		persistence.ConflictResolveWorkflowModeUpdateCurrent,
		targetMutableState,
		nil,
		nil,
		currentContext,
		currentMutableState,
		execution.TransactionPolicyPassive.Ptr(),
	).Return(nil).Times(1)

	require.NoError(t, updateManager.suppressCurrentAndUpdateAsCurrent(ctx.Background(), now, currentWorkflow, targetWorkflow, nil))
}

// TestConflictResolverRebuild_ImportInProgress checks that the rebuild of an import in progress,
// current or a zombie, keeps the import state and the import fields, keeps the close status of the
// replay, and removes the tasks that the rebuild refreshed.
func TestConflictResolverRebuild_ImportInProgress(t *testing.T) {
	tests := map[string]struct {
		state int
	}{
		"current import in progress": {state: persistence.WorkflowStatePreparing},
		"zombie import":              {state: persistence.WorkflowStateZombie},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mockShard := shard.NewTestContext(t, controller, &persistence.ShardInfo{ShardID: 10, RangeID: 1}, config.NewForTest())
			defer mockShard.Finish(t)
			mockContext := execution.NewMockContext(controller)
			mockMutableState := execution.NewMockMutableState(controller)
			mockStateRebuilder := execution.NewMockStateRebuilder(controller)
			resolver := newConflictResolver(mockShard, mockContext, mockMutableState, mockShard.GetLogger()).(*conflictResolverImpl)
			resolver.stateRebuilder = mockStateRebuilder

			version := int64(12)
			versionHistory0 := persistence.NewVersionHistory([]byte("branch-0"), []*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(5, version)})
			versionHistory1 := persistence.NewVersionHistory([]byte("branch-1"), []*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(2, version)})
			versionHistories := persistence.NewVersionHistories(versionHistory0)
			_, _, err := versionHistories.AddVersionHistory(versionHistory1)
			require.NoError(t, err)

			source := &types.WorkflowExecution{WorkflowID: "source-workflow-id", RunID: "source-run-id"}
			importTimestamp := time.Unix(0, 1000)
			mockMutableState.EXPECT().GetUpdateCondition().Return(int64(59)).AnyTimes()
			mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
			mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
				DomainID:              "domain-id",
				WorkflowID:            "workflow-id",
				RunID:                 "run-id",
				State:                 tc.state,
				CloseStatus:           persistence.WorkflowCloseStatusNone,
				ImportSourceExecution: source,
				ImportTimestamp:       importTimestamp,
				ImportInProgress:      true,
			}).AnyTimes()

			// the replay of the new current branch has the close event
			rebuildInfo := &persistence.WorkflowExecutionInfo{
				State:       persistence.WorkflowStateCompleted,
				CloseStatus: persistence.WorkflowCloseStatusCompleted,
			}
			mockRebuildMutableState := execution.NewMockMutableState(controller)
			mockRebuildMutableState.EXPECT().GetVersionHistories().Return(persistence.NewVersionHistories(versionHistory1.Duplicate())).Times(1)
			mockRebuildMutableState.EXPECT().SetVersionHistories(versionHistories).Return(nil).Times(1)
			mockRebuildMutableState.EXPECT().SetUpdateCondition(int64(59)).Times(1)
			mockRebuildMutableState.EXPECT().GetExecutionInfo().Return(rebuildInfo).AnyTimes()
			mockRebuildMutableState.EXPECT().DeleteTransferTasks().Times(1)
			mockRebuildMutableState.EXPECT().DeleteTimerTasks().Times(1)
			mockStateRebuilder.EXPECT().Rebuild(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mockRebuildMutableState, int64(100), nil).Times(1)
			mockContext.EXPECT().Clear().Times(1)
			mockContext.EXPECT().SetHistorySize(int64(100)).Times(1)

			_, err = resolver.rebuild(ctx.Background(), 1, "request-id")
			require.NoError(t, err)
			assert.Equal(t, &persistence.WorkflowExecutionInfo{
				State:                 tc.state,
				CloseStatus:           persistence.WorkflowCloseStatusCompleted,
				ImportSourceExecution: source,
				ImportTimestamp:       importTimestamp,
				ImportInProgress:      true,
			}, rebuildInfo)
		})
	}
}

// TestBackfillWorkflow_ImportInProgress checks that the backfill of an import in progress, current
// or a zombie, reapplies no events: the source cluster already handled them, and a reset of the
// import (case 1.b) would create a new run.
func TestBackfillWorkflow_ImportInProgress(t *testing.T) {
	tests := map[string]struct {
		state      int
		isCurrent  bool
		wantMode   persistence.UpdateWorkflowMode
		currentRun string
	}{
		"current import in progress": {
			state:     persistence.WorkflowStatePreparing,
			isCurrent: true,
			wantMode:  persistence.UpdateWorkflowModeUpdateCurrent,
		},
		"zombie import": {
			state:      persistence.WorkflowStateZombie,
			isCurrent:  false,
			wantMode:   persistence.UpdateWorkflowModeBypassCurrent,
			currentRun: "current-run-id",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mockShard := shard.NewTestContext(t, controller, &persistence.ShardInfo{ShardID: 10, RangeID: 1}, config.NewForTest())
			defer mockShard.Finish(t)
			mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&types.ActiveClusterInfo{ActiveClusterName: mockShard.GetClusterMetadata().GetCurrentClusterName()}, nil).AnyTimes()
			// no events reapplier call is expected
			transactionManager := newTransactionManager(mockShard, execution.NewCache(mockShard), NewMockEventsReapplier(controller), mockShard.GetLogger())
			now := time.Now()

			workflow := execution.NewMockWorkflow(controller)
			context := execution.NewMockContext(controller)
			mutableState := execution.NewMockMutableState(controller)
			workflow.EXPECT().GetContext().Return(context).AnyTimes()
			workflow.EXPECT().GetMutableState().Return(mutableState).AnyTimes()
			workflow.EXPECT().GetReleaseFn().Return(execution.NoopReleaseFn).AnyTimes()
			mutableState.EXPECT().IsCurrentWorkflowGuaranteed().Return(tc.isCurrent).AnyTimes()
			mutableState.EXPECT().IsWorkflowExecutionRunning().Return(false).AnyTimes()
			mutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
				DomainID:         "domain-id",
				WorkflowID:       "workflow-id",
				RunID:            "run-id",
				State:            tc.state,
				ImportInProgress: true,
			}).AnyTimes()
			if !tc.isCurrent {
				mockShard.Resource.DomainCache.EXPECT().GetDomainName("domain-id").Return("domain", nil).AnyTimes()
				mockShard.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, &persistence.GetCurrentExecutionRequest{
					ShardID:    common.Ptr(10),
					DomainID:   "domain-id",
					WorkflowID: "workflow-id",
					DomainName: "domain",
				}).Return(&persistence.GetCurrentExecutionResponse{RunID: tc.currentRun}, nil).Once()
			}

			workflowEvents := &persistence.WorkflowEvents{
				Events:     []*types.HistoryEvent{{ID: 5, EventType: types.EventTypeWorkflowExecutionSignaled.Ptr()}},
				DomainID:   "domain-id",
				WorkflowID: "workflow-id",
			}
			// no context ReapplyEvents call is expected
			context.EXPECT().PersistNonStartWorkflowBatchEvents(gomock.Any(), workflowEvents).Return(events.PersistedBlob{}, nil).Times(1)
			context.EXPECT().UpdateWorkflowExecutionWithNew(
				gomock.Any(), now, tc.wantMode, nil, nil, execution.TransactionPolicyPassive, (*execution.TransactionPolicy)(nil), persistence.CreateWorkflowRequestModeReplicated,
			).Return(nil).Times(1)

			assert.NoError(t, transactionManager.backfillWorkflow(ctx.Background(), now, workflow, workflowEvents))
		})
	}
}
