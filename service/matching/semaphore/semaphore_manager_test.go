package semaphore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

var testBucketID = Identifier{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0}

const testIdleTTL = 100 * time.Millisecond

// heldByCaller is a token held by "caller", the owner the test calls the manager for.
var heldByCaller = tokenEntry{state: tokenHeld, owner: "caller"}

// heldByOther is a token held by "other-owner", an owner other than the caller.
var heldByOther = tokenEntry{state: tokenHeld, owner: "other-owner"}

func tokenRow(tokenID int, holder string) *persistence.SemaphoreOwnership {
	return &persistence.SemaphoreOwnership{
		RowType:       persistence.SemaphoreRowTypeToken,
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		TokenID:       tokenID,
		Holder:        holder,
	}
}

func ownerRow(ownerID string, heldToken int) *persistence.SemaphoreOwnership {
	return &persistence.SemaphoreOwnership{
		RowType:       persistence.SemaphoreRowTypeOwner,
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		OwnerID:       ownerID,
		HeldToken:     heldToken,
	}
}

// expectScan stubs the startup load with the given pages and asserts that the page token
// from each response is threaded into the next request.
func expectScan(t *testing.T, m *persistence.MockSemaphoreTokenManager, pages [][]*persistence.SemaphoreOwnership) {
	t.Helper()
	var calls int
	m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(len(pages)).DoAndReturn(
		func(_ context.Context, req *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
			i := calls
			calls++
			assert.Equal(t, testBucketID.DomainID, req.DomainID)
			assert.Equal(t, testBucketID.SemaphoreName, req.SemaphoreName)
			assert.Equal(t, testBucketID.Bucket, req.Bucket)
			if i == 0 {
				assert.Empty(t, req.NextPageToken, "first page must start with no token")
			} else {
				assert.Equal(t, []byte(fmt.Sprintf("page-%d", i)), req.NextPageToken)
			}
			var next []byte
			if i < len(pages)-1 {
				next = []byte(fmt.Sprintf("page-%d", i+1))
			}
			return &persistence.ScanSemaphoreBucketResponse{Ownerships: pages[i], NextPageToken: next}, nil
		})
}

// singleBucketMetadata returns metadata for a semaphore with one bucket, testBucketID, which
// owns tokens 1 to tokenCount.
func singleBucketMetadata(t *testing.T, tokenCount int) persistence.SemaphoreMetadataManager {
	t.Helper()
	md := persistence.NewMockSemaphoreMetadataManager(gomock.NewController(t))
	// At most once: only the startup load reads it, and some tests never start the manager.
	md.EXPECT().GetSemaphore(gomock.Any(), &persistence.GetSemaphoreRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
	}).MaxTimes(1).Return(&persistence.GetSemaphoreResponse{Semaphore: &persistence.SemaphoreMetadata{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Size:          tokenCount,
		BucketSize:    tokenCount,
	}}, nil)
	return md
}

// claimingTasks returns a task store that answers the startup claim.
func claimingTasks(t *testing.T) *persistence.MockSemaphoreTaskManager {
	t.Helper()
	tasks := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
	// At most once: some tests never start the manager.
	tasks.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).MaxTimes(1).
		Return(&persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 1}, nil)
	return tasks
}

// testConfig returns a config whose buckets unload after idleTime.
func testConfig(idleTime time.Duration) *config.Config {
	return &config.Config{
		SemaphoreIdleTime:          func(string) time.Duration { return idleTime },
		SemaphoreUpdateAckInterval: func(string) time.Duration { return testUpdateAckInterval() },
	}
}

// testManagerParams returns valid params for a bucket of tokens 1 to tokenCount. The clock only
// moves when a test moves it, so nothing is evicted unless the test asks for it. Tests override
// fields as needed.
func testManagerParams(t *testing.T, m persistence.SemaphoreTokenManager, tokenCount int) ManagerParams {
	t.Helper()
	return ManagerParams{
		ID:         testBucketID,
		Tokens:     m,
		Metadata:   singleBucketMetadata(t, tokenCount),
		Tasks:      claimingTasks(t),
		Logger:     testlogger.New(t),
		Cfg:        testConfig(testIdleTTL),
		DomainName: "test-domain",
		OnStopFn:   func(Manager) {},
		TimeSource: clock.NewMockedTimeSource(),
	}
}

// newManagerFromParams builds an unstarted manager and stops it when the test ends.
func newManagerFromParams(t *testing.T, p ManagerParams) *semaphoreManagerImpl {
	t.Helper()
	mgr, err := NewManager(p)
	require.NoError(t, err)
	// Every manager runs an idle clock, so stop it rather than leak the goroutine.
	t.Cleanup(mgr.Stop)
	return mgr.(*semaphoreManagerImpl)
}

// newTestManager builds an unstarted manager for a bucket of tokens 1 to tokenCount.
func newTestManager(t *testing.T, m persistence.SemaphoreTokenManager, tokenCount int) *semaphoreManagerImpl {
	t.Helper()
	return newManagerFromParams(t, testManagerParams(t, m, tokenCount))
}

// startManager returns a started manager for a bucket of tokens 1 to tokenCount, whose startup scan
// read the given single page of rows. With no rows, every token is free.
func startManager(t *testing.T, m *persistence.MockSemaphoreTokenManager, tokenCount int, rows []*persistence.SemaphoreOwnership) *semaphoreManagerImpl {
	t.Helper()
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{rows})
	mgr := newTestManager(t, m, tokenCount)
	require.NoError(t, mgr.Start(context.Background()))
	return mgr
}

// startWithBlockedScan starts mgr and returns while its scan is still blocked.
// releaseScanAndWaitForStart unblocks the scan and returns Start's error.
func startWithBlockedScan(m *persistence.MockSemaphoreTokenManager, mgr *semaphoreManagerImpl) (releaseScanAndWaitForStart func() error) {
	scanning, finishScan := make(chan struct{}), make(chan struct{})
	m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
			close(scanning)
			<-finishScan
			return &persistence.ScanSemaphoreBucketResponse{}, nil
		})
	started := make(chan error, 1)
	go func() { started <- mgr.Start(context.Background()) }()
	<-scanning
	return func() error {
		close(finishScan)
		return <-started
	}
}

