package semaphore

import (
	"context"
	"errors"
	"fmt"
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
// owns tokens 1 to size.
func singleBucketMetadata(t *testing.T, size int) persistence.SemaphoreMetadataManager {
	t.Helper()
	md := persistence.NewMockSemaphoreMetadataManager(gomock.NewController(t))
	// At most once: only the startup load reads it, and some tests never start the manager.
	md.EXPECT().GetSemaphore(gomock.Any(), &persistence.GetSemaphoreRequest{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
	}).MaxTimes(1).Return(&persistence.GetSemaphoreResponse{Semaphore: &persistence.SemaphoreMetadata{
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Size:          size,
		BucketSize:    size,
	}}, nil)
	return md
}

// newTestManager builds an unstarted manager for a bucket of tokens 1 to size, on a clock that
// only moves when a test moves it, so nothing is evicted unless the test asks for it.
func newTestManager(t *testing.T, m persistence.SemaphoreTokenManager, size int) *semaphoreManagerImpl {
	t.Helper()
	mgr, _, _ := newTestManagerWithRegistry(t, m, singleBucketMetadata(t, size))
	return mgr
}

func newTestManagerWithRegistry(
	t *testing.T,
	m persistence.SemaphoreTokenManager,
	md persistence.SemaphoreMetadataManager,
) (*semaphoreManagerImpl, SemaphoreRegistry, clock.MockedTimeSource) {
	t.Helper()
	registry := NewSemaphoreRegistry()
	mockClock := clock.NewMockedTimeSource()
	mgr, err := NewManager(ManagerParams{
		ID:         testBucketID,
		Tokens:     m,
		Metadata:   md,
		Logger:     testlogger.New(t),
		IdleTTL:    testIdleTTL,
		OnStopFn:   func(m Manager) { registry.Unregister(m) },
		TimeSource: mockClock,
	})
	require.NoError(t, err)
	// Every manager runs an idle clock, so stop it rather than leak the goroutine.
	t.Cleanup(mgr.Stop)
	return mgr.(*semaphoreManagerImpl), registry, mockClock
}

// startManager returns a started manager for a bucket of tokens 1 to size, whose startup scan
// read the given single page of rows. With no rows, every token is free.
func startManager(t *testing.T, m *persistence.MockSemaphoreTokenManager, size int, rows []*persistence.SemaphoreOwnership) *semaphoreManagerImpl {
	t.Helper()
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{rows})
	mgr := newTestManager(t, m, size)
	require.NoError(t, mgr.Start(context.Background()))
	return mgr
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
	ctrl := gomock.NewController(t)
	tokens := persistence.NewMockSemaphoreTokenManager(ctrl)
	logger := testlogger.New(t)

	full := ManagerParams{
		ID:         testBucketID,
		Tokens:     tokens,
		Metadata:   persistence.NewMockSemaphoreMetadataManager(ctrl),
		Logger:     logger,
		IdleTTL:    testIdleTTL,
		OnStopFn:   func(Manager) {},
		TimeSource: clock.NewMockedTimeSource(),
	}
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

// Tests that Start() builds both the free tokens and tokenByOwner from a scan that arrives in
// several pages, following the page token to the end. The free tokens are the bucket's tokens minus
// the held ones, so a token with no row counts as free the same as one released back to free.
func TestStartBuildsTheFreeTokensAndTheOwnerIndexAcrossPages(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	// Tokens 1 and 5 have never been granted, so they have no row. Token 3 was granted and
	// released.
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

// Tests that a load fails, without scanning the partition, when the metadata does not say which
// tokens the bucket owns. A manager loaded without a range would hand out no tokens, or wrong ones.
func TestStartFailsWithoutUsableMetadata(t *testing.T) {
	semaphoreOf := func(size, bucketSize int) *persistence.GetSemaphoreResponse {
		return &persistence.GetSemaphoreResponse{Semaphore: &persistence.SemaphoreMetadata{
			DomainID:      testBucketID.DomainID,
			SemaphoreName: testBucketID.SemaphoreName,
			Size:          size,
			BucketSize:    bucketSize,
		}}
	}

	tests := []struct {
		name      string
		bucket    int
		resp      *persistence.GetSemaphoreResponse
		readErr   error
		assertErr func(t *testing.T, err error)
	}{
		{
			name:      "metadata read fails",
			readErr:   assert.AnError,
			assertErr: func(t *testing.T, err error) { assert.ErrorIs(t, err, assert.AnError) },
		},
		{
			name:    "semaphore does not exist",
			readErr: &types.EntityNotExistsError{Message: "not found"},
			assertErr: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*types.EntityNotExistsError))
			},
		},
		{
			name: "read returns no semaphore",
			resp: &persistence.GetSemaphoreResponse{},
			assertErr: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*types.InternalServiceError))
			},
		},
		{
			// Size 4 in buckets of 2 has buckets 0 and 1.
			name:   "bucket is past the last one",
			bucket: 2,
			resp:   semaphoreOf(4, 2),
			assertErr: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*types.BadRequestError))
			},
		},
		{
			// Not the caller's fault: CreateSemaphore rejects a size below 1, so the row is
			// corrupt.
			name: "metadata has no size",
			resp: semaphoreOf(0, 2),
			assertErr: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*types.InternalServiceError))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// No scan is expected: the load stops before it reads the partition.
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			md := persistence.NewMockSemaphoreMetadataManager(ctrl)
			md.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(tc.resp, tc.readErr)

			id := testBucketID
			id.Bucket = tc.bucket
			mgr, err := NewManager(ManagerParams{
				ID:         id,
				Tokens:     m,
				Metadata:   md,
				Logger:     testlogger.New(t),
				IdleTTL:    testIdleTTL,
				OnStopFn:   func(Manager) {},
				TimeSource: clock.NewMockedTimeSource(),
			})
			require.NoError(t, err)
			t.Cleanup(mgr.Stop)

			tc.assertErr(t, mgr.Start(context.Background()))
			_, err = mgr.Acquire(context.Background(), "owner-a")
			assert.ErrorIs(t, err, ErrNotReady, "a failed load leaves the manager stopped")
		})
	}
}

