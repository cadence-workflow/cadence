package semaphore

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
)

// tokenState is what this host knows about one token of the bucket.
type tokenState int

const (
	// tokenFree: can be reserved. The zero value, so a new entry is free.
	tokenFree tokenState = iota
	// tokenReserved: a grant reserved it, and its write has not come back yet.
	tokenReserved
	// tokenHeld: owner holds it, and tokenByOwner[owner] is this token.
	tokenHeld
	// tokenUnavailable: not offered for now, because an owner the cache does not know may hold it.
	// It becomes free again when a release of it applies or the bucket is reloaded. It is set when:
	//   - a grant on it was refused: another owner holds it
	//   - the load found a holder on its token row, but no owner row naming it
	//   - the cache had an owner on it, but that owner was then granted or released another token
	//   - the cache had an owner on it, but its token row names another owner or is missing
	tokenUnavailable
)

// tokenEntry is one token's state.
type tokenEntry struct {
	state tokenState
	// owner is set only when state is tokenHeld.
	owner string
}

// bucketCache is this host's view of a bucket: the state of each token, and which owner holds
// which. It is built from a scan on load and updated after each write this host makes. It may be
// stale; the database's conditional writes decide.
//
// A token moves between states like this:
//
//	free         -> reserved      reserveFreeToken
//	reserved     -> held          grantApplied
//	reserved     -> unavailable   grantTaken
//	reserved     -> free          grantFailed, grantAlreadyHeld
//	held         -> free          releaseApplied, markOwnerHeldTokenFree
//	held         -> unavailable   markOwnerHeldTokenUnavailable, or its owner is granted or released
//	                              from another token
//	unavailable  -> free          releaseApplied
//	any          -> held          grantApplied, grantAlreadyHeld (for the token the owner holds)
//
// The lock is not held during a database write, so an answer can arrive after other answers have
// already changed its token. So each method changes a token only if it is still in the state the
// method expects.
//
// When the cache does not know whether a token is free, it marks it unavailable, not free. Both
// are safe, since the database's conditional write decides, so this is a trade-off:
//   - a wrong free costs a refused write on an acquire's path. An acquire tries only
//     maxGrantAttempts tokens, so a few wrong frees can make it answer NoSlot while tokens are free
//   - a wrong unavailable leaves the token unused until it is released or the bucket is reloaded
//
// Unavailable is chosen to keep refused writes off the acquire path. The cost is capacity, which a
// reload, and later a refresh, gives back.
//
// Each method makes its whole update under mu, and the cache keeps three rules:
//   - tokens has one entry for each token in tokenRange, and no others
//   - tokenByOwner[owner] is tokenID exactly when tokens records owner holding tokenID
//   - freeCount is the number of free entries in tokens
type bucketCache struct {
	// tokenRange is the token ids this bucket owns. It never changes after newBucketCache, so it is
	// read without mu, by the cache and by the manager.
	tokenRange commonsemaphore.TokenRange

	// mu guards every field below.
	mu sync.Mutex
	// tokens holds tokenID's entry at tokens[tokenID-tokenRange.First]. Change entries only through
	// updateTokenLocked, so tokenByOwner and freeCount stay matching.
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
		// Every entry starts as tokenFree, the zero value.
		tokens:       make([]tokenEntry, tokenRange.Count),
		tokenByOwner: make(map[string]int),
		freeCount:    tokenRange.Count,
	}

	// The Locked helpers below run without c.mu. No lock is needed: the cache is not shared until
	// newBucketCache returns.
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
			// This token has an owner
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
			c.updateTokenLocked(row.HeldToken, tokenEntry{state: tokenHeld, owner: row.OwnerID})
		default:
			skipped.unreadable++
		}
	}
	// Applied after every owner row, so an owner row decides who holds its token whatever order
	// the rows came in.
	for _, tokenID := range heldTokenRows {
		if entry, _ := c.lookupTokenLocked(tokenID); entry.state != tokenHeld {
			c.updateTokenLocked(tokenID, tokenEntry{state: tokenUnavailable})
		}
	}
	return c, skipped
}

// lookupHeldToken returns the token the cache records ownerID as holding, if any.
func (c *bucketCache) lookupHeldToken(ownerID string) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tokenID, ok := c.tokenByOwner[ownerID]
	return tokenID, ok
}

// reserveFreeToken marks a random free token reserved, so no other grant picks it while this
// grant's write is in flight. It returns false if no token is free.
func (c *bucketCache) reserveFreeToken() (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.freeCount == 0 {
		return 0, false
	}
	// Walk to the k-th free token, so every free token is equally likely.
	k := rand.Intn(c.freeCount)
	for i := range c.tokens {
		if c.tokens[i].state != tokenFree {
			continue
		}
		if k == 0 {
			tokenID := c.tokenRange.First + i
			c.updateTokenLocked(tokenID, tokenEntry{state: tokenReserved})
			return tokenID, true
		}
		k--
	}
	return 0, false
}

// grantApplied marks tokenID held by ownerID, since the database just granted it to ownerID.
func (c *bucketCache) grantApplied(ownerID string, tokenID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateTokenLocked(tokenID, tokenEntry{state: tokenHeld, owner: ownerID})
}

// grantTaken marks tokenID unavailable if it is still reserved: the database refused the grant
// because another owner holds it. It is not marked held, since the refusal does not name the owner.
// That owner's release frees it (releaseApplied).
func (c *bucketCache) grantTaken(tokenID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.lookupTokenLocked(tokenID); ok && entry.state == tokenReserved {
		c.updateTokenLocked(tokenID, tokenEntry{state: tokenUnavailable})
	}
}

