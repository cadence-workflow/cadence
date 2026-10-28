package semaphore

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"

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

// bucketCache is this host's view of a bucket. It may be stale; the database has the final say.
//
// Two kinds of lock:
//   - token lock (tokenMutexes): one per token. A lock method takes it and returns a lockedToken. The
//     caller holds it across that token's database call, marks the answer, then unlocks.
//   - cache lock (mu): one for the whole cache, guarding the entries. Taken and let go inside one
//     method; never held across a database call or while waiting.
//
// Rules:
//   - Take a token lock before the cache lock, never after. The exception is tryLockFreeToken, which
//     takes one under the cache lock with TryLock, which never waits.
//   - Hold at most one token lock. A caller defers unlock right after getting a lockedToken, and
//     takes no other token lock before returning.
//   - Only the holder of a token's lock changes that token. The exception is marking an owner held,
//     which marks its previous token unavailable (see updateTokenLocked).
//
// Token ids from the database are range-checked where they enter; inside the cache every id is in
// the range.
type bucketCache struct {
	// tokenRange is the ids of the bucket's tokens. Set once by newBucketCache.
	tokenRange commonsemaphore.TokenRange
	// tokenMutexes are the token locks, one per token, in token id order. Set once by newBucketCache.
	tokenMutexes []locks.Mutex

	// mu is the cache lock.
	mu sync.Mutex
	// tokens is the token list, one entry per token in token id order. Change it only through
	// updateTokenLocked.
	tokens []tokenEntry
	// tokenByOwner maps each owner to the token it holds.
	tokenByOwner map[string]int
	freeCount    int
}

// lockedToken is a token whose lock the caller holds.
type lockedToken struct {
	cache   *bucketCache
	tokenID int
	// status becomes unlockCalled on the first unlock, so a second one cannot let go of a lock another
	// call has taken since.
	status int32
}

const (
	unlockNotCalled int32 = 0
	unlockCalled    int32 = 1
)

// skippedRows counts the scanned rows newBucketCache did not use.
type skippedRows struct {
	unreadable int
	outOfRange int
}

// newBucketCache builds the cache from a bucket scan. A token is held if an owner row names it,
// unavailable if only its token row has a holder, and free otherwise. Unreadable and out-of-range
// rows are skipped and counted.
func newBucketCache(tokenRange commonsemaphore.TokenRange, rows []*persistence.SemaphoreOwnership) (*bucketCache, skippedRows) {
	bc := &bucketCache{
		tokenRange:   tokenRange,
		tokenMutexes: make([]locks.Mutex, tokenRange.Count),
		// Every entry starts as tokenFree, the zero value.
		tokens:       make([]tokenEntry, tokenRange.Count),
		tokenByOwner: make(map[string]int),
		freeCount:    tokenRange.Count,
	}
	for i := range bc.tokenMutexes {
		bc.tokenMutexes[i] = locks.NewMutex()
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
			bc.updateTokenLocked(row.HeldToken, tokenEntry{state: tokenHeld, owner: row.OwnerID})
		default:
			skipped.unreadable++
		}
	}
	// Applied after every owner row, so an owner row decides who holds its token whatever order
	// the rows came in.
	for _, tokenID := range heldTokenRows {
		if bc.lookupTokenLocked(tokenID).state != tokenHeld {
			bc.updateTokenLocked(tokenID, tokenEntry{state: tokenUnavailable})
		}
	}
	return bc, skipped
}

// tryLockFreeToken locks a random free token that no other call holds and returns its lockedToken. It
// returns nil if there is no such token.
func (bc *bucketCache) tryLockFreeToken() *lockedToken {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.freeCount == 0 {
		return nil
	}
	// Start at a random free token, so each free token is equally likely when none is locked.
	start, k := 0, rand.Intn(bc.freeCount)
	for i := range bc.tokens {
		if bc.tokens[i].state != tokenFree {
			continue
		}
		if k == 0 {
			start = i
			break
		}
		k--
	}
	for n := range bc.tokens {
		i := (start + n) % len(bc.tokens)
		// A free token can be locked by a grant writing it, or by a call that has just freed it.
		if bc.tokens[i].state == tokenFree && bc.tokenMutexes[i].TryLock() {
			return &lockedToken{cache: bc, tokenID: bc.tokenRange.First + i}
		}
	}
	return nil
}

