package semaphore

import (
	"maps"
	"math/rand"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
)

// testTokenRange is the bucket most tests in this file use: three tokens, 1 to 3, built with
// newCacheFromRows. A test that changes one token uses token 2 and checks that tokens 1 and 3 are
// untouched. Every test ends with assertCacheConsistent, which checks the cache's own rules.
var testTokenRange = commonsemaphore.TokenRange{First: 1, Count: 3}

// The token states the tests start from and expect.
var (
	free        = tokenEntry{state: tokenFree}
	reserved    = tokenEntry{state: tokenReserved}
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.tokenByOwner)
}

// tokensInState returns the tokens in state, in id order.
func tokensInState(c *bucketCache, state tokenState) []int {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, _ := c.lookupTokenLocked(tokenID)
	return *entry
}

// setTokenEntry sets tokenID's entry through updateTokenLocked, the same code the cache's methods
// use.
func setTokenEntry(t *testing.T, c *bucketCache, tokenID int, entry tokenEntry) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	require.True(t, c.updateTokenLocked(tokenID, entry))
}

// assertCacheConsistent checks the cache's rules: tokens has one entry per token in the range, and
// tokenByOwner and freeCount match tokens.
func assertCacheConsistent(t *testing.T, c *bucketCache) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()

	require.Len(t, c.tokens, c.tokenRange.Count, "one entry per token in the range")

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

// Tests that reserving hands out each free token once, then reports none left.
func TestReserveFreeTokenHandsOutEachFreeTokenOnce(t *testing.T) {
	c := newCacheFromRows()

	var got []int
	for range testTokenRange.Count {
		tokenID, ok := c.reserveFreeToken()
		require.True(t, ok)
		got = append(got, tokenID)
		assertCacheConsistent(t, c)
	}
	slices.Sort(got)
	assert.Equal(t, []int{1, 2, 3}, got)
	assert.Equal(t, []int{1, 2, 3}, tokensInState(c, tokenReserved), "a reserved token is tracked, not lost")

	_, ok := c.reserveFreeToken()
	assert.False(t, ok, "no free token is left")
}

// Tests each outcome method from every start state of token 2. changes lists the start states the
// method updates and what it updates them to; every other start state must stay the same.
func TestCacheOutcomes(t *testing.T) {
	startStates := []struct {
		name  string
		entry tokenEntry
	}{
		{"free", free},
		{"reserved", reserved},
		{"heldByA", heldByA},
		{"heldByB", heldByB},
		{"unavailable", unavailable},
	}

	tests := []struct {
		name string
		call func(c *bucketCache)
		// changes maps a start state of token 2 to its state after the call.
		changes map[tokenEntry]tokenEntry
	}{
		{
			// The database just gave the token to owner-a, so it replaces any state.
			name:    "grantApplied",
			call:    func(c *bucketCache) { c.grantApplied("owner-a", 2) },
			changes: map[tokenEntry]tokenEntry{free: heldByA, reserved: heldByA, heldByB: heldByA, unavailable: heldByA},
		},
		{
			name:    "grantTaken",
			call:    func(c *bucketCache) { c.grantTaken(2) },
			changes: map[tokenEntry]tokenEntry{reserved: unavailable},
		},
		{
			name:    "grantFailed",
			call:    func(c *bucketCache) { c.grantFailed(2) },
			changes: map[tokenEntry]tokenEntry{reserved: free},
		},
		{
			name:    "releaseApplied",
			call:    func(c *bucketCache) { c.releaseApplied("owner-a", 2) },
			changes: map[tokenEntry]tokenEntry{heldByA: free, unavailable: free},
		},
		{
			name:    "markOwnerHeldTokenFree",
			call:    func(c *bucketCache) { c.markOwnerHeldTokenFree("owner-a", 2) },
			changes: map[tokenEntry]tokenEntry{heldByA: free},
		},
		{
			name:    "markOwnerHeldTokenUnavailable",
			call:    func(c *bucketCache) { c.markOwnerHeldTokenUnavailable("owner-a", 2) },
			changes: map[tokenEntry]tokenEntry{heldByA: unavailable},
		},
	}

	for _, tc := range tests {
		for _, start := range startStates {
			want, changed := tc.changes[start.entry]
			if !changed {
				want = start.entry
			}
			t.Run(tc.name+" from "+start.name, func(t *testing.T) {
				c := newCacheFromRows()
				setTokenEntry(t, c, 2, start.entry)

				tc.call(c)

				assert.Equal(t, want, getTokenEntry(c, 2))
				assertCacheConsistent(t, c)
				assert.Equal(t, tokenFree, getTokenEntry(c, 1).state, "token 1 is untouched")
				assert.Equal(t, tokenFree, getTokenEntry(c, 3).state, "token 3 is untouched")
			})
		}
	}
}

