package semaphore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
	"github.com/uber/cadence/common/types"
)

var (
	testTaskOwner  = commonsemaphore.Owner{WorkflowID: "wf", RunID: "run", HoldID: 7}
	releaseApplied = &persistence.ReleaseSemaphoreTokenResponse{Applied: true}
)

// startGranterTestManager returns a started manager that doesn't idle out as tests move the clock,
// and accepts ack level saves.
func startGranterTestManager(t *testing.T, tokenCount int, rows []*persistence.SemaphoreOwnership) (*semaphoreManagerImpl, *mockDeps) {
	t.Helper()
	mgr, deps := startTestManager(t, tokenCount, rows, func(p *ManagerParams) { p.Cfg = testConfig(24 * time.Hour) })
	deps.mockTaskManager.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&persistence.UpdateSemaphoreTaskBucketStateResponse{}, nil)
	return mgr, deps
}

// testTask is a task for testTaskOwner with deadline.
func testTask(deadline *time.Time) *persistence.SemaphoreTask {
	return &persistence.SemaphoreTask{
		TaskID:          1,
		WorkflowID:      testTaskOwner.WorkflowID,
		RunID:           testTaskOwner.RunID,
		HoldID:          testTaskOwner.HoldID,
		AcquireDeadline: deadline,
	}
}

// expectRecordGrant expects one call telling History that testTaskOwner holds tokenID.
func expectRecordGrant(h *history.MockClient, tokenID int) *gomock.Call {
	return h.EXPECT().RecordSemaphoreAcquired(gomock.Any(), &types.RecordSemaphoreAcquiredRequest{
		DomainUUID:        testBucketID.DomainID,
		WorkflowExecution: &types.WorkflowExecution{WorkflowID: testTaskOwner.WorkflowID, RunID: testTaskOwner.RunID},
		InitiatedID:       testTaskOwner.HoldID,
		TokenID:           int32(tokenID),
		SemaphoreName:     testBucketID.SemaphoreName,
	}).Times(1)
}

// expectRefreshTasks expects one call asking History to rebuild testTaskOwner's run's tasks.
func expectRefreshTasks(h *history.MockClient) *gomock.Call {
	return h.EXPECT().RefreshWorkflowTasks(gomock.Any(), &types.HistoryRefreshWorkflowTasksRequest{
		DomainUIID: testBucketID.DomainID,
		Request: &types.RefreshWorkflowTasksRequest{
			Execution: &types.WorkflowExecution{WorkflowID: testTaskOwner.WorkflowID, RunID: testTaskOwner.RunID},
		},
	}).Times(1)
}

// runGrantTask runs grantTask for task, advancing the clock so every wait ends, and returns its
// result.
func runGrantTask(t *testing.T, mgr *semaphoreManagerImpl, deps *mockDeps, task *persistence.SemaphoreTask) bool {
	t.Helper()
	done := make(chan bool, 1)
	go func() { done <- mgr.grantTask(context.Background(), task) }()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case finished := <-done:
			return finished
		case <-time.After(time.Millisecond):
			deps.mockTimeSource.Advance(taskRetryMaxInterval)
		case <-timeout:
			t.Fatal("grantTask did not return")
		}
	}
}