// getManagerState returns the manager's lifecycle state, read under its lock.
func getManagerState(mgr *semaphoreManagerImpl) managerState {
	mgr.stateMu.Lock()
	defer mgr.stateMu.Unlock()
	return mgr.state
}

// freeTokenCount returns how many tokens the manager's cache lists as free.
func freeTokenCount(mgr *semaphoreManagerImpl) int {
	return mgr.currentCache().freeTokenCount()
}

// assertBucketUnlocked checks that no grant still holds mgr's bucket lock. A lock left held makes
// every later call on the bucket wait until its deadline.
func assertBucketUnlocked(t *testing.T, mgr *semaphoreManagerImpl) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, mgr.bucketLock.Lock(ctx), "the bucket lock is still held")
	mgr.bucketLock.Unlock()
}

// Tests the manager's lifecycle: construction, the startup load, Stop and the idle unload.
func TestManagerLifecycle(t *testing.T) {
	t.Run("NewManager rejects invalid params", func(t *testing.T) {
		full := testManagerParams(t, persistence.NewMockSemaphoreTokenManager(gomock.NewController(t)), 1)
		// Each case drops exactly one required field from an otherwise valid set.
		without := func(drop func(p *ManagerParams)) ManagerParams {
			p := full
			drop(&p)
			return p
		}

		tests := []struct {
			name   string
			params ManagerParams
		}{
			{name: "no identifier", params: without(func(p *ManagerParams) { p.ID = Identifier{} })},
			{name: "no token manager", params: without(func(p *ManagerParams) { p.Tokens = nil })},
			{name: "no metadata manager", params: without(func(p *ManagerParams) { p.Metadata = nil })},
			{name: "no task manager", params: without(func(p *ManagerParams) { p.Tasks = nil })},
			{name: "no logger", params: without(func(p *ManagerParams) { p.Logger = nil })},
			{name: "no config", params: without(func(p *ManagerParams) { p.Cfg = nil })},
			{name: "no domain name", params: without(func(p *ManagerParams) { p.DomainName = "" })},
			{name: "no stop callback", params: without(func(p *ManagerParams) { p.OnStopFn = nil })},
			{name: "no time source", params: without(func(p *ManagerParams) { p.TimeSource = nil })},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mgr, err := NewManager(tc.params)
				assert.IsType(t, &types.BadRequestError{}, err, "a misconfigured manager is never worth retrying")
				assert.Equal(t, Manager(nil), mgr, "an error carries no manager")
			})
		}
	})

	t.Run("Start builds the cache from every scan page", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := persistence.NewMockSemaphoreTokenManager(ctrl)

		// Tokens 1 and 5 have no row. Token 3 was released.
		expectScan(t, m, [][]*persistence.SemaphoreOwnership{
			{tokenRow(2, "owner-x")},
			{tokenRow(3, ""), tokenRow(4, "owner-y")},
			{ownerRow("owner-x", 2), ownerRow("owner-y", 4)},
		})

		mgr := newTestManager(t, m, 5)
		require.NoError(t, mgr.Start(context.Background()))

		assert.Equal(t, []int{1, 3, 5}, tokensInState(mgr.currentCache(), tokenFree), "every token not held is free")
		assertCacheConsistent(t, mgr.currentCache())
		assert.Equal(t, map[string]int{"owner-x": 2, "owner-y": 4}, copyTokenByOwner(mgr.currentCache()))
	})

	t.Run("Start fails", func(t *testing.T) {
		semaphoreOf := func(size, bucketSize int) *persistence.GetSemaphoreResponse {
			return &persistence.GetSemaphoreResponse{Semaphore: &persistence.SemaphoreMetadata{
				DomainID:      testBucketID.DomainID,
				SemaphoreName: testBucketID.SemaphoreName,
				Size:          size,
				BucketSize:    bucketSize,
			}}
		}
		scanErr := errors.New("scan failed")
		claimed := func(tasks *persistence.MockSemaphoreTaskManager) {
			tasks.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).
				Return(&persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 1}, nil)
		}

		tests := []struct {
			name   string
			bucket int
			// metadata stubs the metadata read. Nil reads one bucket of 2 tokens.
			metadata func(md *persistence.MockSemaphoreMetadataManager)
			// scan stubs the scan. Nil expects no scan.
			scan func(m *persistence.MockSemaphoreTokenManager)
			// claim stubs the claim on the queue. Nil expects no claim.
			claim     func(tasks *persistence.MockSemaphoreTaskManager)
			assertErr func(t *testing.T, err error)
		}{
			{
				name: "claiming the queue fails",
				claim: func(tasks *persistence.MockSemaphoreTaskManager) {
					tasks.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
			},
			{
				name: "metadata read fails",
				metadata: func(md *persistence.MockSemaphoreMetadataManager) {
					md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
			},
			{
				name: "semaphore does not exist",
				metadata: func(md *persistence.MockSemaphoreMetadataManager) {
					md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{Message: "not found"})
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.EntityNotExistsError)) },
			},
			{
				name: "read returns no semaphore",
				metadata: func(md *persistence.MockSemaphoreMetadataManager) {
					md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(&persistence.GetSemaphoreResponse{}, nil)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.InternalServiceError)) },
			},
			{
				name:   "requested bucket 2, but size 4 in buckets of 2 has only buckets 0 and 1",
				bucket: 2,
				metadata: func(md *persistence.MockSemaphoreMetadataManager) {
					md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(semaphoreOf(4, 2), nil)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.BadRequestError)) },
			},
			{
				// CreateSemaphore rejects a size below 1, so the row is corrupt.
				name: "metadata has no size",
				metadata: func(md *persistence.MockSemaphoreMetadataManager) {
					md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(semaphoreOf(0, 2), nil)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.InternalServiceError)) },
			},
			{
				// The first page's rows must not be installed.
				name:  "scan fails on its second page",
				claim: claimed,
				scan: func(m *persistence.MockSemaphoreTokenManager) {
					var calls int
					m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
						func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
							calls++
							if calls == 1 {
								return &persistence.ScanSemaphoreBucketResponse{
									Ownerships:    []*persistence.SemaphoreOwnership{tokenRow(1, ""), ownerRow("owner-x", 2)},
									NextPageToken: []byte("page-1"),
								}, nil
							}
							return nil, scanErr
						})
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, scanErr) },
			},
			{
				name:  "scan returns no response and no error",
				claim: claimed,
				scan: func(m *persistence.MockSemaphoreTokenManager) {
					m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).Return(nil, nil)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.InternalServiceError)) },
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				m := persistence.NewMockSemaphoreTokenManager(ctrl)
				if tc.scan != nil {
					tc.scan(m)
				}
				p := testManagerParams(t, m, 2)
				p.ID.Bucket = tc.bucket
				if tc.metadata != nil {
					md := persistence.NewMockSemaphoreMetadataManager(ctrl)
					tc.metadata(md)
					p.Metadata = md
				}
				// A failed metadata read must write nothing to semaphore_tasks.
				tasks := persistence.NewMockSemaphoreTaskManager(ctrl)
				if tc.claim != nil {
					tc.claim(tasks)
				}
				p.Tasks = tasks
				var stopped []Manager
				p.OnStopFn = func(s Manager) { stopped = append(stopped, s) }
				mgr := newManagerFromParams(t, p)

				tc.assertErr(t, mgr.Start(context.Background()))
				assert.Equal(t, 0, freeTokenCount(mgr), "nothing is installed")
				assert.Empty(t, copyTokenByOwner(mgr.currentCache()))
				assert.Equal(t, managerStateStopped, getManagerState(mgr))
				require.Len(t, stopped, 1)
				assert.Same(t, mgr, stopped[0], "OnStopFn gets the manager itself, so the registry removes only it")
			})
		}
	})

	t.Run("a second Start returns at once without scanning", func(t *testing.T) {
		tests := []struct {
			name    string
			setup   func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl
			wantErr error
		}{
			{
				// Not waited on: the load runs under the scan's deadline, not the caller's.
				name: "the first Start is still scanning",
				setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
					mgr := newTestManager(t, m, 2)
					releaseScanAndWaitForStart := startWithBlockedScan(m, mgr)
					t.Cleanup(func() { assert.NoError(t, releaseScanAndWaitForStart()) })
					return mgr
				},
			},
			{
				name: "the first Start finished loading",
				setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
					return startManager(t, m, 2, nil)
				},
			},
			{
				name: "the manager was stopped after loading",
				setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
					mgr := startManager(t, m, 2, nil)
					mgr.Stop()
					return mgr
				},
				wantErr: ErrNotReady,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				// Each setup expects one scan, so a second scan fails the test.
				m := persistence.NewMockSemaphoreTokenManager(ctrl)
				mgr := tc.setup(t, m)

				done := make(chan error, 1)
				go func() { done <- mgr.Start(context.Background()) }()
				select {
				case err := <-done:
					if tc.wantErr != nil {
						assert.ErrorIs(t, err, tc.wantErr)
					} else {
						assert.NoError(t, err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("Start blocked instead of returning at once")
				}
			})
		}
	})

	// Stop during the scan releases waiting callers at once, and the late scan does not bring the
	// manager back up.
	t.Run("Stop while Start is scanning", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := persistence.NewMockSemaphoreTokenManager(ctrl)
		mgr := newTestManager(t, m, 1)
		releaseScanAndWaitForStart := startWithBlockedScan(m, mgr)

		mgr.Stop()
		select {
		case <-mgr.startupDoneCh:
		default:
			t.Fatal("Stop did not release callers waiting on startup while the scan was still running")
		}

		assert.ErrorIs(t, releaseScanAndWaitForStart(), ErrNotReady, "Start reports that it lost the bucket")
		assert.Equal(t, managerStateStopped, getManagerState(mgr))
		assert.Equal(t, 0, freeTokenCount(mgr), "the late scan's cache is not installed")
	})

	t.Run("a caller's deadline passes while Start is scanning", func(t *testing.T) {
		m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
		mgr := newTestManager(t, m, 1)
		releaseScanAndWaitForStart := startWithBlockedScan(m, mgr)
		defer func() { assert.NoError(t, releaseScanAndWaitForStart()) }()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := mgr.Acquire(ctx, "caller", nil)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	// The idle clock starts after the load and is reset by Acquire, not by GetTokenByOwner.
	t.Run("unloads after the idle TTL", func(t *testing.T) {
		defer goleak.VerifyNone(t)
		m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
		m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
			&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
		p := testManagerParams(t, m, 1)
		mockClock := clock.NewMockedTimeSource()
		p.TimeSource = mockClock
		mgr := newManagerFromParams(t, p)
		isStopped := func() bool { return getManagerState(mgr) == managerStateStopped }

		// A load slower than the idle TTL does not unload the bucket.
		releaseScanAndWaitForStart := startWithBlockedScan(m, mgr)
		mockClock.Advance(2 * testIdleTTL)
		require.NoError(t, releaseScanAndWaitForStart())
		_, err := mgr.Acquire(context.Background(), "caller", nil)
		require.NoError(t, err)

		mockClock.Advance(testIdleTTL / 2)
		require.Never(t, isStopped, 50*time.Millisecond, 5*time.Millisecond, "stopped within the idle TTL of an Acquire")

		_, err = mgr.GetTokenByOwner(context.Background(), "caller")
		require.NoError(t, err)
		mockClock.Advance(testIdleTTL * 3 / 4)
		require.Eventually(t, isStopped, time.Second, 5*time.Millisecond, "did not stop an idle TTL after the Acquire")
	})

	t.Run("a non-positive idle time still starts", func(t *testing.T) {
		// A bad idle time must not fail the request or panic the idle clock.
		m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
		expectScan(t, m, [][]*persistence.SemaphoreOwnership{nil})
		p := testManagerParams(t, m, 1)
		p.Cfg = testConfig(0)
		mgr := newManagerFromParams(t, p)
		require.NoError(t, mgr.Start(context.Background()))
		assert.Equal(t, managerStateRunning, getManagerState(mgr))
	})

	t.Run("losing the claim on the queue stops the bucket", func(t *testing.T) {
		conditionFailed := &persistence.ConditionFailedError{Msg: "range_id changed"}
		tests := []struct {
			name string
			// loseClaim makes mgr find out another host has claimed the bucket.
			loseClaim func(t *testing.T, mgr *semaphoreManagerImpl, tasks *persistence.MockSemaphoreTaskManager, mockClock clock.MockedTimeSource)
		}{
			{
				name: "reader's ack level save fails with ConditionFailedError",
				loseClaim: func(_ *testing.T, _ *semaphoreManagerImpl, tasks *persistence.MockSemaphoreTaskManager, mockClock clock.MockedTimeSource) {
					tasks.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).Return(nil, conditionFailed)
					// The idle clock and the ack level ticker.
					mockClock.BlockUntil(2)
					mockClock.Advance(testUpdateAckInterval())
				},
			},
			{
				name: "writer's waiter write fails with ConditionFailedError",
				loseClaim: func(t *testing.T, mgr *semaphoreManagerImpl, tasks *persistence.MockSemaphoreTaskManager, _ clock.MockedTimeSource) {
					tasks.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).Return(nil, conditionFailed)
					// The failed write still moves maxReadLevel, so the reader may read before the bucket stops.
					tasks.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).
						Return(&persistence.GetSemaphoreTasksResponse{}, nil).AnyTimes()
					// ErrNotReady if the manager stops the writer before the writer answers.
					err := mgr.taskWriter.appendTask(testContext(t), &persistence.SemaphoreTask{WorkflowID: "wf"})
					assert.True(t, errors.Is(err, conditionFailed) || errors.Is(err, ErrNotReady), "got %v", err)
				},
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				defer goleak.VerifyNone(t)
				ctrl := gomock.NewController(t)
				m := persistence.NewMockSemaphoreTokenManager(ctrl)
				expectScan(t, m, [][]*persistence.SemaphoreOwnership{nil})
				tasks := claimingTasks(t)
				p := testManagerParams(t, m, 1)
				p.Tasks = tasks
				mockClock := clock.NewMockedTimeSource()
				p.TimeSource = mockClock
				// Longer than the ack level interval, so the bucket is not unloaded for being idle.
				p.Cfg = testConfig(2 * testUpdateAckInterval())
				stopped := make(chan Manager, 1)
				p.OnStopFn = func(s Manager) { stopped <- s }
				mgr := newManagerFromParams(t, p)
				require.NoError(t, mgr.Start(context.Background()))

				tc.loseClaim(t, mgr, tasks, mockClock)

				select {
				case s := <-stopped:
					assert.Same(t, mgr, s)
				case <-time.After(time.Second):
					t.Fatal("a bucket that lost its claim was not stopped")
				}
				_, err := mgr.Acquire(context.Background(), "owner-a", nil)
				assert.ErrorIs(t, err, ErrNotReady)
			})
		}
	})
}

