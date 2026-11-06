package engineimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
	"github.com/uber/cadence/service/history/execution"
	test "github.com/uber/cadence/service/history/testing"
	"github.com/uber/cadence/service/history/workflow"
)

// addEvents adds events to a test workflow's mutable state.
type addEvents func(t *testing.T, ms execution.MutableState)

func TestRecordSemaphoreAcquired(t *testing.T) {
	const (
		// The started event is 1, so the acquire's initiated event is 2.
		initiatedID   = int64(2)
		tokenID       = int32(3)
		semaphoreName = "my-semaphore"
	)
	we := types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID}

	// newState builds the stored state of a running workflow; build adds the semaphore events.
	newState := func(t *testing.T, eft *testdata.EngineForTest, build addEvents) *persistence.WorkflowMutableState {
		ms := execution.NewMutableStateBuilderWithEventV2(eft.ShardCtx, testlogger.New(t), we.RunID, constants.TestLocalDomainEntry)
		test.AddWorkflowExecutionStartedEvent(ms, we, "wType", "testTaskList", []byte("input"), 100, 200, "identity", nil)
		build(t, ms)
		state := execution.CreatePersistenceMutableState(t, ms)
		state.ExecutionInfo.DomainID = constants.TestDomainID
		return state
	}
	waiting := func(t *testing.T, ms execution.MutableState) {
		_, _, err := ms.AddSemaphoreAcquireInitiatedEvent(1, semaphoreName, 60)
		require.NoError(t, err)
	}
	granted := func(token int32) addEvents {
		return func(t *testing.T, ms execution.MutableState) {
			waiting(t, ms)
			_, err := ms.AddSemaphoreAcquiredEvent(initiatedID, token)
			require.NoError(t, err)
		}
	}
	otherSemaphore := func(t *testing.T, ms execution.MutableState) {
		_, _, err := ms.AddSemaphoreAcquireInitiatedEvent(1, "other", 60)
		require.NoError(t, err)
	}
	failed := func(t *testing.T, ms execution.MutableState) {
		waiting(t, ms)
		_, err := ms.AddSemaphoreAcquireFailedEvent(initiatedID, types.SemaphoreAcquireFailedCauseNotFound)
		require.NoError(t, err)
	}
	noEvents := func(*testing.T, execution.MutableState) {}

	tests := []struct {
		name string
		// loads is the state each load returns, in order. A second entry means the state is reloaded.
		loads         []addEvents
		runClosed     bool
		activeCluster string
		// Only a case with wantRecord mocks the update, so any write in another case fails the test.
		wantRecord bool
		wantErr    error
	}{
		{
			name:          "a domain not active in this cluster is refused",
			activeCluster: "other-cluster",
			wantErr: &types.DomainNotActiveError{
				Message:        "Domain: some random domain name is active in cluster: other-cluster, while current cluster active is a standby cluster.",
				DomainName:     constants.TestDomainName,
				CurrentCluster: cluster.TestCurrentClusterName,
				ActiveCluster:  "other-cluster",
			},
		},
		{
			name:       "a hold waiting for its first token records the grant",
			loads:      []addEvents{waiting},
			wantRecord: true,
		},
		{
			name:  "a hold that already has this token writes nothing",
			loads: []addEvents{granted(tokenID)},
		},
		{
			name:    "a hold with a different token is refused",
			loads:   []addEvents{granted(7)},
			wantErr: &types.EventAlreadyStartedError{Message: "Semaphore hold already has a different token."},
		},
		{
			name:    "a hold for a different semaphore is refused",
			loads:   []addEvents{otherSemaphore},
			wantErr: &types.EntityNotExistsError{Message: "Pending semaphore hold is for a different semaphore."},
		},
		{
			name:    "a hold that is gone is refused",
			loads:   []addEvents{failed},
			wantErr: &types.EntityNotExistsError{Message: "Pending semaphore hold not found."},
		},
		{
			name:       "a hold the cached state hasn't seen is found after a reload",
			loads:      []addEvents{noEvents, waiting},
			wantRecord: true,
		},
		{
			name:    "a hold still missing after a reload is refused",
			loads:   []addEvents{noEvents, noEvents},
			wantErr: &types.EntityNotExistsError{Message: "Pending semaphore hold not found."},
		},
		{
			name:      "a closed run is refused",
			loads:     []addEvents{waiting},
			runClosed: true,
			wantErr:   workflow.ErrAlreadyCompleted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			activeCluster := cluster.TestCurrentClusterName
			if tc.activeCluster != "" {
				activeCluster = tc.activeCluster
			}
			eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, we.WorkflowID, we.RunID).
				Return(&types.ActiveClusterInfo{ActiveClusterName: activeCluster}, nil).AnyTimes()
			for _, build := range tc.loads {
				state := newState(t, eft, build)
				if tc.runClosed {
					state.ExecutionInfo.State = persistence.WorkflowStateCompleted
					state.ExecutionInfo.CloseStatus = persistence.WorkflowCloseStatusCompleted
				}
				eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).
					Return(&persistence.GetWorkflowExecutionResponse{State: state, MutableStateStats: &persistence.MutableStateStats{}}, nil).Once()
			}

			var gotUpdate *persistence.UpdateWorkflowExecutionRequest
			if tc.wantRecord {
				eft.ShardCtx.Resource.HistoryMgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
				eft.ShardCtx.Resource.ExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) { gotUpdate = args.Get(1).(*persistence.UpdateWorkflowExecutionRequest) }).
					Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
			}

			resp, err := eft.Engine.RecordSemaphoreAcquired(context.Background(), &types.RecordSemaphoreAcquiredRequest{
				DomainUUID:        constants.TestDomainID,
				WorkflowExecution: &we,
				InitiatedID:       initiatedID,
				TokenID:           tokenID,
				SemaphoreName:     semaphoreName,
			})

			assert.Equal(t, tc.wantErr, err)
			if tc.wantErr != nil {
				assert.Nil(t, resp)
			} else {
				assert.Equal(t, &types.RecordSemaphoreAcquiredResponse{}, resp)
			}
			eft.ShardCtx.Resource.ExecutionMgr.AssertNumberOfCalls(t, "GetWorkflowExecution", len(tc.loads))
			if tc.wantRecord {
				require.NotNil(t, gotUpdate)
				mutation := gotUpdate.UpdateWorkflowMutation
				require.Len(t, mutation.UpsertSemaphoreInfos, 1)
				assert.Equal(t, int(tokenID), mutation.UpsertSemaphoreInfos[0].TokenID)
				require.NotNil(t, mutation.ExecutionInfo)
				assert.NotEqual(t, commonconstants.EmptyEventID, mutation.ExecutionInfo.DecisionScheduleID, "a decision is scheduled")
			}
		})
	}
}
