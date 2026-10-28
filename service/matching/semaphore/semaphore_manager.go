package semaphore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/liveness"
)

const (
	// maxGrantAttempts caps how many tokens one acquire tries, so badly stale free tokens cannot
	// turn one acquire into hundreds of conditional writes.
	maxGrantAttempts = 3

	// emptyTokenID means no token. Token ids start at 1, so it is never a real token.
	emptyTokenID = -1
)

// AcquireResult is the answer to one acquire. TokenID names a token only when Outcome is
// types.SemaphoreAcquireOutcomeAcquired; the zero value, returned alongside every error, reads
// as the invalid outcome.
type AcquireResult struct {
	Outcome types.SemaphoreAcquireOutcome
	TokenID int
}

// ErrNotReady means this manager has stopped: its load failed, it went idle, the bucket moved
// to another host, or the engine shut down. A ServiceBusyError, so callers retry.
var ErrNotReady = &types.ServiceBusyError{Message: "semaphore manager is not ready"}

// managerState gates Acquire. A Manager only moves forward: created to running, or either to
// stopped. Nothing brings a stopped manager back.
type managerState int

const (
	managerStateCreated managerState = iota
	managerStateStarting
	managerStateRunning
	managerStateStopped
)

var _ Manager = (*semaphoreManagerImpl)(nil)

// semaphoreManagerImpl serves one semaphore bucket from this host.
type semaphoreManagerImpl struct {
	// id names the bucket this manager serves.
	id Identifier
	// db makes this bucket's persistence calls.
	db     *bucketDB
	logger log.Logger

	// startupDoneCh is closed when startup ends. Start and Acquire both wait on it.
	startupDoneCh chan struct{}
	// startupOnce keeps the close to one, since closing twice panics.
	startupOnce sync.Once
	// liveness unloads the bucket once it has gone IdleTTL without serving a request.
	liveness *liveness.Liveness
	// stopOnce runs the teardown once; a second caller blocks until it has finished.
	stopOnce sync.Once
	// onStopFn unregisters this manager, so a stopped one is never handed out again.
	onStopFn func(Manager)

	// mu guards every field below, and is never held across a persistence call.
	mu sync.Mutex
	// state is the manager's lifecycle stage.
	state managerState
	// cache is empty until the load replaces it with one built from the scan.
	cache *bucketCache
}

type ManagerParams struct {
	ID       Identifier
	Tokens   persistence.SemaphoreTokenManager
	Metadata persistence.SemaphoreMetadataManager
	// Tagged with the bucket's identity here, so an already-tagged logger duplicates fields.
	Logger log.Logger

	// IdleTTL is how long the manager may go without a request before it unloads itself.
	IdleTTL time.Duration
	// OnStopFn is called once from Stop to unregister this manager.
	// It must not call Stop and must tolerate a manager that is already unregistered.
	OnStopFn   func(Manager)
	TimeSource clock.TimeSource
}

func validateParams(p ManagerParams) error {
	if err := p.ID.validate(); err != nil {
		return err
	}
	if p.Tokens == nil {
		return fmt.Errorf("ManagerParams.Tokens is required")
	}
	if p.Metadata == nil {
		return fmt.Errorf("ManagerParams.Metadata is required")
	}
	if p.Logger == nil {
		return fmt.Errorf("ManagerParams.Logger is required")
	}
	// Rejected rather than passed through: liveness builds a ticker from this and a
	// non-positive interval panics, which would take the host down on a bad config value.
	if p.IdleTTL <= 0 {
		return fmt.Errorf("ManagerParams.IdleTTL must be positive")
	}
	if p.OnStopFn == nil {
		return fmt.Errorf("ManagerParams.OnStopFn is required")
	}
	if p.TimeSource == nil {
		return fmt.Errorf("ManagerParams.TimeSource is required")
	}
	return nil
}

// NewManager builds the manager for one bucket, call Start before Acquire.
func NewManager(p ManagerParams) (Manager, error) {
	if err := validateParams(p); err != nil {
		return nil, &types.BadRequestError{Message: err.Error()}
	}
	m := &semaphoreManagerImpl{
		id:            p.ID,
		db:            &bucketDB{id: p.ID, tokens: p.Tokens, metadata: p.Metadata},
		logger:        p.Logger.WithTags(p.ID.LogTags()...),
		onStopFn:      p.OnStopFn,
		startupDoneCh: make(chan struct{}),
	}
	m.cache, _ = newBucketCache(commonsemaphore.TokenRange{}, nil)
	m.liveness = liveness.NewLiveness(p.TimeSource, p.IdleTTL, func() {
		m.logger.Info("Semaphore manager unloading after no recent requests",
			tag.Dynamic("idle-ttl", p.IdleTTL))
		m.Stop()
	})
	return m, nil
}

