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

// startBucket builds a bucket whose startup load reads the given single page of rows.
func startBucket(t *testing.T, m *persistence.MockSemaphoreTokenManager, rows []*persistence.SemaphoreOwnership) *Bucket {
	t.Helper()
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{rows})
	b := NewBucket(testBucketID, m, testlogger.New(t))
	require.NoError(t, b.Start(context.Background()))
	return b
}

func freeTokens(ids ...int) []*persistence.SemaphoreOwnership {
	rows := make([]*persistence.SemaphoreOwnership, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, tokenRow(id, ""))
	}
	return rows
}

// assertFreeSetIsConsistent checks the invariant tying the two halves of the free-set
// together: every id in freeList is indexed at the position it actually occupies, and
// freeIndex holds nothing else. A swap-remove is easy to get subtly wrong in a way that
// leaves the two disagreeing while every individual operation still looks right.
func assertFreeSetIsConsistent(t *testing.T, b *Bucket) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()

	require.Len(t, b.freeIndex, len(b.freeList), "freeIndex and freeList must hold the same ids")
	for i, tokenID := range b.freeList {
		got, ok := b.freeIndex[tokenID]
		require.True(t, ok, "token %d is in freeList but not in freeIndex", tokenID)
		// A duplicate in freeList would fail here too: only one copy can own the index.
		assert.Equal(t, i, got, "freeIndex has token %d at %d, freeList has it at %d", tokenID, got, i)
	}
}

func TestStartBuildsTheFreeSetAndTheHeldIndexAcrossPages(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	expectScan(t, m, [][]*persistence.SemaphoreOwnership{
		{tokenRow(1, ""), tokenRow(2, "owner-x")},
		{tokenRow(3, ""), tokenRow(4, "owner-y")},
		{ownerRow("owner-x", 2), ownerRow("owner-y", 4)},
	})

	b := NewBucket(testBucketID, m, testlogger.New(t))
	require.NoError(t, b.Start(context.Background()))

	assert.Equal(t, 2, b.freeCount(), "only the unheld slots are free")
	b.mu.Lock()
	held := map[string]int{}
	for k, v := range b.held {
		held[k] = v
	}
	b.mu.Unlock()
	assert.Equal(t, map[string]int{"owner-x": 2, "owner-y": 4}, held)
}

func TestStartTrustsTheOwnerRowWhenThePagesDisagree(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	// A load is not a snapshot: a grant landing between two pages shows up on one of its two
	// rows and is missed on the other. The slot an owner row claims is not free, whatever the
	// token row said.
	//
	// The owner page comes first here on purpose. Real Cassandra returns token rows first,
	// since the table clusters by type, so this order cannot happen today. It is the order
	// that fails if the two are reconciled per page instead of after the last one.
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{
		{ownerRow("owner-x", 2)},
		{tokenRow(1, ""), tokenRow(2, "")},
	})

	b := NewBucket(testBucketID, m, testlogger.New(t))
	require.NoError(t, b.Start(context.Background()))
	assert.Equal(t, 1, b.freeCount(), "the slot an owner row claims is not free")
}

func TestStartSkipsRowsItCannotClassify(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	b := startBucket(t, m, []*persistence.SemaphoreOwnership{
		tokenRow(1, ""),
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

	assert.Equal(t, 1, b.freeCount())
	b.mu.Lock()
	defer b.mu.Unlock()
	assert.Empty(t, b.held)
}

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

	b := NewBucket(testBucketID, m, testlogger.New(t))
	require.Error(t, b.Start(context.Background()))

	assert.Equal(t, 0, b.freeCount())
	b.mu.Lock()
	defer b.mu.Unlock()
	assert.Empty(t, b.held)
}

func TestReserveDrawsEveryFreeIDExactlyOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1, 2, 3))

	drawn := map[int]bool{}
	for i := 0; i < 3; i++ {
		tokenID, ok := b.reserve()
		require.True(t, ok)
		assert.False(t, drawn[tokenID], "reserve drew %d twice", tokenID)
		drawn[tokenID] = true
	}
	assert.Equal(t, map[int]bool{1: true, 2: true, 3: true}, drawn)
	assertFreeSetIsConsistent(t, b)

	tokenID, ok := b.reserve()
	assert.False(t, ok, "an empty free-set has nothing to draw")
	assert.Equal(t, 0, tokenID)
}

func TestUnreserveIsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1))

	tokenID, ok := b.reserve()
	require.True(t, ok)
	require.Equal(t, 0, b.freeCount())

	b.unreserve(tokenID)
	assert.Equal(t, 1, b.freeCount())
	// A double unreserve must not list the same slot twice, or two acquires could draw it.
	b.unreserve(tokenID)
	assert.Equal(t, 1, b.freeCount())
	assertFreeSetIsConsistent(t, b)
}

func TestRemoveFromFreeSetLockedOnTheTailKeepsTheIndexClean(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	// Start empty and add in order, so freeList is [1, 2, 3] and 3 is known to be the tail.
	b := startBucket(t, m, nil)
	for _, id := range []int{1, 2, 3} {
		b.unreserve(id)
	}

	b.mu.Lock()
	b.removeFromFreeSetLocked(3)
	freeList := append([]int(nil), b.freeList...)
	_, stillIndexed := b.freeIndex[3]
	b.mu.Unlock()

	// Removing the tail moves the tail onto itself, so the index entry for the removed id
	// is written back before it is deleted. If the delete ran first, 3 would keep a stale
	// index entry pointing past the end of the slice.
	assert.Equal(t, []int{1, 2}, freeList)
	assert.False(t, stillIndexed, "the removed id must not stay in the index")

	// A stale entry would make the id unaddable, silently costing the bucket a slot.
	b.unreserve(3)
	assert.Equal(t, 3, b.freeCount())
	assertFreeSetIsConsistent(t, b)
}

func TestRecordHoldTakesTheSlotOutOfTheFreeSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1, 2, 3))

	b.recordHold("owner-a", 2)

	assertFreeSetIsConsistent(t, b)
	b.mu.Lock()
	defer b.mu.Unlock()
	assert.Equal(t, map[string]int{"owner-a": 2}, b.held)
	assert.Equal(t, 2, len(b.freeList), "a held slot cannot stay free")
	assert.NotContains(t, b.freeIndex, 2)
}

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
		{
			// A concurrent grant replaced the entry, so it no longer names the token the
			// caller checked and must survive.
			name:          "keeps an entry that names a different token",
			staleToken:    4,
			stillFree:     false,
			wantHeld:      map[string]int{"owner-a": 5},
			wantFreeCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			b := startBucket(t, m, append(freeTokens(1, 2), tokenRow(5, "owner-a"), ownerRow("owner-a", 5)))

			b.dropStaleHold("owner-a", tc.staleToken, tc.stillFree)

			assert.Equal(t, tc.wantFreeCount, b.freeCount())
			assertFreeSetIsConsistent(t, b)
			b.mu.Lock()
			defer b.mu.Unlock()
			assert.Equal(t, tc.wantHeld, b.held)
		})
	}
}

func TestFreeSetSurvivesConcurrentReserveAndUnreserve(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	const slots = 8
	b := startBucket(t, m, freeTokens(1, 2, 3, 4, 5, 6, 7, 8))

	// Whatever else the mutators get wrong under contention, two callers must never be
	// holding the same id at the same time.
	var outMu sync.Mutex
	out := map[int]bool{}

	var wg sync.WaitGroup
	for i := 0; i < slots; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				tokenID, ok := b.reserve()
				if !ok {
					continue
				}
				outMu.Lock()
				assert.False(t, out[tokenID], "token %d was drawn twice without being returned", tokenID)
				out[tokenID] = true
				outMu.Unlock()

				outMu.Lock()
				delete(out, tokenID)
				outMu.Unlock()
				b.unreserve(tokenID)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, slots, b.freeCount(), "every drawn slot was returned")
	assertFreeSetIsConsistent(t, b)
}

func TestGrantOnAFreeSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(7))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			assert.Equal(t, testBucketID.DomainID, req.DomainID)
			assert.Equal(t, testBucketID.SemaphoreName, req.SemaphoreName)
			assert.Equal(t, testBucketID.Bucket, req.Bucket)
			assert.Equal(t, 7, req.TokenID)
			assert.Equal(t, "owner-a", req.OwnerID)
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
		})

	got, err := b.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: AcquireOutcomeAcquired, TokenID: 7}, got)
	assert.Equal(t, 0, b.freeCount(), "the granted slot must leave the free-set")
}

func TestGrantIsIdempotentForTheSameOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, append(freeTokens(1, 2), tokenRow(5, "owner-a"), ownerRow("owner-a", 5)))

	// The reverse-index hit is confirmed against the row, but nothing is written: the
	// mock would fail the test on an unexpected GrantSemaphoreToken.
	m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.GetSemaphoreOwnershipByTokenRequest) (*persistence.GetSemaphoreOwnershipByTokenResponse, error) {
			assert.Equal(t, 5, req.TokenID)
			return &persistence.GetSemaphoreOwnershipByTokenResponse{Ownership: tokenRow(5, "owner-a")}, nil
		})

	got, err := b.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: 5}, got)
	assert.Equal(t, 2, b.freeCount(), "an idempotent retry must not touch the free-set")
}

func TestGrantRetriesADifferentSlotWhenTheWriteSaysTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1, 2, 3))

	var tried []int
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, req *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			tried = append(tried, req.TokenID)
			if len(tried) == 1 {
				return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil
			}
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil
		})

	got, err := b.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireOutcomeAcquired, got.Outcome)
	require.Len(t, tried, 2)
	assert.NotEqual(t, tried[0], tried[1], "a retry must draw a different slot")
	assert.Equal(t, tried[1], got.TokenID)
	// Three free, one proved taken, one granted.
	assert.Equal(t, 1, b.freeCount())
}

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
			// The reverse index was cold *and* the free-set wrongly listed the held slot.
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
			b := startBucket(t, m, freeTokens(1, 2, 3))

			// Exactly one attempt: retrying an already-held miss would loop forever.
			m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
				&persistence.GrantSemaphoreTokenResponse{
					Outcome:   persistence.SemaphoreGrantAlreadyHeld,
					HeldToken: tc.heldToken,
				}, nil)

			got, err := b.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: tc.heldToken}, got)
			assert.Equal(t, tc.wantFreeCount, b.freeCount())
		})
	}
}

// A store reporting AlreadyHeld without naming a token has nothing to hand back. Recording
// the zero would point the reverse index at a token id that cannot exist, and since only a
// confirming read can clear an entry, and that read rejects id 0, the owner would be refused
// for as long as this host owns the bucket.
func TestGrantRejectsAnAlreadyHeldWriteWithNoToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1, 2, 3))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantAlreadyHeld}, nil)

	_, err := b.grant(context.Background(), "owner-a")
	require.ErrorContains(t, err, "already-held slot without a token")

	// The write did not apply, so the reserved slot goes back.
	assert.Equal(t, 3, b.freeCount())
	b.mu.Lock()
	assert.Empty(t, b.held, "a token-less reply must not reach the reverse index")
	b.mu.Unlock()

	// The owner is not wedged: its next acquire goes through as normal.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

	got, err := b.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireOutcomeAcquired, got.Outcome)
}

func TestGrantGivesUpAfterMaxAttempts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1, 2, 3, 4, 5, 6, 7, 8, 9, 10))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(maxGrantAttempts).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil)

	got, err := b.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireOutcomeContended, got.Outcome, "giving up must under-admit, not error")
	assert.Equal(t, 10-maxGrantAttempts, b.freeCount(), "every slot proved taken stays out")
}

// A deadline reached partway through the retries ends the acquire. Without the check at the
// top of the loop, a bucket answering SlotTaken would keep writing until it ran out of
// attempts, every one of them past the deadline the caller asked for.
func TestGrantStopsRetryingOnceTheDeadlineHasPassed(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1, 2, 3, 4, 5))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Exactly one attempt: the write reports the slot taken and cancels the context as it
	// returns, so the second pass through the loop stops instead of writing again.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(context.Context, *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
			cancel()
			return &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil
		})

	_, err := b.grant(ctx, "owner-a")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 4, b.freeCount(), "the one slot proved taken stays out")
}

func TestGrantOnAFullBucketReportsNoSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, []*persistence.SemaphoreOwnership{
		tokenRow(1, "owner-x"), ownerRow("owner-x", 1),
		tokenRow(2, "owner-y"), ownerRow("owner-y", 2),
	})

	got, err := b.grant(context.Background(), "owner-a")
	require.NoError(t, err)
	assert.Equal(t, AcquireResult{Outcome: AcquireOutcomeNoSlot}, got)
	assert.Equal(t, 0, b.freeCount())
}

