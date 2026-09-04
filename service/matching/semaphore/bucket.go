package semaphore

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	// scanPageSize sizes one page of the ownership load. A bucket holds at most 2*bucket_size
	// rows -- a token row per slot, plus at most one owner row -- and CreateSemaphore caps
	// bucket_size, so one page covers a whole bucket. The +1 keeps the page just above that
	// bound: Cassandra returns a paging state whenever a page fills exactly, and that extra
	// round trip finds nothing. Paging is followed to the end regardless, so this number costs
	// round trips, never correctness.
	scanPageSize = 2*persistence.MaxSemaphoreBucketSize + 1

	// maxGrantAttempts caps how many slots one acquire tries before giving up. Only a stale
	// free-set entry costs a retry, and every miss drops the id it tried, so the free-set
	// corrects itself within a few attempts. The cap bounds the worst case: a badly stale
	// bucket cannot turn one acquire into hundreds of conditional writes.
	maxGrantAttempts = 5
)

// AcquireOutcome says how one acquire ended. Every value is a result, never an error.
//
// It is not persistence.SemaphoreGrantOutcome, which reports one conditional write. An acquire
// may take several writes, or none.
type AcquireOutcome int

const (
	// AcquireOutcomeUnknown is the zero value and is never returned.
	AcquireOutcomeUnknown AcquireOutcome = iota
	// AcquireOutcomeAcquired means this call claimed a token slot; TokenID is the new token.
	AcquireOutcomeAcquired
	// AcquireOutcomeAlreadyHeld means the owner already had a token, so this call claimed
	// nothing; TokenID is the token it already holds. A retried acquire lands here.
	AcquireOutcomeAlreadyHeld
	// AcquireOutcomeNoSlot means no token, because this host has no free slot left to try and
	// nothing in this call said its in-memory free-set was wrong. Most likely the bucket really
	// is full, but not certainly: the free-set is built when the bucket loads and only loses
	// slots from there, so a slot released since then is missing from it.
	AcquireOutcomeNoSlot
	// AcquireOutcomeContended also means no token, but for a different reason: every slot this
	// call tried turned out to be held by someone else. That proves the free-set is out of
	// date, so unlike NoSlot it says nothing about whether the bucket is full -- free slots may
	// be missing from it too. Re-read the partition before acting on it, and never
	// report the semaphore as full or park a waiter on it.
	AcquireOutcomeContended
)

// String names the outcome for logs and error messages.
func (o AcquireOutcome) String() string {
	switch o {
	case AcquireOutcomeAcquired:
		return "Acquired"
	case AcquireOutcomeAlreadyHeld:
		return "AlreadyHeld"
	case AcquireOutcomeNoSlot:
		return "NoSlot"
	case AcquireOutcomeContended:
		return "Contended"
	default:
		return "Unknown"
	}
}

// AcquireResult is the answer to one acquire. TokenID is set unless Outcome is one of the
// two no-token outcomes, AcquireOutcomeNoSlot or AcquireOutcomeContended.
type AcquireResult struct {
	Outcome AcquireOutcome
	TokenID int
}

// bucketState gates Acquire. A Bucket only moves forward: created to running, or either to
// stopped. Nothing brings a stopped bucket back.
type bucketState int

const (
	bucketStateCreated bucketState = iota
	bucketStateStarting
	bucketStateRunning
	bucketStateStopped
)

// ErrNotReady means this host cannot answer for the bucket -- not started, scan failed, or
// stopped. An error, not an AcquireOutcome, so it cannot be read as an answer about the slots.
var ErrNotReady = errors.New("semaphore bucket is not ready")