func TestNewSemaphoreConfigIdleTime(t *testing.T) {
	defaultIdleTime := dynamicproperties.MatchingSemaphoreIdleTime.DefaultDuration()
	tests := []struct {
		name       string
		configured time.Duration
		want       time.Duration
	}{
		{name: "positive idle time is used as is", configured: time.Minute, want: time.Minute},
		{name: "2ns is used as is", configured: 2, want: 2},
		{name: "1ns falls back to the default", configured: 1, want: defaultIdleTime},
		{name: "zero idle time falls back to the default", configured: 0, want: defaultIdleTime},
		{name: "negative idle time falls back to the default", configured: -time.Second, want: defaultIdleTime},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(tc.configured)
			assert.Equal(t, tc.want, newSemaphoreConfig(cfg, "test-domain").IdleTime())
		})
	}
}

func TestNewSemaphoreConfigUpdateAckInterval(t *testing.T) {
	defaultInterval := dynamicproperties.MatchingSemaphoreUpdateAckInterval.DefaultDuration()
	tests := []struct {
		name       string
		configured time.Duration
		want       time.Duration
	}{
		{name: "positive interval is used as is", configured: 30 * time.Second, want: 30 * time.Second},
		{name: "zero interval falls back to the default", configured: 0, want: defaultInterval},
		{name: "negative interval falls back to the default", configured: -time.Second, want: defaultInterval},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(time.Hour)
			cfg.SemaphoreUpdateAckInterval = func(string) time.Duration { return tc.configured }
			assert.Equal(t, tc.want, newSemaphoreConfig(cfg, "test-domain").UpdateAckInterval())
		})
	}
}

