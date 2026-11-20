package engineimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
	"github.com/uber/cadence/service/history/workflow"
)

// TestAPIsOnImportInProgress checks that the APIs that change or read a workflow refuse an import
// in progress: its history is partial, and it is not active until the import completes. A request
// for a child or for a chain, such as the parent close policy, gets a not found error, so it does
// not retry.
func TestAPIsOnImportInProgress(t *testing.T) {
	workflowExecution := &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID}

	tests := map[string]struct {
		call    func(eft *testdata.EngineForTest) error
		wantErr error
	}{
		"signal": {
			call: func(eft *testdata.EngineForTest) error {
				return eft.Engine.SignalWorkflowExecution(context.Background(), &types.HistorySignalWorkflowExecutionRequest{
					DomainUUID: constants.TestDomainID,
					SignalRequest: &types.SignalWorkflowExecutionRequest{
						Domain:            constants.TestDomainName,
						WorkflowExecution: workflowExecution,
						SignalName:        "signal",
					},
				})
			},
			wantErr: workflow.ErrImportInProgress,
		},
		"request cancel": {
			call: func(eft *testdata.EngineForTest) error {
				return eft.Engine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
					DomainUUID: constants.TestDomainID,
					CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
						Domain:            constants.TestDomainName,
						WorkflowExecution: workflowExecution,
					},
				})
			},
			wantErr: workflow.ErrImportInProgress,
		},
		"request cancel by parent close policy": {
			call: func(eft *testdata.EngineForTest) error {
				return eft.Engine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
					DomainUUID: constants.TestDomainID,
					CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
						Domain:              constants.TestDomainName,
						WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID},
						FirstExecutionRunID: "child-first-run-id",
					},
					ExternalWorkflowExecution: &types.WorkflowExecution{WorkflowID: "parent-workflow-id", RunID: "parent-run-id"},
					ChildWorkflowOnly:         true,
				})
			},
			wantErr: workflow.ErrParentMismatch,
		},
		"request cancel by parent close policy worker": {
			call: func(eft *testdata.EngineForTest) error {
				return eft.Engine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
					DomainUUID: constants.TestDomainID,
					CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
						Domain:              constants.TestDomainName,
						WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID},
						FirstExecutionRunID: "child-first-run-id",
					},
				})
			},
			wantErr: &types.EntityNotExistsError{Message: "Workflow execution not found"},
		},
		"reset sticky task list": {
			call: func(eft *testdata.EngineForTest) error {
				_, err := eft.Engine.ResetStickyTaskList(context.Background(), &types.HistoryResetStickyTaskListRequest{
					DomainUUID: constants.TestDomainID,
					Execution:  workflowExecution,
				})
				return err
			},
			wantErr: workflow.ErrImportInProgress,
		},
		"terminate": {
			call: func(eft *testdata.EngineForTest) error {
				return eft.Engine.TerminateWorkflowExecution(context.Background(), &types.HistoryTerminateWorkflowExecutionRequest{
					DomainUUID: constants.TestDomainID,
					TerminateRequest: &types.TerminateWorkflowExecutionRequest{
						Domain:            constants.TestDomainName,
						WorkflowExecution: workflowExecution,
					},
				})
			},
			wantErr: workflow.ErrImportInProgress,
		},
		"terminate by parent close policy": {
			call: func(eft *testdata.EngineForTest) error {
				return eft.Engine.TerminateWorkflowExecution(context.Background(), &types.HistoryTerminateWorkflowExecutionRequest{
					DomainUUID: constants.TestDomainID,
					TerminateRequest: &types.TerminateWorkflowExecutionRequest{
						Domain:              constants.TestDomainName,
						WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID},
						FirstExecutionRunID: "child-first-run-id",
					},
					ExternalWorkflowExecution: &types.WorkflowExecution{WorkflowID: "parent-workflow-id", RunID: "parent-run-id"},
					ChildWorkflowOnly:         true,
				})
			},
			wantErr: workflow.ErrParentMismatch,
		},
		"terminate by parent close policy worker": {
			call: func(eft *testdata.EngineForTest) error {
				return eft.Engine.TerminateWorkflowExecution(context.Background(), &types.HistoryTerminateWorkflowExecutionRequest{
					DomainUUID: constants.TestDomainID,
					TerminateRequest: &types.TerminateWorkflowExecutionRequest{
						Domain:              constants.TestDomainName,
						WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID},
						FirstExecutionRunID: "child-first-run-id",
					},
				})
			},
			wantErr: &types.EntityNotExistsError{Message: "Workflow execution not found"},
		},
		"query": {
			call: func(eft *testdata.EngineForTest) error {
				_, err := eft.Engine.QueryWorkflow(context.Background(), &types.HistoryQueryWorkflowRequest{
					DomainUUID: constants.TestDomainID,
					Request: &types.QueryWorkflowRequest{
						Domain:    constants.TestDomainName,
						Execution: workflowExecution,
						Query:     &types.WorkflowQuery{QueryType: "query"},
					},
				})
				return err
			},
			wantErr: workflow.ErrImportInProgress,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, gomock.Any()).
				Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()
			// a request without a run ID, such as the parent close policy, reads the current run first
			eft.ShardCtx.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).
				Return(&persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}, nil).Maybe()
			eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).
				Return(importInProgressExecution(), nil).Once()

			err := tc.call(eft)
			assert.Equal(t, tc.wantErr, err)
			eft.ShardCtx.Resource.ExecutionMgr.AssertNotCalled(t, "UpdateWorkflowExecution", mock.Anything, mock.Anything)
		})
	}
}

