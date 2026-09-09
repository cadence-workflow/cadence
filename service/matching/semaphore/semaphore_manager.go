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
	// scanPageSize sizes one page of the startup scan. A manager owns one bucket, and a bucket is
	// a single Cassandra partition holding one token row per slot plus one owner row per hold.
	// With every slot held that is 2*bucket_size rows, and CreateSemaphore caps bucket_size, so
	// one page covers the whole partition.
	//
	// The +1 avoids a wasted round trip: a page that fills exactly comes back with a paging
	// state, so the scan would fetch an empty second page. Paging runs to the end regardless,
	// so this number only ever costs round trips, never correctness.
	scanPageSize = 2*persistence.MaxSemaphoreBucketSize + 1

	// maxGrantAttempts caps how many slots one acquire tries. Only a stale free-set entry costs
	// a retry, and each miss drops the id it tried, so the cap keeps a badly stale bucket from
	// turning one acquire into hundreds of conditional writes.
	maxGrantAttempts = 3
)

// AcquireOutcome says how one acquire ended. Every value is a result, never an error.
type AcquireOutcome int

const (
	// AcquireOutcomeUnknown is the zero value. It only ever appears alongside an error.
	AcquireOutcomeUnknown AcquireOutcome = iota
	// AcquireOutcomeAcquired means this call claimed a token slot, tokenID is the new token.
	AcquireOutcomeAcquired
	// AcquireOutcomeAlreadyHeld means the owner already had a token
	// TokenID is the token it already holds. A retried acquire lands here.
	AcquireOutcomeAlreadyHeld
	// AcquireOutcomeNoSlot means no token: this host found no free slot to take.
	AcquireOutcomeNoSlot
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
	default:
		return "Unknown"
	}
}

// AcquireResult is the answer to one acquire. TokenID is set unless Outcome is
// AcquireOutcomeNoSlot.
type AcquireResult struct {
	Outcome AcquireOutcome
	TokenID int
}

// ErrNotReady means this host cannot answer for the bucket: not started, scan failed, or stopped.
var ErrNotReady = errors.New("semaphore manager is not ready")

// ErrInvalidRequest means the request itself is wrong. Never retryable, fix the call.
var ErrInvalidRequest = errors.New("invalid semaphore request")

// ErrInconsistentState means storage returned something that should be impossible. Never
// retryable: it is a bug, not contention.
var ErrInconsistentState = errors.New("semaphore state is inconsistent")

// managerState gates Acquire. A Manager only moves forward: created to running, or either to
// stopped. Nothing brings a stopped manager back.
type managerState int

const (
	managerStateCreated managerState = iota
	managerStateStarting
	managerStateRunning
	managerStateStopped
)

// Manager hands out the slots of one semaphore token bucket, tracking which are open.
//
// Its free-set is only a cache; the conditional write in persistence decides every grant.
// Start reads the partition once and never again, so a lost slot stays lost until the token
// bucket is loaded afresh.
type Manager struct {
	id     Identifier
	tokens persistence.SemaphoreTokenManager
	logger log.Logger

	// startupDoneCh is closed when startup ends, whatever the outcome, and Acquire waits on it.
	// startupOnce keeps the close to one, since closing twice panics.
	startupDoneCh chan struct{}
	startupOnce   sync.Once

	// mu guards the state below, never held across a persistence call: concurrent acquires
	// for one owner must race to the conditional write.
	mu sync.Mutex
	// state is the manager's lifecycle stage; Acquire serves only while it is running. Guarded
	// by mu rather than an atomic so Start can check it and install its scan result in one step,
	// or a Stop landing mid-load would be lost.
	state managerState
	// freeList holds the ids of available tokens
	// freeIndex maps an id back to its position in freeList.
	freeList  []int
	freeIndex map[int]int
	// held is the owner_id -> token_id reverse index, mirroring the partition's owner rows.
	held map[string]int
}

// NewManager builds the manager for one bucket. Call Start before Acquire, and discard it if
// Start returns an error.
//
// Every manager must be started or stopped. Acquire blocks until one of the two happens, so a
// manager left in between makes its callers wait.
func NewManager(
	id Identifier,
	tokens persistence.SemaphoreTokenManager,
	logger log.Logger,
) *Manager {
	return &Manager{
		id:            id,
		tokens:        tokens,
		logger:        logger.WithTags(tag.Dynamic("semaphore-bucket", id.String())),
		startupDoneCh: make(chan struct{}),
		freeIndex:     make(map[int]int),
		held:          make(map[string]int),
	}
}