// Tests a scan failing partway leaves the manager's state as it was, with nothing
// half-installed.
func TestStartLeavesStateUntouchedWhenTheScanFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	// The first page succeeds and the second fails, so anything the first page built has
	// to be thrown away rather than installed.
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
			return nil, errors.New("scan failed")
		})

	mgr := newTestManager(t, m, 2)
	require.Error(t, mgr.Start(context.Background()))

	assert.Equal(t, 0, freeTokenCount(mgr))
	assert.Empty(t, copyTokenByOwner(mgr.currentCache()))
}

func TestSecondStartIsANoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	// Exactly one scan: the second Start must return before it reaches persistence.
	mgr := startManager(t, m, 2, nil)

	assert.NoError(t, mgr.Start(context.Background()))
	assert.Equal(t, 2, freeTokenCount(mgr), "the first load's free tokens survive")
}

// Tests that a manager whose load failed takes itself out of the registry, so the next request
// builds a fresh one instead of finding a manager that can never serve.
func TestAFailedLoadUnregistersTheManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Return(nil, errors.New("scan failed"))

	mgr, registry, _ := newTestManagerWithRegistry(t, m, singleBucketMetadata(t, 1))
	registerForTest(t, registry, mgr)

	require.Error(t, mgr.Start(context.Background()))
	assert.False(t, isRegistered(registry, mgr))
}

func TestAcquireRejectsAnEmptyOwnerID(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 1, nil)

	got, err := mgr.Acquire(context.Background(), "")
	assert.IsType(t, &types.BadRequestError{}, err)
	assert.Equal(t, AcquireResult{}, got)
	assert.Equal(t, 1, freeTokenCount(mgr), "a rejected request must not touch the free tokens")
}

