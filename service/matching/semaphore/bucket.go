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
	// scanPageSize bounds one page of the ownership load. A bucket holds at most
	// 2*bucket_size rows: one token row per slot, plus at most one owner row per slot.
	// CreateSemaphore caps bucket_size, so that whole bound fits in one page.
	//
	// The +1 puts the page just above the bound rather than at it. Cassandra returns a
	// paging state whenever it fills a page, so an exact fit would always cost a second,
	// empty round trip. Paging is followed to the end either way, so this size only ever
	// costs round trips, never correctness.
	scanPageSize = 2*persistence.MaxSemaphoreBucketSize + 1

	// maxGrantAttempts caps how many slots one acquire will try before giving up. Each
	// retry costs a conditional write, and only a stale free-set entry causes one, which
	// happens while a bucket has just changed hands. Every miss also drops the id it
	// tried, so the free-set corrects itself within a few attempts; the cap is there so a
	// pathologically stale bucket cannot turn one acquire into hundreds of round trips.
	maxGrantAttempts = 5
)

// GrantOutcome says how one acquire ended. Every value is a result, never an error.
//
// It differs from persistence.SemaphoreGrantOutcome, which reports one conditional write:
// an acquire may take several writes or none. SlotTaken is retried and never reaches the
// caller, while NoSlot and AlreadyHeld can both be decided without a write.
type GrantOutcome int

const (
	// GrantOutcomeUnknown is the zero value and is never returned.
	GrantOutcomeUnknown GrantOutcome = iota
	// GrantOutcomeAcquired means this call claimed a slot; TokenID is the new token.
	GrantOutcomeAcquired
	// GrantOutcomeAlreadyHeld means the owner already had a token, so this call claimed
	// nothing; TokenID is the token it already holds. A retried acquire lands here.
	GrantOutcomeAlreadyHeld
	// GrantOutcomeNoSlot means no slot was available.
	GrantOutcomeNoSlot
)

func (o GrantOutcome) String() string {
	switch o {
	case GrantOutcomeAcquired:
		return "Acquired"
	case GrantOutcomeAlreadyHeld:
		return "AlreadyHeld"
	case GrantOutcomeNoSlot:
		return "NoSlot"
	default:
		return "Unknown"
	}
}

// bucketState gates Grant. A Bucket only moves forward: created to running, or either to
// stopped. Nothing brings a stopped bucket back.
type bucketState int

const (
	bucketStateCreated bucketState = iota
	bucketStateStarting
	bucketStateRunning
	bucketStateStopped
)

// GrantResult is the answer to one acquire. TokenID is set unless Outcome is
// GrantOutcomeNoSlot.
type GrantResult struct {
	Outcome GrantOutcome
	TokenID int
}

// ErrNotReady is returned by Grant when this host cannot answer for the bucket: Start has
// not been called, its scan failed, or the bucket has been stopped. It is an error rather
// than GrantOutcomeNoSlot so a caller can tell "ask somewhere else" from "the semaphore is
// full", which are otherwise indistinguishable.
var ErrNotReady = errors.New("semaphore bucket is not ready")

// Bucket hands out the slots of one semaphore bucket, tracking which are open.
//
// What it holds in memory is only a cache of the bucket's partition; the conditional write
// in persistence is what decides a grant. That is why a stale cache is safe either way. If
// it offers a slot someone else holds, the write rejects it and the grant retries. If it
// has lost track of a slot that is free, grants are turned away that could have succeeded.
// Neither can give one slot to two owners, and neither can lose a grant.
//
// The partition is read once, by Start, and nothing reads it again. A slot this bucket loses
// track of stays lost until the bucket changes hands or the host restarts.
//
// TODO: return a slot to the free-set when its hold is released, and reload the partition
// periodically so slots dropped by a failed write come back without waiting for a handoff.
type Bucket struct {
	id      Identifier
	manager persistence.SemaphoreTokenManager
	logger  log.Logger

	// startupDoneCh is closed when startup ends, whatever the outcome, and Grant waits on it.
	// A new bucket knows of no free slots until Start's scan reads them from the partition,
	// so a grant answered before that would find the free-set empty and report no slot even
	// when every slot is free. startupOnce keeps the close to one, since closing a channel
	// twice panics.
	startupDoneCh chan struct{}
	startupOnce   sync.Once

	// mu guards the state below. It is never held across a persistence call: two
	// concurrent acquires for the same owner are meant to race down to the conditional
	// write, where the loser is told which token it already holds.
	mu sync.Mutex
	// state is guarded by mu rather than an atomic because Start has to check it and install
	// what it loaded as one step. Stop can land while that load is still in flight, and with
	// a separate check Start would go running afterwards and lose the stop.
	state bucketState
	// freeList holds the ids of token rows with no holder; freeIndex maps an id back to its
	// position in freeList. Grant needs a uniform random pick and removal of one named id,
	// both in O(1), and neither structure alone gives both. Keeping both, a removal
	// swaps the tail into the hole freeIndex names and stays O(1).
	freeList  []int
	freeIndex map[int]int
	// held is the owner_id -> token_id reverse index, mirroring the partition's owner rows.
	held map[string]int
}

