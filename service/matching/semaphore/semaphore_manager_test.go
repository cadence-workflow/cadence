package semaphore

import (
	"context"
	"errors"
	"fmt"
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

// callerOwnerRow is the read of "caller"'s owner row, naming tokenID.
func callerOwnerRow(tokenID int) *persistence.GetSemaphoreOwnershipByOwnerResponse {
	return &persistence.GetSemaphoreOwnershipByOwnerResponse{Ownership: ownerRow("caller", tokenID)}
}

type mockDeps struct {
	mockTokenManager *persistence.MockSemaphoreTokenManager
	mockTaskManager  *persistence.MockSemaphoreTaskManager
	mockTimeSource   clock.MockedTimeSource
}

// testConfig returns a config whose buckets unload after idleTime.
func testConfig(idleTime time.Duration) *config.Config {
	return &config.Config{
		SemaphoreIdleTime:          func(string) time.Duration { return idleTime },
		SemaphoreUpdateAckInterval: func(string) time.Duration { return testUpdateAckInterval() },
	}
}

// testManagerParams returns valid params for a bucket of tokens 1 to tokenCount, and their mocks.
// The metadata read and the queue claim may happen at most once, since not every test calls Start.
func testManagerParams(t *testing.T, tokenCount int) (ManagerParams, *mockDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	deps := &mockDeps{
		mockTokenManager: persistence.NewMockSemaphoreTokenManager(ctrl),
		mockTaskManager:  persistence.NewMockSemaphoreTaskManager(ctrl),
		mockTimeSource:   clock.NewMockedTimeSource(),
	}
	metadata := persistence.NewMockSemaphoreMetadataManager(ctrl)
	metadata.EXPECT().GetSemaphore(gomock.Any(), &persistence.GetSemaphoreRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
	}).MaxTimes(1).Return(&persistence.GetSemaphoreResponse{Semaphore: &persistence.SemaphoreMetadata{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Size:          tokenCount,
		BucketSize:    tokenCount,
	}}, nil)
	deps.mockTaskManager.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).MaxTimes(1).
		Return(&persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 1}, nil)
	return ManagerParams{
		ID:         testBucketID,
		Tokens:     deps.mockTokenManager,
		Metadata:   metadata,
		Tasks:      deps.mockTaskManager,
		Logger:     testlogger.New(t),
		Cfg:        testConfig(testIdleTTL),
		DomainName: "test-domain",
		OnStopFn:   func(Manager) {},
		TimeSource: deps.mockTimeSource,
	}, deps
}

// setupMocksForSemaphoreManager builds an unstarted manager from testManagerParams, changed by opts, and stops it
// when the test ends.
func setupMocksForSemaphoreManager(t *testing.T, tokenCount int, opts ...func(*ManagerParams)) (*semaphoreManagerImpl, *mockDeps) {
	t.Helper()
	p, deps := testManagerParams(t, tokenCount)
	for _, opt := range opts {
		opt(&p)
	}
	mgr, err := NewManager(p)
	require.NoError(t, err)
	// Every manager runs an idle clock, so stop it rather than leak the goroutine.
	t.Cleanup(mgr.Stop)
	return mgr.(*semaphoreManagerImpl), deps
}

// startTestManager returns a started manager whose startup scan read rows in one page. With no
// rows, every token is free.
func startTestManager(t *testing.T, tokenCount int, rows []*persistence.SemaphoreOwnership, opts ...func(*ManagerParams)) (*semaphoreManagerImpl, *mockDeps) {
	t.Helper()
	mgr, deps := setupMocksForSemaphoreManager(t, tokenCount, opts...)
	deps.mockTokenManager.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).
		Return(&persistence.ScanSemaphoreBucketResponse{Ownerships: rows}, nil)
	require.NoError(t, mgr.Start(context.Background()))
	return mgr, deps
}

// startWithBlockedScan starts mgr and returns while its scan is still blocked.
// releaseScanAndWaitForStart unblocks the scan and returns Start's error.
func startWithBlockedScan(mgr *semaphoreManagerImpl, deps *mockDeps) (releaseScanAndWaitForStart func() error) {
	scanning, finishScan := make(chan struct{}), make(chan struct{})
	deps.mockTokenManager.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
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

// cacheEntries returns each token's entry in c.
func cacheEntries(c *bucketCache) map[int]tokenEntry {
	entries := make(map[int]tokenEntry)
	for tokenID := c.tokenRange.First; tokenID <= c.tokenRange.Last(); tokenID++ {
		entries[tokenID] = getTokenEntry(c, tokenID)
	}
	return entries
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

// expectGetTokenRow expects one read of tokenID's token row.
func expectGetTokenRow(m *persistence.MockSemaphoreTokenManager, tokenID int) *gomock.Call {
	return m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		TokenID:       tokenID,
	}).Times(1)
}

// tokenRowRead is a read of tokenID's token row, naming holder.
func tokenRowRead(tokenID int, holder string) *persistence.GetSemaphoreOwnershipByTokenResponse {
	return &persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tokenRow(tokenID, holder)}
}

// expectGrantToken expects one grant of tokenID to "caller".
func expectGrantToken(m *persistence.MockSemaphoreTokenManager, tokenID int) *gomock.Call {
	return m.EXPECT().GrantSemaphoreToken(gomock.Any(), &persistence.GrantSemaphoreTokenRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		TokenID:       tokenID,
		OwnerID:       "caller",
	}).Times(1)
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

