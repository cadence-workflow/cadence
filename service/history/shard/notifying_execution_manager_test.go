package shard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
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
			wrapped.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, tc.writeErr)

			_, err := m.CreateWorkflowExecution(context.Background(), &persistence.CreateWorkflowExecutionRequest{
				NewWorkflowSnapshot: persistence.WorkflowSnapshot{TasksByCategory: timerTasks()},
			})

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
			wrapped.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, tc.writeErr)

			_, err := m.UpdateWorkflowExecution(context.Background(), &persistence.UpdateWorkflowExecutionRequest{
				UpdateWorkflowMutation: persistence.WorkflowMutation{TasksByCategory: timerTasks()},
			})

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
			wrapped.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, tc.writeErr)

			_, err := m.ConflictResolveWorkflowExecution(context.Background(), &persistence.ConflictResolveWorkflowExecutionRequest{
				ResetWorkflowSnapshot: persistence.WorkflowSnapshot{TasksByCategory: timerTasks()},
			})

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
// Each of these constructs the wrapper with an engine mock that has no expectations, so a
// notification would fail the test, and makes the inner manager return a sentinel error. That one
// assertion covers both halves of a passthrough: it reached the right inner method, and it handed
// back what that method returned.

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
	wrapped.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.GetWorkflowExecution(context.Background(), &persistence.GetWorkflowExecutionRequest{})

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
	wrapped.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.GetCurrentExecution(context.Background(), &persistence.GetCurrentExecutionRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_IsWorkflowExecutionExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.IsWorkflowExecutionExists(context.Background(), &persistence.IsWorkflowExecutionExistsRequest{})

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
	wrapped.EXPECT().GetReplicationTasksFromDLQ(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.GetReplicationTasksFromDLQ(context.Background(), &persistence.GetReplicationTasksFromDLQRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetReplicationDLQSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().GetReplicationDLQSize(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.GetReplicationDLQSize(context.Background(), &persistence.GetReplicationDLQSizeRequest{})

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
	wrapped.EXPECT().RangeDeleteReplicationTaskFromDLQ(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.RangeDeleteReplicationTaskFromDLQ(context.Background(), &persistence.RangeDeleteReplicationTaskFromDLQRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetHistoryTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.GetHistoryTasks(context.Background(), &persistence.GetHistoryTasksRequest{})

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
	wrapped.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.RangeCompleteHistoryTask(context.Background(), &persistence.RangeCompleteHistoryTaskRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_FetchWorkflowTimerTasksForCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().FetchWorkflowTimerTasksForCleanup(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.FetchWorkflowTimerTasksForCleanup(context.Background(), &persistence.FetchWorkflowTimerTasksForCleanupRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_ListConcreteExecutions(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.ListConcreteExecutions(context.Background(), &persistence.ListConcreteExecutionsRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_ListCurrentExecutions(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().ListCurrentExecutions(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := m.ListCurrentExecutions(context.Background(), &persistence.ListCurrentExecutionsRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_GetActiveClusterSelectionPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(&types.ActiveClusterSelectionPolicy{}, assert.AnError)

	_, err := m.GetActiveClusterSelectionPolicy(context.Background(), &persistence.GetActiveClusterSelectionPolicyRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func TestNotifyingExecutionManager_DeleteActiveClusterSelectionPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m, wrapped, _ := newTestNotifyingExecutionManager(t, ctrl)
	wrapped.EXPECT().DeleteActiveClusterSelectionPolicy(gomock.Any(), gomock.Any()).Return(assert.AnError)

	err := m.DeleteActiveClusterSelectionPolicy(context.Background(), &persistence.DeleteActiveClusterSelectionPolicyRequest{})

	require.ErrorIs(t, err, assert.AnError)
}

func testDomainEntry() *cache.DomainCacheEntry {
	return cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testDomainID},
		&persistence.DomainConfig{Retention: 7},
		testCluster,
	)
}

func updateRequestWithTimerTask() *persistence.UpdateWorkflowExecutionRequest {
	return &persistence.UpdateWorkflowExecutionRequest{
		RangeID: testRangeID,
		Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
		UpdateWorkflowMutation: persistence.WorkflowMutation{
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:   testDomainID,
				WorkflowID: testWorkflowID,
			},
			TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
				persistence.HistoryTaskCategoryTimer: {&persistence.DecisionTimeoutTask{}},
			},
		},
		DomainName: testDomain,
	}
}

// TestNoNotificationWhenWriteNeverAttempted pins a behavior change from moving notification behind
// the execution manager: a failure before the persistence write is reached now notifies nothing.
//
// The notify call used to live in the shard method and ran on every path once the lock was taken,
// so an allocation failure notified with an unclassified error. That counts as "possibly
// successful" and reaches the cached queue reader as a Clear(). It no longer does, which is safe:
// nothing was written, so the cache is still accurate. The cached reader's rangeID fallback does
// not cover for us here -- it reads a single-increment bump as "same host, cache still valid" --
// so the Clear() really is gone rather than arriving by another route.
func TestNoNotificationWhenWriteNeverAttempted(t *testing.T) {
	ctrl := gomock.NewController(t)
	shard := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
	defer shard.Finish(t)

	// A mock engine with no expectations: any notification fails the test.
	shard.SetEngine(engine.NewMockEngine(ctrl))
	shard.Resource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(testDomainEntry(), nil)

	// Exhaust the task ID range so allocation must renew it, and make that renewal fail with an
	// unclassified error -- the one branch that leaves the shard alive and its rangeID untouched.
	shard.contextImpl.taskSequenceNumber = shard.contextImpl.maxTaskSequenceNumber
	shard.Resource.ShardMgr.On("UpdateShard", mock.Anything, mock.Anything).Return(assert.AnError)

	_, err := shard.UpdateWorkflowExecution(context.Background(), updateRequestWithTimerTask())

	assert.ErrorIs(t, err, assert.AnError)
	// The write was never attempted, so the execution manager saw nothing either.
	shard.Resource.ExecutionMgr.AssertNotCalled(t, "UpdateWorkflowExecution", mock.Anything, mock.Anything)
	assert.NoError(t, shard.contextImpl.closedError(), "shard should stay open on an unclassified renewal error")
}

// TestNotifyPrecedesRangeRenewalOnAmbiguousError pins the other ordering change: notification
// happens inside the execution manager call, so on an ambiguous write error it now runs before the
// shard renews its range rather than after.
//
// Both still happen in the same critical section, so nothing taking the shard lock can tell. It
// matters only to the cached queue reader, which the notification clears synchronously: clearing
// before the rangeID bump and clearing after it end in the same state, since a cleared cache has
// nothing left to invalidate.
func TestNotifyPrecedesRangeRenewalOnAmbiguousError(t *testing.T) {
	ctrl := gomock.NewController(t)
	shard := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
	defer shard.Finish(t)

	var calls []string

	mockEngine := engine.NewMockEngine(ctrl)
	mockEngine.EXPECT().NotifyNewTimerTasks(gomock.Any()).Do(func(info interface{}) {
		calls = append(calls, "notify")
	}).Times(1)
	shard.SetEngine(mockEngine)

	shard.Resource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(testDomainEntry(), nil)
	shard.Resource.ExecutionMgr.
		On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
		Once().
		Return(nil, assert.AnError)
	shard.Resource.ShardMgr.
		On("UpdateShard", mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { calls = append(calls, "renewRange") }).
		Return(nil)

	_, err := shard.UpdateWorkflowExecution(context.Background(), updateRequestWithTimerTask())

	assert.ErrorIs(t, err, assert.AnError)
	require.Equal(t, []string{"notify", "renewRange"}, calls)
}