// lockHeldToken locks ownerID's cached token, waiting until another holder unlocks it or the ctx
// deadline passes. It returns nil if ownerID has no cached token, or lost it before the lock.
func (bc *bucketCache) lockHeldToken(ctx context.Context, ownerID string) (*lockedToken, error) {
	tokenID, ok := bc.lookupHeldToken(ownerID)
	if !ok {
		return nil, nil
	}
	lockedToken, err := bc.lockToken(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	// Another call may have changed ownerID's token between the lookup and taking the lock
	if heldTokenID, ok := bc.lookupHeldToken(ownerID); !ok || heldTokenID != tokenID {
		lockedToken.unlock()
		return nil, nil
	}
	return lockedToken, nil
}

// lockToken locks tokenID, waiting until another holder unlocks it or the ctx deadline passes.
// It returns an error only if the deadline passes.
func (bc *bucketCache) lockToken(ctx context.Context, tokenID int) (*lockedToken, error) {
	if err := bc.tokenMutexes[bc.index(tokenID)].Lock(ctx); err != nil {
		return nil, err
	}
	return &lockedToken{cache: bc, tokenID: tokenID}, nil
}

// tryLockToken locks tokenID without waiting. It returns nil if another call holds it.
func (bc *bucketCache) tryLockToken(tokenID int) *lockedToken {
	if !bc.tokenMutexes[bc.index(tokenID)].TryLock() {
		return nil
	}
	return &lockedToken{cache: bc, tokenID: tokenID}
}

// lookupHeldToken returns the token the cache records ownerID as holding, or false if none.
func (bc *bucketCache) lookupHeldToken(ownerID string) (int, bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	tokenID, ok := bc.tokenByOwner[ownerID]
	return tokenID, ok
}

// freeTokenCount returns how many tokens are free.
func (bc *bucketCache) freeTokenCount() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.freeCount
}

// heldTokenCount returns how many tokens are held.
func (bc *bucketCache) heldTokenCount() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return len(bc.tokenByOwner)
}

// updateTokenLocked sets tokenID's entry to newEntry and updates tokenByOwner and freeCount to
// match. The caller holds mu.
func (bc *bucketCache) updateTokenLocked(tokenID int, newEntry tokenEntry) {
	oldEntry := bc.lookupTokenLocked(tokenID)
	if *oldEntry == newEntry {
		return
	}
	if oldEntry.state == tokenFree {
		bc.freeCount--
	}
	if newEntry.state == tokenFree {
		bc.freeCount++
	}
	if oldEntry.state == tokenHeld {
		delete(bc.tokenByOwner, oldEntry.owner)
	}
	if newEntry.state == tokenHeld {
		// The owner holds one token, so a record of this owner on another token is out of date. That
		// token becomes unavailable, not free, since the database has not said it is free. Its lock
		// is not taken (a call holds one token lock at a time). A call holding it records what the
		// database says for that token afterwards, which replaces this.
		if otherToken, ok := bc.tokenByOwner[newEntry.owner]; ok && otherToken != tokenID {
			bc.tokens[bc.index(otherToken)] = tokenEntry{state: tokenUnavailable}
		}
		bc.tokenByOwner[newEntry.owner] = tokenID
	}
	*oldEntry = newEntry
}

// lookupTokenLocked returns tokenID's entry. The caller holds mu.
func (bc *bucketCache) lookupTokenLocked(tokenID int) *tokenEntry {
	return &bc.tokens[bc.index(tokenID)]
}

// index returns tokenID's position in tokens and tokenMutexes.
func (bc *bucketCache) index(tokenID int) int {
	return tokenID - bc.tokenRange.First
}

// id returns the locked token's id.
func (lt *lockedToken) id() int {
	return lt.tokenID
}

// unlock lets go of the token's lock. Only the first call does anything.
func (lt *lockedToken) unlock() {
	if !atomic.CompareAndSwapInt32(&lt.status, unlockNotCalled, unlockCalled) {
		return
	}
	lt.cache.tokenMutexes[lt.cache.index(lt.tokenID)].Unlock()
}

// markHeld marks the token as held by ownerID.
func (lt *lockedToken) markHeld(ownerID string) {
	lt.mark(tokenEntry{state: tokenHeld, owner: ownerID})
}

// markFree marks the token as free.
func (lt *lockedToken) markFree() {
	lt.mark(tokenEntry{state: tokenFree})
}

// markUnavailable marks the token as unavailable.
func (lt *lockedToken) markUnavailable() {
	lt.mark(tokenEntry{state: tokenUnavailable})
}

// mark sets the token's entry to newEntry. It panics after unlock, since the token may then be
// locked by another call.
func (lt *lockedToken) mark(newEntry tokenEntry) {
	if atomic.LoadInt32(&lt.status) == unlockCalled {
		panic(fmt.Sprintf("token %d marked after unlock", lt.tokenID))
	}
	bc := lt.cache
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.updateTokenLocked(lt.tokenID, newEntry)
}