func TestNewManager(t *testing.T) {
	full, _ := testManagerParams(t, 1)
	// Each case drops exactly one required field from an otherwise valid set.
	without := func(drop func(p *ManagerParams)) ManagerParams {
		p := full
		drop(&p)
		return p
	}

	testCases := []struct {
		name   string
		params ManagerParams
	}{
		{name: "failure - no identifier", params: without(func(p *ManagerParams) { p.ID = Identifier{} })},
		{name: "failure - no token manager", params: without(func(p *ManagerParams) { p.Tokens = nil })},
		{name: "failure - no metadata manager", params: without(func(p *ManagerParams) { p.Metadata = nil })},
		{name: "failure - no task manager", params: without(func(p *ManagerParams) { p.Tasks = nil })},
		{name: "failure - no logger", params: without(func(p *ManagerParams) { p.Logger = nil })},
		{name: "failure - no config", params: without(func(p *ManagerParams) { p.Cfg = nil })},
		{name: "failure - no domain name", params: without(func(p *ManagerParams) { p.DomainName = "" })},
		{name: "failure - no stop callback", params: without(func(p *ManagerParams) { p.OnStopFn = nil })},
		{name: "failure - no time source", params: without(func(p *ManagerParams) { p.TimeSource = nil })},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, err := NewManager(tc.params)
			assert.IsType(t, &types.BadRequestError{}, err, "a misconfigured manager is never worth retrying")
			assert.Equal(t, Manager(nil), mgr, "an error carries no manager")
		})
	}
}

func TestNewSemaphoreConfig(t *testing.T) {
	defaultIdleTime := dynamicproperties.MatchingSemaphoreIdleTime.DefaultDuration()
	defaultAckInterval := dynamicproperties.MatchingSemaphoreUpdateAckInterval.DefaultDuration()
	testCases := []struct {
		name                string
		idleTime            time.Duration
		ackInterval         time.Duration
		expectedIdleTime    time.Duration
		expectedAckInterval time.Duration
	}{
		{name: "positive - used as is", idleTime: time.Minute, ackInterval: 30 * time.Second, expectedIdleTime: time.Minute, expectedAckInterval: 30 * time.Second},
		{name: "2ns idle time - used as is", idleTime: 2, ackInterval: time.Second, expectedIdleTime: 2, expectedAckInterval: time.Second},
		{name: "1ns idle time - falls back to the default", idleTime: 1, ackInterval: time.Second, expectedIdleTime: defaultIdleTime, expectedAckInterval: time.Second},
		{name: "zero - falls back to the default", expectedIdleTime: defaultIdleTime, expectedAckInterval: defaultAckInterval},
		{name: "negative - falls back to the default", idleTime: -time.Second, ackInterval: -time.Second, expectedIdleTime: defaultIdleTime, expectedAckInterval: defaultAckInterval},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(tc.idleTime)
			cfg.SemaphoreUpdateAckInterval = func(string) time.Duration { return tc.ackInterval }
			got := newSemaphoreConfig(cfg, "test-domain")
			assert.Equal(t, tc.expectedIdleTime, got.IdleTime())
			assert.Equal(t, tc.expectedAckInterval, got.UpdateAckInterval())
		})
	}
}

// Tests that Start builds the cache from every page of the scan.
func TestStart_Success(t *testing.T) {
	mgr, deps := setupMocksForSemaphoreManager(t, 3)
	scanRequest := func(pageToken []byte) *persistence.ScanSemaphoreBucketRequest {
		return &persistence.ScanSemaphoreBucketRequest{
			DomainID:      testBucketID.DomainID,
			SemaphoreName: testBucketID.SemaphoreName,
			Bucket:        testBucketID.Bucket,
			PageSize:      scanPageSize,
			NextPageToken: pageToken,
		}
	}
	// Token 1 has no row, token 2 is held by owner-x, and token 3 was released.
	gomock.InOrder(
		deps.mockTokenManager.EXPECT().ScanSemaphoreBucket(gomock.Any(), scanRequest(nil)).Return(&persistence.ScanSemaphoreBucketResponse{
			Ownerships:    []*persistence.SemaphoreOwnership{tokenRow(2, "owner-x"), tokenRow(3, "")},
			NextPageToken: []byte("page-2"),
		}, nil),
		deps.mockTokenManager.EXPECT().ScanSemaphoreBucket(gomock.Any(), scanRequest([]byte("page-2"))).Return(&persistence.ScanSemaphoreBucketResponse{
			Ownerships: []*persistence.SemaphoreOwnership{ownerRow("owner-x", 2)},
		}, nil),
	)

	require.NoError(t, mgr.Start(context.Background()))
	heldByX := tokenEntry{state: tokenHeld, owner: "owner-x"}
	assert.Equal(t, map[int]tokenEntry{1: free, 2: heldByX, 3: free}, cacheEntries(mgr.currentCache()))
	assert.Equal(t, map[string]int{"owner-x": 2}, copyTokenByOwner(mgr.currentCache()))
	assertCacheConsistent(t, mgr.currentCache())
}