// Identifier names the bucket this manager serves.
func (m *semaphoreManagerImpl) Identifier() Identifier {
	return m.id
}

// markStartupDone releases everything waiting on startup. It means startup ended, not that it
// succeeded, so Stop calls it too rather than leave callers blocked on a load that never ran.
func (m *semaphoreManagerImpl) markStartupDone() {
	m.startupOnce.Do(func() { close(m.startupDoneCh) })
}

// Start loads the bucket's in-memory cache from the semaphore's metadata and the partition.
// Only the first caller runs the load. Later callers return at once, and their Acquire waits
// for the load under the caller's own deadline.
func (m *semaphoreManagerImpl) Start(ctx context.Context) error {
	m.mu.Lock()
	found := m.state
	if found == managerStateCreated {
		m.state = managerStateStarting
	}
	m.mu.Unlock()

	switch found {
	case managerStateCreated:
		return m.load(ctx)
	case managerStateStarting, managerStateRunning:
		// A load already in flight is not waited on here: ctx carries the scan's deadline, not
		// the caller's, and waiting under it would hold a caller long after it gave up.
		return nil
	default:
		return ErrNotReady
	}
}

// awaitStartup blocks until the startup load has ended and reports whether it left the bucket
// usable.
func (m *semaphoreManagerImpl) awaitStartup(ctx context.Context) error {
	select {
	case <-m.startupDoneCh:
		// Startup is over. Whether it left the bucket usable is the isRunning check below.
	case <-ctx.Done():
		// The caller's deadline expired while startup was still running. Returning here keeps
		// a slow scan from holding every caller past the deadline it asked for.
		return ctx.Err()
	}
	if !m.isRunning() {
		return ErrNotReady
	}
	return nil
}

// load reads the bucket's token range and builds its cache from the database, then starts
// serving.
func (m *semaphoreManagerImpl) load(ctx context.Context) error {
	// Deferred, so startup ends however this returns and no caller is left waiting. Ending it
	// any earlier would answer ErrNotReady for a bucket that is still loading.
	defer m.markStartupDone()

	m.logger.Info("Semaphore manager starting", tag.LifeCycleStarting)

	tokenRange, err := m.readTokenRange(ctx)
	if err != nil {
		// Stop unregisters, so the next request builds a fresh manager and loads again.
		m.Stop()
		return fmt.Errorf("failed to load semaphore bucket %v: %w", m.id, err)
	}
	cache, err := m.buildCache(ctx, tokenRange)
	if err != nil {
		m.Stop()
		return fmt.Errorf("failed to load semaphore bucket %v: %w", m.id, err)
	}
	freeTokens, heldTokens := cache.freeTokenCount(), cache.heldTokenCount()

	m.mu.Lock()
	if m.state == managerStateStopped {
		m.mu.Unlock()
		// Stop landed during the scan, so this host no longer owns the bucket and the scan
		// result is already stale.
		return fmt.Errorf("semaphore manager %v was stopped while it was loading: %w", m.id, ErrNotReady)
	}
	m.cache = cache
	m.state = managerStateRunning
	// Armed here, not earlier or later. Earlier, the idle clock would run during the scan, so a
	// slow load could unload the bucket before its first request. Later, outside the lock, a
	// concurrent Stop could finish first and leave the idle clock running with nothing to stop it.
	m.liveness.Start()
	m.mu.Unlock()

	m.logger.Info("Semaphore manager started",
		tag.LifeCycleStarted,
		tag.Dynamic("free-tokens", freeTokens),
		tag.Dynamic("held-tokens", heldTokens),
	)
	return nil
}

// readTokenRange reads the semaphore's metadata and returns the token ids this bucket owns.
func (m *semaphoreManagerImpl) readTokenRange(ctx context.Context) (commonsemaphore.TokenRange, error) {
	meta, err := m.db.getMetadata(ctx)
	if err != nil {
		return commonsemaphore.TokenRange{}, err
	}
	tokenRange, err := commonsemaphore.BucketTokenRange(meta.Size, meta.BucketSize, m.id.Bucket)
	if errors.Is(err, commonsemaphore.ErrBucketOutOfRange) {
		return commonsemaphore.TokenRange{}, &types.BadRequestError{Message: fmt.Sprintf("semaphore bucket %v: %v", m.id, err)}
	}
	if err != nil {
		// The stored size or bucket size is below 1, so the tokens cannot be split into buckets.
		// CreateSemaphore never writes such values, so the metadata row is corrupt, not the
		// request.
		return commonsemaphore.TokenRange{}, &types.InternalServiceError{Message: fmt.Sprintf("semaphore metadata for domain %v, semaphore %v is invalid: %v", m.id.DomainID, m.id.SemaphoreName, err)}
	}
	return tokenRange, nil
}