// A failed write must not cost a slot. Grant explains why returning the id is safe even when
// the write may have landed; what this test adds is the aggregate, since leaking one slot per
// failure would drain the bucket during an outage.
func TestGrantReturnsTheSlotWhenTheWriteFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	const slots = 5
	b := startBucket(t, m, freeTokens(1, 2, 3, 4, 5))

	writeErr := errors.New("cassandra unavailable")
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(slots*3).Return(nil, writeErr)

	// More attempts than there are slots, so a leak of even one per call runs the bucket dry.
	// Checked every round rather than only at the end: a leak then names the attempt it began
	// on, instead of surfacing later as a bucket with nothing left to draw.
	for i := range slots * 3 {
		_, err := b.grant(context.Background(), fmt.Sprintf("owner-%d", i))
		require.ErrorIs(t, err, writeErr)
		require.Equal(t, slots, b.freeCount(), "the slot must come back after attempt %d", i)
	}
	assertFreeSetIsConsistent(t, b)
}

// The nosql store screens outcomes before they reach here, so only a different store could
// produce one. It is reported as an error rather than guessed at, and the slot goes back for
// the same reason a failed write's does: what the write did is unknown, and the conditional
// guard settles it on the next attempt.
func TestGrantRejectsAnUnrecognizedWriteOutcome(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1))

	// The zero value, which is what a store that never set the field returns.
	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(1).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown}, nil)

	_, err := b.grant(context.Background(), "owner-a")
	require.ErrorContains(t, err, "unexpected semaphore grant outcome")
	assert.Equal(t, 1, b.freeCount())
}

func TestGrantWhenTheReverseIndexIsStale(t *testing.T) {
	tests := []struct {
		name string
		// what the confirming point read reports about the slot the index named
		ownership *persistence.SemaphoreOwnership
		readErr   error
		// free-set size once the stale entry is dealt with and one new slot is granted
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
			b := startBucket(t, m, append(freeTokens(1, 2, 3), ownerRow("owner-a", 5)))

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

			got, err := b.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, AcquireOutcomeAcquired, got.Outcome,
				"a stale index entry must fall through to a normal pick")
			assert.Equal(t, tc.wantFreeCount, b.freeCount())
		})
	}
}

func TestGrantSurfacesAConfirmingReadFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, append(freeTokens(1), ownerRow("owner-a", 5)))

	readErr := errors.New("cassandra unavailable")
	m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).Times(1).Return(nil, readErr)
	_, err := b.grant(context.Background(), "owner-a")
	assert.ErrorIs(t, err, readErr)
	assert.Equal(t, 1, b.freeCount())
}

func TestAcquireRejectsAnEmptyOwnerID(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1))

	_, err := b.Acquire(context.Background(), "")
	assert.Error(t, err)
	assert.Equal(t, 1, b.freeCount())
}

func TestAcquireBeforeTheBucketIsUsable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *Bucket
	}{
		{
			// Stop before Start must fail callers rather than leave them blocked on the
			// barrier forever.
			name: "stopped before it was started",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *Bucket {
				b := NewBucket(testBucketID, m, testlogger.New(t))
				b.Stop()
				return b
			},
		},
		{
			name: "the startup load failed",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *Bucket {
				m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Return(nil, errors.New("scan failed"))
				b := NewBucket(testBucketID, m, testlogger.New(t))
				assert.Error(t, b.Start(context.Background()))
				return b
			},
		},
		{
			name: "stopped after it was started",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *Bucket {
				b := startBucket(t, m, freeTokens(1))
				b.Stop()
				return b
			},
		},
		{
			// Losing the bucket mid-load has to stick. A scan that finishes afterwards
			// describes a bucket this host no longer serves, and going live on it would hand
			// out slots the real owner has already given away.
			name: "stopped while it was still loading",
			setup: func(t *testing.T, m *persistence.MockSemaphoreTokenManager) *Bucket {
				scanning, release := make(chan struct{}), make(chan struct{})
				m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
						close(scanning)
						<-release
						return &persistence.ScanSemaphoreBucketResponse{Ownerships: freeTokens(1, 2)}, nil
					})

				b := NewBucket(testBucketID, m, testlogger.New(t))
				started := make(chan error, 1)
				go func() { started <- b.Start(context.Background()) }()

				<-scanning // pin Stop to the window where the scan is in flight
				b.Stop()
				close(release)
				assert.Error(t, <-started, "Start must report that it lost the bucket")
				return b
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			b := tc.setup(t, m)

			// Not "no slot available": a caller cannot tell an unusable bucket from a full
			// one, and would wait on a bucket that is never going to answer.
			_, err := b.Acquire(context.Background(), "owner-a")
			assert.ErrorIs(t, err, ErrNotReady)
		})
	}
}