// markStartupDone releases everything waiting for startup. It says startup ended, not that it
// succeeded, so Stop calls it too: an acquire on a manager that never started gets ErrNotReady
// rather than blocking forever.
func (m *Manager) markStartupDone() {
	m.startupOnce.Do(func() { close(m.startupDoneCh) })
}

// Start scans the partition and builds the free-set and the reverse index from what
// is stored there. Call it exactly once, and discard the Manager if it returns an error.
func (m *Manager) Start(ctx context.Context) error {
	defer m.markStartupDone()

	// Move to starting before the scan so a second Start fails here.
	m.mu.Lock()
	if m.state != managerStateCreated {
		m.mu.Unlock()
		return fmt.Errorf("semaphore manager %v has already been started or stopped", m.id)
	}
	m.state = managerStateStarting
	m.mu.Unlock()

	freeList, freeIndex, held, err := m.loadTokenOwnership(ctx)
	if err != nil {
		return fmt.Errorf("load semaphore bucket %v: %w", m.id, err)
	}

	m.mu.Lock()
	if m.state == managerStateStopped {
		m.mu.Unlock()
		// Stop landed during the scan, so this host no longer owns the bucket and the scan
		// result is already stale.
		return fmt.Errorf("semaphore manager %v was stopped while it was loading", m.id)
	}
	m.freeList, m.freeIndex, m.held = freeList, freeIndex, held
	m.state = managerStateRunning
	// Counted under the lock: the assignment above aliases the locals into the manager, so
	// reading their length after the unlock would race a concurrent grant.
	freeSlots, heldSlots := len(m.freeList), len(m.held)
	m.mu.Unlock()

	m.logger.Info("Semaphore manager started",
		tag.LifeCycleStarted,
		tag.Dynamic("free-slots", freeSlots),
		tag.Dynamic("held-slots", heldSlots),
	)
	return nil
}

// Stop gives up the bucket: later acquires get ErrNotReady. A grant
// already past the state check still finishes its write.
func (m *Manager) Stop() {
	m.mu.Lock()
	m.state = managerStateStopped
	m.mu.Unlock()
	m.markStartupDone()
	m.logger.Info("Semaphore manager stopped", tag.LifeCycleStopped)
}

func (m *Manager) isRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == managerStateRunning
}

// Acquire is the entry point: it asks for a slot on behalf of ownerID, and queues a waiter when
// this host cannot find one.
func (m *Manager) Acquire(ctx context.Context, ownerID string) (AcquireResult, error) {
	if ownerID == "" {
		return AcquireResult{}, fmt.Errorf("%w: ownerID is required", ErrInvalidRequest)
	}

	// Wait for startup to finish before reading any state, since the free-set is empty until
	// the scan fills it.
	select {
	case <-m.startupDoneCh:
		// Startup is over. Whether it left the bucket usable is the isRunning check below.
	case <-ctx.Done():
		// The caller's deadline expired while startup was still running. Returning here keeps
		// a slow scan from holding every acquire past the deadline it asked for.
		return AcquireResult{}, ctx.Err()
	}
	if !m.isRunning() {
		return AcquireResult{}, ErrNotReady
	}

	res, err := m.grant(ctx, ownerID)
	if err != nil {
		return AcquireResult{}, err
	}
	if res.Outcome == AcquireOutcomeNoSlot {
		if err := m.enqueue(ctx, ownerID); err != nil {
			return AcquireResult{}, err
		}
	}
	return res, nil
}

// enqueue records ownerID as a waiter on this bucket, to be granted a slot when one frees.
// Acquire calls it when a grant finds the bucket full.
//
// TODO: allocate a task id from the range this host holds and write the waiter row to
// semaphore_tasks.
func (m *Manager) enqueue(ctx context.Context, ownerID string) error {
	return nil
}

