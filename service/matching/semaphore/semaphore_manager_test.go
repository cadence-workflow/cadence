package semaphore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
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

// testManagerParams returns valid params for a bucket of tokens 1 to tokenCount. The clock only
// moves when a test moves it, so nothing is evicted unless the test asks for it. Tests override
// fields as needed.
func testManagerParams(t *testing.T, m persistence.SemaphoreTokenManager, tokenCount int) ManagerParams {
	t.Helper()
	return ManagerParams{
		ID:         testBucketID,
		Tokens:     m,
		Metadata:   singleBucketMetadata(t, tokenCount),
		Logger:     testlogger.New(t),
		IdleTTL:    testIdleTTL,
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
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.state
}

// freeTokenCount returns how many tokens the manager's cache lists as free.
func freeTokenCount(mgr *semaphoreManagerImpl) int {
	return mgr.currentCache().freeTokenCount()
}

// assertNoReservedTokens checks that grant settled every token it reserved. A token left reserved
// is never offered again until the bucket reloads.
func assertNoReservedTokens(t *testing.T, mgr *semaphoreManagerImpl) {
	t.Helper()
	assert.Empty(t, tokensInState(mgr.currentCache(), tokenReserved), "every reserved token must be settled when grant returns")
}

func TestNewManagerValidatesItsParams(t *testing.T) {
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
		{name: "no logger", params: without(func(p *ManagerParams) { p.Logger = nil })},
		{name: "no idle ttl", params: without(func(p *ManagerParams) { p.IdleTTL = 0 })},
		{name: "negative idle ttl", params: without(func(p *ManagerParams) { p.IdleTTL = -time.Second })},
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
}

// Tests that Start builds the cache from every scan page.
func TestManagerLifecycle_StartLoadsTheCache(t *testing.T) {
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
}

// Tests that a failed Start returns the error, installs nothing, and stops the manager.
func TestManagerLifecycle_StartFails(t *testing.T) {
	semaphoreOf := func(size, bucketSize int) *persistence.GetSemaphoreResponse {
		return &persistence.GetSemaphoreResponse{Semaphore: &persistence.SemaphoreMetadata{
			DomainID:      testBucketID.DomainID,
			SemaphoreName: testBucketID.SemaphoreName,
			Size:          size,
			BucketSize:    bucketSize,
		}}
	}
	scanErr := errors.New("scan failed")

	tests := []struct {
		name   string
		bucket int
		// metadata stubs the metadata read. Nil reads one bucket of 2 tokens.
		metadata func(md *persistence.MockSemaphoreMetadataManager)
		// scan stubs the scan. Nil expects no scan.
		scan      func(m *persistence.MockSemaphoreTokenManager)
		assertErr func(t *testing.T, err error)
	}{
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
			name: "scan fails on its second page",
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
}

// Tests that a second Start returns at once without scanning.
func TestManagerLifecycle_StartCalledAgain(t *testing.T) {
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
}

// Tests that Stop during the scan releases waiting callers at once, and the late scan does not bring
// the manager back up.
func TestManagerLifecycle_StopWhileStartIsScanning(t *testing.T) {
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
}

// Tests that the idle clock does not run during the load, is reset by Acquire, and stops the manager
// once after the idle TTL.
func TestManagerLifecycle_IdleTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

	p := testManagerParams(t, m, 1)
	mockClock := clock.NewMockedTimeSource()
	p.TimeSource = mockClock
	// Room for two calls, so a second call is seen instead of blocking Stop.
	stopped := make(chan Manager, 2)
	p.OnStopFn = func(s Manager) { stopped <- s }
	mgr := newManagerFromParams(t, p)
	isStopped := func() bool { return getManagerState(mgr) == managerStateStopped }

	// A load slower than the idle TTL must not unload the bucket.
	releaseScanAndWaitForStart := startWithBlockedScan(m, mgr)
	mockClock.Advance(2 * testIdleTTL)
	require.NoError(t, releaseScanAndWaitForStart())
	// The engine calls Acquire right after Start.
	_, err := mgr.Acquire(context.Background(), "owner-a")
	require.NoError(t, err)

	// Within the idle TTL of the Acquire.
	mockClock.Advance(testIdleTTL / 2)
	require.Never(t, isStopped, 50*time.Millisecond, 5*time.Millisecond, "a bucket that just served a request stays loaded")

	mockClock.Advance(testIdleTTL)
	select {
	case s := <-stopped:
		assert.Same(t, mgr, s)
	case <-time.After(time.Second):
		t.Fatal("an idle manager did not stop itself")
	}
	assert.True(t, isStopped())

	// The engine can also stop it. A second Stop does nothing.
	mgr.Stop()
	select {
	case <-stopped:
		t.Fatal("a second Stop called OnStopFn again")
	default:
	}
}

// Tests that the cache stays consistent when Start, Stop and grants race.
func TestManagerLifecycle_StartAndStopRaceGrants(t *testing.T) {
	// The race is easy to miss in one round; ten rounds have caught an unlocked install every time.
	const rounds = 10
	// One grant per token.
	const grants = 8

	for range rounds {
		ctrl := gomock.NewController(t)
		m := persistence.NewMockSemaphoreTokenManager(ctrl)
		// Which calls happen depends on who wins the race, so every call is optional.
		m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Return(
			&persistence.ScanSemaphoreBucketResponse{}, nil).AnyTimes()
		m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
			&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil).AnyTimes()

		mgr := newTestManager(t, m, grants)

		var wg sync.WaitGroup
		race := func(f func()) {
			wg.Add(1)
			go func() { defer wg.Done(); f() }()
		}

		// Results depend on who wins, so they are ignored.
		race(func() { _ = mgr.Start(context.Background()) })
		race(func() { mgr.Stop() })
		for g := range grants {
			race(func() { _, _ = mgr.grant(context.Background(), fmt.Sprintf("owner-%d", g)) })
		}
		wg.Wait()

		assertCacheConsistent(t, mgr.currentCache())
	}
}

