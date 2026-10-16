package semaphore

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/uber/cadence/common/locks"
	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
)

// tokenState is what this host knows about one token of the bucket.
type tokenState int

const (
	// tokenFree: can be granted. The zero value, so a new entry is free.
	tokenFree tokenState = iota
	// tokenHeld: owner holds it, and tokenByOwner[owner] is this token.
	tokenHeld
	// tokenUnavailable: the cache does not know whether it is free, so it is not offered. It becomes
	// free again only when the bucket reloads.
	tokenUnavailable
)

// tokenEntry is one token's state.
type tokenEntry struct {
	state tokenState
	// owner is set only when state is tokenHeld.
	owner string
}

// bucketCache is this host's view of a bucket, loaded from a scan. It may be stale; the database's
// conditional writes have the final say.
//
// One lock guards the bucket. The manager holds it across each database call: lock, look in the
// cache, call the database, record the answer, unlock. So a bucket makes one call at a time, like
// History's shard lock; Cassandra runs conditional writes on one partition one at a time anyway.
// Every method except lock, unlock and newBucketCache needs the caller to hold the lock.
//
// When unsure whether a token is free, the cache marks it unavailable: a wrong free costs a refused
// write, and enough of them make acquires answer NoSlot while tokens are free.
type bucketCache struct {
	// tokenRange is the bucket's token ids. It is set once by newBucketCache.
	tokenRange commonsemaphore.TokenRange

	// mu is the bucket lock. Lock gives up when the caller's context ends, so a request waiting
	// behind a slow database call leaves at its own deadline.
	mu locks.Mutex
	// tokens holds tokenID's entry at tokens[tokenID-tokenRange.First]. Change entries only through
	// updateToken, so tokenByOwner and freeCount stay matching.
	tokens []tokenEntry
	// tokenByOwner maps each owner to the token it holds.
	tokenByOwner map[string]int
	// freeCount is how many entries in tokens are free.
	freeCount int
}

// skippedRows counts the scanned rows newBucketCache did not use.
type skippedRows struct {
	unreadable int
	outOfRange int
}

// newBucketCache builds the cache from a bucket scan. Each token in tokenRange is:
//   - held, if an owner row names it
//   - unavailable, if its token row has a holder but no owner row names it
//   - free, otherwise, including a token with no row
//
// Unreadable rows and rows for tokens outside tokenRange are skipped and counted in skippedRows.
func newBucketCache(tokenRange commonsemaphore.TokenRange, rows []*persistence.SemaphoreOwnership) (*bucketCache, skippedRows) {
	c := &bucketCache{
		tokenRange: tokenRange,
		mu:         locks.NewMutex(),
		// Every entry starts as tokenFree, the zero value.
		tokens:       make([]tokenEntry, tokenRange.Count),
		tokenByOwner: make(map[string]int),
		freeCount:    tokenRange.Count,
	}

	// No lock is needed below: the cache is not shared until newBucketCache returns.
	var skipped skippedRows
	var heldTokenRows []int
	for _, row := range rows {
		if row == nil {
			// The nosql store never returns one, but the interface does not promise it,
			// and one nil row would panic the whole host.
			skipped.unreadable++
			continue
		}
		switch row.RowType {
		case persistence.SemaphoreRowTypeToken:
			if !tokenRange.Contains(row.TokenID) {
				skipped.outOfRange++
				continue
			}
			// This token has an owner.
			if row.Holder != "" {
				heldTokenRows = append(heldTokenRows, row.TokenID)
			}
		case persistence.SemaphoreRowTypeOwner:
			if row.OwnerID == "" {
				skipped.unreadable++
				continue
			}
			if !tokenRange.Contains(row.HeldToken) {
				skipped.outOfRange++
				continue
			}
			c.updateToken(row.HeldToken, tokenEntry{state: tokenHeld, owner: row.OwnerID})
		default:
			skipped.unreadable++
		}
	}
	// Applied after every owner row, so an owner row decides who holds its token whatever order
	// the rows came in.
	for _, tokenID := range heldTokenRows {
		if entry, _ := c.lookupToken(tokenID); entry.state != tokenHeld {
			c.updateToken(tokenID, tokenEntry{state: tokenUnavailable})
		}
	}
	return c, skipped
}

// lock takes the bucket lock, waiting while another call holds it. It returns an error if ctx
// ends while it waits.
func (c *bucketCache) lock(ctx context.Context) error {
	return c.mu.Lock(ctx)
}

// unlock lets go of the bucket lock.
func (c *bucketCache) unlock() {
	c.mu.Unlock()
}

