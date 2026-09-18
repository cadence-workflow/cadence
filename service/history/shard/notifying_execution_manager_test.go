package shard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
)

// One test per wrapper method, in the order the wrapper declares them. The file mirrors the wrapper
// one for one, so a method that arrived without a test is easy to spot.

// newTestNotifyingExecutionManager builds the wrapper over mocks, with no shard involved. The
// notifier's two function fields are stubbed: the engine is fixed, and cluster times are irrelevant
// because these tests use tasks with no failover version.
func newTestNotifyingExecutionManager(t *testing.T, ctrl *gomock.Controller) (
	*notifyingExecutionManager, *persistence.MockExecutionManager, *engine.MockEngine,
) {
	wrapped := persistence.NewMockExecutionManager(ctrl)
	mockEngine := engine.NewMockEngine(ctrl)
	notifier := newTaskNotifier(
		testShardID,
		config.NewForTest(),
		testlogger.New(t),
		func() engine.Engine { return mockEngine },
		func([]persistence.Task) map[string]time.Time { return nil },
	)
	return newNotifyingExecutionManager(wrapped, notifier), wrapped, mockEngine
}

func timerTasks() map[persistence.HistoryTaskCategory][]persistence.Task {
	return map[persistence.HistoryTaskCategory][]persistence.Task{
		persistence.HistoryTaskCategoryTimer: {&persistence.DecisionTimeoutTask{}},
	}
}

// writeOutcomes are the three cases every task-carrying method has to get right. An ambiguous error
// means the write may still have landed, so the processors are told and warned to expect
// duplicates; a definitive one means nothing was written, so there is nothing to tell them about.
var writeOutcomes = map[string]struct {
	writeErr             error
	wantNotify           bool
	wantPersistenceError bool
}{
	"success":          {wantNotify: true},
	"ambiguous error":  {writeErr: assert.AnError, wantNotify: true, wantPersistenceError: true},
	"definitive error": {writeErr: &persistence.ConditionFailedError{}},
}

// expectNotify sets the engine expectation for one outcome. Every notifying test sends a request
// carrying exactly one timer task, so that is what the notification should hold.
func expectNotify(t *testing.T, mockEngine *engine.MockEngine, wantNotify, wantPersistenceError bool) {
	if !wantNotify {
		// No expectation: any notification fails the test.
		return
	}
	mockEngine.EXPECT().NotifyNewTimerTasks(gomock.Any()).Do(func(info *hcommon.NotifyTaskInfo) {
		assert.Len(t, info.Tasks, 1)
		assert.Equal(t, wantPersistenceError, info.PersistenceError)
	}).Times(1)
}

func requireSameError(t *testing.T, want, got error) {
	if want == nil {
		require.NoError(t, got)
		return
	}
	require.ErrorIs(t, got, want)
}

// --- writes that carry history tasks ---

func TestNotifyingExecutionManager_CreateWorkflowExecution(t *testing.T) {
	for name, tc := range writeOutcomes {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m, wrapped, mockEngine := newTestNotifyingExecutionManager(t, ctrl)
			expectNotify(t, mockEngine, tc.wantNotify, tc.wantPersistenceError)
			want := &persistence.CreateWorkflowExecutionResponse{}
			wrapped.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(want, tc.writeErr)

			got, err := m.CreateWorkflowExecution(context.Background(), &persistence.CreateWorkflowExecutionRequest{
				NewWorkflowSnapshot: persistence.WorkflowSnapshot{TasksByCategory: timerTasks()},
			})

			assert.Same(t, want, got)
			requireSameError(t, tc.writeErr, err)
		})
	}
}

func TestNotifyingExecutionManager_UpdateWorkflowExecution(t *testing.T) {
	for name, tc := range writeOutcomes {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m, wrapped, mockEngine := newTestNotifyingExecutionManager(t, ctrl)
			expectNotify(t, mockEngine, tc.wantNotify, tc.wantPersistenceError)
			want := &persistence.UpdateWorkflowExecutionResponse{}
			wrapped.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(want, tc.writeErr)

			got, err := m.UpdateWorkflowExecution(context.Background(), &persistence.UpdateWorkflowExecutionRequest{
				UpdateWorkflowMutation: persistence.WorkflowMutation{TasksByCategory: timerTasks()},
			})

			assert.Same(t, want, got)
			requireSameError(t, tc.writeErr, err)
		})
	}
}

func TestNotifyingExecutionManager_ConflictResolveWorkflowExecution(t *testing.T) {
	for name, tc := range writeOutcomes {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m, wrapped, mockEngine := newTestNotifyingExecutionManager(t, ctrl)
			expectNotify(t, mockEngine, tc.wantNotify, tc.wantPersistenceError)
			want := &persistence.ConflictResolveWorkflowExecutionResponse{}
			wrapped.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).Return(want, tc.writeErr)

			got, err := m.ConflictResolveWorkflowExecution(context.Background(), &persistence.ConflictResolveWorkflowExecutionRequest{
				ResetWorkflowSnapshot: persistence.WorkflowSnapshot{TasksByCategory: timerTasks()},
			})

			assert.Same(t, want, got)
			requireSameError(t, tc.writeErr, err)
		})
	}
}

