package shard

import (
	"context"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// notifyingExecutionManager wraps persistence.ExecutionManager so that every write carrying
// history tasks notifies the transfer/timer queue processors with that write's error.
//
// The cached queue reader fills its cache only from notification payloads, and tracks a key range
// it then serves reads from. A task write that skips notification leaves the cache believing it
// covers a range it is missing tasks from. Notifying at the persistence boundary makes that the
// default: a new shard.Context method that writes tasks notifies without its author doing
// anything, because writing is only possible through here.
//
// DO NOT generate this file, and DO NOT embed persistence.ExecutionManager in the struct.
//
// The explicit passthroughs below are the mechanism, not boilerplate to tidy away. With every
// method spelled out, adding one to persistence.ExecutionManager stops this type from satisfying
// the interface, which breaks the build and drags whoever added it here to say whether their
// method carries tasks. Embedding the interface, or generating this file with gowrap like the
// metered/ratelimited/errorinjectors wrappers, hands that new method a silent non-notifying
// passthrough instead -- the exact bug this type prevents.
//
// Notification runs inside the wrapped call, so it inherits the caller's lock context.
// shard.Context does every task-carrying write under the shard lock, which is what the notifier's
// cluster-time lookup needs, and what keeps notification atomic with the max read level update
// the cached queue reader's coverage tracking relies on.
type notifyingExecutionManager struct {
	wrapped  persistence.ExecutionManager
	notifier *taskNotifier
}

var _ persistence.ExecutionManager = (*notifyingExecutionManager)(nil)

func newNotifyingExecutionManager(
	wrapped persistence.ExecutionManager,
	notifier *taskNotifier,
) *notifyingExecutionManager {
	return &notifyingExecutionManager{
		wrapped:  wrapped,
		notifier: notifier,
	}
}

// --- writes that carry history tasks: these notify ---

func (m *notifyingExecutionManager) CreateWorkflowExecution(ctx context.Context, request *persistence.CreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error) {
	resp, err := m.wrapped.CreateWorkflowExecution(ctx, request)
	m.notifier.onCreateWorkflowExecution(request, err)
	return resp, err
}

func (m *notifyingExecutionManager) UpdateWorkflowExecution(ctx context.Context, request *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error) {
	resp, err := m.wrapped.UpdateWorkflowExecution(ctx, request)
	m.notifier.onUpdateWorkflowExecution(request, err)
	return resp, err
}

func (m *notifyingExecutionManager) ConflictResolveWorkflowExecution(ctx context.Context, request *persistence.ConflictResolveWorkflowExecutionRequest) (*persistence.ConflictResolveWorkflowExecutionResponse, error) {
	resp, err := m.wrapped.ConflictResolveWorkflowExecution(ctx, request)
	m.notifier.onConflictResolveWorkflowExecution(request, err)
	return resp, err
}

func (m *notifyingExecutionManager) CreateHistoryTasks(ctx context.Context, request *persistence.CreateHistoryTasksRequest) error {
	err := m.wrapped.CreateHistoryTasks(ctx, request)
	m.notifier.onCreateHistoryTasks(request, err)
	return err
}

// --- no tasks in request: plain passthrough ---

// CreateFailoverMarkerTasks writes replication-category tasks, and notifyTasks reads the transfer
// and timer categories only, so there is nothing to notify about.
func (m *notifyingExecutionManager) CreateFailoverMarkerTasks(ctx context.Context, request *persistence.CreateFailoverMarkersRequest) error {
	return m.wrapped.CreateFailoverMarkerTasks(ctx, request)
}

func (m *notifyingExecutionManager) Close() {
	m.wrapped.Close()
}

func (m *notifyingExecutionManager) GetName() string {
	return m.wrapped.GetName()
}

func (m *notifyingExecutionManager) GetWorkflowExecution(ctx context.Context, request *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
	return m.wrapped.GetWorkflowExecution(ctx, request)
}

func (m *notifyingExecutionManager) DeleteWorkflowExecution(ctx context.Context, request *persistence.DeleteWorkflowExecutionRequest) error {
	return m.wrapped.DeleteWorkflowExecution(ctx, request)
}

func (m *notifyingExecutionManager) DeleteCurrentWorkflowExecution(ctx context.Context, request *persistence.DeleteCurrentWorkflowExecutionRequest) error {
	return m.wrapped.DeleteCurrentWorkflowExecution(ctx, request)
}

func (m *notifyingExecutionManager) GetCurrentExecution(ctx context.Context, request *persistence.GetCurrentExecutionRequest) (*persistence.GetCurrentExecutionResponse, error) {
	return m.wrapped.GetCurrentExecution(ctx, request)
}

func (m *notifyingExecutionManager) IsWorkflowExecutionExists(ctx context.Context, request *persistence.IsWorkflowExecutionExistsRequest) (*persistence.IsWorkflowExecutionExistsResponse, error) {
	return m.wrapped.IsWorkflowExecutionExists(ctx, request)
}

func (m *notifyingExecutionManager) PutReplicationTaskToDLQ(ctx context.Context, request *persistence.PutReplicationTaskToDLQRequest) error {
	return m.wrapped.PutReplicationTaskToDLQ(ctx, request)
}

func (m *notifyingExecutionManager) GetReplicationTasksFromDLQ(ctx context.Context, request *persistence.GetReplicationTasksFromDLQRequest) (*persistence.GetReplicationDLQTasksResponse, error) {
	return m.wrapped.GetReplicationTasksFromDLQ(ctx, request)
}

func (m *notifyingExecutionManager) GetReplicationDLQSize(ctx context.Context, request *persistence.GetReplicationDLQSizeRequest) (*persistence.GetReplicationDLQSizeResponse, error) {
	return m.wrapped.GetReplicationDLQSize(ctx, request)
}

func (m *notifyingExecutionManager) DeleteReplicationTaskFromDLQ(ctx context.Context, request *persistence.DeleteReplicationTaskFromDLQRequest) error {
	return m.wrapped.DeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *notifyingExecutionManager) RangeDeleteReplicationTaskFromDLQ(ctx context.Context, request *persistence.RangeDeleteReplicationTaskFromDLQRequest) (*persistence.RangeDeleteReplicationTaskFromDLQResponse, error) {
	return m.wrapped.RangeDeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *notifyingExecutionManager) GetHistoryTasks(ctx context.Context, request *persistence.GetHistoryTasksRequest) (*persistence.GetHistoryTasksResponse, error) {
	return m.wrapped.GetHistoryTasks(ctx, request)
}

func (m *notifyingExecutionManager) CompleteHistoryTask(ctx context.Context, request *persistence.CompleteHistoryTaskRequest) error {
	return m.wrapped.CompleteHistoryTask(ctx, request)
}

func (m *notifyingExecutionManager) RangeCompleteHistoryTask(ctx context.Context, request *persistence.RangeCompleteHistoryTaskRequest) (*persistence.RangeCompleteHistoryTaskResponse, error) {
	return m.wrapped.RangeCompleteHistoryTask(ctx, request)
}

func (m *notifyingExecutionManager) FetchWorkflowTimerTasksForCleanup(ctx context.Context, request *persistence.FetchWorkflowTimerTasksForCleanupRequest) ([]persistence.HistoryTaskKey, error) {
	return m.wrapped.FetchWorkflowTimerTasksForCleanup(ctx, request)
}

func (m *notifyingExecutionManager) ListConcreteExecutions(ctx context.Context, request *persistence.ListConcreteExecutionsRequest) (*persistence.ListConcreteExecutionsResponse, error) {
	return m.wrapped.ListConcreteExecutions(ctx, request)
}

func (m *notifyingExecutionManager) ListCurrentExecutions(ctx context.Context, request *persistence.ListCurrentExecutionsRequest) (*persistence.ListCurrentExecutionsResponse, error) {
	return m.wrapped.ListCurrentExecutions(ctx, request)
}

func (m *notifyingExecutionManager) GetActiveClusterSelectionPolicy(ctx context.Context, request *persistence.GetActiveClusterSelectionPolicyRequest) (*types.ActiveClusterSelectionPolicy, error) {
	return m.wrapped.GetActiveClusterSelectionPolicy(ctx, request)
}

func (m *notifyingExecutionManager) DeleteActiveClusterSelectionPolicy(ctx context.Context, request *persistence.DeleteActiveClusterSelectionPolicyRequest) error {
	return m.wrapped.DeleteActiveClusterSelectionPolicy(ctx, request)
}