// Tests that Acquire on a manager that is not running returns ErrNotReady and no result,
// whichever way the manager became unusable.
func TestAcquireBeforeTheBucketIsUsable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl
	}{
		{
			// Stop must release the startup wait, or callers block forever.
			name: "stopped before it was started",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				mgr := newTestManager(t, m, 1)
				mgr.Stop()
				return mgr
			},
		},
		{
			name: "the startup load failed",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Return(nil, errors.New("scan failed"))
				mgr := newTestManager(t, m, 1)
				assert.Error(t, mgr.Start(context.Background()))
				return mgr
			},
		},
		{
			// Losing the bucket mid-load has to stick. A scan that finishes afterwards
			// describes a bucket this host no longer serves, and going live on it would hand
			// out tokens the real owner has already given away.
			name: "stopped while it was still loading",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				scanning, finishScan := make(chan struct{}), make(chan struct{})
				m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
						close(scanning)
						<-finishScan
						return &persistence.ScanSemaphoreBucketResponse{}, nil
					})

				mgr := newTestManager(t, m, 1)
				started := make(chan error, 1)
				go func() { started <- mgr.Start(context.Background()) }()

				<-scanning // pin Stop to the window where the scan is in flight
				mgr.Stop()
				close(finishScan)
				assert.ErrorIs(t, <-started, ErrNotReady, "Start must report that it lost the bucket")
				return mgr
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := tc.setup(t, m)

			// Bounded, so a manager that leaves callers blocked fails the test instead of hanging
			// it.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := mgr.Acquire(ctx, "owner-a")
			assert.ErrorIs(t, err, ErrNotReady)
			assert.Equal(t, AcquireResult{}, got, "an error carries no result")
			// Retryable, because the registry drops this manager and a retry builds a fresh one
			// that may load.
			assert.True(t, common.IsServiceTransientError(err), "a retry can get a different answer")
		})
	}
}

// Tests that an acquire waiting on a slow startup load returns at its own deadline
func TestAcquireHonorsItsDeadlineWhileStarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	scanning, finishScan := make(chan struct{}), make(chan struct{})
	m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
			close(scanning)
			<-finishScan
			return &persistence.ScanSemaphoreBucketResponse{}, nil
		})

	mgr := newTestManager(t, m, 1)
	started := make(chan error, 1)
	go func() { started <- mgr.Start(context.Background()) }()
	<-scanning
	defer func() {
		close(finishScan)
		assert.NoError(t, <-started)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := mgr.Acquire(ctx, "owner-a")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// Tests that a Start arriving while another caller's load is in flight returns at once instead of
// waiting for that load. Its context carries the scan's deadline, not the caller's, so waiting on
// it would hold the caller long after it gave up; Acquire does the waiting instead.
func TestStartDoesNotWaitOnALoadInFlight(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	scanning, finishScan := make(chan struct{}), make(chan struct{})
	m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
			close(scanning)
			<-finishScan
			return &persistence.ScanSemaphoreBucketResponse{}, nil
		})

	mgr := newTestManager(t, m, 1)
	started := make(chan error, 1)
	go func() { started <- mgr.Start(context.Background()) }()
	<-scanning
	defer func() {
		close(finishScan)
		assert.NoError(t, <-started)
	}()

	// If Start waited on the load, this would hang until the deadline below and fail.
	secondStart := make(chan error, 1)
	go func() { secondStart <- mgr.Start(context.Background()) }()
	select {
	case err := <-secondStart:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("a second Start waited on the load already in flight")
	}
}