// grant tries to get ownerID a slot, up to maxGrantAttempts times:
//   - Check the reverse index first to confirm this owner holds a token
//   - Otherwise draw a random free id and settle it with a conditional write
//   - A write refused as taken, or one that failed on a blip, costs an attempt and is retried
//
// It answers with one of three outcomes:
//   - Acquired: the write applied, and TokenID is the new token.
//   - AlreadyHeld: the owner already had a token, and TokenID is that token.
//   - NoSlot: no free id was left to try.
func (m *Manager) grant(ctx context.Context, ownerID string) (AcquireResult, error) {
	// Check the reverse index first, to see whether this owner already holds a token.
	m.mu.Lock()
	tokenID, ok := m.held[ownerID]
	m.mu.Unlock()

	if ok {
		stillHeld, err := m.confirmHold(ctx, ownerID, tokenID)
		if err != nil {
			return AcquireResult{}, err
		}
		if stillHeld {
			return AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: tokenID}, nil
		}
	}

	// Remembers a write failure that was retried. Without it, running out of attempts would
	// look like a full bucket instead of a store this host could not reach.
	var lastErr error

	for range maxGrantAttempts {
		// Stop as soon as the caller gives up. Letting the write fail instead reports a
		// persistence.TimeoutError, which hides whether the caller ran out of time or the store did.
		if err := ctx.Err(); err != nil {
			return AcquireResult{}, err
		}

		// Draw a free id and reserve it before the write
		tokenID, ok := m.reserve()
		if !ok {
			// Nothing left to draw
			break
		}

		// The conditional batch write, the one authoritative step.
		// A write that does not apply comes back as an outcome, not an error.
		resp, err := m.tokens.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
			DomainID:      m.id.DomainID,
			SemaphoreName: m.id.SemaphoreName,
			Bucket:        m.id.Bucket,
			TokenID:       tokenID,
			OwnerID:       ownerID,
		})
		if err != nil {
			// The error does not say whether the write landed, so put the id back either way.
			// If it did land, the next grant to draw it is refused and drops it.
			// Keeping it out would lose a slot per failed write, emptying the free-set.
			m.unreserve(tokenID)
			if !persistence.IsTransientError(err) {
				return AcquireResult{}, err
			}
			// A blip, retry. Safe even if the write did land,
			// because the owner row is inserted IF NOT EXISTS,
			// so the next attempt reports AlreadyHeld.
			lastErr = err
			continue
		}

		switch resp.Outcome {
		case persistence.SemaphoreGrantApplied:
			m.recordHold(ownerID, tokenID)
			return AcquireResult{Outcome: AcquireOutcomeAcquired, TokenID: tokenID}, nil

		case persistence.SemaphoreGrantSlotTaken:
			// A stale free-set entry: someone else holds this slot.
			// This is the one outcome worth retrying.
			continue

		case persistence.SemaphoreGrantAlreadyHeld:
			// This owner already holds a token, so put the reserved id back and report
			// the token they have.
			m.unreserve(tokenID)
			if resp.HeldToken < 1 {
				// AlreadyHeld must name a token, so a zero means the owner row has no
				// held_token: a corrupt row or a store bug. Recording it
				// would leave this owner failing every later acquire on a token that cannot exist.
				return AcquireResult{}, fmt.Errorf("%w: grant reported an already-held slot without a token for bucket %v", ErrInconsistentState, m.id)
			}
			m.recordHold(ownerID, resp.HeldToken)
			return AcquireResult{Outcome: AcquireOutcomeAlreadyHeld, TokenID: resp.HeldToken}, nil

		default:
			// Unreachable through the nosql store, which rejects unknown outcomes itself,
			// but that is one store's guarantee, not the interface's, so check anyway. An
			// outcome we cannot read says nothing about the slot, so the id goes back.
			m.unreserve(tokenID)
			return AcquireResult{}, fmt.Errorf("%w: unexpected grant outcome %v for bucket %v", ErrInconsistentState, resp.Outcome, m.id)
		}
	}
	if lastErr != nil {
		return AcquireResult{}, lastErr
	}
	return AcquireResult{Outcome: AcquireOutcomeNoSlot}, nil
}

// confirmHold checks whether ownerID still holds tokenID by reading the token row.
// A stale entry is dropped, and its slot returned to the free-set when the row proves the slot unheld.
func (m *Manager) confirmHold(ctx context.Context, ownerID string, tokenID int) (bool, error) {
	resp, err := m.tokens.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID:      m.id.DomainID,
		SemaphoreName: m.id.SemaphoreName,
		Bucket:        m.id.Bucket,
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
		m.dropStaleHold(ownerID, tokenID, false)
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
	m.dropStaleHold(ownerID, tokenID, stillFree)
	return false, nil
}

