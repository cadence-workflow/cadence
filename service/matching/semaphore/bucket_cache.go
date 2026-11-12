package semaphore

import (
	"math/rand"
	"sync"

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

// bucketCache is this host's view of a bucket, loaded from a scan. It may be stale; the database
// has the final say. The manager decides what each database answer means and records it with the
// mark methods.
//
// cacheMu guards the token states and is never held across I/O. Methods take it themselves, except
// those ending in Locked. Changes are made only under the bucket lock.
type bucketCache struct {
	// tokenRange is the bucket's token ids. It is set once by newBucketCache.
	tokenRange commonsemaphore.TokenRange

	// cacheMu is the cache lock. It guards tokens, tokenByOwner and freeCount.
	cacheMu sync.Mutex
	// tokens is the token list, one entry per token in token id order. Change it only through
	// updateTokenLocked.
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

// newBucketCache builds the cache from a bucket scan. A token is held if an owner row names it,
// unavailable if only its token row has a holder, and free otherwise. Unreadable and out-of-range
// rows are skipped and counted.
func newBucketCache(tokenRange commonsemaphore.TokenRange, rows []*persistence.SemaphoreOwnership) (*bucketCache, skippedRows) {
	c := &bucketCache{
		tokenRange: tokenRange,
		// Every entry starts as tokenFree, the zero value.
		tokens:       make([]tokenEntry, tokenRange.Count),
		tokenByOwner: make(map[string]int),
		freeCount:    tokenRange.Count,
	}

	var skipped skippedRows
	var heldTokenRows []int
	var ownerRows []*persistence.SemaphoreOwnership
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
			ownerRows = append(ownerRows, row)
		default:
			skipped.unreadable++
		}
	}
	// The scan is paged, so a grant or release landing mid-scan can leave a token row and owner row
	// out of step. Owner rows decide, since the database keeps at most one per owner, so they are
	// applied last.
	for _, tokenID := range heldTokenRows {
		c.markTokenUnavailable(tokenID)
	}
	for _, row := range ownerRows {
		c.markTokenHeld(row.HeldToken, row.OwnerID)
	}
	return c, skipped
}

// updateTokenLocked sets tokenID's entry to newEntry and updates tokenByOwner and freeCount to
// match. If newEntry's owner held another token, that token becomes unavailable. The caller holds
// cacheMu.
func (c *bucketCache) updateTokenLocked(tokenID int, newEntry tokenEntry) {
	oldEntry := c.lookupTokenLocked(tokenID)
	if *oldEntry == newEntry {
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
			c.tokens[c.index(otherToken)] = tokenEntry{state: tokenUnavailable}
		}
		c.tokenByOwner[newEntry.owner] = tokenID
	}
	*oldEntry = newEntry
}

// lookupTokenLocked returns tokenID's entry. The caller holds cacheMu.
func (c *bucketCache) lookupTokenLocked(tokenID int) *tokenEntry {
	return &c.tokens[c.index(tokenID)]
}

// index returns tokenID's position in tokens.
func (c *bucketCache) index(tokenID int) int {
	return tokenID - c.tokenRange.First
}

// lookupHeldToken returns the token the cache records ownerID as holding, or false if none.
func (c *bucketCache) lookupHeldToken(ownerID string) (int, bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	tokenID, ok := c.tokenByOwner[ownerID]
	return tokenID, ok
}

// pickFreeToken returns a random free token, or false if there is none. It does not reserve the
// token; the caller holds the bucket lock until it marks it.
func (c *bucketCache) pickFreeToken() (int, bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
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

// markTokenHeld records that ownerID holds tokenID.
func (c *bucketCache) markTokenHeld(tokenID int, ownerID string) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.updateTokenLocked(tokenID, tokenEntry{state: tokenHeld, owner: ownerID})
}

// markTokenFree records that tokenID is free.
func (c *bucketCache) markTokenFree(tokenID int) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.updateTokenLocked(tokenID, tokenEntry{state: tokenFree})
}

// markTokenUnavailable records that the cache does not know whether tokenID is free, so it is not
// offered until the bucket reloads.
func (c *bucketCache) markTokenUnavailable(tokenID int) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.updateTokenLocked(tokenID, tokenEntry{state: tokenUnavailable})
}

// freeTokenCount returns how many tokens are free.
func (c *bucketCache) freeTokenCount() int {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	return c.freeCount
}

// heldTokenCount returns how many tokens are held.
func (c *bucketCache) heldTokenCount() int {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	return len(c.tokenByOwner)
}