func TestGrantTask(t *testing.T) {
	ownerID := testTaskOwner.String()
	applied := &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}
	// heldByOwner is the bucket's only token, held by the task's owner since before the load.
	heldByOwner := []*persistence.SemaphoreOwnership{tokenRow(1, ownerID), ownerRow(ownerID, 1)}
	// fullBucket is the bucket's only token, held by another owner.
	fullBucket := []*persistence.SemaphoreOwnership{tokenRow(1, "other-owner"), ownerRow("other-owner", 1)}

	testCases := []struct {
		name string
		// rows are what the startup scan reads. Nil leaves the bucket's only token free.
		rows []*persistence.SemaphoreOwnership
		// deadline is the task's deadline, from the load time. Nil means no deadline.
		deadline *time.Duration
		// setupMocks expects the calls, in order.
		setupMocks func(deps *mockDeps)
	}{
		{
			name: "granted, History records it",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
				)
			},
		},
		{
			name: "History fails, retried until it records the grant, the token is not freed meanwhile",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, assert.AnError),
					expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
				)
			},
		},
		{
			name: "the hold is gone, the token is freed",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.EntityNotExistsError{}),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(releaseApplied, nil),
				)
			},
		},
		{
			name: "the run closed, the token is freed",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.WorkflowExecutionAlreadyCompletedError{}),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(releaseApplied, nil),
				)
			},
		},
		{
			name: "the hold has another token, the token is freed",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.EventAlreadyStartedError{}),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(releaseApplied, nil),
				)
			},
		},
		{
			name: "the freeing fails, retried from the owner row",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.EntityNotExistsError{}),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(nil, assert.AnError),
					expectGetOwnerRow(deps.mockTokenManager, ownerID).
						Return(&persistence.GetSemaphoreOwnershipByOwnerResponse{Ownership: ownerRow(ownerID, 1)}, nil),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(releaseApplied, nil),
				)
			},
		},
		{
			name: "the release keeps failing, given up after taskReleaseTimeout",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.EntityNotExistsError{}),
					// The token row does not name the owner, but the owner row names the token.
					deps.mockTokenManager.EXPECT().ReleaseSemaphoreToken(gomock.Any(), gomock.Any()).MinTimes(1).
						Return(&persistence.ReleaseSemaphoreTokenResponse{Applied: false}, nil),
				)
				expectGetOwnerRow(deps.mockTokenManager, ownerID).MinTimes(1).
					Return(&persistence.GetSemaphoreOwnershipByOwnerResponse{Ownership: ownerRow(ownerID, 1)}, nil)
			},
		},
		{
			name: "the domain is not active, tasks refreshed, the token is freed",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.DomainNotActiveError{}),
					expectRefreshTasks(deps.mockHistory).Return(nil),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(releaseApplied, nil),
				)
			},
		},
		{
			name: "the domain is not active and the run is gone, the token is freed",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.DomainNotActiveError{}),
					expectRefreshTasks(deps.mockHistory).Return(&types.EntityNotExistsError{}),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(releaseApplied, nil),
				)
			},
		},
		{
			name: "the domain is not active and the refresh fails, the grant is sent again",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.DomainNotActiveError{}),
					expectRefreshTasks(deps.mockHistory).Return(assert.AnError),
					expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
				)
			},
		},
		{
			name: "the grant write fails, retried",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(nil, assert.AnError),
					expectGrantToken(deps.mockTokenManager, 1, ownerID).Return(applied, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
				)
			},
		},
		{
			name: "holds a token from before the load, sent to History again",
			rows: heldByOwner,
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetTokenRow(deps.mockTokenManager, 1).Return(tokenRowRead(1, ownerID), nil),
					expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
				)
			},
		},
		{
			name:     "expired with no owner row, finished",
			deadline: common.DurationPtr(0),
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager, ownerID).Return(nil, &types.EntityNotExistsError{})
			},
		},
		{
			name:     "expired holding a token from before the load, sent to History, not freed",
			rows:     heldByOwner,
			deadline: common.DurationPtr(0),
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetTokenRow(deps.mockTokenManager, 1).Return(tokenRowRead(1, ownerID), nil),
					expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
				)
			},
		},
		{
			name:     "expired holding a token History refuses, the token is freed",
			rows:     heldByOwner,
			deadline: common.DurationPtr(0),
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetTokenRow(deps.mockTokenManager, 1).Return(tokenRowRead(1, ownerID), nil),
					expectRecordGrant(deps.mockHistory, 1).Return(nil, &types.EntityNotExistsError{}),
					expectReleaseToken(deps.mockTokenManager, 1, ownerID).Return(releaseApplied, nil),
				)
			},
		},
		{
			name:     "expired, the owner row names a token the cache missed, sent to History",
			deadline: common.DurationPtr(0),
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetOwnerRow(deps.mockTokenManager, ownerID).
						Return(&persistence.GetSemaphoreOwnershipByOwnerResponse{Ownership: ownerRow(ownerID, 1)}, nil),
					expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
				)
			},
		},
		{
			name:     "the owner row read fails, retried",
			deadline: common.DurationPtr(0),
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetOwnerRow(deps.mockTokenManager, ownerID).Return(nil, assert.AnError),
					expectGetOwnerRow(deps.mockTokenManager, ownerID).Return(nil, &types.EntityNotExistsError{}),
				)
			},
		},
		{
			name:     "the owner row names a token outside the bucket, retried",
			deadline: common.DurationPtr(0),
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetOwnerRow(deps.mockTokenManager, ownerID).
						Return(&persistence.GetSemaphoreOwnershipByOwnerResponse{Ownership: ownerRow(ownerID, 5)}, nil),
					expectGetOwnerRow(deps.mockTokenManager, ownerID).Return(nil, &types.EntityNotExistsError{}),
				)
			},
		},
		{
			name:     "no free token until the deadline, no owner row, finished",
			rows:     fullBucket,
			deadline: common.DurationPtr(time.Minute),
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager, ownerID).Return(nil, &types.EntityNotExistsError{})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startGranterTestManager(t, 1, tc.rows)
			var deadline *time.Time
			if tc.deadline != nil {
				deadline = common.TimePtr(deps.mockTimeSource.Now().Add(*tc.deadline))
			}
			tc.setupMocks(deps)

			assert.True(t, runGrantTask(t, mgr, deps, testTask(deadline)))
			assertBucketUnlocked(t, mgr)
		})
	}
}