// Tests that Acquire passes grant's answer through unchanged, since Acquire is the seam callers
// use and must not edit the result on the way out.
func TestAcquireHandsBackWhatTheGrantDecided(t *testing.T) {
	tests := []struct {
		name  string
		size  int
		rows  []*persistence.SemaphoreOwnership
		setup func(m *persistence.MockSemaphoreTokenManager)
		want  AcquireResult
	}{
		{
			name: "an owner that already holds one gets the same token back",
			size: 5,
			rows: []*persistence.SemaphoreOwnership{tokenRow(5, "owner-a"), ownerRow("owner-a", 5)},
			setup: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).Return(
					&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tokenRow(5, "owner-a")}, nil)
			},
			want: AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 5},
		},
		{
			// Nothing free to reserve and no write to contradict the free tokens.
			name: "a full bucket answers no-slot",
			size: 1,
			rows: []*persistence.SemaphoreOwnership{tokenRow(1, "owner-x"), ownerRow("owner-x", 1)},
			want: AcquireResult{Outcome: types.SemaphoreAcquireOutcomeNoSlot},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, tc.size, tc.rows)
			if tc.setup != nil {
				tc.setup(m)
			}

			got, err := mgr.Acquire(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Tests an error from grant reaches the caller, rather than being flattened into a
// no-token answer.
func TestAcquireSurfacesAFailedGrant(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 1, nil)

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	got, err := mgr.Acquire(context.Background(), "owner-a")
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, AcquireResult{}, got)
	assert.Equal(t, 1, freeTokenCount(mgr), "a failed write must not cost the token")
}

func TestGrantOnAFreeToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 1, nil)

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			assert.Equal(t, testBucketID.DomainID, req.DomainID)
			assert.Equal(t, testBucketID.SemaphoreName, req.SemaphoreName)
			assert.Equal(t, testBucketID.Bucket, req.Bucket)
			assert.Equal(t, 1, req.TokenID)
			assert.Equal(t, "owner-a", req.OwnerID)
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
		})

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1}, got)
	assert.Equal(t, 0, freeTokenCount(mgr), "the granted token must leave the free tokens")
	// Without recording owner-a's token, the next acquire by owner-a would reserve a second token.
	assert.Equal(t, map[string]int{"owner-a": 1}, copyTokenByOwner(mgr.currentCache()))
}

// Tests that a write refused as SlotTaken sends grant to a different token, and that the refused
// id stays out of the free tokens.
func TestGrantRetriesADifferentTokenWhenTheWriteSaysTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 3, nil)

	var tried []int
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			tried = append(tried, req.TokenID)
			if len(tried) == 1 {
				return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil
			}
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
		})

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, types.SemaphoreAcquireOutcomeAcquired, got.Outcome)
	require.Len(t, tried, 2)
	assert.NotEqual(t, tried[0], tried[1], "a retry must reserve a different token")
	assert.Equal(t, tried[1], got.TokenID)
	// Three free, one proved taken, one granted.
	assert.Equal(t, 1, freeTokenCount(mgr))
	assert.Equal(t, []int{tried[0]}, tokensInState(mgr.currentCache(), tokenUnavailable), "the refused token is marked unavailable")
	assertNoReservedTokens(t, mgr)
}

func TestGrantGivesUpAfterMaxAttempts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 10, nil)

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(maxGrantAttempts).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil)

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, types.SemaphoreAcquireOutcomeNoSlot, got.Outcome)
	assert.Zero(t, got.TokenID)
	assert.Equal(t, 10-maxGrantAttempts, freeTokenCount(mgr), "every token proved taken stays out")
	assertNoReservedTokens(t, mgr)
}

func TestGrantStopsRetryingOnceTheDeadlineHasPassed(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 5, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Exactly one attempt: the write reports the token taken and cancels the context as it
	// returns, so the second pass through the loop stops instead of writing again.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(context.Context, *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			cancel()
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil
		})

	_, err := mgr.grant(ctx, "owner-a")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 4, freeTokenCount(mgr), "the one token proved taken stays out")
	assertNoReservedTokens(t, mgr)
}