// Bucket hands out the slots of one semaphore bucket, tracking which are open.
//
// Its free-set is only a cache; the conditional write in persistence decides every grant.
// Start reads the partition once and never again, so a lost slot stays lost until the bucket
// is loaded afresh.
type Bucket struct {
	id      Identifier
	manager persistence.SemaphoreTokenManager
	logger  log.Logger

	// startupDoneCh is closed when startup ends, whatever the outcome, and Acquire waits on it.
	// startupOnce keeps the close to one, since closing twice panics.
	startupDoneCh chan struct{}
	startupOnce   sync.Once

	// mu guards the state below, never held across a persistence call: concurrent acquires
	// for one owner must race to the conditional write.
	mu sync.Mutex
	// state is the bucket's current lifecycle stage, and Acquire serves only while it is running.
	// mu guards it rather than an atomic so Start can check it and install its scan result as
	// one step; otherwise a Stop landing mid-load would be lost.
	state bucketState
	// freeList holds the ids of token rows with no holder; freeIndex maps an id back to its
	// position in freeList. A grant needs both a uniform random pick and removal of one named
	// id in O(1): freeIndex names the hole, and the tail swaps into it.
	freeList  []int
	freeIndex map[int]int
	// held is the owner_id -> token_id reverse index, mirroring the partition's owner rows.
	held map[string]int
}

// NewBucket builds the owner of one bucket. Call Start before Acquire, and discard the Bucket
// if Start returns an error.
func NewBucket(
	id Identifier,
	manager persistence.SemaphoreTokenManager,
	logger log.Logger,
) *Bucket {
	b := &Bucket{
		id:            id,
		manager:       manager,
		logger:        logger.WithTags(tag.Dynamic("semaphore-bucket", id.String())),
		startupDoneCh: make(chan struct{}),
		freeIndex:     make(map[int]int),
		held:          make(map[string]int),
	}
	return b
}

// markStartupDone releases everything waiting for startup. It says startup ended, not that it
// succeeded, so Stop calls it too: an acquire on a bucket that never started gets ErrNotReady
// rather than blocking forever.
func (b *Bucket) markStartupDone() {
	b.startupOnce.Do(func() { close(b.startupDoneCh) })
}

// Start scans the bucket's partition and builds the free-set and the reverse index from what
// is stored there. Call it exactly once, and discard the Bucket if it returns an error.
func (b *Bucket) Start(ctx context.Context) error {
	defer b.markStartupDone()

	// Mark the bucket as starting before the scan, so a second Start fails here. Two loads
	// running at once would both scan, and whichever finished last would replace the
	// other's indexes, losing every hold recorded in between.
	b.mu.Lock()
	if b.state != bucketStateCreated {
		b.mu.Unlock()
		return fmt.Errorf("semaphore bucket %v has already been started or stopped", b.id)
	}
	b.state = bucketStateStarting
	b.mu.Unlock()

	freeList, freeIndex, held, err := b.loadTokenOwnership(ctx)
	if err != nil {
		return fmt.Errorf("load semaphore bucket %v: %w", b.id, err)
	}

	b.mu.Lock()
	if b.state == bucketStateStopped {
		b.mu.Unlock()
		// Stop landed during the scan, so this host no longer owns the bucket and the scan
		// result is already stale.
		return fmt.Errorf("semaphore bucket %v was stopped while it was loading", b.id)
	}
	b.freeList, b.freeIndex, b.held = freeList, freeIndex, held
	b.state = bucketStateRunning
	// Counted here rather than off the locals below: the assignment above aliases them into
	// the bucket, so reading their length after the unlock would race a concurrent grant.
	freeSlots, heldSlots := len(b.freeList), len(b.held)
	b.mu.Unlock()

	b.logger.Info("Semaphore bucket owner started",
		tag.LifeCycleStarted,
		tag.Dynamic("free-slots", freeSlots),
		tag.Dynamic("held-slots", heldSlots),
	)
	return nil
}

// Stop gives up the bucket: acquires arriving after it report ErrNotReady rather than
// answering from state this host no longer owns.
//
// It is not a barrier -- a grant already past the state check can still finish its write. That
// is safe: the conditional write, not ownership, keeps one slot from reaching two owners.
func (b *Bucket) Stop() {
	b.mu.Lock()
	b.state = bucketStateStopped
	b.mu.Unlock()
	b.markStartupDone()
	b.logger.Info("Semaphore bucket owner stopped", tag.LifeCycleStopped)
}

func (b *Bucket) isRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state == bucketStateRunning
}