func TestStart_Fails(t *testing.T) {
	semaphoreOf := func(size, bucketSize int) *persistence.GetSemaphoreResponse {
		return &persistence.GetSemaphoreResponse{Semaphore: &persistence.SemaphoreMetadata{
			DomainID:      testBucketID.DomainID,
			SemaphoreName: testBucketID.SemaphoreName,
			Size:          size,
			BucketSize:    bucketSize,
		}}
	}
	claimed := func(tasks *persistence.MockSemaphoreTaskManager) {
		tasks.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).
			Return(&persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 1}, nil)
	}

	testCases := []struct {
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
			name: "failure - claiming the queue fails",
			claim: func(tasks *persistence.MockSemaphoreTaskManager) {
				tasks.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
		{
			name: "failure - semaphore does not exist",
			metadata: func(md *persistence.MockSemaphoreMetadataManager) {
				md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{Message: "not found"})
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.EntityNotExistsError)) },
		},
		{
			name: "failure - read returns no semaphore",
			metadata: func(md *persistence.MockSemaphoreMetadataManager) {
				md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(&persistence.GetSemaphoreResponse{}, nil)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.InternalServiceError)) },
		},
		{
			// The default semaphore is one bucket of 2 tokens, so only bucket 0 exists.
			name:      "failure - bucket 1 does not exist",
			bucket:    1,
			assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.BadRequestError)) },
		},
		{
			// CreateSemaphore rejects a size below 1, so the row is corrupt.
			name: "failure - metadata has no size",
			metadata: func(md *persistence.MockSemaphoreMetadataManager) {
				md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(semaphoreOf(0, 2), nil)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.InternalServiceError)) },
		},
		{
			// The first page's rows must not be installed.
			name:  "failure - scan fails on its second page",
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
						return nil, assert.AnError
					})
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
		{
			name:  "failure - scan returns no response and no error",
			claim: claimed,
			scan: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).Return(nil, nil)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.InternalServiceError)) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// A failed metadata read must write nothing to semaphore_tasks.
			tasks := persistence.NewMockSemaphoreTaskManager(ctrl)
			if tc.claim != nil {
				tc.claim(tasks)
			}
			var stopped []Manager
			mgr, deps := setupMocksForSemaphoreManager(t, 2, func(p *ManagerParams) {
				p.ID.Bucket = tc.bucket
				if tc.metadata != nil {
					md := persistence.NewMockSemaphoreMetadataManager(ctrl)
					tc.metadata(md)
					p.Metadata = md
				}
				p.Tasks = tasks
				p.OnStopFn = func(s Manager) { stopped = append(stopped, s) }
			})
			if tc.scan != nil {
				tc.scan(deps.mockTokenManager)
			}

			tc.assertErr(t, mgr.Start(context.Background()))
			assert.Equal(t, 0, mgr.currentCache().freeTokenCount(), "nothing is installed")
			assert.Empty(t, copyTokenByOwner(mgr.currentCache()))
			assert.Equal(t, managerStateStopped, getManagerState(mgr))
			require.Len(t, stopped, 1)
			assert.Same(t, mgr, stopped[0], "OnStopFn gets the manager itself, so the registry removes only it")
		})
	}
}