// TestQueryWorkflow_ZombieImport checks that a query by run ID on a zombie import is refused. The
// zombie is not running and its history has a completed decision, so without the check the query
// goes to a worker that replays the partial history.
func TestQueryWorkflow_ZombieImport(t *testing.T) {
	eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
	eft.Engine.Start()
	defer eft.Engine.Stop()

	execution := importInProgressExecution()
	execution.State.ExecutionInfo.State = persistence.WorkflowStateZombie
	execution.State.ExecutionInfo.CloseStatus = persistence.WorkflowCloseStatusCompleted
	execution.State.ExecutionInfo.LastProcessedEvent = 3
	eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, gomock.Any()).
		Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()
	eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).
		Return(execution, nil).Once()

	_, err := eft.Engine.QueryWorkflow(context.Background(), &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Domain:    constants.TestDomainName,
			Execution: &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
			Query:     &types.WorkflowQuery{QueryType: "query"},
		},
	})
	assert.Equal(t, workflow.ErrImportInProgress, err)
}

// TestReapplyEvents_ImportInProgress checks that the reapply skips an import in progress without
// an error: the reapply would reset it into a new run, and an error makes replication retry forever.
func TestReapplyEvents_ImportInProgress(t *testing.T) {
	eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
	eft.Engine.Start()
	defer eft.Engine.Stop()

	eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, gomock.Any()).
		Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()
	eft.ShardCtx.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).
		Return(&persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}, nil).Once()
	eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).
		Return(importInProgressExecution(), nil).Once()

	signal := []*types.HistoryEvent{{ID: 1, EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(), Version: 100}}
	err := eft.Engine.ReapplyEvents(context.Background(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID, signal)
	assert.NoError(t, err)
	eft.ShardCtx.Resource.ExecutionMgr.AssertNotCalled(t, "UpdateWorkflowExecution", mock.Anything, mock.Anything)
}

