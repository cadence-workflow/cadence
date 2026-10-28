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
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/locks"
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

// testManagerParams returns valid params for a bucket of tokens 1 to tokenCount. The clock is
// mocked, so the manager never goes idle unless a test moves it.
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

// assertTokensUnlocked checks that no token lock is left held.
func assertTokensUnlocked(t *testing.T, mgr *semaphoreManagerImpl) {
	t.Helper()
	cache := mgr.currentCache()
	for i, lock := range cache.tokenMutexes {
		if assert.True(t, lock.TryLock(), "token %d is still locked", cache.tokenRange.First+i) {
			lock.Unlock()
		}
	}
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
			{name: "no logger", params: without(func(p *ManagerParams) { p.Logger = nil })},
			{name: "no idle ttl", params: without(func(p *ManagerParams) { p.IdleTTL = 0 })},
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
				name: "metadata read returns an empty response",
				metadata: func(md *persistence.MockSemaphoreMetadataManager) {
					md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(&persistence.GetSemaphoreResponse{}, nil)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.InternalServiceError)) },
			},
			{
				name:   "bucket 2 does not exist (size 4 in buckets of 2)",
				bucket: 2,
				metadata: func(md *persistence.MockSemaphoreMetadataManager) {
					md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(semaphoreOf(4, 2), nil)
				},
				assertErr: func(t *testing.T, err error) { assert.ErrorAs(t, err, new(*types.BadRequestError)) },
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
			{
				name: "scan returns an empty response",
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
				var stopped []Manager
				p.OnStopFn = func(s Manager) { stopped = append(stopped, s) }
				mgr := newManagerFromParams(t, p)

				tc.assertErr(t, mgr.Start(context.Background()))
				assert.Equal(t, 0, freeTokenCount(mgr), "nothing is installed")
				assert.Empty(t, copyTokenByOwner(mgr.currentCache()))
				assert.Equal(t, managerStateStopped, getManagerState(mgr))
				// Start already stopped it, so this Stop does nothing.
				mgr.Stop()
				require.Len(t, stopped, 1)
				assert.Same(t, mgr, stopped[0], "OnStopFn gets the manager itself, so the registry removes only it")
			})
		}
	})

	// Tests that a second Start returns at once without scanning.
	t.Run("Start called again", func(t *testing.T) {
		tests := []struct {
			name    string
			setup   func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl
			wantErr error
		}{
			{
				// Not waited on: the load runs under the scan's deadline, not the caller's.
				name: "returns nil at once while the first Start is still scanning",
				setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
					mgr := newTestManager(t, m, 2)
					releaseScanAndWaitForStart := startWithBlockedScan(m, mgr)
					t.Cleanup(func() { assert.NoError(t, releaseScanAndWaitForStart()) })
					return mgr
				},
			},
			{
				name: "returns ErrNotReady once the manager is stopped",
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

	// Tests that the idle clock starts only once the load is done, is reset by Acquire, and stops the
	// manager after the idle TTL.
	t.Run("unloads after the idle TTL", func(t *testing.T) {
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
		_, err := mgr.Acquire(context.Background(), "owner-a")
		require.NoError(t, err)

		mockClock.Advance(testIdleTTL / 2)
		require.Never(t, isStopped, 50*time.Millisecond, 5*time.Millisecond, "stopped within the idle TTL of an Acquire")

		mockClock.Advance(testIdleTTL)
		require.Eventually(t, isStopped, time.Second, 5*time.Millisecond, "did not stop after the idle TTL")
	})
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
			name: "acquires a free token",
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

// Tests grant when the cache has a token for the owner: grant reads the token's row to confirm it.
func TestGrant_CachedToken(t *testing.T) {
	readErr := errors.New("cassandra unavailable")
	acquiredToken1 := AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1}
	noSlot := AcquireResult{Outcome: types.SemaphoreAcquireOutcomeNoSlot}
	// In every case the cache says owner-a holds token 1, the bucket's only token.
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
			name:       "row names owner-a: returns token 1 without a write",
			row:        tokenRow(1, "owner-a"),
			want:       acquiredToken1,
			wantOwners: map[string]int{"owner-a": 1},
		},
		{
			name:       "row is free: grants token 1 again",
			row:        tokenRow(1, ""),
			wantWrite:  true,
			want:       acquiredToken1,
			wantOwners: map[string]int{"owner-a": 1},
		},
		{
			// The owner row may still name the token, so it is not freed.
			name:            "row does not exist: token 1 becomes unavailable, NoSlot",
			readErr:         &types.EntityNotExistsError{Message: "not found"},
			want:            noSlot,
			wantOwners:      map[string]int{},
			wantUnavailable: []int{1},
		},
		{
			name:            "row names owner-b: token 1 becomes unavailable, NoSlot",
			row:             tokenRow(1, "owner-b"),
			want:            noSlot,
			wantOwners:      map[string]int{},
			wantUnavailable: []int{1},
		},
		{
			name:       "read returns an empty response: error, cache unchanged",
			wantOwners: map[string]int{"owner-a": 1},
			assertErr:  func(t *testing.T, err error) { assert.IsType(t, &types.InternalServiceError{}, err) },
		},
		{
			name:       "read fails: error, cache unchanged",
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

// Tests grant when the owner has no cached token: grant writes a free token.
func TestGrant_FreeToken(t *testing.T) {
	// Token 2's row names owner-a but has no owner row, so token 2 starts unavailable and token 1, the only
	// free token, is the one written.
	t.Run("one write: error or AlreadyHeld", func(t *testing.T) {
		writeErr := &persistence.TimeoutError{Msg: "write timed out"}
		tests := []struct {
			name     string
			resp     *persistence.GrantSemaphoreTokenResponse
			writeErr error
			// lockToken2 has another call hold token 2's lock during the grant.
			lockToken2      bool
			want            AcquireResult
			wantOwners      map[string]int
			wantUnavailable []int
			// assertErr checks the error. Nil expects no error.
			assertErr func(t *testing.T, err error)
		}{
			{
				name:            "write fails: error",
				writeErr:        writeErr,
				wantOwners:      map[string]int{},
				wantUnavailable: []int{2},
				assertErr:       func(t *testing.T, err error) { assert.ErrorIs(t, err, writeErr) },
			},
			{
				// The zero value, which is what a store that never set the field returns.
				name:            "unknown outcome: error",
				resp:            &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown},
				wantOwners:      map[string]int{},
				wantUnavailable: []int{2},
				assertErr: func(t *testing.T, err error) {
					assert.IsType(t, &types.InternalServiceError{}, err)
					assert.ErrorContains(t, err, "unexpected grant outcome")
				},
			},
			{
				name:       "AlreadyHeld token 2: returns and records token 2",
				resp:       &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 2},
				want:       AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 2},
				wantOwners: map[string]int{"owner-a": 2},
			},
			{
				name:            "AlreadyHeld token 2, locked by another call: returns token 2 unrecorded",
				resp:            &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 2},
				lockToken2:      true,
				want:            AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 2},
				wantOwners:      map[string]int{},
				wantUnavailable: []int{2},
			},
			{
				name:            "AlreadyHeld a token outside the bucket: error",
				resp:            &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 9},
				wantOwners:      map[string]int{},
				wantUnavailable: []int{2},
				assertErr: func(t *testing.T, err error) {
					assert.IsType(t, &types.InternalServiceError{}, err)
					assert.ErrorContains(t, err, "grant reported AlreadyHeld with token 9, outside tokens 1 to 2")
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
				mgr := startManager(t, m, 2, []*persistence.SemaphoreOwnership{tokenRow(2, "owner-a")})
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), &persistence.GrantSemaphoreTokenRequest{
					DomainID:      testBucketID.DomainID,
					SemaphoreName: testBucketID.SemaphoreName,
					Bucket:        testBucketID.Bucket,
					TokenID:       1,
					OwnerID:       "owner-a",
				}).Times(1).Return(tc.resp, tc.writeErr)
				var token2 *lockedToken
				if tc.lockToken2 {
					token2 = lockTokenForTest(t, mgr.currentCache(), 2)
				}

				got, err := mgr.grant(context.Background(), "owner-a")
				if token2 != nil {
					token2.unlock()
				}

				if tc.assertErr != nil {
					tc.assertErr(t, err)
				} else {
					require.NoError(t, err)
				}
				assert.Equal(t, tc.want, got)
				assert.Equal(t, tc.wantOwners, copyTokenByOwner(mgr.currentCache()))
				assert.Equal(t, []int{1}, tokensInState(mgr.currentCache(), tokenFree), "token 1 stays free")
				assert.Equal(t, tc.wantUnavailable, tokensInState(mgr.currentCache(), tokenUnavailable))
				assertTokensUnlocked(t, mgr)
				assertCacheConsistent(t, mgr.currentCache())
			})
		}
	})

	// Each SlotTaken marks its token unavailable and grant tries another free token, picked at random.
	t.Run("retries after SlotTaken", func(t *testing.T) {
		taken, applied := persistence.SemaphoreGrantSlotTaken, persistence.SemaphoreGrantApplied
		tests := []struct {
			name       string
			tokenCount int
			// answers is what each grant write answers, in order.
			answers []persistence.SemaphoreGrantOutcome
			// cancelOnWrite cancels the caller's context during the first write.
			cancelOnWrite bool
			wantOutcome   types.SemaphoreAcquireOutcome
			wantFree      int
			wantErr       error
		}{
			{
				name:        "the next write applies: returns that token",
				tokenCount:  3,
				answers:     []persistence.SemaphoreGrantOutcome{taken, applied},
				wantOutcome: types.SemaphoreAcquireOutcomeAcquired,
				wantFree:    1,
			},
			{
				// One more free token than the cap, so only the cap can stop the retries.
				name:        "SlotTaken on every attempt: NoSlot after maxGrantAttempts",
				tokenCount:  maxGrantAttempts + 1,
				answers:     slices.Repeat([]persistence.SemaphoreGrantOutcome{taken}, maxGrantAttempts),
				wantOutcome: types.SemaphoreAcquireOutcomeNoSlot,
				wantFree:    1,
			},
			{
				name:          "caller's context ends after a SlotTaken: error, no second write",
				tokenCount:    5,
				answers:       []persistence.SemaphoreGrantOutcome{taken},
				cancelOnWrite: true,
				wantFree:      4,
				wantErr:       context.Canceled,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
				mgr := startManager(t, m, tc.tokenCount, nil)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				var tried []int
				// One write per answer, so the SlotTaken-on-every-attempt case also checks the attempt cap.
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(len(tc.answers)).DoAndReturn(
					func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
						tried = append(tried, req.TokenID)
						if tc.cancelOnWrite {
							cancel()
						}
						return &persistence.GrantSemaphoreTokenResponse{Outcome: tc.answers[len(tried)-1]}, nil
					})

				got, err := mgr.grant(ctx, "owner-a")

				if tc.wantErr != nil {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					require.NoError(t, err)
				}
				require.Len(t, tried, len(tc.answers))
				var slotTaken []int
				for i, answer := range tc.answers {
					if answer == taken {
						slotTaken = append(slotTaken, tried[i])
					}
				}
				assert.ElementsMatch(t, slotTaken, tokensInState(mgr.currentCache(), tokenUnavailable), "each SlotTaken token becomes unavailable")
				want, wantOwners := AcquireResult{Outcome: tc.wantOutcome}, map[string]int{}
				if tc.answers[len(tc.answers)-1] == applied {
					want.TokenID = tried[len(tried)-1]
					wantOwners["owner-a"] = want.TokenID
				}
				assert.Equal(t, want, got)
				assert.Equal(t, wantOwners, copyTokenByOwner(mgr.currentCache()))
				assert.Equal(t, tc.wantFree, freeTokenCount(mgr))
				assertTokensUnlocked(t, mgr)
				assertCacheConsistent(t, mgr.currentCache())
			})
		}
	})

	// The range check before the write. No cache method can make a token outside the range free, so
	// the cache is changed directly.
	t.Run("picks a token outside the bucket: error, no write", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		// No write is expected: the mock fails the test if GrantSemaphoreToken is called.
		m := persistence.NewMockSemaphoreTokenManager(ctrl)
		mgr := startManager(t, m, 3, nil)

		c := mgr.currentCache()
		for i := range c.tokens {
			c.tokens[i].state = tokenUnavailable
		}
		c.tokens = append(c.tokens, tokenEntry{state: tokenFree})
		c.tokenMutexes = append(c.tokenMutexes, locks.NewMutex())
		c.freeCount = 1

		got, err := mgr.grant(context.Background(), "owner-a")
		require.IsType(t, &types.InternalServiceError{}, err)
		assert.ErrorContains(t, err, "token 4 is outside the range")
		assert.Equal(t, AcquireResult{}, got)
	})
}