func TestAcquire(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl
		ownerID string
		// timeout is the caller's deadline. Cases that expect a quick answer use 5s, so a blocked
		// Acquire fails the test instead of hanging it.
		timeout time.Duration
		want    AcquireResult
		// assertErr checks the error. Nil expects no error.
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "the owner ID is empty",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				return startManager(t, m, 1, nil)
			},
			ownerID:   "",
			timeout:   5 * time.Second,
			assertErr: func(t *testing.T, err error) { assert.IsType(t, &types.BadRequestError{}, err) },
		},
		{
			name: "the manager is stopped",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				mgr := startManager(t, m, 1, nil)
				mgr.Stop()
				return mgr
			},
			ownerID: "owner-a",
			timeout: 5 * time.Second,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrNotReady)
				// Retryable: a retry builds a fresh manager that may load.
				assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
			},
		},
		{
			name: "the caller's deadline passes while Start is scanning",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				mgr := newTestManager(t, m, 1)
				releaseScanAndWaitForStart := startWithBlockedScan(m, mgr)
				t.Cleanup(func() { assert.NoError(t, releaseScanAndWaitForStart()) })
				return mgr
			},
			ownerID:   "owner-a",
			timeout:   20 * time.Millisecond,
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, context.DeadlineExceeded) },
		},
		{
			name: "grant succeeds",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				mgr := startManager(t, m, 1, nil)
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
				return mgr
			},
			ownerID: "owner-a",
			timeout: 5 * time.Second,
			want:    AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1},
		},
		{
			name: "grant fails",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				mgr := startManager(t, m, 1, nil)
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				return mgr
			},
			ownerID:   "owner-a",
			timeout:   5 * time.Second,
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// Only the grant cases expect a write, so the mock fails any other case that writes.
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := tc.setup(t, m)

			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			got, err := mgr.Acquire(ctx, tc.ownerID)
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// Tests how grant handles each answer to a single write.
func TestGrant_OneWrite(t *testing.T) {
	writeErr := &persistence.TimeoutError{Msg: "write timed out"}
	tests := []struct {
		name          string
		tokenCount    int
		rows          []*persistence.SemaphoreOwnership
		resp          *persistence.GrantSemaphoreTokenResponse
		writeErr      error
		want          AcquireResult
		wantFreeCount int
		// assertErr checks the error. Nil expects no error, and owner-a recorded on want's token.
		assertErr func(t *testing.T, err error)
	}{
		{
			name:          "write applies",
			tokenCount:    1,
			resp:          &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied},
			want:          AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1},
			wantFreeCount: 0,
		},
		{
			// Tokens 1 and 2. Token 2's row names owner-a but has no owner row, so it is unavailable
			// and the write goes to token 1.
			name:          "write says owner-a already holds token 2",
			tokenCount:    2,
			rows:          []*persistence.SemaphoreOwnership{tokenRow(2, "owner-a")},
			resp:          &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 2},
			want:          AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 2},
			wantFreeCount: 1,
		},
		{
			name:          "write returns an error",
			tokenCount:    1,
			writeErr:      writeErr,
			wantFreeCount: 1,
			assertErr:     func(t *testing.T, err error) { assert.ErrorIs(t, err, writeErr) },
		},
		{
			// The zero value, which is what a store that never set the field returns.
			name:          "write returns an unknown outcome",
			tokenCount:    1,
			resp:          &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown},
			wantFreeCount: 1,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "unexpected grant outcome")
			},
		},
		{
			name:          "write says AlreadyHeld on a token outside the bucket",
			tokenCount:    1,
			resp:          &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 9},
			wantFreeCount: 1,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.ErrorContains(t, err, "AlreadyHeld with an invalid token")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, tc.tokenCount, tc.rows)

			// Exactly one write, so a retry fails the test.
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
				func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
					assert.Equal(t, &persistence.GrantSemaphoreTokenRequest{
						DomainID:      testBucketID.DomainID,
						SemaphoreName: testBucketID.SemaphoreName,
						Bucket:        testBucketID.Bucket,
						TokenID:       1,
						OwnerID:       "owner-a",
					}, req)
					return tc.resp, tc.writeErr
				})

			got, err := mgr.grant(context.Background(), "owner-a")
			wantOwners := map[string]int{}
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
				wantOwners["owner-a"] = tc.want.TokenID
			}
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantFreeCount, freeTokenCount(mgr))
			assert.Equal(t, wantOwners, copyTokenByOwner(mgr.currentCache()))
			assertNoReservedTokens(t, mgr)
			assertCacheConsistent(t, mgr.currentCache())
		})
	}
}