// Start is a one-shot. A second call would scan again and overwrite the indexes the first
// one installed, discarding every hold recorded in between.
func TestSecondStartIsRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	// Exactly one scan: the second Start must be turned away before it reaches persistence.
	b := startBucket(t, m, freeTokens(1, 2))

	assert.Error(t, b.Start(context.Background()))
	assert.Equal(t, 2, b.freeCount(), "the first load's free-set survives")
}

// Start, Stop and a burst of grants all racing. The assertion is mostly the race detector:
// every read of the indexes has to be under the mutex, including the ones on the paths that
// only run at startup.
func TestLifecycleUnderConcurrentGrants(t *testing.T) {
	const grants = 8

	for i := 0; i < 10; i++ {
		ctrl := gomock.NewController(t)
		m := persistence.NewMockSemaphoreTokenManager(ctrl)
		m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).Return(
			&persistence.ScanSemaphoreBucketResponse{Ownerships: freeTokens(1, 2, 3, 4, 5, 6, 7, 8)}, nil).AnyTimes()
		m.EXPECT().GetSemaphoreOwnershipByToken(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, r *persistence.GetSemaphoreOwnershipByTokenRequest) (*persistence.GetSemaphoreOwnershipByTokenResponse, error) {
				return &persistence.GetSemaphoreOwnershipByTokenResponse{
					Ownership: &persistence.SemaphoreOwnership{TokenID: r.TokenID},
				}, nil
			}).AnyTimes()
		m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
			&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil).AnyTimes()

		b := NewBucket(testBucketID, m, testlogger.New(t))

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _ = b.Start(context.Background()) }()
		go func() { defer wg.Done(); b.Stop() }()
		for g := 0; g < grants; g++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				// Any outcome is fine here, including ErrNotReady once Stop has landed.
				_, _ = b.grant(context.Background(), fmt.Sprintf("owner-%d", n))
			}(g)
		}
		wg.Wait()

		assertFreeSetIsConsistent(t, b)
	}
}

// A slow startup load must not hold acquires past their own deadlines. Blocking on it would
// put every request behind one scan, and the caller cannot tell that apart from a slow grant.
func TestAcquireHonorsItsDeadlineWhileStarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	scanning, release := make(chan struct{}), make(chan struct{})
	m.EXPECT().ScanSemaphoreBucket(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *persistence.ScanSemaphoreBucketRequest) (*persistence.ScanSemaphoreBucketResponse, error) {
			close(scanning)
			<-release
			return &persistence.ScanSemaphoreBucketResponse{Ownerships: freeTokens(1)}, nil
		})

	b := NewBucket(testBucketID, m, testlogger.New(t))
	started := make(chan error, 1)
	go func() { started <- b.Start(context.Background()) }()
	<-scanning
	defer func() {
		close(release)
		assert.NoError(t, <-started)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := b.Acquire(ctx, "owner-a")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestConcurrentGrantsHandOutDistinctSlots(t *testing.T) {
	const owners = 20

	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	ids := make([]int, 0, owners)
	for i := 1; i <= owners; i++ {
		ids = append(ids, i)
	}
	b := startBucket(t, m, freeTokens(ids...))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(owners).Return(
		&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}, nil)

	var wg sync.WaitGroup
	results := make([]AcquireResult, owners)
	errs := make([]error, owners)
	for i := 0; i < owners; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = b.grant(context.Background(), fmt.Sprintf("owner-%d", i))
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
	assert.Equal(t, 0, b.freeCount())
}