// Tests calls running at the same time.
func TestConcurrency(t *testing.T) {
	// Tests that the cache stays consistent when Start, Stop and grants race.
	t.Run("Start, Stop and grants at once", func(t *testing.T) {
		// The race is easy to miss in one round; ten rounds have caught a cache installed without m.mu
		// every time.
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
	})

	// During its write, a grant holds only its token's lock: not the cache lock, not other tokens' locks.
	t.Run("while a grant's write is blocked, reads and grants on other tokens go ahead", func(t *testing.T) {
		m := persistence.NewMockSemaphoreTokenManager(gomock.NewController(t))
		mgr := startManager(t, m, 2, nil)
		// owner-a's write blocks until finishWrite closes; owner-b's applies at once.
		writing, finishWrite := make(chan struct{}), make(chan struct{})
		m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
			func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
				if req.OwnerID == "owner-a" {
					close(writing)
					<-finishWrite
				}
				return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
			})
		firstDone := make(chan AcquireResult, 1)
		go func() {
			res, err := mgr.grant(context.Background(), "owner-a")
			assert.NoError(t, err)
			firstDone <- res
		}()
		<-writing

		var second AcquireResult
		done := make(chan struct{})
		go func() {
			defer close(done)
			cache := mgr.currentCache()
			assert.Equal(t, 2, cache.freeTokenCount(), "the token being written is still free")
			assert.Equal(t, 0, cache.heldTokenCount())
			var err error
			second, err = mgr.grant(context.Background(), "owner-b")
			assert.NoError(t, err)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("a read or owner-b's grant waited for owner-a's write")
		}

		close(finishWrite)
		<-done
		first := <-firstDone
		assert.Equal(t, types.SemaphoreAcquireOutcomeAcquired, first.Outcome)
		assert.Equal(t, types.SemaphoreAcquireOutcomeAcquired, second.Outcome)
		assert.Equal(t, map[string]int{"owner-a": first.TokenID, "owner-b": second.TokenID}, copyTokenByOwner(mgr.currentCache()))
		assertCacheConsistent(t, mgr.currentCache())
	})
}