// Tests the retry loop: after SlotTaken, grant marks the token unavailable and writes another, until
// a write applies, it has tried maxGrantAttempts tokens, or the caller gives up.
func TestGrant_RetriesAfterSlotTaken(t *testing.T) {
	taken := persistence.SemaphoreGrantSlotTaken
	applied := persistence.SemaphoreGrantApplied
	tests := []struct {
		name       string
		tokenCount int
		// outcomes are the answers to each write, in order.
		outcomes []persistence.SemaphoreGrantOutcome
		// cancelOnWrite cancels the caller's context during the first write.
		cancelOnWrite   bool
		wantOutcome     types.SemaphoreAcquireOutcome
		wantErr         error
		wantFree        int
		wantUnavailable int
	}{
		{
			name:            "SlotTaken, then the next token applies",
			tokenCount:      3,
			outcomes:        []persistence.SemaphoreGrantOutcome{taken, applied},
			wantOutcome:     types.SemaphoreAcquireOutcomeAcquired,
			wantFree:        1,
			wantUnavailable: 1,
		},
		{
			name:            "SlotTaken on every attempt",
			tokenCount:      10,
			outcomes:        slices.Repeat([]persistence.SemaphoreGrantOutcome{taken}, maxGrantAttempts),
			wantOutcome:     types.SemaphoreAcquireOutcomeNoSlot,
			wantFree:        10 - maxGrantAttempts,
			wantUnavailable: maxGrantAttempts,
		},
		{
			name:            "the caller gives up after a SlotTaken",
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

			got, err := mgr.grant(ctx, "owner-a")
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantOutcome, got.Outcome)
			assert.Len(t, slices.Compact(slices.Sorted(slices.Values(tried))), len(tried), "each attempt writes a different token")
			if got.Outcome == types.SemaphoreAcquireOutcomeAcquired {
				assert.Equal(t, tried[len(tried)-1], got.TokenID)
			}
			assert.Equal(t, tc.wantFree, freeTokenCount(mgr))
			assert.Len(t, tokensInState(mgr.currentCache(), tokenUnavailable), tc.wantUnavailable)
			assertNoReservedTokens(t, mgr)
		})
	}
}

