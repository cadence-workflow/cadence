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

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var testBucketID = Identifier{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 2}

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

// newTestManager builds an unstarted manager.
func newTestManager(t *testing.T, m persistence.SemaphoreTokenManager) *semaphoreManagerImpl {
	t.Helper()
	mgr, err := NewManager(ManagerParams{ID: testBucketID, Tokens: m, Logger: testlogger.New(t)})
	require.NoError(t, err)
	return mgr.(*semaphoreManagerImpl)
}

// startManager returns a started manager whose startup scan read the given single page of rows.
func startManager(t *testing.T, m *persistence.MockSemaphoreTokenManager, rows []*persistence.SemaphoreOwnership) *semaphoreManagerImpl {
	t.Helper()
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{rows})
	mgr := newTestManager(t, m)
	require.NoError(t, mgr.Start(context.Background()))
	return mgr
}

func freeTokens(ids ...int) []*persistence.SemaphoreOwnership {
	rows := make([]*persistence.SemaphoreOwnership, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, tokenRow(id, ""))
	}
	return rows
}

// assertFreeSetIsConsistent checks that freeList and freeIndex still agree: freeIndex points at
// the position each id really sits at in freeList, and holds no ids beyond those.
func assertFreeSetIsConsistent(t *testing.T, mgr *semaphoreManagerImpl) {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	require.Len(t, mgr.freeIndex, len(mgr.freeList), "freeIndex and freeList must hold the same ids")
	for i, tokenID := range mgr.freeList {
		got, ok := mgr.freeIndex[tokenID]
		require.True(t, ok, "token %d is in freeList but not in freeIndex", tokenID)
		// A duplicate in freeList would fail here too: only one copy can own the index.
		assert.Equal(t, i, got, "freeIndex has token %d at %d, freeList has it at %d", tokenID, got, i)
	}
}

func TestNewManagerValidatesItsParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := persistence.NewMockSemaphoreTokenManager(ctrl)
	logger := testlogger.New(t)

	tests := []struct {
		name   string
		params ManagerParams
	}{
		{name: "no identifier", params: ManagerParams{Tokens: tokens, Logger: logger}},
		{name: "no token manager", params: ManagerParams{ID: testBucketID, Logger: logger}},
		{name: "no logger", params: ManagerParams{ID: testBucketID, Tokens: tokens}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr, err := NewManager(tc.params)
			assert.ErrorIs(t, err, ErrInvalidRequest, "a misconfigured manager is never worth retrying")
			assert.Equal(t, Manager(nil), mgr, "an error carries no manager")
		})
	}
}

// Tests that a manager reports the bucket it was built for. The registry and the ring lookup
// both key on this.
func TestNewManagerReportsItsIdentifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	mgr, err := NewManager(ManagerParams{
		ID:     testBucketID,
		Tokens: persistence.NewMockSemaphoreTokenManager(ctrl),
		Logger: testlogger.New(t),
	})
	require.NoError(t, err)
	assert.Equal(t, testBucketID, mgr.Identifier())
}

// Tests that Start() builds both the free-set and the reverse index from a scan that arrives in
// several pages, following the page token to the end.
func TestStartBuildsTheFreeSetAndTheHeldIndexAcrossPages(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	expectScan(t, m, [][]*persistence.SemaphoreOwnership{
		{tokenRow(1, ""), tokenRow(2, "owner-x")},
		{tokenRow(3, ""), tokenRow(4, "owner-y")},
		{ownerRow("owner-x", 2), ownerRow("owner-y", 4)},
	})

	mgr := newTestManager(t, m)
	require.NoError(t, mgr.Start(context.Background()))

	assert.Equal(t, 2, mgr.freeCount(), "only the unheld slots are free")
	mgr.mu.Lock()
	held := map[string]int{}
	for k, v := range mgr.held {
		held[k] = v
	}
	mgr.mu.Unlock()
	assert.Equal(t, map[string]int{"owner-x": 2, "owner-y": 4}, held)
}

// Tests that a slot an owner row claims stays out of the free-set even though its token row was
// read as free on an earlier page. A scan is not a snapshot, so the two can disagree; believing
// the token row would offer out a slot someone already holds.
func TestStartKeepsAClaimedSlotOutOfTheFreeSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	expectScan(t, m, [][]*persistence.SemaphoreOwnership{
		{tokenRow(1, ""), tokenRow(2, "")},
		{ownerRow("owner-x", 2)},
	})

	mgr := newTestManager(t, m)
	require.NoError(t, mgr.Start(context.Background()))

	assert.Equal(t, 1, mgr.freeCount(), "slot 2 is claimed by an owner row")
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	assert.NotContains(t, mgr.freeIndex, 2)
	assert.Equal(t, map[string]int{"owner-x": 2}, mgr.held)
}