func TestAcquire(t *testing.T) {
	owner := commonsemaphore.Owner{WorkflowID: "wf", RunID: "run", HoldID: 7}
	deadline := time.Unix(1767258000, 0)
	tests := []struct {
		name string
		// managerStopped stops the manager before Acquire.
		managerStopped bool
		// bucketFull starts the manager with the bucket's only token held by another owner.
		bucketFull bool
		// ownerID is who acquires. Empty means owner.
		ownerID  string
		deadline *time.Time
		// mockSetup expects Acquire's token writes. Nil expects none. The mock fails the test on
		// any other call.
		mockSetup func(m *persistence.MockSemaphoreTokenManager)
		// wantWaiter is the waiter Acquire writes, without its task id and created time. Nil
		// expects no write.
		wantWaiter *persistence.SemaphoreTask
		// writeErr is what the waiter write returns.
		writeErr error
		want     AcquireResult
		// assertErr checks the error. Nil expects no error.
		assertErr func(t *testing.T, err error)
	}{
		{
			name:           "manager is stopped: retryable ErrNotReady",
			managerStopped: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotReady)
				assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
			},
		},
		{
			name: "a token is free: Acquired with that token",
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
			},
			want: AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1},
		},
		{
			name: "grant write fails: error",
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
		{
			name:       "bucket is full: waiter is written, Queued",
			bucketFull: true,
			deadline:   &deadline,
			wantWaiter: &persistence.SemaphoreTask{WorkflowID: "wf", RunID: "run", HoldID: 7, AcquireDeadline: &deadline},
			want:       AcquireResult{Outcome: types.SemaphoreAcquireOutcomeQueued},
		},
		{
			name:       "bucket is full, no deadline: waiter is written without one",
			bucketFull: true,
			wantWaiter: &persistence.SemaphoreTask{WorkflowID: "wf", RunID: "run", HoldID: 7},
			want:       AcquireResult{Outcome: types.SemaphoreAcquireOutcomeQueued},
		},
		{
			name:       "bucket is full, waiter write fails: error",
			bucketFull: true,
			wantWaiter: &persistence.SemaphoreTask{WorkflowID: "wf", RunID: "run", HoldID: 7},
			writeErr:   assert.AnError,
			assertErr:  func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
		{
			name:       "bucket is full, owner id cannot be decoded: bad request, nothing written",
			bucketFull: true,
			ownerID:    "caller",
			assertErr:  func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.BadRequestError)) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
			var rows []*persistence.SemaphoreOwnership
			if tc.bucketFull {
				rows = []*persistence.SemaphoreOwnership{tokenRow(1, "other-owner"), ownerRow("other-owner", 1)}
			}
			expectScan(t, m, [][]*persistence.SemaphoreOwnership{rows})
			tasks := claimingTasks(t)
			p := testManagerParams(t, m, 1)
			p.Tasks = tasks
			// read is sent to when the reader reads, which it does after a waiter write since enqueue
			// signals it.
			read := make(chan struct{}, 1)
			if tc.wantWaiter != nil {
				waiter := *tc.wantWaiter
				waiter.TaskID = 1
				waiter.CreatedTime = p.TimeSource.Now()
				tasks.EXPECT().CreateSemaphoreTasks(gomock.Any(), &persistence.CreateSemaphoreTasksRequest{
					DomainID:      testBucketID.DomainID,
					SemaphoreName: testBucketID.SemaphoreName,
					Bucket:        testBucketID.Bucket,
					RangeID:       1,
					Tasks:         []*persistence.SemaphoreTask{&waiter},
				}).Return(&persistence.CreateSemaphoreTasksResponse{}, tc.writeErr)
				tasks.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
					func(context.Context, *persistence.GetSemaphoreTasksRequest) (*persistence.GetSemaphoreTasksResponse, error) {
						select {
						case read <- struct{}{}:
						default:
						}
						return &persistence.GetSemaphoreTasksResponse{}, nil
					})
			}
			mgr := newManagerFromParams(t, p)
			require.NoError(t, mgr.Start(context.Background()))
			if tc.managerStopped {
				mgr.Stop()
			}
			if tc.mockSetup != nil {
				tc.mockSetup(m)
			}
			ownerID := tc.ownerID
			if ownerID == "" {
				ownerID = owner.String()
			}

			got, err := mgr.Acquire(context.Background(), ownerID, tc.deadline)
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
			if err == nil && tc.wantWaiter != nil {
				select {
				case <-read:
				case <-time.After(time.Second):
					t.Fatal("the reader was not signaled after the waiter was written")
				}
			}
		})
	}
}