func TestAcquireOutcomeString(t *testing.T) {
	tests := []struct {
		outcome AcquireOutcome
		want    string
	}{
		{AcquireOutcomeAcquired, "Acquired"},
		{AcquireOutcomeAlreadyHeld, "AlreadyHeld"},
		{AcquireOutcomeNoSlot, "NoSlot"},
		{AcquireOutcomeContended, "Contended"},
		{AcquireOutcomeUnknown, "Unknown"},
		{AcquireOutcome(99), "Unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.outcome.String())
		})
	}
}

// The two no-token answers are told apart by whether a write said the slot was taken, not by
// where the acquire gave up. reserve draws from inside the retry loop, so a call can prove the
// free-set wrong and then run it dry in the same pass. Reporting that as NoSlot would hand back
// a confident "the bucket is full" from a call that had just disproved its own cache -- and from
// Phase 4 that answer is what queues a waiter nothing will wake.
func TestGrantSeparatesAFullBucketFromAStaleOne(t *testing.T) {
	tests := []struct {
		name  string
		free  []int
		taken int // how many conditional writes report the slot already taken
		want  AcquireOutcome
	}{
		{
			// Nothing to draw, so no write happens and nothing contradicts the free-set.
			name: "empty free-set answers full",
			free: nil,
			want: AcquireOutcomeNoSlot,
		},
		{
			// Fewer ids than attempts, so the loop drains the set and exits through the
			// same return the empty case uses -- but here the cache was proved wrong first.
			name:  "free-set drained mid-call answers contended",
			free:  []int{1, 2},
			taken: 2,
			want:  AcquireOutcomeContended,
		},
		{
			// More ids than attempts, so the loop exhausts its budget instead.
			name:  "retry budget exhausted answers contended",
			free:  []int{1, 2, 3, 4, 5, 6, 7},
			taken: maxGrantAttempts,
			want:  AcquireOutcomeContended,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			b := startBucket(t, m, freeTokens(tc.free...))

			if tc.taken > 0 {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Times(tc.taken).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil)
			}

			got, err := b.grant(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Outcome)
			assert.Zero(t, got.TokenID, "neither answer carries a token")
			assert.Equal(t, len(tc.free)-tc.taken, b.freeCount(), "every id proved taken stays out")
			assertFreeSetIsConsistent(t, b)
		})
	}
}

// Acquire is the seam callers use, so what it must not do is edit the answer on the way out.
// Both no-token outcomes have to arrive intact: collapsing Contended into NoSlot here would
// hand a caller a confident "the bucket is full" from a grant that had just disproved its own
// free-set, which is the whole thing the two outcomes exist to keep apart.
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
			// The one id on offer turns out to be held, so the free-set is known to be wrong.
			// This must not reach the caller looking like the case above it.
			name: "a stale free-set answers contended",
			rows: freeTokens(1),
			setup: func(m *persistence.MockSemaphoreTokenManager) {
				m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(
					&persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}, nil)
			},
			want: AcquireResult{Outcome: AcquireOutcomeContended},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := persistence.NewMockSemaphoreTokenManager(ctrl)
			b := startBucket(t, m, tc.rows)
			if tc.setup != nil {
				tc.setup(m)
			}

			got, err := b.Acquire(context.Background(), "owner-a")
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// An error is not an outcome: a bucket this host cannot answer for has to stay distinguishable
// from one that answered "no slot", or a caller reads "ask somewhere else" as "the semaphore is
// full".
func TestAcquireSurfacesAnErrorRatherThanAnOutcome(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := NewBucket(testBucketID, m, testlogger.New(t))
	b.Stop()

	got, err := b.Acquire(context.Background(), "owner-a")
	assert.ErrorIs(t, err, ErrNotReady)
	assert.Equal(t, AcquireResult{}, got)
}

// A grant that fails is the caller's failure too. Readiness is checked before grant runs, so
// this is the only remaining way Acquire returns an error, and it must not be flattened into a
// no-token answer.
func TestAcquireSurfacesAFailedGrant(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	b := startBucket(t, m, freeTokens(1))

	m.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	got, err := b.Acquire(context.Background(), "owner-a")
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, AcquireResult{}, got)
	assert.Equal(t, 1, b.freeCount(), "a failed write must not cost the slot")
}