// Tests that when the cache records a token for the owner, grant reads its token row and acts on
// each result. The bucket has one token, held by owner-a, so a token marked unavailable means NoSlot.
func TestGrant_OwnerHasACachedToken(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	acquiredToken1 := AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1}
	noSlot := AcquireResult{Outcome: types.SemaphoreAcquireOutcomeNoSlot}
	tests := []struct {
		name    string
		row     *persistence.SemaphoreOwnership
		readErr error
		// wantWrite expects one write for token 1, which applies.
		wantWrite       bool
		want            AcquireResult
		wantOwners      map[string]int
		wantUnavailable []int
		// assertErr checks the error. Nil expects no error.
		assertErr func(t *testing.T, err error)
	}{
		{
			name:       "row names owner-a",
			row:        tokenRow(1, "owner-a"),
			want:       acquiredToken1,
			wantOwners: map[string]int{"owner-a": 1},
		},
		{
			name:       "row is FREE",
			row:        tokenRow(1, ""),
			wantWrite:  true,
			want:       acquiredToken1,
			wantOwners: map[string]int{"owner-a": 1},
		},
		{
			// The owner row may still name the token, so it is not freed.
			name:            "no row",
			readErr:         &types.EntityNotExistsError{Message: "not found"},
			want:            noSlot,
			wantOwners:      map[string]int{},
			wantUnavailable: []int{1},
		},
		{
			name:            "row names owner-b",
			row:             tokenRow(1, "owner-b"),
			want:            noSlot,
			wantOwners:      map[string]int{},
			wantUnavailable: []int{1},
		},
		{
			name:       "read returns no row and no error",
			wantOwners: map[string]int{"owner-a": 1},
			assertErr:  func(t *testing.T, err error) { assert.IsType(t, &types.InternalServiceError{}, err) },
		},
		{
			name:       "read fails",
			readErr:    readErr,
			wantOwners: map[string]int{"owner-a": 1},
			assertErr:  func(t *testing.T, err error) { assert.ErrorIs(t, err, readErr) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, 1, []*persistence.SemaphoreOwnership{tokenRow(1, "owner-a"), ownerRow("owner-a", 1)})

			m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), &persistence.GetSemaphoreOwnershipByTokenRequest{
				DomainID:      testBucketID.DomainID,
				SemaphoreName: testBucketID.SemaphoreName,
				Bucket:        testBucketID.Bucket,
				TokenID:       1,
			}).Times(1).Return(&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tc.row}, tc.readErr)
			// With no write expected, the mock fails the test if grant writes anyway.
			if tc.wantWrite {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), &persistence.GrantSemaphoreTokenRequest{
					DomainID:      testBucketID.DomainID,
					SemaphoreName: testBucketID.SemaphoreName,
					Bucket:        testBucketID.Bucket,
					TokenID:       1,
					OwnerID:       "owner-a",
				}).Times(1).Return(&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
			}

			got, err := mgr.grant(context.Background(), "owner-a")
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantOwners, copyTokenByOwner(mgr.currentCache()))
			assert.Equal(t, tc.wantUnavailable, tokensInState(mgr.currentCache(), tokenUnavailable))
			assertCacheConsistent(t, mgr.currentCache())
		})
	}
}