// Tests grant for "caller" when the cache records it on token 1, the bucket's only token.
func TestGrant_CachedToken(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	tests := []struct {
		name string
		// readRow and readErr are what reading token 1's row returns.
		readRow *persistence.SemaphoreOwnership
		readErr error
		// wantWrite expects grant to write token 1. The mock answers Applied.
		wantWrite      bool
		wantTokenID    int        // what grant returns
		wantCacheEntry tokenEntry // token 1's entry afterwards
		// assertErr checks the error. Nil expects no error.
		assertErr func(t *testing.T, err error)
	}{
		{
			name:           "row names the caller: returns token 1 without a write",
			readRow:        tokenRow(1, "caller"),
			wantTokenID:    1,
			wantCacheEntry: heldByCaller,
		},
		{
			name:           "row is FREE: grants token 1 again",
			readRow:        tokenRow(1, ""),
			wantWrite:      true,
			wantTokenID:    1,
			wantCacheEntry: heldByCaller,
		},
		{
			// Should not happen: a grant writes the token row with the owner row, and nothing deletes it.
			// The owner row may still name the token, so it is not freed.
			name:           "no row: token 1 becomes unavailable",
			readErr:        &types.EntityNotExistsError{Message: "not found"},
			wantTokenID:    emptyTokenID,
			wantCacheEntry: unavailable,
		},
		{
			name:           "row names another owner: token 1 becomes unavailable",
			readRow:        tokenRow(1, "other-owner"),
			wantTokenID:    emptyTokenID,
			wantCacheEntry: unavailable,
		},
		{
			name:           "read fails: error, cache unchanged",
			readErr:        readErr,
			wantTokenID:    emptyTokenID,
			wantCacheEntry: heldByCaller,
			assertErr:      func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, 1, []*persistence.SemaphoreOwnership{tokenRow(1, "caller"), ownerRow("caller", 1)})

			m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), &persistence.GetSemaphoreOwnershipByTokenRequest{
				DomainID:      testBucketID.DomainID,
				SemaphoreName: testBucketID.SemaphoreName,
				Bucket:        testBucketID.Bucket,
				TokenID:       1,
			}).MinTimes(1).Return(&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tc.readRow}, tc.readErr)
			// With no write expected, the mock fails the test if grant writes anyway.
			if tc.wantWrite {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), &persistence.GrantSemaphoreTokenRequest{
					DomainID:      testBucketID.DomainID,
					SemaphoreName: testBucketID.SemaphoreName,
					Bucket:        testBucketID.Bucket,
					TokenID:       1,
					OwnerID:       "caller",
				}).Times(1).Return(&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
			}

			got, err := mgr.grant(context.Background(), "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantTokenID, got)
			assert.Equal(t, tc.wantCacheEntry, getTokenEntry(mgr.currentCache(), 1))
			assertCacheConsistent(t, mgr.currentCache())
		})
	}
}