// Tests the case where the DB write finds the owner already holds a token: grant returns the token
// the write named and puts back the id it reserved.
func TestGrantWhenTheWriteSaysTheOwnerAlreadyHolds(t *testing.T) {
	tests := []struct {
		name          string
		rows          []*persistence.SemaphoreOwnership
		heldToken     int
		wantFreeCount int
	}{
		{
			// Token 3 is unavailable: its token row has a holder, but no owner row was read. The
			// reserved token going back is the only change to the free tokens.
			name:          "held token is not a free token",
			rows:          []*persistence.SemaphoreOwnership{tokenRow(3, "owner-a")},
			heldToken:     3,
			wantFreeCount: 2,
		},
		{
			// tokenByOwner was cold and the free tokens wrongly listed the held token. Recording
			// the owner's token has to take it out, or the next acquire would reserve a token this
			// owner holds.
			name:          "held token was wrongly listed as free",
			heldToken:     2,
			wantFreeCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, 3, tc.rows)

			// Exactly one attempt: retrying an already-held miss would loop forever.
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
				&persistence.GrantSemaphoreTokenResponse{
					Outcome:   persistence.SemaphoreGrantAlreadyHeld,
					HeldToken: tc.heldToken,
				}, nil)

			got, err := mgr.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: tc.heldToken}, got)
			assert.Equal(t, tc.wantFreeCount, freeTokenCount(mgr))
			assertCacheConsistent(t, mgr.currentCache())
			assert.Equal(t, map[string]int{"owner-a": tc.heldToken}, copyTokenByOwner(mgr.currentCache()))
			assertNoReservedTokens(t, mgr)
		})
	}
}

// Tests that grant rejects an AlreadyHeld write that names no token, or a token this bucket does
// not own, rather than recording it. Recording it would leave the owner failing every later acquire
// on a token this bucket cannot confirm or hand out.
func TestGrantRejectsAnAlreadyHeldWriteWithAnInvalidToken(t *testing.T) {
	tests := []struct {
		name      string
		heldToken int
	}{
		{name: "no token", heldToken: 0},
		{name: "token outside the bucket", heldToken: 9},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, 3, nil)

			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
				&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: tc.heldToken}, nil)

			_, err := mgr.grant(context.Background(), "owner-a")
			require.IsType(t, &types.InternalServiceError{}, err, "a caller must be able to tell this apart from contention")
			require.ErrorContains(t, err, "AlreadyHeld with an invalid token")

			// The write did not apply, so the reserved token goes back.
			assert.Equal(t, 3, freeTokenCount(mgr))
			assert.Empty(t, copyTokenByOwner(mgr.currentCache()), "an invalid token must not be cached")
			assertCacheConsistent(t, mgr.currentCache())
			assertNoReservedTokens(t, mgr)

			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
				&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

			got, err := mgr.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, types.SemaphoreAcquireOutcomeAcquired, got.Outcome)
		})
	}
}

// Tests that grant refuses, without writing, a free token outside the bucket's range. The store
// would create its row, letting one more owner hold a token than the limit allows.
func TestGrantRefusesATokenOutsideTheBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	// No write is expected: the mock fails the test if GrantSemaphoreToken is called.
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 3, nil)

	// Plant a bug: tokens 1 to 3 are unavailable, and the table gets a fourth entry, for token 4,
	// outside tokens 1 to 3, which is the only free one. No cache method can do this, so the table
	// and its free count are changed directly.
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

// Tests that a failed write costs no token: the id goes back every time, so an outage cannot
// drain the free tokens one write at a time.
func TestGrantReturnsTheTokenWhenTheWriteFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	const size = 5
	const acquires = 3
	mgr := startManager(t, m, size, nil)

	writeErr := &persistence.TimeoutError{Msg: "write timed out"}
	// One write per acquire: grant puts the id back and returns the error without trying again.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(acquires).Return(nil, writeErr)

	for i := range acquires {
		_, err := mgr.grant(context.Background(), fmt.Sprintf("owner-%d", i))
		require.ErrorIs(t, err, writeErr)
		require.Equal(t, size, freeTokenCount(mgr), "the token must come back after acquire %d", i)
	}
	assertCacheConsistent(t, mgr.currentCache())
	assertNoReservedTokens(t, mgr)
}