// loadTokenOwnership reads the bucket partition, following NextPageToken to the end, and
// returns which slots are free and which owner holds what. It builds into locals, so a read
// that fails partway leaves the live state untouched.
func (m *Manager) loadTokenOwnership(ctx context.Context) ([]int, map[int]int, map[string]int, error) {
	free := make(map[int]struct{})
	held := make(map[string]int)
	var skipped int

	var pageToken []byte
	for {
		resp, err := m.tokens.ScanSemaphoreBucket(ctx, &persistence.ScanSemaphoreBucketRequest{
			DomainID:      m.id.DomainID,
			SemaphoreName: m.id.SemaphoreName,
			Bucket:        m.id.Bucket,
			PageSize:      scanPageSize,
			NextPageToken: pageToken,
		})
		if err != nil {
			return nil, nil, nil, err
		}

		for _, row := range resp.Ownerships {
			if row == nil {
				// The nosql store never returns one, but the interface does not promise it,
				// and one nil row would panic the whole host.
				skipped++
				continue
			}
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
				// A type a newer version wrote, or the zero value because nothing set it.
				skipped++
			}
		}

		pageToken = resp.NextPageToken
		if len(pageToken) == 0 {
			break
		}
	}

	if skipped > 0 {
		m.logger.Warn("Skipped semaphore rows of unknown type while loading bucket state",
			tag.Dynamic("skipped-rows", skipped))
	}

	// A slot an owner row claims is not free, whatever its token row said: a scan is not a
	// snapshot, so the two can disagree. This keeps the indexes agreeing on what was read.
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
func (m *Manager) reserve() (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.freeList) == 0 {
		return 0, false
	}
	tokenID := m.freeList[rand.Intn(len(m.freeList))]
	m.removeFromFreeSetLocked(tokenID)
	return tokenID, true
}

// unreserve puts back an id a grant drew but did not take. It may in fact be held, when the
// write's outcome is unknown -- safe, because the conditional write settles the next attempt.
func (m *Manager) unreserve(tokenID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addToFreeSetLocked(tokenID)
}

// recordHold marks ownerID as holding tokenID, mirroring the owner row the write just put
// down. Apart from the startup load, this is the only place the reverse index grows.
func (m *Manager) recordHold(ownerID string, tokenID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.held[ownerID] = tokenID
	// A held slot is never in the free-set. The grant already reserved it, except in the
	// already-held case, which names a token this host never drew.
	m.removeFromFreeSetLocked(tokenID)
}

// dropStaleHold removes a reverse-index entry, returning its slot
// to the free-set when stillFree says the token row proved the slot unheld.
func (m *Manager) dropStaleHold(ownerID string, tokenID int, stillFree bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current, ok := m.held[ownerID]; ok && current == tokenID {
		delete(m.held, ownerID)
	}
	if stillFree {
		m.addToFreeSetLocked(tokenID)
	}
}

// addToFreeSetLocked puts an id back, ignoring one already there. That check keeps freeList free
// of duplicates, which would otherwise let two grants draw the same slot.
func (m *Manager) addToFreeSetLocked(tokenID int) {
	if _, ok := m.freeIndex[tokenID]; ok {
		return
	}
	m.freeIndex[tokenID] = len(m.freeList)
	m.freeList = append(m.freeList, tokenID)
}

// removeFromFreeSetLocked takes one id out in constant time by moving the tail element into its
// slot. Order in freeList carries no meaning, since picks are random.
func (m *Manager) removeFromFreeSetLocked(tokenID int) {
	i, ok := m.freeIndex[tokenID]
	if !ok {
		return
	}
	last := len(m.freeList) - 1
	moved := m.freeList[last]
	m.freeList[i] = moved
	m.freeIndex[moved] = i
	m.freeList = m.freeList[:last]
	// When the removed id was itself the tail, moved == tokenID and the line above just
	// re-added the entry being removed, so this delete has to come last.
	delete(m.freeIndex, tokenID)
}

// freeCount reports how many slots this host believes are open. A hint, not the truth, and
// exists for tests.
func (m *Manager) freeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.freeList)
}