// Tests grant's one write of a free token, for each answer. Token 2 is held by other-owner, so the
// write goes to token 1.
func TestGrant_FreeToken(t *testing.T) {
	writeErr := &persistence.TimeoutError{Msg: "write timed out"}
	tests := []struct {
		name     string
		resp     *persistence.GrantSemaphoreTokenResponse
		writeErr error
		// wantTokenID is what grant returns.
		wantTokenID int
		// wantCacheEntries are each token's entry afterwards.
		wantCacheEntries map[int]tokenEntry
		// assertErr checks the error. Nil expects no error.
		assertErr func(t *testing.T, err error)
	}{
		{
			name:             "write applies: grants token 1",
			resp:             &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied},
			wantTokenID:      1,
			wantCacheEntries: map[int]tokenEntry{1: heldByCaller, 2: heldByOther},
		},
		{
			// The database is right and the cache was stale: the caller holds token 2.
			name:             "write says the caller already holds token 2: returns token 2",
			resp:             &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 2},
			wantTokenID:      2,
			wantCacheEntries: map[int]tokenEntry{1: free, 2: heldByCaller},
		},
		{
			name:             "write fails: error, token 1 stays free",
			writeErr:         writeErr,
			wantTokenID:      emptyTokenID,
			wantCacheEntries: map[int]tokenEntry{1: free, 2: heldByOther},
			assertErr:        func(t *testing.T, err error) { assert.ErrorIs(t, err, writeErr) },
		},
		{
			// The zero value, which is what a store that never set the field returns.
			name:             "write returns an unknown outcome: error, token 1 stays free",
			resp:             &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown},
			wantTokenID:      emptyTokenID,
			wantCacheEntries: map[int]tokenEntry{1: free, 2: heldByOther},
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "unexpected grant outcome")
			},
		},
		{
			name:             "write says the caller holds token 9, outside the bucket: error, token 1 stays free",
			resp:             &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 9},
			wantTokenID:      emptyTokenID,
			wantCacheEntries: map[int]tokenEntry{1: free, 2: heldByOther},
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "AlreadyHeld with token 9, outside tokens 1 to 2")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, 2, []*persistence.SemaphoreOwnership{tokenRow(2, "other-owner"), ownerRow("other-owner", 2)})

			// Exactly one write, so a retry fails the test.
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), &persistence.GrantSemaphoreTokenRequest{
				DomainID:      testBucketID.DomainID,
				SemaphoreName: testBucketID.SemaphoreName,
				Bucket:        testBucketID.Bucket,
				TokenID:       1,
				OwnerID:       "caller",
			}).Times(1).Return(tc.resp, tc.writeErr)

			got, err := mgr.grant(context.Background(), "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantTokenID, got)
			cache := mgr.currentCache()
			assert.Equal(t, tc.wantCacheEntries, map[int]tokenEntry{1: getTokenEntry(cache, 1), 2: getTokenEntry(cache, 2)})
			assertBucketUnlocked(t, mgr)
			assertCacheConsistent(t, cache)
		})
	}
}

// Tests that after SlotTaken, grant tries another token until a write applies, maxGrantAttempts
// tokens are tried, or the caller gives up.
func TestGrant_FreeTokenRetries(t *testing.T) {
	taken := persistence.SemaphoreGrantSlotTaken
	applied := persistence.SemaphoreGrantApplied
	tests := []struct {
		name       string
		tokenCount int
		// outcomes are the answers to each write, in order.
		outcomes []persistence.SemaphoreGrantOutcome
		// cancelOnWrite cancels the caller's context during the first write.
		cancelOnWrite bool
		// wantGranted is true if the last write applies, so grant returns its token.
		wantGranted     bool
		wantErr         error
		wantFree        int
		wantUnavailable int
	}{
		{
			name:            "first token is taken, the second applies: grants the second",
			tokenCount:      3,
			outcomes:        []persistence.SemaphoreGrantOutcome{taken, applied},
			wantGranted:     true,
			wantFree:        1,
			wantUnavailable: 1,
		},
		{
			name:            "every token tried is taken: no token after maxGrantAttempts writes",
			tokenCount:      10,
			outcomes:        slices.Repeat([]persistence.SemaphoreGrantOutcome{taken}, maxGrantAttempts),
			wantFree:        10 - maxGrantAttempts,
			wantUnavailable: maxGrantAttempts,
		},
		{
			name:            "caller cancels during a write that is taken: error, no more writes",
			tokenCount:      5,
			outcomes:        []persistence.SemaphoreGrantOutcome{taken},
			cancelOnWrite:   true,
			wantErr:         context.Canceled,
			wantFree:        4,
			wantUnavailable: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, tc.tokenCount, nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var tried []int
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(len(tc.outcomes)).DoAndReturn(
				func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
					tried = append(tried, req.TokenID)
					if tc.cancelOnWrite {
						cancel()
					}
					return &persistence.GrantSemaphoreTokenResponse{Outcome: tc.outcomes[len(tried)-1]}, nil
				})

			got, err := mgr.grant(ctx, "caller")
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Len(t, slices.Compact(slices.Sorted(slices.Values(tried))), len(tried), "each attempt writes a different token")
			if tc.wantGranted {
				assert.Equal(t, tried[len(tried)-1], got)
			} else {
				assert.Equal(t, emptyTokenID, got)
			}
			assert.Equal(t, tc.wantFree, freeTokenCount(mgr))
			assert.Len(t, tokensInState(mgr.currentCache(), tokenUnavailable), tc.wantUnavailable)
			assertBucketUnlocked(t, mgr)
		})
	}
}

