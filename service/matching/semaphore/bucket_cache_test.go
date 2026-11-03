package semaphore

import (
	"maps"
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
// The helpers below read the cache without the bucket lock: the tests that use them run on one
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
	entry, _ := c.lookupToken(tokenID)
	return *entry
}

// setTokenEntry sets tokenID's entry through updateToken, the same code the cache's methods use.
func setTokenEntry(t *testing.T, c *bucketCache, tokenID int, entry tokenEntry) {
	t.Helper()
	require.True(t, c.tokenRange.Contains(tokenID))
	c.updateToken(tokenID, entry)
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
}

// Tests every database answer the cache records, from every start state of token 2.
func TestRecordAnswers(t *testing.T) {
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
		name   string
		record func(c *bucketCache)
		// want is token 2's entry after the answer, whatever it started as. Nil means it does not
		// change.
		want *tokenEntry
	}{
		{
			name: "grant applied",
			record: func(c *bucketCache) {
				require.NoError(t, c.recordGrantResult("owner-a", 2, &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}))
			},
			want: &heldByA,
		},
		{
			name: "grant taken",
			record: func(c *bucketCache) {
				require.NoError(t, c.recordGrantResult("owner-a", 2, &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantSlotTaken}))
			},
			want: &unavailable,
		},
		{
			name: "grant unknown outcome",
			record: func(c *bucketCache) {
				require.NoError(t, c.recordGrantResult("owner-a", 2, &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantUnknown}))
			},
		},
		{
			name:   "read finds owner-a",
			record: func(c *bucketCache) { c.updateFromTokenRow("owner-a", 2, tokenRow(2, "owner-a")) },
			want:   &heldByA,
		},
		{
			name:   "read finds no holder",
			record: func(c *bucketCache) { c.updateFromTokenRow("owner-a", 2, tokenRow(2, "")) },
			want:   &free,
		},
		{
			name:   "read finds another owner",
			record: func(c *bucketCache) { c.updateFromTokenRow("owner-a", 2, tokenRow(2, "owner-b")) },
			want:   &unavailable,
		},
		{
			name:   "read finds no row",
			record: func(c *bucketCache) { c.updateFromTokenRow("owner-a", 2, nil) },
			want:   &unavailable,
		},
	}

	for _, tc := range tests {
		for _, start := range startStates {
			t.Run(tc.name+" from "+start.name, func(t *testing.T) {
				c := newCacheFromRows()
				setTokenEntry(t, c, 2, start.entry)

				tc.record(c)

				want := start.entry
				if tc.want != nil {
					want = *tc.want
				}
				assert.Equal(t, want, getTokenEntry(c, 2))
				assertCacheConsistent(t, c)
				assert.Equal(t, free, getTokenEntry(c, 1), "token 1 is untouched")
				assert.Equal(t, free, getTokenEntry(c, 3), "token 3 is untouched")
			})
		}
	}
}

// Tests that when owner-a holds token 1 and a grant gives owner-a token 2, token 1 becomes
// unavailable: an owner holds one token, so token 1 is no longer owner-a's, but the database has
// not said it is free.
func TestRecordGrantMarksTheOwnersPreviousTokenUnavailable(t *testing.T) {
	c := newCacheFromRows(tokenRow(1, "owner-a"), ownerRow("owner-a", 1))

	require.NoError(t, c.recordGrantResult("owner-a", 2, &persistence.GrantSemaphoreTokenResponse{Outcome: persistence.SemaphoreGrantApplied}))

	assert.Equal(t, map[string]int{"owner-a": 2}, copyTokenByOwner(c))
	assert.Equal(t, []int{1}, tokensInState(c, tokenUnavailable))
	assert.Equal(t, []int{3}, tokensInState(c, tokenFree))
	assertCacheConsistent(t, c)
}

// Tests a grant of token 1 answered AlreadyHeld: token 1 stays free, and owner-a is recorded on the
// token it holds, or an error is returned if that token is outside the range.
func TestRecordGrantAlreadyHeld(t *testing.T) {
	tests := []struct {
		name            string
		token3Start     tokenEntry
		heldToken       int
		wantErr         bool
		wantOwnerTokens map[string]int
		wantFree        []int
	}{
		{
			name:            "held token was free",
			token3Start:     free,
			heldToken:       3,
			wantOwnerTokens: map[string]int{"owner-a": 3},
			wantFree:        []int{1, 2},
		},
		{
			name:            "held token was recorded for another owner",
			token3Start:     heldByB,
			heldToken:       3,
			wantOwnerTokens: map[string]int{"owner-a": 3},
			wantFree:        []int{1, 2},
		},
		{
			name:            "held token was unavailable",
			token3Start:     unavailable,
			heldToken:       3,
			wantOwnerTokens: map[string]int{"owner-a": 3},
			wantFree:        []int{1, 2},
		},
		{
			name:            "held token is outside the range",
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
			setTokenEntry(t, c, 3, tc.token3Start)

			err := c.recordGrantResult("owner-a", 1, &persistence.GrantSemaphoreTokenResponse{
				Outcome:   persistence.SemaphoreGrantAlreadyHeld,
				HeldToken: tc.heldToken,
			})

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

// Tests that no call can put a token outside the range into the cache, including the empty range
// a manager's cache has before its load finishes.
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
				for _, outcome := range []persistence.SemaphoreGrantOutcome{persistence.SemaphoreGrantApplied, persistence.SemaphoreGrantSlotTaken, persistence.SemaphoreGrantUnknown} {
					assert.NoError(t, c.recordGrantResult("owner-a", tokenID, &persistence.GrantSemaphoreTokenResponse{Outcome: outcome}))
				}
				assert.Error(t, c.recordGrantResult("owner-a", tokenID, &persistence.GrantSemaphoreTokenResponse{
					Outcome:   persistence.SemaphoreGrantAlreadyHeld,
					HeldToken: tokenID,
				}))
				for _, holder := range []string{"owner-a", "", "owner-b"} {
					c.updateFromTokenRow("owner-a", tokenID, tokenRow(tokenID, holder))
				}
				c.updateFromTokenRow("owner-a", tokenID, nil)
			}

			assert.Equal(t, tc.wantFree, tokensInState(c, tokenFree))
			assert.Empty(t, copyTokenByOwner(c))
			assertCacheConsistent(t, c)
		})
	}
}