// buildCache scans the bucket's rows in semaphore_tokens and builds a new cache for tokenRange
// from them.
func (m *semaphoreManagerImpl) buildCache(ctx context.Context, tokenRange commonsemaphore.TokenRange) (*bucketCache, error) {
	rows, err := m.db.scanTokenRows(ctx)
	if err != nil {
		return nil, err
	}
	cache, skipped := newBucketCache(tokenRange, rows)
	if skipped.unreadable > 0 {
		m.logger.Warn("Skipped unreadable semaphore rows while loading the bucket",
			tag.Dynamic("skipped-rows", skipped.unreadable))
	}
	if skipped.outOfRange > 0 {
		m.logger.Warn("Skipped semaphore rows for tokens this bucket does not own",
			tag.Dynamic("skipped-rows", skipped.outOfRange),
			tag.Dynamic("first-token", tokenRange.First),
			tag.Dynamic("last-token", tokenRange.Last()))
	}
	return cache, nil
}

// Stop shuts the manager down: later acquires get ErrNotReady. A grant
// already past the state check still finishes its write.
func (m *semaphoreManagerImpl) Stop() {
	m.stopOnce.Do(func() {
		m.mu.Lock()
		m.state = managerStateStopped
		m.mu.Unlock()

		// Unregistered before the rest of the teardown, so nothing is handed a manager that can
		// no longer serve.
		m.onStopFn(m)
		m.liveness.Stop()

		m.markStartupDone()
		m.logger.Info("Semaphore manager stopped", tag.LifeCycleStopped)
	})
}

func (m *semaphoreManagerImpl) isRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == managerStateRunning
}

// currentCache returns the bucket's cache, which is empty until the load finishes.
func (m *semaphoreManagerImpl) currentCache() *bucketCache {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cache
}

// Acquire is the entry point: it asks for a token on behalf of ownerID, and queues a waiter when
// this host cannot find one.
func (m *semaphoreManagerImpl) Acquire(ctx context.Context, ownerID string) (AcquireResult, error) {
	m.liveness.MarkAlive()
	if err := m.awaitStartup(ctx); err != nil {
		return AcquireResult{}, err
	}

	res, err := m.grant(ctx, ownerID)
	if err != nil {
		return AcquireResult{}, err
	}
	if res.Outcome == types.SemaphoreAcquireOutcomeNoSlot {
		// TODO: Once enqueue writes a waiter, answer QUEUED here, since NO_SLOT means nothing is
		// queued. Before enqueuing, refresh the free tokens and retry the grant once: a stale cache
		// can answer NO_SLOT while tokens are free, and a waiter queued on that answer may never be
		// woken.
		if err := m.enqueue(ctx, ownerID); err != nil {
			return AcquireResult{}, err
		}
	}
	return res, nil
}

// grant answers Acquired with the token ownerID already holds, so a retried acquire gets the same
// token. Otherwise it grants a free token, or answers NoSlot if it cannot.
func (m *semaphoreManagerImpl) grant(ctx context.Context, ownerID string) (AcquireResult, error) {
	cache := m.currentCache()
	tokenID, err := m.confirmCachedToken(ctx, cache, ownerID)
	if err != nil {
		return AcquireResult{}, err
	}
	if tokenID != emptyTokenID {
		return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: tokenID}, nil
	}

	tokenID, err = m.grantFreeToken(ctx, cache, ownerID)
	if err != nil {
		return AcquireResult{}, err
	}
	if tokenID == emptyTokenID {
		return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeNoSlot}, nil
	}
	return AcquireResult{Outcome: types.SemaphoreAcquireOutcomeAcquired, TokenID: tokenID}, nil
}

// confirmCachedToken returns ownerID's token from the cache, confirmed by the database while holding
// the token's lock.
func (m *semaphoreManagerImpl) confirmCachedToken(ctx context.Context, cache *bucketCache, ownerID string) (int, error) {
	lockedToken, err := cache.lockHeldToken(ctx, ownerID)
	if err != nil || lockedToken == nil {
		return emptyTokenID, err
	}
	defer lockedToken.unlock()

	row, err := m.db.getTokenRow(ctx, lockedToken.id())
	if err != nil {
		return emptyTokenID, err
	}
	if row == nil {
		m.logger.Warn("Semaphore owner's token has no token row", tag.Dynamic("token-id", lockedToken.id()))
		lockedToken.markUnavailable()
		return emptyTokenID, nil
	}
	switch row.Holder {
	case ownerID:
		lockedToken.markHeld(ownerID)
		return lockedToken.id(), nil
	case "":
		lockedToken.markFree()
	default:
		lockedToken.markUnavailable()
	}
	return emptyTokenID, nil
}