// Tests that Start() counts and steps over a row type it does not recognise, rather than failing
// the load -- a newer writer must not be able to stop this host from loading the bucket.
func TestStartSkipsRowsItCannotClassify(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	mgr := startManager(t, m, []*persistence.SemaphoreOwnership{
		tokenRow(1, ""),
		// A nil row would panic the load if it reached the switch below.
		nil,
		// The zero value: nothing set RowType. The enum starts at 1 so this cannot
		// collide with a real type.
		{TokenID: 2},
		// A type this version does not have a case for.
		{RowType: persistence.SemaphoreRowType(7), TokenID: 3},
		// Malformed rows of a known type are dropped the same way.
		{RowType: persistence.SemaphoreRowTypeToken, TokenID: 0},
		{RowType: persistence.SemaphoreRowTypeOwner, OwnerID: "owner-x", HeldToken: 0},
		{RowType: persistence.SemaphoreRowTypeOwner, OwnerID: "", HeldToken: 3},
	})

	assert.Equal(t, 1, mgr.freeCount())
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	assert.Empty(t, mgr.held)
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

	mgr := newTestManager(t, m)
	require.Error(t, mgr.Start(context.Background()))

	assert.Equal(t, 0, mgr.freeCount())
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	assert.Empty(t, mgr.held)
}

func TestSecondStartIsRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	// Exactly one scan: the second Start must be turned away before it reaches persistence.
	mgr := startManager(t, m, freeTokens(1, 2))

	assert.Error(t, mgr.Start(context.Background()))
	assert.Equal(t, 2, mgr.freeCount(), "the first load's free-set survives")
}

// Testing a manager is started and then stopped leaves no goroutine running.
func TestStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	mgr := startManager(t, m, freeTokens(1, 2))
	mgr.Stop()
}

// Tests that Acquire refuses an empty owner id. Every grant is conditional on the owner, so an
// empty one is a caller bug and must not reach persistence.
func TestAcquireRejectsAnEmptyOwnerID(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1))

	got, err := mgr.Acquire(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidRequest)
	assert.Equal(t, AcquireResult{}, got)
	assert.Equal(t, 1, mgr.freeCount(), "a rejected request must not touch the free-set")
}

// Tests that Acquire on a manager that is not running returns ErrNotReady and no result,
// whichever way the manager became unusable
func TestAcquireBeforeTheBucketIsUsable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl
	}{
		{
			// Stop() before Start() must fail callers rather than leave them blocked on the
			// barrier forever.
			name: "stopped before it was started",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				mgr := newTestManager(t, m)
				mgr.Stop()
				return mgr
			},
		},
		{
			name: "the startup load failed",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Return(nil, errors.New("scan failed"))
				mgr := newTestManager(t, m)
				assert.Error(t, mgr.Start(context.Background()))
				return mgr
			},
		},
		{
			name: "stopped after it was started",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				mgr := startManager(t, m, freeTokens(1))
				mgr.Stop()
				return mgr
			},
		},
		{
			// Losing the bucket mid-load has to stick. A scan that finishes afterwards
			// describes a bucket this host no longer serves, and going live on it would hand
			// out slots the real owner has already given away.
			name: "stopped while it was still loading",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *semaphoreManagerImpl {
				scanning, finishScan := make(chan struct{}), make(chan struct{})
				m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
						close(scanning)
						<-finishScan
						return &persistence.ScanSemaphoreBucketResponse{Ownerships: freeTokens(1, 2)}, nil
					})

				mgr := newTestManager(t, m)
				started := make(chan error, 1)
				go func() { started <- mgr.Start(context.Background()) }()

				<-scanning // pin Stop to the window where the scan is in flight
				mgr.Stop()
				close(finishScan)
				assert.Error(t, <-started, "Start must report that it lost the bucket")
				return mgr
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := tc.setup(t, m)

			// Bounded, so a manager that leaves its callers waiting fails here instead of
			// hanging the whole package until go test gives up. A running manager never
			// reaches this deadline: startup is already over in every case above.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Not "no slot available": a caller cannot tell an unusable bucket from a full
			// one, and would wait on a bucket that is never going to answer.
			got, err := mgr.Acquire(ctx, "owner-a")
			assert.ErrorIs(t, err, ErrNotReady)
			assert.Equal(t, AcquireResult{}, got, "an error carries no result")
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
			return &persistence.ScanSemaphoreBucketResponse{Ownerships: freeTokens(1)}, nil
		})

	mgr := newTestManager(t, m)
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