// NewBucket builds the owner of one bucket. The caller must call Start before Grant, and
// must discard the Bucket if Start returns an error.
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

// markStartupDone releases anyone waiting in Grant. It says startup is over, not that it
// succeeded, so Stop calls it too: a bucket given up on before it finished starting fails
// its callers instead of hanging them.
func (b *Bucket) markStartupDone() {
	b.startupOnce.Do(func() { close(b.startupDoneCh) })
}

// Start takes ownership of the bucket by scanning its partition and building the
// free-set and the reverse index from what is actually stored. It must be called exactly
// once, and the Bucket must be discarded if it returns an error.
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
		// Ownership was given up while the scan was in flight, so these indexes describe a
		// bucket this host no longer serves. Going running now would leave a Bucket whose
		// free-set can never refresh: it hands out slots the real owner has already given
		// away, and every one of those costs a conditional write that cannot apply.
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

// Stop gives up the bucket. Grants that arrive after it report ErrNotReady rather than
// answering from state this host no longer owns.
//
// It is not a barrier: a grant that passed the check before Stop ran can still complete
// its write afterwards. That is safe because the conditional write, not ownership, is what
// keeps a slot from going to two owners.
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

// Grant claims a slot for ownerID:
//
//   - Wait for startup, then refuse the call if the bucket is not usable.
//   - Return the token the owner already holds, if it holds one, so a retry costs no write.
//   - Draw a free id at random and reserve it, so no other acquire on this host draws it too.
//   - Write it conditionally. This is the only step that decides anything; everything before
//     it is an in-memory guess the write either confirms or rejects.
//   - If the write says the slot was taken, drop that id and draw another, up to
//     maxGrantAttempts.
//
// A bucket with no free ids, or one where every attempt lost the race, reports
// GrantOutcomeNoSlot.
//
// TODO: serve waiters first, and enqueue a blocking acquire that finds no slot. Reporting
// GrantOutcomeNoSlot stays for the non-blocking entry point, which skips the wait rather
// than joining the queue.
func (b *Bucket) Grant(ctx context.Context, ownerID string) (GrantResult, error) {
	// Wait for startup to finish before reading any state, since the free-set is empty until
	// the scan fills it.
	select {
	case <-b.startupDoneCh:
		// Startup is over. Whether it left the bucket usable is the isRunning check below.
	case <-ctx.Done():
		// The caller's deadline expired while startup was still running. Returning here keeps
		// a slow scan from holding every acquire past the deadline it asked for.
		return GrantResult{}, ctx.Err()
	}
	if !b.isRunning() {
		return GrantResult{}, ErrNotReady
	}
	if ownerID == "" {
		return GrantResult{}, fmt.Errorf("ownerID is required")
	}

	// A retried acquire routes back to the same bucket, so an earlier hold is recorded here
	// and the call can be answered without a write.
	heldToken, isHeld, holdErr := b.getConfirmedHold(ctx, ownerID)
	if holdErr != nil {
		return GrantResult{}, holdErr
	}
	if isHeld {
		return GrantResult{Outcome: GrantOutcomeAlreadyHeld, TokenID: heldToken}, nil
	}

	for range maxGrantAttempts {
		if err := ctx.Err(); err != nil {
			return GrantResult{}, err
		}

		// Draw a free id and reserve it before the write, so no other acquire on this host
		// can draw the same one.
		tokenID, ok := b.reserve()
		if !ok {
			return GrantResult{Outcome: GrantOutcomeNoSlot}, nil
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
			// An error does not mean the write was rejected: a timeout says the database
			// stopped waiting for acknowledgements, not that anything was rolled back. The
			// id goes back either way, because both cases are safe:
			//
			//   - the write did not land, so the slot really is free;
			//   - the write did land, so the next attempt on this id fails the "only if
			//     free" guard and is reported taken. No second owner can get the slot.
			//
			// Keeping the id out instead would cost a slot per failed write, and an outage
			// would drain the bucket to nothing.
			b.unreserve(tokenID)
			return GrantResult{}, err
		}

		switch resp.Outcome {
		case persistence.SemaphoreGrantApplied:
			// Record the hold.
			// Its durable owner row went in with the same batch.
			b.recordHold(ownerID, tokenID)
			return GrantResult{Outcome: GrantOutcomeAcquired, TokenID: tokenID}, nil

		case persistence.SemaphoreGrantSlotTaken:
			// A stale free-set entry: someone else holds this slot. Keep the id out (it is
			// genuinely not free) and draw another.
			// This is the one outcome worth retrying.
			continue

		case persistence.SemaphoreGrantAlreadyHeld:
			// This owner already holds a token, which the check above missed because the
			// reverse index cache was cold — the window right after the bucket changed hands.
			// Retrying cannot help, since every id would hit the same owner-row conflict, and each
			// attempt would strand another reserved slot. So put this id back and report
			// the token the owner already has.
			//
			// Note the un-reserve is the opposite of the slot-taken branch above: there
			// the id was not free, here it still is. Swapping the two either leaks free
			// slots or offers out held ones.
			b.unreserve(tokenID)
			if resp.HeldToken < 1 {
				// AlreadyHeld without a token names nothing to hand back. Recording it would
				// put a zero in the reverse index, and every later acquire by this owner would
				// fail confirming a token id that cannot exist, with nothing to clear the
				// entry. Refuse the call and leave the index alone.
				return GrantResult{}, fmt.Errorf("semaphore grant reported an already-held slot without a token for bucket %v", b.id)
			}
			b.recordHold(ownerID, resp.HeldToken)
			return GrantResult{Outcome: GrantOutcomeAlreadyHeld, TokenID: resp.HeldToken}, nil

		default:
			// The nosql store rejects an unrecognized outcome before it gets here, so this
			// is unreachable today. It is still checked because that guarantee belongs to
			// one store rather than to the manager interface.
			//
			// An outcome this code cannot read says nothing about whether the slot was
			// claimed, so the id goes back. Keeping it out would lose a slot for good over
			// an unfamiliar response. If the slot was in fact claimed, the next attempt on
			// it fails the "only if free" guard and is reported taken.
			b.unreserve(tokenID)
			return GrantResult{}, fmt.Errorf("unexpected semaphore grant outcome %v for bucket %v", resp.Outcome, b.id)
		}
	}

	// Every attempt found its slot taken. Reporting no slot available is the safe answer:
	// it under-admits, and the caller either retries or (once the queue exists) waits.
	b.logger.Warn("Semaphore grant gave up after repeated stale free-set hits",
		tag.Dynamic("attempts", maxGrantAttempts))
	return GrantResult{Outcome: GrantOutcomeNoSlot}, nil
}