// Tests grant in bucket 1 of a semaphore of size 4 in buckets of 2, which owns tokens 3 and 4.
func TestGrant_InABucketNotStartingAtToken1(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	id := testBucketID
	id.Bucket = 1

	md := persistence.NewMockSemaphoreMetadataManager(ctrl)
	md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Times(1).Return(&persistence.GetSemaphoreResponse{
		Semaphore: &persistence.SemaphoreMetadata{DomainID: id.DomainID, SemaphoreName: id.SemaphoreName, Size: 4, BucketSize: 2},
	}, nil)
	// owner-b holds token 4, so token 3 is the only free token.
	heldRow, heldOwnerRow := tokenRow(4, "owner-b"), ownerRow("owner-b", 4)
	heldRow.Bucket, heldOwnerRow.Bucket = id.Bucket, id.Bucket
	m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.ScanSemaphoreBucketResponse{Ownerships: []*persistence.SemaphoreOwnership{heldRow, heldOwnerRow}}, nil)

	p := testManagerParams(t, m, 2)
	p.ID, p.Metadata = id, md
	mgr := newManagerFromParams(t, p)
	require.NoError(t, mgr.Start(context.Background()))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), &persistence.GrantSemaphoreTokenRequest{
		DomainID: id.DomainID, SemaphoreName: id.SemaphoreName, Bucket: 1, TokenID: 3, OwnerID: "owner-a",
	}).Times(1).Return(&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 3}, got)

	m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID: id.DomainID, SemaphoreName: id.SemaphoreName, Bucket: 1, TokenID: 4,
	}).Times(1).Return(&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: heldRow}, nil)
	got, err = mgr.grant(context.Background(), "owner-b")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 4}, got)

	assert.Equal(t, map[string]int{"owner-a": 3, "owner-b": 4}, copyTokenByOwner(mgr.currentCache()))
	assert.Equal(t, 0, freeTokenCount(mgr))
	assertCacheConsistent(t, mgr.currentCache())
}

// Tests that grant refuses a token outside the bucket's range without writing it.
func TestGrant_RefusesATokenOutsideTheBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	// No write is expected: the mock fails the test if GrantSemaphoreToken is called.
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 3, nil)

	// No cache method can make token 4 the only free token, so the cache is changed directly.
	c := mgr.currentCache()
	c.mu.Lock()
	for i := range c.tokens {
		c.tokens[i].state = tokenUnavailable
	}
	c.tokens = append(c.tokens, tokenEntry{state: tokenFree})
	c.freeCount = 1
	c.mu.Unlock()

	got, err := mgr.grant(context.Background(), "owner-a")
	require.IsType(t, &types.InternalServiceError{}, err)
	assert.ErrorContains(t, err, "token 4 is outside the range")
	assert.Equal(t, AcquireResult{}, got)
}

// Tests that grants running at the same time each get a different token.
func TestGrant_Concurrently(t *testing.T) {
	const owners = 20

	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	mgr := startManager(t, m, owners, nil)

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(owners).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

	var wg sync.WaitGroup
	results := make([]AcquireResult, owners)
	errs := make([]error, owners)
	for i := range owners {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = mgr.grant(context.Background(), fmt.Sprintf("owner-%d", i))
		}(i)
	}
	wg.Wait()

	seen := make(map[int]bool, owners)
	for i, res := range results {
		require.NoError(t, errs[i])
		require.Equal(t, types.SemaphoreAcquireOutcomeAcquired, res.Outcome)
		assert.False(t, seen[res.TokenID], "token %d handed out twice", res.TokenID)
		seen[res.TokenID] = true
	}
	assert.Equal(t, 0, freeTokenCount(mgr))
	assertNoReservedTokens(t, mgr)
}