// Tests that Acquire passes grant's answer through unchanged, since Acquire is the seam callers
// use and must not edit the result on the way out.
func TestAcquireHandsBackWhatTheGrantDecided(t *testing.T) {
	tests := []struct {
		name  string
		rows  []*persistence.SemaphoreOwnership
		setup func(m *persistence.MockSemaphoreTokenManager)
		want  AcquireResult
	}{
		{
			name: "a free slot is acquired",
			rows: freeTokens(7),
			setup: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)
			},
			want: AcquireResult{Outcome: AcquireOutcomeAcquired, TokenID: 7},
		},
		{
			name: "an owner that already holds one gets the same token back",
			rows: append(freeTokens(1), tokenRow(5, "owner-a"), ownerRow("owner-a", 5)),
			setup: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).Return(
					&persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tokenRow(5, "owner-a")}, nil)
			},
			want: AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: 5},
		},
		{
			// Nothing free to draw and no write to contradict the free-set.
			name: "a full bucket answers no-slot",
			rows: []*persistence.SemaphoreOwnership{tokenRow(1, "owner-x"), ownerRow("owner-x", 1)},
			want: AcquireResult{Outcome: AcquireOutcomeNoSlot},
		},
		{
			// The one id on offer turns out to be held, so the free-set was wrong -- but the
			// answer is the same NoSlot the case above gives, which is the under-admission the
			// free-set refresh is meant to close.
			name: "a stale free-set also answers no-slot",
			rows: freeTokens(1),
			setup: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil)
			},
			want: AcquireResult{Outcome: AcquireOutcomeNoSlot},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, tc.rows)
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
	mgr := startManager(t, m, freeTokens(1))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	got, err := mgr.Acquire(context.Background(), "owner-a")
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, AcquireResult{}, got)
	assert.Equal(t, 1, mgr.freeCount(), "a failed write must not cost the slot")
}

func TestGrantOnAFreeSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(7))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			assert.Equal(t, testBucketID.DomainID, req.DomainID)
			assert.Equal(t, testBucketID.SemaphoreName, req.SemaphoreName)
			assert.Equal(t, testBucketID.Bucket, req.Bucket)
			assert.Equal(t, 7, req.TokenID)
			assert.Equal(t, "owner-a", req.OwnerID)
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
		})

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: AcquireOutcomeAcquired, TokenID: 7}, got)
	assert.Equal(t, 0, mgr.freeCount(), "the granted slot must leave the free-set")
	// The other half of recordHold: without this the next acquire by owner-a would draw a
	// second slot instead of being answered from the index.
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	assert.Equal(t, map[string]int{"owner-a": 7}, mgr.held)
}

// Tests that a write refused as SlotTaken sends grant to a different slot, and that the refused
// id stays out of the free-set.
func TestGrantRetriesADifferentSlotWhenTheWriteSaysTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3))

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
	assert.Equal(t, AcquireOutcomeAcquired, got.Outcome)
	require.Len(t, tried, 2)
	assert.NotEqual(t, tried[0], tried[1], "a retry must draw a different slot")
	assert.Equal(t, tried[1], got.TokenID)
	// Three free, one proved taken, one granted.
	assert.Equal(t, 1, mgr.freeCount())
}

func TestGrantGivesUpAfterMaxAttempts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3, 4, 5, 6, 7, 8, 9, 10))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(maxGrantAttempts).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil)

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireOutcomeNoSlot, got.Outcome, "giving up must under-admit, not error")
	assert.Equal(t, 10-maxGrantAttempts, mgr.freeCount(), "every slot proved taken stays out")
}

func TestGrantStopsRetryingOnceTheDeadlineHasPassed(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3, 4, 5))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Exactly one attempt: the write reports the slot taken and cancels the context as it
	// returns, so the second pass through the loop stops instead of writing again.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(context.Context, *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			cancel()
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil
		})

	_, err := mgr.grant(ctx, "owner-a")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 4, mgr.freeCount(), "the one slot proved taken stays out")
}