// Tests that a second Start returns at once without scanning again.
func TestStart_Twice(t *testing.T) {
	testCases := []struct {
		name string
		// setup returns a manager whose first Start expects one scan, so a second scan fails the test.
		setup       func(t *testing.T) *semaphoreManagerImpl
		expectedErr error
	}{
		{
			// Not waited on: the load runs under the scan's deadline, not the caller's.
			name: "success - the first Start is still scanning",
			setup: func(t *testing.T) *semaphoreManagerImpl {
				mgr, deps := setupMocksForSemaphoreManager(t, 2)
				releaseScanAndWaitForStart := startWithBlockedScan(mgr, deps)
				t.Cleanup(func() { assert.NoError(t, releaseScanAndWaitForStart()) })
				return mgr
			},
		},
		{
			name: "success - the first Start finished loading",
			setup: func(t *testing.T) *semaphoreManagerImpl {
				mgr, _ := startTestManager(t, 2, nil)
				return mgr
			},
		},
		{
			name: "failure - the manager was stopped after loading, ErrNotReady",
			setup: func(t *testing.T) *semaphoreManagerImpl {
				mgr, _ := startTestManager(t, 2, nil)
				mgr.Stop()
				return mgr
			},
			expectedErr: ErrNotReady,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := tc.setup(t)

			done := make(chan error, 1)
			go func() { done <- mgr.Start(context.Background()) }()
			select {
			case err := <-done:
				if tc.expectedErr != nil {
					assert.ErrorIs(t, err, tc.expectedErr)
				} else {
					assert.NoError(t, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Start blocked instead of returning at once")
			}
		})
	}
}

// Tests that Stop during the scan releases waiting callers at once, and that the late scan does
// not bring the manager back up.
func TestStop_WhileStartIsScanning(t *testing.T) {
	mgr, deps := setupMocksForSemaphoreManager(t, 1)
	releaseScanAndWaitForStart := startWithBlockedScan(mgr, deps)

	mgr.Stop()
	select {
	case <-mgr.startupDoneCh:
	default:
		t.Fatal("Stop did not release callers waiting on startup while the scan was still running")
	}

	assert.ErrorIs(t, releaseScanAndWaitForStart(), ErrNotReady, "Start reports that it lost the bucket")
	assert.Equal(t, managerStateStopped, getManagerState(mgr))
	assert.Equal(t, 0, mgr.currentCache().freeTokenCount(), "the late scan's cache is not installed")
}

// Tests that a caller waiting on startup gives up at its own deadline while the scan is still
// running.
func TestAwaitStartup_CallerDeadline(t *testing.T) {
	mgr, deps := setupMocksForSemaphoreManager(t, 1)
	releaseScanAndWaitForStart := startWithBlockedScan(mgr, deps)
	defer func() { assert.NoError(t, releaseScanAndWaitForStart()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := mgr.Acquire(ctx, "caller", nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// Tests that the idle clock starts after the load and is reset by Acquire, not by GetTokenByOwner.
func TestIdleUnload(t *testing.T) {
	defer goleak.VerifyNone(t)
	mgr, deps := setupMocksForSemaphoreManager(t, 1)
	deps.mockTokenManager.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
	isStopped := func() bool { return getManagerState(mgr) == managerStateStopped }

	// A load slower than the idle TTL does not unload the bucket.
	releaseScanAndWaitForStart := startWithBlockedScan(mgr, deps)
	deps.mockTimeSource.Advance(2 * testIdleTTL)
	require.NoError(t, releaseScanAndWaitForStart())
	_, err := mgr.Acquire(context.Background(), "caller", nil)
	require.NoError(t, err)

	deps.mockTimeSource.Advance(testIdleTTL / 2)
	require.Never(t, isStopped, 50*time.Millisecond, 5*time.Millisecond, "stopped within the idle TTL of an Acquire")

	_, err = mgr.GetTokenByOwner(context.Background(), "caller")
	require.NoError(t, err)
	deps.mockTimeSource.Advance(testIdleTTL * 3 / 4)
	require.Eventually(t, isStopped, time.Second, 5*time.Millisecond, "did not stop an idle TTL after the Acquire")
}

// Tests that a bucket whose claim on the queue is lost stops.
func TestLostClaim(t *testing.T) {
	conditionFailed := &persistence.ConditionFailedError{Msg: "range_id changed"}
	testCases := []struct {
		name string
		// loseClaim makes mgr find out another host has claimed the bucket.
		loseClaim func(t *testing.T, mgr *semaphoreManagerImpl, deps *mockDeps)
	}{
		{
			name: "reader - ack level save fails with ConditionFailedError",
			loseClaim: func(_ *testing.T, _ *semaphoreManagerImpl, deps *mockDeps) {
				deps.mockTaskManager.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).Return(nil, conditionFailed)
				// The idle clock and the ack level ticker.
				deps.mockTimeSource.BlockUntil(2)
				deps.mockTimeSource.Advance(testUpdateAckInterval())
			},
		},
		{
			name: "writer - waiter write fails with ConditionFailedError",
			loseClaim: func(t *testing.T, mgr *semaphoreManagerImpl, deps *mockDeps) {
				deps.mockTaskManager.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).Return(nil, conditionFailed)
				// The failed write still moves maxReadLevel, so the reader may read before the bucket stops.
				deps.mockTaskManager.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).
					Return(&persistence.GetSemaphoreTasksResponse{}, nil).AnyTimes()
				// ErrNotReady if the manager stops the writer before the writer answers.
				err := mgr.taskWriter.appendTask(testContext(t), &persistence.SemaphoreTask{WorkflowID: "wf"})
				assert.True(t, errors.Is(err, conditionFailed) || errors.Is(err, ErrNotReady), "got %v", err)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer goleak.VerifyNone(t)
			stopped := make(chan Manager, 1)
			mgr, deps := startTestManager(t, 1, nil, func(p *ManagerParams) {
				// Longer than the ack level interval, so the bucket is not unloaded for being idle.
				p.Cfg = testConfig(2 * testUpdateAckInterval())
				p.OnStopFn = func(s Manager) { stopped <- s }
			})

			tc.loseClaim(t, mgr, deps)

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
}

// Tests Acquire's outcomes.
func TestAcquire(t *testing.T) {
	owner := commonsemaphore.Owner{WorkflowID: "wf", RunID: "run", HoldID: 7}
	deadline := time.Unix(1767258000, 0)
	// fullBucket is the bucket's only token, held by another owner.
	fullBucket := []*persistence.SemaphoreOwnership{tokenRow(1, "other-owner"), ownerRow("other-owner", 1)}
	// expectGrant expects one grant of token 1 for owner.
	expectGrant := func(deps *mockDeps) *gomock.Call {
		return deps.mockTokenManager.EXPECT().GrantSemaphoreToken(gomock.Any(), &persistence.GrantSemaphoreTokenRequest{
			DomainID:      testBucketID.DomainID,
			SemaphoreName: testBucketID.SemaphoreName,
			Bucket:        testBucketID.Bucket,
			TokenID:       1,
			OwnerID:       owner.String(),
		}).Times(1)
	}
	// expectWaiterWrite expects one waiter write for owner with deadline. A written waiter signals
	// the reader, which reads.
	expectWaiterWrite := func(deps *mockDeps) *gomock.Call {
		deps.mockTaskManager.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().
			Return(&persistence.GetSemaphoreTasksResponse{}, nil)
		waiter := &persistence.SemaphoreTask{
			TaskID:          1,
			WorkflowID:      owner.WorkflowID,
			RunID:           owner.RunID,
			HoldID:          owner.HoldID,
			AcquireDeadline: &deadline,
			CreatedTime:     deps.mockTimeSource.Now(),
		}
		return deps.mockTaskManager.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, waiter)).Times(1)
	}

	testCases := []struct {
		name string
		// rows are what the startup scan reads. Nil leaves the bucket's only token free.
		rows []*persistence.SemaphoreOwnership
		// managerStopped stops the manager before Acquire.
		managerStopped bool
		// setupMocks expects Acquire's database calls. Nil expects none.
		setupMocks func(deps *mockDeps)
		expected   AcquireResult
		// assertErr checks the error. Nil expects no error.
		assertErr func(t *testing.T, err error)
	}{
		{
			name:           "failure - manager is stopped, retryable ErrNotReady",
			managerStopped: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotReady)
				assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
			},
		},
		{
			name: "success - grant succeeds, Acquired",
			setupMocks: func(deps *mockDeps) {
				expectGrant(deps).Return(&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
			},
			expected: AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1},
		},
		{
			name: "failure - grant fails",
			setupMocks: func(deps *mockDeps) {
				expectGrant(deps).Return(nil, assert.AnError)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
		{
			name: "success - bucket is full, Queued",
			rows: fullBucket,
			setupMocks: func(deps *mockDeps) {
				expectWaiterWrite(deps).Return(&persistence.CreateSemaphoreTasksResponse{}, nil)
			},
			expected: AcquireResult{Outcome: types.SemaphoreAcquireOutcomeQueued},
		},
		{
			name: "failure - bucket is full, enqueue fails",
			rows: fullBucket,
			setupMocks: func(deps *mockDeps) {
				expectWaiterWrite(deps).Return(nil, assert.AnError)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startTestManager(t, 1, tc.rows)
			if tc.managerStopped {
				mgr.Stop()
			}
			if tc.setupMocks != nil {
				tc.setupMocks(deps)
			}

			got, err := mgr.Acquire(context.Background(), owner.String(), &deadline)
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expected, got)
		})
	}
}

// Tests that enqueue writes ownerID as a waiter and signals the reader.
func TestEnqueue(t *testing.T) {
	mgr, deps := startTestManager(t, 1, nil)
	owner := commonsemaphore.Owner{WorkflowID: "wf", RunID: "run", HoldID: 7}
	deadline := time.Unix(1767258000, 0)
	deps.mockTaskManager.EXPECT().CreateSemaphoreTasks(gomock.Any(), createRequest(1, &persistence.SemaphoreTask{
		TaskID:          1,
		WorkflowID:      owner.WorkflowID,
		RunID:           owner.RunID,
		HoldID:          owner.HoldID,
		AcquireDeadline: &deadline,
		CreatedTime:     deps.mockTimeSource.Now(),
	})).Times(1).Return(&persistence.CreateSemaphoreTasksResponse{}, nil)
	// read is sent to when the reader reads, which it does once enqueue signals it.
	read := make(chan struct{}, 1)
	deps.mockTaskManager.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(context.Context, *persistence.GetSemaphoreTasksRequest) (*persistence.GetSemaphoreTasksResponse, error) {
			select {
			case read <- struct{}{}:
			default:
			}
			return &persistence.GetSemaphoreTasksResponse{}, nil
		})

	require.NoError(t, mgr.enqueue(context.Background(), owner.String(), &deadline))
	select {
	case <-read:
	case <-time.After(time.Second):
		t.Fatal("the reader was not signaled after the waiter was written")
	}
}

func TestEnqueue_BadOwnerID(t *testing.T) {
	mgr, _ := startTestManager(t, 1, nil)
	err := mgr.enqueue(context.Background(), "caller", nil)
	assert.ErrorAs(t, err, new(*types.BadRequestError))
}

// Tests how grant uses its steps. The cache says "caller" holds token 1, the bucket's only token.
func TestGrant(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	testCases := []struct {
		name string
		// bucketLocked holds the bucket lock during grant, as another grant would.
		bucketLocked    bool
		setupMocks      func(deps *mockDeps)
		expectedTokenID int
		assertErr       func(t *testing.T, err error)
	}{
		{
			name:         "success - DB confirms the caller's token, returned without the bucket lock",
			bucketLocked: true,
			setupMocks: func(deps *mockDeps) {
				expectGetTokenRow(deps.mockTokenManager, 1).Return(tokenRowRead(1, "caller"), nil)
			},
			expectedTokenID: 1,
		},
		{
			name: "success - caller was granted while waiting for the lock, returned without a write",
			setupMocks: func(deps *mockDeps) {
				expectGetTokenRow(deps.mockTokenManager, 1).Return(tokenRowRead(1, ""), nil)
				expectGetTokenRow(deps.mockTokenManager, 1).Return(tokenRowRead(1, "caller"), nil)
			},
			expectedTokenID: 1,
		},
		{
			name: "success - caller's token was freed, grants a free token",
			setupMocks: func(deps *mockDeps) {
				expectGetTokenRow(deps.mockTokenManager, 1).Times(2).Return(tokenRowRead(1, ""), nil)
				expectGrantToken(deps.mockTokenManager, 1).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
			},
			expectedTokenID: 1,
		},
		{
			name: "failure - read fails, returns the error",
			setupMocks: func(deps *mockDeps) {
				expectGetTokenRow(deps.mockTokenManager, 1).Return(nil, readErr)
			},
			expectedTokenID: emptyTokenID,
			assertErr:       func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startTestManager(t, 1, []*persistence.SemaphoreOwnership{tokenRow(1, "caller"), ownerRow("caller", 1)})
			tc.setupMocks(deps)
			if tc.bucketLocked {
				require.NoError(t, mgr.bucketLock.Lock(context.Background()))
				defer mgr.bucketLock.Unlock()
			}

			// Fails grant instead of hanging if it waits for the held lock.
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, err := mgr.grant(ctx, "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedTokenID, got)
			if !tc.bucketLocked {
				assertBucketUnlocked(t, mgr)
			}
		})
	}
}

// Tests that confirmHeldToken returns the caller's cached token only if the DB row names the
// caller, and never changes the cache.
func TestGrant_ConfirmHeldToken(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	testCases := []struct {
		name string
		// callerCached starts the cache with "caller" holding token 1. Otherwise no read is expected.
		callerCached    bool
		dbRow           *persistence.SemaphoreOwnership
		readErr         error
		expectedTokenID int
		assertErr       func(t *testing.T, err error)
	}{
		{
			name:            "success - DB row names the caller, returns token 1",
			callerCached:    true,
			dbRow:           tokenRow(1, "caller"),
			expectedTokenID: 1,
		},
		{
			name:            "success - no cached token, no read",
			expectedTokenID: emptyTokenID,
		},
		{
			name:            "success - DB row is FREE, no token",
			callerCached:    true,
			dbRow:           tokenRow(1, ""),
			expectedTokenID: emptyTokenID,
		},
		{
			name:            "success - DB row names another owner, no token",
			callerCached:    true,
			dbRow:           tokenRow(1, "other-owner"),
			expectedTokenID: emptyTokenID,
		},
		{
			name:            "success - no DB row, no token",
			callerCached:    true,
			readErr:         &types.EntityNotExistsError{Message: "not found"},
			expectedTokenID: emptyTokenID,
		},
		{
			name:            "failure - read fails, returns the error",
			callerCached:    true,
			readErr:         readErr,
			expectedTokenID: emptyTokenID,
			assertErr:       func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rows []*persistence.SemaphoreOwnership
			if tc.callerCached {
				rows = []*persistence.SemaphoreOwnership{tokenRow(1, "caller"), ownerRow("caller", 1)}
			}
			mgr, deps := startTestManager(t, 1, rows)
			if tc.callerCached {
				expectGetTokenRow(deps.mockTokenManager, 1).Return(&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tc.dbRow}, tc.readErr)
			}
			before := cacheEntries(mgr.currentCache())

			got, err := mgr.confirmHeldToken(context.Background(), "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedTokenID, got)
			assert.Equal(t, before, cacheEntries(mgr.currentCache()), "the cache is unchanged")
		})
	}
}

