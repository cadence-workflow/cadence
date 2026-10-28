package semaphore

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
)

// testTokenRange is the bucket most tests in this file use: three tokens, 1 to 3, built with
// newCacheFromRows. A test that changes one token uses token 2 and checks that tokens 1 and 3 are
// untouched. Every test ends with assertCacheConsistent, which checks the cache's own rules.
//
// The helpers below read the cache without the cache lock: the tests that use them run on one
// goroutine, or read only after every goroutine has finished.
var testTokenRange = commonsemaphore.TokenRange{First: 1, Count: 3}

// The token states the tests start from and expect.
var (
	free        = tokenEntry{state: tokenFree}
	heldByA     = tokenEntry{state: tokenHeld, owner: "owner-a"}
	heldByB     = tokenEntry{state: tokenHeld, owner: "owner-b"}
	unavailable = tokenEntry{state: tokenUnavailable}
)

// newCacheFromRows builds a cache over testTokenRange from rows.
func newCacheFromRows(rows ...*persistence.SemaphoreOwnership) *bucketCache {
	c, _ := newBucketCache(testTokenRange, rows)
	return c
}

// copyTokenByOwner returns a copy of the cache's tokenByOwner.
func copyTokenByOwner(c *bucketCache) map[string]int {
	return maps.Clone(c.tokenByOwner)
}

// tokensInState returns the tokens in state, in id order.
func tokensInState(c *bucketCache, state tokenState) []int {
	var ids []int
	for i, entry := range c.tokens {
		if entry.state == state {
			ids = append(ids, c.tokenRange.First+i)
		}
	}
	return ids
}

// getTokenEntry returns a copy of tokenID's entry.
func getTokenEntry(c *bucketCache, tokenID int) tokenEntry {
	entry := c.lookupTokenLocked(tokenID)
	return *entry
}

// setTokenEntry sets tokenID's entry through updateTokenLocked, the same code the cache's methods
// use.
func setTokenEntry(t *testing.T, c *bucketCache, tokenID int, entry tokenEntry) {
	t.Helper()
	require.True(t, c.tokenRange.Contains(tokenID))
	c.updateTokenLocked(tokenID, entry)
}

// lockTokenForTest locks tokenID for the test
func lockTokenForTest(t *testing.T, c *bucketCache, tokenID int) *lockedToken {
	t.Helper()
	lockedToken := c.tryLockToken(tokenID)
	require.NotNil(t, lockedToken, "token %d is already locked", tokenID)
	return lockedToken
}

// assertCacheConsistent checks the cache's rules: tokens has one entry and one lock per token in the
// range, tokenByOwner and freeCount match tokens, and no token is left locked.
func assertCacheConsistent(t *testing.T, c *bucketCache) {
	t.Helper()

	require.Len(t, c.tokens, c.tokenRange.Count, "one entry per token in the range")
	require.Len(t, c.tokenMutexes, c.tokenRange.Count, "one lock per token in the range")
	for i, lock := range c.tokenMutexes {
		if assert.True(t, lock.TryLock(), "token %d is still locked", c.tokenRange.First+i) {
			lock.Unlock()
		}
	}

	wantTokenByOwner := make(map[string]int)
	wantFreeCount := 0
	for i, entry := range c.tokens {
		tokenID := c.tokenRange.First + i
		switch entry.state {
		case tokenHeld:
			require.NotEmpty(t, entry.owner, "token %d: held with no owner", tokenID)
			require.NotContains(t, wantTokenByOwner, entry.owner, "owner %v holds two tokens", entry.owner)
			wantTokenByOwner[entry.owner] = tokenID
		case tokenFree:
			wantFreeCount++
		}
		if entry.state != tokenHeld {
			assert.Empty(t, entry.owner, "token %d: owner set on a token that is not held", tokenID)
		}
	}
	assert.Equal(t, wantTokenByOwner, c.tokenByOwner, "tokenByOwner must match the held tokens")
	assert.Equal(t, wantFreeCount, c.freeCount, "freeCount must match the free tokens")
}