// grantFailed marks tokenID free if it is still reserved: the write returned an error, so it may or
// may not have landed. If it did land, the next grant on the token is refused (grantTaken).
func (c *bucketCache) grantFailed(tokenID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.lookupTokenLocked(tokenID); ok && entry.state == tokenReserved {
		c.updateTokenLocked(tokenID, tokenEntry{state: tokenFree})
	}
}

// grantAlreadyHeld handles a grant the database refused because ownerID already holds heldToken. It
// marks reservedToken free if it is still reserved, and heldToken held by ownerID. If heldToken is
// outside the range, the owner row is corrupt: it returns an error and does not record ownerID.
func (c *bucketCache) grantAlreadyHeld(ownerID string, reservedToken, heldToken int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.lookupTokenLocked(reservedToken); ok && entry.state == tokenReserved {
		c.updateTokenLocked(reservedToken, tokenEntry{state: tokenFree})
	}
	if !c.updateTokenLocked(heldToken, tokenEntry{state: tokenHeld, owner: ownerID}) {
		return fmt.Errorf("owner %v holds token %d, which is outside tokens %d to %d", ownerID, heldToken, c.tokenRange.First, c.tokenRange.Last())
	}
	return nil
}

// releaseApplied marks tokenID free if it is held by ownerID or unavailable: the database just
// released it from ownerID. A token that is reserved or held by another owner was already updated by
// a newer grant, so it is left as it is. ownerID now holds no token, so if the cache records it on
// another token, that token is marked unavailable.
func (c *bucketCache) releaseApplied(ownerID string, tokenID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.lookupTokenLocked(tokenID)
	if !ok {
		return
	}
	if (entry.state == tokenHeld && entry.owner == ownerID) || entry.state == tokenUnavailable {
		c.updateTokenLocked(tokenID, tokenEntry{state: tokenFree})
	}
	// The release also deleted ownerID's owner row, so a different token recorded for ownerID is
	// stale. Unavailable, not free: the database did not say whether anyone else holds it.
	if heldToken, ok := c.tokenByOwner[ownerID]; ok && heldToken != tokenID {
		c.updateTokenLocked(heldToken, tokenEntry{state: tokenUnavailable})
	}
}

// markOwnerHeldTokenFree is called when the cache records ownerID on tokenID, but the token row has
// no holder. It marks tokenID free. It does nothing if the cache no longer records ownerID on
// tokenID.
func (c *bucketCache) markOwnerHeldTokenFree(ownerID string, tokenID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.lookupTokenLocked(tokenID); ok && entry.state == tokenHeld && entry.owner == ownerID {
		c.updateTokenLocked(tokenID, tokenEntry{state: tokenFree})
	}
}

// markOwnerHeldTokenUnavailable is called when the cache records ownerID on tokenID, but the token
// row names another owner or is missing. It marks tokenID unavailable, since it is not known to be
// free. It does nothing if the cache no longer records ownerID on tokenID.
func (c *bucketCache) markOwnerHeldTokenUnavailable(ownerID string, tokenID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.lookupTokenLocked(tokenID); ok && entry.state == tokenHeld && entry.owner == ownerID {
		c.updateTokenLocked(tokenID, tokenEntry{state: tokenUnavailable})
	}
}

// freeTokenCount returns how many tokens are free.
func (c *bucketCache) freeTokenCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.freeCount
}

// ownerCount returns how many owners hold a token.
func (c *bucketCache) ownerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tokenByOwner)
}

// lookupTokenLocked returns tokenID's entry, or false if tokenID is outside the range. The caller
// must hold c.mu.
func (c *bucketCache) lookupTokenLocked(tokenID int) (*tokenEntry, bool) {
	if !c.tokenRange.Contains(tokenID) {
		return nil, false
	}
	return &c.tokens[tokenID-c.tokenRange.First], true
}

// updateTokenLocked sets tokenID's entry to newEntry, and keeps tokenByOwner and freeCount matching:
//   - if tokenID becomes free, freeCount goes up; if it stops being free, freeCount goes down
//   - if tokenID was held, its old owner is removed from tokenByOwner
//   - if newEntry's owner was on another token, that token becomes unavailable: an owner holds one
//     token, so it is no longer the owner's, but it is not known to be free
//
// It returns false, and changes nothing, if tokenID is outside the range. The caller must hold c.mu.
func (c *bucketCache) updateTokenLocked(tokenID int, newEntry tokenEntry) bool {
	entry, ok := c.lookupTokenLocked(tokenID)
	if !ok {
		return false
	}
	// The old entry was free, so it leaves the free count.
	if entry.state == tokenFree {
		c.freeCount--
	}
	// The new entry is free, so it joins the free count.
	if newEntry.state == tokenFree {
		c.freeCount++
	}
	if entry.state == tokenHeld {
		delete(c.tokenByOwner, entry.owner)
	}
	if newEntry.state == tokenHeld {
		// The delete above removed tokenID's own owner, so a token found here is a different one.
		// It came from tokenByOwner, so it is in range and held, and the map entry is overwritten
		// below. Held to unavailable leaves freeCount as it is.
		if prevToken, ok := c.tokenByOwner[newEntry.owner]; ok {
			c.tokens[prevToken-c.tokenRange.First] = tokenEntry{state: tokenUnavailable}
		}
		c.tokenByOwner[newEntry.owner] = tokenID
	}
	*entry = newEntry
	return true
}