// Tests that an outcome grant cannot read is reported as an error and its token returned
func TestGrantRejectsAnUnrecognizedWriteOutcome(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 1, nil)

	// The zero value, which is what a store that never set the field returns.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown}, nil)

	_, err := mgr.grant(context.Background(), "owner-a")
	require.IsType(t, &types.InternalServiceError{}, err, "a caller must be able to tell this apart from contention")
	require.ErrorContains(t, err, "unexpected grant outcome")
	assert.Equal(t, 1, freeTokenCount(mgr))
	assertNoReservedTokens(t, mgr)
}

// Tests that a cached owner token the token row no longer agrees with is fixed in the cache, and
// the grant then falls through to a normal reserve.
func TestGrantWhenTheCachedOwnerTokenIsStale(t *testing.T) {
	tests := []struct {
		name          string
		ownership     *persistence.SemaphoreOwnership
		readErr       error
		wantFreeCount int
	}{
		{
			// Released behind our back: the token is genuinely free again, so it goes back
			// into the free tokens before the normal pick.
			name:          "token is free again",
			ownership:     tokenRow(4, ""),
			wantFreeCount: 3,
		},
		{
			// Someone else holds it now. Dropping the cached owner token is right; adding the
			// token back would offer out a held token.
			name:          "another owner holds it now",
			ownership:     tokenRow(4, "owner-b"),
			wantFreeCount: 2,
		},
		{
			// An owner's token with no token row is inconsistent data. The owner row may still name
			// the token, so it stays out of the free tokens.
			name:          "token has no row",
			readErr:       &types.EntityNotExistsError{Message: "not found"},
			wantFreeCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			// Tokens 1 to 3 are free, and owner-a holds token 4.
			mgr := startManager(t, m, 4, []*persistence.SemaphoreOwnership{tokenRow(4, "owner-a"), ownerRow("owner-a", 4)})

			m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
				func(_ context.Context, req *persistence.GetSemaphoreOwnershipByTokenRequest) (*persistence.GetSemaphoreOwnershipByTokenResponse, error) {
					assert.Equal(t, 4, req.TokenID, "the read is for the token the cache says owner-a holds")
					if tc.readErr != nil {
						return nil, tc.readErr
					}
					return &persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tc.ownership}, nil
				})
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
				&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

			got, err := mgr.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, types.SemaphoreAcquireOutcomeAcquired, got.Outcome,
				"a stale cached owner token must fall through to a normal pick")
			assert.Equal(t, tc.wantFreeCount, freeTokenCount(mgr))
		})
	}
}

func TestGrantSurfacesAConfirmingReadFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 2, []*persistence.SemaphoreOwnership{tokenRow(2, "owner-a"), ownerRow("owner-a", 2)})

	readErr := errors.New("cassandra unavailable")
	m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).Times(1).Return(nil, readErr)
	_, err := mgr.grant(context.Background(), "owner-a")
	assert.ErrorIs(t, err, readErr)
	assert.Equal(t, 1, freeTokenCount(mgr))
}

