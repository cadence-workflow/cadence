// Copyright (c) 2025 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package semaphore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
)

var testBucketID = Identifier{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 2}

func tokenRow(tokenID int, holder string) *persistence.SemaphoreOwnership {
	return &persistence.SemaphoreOwnership{
		Kind:          persistence.SemaphoreRowKindToken,
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		TokenID:       tokenID,
		Holder:        holder,
	}
}

func ownerRow(ownerID string, heldToken int) *persistence.SemaphoreOwnership {
	return &persistence.SemaphoreOwnership{
		Kind:          persistence.SemaphoreRowKindOwner,
		DomainID:      testBucketID.DomainID,
		SemaphoreName: testBucketID.SemaphoreName,
		Bucket:        testBucketID.Bucket,
		OwnerID:       ownerID,
		HeldToken:     heldToken,
	}
}

// expectScan stubs the rebuild with the given pages and asserts that the page token from
// each response is threaded into the next request.
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

// startBucket builds a bucket whose rebuild reads the given single page of rows.
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

func TestRebuildAssemblesBothIndexesAcrossPages(t *testing.T) {
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

func TestRebuildTrustsTheOwnerRowWhenThePagesDisagree(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	// A grant landing mid-scan can be missed on the token row and seen on the owner row.
	// The owner row wins, whichever page it arrived on.
	expectScan(t, m, [][]*persistence.SemaphoreOwnership{
		{ownerRow("owner-x", 2)},
		{tokenRow(1, ""), tokenRow(2, "")},
	})

	b := NewBucket(testBucketID, m, testlogger.New(t))
	require.NoError(t, b.Start(context.Background()))
	assert.Equal(t, 1, b.freeCount(), "the slot an owner row claims is not free")
}

func TestRebuildSkipsRowsItCannotClassify(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)

	b := startBucket(t, m, []*persistence.SemaphoreOwnership{
		tokenRow(1, ""),
		{Kind: persistence.SemaphoreRowKindUnknown, TokenID: 2},
		// Malformed rows of a known kind are dropped the same way.
		{Kind: persistence.SemaphoreRowKindToken, TokenID: 0},
		{Kind: persistence.SemaphoreRowKindOwner, OwnerID: "owner-x", HeldToken: 0},
		{Kind: persistence.SemaphoreRowKindOwner, OwnerID: "", HeldToken: 3},
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

func TestRemoveFreeLockedOnTheTailElement(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := persistence.NewMockSemaphoreTokenManager(ctrl)
	// Start empty and add in order, so freeList is [1, 2, 3] and 3 is known to be the tail.
	b := startBucket(t, m, nil)
	for _, id := range []int{1, 2, 3} {
		b.unreserve(id)
	}

	b.mu.Lock()
	b.removeFreeLocked(3)
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

func TestForgetHold(t *testing.T) {
	tests := []struct {
		name          string
		forgetToken   int
		stillFree     bool
		wantHeld      map[string]int
		wantFreeCount int
	}{
		{
			name:          "drops the entry and leaves the slot out when it is still held",
			forgetToken:   5,
			stillFree:     false,
			wantHeld:      map[string]int{},
			wantFreeCount: 2,
		},
		{
			name:          "returns the slot when the row says it is unheld",
			forgetToken:   5,
			stillFree:     true,
			wantHeld:      map[string]int{},
			wantFreeCount: 3,
		},
		{
			// A concurrent grant replaced the entry, so it no longer names the token the
			// caller checked and must survive.
			name:          "keeps an entry that names a different token",
			forgetToken:   4,
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

			b.forgetHold("owner-a", tc.forgetToken, tc.stillFree)

			assert.Equal(t, tc.wantFreeCount, b.freeCount())
			assertFreeSetIsConsistent(t, b)
			b.mu.Lock()
			defer b.mu.Unlock()
			assert.Equal(t, tc.wantHeld, b.held)
		})
	}
}

func TestTheFreeSetSurvivesConcurrentUse(t *testing.T) {
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