func TestRelease(t *testing.T) {
	writeErr := &persistence.TimeoutError{Msg: "write timed out"}
	readErr := errors.New("cassandra unavailable")
	notFound := &types.EntityNotExistsError{Message: "not found"}
	applied := &persistence.ReleaseSemaphoreTokenResponse{Applied: true}
	notApplied := &persistence.ReleaseSemaphoreTokenResponse{Applied: false}
	ownerRowNaming := func(tokenID int) *persistence.GetSemaphoreOwnershipByOwnerResponse {
		return &persistence.GetSemaphoreOwnershipByOwnerResponse{Ownership: ownerRow("caller", tokenID)}
	}

	tests := []struct {
		name string
		// cacheEntries are each token's entry before Release.
		cacheEntries map[int]tokenEntry
		// managerStopped stops the manager before Release.
		managerStopped bool
		// mockSetup expects Release's database calls. Nil expects none. The mock fails the test on
		// any other call.
		mockSetup func(m *persistence.MockSemaphoreTokenManager)
		// wantCacheEntries are each token's entry afterwards.
		wantCacheEntries map[int]tokenEntry
		// assertErr checks the error. Nil expects no error.
		assertErr func(t *testing.T, err error)
	}{
		{
			name:         "cached token's release applies: token 1 is free",
			cacheEntries: map[int]tokenEntry{1: heldByCaller, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectReleaseToken(m, 1).Return(applied, nil)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: free},
		},
		{
			name:         "cached token 1 is no longer the caller's, and the caller holds no token: success, token 1 unavailable",
			cacheEntries: map[int]tokenEntry{1: heldByCaller, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				gomock.InOrder(
					expectReleaseToken(m, 1).Return(notApplied, nil),
					expectGetOwnerRow(m).Return(nil, notFound),
				)
			},
			wantCacheEntries: map[int]tokenEntry{1: unavailable, 2: free},
		},
		{
			name:         "cached token's release does not apply, owner row names token 2: releases token 2",
			cacheEntries: map[int]tokenEntry{1: heldByCaller, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				gomock.InOrder(
					expectReleaseToken(m, 1).Return(notApplied, nil),
					expectGetOwnerRow(m).Return(ownerRowNaming(2), nil),
					expectReleaseToken(m, 2).Return(applied, nil),
				)
			},
			wantCacheEntries: map[int]tokenEntry{1: unavailable, 2: free},
		},
		{
			name:         "cached token's release fails: error, token 1 free",
			cacheEntries: map[int]tokenEntry{1: heldByCaller, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectReleaseToken(m, 1).Return(nil, writeErr)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: free},
			assertErr:        func(t *testing.T, err error) { assert.ErrorIs(t, err, writeErr) },
		},
		{
			name:         "no cached token, no owner row: success, nothing released",
			cacheEntries: map[int]tokenEntry{1: free, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectGetOwnerRow(m).Return(nil, notFound)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: free},
		},
		{
			name:         "no cached token, owner row names token 2: releases token 2",
			cacheEntries: map[int]tokenEntry{1: free, 2: unavailable},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				gomock.InOrder(
					expectGetOwnerRow(m).Return(ownerRowNaming(2), nil),
					expectReleaseToken(m, 2).Return(applied, nil),
				)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: free},
		},
		{
			name:         "no cached token, token 2's release does not apply: error, token 2 unavailable",
			cacheEntries: map[int]tokenEntry{1: free, 2: unavailable},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				gomock.InOrder(
					expectGetOwnerRow(m).Return(ownerRowNaming(2), nil),
					expectReleaseToken(m, 2).Return(notApplied, nil),
				)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: unavailable},
			assertErr:        func(t *testing.T, err error) { assert.IsType(t, &types.InternalServiceError{}, err) },
		},
		{
			name:         "no cached token, token 2's release fails: error, token 2 free",
			cacheEntries: map[int]tokenEntry{1: free, 2: unavailable},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				gomock.InOrder(
					expectGetOwnerRow(m).Return(ownerRowNaming(2), nil),
					expectReleaseToken(m, 2).Return(nil, writeErr),
				)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: free},
			assertErr:        func(t *testing.T, err error) { assert.ErrorIs(t, err, writeErr) },
		},
		{
			name:         "no cached token, owner row read fails: error",
			cacheEntries: map[int]tokenEntry{1: free, 2: unavailable},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectGetOwnerRow(m).Return(nil, readErr)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: unavailable},
			assertErr:        func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
		{
			name:         "owner row names token 3, outside the bucket's range of 1 to 2: error, no release write",
			cacheEntries: map[int]tokenEntry{1: free, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectGetOwnerRow(m).Return(ownerRowNaming(3), nil)
			},
			wantCacheEntries: map[int]tokenEntry{1: free, 2: free},
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "token 3 of owner caller is outside the range")
			},
		},
		{
			name:             "manager is stopped: retryable ErrNotReady, nothing released",
			cacheEntries:     map[int]tokenEntry{1: heldByCaller, 2: free},
			managerStopped:   true,
			wantCacheEntries: map[int]tokenEntry{1: heldByCaller, 2: free},
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotReady)
				assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
			mgr := startManager(t, m, 2, nil)
			for tokenID, entry := range tc.cacheEntries {
				setTokenEntry(t, mgr.currentCache(), tokenID, entry)
			}
			if tc.managerStopped {
				mgr.Stop()
			}
			if tc.mockSetup != nil {
				tc.mockSetup(m)
			}

			err := mgr.Release(context.Background(), "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			cache := mgr.currentCache()
			assert.Equal(t, tc.wantCacheEntries, map[int]tokenEntry{1: getTokenEntry(cache, 1), 2: getTokenEntry(cache, 2)})
			assertBucketUnlocked(t, mgr)
			assertCacheConsistent(t, cache)
		})
	}
}