// Tests that when owner-a holds token 1 and is granted token 2, token 1 becomes unavailable: an
// owner holds one token, so token 1 is no longer owner-a's, but the database has not said it is
// free.
func TestGrantAppliedMarksTheOwnersPreviousTokenUnavailable(t *testing.T) {
	c := newCacheFromRows(tokenRow(1, "owner-a"), ownerRow("owner-a", 1))
	setTokenEntry(t, c, 2, reserved)

	c.grantApplied("owner-a", 2)

	assert.Equal(t, map[string]int{"owner-a": 2}, copyTokenByOwner(c))
	assert.Equal(t, []int{1}, tokensInState(c, tokenUnavailable))
	assert.Equal(t, []int{3}, tokensInState(c, tokenFree))
	assertCacheConsistent(t, c)
}

// Tests that when the cache wrongly records owner-a on token 1 and owner-a's release of token 2
// applies, token 1 becomes unavailable: the release deleted owner-a's owner row, so owner-a holds no
// token, but the database has not said token 1 is free.
func TestReleaseAppliedMarksTheOwnersOtherTokenUnavailable(t *testing.T) {
	c := newCacheFromRows(tokenRow(1, "owner-a"), ownerRow("owner-a", 1))

	c.releaseApplied("owner-a", 2)

	assert.Empty(t, copyTokenByOwner(c))
	assert.Equal(t, []int{1}, tokensInState(c, tokenUnavailable))
	assert.Equal(t, []int{2, 3}, tokensInState(c, tokenFree))
	assertCacheConsistent(t, c)
}

// Tests that grantAlreadyHeld frees the reserved token, unless a newer outcome changed it, and
// records the token the owner already holds, or returns an error if that token is outside the range.
func TestGrantAlreadyHeld(t *testing.T) {
	tests := []struct {
		name string
		// token1Start is token 1's state when the outcome arrives. The grant reserved token 1, so it is
		// reserved unless a newer outcome already changed it.
		token1Start     tokenEntry
		token3Start     tokenEntry
		heldToken       int
		wantErr         bool
		wantOwnerTokens map[string]int
		wantFree        []int
	}{
		{
			name:            "held token was free",
			token1Start:     reserved,
			token3Start:     free,
			heldToken:       3,
			wantOwnerTokens: map[string]int{"owner-a": 3},
			wantFree:        []int{1, 2},
		},
		{
			name:            "held token was recorded for another owner",
			token1Start:     reserved,
			token3Start:     heldByB,
			heldToken:       3,
			wantOwnerTokens: map[string]int{"owner-a": 3},
			wantFree:        []int{1, 2},
		},
		{
			// owner-a's grant reserved token 1. Before its answer came back, owner-b's grant was
			// answered AlreadyHeld naming token 1, so token 1 is now owner-b's and is not freed.
			name:            "reserved token was already given to another owner",
			token1Start:     heldByB,
			token3Start:     free,
			heldToken:       3,
			wantOwnerTokens: map[string]int{"owner-a": 3, "owner-b": 1},
			wantFree:        []int{2},
		},
		{
			name:            "held token is outside the range",
			token1Start:     reserved,
			token3Start:     free,
			heldToken:       9,
			wantErr:         true,
			wantOwnerTokens: map[string]int{},
			wantFree:        []int{1, 2, 3},
		},
		{
			// 0 is below every bucket's range. The Cassandra store answers SlotTaken rather than
			// AlreadyHeld(0), but the interface does not promise that.
			name:            "held token is 0",
			token1Start:     reserved,
			token3Start:     free,
			heldToken:       0,
			wantErr:         true,
			wantOwnerTokens: map[string]int{},
			wantFree:        []int{1, 2, 3},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCacheFromRows()
			setTokenEntry(t, c, 1, tc.token1Start)
			setTokenEntry(t, c, 3, tc.token3Start)

			err := c.grantAlreadyHeld("owner-a", 1, tc.heldToken)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.wantOwnerTokens, copyTokenByOwner(c))
			assert.Equal(t, tc.wantFree, tokensInState(c, tokenFree))
			assertCacheConsistent(t, c)
		})
	}
}