// Tests the state each token gets from the rows of a load scan.
func TestNewBucketCache(t *testing.T) {
	tests := []struct {
		name            string
		tokenRange      commonsemaphore.TokenRange
		rows            []*persistence.SemaphoreOwnership
		wantFree        []int
		wantOwnerTokens map[string]int
		wantUnavailable []int
		wantSkipped     skippedRows
	}{
		{
			name:            "no rows: every token is free",
			tokenRange:      testTokenRange,
			wantFree:        []int{1, 2, 3},
			wantOwnerTokens: map[string]int{},
		},
		{
			name:            "range not starting at 1: its tokens are free",
			tokenRange:      commonsemaphore.TokenRange{First: 11, Count: 2},
			wantFree:        []int{11, 12},
			wantOwnerTokens: map[string]int{},
		},
		{
			name:            "token row and owner row agree: held by that owner",
			tokenRange:      testTokenRange,
			rows:            []*persistence.SemaphoreOwnership{tokenRow(2, "owner-a"), ownerRow("owner-a", 2)},
			wantFree:        []int{1, 3},
			wantOwnerTokens: map[string]int{"owner-a": 2},
		},
		{
			name:            "token row is FREE: free",
			tokenRange:      testTokenRange,
			rows:            []*persistence.SemaphoreOwnership{tokenRow(2, "")},
			wantFree:        []int{1, 2, 3},
			wantOwnerTokens: map[string]int{},
		},
		{
			name:            "owner row names a token whose token row is FREE: held by that owner",
			tokenRange:      testTokenRange,
			rows:            []*persistence.SemaphoreOwnership{tokenRow(2, ""), ownerRow("owner-a", 2)},
			wantFree:        []int{1, 3},
			wantOwnerTokens: map[string]int{"owner-a": 2},
		},
		{
			name:            "token row names another holder than the owner row: the owner row decides",
			tokenRange:      testTokenRange,
			rows:            []*persistence.SemaphoreOwnership{tokenRow(2, "owner-b"), ownerRow("owner-a", 2)},
			wantFree:        []int{1, 3},
			wantOwnerTokens: map[string]int{"owner-a": 2},
		},
		{
			name:            "token row has a holder but no owner row was read: unavailable",
			tokenRange:      testTokenRange,
			rows:            []*persistence.SemaphoreOwnership{tokenRow(2, "owner-a")},
			wantFree:        []int{1, 3},
			wantOwnerTokens: map[string]int{},
			wantUnavailable: []int{2},
		},

		{
			// The range comes from the metadata, not the rows.
			name:       "rows for tokens outside the range: skipped, and they hold nothing",
			tokenRange: testTokenRange,
			rows: []*persistence.SemaphoreOwnership{
				tokenRow(0, "owner-a"),
				tokenRow(4, "owner-b"),
				ownerRow("owner-b", 4),
				ownerRow("owner-c", 0),
			},
			wantFree:        []int{1, 2, 3},
			wantOwnerTokens: map[string]int{},
			wantSkipped:     skippedRows{outOfRange: 4},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, skipped := newBucketCache(tc.tokenRange, tc.rows)

			assertCacheConsistent(t, c)
			assert.Equal(t, tc.wantFree, tokensInState(c, tokenFree))
			assert.Equal(t, tc.wantOwnerTokens, copyTokenByOwner(c))
			assert.Equal(t, tc.wantUnavailable, tokensInState(c, tokenUnavailable))
			assert.Equal(t, tc.wantSkipped, skipped)
		})
	}
}

func TestTryLockFreeToken(t *testing.T) {
	tests := []struct {
		name           string
		tokenRange     commonsemaphore.TokenRange
		entries        map[int]tokenEntry
		lockedByOthers []int
		wantToken      int
		wantLocked     bool
	}{
		{
			name:           "skips a free token another call holds",
			tokenRange:     testTokenRange,
			entries:        map[int]tokenEntry{2: heldByA},
			lockedByOthers: []int{1},
			wantToken:      3,
			wantLocked:     true,
		},
		{
			name:           "range not starting at 1: returns the free token's id",
			tokenRange:     commonsemaphore.TokenRange{First: 11, Count: 3},
			entries:        map[int]tokenEntry{12: heldByA},
			lockedByOthers: []int{11},
			wantToken:      13,
			wantLocked:     true,
		},
		{
			name:           "returns none when every free token is held by another call",
			tokenRange:     testTokenRange,
			entries:        map[int]tokenEntry{2: heldByA},
			lockedByOthers: []int{1, 3},
		},
		{
			name:       "returns none when no token is free",
			tokenRange: testTokenRange,
			entries:    map[int]tokenEntry{1: heldByA, 2: heldByB, 3: unavailable},
		},
		{
			name:       "returns none for the empty range a cache has before its load",
			tokenRange: commonsemaphore.TokenRange{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newBucketCache(tc.tokenRange, nil)
			for tokenID, entry := range tc.entries {
				setTokenEntry(t, c, tokenID, entry)
			}
			var others []*lockedToken
			for _, tokenID := range tc.lockedByOthers {
				others = append(others, lockTokenForTest(t, c, tokenID))
			}

			for range 20 {
				lockedToken := c.tryLockFreeToken()
				require.Equal(t, tc.wantLocked, lockedToken != nil)
				if lockedToken != nil {
					assert.Equal(t, tc.wantToken, lockedToken.id())
					lockedToken.unlock()
				}
			}

			for _, other := range others {
				other.unlock()
			}
			assertCacheConsistent(t, c)
		})
	}
}