// Tests that confirmOwnerHoldsToken confirms an owner's token the token row still names, and
// otherwise drops it from the cache, freeing the token only when no one else holds it.
func TestConfirmOwnerHoldsToken(t *testing.T) {
	tests := []struct {
		name                string
		ownership           *persistence.SemaphoreOwnership
		readErr             error
		wantConfirmed       bool
		wantErr             bool
		wantFree            []int
		wantOwnerTokensLeft map[string]int
	}{
		{
			name:                "row names the owner",
			ownership:           tokenRow(2, "owner-a"),
			wantConfirmed:       true,
			wantFree:            []int{1},
			wantOwnerTokensLeft: map[string]int{"owner-a": 2},
		},
		{
			name:                "row is FREE",
			ownership:           tokenRow(2, ""),
			wantFree:            []int{1, 2},
			wantOwnerTokensLeft: map[string]int{},
		},
		{
			// An owner's token with no token row is inconsistent data. The owner row may still name
			// the token, so it stays out of the free tokens.
			name:                "no row",
			readErr:             &types.EntityNotExistsError{Message: "not found"},
			wantFree:            []int{1},
			wantOwnerTokensLeft: map[string]int{},
		},
		{
			name:                "row names another owner",
			ownership:           tokenRow(2, "owner-b"),
			wantFree:            []int{1},
			wantOwnerTokensLeft: map[string]int{},
		},
		{
			// The store promises a row or EntityNotExistsError, so an empty response is a store
			// bug.
			name:                "read returns no ownership",
			wantErr:             true,
			wantFree:            []int{1},
			wantOwnerTokensLeft: map[string]int{"owner-a": 2},
		},
		{
			name:                "read fails",
			readErr:             assert.AnError,
			wantErr:             true,
			wantFree:            []int{1},
			wantOwnerTokensLeft: map[string]int{"owner-a": 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, 2, []*persistence.SemaphoreOwnership{tokenRow(2, "owner-a"), ownerRow("owner-a", 2)})

			m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), &persistence.GetSemaphoreOwnershipByTokenRequest{
				DomainID:      testBucketID.DomainID,
				SemaphoreName: testBucketID.SemaphoreName,
				Bucket:        testBucketID.Bucket,
				TokenID:       2,
			}).Return(&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tc.ownership}, tc.readErr)

			confirmed, err := mgr.confirmOwnerHoldsToken(context.Background(), mgr.currentCache(), "owner-a", 2)
			switch {
			case tc.readErr != nil && tc.wantErr:
				assert.ErrorIs(t, err, tc.readErr)
			case tc.wantErr:
				assert.ErrorAs(t, err, new(*types.InternalServiceError))
			default:
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantConfirmed, confirmed)
			assert.Equal(t, tc.wantFree, tokensInState(mgr.currentCache(), tokenFree))
			assert.Equal(t, tc.wantOwnerTokensLeft, copyTokenByOwner(mgr.currentCache()))
		})
	}
}

// Tests that grant reads no token row when the cache records no token for the owner, and grants a
// free token instead.
func TestGrantWithoutACachedOwnerTokenReadsNoRow(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, 1, nil)

	// No GetSemaphoreOwnershipByToken is expected, so gomock fails the test if one is made.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: 1}, got)
}