// grantFreeToken grants ownerID a free token, trying up to maxGrantAttempts tokens.
func (m *semaphoreManagerImpl) grantFreeToken(ctx context.Context, cache *bucketCache, ownerID string) (int, error) {
	for range maxGrantAttempts {
		// Return the caller's ctx error. A write with an ended ctx is not sent, and comes back as a
		// persistence.TimeoutError, or an InternalServiceError if ctx was cancelled.
		if err := ctx.Err(); err != nil {
			return emptyTokenID, err
		}
		tokenID, alreadyHeld, err := m.tryGrantFreeToken(ctx, cache, ownerID)
		if err != nil {
			return emptyTokenID, err
		}
		if alreadyHeld {
			m.markAlreadyHeldToken(cache, tokenID, ownerID)
			return tokenID, nil
		}
		if tokenID != emptyTokenID {
			return tokenID, nil
		}
	}
	return emptyTokenID, nil
}

// tryGrantFreeToken tries once to grant ownerID a free token. alreadyHeld means ownerID already holds
// the returned token and the caller must mark it.
func (m *semaphoreManagerImpl) tryGrantFreeToken(ctx context.Context, cache *bucketCache, ownerID string) (tokenID int, alreadyHeld bool, err error) {
	lockedToken := cache.tryLockFreeToken()
	if lockedToken == nil {
		return emptyTokenID, false, nil
	}
	defer lockedToken.unlock()

	tokenID = lockedToken.id()

	// The cache holds only tokens in the range, but a grant on a token outside it would create a
	// new row and let in one owner more than the semaphore's size.
	if !cache.tokenRange.Contains(tokenID) {
		m.logger.Error("Semaphore grant picked a token this bucket does not own",
			tag.Dynamic("token-id", tokenID),
			tag.Dynamic("first-token", cache.tokenRange.First),
			tag.Dynamic("last-token", cache.tokenRange.Last()))
		return emptyTokenID, false, &types.InternalServiceError{Message: fmt.Sprintf("token %d is outside the range of bucket %v", tokenID, m.id)}
	}

	resp, err := m.db.grantToken(ctx, tokenID, ownerID)
	if err != nil {
		// tokenID stays free. If the write landed, the next grant on it fails and marks it unavailable.
		return emptyTokenID, false, err
	}
	switch resp.Outcome {
	case persistence.SemaphoreGrantApplied: // ownerID now holds the token
		lockedToken.markHeld(ownerID)
		return tokenID, false, nil

	case persistence.SemaphoreGrantSlotTaken: // another owner holds the token, so the cache was stale
		lockedToken.markUnavailable()
		return emptyTokenID, false, nil

	case persistence.SemaphoreGrantAlreadyHeld: // ownerID already holds resp.HeldToken
		if !cache.tokenRange.Contains(resp.HeldToken) {
			// The owner row names a token this bucket does not own, so it is corrupt. Every acquire
			// for this owner fails this way until the owner row is fixed.
			return emptyTokenID, false, &types.InternalServiceError{Message: fmt.Sprintf("grant reported AlreadyHeld with token %d, outside tokens %d to %d of bucket %v", resp.HeldToken, cache.tokenRange.First, cache.tokenRange.Last(), m.id)}
		}
		if resp.HeldToken == tokenID {
			// The cache had ownerID's own token as free, for example after a grant that timed out
			// but landed. Mark it before unlocking, so no other call sees it as free.
			lockedToken.markHeld(ownerID)
			return tokenID, false, nil
		}
		// tokenID stays free.
		return resp.HeldToken, true, nil

	default:
		// Unreachable through the nosql store, which rejects unknown outcomes itself, but that is
		// one store's guarantee, not the interface's, so check anyway.
		return emptyTokenID, false, &types.InternalServiceError{Message: fmt.Sprintf("unexpected grant outcome %v for bucket %v", resp.Outcome, m.id)}
	}
}

// markAlreadyHeldToken marks tokenID as held by ownerID. If tokenID is locked, it skips the mark,
// since the call holding the lock will mark it.
func (m *semaphoreManagerImpl) markAlreadyHeldToken(cache *bucketCache, tokenID int, ownerID string) {
	lockedToken := cache.tryLockToken(tokenID)
	if lockedToken == nil {
		return
	}
	defer lockedToken.unlock()
	lockedToken.markHeld(ownerID)
}

// enqueue records ownerID as a waiter on this bucket, to be granted a token when one frees.
// Acquire calls it when a grant finds the bucket full.
//
// TODO: Not built yet, so it queues nothing. It will write a waiter row to semaphore_tasks under
// the bucket's range_id, with a TTL taken from the acquire deadline, and a background reader will
// grant waiters in order as tokens are released.
func (m *semaphoreManagerImpl) enqueue(ctx context.Context, ownerID string) error {
	return nil
}