func TestLockToken(t *testing.T) {
	tests := []struct {
		name          string
		tokenID       int
		heldByAnother bool
		wantErr       bool
	}{
		{name: "no other call holds the token", tokenID: 2},
		{name: "context ends while another call holds the token", tokenID: 2, heldByAnother: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newBucketCache(testTokenRange, nil)
			var other *lockedToken
			if tc.heldByAnother {
				other = lockTokenForTest(t, c, tc.tokenID)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()

			lockedToken, err := c.lockToken(ctx, tc.tokenID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.tokenID, lockedToken.id())
				lockedToken.unlock()
			}
			if other != nil {
				other.unlock()
			}
			assertCacheConsistent(t, c)
		})
	}
}

func TestTryLockToken(t *testing.T) {
	tests := []struct {
		name          string
		tokenID       int
		heldByAnother bool
		wantLocked    bool
	}{
		{name: "no other call holds the token", tokenID: 2, wantLocked: true},
		{name: "another call holds the token", tokenID: 2, heldByAnother: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newBucketCache(testTokenRange, nil)
			var other *lockedToken
			if tc.heldByAnother {
				other = lockTokenForTest(t, c, tc.tokenID)
			}

			lockedToken := c.tryLockToken(tc.tokenID)

			if tc.wantLocked {
				require.NotNil(t, lockedToken)
				assert.Equal(t, tc.tokenID, lockedToken.id())
				lockedToken.unlock()
			} else {
				assert.Nil(t, lockedToken)
			}
			if other != nil {
				other.unlock()
			}
			assertCacheConsistent(t, c)
		})
	}
}

func TestLockHeldToken(t *testing.T) {
	tests := []struct {
		name          string
		ownerID       string
		heldByAnother bool
		wantLocked    bool
		wantErr       bool
	}{
		{name: "owner has no cached token", ownerID: "owner-b"},
		{name: "locks the owner's token", ownerID: "owner-a", wantLocked: true},
		{name: "context ends while another call holds the owner's token", ownerID: "owner-a", heldByAnother: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCacheFromRows(tokenRow(2, "owner-a"), ownerRow("owner-a", 2))
			var other *lockedToken
			if tc.heldByAnother {
				other = lockTokenForTest(t, c, 2)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()

			got, err := c.lockHeldToken(ctx, tc.ownerID)
			if other != nil {
				other.unlock()
			}

			if tc.wantErr {
				assert.ErrorIs(t, err, context.DeadlineExceeded)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantLocked, got != nil)
			if got != nil {
				assert.Equal(t, 2, got.id())
				got.unlock()
			}
			assertCacheConsistent(t, c)
		})
	}
}

// Tests lockHeldToken when another call holds owner-a's token 2: lockHeldToken waits, the other call
// changes the token and unlocks, and lockHeldToken returns the token only if owner-a is still on it.
func TestLockHeldTokenAfterWaiting(t *testing.T) {
	tests := []struct {
		name string
		// change is what the other call does to the cache before it unlocks token 2.
		change     func(t *testing.T, holder *lockedToken)
		wantLocked bool
	}{
		{
			name:       "the owner stays on the token",
			change:     func(_ *testing.T, holder *lockedToken) { holder.markHeld("owner-a") },
			wantLocked: true,
		},
		{
			name:   "the token was freed",
			change: func(_ *testing.T, holder *lockedToken) { holder.markFree() },
		},
		{
			name: "the owner moved to another token",
			change: func(t *testing.T, holder *lockedToken) {
				other := lockTokenForTest(t, holder.cache, 3)
				other.markHeld("owner-a")
				other.unlock()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCacheFromRows(tokenRow(2, "owner-a"), ownerRow("owner-a", 2))
			holder := lockTokenForTest(t, c, 2)

			result := lockHeldTokenInBackground(t, c, "owner-a")
			waitBriefly() // let lockHeldToken start waiting for token 2
			tc.change(t, holder)
			holder.unlock()
			got := <-result

			require.Equal(t, tc.wantLocked, got != nil)
			if got != nil {
				assert.Equal(t, 2, got.id())
				got.unlock()
			}
			assertCacheConsistent(t, c)
		})
	}
}

// waitBriefly gives a goroutine time to reach a blocking call. Tests use it only to check that
// something has not happened yet; a slow machine can make the check pass when it should fail, but
// never fail when it should pass.
func waitBriefly() { time.Sleep(20 * time.Millisecond) }

// lockHeldTokenInBackground runs lockHeldToken in a goroutine and returns a channel with its result.
func lockHeldTokenInBackground(t *testing.T, c *bucketCache, ownerID string) <-chan *lockedToken {
	result := make(chan *lockedToken, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		lockedToken, err := c.lockHeldToken(ctx, ownerID)
		assert.NoError(t, err)
		result <- lockedToken
	}()
	return result
}