// Tests the case where the DB write discovers the hold: grant returns the token the
// write named and puts back the id it drew.
func TestGrantWhenTheWriteSaysTheOwnerAlreadyHolds(t *testing.T) {
	tests := []struct {
		name          string
		heldToken     int
		wantFreeCount int
	}{
		{
			// The token the owner already holds is outside this bucket's free-set, so the
			// reserved id going back is the only change and the count is unmoved.
			name:          "held token is not in the free set",
			heldToken:     9,
			wantFreeCount: 3,
		},
		{
			// The reverse index was cold and the free-set wrongly listed the held slot.
			// Recording the hold has to take it out, or the next acquire would draw a slot
			// this owner holds.
			name:          "held token was wrongly listed as free",
			heldToken:     2,
			wantFreeCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, freeTokens(1, 2, 3))

			// Exactly one attempt: retrying an already-held miss would loop forever.
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
				&persistence.GrantSemaphoreTokenResponse{
					Outcome:   persistence.SemaphoreGrantAlreadyHeld,
					HeldToken: tc.heldToken,
				}, nil)

			got, err := mgr.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: tc.heldToken}, got)
			assert.Equal(t, tc.wantFreeCount, mgr.freeCount())
		})
	}
}

// Tests that grant rejects an AlreadyHeld write that names no token, rather than recording the
// zero. Only a confirming read can clear an index entry, and that read rejects id 0, so the
// owner would be refused for as long as this host owns the bucket.
func TestGrantRejectsAnAlreadyHeldWriteWithNoToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld}, nil)

	_, err := mgr.grant(context.Background(), "owner-a")
	require.ErrorIs(t, err, ErrInconsistentState, "a caller must be able to tell this apart from contention")
	require.ErrorContains(t, err, "already-held slot without a token")

	// The write did not apply, so the reserved slot goes back.
	assert.Equal(t, 3, mgr.freeCount())
	mgr.mu.Lock()
	assert.Empty(t, mgr.held, "a no-token reply must not reach the reverse index")
	mgr.mu.Unlock()

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireOutcomeAcquired, got.Outcome)
}

func TestGrantRetriesATransientWriteFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3))

	gomock.InOrder(
		m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(nil, &persistence.TimeoutError{Msg: "write timed out"}),
		m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
			&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil),
	)

	got, err := mgr.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireOutcomeAcquired, got.Outcome)
	assert.Equal(t, 2, mgr.freeCount(), "only the granted slot leaves the free-set")
	assertFreeSetIsConsistent(t, mgr)
}

func TestGrantReportsTheErrorWhenEveryAttemptFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3))

	writeErr := &persistence.TimeoutError{Msg: "write timed out"}
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(maxGrantAttempts).Return(nil, writeErr)

	got, err := mgr.grant(context.Background(), "owner-a")
	assert.ErrorIs(t, err, writeErr)
	assert.Equal(t, AcquireResult{}, got)
	assert.Equal(t, 3, mgr.freeCount(), "every drawn slot comes back")
	assertFreeSetIsConsistent(t, mgr)
}

// Tests that a failed write costs no slot: the id goes back every time, so an outage cannot
// drain the free-set one write at a time.
func TestGrantReturnsTheSlotWhenTheWriteFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	const slots = 5
	mgr := startManager(t, m, freeTokens(1, 2, 3, 4, 5))

	writeErr := errors.New("cassandra unavailable")
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(slots*3).Return(nil, writeErr)

	for i := range slots * 3 {
		_, err := mgr.grant(context.Background(), fmt.Sprintf("owner-%d", i))
		require.ErrorIs(t, err, writeErr)
		require.Equal(t, slots, mgr.freeCount(), "the slot must come back after attempt %d", i)
	}
	assertFreeSetIsConsistent(t, mgr)
}

// Tests that an outcome grant cannot read is reported as an error and its slot returned
func TestGrantRejectsAnUnrecognizedWriteOutcome(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1))

	// The zero value, which is what a store that never set the field returns.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown}, nil)

	_, err := mgr.grant(context.Background(), "owner-a")
	require.ErrorIs(t, err, ErrInconsistentState, "a caller must be able to tell this apart from contention")
	require.ErrorContains(t, err, "unexpected grant outcome")
	assert.Equal(t, 1, mgr.freeCount())
}