// Tests that when many owners call Acquire and Release at once, the bucket makes one database write
// at a time and the cache stays correct.
func TestManager_ConcurrentCalls(t *testing.T) {
	const tokens, owners = 3, 10
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{nil})
	// Owners that find no free token are queued.
	tasks := claimingTasks(t)
	tasks.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&persistence.CreateSemaphoreTasksResponse{}, nil)
	tasks.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&persistence.GetSemaphoreTasksResponse{}, nil)
	p := testManagerParams(t, m, tokens)
	p.Tasks = tasks
	mgr := newManagerFromParams(t, p)
	require.NoError(t, mgr.Start(context.Background()))

	// write fails the test if another write is running. A real write takes milliseconds; without
	// the pause, two unlocked writes rarely overlap and the test misses them.
	var writing atomic.Bool
	write := func() {
		assert.True(t, writing.CompareAndSwap(false, true), "two writes ran at once")
		time.Sleep(time.Millisecond)
		writing.Store(false)
	}
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(context.Context, *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			write()
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
		})
	m.EXPECT().ReleaseSemaphoreToken(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(context.Context, *persistence.ReleaseSemaphoreTokenRequest) (*persistence.ReleaseSemaphoreTokenResponse, error) {
			write()
			return &persistence.ReleaseSemaphoreTokenResponse{Applied: true}, nil
		})

	// Each owner acquires, and releases if it got a token.
	var wg sync.WaitGroup
	for i := range owners {
		wg.Add(1)
		go func(ownerID string) {
			defer wg.Done()
			got, err := mgr.Acquire(context.Background(), ownerID, nil)
			if !assert.NoError(t, err) || got.Outcome != types.SemaphoreAcquireOutcomeAcquired {
				return
			}
			assert.NoError(t, mgr.Release(context.Background(), ownerID))
		}(commonsemaphore.Owner{WorkflowID: fmt.Sprintf("wf-%d", i), RunID: "run", HoldID: 1}.String())
	}
	wg.Wait()

	assert.Equal(t, tokens, freeTokenCount(mgr), "every acquired token was released")
	assertBucketUnlocked(t, mgr)
	assertCacheConsistent(t, mgr.currentCache())
}

func TestGetTokenByOwner(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	notFound := &types.EntityNotExistsError{Message: "not found"}
	ownerRowNaming := func(tokenID int) *persistence.GetSemaphoreOwnershipByOwnerResponse {
		return &persistence.GetSemaphoreOwnershipByOwnerResponse{Ownership: ownerRow("caller", tokenID)}
	}

	tests := []struct {
		name           string
		cacheEntries   map[int]tokenEntry
		managerStopped bool
		bucketLocked   bool
		mockSetup      func(m *persistence.MockSemaphoreTokenManager)
		wantTokenID    int
		assertErr      func(t *testing.T, err error)
	}{
		{
			name:         "success - token in cache, returned without reading the database",
			cacheEntries: map[int]tokenEntry{1: heldByCaller, 2: free},
			wantTokenID:  1,
		},
		{
			name:         "success - token in cache while a grant holds the bucket lock, returned without waiting",
			cacheEntries: map[int]tokenEntry{1: heldByCaller, 2: free},
			bucketLocked: true,
			wantTokenID:  1,
		},
		{
			name:         "success - not in cache and no owner row, returns 0",
			cacheEntries: map[int]tokenEntry{1: heldByOther, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectGetOwnerRow(m).Return(nil, notFound)
			},
		},
		{
			name:         "success - not in cache, owner row has token 2, returns 2",
			cacheEntries: map[int]tokenEntry{1: free, 2: unavailable},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectGetOwnerRow(m).Return(ownerRowNaming(2), nil)
			},
			wantTokenID: 2,
		},
		{
			name:         "failure - not in cache, owner row read error is returned",
			cacheEntries: map[int]tokenEntry{1: free, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectGetOwnerRow(m).Return(nil, readErr)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
		{
			name:         "failure - not in cache, owner row token outside the bucket, returns an error",
			cacheEntries: map[int]tokenEntry{1: free, 2: free},
			mockSetup: func(m *persistence.MockSemaphoreTokenManager) {
				expectGetOwnerRow(m).Return(ownerRowNaming(3), nil)
			},
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "owner caller's owner row names token 3, outside tokens 1 to 2")
			},
		},
		{
			name:           "failure - manager stopped, returns ErrNotReady",
			cacheEntries:   map[int]tokenEntry{1: heldByCaller, 2: free},
			managerStopped: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotReady)
				assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
			mgr := startManager(t, m, 2, nil)
			for tokenID, entry := range tc.cacheEntries {
				setTokenEntry(t, mgr.currentCache(), tokenID, entry)
			}
			if tc.managerStopped {
				mgr.Stop()
			}
			if tc.bucketLocked {
				require.NoError(t, mgr.bucketLock.Lock(context.Background()))
				defer mgr.bucketLock.Unlock()
			}
			if tc.mockSetup != nil {
				tc.mockSetup(m)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tokenID, err := mgr.GetTokenByOwner(ctx, "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantTokenID, tokenID)
			cache := mgr.currentCache()
			assert.Equal(t, tc.cacheEntries, map[int]tokenEntry{1: getTokenEntry(cache, 1), 2: getTokenEntry(cache, 2)}, "a lookup never changes the cache")
		})
	}
}

// expectReleaseToken expects one release of tokenID for "caller".
func expectReleaseToken(m *persistence.MockSemaphoreTokenManager, tokenID int) *gomock.Call {
	return m.EXPECT().ReleaseSemaphoreToken(gomock.Any(), &persistence.ReleaseSemaphoreTokenRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		TokenID:       tokenID,
		OwnerID:       "caller",
	}).Times(1)
}

// expectGetOwnerRow expects one read of "caller"'s owner row.
func expectGetOwnerRow(m *persistence.MockSemaphoreTokenManager) *gomock.Call {
	return m.EXPECT().GetSemaphoreOwnershipByOwner(gomock.Any(), &persistence.GetSemaphoreOwnershipByOwnerRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		OwnerID:       "caller",
	}).Times(1)
}