// Tests that syncHeldTokenLocked returns the caller's cached token only if the DB row names the
// caller, and fixes token 1's cache entry from the row.
func TestGrant_SyncHeldTokenLocked(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	testCases := []struct {
		name string
		// callerCached starts the cache with "caller" holding token 1. Otherwise no read is expected.
		callerCached       bool
		dbRow              *persistence.SemaphoreOwnership
		readErr            error
		expectedTokenID    int
		expectedCacheEntry tokenEntry
		assertErr          func(t *testing.T, err error)
	}{
		{
			name:               "success - DB row names the caller, returns token 1",
			callerCached:       true,
			dbRow:              tokenRow(1, "caller"),
			expectedTokenID:    1,
			expectedCacheEntry: heldByCaller,
		},
		{
			name:               "success - no cached token, no read",
			expectedTokenID:    emptyTokenID,
			expectedCacheEntry: free,
		},
		{
			name:               "success - DB row is FREE, token 1 becomes free",
			callerCached:       true,
			dbRow:              tokenRow(1, ""),
			expectedTokenID:    emptyTokenID,
			expectedCacheEntry: free,
		},
		{
			name:               "success - DB row names another owner, token 1 becomes unavailable",
			callerCached:       true,
			dbRow:              tokenRow(1, "other-owner"),
			expectedTokenID:    emptyTokenID,
			expectedCacheEntry: unavailable,
		},
		{
			name:               "success - no DB row, token 1 becomes unavailable",
			callerCached:       true,
			readErr:            &types.EntityNotExistsError{Message: "not found"},
			expectedTokenID:    emptyTokenID,
			expectedCacheEntry: unavailable,
		},
		{
			name:               "failure - read fails, cache unchanged",
			callerCached:       true,
			readErr:            readErr,
			expectedTokenID:    emptyTokenID,
			expectedCacheEntry: heldByCaller,
			assertErr:          func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rows []*persistence.SemaphoreOwnership
			if tc.callerCached {
				rows = []*persistence.SemaphoreOwnership{tokenRow(1, "caller"), ownerRow("caller", 1)}
			}
			mgr, deps := startTestManager(t, 1, rows)
			if tc.callerCached {
				expectGetTokenRow(deps.mockTokenManager, 1).Return(&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tc.dbRow}, tc.readErr)
			}

			got, err := mgr.syncHeldTokenLocked(context.Background(), mgr.currentCache(), "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedTokenID, got)
			assert.Equal(t, tc.expectedCacheEntry, getTokenEntry(mgr.currentCache(), 1))
			assertCacheConsistent(t, mgr.currentCache())
		})
	}
}