// getConfirmedHold reports the token this owner already holds, which is what a retried
// acquire finds. A false second return means the owner holds nothing here and Grant should
// pick a slot as usual.
//
// The index is only a cache, so a hit is confirmed against the token row before it is
// trusted. Without that check an owner could be told it still holds a slot that was
// released and given to someone else, putting two owners on one slot. The extra read
// happens only when the index has an entry for this owner.
func (b *Bucket) getConfirmedHold(ctx context.Context, ownerID string) (int, bool, error) {
	b.mu.Lock()
	tokenID, ok := b.held[ownerID]
	b.mu.Unlock()
	if !ok {
		return 0, false, nil
	}

	resp, err := b.manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID:      b.id.DomainID,
		SemaphoreName: b.id.SemaphoreName,
		Bucket:        b.id.Bucket,
		TokenID:       tokenID,
	})
	if err != nil {
		var notExists *types.EntityNotExistsError
		if !errors.As(err, &notExists) {
			return 0, false, err
		}
		// Token rows are seeded once and never deleted, so a missing one means the index
		// named a slot this bucket does not own. Drop the entry and fall through to a
		// normal pick.
		b.dropStaleHold(ownerID, tokenID, false)
		return 0, false, nil
	}

	ownership := resp.Ownership
	// The row agrees with the index, so the owner really does hold this slot.
	if ownership != nil && ownership.Holder == ownerID {
		return tokenID, true, nil
	}

	// The index is stale. Drop the entry, and return the slot to the free-set only if the
	// row says it really is unheld — if another owner has it now, adding it back would
	// offer out a held slot.
	stillFree := ownership != nil && ownership.Holder == ""
	b.dropStaleHold(ownerID, tokenID, stillFree)
	return 0, false, nil
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
				// set it. Skipping is safe in both directions: a dropped token row costs one
				// slot for as long as this host owns the bucket, and a dropped owner row
				// leaves the conditional write to catch the duplicate. Counted rather than
				// logged per row, because either cause makes every row in the bucket match
				// here.
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

	// A slot claimed by an owner row is not free, whatever its token row said. The two can
	// disagree because a scan is not a snapshot: a grant landing mid-scan can be missed on
	// the token row and seen on the owner row. Reconciling them here just keeps the two
	// indexes agreeing on the pages we did read; repairing the table itself is a separate
	// job. Doing it after the loop rather than inside keeps it independent of the order
	// the two row types come back in.
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
	b.removeFreeLocked(tokenID)
	return tokenID, true
}