// TestRefreshWorkflowTasks_ImportInProgress checks that an import in progress, current or a zombie,
// gets no refreshed tasks: they are generated when the import ends.
func TestRefreshWorkflowTasks_ImportInProgress(t *testing.T) {
	tests := map[string]struct {
		state       int
		closeStatus int
	}{
		"current import in progress": {
			state:       persistence.WorkflowStatePreparing,
			closeStatus: persistence.WorkflowCloseStatusNone,
		},
		"zombie import with a replicated close event": {
			state:       persistence.WorkflowStateZombie,
			closeStatus: persistence.WorkflowCloseStatusCompleted,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			execution := importInProgressExecution()
			execution.State.ExecutionInfo.State = tc.state
			execution.State.ExecutionInfo.CloseStatus = tc.closeStatus
			eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, gomock.Any()).
				Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()
			eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).
				Return(execution, nil).Once()

			err := eft.Engine.RefreshWorkflowTasks(context.Background(), constants.TestDomainID, types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			})
			assert.NoError(t, err)
			eft.ShardCtx.Resource.ExecutionMgr.AssertNotCalled(t, "UpdateWorkflowExecution", mock.Anything, mock.Anything)
		})
	}
}

func TestIsOpenForLongPoll(t *testing.T) {
	state := func(s int) *int32 {
		v := int32(s)
		return &v
	}

	tests := map[string]struct {
		response *types.GetMutableStateResponse
		want     bool
	}{
		"running": {
			response: &types.GetMutableStateResponse{IsWorkflowRunning: true},
			want:     true,
		},
		"import in progress": {
			response: &types.GetMutableStateResponse{WorkflowState: state(persistence.WorkflowStatePreparing)},
			want:     true,
		},
		// The imported history has the close event, so no more events come.
		"import in progress with a close status": {
			response: &types.GetMutableStateResponse{
				WorkflowState:      state(persistence.WorkflowStatePreparing),
				WorkflowCloseState: state(persistence.WorkflowCloseStatusCompleted),
			},
		},
		"completed": {
			response: &types.GetMutableStateResponse{WorkflowState: state(persistence.WorkflowStateCompleted)},
		},
		"no state": {
			response: &types.GetMutableStateResponse{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, isOpenForLongPoll(tc.response))
		})
	}
}

func TestApplyWorkflowIDReusePolicyHelper_ImportInProgress(t *testing.T) {
	workflowExecution := types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID}

	tests := map[string]struct {
		prevState int
		wantErr   error
	}{
		"running": {
			prevState: persistence.WorkflowStateRunning,
			wantErr: &types.WorkflowExecutionAlreadyStartedError{
				Message:        "Workflow execution is already running. WorkflowId: " + constants.TestWorkflowID + ", RunId: prev-run-id.",
				StartRequestID: "prev-request-id",
				RunID:          "prev-run-id",
			},
		},
		// An import in progress is treated as running, so the start is refused for any reuse policy.
		"import in progress": {
			prevState: persistence.WorkflowStatePreparing,
			wantErr: &types.WorkflowExecutionAlreadyStartedError{
				Message:        "Workflow execution is already running. WorkflowId: " + constants.TestWorkflowID + ", RunId: prev-run-id.",
				StartRequestID: "prev-request-id",
				RunID:          "prev-run-id",
			},
		},
		"completed": {
			prevState: persistence.WorkflowStateCompleted,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			engine := &historyEngineImpl{}
			err := engine.applyWorkflowIDReusePolicyHelper(
				"prev-request-id",
				"prev-run-id",
				tc.prevState,
				persistence.WorkflowCloseStatusNone,
				workflowExecution,
				types.WorkflowIDReusePolicyAllowDuplicate,
			)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func importInProgressExecution() *persistence.GetWorkflowExecutionResponse {
	return &persistence.GetWorkflowExecutionResponse{
		State: &persistence.WorkflowMutableState{
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:              constants.TestDomainID,
				WorkflowID:            constants.TestWorkflowID,
				RunID:                 constants.TestRunID,
				State:                 persistence.WorkflowStatePreparing,
				ImportSourceExecution: &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: "source-run-id"},
				ImportInProgress:      true,
				NextEventID:           5,
			},
			ExecutionStats: &persistence.ExecutionStats{},
		},
		MutableStateStats: &persistence.MutableStateStats{},
	}
}