// Tests grantFreeTokenLocked for each write answer. Token 2 is held, so the first write goes to token 1.
func TestGrant_FreeToken(t *testing.T) {
	writeErr := &persistence.TimeoutError{Msg: "write timed out"}
	taken := &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}
	testCases := []struct {
		name         string
		resp         *persistence.GrantSemaphoreTokenResponse
		writeErr     error
		callerGaveUp bool
		// justReloaded sets the last reload to just under minBucketReloadInterval ago.
		justReloaded bool
		// expectRetry expects a reload that shows token 2 freed, and a write of token 2 that applies.
		expectRetry          bool
		expectedTokenID      int
		expectedCacheEntries map[int]tokenEntry
		assertErr            func(t *testing.T, err error)
	}{
		{
			name:                 "success - write applies, grants token 1",
			resp:                 &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied},
			expectedTokenID:      1,
			expectedCacheEntries: map[int]tokenEntry{1: heldByCaller, 2: heldByOther},
		},
		{
			name:                 "success - write says the caller already holds token 2, returns token 2",
			resp:                 &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 2},
			expectedTokenID:      2,
			expectedCacheEntries: map[int]tokenEntry{1: free, 2: heldByCaller},
		},
		{
			name:                 "failure - write fails, token 1 stays free",
			writeErr:             writeErr,
			expectedTokenID:      emptyTokenID,
			expectedCacheEntries: map[int]tokenEntry{1: free, 2: heldByOther},
			assertErr:            func(t *testing.T, err error) { assert.ErrorIs(t, err, writeErr) },
		},
		{
			name:                 "failure - write returns an unknown outcome, token 1 stays free",
			resp:                 &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown},
			expectedTokenID:      emptyTokenID,
			expectedCacheEntries: map[int]tokenEntry{1: free, 2: heldByOther},
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "unexpected grant outcome")
			},
		},
		{
			name:                 "failure - write says the caller holds token 9 outside the bucket, token 1 stays free",
			resp:                 &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 9},
			expectedTokenID:      emptyTokenID,
			expectedCacheEntries: map[int]tokenEntry{1: free, 2: heldByOther},
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "AlreadyHeld with token 9, outside tokens 1 to 2")
			},
		},
		{
			name:                 "success - slot taken, reloads and grants token 2",
			resp:                 taken,
			expectRetry:          true,
			expectedTokenID:      2,
			expectedCacheEntries: map[int]tokenEntry{1: heldByA, 2: heldByCaller},
		},
		{
			name:                 "success - slot taken, bucket was just reloaded, no reload and no token",
			resp:                 taken,
			justReloaded:         true,
			expectedTokenID:      emptyTokenID,
			expectedCacheEntries: map[int]tokenEntry{1: unavailable, 2: heldByOther},
		},
		{
			name:                 "failure - caller already gave up, no write, token 1 stays free",
			callerGaveUp:         true,
			expectedTokenID:      emptyTokenID,
			expectedCacheEntries: map[int]tokenEntry{1: free, 2: heldByOther},
			assertErr:            func(t *testing.T, err error) { assert.ErrorIs(t, err, context.Canceled) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startTestManager(t, 2, []*persistence.SemaphoreOwnership{tokenRow(2, "other-owner"), ownerRow("other-owner", 2)})
			// Old enough that a refused write reloads.
			mgr.lastReload = deps.mockTimeSource.Now().Add(-minBucketReloadInterval)
			if tc.justReloaded {
				mgr.lastReload = deps.mockTimeSource.Now().Add(-minBucketReloadInterval + time.Millisecond)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.callerGaveUp {
				cancel()
			} else {
				expectGrantToken(deps.mockTokenManager, 1).Return(tc.resp, tc.writeErr)
			}
			if tc.expectRetry {
				deps.mockTokenManager.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).Return(
					&persistence.ScanSemaphoreBucketResponse{Ownerships: []*persistence.SemaphoreOwnership{
						tokenRow(1, "owner-a"), ownerRow("owner-a", 1), tokenRow(2, ""),
					}}, nil)
				expectGrantToken(deps.mockTokenManager, 2).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
			}

			got, err := mgr.grantFreeTokenLocked(ctx, mgr.currentCache(), "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedTokenID, got)
			assert.Equal(t, tc.expectedCacheEntries, cacheEntries(mgr.currentCache()))
			assertCacheConsistent(t, mgr.currentCache())
		})
	}
}