// Acquire asks the bucket for a slot on behalf of ownerID, and it's the entry point.
//
// TODO: re-read the partition and retry on Contended, and serve queued waiters before new
// callers so the queue cannot be jumped.
func (b *Bucket) Acquire(ctx context.Context, ownerID string) (AcquireResult, error) {
	// Wait for startup to finish before reading any state, since the free-set is empty until
	// the scan fills it.
	select {
	case <-b.startupDoneCh:
		// Startup is over. Whether it left the bucket usable is the isRunning check below.
	case <-ctx.Done():
		// The caller's deadline expired while startup was still running. Returning here keeps
		// a slow scan from holding every acquire past the deadline it asked for.
		return AcquireResult{}, ctx.Err()
	}
	if !b.isRunning() {
		return AcquireResult{}, ErrNotReady
	}
	if ownerID == "" {
		return AcquireResult{}, fmt.Errorf("ownerID is required")
	}

	res, err := b.grant(ctx, ownerID)
	if err != nil {
		return AcquireResult{}, err
	}

	// A full bucket is the one answer a caller can wait out, so it is the only one that becomes
	// a waiter. Contended is never queued: the slots such a waiter would be waiting for may not
	// be held by anyone, so no release is coming to wake it.
	if res.Outcome == AcquireOutcomeNoSlot {
		if err := b.enqueue(ctx, ownerID); err != nil {
			return AcquireResult{}, err
		}
		return AcquireResult{Outcome: AcquireOutcomeNoSlot}, nil
	}
	return res, nil
}

// enqueue records ownerID as a waiter on this bucket, to be granted a slot when one frees.
// Acquire calls it when a grant finds the bucket full.
//
// TODO: allocate a task id from the range this host holds and write the waiter row to
// semaphore_tasks.
func (b *Bucket) enqueue(ctx context.Context, ownerID string) error {
	return nil
}

// grant makes one attempt at a slot for ownerID:
//
//   - Check the reverse index and return the token the owner already holds.
//   - Draw a free id at random and settle it with a conditional write. Only the write decides.
//   - A slot the write reports taken is dropped, and another drawn, up to maxGrantAttempts.
//
// It answers with one of four outcomes:
//
//   - Acquired: the write applied, and TokenID is the new token.
//   - AlreadyHeld: the owner already had a token, and TokenID is that token.
//   - NoSlot: no free id was left to try, and nothing contradicted the free-set.
//   - Contended: every id tried turned out to be held, so the free-set is stale. Unlike NoSlot,
//     this says nothing about whether the bucket is full.
func (b *Bucket) grant(ctx context.Context, ownerID string) (AcquireResult, error) {
	// Check the reverse index first, to see whether this owner already holds a token.
	// A retried acquire is deduped here, and costs no write.
	b.mu.Lock()
	tokenID, ok := b.held[ownerID]
	b.mu.Unlock()

	if ok {
		stillHeld, err := b.confirmHold(ctx, ownerID, tokenID)
		if err != nil {
			return AcquireResult{}, err
		}
		if stillHeld {
			return AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: tokenID}, nil
		}
	}

	// True once a write reports a supposedly-free slot as held, which proves the free-set wrong.
	freeSetWasWrong := false

	for range maxGrantAttempts {
		// Stop as soon as the caller gives up. Letting the write fail instead reports a
		// persistence.TimeoutError, which hides whether the caller ran out of time or the store did.
		if err := ctx.Err(); err != nil {
			return AcquireResult{}, err
		}

		// Draw a free id and reserve it before the write
		tokenID, ok := b.reserve()
		if !ok {
			return AcquireResult{Outcome: noTokenOutcome(freeSetWasWrong)}, nil
		}

		// The conditional batch write, the one authoritative step. A write that does not
		// apply comes back as an outcome, not an error.
		resp, err := b.manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
			DomainID:      b.id.DomainID,
			SemaphoreName: b.id.SemaphoreName,
			Bucket:        b.id.Bucket,
			TokenID:       tokenID,
			OwnerID:       ownerID,
		})
		if err != nil {
			// An error does not mean the write was rejected. Keeping the id
			// out instead would cost a slot per failed write, and an outage
			// would drain the bucket to nothing.
			b.unreserve(tokenID)
			return AcquireResult{}, err
		}

		switch resp.Outcome {
		case persistence.SemaphoreGrantApplied:
			// Record the hold.
			b.recordHold(ownerID, tokenID)
			return AcquireResult{Outcome: AcquireOutcomeAcquired, TokenID: tokenID}, nil

		case persistence.SemaphoreGrantSlotTaken:
			// A stale free-set entry: someone else holds this slot.
			// Keep the id out (it is genuinely not free) and draw another.
			// This is the one outcome worth retrying.
			freeSetWasWrong = true
			continue

		case persistence.SemaphoreGrantAlreadyHeld:
			// This owner already holds a token, put this id back and report the token the owner already has.
			b.unreserve(tokenID)
			if resp.HeldToken < 1 {
				// AlreadyHeld without a token names nothing to hand back. Recording it would
				// put a zero in the reverse index, and every later acquire by this owner would
				// fail confirming a token id that cannot exist.
				return AcquireResult{}, fmt.Errorf("semaphore grant reported an already-held slot without a token for bucket %v", b.id)
			}
			b.recordHold(ownerID, resp.HeldToken)
			return AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: resp.HeldToken}, nil

		default:
			// The nosql store rejects an unrecognized outcome before it gets here, so this
			// is unreachable today. It is still checked because that guarantee belongs to
			// one store rather than to the manager interface.
			//
			// An outcome this code cannot read says nothing about whether the slot was
			// claimed, so the id goes back.
			b.unreserve(tokenID)
			return AcquireResult{}, fmt.Errorf("unexpected semaphore grant outcome %v for bucket %v", resp.Outcome, b.id)
		}
	}

	// Every attempt found its slot taken, so the free-set is known to be stale and the
	// answer is Contended by construction. Under-admitting is the safe direction, and the
	// caller re-reads the partition before deciding what to do about it.
	b.logger.Warn("Semaphore grant gave up after repeated stale free-set hits",
		tag.Dynamic("attempts", maxGrantAttempts))
	return AcquireResult{Outcome: AcquireOutcomeContended}, nil
}