// Tests that no method can put a token outside the range into the cache, including the empty
// range a manager's cache has before its load finishes.
func TestCacheIgnoresTokensOutsideTheRange(t *testing.T) {
	tests := []struct {
		name       string
		tokenRange commonsemaphore.TokenRange
		tokenIDs   []int
		wantFree   []int
	}{
		{
			name:       "tokens just outside the range",
			tokenRange: testTokenRange,
			tokenIDs:   []int{0, 4, -1},
			wantFree:   []int{1, 2, 3},
		},
		{
			// The zero value must contain no token: with no entries, a token it let through would
			// index past the end of tokens.
			name:       "empty range",
			tokenRange: commonsemaphore.TokenRange{},
			tokenIDs:   []int{-1, 0, 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newBucketCache(tc.tokenRange, nil)

			for _, tokenID := range tc.tokenIDs {
				c.grantApplied("owner-a", tokenID)
				c.grantTaken(tokenID)
				c.grantFailed(tokenID)
				c.releaseApplied("owner-a", tokenID)
				c.markOwnerHeldTokenFree("owner-a", tokenID)
				c.markOwnerHeldTokenUnavailable("owner-a", tokenID)
			}

			assert.Equal(t, tc.wantFree, tokensInState(c, tokenFree))
			assert.Empty(t, copyTokenByOwner(c))
			assertCacheConsistent(t, c)
		})
	}
}

// Tests that no mix of calls breaks the cache's rules. 8 goroutines each make 2000 random calls,
// with random owners and token ids, including ids outside the range. Each call is one of:
//   - reserve a token, then report a grant result for it, as the manager does
//   - report a result for any token, even one that was never reserved or is held by another
//     owner, as a late answer can
//   - look up an owner's token
//
// At the end, assertCacheConsistent checks the rules. With -race, it also checks that every method
// holds mu.
func TestCacheStaysConsistentUnderRandomConcurrentUse(t *testing.T) {
	const tokens = 8
	c, _ := newBucketCache(commonsemaphore.TokenRange{First: 1, Count: tokens}, nil)
	owners := []string{"owner-a", "owner-b", "owner-c", "owner-d"}

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			r := rand.New(rand.NewSource(seed))
			for range 2000 {
				owner := owners[r.Intn(len(owners))]
				// 0 and tokens+1 are outside the range.
				tokenID := r.Intn(tokens + 2)
				switch r.Intn(8) {
				case 0:
					if reservedToken, ok := c.reserveFreeToken(); ok {
						switch r.Intn(4) {
						case 0:
							c.grantApplied(owner, reservedToken)
						case 1:
							c.grantTaken(reservedToken)
						case 2:
							c.grantFailed(reservedToken)
						case 3:
							_ = c.grantAlreadyHeld(owner, reservedToken, tokenID)
						}
					}
				case 1:
					c.grantApplied(owner, tokenID)
				case 2:
					c.grantTaken(tokenID)
				case 3:
					c.grantFailed(tokenID)
				case 4:
					c.markOwnerHeldTokenFree(owner, tokenID)
				case 5:
					c.markOwnerHeldTokenUnavailable(owner, tokenID)
				case 6:
					c.lookupHeldToken(owner)
				case 7:
					c.releaseApplied(owner, tokenID)
				}
			}
		}(int64(g))
	}
	wg.Wait()

	assertCacheConsistent(t, c)
}