// Tests Release by owner. The bucket has one token.
func TestRelease(t *testing.T) {
	applied := &persistence.ReleaseSemaphoreTokenResponse{Applied: true}
	testCases := []struct {
		name           string
		rows           []*persistence.SemaphoreOwnership
		managerStopped bool
		setupMocks     func(deps *mockDeps)
		assertErr      func(t *testing.T, err error)
	}{
		{
			name: "success - caller's token is cached, releases it without reading the owner row",
			rows: []*persistence.SemaphoreOwnership{tokenRow(1, "caller"), ownerRow("caller", 1)},
			setupMocks: func(deps *mockDeps) {
				expectReleaseToken(deps.mockTokenManager, 1).Return(applied, nil)
			},
		},
		{
			name: "success - caller's token isn't cached, releases the token its owner row names",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetOwnerRow(deps.mockTokenManager).Return(callerOwnerRow(1), nil),
					expectReleaseToken(deps.mockTokenManager, 1).Return(applied, nil),
				)
			},
		},
		{
			name: "success - caller holds nothing, no release",
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager).Return(nil, &types.EntityNotExistsError{Message: "not found"})
			},
		},
		{
			name:           "failure - manager is stopped, retryable ErrNotReady",
			managerStopped: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotReady)
				assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startTestManager(t, 1, tc.rows)
			if tc.managerStopped {
				mgr.Stop()
			}
			if tc.setupMocks != nil {
				tc.setupMocks(deps)
			}

			err := mgr.Release(context.Background(), "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assertBucketUnlocked(t, mgr)
		})
	}
}

// Tests tryReleaseTokenLocked for each write answer. The cache says "caller" holds token 1, the
// bucket's only token.
func TestRelease_TryReleaseToken(t *testing.T) {
	writeErr := &persistence.TimeoutError{Msg: "write timed out"}
	testCases := []struct {
		name    string
		tokenID int
		// resp and writeErr answer the write. Both nil expects no write.
		resp               *persistence.ReleaseSemaphoreTokenResponse
		writeErr           error
		expectedApplied    bool
		expectedCacheEntry tokenEntry
		assertErr          func(t *testing.T, err error)
	}{
		{
			name:               "success - write applies, token 1 is free",
			tokenID:            1,
			resp:               &persistence.ReleaseSemaphoreTokenResponse{Applied: true},
			expectedApplied:    true,
			expectedCacheEntry: free,
		},
		{
			name:               "success - write does not apply, token 1 is unavailable",
			tokenID:            1,
			resp:               &persistence.ReleaseSemaphoreTokenResponse{Applied: false},
			expectedCacheEntry: unavailable,
		},
		{
			name:               "failure - write fails, token 1 is free",
			tokenID:            1,
			writeErr:           writeErr,
			expectedCacheEntry: free,
			assertErr:          func(t *testing.T, err error) { assert.ErrorIs(t, err, writeErr) },
		},
		{
			name:               "failure - token 2 is outside the bucket, no write",
			tokenID:            2,
			expectedCacheEntry: heldByCaller,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "token 2 of owner caller is outside the range")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startTestManager(t, 1, []*persistence.SemaphoreOwnership{tokenRow(1, "caller"), ownerRow("caller", 1)})
			if tc.resp != nil || tc.writeErr != nil {
				expectReleaseToken(deps.mockTokenManager, tc.tokenID).Return(tc.resp, tc.writeErr)
			}

			applied, err := mgr.tryReleaseTokenLocked(context.Background(), mgr.currentCache(), tc.tokenID, "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedApplied, applied)
			assert.Equal(t, tc.expectedCacheEntry, getTokenEntry(mgr.currentCache(), 1))
			assertCacheConsistent(t, mgr.currentCache())
		})
	}
}