// updateToken replaces tokenID's entry with newEntry, and keeps tokenByOwner and freeCount in
// step:
//   - freeCount goes up if tokenID becomes free, and down if it stops being free
//   - if tokenID was held, its old owner leaves tokenByOwner
//   - if newEntry's owner held another token, that token becomes unavailable: an owner holds one
//     token, but the database has not said the other one is free
//
// A token outside the range is ignored.
func (c *bucketCache) updateToken(tokenID int, newEntry tokenEntry) {
	oldEntry, ok := c.lookupToken(tokenID)
	if !ok || *oldEntry == newEntry {
		return
	}
	if oldEntry.state == tokenFree {
		c.freeCount--
	}
	if newEntry.state == tokenFree {
		c.freeCount++
	}
	if oldEntry.state == tokenHeld {
		delete(c.tokenByOwner, oldEntry.owner)
	}
	if newEntry.state == tokenHeld {
		// An owner holds at most one token. If the cache already has this owner on another token,
		// that record is now out of date, because the database just said the owner holds tokenID.
		// But the database didn't say the other token is free, so it becomes unavailable rather
		// than free.
		if otherToken, ok := c.tokenByOwner[newEntry.owner]; ok && otherToken != tokenID {
			c.tokens[otherToken-c.tokenRange.First] = tokenEntry{state: tokenUnavailable}
		}
		c.tokenByOwner[newEntry.owner] = tokenID
	}
	*oldEntry = newEntry
}

// lookupToken returns tokenID's entry, or false if tokenID is outside the range.
func (c *bucketCache) lookupToken(tokenID int) (*tokenEntry, bool) {
	if !c.tokenRange.Contains(tokenID) {
		return nil, false
	}
	return &c.tokens[tokenID-c.tokenRange.First], true
}

// lookupHeldToken returns the token the cache records ownerID as holding, or false if none.
func (c *bucketCache) lookupHeldToken(ownerID string) (int, bool) {
	tokenID, ok := c.tokenByOwner[ownerID]
	return tokenID, ok
}

// updateFromTokenRow updates tokenID's entry from what its token row says. row is nil if the token
// has no row.
//   - row names ownerID: tokenID is held by ownerID
//   - row has no holder: tokenID is free
//   - row names another owner: tokenID becomes unavailable. The cache records an owner only from an
//     owner row, as load does
//   - no row: tokenID becomes unavailable. This should not happen, since a grant writes the token
//     row together with the owner row and nothing deletes it
func (c *bucketCache) updateFromTokenRow(ownerID string, tokenID int, row *persistence.SemaphoreOwnership) {
	switch {
	case row == nil:
		c.updateToken(tokenID, tokenEntry{state: tokenUnavailable})
	case row.Holder == ownerID:
		c.updateToken(tokenID, tokenEntry{state: tokenHeld, owner: ownerID})
	case row.Holder == "":
		c.updateToken(tokenID, tokenEntry{state: tokenFree})
	default:
		c.updateToken(tokenID, tokenEntry{state: tokenUnavailable})
	}
}

// pickFreeToken returns a random free token, or false if there is none. It changes nothing: the
// caller holds the lock until it records the grant's answer, so no other call can pick it meanwhile.
func (c *bucketCache) pickFreeToken() (int, bool) {
	if c.freeCount == 0 {
		return 0, false
	}
	k := rand.Intn(c.freeCount)
	for i := range c.tokens {
		if c.tokens[i].state != tokenFree {
			continue
		}
		if k == 0 {
			return c.tokenRange.First + i, true
		}
		k--
	}
	return 0, false
}

// recordGrantResult records the answer to ownerID's grant of tokenID. For a write that returned an
// error, record nothing: tokenID stays free, and if the write did land, the database refuses the
// next grant on it and the cache marks it unavailable.
//   - applied: tokenID is held by ownerID
//   - taken: someone else holds tokenID, so it becomes unavailable
//   - AlreadyHeld: the grant did not take tokenID, and ownerID is recorded on resp.HeldToken. If
//     resp.HeldToken is outside the range, nothing changes and an error is returned
//   - an outcome the cache cannot read: nothing changes
func (c *bucketCache) recordGrantResult(ownerID string, tokenID int, resp *persistence.GrantSemaphoreTokenResponse) error {
	switch resp.Outcome {
	case persistence.SemaphoreGrantApplied:
		c.updateToken(tokenID, tokenEntry{state: tokenHeld, owner: ownerID})
	case persistence.SemaphoreGrantSlotTaken:
		c.updateToken(tokenID, tokenEntry{state: tokenUnavailable})
	case persistence.SemaphoreGrantAlreadyHeld:
		if !c.tokenRange.Contains(resp.HeldToken) {
			return fmt.Errorf("owner %v holds token %d, which is outside tokens %d to %d", ownerID, resp.HeldToken, c.tokenRange.First, c.tokenRange.Last())
		}
		c.updateToken(resp.HeldToken, tokenEntry{state: tokenHeld, owner: ownerID})
	}
	return nil
}

// freeTokenCount returns how many tokens are free.
func (c *bucketCache) freeTokenCount() int {
	return c.freeCount
}

// ownerCount returns how many owners hold a token.
func (c *bucketCache) ownerCount() int {
	return len(c.tokenByOwner)
}