// Tests each mark on token 2, checking every token afterwards. Only marking an owner held changes
// another token: the owner's previous token becomes unavailable.
func TestMark(t *testing.T) {
	tests := []struct {
		name string
		// before and want are the entries before and after the mark. A token not listed is free.
		before map[int]tokenEntry
		mark   func(lockedToken *lockedToken)
		want   map[int]tokenEntry
	}{
		{
			name:   "mark held by owner-a on a free token",
			before: map[int]tokenEntry{2: free},
			mark:   func(lockedToken *lockedToken) { lockedToken.markHeld("owner-a") },
			want:   map[int]tokenEntry{2: heldByA},
		},
		{
			name:   "mark held by owner-a on a token held by owner-a",
			before: map[int]tokenEntry{2: heldByA},
			mark:   func(lockedToken *lockedToken) { lockedToken.markHeld("owner-a") },
			want:   map[int]tokenEntry{2: heldByA},
		},
		{
			name:   "mark held by owner-a on a token held by a different owner",
			before: map[int]tokenEntry{2: heldByB},
			mark:   func(lockedToken *lockedToken) { lockedToken.markHeld("owner-a") },
			want:   map[int]tokenEntry{2: heldByA},
		},
		{
			name:   "mark held by owner-a on an unavailable token",
			before: map[int]tokenEntry{2: unavailable},
			mark:   func(lockedToken *lockedToken) { lockedToken.markHeld("owner-a") },
			want:   map[int]tokenEntry{2: heldByA},
		},
		{
			name:   "mark held by owner-a on a free token while owner-a holds token 1: token 1 becomes unavailable",
			before: map[int]tokenEntry{1: heldByA},
			mark:   func(lockedToken *lockedToken) { lockedToken.markHeld("owner-a") },
			want:   map[int]tokenEntry{1: unavailable, 2: heldByA},
		},
		{
			name:   "mark free on a token held by owner-a",
			before: map[int]tokenEntry{2: heldByA},
			mark:   func(lockedToken *lockedToken) { lockedToken.markFree() },
			want:   map[int]tokenEntry{2: free},
		},
		{
			name:   "mark free on an unavailable token",
			before: map[int]tokenEntry{2: unavailable},
			mark:   func(lockedToken *lockedToken) { lockedToken.markFree() },
			want:   map[int]tokenEntry{2: free},
		},
		{
			name:   "mark unavailable on a free token",
			before: map[int]tokenEntry{2: free},
			mark:   func(lockedToken *lockedToken) { lockedToken.markUnavailable() },
			want:   map[int]tokenEntry{2: unavailable},
		},
		{
			name:   "mark unavailable on a token held by owner-a",
			before: map[int]tokenEntry{2: heldByA},
			mark:   func(lockedToken *lockedToken) { lockedToken.markUnavailable() },
			want:   map[int]tokenEntry{2: unavailable},
		},
		{
			name:   "mark unavailable on an unavailable token",
			before: map[int]tokenEntry{2: unavailable},
			mark:   func(lockedToken *lockedToken) { lockedToken.markUnavailable() },
			want:   map[int]tokenEntry{2: unavailable},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCacheFromRows()
			for tokenID, entry := range tc.before {
				setTokenEntry(t, c, tokenID, entry)
			}

			lockedToken := lockTokenForTest(t, c, 2)
			tc.mark(lockedToken)
			lockedToken.unlock()

			for tokenID := testTokenRange.First; tokenID <= testTokenRange.Last(); tokenID++ {
				want, ok := tc.want[tokenID]
				if !ok {
					want = free
				}
				assert.Equal(t, want, getTokenEntry(c, tokenID), "token %d", tokenID)
			}
			assertCacheConsistent(t, c)
		})
	}
}

// Tests that a second unlock does nothing, even after another call has locked the token.
func TestUnlockTwice(t *testing.T) {
	c, _ := newBucketCache(testTokenRange, nil)
	first := lockTokenForTest(t, c, 2)
	first.unlock()
	second := lockTokenForTest(t, c, 2)

	first.unlock()

	assert.Nil(t, c.tryLockToken(2), "the second unlock let go of another call's lock")
	second.unlock()
	assertCacheConsistent(t, c)
}

// Tests that a mark after unlock panics and leaves the token unchanged.
func TestMarkAfterUnlock(t *testing.T) {
	c := newCacheFromRows(tokenRow(2, "owner-b"), ownerRow("owner-b", 2))
	lockedToken := lockTokenForTest(t, c, 2)
	lockedToken.unlock()

	assert.PanicsWithValue(t, "token 2 marked after unlock", func() { lockedToken.markFree() })

	assert.Equal(t, map[string]int{"owner-b": 2}, copyTokenByOwner(c))
	assertCacheConsistent(t, c)
}
