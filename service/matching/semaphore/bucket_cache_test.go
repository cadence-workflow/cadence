package semaphore

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
)

// testTokenRange is the bucket most tests in this file use: three tokens, 1 to 3, built with
// newCacheFromRows. A test that changes one token uses token 2 and checks that tokens 1 and 3 are
// untouched. Every test ends with assertCacheConsistent, which checks the cache's own rules.
//
// The helpers below use the cache without cacheMu: the tests that use them run on one goroutine, or
// read only after every goroutine has finished.
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
	return *c.lookupTokenLocked(tokenID)
}

// setTokenEntry sets tokenID's entry through updateTokenLocked, the code the cache's methods use.
func setTokenEntry(t *testing.T, c *bucketCache, tokenID int, entry tokenEntry) {
	t.Helper()
	require.True(t, c.tokenRange.Contains(tokenID))
	c.updateTokenLocked(tokenID, entry)
}

// assertCacheConsistent checks the cache's rules: tokens has one entry per token in the range, and
// tokenByOwner and freeCount match tokens.
func assertCacheConsistent(t *testing.T, c *bucketCache) {
	t.Helper()

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

			// The result does not depend on the order of the rows.
			reversedRows := slices.Clone(tc.rows)
			slices.Reverse(reversedRows)
			reversed, reversedSkipped := newBucketCache(tc.tokenRange, reversedRows)
			assert.Equal(t, c.tokens, reversed.tokens)
			assert.Equal(t, c.tokenByOwner, reversed.tokenByOwner)
			assert.Equal(t, skipped, reversedSkipped)
		})
	}
}

// Tests that pickFreeToken returns only free tokens, can return each of them, changes nothing, and
// reports none when no token is free.
func TestPickFreeToken(t *testing.T) {
	c := newCacheFromRows(tokenRow(2, "owner-a"), ownerRow("owner-a", 2))

	seen := map[int]bool{}
	for range 200 {
		tokenID, ok := c.pickFreeToken()
		require.True(t, ok)
		seen[tokenID] = true
	}
	assert.Equal(t, map[int]bool{1: true, 3: true}, seen, "only free tokens, and each of them")
	assert.Equal(t, []int{1, 3}, tokensInState(c, tokenFree), "picking changes nothing")

	setTokenEntry(t, c, 1, unavailable)
	setTokenEntry(t, c, 3, heldByB)
	_, ok := c.pickFreeToken()
	assert.False(t, ok, "no free token is left")
	assertCacheConsistent(t, c)

	// pickFreeToken returns the bucket's first token id plus the position. This guards against
	// accidentally returning position+1, which gives the right token id only in bucket 0.
	c, _ = newBucketCache(commonsemaphore.TokenRange{First: 11, Count: 3}, nil)
	setTokenEntry(t, c, 12, heldByA)
	seen = map[int]bool{}
	for range 200 {
		tokenID, ok := c.pickFreeToken()
		require.True(t, ok)
		seen[tokenID] = true
	}
	assert.Equal(t, map[int]bool{11: true, 13: true}, seen, "range not starting at 1")
	assertCacheConsistent(t, c)
}

// Tests each mark method from every start state of token 2.
func TestMarkToken(t *testing.T) {
	startStates := []struct {
		name  string
		entry tokenEntry
	}{
		{"free", free},
		{"heldByA", heldByA},
		{"heldByB", heldByB},
		{"unavailable", unavailable},
	}

	tests := []struct {
		name string
		mark func(c *bucketCache)
		want tokenEntry
	}{
		{
			name: "held by owner-a",
			mark: func(c *bucketCache) { c.markTokenHeld(2, "owner-a") },
			want: heldByA,
		},
		{
			name: "free",
			mark: func(c *bucketCache) { c.markTokenFree(2) },
			want: free,
		},
		{
			name: "unavailable",
			mark: func(c *bucketCache) { c.markTokenUnavailable(2) },
			want: unavailable,
		},
	}

	for _, tc := range tests {
		for _, start := range startStates {
			t.Run(tc.name+" from "+start.name, func(t *testing.T) {
				c := newCacheFromRows()
				setTokenEntry(t, c, 2, start.entry)

				tc.mark(c)

				assert.Equal(t, tc.want, getTokenEntry(c, 2))
				assertCacheConsistent(t, c)
				assert.Equal(t, free, getTokenEntry(c, 1), "token 1 is untouched")
				assert.Equal(t, free, getTokenEntry(c, 3), "token 3 is untouched")
			})
		}
	}
}

// Tests that when owner-a holds token 1 and is marked holding token 2, token 1 becomes unavailable:
// an owner holds one token, so token 1 is no longer owner-a's, but the database has not said it is
// free.
func TestMarkTokenHeldMarksTheOwnersPreviousTokenUnavailable(t *testing.T) {
	c := newCacheFromRows(tokenRow(1, "owner-a"), ownerRow("owner-a", 1))

	c.markTokenHeld(2, "owner-a")

	assert.Equal(t, map[string]int{"owner-a": 2}, copyTokenByOwner(c))
	assert.Equal(t, []int{1}, tokensInState(c, tokenUnavailable))
	assert.Equal(t, []int{3}, tokensInState(c, tokenFree))
	assertCacheConsistent(t, c)
}