// Tests releaseOwnerRowTokenLocked's failures. The bucket has one token.
func TestRelease_OwnerRowToken(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	testCases := []struct {
		name       string
		setupMocks func(deps *mockDeps)
		assertErr  func(t *testing.T, err error)
	}{
		{
			name: "failure - owner row read fails, returns the error",
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager).Return(nil, readErr)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
		{
			name: "failure - owner row names token 1 but its release does not apply, InternalServiceError",
			setupMocks: func(deps *mockDeps) {
				gomock.InOrder(
					expectGetOwnerRow(deps.mockTokenManager).Return(callerOwnerRow(1), nil),
					expectReleaseToken(deps.mockTokenManager, 1).Return(&persistence.ReleaseSemaphoreTokenResponse{Applied: false}, nil),
				)
			},
			assertErr: func(t *testing.T, err error) { assert.IsType(t, &types.InternalServiceError{}, err) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startTestManager(t, 1, nil)
			tc.setupMocks(deps)

			err := mgr.releaseOwnerRowTokenLocked(context.Background(), mgr.currentCache(), "caller")
			tc.assertErr(t, err)
		})
	}
}

// Tests GetTokenByOwner. The bucket has one token, and a grant holds the bucket lock during every
// lookup, since a lookup never waits for it.
func TestGetTokenByOwner(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	testCases := []struct {
		name            string
		rows            []*persistence.SemaphoreOwnership
		managerStopped  bool
		setupMocks      func(deps *mockDeps)
		expectedTokenID int
		assertErr       func(t *testing.T, err error)
	}{
		{
			name:            "success - caller's token is cached, returned without a read",
			rows:            []*persistence.SemaphoreOwnership{tokenRow(1, "caller"), ownerRow("caller", 1)},
			expectedTokenID: 1,
		},
		{
			name: "success - caller's token isn't cached, returns the token its owner row names",
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager).Return(callerOwnerRow(1), nil)
			},
			expectedTokenID: 1,
		},
		{
			name: "success - caller holds nothing, returns 0",
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager).Return(nil, &types.EntityNotExistsError{Message: "not found"})
			},
		},
		{
			name: "failure - owner row read fails",
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager).Return(nil, readErr)
			},
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
		{
			name: "failure - owner row names token 2 outside the bucket",
			setupMocks: func(deps *mockDeps) {
				expectGetOwnerRow(deps.mockTokenManager).Return(callerOwnerRow(2), nil)
			},
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "owner caller's owner row names token 2, outside tokens 1 to 1")
			},
		},
		{
			name:           "failure - manager is stopped, retryable ErrNotReady",
			managerStopped: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotReady)
				assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, deps := startTestManager(t, 1, tc.rows)
			if tc.managerStopped {
				mgr.Stop()
			}
			if tc.setupMocks != nil {
				tc.setupMocks(deps)
			}
			require.NoError(t, mgr.bucketLock.Lock(context.Background()))
			defer mgr.bucketLock.Unlock()
			before := cacheEntries(mgr.currentCache())

			// Fails the lookup instead of hanging if it waits for the lock.
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tokenID, err := mgr.GetTokenByOwner(ctx, "caller")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedTokenID, tokenID)
			assert.Equal(t, before, cacheEntries(mgr.currentCache()), "a lookup never changes the cache")
		})
	}
}

// Tests that when many owners call Acquire and Release at once, the bucket makes one database write
// at a time and the cache stays correct.
func TestManager_ConcurrentCalls(t *testing.T) {
	const tokens, owners = 3, 10
	mgr, deps := startTestManager(t, tokens, nil)
	// Owners that find no free token are queued.
	deps.mockTaskManager.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&persistence.CreateSemaphoreTasksResponse{}, nil)
	deps.mockTaskManager.EXPECT().GetSemaphoreTasks(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&persistence.GetSemaphoreTasksResponse{}, nil)

	// write fails the test if another write is running. A real write takes milliseconds; without
	// the pause, two unlocked writes rarely overlap and the test misses them.
	var writing atomic.Bool
	write := func() {
		assert.True(t, writing.CompareAndSwap(false, true), "two writes ran at once")
		time.Sleep(time.Millisecond)
		writing.Store(false)
	}
	deps.mockTokenManager.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(context.Context, *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			write()
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
		})
	deps.mockTokenManager.EXPECT().ReleaseSemaphoreToken(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(context.Context, *persistence.ReleaseSemaphoreTokenRequest) (*persistence.ReleaseSemaphoreTokenResponse, error) {
			write()
			return &persistence.ReleaseSemaphoreTokenResponse{Applied: true}, nil
		})

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

	assert.Equal(t, tokens, mgr.currentCache().freeTokenCount(), "every acquired token was released")
	assertBucketUnlocked(t, mgr)
	assertCacheConsistent(t, mgr.currentCache())
}