// Tests that an index entry storage no longer agrees with is repaired by the confirming read,
// and the grant then falls through to a normal draw.
func TestGrantWhenTheReverseIndexIsStale(t *testing.T) {
	tests := []struct {
		name          string
		ownership     *persistence.SemaphoreOwnership
		readErr       error
		wantFreeCount int
	}{
		{
			// Released behind our back: the slot is genuinely free again, so it goes back
			// into the free-set before the normal pick.
			name:          "slot is free again",
			ownership:     tokenRow(5, ""),
			wantFreeCount: 3,
		},
		{
			// Someone else holds it now. Dropping the index entry is right; adding the
			// slot back would offer out a held slot.
			name:          "another owner holds it now",
			ownership:     tokenRow(5, "owner-b"),
			wantFreeCount: 2,
		},
		{
			// Token rows are seeded once and never deleted, so a missing one means the
			// index named a slot this bucket does not have.
			name:          "token row is gone",
			readErr:       &types.EntityNotExistsError{Message: "not found"},
			wantFreeCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, append(freeTokens(1, 2, 3), ownerRow("owner-a", 5)))

			m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
				func(_ context.Context, req *persistence.GetSemaphoreOwnershipByTokenRequest) (*persistence.GetSemaphoreOwnershipByTokenResponse, error) {
					assert.Equal(t, 5, req.TokenID)
					if tc.readErr != nil {
						return nil, tc.readErr
					}
					return &persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tc.ownership}, nil
				})
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
				&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

			got, err := mgr.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, AcquireOutcomeAcquired, got.Outcome,
				"a stale index entry must fall through to a normal pick")
			assert.Equal(t, tc.wantFreeCount, mgr.freeCount())
		})
	}
}

func TestGrantSurfacesAConfirmingReadFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, append(freeTokens(1), ownerRow("owner-a", 5)))

	readErr := errors.New("cassandra unavailable")
	m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).Times(1).Return(nil, readErr)
	_, err := mgr.grant(context.Background(), "owner-a")
	assert.ErrorIs(t, err, readErr)
	assert.Equal(t, 1, mgr.freeCount())
}

// Tests that recording a hold takes the slot out of the free-set, including a slot this host
// never drew itself.
func TestRecordHoldTakesTheSlotOutOfTheFreeSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3))

	mgr.recordHold("owner-a", 2)

	assertFreeSetIsConsistent(t, mgr)
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	assert.Equal(t, map[string]int{"owner-a": 2}, mgr.held)
	assert.Equal(t, 2, len(mgr.freeList), "a held slot cannot stay free")
	assert.NotContains(t, mgr.freeIndex, 2)
}

// Tests dropping a stale index entry always removes the entry, but returns the slot to the
// free-set only when the token row proved the slot unheld.
func TestDropStaleHold(t *testing.T) {
	tests := []struct {
		name          string
		staleToken    int
		stillFree     bool
		wantHeld      map[string]int
		wantFreeCount int
	}{
		{
			name:          "drops the entry and leaves the slot out when it is still held",
			staleToken:    5,
			stillFree:     false,
			wantHeld:      map[string]int{},
			wantFreeCount: 2,
		},
		{
			name:          "returns the slot when the row says it is unheld",
			staleToken:    5,
			stillFree:     true,
			wantHeld:      map[string]int{},
			wantFreeCount: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			mgr := startManager(t, m, append(freeTokens(1, 2), tokenRow(5, "owner-a"), ownerRow("owner-a", 5)))

			mgr.dropStaleHold("owner-a", tc.staleToken, tc.stillFree)

			assert.Equal(t, tc.wantFreeCount, mgr.freeCount())
			assertFreeSetIsConsistent(t, mgr)
			mgr.mu.Lock()
			defer mgr.mu.Unlock()
			assert.Equal(t, tc.wantHeld, mgr.held)
		})
	}
}

// ---- The free-set ----

// Tests that repeated reserve calls hand out every free id exactly once, and report the set
// empty only after the last one.
func TestReserveDrawsEveryFreeIDExactlyOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1, 2, 3))

	drawn := map[int]bool{}
	for i := 0; i < 3; i++ {
		tokenID, ok := mgr.reserve()
		require.True(t, ok)
		assert.False(t, drawn[tokenID], "reserve drew %d twice", tokenID)
		drawn[tokenID] = true
	}
	assert.Equal(t, map[int]bool{1: true, 2: true, 3: true}, drawn)
	assertFreeSetIsConsistent(t, mgr)

	tokenID, ok := mgr.reserve()
	assert.False(t, ok, "an empty free-set has nothing to draw")
	assert.Equal(t, 0, tokenID)
}

func TestUnreserveIsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	mgr := startManager(t, m, freeTokens(1))

	tokenID, ok := mgr.reserve()
	require.True(t, ok)
	require.Equal(t, 0, mgr.freeCount())

	mgr.unreserve(tokenID)
	assert.Equal(t, 1, mgr.freeCount())
	// A double unreserve must not list the same slot twice, or two acquires could draw it.
	mgr.unreserve(tokenID)
	assert.Equal(t, 1, mgr.freeCount())
	assertFreeSetIsConsistent(t, mgr)
}

// ---- Under concurrency ----

// Tests that reserve and unreserve stay correct when they run concurrently. This is the only
// test that calls unreserve from several goroutines at once: the concurrent grant tests never
// do, since every grant in them applies and so no id is ever put back.
//
// The assertion is mostly the race detector. The two closing checks add that a balanced run of
// draws and returns leaves the free-set whole, rather than slowly losing or duplicating slots.
func TestFreeSetSurvivesConcurrentReserveAndUnreserve(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	const slots = 8
	mgr := startManager(t, m, freeTokens(1, 2, 3, 4, 5, 6, 7, 8))

	var wg sync.WaitGroup
	for i := 0; i < slots; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// A draw can fail when every slot is out on another goroutine, and then
				// there is nothing to return.
				if tokenID, ok := mgr.reserve(); ok {
					mgr.unreserve(tokenID)
				}
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, slots, mgr.freeCount(), "every drawn slot was returned")
	assertFreeSetIsConsistent(t, mgr)
}

func TestConcurrentGrantsHandOutDistinctSlots(t *testing.T) {
	const owners = 20

	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	ids := make([]int, 0, owners)
	for i := 1; i <= owners; i++ {
		ids = append(ids, i)
	}
	mgr := startManager(t, m, freeTokens(ids...))

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
		require.Equal(t, AcquireOutcomeAcquired, res.Outcome)
		assert.False(t, seen[res.TokenID], "slot %d handed out twice", res.TokenID)
		seen[res.TokenID] = true
	}
	assert.Equal(t, 0, mgr.freeCount())
}

// Tests the manager's locking holds while its lifecycle races its work: Start installing
// the scan result, Stop writing the state, and a burst of grants, all at once.
func TestLifecycleUnderConcurrentGrants(t *testing.T) {
	// Each round is a fresh manager. The moment where Start's install meets a grant is easy
	// to miss, so the test repeats: a single round misses an unlocked install more often
	// than it finds one, while ten rounds have found it every time.
	const rounds = 10
	// One grant per token, so every grant can succeed and the free-set ends up empty.
	const grants = 8

	for range rounds {
		ctrl := gomock.NewController(t)
		m := persistence.NewMockSemaphoreTokenManager(ctrl)
		// Which calls happen at all depends on who wins the race -- Stop can land before the
		// scan, and a grant can find no free-set yet -- so every call is optional. Nothing
		// here is asserted: the outcomes are not what this test is about.
		m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Return(
			&persistence.ScanSemaphoreBucketResponse{Ownerships: freeTokens(1, 2, 3, 4, 5, 6, 7, 8)}, nil).AnyTimes()
		m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
			&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil).AnyTimes()

		mgr := newTestManager(t, m)

		var wg sync.WaitGroup
		race := func(f func()) {
			wg.Add(1)
			go func() { defer wg.Done(); f() }()
		}

		// Both errors are discarded on purpose: Start fails with "stopped while loading" when
		// Stop wins, and grant answers NoSlot when the free-set is not installed yet.
		race(func() { _ = mgr.Start(context.Background()) })
		race(func() { mgr.Stop() })
		for g := range grants {
			race(func() { _, _ = mgr.grant(context.Background(), fmt.Sprintf("owner-%d", g)) })
		}
		wg.Wait()

		assertFreeSetIsConsistent(t, mgr)
	}
}

// Tests that every outcome renders a name, and that an unrecognised one renders Unknown rather
// than an empty string.
func TestAcquireOutcomeString(t *testing.T) {
	tests := []struct {
		outcome AcquireOutcome
		want    string
	}{
		{AcquireOutcomeAcquired, "Acquired"},
		{AcquireOutcomeAlreadyHeld, "AlreadyHeld"},
		{AcquireOutcomeNoSlot, "NoSlot"},
		{AcquireOutcomeUnknown, "Unknown"},
		{AcquireOutcome(99), "Unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.outcome.String())
		})
	}
}