func TestGrantTask_StaleCacheWaitsForReload(t *testing.T) {
	ownerID := testTaskOwner.String()
	mgr, deps := startGranterTestManager(t, 2, nil)
	var refusedToken int
	gomock.InOrder(
		// The bucket was just loaded, so the refused grant is not followed by a reload.
		deps.mockTokenManager.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
				refusedToken = req.TokenID
				return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil
			}),
		deps.mockTokenManager.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
				assert.NotEqual(t, refusedToken, req.TokenID, "the refused token is unavailable")
				assert.Equal(t, ownerID, req.OwnerID)
				return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
			}),
		deps.mockHistory.EXPECT().RecordSemaphoreAcquired(gomock.Any(), gomock.Any()).Times(1).
			Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
	)

	assert.True(t, runGrantTask(t, mgr, deps, testTask(nil)))
}

func TestGrantTask_WokenByARelease(t *testing.T) {
	ownerID := testTaskOwner.String()
	mgr, deps := startGranterTestManager(t, 1, []*persistence.SemaphoreOwnership{tokenRow(1, "other-owner"), ownerRow("other-owner", 1)})
	gomock.InOrder(
		expectReleaseToken(deps.mockTokenManager, 1, "other-owner").Return(releaseApplied, nil),
		expectGrantToken(deps.mockTokenManager, 1, ownerID).
			Return(&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil),
		expectRecordGrant(deps.mockHistory, 1).Return(&types.RecordSemaphoreAcquiredResponse{}, nil),
	)

	done := make(chan bool, 1)
	// No deadline, so only a wake-up ends the wait.
	go func() { done <- mgr.grantTask(context.Background(), testTask(nil)) }()
	require.NoError(t, mgr.Release(context.Background(), "other-owner"))
	select {
	case finished := <-done:
		assert.True(t, finished)
	case <-time.After(10 * time.Second):
		t.Fatal("the release did not wake the grant loop")
	}
}

func TestGrantTask_ContextEnds(t *testing.T) {
	ownerID := testTaskOwner.String()
	testCases := []struct {
		name string
		rows []*persistence.SemaphoreOwnership
		// setupMocks expects the calls made before ctx ends, and returns once they are made.
		setupMocks func(deps *mockDeps) <-chan struct{}
	}{
		{
			name: "waiting for a free token",
			rows: []*persistence.SemaphoreOwnership{tokenRow(1, "other-owner"), ownerRow("other-owner", 1)},
			setupMocks: func(*mockDeps) <-chan struct{} {
				called := make(chan struct{})
				close(called)
				return called
			},
		},
		{
			name: "History keeps failing, the token is kept",
			setupMocks: func(deps *mockDeps) <-chan struct{} {
				called := make(chan struct{})
				expectGrantToken(deps.mockTokenManager, 1, ownerID).
					Return(&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
				expectRecordGrant(deps.mockHistory, 1).DoAndReturn(
					func(context.Context, *types.RecordSemaphoreAcquiredRequest, ...yarpc.CallOption) (*types.RecordSemaphoreAcquiredResponse, error) {
						close(called)
						return nil, assert.AnError
					})
				return called
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startGranterTestManager(t, 1, tc.rows)
			called := tc.setupMocks(deps)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan bool, 1)
			go func() { done <- mgr.grantTask(ctx, testTask(nil)) }()
			<-called
			cancel()
			select {
			case finished := <-done:
				assert.False(t, finished, "the task is not finished")
			case <-time.After(10 * time.Second):
				t.Fatal("grantTask did not return")
			}
		})
	}
}
