package shard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
)

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

// TestNoNotificationWhenWriteNeverAttempted pins a behavior change introduced when notification
// moved behind the execution manager: a failure that happens BEFORE the persistence write is
// reached no longer produces a notification at all.
//
// Previously the notify call lived in the shard method and ran on every path once the lock was
// taken, so an allocation failure notified with an unclassified error. That classifies as
// "possibly successful", which reaches the cached queue reader as a Clear(). That no longer
// happens, and it is safe: nothing was written, so the cache's contents remain accurate. Note the
// cached reader's rangeID fallback does NOT compensate here -- it treats a single-increment bump
// as "same host, cache still valid" -- so this really is a dropped Clear() rather than one that
// arrives by another route.
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

// TestNotifyPrecedesRangeRenewalOnAmbiguousError pins the other ordering change: notification now
// happens inside the execution manager call, so on an ambiguous write error it is emitted BEFORE
// the shard renews its range, where it previously ran after.
//
// Both still happen within the same critical section, so the ordering is invisible to anything
// taking the shard lock. It matters only to the cached queue reader, which is cleared
// synchronously by the notification: clearing before the rangeID bump and clearing after it
// converge on the same state, since a cleared cache has nothing to invalidate.
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

// TestFailoverMarkersNotifyIsNoOp covers the one write that is routed through the notification
// path purely for uniformity. Failover markers are replication-category tasks and the notifier
// reads only the transfer and timer categories, so no processor is notified. Routing it anyway
// means no task-carrying persistence write is exempt by construction.
func TestFailoverMarkersNotifyIsNoOp(t *testing.T) {
	for _, tc := range []struct {
		name     string
		writeErr error
		wantErr  error
	}{
		{name: "success"},
		{name: "ambiguous error", writeErr: assert.AnError, wantErr: assert.AnError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			shard := NewTestContext(t, ctrl, &persistence.ShardInfo{ShardID: testShardID, RangeID: testRangeID}, config.NewForTest())
			defer shard.Finish(t)

			// No expectations: the test fails if any queue processor is notified.
			shard.SetEngine(engine.NewMockEngine(ctrl))
			shard.Resource.ExecutionMgr.
				On("CreateFailoverMarkerTasks", mock.Anything, mock.Anything).
				Once().
				Return(tc.writeErr)

			err := shard.ReplicateFailoverMarkers(context.Background(), []*persistence.FailoverMarkerTask{
				{DomainID: testDomainID},
			})

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