// noTokenOutcome picks between the two no-token answers. A write that reported its slot taken
// is proof the free-set is wrong, which makes every other entry in it suspect too.
func noTokenOutcome(freeSetWasWrong bool) AcquireOutcome {
	if freeSetWasWrong {
		return AcquireOutcomeContended
	}
	return AcquireOutcomeNoSlot
}

// confirmHold checks whether ownerID still holds tokenID by reading the token row.
// A stale entry is dropped, and its slot returned to the free-set when the row proves the slot unheld.
func (b *Bucket) confirmHold(ctx context.Context, ownerID string, tokenID int) (bool, error) {
	resp, err := b.manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID:      b.id.DomainID,
		SemaphoreName: b.id.SemaphoreName,
		Bucket:        b.id.Bucket,
		TokenID:       tokenID,
	})
	if err != nil {
		var notExists *types.EntityNotExistsError
		if !errors.As(err, &notExists) {
			return false, err
		}
		// Token rows are seeded once and never deleted, so a missing one means the index
		// named a slot this bucket does not own. Drop the entry and fall through to a
		// normal pick.
		b.dropStaleHold(ownerID, tokenID, false)
		return false, nil
	}

	ownership := resp.Ownership
	// The row agrees with the index, so the owner really does hold this slot.
	if ownership != nil && ownership.Holder == ownerID {
		return true, nil
	}

	// The index is stale. Drop the entry, and return the slot to the free-set only if the
	// row says it really is unheld — if another owner has it now, adding it back would
	// offer out a held slot.
	stillFree := ownership != nil && ownership.Holder == ""
	b.dropStaleHold(ownerID, tokenID, stillFree)
	return false, nil
}