func TestConcurrentGrantsHandOutDistinctTokens(t *testing.T) {
	const owners = 20

	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	mgr := startManager(t, m, owners, nil)

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(owners).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

	var wg sync.WaitGroup
	results := make([]AcquireResult, owners)
	errs := make([]error, owners)
	for i := 0; i < owners; i++ {
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

// Tests the manager's locking holds while its lifecycle races its work: Start installing
// the scan result, Stop writing the state, and a burst of grants, all at once.
func TestLifecycleUnderConcurrentGrants(t *testing.T) {
	// Each round is a fresh manager. The moment where Start's install meets a grant is easy
	// to miss, so the test repeats: a single round misses an unlocked install more often
	// than it finds one, while ten rounds have found it every time.
	const rounds = 10
	// One grant per token, so every grant can succeed and no free tokens are left.
	const grants = 8

	for range rounds {
		ctrl := gomock.NewController(t)
		m := persistence.NewMockSemaphoreTokenManager(ctrl)
		// Which calls happen at all depends on who wins the race -- Stop can land before the
		// scan, and a grant can find no free tokens yet -- so every call is optional. Nothing
		// here is asserted: the outcomes are not what this test is about.
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

		// Both errors are discarded on purpose: Start fails with "stopped while loading" when
		// Stop wins, and grant answers NoSlot when the free tokens are not installed yet.
		race(func() { _ = mgr.Start(context.Background()) })
		race(func() { mgr.Stop() })
		for g := range grants {
			race(func() { _, _ = mgr.grant(context.Background(), fmt.Sprintf("owner-%d", g)) })
		}
		wg.Wait()

		assertCacheConsistent(t, mgr.currentCache())
	}
}

// startIdleManager returns a started manager already registered the way the engine registers it.
func startIdleManager(
	t *testing.T,
	m *persistence.MockSemaphoreTokenManager,
	size int,
	rows []*persistence.SemaphoreOwnership,
) (*semaphoreManagerImpl, SemaphoreRegistry, clock.MockedTimeSource) {
	t.Helper()
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{rows})
	mgr, registry, mockClock := newTestManagerWithRegistry(t, m, singleBucketMetadata(t, size))
	registerForTest(t, registry, mgr)
	require.NoError(t, mgr.Start(context.Background()))
	return mgr, registry, mockClock
}

func isRegistered(registry SemaphoreRegistry, mgr Manager) bool {
	current, ok := registry.ManagerByIdentifier(testBucketID)
	return ok && current == mgr
}

// Tests the whole eviction path: once the bucket has gone its TTL without a request it stops
// itself and leaves the registry, so the next caller builds a fresh manager rather than being
// handed a stopped one.
func TestAnIdleManagerUnloadsItselfAndLeavesTheRegistry(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr, registry, mockClock := startIdleManager(t, m, 1, nil)

	require.True(t, isRegistered(registry, mgr), "a started bucket is reachable")

	mockClock.Advance(testIdleTTL)
	require.Eventually(t, func() bool {
		return !isRegistered(registry, mgr)
	}, time.Second, 5*time.Millisecond, "an idle bucket unloads itself")

	assert.False(t, mgr.isRunning(), "the unloaded manager is stopped, not merely unregistered")
}

// Tests that serving a request resets the idle clock, so a bucket under steady traffic is never
// unloaded out from under it.
func TestAcquireKeepsAManagerLoaded(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	// A full bucket answers NoSlot without any write, which is enough to count as a request.
	mgr, registry, mockClock := startIdleManager(t, m, 1,
		[]*persistence.SemaphoreOwnership{tokenRow(1, "owner-x"), ownerRow("owner-x", 1)})
	t.Cleanup(mgr.Stop)

	mockClock.Advance(testIdleTTL / 2)
	got, err := mgr.Acquire(context.Background(), "owner-a")
	require.NoError(t, err)
	require.Equal(t, types.SemaphoreAcquireOutcomeNoSlot, got.Outcome)

	// Past the original deadline, but within the TTL measured from the request.
	mockClock.Advance(testIdleTTL / 2)
	require.Never(t, func() bool {
		return !isRegistered(registry, mgr)
	}, 50*time.Millisecond, 5*time.Millisecond, "a bucket that just served a request stays loaded")

	mockClock.Advance(testIdleTTL)
	require.Eventually(t, func() bool {
		return !isRegistered(registry, mgr)
	}, time.Second, 5*time.Millisecond, "the idle clock still runs once traffic stops")
}

// Tests that a manager stopping late unregisters only itself, so a manager that has already
// replaced it stays registered. The registry tests that guard directly; this one pins the
// teardown path passing the manager being removed rather than only its identifier.
func TestALateStopUnregistersOnlyItself(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	old, registry, _ := startIdleManager(t, m, 1, nil)

	replacement, err := NewManager(ManagerParams{
		ID:         testBucketID,
		Tokens:     m,
		Metadata:   persistence.NewMockSemaphoreMetadataManager(ctrl),
		Logger:     testlogger.New(t),
		IdleTTL:    testIdleTTL,
		OnStopFn:   func(m Manager) { registry.Unregister(m) },
		TimeSource: clock.NewMockedTimeSource(),
	})
	require.NoError(t, err)
	t.Cleanup(replacement.Stop)
	// The window the ring-change path opens: unloadSemaphoreManager unregisters the old manager,
	// a request loads a replacement, and only then does the old manager's Stop run.
	require.True(t, registry.Unregister(old))
	registerForTest(t, registry, replacement)

	old.Stop()
	assert.True(t, isRegistered(registry, replacement), "the replacement is still the registered manager")
}