// unreserve puts a reserved id back, for a grant that drew it but did not take it. The id
// may in fact be held, when the write's outcome is unknown. That is safe because the next
// attempt on it is settled by the conditional write, not by this set.
func (b *Bucket) unreserve(tokenID int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addFreeLocked(tokenID)
}

// recordHold marks ownerID as holding tokenID, mirroring the owner row the write just put
// down. Apart from the startup load, this is the only place the reverse index grows.
func (b *Bucket) recordHold(ownerID string, tokenID int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.held[ownerID] = tokenID
	// A held slot is never in the free-set. It normally is not here anyway, since the
	// grant reserved it; the already-held case reports a token this host never drew.
	b.removeFreeLocked(tokenID)
}

// dropStaleHold removes a reverse-index entry the caller has found to be wrong, and puts its
// slot back in the free-set if stillFree says the token row proved the slot unheld.
//
// This is not the undo of recordHold: nothing was released here, the index was simply out of
// date. Releasing a hold is a separate job with its own durable write.
func (b *Bucket) dropStaleHold(ownerID string, tokenID int, stillFree bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// The caller read tokenID from this map and then released the lock to check the row, so
	// this owner may have been granted a different token since. Deleting that would throw
	// away a valid hold, so only delete while the entry still names tokenID.
	if current, ok := b.held[ownerID]; ok && current == tokenID {
		delete(b.held, ownerID)
	}
	if stillFree {
		b.addFreeLocked(tokenID)
	}
}

// addFreeLocked puts an id back in the free-set, ignoring one that is already there. That
// check keeps freeList free of duplicates, which would otherwise let two grants draw the
// same slot.
func (b *Bucket) addFreeLocked(tokenID int) {
	if _, ok := b.freeIndex[tokenID]; ok {
		return
	}
	b.freeIndex[tokenID] = len(b.freeList)
	b.freeList = append(b.freeList, tokenID)
}

// removeFreeLocked takes one id out in constant time by moving the tail element into its
// slot. Order in freeList carries no meaning, since picks are random.
func (b *Bucket) removeFreeLocked(tokenID int) {
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

// freeCount reports how many slots the bucket believes are open. It is a hint, not the
// truth, and exists for tests and metrics.
func (b *Bucket) freeCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.freeList)
}