func TestNotifyingExecutionManager_CreateHistoryTasks(t *testing.T) {
	for name, tc := range writeOutcomes {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m, wrapped, mockEngine := newTestNotifyingExecutionManager(t, ctrl)
			expectNotify(t, mockEngine, tc.wantNotify, tc.wantPersistenceError)
			wrapped.EXPECT().CreateHistoryTasks(gomock.Any(), gomock.Any()).Return(tc.writeErr)

			err := m.CreateHistoryTasks(context.Background(), &persistence.CreateHistoryTasksRequest{
				TasksByCategory: timerTasks(),
			})

			requireSameError(t, tc.writeErr, err)
		})
	}
}

// --- passthroughs ---
//
// The engine mock carries no expectations, so a notification here fails the test. The inner manager
// returns a sentinel error, plus a response value for the methods that have one, and each test
// checks both come back untouched. A passthrough wired to the wrong inner method fails on the
// missing call; one that drops the inner result fails on the value or the error.

// CreateFailoverMarkerTasks is the one passthrough that does write tasks. They are
// replication-category, and the notifier reads the transfer and timer categories only.
func TestNotifyingExecutionManager_CreateFailoverMarkerTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().CreateFailoverMarkerTasks(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.CreateFailoverMarkerTasks(context.Background(), &persistence.CreateFailoverMarkersRequest{
		Markers: []*persistence.FailoverMarkerTask{{DomainID: testDomainID}},
	})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().Close().Times(1)

	m.Close()
}

func TestNotifyingExecutionManager_GetName(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().GetName().Return("inner")

	assert.Equal(t, "inner", m.GetName())
}

func TestNotifyingExecutionManager_GetWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.GetWorkflowExecutionResponse{}
	wrapped.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.GetWorkflowExecution(context.Background(), &persistence.GetWorkflowExecutionRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_DeleteWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.DeleteWorkflowExecution(context.Background(), &persistence.DeleteWorkflowExecutionRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_DeleteCurrentWorkflowExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.DeleteCurrentWorkflowExecution(context.Background(), &persistence.DeleteCurrentWorkflowExecutionRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetCurrentExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.GetCurrentExecutionResponse{}
	wrapped.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.GetCurrentExecution(context.Background(), &persistence.GetCurrentExecutionRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_IsWorkflowExecutionExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.IsWorkflowExecutionExistsResponse{}
	wrapped.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.IsWorkflowExecutionExists(context.Background(), &persistence.IsWorkflowExecutionExistsRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_PutReplicationTaskToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().PutReplicationTaskToDLQ(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.PutReplicationTaskToDLQ(context.Background(), &persistence.PutReplicationTaskToDLQRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetReplicationTasksFromDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.GetReplicationDLQTasksResponse{}
	wrapped.EXPECT().GetReplicationTasksFromDLQ(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.GetReplicationTasksFromDLQ(context.Background(), &persistence.GetReplicationTasksFromDLQRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetReplicationDLQSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.GetReplicationDLQSizeResponse{}
	wrapped.EXPECT().GetReplicationDLQSize(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.GetReplicationDLQSize(context.Background(), &persistence.GetReplicationDLQSizeRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_DeleteReplicationTaskFromDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().DeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.DeleteReplicationTaskFromDLQ(context.Background(), &persistence.DeleteReplicationTaskFromDLQRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_RangeDeleteReplicationTaskFromDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.RangeDeleteReplicationTaskFromDLQResponse{}
	wrapped.EXPECT().RangeDeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.RangeDeleteReplicationTaskFromDLQ(context.Background(), &persistence.RangeDeleteReplicationTaskFromDLQRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetHistoryTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.GetHistoryTasksResponse{}
	wrapped.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.GetHistoryTasks(context.Background(), &persistence.GetHistoryTasksRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_CompleteHistoryTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().CompleteHistoryTask(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.CompleteHistoryTask(context.Background(), &persistence.CompleteHistoryTaskRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_RangeCompleteHistoryTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.RangeCompleteHistoryTaskResponse{}
	wrapped.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.RangeCompleteHistoryTask(context.Background(), &persistence.RangeCompleteHistoryTaskRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_FetchWorkflowTimerTasksForCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := []persistence.HistoryTaskKey{persistence.NewImmediateTaskKey(1)}
	wrapped.EXPECT().FetchWorkflowTimerTasksForCleanup(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.FetchWorkflowTimerTasksForCleanup(context.Background(), &persistence.FetchWorkflowTimerTasksForCleanupRequest{})

	assert.Equal(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_ListConcreteExecutions(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.ListConcreteExecutionsResponse{}
	wrapped.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.ListConcreteExecutions(context.Background(), &persistence.ListConcreteExecutionsRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_ListCurrentExecutions(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &persistence.ListCurrentExecutionsResponse{}
	wrapped.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.ListCurrentExecutions(context.Background(), &persistence.ListCurrentExecutionsRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetActiveClusterSelectionPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	want := &types.ActiveClusterSelectionPolicy{}
	wrapped.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(want, assert.AnError)

	got, err := m.GetActiveClusterSelectionPolicy(context.Background(), &persistence.GetActiveClusterSelectionPolicyRequest{})

	assert.Same(t, want, got)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_DeleteActiveClusterSelectionPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().DeleteActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.DeleteActiveClusterSelectionPolicy(context.Background(), &persistence.DeleteActiveClusterSelectionPolicyRequest{})

	require.ErrorIs(t, err, assert.AnError)
}