// loadTokenOwnership reads the bucket's partition, following NextPageToken to the end, and
// returns which slots are free and which owner holds what. It builds into locals, so a read
// that fails partway leaves the live state untouched.
func (b *Bucket) loadTokenOwnership(ctx context.Context) ([]int, map[int]int, map[string]int, error) {
	free := make(map[int]struct{})
	held := make(map[string]int)
	var skipped int

	var pageToken []byte
	for {
		resp, err := b.manager.ScanSemaphoreBucket(ctx, &persistence.ScanSemaphoreBucketRequest{
			DomainID:      b.id.DomainID,
			SemaphoreName: b.id.SemaphoreName,
			Bucket:        b.id.Bucket,
			PageSize:      scanPageSize,
			NextPageToken: pageToken,
		})
		if err != nil {
			return nil, nil, nil, err
		}

		for _, row := range resp.Ownerships {
			switch row.RowType {
			case persistence.SemaphoreRowTypeToken:
				// Holder is empty exactly when the slot is unheld.
				if row.Holder == "" && row.TokenID > 0 {
					free[row.TokenID] = struct{}{}
				}
			case persistence.SemaphoreRowTypeOwner:
				if row.OwnerID != "" && row.HeldToken > 0 {
					held[row.OwnerID] = row.HeldToken
				}
			default:
				// Either a type a newer version wrote, or the zero value because nothing
				// set it.
				skipped++
			}
		}

		pageToken = resp.NextPageToken
		if len(pageToken) == 0 {
			break
		}
	}

	if skipped > 0 {
		b.logger.Warn("Skipped semaphore rows of unknown type while loading bucket state",
			tag.Dynamic("skipped-rows", skipped))
	}

	// A slot claimed by an owner row is not free, whatever its token row said.
	// The two can disagree because a scan is not a snapshot. Reconciling them here
	// just keeps the two indexes agreeing on the pages we did read
	for _, tokenID := range held {
		delete(free, tokenID)
	}

	freeList := make([]int, 0, len(free))
	freeIndex := make(map[int]int, len(free))
	for tokenID := range free {
		freeIndex[tokenID] = len(freeList)
		freeList = append(freeList, tokenID)
	}
	return freeList, freeIndex, held, nil
}

// reserve draws a uniform-random free id and takes it out of the free-set.
func (b *Bucket) reserve() (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.freeList) == 0 {
		return 0, false
	}
	tokenID := b.freeList[rand.Intn(len(b.freeList))]
	b.removeFromFreeSetLocked(tokenID)
	return tokenID, true
}

// unreserve puts back an id a grant drew but did not take. It may in fact be held, when the
// write's outcome is unknown -- safe, because the conditional write settles the next attempt.
func (b *Bucket) unreserve(tokenID int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addToFreeSetLocked(tokenID)
}

// recordHold marks ownerID as holding tokenID, mirroring the owner row the write just put
// down. Apart from the startup load, this is the only place the reverse index grows.
func (b *Bucket) recordHold(ownerID string, tokenID int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.held[ownerID] = tokenID
	// A held slot is never in the free-set. It normally is not here anyway, since the grant
	// reserved it; the already-held case reports a token this host never drew.
	b.removeFromFreeSetLocked(tokenID)
}

// dropStaleHold removes a reverse-index entry, returning its slot
// to the free-set when stillFree says the token row proved the slot unheld.
func (b *Bucket) dropStaleHold(ownerID string, tokenID int, stillFree bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if current, ok := b.held[ownerID]; ok && current == tokenID {
		delete(b.held, ownerID)
	}
	if stillFree {
		b.addToFreeSetLocked(tokenID)
	}
}

// addToFreeSetLocked puts an id back, ignoring one already there. That check keeps freeList free
// of duplicates, which would otherwise let two grants draw the same slot.
func (b *Bucket) addToFreeSetLocked(tokenID int) {
	if _, ok := b.freeIndex[tokenID]; ok {
		return
	}
	b.freeIndex[tokenID] = len(b.freeList)
	b.freeList = append(b.freeList, tokenID)
}

// removeFromFreeSetLocked takes one id out in constant time by moving the tail element into its
// slot. Order in freeList carries no meaning, since picks are random.
func (b *Bucket) removeFromFreeSetLocked(tokenID int) {
	i, ok := b.freeIndex[tokenID]
	if !ok {
		return
	}
	last := len(b.freeList) - 1
	moved := b.freeList[last]
	b.freeList[i] = moved
	b.freeIndex[moved] = i
	b.freeList = b.freeList[:last]
	// When the removed id was itself the tail, moved == tokenID and the line above just
	// re-added the entry being removed, so this delete has to come last.
	delete(b.freeIndex, tokenID)
}

// freeCount reports how many slots the bucket believes are open. It is a hint, not the truth,
// and exists for tests.
func (b *Bucket) freeCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.freeList)
}
